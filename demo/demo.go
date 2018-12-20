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

// Package demo is used for demonstration and testing of walkabout.
package demo

import "github.com/cockroachdb/walkabout/demo/other"

//go:generate walkabout Target

// Target is a base interface that we run the code-generator against.
// There's nothing special about this interface.
type Target interface {
	Value() string
}

// Just an FYI to show that we support types that implement the
// interface by-value and by-reference.
var (
	_ Target = &ByRefType{}
	_ Target = ByValType{}
	_ Target = &ContainerType{}
	_ Target = &ignoredType{}
)

// EmbedsTarget demonstrates an interface hierarchy.
type EmbedsTarget interface {
	Target
	embedsTarget()
}

var (
	_ EmbedsTarget = ByValType{}
)

// Targets is a named slice of a visitable interface.
type Targets []Target

// ByRefType implements Target with a pointer receiver.
type ByRefType struct {
	Val string
}

// Value implements the Target interface.
func (x *ByRefType) Value() string { return x.Val }

// ByValType implements the Target interface with a value receiver.
type ByValType struct {
	Val string
}

func (ByValType) embedsTarget() {}

// Value implements the Target interface.
func (x ByValType) Value() string { return x.Val }

// ignoredType is not exported, so it won't appear in the API.
type ignoredType struct{}

// Value implements the Target interface.
func (ignoredType) Value() string { return "Should never see this" }

// This type will be included in --union --reachable mode. It doesn't
// implement the Target interface, but it is a field in ContainerType.
type ReachableType struct{}

// This type isn't reachable from any type that implements Target,
// so it will never be generated.
type NeverType struct{}

// Unionable demonstrates how multiple type hierarchies can be
// unioned using another generated interface.
type Unionable interface {
	isUnionable()
}

type UnionableType struct{}

func (UnionableType) isUnionable() {}

// ContainerType is just a regular struct that contains fields
// whose types implement or contain Target.
type ContainerType struct {
	ByRef         ByRefType
	ByRefPtr      *ByRefType
	ByRefSlice    []ByRefType
	ByRefPtrSlice []*ByRefType

	ByVal         ByValType
	ByValPtr      *ByValType
	ByValSlice    []ByValType
	ByValPtrSlice []*ByValType

	// We can break cycles, too.
	Container *ContainerType

	AnotherTarget    Target
	AnotherTargetPtr *Target

	// Interfaces which extend the visitable interface are supported.
	EmbedsTarget    EmbedsTarget
	EmbedsTargetPtr *EmbedsTarget

	// Slices of interfaces are supported.
	TargetSlice []Target

	// We can support slices of interface pointers.
	InterfacePtrSlice []*Target

	// Demonstrate use of named visitable type.
	NamedTargets Targets

	// Unexported fields aren't generated.
	ignored ByRefType
	// Unexported types aren't generated.
	Ignored *ignoredType

	// This field will be generated one when in --union mode.
	UnionableType *UnionableType

	/// This field will only be visited when in --union --reachable mode.
	ReachableType ReachableType

	// This type is declared in another package. It shouldn't be present
	// in any configuration, unless we allow the code generator to
	// start writing to multiple directories, in which case we can make
	// --union --reachable work.
	OtherReachable other.Reachable

	// This field is in --reachable mode, since it does implement
	// our Target interface.
	OtherImplementor other.Implemetor
}

// Value implements the Target interface.
func (*ContainerType) Value() string { return "Container" }

// NewContainer generates test data.
func NewContainer(useValuePtrs bool) (*ContainerType, int) {
	count := 0
	olleh := func() string {
		count++
		return "olleH"
	}

	embedsTarget := func() EmbedsTarget {
		if useValuePtrs {
			return &ByValType{olleh()}
		} else {
			return ByValType{olleh()}
		}
	}

	target := func() Target {
		if useValuePtrs {
			return &ByValType{olleh()}
		} else {
			return ByValType{olleh()}
		}
	}

	p1 := target()
	p2 := target()
	p3 := target()
	p4 := embedsTarget()
	p5 := target()
	var nilTarget Target
	typedNil := Target(nil)

	x := &ContainerType{
		ByRef:         ByRefType{olleh()},
		ByRefPtr:      &ByRefType{olleh()},
		ByRefSlice:    []ByRefType{{olleh()}, {olleh()}},
		ByRefPtrSlice: []*ByRefType{{olleh()}, nil, {olleh()}},

		ByVal:         ByValType{olleh()},
		ByValPtr:      &ByValType{olleh()},
		ByValSlice:    []ByValType{{olleh()}, {olleh()}},
		ByValPtrSlice: []*ByValType{{olleh()}, nil, {olleh()}},

		AnotherTarget:    target(),
		AnotherTargetPtr: &p5,

		EmbedsTarget:    &ByValType{olleh()},
		EmbedsTargetPtr: &p4,

		TargetSlice:  []Target{target(), target()},
		NamedTargets: []Target{target(), target()},

		InterfacePtrSlice: []*Target{&p1, nil, &nilTarget, &typedNil, &p2, &p3},
	}
	return x, count
}
