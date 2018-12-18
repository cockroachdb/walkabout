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

package templates

func init() {
	TemplateSources["50typemap"] = `
{{- $v := . -}}
{{- $Context := T $v "Context" -}}
{{- $Engine := t $v "Engine" -}}
{{- $TypeId := T $v "TypeId" -}}
{{- $WalkerFn := T $v "WalkerFn" -}}

var {{ $Engine }} = e.New(e.TypeMap {
// ------ Structs ------
{{ range $s := $v.Structs }}{{ TypeId $s }}: {
	Copy: func(dest, from e.Ptr) { *(*{{ $s }})(dest) = *(*{{ $s }})(from) },
	Facade: func(impl e.ContextImpl, fn e.FacadeFn, x e.Ptr) e.DecisionImpl {
		return fn.({{ $WalkerFn }})({{ $Context }}{impl}, (*{{ $s }})(x)).impl
	},
	Fields: []e.FieldInfo {
		{{ range $f := $s.Fields -}}
		{ Name: "{{ $f }}", Offset: unsafe.Offsetof({{ $s }}{}.{{ $f }}), Target: e.TypeId({{ TypeId $f.Target }})},
		{{ end }}
	},
	NewStruct: func() e.Ptr { return e.Ptr(&{{ $s }}{}) },
	SizeOf: unsafe.Sizeof({{ $s }}{}),
	Kind: e.KindStruct,
	TypeId: e.TypeId({{ TypeId $s }}),
},
{{ end }}
// ------ Interfaces ------
{{ range $s := Intfs $v }}{{ TypeId $s }}: {
	Copy: func(dest, from e.Ptr) {
		*(*{{ $s }})(dest) = *(*{{ $s }})(from)
	},
	IntfType: func(x e.Ptr) e.TypeId {
		d := *(*{{ $s }})(x)
		switch d.(type) {
		{{ range $imp := Implementors $s -}}
		case {{ $imp.Actual }}: return e.TypeId({{ TypeId $imp.Underlying }});
		{{- end }}
		default:
			return 0
		}
	},
	IntfWrap: func(id e.TypeId, x e.Ptr) e.Ptr {
		var d {{ $s }}
		switch {{ $TypeId }}(id) {
		{{ range $imp := Implementors $s -}}
		case {{ TypeId $imp.Actual }}: d = *(*{{ $imp.Actual }})(x);
		{{- end }}
		}
		return e.Ptr(&d)
	},
	SizeOf: unsafe.Sizeof({{ $s }}(nil)),
	Kind: e.KindInterface,
	TypeId: e.TypeId({{ TypeId $s }}),
},
{{ end }}
// ------ Pointers ------
{{ range $s := Pointers $v }}{{ TypeId $s }}: {
	Copy: func(dest, from e.Ptr) {
		*(*{{ $s }})(dest) = *(*{{ $s }})(from)
	},
	Elem: e.TypeId({{ TypeId $s.Elem }}),
	SizeOf: unsafe.Sizeof(({{ $s }})(nil)),
	Kind: e.KindPointer,
	TypeId: e.TypeId({{ TypeId $s }}),
},
{{ end }}
// ------ Slices ------
{{ range $s := Slices $v }}{{ TypeId $s }}: {
	Copy: func(dest, from e.Ptr) {
		*(*{{ $s }})(dest) = *(*{{ $s }})(from)
	},
	Elem: e.TypeId({{ TypeId $s.Elem }}),
	Kind: e.KindSlice,
	NewSlice: func(size int) e.Ptr {
		x := make({{ $s }}, size)
		return e.Ptr(&x)
	},
	SizeOf: unsafe.Sizeof(({{ $s }})(nil)),
	TypeId: e.TypeId({{ TypeId $s }}),
},
{{ end }}
})

// These are lightweight type tokens. 
const (
	_ {{ T $v "TypeId" }} = iota
{{ range $k, $v := $v.TypeIds }}{{ $k }};{{ end }}
)

var {{ t $v "TypeIdNames" }} = [...]string{
	"<NIL>",
	{{ range $t := $v.TypeIds }}"{{ $t }}",
{{ end }} }

// String is for debugging use only.
func (t {{ $TypeId }}) String() string {
	return {{ t $v "TypeIdNames" }}[t]
}
`
}
