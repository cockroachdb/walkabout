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
	TemplateSources["75typemap"] = `
{{- $v := . -}}
{{- $Context := T $v "Context" -}}
{{- $Engine := t $v "Engine" -}}
{{- $TypeID := T $v "TypeID" -}}
{{- $WalkerFn := T $v "WalkerFn" -}}
// ------ Type Mapping ------
var {{ $Engine }} = e.New(e.TypeMap {
// ------ Structs ------
{{ range $s := Structs $v }}{{ TypeID $s }}: {
	Copy: func(dest, from e.Ptr) { *(*{{ $s }})(dest) = *(*{{ $s }})(from) },
	Facade: func(impl e.Context, fn e.FacadeFn, x e.Ptr) e.Decision {
		return e.Decision(fn.({{ $WalkerFn }})({{ $Context }}{impl}, (*{{ $s }})(x)))
	},
	Fields: []e.FieldInfo {
		{{ range $f := $s.Fields -}}
		{ Name: "{{ $f }}", Offset: unsafe.Offsetof({{ $s }}{}.{{ $f }}), Target: e.TypeID({{ TypeID $f.Target }})},
		{{ end }}
	},
	Name: "{{ $s }}",
	NewStruct: func() e.Ptr { return e.Ptr(&{{ $s }}{}) },
	SizeOf: unsafe.Sizeof({{ $s }}{}),
	Kind: e.KindStruct,
	TypeID: e.TypeID({{ TypeID $s }}),
},
{{ end }}
// ------ Interfaces ------
{{ range $s := Intfs $v }}{{ TypeID $s }}: {
	Copy: func(dest, from e.Ptr) {
		*(*{{ $s }})(dest) = *(*{{ $s }})(from)
	},
	IntfType: func(x e.Ptr) e.TypeID {
		d := *(*{{ $s }})(x)
		switch d.(type) {
		{{ range $imp := Implementors $s -}}
		case {{ $imp.Actual }}: return e.TypeID({{ TypeID $imp.Underlying }});
		{{- end }}
		default:
			return 0
		}
	},
	IntfWrap: func(id e.TypeID, x e.Ptr) e.Ptr {
		var d {{ $s }}
		switch {{ $TypeID }}(id) {
		{{ range $imp := Implementors $s -}}
			{{- if IsPointer $imp.Actual -}}
				case {{ TypeID $imp.Actual.Elem }}: d = (*{{ $imp.Actual.Elem }})(x);
				case {{ TypeID $imp.Actual }}: d = *(*{{ $imp.Actual }})(x);
			{{- end -}}
		{{- end }}
		default:
			return nil
		}
		return e.Ptr(&d)
	},
	Kind: e.KindInterface,
	Name: "{{ $s }}",
	SizeOf: unsafe.Sizeof({{ $s }}(nil)),
	TypeID: e.TypeID({{ TypeID $s }}),
},
{{ end }}
// ------ Pointers ------
{{ range $s := Pointers $v }}{{ TypeID $s }}: {
	Copy: func(dest, from e.Ptr) {
		*(*{{ $s }})(dest) = *(*{{ $s }})(from)
	},
	Elem: e.TypeID({{ TypeID $s.Elem }}),
	SizeOf: unsafe.Sizeof(({{ $s }})(nil)),
	Kind: e.KindPointer,
	TypeID: e.TypeID({{ TypeID $s }}),
},
{{ end }}
// ------ Slices ------
{{ range $s := Slices $v }}{{ TypeID $s }}: {
	Copy: func(dest, from e.Ptr) {
		*(*{{ $s }})(dest) = *(*{{ $s }})(from)
	},
	Elem: e.TypeID({{ TypeID $s.Elem }}),
	Kind: e.KindSlice,
	NewSlice: func(size int) e.Ptr {
		x := make({{ $s }}, size)
		return e.Ptr(&x)
	},
	SizeOf: unsafe.Sizeof(({{ $s }})(nil)),
	TypeID: e.TypeID({{ TypeID $s }}),
},
{{ end }}
})

// These are lightweight type tokens. 
const (
	_ {{ T $v "TypeID" }} = iota
{{ range $t := $v.Types }}{{ TypeID $t }};{{ end }}
)

// String is for debugging use only.
func (t {{ $TypeID }}) String() string {
	return {{ $Engine }}.Stringify(e.TypeID(t))
}
`
}
