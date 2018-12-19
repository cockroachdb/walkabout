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
	"runtime"
	"testing"
	"time"

	"github.com/cockroachdb/walkabout/demo"
	"github.com/stretchr/testify/assert"
)

// This is a quick check to keep the core loop allocation-free.
func TestNoMallocs(t *testing.T) {
	t.Run("useValuePtrs=true", func(t *testing.T) {
		a := assert.New(t)
		x, _ := demo.NewContainer(true)
		testNoMallocs(a, x)
	})
	t.Run("useValuePtrs=false", func(t *testing.T) {
		a := assert.New(t)
		x, _ := demo.NewContainer(false)
		testNoMallocs(a, x)
	})
}

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

// This runs in a loop until we have demonstrated that no mallocs
// occur, or a timeout occurs. This allows us to account for any
// other threads that may be running.
func testNoMallocs(a *assert.Assertions, x *demo.ContainerType) {
	stats := runtime.MemStats{}
	timer := time.NewTimer(1 * time.Second)
	fn := func(ctx demo.TargetContext, x demo.Target) (ret demo.TargetDecision) { return }

	for {
		select {
		case <-timer.C:
			a.Fail("timeout")
			return
		default:
			runtime.ReadMemStats(&stats)
			memBefore := stats.Mallocs

			_, _, err := x.WalkTarget(fn)
			runtime.ReadMemStats(&stats)

			a.NoError(err)
			memAfter := stats.Mallocs
			if memAfter == memBefore {
				return
			}
		}
	}
}
