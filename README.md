# Walkabout

Walkabout is a code-generation tool to enhance struct types.

Walkabout generates two complementary APIs:
* A traversal API in which a visitor function is applied to a value and
  all of its children. The visitor function can mutate values in-place,
  or apply a copy-on-mutate behavior to edit "immutable" object graphs.
* An "abstract accessor" API, which allows a visitable type to be
  treated as though it were simply a tree of homogeneous nodes.

## Features

* Recursion-free: the [core traversal code](./engine/engine.go) simply
  operates in a loop.
* Allocation-free: running a no-op visitor over a structure
  causes [no heap allocations](./demo/benchmark_test.go).
* Cycle-free: cycles are detected and broken. Note that this does not
  implement exactly-once behavior, but it will prevent infinite loops. 
* Reflection-free: all type analysis is performed at generation time
  and `reflect.Value` is not used.
* Dependency-free: the generated code and support library depend only
  on built-in packages.

## Walkthrough

Walkabout enhances named struct types with additional methods and
metadata. In order to know which types to operate on, you must define
a common interface.  In the [demo](./demo/demo.go) code, this interface
is called `Target`. There is nothing special about this interface,
save that there are various named struct types which implement it. 

The `walkabout` generator is invoked in a source-code directory and
given the name of one or more of these "visitable" interfaces.
In our case running `walkabout Target` in the `demo` directory will
result in the creation of a `target_walkabout.g.go` file.

For each struct which implements the interface, walkabout identifies
exported fields of visitable types and produces code and metadata to
traverse it. A visitable type is:
* A struct that implements the visitable interface, with either
  pointer or value receiver methods.
* A slice of a visitable type.
* A pointer to a visitable type.
* An alias of a visitable type.
* Any combination of the above.

## Installing

`go get github.com/cockroachdb/walkabout`

## Status

Walkabout is currently experimental and is under active development as
of December 2018.

## Future work

* Implement support for map-valued fields.
* Implement a `Parallel()` decision type to allow the fields of a struct
  or elements of a slice to be visited concurrently.