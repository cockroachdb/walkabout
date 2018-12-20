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
)

// TypeId is a constant string to be emitted in the generated code.
type TypeId string

func (s TypeId) String() string { return string(s) }

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
	pkg              *types.Package
	// The root visitable interface.
	Root namedInterfaceType
	// types collects all referenced types, indexed by their type id.
	Types       map[TypeId]visitableType
	SourceTypes map[SourceName]visitableType
}

// populateGeneratedTypes finds top-level types that we will generate
// additional methods for.
func (v *visitation) populateGeneratedTypes() {
	g := v.gen
	scope := g.pkg.Scope()

	// Bootstrap our type info by looking for named struct and interface
	// types in the package.
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

// ensureTypeId ensures that the types map contains an entry
// for the given type.
func (v *visitation) ensureTypeId(i visitableType) TypeId {
	ret := v.typeId(i)
	if _, found := v.Types[ret]; !found {
		v.Types[ret] = i
	}
	return ret
}

// typeId generates a reasonable description of a type. Generated tokens
// are attached to the underlying visitation so that we can be sure
// to actually generate them in a subsequent pass.
//   *Foo -> FooPtr
//   []Foo -> FooSlice
//   []*Foo -> FooPtrSlice
//   *[]Foo -> FooSlicePtr
func (v *visitation) typeId(i visitableType) TypeId {
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
			return TypeId(fmt.Sprintf("%sType%s%s", v.Root, t, suffix))
		}
	}
}

// visitableType extracts the type information that we care about
// from typ. This handles named and anonymous types that are visitable.
func (v *visitation) visitableType(typ types.Type, isReachable bool) (visitableType, bool) {
	switch t := typ.(type) {
	case *types.Named:
		// Ignore un-exported types.
		if !t.Obj().Exported() {
			return nil, false
		}
		sourceName := SourceName(t.Obj().Name())
		if ret, ok := v.SourceTypes[sourceName]; ok {
			return ret, true
		}

		switch u := t.Underlying().(type) {
		case *types.Struct:
			ok := v.includeReachable && isReachable && t.Obj().Pkg() == v.pkg

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
				v.ensureTypeId(ret)
				ret.Fields()
				return ret, true
			}

		case *types.Interface:
			ok := v.includeReachable && isReachable && t.Obj().Pkg() == v.pkg
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
				v.ensureTypeId(ret)
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
