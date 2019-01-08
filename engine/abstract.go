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

package engine

// This file contains base definitions for creating abstract accessors
// around user-defined types.

import (
	"fmt"
	"reflect"
)

// Abstract allows a visitable object to be manipulated as an abstract
// tree of nodes. This should be enclosed in a type-safe wrapper.
// An Abstract should only ever represent a struct or a slice;
// pointers and interfaces should be resolved to their respective
// targets before being wrapped in an Abstract.
type Abstract struct {
	engine   *Engine
	typeData *TypeData
	value    Ptr
}

// ChildAt returns the nth field or slice element. If that value is
// a pointer or an interface, it is dereferenced before returning.
// Nil pointers, interfaces, and empty slices will return nil here.
func (a *Abstract) ChildAt(index int) *Abstract {
	var chaseType *TypeData
	var chaseValue Ptr

	// First, we select the child value.
	switch a.typeData.Kind {
	case KindStruct:
		f := a.typeData.Fields[index]
		chaseType = f.targetData
		chaseValue = Ptr(uintptr(a.value) + f.Offset)
	case KindSlice:
		header := (*reflect.SliceHeader)(a.value)
		if index < 0 || index >= header.Len {
			panic(fmt.Errorf("index out of range: %d", index))
		}
		chaseType = a.typeData.elemData
		chaseValue = Ptr(header.Data + uintptr(index)*chaseType.SizeOf)
	default:
		// We should never have returned an Abstract wrapping anything other
		// than a struct or a slice. Getting here indicates a problem
		// with code-generation.
		panic(fmt.Errorf("unimplemented: %d", a.typeData.Kind))
	}

	// Now, we traverse pointers and interfaces until we arrive at
	// a struct or a slice.
	for {
		if chaseValue == nil {
			return nil
		}
		switch chaseType.Kind {
		case KindSlice:
			// Special-case: If the slice is empty, return nil
			header := (*reflect.SliceHeader)(chaseValue)
			if header.Len == 0 {
				return nil
			}
			fallthrough
		case KindStruct:
			// We wrap structs and slices in a new Abstract.
			return &Abstract{
				engine:   a.engine,
				typeData: chaseType,
				value:    chaseValue,
			}
		case KindPointer:
			// We try to dereference pointers and loop around.
			chaseValue = *(*Ptr)(chaseValue)
			chaseType = chaseType.elemData
		case KindInterface:
			// Interfaces return a more specialized type.
			elemType := chaseType.IntfType(chaseValue)
			if elemType == 0 {
				return nil
			}
			chaseType = a.engine.typeData(elemType)
			chaseValue = ((*[2]Ptr)(chaseValue))[1]
		default:
			panic(fmt.Errorf("unimplemented: %d", chaseType.Kind))
		}
	}
}

// NumChildren returns the number of fields or slice elements.
func (a *Abstract) NumChildren() int {
	if a.value == nil {
		return 0
	}
	switch a.typeData.Kind {
	case KindStruct:
		return len(a.typeData.Fields)
	case KindSlice:
		return (*reflect.SliceHeader)(a.value).Len
	default:
		// Interfaces should be replaced by a more specific type and
		// pointers should be dereferenced.
		panic(fmt.Errorf("unimplemented: %d", a.typeData.Kind))
	}
}

// Ptr returns the embedded pointer. This should not be exposed to
// user code, but should instead be provided via a type-safe facade.
func (a *Abstract) Ptr() Ptr {
	return a.value
}

// TypeID returns the type token of the embedded value.
func (a *Abstract) TypeID() TypeID {
	return a.typeData.TypeID
}
