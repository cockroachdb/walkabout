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

import "go/types"

// visitation encapsulates the state of generating a single
// visitable interface. This type is used extensively by the
// API template and exposes many convenience functions to keep
// the template simple.
type visitation struct {
	gen *generation
	// The visitable interface.
	Intf    namedInterfaceType
	inTest  bool
	pkg     *types.Package
	Structs map[string]namedStruct
	// TypeIds collects all referenced types, indexed by their
	// generated TypeId name.
	TypeIds map[string]visitableType
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
				v.visitableType(obj.Type())
			}
		}
	}
}

// visitableType extracts the type information that we care about
// from typ. This handles named and anonymous types that are visitable.
func (v *visitation) visitableType(typ types.Type) (visitableType, bool) {
	switch t := typ.(type) {
	case *types.Named:
		// Ignore un-exported types.
		if !t.Obj().Exported() {
			return nil, false
		}
		switch u := t.Underlying().(type) {
		case *types.Struct:
			if s, ok := v.Structs[t.Obj().Name()]; ok {
				return s, true
			}

			var mode refMode
			if types.Implements(t, v.Intf.Interface) {
				mode = byValue
			} else if types.Implements(types.NewPointer(t), v.Intf.Interface) {
				mode = byRef
			} else {
				return nil, false
			}

			ret := namedStruct{
				Named:    t,
				Struct:   u,
				implMode: mode,
				v:        v,
			}
			v.Structs[t.Obj().Name()] = ret
			v.TypeIds[typeId(ret)] = ret
			return ret, true

		case *types.Interface:
			if types.Implements(u, v.Intf.Interface) {
				ret := namedInterfaceType{
					Named:     t,
					Interface: u,
					v:         v,
				}
				v.TypeIds[typeId(ret)] = ret
				return ret, true
			}

		//case *types.Slice:
		//	if s, ok := v.visitableType(u); ok {
		//		st, ok := s.(namedSliceType)
		//		return st, ok
		//	}
		default:
			// Any other named visitable type: type Foos []Foo
			if under, ok := v.visitableType(u); ok {
				return namedVisitableType{Named: t, Underlying: under}, true
			}
		}

	case *types.Pointer:
		if elem, ok := v.visitableType(t.Elem()); ok {
			ret := pointerType{Elem: elem}
			v.TypeIds[typeId(ret)] = ret
			return ret, true
		}

	case *types.Slice:
		if elem, ok := v.visitableType(t.Elem()); ok {
			ret := namedSliceType{Elem: elem}
			v.TypeIds[typeId(ret)] = ret
			return ret, true
		}
	}
	return nil, false
}

// String is for debugging use only.
func (v *visitation) String() string {
	return v.Intf.String()
}
