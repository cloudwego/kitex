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

// Enum .
var Enum = `
{{define "Enum"}}
{{- $EnumType := .GoName}}
{{InsertionPoint "enum" .Name}}
type {{$EnumType}} int64

const (
	{{- range .Values}}
	{{.GoName}} {{$EnumType}} = {{.Value}}
	{{- end}}
)

func (p {{$EnumType}}) String() string {
	switch p {
	{{- range .Values}}
	case {{.GoName}}:
		return "{{.GoLiteral}}"
	{{- end}}
	}
	return "<UNSET>"
}

func {{$EnumType}}FromString(s string) ({{$EnumType}}, error) {
	switch s {
	{{- range .Values}}
	case "{{.GoLiteral}}":
		return {{.GoName}}, nil
	{{- end}}
	}
	return {{$EnumType}}(0), fmt.Errorf("not a valid {{$EnumType}} string")
}

func {{$EnumType}}Ptr(v {{$EnumType}} ) *{{$EnumType}}  { return &v }

{{- if Features.MarshalEnumToText}}

func (p {{$EnumType}}) MarshalText() ([]byte, error) {
	return []byte(p.String()), nil
}

func (p *{{$EnumType}}) UnmarshalText(text []byte) error {
	q, err := {{$EnumType}}FromString(string(text))
	if err != nil {
		return err
	}
	*p = q
	return nil
}
{{- end}}{{/* if Features.MarshalEnumToText */}}

{{- if Features.ScanValueForEnum}}

func (p *{{$EnumType}}) Scan(value interface{}) (err error) {
	var result sql.NullInt64
	err = result.Scan(value)
	*p = {{$EnumType}}(result.Int64)
	return
}

func (p *{{$EnumType}}) Value() (driver.Value, error) {
	if p == nil {
		return nil, nil
	}
	return int64(*p), nil
}
{{- end}}{{/* if .Features.ScanValueForEnum */}}
{{end}}
`
