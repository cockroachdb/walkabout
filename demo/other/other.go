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

// Package other is used to check reachable types declared in
// other packages.
package other

// Reachable is reachable from our Container type, but we can't
// do anything to make it implement a common interface (unless
// we want the generator to start writing into multiple output
// directories in a single pass, which seems fraught).
type Reachable struct{}

// Implementor implements demo.Target, but since it's in another
// package, it's not eligible for inclusion.
type Implementor struct {
	val string
}

// Value implements the Target interface.
func (i Implementor) Value() string {
	return i.val
}
