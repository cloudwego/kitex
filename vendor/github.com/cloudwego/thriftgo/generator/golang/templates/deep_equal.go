// Copyright 2021 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package templates

// StructLikeDeepEqual .
var StructLikeDeepEqual = `
{{define "StructLikeDeepEqual"}}
{{- $TypeName := .GoName}}
func (p *{{$TypeName}}) DeepEqual(ano *{{$TypeName}}) bool {
	if p == ano {
		return true
	} else if p == nil || ano == nil {
		return false
	}
	{{- range .Fields}}
	if !p.{{.DeepEqual}}(ano.{{.GoName}}) {
		return false
	}
	{{- end}}
	return true
}
{{- end}}{{/* "StructLikeDeepEqual" */}}
`

// StructLikeDeepEqualField .
var StructLikeDeepEqualField = `
{{define "StructLikeDeepEqualField"}}
{{- $TypeName := .GoName}}
{{- range .Fields}}
{{- $ctx := MkRWCtx .}}
func (p *{{$TypeName}}) {{.DeepEqual}}({{$ctx.Source}} {{$ctx.TypeName}}) bool {
	{{template "FieldDeepEqual" $ctx}}
	return true
}
{{- end}}{{/* range .Fields */}}
{{- end}}{{/* "StructLikeDeepEqualField" */}}
`

// FieldDeepEqual .
var FieldDeepEqual = `
{{define "FieldDeepEqual"}}
{{- if .Type.Category.IsStructLike}}
	{{- template "FieldDeepEqualStructLike" .}}
{{- else if .Type.Category.IsContainerType}}
	{{- template "FieldDeepEqualContainer" .}}
{{- else}}{{/* IsBaseType */}}
	{{- template "FieldDeepEqualBase" .}}
{{- end}}
{{- end}}{{/* "FieldDeepEqual" */}}
`

// FieldDeepEqualStructLike .
var FieldDeepEqualStructLike = `
{{define "FieldDeepEqualStructLike"}}
	if !{{.Target}}.DeepEqual({{.Source}}) {
		return false
	}
{{- end}}{{/* "FieldDeepEqualStructLike" */}}
`

// FieldDeepEqualBase .
var FieldDeepEqualBase = `
{{define "FieldDeepEqualBase"}}
	{{- if .IsPointer}}
	if {{.Target}} == {{.Source}} {
		return true
	} else if {{.Target}} == nil || {{.Source}} == nil {
		return false
	}
	{{- end}}{{/* if .IsPointer */}}
	{{- $tgt := .Target}}
	{{- $src := .Source}}
	{{- if .IsPointer}}{{$tgt = printf "*%s" $tgt}}{{$src = printf "*%s" $src}}{{end}}
	{{- if .Type.Category.IsString}}
		{{- UseStdLibrary "strings"}}
		if strings.Compare({{$tgt}}, {{$src}}) != 0 {
			return false
		}
	{{- else if .Type.Category.IsBinary}}
		{{- UseStdLibrary "bytes"}}
		if bytes.Compare({{$tgt}}, {{$src}}) != 0 {
			return false
		}
	{{- else}}{{/* IsFixedLengthType */}}
		if {{$tgt}} != {{$src}} {
			return false
		}
	{{- end}}{{/* if .Type.Category.IsString */}}
{{- end}}{{/* "FieldDeepEqualBase" */}}
`

// FieldDeepEqualContainer .
var FieldDeepEqualContainer = `
{{define "FieldDeepEqualContainer"}}
	{{- if .IsPointer}}
	if {{.Target}} == {{.Source}} {
		return true
	} else if {{.Target}} == nil || {{.Source}} == nil {
		return false
	}
	{{- end}}{{/* if .IsPointer */}}
	if len({{.Target}}) != len({{.Source}}) {
		return false
	}
	{{- $src := .GenID "_src"}}
	{{- $idx := "i"}}
	{{- if eq .Type.Category.String "Map" }}{{$idx = "k"}}{{end}}
	for {{$idx}}, v := range {{.Target}} {
		{{$src}} := {{.Source}}[{{$idx}}]
		{{- $ctx := (.ValCtx.WithTarget "v").WithSource $src}}
		{{- template "FieldDeepEqual" $ctx}}
	}
{{- end}}{{/* "FieldDeepEqualContainer" */}}
`
