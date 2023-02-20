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

	if p.ByteField != src {
		return false
	}
	return true
}
func (p *Simple) Field2DeepEqual(src int64) bool {

	if p.I64Field != src {
		return false
	}
	return true
}
func (p *Simple) Field3DeepEqual(src float64) bool {

	if p.DoubleField != src {
		return false
	}
	return true
}
func (p *Simple) Field4DeepEqual(src int32) bool {

	if p.I32Field != src {
		return false
	}
	return true
}
func (p *Simple) Field5DeepEqual(src string) bool {

	if strings.Compare(p.StringField, src) != 0 {
		return false
	}
	return true
}
func (p *Simple) Field6DeepEqual(src []byte) bool {

	if bytes.Compare(p.BinaryField, src) != 0 {
		return false
	}
	return true
}

type PartialSimple struct {
	ByteField   int8    `thrift:"ByteField,1" json:"ByteField"`
	DoubleField float64 `thrift:"DoubleField,3" json:"DoubleField"`
	BinaryField []byte  `thrift:"BinaryField,6" json:"BinaryField"`
}

func NewPartialSimple() *PartialSimple {
	return &PartialSimple{}
}

func (p *PartialSimple) GetByteField() (v int8) {
	return p.ByteField
}

func (p *PartialSimple) GetDoubleField() (v float64) {
	return p.DoubleField
}

func (p *PartialSimple) GetBinaryField() (v []byte) {
	return p.BinaryField
}
func (p *PartialSimple) SetByteField(val int8) {
	p.ByteField = val
}
func (p *PartialSimple) SetDoubleField(val float64) {
	p.DoubleField = val
}
func (p *PartialSimple) SetBinaryField(val []byte) {
	p.BinaryField = val
}

var fieldIDToName_PartialSimple = map[int16]string{
	1: "ByteField",
	3: "DoubleField",
	6: "BinaryField",
}

func (p *PartialSimple) Read(iprot thrift.TProtocol) (err error) {

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
	return thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_PartialSimple[fieldId]), err)
SkipFieldError:
	return thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)

ReadFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T read field end error", p), err)
ReadStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
}

func (p *PartialSimple) ReadField1(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadByte(); err != nil {
		return err
	} else {
		p.ByteField = v
	}
	return nil
}

func (p *PartialSimple) ReadField3(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadDouble(); err != nil {
		return err
	} else {
		p.DoubleField = v
	}
	return nil
}

func (p *PartialSimple) ReadField6(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadBinary(); err != nil {
		return err
	} else {
		p.BinaryField = []byte(v)
	}
	return nil
}

func (p *PartialSimple) Write(oprot thrift.TProtocol) (err error) {
	var fieldId int16
	if err = oprot.WriteStructBegin("PartialSimple"); err != nil {
		goto WriteStructBeginError
	}
	if p != nil {
		if err = p.writeField1(oprot); err != nil {
			fieldId = 1
			goto WriteFieldError
		}
		if err = p.writeField3(oprot); err != nil {
			fieldId = 3
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

func (p *PartialSimple) writeField1(oprot thrift.TProtocol) (err error) {
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

func (p *PartialSimple) writeField3(oprot thrift.TProtocol) (err error) {
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

func (p *PartialSimple) writeField6(oprot thrift.TProtocol) (err error) {
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

func (p *PartialSimple) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("PartialSimple(%+v)", *p)
}

func (p *PartialSimple) DeepEqual(ano *PartialSimple) bool {
	if p == ano {
		return true
	} else if p == nil || ano == nil {
		return false
	}
	if !p.Field1DeepEqual(ano.ByteField) {
		return false
	}
	if !p.Field3DeepEqual(ano.DoubleField) {
		return false
	}
	if !p.Field6DeepEqual(ano.BinaryField) {
		return false
	}
	return true
}

func (p *PartialSimple) Field1DeepEqual(src int8) bool {

	if p.ByteField != src {
		return false
	}
	return true
}
func (p *PartialSimple) Field3DeepEqual(src float64) bool {

	if p.DoubleField != src {
		return false
	}
	return true
}
func (p *PartialSimple) Field6DeepEqual(src []byte) bool {

	if bytes.Compare(p.BinaryField, src) != 0 {
		return false
	}
	return true
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

	if strings.Compare(p.String_, src) != 0 {
		return false
	}
	return true
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

	if p.Double != src {
		return false
	}
	return true
}
func (p *Nesting) Field4DeepEqual(src int32) bool {

	if p.I32 != src {
		return false
	}
	return true
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

	if p.I64 != src {
		return false
	}
	return true
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

	if !p.SimpleStruct.DeepEqual(src) {
		return false
	}
	return true
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

	if bytes.Compare(p.Binary, src) != 0 {
		return false
	}
	return true
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

	if p.Byte != src {
		return false
	}
	return true
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

type PartialNesting struct {
	ListSimple      []*PartialSimple          `thrift:"ListSimple,2" json:"ListSimple"`
	SimpleStruct    *PartialSimple            `thrift:"SimpleStruct,8" json:"SimpleStruct"`
	MapStringSimple map[string]*PartialSimple `thrift:"MapStringSimple,15" json:"MapStringSimple"`
}

func NewPartialNesting() *PartialNesting {
	return &PartialNesting{}
}

func (p *PartialNesting) GetListSimple() (v []*PartialSimple) {
	return p.ListSimple
}

var PartialNesting_SimpleStruct_DEFAULT *PartialSimple

func (p *PartialNesting) GetSimpleStruct() (v *PartialSimple) {
	if !p.IsSetSimpleStruct() {
		return PartialNesting_SimpleStruct_DEFAULT
	}
	return p.SimpleStruct
}

func (p *PartialNesting) GetMapStringSimple() (v map[string]*PartialSimple) {
	return p.MapStringSimple
}
func (p *PartialNesting) SetListSimple(val []*PartialSimple) {
	p.ListSimple = val
}
func (p *PartialNesting) SetSimpleStruct(val *PartialSimple) {
	p.SimpleStruct = val
}
func (p *PartialNesting) SetMapStringSimple(val map[string]*PartialSimple) {
	p.MapStringSimple = val
}

var fieldIDToName_PartialNesting = map[int16]string{
	2:  "ListSimple",
	8:  "SimpleStruct",
	15: "MapStringSimple",
}

func (p *PartialNesting) IsSetSimpleStruct() bool {
	return p.SimpleStruct != nil
}

func (p *PartialNesting) Read(iprot thrift.TProtocol) (err error) {

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
	return thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_PartialNesting[fieldId]), err)
SkipFieldError:
	return thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)

ReadFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T read field end error", p), err)
ReadStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
}

func (p *PartialNesting) ReadField2(iprot thrift.TProtocol) error {
	_, size, err := iprot.ReadListBegin()
	if err != nil {
		return err
	}
	p.ListSimple = make([]*PartialSimple, 0, size)
	for i := 0; i < size; i++ {
		_elem := NewPartialSimple()
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

func (p *PartialNesting) ReadField8(iprot thrift.TProtocol) error {
	p.SimpleStruct = NewPartialSimple()
	if err := p.SimpleStruct.Read(iprot); err != nil {
		return err
	}
	return nil
}

func (p *PartialNesting) ReadField15(iprot thrift.TProtocol) error {
	_, _, size, err := iprot.ReadMapBegin()
	if err != nil {
		return err
	}
	p.MapStringSimple = make(map[string]*PartialSimple, size)
	for i := 0; i < size; i++ {
		var _key string
		if v, err := iprot.ReadString(); err != nil {
			return err
		} else {
			_key = v
		}
		_val := NewPartialSimple()
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

func (p *PartialNesting) Write(oprot thrift.TProtocol) (err error) {
	var fieldId int16
	if err = oprot.WriteStructBegin("PartialNesting"); err != nil {
		goto WriteStructBeginError
	}
	if p != nil {
		if err = p.writeField2(oprot); err != nil {
			fieldId = 2
			goto WriteFieldError
		}
		if err = p.writeField8(oprot); err != nil {
			fieldId = 8
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

func (p *PartialNesting) writeField2(oprot thrift.TProtocol) (err error) {
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

func (p *PartialNesting) writeField8(oprot thrift.TProtocol) (err error) {
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

func (p *PartialNesting) writeField15(oprot thrift.TProtocol) (err error) {
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

func (p *PartialNesting) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("PartialNesting(%+v)", *p)
}

func (p *PartialNesting) DeepEqual(ano *PartialNesting) bool {
	if p == ano {
		return true
	} else if p == nil || ano == nil {
		return false
	}
	if !p.Field2DeepEqual(ano.ListSimple) {
		return false
	}
	if !p.Field8DeepEqual(ano.SimpleStruct) {
		return false
	}
	if !p.Field15DeepEqual(ano.MapStringSimple) {
		return false
	}
	return true
}

func (p *PartialNesting) Field2DeepEqual(src []*PartialSimple) bool {

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
func (p *PartialNesting) Field8DeepEqual(src *PartialSimple) bool {

	if !p.SimpleStruct.DeepEqual(src) {
		return false
	}
	return true
}
func (p *PartialNesting) Field15DeepEqual(src map[string]*PartialSimple) bool {

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

type Nesting2 struct {
	MapSimpleNesting map[*Simple]*Nesting `thrift:"MapSimpleNesting,1" json:"MapSimpleNesting"`
	SimpleStruct     *Simple              `thrift:"SimpleStruct,2" json:"SimpleStruct"`
	Byte             int8                 `thrift:"Byte,3" json:"Byte"`
	Double           float64              `thrift:"Double,4" json:"Double"`
	ListNesting      []*Nesting           `thrift:"ListNesting,5" json:"ListNesting"`
	I64              int64                `thrift:"I64,6" json:"I64"`
	NestingStruct    *Nesting             `thrift:"NestingStruct,7" json:"NestingStruct"`
	Binary           []byte               `thrift:"Binary,8" json:"Binary"`
	String_          string               `thrift:"String,9" json:"String"`
	SetNesting       []*Nesting           `thrift:"SetNesting,10" json:"SetNesting"`
	I32              int32                `thrift:"I32,11" json:"I32"`
}

func NewNesting2() *Nesting2 {
	return &Nesting2{}
}

func (p *Nesting2) GetMapSimpleNesting() (v map[*Simple]*Nesting) {
	return p.MapSimpleNesting
}

var Nesting2_SimpleStruct_DEFAULT *Simple

func (p *Nesting2) GetSimpleStruct() (v *Simple) {
	if !p.IsSetSimpleStruct() {
		return Nesting2_SimpleStruct_DEFAULT
	}
	return p.SimpleStruct
}

func (p *Nesting2) GetByte() (v int8) {
	return p.Byte
}

func (p *Nesting2) GetDouble() (v float64) {
	return p.Double
}

func (p *Nesting2) GetListNesting() (v []*Nesting) {
	return p.ListNesting
}

func (p *Nesting2) GetI64() (v int64) {
	return p.I64
}

var Nesting2_NestingStruct_DEFAULT *Nesting

func (p *Nesting2) GetNestingStruct() (v *Nesting) {
	if !p.IsSetNestingStruct() {
		return Nesting2_NestingStruct_DEFAULT
	}
	return p.NestingStruct
}

func (p *Nesting2) GetBinary() (v []byte) {
	return p.Binary
}

func (p *Nesting2) GetString() (v string) {
	return p.String_
}

func (p *Nesting2) GetSetNesting() (v []*Nesting) {
	return p.SetNesting
}

func (p *Nesting2) GetI32() (v int32) {
	return p.I32
}
func (p *Nesting2) SetMapSimpleNesting(val map[*Simple]*Nesting) {
	p.MapSimpleNesting = val
}
func (p *Nesting2) SetSimpleStruct(val *Simple) {
	p.SimpleStruct = val
}
func (p *Nesting2) SetByte(val int8) {
	p.Byte = val
}
func (p *Nesting2) SetDouble(val float64) {
	p.Double = val
}
func (p *Nesting2) SetListNesting(val []*Nesting) {
	p.ListNesting = val
}
func (p *Nesting2) SetI64(val int64) {
	p.I64 = val
}
func (p *Nesting2) SetNestingStruct(val *Nesting) {
	p.NestingStruct = val
}
func (p *Nesting2) SetBinary(val []byte) {
	p.Binary = val
}
func (p *Nesting2) SetString(val string) {
	p.String_ = val
}
func (p *Nesting2) SetSetNesting(val []*Nesting) {
	p.SetNesting = val
}
func (p *Nesting2) SetI32(val int32) {
	p.I32 = val
}

var fieldIDToName_Nesting2 = map[int16]string{
	1:  "MapSimpleNesting",
	2:  "SimpleStruct",
	3:  "Byte",
	4:  "Double",
	5:  "ListNesting",
	6:  "I64",
	7:  "NestingStruct",
	8:  "Binary",
	9:  "String",
	10: "SetNesting",
	11: "I32",
}

func (p *Nesting2) IsSetSimpleStruct() bool {
	return p.SimpleStruct != nil
}

func (p *Nesting2) IsSetNestingStruct() bool {
	return p.NestingStruct != nil
}

func (p *Nesting2) Read(iprot thrift.TProtocol) (err error) {

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
			if fieldTypeId == thrift.MAP {
				if err = p.ReadField1(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 2:
			if fieldTypeId == thrift.STRUCT {
				if err = p.ReadField2(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 3:
			if fieldTypeId == thrift.BYTE {
				if err = p.ReadField3(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 4:
			if fieldTypeId == thrift.DOUBLE {
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
			if fieldTypeId == thrift.STRUCT {
				if err = p.ReadField7(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 8:
			if fieldTypeId == thrift.STRING {
				if err = p.ReadField8(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 9:
			if fieldTypeId == thrift.STRING {
				if err = p.ReadField9(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 10:
			if fieldTypeId == thrift.SET {
				if err = p.ReadField10(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 11:
			if fieldTypeId == thrift.I32 {
				if err = p.ReadField11(iprot); err != nil {
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
	return thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_Nesting2[fieldId]), err)
SkipFieldError:
	return thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)

ReadFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T read field end error", p), err)
ReadStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
}

func (p *Nesting2) ReadField1(iprot thrift.TProtocol) error {
	_, _, size, err := iprot.ReadMapBegin()
	if err != nil {
		return err
	}
	p.MapSimpleNesting = make(map[*Simple]*Nesting, size)
	for i := 0; i < size; i++ {
		_key := NewSimple()
		if err := _key.Read(iprot); err != nil {
			return err
		}
		_val := NewNesting()
		if err := _val.Read(iprot); err != nil {
			return err
		}

		p.MapSimpleNesting[_key] = _val
	}
	if err := iprot.ReadMapEnd(); err != nil {
		return err
	}
	return nil
}

func (p *Nesting2) ReadField2(iprot thrift.TProtocol) error {
	p.SimpleStruct = NewSimple()
	if err := p.SimpleStruct.Read(iprot); err != nil {
		return err
	}
	return nil
}

func (p *Nesting2) ReadField3(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadByte(); err != nil {
		return err
	} else {
		p.Byte = v
	}
	return nil
}

func (p *Nesting2) ReadField4(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadDouble(); err != nil {
		return err
	} else {
		p.Double = v
	}
	return nil
}

func (p *Nesting2) ReadField5(iprot thrift.TProtocol) error {
	_, size, err := iprot.ReadListBegin()
	if err != nil {
		return err
	}
	p.ListNesting = make([]*Nesting, 0, size)
	for i := 0; i < size; i++ {
		_elem := NewNesting()
		if err := _elem.Read(iprot); err != nil {
			return err
		}

		p.ListNesting = append(p.ListNesting, _elem)
	}
	if err := iprot.ReadListEnd(); err != nil {
		return err
	}
	return nil
}

func (p *Nesting2) ReadField6(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadI64(); err != nil {
		return err
	} else {
		p.I64 = v
	}
	return nil
}

func (p *Nesting2) ReadField7(iprot thrift.TProtocol) error {
	p.NestingStruct = NewNesting()
	if err := p.NestingStruct.Read(iprot); err != nil {
		return err
	}
	return nil
}

func (p *Nesting2) ReadField8(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadBinary(); err != nil {
		return err
	} else {
		p.Binary = []byte(v)
	}
	return nil
}

func (p *Nesting2) ReadField9(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadString(); err != nil {
		return err
	} else {
		p.String_ = v
	}
	return nil
}

func (p *Nesting2) ReadField10(iprot thrift.TProtocol) error {
	_, size, err := iprot.ReadSetBegin()
	if err != nil {
		return err
	}
	p.SetNesting = make([]*Nesting, 0, size)
	for i := 0; i < size; i++ {
		_elem := NewNesting()
		if err := _elem.Read(iprot); err != nil {
			return err
		}

		p.SetNesting = append(p.SetNesting, _elem)
	}
	if err := iprot.ReadSetEnd(); err != nil {
		return err
	}
	return nil
}

func (p *Nesting2) ReadField11(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadI32(); err != nil {
		return err
	} else {
		p.I32 = v
	}
	return nil
}

func (p *Nesting2) Write(oprot thrift.TProtocol) (err error) {
	var fieldId int16
	if err = oprot.WriteStructBegin("Nesting2"); err != nil {
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

func (p *Nesting2) writeField1(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("MapSimpleNesting", thrift.MAP, 1); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteMapBegin(thrift.STRUCT, thrift.STRUCT, len(p.MapSimpleNesting)); err != nil {
		return err
	}
	for k, v := range p.MapSimpleNesting {

		if err := k.Write(oprot); err != nil {
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
	return thrift.PrependError(fmt.Sprintf("%T write field 1 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 1 end error: ", p), err)
}

func (p *Nesting2) writeField2(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("SimpleStruct", thrift.STRUCT, 2); err != nil {
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
	return thrift.PrependError(fmt.Sprintf("%T write field 2 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 2 end error: ", p), err)
}

func (p *Nesting2) writeField3(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("Byte", thrift.BYTE, 3); err != nil {
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
	return thrift.PrependError(fmt.Sprintf("%T write field 3 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 3 end error: ", p), err)
}

func (p *Nesting2) writeField4(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("Double", thrift.DOUBLE, 4); err != nil {
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
	return thrift.PrependError(fmt.Sprintf("%T write field 4 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 4 end error: ", p), err)
}

func (p *Nesting2) writeField5(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("ListNesting", thrift.LIST, 5); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteListBegin(thrift.STRUCT, len(p.ListNesting)); err != nil {
		return err
	}
	for _, v := range p.ListNesting {
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
	return thrift.PrependError(fmt.Sprintf("%T write field 5 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 5 end error: ", p), err)
}

func (p *Nesting2) writeField6(oprot thrift.TProtocol) (err error) {
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

func (p *Nesting2) writeField7(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("NestingStruct", thrift.STRUCT, 7); err != nil {
		goto WriteFieldBeginError
	}
	if err := p.NestingStruct.Write(oprot); err != nil {
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

func (p *Nesting2) writeField8(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("Binary", thrift.STRING, 8); err != nil {
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
	return thrift.PrependError(fmt.Sprintf("%T write field 8 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 8 end error: ", p), err)
}

func (p *Nesting2) writeField9(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("String", thrift.STRING, 9); err != nil {
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
	return thrift.PrependError(fmt.Sprintf("%T write field 9 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 9 end error: ", p), err)
}

func (p *Nesting2) writeField10(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("SetNesting", thrift.SET, 10); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteSetBegin(thrift.STRUCT, len(p.SetNesting)); err != nil {
		return err
	}
	for i := 0; i < len(p.SetNesting); i++ {
		for j := i + 1; j < len(p.SetNesting); j++ {
			if func(tgt, src *Nesting) bool {
				if !tgt.DeepEqual(src) {
					return false
				}
				return true
			}(p.SetNesting[i], p.SetNesting[j]) {
				return thrift.PrependError("", fmt.Errorf("%T error writing set field: slice is not unique", p.SetNesting[i]))
			}
		}
	}
	for _, v := range p.SetNesting {
		if err := v.Write(oprot); err != nil {
			return err
		}
	}
	if err := oprot.WriteSetEnd(); err != nil {
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

func (p *Nesting2) writeField11(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("I32", thrift.I32, 11); err != nil {
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
	return thrift.PrependError(fmt.Sprintf("%T write field 11 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 11 end error: ", p), err)
}

func (p *Nesting2) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("Nesting2(%+v)", *p)
}

func (p *Nesting2) DeepEqual(ano *Nesting2) bool {
	if p == ano {
		return true
	} else if p == nil || ano == nil {
		return false
	}
	if !p.Field1DeepEqual(ano.MapSimpleNesting) {
		return false
	}
	if !p.Field2DeepEqual(ano.SimpleStruct) {
		return false
	}
	if !p.Field3DeepEqual(ano.Byte) {
		return false
	}
	if !p.Field4DeepEqual(ano.Double) {
		return false
	}
	if !p.Field5DeepEqual(ano.ListNesting) {
		return false
	}
	if !p.Field6DeepEqual(ano.I64) {
		return false
	}
	if !p.Field7DeepEqual(ano.NestingStruct) {
		return false
	}
	if !p.Field8DeepEqual(ano.Binary) {
		return false
	}
	if !p.Field9DeepEqual(ano.String_) {
		return false
	}
	if !p.Field10DeepEqual(ano.SetNesting) {
		return false
	}
	if !p.Field11DeepEqual(ano.I32) {
		return false
	}
	return true
}

func (p *Nesting2) Field1DeepEqual(src map[*Simple]*Nesting) bool {

	if len(p.MapSimpleNesting) != len(src) {
		return false
	}
	for k, v := range p.MapSimpleNesting {
		_src := src[k]
		if !v.DeepEqual(_src) {
			return false
		}
	}
	return true
}
func (p *Nesting2) Field2DeepEqual(src *Simple) bool {

	if !p.SimpleStruct.DeepEqual(src) {
		return false
	}
	return true
}
func (p *Nesting2) Field3DeepEqual(src int8) bool {

	if p.Byte != src {
		return false
	}
	return true
}
func (p *Nesting2) Field4DeepEqual(src float64) bool {

	if p.Double != src {
		return false
	}
	return true
}
func (p *Nesting2) Field5DeepEqual(src []*Nesting) bool {

	if len(p.ListNesting) != len(src) {
		return false
	}
	for i, v := range p.ListNesting {
		_src := src[i]
		if !v.DeepEqual(_src) {
			return false
		}
	}
	return true
}
func (p *Nesting2) Field6DeepEqual(src int64) bool {

	if p.I64 != src {
		return false
	}
	return true
}
func (p *Nesting2) Field7DeepEqual(src *Nesting) bool {

	if !p.NestingStruct.DeepEqual(src) {
		return false
	}
	return true
}
func (p *Nesting2) Field8DeepEqual(src []byte) bool {

	if bytes.Compare(p.Binary, src) != 0 {
		return false
	}
	return true
}
func (p *Nesting2) Field9DeepEqual(src string) bool {

	if strings.Compare(p.String_, src) != 0 {
		return false
	}
	return true
}
func (p *Nesting2) Field10DeepEqual(src []*Nesting) bool {

	if len(p.SetNesting) != len(src) {
		return false
	}
	for i, v := range p.SetNesting {
		_src := src[i]
		if !v.DeepEqual(_src) {
			return false
		}
	}
	return true
}
func (p *Nesting2) Field11DeepEqual(src int32) bool {

	if p.I32 != src {
		return false
	}
	return true
}

type BaselineService interface {
	SimpleMethod(ctx context.Context, req *Simple) (r *Simple, err error)

	PartialSimpleMethod(ctx context.Context, req *PartialSimple) (r *PartialSimple, err error)

	NestingMethod(ctx context.Context, req *Nesting) (r *Nesting, err error)

	PartialNestingMethod(ctx context.Context, req *PartialNesting) (r *PartialNesting, err error)

	Nesting2Method(ctx context.Context, req *Nesting2) (r *Nesting2, err error)
}

type BaselineServiceClient struct {
	c thrift.TClient
}

func NewBaselineServiceClientFactory(t thrift.TTransport, f thrift.TProtocolFactory) *BaselineServiceClient {
	return &BaselineServiceClient{
		c: thrift.NewTStandardClient(f.GetProtocol(t), f.GetProtocol(t)),
	}
}

func NewBaselineServiceClientProtocol(t thrift.TTransport, iprot thrift.TProtocol, oprot thrift.TProtocol) *BaselineServiceClient {
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

func (p *BaselineServiceClient) PartialSimpleMethod(ctx context.Context, req *PartialSimple) (r *PartialSimple, err error) {
	var _args BaselineServicePartialSimpleMethodArgs
	_args.Req = req
	var _result BaselineServicePartialSimpleMethodResult
	if err = p.Client_().Call(ctx, "PartialSimpleMethod", &_args, &_result); err != nil {
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

func (p *BaselineServiceClient) PartialNestingMethod(ctx context.Context, req *PartialNesting) (r *PartialNesting, err error) {
	var _args BaselineServicePartialNestingMethodArgs
	_args.Req = req
	var _result BaselineServicePartialNestingMethodResult
	if err = p.Client_().Call(ctx, "PartialNestingMethod", &_args, &_result); err != nil {
		return
	}
	return _result.GetSuccess(), nil
}

func (p *BaselineServiceClient) Nesting2Method(ctx context.Context, req *Nesting2) (r *Nesting2, err error) {
	var _args BaselineServiceNesting2MethodArgs
	_args.Req = req
	var _result BaselineServiceNesting2MethodResult
	if err = p.Client_().Call(ctx, "Nesting2Method", &_args, &_result); err != nil {
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
	self.AddToProcessorMap("PartialSimpleMethod", &baselineServiceProcessorPartialSimpleMethod{handler: handler})
	self.AddToProcessorMap("NestingMethod", &baselineServiceProcessorNestingMethod{handler: handler})
	self.AddToProcessorMap("PartialNestingMethod", &baselineServiceProcessorPartialNestingMethod{handler: handler})
	self.AddToProcessorMap("Nesting2Method", &baselineServiceProcessorNesting2Method{handler: handler})
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

type baselineServiceProcessorPartialSimpleMethod struct {
	handler BaselineService
}

func (p *baselineServiceProcessorPartialSimpleMethod) Process(ctx context.Context, seqId int32, iprot, oprot thrift.TProtocol) (success bool, err thrift.TException) {
	args := BaselineServicePartialSimpleMethodArgs{}
	if err = args.Read(iprot); err != nil {
		iprot.ReadMessageEnd()
		x := thrift.NewTApplicationException(thrift.PROTOCOL_ERROR, err.Error())
		oprot.WriteMessageBegin("PartialSimpleMethod", thrift.EXCEPTION, seqId)
		x.Write(oprot)
		oprot.WriteMessageEnd()
		oprot.Flush(ctx)
		return false, err
	}

	iprot.ReadMessageEnd()
	var err2 error
	result := BaselineServicePartialSimpleMethodResult{}
	var retval *PartialSimple
	if retval, err2 = p.handler.PartialSimpleMethod(ctx, args.Req); err2 != nil {
		x := thrift.NewTApplicationException(thrift.INTERNAL_ERROR, "Internal error processing PartialSimpleMethod: "+err2.Error())
		oprot.WriteMessageBegin("PartialSimpleMethod", thrift.EXCEPTION, seqId)
		x.Write(oprot)
		oprot.WriteMessageEnd()
		oprot.Flush(ctx)
		return true, err2
	} else {
		result.Success = retval
	}
	if err2 = oprot.WriteMessageBegin("PartialSimpleMethod", thrift.REPLY, seqId); err2 != nil {
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

type baselineServiceProcessorPartialNestingMethod struct {
	handler BaselineService
}

func (p *baselineServiceProcessorPartialNestingMethod) Process(ctx context.Context, seqId int32, iprot, oprot thrift.TProtocol) (success bool, err thrift.TException) {
	args := BaselineServicePartialNestingMethodArgs{}
	if err = args.Read(iprot); err != nil {
		iprot.ReadMessageEnd()
		x := thrift.NewTApplicationException(thrift.PROTOCOL_ERROR, err.Error())
		oprot.WriteMessageBegin("PartialNestingMethod", thrift.EXCEPTION, seqId)
		x.Write(oprot)
		oprot.WriteMessageEnd()
		oprot.Flush(ctx)
		return false, err
	}

	iprot.ReadMessageEnd()
	var err2 error
	result := BaselineServicePartialNestingMethodResult{}
	var retval *PartialNesting
	if retval, err2 = p.handler.PartialNestingMethod(ctx, args.Req); err2 != nil {
		x := thrift.NewTApplicationException(thrift.INTERNAL_ERROR, "Internal error processing PartialNestingMethod: "+err2.Error())
		oprot.WriteMessageBegin("PartialNestingMethod", thrift.EXCEPTION, seqId)
		x.Write(oprot)
		oprot.WriteMessageEnd()
		oprot.Flush(ctx)
		return true, err2
	} else {
		result.Success = retval
	}
	if err2 = oprot.WriteMessageBegin("PartialNestingMethod", thrift.REPLY, seqId); err2 != nil {
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

type baselineServiceProcessorNesting2Method struct {
	handler BaselineService
}

func (p *baselineServiceProcessorNesting2Method) Process(ctx context.Context, seqId int32, iprot, oprot thrift.TProtocol) (success bool, err thrift.TException) {
	args := BaselineServiceNesting2MethodArgs{}
	if err = args.Read(iprot); err != nil {
		iprot.ReadMessageEnd()
		x := thrift.NewTApplicationException(thrift.PROTOCOL_ERROR, err.Error())
		oprot.WriteMessageBegin("Nesting2Method", thrift.EXCEPTION, seqId)
		x.Write(oprot)
		oprot.WriteMessageEnd()
		oprot.Flush(ctx)
		return false, err
	}

	iprot.ReadMessageEnd()
	var err2 error
	result := BaselineServiceNesting2MethodResult{}
	var retval *Nesting2
	if retval, err2 = p.handler.Nesting2Method(ctx, args.Req); err2 != nil {
		x := thrift.NewTApplicationException(thrift.INTERNAL_ERROR, "Internal error processing Nesting2Method: "+err2.Error())
		oprot.WriteMessageBegin("Nesting2Method", thrift.EXCEPTION, seqId)
		x.Write(oprot)
		oprot.WriteMessageEnd()
		oprot.Flush(ctx)
		return true, err2
	} else {
		result.Success = retval
	}
	if err2 = oprot.WriteMessageBegin("Nesting2Method", thrift.REPLY, seqId); err2 != nil {
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

	if !p.Req.DeepEqual(src) {
		return false
	}
	return true
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

	if !p.Success.DeepEqual(src) {
		return false
	}
	return true
}

type BaselineServicePartialSimpleMethodArgs struct {
	Req *PartialSimple `thrift:"req,1" json:"req"`
}

func NewBaselineServicePartialSimpleMethodArgs() *BaselineServicePartialSimpleMethodArgs {
	return &BaselineServicePartialSimpleMethodArgs{}
}

var BaselineServicePartialSimpleMethodArgs_Req_DEFAULT *PartialSimple

func (p *BaselineServicePartialSimpleMethodArgs) GetReq() (v *PartialSimple) {
	if !p.IsSetReq() {
		return BaselineServicePartialSimpleMethodArgs_Req_DEFAULT
	}
	return p.Req
}
func (p *BaselineServicePartialSimpleMethodArgs) SetReq(val *PartialSimple) {
	p.Req = val
}

var fieldIDToName_BaselineServicePartialSimpleMethodArgs = map[int16]string{
	1: "req",
}

func (p *BaselineServicePartialSimpleMethodArgs) IsSetReq() bool {
	return p.Req != nil
}

func (p *BaselineServicePartialSimpleMethodArgs) Read(iprot thrift.TProtocol) (err error) {

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
	return thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_BaselineServicePartialSimpleMethodArgs[fieldId]), err)
SkipFieldError:
	return thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)

ReadFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T read field end error", p), err)
ReadStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
}

func (p *BaselineServicePartialSimpleMethodArgs) ReadField1(iprot thrift.TProtocol) error {
	p.Req = NewPartialSimple()
	if err := p.Req.Read(iprot); err != nil {
		return err
	}
	return nil
}

func (p *BaselineServicePartialSimpleMethodArgs) Write(oprot thrift.TProtocol) (err error) {
	var fieldId int16
	if err = oprot.WriteStructBegin("PartialSimpleMethod_args"); err != nil {
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

func (p *BaselineServicePartialSimpleMethodArgs) writeField1(oprot thrift.TProtocol) (err error) {
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

func (p *BaselineServicePartialSimpleMethodArgs) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("BaselineServicePartialSimpleMethodArgs(%+v)", *p)
}

func (p *BaselineServicePartialSimpleMethodArgs) DeepEqual(ano *BaselineServicePartialSimpleMethodArgs) bool {
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

func (p *BaselineServicePartialSimpleMethodArgs) Field1DeepEqual(src *PartialSimple) bool {

	if !p.Req.DeepEqual(src) {
		return false
	}
	return true
}

type BaselineServicePartialSimpleMethodResult struct {
	Success *PartialSimple `thrift:"success,0" json:"success,omitempty"`
}

func NewBaselineServicePartialSimpleMethodResult() *BaselineServicePartialSimpleMethodResult {
	return &BaselineServicePartialSimpleMethodResult{}
}

var BaselineServicePartialSimpleMethodResult_Success_DEFAULT *PartialSimple

func (p *BaselineServicePartialSimpleMethodResult) GetSuccess() (v *PartialSimple) {
	if !p.IsSetSuccess() {
		return BaselineServicePartialSimpleMethodResult_Success_DEFAULT
	}
	return p.Success
}
func (p *BaselineServicePartialSimpleMethodResult) SetSuccess(x interface{}) {
	p.Success = x.(*PartialSimple)
}

var fieldIDToName_BaselineServicePartialSimpleMethodResult = map[int16]string{
	0: "success",
}

func (p *BaselineServicePartialSimpleMethodResult) IsSetSuccess() bool {
	return p.Success != nil
}

func (p *BaselineServicePartialSimpleMethodResult) Read(iprot thrift.TProtocol) (err error) {

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
	return thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_BaselineServicePartialSimpleMethodResult[fieldId]), err)
SkipFieldError:
	return thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)

ReadFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T read field end error", p), err)
ReadStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
}

func (p *BaselineServicePartialSimpleMethodResult) ReadField0(iprot thrift.TProtocol) error {
	p.Success = NewPartialSimple()
	if err := p.Success.Read(iprot); err != nil {
		return err
	}
	return nil
}

func (p *BaselineServicePartialSimpleMethodResult) Write(oprot thrift.TProtocol) (err error) {
	var fieldId int16
	if err = oprot.WriteStructBegin("PartialSimpleMethod_result"); err != nil {
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

func (p *BaselineServicePartialSimpleMethodResult) writeField0(oprot thrift.TProtocol) (err error) {
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

func (p *BaselineServicePartialSimpleMethodResult) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("BaselineServicePartialSimpleMethodResult(%+v)", *p)
}

func (p *BaselineServicePartialSimpleMethodResult) DeepEqual(ano *BaselineServicePartialSimpleMethodResult) bool {
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

func (p *BaselineServicePartialSimpleMethodResult) Field0DeepEqual(src *PartialSimple) bool {

	if !p.Success.DeepEqual(src) {
		return false
	}
	return true
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

	if !p.Req.DeepEqual(src) {
		return false
	}
	return true
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

	if !p.Success.DeepEqual(src) {
		return false
	}
	return true
}

type BaselineServicePartialNestingMethodArgs struct {
	Req *PartialNesting `thrift:"req,1" json:"req"`
}

func NewBaselineServicePartialNestingMethodArgs() *BaselineServicePartialNestingMethodArgs {
	return &BaselineServicePartialNestingMethodArgs{}
}

var BaselineServicePartialNestingMethodArgs_Req_DEFAULT *PartialNesting

func (p *BaselineServicePartialNestingMethodArgs) GetReq() (v *PartialNesting) {
	if !p.IsSetReq() {
		return BaselineServicePartialNestingMethodArgs_Req_DEFAULT
	}
	return p.Req
}
func (p *BaselineServicePartialNestingMethodArgs) SetReq(val *PartialNesting) {
	p.Req = val
}

var fieldIDToName_BaselineServicePartialNestingMethodArgs = map[int16]string{
	1: "req",
}

func (p *BaselineServicePartialNestingMethodArgs) IsSetReq() bool {
	return p.Req != nil
}

func (p *BaselineServicePartialNestingMethodArgs) Read(iprot thrift.TProtocol) (err error) {

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
	return thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_BaselineServicePartialNestingMethodArgs[fieldId]), err)
SkipFieldError:
	return thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)

ReadFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T read field end error", p), err)
ReadStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
}

func (p *BaselineServicePartialNestingMethodArgs) ReadField1(iprot thrift.TProtocol) error {
	p.Req = NewPartialNesting()
	if err := p.Req.Read(iprot); err != nil {
		return err
	}
	return nil
}

func (p *BaselineServicePartialNestingMethodArgs) Write(oprot thrift.TProtocol) (err error) {
	var fieldId int16
	if err = oprot.WriteStructBegin("PartialNestingMethod_args"); err != nil {
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

func (p *BaselineServicePartialNestingMethodArgs) writeField1(oprot thrift.TProtocol) (err error) {
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

func (p *BaselineServicePartialNestingMethodArgs) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("BaselineServicePartialNestingMethodArgs(%+v)", *p)
}

func (p *BaselineServicePartialNestingMethodArgs) DeepEqual(ano *BaselineServicePartialNestingMethodArgs) bool {
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

func (p *BaselineServicePartialNestingMethodArgs) Field1DeepEqual(src *PartialNesting) bool {

	if !p.Req.DeepEqual(src) {
		return false
	}
	return true
}

type BaselineServicePartialNestingMethodResult struct {
	Success *PartialNesting `thrift:"success,0" json:"success,omitempty"`
}

func NewBaselineServicePartialNestingMethodResult() *BaselineServicePartialNestingMethodResult {
	return &BaselineServicePartialNestingMethodResult{}
}

var BaselineServicePartialNestingMethodResult_Success_DEFAULT *PartialNesting

func (p *BaselineServicePartialNestingMethodResult) GetSuccess() (v *PartialNesting) {
	if !p.IsSetSuccess() {
		return BaselineServicePartialNestingMethodResult_Success_DEFAULT
	}
	return p.Success
}
func (p *BaselineServicePartialNestingMethodResult) SetSuccess(x interface{}) {
	p.Success = x.(*PartialNesting)
}

var fieldIDToName_BaselineServicePartialNestingMethodResult = map[int16]string{
	0: "success",
}

func (p *BaselineServicePartialNestingMethodResult) IsSetSuccess() bool {
	return p.Success != nil
}

func (p *BaselineServicePartialNestingMethodResult) Read(iprot thrift.TProtocol) (err error) {

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
	return thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_BaselineServicePartialNestingMethodResult[fieldId]), err)
SkipFieldError:
	return thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)

ReadFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T read field end error", p), err)
ReadStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
}

func (p *BaselineServicePartialNestingMethodResult) ReadField0(iprot thrift.TProtocol) error {
	p.Success = NewPartialNesting()
	if err := p.Success.Read(iprot); err != nil {
		return err
	}
	return nil
}

func (p *BaselineServicePartialNestingMethodResult) Write(oprot thrift.TProtocol) (err error) {
	var fieldId int16
	if err = oprot.WriteStructBegin("PartialNestingMethod_result"); err != nil {
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

func (p *BaselineServicePartialNestingMethodResult) writeField0(oprot thrift.TProtocol) (err error) {
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

func (p *BaselineServicePartialNestingMethodResult) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("BaselineServicePartialNestingMethodResult(%+v)", *p)
}

func (p *BaselineServicePartialNestingMethodResult) DeepEqual(ano *BaselineServicePartialNestingMethodResult) bool {
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

func (p *BaselineServicePartialNestingMethodResult) Field0DeepEqual(src *PartialNesting) bool {

	if !p.Success.DeepEqual(src) {
		return false
	}
	return true
}

type BaselineServiceNesting2MethodArgs struct {
	Req *Nesting2 `thrift:"req,1" json:"req"`
}

func NewBaselineServiceNesting2MethodArgs() *BaselineServiceNesting2MethodArgs {
	return &BaselineServiceNesting2MethodArgs{}
}

var BaselineServiceNesting2MethodArgs_Req_DEFAULT *Nesting2

func (p *BaselineServiceNesting2MethodArgs) GetReq() (v *Nesting2) {
	if !p.IsSetReq() {
		return BaselineServiceNesting2MethodArgs_Req_DEFAULT
	}
	return p.Req
}
func (p *BaselineServiceNesting2MethodArgs) SetReq(val *Nesting2) {
	p.Req = val
}

var fieldIDToName_BaselineServiceNesting2MethodArgs = map[int16]string{
	1: "req",
}

func (p *BaselineServiceNesting2MethodArgs) IsSetReq() bool {
	return p.Req != nil
}

func (p *BaselineServiceNesting2MethodArgs) Read(iprot thrift.TProtocol) (err error) {

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
	return thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_BaselineServiceNesting2MethodArgs[fieldId]), err)
SkipFieldError:
	return thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)

ReadFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T read field end error", p), err)
ReadStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
}

func (p *BaselineServiceNesting2MethodArgs) ReadField1(iprot thrift.TProtocol) error {
	p.Req = NewNesting2()
	if err := p.Req.Read(iprot); err != nil {
		return err
	}
	return nil
}

func (p *BaselineServiceNesting2MethodArgs) Write(oprot thrift.TProtocol) (err error) {
	var fieldId int16
	if err = oprot.WriteStructBegin("Nesting2Method_args"); err != nil {
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

func (p *BaselineServiceNesting2MethodArgs) writeField1(oprot thrift.TProtocol) (err error) {
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

func (p *BaselineServiceNesting2MethodArgs) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("BaselineServiceNesting2MethodArgs(%+v)", *p)
}

func (p *BaselineServiceNesting2MethodArgs) DeepEqual(ano *BaselineServiceNesting2MethodArgs) bool {
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

func (p *BaselineServiceNesting2MethodArgs) Field1DeepEqual(src *Nesting2) bool {

	if !p.Req.DeepEqual(src) {
		return false
	}
	return true
}

type BaselineServiceNesting2MethodResult struct {
	Success *Nesting2 `thrift:"success,0" json:"success,omitempty"`
}

func NewBaselineServiceNesting2MethodResult() *BaselineServiceNesting2MethodResult {
	return &BaselineServiceNesting2MethodResult{}
}

var BaselineServiceNesting2MethodResult_Success_DEFAULT *Nesting2

func (p *BaselineServiceNesting2MethodResult) GetSuccess() (v *Nesting2) {
	if !p.IsSetSuccess() {
		return BaselineServiceNesting2MethodResult_Success_DEFAULT
	}
	return p.Success
}
func (p *BaselineServiceNesting2MethodResult) SetSuccess(x interface{}) {
	p.Success = x.(*Nesting2)
}

var fieldIDToName_BaselineServiceNesting2MethodResult = map[int16]string{
	0: "success",
}

func (p *BaselineServiceNesting2MethodResult) IsSetSuccess() bool {
	return p.Success != nil
}

func (p *BaselineServiceNesting2MethodResult) Read(iprot thrift.TProtocol) (err error) {

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
	return thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_BaselineServiceNesting2MethodResult[fieldId]), err)
SkipFieldError:
	return thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)

ReadFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T read field end error", p), err)
ReadStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
}

func (p *BaselineServiceNesting2MethodResult) ReadField0(iprot thrift.TProtocol) error {
	p.Success = NewNesting2()
	if err := p.Success.Read(iprot); err != nil {
		return err
	}
	return nil
}

func (p *BaselineServiceNesting2MethodResult) Write(oprot thrift.TProtocol) (err error) {
	var fieldId int16
	if err = oprot.WriteStructBegin("Nesting2Method_result"); err != nil {
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

func (p *BaselineServiceNesting2MethodResult) writeField0(oprot thrift.TProtocol) (err error) {
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

func (p *BaselineServiceNesting2MethodResult) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("BaselineServiceNesting2MethodResult(%+v)", *p)
}

func (p *BaselineServiceNesting2MethodResult) DeepEqual(ano *BaselineServiceNesting2MethodResult) bool {
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

func (p *BaselineServiceNesting2MethodResult) Field0DeepEqual(src *Nesting2) bool {

	if !p.Success.DeepEqual(src) {
		return false
	}
	return true
}
