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

package thriftgo

const structLike = `
{{define "StructLike"}}
{{template "StructLikeFastRead" .}}

{{template "StructLikeFastReadField" .}}

{{template "StructLikeFastWrite" .}}

{{template "StructLikeFastWriteNocopy" .}}

{{template "StructLikeLength" .}}

{{template "StructLikeFastWriteField" .}}

{{template "StructLikeFieldLength" .}}
{{- end}}{{/* define "StructLike" */}}
`

const structLikeFastRead = `
{{define "StructLikeFastRead"}}
{{- $TypeName := .Name | Identify -}}
func (p *{{$TypeName}}) FastRead(buf []byte) (int, error) {
	var err error
	var offset int
	var l int
	var fieldTypeId thrift.TType
	var fieldId int16
	{{- range .Fields}}
	{{- if IsRequired .}}
	var isset{{ResolveFieldName .}} bool = false
	{{- end}}
	{{- end}}
	_, l, err = bthrift.Binary.ReadStructBegin(buf)
	offset += l
	if err != nil {
		goto ReadStructBeginError
	}

	for {
		_, fieldTypeId, fieldId, l, err = bthrift.Binary.ReadFieldBegin(buf[offset:])
		offset += l
		if err != nil {
			goto ReadFieldBeginError
		}
		if fieldTypeId == thrift.STOP {
			break;
		}
		{{if gt (len .Fields) 0 -}}
		switch fieldId {
		{{- range .Fields}}
		case {{.ID}}:
			if fieldTypeId == thrift.{{.Type | GetTypeIDConstant }} {
				l, err = p.FastReadField{{ID .}}(buf[offset:])
				offset += l
				if err != nil {
					goto ReadFieldError
				}
				{{- if IsRequired .}}
				isset{{ResolveFieldName .}} = true
				{{- end}}
			} else {
				l, err = bthrift.Binary.Skip(buf[offset:], fieldTypeId)
				offset += l
				if err != nil {
					goto SkipFieldError
				}
			}
		{{- end}}{{/* range .Fields */}}
		default:
			l, err = bthrift.Binary.Skip(buf[offset:], fieldTypeId)
			offset += l
		    if err != nil {
				goto SkipFieldError
		    }
		}
		{{- else -}}
		l, err = bthrift.Binary.Skip(buf[offset:], fieldTypeId)
		offset += l
		if err != nil {
			goto SkipFieldTypeError
		}
		{{- end}}{{/* if len(.Fields) > 0 */}}

		l, err = bthrift.Binary.ReadFieldEnd(buf[offset:])
		offset += l
		if err != nil {
		  goto ReadFieldEndError
		}
	}
	l, err = bthrift.Binary.ReadStructEnd(buf[offset:])
	offset += l
	if err != nil {
		goto ReadStructEndError
	}
	{{ $NeedRequiredFieldNotSetError := false }}
	{{- range .Fields}}
	{{- if IsRequired .}}
	{{ $NeedRequiredFieldNotSetError = true }}
	if !isset{{ResolveFieldName .}} {
		fieldId = {{.ID}}
		goto RequiredFieldNotSetError
	}
	{{- end}}
	{{- end}}
	return offset, nil
ReadStructBeginError:
	return offset, thrift.PrependError(fmt.Sprintf("%T read struct begin error: ", p), err)
ReadFieldBeginError:
	return offset, thrift.PrependError(fmt.Sprintf("%T read field %d begin error: ", p, fieldId), err)
{{if gt (len .Fields) 0 -}}
ReadFieldError:
	return offset, thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_{{$TypeName}}[fieldId]), err)
SkipFieldError:
	return offset, thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)
{{ else }}
SkipFieldTypeError:
	return offset, thrift.PrependError(fmt.Sprintf("%T skip field type %d error", p, fieldTypeId), err)
{{ end -}}
ReadFieldEndError:
	return offset, thrift.PrependError(fmt.Sprintf("%T read field end error", p), err)
ReadStructEndError:
	return offset, thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
{{- if $NeedRequiredFieldNotSetError }}
RequiredFieldNotSetError:
	return offset, thrift.NewTProtocolExceptionWithType(thrift.INVALID_DATA, fmt.Errorf("Required field %s is not set", fieldIDToName_{{$TypeName}}[fieldId]))
{{- end}}{{/* if $NeedRequiredFieldNotSetError */}}
}
{{- end}}{{/* define "StructLikeFastRead" */}}
`

const structLikeFastReadField = `
{{define "StructLikeFastReadField"}}
{{- $TypeName := .Name | Identify -}}
{{- range .Fields}}
{{$FieldName := ResolveFieldName .}}
func (p *{{$TypeName}}) FastReadField{{ID .}}(buf []byte) (int, error) {
	offset := 0
	{{- ResetIDGenerator}}
	{{- $ctx := MkRWCtx . "" false false}}
	{{ template "FieldFastRead" $ctx}}
	return offset, nil
}
{{- end}}{{/* range .Fields */}}
{{- end}}{{/* define "StructLikeFastReadField" */}}
`

const structLikeFastWrite = `
{{define "StructLikeFastWrite"}}
{{- $TypeName := .Name | Identify -}}
// for compatibility
func (p *{{$TypeName}}) FastWrite(buf []byte) int {
	return 0
}
{{- end}}{{/* define "StructLikeFastWrite" */}}
`

const structLikeFastWriteNocopy = `
{{define "StructLikeFastWriteNocopy"}}
{{- $TypeName := .Name | Identify -}}
func (p *{{$TypeName}}) FastWriteNocopy(buf []byte, binaryWriter bthrift.BinaryWriter) int {
	offset := 0
	{{- if eq .Category "union"}}
	var c int
	if c = p.CountSetFields{{$TypeName}}(); c != 1 {
		goto CountSetFieldsError
	}
	{{- end}}
	offset += bthrift.Binary.WriteStructBegin(buf[offset:], "{{GetStructName .}}")
	if p != nil {
		{{- $reorderedFields := ReorderStructFields .Fields}}
		{{- range $reorderedFields}}
		offset += p.fastWriteField{{ID .}}(buf[offset:], binaryWriter)
		{{- end}}
	}
	offset += bthrift.Binary.WriteFieldStop(buf[offset:])
	offset += bthrift.Binary.WriteStructEnd(buf[offset:])
	return offset
{{- if eq .Category "union"}}
CountSetFieldsError:
	panic(fmt.Errorf("%T write union: exactly one field must be set (%d set).", p, c))
{{- end}}
}
{{- end}}{{/* define "StructLikeFastWriteNocopy" */}}
`

const structLikeLength = `
{{define "StructLikeLength"}}
{{- $TypeName := .Name | Identify -}}
func (p *{{$TypeName}}) BLength() int {
	l := 0
	{{- if eq .Category "union"}}
	var c int
	if c = p.CountSetFields{{$TypeName}}(); c != 1 {
		goto CountSetFieldsError
	}
	{{- end}}
	l += bthrift.Binary.StructBeginLength("{{GetStructName .}}")
	if p != nil {
		{{- range .Fields}}
		l += p.field{{ID .}}Length()
		{{- end}}
	}
	l += bthrift.Binary.FieldStopLength()
	l += bthrift.Binary.StructEndLength()
	return l
{{- if eq .Category "union"}}
CountSetFieldsError:
	panic(fmt.Errorf("%T write union: exactly one field must be set (%d set).", p, c))
{{- end}}
}
{{- end}}{{/* define "StructLikeLength" */}}
`

const structLikeFastWriteField = `
{{define "StructLikeFastWriteField"}}
{{- $TypeName := .Name | Identify -}}
{{- range .Fields}}
{{- $FieldName := ResolveFieldName .}}
{{- $TypeID := .Type | GetTypeIDConstant }}
func (p *{{$TypeName}}) fastWriteField{{ID .}}(buf []byte, binaryWriter bthrift.BinaryWriter) int {
	offset := 0
	{{- ResetIDGenerator}}
	{{- if IsOptional .}}
	if p.IsSet{{$FieldName}}() {
	{{- end}}
	offset += bthrift.Binary.WriteFieldBegin(buf[offset:], "{{.Name}}", thrift.{{$TypeID}}, {{.ID}})
	{{- $ctx := MkRWCtx . "" false false}}
	{{- template "FieldFastWrite" $ctx}}
	offset += bthrift.Binary.WriteFieldEnd(buf[offset:])
	{{- if IsOptional .}}
	}
	{{- end}}
	return offset
}
{{end}}{{/* range .Fields */}}
{{- end}}{{/* define "StructLikeFastWriteField" */}}
`

const structLikeFieldLength = `
{{define "StructLikeFieldLength"}}
{{- $TypeName := .Name | Identify -}}
{{- range .Fields}}
{{- $FieldName := ResolveFieldName .}}
{{- $TypeID := .Type | GetTypeIDConstant }}
func (p *{{$TypeName}}) field{{ID .}}Length() int {
	l := 0
	{{- ResetIDGenerator}}
	{{- if IsOptional .}}
	if p.IsSet{{$FieldName}}() {
	{{- end}}
	l += bthrift.Binary.FieldBeginLength("{{.Name}}", thrift.{{$TypeID}}, {{.ID}})
	{{- $ctx := MkRWCtx . "" false false}}
	{{- template "FieldLength" $ctx}}
	l += bthrift.Binary.FieldEndLength()
	{{- if IsOptional .}}
	}
	{{- end}}
	return l
}
{{end}}{{/* range .Fields */}}
{{- end}}{{/* define "StructLikeFieldLength" */}}
`

const fieldFastRead = `
{{define "FieldFastRead"}}
	{{- if IsStructLike .Type}}
		{{- template "FieldFastReadStructLike" .}}
	{{- else if IsBaseType .Type }}
		{{- template "FieldFastReadBaseType" .}}
	{{- else}}{{/* IsContainerType */}}
		{{- template "FieldFastReadContainer" .}}
	{{- end}}
{{- end}}{{/* define "FieldFastRead" */}}
`

const fieldFastReadStructLike = `
{{define "FieldFastReadStructLike"}}
	{{- .Target}} {{if .NeedDecl}}:{{end}}= {{.TypeName | Deref | GetNewFunc}}()
	if l, err := {{.Target}}.FastRead(buf[offset:]); err != nil {
		return offset, err
	} else {
		offset += l
	}
{{- end}}{{/* define "FieldFastReadStructLike" */}} 
`

const fieldFastReadBaseType = `
{{define "FieldFastReadBaseType"}}
	{{- $DiffType := or (IsEnumType .Type) (IsBinaryType .Type)}}
	{{- if .NeedDecl}}
	var {{.Target}} {{.TypeName}}
	{{- end}}
	if v, l, err := bthrift.Binary.Read{{.TypeID}}(buf[offset:]); err != nil {
		return offset, err
	} else {
		offset += l
	{{ if .IsPointer}}
		{{- if $DiffType}}
		tmp := {{.TypeName | Deref}}(v)
		{{.Target}} = &tmp
		{{- else -}}
		{{.Target}} = &v
		{{- end}}
	{{ else}}
		{{- if $DiffType}}
		{{.Target}} = {{.TypeName}}(v)
		{{- else}}
		{{.Target}} = v
		{{- end}}
	{{ end}}
	}
{{- end}}{{/* define "FieldFastReadBaseType" */}}
`

const fieldFastReadContainer = `
{{define "FieldFastReadContainer"}}
	{{- if eq "Map" .TypeID}}
	     {{- template "FieldFastReadMap" .}}
	{{- else if eq "List" .TypeID}}
	     {{- template "FieldFastReadList" .}}
	{{- else}}
	     {{- template "FieldFastReadSet" .}}
	{{- end}}
{{- end}}{{/* define "FieldFastReadContainer" */}}
`

const fieldFastReadMap = `
{{define "FieldFastReadMap"}}
	_, _, size, l, err := bthrift.Binary.ReadMapBegin(buf[offset:])
	offset += l
	if err != nil {
		return offset, err
	}
	{{.Target}} {{if .NeedDecl}}:{{end}}= make({{.TypeName}}, size)
	for i := 0; i < size; i++ {
		{{- $key := GenID "_key"}}
		{{- $ctx := MkRWCtx (.Type | GetKeyType) $key true true}}
		{{- template "FieldFastRead" $ctx}}
		{{/* line break */}}
		{{- $val := GenID "_val"}}
		{{- $ctx = MkRWCtx (.Type | GetValType) $val true false}}
		{{- template "FieldFastRead" $ctx}}

		{{if and (IsStructLike (.Type | GetValType)) Features.ValueTypeForSIC}}
			{{$val = printf "*%s" $val}}
		{{end}}

		{{.Target}}[{{$key}}] = {{$val}}
	}
	if l, err := bthrift.Binary.ReadMapEnd(buf[offset:]); err != nil {
		return offset, err
	} else {
		offset += l
	}
{{- end}}{{/* define "FieldFastReadMap" */}}
`

const fieldFastReadSet = `
{{define "FieldFastReadSet"}}
	_, size, l, err := bthrift.Binary.ReadSetBegin(buf[offset:])
	offset += l
	if err != nil {
		return offset, err
	}
	{{.Target}} {{if .NeedDecl}}:{{end}}= make({{.TypeName}}, 0, size)
	for i := 0; i < size; i++ {
		{{- $val := GenID "_elem"}}
		{{- $ctx := MkRWCtx (.Type | GetValType) $val true false}}
		{{- template "FieldFastRead" $ctx}}

		{{if and (IsStructLike (.Type | GetValType)) Features.ValueTypeForSIC}}
			{{$val = printf "*%s" $val}}
		{{end}}

		{{.Target}} = append({{.Target}}, {{$val}})
	}
	if l, err := bthrift.Binary.ReadSetEnd(buf[offset:]); err != nil {
		return offset, err
	} else {
		offset += l
	}
{{- end}}{{/* define "FieldFastReadSet" */}}
`

const fieldFastReadList = `
{{define "FieldFastReadList"}}
	_, size, l, err := bthrift.Binary.ReadListBegin(buf[offset:])
	offset += l
	if err != nil {
		return offset, err
	}
	{{.Target}} {{if .NeedDecl}}:{{end}}= make({{.TypeName}}, 0, size)
	for i := 0; i < size; i++ {
		{{- $val := GenID "_elem"}}
		{{- $ctx := MkRWCtx (.Type | GetValType) $val true false}}
		{{- template "FieldFastRead" $ctx}}

		{{if and (IsStructLike (.Type | GetValType)) Features.ValueTypeForSIC}}
			{{$val = printf "*%s" $val}}
		{{end}}

		{{.Target}} = append({{.Target}}, {{$val}})
	}
	if l, err := bthrift.Binary.ReadListEnd(buf[offset:]); err != nil {
		return offset, err
	} else {
		offset += l
	}
{{- end}}{{/* define "FieldFastReadList" */}}
`

const fieldFastWrite = `
{{define "FieldFastWrite"}}
	{{- if IsStructLike .Type}}
		{{- template "FieldFastWriteStructLike" . -}}
	{{- else if IsBaseType .Type }}
		{{- template "FieldFastWriteBaseType" . -}}
	{{- else}}{{/* IsContainerType */}}
		{{- template "FieldFastWriteContainer" . -}}
	{{- end}}
{{- end}}{{/* define "FieldFastWrite" */}}
`

const fieldLength = `
{{define "FieldLength"}}
	{{- if IsStructLike .Type}}
		{{- template "FieldStructLikeLength" . -}}
	{{- else if IsBaseType .Type }}
		{{- template "FieldBaseTypeLength" . -}}
	{{- else}}{{/* IsContainerType */}}
		{{- template "FieldContainerLength" . -}}
	{{- end}}
{{- end}}{{/* define "FieldLength" */}}
`

const fieldFastWriteStructLike = `
{{define "FieldFastWriteStructLike"}}
	offset += {{.Target}}.FastWriteNocopy(buf[offset:], binaryWriter)
{{- end}}{{/* define "FieldFastWriteStructLike" */}}
`

const fieldStructLikeLength = `
{{define "FieldStructLikeLength"}}
	l += {{.Target}}.BLength()
{{- end}}{{/* define "FieldStructLikeLength" */}}
`

const fieldFastWriteBaseType = `
{{define "FieldFastWriteBaseType"}}
{{- $Value := .Target}}
{{- if .IsPointer}}{{$Value = printf "*%s" $Value}}{{end}}
{{- if IsEnumType .Type}}{{$Value = printf "int32(%s)" $Value}}{{end}}
{{- if IsBinaryType .Type}}{{$Value = printf "[]byte(%s)" $Value}}{{end}}
{{- if IsBinaryOrStringType .Type}}
	offset += bthrift.Binary.Write{{.TypeID}}Nocopy(buf[offset:], binaryWriter, {{$Value}})
{{else}}
	offset += bthrift.Binary.Write{{.TypeID}}(buf[offset:], {{$Value}})
{{end}}
{{- end}}{{/* define "FieldFastWriteBaseType" */}}
`

const fieldBaseTypeLength = `
{{define "FieldBaseTypeLength"}}
{{- $Value := .Target}}
{{- if .IsPointer}}{{$Value = printf "*%s" $Value}}{{end}}
{{- if IsEnumType .Type}}{{$Value = printf "int32(%s)" $Value}}{{end}}
{{- if IsBinaryType .Type}}{{$Value = printf "[]byte(%s)" $Value}}{{end}}
{{- if IsBinaryOrStringType .Type}}
	l += bthrift.Binary.{{.TypeID}}LengthNocopy({{$Value}})
{{else}}
	l += bthrift.Binary.{{.TypeID}}Length({{$Value}})
{{end}}
{{- end}}{{/* define "FieldBaseTypeLength" */}}
`

const fieldFixedLengthTypeLength = `
{{define "FieldFixedLengthTypeLength"}}
{{- $Value := .Target -}}
bthrift.Binary.{{.TypeID}}Length({{TypeIDToGoType .TypeID}}({{$Value}}))
{{- end -}}{{/* define "FieldFixedLengthTypeLength" */}}
`

const fieldFastWriteContainer = `
{{define "FieldFastWriteContainer"}}
	{{- if eq "Map" .TypeID}}
		{{- template "FieldFastWriteMap" .}}
	{{- else if eq "List" .TypeID}}
		{{- template "FieldFastWriteList" .}}
	{{- else}}
		{{- template "FieldFastWriteSet" .}}
	{{- end}}
{{- end}}{{/* define "FieldFastWriteContainer" */}}
`

const fieldContainerLength = `
{{define "FieldContainerLength"}}
	{{- if eq "Map" .TypeID}}
		{{- template "FieldMapLength" .}}
	{{- else if eq "List" .TypeID}}
		{{- template "FieldListLength" .}}
	{{- else}}
		{{- template "FieldSetLength" .}}
	{{- end}}
{{- end}}{{/* define "FieldContainerLength" */}}
`

const fieldFastWriteMap = `
{{define "FieldFastWriteMap"}}
	offset += bthrift.Binary.WriteMapBegin(buf[offset:], thrift.
		{{- .Type| GetKeyType | GetTypeIDConstant -}}
		, thrift.{{- .Type | GetValType | GetTypeIDConstant -}}
		, len({{.Target}}))
	for k, v := range {{.Target}}{
		{{$ctx := MkRWCtx (.Type | GetKeyType) "k" false true}}
		{{- template "FieldFastWrite" $ctx}}
		{{$ctx := MkRWCtx (.Type | GetValType) "v" false false}}
		{{- template "FieldFastWrite" $ctx}}
	}
	offset += bthrift.Binary.WriteMapEnd(buf[offset:])
{{- end}}{{/* define "FieldFastWriteMap" */}}
`

const fieldMapLength = `
{{define "FieldMapLength"}}
	l += bthrift.Binary.MapBeginLength(thrift.
		{{- .Type| GetKeyType | GetTypeIDConstant -}}
		, thrift.{{- .Type | GetValType | GetTypeIDConstant -}}
		, len({{.Target}}))
	{{- if and (IsFixedLengthType (.Type | GetKeyType)) (IsFixedLengthType (.Type | GetValType))}}
	{{- $ctx := MkRWCtx (.Type | GetKeyType) "" false true }}
	var tmpK {{$ctx.TypeName}}
	{{- $ctx := MkRWCtx (.Type | GetValType) "" false false }}
	var tmpV {{$ctx.TypeName}}
	l += ({{- $ctx := MkRWCtx (.Type | GetKeyType) "tmpK" false true -}}
		{{- template "FieldFixedLengthTypeLength" $ctx}} +
		{{- $ctx := MkRWCtx (.Type | GetValType) "tmpV" false false -}}
		{{- template "FieldFixedLengthTypeLength" $ctx}}) * len({{.Target}})
	{{- else}}
	for k, v := range {{.Target}}{
		{{$ctx := MkRWCtx (.Type | GetKeyType) "k" false true}}
		{{- template "FieldLength" $ctx}}
		{{$ctx := MkRWCtx (.Type | GetValType) "v" false false}}
		{{- template "FieldLength" $ctx}}
	}
	{{- end}}{{/* if */}}
	l += bthrift.Binary.MapEndLength()
{{- end}}{{/* define "FieldMapLength" */}}
`

const fieldFastWriteSet = `
{{define "FieldFastWriteSet"}}
		offset += bthrift.Binary.WriteSetBegin(buf[offset:], thrift.
		{{- .Type | GetValType | GetTypeIDConstant -}}
		, len({{.Target}}))
		{{template "ValidateSet" .}}
		for _, v := range {{.Target}} {
			{{- $ctx := MkRWCtx (.Type | GetValType) "v" false false}}
			{{- template "FieldFastWrite" $ctx}}
		}
		offset += bthrift.Binary.WriteSetEnd(buf[offset:])
{{- end}}{{/* define "FieldFastWriteSet" */}}
`

const fieldSetLength = `
{{define "FieldSetLength"}}
		l += bthrift.Binary.SetBeginLength(thrift.
		{{- .Type | GetValType | GetTypeIDConstant -}}
		, len({{.Target}}))
		{{template "ValidateSet" .}}
		{{- if IsFixedLengthType (.Type | GetValType)}}
		{{- $ctx := MkRWCtx (.Type | GetValType) "" false false }}
		var tmpV {{$ctx.TypeName}}
		l += {{- $ctx := MkRWCtx (.Type | GetValType) "tmpV" false false -}}
			{{- template "FieldFixedLengthTypeLength" $ctx -}} * len({{.Target}})
		{{- else}}
		for _, v := range {{.Target}} {
			{{- $ctx := MkRWCtx (.Type | GetValType) "v" false false}}
			{{- template "FieldLength" $ctx}}
		}
		{{- end}}{{/* if */}}
		l += bthrift.Binary.SetEndLength()
{{- end}}{{/* define "FieldSetLength" */}}
`

const fieldFastWriteList = `
{{define "FieldFastWriteList"}}
		offset += bthrift.Binary.WriteListBegin(buf[offset:], thrift.
		{{- .Type | GetValType | GetTypeIDConstant -}}
		, len({{.Target}}))
		for _, v := range {{.Target}} {
			{{- $ctx := MkRWCtx (.Type | GetValType) "v" false false}}
			{{- template "FieldFastWrite" $ctx}}
		}
		offset += bthrift.Binary.WriteListEnd(buf[offset:])
{{- end}}{{/* define "FieldFastWriteList" */}}
`

const fieldListLength = `
{{define "FieldListLength"}}
		l += bthrift.Binary.ListBeginLength(thrift.
		{{- .Type | GetValType | GetTypeIDConstant -}}
		, len({{.Target}}))
		{{- if IsFixedLengthType (.Type | GetValType)}}
		{{- $ctx := MkRWCtx (.Type | GetValType) "" false false }}
		var tmpV {{$ctx.TypeName}}
		l += {{- $ctx := MkRWCtx (.Type | GetValType) "tmpV" false false -}}
			{{- template "FieldFixedLengthTypeLength" $ctx -}} * len({{.Target}})
		{{- else}}
		for _, v := range {{.Target}} {
			{{- $ctx := MkRWCtx (.Type | GetValType) "v" false false}}
			{{- template "FieldLength" $ctx}}
		}
		{{- end}}{{/* if */}}
		l += bthrift.Binary.ListEndLength()
{{- end}}{{/* define "FieldListLength" */}}
`

const processor = `
{{define "Processor"}}
{{- range .Functions}}
{{$ArgsType := BuildArgsType $.Name .}}
{{template "StructLike" $ArgsType}}
{{- if not .Oneway}}
	{{$ResType := BuildResType $.Name .}}
	{{template "StructLike" $ResType}}
{{- end}}
{{- end}}{{/* range .Functions */}}
{{- end}}{{/* define "Processor" */}}
`

const validateSet = `
{{define "ValidateSet"}}
{{- if Features.ValidateSet}}
{{- $ctx := MkRWCtx2 (.Type | GetValType) "tgt" "src" false false}}
for i := 0; i < len({{.Target}}); i++ {
	for j := i + 1; j < len({{.Target}}); j++ {
{{- if Features.GenDeepEqual}}
		if func(tgt, src {{$ctx.TypeName}}) bool {
			{{- template "FieldDeepEqual" $ctx}}
			return true
		}({{.Target}}[i], {{.Target}}[j]) {
{{- else}}
		if reflect.DeepEqual({{.Target}}[i], {{.Target}}[j]) {
{{- end}}
			panic(fmt.Errorf("%T error writing set field: slice is not unique", {{.Target}}[i]))
		}
	}
}
{{- end}}
{{- end}}{{/* define "ValidateSet" */}}`
