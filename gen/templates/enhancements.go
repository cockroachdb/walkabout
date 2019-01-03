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
	TemplateSources["50enhancements"] = `
{{- $v := . -}}
{{- $abstract := t $v "Abstract" -}}
{{- $Abstract := T $v "Abstract" -}}
{{- $Engine := t $v "Engine" -}}
{{- $identify := t $v "Identify" -}}
{{- $Root := $v.Root -}}
{{- $TypeId := T $v "TypeId" -}}
{{- $WalkerFn := T $v "WalkerFn" -}}
{{- $wrap := t $v "Wrap" -}}

// ------ Type Enhancements ------

// {{ $abstract }} is a type-safe facade around e.Abstract.
type {{ $abstract }} struct {
	delegate *e.Abstract
}
var _ {{ $Abstract }} = &{{ $abstract }}{}

// ChildAt implements {{ $Abstract }}.
func (a *{{ $abstract }}) ChildAt(index int) (ret {{ $Abstract }}) {
	impl := a.delegate.ChildAt(index)
	if impl == nil {
		return nil
	}
	switch {{ $TypeId }}(impl.TypeId()) {
	{{ range $s := Structs $v -}}
	case {{ TypeId $s }}: ret = (*{{ $s }})(impl.Ptr());
	case {{ TypeId $s }}Ptr: ret = *(**{{ $s }})(impl.Ptr());
	{{- end }}
	default:
		ret = &{{ $abstract}}{impl}
	}
	return
}

// NumChildren implements {{ $Abstract }}.
func (a *{{ $abstract }}) NumChildren() int {
	return a.delegate.NumChildren()
}

// TypeId implements {{ $Abstract }}.
func (a *{{ $abstract }}) TypeId() {{ $TypeId }} {
	return {{ $TypeId }}(a.delegate.TypeId())
}

{{ range $s := Structs $v }}
// ChildAt implements {{ $Abstract }}.
func (x *{{ $s }}) ChildAt(index int) {{ $Abstract }} {
	self := &{{ $abstract }}{ {{ $Engine }}.Abstract(e.TypeId({{ TypeId $s }}), e.Ptr(x)) }
	return self.ChildAt(index)
}

// NumChildren returns {{ len $s.Fields }}.
func (x *{{ $s }}) NumChildren() int { return {{ len $s.Fields }} }

// TypeId returns {{ TypeId $s }}.
func (*{{ $s }}) TypeId() {{ $TypeId }} { return {{ TypeId $s }} }

// Walk{{ $Root }} visits the receiver with the provided callback. 
func (x *{{ $s }}) Walk{{ $Root }}(fn {{ $WalkerFn }}) (_ *{{ $s }}, changed bool, err error) {
	var y e.Ptr
	_, y, changed, err = {{ $Engine }}.Execute(fn, e.TypeId({{ TypeId $s }}), e.Ptr(x), e.TypeId({{ TypeId $s }}))
	if err != nil {
		return nil, false, err
	}
	return (*{{ $s }})(y), changed, nil
}
{{ end }}

// Walk{{ $Root }} visits the receiver with the provided callback. 
func Walk{{ $Root }}(x {{ $Root }}, fn {{ $WalkerFn }}) (_ {{ $Root }}, changed bool, err error) {
  id, ptr := {{ $identify }}(x)
	id, ptr, changed, err = {{ $Engine }}.Execute(fn, id, ptr, e.TypeId({{ TypeId $Root }}))
	if err != nil {
		return nil, false, err
	}
	if changed {
		return {{ $wrap }}(id, ptr), true, nil
	}
	return x, false, nil
}
`
}
