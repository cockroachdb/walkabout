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
	"fmt"
	"go/types"
	"strings"

	"github.com/pkg/errors"
)

// TypeID is a constant string to be emitted in the generated code.
type TypeID string

func (s TypeID) String() string { return string(s) }

// SourceName is the name of a type as it appears in the input source.
type SourceName string

func (s SourceName) String() string { return string(s) }

// visitation encapsulates the state of generating a single
// visitable interface. This type is used extensively by the
// API template and exposes many convenience functions to keep
// the template simple.
type visitation struct {
	// The interfaces that are used to select structs to be included
	// in the visitation.
	filters []visitableType
	gen     *generation
	// If true, any struct that is in the same package will be eligible
	// for inclusion.
	includeReachable bool
	inTest           bool
	packagePath      string
	// The root visitable interface.
	Root namedInterfaceType
	// types collects all referenced types, indexed by their type id.
	Types       map[TypeID]visitableType
	SourceTypes map[SourceName]visitableType
}

func (v *visitation) findSeedTypes(scopes []*types.Scope) error {
	g := v.gen

	// Resolve all of the specified type names to an interface or struct.
name:
	for _, name := range g.typeNames {
		for _, scope := range scopes {
			obj := scope.Lookup(name)
			if obj == nil {
				continue
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

				// If the type refers to anything defined in a test file, generate
				// into a _test.go file as well.
				if obj.Pos().IsValid() {
					position := g.fileSet.Position(obj.Pos())
					if strings.HasSuffix(position.Filename, "_test.go") {
						v.inTest = true
					}
				}
				continue name
			}
		}
		return errors.Errorf("unknown type %q", name)
	}
	return nil
}

// populateGeneratedTypes finds top-level types that we will generate
// additional methods for.
func (v *visitation) populateGeneratedTypes(scopes []*types.Scope) {
	// Bootstrap our type info by looking for named struct and interface
	// types in the package.
	for _, scope := range scopes {
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if named, ok := obj.Type().(*types.Named); ok {
				switch named.Underlying().(type) {
				case *types.Struct, *types.Interface:
					v.visitableType(obj.Type(), false)
				}
			}
		}
	}
}

// ensureTypeID ensures that the types map contains an entry
// for the given type.
func (v *visitation) ensureTypeID(i visitableType) TypeID {
	ret := v.typeID(i)
	if _, found := v.Types[ret]; !found {
		v.Types[ret] = i
	}
	return ret
}

// typeID generates a reasonable description of a type. Generated tokens
// are attached to the underlying visitation so that we can be sure
// to actually generate them in a subsequent pass.
//   *Foo -> FooPtr
//   []Foo -> FooSlice
//   []*Foo -> FooPtrSlice
//   *[]Foo -> FooSlicePtr
func (v *visitation) typeID(i visitableType) TypeID {
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
			return TypeID(fmt.Sprintf("%sType%s%s", v.Root, t, suffix))
		}
	}
}

// visitableType extracts the type information that we care about
// from typ. This handles named and anonymous types that are visitable.
func (v *visitation) visitableType(typ types.Type, isReachable bool) (visitableType, bool) {
	switch t := typ.(type) {
	case *types.Named:
		// Ignore un-exported types or those from other packages.
		if !t.Obj().Exported() || t.Obj().Pkg().Path() != v.packagePath {
			return nil, false
		}

		sourceName := SourceName(t.Obj().Name())
		if ret, ok := v.SourceTypes[sourceName]; ok {
			return ret, true
		}

		switch u := t.Underlying().(type) {
		case *types.Struct:
			ok := v.includeReachable && isReachable

			if !ok {
			outer:
				for _, filter := range v.filters {
					switch tFilter := filter.(type) {
					case namedStruct:
						if types.Identical(u, tFilter.Struct) {
							ok = true
							break outer
						}
					case namedInterfaceType:
						if types.Implements(t, tFilter.Interface) ||
							types.Implements(types.NewPointer(t), tFilter.Interface) {
							ok = true
							break outer
						}
					}
				}
			}

			if ok {
				ret := namedStruct{
					Named:  t,
					Struct: u,
					v:      v,
				}
				v.SourceTypes[sourceName] = ret
				v.ensureTypeID(ret)
				ret.Fields()
				return ret, true
			}

		case *types.Interface:
			ok := v.includeReachable && isReachable
			if !ok {
				for _, filter := range v.filters {
					if filterIntf, isIntf := filter.(namedInterfaceType); isIntf {
						if types.Implements(u, filterIntf.Interface) {
							ok = true
							break
						}
					}
				}
			}
			if ok {
				ret := namedInterfaceType{
					Named:     t,
					Interface: u,
					v:         v,
				}
				v.SourceTypes[sourceName] = ret
				v.ensureTypeID(ret)

				// If we've added an interface because it's reachable, we need
				// to also go back and look for any structs that may be implied
				// by the interface.
				if isReachable && v.includeReachable {
					v.filters = append(v.filters, ret)
					v.populateGeneratedTypes([]*types.Scope{t.Obj().Parent()})
				}

				return ret, true
			}

		default:
			// Any other named visitable type: type Foos []Foo
			if under, ok := v.visitableType(u, isReachable); ok {
				ret := namedVisitableType{Named: t, Underlying: under}
				v.SourceTypes[sourceName] = ret
				return ret, true
			}
		}

	case *types.Pointer:
		if elem, ok := v.visitableType(t.Elem(), isReachable); ok {
			return pointerType{Elem: elem}, true
		}

	case *types.Slice:
		if elem, ok := v.visitableType(t.Elem(), isReachable); ok {
			return namedSliceType{Elem: elem}, true
		}
	}
	return nil, false
}

// String is for debugging use only.
func (v *visitation) String() string {
	return v.Root.String()
}
