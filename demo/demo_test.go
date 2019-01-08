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
	"errors"
	"fmt"
	"reflect"

	"github.com/cockroachdb/walkabout/demo"
)

// This example demonstrates how an enhanced visitable type can be
// used as though it were a tree of nodes.
func Example_abstract() {
	data, _ := demo.NewContainer(true)

	for i := 0; i < data.TargetCount(); i++ {
		child := data.TargetAt(i)
		if child == nil {
			fmt.Printf("%d: nil\n", i)
		} else {
			fmt.Printf("%d: %s %s\n", i, child.TargetTypeId(), reflect.TypeOf(child))
		}
	}

	//Output:
	//0: ByRefType *demo.ByRefType
	//1: ByRefType *demo.ByRefType
	//2: []ByRefType *demo.targetAbstract
	//3: []*ByRefType *demo.targetAbstract
	//4: ByValType *demo.ByValType
	//5: ByValType *demo.ByValType
	//6: []ByValType *demo.targetAbstract
	//7: []*ByValType *demo.targetAbstract
	//8: nil
	//9: ByValType *demo.ByValType
	//10: ByValType *demo.ByValType
	//11: ByValType *demo.ByValType
	//12: ByValType *demo.ByValType
	//13: []Target *demo.targetAbstract
	//14: []*Target *demo.targetAbstract
	//15: []Target *demo.targetAbstract
}

// This example shows how an error can be returned from a visitor function.
func Example_error() {
	data, _ := demo.NewContainer(true)
	ret, changed, err := data.WalkTarget(
		func(ctx demo.TargetContext, x demo.Target) demo.TargetDecision {
			return ctx.Error(errors.New("an error"))
		})
	fmt.Println(ret, changed, err)

	//Output:
	//<nil> false an error
}

// This example demonstrates how enhanced visitable types can be
// visited with a function.
func Example_walk() {
	data, _ := demo.NewContainer(true)
	var container, byVal, byRef int
	_, _, err := data.WalkTarget(func(ctx demo.TargetContext, x demo.Target) (d demo.TargetDecision) {
		switch x.(type) {
		case *demo.ContainerType:
			container++
		case *demo.ByValType:
			byVal++
		case *demo.ByRefType:
			byRef++
		default:
			return ctx.Error(fmt.Errorf("unknown type %s", reflect.TypeOf(x)))
		}
		return
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Saw %d Container, %d ByValType, and %d ByRefType",
		container, byVal, byRef)
	//Output:
	//Saw 1 Container, 17 ByValType, and 6 ByRefType
}

// This example demonstrates how pre- and post-visitation works. It
// also shows how visitation can be short-circuited.
func Example_post() {
	data, _ := demo.NewContainer(true)
	_, _, err := data.WalkTarget(func(ctx demo.TargetContext, x demo.Target) demo.TargetDecision {
		switch x.(type) {
		case *demo.ContainerType:
			fmt.Println("pre container")
			return ctx.Continue().Post(func(demo.TargetContext, demo.Target) (d demo.TargetDecision) {
				fmt.Println("post container")
				return
			})
		default:
			fmt.Println("halting")
			return ctx.Halt()
		}
	})
	if err != nil {
		panic(err)
	}

	//Output:
	//pre container
	//halting
	//post container
}

// This example demonstrates how copy-on-replace can be implemented
// for "immutable" datastructures.
func Example_replace() {
	data, _ := demo.NewContainer(true)
	count := 0
	data2, changed, err := data.WalkTarget(
		func(ctx demo.TargetContext, x demo.Target) demo.TargetDecision {
			switch t := x.(type) {
			case *demo.ByRefType:
				count++
				cp := *t
				cp.Val = fmt.Sprintf("ByRef %d", count)
				return ctx.Skip().Replace(&cp)

			case *demo.ByValType:
				count++
				cp := *t
				cp.Val = fmt.Sprintf("ByVal %d", count)
				return ctx.Skip().Replace(&cp)
			default:
				return ctx.Continue()
			}
		})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Changed: %v\n", changed)
	fmt.Printf("data != data2: %v", data != data2)

	//Output:
	//Changed: true
	//data != data2: true
}
