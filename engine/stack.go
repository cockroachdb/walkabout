// Copyright 2019 The Cockroach Authors.
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

type stack struct {
	data  []frame
	depth int
}

func newStack() *stack {
	return &stack{data: make([]frame, defaultStackDepth)}
}

// Depth returns the current stack depth.
func (s *stack) Depth() int {
	return s.depth
}

// Enter pushes a new frame onto the stack, configures, and returns it.
func (s *stack) Enter(intercept FacadeFn, slotCount int) *frame {
	if s.depth == len(s.data) {
		temp := make([]frame, len(s.data)*3/2+1)
		copy(temp, s.data)
		s.data = temp
	}
	entering := &s.data[s.depth]
	s.depth++

	entering.Count = slotCount
	entering.Intercept = intercept
	entering.Idx = 0
	if slotCount > fixedSlotCount {
		entering.Overflow = make([]Action, slotCount-fixedSlotCount)
	}
	return entering
}

// Peek retrieves the frame at the given depth.
func (s *stack) Peek(depth int) *frame {
	return &s.data[depth]
}

// Pop removes and returns the top frame.
func (s *stack) Pop() *frame {
	s.depth--
	return &s.data[s.depth]
}

// Top access the Nth frame from the top of the stack.
func (s *stack) Top(offset int) *frame {
	return &s.data[s.depth-1-offset]
}
