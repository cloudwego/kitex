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

// StructLike is the code template for struct, union, and exception.
var StructLike = `
{{define "StructLike"}}
{{- $TypeName := .GoName}}
{{InsertionPoint .Category .Name}}
type {{$TypeName}} struct {
{{- range .Fields}}
	{{- InsertionPoint $.Category $.Name .Name}}
	{{(.GoName)}} {{.GoTypeName}} {{GenTags .Field (InsertionPoint $.Category $.Name .Name "tag")}} 
{{- end}}
	{{if Features.KeepUnknownFields}}_unknownFields unknown.Fields{{end}}
}

func New{{$TypeName}}() *{{$TypeName}} {
	return &{{$TypeName}}{
		{{template "StructLikeDefault" .}}
	}
}

{{template "FieldGetOrSet" .}}

{{if eq .Category "union"}}
func (p *{{$TypeName}}) CountSetFields{{$TypeName}}() int {
	count := 0
	{{- range .Fields}}
	{{- if SupportIsSet .Field}}
	if p.{{.IsSetter}}() {
		count++
	}
	{{- end}}
	{{- end}}
	return count
}
{{- end}}

{{if Features.KeepUnknownFields}}
func (p *{{$TypeName}}) CarryingUnknownFields() bool {
	return len(p._unknownFields) > 0
}
{{end}}{{/* if Features.KeepUnknownFields */}}

var fieldIDToName_{{$TypeName}} = map[int16]string{
{{- range .Fields}}
	{{.ID}}: "{{.Name}}",
{{- end}}{{/* range .Fields */}}
}

{{template "FieldIsSet" .}}

{{template "StructLikeRead" .}}

{{template "StructLikeReadField" .}}

{{template "StructLikeWrite" .}}

{{template "StructLikeWriteField" .}}

func (p *{{$TypeName}}) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("{{$TypeName}}(%+v)", *p)
}

{{- if eq .Category "exception"}}
func (p *{{$TypeName}}) Error() string {
	return p.String()
}
{{- end}}

{{- if Features.GenDeepEqual}}
{{template "StructLikeDeepEqual" .}}

{{template "StructLikeDeepEqualField" .}}
{{- end}}

{{- end}}{{/* define "StructLike" */}}
`

// StructLikeDefault is the code template for structure initialization.
var StructLikeDefault = `
{{- define "StructLikeDefault"}}
{{- range .Fields}}
	{{- if .IsSetDefault}}
		{{.GoName}}: {{.DefaultValue}},
	{{- end}}
{{- end}}
{{- end -}}`

// StructLikeRead .
var StructLikeRead = `
{{define "StructLikeRead"}}
{{- $TypeName := .GoName}}
func (p *{{$TypeName}}) Read(iprot thrift.TProtocol) (err error) {
	{{if Features.KeepUnknownFields}}var name string{{end}}
	var fieldTypeId thrift.TType
	var fieldId int16
	{{- range .Fields}}
	{{- if .Requiredness.IsRequired}}
	var isset{{.GoName}} bool = false
	{{- end}}
	{{- end}}

	if _, err = iprot.ReadStructBegin(); err != nil {
		goto ReadStructBeginError
	}

	for {
		{{if Features.KeepUnknownFields}}name{{else}}_{{end}}, fieldTypeId, fieldId, err = iprot.ReadFieldBegin()
		if err != nil {
		    goto ReadFieldBeginError
		}
		if fieldTypeId == thrift.STOP {
			break;
		}
		{{if or (gt (len .Fields) 0) Features.KeepUnknownFields}}
		switch fieldId {
		{{- range .Fields}}
		case {{.ID}}:
			if fieldTypeId == thrift.{{.Type | GetTypeIDConstant }} {
				if err = p.{{.Reader}}(iprot); err != nil {
					goto ReadFieldError
				}
				{{- if .Requiredness.IsRequired}}
				isset{{.GoName}} = true
				{{- end}}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		{{- end}}{{/* range .Fields */}}
		default:
			{{- if Features.KeepUnknownFields}}
			if err = p._unknownFields.Append(iprot, name, fieldTypeId, fieldId); err != nil {
				goto UnknownFieldsAppendError
			}
			{{- else}}
		    if err = iprot.Skip(fieldTypeId); err != nil {
				goto SkipFieldError
		    }
			{{- end}}{{/* if Features.KeepUnknownFields */}}
		}
		{{- else -}}
		if err = iprot.Skip(fieldTypeId); err != nil {
		    goto SkipFieldTypeError
		}
		{{- end}}{{/* if len(.Fields) > 0 */}}

		if err = iprot.ReadFieldEnd(); err != nil {
		  goto ReadFieldEndError
		}
	}
	if err = iprot.ReadStructEnd(); err != nil {
		goto ReadStructEndError
	}
	{{ $RequiredFieldNotSetError := false }}
	{{- range .Fields}}
	{{- if .Requiredness.IsRequired}}
	{{ $RequiredFieldNotSetError = true }}
	if !isset{{.GoName}} {
		fieldId = {{.ID}}
		goto RequiredFieldNotSetError
	}
	{{- end}}
	{{- end}}{{/* range .Fields */}}
	return nil
ReadStructBeginError:
	return thrift.PrependError(fmt.Sprintf("%T read struct begin error: ", p), err)
ReadFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T read field %d begin error: ", p, fieldId), err)

{{- if gt (len .Fields) 0}}
ReadFieldError:
	return thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_{{$TypeName}}[fieldId]), err)
SkipFieldError:
	return thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)
{{- end}}

{{- if Features.KeepUnknownFields}}
UnknownFieldsAppendError:
	return thrift.PrependError(fmt.Sprintf("%T append unknown field(name:%s type:%d id:%d) error: ", p, name, fieldTypeId, fieldId), err)
{{- end}}

{{- if and (eq (len .Fields) 0) (not Features.KeepUnknownFields)}}
SkipFieldTypeError:
	return thrift.PrependError(fmt.Sprintf("%T skip field type %d error", p, fieldTypeId), err)
{{- end}}

ReadFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T read field end error", p), err)
ReadStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
{{- if $RequiredFieldNotSetError}}
RequiredFieldNotSetError:
	return thrift.NewTProtocolExceptionWithType(thrift.INVALID_DATA, fmt.Errorf("Required field %s is not set", fieldIDToName_{{$TypeName}}[fieldId]))
{{- end}}{{/* if $RequiredFieldNotSetError */}}
}
{{- end}}{{/* define "StructLikeRead" */}}
`

// StructLikeReadField .
var StructLikeReadField = `
{{define "StructLikeReadField"}}
{{- $TypeName := .GoName}}
{{- range .Fields}}
{{$FieldName := .GoName}}
func (p *{{$TypeName}}) {{.Reader}}(iprot thrift.TProtocol) error {
	{{- $ctx := MkRWCtx .}}
	{{- template "FieldRead" $ctx}}
	return nil
}
{{- end}}{{/* range .Fields */}}
{{- end}}{{/* define "StructLikeReadField" */}}
`

// StructLikeWrite .
var StructLikeWrite = `
{{define "StructLikeWrite"}}
{{- $TypeName := .GoName}}
func (p *{{$TypeName}}) Write(oprot thrift.TProtocol) (err error) {
	{{- if gt (len .Fields) 0 }}
	var fieldId int16
	{{- end}}
	{{- if eq .Category "union"}}
	var c int
	if c = p.CountSetFields{{$TypeName}}(); c != 1 {
		goto CountSetFieldsError
	}
	{{- end}}
	if err = oprot.WriteStructBegin("{{.Name}}"); err != nil {
		goto WriteStructBeginError
	}
	if p != nil {
		{{- range .Fields}}
		if err = p.{{.Writer}}(oprot); err != nil {
			fieldId = {{.ID}}
			goto WriteFieldError
		}
		{{- end}}
		{{if Features.KeepUnknownFields}}
		if err = p._unknownFields.Write(oprot); err != nil {
			goto UnknownFieldsWriteError
		}
		{{- end}}
	}
	if err = oprot.WriteFieldStop(); err != nil {
		goto WriteFieldStopError
	}
	if err = oprot.WriteStructEnd(); err != nil {
		goto WriteStructEndError
	}
	return nil
{{- if eq .Category "union"}}
CountSetFieldsError:
	return fmt.Errorf("%T write union: exactly one field must be set (%d set).", p, c)
{{- end}}
WriteStructBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write struct begin error: ", p), err)
{{- if gt (len .Fields) 0 }}
WriteFieldError:
	return thrift.PrependError(fmt.Sprintf("%T write field %d error: ", p, fieldId), err)
{{- end}}
WriteFieldStopError:
	return thrift.PrependError(fmt.Sprintf("%T write field stop error: ", p), err)
WriteStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T write struct end error: ", p), err)
{{- if Features.KeepUnknownFields}}
UnknownFieldsWriteError:
	return thrift.PrependError(fmt.Sprintf("%T write unknown fields error: ", p), err)
{{- end}}{{/* if Features.KeepUnknownFields */}}
}
{{- end}}{{/* define "StructLikeWrite" */}}
`

// StructLikeWriteField .
var StructLikeWriteField = `
{{define "StructLikeWriteField"}}
{{- $TypeName := .GoName}}
{{- range .Fields}}
{{- $FieldName := .GoName}}
{{- $IsSetName := .IsSetter}}
{{- $TypeID := .Type | GetTypeIDConstant }}
func (p *{{$TypeName}}) {{.Writer}}(oprot thrift.TProtocol) (err error) {
	{{- if .Requiredness.IsOptional}}
	if p.{{$IsSetName}}() {
	{{- end}}
	if err = oprot.WriteFieldBegin("{{.Name}}", thrift.{{$TypeID}}, {{.ID}}); err != nil {
		goto WriteFieldBeginError
	}
	{{- $ctx := MkRWCtx .}}
	{{- template "FieldWrite" $ctx}}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	{{- if .Requiredness.IsOptional}}
	}
	{{- end}}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field {{.ID}} begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field {{.ID}} end error: ", p), err)
}
{{end}}{{/* range .Fields */}}
{{- end}}{{/* define "StructLikeWriteField" */}}
`

// FieldGetOrSet .
var FieldGetOrSet = `
{{define "FieldGetOrSet"}}
{{- $TypeName := .GoName}}
{{- range .Fields}}
{{- $FieldName := .GoName}}
{{- $FieldTypeName := .GoTypeName}}
{{- $DefaultVarTypeName := .DefaultTypeName}}
{{- $GetterName := .Getter}}
{{- $SetterName := .Setter}}
{{- $IsSetName := .IsSetter}}

{{if SupportIsSet .Field}}
{{$DefaultVarName := printf "%s_%s_%s" $TypeName $FieldName "DEFAULT"}}
var {{$DefaultVarName}} {{$DefaultVarTypeName}}
{{- if .Default}} = {{.DefaultValue}}{{- end}}

func (p *{{$TypeName}}) {{$GetterName}}() {{$DefaultVarTypeName}} {
	if !p.{{$IsSetName}}() {
		return {{$DefaultVarName}}
	}
	{{- if and (NeedRedirect .Field) (IsBaseType .Type)}}
	return *p.{{$FieldName}}
	{{- else}}
	return p.{{$FieldName}}
	{{- end}}
}

{{- else}}{{/*if SupportIsSet . */}}

func (p *{{$TypeName}}) {{$GetterName}}() {{$FieldTypeName}} {
	return p.{{$FieldName}}
}

{{- end}}{{/* if SupportIsSet . */}}
{{- end}}{{/* range .Fields */}}

{{- if Features.GenerateSetter}}
{{- range .Fields}}
{{- $FieldName := .GoName}}
{{- $FieldTypeName := .GoTypeName}}
{{- $SetterName := .Setter}}
{{- if .IsResponseFieldOfResult}}
func (p *{{$TypeName}}) {{$SetterName}}(x interface{}) {
    p.{{$FieldName}} = x.({{$FieldTypeName}})
}
{{- else}}
func (p *{{$TypeName}}) {{$SetterName}}(val {{$FieldTypeName}}) {
	p.{{$FieldName}} = val
}
{{- end}}
{{- end}}{{/* range .Fields */}}
{{- end}}{{/* if Features.GenerateSetter */}}

{{- end}}{{/* define "FieldGetOrSet" */}}
`

// FieldIsSet .
var FieldIsSet = `
{{define "FieldIsSet"}}
{{- $TypeName := .GoName}}
{{- range .Fields}}
{{- $FieldName := .GoName}}
{{- $IsSetName := .IsSetter}}
{{- $FieldTypeName := .GoTypeName}}
{{- $DefaultVarName := printf "%s_%s_%s" $TypeName $FieldName "DEFAULT"}}
{{- if SupportIsSet .Field}}
func (p *{{$TypeName}}) {{$IsSetName}}() bool {
	{{- if .IsSetDefault}}
		{{- if IsBaseType .Type}}
			{{- if .Type.Category.IsBinary}}
				return string(p.{{$FieldName}}) != string({{$DefaultVarName}})
			{{- else}}
				return p.{{$FieldName}} != {{$DefaultVarName}}
			{{- end}}
		{{- else}}{{/* container type or struct-like */}}
			return p.{{$FieldName}} != nil
		{{- end}}
	{{- else}}
		return p.{{$FieldName}} != nil
	{{- end}}
}
{{end}}
{{- end}}{{/* range .Fields */}}
{{- end}}{{/* define "FieldIsSet" */}}
`

// FieldRead .
var FieldRead = `
{{define "FieldRead"}}
	{{- if .Type.Category.IsStructLike}}
		{{- template "FieldReadStructLike" .}}
	{{- else if .Type.Category.IsContainerType}}
		{{- template "FieldReadContainer" .}}
	{{- else}}{{/* IsBaseType */}}
		{{- template "FieldReadBaseType" .}}
	{{- end}}
{{- end}}{{/* define "FieldRead" */}}
`

// FieldReadStructLike .
var FieldReadStructLike = `
{{define "FieldReadStructLike"}}
	{{- .Target}} {{if .NeedDecl}}:{{end}}= {{.TypeName.Deref.NewFunc}}()
	if err := {{.Target}}.Read(iprot); err != nil {
		return err
	}
{{- end}}{{/* define "FieldReadStructLike" */}} 
`

// FieldReadBaseType .
var FieldReadBaseType = `
{{define "FieldReadBaseType"}}
	{{- $DiffType := or .Type.Category.IsEnum .Type.Category.IsBinary}}
	{{- if .NeedDecl}}
	var {{.Target}} {{.TypeName}}
	{{- end}}
	if v, err := iprot.Read{{.TypeID}}(); err != nil {
		return err
	} else {
	{{- if .IsPointer}}
		{{- if $DiffType}}
		tmp := {{.TypeName.Deref}}(v)
		{{.Target}} = &tmp
		{{- else -}}
		{{.Target}} = &v
		{{- end}}
	{{- else}}
		{{- if $DiffType}}
		{{.Target}} = {{.TypeName}}(v)
		{{- else}}
		{{.Target}} = v
		{{- end}}
	{{- end}}
	}
{{- end}}{{/* define "FieldReadBaseType" */}}
`

// FieldReadContainer .
var FieldReadContainer = `
{{define "FieldReadContainer"}}
	{{- if eq "Map" .TypeID}}
	     {{- template "FieldReadMap" .}}
	{{- else if eq "List" .TypeID}}
	     {{- template "FieldReadList" .}}
	{{- else}}
	     {{- template "FieldReadSet" .}}
	{{- end}}
{{- end}}{{/* define "FieldReadContainer" */}}
`

// FieldReadMap .
var FieldReadMap = `
{{define "FieldReadMap"}}
	_, _, size, err := iprot.ReadMapBegin()
	if err != nil {
		return err
	}
	{{.Target}} {{if .NeedDecl}}:{{end}}= make({{.TypeName}}, size)
	for i := 0; i < size; i++ {
		{{- $key := .GenID "_key"}}
		{{- $ctx := .KeyCtx.WithDecl.WithTarget $key}}
		{{- template "FieldRead" $ctx}}
		{{/* line break */}}
		{{- $val := .GenID "_val"}}
		{{- $ctx := .ValCtx.WithDecl.WithTarget $val}}
		{{- template "FieldRead" $ctx}}

		{{if and .ValCtx.Type.Category.IsStructLike Features.ValueTypeForSIC}}
			{{$val = printf "*%s" $val}}
		{{end}}

		{{.Target}}[{{$key}}] = {{$val}}
	}
	if err := iprot.ReadMapEnd(); err != nil {
		return err
	}
{{- end}}{{/* define "FieldReadMap" */}}
`

// FieldReadSet .
var FieldReadSet = `
{{define "FieldReadSet"}}
	_, size, err := iprot.ReadSetBegin()
	if err != nil {
		return err
	}
	{{.Target}} {{if .NeedDecl}}:{{end}}= make({{.TypeName}}, 0, size)
	for i := 0; i < size; i++ {
		{{- $val := .GenID "_elem"}}
		{{- $ctx := .ValCtx.WithDecl.WithTarget $val}}
		{{- template "FieldRead" $ctx}}

		{{if and .ValCtx.Type.Category.IsStructLike Features.ValueTypeForSIC}}
			{{$val = printf "*%s" $val}}
		{{end}}

		{{.Target}} = append({{.Target}}, {{$val}})
	}
	if err := iprot.ReadSetEnd(); err != nil {
		return err
	}
{{- end}}{{/* define "FieldReadSet" */}}
`

// FieldReadList .
var FieldReadList = `
{{define "FieldReadList"}}
	_, size, err := iprot.ReadListBegin()
	if err != nil {
		return err
	}
	{{.Target}} {{if .NeedDecl}}:{{end}}= make({{.TypeName}}, 0, size)
	for i := 0; i < size; i++ {
		{{- $val := .GenID "_elem"}}
		{{- $ctx := .ValCtx.WithDecl.WithTarget $val}}
		{{- template "FieldRead" $ctx}}

		{{if and .ValCtx.Type.Category.IsStructLike Features.ValueTypeForSIC}}
			{{$val = printf "*%s" $val}}
		{{end}}

		{{.Target}} = append({{.Target}}, {{$val}})
	}
	if err := iprot.ReadListEnd(); err != nil {
		return err
	}
{{- end}}{{/* define "FieldReadList" */}}
`

// FieldWrite .
var FieldWrite = `
{{define "FieldWrite"}}
	{{- if .Type.Category.IsStructLike}}
		{{- template "FieldWriteStructLike" .}}
	{{- else if .Type.Category.IsContainerType}}
		{{- template "FieldWriteContainer" .}}
	{{- else}}{{/* IsBaseType */}}
		{{- template "FieldWriteBaseType" .}}
	{{- end}}
{{- end}}{{/* define "FieldWrite" */}}
`

// FieldWriteStructLike .
var FieldWriteStructLike = `
{{define "FieldWriteStructLike"}}
	if err := {{.Target}}.Write(oprot); err != nil {
		return err
	}
{{- end}}{{/* define "FieldWriteStructLike" */}}
`

// FieldWriteBaseType .
var FieldWriteBaseType = `
{{define "FieldWriteBaseType"}}
{{- $Value := .Target}}
{{- if .IsPointer}}{{$Value = printf "*%s" $Value}}{{end}}
{{- if .Type.Category.IsEnum}}{{$Value = printf "int32(%s)" $Value}}{{end}}
{{- if .Type.Category.IsBinary}}{{$Value = printf "[]byte(%s)" $Value}}{{end}}
	if err := oprot.Write{{.TypeID}}({{$Value}}); err != nil {
		return err
	}
{{- end}}{{/* define "FieldWriteBaseType" */}}
`

// FieldWriteContainer .
var FieldWriteContainer = `
{{define "FieldWriteContainer"}}
	{{- if eq "Map" .TypeID}}
		{{- template "FieldWriteMap" .}}
	{{- else if eq "List" .TypeID}}
		{{- template "FieldWriteList" .}}
	{{- else}}
		{{- template "FieldWriteSet" .}}
	{{- end}}
{{- end}}{{/* define "FieldWriteContainer" */}}
`

// FieldWriteMap .
var FieldWriteMap = `
{{define "FieldWriteMap"}}
	if err := oprot.WriteMapBegin(thrift.
		{{- .KeyCtx.Type | GetTypeIDConstant -}}
		, thrift.{{- .ValCtx.Type | GetTypeIDConstant -}}
		, len({{.Target}})); err != nil {
		return err
	}
	for k, v := range {{.Target}}{
		{{$ctx := .KeyCtx.WithTarget "k"}}
		{{- template "FieldWrite" $ctx}}
		{{$ctx := .ValCtx.WithTarget "v"}}
		{{- template "FieldWrite" $ctx}}
	}
	if err := oprot.WriteMapEnd(); err != nil {
		return err
	}
{{- end}}{{/* define "FieldWriteMap" */}}
`

// FieldWriteSet .
var FieldWriteSet = `
{{define "FieldWriteSet"}}
		if err := oprot.WriteSetBegin(thrift.
		{{- .ValCtx.Type | GetTypeIDConstant -}}
		, len({{.Target}})); err != nil {
			return err
		}
		{{- if Features.ValidateSet}}
		{{- $ctx := (.ValCtx.WithTarget "tgt").WithSource "src"}}
		for i := 0; i < len({{.Target}}); i++ {
			for j := i + 1; j < len({{.Target}}); j++ {
		{{- if Features.GenDeepEqual}}
				if func(tgt, src {{$ctx.TypeName}}) bool {
					{{- template "FieldDeepEqual" $ctx}}
					return true
				}({{.Target}}[i], {{.Target}}[j]) {
		{{- else}}
				{{- UseStdLibrary "reflect"}}
				if reflect.DeepEqual({{.Target}}[i], {{.Target}}[j]) {
		{{- end}}
					return thrift.PrependError("", fmt.Errorf("%T error writing set field: slice is not unique", {{.Target}}[i]))
				}
			}
		}
		{{- end}}
		for _, v := range {{.Target}} {
			{{- $ctx := .ValCtx.WithTarget "v"}}
			{{- template "FieldWrite" $ctx}}
		}
		if err := oprot.WriteSetEnd(); err != nil {
			return err
		}
{{- end}}{{/* define "FieldWriteSet" */}}
`

// FieldWriteList .
var FieldWriteList = `
{{define "FieldWriteList"}}
		if err := oprot.WriteListBegin(thrift.
		{{- .ValCtx.Type | GetTypeIDConstant -}}
		, len({{.Target}})); err != nil {
			return err
		}
		for _, v := range {{.Target}} {
			{{- $ctx := .ValCtx.WithTarget "v"}}
			{{- template "FieldWrite" $ctx}}
		}
		if err := oprot.WriteListEnd(); err != nil {
			return err
		}
{{- end}}{{/* define "FieldWriteList" */}}
`
