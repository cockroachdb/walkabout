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
	TemplateSources["50union"] = `
{{- $v := . -}}
{{- $Union := $v.Root.Union -}}
{{- if $Union -}}
// ------ Union Support -----
type {{ $Union }} interface {
	{{ $Union }}Abstract
	is{{ $Union }}Type()
}

var (
{{- range $s := Structs $v }}
	_ {{ $Union }} = &{{ $s }}{}
{{- end -}}
)

{{- range $s := Structs $v }}
func (*{{ $s }}) is{{ $Union }}Type() {}
{{- end -}}
{{- end -}}
`
}
