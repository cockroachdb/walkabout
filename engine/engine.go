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

// Package engine holds base implementation details for use by
// generated code. Users should not depend on any particular feature
// of this package.
package engine

import (
	"fmt"
	"reflect"
	"strings"
)

// See discussion on frame.Slots.
const fixedSlotCount = 16

// A frame represents the visitation of a single struct,
// interface, or slice.
type frame struct {
	// Count holds the number of slots to be visited.
	Count int
	// Idx is the current slot being visited.
	Idx       int
	Intercept FacadeFn
	// We keep a fixed-size array of slots per frame so that most
	// visitable objects won't need a heap allocation to store
	// the intermediate state.
	Slots [fixedSlotCount]ActionImpl
	// Large targets (such as slices) will use additional, heap-allocated
	// memory to store the intermediate state.
	Overflow []ActionImpl
}

// Active retrieves the active slot.
func (f *frame) Active() (s *ActionImpl, td *TypeData) {
	s = f.Slot(f.Idx)
	td = s.typeData
	return
}

// Slot is used to access a storage slot within the frame.
func (f *frame) Slot(idx int) *ActionImpl {
	if idx < fixedSlotCount {
		return &f.Slots[idx]
	} else {
		return &f.Overflow[idx-fixedSlotCount]
	}
}

// SetSlot is a helper function to configure a slot.
func (f *frame) SetSlot(e *Engine, idx int, action ActionImpl) *ActionImpl {
	ret := f.Slot(idx)
	*ret = action
	if ret.typeData == nil {
		ret.typeData = e.typeData(ret.valueType)
	}
	return ret
}

// Zero returns Slot(0).
func (f *frame) Zero() *ActionImpl {
	return &f.Slots[0]
}

// An Engine holds the necessary information to pass a visitor over
// a field.
type Engine struct {
	typeMap TypeMap
}

// New constructs an Engine.
func New(m TypeMap) *Engine {
	// Make a copy of the TypeMap and link all of the TypeDatas together.
	e := &Engine{typeMap: append(m[:0:0], m...)}
	for idx, td := range e.typeMap {
		if td.Elem != 0 {
			found := e.typeData(td.Elem)
			if found.TypeId == 0 {
				panic(fmt.Errorf("bad codegen: missing %d.Elem %d",
					td.TypeId, td.Elem))
			}
			e.typeMap[idx].elemData = found
		}

		for fIdx, field := range td.Fields {
			found := e.typeData(field.Target)
			if found.TypeId == 0 {
				panic(fmt.Errorf("bad codegen: missing %d.%s.Target %d",
					td.TypeId, field.Name, field.Target))
			}
			e.typeMap[idx].Fields[fIdx].targetData = found
		}
	}
	return e
}

// Abstract constructs an abstract accessor around a struct's field.
func (e *Engine) Abstract(typeId TypeId, x Ptr) *Abstract {
	if x == nil {
		return nil
	}
	return &Abstract{
		engine:   e,
		typeData: e.typeData(typeId),
		value:    x,
	}
}

// Execute drives the visitation process. This is an "unrolled
// recursive" function that maintains its own stack to avoid
// deeply-nested call stacks. We can also perform cycle-detection at
// fairly low cost.
func (e *Engine) Execute(fn FacadeFn, t TypeId, x Ptr) (Ptr, bool, error) {
	stack := make([]frame, 8)
	stackIdx := 0

	// Entering is a temporary pointer to the frame that we might be
	// entering into next, if the current value is a struct with fields, a
	// slice, etc.
	var entering *frame
	// enter() configures the entering frame to have at least a minimum
	// number of variable slots.
	enter := func(intercept FacadeFn, slotCount int) {
		entering.Count = slotCount
		entering.Intercept = intercept
		entering.Idx = 0
		if slotCount > fixedSlotCount {
			entering.Overflow = make([]ActionImpl, slotCount-fixedSlotCount)
		}
	}

	// This variable holds a pointer to a frame that we've just completed.
	// When we have a returning frame that's dirty, we'll want to unpack
	// its values into the current slot.
	var returning *frame

	// Bootstrap the stack.
	stack[0].Count = 1
	stack[0].SetSlot(e, 0, ActionVisit(e.typeData(t), x))

	curFrame := &stack[0]
	curSlot := curFrame.Zero()
	curType := curSlot.typeData
	halting := false

enter:
	if curSlot.call != nil {
		if err := curSlot.call(); err != nil {
			return nil, false, err
		}
		goto unwind
	}

	// Linear search for cycle-breaking. Note that this does not guarantee
	// exactly-once behavior if there are multiple pointers to an object
	// within a visitable graph. pprof says this is much faster than using
	// a map structure, especially since we expect the stack to be fairly
	// shallow. We use both the type and pointer as a unique key in order
	// to distinguish a struct from the first field of the struct. go
	// disallows recursive type definitions, so it's impossible for the
	// first field of a struct to be exactly the struct type.
	for l := 0; l < stackIdx; l++ {
		onStack, onStackType := stack[l].Active()
		if onStack.value == curSlot.value && onStackType.TypeId == curType.TypeId {
			goto nextSlot
		}
	}

	// In this switch statement, we're going to set up the next frame. If
	// the current value doesn't need a new frame to be pushed, we'll jump
	// into the unwind block.
	entering = &stack[stackIdx+1]
	switch curType.Kind {
	case KindPointer:
		// We dereference the pointer and push the resulting memory
		// location as a 1-slot frame.
		ptr := *(*Ptr)(curSlot.value)
		if ptr == nil {
			goto unwind
		}
		enter(curFrame.Intercept, 1)
		entering.SetSlot(e, 0, ActionVisit(curType.elemData, ptr))

	case KindStruct:
		// Allow parent frames to intercept child values.
		if curFrame.Intercept != nil {
			d := curType.Facade(ContextImpl{}, curFrame.Intercept, curSlot.value)
			if err := curSlot.apply(e, d); err != nil {
				return nil, false, err
			}
			if d.Halt {
				halting = true
			}
			// Allow interceptors to replace themselves.
			if d.Intercept != nil {
				curFrame.Intercept = d.Intercept
			}
		}

		// Structs are where we call out to user logic via a generated,
		// type-safe facade. The user code can trigger various flow-control
		// to happen.
		d := curType.Facade(ContextImpl{}, fn, curSlot.value)
		// Incorporate replacements, bail on error, etc.
		if err := curSlot.apply(e, d); err != nil {
			return nil, false, err
		}
		// If the user wants to stop, we'll set the flag and just let the
		// unwind loop run to completion.
		if d.Halt {
			halting = true
		}
		// Slices and structs have very similar approaches, we create a new
		// frame, add slots for each field or slice element, and then jump
		// back to the top.
		fieldCount := len(curType.Fields)
		switch {
		case halting, d.Skip:
			goto unwind

		case d.Actions != nil:
			if len(d.Actions) == 0 {
				goto unwind
			}
			enter(d.Intercept, len(d.Actions))
			for i, a := range d.Actions {
				entering.SetSlot(e, i, a)
			}

		default:
			if fieldCount == 0 {
				goto unwind
			}
			enter(d.Intercept, fieldCount)
			for i, f := range curType.Fields {
				fPtr := Ptr(uintptr(curSlot.value) + f.Offset)
				entering.SetSlot(e, i, ActionVisit(f.targetData, fPtr))
			}
		}

	case KindSlice:
		// Slices have the same general flow as a struct; they're just
		// a sequence of visitable values.
		header := (*reflect.SliceHeader)(curSlot.value)
		if header.Len == 0 {
			goto unwind
		}
		enter(curFrame.Intercept, header.Len)
		eltTd := curType.elemData
		for i, off := 0, uintptr(0); i < header.Len; i, off = i+1, off+eltTd.SizeOf {
			entering.SetSlot(e, i, ActionVisit(eltTd, Ptr(header.Data+off)))
		}

	case KindInterface:
		// An interface is a type-tag and a pointer.
		ptr := (*[2]Ptr)(curSlot.value)[1]
		// We do need to map the type-tag to our TypeId.
		// Perhaps this could be accomplished with a map?
		elem := curType.IntfType(curSlot.value)
		// Need to check elem==0 in the case of a "typed nil" value.
		if elem == 0 || ptr == nil {
			goto unwind
		}
		enter(curFrame.Intercept, 1)
		entering.SetSlot(e, 0, ActionVisit(e.typeData(elem), ptr))

	default:
		panic(fmt.Errorf("unexpected kind: %d", curType.Kind))
	}

	// TODO(bob): Be able to fork off to visit the slots in parallel
	// on a per-node basis.
	stackIdx++
	// We want the extra -1 here to maintain an offset for enter().
	if stackIdx == len(stack)-1 {
		temp := make([]frame, 3*len(stack)/2+1)
		copy(temp, stack)
		stack = temp
	}
	curFrame = entering
	curSlot = curFrame.Zero()
	curType = curSlot.typeData

	// We've pushed a new frame onto the stack, so we'll restart.
	goto enter

unwind:
	// Execute any user-provided callback. This logic is pretty much
	// the same as above, although we don't respect all decision options.
	if curSlot.post != nil {
		d := curType.Facade(ContextImpl{}, curSlot.post, curSlot.value)
		if err := curSlot.apply(e, d); err != nil {
			return nil, false, err
		}
		if d.Halt {
			halting = true
		}
	}

	// If the slot reports that it's dirty, we want to propagate
	// the changes upwards in the stack.
	if curSlot.dirty {
		if stackIdx > 0 {
			parentFrame, _ := stack[stackIdx-1].Active()
			parentFrame.dirty = true
		}

		// This switch statement is the inverse of the above. We'll fold the
		// returning frame into a replacement value for the current slot.
		switch curType.Kind {
		case KindStruct:
			// Allocate a replacement instance of the struct.
			next := curType.NewStruct()
			// Perform a shallow copy to catch non-visitable fields.
			curType.Copy(next, curSlot.value)

			// Copy the visitable fields into the new struct.
			for i, f := range curType.Fields {
				fPtr := Ptr(uintptr(next) + f.Offset)
				f.targetData.Copy(fPtr, returning.Slot(i).value)
			}
			curSlot.value = next

		case KindPointer:
			// Copy out the pointer to a local var so we don't stomp on it.
			next := returning.Zero().value
			curSlot.value = Ptr(&next)

		case KindSlice:
			// Create a new slice instance and populate the elements.
			next := curType.NewSlice(returning.Count)
			toHeader := (*reflect.SliceHeader)(next)
			elemTd := curType.elemData

			// Copy the elements across.
			for i := 0; i < returning.Count; i++ {
				toElem := Ptr(toHeader.Data + uintptr(i)*elemTd.SizeOf)
				elemTd.Copy(toElem, returning.Slot(i).value)
			}
			curSlot.value = next

		case KindInterface:
			// Swap out the iface pointer just like the pointer case above.
			next := returning.Zero()
			curSlot.value = curType.IntfWrap(next.typeData.TypeId, next.value)

		default:
			panic(fmt.Errorf("unimplemented: %d", curType.Kind))
		}
	}

nextSlot:
	// We'll advance the current slot or unwind one level if we've
	// processed the last slot in the frame.
	curFrame.Idx++
	// If the user wants to stop early, we'll just keep running the
	// unwind loop until we hit the top frame.
	if curFrame.Idx == curFrame.Count || halting {
		// If we've finished the bootstrap frame, we're done.
		if stackIdx == 0 {
			return curFrame.Zero().value, curFrame.Zero().dirty, nil
		}
		// Save off the current frame so we can copy the data out.
		returning = curFrame
		// Pop a frame off of the stack and update local vars.
		stackIdx--
		curFrame = &stack[stackIdx]
		curSlot, curType = curFrame.Active()
		// We'll jump back to the unwinding code to finish the slot of the
		// frame which is now on top.
		goto unwind
	} else {
		// We're just advancing to the next slot, so we jump back to the
		// top.
		curSlot, curType = curFrame.Active()
		goto enter
	}
}

// Stringify returns a string representation of the given type that
// is suitable for debugging purposes.
func (e *Engine) Stringify(id TypeId) string {
	if id == 0 {
		return "<NIL>"
	}
	ret := strings.Builder{}
	td := e.typeData(id)
	for {
		switch td.Kind {
		case KindInterface, KindStruct:
			if ret.Len() == 0 {
				return td.Name
			}
			ret.WriteString(td.Name)
			return ret.String()
		case KindPointer:
			ret.WriteRune('*')
			td = td.elemData
		case KindSlice:
			ret.WriteString("[]")
			td = td.elemData
		default:
			panic(fmt.Errorf("unsupported: %d", td.Kind))
		}
	}
}

// typeData returns a pointer to the TypeData for the given type.
func (e *Engine) typeData(id TypeId) *TypeData {
	return &e.typeMap[id]
}
