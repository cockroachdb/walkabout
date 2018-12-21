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

// This file contains various type definitions.

import (
	"fmt"
	"unsafe"
)

// A TypeId is an opaque reference to a visitable type. These are
// assigned by the code-generator and their specific values and order
// are arbitrary.
type TypeId int

// A TypeMap holds the necessary metadata to visit a collection of types.
type TypeMap []TypeData

// Kind determines the dispatch strategy for a given visitable type.
type Kind int

// A visitable type has some combinations of kinds which determine
// its access pattern.
const (
	_ Kind = iota
	KindInterface
	KindPointer
	KindSlice
	KindStruct
)

// Ptr is an alias for unsafe.Pointer.
type Ptr unsafe.Pointer

// FacadeFn is a generated function type that depends on the visitable
// interface.
type FacadeFn interface{}

// ActionFn describes a simple callback function.
type ActionFn func() error

// TypeData contains metadata and accessors that are produced by the
// code generator.
type TypeData struct {
	// Copy will effect a type aware copy of the data at from to dest.
	Copy func(dest, from Ptr)
	// Elem is the element type of a slice or of a pointer.
	Elem TypeId
	// Facade will call a user-provided facade function in a
	// type-safe fashion.
	Facade func(ContextImpl, FacadeFn, Ptr) DecisionImpl
	// Fields holds information about the fields of a struct.
	Fields []FieldInfo
	// IntfType accepts a pointer to an interface type and returns a
	// TypeId for the enclosed datatype.
	//
	// An interface's type-tag contains several flag bits which
	// fall into the category of "too much magic" for us to want
	// to handle ourselves. Instead, we generate functions which
	// will perform the necessary type mapping.
	IntfType func(Ptr) TypeId
	// IntfWrap provides the opposite function of IntfType. It accepts
	// a TypeId and a pointer to the interface's value and returns a
	// pointer to the resulting interface array.
	IntfWrap func(TypeId, Ptr) Ptr
	// Kind selects various strategies for handling the given type.
	Kind Kind
	// NewSlice constructs a slice of the given length and returns a
	// pointer to the slice's header.
	NewSlice func(size int) Ptr
	// NewStruct returns a pointer to a newly-allocated struct.
	NewStruct func() Ptr
	// SizeOf is the size of the data type. This is used for traversing
	// slices. It could be expanded in the future to generalizing the
	// Copy() function.
	SizeOf uintptr
	// TypeId is a generated id.
	TypeId TypeId

	// This field is populated when an Engine is constructed.
	elemData *TypeData
}

// FieldInto describes a field within a struct.
type FieldInfo struct {
	Name   string
	Offset uintptr
	Target TypeId

	// This field is populated when an Engine is constructed.
	targetData *TypeData
}

// ContextImpl is provided to generated, type-safe facades.
type ContextImpl struct{}

// DecisionImpl is wrapped by generated, type-safe facades.
type DecisionImpl struct {
	Actions         []ActionImpl
	Error           error
	Halt            bool
	Intercept       FacadeFn
	Post            FacadeFn
	Replacement     Ptr
	ReplacementType TypeId
	Skip            bool
}

// ActionImpl allows user-defined actions to be inserted into the
// visitation flow.
type ActionImpl struct {
	call      ActionFn
	dirty     bool
	post      FacadeFn
	typeData  *TypeData
	value     Ptr
	valueType TypeId
}

// ActionCall constructs an action which will invoke the function.
func ActionCall(fn ActionFn) ActionImpl {
	return ActionImpl{call: fn}
}

// ActionVisit constructs an action which will visit the given value.
func ActionVisit(td *TypeData, value Ptr) ActionImpl {
	return ActionImpl{typeData: td, value: value, valueType: td.TypeId}
}

// ActionVisitTypeId constructs an action which will visit the given value.
func ActionVisitTypeId(id TypeId, value Ptr) ActionImpl {
	return ActionImpl{value: value, valueType: id}
}

// apply updates the action with information from a decision.
func (a *ActionImpl) apply(e *Engine, d DecisionImpl) error {
	if d.Error != nil {
		return d.Error
	}
	if d.Post != nil {
		a.post = d.Post
	}
	if d.Replacement != nil {
		curType := a.typeData
		// The user can only change the type of the object if it's being
		// assigned to an interface slot. Even then, we'll want to
		// check the assignability.
		if curType.TypeId != d.ReplacementType {
			if curType.Kind == KindInterface {
				nextTypeId := curType.IntfType(d.Replacement)
				if nextTypeId == 0 {
					return fmt.Errorf(
						"type %d is unknown or not assignable to %d", nextTypeId, curType.TypeId)
				}
				curType = e.typeData(nextTypeId)
			} else {
				return fmt.Errorf(
					"cannot change type of %d to %d", curType.TypeId, d.ReplacementType)
			}
		}
		a.dirty = true
		a.value = d.Replacement
	}
	return nil
}
