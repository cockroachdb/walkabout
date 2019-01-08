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
	"go/token"
	"go/types"
	"io"
	"os"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/packages"
)

type config struct {
	dir string
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

	// Allows additional files to be added to the parse phase for testing.
	extraTestSource map[string][]byte
	fileSet         token.FileSet
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
		config: cfg,
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
	// This will return multiple packages.Package if we're also loading
	// test files. Note that the error here is whether or not the Load()
	// was able to perform its work. The underlying source may still have
	// syntax/type errors, but we ignore that in case of a "make clean"
	// situation, where we're likely to see code that depends on generated
	// code.
	pkgs, err := packages.Load(g.packageConfig(), ".")
	if err != nil {
		return err
	}

	v := &visitation{
		gen:              g,
		includeReachable: g.config.reachable,
		packagePath:      pkgs[0].PkgPath,
		Types:            make(map[TypeId]visitableType),
		SourceTypes:      make(map[SourceName]visitableType),
	}
	g.visitation = v

	// Synthesize a union interface, if configured.
	if g.config.union != "" {
		v.Root = namedInterfaceType{
			Union: g.union,
			v:     v,
		}
	}

	scopes := make([]*types.Scope, len(pkgs))
	for idx, pkg := range pkgs {
		scopes[idx] = pkg.Types.Scope()
	}

	if err := v.findSeedTypes(scopes); err != nil {
		return err
	}
	v.populateGeneratedTypes(scopes)
	return v.generateAPI()
}

func (g *generation) packageConfig() *packages.Config {
	return &packages.Config{
		Dir:     g.dir,
		Fset:    &g.fileSet,
		Mode:    packages.LoadTypes,
		Overlay: g.extraTestSource,
		Tests:   true,
	}
}
