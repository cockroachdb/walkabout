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

// ActionFn describes a simple callback function.
type ActionFn func() error

// FacadeFn is a generated function type that depends on the visitable
// interface.
type FacadeFn interface{}

// Ptr is an alias for unsafe.Pointer.
type Ptr unsafe.Pointer

// TypeData contains metadata and accessors that are produced by the
// code generator.
type TypeData struct {
	// Copy will effect a type aware copy of the data at from to dest.
	Copy func(dest, from Ptr)
	// Elem is the element type of a slice or of a pointer.
	Elem TypeId
	// Facade will call a user-provided facade function in a
	// type-safe fashion.
	Facade func(Context, FacadeFn, Ptr) Decision
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
	// Name is the source name of the type.
	Name string
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

// Context is provided to generated, type-safe facades.
type Context struct{}

// ActionCall constructs an action which will invoke the function.
func (Context) ActionCall(fn ActionFn) Action {
	return Action{call: fn}
}

// ActionVisit constructs an action which will visit the given value.
func (Context) ActionVisit(td *TypeData, value Ptr) Action {
	return Action{typeData: td, value: value, valueType: td.TypeId}
}

// ActionVisitTypeId constructs an action which will visit the given value.
func (Context) ActionVisitTypeId(id TypeId, value Ptr) Action {
	return Action{value: value, valueType: id}
}

// Actions is for use by generated code only.
func (Context) Actions(actions []Action) Decision {
	return Decision{actions: actions}
}

// Continue is for use by generated code only.
func (Context) Continue() Decision {
	return Decision{}
}

// Error is for use by generated code only.
func (Context) Error(err error) Decision {
	return Decision{error: err}
}

// Halt is for use by generated code only.
func (Context) Halt() Decision {
	return Decision{halt: true}
}

// Skip is for use by generated code only.
func (Context) Skip() Decision {
	return Decision{skip: true}
}

// Decision is wrapped by generated, type-safe facades.
type Decision struct {
	actions         []Action
	error           error
	halt            bool
	intercept       FacadeFn
	post            FacadeFn
	replacement     Ptr
	replacementType TypeId
	skip            bool
}

// Intercept is for use by generated code only.
func (d Decision) Intercept(fn FacadeFn) Decision {
	d.intercept = fn
	return d
}

// Post is for use by generated code only.
func (d Decision) Post(fn FacadeFn) Decision {
	d.post = fn
	return d
}

// Replace is for use by generated code only.
func (d Decision) Replace(id TypeId, x Ptr) Decision {
	d.replacement = x
	d.replacementType = id
	return d
}

// Action allows user-defined actions to be inserted into the
// visitation flow.
type Action struct {
	call      ActionFn
	dirty     bool
	post      FacadeFn
	typeData  *TypeData
	value     Ptr
	valueType TypeId
}

// apply updates the action with information from a decision.
func (a *Action) apply(e *Engine, d Decision) error {
	if d.error != nil {
		return d.error
	}
	if d.post != nil {
		a.post = d.post
	}
	if d.replacement != nil {
		curType := a.typeData
		// The user can only change the type of the object if it's being
		// assigned to an interface slot. Even then, we'll want to
		// check the assignability.
		if curType.TypeId != d.replacementType {
			if curType.Kind == KindInterface {
				nextTypeId := curType.IntfType(d.replacement)
				if nextTypeId == 0 {
					return fmt.Errorf(
						"type %d is unknown or not assignable to %d", nextTypeId, curType.TypeId)
				}
				curType = e.typeData(nextTypeId)
			} else {
				return fmt.Errorf(
					"cannot change type of %d to %d", curType.TypeId, d.replacementType)
			}
		}
		a.dirty = true
		a.value = d.replacement
	}
	return nil
}
