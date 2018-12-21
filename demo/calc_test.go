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

//+build !generationTest

package demo

import (
	"fmt"
	"strconv"
	"strings"
)

// This generation flow will find all types in this package that
// are reachable from the Calculation struct and create a
// Calc interface to unify them.
//go:generate walkabout --union Calc --reachable Calculation

// This example shows a toy calculator AST and how custom actions can be
// introduced into the visitation flow. We've decided to use a visitor
// to stringify the Calculation. It's not how one would generally
// approach this problem, but it's useful to demonstrate more advanced
// flow-control.
func Example_actions() {
	c := &Calculation{
		Expr: &Func{"Avg", []Expr{
			&BinaryOp{"+", &Scalar{1}, &Scalar{3}},
			&Func{"Sum", []Expr{
				&Scalar{10},
				&Func{"Random", []Expr{&Scalar{1}, &Scalar{10}}},
				&Scalar{99}}},
			&BinaryOp{"*", &Scalar{5}, &Scalar{3}},
		}},
	}

	var w strings.Builder
	_, _, err := WalkCalc(c, func(ctx CalcContext, x Calc) CalcDecision {
		switch t := x.(type) {
		case *BinaryOp:
			// With a BinaryOp, we want to visit left, print the operator,
			// then right. We'll generate a sequence of actions to be
			// executed.
			return ctx.Actions(
				ctx.ActionVisit(t.Left),
				ctx.ActionCall(func() error { w.WriteString(t.Operator); return nil }),
				ctx.ActionVisit(t.Right),
			)

		case *Func:
			// With a function, we want to print commas between the arguments.
			// Rather than creating an arbitrarily number of actions to
			// perform, an Intercept() function can be registered to
			// traverse a struct normally and receive a pre-visit for
			// the immediate children. We combine Intercept() and Post()
			// to print the closing parenthesis.
			w.WriteString(t.Fn)
			w.WriteString("(")

			comma := ""
			return ctx.Continue().Intercept(func(CalcContext, Calc) (ret CalcDecision) {
				w.WriteString(comma)
				comma = ", "
				return
			}).Post(func(CalcContext, Calc) (ret CalcDecision) {
				w.WriteString(")")
				return
			})

		case *Scalar:
			w.WriteString(strconv.Itoa(t.val))
		}
		return ctx.Continue()
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(w.String())

	//Output:
	//Avg(1+3, Sum(10, Random(1, 10), 99), 5*3)
}

type Calculation struct{ Expr Expr }

type Expr interface {
	Calc
	isExpr()
}

type BinaryOp struct {
	Operator string
	Left     Expr
	Right    Expr
}

func (*BinaryOp) isExpr() {}

type Scalar struct{ val int }

func (*Scalar) isExpr() {}

type Func struct {
	Fn   string
	Args []Expr
}

func (*Func) isExpr() {}
