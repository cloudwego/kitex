/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package streamx_test

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"

	"github.com/cloudwego/gopkg/protocol/thrift"
)

// unused protection
var (
	_ = fmt.Formatter(nil)
	_ = (*bytes.Buffer)(nil)
	_ = (*strings.Builder)(nil)
	_ = reflect.Type(nil)
)

var fieldIDToName_Request = map[int16]string{
	1: "Type",
	2: "Message",
}

var fieldIDToName_Response = map[int16]string{
	1: "Type",
	2: "Message",
}

type Request struct {
	Type    int32  `thrift:"Type,1" frugal:"1,default,i32" json:"Type"`
	Message string `thrift:"Message,2" frugal:"2,default,string" json:"Message"`
}

type Response struct {
	Type    int32  `thrift:"Type,1" frugal:"1,default,i32" json:"Type"`
	Message string `thrift:"Message,2" frugal:"2,default,string" json:"Message"`
}

type ServerPingPongArgs struct {
	Req *Request `thrift:"req,1" frugal:"1,default,Request" json:"req"`
}

type ServerPingPongResult struct {
	Success *Response `thrift:"success,0,optional" frugal:"0,optional,Response" json:"success,omitempty"`
}

func NewServerPingPongArgs() *ServerPingPongArgs {
	return &ServerPingPongArgs{}
}

func NewServerPingPongResult() *ServerPingPongResult {
	return &ServerPingPongResult{}
}

func (p ServerPingPongResult) GetSuccess() *Response {
	return p.Success
}

func (p *Request) FastRead(buf []byte) (int, error) {
	var err error
	var offset int
	var l int
	var fieldTypeId thrift.TType
	var fieldId int16
	var issetType bool = false
	var issetMessage bool = false
	for {
		fieldTypeId, fieldId, l, err = thrift.Binary.ReadFieldBegin(buf[offset:])
		offset += l
		if err != nil {
			goto ReadFieldBeginError
		}
		if fieldTypeId == thrift.STOP {
			break
		}
		switch fieldId {
		case 1:
			if fieldTypeId == thrift.I32 {
				l, err = p.FastReadField1(buf[offset:])
				offset += l
				if err != nil {
					goto ReadFieldError
				}
				issetType = true
			} else {
				l, err = thrift.Binary.Skip(buf[offset:], fieldTypeId)
				offset += l
				if err != nil {
					goto SkipFieldError
				}
			}
		case 2:
			if fieldTypeId == thrift.STRING {
				l, err = p.FastReadField2(buf[offset:])
				offset += l
				if err != nil {
					goto ReadFieldError
				}
				issetMessage = true
			} else {
				l, err = thrift.Binary.Skip(buf[offset:], fieldTypeId)
				offset += l
				if err != nil {
					goto SkipFieldError
				}
			}
		default:
			l, err = thrift.Binary.Skip(buf[offset:], fieldTypeId)
			offset += l
			if err != nil {
				goto SkipFieldError
			}
		}
	}

	if !issetType {
		fieldId = 1
		goto RequiredFieldNotSetError
	}

	if !issetMessage {
		fieldId = 2
		goto RequiredFieldNotSetError
	}
	return offset, nil
ReadFieldBeginError:
	return offset, thrift.PrependError(fmt.Sprintf("%T read field %d begin error: ", p, fieldId), err)
ReadFieldError:
	return offset, thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_Request[fieldId]), err)
SkipFieldError:
	return offset, thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)
RequiredFieldNotSetError:
	return offset, thrift.NewProtocolException(thrift.INVALID_DATA, fmt.Sprintf("required field %s is not set", fieldIDToName_Request[fieldId]))
}

func (p *Request) FastReadField1(buf []byte) (int, error) {
	offset := 0

	var _field int32
	if v, l, err := thrift.Binary.ReadI32(buf[offset:]); err != nil {
		return offset, err
	} else {
		offset += l
		_field = v
	}
	p.Type = _field
	return offset, nil
}

func (p *Request) FastReadField2(buf []byte) (int, error) {
	offset := 0

	var _field string
	if v, l, err := thrift.Binary.ReadString(buf[offset:]); err != nil {
		return offset, err
	} else {
		offset += l
		_field = v
	}
	p.Message = _field
	return offset, nil
}

func (p *Request) FastWrite(buf []byte) int {
	return p.FastWriteNocopy(buf, nil)
}

func (p *Request) FastWriteNocopy(buf []byte, w thrift.NocopyWriter) int {
	offset := 0
	if p != nil {
		offset += p.fastWriteField1(buf[offset:], w)
		offset += p.fastWriteField2(buf[offset:], w)
	}
	offset += thrift.Binary.WriteFieldStop(buf[offset:])
	return offset
}

func (p *Request) BLength() int {
	l := 0
	if p != nil {
		l += p.field1Length()
		l += p.field2Length()
	}
	l += thrift.Binary.FieldStopLength()
	return l
}

func (p *Request) fastWriteField1(buf []byte, w thrift.NocopyWriter) int {
	offset := 0
	offset += thrift.Binary.WriteFieldBegin(buf[offset:], thrift.I32, 1)
	offset += thrift.Binary.WriteI32(buf[offset:], p.Type)
	return offset
}

func (p *Request) fastWriteField2(buf []byte, w thrift.NocopyWriter) int {
	offset := 0
	offset += thrift.Binary.WriteFieldBegin(buf[offset:], thrift.STRING, 2)
	offset += thrift.Binary.WriteStringNocopy(buf[offset:], w, p.Message)
	return offset
}

func (p *Request) field1Length() int {
	l := 0
	l += thrift.Binary.FieldBeginLength()
	l += thrift.Binary.I32Length()
	return l
}

func (p *Request) field2Length() int {
	l := 0
	l += thrift.Binary.FieldBeginLength()
	l += thrift.Binary.StringLengthNocopy(p.Message)
	return l
}

func (p *Response) FastRead(buf []byte) (int, error) {
	var err error
	var offset int
	var l int
	var fieldTypeId thrift.TType
	var fieldId int16
	var issetType bool = false
	var issetMessage bool = false
	for {
		fieldTypeId, fieldId, l, err = thrift.Binary.ReadFieldBegin(buf[offset:])
		offset += l
		if err != nil {
			goto ReadFieldBeginError
		}
		if fieldTypeId == thrift.STOP {
			break
		}
		switch fieldId {
		case 1:
			if fieldTypeId == thrift.I32 {
				l, err = p.FastReadField1(buf[offset:])
				offset += l
				if err != nil {
					goto ReadFieldError
				}
				issetType = true
			} else {
				l, err = thrift.Binary.Skip(buf[offset:], fieldTypeId)
				offset += l
				if err != nil {
					goto SkipFieldError
				}
			}
		case 2:
			if fieldTypeId == thrift.STRING {
				l, err = p.FastReadField2(buf[offset:])
				offset += l
				if err != nil {
					goto ReadFieldError
				}
				issetMessage = true
			} else {
				l, err = thrift.Binary.Skip(buf[offset:], fieldTypeId)
				offset += l
				if err != nil {
					goto SkipFieldError
				}
			}
		default:
			l, err = thrift.Binary.Skip(buf[offset:], fieldTypeId)
			offset += l
			if err != nil {
				goto SkipFieldError
			}
		}
	}

	if !issetType {
		fieldId = 1
		goto RequiredFieldNotSetError
	}

	if !issetMessage {
		fieldId = 2
		goto RequiredFieldNotSetError
	}
	return offset, nil
ReadFieldBeginError:
	return offset, thrift.PrependError(fmt.Sprintf("%T read field %d begin error: ", p, fieldId), err)
ReadFieldError:
	return offset, thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_Response[fieldId]), err)
SkipFieldError:
	return offset, thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)
RequiredFieldNotSetError:
	return offset, thrift.NewProtocolException(thrift.INVALID_DATA, fmt.Sprintf("required field %s is not set", fieldIDToName_Response[fieldId]))
}

func (p *Response) FastReadField1(buf []byte) (int, error) {
	offset := 0

	var _field int32
	if v, l, err := thrift.Binary.ReadI32(buf[offset:]); err != nil {
		return offset, err
	} else {
		offset += l
		_field = v
	}
	p.Type = _field
	return offset, nil
}

func (p *Response) FastReadField2(buf []byte) (int, error) {
	offset := 0

	var _field string
	if v, l, err := thrift.Binary.ReadString(buf[offset:]); err != nil {
		return offset, err
	} else {
		offset += l
		_field = v
	}
	p.Message = _field
	return offset, nil
}

func (p *Response) FastWrite(buf []byte) int {
	return p.FastWriteNocopy(buf, nil)
}

func (p *Response) FastWriteNocopy(buf []byte, w thrift.NocopyWriter) int {
	offset := 0
	if p != nil {
		offset += p.fastWriteField1(buf[offset:], w)
		offset += p.fastWriteField2(buf[offset:], w)
	}
	offset += thrift.Binary.WriteFieldStop(buf[offset:])
	return offset
}

func (p *Response) BLength() int {
	l := 0
	if p != nil {
		l += p.field1Length()
		l += p.field2Length()
	}
	l += thrift.Binary.FieldStopLength()
	return l
}

func (p *Response) fastWriteField1(buf []byte, w thrift.NocopyWriter) int {
	offset := 0
	offset += thrift.Binary.WriteFieldBegin(buf[offset:], thrift.I32, 1)
	offset += thrift.Binary.WriteI32(buf[offset:], p.Type)
	return offset
}

func (p *Response) fastWriteField2(buf []byte, w thrift.NocopyWriter) int {
	offset := 0
	offset += thrift.Binary.WriteFieldBegin(buf[offset:], thrift.STRING, 2)
	offset += thrift.Binary.WriteStringNocopy(buf[offset:], w, p.Message)
	return offset
}

func (p *Response) field1Length() int {
	l := 0
	l += thrift.Binary.FieldBeginLength()
	l += thrift.Binary.I32Length()
	return l
}

func (p *Response) field2Length() int {
	l := 0
	l += thrift.Binary.FieldBeginLength()
	l += thrift.Binary.StringLengthNocopy(p.Message)
	return l
}
