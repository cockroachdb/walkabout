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
	"bytes"
	"fmt"
	"go/format"
	"go/types"
	"sort"
	"strings"
	"text/template"

	"github.com/cockroachdb/walkabout/gen/templates"
	"github.com/pkg/errors"
)

var allTemplates = make(map[string]*template.Template)

// Register all templates to be generated.
func init() {
	for name, src := range templates.TemplateSources {
		allTemplates[name] = template.Must(template.New(name).Funcs(funcMap).Parse(src))
	}
}

// implementor is returned by the Implementors function.
type implementor struct {
	Intf       namedInterfaceType
	Actual     visitableType
	Underlying namedStruct
}

// TypeId generates a reasonable description of a type. Generated tokens
// are attached to the underlying visitation so that we can be sure
// to actually generate them in a subsequent pass.
//   *Foo -> FooPtr
//   []Foo -> FooSlice
//   []*Foo -> FooPtrSlice
//   *[]Foo -> FooSlicePtr
func typeId(i visitableType) string {
	org := i
	suffix := ""
	for {
		switch t := i.(type) {
		case pointerType:
			suffix = "Ptr" + suffix
			i = t.Elem
		case namedSliceType:
			suffix = "Slice" + suffix
			i = t.Elem
		case namedVisitableType:
			i = t.Underlying
		default:
			ret := fmt.Sprintf("%sType%s%s", t.Visitation().Intf, t, suffix)
			org.Visitation().TypeIds[ret] = org
			return ret
		}
	}
}

// funcMap contains a map of functions that can be called from within
// the templates.
var funcMap = template.FuncMap{
	// Implementors returns a sortable map of implementors. The first
	// element in the pair is a type which implements the interface. The
	// second element is the underlying struct type (which might be the
	// same as the first element).
	"Implementors": func(t namedInterfaceType) map[string]implementor {
		ret := make(map[string]implementor)
		for _, s := range t.Visitation().Structs {
			if types.Implements(s.Named, t.Interface) {
				ret[s.String()] = implementor{t, s, s}
			}
			if types.Implements(types.NewPointer(s.Named), t.Interface) {
				p := pointerType{s}
				ret[s.String()+"*"] = implementor{t, p, s}
			}
		}
		return ret
	},
	// Intfs returns a sortable map of all interface types used.
	"Intfs": func(v *visitation) map[string]namedInterfaceType {
		ret := make(map[string]namedInterfaceType)
		for _, t := range v.TypeIds {
			if s, ok := t.Implementation().(namedInterfaceType); ok {
				ret[t.String()] = s
			}
		}
		return ret
	},
	// IsPointer returns true if the type is a pointer or resolves
	// to a pointer type.
	"IsPointer": func(v visitableType) bool {
		for {
			switch tv := v.(type) {
			case namedVisitableType:
				v = tv.Underlying
			case pointerType:
				return true
			default:
				return false
			}
		}
	},
	// Slices returns a sortable map of all slice types used.
	"Slices": func(v *visitation) map[string]namedSliceType {
		ret := make(map[string]namedSliceType)
		for _, t := range v.TypeIds {
			if s, ok := t.Implementation().(namedSliceType); ok {
				ret[t.String()] = s
			}
		}
		return ret
	},
	// Package returns the name of the package we're working in.
	"Package": func(v *visitation) string { return v.pkg.Name() },
	// Pointers returns a sortable map of all pointer types used.
	"Pointers": func(v *visitation) map[string]pointerType {
		ret := make(map[string]pointerType)
		for _, t := range v.TypeIds {
			if ptr, ok := t.Implementation().(pointerType); ok {
				ret[t.String()] = ptr
			}
		}
		return ret
	},
	// SourceFile returns the name of the file that defines the interface.
	"SourceFile": func(v *visitation) string {
		return v.gen.fileSet.Position(v.pkg.Scope().Lookup(v.Intf.String()).Pos()).Filename
	},
	// t returns an un-exported named based on the visitable interface name.
	"t": func(v *visitation, name string) string {
		intfName := v.Intf.String()
		return fmt.Sprintf("%s%s%s", strings.ToLower(intfName[:1]), intfName[1:], name)
	},
	// T returns an exported named based on the visitable interface name.
	"T": func(v *visitation, name string) string {
		return fmt.Sprintf("%s%s", v.Intf, name)
	},
	// TypeId generates a reasonable description of a type.
	"TypeId": typeId,
}

// generateAPI is the main code-generation function. It evaluates
// the embedded template and then calls go/format on the resulting
// code.
func (v *visitation) generateAPI() error {

	// Parse each template and sort the keys.
	sorted := make([]string, 0, len(allTemplates))
	var err error
	for key := range allTemplates {
		sorted = append(sorted, key)
	}
	sort.Strings(sorted)

	// Execute each template in sorted order.
	var buf bytes.Buffer
	for _, key := range sorted {
		if err := allTemplates[key].ExecuteTemplate(&buf, key, v); err != nil {
			return errors.Wrap(err, key)
		}
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		println(buf.String())
		return err
	}

	outName := strings.ToLower(v.Intf.String()) + "_walkabout.g"
	if v.inTest {
		outName += "_test"
	}
	outName += ".go"
	out, err := v.gen.writeCloser(outName)
	if err != nil {
		return err
	}

	_, err = out.Write(formatted)
	if x := out.Close(); x != nil && err == nil {
		err = x
	}
	return err
}
