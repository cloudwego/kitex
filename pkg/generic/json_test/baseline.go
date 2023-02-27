/*
 * Copyright 2021 CloudWeGo Authors
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

package test

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/apache/thrift/lib/go/thrift"
)

type Simple struct {
	ByteField   int8    `thrift:"ByteField,1" json:"ByteField"`
	I64Field    int64   `thrift:"I64Field,2" json:"I64Field"`
	DoubleField float64 `thrift:"DoubleField,3" json:"DoubleField"`
	I32Field    int32   `thrift:"I32Field,4" json:"I32Field"`
	StringField string  `thrift:"StringField,5" json:"StringField"`
	BinaryField []byte  `thrift:"BinaryField,6" json:"BinaryField"`
}

func NewSimple() *Simple {
	return &Simple{}
}

func (p *Simple) GetByteField() (v int8) {
	return p.ByteField
}

func (p *Simple) GetI64Field() (v int64) {
	return p.I64Field
}

func (p *Simple) GetDoubleField() (v float64) {
	return p.DoubleField
}

func (p *Simple) GetI32Field() (v int32) {
	return p.I32Field
}

func (p *Simple) GetStringField() (v string) {
	return p.StringField
}

func (p *Simple) GetBinaryField() (v []byte) {
	return p.BinaryField
}

func (p *Simple) SetByteField(val int8) {
	p.ByteField = val
}

func (p *Simple) SetI64Field(val int64) {
	p.I64Field = val
}

func (p *Simple) SetDoubleField(val float64) {
	p.DoubleField = val
}

func (p *Simple) SetI32Field(val int32) {
	p.I32Field = val
}

func (p *Simple) SetStringField(val string) {
	p.StringField = val
}

func (p *Simple) SetBinaryField(val []byte) {
	p.BinaryField = val
}

var fieldIDToName_Simple = map[int16]string{
	1: "ByteField",
	2: "I64Field",
	3: "DoubleField",
	4: "I32Field",
	5: "StringField",
	6: "BinaryField",
}

func (p *Simple) Read(iprot thrift.TProtocol) (err error) {
	var fieldTypeId thrift.TType
	var fieldId int16

	if _, err = iprot.ReadStructBegin(); err != nil {
		goto ReadStructBeginError
	}

	for {
		_, fieldTypeId, fieldId, err = iprot.ReadFieldBegin()
		if err != nil {
			goto ReadFieldBeginError
		}
		if fieldTypeId == thrift.STOP {
			break
		}

		switch fieldId {
		case 1:
			if fieldTypeId == thrift.BYTE {
				if err = p.ReadField1(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 2:
			if fieldTypeId == thrift.I64 {
				if err = p.ReadField2(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 3:
			if fieldTypeId == thrift.DOUBLE {
				if err = p.ReadField3(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 4:
			if fieldTypeId == thrift.I32 {
				if err = p.ReadField4(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 5:
			if fieldTypeId == thrift.STRING {
				if err = p.ReadField5(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 6:
			if fieldTypeId == thrift.STRING {
				if err = p.ReadField6(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		default:
			if err = iprot.Skip(fieldTypeId); err != nil {
				goto SkipFieldError
			}
		}

		if err = iprot.ReadFieldEnd(); err != nil {
			goto ReadFieldEndError
		}
	}
	if err = iprot.ReadStructEnd(); err != nil {
		goto ReadStructEndError
	}

	return nil
ReadStructBeginError:
	return thrift.PrependError(fmt.Sprintf("%T read struct begin error: ", p), err)
ReadFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T read field %d begin error: ", p, fieldId), err)
ReadFieldError:
	return thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_Simple[fieldId]), err)
SkipFieldError:
	return thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)

ReadFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T read field end error", p), err)
ReadStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
}

func (p *Simple) ReadField1(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadByte(); err != nil {
		return err
	} else {
		p.ByteField = v
	}
	return nil
}

func (p *Simple) ReadField2(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadI64(); err != nil {
		return err
	} else {
		p.I64Field = v
	}
	return nil
}

func (p *Simple) ReadField3(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadDouble(); err != nil {
		return err
	} else {
		p.DoubleField = v
	}
	return nil
}

func (p *Simple) ReadField4(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadI32(); err != nil {
		return err
	} else {
		p.I32Field = v
	}
	return nil
}

func (p *Simple) ReadField5(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadString(); err != nil {
		return err
	} else {
		p.StringField = v
	}
	return nil
}

func (p *Simple) ReadField6(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadBinary(); err != nil {
		return err
	} else {
		p.BinaryField = []byte(v)
	}
	return nil
}

func (p *Simple) Write(oprot thrift.TProtocol) (err error) {
	var fieldId int16
	if err = oprot.WriteStructBegin("Simple"); err != nil {
		goto WriteStructBeginError
	}
	if p != nil {
		if err = p.writeField1(oprot); err != nil {
			fieldId = 1
			goto WriteFieldError
		}
		if err = p.writeField2(oprot); err != nil {
			fieldId = 2
			goto WriteFieldError
		}
		if err = p.writeField3(oprot); err != nil {
			fieldId = 3
			goto WriteFieldError
		}
		if err = p.writeField4(oprot); err != nil {
			fieldId = 4
			goto WriteFieldError
		}
		if err = p.writeField5(oprot); err != nil {
			fieldId = 5
			goto WriteFieldError
		}
		if err = p.writeField6(oprot); err != nil {
			fieldId = 6
			goto WriteFieldError
		}

	}
	if err = oprot.WriteFieldStop(); err != nil {
		goto WriteFieldStopError
	}
	if err = oprot.WriteStructEnd(); err != nil {
		goto WriteStructEndError
	}
	return nil
WriteStructBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write struct begin error: ", p), err)
WriteFieldError:
	return thrift.PrependError(fmt.Sprintf("%T write field %d error: ", p, fieldId), err)
WriteFieldStopError:
	return thrift.PrependError(fmt.Sprintf("%T write field stop error: ", p), err)
WriteStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T write struct end error: ", p), err)
}

func (p *Simple) writeField1(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("ByteField", thrift.BYTE, 1); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteByte(p.ByteField); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 1 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 1 end error: ", p), err)
}

func (p *Simple) writeField2(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("I64Field", thrift.I64, 2); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteI64(p.I64Field); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 2 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 2 end error: ", p), err)
}

func (p *Simple) writeField3(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("DoubleField", thrift.DOUBLE, 3); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteDouble(p.DoubleField); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 3 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 3 end error: ", p), err)
}

func (p *Simple) writeField4(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("I32Field", thrift.I32, 4); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteI32(p.I32Field); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 4 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 4 end error: ", p), err)
}

func (p *Simple) writeField5(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("StringField", thrift.STRING, 5); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteString(p.StringField); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 5 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 5 end error: ", p), err)
}

func (p *Simple) writeField6(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("BinaryField", thrift.STRING, 6); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteBinary([]byte(p.BinaryField)); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 6 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 6 end error: ", p), err)
}

func (p *Simple) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("Simple(%+v)", *p)
}

func (p *Simple) DeepEqual(ano *Simple) bool {
	if p == ano {
		return true
	} else if p == nil || ano == nil {
		return false
	}
	if !p.Field1DeepEqual(ano.ByteField) {
		return false
	}
	if !p.Field2DeepEqual(ano.I64Field) {
		return false
	}
	if !p.Field3DeepEqual(ano.DoubleField) {
		return false
	}
	if !p.Field4DeepEqual(ano.I32Field) {
		return false
	}
	if !p.Field5DeepEqual(ano.StringField) {
		return false
	}
	if !p.Field6DeepEqual(ano.BinaryField) {
		return false
	}
	return true
}

func (p *Simple) Field1DeepEqual(src int8) bool {
	return p.ByteField == src
}

func (p *Simple) Field2DeepEqual(src int64) bool {
	return p.I64Field == src
}

func (p *Simple) Field3DeepEqual(src float64) bool {
	return p.DoubleField == src
}

func (p *Simple) Field4DeepEqual(src int32) bool {
	return p.I32Field == src
}

func (p *Simple) Field5DeepEqual(src string) bool {
	return strings.Compare(p.StringField, src) == 0
}

func (p *Simple) Field6DeepEqual(src []byte) bool {
	return bytes.Equal(p.BinaryField, src)
}

type Nesting struct {
	String_         string             `thrift:"String,1" json:"String"`
	ListSimple      []*Simple          `thrift:"ListSimple,2" json:"ListSimple"`
	Double          float64            `thrift:"Double,3" json:"Double"`
	I32             int32              `thrift:"I32,4" json:"I32"`
	ListI32         []int32            `thrift:"ListI32,5" json:"ListI32"`
	I64             int64              `thrift:"I64,6" json:"I64"`
	MapStringString map[string]string  `thrift:"MapStringString,7" json:"MapStringString"`
	SimpleStruct    *Simple            `thrift:"SimpleStruct,8" json:"SimpleStruct"`
	MapI32I64       map[int32]int64    `thrift:"MapI32I64,9" json:"MapI32I64"`
	ListString      []string           `thrift:"ListString,10" json:"ListString"`
	Binary          []byte             `thrift:"Binary,11" json:"Binary"`
	MapI64String    map[int64]string   `thrift:"MapI64String,12" json:"MapI64String"`
	ListI64         []int64            `thrift:"ListI64,13" json:"ListI64"`
	Byte            int8               `thrift:"Byte,14" json:"Byte"`
	MapStringSimple map[string]*Simple `thrift:"MapStringSimple,15" json:"MapStringSimple"`
}

func NewNesting() *Nesting {
	return &Nesting{}
}

func (p *Nesting) GetString() (v string) {
	return p.String_
}

func (p *Nesting) GetListSimple() (v []*Simple) {
	return p.ListSimple
}

func (p *Nesting) GetDouble() (v float64) {
	return p.Double
}

func (p *Nesting) GetI32() (v int32) {
	return p.I32
}

func (p *Nesting) GetListI32() (v []int32) {
	return p.ListI32
}

func (p *Nesting) GetI64() (v int64) {
	return p.I64
}

func (p *Nesting) GetMapStringString() (v map[string]string) {
	return p.MapStringString
}

var Nesting_SimpleStruct_DEFAULT *Simple

func (p *Nesting) GetSimpleStruct() (v *Simple) {
	if !p.IsSetSimpleStruct() {
		return Nesting_SimpleStruct_DEFAULT
	}
	return p.SimpleStruct
}

func (p *Nesting) GetMapI32I64() (v map[int32]int64) {
	return p.MapI32I64
}

func (p *Nesting) GetListString() (v []string) {
	return p.ListString
}

func (p *Nesting) GetBinary() (v []byte) {
	return p.Binary
}

func (p *Nesting) GetMapI64String() (v map[int64]string) {
	return p.MapI64String
}

func (p *Nesting) GetListI64() (v []int64) {
	return p.ListI64
}

func (p *Nesting) GetByte() (v int8) {
	return p.Byte
}

func (p *Nesting) GetMapStringSimple() (v map[string]*Simple) {
	return p.MapStringSimple
}

func (p *Nesting) SetString(val string) {
	p.String_ = val
}

func (p *Nesting) SetListSimple(val []*Simple) {
	p.ListSimple = val
}

func (p *Nesting) SetDouble(val float64) {
	p.Double = val
}

func (p *Nesting) SetI32(val int32) {
	p.I32 = val
}

func (p *Nesting) SetListI32(val []int32) {
	p.ListI32 = val
}

func (p *Nesting) SetI64(val int64) {
	p.I64 = val
}

func (p *Nesting) SetMapStringString(val map[string]string) {
	p.MapStringString = val
}

func (p *Nesting) SetSimpleStruct(val *Simple) {
	p.SimpleStruct = val
}

func (p *Nesting) SetMapI32I64(val map[int32]int64) {
	p.MapI32I64 = val
}

func (p *Nesting) SetListString(val []string) {
	p.ListString = val
}

func (p *Nesting) SetBinary(val []byte) {
	p.Binary = val
}

func (p *Nesting) SetMapI64String(val map[int64]string) {
	p.MapI64String = val
}

func (p *Nesting) SetListI64(val []int64) {
	p.ListI64 = val
}

func (p *Nesting) SetByte(val int8) {
	p.Byte = val
}

func (p *Nesting) SetMapStringSimple(val map[string]*Simple) {
	p.MapStringSimple = val
}

var fieldIDToName_Nesting = map[int16]string{
	1:  "String",
	2:  "ListSimple",
	3:  "Double",
	4:  "I32",
	5:  "ListI32",
	6:  "I64",
	7:  "MapStringString",
	8:  "SimpleStruct",
	9:  "MapI32I64",
	10: "ListString",
	11: "Binary",
	12: "MapI64String",
	13: "ListI64",
	14: "Byte",
	15: "MapStringSimple",
}

func (p *Nesting) IsSetSimpleStruct() bool {
	return p.SimpleStruct != nil
}

func (p *Nesting) Read(iprot thrift.TProtocol) (err error) {
	var fieldTypeId thrift.TType
	var fieldId int16

	if _, err = iprot.ReadStructBegin(); err != nil {
		goto ReadStructBeginError
	}

	for {
		_, fieldTypeId, fieldId, err = iprot.ReadFieldBegin()
		if err != nil {
			goto ReadFieldBeginError
		}
		if fieldTypeId == thrift.STOP {
			break
		}

		switch fieldId {
		case 1:
			if fieldTypeId == thrift.STRING {
				if err = p.ReadField1(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 2:
			if fieldTypeId == thrift.LIST {
				if err = p.ReadField2(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 3:
			if fieldTypeId == thrift.DOUBLE {
				if err = p.ReadField3(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 4:
			if fieldTypeId == thrift.I32 {
				if err = p.ReadField4(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 5:
			if fieldTypeId == thrift.LIST {
				if err = p.ReadField5(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 6:
			if fieldTypeId == thrift.I64 {
				if err = p.ReadField6(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 7:
			if fieldTypeId == thrift.MAP {
				if err = p.ReadField7(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 8:
			if fieldTypeId == thrift.STRUCT {
				if err = p.ReadField8(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 9:
			if fieldTypeId == thrift.MAP {
				if err = p.ReadField9(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 10:
			if fieldTypeId == thrift.LIST {
				if err = p.ReadField10(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 11:
			if fieldTypeId == thrift.STRING {
				if err = p.ReadField11(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 12:
			if fieldTypeId == thrift.MAP {
				if err = p.ReadField12(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 13:
			if fieldTypeId == thrift.LIST {
				if err = p.ReadField13(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 14:
			if fieldTypeId == thrift.BYTE {
				if err = p.ReadField14(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 15:
			if fieldTypeId == thrift.MAP {
				if err = p.ReadField15(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		default:
			if err = iprot.Skip(fieldTypeId); err != nil {
				goto SkipFieldError
			}
		}

		if err = iprot.ReadFieldEnd(); err != nil {
			goto ReadFieldEndError
		}
	}
	if err = iprot.ReadStructEnd(); err != nil {
		goto ReadStructEndError
	}

	return nil
ReadStructBeginError:
	return thrift.PrependError(fmt.Sprintf("%T read struct begin error: ", p), err)
ReadFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T read field %d begin error: ", p, fieldId), err)
ReadFieldError:
	return thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_Nesting[fieldId]), err)
SkipFieldError:
	return thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)

ReadFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T read field end error", p), err)
ReadStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
}

func (p *Nesting) ReadField1(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadString(); err != nil {
		return err
	} else {
		p.String_ = v
	}
	return nil
}

func (p *Nesting) ReadField2(iprot thrift.TProtocol) error {
	_, size, err := iprot.ReadListBegin()
	if err != nil {
		return err
	}
	p.ListSimple = make([]*Simple, 0, size)
	for i := 0; i < size; i++ {
		_elem := NewSimple()
		if err := _elem.Read(iprot); err != nil {
			return err
		}

		p.ListSimple = append(p.ListSimple, _elem)
	}
	if err := iprot.ReadListEnd(); err != nil {
		return err
	}
	return nil
}

func (p *Nesting) ReadField3(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadDouble(); err != nil {
		return err
	} else {
		p.Double = v
	}
	return nil
}

func (p *Nesting) ReadField4(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadI32(); err != nil {
		return err
	} else {
		p.I32 = v
	}
	return nil
}

func (p *Nesting) ReadField5(iprot thrift.TProtocol) error {
	_, size, err := iprot.ReadListBegin()
	if err != nil {
		return err
	}
	p.ListI32 = make([]int32, 0, size)
	for i := 0; i < size; i++ {
		var _elem int32
		if v, err := iprot.ReadI32(); err != nil {
			return err
		} else {
			_elem = v
		}

		p.ListI32 = append(p.ListI32, _elem)
	}
	if err := iprot.ReadListEnd(); err != nil {
		return err
	}
	return nil
}

func (p *Nesting) ReadField6(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadI64(); err != nil {
		return err
	} else {
		p.I64 = v
	}
	return nil
}

func (p *Nesting) ReadField7(iprot thrift.TProtocol) error {
	_, _, size, err := iprot.ReadMapBegin()
	if err != nil {
		return err
	}
	p.MapStringString = make(map[string]string, size)
	for i := 0; i < size; i++ {
		var _key string
		if v, err := iprot.ReadString(); err != nil {
			return err
		} else {
			_key = v
		}

		var _val string
		if v, err := iprot.ReadString(); err != nil {
			return err
		} else {
			_val = v
		}

		p.MapStringString[_key] = _val
	}
	if err := iprot.ReadMapEnd(); err != nil {
		return err
	}
	return nil
}

func (p *Nesting) ReadField8(iprot thrift.TProtocol) error {
	p.SimpleStruct = NewSimple()
	if err := p.SimpleStruct.Read(iprot); err != nil {
		return err
	}
	return nil
}

func (p *Nesting) ReadField9(iprot thrift.TProtocol) error {
	_, _, size, err := iprot.ReadMapBegin()
	if err != nil {
		return err
	}
	p.MapI32I64 = make(map[int32]int64, size)
	for i := 0; i < size; i++ {
		var _key int32
		if v, err := iprot.ReadI32(); err != nil {
			return err
		} else {
			_key = v
		}

		var _val int64
		if v, err := iprot.ReadI64(); err != nil {
			return err
		} else {
			_val = v
		}

		p.MapI32I64[_key] = _val
	}
	if err := iprot.ReadMapEnd(); err != nil {
		return err
	}
	return nil
}

func (p *Nesting) ReadField10(iprot thrift.TProtocol) error {
	_, size, err := iprot.ReadListBegin()
	if err != nil {
		return err
	}
	p.ListString = make([]string, 0, size)
	for i := 0; i < size; i++ {
		var _elem string
		if v, err := iprot.ReadString(); err != nil {
			return err
		} else {
			_elem = v
		}

		p.ListString = append(p.ListString, _elem)
	}
	if err := iprot.ReadListEnd(); err != nil {
		return err
	}
	return nil
}

func (p *Nesting) ReadField11(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadBinary(); err != nil {
		return err
	} else {
		p.Binary = []byte(v)
	}
	return nil
}

func (p *Nesting) ReadField12(iprot thrift.TProtocol) error {
	_, _, size, err := iprot.ReadMapBegin()
	if err != nil {
		return err
	}
	p.MapI64String = make(map[int64]string, size)
	for i := 0; i < size; i++ {
		var _key int64
		if v, err := iprot.ReadI64(); err != nil {
			return err
		} else {
			_key = v
		}

		var _val string
		if v, err := iprot.ReadString(); err != nil {
			return err
		} else {
			_val = v
		}

		p.MapI64String[_key] = _val
	}
	if err := iprot.ReadMapEnd(); err != nil {
		return err
	}
	return nil
}

func (p *Nesting) ReadField13(iprot thrift.TProtocol) error {
	_, size, err := iprot.ReadListBegin()
	if err != nil {
		return err
	}
	p.ListI64 = make([]int64, 0, size)
	for i := 0; i < size; i++ {
		var _elem int64
		if v, err := iprot.ReadI64(); err != nil {
			return err
		} else {
			_elem = v
		}

		p.ListI64 = append(p.ListI64, _elem)
	}
	if err := iprot.ReadListEnd(); err != nil {
		return err
	}
	return nil
}

func (p *Nesting) ReadField14(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadByte(); err != nil {
		return err
	} else {
		p.Byte = v
	}
	return nil
}

func (p *Nesting) ReadField15(iprot thrift.TProtocol) error {
	_, _, size, err := iprot.ReadMapBegin()
	if err != nil {
		return err
	}
	p.MapStringSimple = make(map[string]*Simple, size)
	for i := 0; i < size; i++ {
		var _key string
		if v, err := iprot.ReadString(); err != nil {
			return err
		} else {
			_key = v
		}
		_val := NewSimple()
		if err := _val.Read(iprot); err != nil {
			return err
		}

		p.MapStringSimple[_key] = _val
	}
	if err := iprot.ReadMapEnd(); err != nil {
		return err
	}
	return nil
}

func (p *Nesting) Write(oprot thrift.TProtocol) (err error) {
	var fieldId int16
	if err = oprot.WriteStructBegin("Nesting"); err != nil {
		goto WriteStructBeginError
	}
	if p != nil {
		if err = p.writeField1(oprot); err != nil {
			fieldId = 1
			goto WriteFieldError
		}
		if err = p.writeField2(oprot); err != nil {
			fieldId = 2
			goto WriteFieldError
		}
		if err = p.writeField3(oprot); err != nil {
			fieldId = 3
			goto WriteFieldError
		}
		if err = p.writeField4(oprot); err != nil {
			fieldId = 4
			goto WriteFieldError
		}
		if err = p.writeField5(oprot); err != nil {
			fieldId = 5
			goto WriteFieldError
		}
		if err = p.writeField6(oprot); err != nil {
			fieldId = 6
			goto WriteFieldError
		}
		if err = p.writeField7(oprot); err != nil {
			fieldId = 7
			goto WriteFieldError
		}
		if err = p.writeField8(oprot); err != nil {
			fieldId = 8
			goto WriteFieldError
		}
		if err = p.writeField9(oprot); err != nil {
			fieldId = 9
			goto WriteFieldError
		}
		if err = p.writeField10(oprot); err != nil {
			fieldId = 10
			goto WriteFieldError
		}
		if err = p.writeField11(oprot); err != nil {
			fieldId = 11
			goto WriteFieldError
		}
		if err = p.writeField12(oprot); err != nil {
			fieldId = 12
			goto WriteFieldError
		}
		if err = p.writeField13(oprot); err != nil {
			fieldId = 13
			goto WriteFieldError
		}
		if err = p.writeField14(oprot); err != nil {
			fieldId = 14
			goto WriteFieldError
		}
		if err = p.writeField15(oprot); err != nil {
			fieldId = 15
			goto WriteFieldError
		}

	}
	if err = oprot.WriteFieldStop(); err != nil {
		goto WriteFieldStopError
	}
	if err = oprot.WriteStructEnd(); err != nil {
		goto WriteStructEndError
	}
	return nil
WriteStructBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write struct begin error: ", p), err)
WriteFieldError:
	return thrift.PrependError(fmt.Sprintf("%T write field %d error: ", p, fieldId), err)
WriteFieldStopError:
	return thrift.PrependError(fmt.Sprintf("%T write field stop error: ", p), err)
WriteStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T write struct end error: ", p), err)
}

func (p *Nesting) writeField1(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("String", thrift.STRING, 1); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteString(p.String_); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 1 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 1 end error: ", p), err)
}

func (p *Nesting) writeField2(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("ListSimple", thrift.LIST, 2); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteListBegin(thrift.STRUCT, len(p.ListSimple)); err != nil {
		return err
	}
	for _, v := range p.ListSimple {
		if err := v.Write(oprot); err != nil {
			return err
		}
	}
	if err := oprot.WriteListEnd(); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 2 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 2 end error: ", p), err)
}

func (p *Nesting) writeField3(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("Double", thrift.DOUBLE, 3); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteDouble(p.Double); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 3 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 3 end error: ", p), err)
}

func (p *Nesting) writeField4(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("I32", thrift.I32, 4); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteI32(p.I32); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 4 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 4 end error: ", p), err)
}

func (p *Nesting) writeField5(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("ListI32", thrift.LIST, 5); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteListBegin(thrift.I32, len(p.ListI32)); err != nil {
		return err
	}
	for _, v := range p.ListI32 {
		if err := oprot.WriteI32(v); err != nil {
			return err
		}
	}
	if err := oprot.WriteListEnd(); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 5 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 5 end error: ", p), err)
}

func (p *Nesting) writeField6(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("I64", thrift.I64, 6); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteI64(p.I64); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 6 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 6 end error: ", p), err)
}

func (p *Nesting) writeField7(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("MapStringString", thrift.MAP, 7); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteMapBegin(thrift.STRING, thrift.STRING, len(p.MapStringString)); err != nil {
		return err
	}
	for k, v := range p.MapStringString {

		if err := oprot.WriteString(k); err != nil {
			return err
		}

		if err := oprot.WriteString(v); err != nil {
			return err
		}
	}
	if err := oprot.WriteMapEnd(); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 7 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 7 end error: ", p), err)
}

func (p *Nesting) writeField8(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("SimpleStruct", thrift.STRUCT, 8); err != nil {
		goto WriteFieldBeginError
	}
	if err := p.SimpleStruct.Write(oprot); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 8 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 8 end error: ", p), err)
}

func (p *Nesting) writeField9(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("MapI32I64", thrift.MAP, 9); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteMapBegin(thrift.I32, thrift.I64, len(p.MapI32I64)); err != nil {
		return err
	}
	for k, v := range p.MapI32I64 {

		if err := oprot.WriteI32(k); err != nil {
			return err
		}

		if err := oprot.WriteI64(v); err != nil {
			return err
		}
	}
	if err := oprot.WriteMapEnd(); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 9 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 9 end error: ", p), err)
}

func (p *Nesting) writeField10(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("ListString", thrift.LIST, 10); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteListBegin(thrift.STRING, len(p.ListString)); err != nil {
		return err
	}
	for _, v := range p.ListString {
		if err := oprot.WriteString(v); err != nil {
			return err
		}
	}
	if err := oprot.WriteListEnd(); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 10 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 10 end error: ", p), err)
}

func (p *Nesting) writeField11(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("Binary", thrift.STRING, 11); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteBinary([]byte(p.Binary)); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 11 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 11 end error: ", p), err)
}

func (p *Nesting) writeField12(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("MapI64String", thrift.MAP, 12); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteMapBegin(thrift.I64, thrift.STRING, len(p.MapI64String)); err != nil {
		return err
	}
	for k, v := range p.MapI64String {

		if err := oprot.WriteI64(k); err != nil {
			return err
		}

		if err := oprot.WriteString(v); err != nil {
			return err
		}
	}
	if err := oprot.WriteMapEnd(); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 12 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 12 end error: ", p), err)
}

func (p *Nesting) writeField13(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("ListI64", thrift.LIST, 13); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteListBegin(thrift.I64, len(p.ListI64)); err != nil {
		return err
	}
	for _, v := range p.ListI64 {
		if err := oprot.WriteI64(v); err != nil {
			return err
		}
	}
	if err := oprot.WriteListEnd(); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 13 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 13 end error: ", p), err)
}

func (p *Nesting) writeField14(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("Byte", thrift.BYTE, 14); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteByte(p.Byte); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 14 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 14 end error: ", p), err)
}

func (p *Nesting) writeField15(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("MapStringSimple", thrift.MAP, 15); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteMapBegin(thrift.STRING, thrift.STRUCT, len(p.MapStringSimple)); err != nil {
		return err
	}
	for k, v := range p.MapStringSimple {

		if err := oprot.WriteString(k); err != nil {
			return err
		}

		if err := v.Write(oprot); err != nil {
			return err
		}
	}
	if err := oprot.WriteMapEnd(); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 15 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 15 end error: ", p), err)
}

func (p *Nesting) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("Nesting(%+v)", *p)
}

func (p *Nesting) DeepEqual(ano *Nesting) bool {
	if p == ano {
		return true
	} else if p == nil || ano == nil {
		return false
	}
	if !p.Field1DeepEqual(ano.String_) {
		return false
	}
	if !p.Field2DeepEqual(ano.ListSimple) {
		return false
	}
	if !p.Field3DeepEqual(ano.Double) {
		return false
	}
	if !p.Field4DeepEqual(ano.I32) {
		return false
	}
	if !p.Field5DeepEqual(ano.ListI32) {
		return false
	}
	if !p.Field6DeepEqual(ano.I64) {
		return false
	}
	if !p.Field7DeepEqual(ano.MapStringString) {
		return false
	}
	if !p.Field8DeepEqual(ano.SimpleStruct) {
		return false
	}
	if !p.Field9DeepEqual(ano.MapI32I64) {
		return false
	}
	if !p.Field10DeepEqual(ano.ListString) {
		return false
	}
	if !p.Field11DeepEqual(ano.Binary) {
		return false
	}
	if !p.Field12DeepEqual(ano.MapI64String) {
		return false
	}
	if !p.Field13DeepEqual(ano.ListI64) {
		return false
	}
	if !p.Field14DeepEqual(ano.Byte) {
		return false
	}
	if !p.Field15DeepEqual(ano.MapStringSimple) {
		return false
	}
	return true
}

func (p *Nesting) Field1DeepEqual(src string) bool {
	return strings.Compare(p.String_, src) == 0
}

func (p *Nesting) Field2DeepEqual(src []*Simple) bool {
	if len(p.ListSimple) != len(src) {
		return false
	}
	for i, v := range p.ListSimple {
		_src := src[i]
		if !v.DeepEqual(_src) {
			return false
		}
	}
	return true
}

func (p *Nesting) Field3DeepEqual(src float64) bool {
	return p.Double == src
}

func (p *Nesting) Field4DeepEqual(src int32) bool {
	return p.I32 == src
}

func (p *Nesting) Field5DeepEqual(src []int32) bool {
	if len(p.ListI32) != len(src) {
		return false
	}
	for i, v := range p.ListI32 {
		_src := src[i]
		if v != _src {
			return false
		}
	}
	return true
}

func (p *Nesting) Field6DeepEqual(src int64) bool {
	return p.I64 == src
}

func (p *Nesting) Field7DeepEqual(src map[string]string) bool {
	if len(p.MapStringString) != len(src) {
		return false
	}
	for k, v := range p.MapStringString {
		_src := src[k]
		if strings.Compare(v, _src) != 0 {
			return false
		}
	}
	return true
}

func (p *Nesting) Field8DeepEqual(src *Simple) bool {
	return p.SimpleStruct.DeepEqual(src)
}

func (p *Nesting) Field9DeepEqual(src map[int32]int64) bool {
	if len(p.MapI32I64) != len(src) {
		return false
	}
	for k, v := range p.MapI32I64 {
		_src := src[k]
		if v != _src {
			return false
		}
	}
	return true
}

func (p *Nesting) Field10DeepEqual(src []string) bool {
	if len(p.ListString) != len(src) {
		return false
	}
	for i, v := range p.ListString {
		_src := src[i]
		if strings.Compare(v, _src) != 0 {
			return false
		}
	}
	return true
}

func (p *Nesting) Field11DeepEqual(src []byte) bool {
	return bytes.Equal(p.Binary, src)
}

func (p *Nesting) Field12DeepEqual(src map[int64]string) bool {
	if len(p.MapI64String) != len(src) {
		return false
	}
	for k, v := range p.MapI64String {
		_src := src[k]
		if strings.Compare(v, _src) != 0 {
			return false
		}
	}
	return true
}

func (p *Nesting) Field13DeepEqual(src []int64) bool {
	if len(p.ListI64) != len(src) {
		return false
	}
	for i, v := range p.ListI64 {
		_src := src[i]
		if v != _src {
			return false
		}
	}
	return true
}

func (p *Nesting) Field14DeepEqual(src int8) bool {
	return p.Byte == src
}

func (p *Nesting) Field15DeepEqual(src map[string]*Simple) bool {
	if len(p.MapStringSimple) != len(src) {
		return false
	}
	for k, v := range p.MapStringSimple {
		_src := src[k]
		if !v.DeepEqual(_src) {
			return false
		}
	}
	return true
}

type BaselineService interface {
	SimpleMethod(ctx context.Context, req *Simple) (r *Simple, err error)

	NestingMethod(ctx context.Context, req *Nesting) (r *Nesting, err error)
}

type BaselineServiceClient struct {
	c thrift.TClient
}

func NewBaselineServiceClientFactory(t thrift.TTransport, f thrift.TProtocolFactory) *BaselineServiceClient {
	return &BaselineServiceClient{
		c: thrift.NewTStandardClient(f.GetProtocol(t), f.GetProtocol(t)),
	}
}

func NewBaselineServiceClientProtocol(t thrift.TTransport, iprot, oprot thrift.TProtocol) *BaselineServiceClient {
	return &BaselineServiceClient{
		c: thrift.NewTStandardClient(iprot, oprot),
	}
}

func NewBaselineServiceClient(c thrift.TClient) *BaselineServiceClient {
	return &BaselineServiceClient{
		c: c,
	}
}

func (p *BaselineServiceClient) Client_() thrift.TClient {
	return p.c
}

func (p *BaselineServiceClient) SimpleMethod(ctx context.Context, req *Simple) (r *Simple, err error) {
	var _args BaselineServiceSimpleMethodArgs
	_args.Req = req
	var _result BaselineServiceSimpleMethodResult
	if err = p.Client_().Call(ctx, "SimpleMethod", &_args, &_result); err != nil {
		return
	}
	return _result.GetSuccess(), nil
}

func (p *BaselineServiceClient) NestingMethod(ctx context.Context, req *Nesting) (r *Nesting, err error) {
	var _args BaselineServiceNestingMethodArgs
	_args.Req = req
	var _result BaselineServiceNestingMethodResult
	if err = p.Client_().Call(ctx, "NestingMethod", &_args, &_result); err != nil {
		return
	}
	return _result.GetSuccess(), nil
}

type BaselineServiceProcessor struct {
	processorMap map[string]thrift.TProcessorFunction
	handler      BaselineService
}

func (p *BaselineServiceProcessor) AddToProcessorMap(key string, processor thrift.TProcessorFunction) {
	p.processorMap[key] = processor
}

func (p *BaselineServiceProcessor) GetProcessorFunction(key string) (processor thrift.TProcessorFunction, ok bool) {
	processor, ok = p.processorMap[key]
	return processor, ok
}

func (p *BaselineServiceProcessor) ProcessorMap() map[string]thrift.TProcessorFunction {
	return p.processorMap
}

func NewBaselineServiceProcessor(handler BaselineService) *BaselineServiceProcessor {
	self := &BaselineServiceProcessor{handler: handler, processorMap: make(map[string]thrift.TProcessorFunction)}
	self.AddToProcessorMap("SimpleMethod", &baselineServiceProcessorSimpleMethod{handler: handler})
	self.AddToProcessorMap("NestingMethod", &baselineServiceProcessorNestingMethod{handler: handler})
	return self
}

func (p *BaselineServiceProcessor) Process(ctx context.Context, iprot, oprot thrift.TProtocol) (success bool, err thrift.TException) {
	name, _, seqId, err := iprot.ReadMessageBegin()
	if err != nil {
		return false, err
	}
	if processor, ok := p.GetProcessorFunction(name); ok {
		return processor.Process(ctx, seqId, iprot, oprot)
	}
	iprot.Skip(thrift.STRUCT)
	iprot.ReadMessageEnd()
	x := thrift.NewTApplicationException(thrift.UNKNOWN_METHOD, "Unknown function "+name)
	oprot.WriteMessageBegin(name, thrift.EXCEPTION, seqId)
	x.Write(oprot)
	oprot.WriteMessageEnd()
	oprot.Flush(ctx)
	return false, x
}

type baselineServiceProcessorSimpleMethod struct {
	handler BaselineService
}

func (p *baselineServiceProcessorSimpleMethod) Process(ctx context.Context, seqId int32, iprot, oprot thrift.TProtocol) (success bool, err thrift.TException) {
	args := BaselineServiceSimpleMethodArgs{}
	if err = args.Read(iprot); err != nil {
		iprot.ReadMessageEnd()
		x := thrift.NewTApplicationException(thrift.PROTOCOL_ERROR, err.Error())
		oprot.WriteMessageBegin("SimpleMethod", thrift.EXCEPTION, seqId)
		x.Write(oprot)
		oprot.WriteMessageEnd()
		oprot.Flush(ctx)
		return false, err
	}

	iprot.ReadMessageEnd()
	var err2 error
	result := BaselineServiceSimpleMethodResult{}
	var retval *Simple
	if retval, err2 = p.handler.SimpleMethod(ctx, args.Req); err2 != nil {
		x := thrift.NewTApplicationException(thrift.INTERNAL_ERROR, "Internal error processing SimpleMethod: "+err2.Error())
		oprot.WriteMessageBegin("SimpleMethod", thrift.EXCEPTION, seqId)
		x.Write(oprot)
		oprot.WriteMessageEnd()
		oprot.Flush(ctx)
		return true, err2
	} else {
		result.Success = retval
	}
	if err2 = oprot.WriteMessageBegin("SimpleMethod", thrift.REPLY, seqId); err2 != nil {
		err = err2
	}
	if err2 = result.Write(oprot); err == nil && err2 != nil {
		err = err2
	}
	if err2 = oprot.WriteMessageEnd(); err == nil && err2 != nil {
		err = err2
	}
	if err2 = oprot.Flush(ctx); err == nil && err2 != nil {
		err = err2
	}
	if err != nil {
		return
	}
	return true, err
}

type baselineServiceProcessorNestingMethod struct {
	handler BaselineService
}

func (p *baselineServiceProcessorNestingMethod) Process(ctx context.Context, seqId int32, iprot, oprot thrift.TProtocol) (success bool, err thrift.TException) {
	args := BaselineServiceNestingMethodArgs{}
	if err = args.Read(iprot); err != nil {
		iprot.ReadMessageEnd()
		x := thrift.NewTApplicationException(thrift.PROTOCOL_ERROR, err.Error())
		oprot.WriteMessageBegin("NestingMethod", thrift.EXCEPTION, seqId)
		x.Write(oprot)
		oprot.WriteMessageEnd()
		oprot.Flush(ctx)
		return false, err
	}

	iprot.ReadMessageEnd()
	var err2 error
	result := BaselineServiceNestingMethodResult{}
	var retval *Nesting
	if retval, err2 = p.handler.NestingMethod(ctx, args.Req); err2 != nil {
		x := thrift.NewTApplicationException(thrift.INTERNAL_ERROR, "Internal error processing NestingMethod: "+err2.Error())
		oprot.WriteMessageBegin("NestingMethod", thrift.EXCEPTION, seqId)
		x.Write(oprot)
		oprot.WriteMessageEnd()
		oprot.Flush(ctx)
		return true, err2
	} else {
		result.Success = retval
	}
	if err2 = oprot.WriteMessageBegin("NestingMethod", thrift.REPLY, seqId); err2 != nil {
		err = err2
	}
	if err2 = result.Write(oprot); err == nil && err2 != nil {
		err = err2
	}
	if err2 = oprot.WriteMessageEnd(); err == nil && err2 != nil {
		err = err2
	}
	if err2 = oprot.Flush(ctx); err == nil && err2 != nil {
		err = err2
	}
	if err != nil {
		return
	}
	return true, err
}

type BaselineServiceSimpleMethodArgs struct {
	Req *Simple `thrift:"req,1" json:"req"`
}

func NewBaselineServiceSimpleMethodArgs() *BaselineServiceSimpleMethodArgs {
	return &BaselineServiceSimpleMethodArgs{}
}

var BaselineServiceSimpleMethodArgs_Req_DEFAULT *Simple

func (p *BaselineServiceSimpleMethodArgs) GetReq() (v *Simple) {
	if !p.IsSetReq() {
		return BaselineServiceSimpleMethodArgs_Req_DEFAULT
	}
	return p.Req
}

func (p *BaselineServiceSimpleMethodArgs) SetReq(val *Simple) {
	p.Req = val
}

var fieldIDToName_BaselineServiceSimpleMethodArgs = map[int16]string{
	1: "req",
}

func (p *BaselineServiceSimpleMethodArgs) IsSetReq() bool {
	return p.Req != nil
}

func (p *BaselineServiceSimpleMethodArgs) Read(iprot thrift.TProtocol) (err error) {
	var fieldTypeId thrift.TType
	var fieldId int16

	if _, err = iprot.ReadStructBegin(); err != nil {
		goto ReadStructBeginError
	}

	for {
		_, fieldTypeId, fieldId, err = iprot.ReadFieldBegin()
		if err != nil {
			goto ReadFieldBeginError
		}
		if fieldTypeId == thrift.STOP {
			break
		}

		switch fieldId {
		case 1:
			if fieldTypeId == thrift.STRUCT {
				if err = p.ReadField1(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		default:
			if err = iprot.Skip(fieldTypeId); err != nil {
				goto SkipFieldError
			}
		}

		if err = iprot.ReadFieldEnd(); err != nil {
			goto ReadFieldEndError
		}
	}
	if err = iprot.ReadStructEnd(); err != nil {
		goto ReadStructEndError
	}

	return nil
ReadStructBeginError:
	return thrift.PrependError(fmt.Sprintf("%T read struct begin error: ", p), err)
ReadFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T read field %d begin error: ", p, fieldId), err)
ReadFieldError:
	return thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_BaselineServiceSimpleMethodArgs[fieldId]), err)
SkipFieldError:
	return thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)

ReadFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T read field end error", p), err)
ReadStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
}

func (p *BaselineServiceSimpleMethodArgs) ReadField1(iprot thrift.TProtocol) error {
	p.Req = NewSimple()
	if err := p.Req.Read(iprot); err != nil {
		return err
	}
	return nil
}

func (p *BaselineServiceSimpleMethodArgs) Write(oprot thrift.TProtocol) (err error) {
	var fieldId int16
	if err = oprot.WriteStructBegin("SimpleMethod_args"); err != nil {
		goto WriteStructBeginError
	}
	if p != nil {
		if err = p.writeField1(oprot); err != nil {
			fieldId = 1
			goto WriteFieldError
		}
	}
	if err = oprot.WriteFieldStop(); err != nil {
		goto WriteFieldStopError
	}
	if err = oprot.WriteStructEnd(); err != nil {
		goto WriteStructEndError
	}
	return nil
WriteStructBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write struct begin error: ", p), err)
WriteFieldError:
	return thrift.PrependError(fmt.Sprintf("%T write field %d error: ", p, fieldId), err)
WriteFieldStopError:
	return thrift.PrependError(fmt.Sprintf("%T write field stop error: ", p), err)
WriteStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T write struct end error: ", p), err)
}

func (p *BaselineServiceSimpleMethodArgs) writeField1(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("req", thrift.STRUCT, 1); err != nil {
		goto WriteFieldBeginError
	}
	if err := p.Req.Write(oprot); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 1 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 1 end error: ", p), err)
}

func (p *BaselineServiceSimpleMethodArgs) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("BaselineServiceSimpleMethodArgs(%+v)", *p)
}

func (p *BaselineServiceSimpleMethodArgs) DeepEqual(ano *BaselineServiceSimpleMethodArgs) bool {
	if p == ano {
		return true
	} else if p == nil || ano == nil {
		return false
	}
	if !p.Field1DeepEqual(ano.Req) {
		return false
	}
	return true
}

func (p *BaselineServiceSimpleMethodArgs) Field1DeepEqual(src *Simple) bool {
	return p.Req.DeepEqual(src)
}

type BaselineServiceSimpleMethodResult struct {
	Success *Simple `thrift:"success,0" json:"success,omitempty"`
}

func NewBaselineServiceSimpleMethodResult() *BaselineServiceSimpleMethodResult {
	return &BaselineServiceSimpleMethodResult{}
}

var BaselineServiceSimpleMethodResult_Success_DEFAULT *Simple

func (p *BaselineServiceSimpleMethodResult) GetSuccess() (v *Simple) {
	if !p.IsSetSuccess() {
		return BaselineServiceSimpleMethodResult_Success_DEFAULT
	}
	return p.Success
}

func (p *BaselineServiceSimpleMethodResult) SetSuccess(x interface{}) {
	p.Success = x.(*Simple)
}

var fieldIDToName_BaselineServiceSimpleMethodResult = map[int16]string{
	0: "success",
}

func (p *BaselineServiceSimpleMethodResult) IsSetSuccess() bool {
	return p.Success != nil
}

func (p *BaselineServiceSimpleMethodResult) Read(iprot thrift.TProtocol) (err error) {
	var fieldTypeId thrift.TType
	var fieldId int16

	if _, err = iprot.ReadStructBegin(); err != nil {
		goto ReadStructBeginError
	}

	for {
		_, fieldTypeId, fieldId, err = iprot.ReadFieldBegin()
		if err != nil {
			goto ReadFieldBeginError
		}
		if fieldTypeId == thrift.STOP {
			break
		}

		switch fieldId {
		case 0:
			if fieldTypeId == thrift.STRUCT {
				if err = p.ReadField0(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		default:
			if err = iprot.Skip(fieldTypeId); err != nil {
				goto SkipFieldError
			}
		}

		if err = iprot.ReadFieldEnd(); err != nil {
			goto ReadFieldEndError
		}
	}
	if err = iprot.ReadStructEnd(); err != nil {
		goto ReadStructEndError
	}

	return nil
ReadStructBeginError:
	return thrift.PrependError(fmt.Sprintf("%T read struct begin error: ", p), err)
ReadFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T read field %d begin error: ", p, fieldId), err)
ReadFieldError:
	return thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_BaselineServiceSimpleMethodResult[fieldId]), err)
SkipFieldError:
	return thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)

ReadFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T read field end error", p), err)
ReadStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
}

func (p *BaselineServiceSimpleMethodResult) ReadField0(iprot thrift.TProtocol) error {
	p.Success = NewSimple()
	if err := p.Success.Read(iprot); err != nil {
		return err
	}
	return nil
}

func (p *BaselineServiceSimpleMethodResult) Write(oprot thrift.TProtocol) (err error) {
	var fieldId int16
	if err = oprot.WriteStructBegin("SimpleMethod_result"); err != nil {
		goto WriteStructBeginError
	}
	if p != nil {
		if err = p.writeField0(oprot); err != nil {
			fieldId = 0
			goto WriteFieldError
		}
	}
	if err = oprot.WriteFieldStop(); err != nil {
		goto WriteFieldStopError
	}
	if err = oprot.WriteStructEnd(); err != nil {
		goto WriteStructEndError
	}
	return nil
WriteStructBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write struct begin error: ", p), err)
WriteFieldError:
	return thrift.PrependError(fmt.Sprintf("%T write field %d error: ", p, fieldId), err)
WriteFieldStopError:
	return thrift.PrependError(fmt.Sprintf("%T write field stop error: ", p), err)
WriteStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T write struct end error: ", p), err)
}

func (p *BaselineServiceSimpleMethodResult) writeField0(oprot thrift.TProtocol) (err error) {
	if p.IsSetSuccess() {
		if err = oprot.WriteFieldBegin("success", thrift.STRUCT, 0); err != nil {
			goto WriteFieldBeginError
		}
		if err := p.Success.Write(oprot); err != nil {
			return err
		}
		if err = oprot.WriteFieldEnd(); err != nil {
			goto WriteFieldEndError
		}
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 0 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 0 end error: ", p), err)
}

func (p *BaselineServiceSimpleMethodResult) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("BaselineServiceSimpleMethodResult(%+v)", *p)
}

func (p *BaselineServiceSimpleMethodResult) DeepEqual(ano *BaselineServiceSimpleMethodResult) bool {
	if p == ano {
		return true
	} else if p == nil || ano == nil {
		return false
	}
	if !p.Field0DeepEqual(ano.Success) {
		return false
	}
	return true
}

func (p *BaselineServiceSimpleMethodResult) Field0DeepEqual(src *Simple) bool {
	return p.Success.DeepEqual(src)
}

type BaselineServiceNestingMethodArgs struct {
	Req *Nesting `thrift:"req,1" json:"req"`
}

func NewBaselineServiceNestingMethodArgs() *BaselineServiceNestingMethodArgs {
	return &BaselineServiceNestingMethodArgs{}
}

var BaselineServiceNestingMethodArgs_Req_DEFAULT *Nesting

func (p *BaselineServiceNestingMethodArgs) GetReq() (v *Nesting) {
	if !p.IsSetReq() {
		return BaselineServiceNestingMethodArgs_Req_DEFAULT
	}
	return p.Req
}

func (p *BaselineServiceNestingMethodArgs) SetReq(val *Nesting) {
	p.Req = val
}

var fieldIDToName_BaselineServiceNestingMethodArgs = map[int16]string{
	1: "req",
}

func (p *BaselineServiceNestingMethodArgs) IsSetReq() bool {
	return p.Req != nil
}

func (p *BaselineServiceNestingMethodArgs) Read(iprot thrift.TProtocol) (err error) {
	var fieldTypeId thrift.TType
	var fieldId int16

	if _, err = iprot.ReadStructBegin(); err != nil {
		goto ReadStructBeginError
	}

	for {
		_, fieldTypeId, fieldId, err = iprot.ReadFieldBegin()
		if err != nil {
			goto ReadFieldBeginError
		}
		if fieldTypeId == thrift.STOP {
			break
		}

		switch fieldId {
		case 1:
			if fieldTypeId == thrift.STRUCT {
				if err = p.ReadField1(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		default:
			if err = iprot.Skip(fieldTypeId); err != nil {
				goto SkipFieldError
			}
		}

		if err = iprot.ReadFieldEnd(); err != nil {
			goto ReadFieldEndError
		}
	}
	if err = iprot.ReadStructEnd(); err != nil {
		goto ReadStructEndError
	}

	return nil
ReadStructBeginError:
	return thrift.PrependError(fmt.Sprintf("%T read struct begin error: ", p), err)
ReadFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T read field %d begin error: ", p, fieldId), err)
ReadFieldError:
	return thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_BaselineServiceNestingMethodArgs[fieldId]), err)
SkipFieldError:
	return thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)

ReadFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T read field end error", p), err)
ReadStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
}

func (p *BaselineServiceNestingMethodArgs) ReadField1(iprot thrift.TProtocol) error {
	p.Req = NewNesting()
	if err := p.Req.Read(iprot); err != nil {
		return err
	}
	return nil
}

func (p *BaselineServiceNestingMethodArgs) Write(oprot thrift.TProtocol) (err error) {
	var fieldId int16
	if err = oprot.WriteStructBegin("NestingMethod_args"); err != nil {
		goto WriteStructBeginError
	}
	if p != nil {
		if err = p.writeField1(oprot); err != nil {
			fieldId = 1
			goto WriteFieldError
		}
	}
	if err = oprot.WriteFieldStop(); err != nil {
		goto WriteFieldStopError
	}
	if err = oprot.WriteStructEnd(); err != nil {
		goto WriteStructEndError
	}
	return nil
WriteStructBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write struct begin error: ", p), err)
WriteFieldError:
	return thrift.PrependError(fmt.Sprintf("%T write field %d error: ", p, fieldId), err)
WriteFieldStopError:
	return thrift.PrependError(fmt.Sprintf("%T write field stop error: ", p), err)
WriteStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T write struct end error: ", p), err)
}

func (p *BaselineServiceNestingMethodArgs) writeField1(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("req", thrift.STRUCT, 1); err != nil {
		goto WriteFieldBeginError
	}
	if err := p.Req.Write(oprot); err != nil {
		return err
	}
	if err = oprot.WriteFieldEnd(); err != nil {
		goto WriteFieldEndError
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 1 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 1 end error: ", p), err)
}

func (p *BaselineServiceNestingMethodArgs) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("BaselineServiceNestingMethodArgs(%+v)", *p)
}

func (p *BaselineServiceNestingMethodArgs) DeepEqual(ano *BaselineServiceNestingMethodArgs) bool {
	if p == ano {
		return true
	} else if p == nil || ano == nil {
		return false
	}
	if !p.Field1DeepEqual(ano.Req) {
		return false
	}
	return true
}

func (p *BaselineServiceNestingMethodArgs) Field1DeepEqual(src *Nesting) bool {
	return p.Req.DeepEqual(src)
}

type BaselineServiceNestingMethodResult struct {
	Success *Nesting `thrift:"success,0" json:"success,omitempty"`
}

func NewBaselineServiceNestingMethodResult() *BaselineServiceNestingMethodResult {
	return &BaselineServiceNestingMethodResult{}
}

var BaselineServiceNestingMethodResult_Success_DEFAULT *Nesting

func (p *BaselineServiceNestingMethodResult) GetSuccess() (v *Nesting) {
	if !p.IsSetSuccess() {
		return BaselineServiceNestingMethodResult_Success_DEFAULT
	}
	return p.Success
}

func (p *BaselineServiceNestingMethodResult) SetSuccess(x interface{}) {
	p.Success = x.(*Nesting)
}

var fieldIDToName_BaselineServiceNestingMethodResult = map[int16]string{
	0: "success",
}

func (p *BaselineServiceNestingMethodResult) IsSetSuccess() bool {
	return p.Success != nil
}

func (p *BaselineServiceNestingMethodResult) Read(iprot thrift.TProtocol) (err error) {
	var fieldTypeId thrift.TType
	var fieldId int16

	if _, err = iprot.ReadStructBegin(); err != nil {
		goto ReadStructBeginError
	}

	for {
		_, fieldTypeId, fieldId, err = iprot.ReadFieldBegin()
		if err != nil {
			goto ReadFieldBeginError
		}
		if fieldTypeId == thrift.STOP {
			break
		}

		switch fieldId {
		case 0:
			if fieldTypeId == thrift.STRUCT {
				if err = p.ReadField0(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		default:
			if err = iprot.Skip(fieldTypeId); err != nil {
				goto SkipFieldError
			}
		}

		if err = iprot.ReadFieldEnd(); err != nil {
			goto ReadFieldEndError
		}
	}
	if err = iprot.ReadStructEnd(); err != nil {
		goto ReadStructEndError
	}

	return nil
ReadStructBeginError:
	return thrift.PrependError(fmt.Sprintf("%T read struct begin error: ", p), err)
ReadFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T read field %d begin error: ", p, fieldId), err)
ReadFieldError:
	return thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_BaselineServiceNestingMethodResult[fieldId]), err)
SkipFieldError:
	return thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)

ReadFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T read field end error", p), err)
ReadStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
}

func (p *BaselineServiceNestingMethodResult) ReadField0(iprot thrift.TProtocol) error {
	p.Success = NewNesting()
	if err := p.Success.Read(iprot); err != nil {
		return err
	}
	return nil
}

func (p *BaselineServiceNestingMethodResult) Write(oprot thrift.TProtocol) (err error) {
	var fieldId int16
	if err = oprot.WriteStructBegin("NestingMethod_result"); err != nil {
		goto WriteStructBeginError
	}
	if p != nil {
		if err = p.writeField0(oprot); err != nil {
			fieldId = 0
			goto WriteFieldError
		}
	}
	if err = oprot.WriteFieldStop(); err != nil {
		goto WriteFieldStopError
	}
	if err = oprot.WriteStructEnd(); err != nil {
		goto WriteStructEndError
	}
	return nil
WriteStructBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write struct begin error: ", p), err)
WriteFieldError:
	return thrift.PrependError(fmt.Sprintf("%T write field %d error: ", p, fieldId), err)
WriteFieldStopError:
	return thrift.PrependError(fmt.Sprintf("%T write field stop error: ", p), err)
WriteStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T write struct end error: ", p), err)
}

func (p *BaselineServiceNestingMethodResult) writeField0(oprot thrift.TProtocol) (err error) {
	if p.IsSetSuccess() {
		if err = oprot.WriteFieldBegin("success", thrift.STRUCT, 0); err != nil {
			goto WriteFieldBeginError
		}
		if err := p.Success.Write(oprot); err != nil {
			return err
		}
		if err = oprot.WriteFieldEnd(); err != nil {
			goto WriteFieldEndError
		}
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 0 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 0 end error: ", p), err)
}

func (p *BaselineServiceNestingMethodResult) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("BaselineServiceNestingMethodResult(%+v)", *p)
}

func (p *BaselineServiceNestingMethodResult) DeepEqual(ano *BaselineServiceNestingMethodResult) bool {
	if p == ano {
		return true
	} else if p == nil || ano == nil {
		return false
	}
	if !p.Field0DeepEqual(ano.Success) {
		return false
	}
	return true
}

func (p *BaselineServiceNestingMethodResult) Field0DeepEqual(src *Nesting) bool {
	return p.Success.DeepEqual(src)
}
