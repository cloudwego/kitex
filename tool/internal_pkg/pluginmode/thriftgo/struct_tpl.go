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

const structLikeCodec = `
{{define "StructLikeCodec"}}
{{if GenerateFastAPIs}}
{{template "StructLikeFastRead" .}}

{{template "StructLikeFastReadField" .}}

{{template "StructLikeFastWrite" .}}

{{template "StructLikeFastWriteNocopy" .}}

{{template "StructLikeLength" .}}

{{template "StructLikeFastWriteField" .}}

{{template "StructLikeFieldLength" .}}
{{- end}}{{/* if GenerateFastAPIs */}}

{{if GenerateDeepCopyAPIs}}
{{template "StructLikeDeepCopy" .}}
{{- end}}{{/* if GenerateDeepCopyAPIs */}}
{{- end}}{{/* define "StructLikeCodec" */}}
`

const structLikeFastRead = `
{{define "StructLikeFastRead"}}
{{- $TypeName := .GoName}}
func (p *{{$TypeName}}) FastRead(buf []byte) (int, error) {
	var err error
	var offset int
	var l int
	var fieldTypeId thrift.TType
	var fieldId int16
	{{- range .Fields}}
	{{- if .Requiredness.IsRequired}}
	var isset{{.GoName}} bool = false
	{{- end}}
	{{- end}}
	_, l, err = bthrift.Binary.ReadStructBegin(buf)
	offset += l
	if err != nil {
		goto ReadStructBeginError
	}

	for {
		{{- if Features.KeepUnknownFields}}
		{{- if gt (len .Fields) 0}}
		var isUnknownField bool
		{{- end}}
		var beginOff int = offset
		{{- end}}
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
		{{- $isBaseVal := .Type | IsBaseType}}
		case {{.ID}}:
			if fieldTypeId == thrift.{{.Type | GetTypeIDConstant }} {
				{{- if Features.WithFieldMask}}
				if {{if $isBaseVal}}_{{else}}nfm{{end}}, ex := p._fieldmask.Field(fieldId); ex {
				{{- end}}
				l, err = p.FastReadField{{Str .ID}}(buf[offset:])
				offset += l
				if err != nil {
					goto ReadFieldError
				}
				{{- if .Requiredness.IsRequired}}
				isset{{.GoName}} = true
				{{- end}}
				break
				{{- if Features.WithFieldMask}}
				}
				{{- end}}
			}
			l, err = bthrift.Binary.Skip(buf[offset:], fieldTypeId)
			offset += l
			if err != nil {
				goto SkipFieldError
			}
		{{- end}}{{/* range .Fields */}}
		default:
			l, err = bthrift.Binary.Skip(buf[offset:], fieldTypeId)
			offset += l
			if err != nil {
				goto SkipFieldError
			}
			{{- if Features.KeepUnknownFields}}
			isUnknownField = true
			{{- end}}{{/* if Features.KeepUnknownFields */}}
		}
		{{- else -}}
		l, err = bthrift.Binary.Skip(buf[offset:], fieldTypeId)
		offset += l
		if err != nil {
			goto SkipFieldError
		}
		{{- end}}{{/* if len(.Fields) > 0 */}}

		l, err = bthrift.Binary.ReadFieldEnd(buf[offset:])
		offset += l
		if err != nil {
		  goto ReadFieldEndError
		}

		{{- if Features.KeepUnknownFields}}
		{{if gt (len .Fields) 0 -}}
		if isUnknownField {
			p._unknownFields = append(p._unknownFields, buf[beginOff:offset]...)
		}
		{{- else -}}
		p._unknownFields = append(p._unknownFields, buf[beginOff:offset]...)
		{{- end}}
		{{- end}}{{/* if Features.KeepUnknownFields */}}
	}
	l, err = bthrift.Binary.ReadStructEnd(buf[offset:])
	offset += l
	if err != nil {
		goto ReadStructEndError
	}
	{{ $NeedRequiredFieldNotSetError := false }}
	{{- range .Fields}}
	{{- if .Requiredness.IsRequired}}
	{{ $NeedRequiredFieldNotSetError = true }}
	if !isset{{.GoName}} {
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
{{- if gt (len .Fields) 0}}
ReadFieldError:
	return offset, thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_{{$TypeName}}[fieldId]), err)
{{- end}}
SkipFieldError:
	return offset, thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)
ReadFieldEndError:
	return offset, thrift.PrependError(fmt.Sprintf("%T read field end error", p), err)
ReadStructEndError:
	return offset, thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
{{- if $NeedRequiredFieldNotSetError }}
RequiredFieldNotSetError:
	return offset, thrift.NewTProtocolExceptionWithType(thrift.INVALID_DATA, fmt.Errorf("required field %s is not set", fieldIDToName_{{$TypeName}}[fieldId]))
{{- end}}{{/* if $NeedRequiredFieldNotSetError */}}
}
{{- end}}{{/* define "StructLikeFastRead" */}}
`

const structLikeFastReadField = `
{{define "StructLikeFastReadField"}}
{{- $TypeName := .GoName}}
{{- range .Fields}}
{{$FieldName := .GoName}}
{{- $isBaseVal := .Type | IsBaseType -}}
func (p *{{$TypeName}}) FastReadField{{Str .ID}}(buf []byte{{if and Features.WithFieldMask (not $isBaseVal)}}, fm *fieldmask.FieldMask{{end}}) (int, error) {
	offset := 0
	{{- $ctx := (MkRWCtx .).WithFieldMask "fm" -}}
	{{ template "FieldFastRead" $ctx}}
	return offset, nil
}
{{- end}}{{/* range .Fields */}}
{{- end}}{{/* define "StructLikeFastReadField" */}}
`

// TODO: check required
const structLikeDeepCopy = `
{{define "StructLikeDeepCopy"}}
{{- $TypeName := .GoName}}
func (p *{{$TypeName}}) DeepCopy(s interface{}) error {
	{{if gt (len .Fields) 0 -}}
	src, ok := s.(*{{$TypeName}})
	if !ok {
		return fmt.Errorf("%T's type not matched %T", s, p)
	}
	{{- end -}}
	{{- range .Fields}}
	{{- $ctx := MkRWCtx .}}
	{{ template "FieldDeepCopy" $ctx}}
	{{- end}}{{/* range .Fields */}}
	{{/* line break */}}
	return nil
}
{{- end}}{{/* define "StructLikeDeepCopy" */}}
`

const structLikeFastWrite = `
{{define "StructLikeFastWrite"}}
{{- $TypeName := .GoName}}
// for compatibility
func (p *{{$TypeName}}) FastWrite(buf []byte) int {
	return 0
}
{{- end}}{{/* define "StructLikeFastWrite" */}}
`

const structLikeFastWriteNocopy = `
{{define "StructLikeFastWriteNocopy"}}
{{- $TypeName := .GoName}}
func (p *{{$TypeName}}) FastWriteNocopy(buf []byte, binaryWriter bthrift.BinaryWriter) int {
	offset := 0
	{{- if eq .Category "union"}}
	var c int
	if p != nil {
		if c = p.CountSetFields{{$TypeName}}(); c != 1 {
			goto CountSetFieldsError
		}
	}
	{{- end}}
	offset += bthrift.Binary.WriteStructBegin(buf[offset:], "{{.Name}}")
	if p != nil {
		{{- $reorderedFields := ReorderStructFields .Fields}}
		{{- range $reorderedFields}}
		{{- $isBaseVal := .Type | IsBaseType}}
		{{- if Features.WithFieldMask}}
		if {{if $isBaseVal}}_{{else}}nfm{{end}}, ex := p._fieldmask.Field({{.ID}}); ex { 
		{{- end}}
		offset += p.fastWriteField{{Str .ID}}(buf[offset:], binaryWriter{{if and Features.WithFieldMask (not $isBaseVal)}}, nfm{{end}})
		{{- if Features.WithFieldMask}}
		}
		{{- end}}
		{{- end}}
		{{- if Features.KeepUnknownFields}}
		offset += copy(buf[offset:], p._unknownFields)
		{{- end}}{{/* if Features.KeepUnknownFields */}}
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
{{- $TypeName := .GoName}}
func (p *{{$TypeName}}) BLength() int {
	l := 0
	{{- if eq .Category "union"}}
	var c int
	if p != nil {
		if c = p.CountSetFields{{$TypeName}}(); c != 1 {
			goto CountSetFieldsError
		}
	}
	{{- end}}
	l += bthrift.Binary.StructBeginLength("{{.Name}}")
	if p != nil {
		{{- range .Fields}}
		l += p.field{{Str .ID}}Length()
		{{- end}}
		{{- if Features.KeepUnknownFields}}
		l += len(p._unknownFields)
		{{- end}}{{/* if Features.KeepUnknownFields */}}
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
{{- $TypeName := .GoName}}
{{- range .Fields}}
{{- $FieldName := .GoName}}
{{- $TypeID := .Type | GetTypeIDConstant }}
{{- $isBaseVal := .Type | IsBaseType -}}
func (p *{{$TypeName}}) fastWriteField{{Str .ID}}(buf []byte, binaryWriter bthrift.BinaryWriter{{if and Features.WithFieldMask (not $isBaseVal)}}, fm *fieldmask.FieldMask{{end}}) int {
	offset := 0
	{{- if .Requiredness.IsOptional}}
	if p.{{.IsSetter}}() {
	{{- end}}
	offset += bthrift.Binary.WriteFieldBegin(buf[offset:], "{{.Name}}", thrift.{{$TypeID}}, {{.ID}})
	{{- $ctx := (MkRWCtx .).WithFieldMask "fm"}}
	{{- template "FieldFastWrite" $ctx}}
	offset += bthrift.Binary.WriteFieldEnd(buf[offset:])
	{{- if .Requiredness.IsOptional}}
	}
	{{- end}}
	return offset
}
{{end}}{{/* range .Fields */}}
{{- end}}{{/* define "StructLikeFastWriteField" */}}
`

const structLikeFieldLength = `
{{define "StructLikeFieldLength"}}
{{- $TypeName := .GoName}}
{{- range .Fields}}
{{- $FieldName := .GoName}}
{{- $TypeID := .Type | GetTypeIDConstant }}
func (p *{{$TypeName}}) field{{Str .ID}}Length() int {
	l := 0
	{{- if .Requiredness.IsOptional}}
	if p.{{.IsSetter}}() {
	{{- end}}
	l += bthrift.Binary.FieldBeginLength("{{.Name}}", thrift.{{$TypeID}}, {{.ID}})
	{{- $ctx := MkRWCtx .}}
	{{- template "FieldLength" $ctx}}
	l += bthrift.Binary.FieldEndLength()
	{{- if .Requiredness.IsOptional}}
	}
	{{- end}}
	return l
}
{{end}}{{/* range .Fields */}}
{{- end}}{{/* define "StructLikeFieldLength" */}}
`

const fieldFastRead = `
{{define "FieldFastRead"}}
	{{- if .Type.Category.IsStructLike}}
		{{- template "FieldFastReadStructLike" .}}
	{{- else if .Type.Category.IsContainerType}}
		{{- template "FieldFastReadContainer" .}}
	{{- else}}{{/* IsBaseType */}}
		{{- template "FieldFastReadBaseType" .}}
	{{- end}}
{{- end}}{{/* define "FieldFastRead" */}}
`

const fieldFastReadStructLike = `
{{define "FieldFastReadStructLike"}}
	{{- if .NeedDecl}}
	{{- .Target}} := {{.TypeName.Deref.NewFunc}}()
	{{- else}}
	tmp := {{.TypeName.Deref.NewFunc}}()
	{{- end}}
	{{- if Features.WithFieldMask}}
	{{.Target}}.SetFieldMask({{.FieldMask}})
	{{- end}}
	if l, err := {{- if .NeedDecl}}{{.Target}}{{else}}tmp{{end}}.FastRead(buf[offset:]); err != nil {
		return offset, err
	} else {
		offset += l
	}
	{{if not .NeedDecl}}{{- .Target}} = tmp{{end}}
{{- end}}{{/* define "FieldFastReadStructLike" */}} 
`

const fieldFastReadBaseType = `
{{define "FieldFastReadBaseType"}}
	{{- $DiffType := or .Type.Category.IsEnum .Type.Category.IsBinary}}
	{{- if .NeedDecl}}
	var {{.Target}} {{.TypeName}}
	{{- end}}
	if v, l, err := bthrift.Binary.Read{{.TypeID}}(buf[offset:]); err != nil {
		return offset, err
	} else {
		offset += l
	{{ if .IsPointer}}
		{{- if $DiffType}}
		tmp := {{.TypeName.Deref}}(v)
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
{{- $isIntKey := .KeyCtx.Type | IsIntType -}}
{{- $isStrKey := .KeyCtx.Type | IsStrType -}}
{{- $isBaseVal := .ValCtx.Type | IsBaseType -}}
{{- $curFieldMask := .FieldMask -}}
	_, _, size, l, err := bthrift.Binary.ReadMapBegin(buf[offset:])
	offset += l
	if err != nil {
		return offset, err
	}
	{{.Target}} {{if .NeedDecl}}:{{end}}= make({{.TypeName}}, size)
	for i := 0; i < size; i++ {
		{{- $key := .GenID "_key"}}
		{{- $ctx := .KeyCtx.WithDecl.WithTarget $key}}
		{{- template "FieldFastRead" $ctx}}
		{{- if Features.WithFieldMask}}
		{{- if $isIntKey}}
		{{- $curFieldMask = "nfm"}}
		if {{if $isBaseVal}}_{{else}}{{$curFieldMask}}{{end}}, ex := {{.FieldMask}}.Int(int({{$key}})); !ex {
			if err := iprot.Skip(thrift.{{.ValCtx.Type | GetTypeIDConstant}}); err != nil {
				return err
			}
			continue
		} else {
		{{- else if $isStrKey}}
		{{- $curFieldMask = "nfm"}}
		if {{if $isBaseVal}}_{{else}}{{$curFieldMask}}{{end}}, ex := {{.FieldMask}}.Str(string({{$key}})); !ex {
			if err := iprot.Skip(thrift.{{.ValCtx.Type | GetTypeIDConstant}}); err != nil {
				return err
			}
			continue
		} else {
		{{- else}}
		{{$curFieldMask}} = nil
		{{- end}}
		{{- end}}{{/* end WithFieldMask */}}
		{{/* line break */}}
		{{- $val := .GenID "_val"}}
		{{- $ctx := (.ValCtx.WithDecl.WithTarget $val).WithFieldMask $curFieldMask}}
		{{- template "FieldFastRead" $ctx}}

		{{if and .ValCtx.Type.Category.IsStructLike Features.ValueTypeForSIC}}
			{{$val = printf "*%s" $val}}
		{{end}}

		{{.Target}}[{{$key}}] = {{$val}}
		{{- if and Features.WithFieldMask (or $isIntKey $isStrKey)}}
		}
		{{- end}}
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
{{- $isBaseVal := .ValCtx.Type | IsBaseType -}}
{{- $curFieldMask := .FieldMask -}}
	_, size, l, err := bthrift.Binary.ReadSetBegin(buf[offset:])
	offset += l
	if err != nil {
		return offset, err
	}
	{{.Target}} {{if .NeedDecl}}:{{end}}= make({{.TypeName}}, 0, size)
	for i := 0; i < size; i++ {
		{{- $val := .GenID "_elem"}}
		{{- if Features.WithFieldMask}}
		{{- $curFieldMask = "nfm"}}
		if {{if $isBaseVal}}_{{else}}{{$curFieldMask}}{{end}}, ex := {{.FieldMask}}.Int(i); !ex {
			if err := iprot.Skip(thrift.{{.ValCtx.Type | GetTypeIDConstant}}); err != nil {
				return err
			}
			continue
		} else {
		{{- end}}
		{{- $ctx := (.ValCtx.WithDecl.WithTarget $val).WithFieldMask $curFieldMask}}
		{{- template "FieldFastRead" $ctx}}

		{{if and .ValCtx.Type.Category.IsStructLike Features.ValueTypeForSIC}}
			{{$val = printf "*%s" $val}}
		{{end}}

		{{.Target}} = append({{.Target}}, {{$val}})
		{{- if Features.WithFieldMask}}
		}
		{{- end}}
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
{{- $isBaseVal := .ValCtx.Type | IsBaseType -}}
{{- $curFieldMask := .FieldMask -}}
	_, size, l, err := bthrift.Binary.ReadListBegin(buf[offset:])
	offset += l
	if err != nil {
		return offset, err
	}
	{{.Target}} {{if .NeedDecl}}:{{end}}= make({{.TypeName}}, 0, size)
	for i := 0; i < size; i++ {
		{{- $val := .GenID "_elem"}}
		{{- if Features.WithFieldMask}}
		{{- $curFieldMask = "nfm"}}
		if {{if $isBaseVal}}_{{else}}{{$curFieldMask}}{{end}}, ex := {{.FieldMask}}.Int(i); !ex {
			if err := iprot.Skip(thrift.{{.ValCtx.Type | GetTypeIDConstant}}); err != nil {
				return err
			}
			continue
		} else {
		{{- end}}
		{{- $ctx := (.ValCtx.WithDecl.WithTarget $val).WithFieldMask $curFieldMask}}
		{{- template "FieldFastRead" $ctx}}

		{{if and .ValCtx.Type.Category.IsStructLike Features.ValueTypeForSIC}}
			{{$val = printf "*%s" $val}}
		{{end}}

		{{.Target}} = append({{.Target}}, {{$val}})
		{{- if Features.WithFieldMask}}
		}
		{{- end}}
	}
	if l, err := bthrift.Binary.ReadListEnd(buf[offset:]); err != nil {
		return offset, err
	} else {
		offset += l
	}
{{- end}}{{/* define "FieldFastReadList" */}}
`

const fieldDeepCopy = `
{{define "FieldDeepCopy"}}
	{{- if .Type.Category.IsStructLike}}
		{{- template "FieldDeepCopyStructLike" .}}
	{{- else if .Type.Category.IsContainerType}}
		{{- template "FieldDeepCopyContainer" .}}
	{{- else}}{{/* IsBaseType */}}
		{{- template "FieldDeepCopyBaseType" .}}
	{{- end}}
{{- end}}{{/* define "FieldDeepCopy" */}}
`

const fieldDeepCopyStructLike = `
{{define "FieldDeepCopyStructLike"}}
{{- $Src := SourceTarget .Target}}
	{{- if .NeedDecl}}
	var {{.Target}} *{{.TypeName.Deref}}
	{{- else}}
	var _{{FieldName .Target}} *{{.TypeName.Deref}}
	{{- end}}
	if {{$Src}} != nil {
		{{- if .NeedDecl}}{{.Target}}{{else}}_{{FieldName .Target}}{{end}} = &{{.TypeName.Deref}}{}
		if err := {{- if .NeedDecl}}{{.Target}}{{else}}_{{FieldName .Target}}{{end}}.DeepCopy({{$Src}}); err != nil {
			return err
		}
	}
	{{if not .NeedDecl}}{{- .Target}} = _{{FieldName .Target}}{{end}}
{{- end}}{{/* define "FieldDeepCopyStructLike" */}} 
`

const fieldDeepCopyContainer = `
{{define "FieldDeepCopyContainer"}}
	{{- if eq "Map" .TypeID}}
	     {{- template "FieldDeepCopyMap" .}}
	{{- else if eq "List" .TypeID}}
	     {{- template "FieldDeepCopyList" .}}
	{{- else}}
	     {{- template "FieldDeepCopySet" .}}
	{{- end}}
{{- end}}{{/* define "FieldDeepCopyContainer" */}}
`

const fieldDeepCopyMap = `
{{define "FieldDeepCopyMap"}}
{{- $Src := SourceTarget .Target}}
	{{- if .NeedDecl}}var {{.Target}} {{.TypeName}}{{- end}}
	if {{$Src}} != nil {
		{{.Target}} = make({{.TypeName}}, len({{$Src}}))
		{{- $key := .GenID "_key"}}
		{{- $val := .GenID "_val"}}
		for {{SourceTarget $key}}, {{SourceTarget $val}} := range {{$Src}} {
			{{- $ctx := .KeyCtx.WithDecl.WithTarget $key}}
			{{- template "FieldDeepCopy" $ctx}}
			{{/* line break */}}
			{{- $ctx := .ValCtx.WithDecl.WithTarget $val}}
			{{- template "FieldDeepCopy" $ctx}}

			{{- if and .ValCtx.Type.Category.IsStructLike Features.ValueTypeForSIC}}
				{{$val = printf "*%s" $val}}
			{{- end}}

			{{.Target}}[{{$key}}] = {{$val}}
		}
	}
{{- end}}{{/* define "FieldDeepCopyMap" */}}
`

const fieldDeepCopyList = `
{{define "FieldDeepCopyList"}}
{{- $Src := SourceTarget .Target}}
	{{if .NeedDecl}}var {{.Target}} {{.TypeName}}{{end}}
	if {{$Src}} != nil {
		{{.Target}} = make({{.TypeName}}, 0, len({{$Src}}))
		{{- $val := .GenID "_elem"}}
		for _, {{SourceTarget $val}} := range {{$Src}} {
			{{- $ctx := .ValCtx.WithDecl.WithTarget $val}}
			{{- template "FieldDeepCopy" $ctx}}

			{{- if and .ValCtx.Type.Category.IsStructLike Features.ValueTypeForSIC}}
				{{$val = printf "*%s" $val}}
			{{- end}}

			{{.Target}} = append({{.Target}}, {{$val}})
		}
	}
{{- end}}{{/* define "FieldDeepCopyList" */}}
`

const fieldDeepCopySet = `
{{define "FieldDeepCopySet"}}
{{- $Src := SourceTarget .Target}}
	{{if .NeedDecl}}var {{.Target}} {{.TypeName}}{{end}}
	if {{$Src}} != nil {
		{{.Target}} = make({{.TypeName}}, 0, len({{$Src}}))
		{{- $val := .GenID "_elem"}}
		for _, {{SourceTarget $val}} := range {{$Src}} {
			{{- $ctx := .ValCtx.WithDecl.WithTarget $val}}
			{{- template "FieldDeepCopy" $ctx}}

			{{- if and .ValCtx.Type.Category.IsStructLike Features.ValueTypeForSIC}}
				{{$val = printf "*%s" $val}}
			{{- end}}

			{{.Target}} = append({{.Target}}, {{$val}})
		}
	}
{{- end}}{{/* define "FieldDeepCopySet" */}}
`

const fieldDeepCopyBaseType = `
{{define "FieldDeepCopyBaseType"}}
{{- $Src := SourceTarget .Target}}
	{{- if .NeedDecl}}
	var {{.Target}} {{.TypeName}}
	{{- end}}
	{{- if .IsPointer}}
		if {{$Src}} != nil {
			{{- if IsGoStringType .TypeName}}
			if *{{$Src}} != "" {
				tmp := kutils.StringDeepCopy(*{{$Src}})
				{{.Target}} = &tmp
			}
			{{- else if .Type.Category.IsBinary}}
			if len(*{{$Src}}) != 0 {
				tmp := make([]byte, len(*{{$Src}}))
				copy(tmp, *{{$Src}})
				{{.Target}} = &tmp
			}
			{{- else}}
			tmp := *{{$Src}}
			{{.Target}} = &tmp
			{{- end}}
		}
	{{- else}}
		{{- if IsGoStringType .TypeName}}
		if {{$Src}} != "" {
			{{.Target}} = kutils.StringDeepCopy({{$Src}})
		}
		{{- else if .Type.Category.IsBinary}}
		if len({{$Src}}) != 0 {
			tmp := make([]byte, len({{$Src}}))
			copy(tmp, {{$Src}})
			{{.Target}} = tmp
		}
		{{- else}}
		{{.Target}} = {{$Src}}
		{{- end}}
	{{- end}}
{{- end}}{{/* define "FieldDeepCopyBaseType" */}}
`

const fieldFastWrite = `
{{define "FieldFastWrite"}}
	{{- if .Type.Category.IsStructLike}}
		{{- template "FieldFastWriteStructLike" . -}}
	{{- else if .Type.Category.IsContainerType}}
		{{- template "FieldFastWriteContainer" . -}}
	{{- else}}{{/* IsBaseType */}}
		{{- template "FieldFastWriteBaseType" . -}}
	{{- end}}
{{- end}}{{/* define "FieldFastWrite" */}}
`

const fieldLength = `
{{define "FieldLength"}}
	{{- if .Type.Category.IsStructLike}}
		{{- template "FieldStructLikeLength" . -}}
	{{- else if .Type.Category.IsContainerType}}
		{{- template "FieldContainerLength" . -}}
	{{- else}}{{/* IsBaseType */}}
		{{- template "FieldBaseTypeLength" . -}}
	{{- end}}
{{- end}}{{/* define "FieldLength" */}}
`

const fieldFastWriteStructLike = `
{{define "FieldFastWriteStructLike"}}
	{{- if Features.WithFieldMask}}
	if {{.Target}} != nil {
		{{.Target}}.SetFieldMask({{.FieldMask}})
	}
	{{- end}}
	offset += {{.Target}}.FastWriteNocopy(buf[offset:], binaryWriter)
{{- end}}{{/* define "FieldFastWriteStructLike" */}}
`

const fieldStructLikeLength = `
{{define "FieldStructLikeLength"}}
{{- if Features.WithFieldMask}}
	if {{.Target}} != nil {
		{{.Target}}.SetFieldMask({{.FieldMask}})
	}
	{{- end}}
	l += {{.Target}}.BLength()
{{- end}}{{/* define "FieldStructLikeLength" */}}
`

const fieldFastWriteBaseType = `
{{define "FieldFastWriteBaseType"}}
{{- $Value := .Target}}
{{- if .IsPointer}}{{$Value = printf "*%s" $Value}}{{end}}
{{- if .Type.Category.IsEnum}}{{$Value = printf "int32(%s)" $Value}}{{end}}
{{- if .Type.Category.IsBinary}}{{$Value = printf "[]byte(%s)" $Value}}{{end}}
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
{{- if .Type.Category.IsEnum}}{{$Value = printf "int32(%s)" $Value}}{{end}}
{{- if .Type.Category.IsBinary}}{{$Value = printf "[]byte(%s)" $Value}}{{end}}
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
{{- $isIntKey := .KeyCtx.Type | IsIntType -}}
{{- $isStrKey := .KeyCtx.Type | IsStrType -}}
{{- $isBaseVal := .ValCtx.Type | IsBaseType -}}
{{- $curFieldMask := .FieldMask -}}
	{{- if and Features.WithFieldMask (or $isIntKey $isStrKey) }}
	{{- }}
	mapBeginOffset := offset
	offset += bthrift.Binary.MapBeginLength(thrift.
	{{- .KeyCtx.Type | GetTypeIDConstant -}}
	, thrift.{{- .ValCtx.Type | GetTypeIDConstant -}}, 0)
	var length int
	for k, v := range {{.Target}}{
		{{- if Features.WithFieldMask}}
		{{- if $isIntKey}}
		{{- $curFieldMask = "nfm"}}
		if {{if $isBaseVal}}_{{else}}{{$curFieldMask}}{{end}}, ex := {{.FieldMask}}.Int(int(k)); !ex {
			continue
		} else {
		{{- else if $isStrKey}}
		{{- $curFieldMask = "nfm"}}
		ks := string(k)
		if {{if $isBaseVal}}_{{else}}{{$curFieldMask}}{{end}}, ex := {{.FieldMask}}.Str(ks); !ex {
			continue
		} else {
		{{- else}}
		{{$curFieldMask}} = nil
		{{- end}}
		{{- end}}{{/* end Features.WithFieldMask */}}
		length++
		{{$ctx := .KeyCtx.WithTarget "k"}}
		{{- template "FieldFastWrite" $ctx}}
		{{- $ctx := (.ValCtx.WithTarget "v").WithFieldMask $curFieldMask -}}
		{{- template "FieldFastWrite" $ctx}}
		{{- if and Features.WithFieldMask (or $isIntKey $isStrKey)}}
		}
		{{- end}}
	}
	bthrift.Binary.WriteMapBegin(buf[mapBeginOffset:], thrift.
		{{- .KeyCtx.Type | GetTypeIDConstant -}}
		, thrift.{{- .ValCtx.Type | GetTypeIDConstant -}}
		, length)
	offset += bthrift.Binary.WriteMapEnd(buf[offset:])
{{- end}}{{/* define "FieldFastWriteMap" */}}
`

const fieldMapLength = `
{{define "FieldMapLength"}}
{{- $isIntKey := .KeyCtx.Type | IsIntType -}}
{{- $isStrKey := .KeyCtx.Type | IsStrType -}}
{{- $isBaseVal := .ValCtx.Type | IsBaseType -}}
{{- $curFieldMask := .FieldMask -}}
	l += bthrift.Binary.MapBeginLength(thrift.
		{{- .KeyCtx.Type | GetTypeIDConstant -}}
		, thrift.{{- .ValCtx.Type | GetTypeIDConstant -}}
		, len({{.Target}}))
	{{- if and (IsFixedLengthType .KeyCtx.Type) (IsFixedLengthType .ValCtx.Type)}}
	var tmpK {{.KeyCtx.TypeName}}
	var tmpV {{.ValCtx.TypeName}}
	l += ({{- $ctx := .KeyCtx.WithTarget "tmpK" -}}
		{{- template "FieldFixedLengthTypeLength" $ctx}} +
		{{- $ctx := .ValCtx.WithTarget "tmpV" -}}
		{{- template "FieldFixedLengthTypeLength" $ctx}}) * len({{.Target}})
	{{- else}}
	for k, v := range {{.Target}}{
		{{- if Features.WithFieldMask}}
		{{- if $isIntKey}}
		{{- $curFieldMask = "nfm"}}
		if {{if $isBaseVal}}_{{else}}{{$curFieldMask}}{{end}}, ex := {{.FieldMask}}.Int(int(k)); !ex {
			continue
		} else {
		{{- else if $isStrKey}}
		{{- $curFieldMask = "nfm"}}
		ks := string(k)
		if {{if $isBaseVal}}_{{else}}{{$curFieldMask}}{{end}}, ex := {{.FieldMask}}.Str(ks); !ex {
			continue
		} else {
		{{- else}}
		{{$curFieldMask}} = nil
		{{- end}}
		{{- end}}{{/* end Features.WithFieldMask */}}
		{{$ctx := .KeyCtx.WithTarget "k"}}
		{{- template "FieldLength" $ctx}}
		{{- $ctx := (.ValCtx.WithTarget "v").WithFieldMask $curFieldMask -}}
		{{- template "FieldLength" $ctx}}
		{{- if and Features.WithFieldMask (or $isIntKey $isStrKey)}}
		}
		{{- end}}
	}
	{{- end}}{{/* if */}}
	l += bthrift.Binary.MapEndLength()
{{- end}}{{/* define "FieldMapLength" */}}
`

const fieldFastWriteSet = `
{{define "FieldFastWriteSet"}}
{{- $isBaseVal := .ValCtx.Type | IsBaseType -}}
{{- $curFieldMask := .FieldMask -}}
		setBeginOffset := offset
		offset += bthrift.Binary.SetBeginLength(thrift.
		{{- .ValCtx.Type | GetTypeIDConstant -}}, 0)
		{{template "ValidateSet" .}}
		var length int
		for {{if Features.WithFieldMask}}i{{else}}_{{end}}, v := range {{.Target}} {
			{{- if Features.WithFieldMask}}
			{{- $curFieldMask = "nfm"}}
			if {{if $isBaseVal}}_{{else}}{{$curFieldMask}}{{end}}, ex := {{.FieldMask}}.Int(i); !ex {
				continue
			} else {
			{{- end}}
			length++
			{{- $ctx := (.ValCtx.WithTarget "v").WithFieldMask $curFieldMask -}}
			{{- template "FieldFastWrite" $ctx}}
			{{- if Features.WithFieldMask}}
			}
			{{- end}}
		}
		bthrift.Binary.WriteSetBegin(buf[setBeginOffset:], thrift.
		{{- .ValCtx.Type | GetTypeIDConstant -}}
		, length)
		offset += bthrift.Binary.WriteSetEnd(buf[offset:])
{{- end}}{{/* define "FieldFastWriteSet" */}}
`

const fieldSetLength = `
{{define "FieldSetLength"}}
{{- $isBaseVal := .ValCtx.Type | IsBaseType -}}
{{- $curFieldMask := .FieldMask -}}
		l += bthrift.Binary.SetBeginLength(thrift.
		{{- .ValCtx.Type | GetTypeIDConstant -}}
		, len({{.Target}}))
		{{template "ValidateSet" .}}
		{{- if IsFixedLengthType .ValCtx.Type}}
		var tmpV {{.ValCtx.TypeName}}
		l += {{- $ctx := .ValCtx.WithTarget "tmpV" -}}
			{{- template "FieldFixedLengthTypeLength" $ctx -}} * len({{.Target}})
		{{- else}}
		for _, v := range {{.Target}} {
			{{- if Features.WithFieldMask}}
			{{- $curFieldMask = "nfm"}}
			if {{if $isBaseVal}}_{{else}}{{$curFieldMask}}{{end}}, ex := {{.FieldMask}}.Int(i); !ex {
				continue
			} else {
			{{- end}}
			{{- $ctx := (.ValCtx.WithTarget "v").WithFieldMask $curFieldMask -}}
			{{- template "FieldLength" $ctx}}
			{{- if Features.WithFieldMask}}
			}
			{{- end}}
		}
		{{- end}}{{/* if */}}
		l += bthrift.Binary.SetEndLength()
{{- end}}{{/* define "FieldSetLength" */}}
`

const fieldFastWriteList = `
{{define "FieldFastWriteList"}}
{{- $isBaseVal := .ValCtx.Type | IsBaseType -}}
{{- $curFieldMask := .FieldMask -}}
		listBeginOffset := offset
		offset += bthrift.Binary.ListBeginLength(thrift.
		{{- .ValCtx.Type | GetTypeIDConstant -}}, 0)
		var length int
		for {{if Features.WithFieldMask}}i{{else}}_{{end}}, v := range {{.Target}} {
			{{- if Features.WithFieldMask}}
			{{- $curFieldMask = "nfm"}}
			if {{if $isBaseVal}}_{{else}}{{$curFieldMask}}{{end}}, ex := {{.FieldMask}}.Int(i); !ex {
				continue
			} else {
			{{- end}}
			length++
			{{- $ctx := (.ValCtx.WithTarget "v").WithFieldMask $curFieldMask -}}
			{{- template "FieldFastWrite" $ctx}}
			{{- if Features.WithFieldMask}}
			}
			{{- end}}
		}
		bthrift.Binary.WriteListBegin(buf[listBeginOffset:], thrift.
		{{- .ValCtx.Type | GetTypeIDConstant -}}
		, length)
		offset += bthrift.Binary.WriteListEnd(buf[offset:])
{{- end}}{{/* define "FieldFastWriteList" */}}
`

const fieldListLength = `
{{define "FieldListLength"}}
{{- $isBaseVal := .ValCtx.Type | IsBaseType -}}
{{- $curFieldMask := .FieldMask -}}
		l += bthrift.Binary.ListBeginLength(thrift.
		{{- .ValCtx.Type | GetTypeIDConstant -}}
		, len({{.Target}}))
		{{- if IsFixedLengthType .ValCtx.Type}}
		var tmpV {{.ValCtx.TypeName}}
		l += {{- $ctx := .ValCtx.WithTarget "tmpV" -}}
			{{- template "FieldFixedLengthTypeLength" $ctx -}} * len({{.Target}})
		{{- else}}
		for {{if Features.WithFieldMask}}i{{else}}_{{end}}, v := range {{.Target}} {
			{{- if Features.WithFieldMask}}
			{{- $curFieldMask = "nfm"}}
			if {{if $isBaseVal}}_{{else}}{{$curFieldMask}}{{end}}, ex := {{.FieldMask}}.Int(i); !ex {
				continue
			} else {
			{{- end}}
			{{- $ctx := (.ValCtx.WithTarget "v").WithFieldMask $curFieldMask -}}
			{{- template "FieldLength" $ctx}}
			{{- if Features.WithFieldMask}}
			}
			{{- end}}
		}
		{{- end}}{{/* if */}}
		l += bthrift.Binary.ListEndLength()
{{- end}}{{/* define "FieldListLength" */}}
`

const processor = `
{{define "Processor"}}
{{- range .Functions}}
{{$ArgsType := .ArgType}}
{{template "StructLikeCodec" $ArgsType}}
{{- if not .Oneway}}
	{{$ResType := .ResType}}
	{{template "StructLikeCodec" $ResType}}
{{- end}}
{{- end}}{{/* range .Functions */}}
{{- end}}{{/* define "Processor" */}}
`

const validateSet = `
{{define "ValidateSet"}}
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
		if reflect.DeepEqual({{.Target}}[i], {{.Target}}[j]) {
{{- end}}
			panic(fmt.Errorf("%T error writing set field: slice is not unique", {{.Target}}[i]))
		}
	}
}
{{- end}}
{{- end}}{{/* define "ValidateSet" */}}`
