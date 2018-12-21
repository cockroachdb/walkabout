# Walkabout

```
$ walkabout --help
walkabout is a code-generation tool to enhance struct types.
https://github.com/cockroachdb/walkabout

Usage:
  walkabout [flags]

Examples:

walkabout InterfaceName 
  Generates support code to make all struct types that implement
  the given interface walkable.

walkabout --union UnionInterface ( InterfaceName | StructName ) ...
  Generates an interface called "UnionInterface" which will be
  implemented by the named struct types, or those structs that implement
  the named interface(s).

walkabout --union UnionInterface --reachable ( InterfaceName | StructName ) ...
  As above, but also includes all types in the same package that are
  transitively reachable from the named types.  This is useful for
  refitting an entire package where the existing types may not all
  share a common interface.


Flags:
  -d, --dir string     the directory to operate in (default ".")
  -h, --help           help for walkabout
  -o, --out string     overrides the output file name
  -r, --reachable      make all transitively reachable types in the same package also
                       implement the --union interface. Only valid when using --union.
  -u, --union string   generate a new interface with the given name to be used as the
                       visitable interface.
```

## Api

Walkabout generates two complementary APIs from existing golang sources:
* A [traversal API](https://godoc.org/github.com/cockroachdb/walkabout/demo#example-package--Walk)
  in which a visitor function is applied to a value and
  all of its children. The visitor function can mutate values in-place,
  or apply a copy-on-mutate behavior to edit "immutable" object graphs.
* An ["abstract accessor"](https://godoc.org/github.com/cockroachdb/walkabout/demo#example-package--Abstract)
  API, which allows a visitable type to be treated as though it were
  simply a tree of homogeneous nodes.

## Features

* Allocation-free: running a no-op visitor over a structure
  causes [no heap allocations](./demo/benchmark_test.go).
* Cycle-free: cycles are detected and broken. Note that this does not
  implement exactly-once behavior, but it will prevent infinite loops. 
* Dependency-free: the generated code and support library depend only
  on built-in packages.
* Recursion-free: the [core traversal code](./engine/engine.go) simply
  operates in a loop.
* Reflection-free: all type analysis is performed at generation time
  and `reflect.Value` is not used.

## Use

Walkabout is driven by your existing source code. There is no special
DSL, no field tags, just plain-old, idiomatic `struct` types. If the
types that you want to make visitable already implement a common
interface, you're all set. If not, don't worry, there's a flag for that.

Whenever Walkabout generates code, there is always a singular
"visitable" interface in mind.  This can either be a single, existing
interface specified on the command line, or one can be synthesized by
using the `--union` flag.  When using the `--union` flag, the types
specified on the command line may be interface or struct types. In
either mode, we'll refer to the types specified on the command-line as
"seed" types.

Walkabout will generate methods for the following "visitable" types:
* An exported struct which implements a seed interface or is a seed type.
* A slice of a visitable type.
* A pointer to a visitable type.
* An alias of a visitable type.
* Any combination of the above.
* If `--reachable` is used, any potentially-visitable type in the
  current package that is reachable from another visitable type.

## Installing

`go get github.com/cockroachdb/walkabout`

## Status

Walkabout is currently experimental and is under active development as
of December 2018 with the goal of being able to refit many of
CockroachDB's type hierarchies with traversal code.

## Future work

* Implement support for map-valued fields.
* Implement a `Parallel()` decision type to allow the fields of a struct
  or elements of a slice to be visited concurrently.
* Override field-traversal order / filtering of fields.
* Feature flags to turn off e.g. cycle-checking, abstract accessors, etc.
* Visiting arbitrary named types that implement a seed interface
  (e.g. `type ScalarValue int`).
