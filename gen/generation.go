// Copyright 2018 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License. See the AUTHORS file
// for names of contributors.

package gen

import (
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

type config struct {
	dir string
	// By default, we don't fully type-check the input. This can be
	// enabled for testing to validate generated code.
	fullCheck bool
	// If present, overrides the output file name.
	outFile string
	// Include all types reachable from visitable types that implement
	// the root visitable interface.
	reachable bool
	// The requested type names.
	typeNames []string
	// If present, unifies all specified interfaces under a single
	// visitable interface with this name.
	union string
}

// generation represents an entire run of the code generator. The
// overall flow is broken up into various stages, which can be seen in
// Execute().
type generation struct {
	config

	astFiles []*ast.File
	// Allows additional files to be added to the parse phase for testing.
	extraTestSource map[string][]byte
	fileSet         *token.FileSet
	pkg             *types.Package
	// The sources being considered.
	source *build.Package
	// Stores the executed visitation for testing.
	visitation  *visitation
	writeCloser func(name string) (io.WriteCloser, error)
}

// newGeneration constructs a generation which will look for the
// named interface types in the given directory.
func newGeneration(cfg config) (*generation, error) {
	if len(cfg.typeNames) > 1 && cfg.union == "" {
		return nil, errors.New("multiple input types can only be used with --union")
	}
	if cfg.reachable && cfg.union == "" {
		return nil, errors.New("--reachable can only be used with --union")
	}
	return &generation{
		fileSet: token.NewFileSet(),
		config:  cfg,
		writeCloser: func(name string) (io.WriteCloser, error) {
			if name == "-" {
				return os.Stdout, nil
			} else {
				return os.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
			}
		},
	}, nil
}

// Execute runs the complete code-generation cycle.
func (g *generation) Execute() error {
	// Scan the input directory for source files.
	if err := g.importSources(); err != nil {
		return err
	}

	// Assemble source files, which may include files injected
	// when testing.
	files := append(g.source.GoFiles, g.source.TestGoFiles...)
	if len(g.extraTestSource) > 0 {
		// Mix in extra sources.
		if err := g.addSource(g.extraTestSource); err != nil {
			return err
		}
		// Filter our input sources if an input file is being overridden.
		filtered := files[:0]
		for _, file := range files {
			if g.extraTestSource[file] == nil {
				filtered = append(filtered, file)
			}
		}
		files = filtered
	}

	if err := g.parseFiles(files); err != nil {
		return err
	}
	if err := g.typeCheck(); err != nil {
		return err
	}

	v := &visitation{
		gen:              g,
		includeReachable: g.config.reachable,
		pkg:              g.pkg,
		Types:            make(map[TypeId]visitableType),
		SourceTypes:      make(map[SourceName]visitableType),
	}
	g.visitation = v

	if g.config.union != "" {
		v.Root = namedInterfaceType{
			Union: g.union,
			v:     v,
		}
	}

	// Resolve all of the specified type names to an interface or struct.
	for _, name := range g.typeNames {
		obj := v.pkg.Scope().Lookup(name)
		if obj == nil {
			return errors.Errorf("unknown type %q", name)
		}
		if named, ok := obj.Type().(*types.Named); ok {
			var filter visitableType
			switch u := named.Underlying().(type) {
			case *types.Interface:
				// The default case, we expect to see an interface type.
				intf := namedInterfaceType{
					Named:     named,
					Interface: u,
					v:         v,
				}
				if g.union == "" && len(g.typeNames) == 1 {
					v.Root = intf
				}
				filter = intf
			case *types.Struct:
				// If we're generating the visitable interface with --union,
				// we'll allow structs to be specified, too.
				if g.union == "" {
					return errors.Errorf("structs may only be used with --union")
				}
				filter = namedStruct{
					Named:  named,
					Struct: u,
					v:      v,
				}
			default:
				return errors.Errorf("%q is neither a struct nor an interface", name)
			}

			v.filters = append(v.filters, filter)
		}
	}

	v.populateGeneratedTypes()

	return v.generateAPI()
}

func (g *generation) addSource(source map[string][]byte) error {
	for name, data := range source {
		astFile, err := parser.ParseFile(g.fileSet, name, string(data), 0 /* Mode */)
		if err != nil {
			return err
		}
		g.astFiles = append(g.astFiles, astFile)
	}
	return nil
}

// importSources finds files on disk that we want to read. The generated
// code has a build tag added so that we can ignore it in this phase.
// We don't want out-of-sync generated code to break regeneration.
func (g *generation) importSources() error {
	ctx := build.Default
	// Don't re-import code that we've generated.
	ctx.BuildTags = append(ctx.BuildTags, "walkaboutAnalysis")

	pkg, err := ctx.ImportDir(g.dir, 0)
	if err != nil {
		return err
	}
	g.source = pkg
	return nil
}

// parseFiles runs the golang parser to produce AST elements.
func (g *generation) parseFiles(files []string) error {
	for _, path := range files {
		astFile, err := parser.ParseFile(g.fileSet, filepath.Join(g.dir, path), nil, 0 /* Mode */)
		if err != nil {
			return err
		}
		g.astFiles = append(g.astFiles, astFile)
	}
	return nil
}

// typeCheck will run the go type checker over the parsed imports. This
// method is lenient, unless g.fullCheck has been set. The leniency
// helps in cases where code in the package that we're parsing depends
// on code that may not yet be generated (e.g. make clean).
func (g *generation) typeCheck() error {
	// We prefer to use the already-compiled and cached information
	// available from the go compiler. We switch to source-based mode
	// when we're injecting generated sources as part of the test suite.
	importerName := "gc"
	if g.fullCheck {
		importerName = "source"
	}
	cfg := &types.Config{
		Importer: importer.For(importerName, nil),
	}
	if !g.fullCheck {
		cfg.DisableUnusedImportCheck = true
		// Just drain errors from the checker.
		cfg.Error = func(err error) {}
		cfg.IgnoreFuncBodies = true
	}
	var err error
	g.pkg, err = cfg.Check(g.dir, g.fileSet, g.astFiles, nil /* info */)
	if err != nil && g.fullCheck {
		return err
	}
	return nil
}
