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

package demo_test

import (
	"testing"

	"github.com/cockroachdb/walkabout/demo"
)

// BenchmarkNoop should demonstrate that visitations are allocation-free.
func BenchmarkNoop(b *testing.B) {
	b.Run("useValuePtrs=true", func(b *testing.B) {
		x, _ := demo.NewContainer(true)
		bench(b, x)
	})
	b.Run("useValuePtrs=false", func(b *testing.B) {
		x, _ := demo.NewContainer(false)
		bench(b, x)
	})
}

func bench(b *testing.B, x *demo.ContainerType) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	fn := func(ctx demo.TargetContext, x demo.Target) (ret demo.TargetDecision) { return }
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, _, err := x.WalkTarget(fn); err != nil {
				b.Fatal(err)
			}
		}
	})
}
