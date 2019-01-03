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

// In this test, we're going to show mutations performed in-place
// as well as mutations performed by replacement.  We have visitable
// types *ByRefType and ByValType.  We can modify *ByRefType in place,
// but must replace values of ByValType.

import (
	"strings"
	"testing"

	"github.com/cockroachdb/walkabout/demo/other"

	l "github.com/cockroachdb/walkabout/demo"
	"github.com/stretchr/testify/assert"
)

func TestBadMutations(t *testing.T) {
	a := assert.New(t)

	d := l.ByValType{}
	_, _, err := d.WalkTarget(func(ctx l.TargetContext, x l.Target) l.TargetDecision {
		return ctx.Continue().Replace(&l.ByRefType{})
	})
	a.EqualError(err, "cannot change type of ByValType to ByRefType")

}

// Verify data extraction.
func TestChildAt(t *testing.T) {
	// Expect all but by-value values to be nil.
	t.Run("empty", func(t *testing.T) {
		a := assert.New(t)
		c := &l.ContainerType{}
		abstractWalk(c)
		for i, j := 0, c.NumChildren(); i < j; i++ {
			child := c.ChildAt(i)
			switch i {
			case 0, 4:
				a.NotNilf(child, "at index %d", i)
			default:
				a.Nilf(child, "at index %d", i)
			}
		}
	})

	// Only our inner *Container field should be nil.
	t.Run("useValuePtrs=true", func(t *testing.T) {
		a := assert.New(t)
		c, _ := l.NewContainer(true)
		abstractWalk(c)
		for i, j := 0, c.NumChildren(); i < j; i++ {
			child := c.ChildAt(i)
			switch i {
			case 8:
				// The *Container field
				a.Nilf(child, "at index %d", i)
			default:
				a.NotNilf(child, "at index %d", i)
			}
		}
	})
	t.Run("useValuePtrs=false", func(t *testing.T) {
		a := assert.New(t)
		c, _ := l.NewContainer(false)
		abstractWalk(c)
		for i, j := 0, c.NumChildren(); i < j; i++ {
			child := c.ChildAt(i)
			switch i {
			case 8:
				// The *Container field
				a.Nilf(child, "at index %d", i)
			default:
				a.NotNilf(child, "at index %d", i)
			}
		}
	})
}

// TestCycleBreak creates a cyclical datastructure.
func TestCycleBreak(t *testing.T) {
	d, _ := l.NewContainer(false)
	d.Container = d
	_, _, _ = d.WalkTarget(func(ctx l.TargetContext, x l.Target) (d l.TargetDecision) {
		return
	})
}

// TestInterfaceChange ensures that an interface context allows the
// concrete type to be changed out.
func TestInterfaceChange(t *testing.T) {
	t.Run("test top level", func(t *testing.T) {
		a := assert.New(t)

		d := l.ByValType{}
		ret, changed, err := l.WalkTarget(d, func(ctx l.TargetContext, x l.Target) l.TargetDecision {
			return ctx.Continue().Replace(&l.ByRefType{})
		})
		if !a.NoError(err) {
			return
		}
		a.True(changed, "should have changed")
		a.IsType(&l.ByRefType{}, ret)
	})
	t.Run("test interface field", func(t *testing.T) {
		a := assert.New(t)

		ref := l.Target(l.ByValType{Val: "ChangeMe"})
		d := l.ContainerType{
			AnotherTarget:    l.ByValType{Val: "ChangeMe"},
			AnotherTargetPtr: &ref,
		}
		d2, changed, err := d.WalkTarget(func(ctx l.TargetContext, x l.Target) (d l.TargetDecision) {
			if x.Value() == "ChangeMe" {
				d = d.Replace(&l.ByRefType{Val: "Changed"})
			}
			return
		})
		if !a.NoError(err) {
			return
		}
		a.True(changed)
		a.NotEqual(d, d2)
		a.IsType(&l.ByRefType{}, d2.AnotherTarget)
		a.IsType(&l.ByRefType{}, *d2.AnotherTargetPtr)
		// Ensure that the underlying field wasn't touched.
		a.IsType(l.ByValType{}, ref)
	})
	t.Run("cross-assign", func(t *testing.T) {
		a := assert.New(t)

		c := l.ContainerType{
			EmbedsTarget: l.ByValType{Val: "ChangeMe"},
		}
		_, _, err := c.WalkTarget(func(ctx l.TargetContext, x l.Target) (d l.TargetDecision) {
			if x.Value() == "ChangeMe" {
				d = d.Replace(&l.ByRefType{Val: "Not an EmbedsTarget"})
			}
			return
		})
		a.EqualError(err, "type ByRefType is unknown or not assignable to EmbedsTarget")
	})
	t.Run("unknown type", func(t *testing.T) {
		a := assert.New(t)
		a.PanicsWithValue("unhandled value of type: *other.Implementor", func() {
			d := l.ByValType{}
			_, _, _ = l.WalkTarget(d, func(ctx l.TargetContext, x l.Target) l.TargetDecision {
				return ctx.Continue().Replace(&other.Implementor{})
			})
		})
	})
}

// TestMutations applies a string-reversing visitor to our Container
// and then prints the resulting structure.
func TestMutations(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		checkMutations(t, &l.ContainerType{}, 0)
	})
	t.Run("useValuePtrs=true", func(t *testing.T) {
		x, count := l.NewContainer(true)
		checkMutations(t, x, count)
	})
	t.Run("useValuePtrs=false", func(t *testing.T) {
		x, count := l.NewContainer(false)
		checkMutations(t, x, count)
	})
}

func abstractWalk(x l.TargetAbstract) {
	if x == nil {
		return
	}
	for i, j := 0, x.NumChildren(); i < j; i++ {
		abstractWalk(x.ChildAt(i))
	}
}

func checkMutations(t *testing.T, x *l.ContainerType, count int) {
	t.Helper()
	a := assert.New(t)
	var expected string
	for i := 0; i < count; i++ {
		expected += "Hello"
	}

	x2, changed, err := x.WalkTarget(func(ctx l.TargetContext, x l.Target) (d l.TargetDecision) {
		switch t := x.(type) {
		case *l.ByRefType:
			cp := *t
			cp.Val = reverse(cp.Val)
			d = d.Replace(&cp)
		case *l.ByValType:
			cp := *t
			cp.Val = reverse(cp.Val)
			d = d.Replace(&cp)
		}
		return
	})
	if err != nil {
		t.Fatal(err)
	}
	a.True(changed, "not changed")
	if x.ByRefPtr != nil {
		a.NotEqual(x.ByRefPtr, x2.ByRefPtr, "pointer should have changed")
	}

	var w strings.Builder
	x3, changed, err := x2.WalkTarget(func(_ l.TargetContext, x l.Target) (d l.TargetDecision) {
		switch t := x.(type) {
		case *l.ByRefType:
			w.WriteString(t.Val)
		case *l.ByValType:
			w.WriteString(t.Val)
		}
		return
	})

	a.Nil(err)
	a.Equal(expected, w.String())
	a.False(changed, "should not have changed")
	a.Equal(x2.ByRefPtr, x3.ByRefPtr, "pointer should not have changed")
}

// Via Russ Cox
// https://groups.google.com/d/msg/golang-nuts/oPuBaYJ17t4/PCmhdAyrNVkJ
func reverse(s string) string {
	n := 0
	runes := make([]rune, len(s))
	for _, r := range s {
		runes[n] = r
		n++
	}
	// Account for multi-byte points.
	runes = runes[0:n]
	// Reverse.
	for i := 0; i < n/2; i++ {
		runes[i], runes[n-1-i] = runes[n-1-i], runes[i]
	}

	return string(runes)
}
