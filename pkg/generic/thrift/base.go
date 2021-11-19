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

package thrift

import (
	"fmt"

	"github.com/apache/thrift/lib/go/thrift"
)

type TrafficEnv struct {
	Open bool `thrift:"Open,1" json:"Open"`

	Env string `thrift:"Env,2" json:"Env"`
}

func NewTrafficEnv() *TrafficEnv {
	return &TrafficEnv{
		Open: false,
		Env:  "",
	}
}

func (p *TrafficEnv) GetOpen() bool {
	return p.Open
}

func (p *TrafficEnv) GetEnv() string {
	return p.Env
}

func (p *TrafficEnv) SetOpen(val bool) {
	p.Open = val
}

func (p *TrafficEnv) SetEnv(val string) {
	p.Env = val
}

var fieldIDToName_TrafficEnv = map[int16]string{
	1: "Open",
	2: "Env",
}

func (p *TrafficEnv) Read(iprot thrift.TProtocol) (err error) {
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
			if fieldTypeId == thrift.BOOL {
				if err = p.ReadField1(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 2:
			if fieldTypeId == thrift.STRING {
				if err = p.ReadField2(iprot); err != nil {
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
	return thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_TrafficEnv[fieldId]), err)
SkipFieldError:
	return thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)

ReadFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T read field end error", p), err)
ReadStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
}

func (p *TrafficEnv) ReadField1(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadBool(); err != nil {
		return err
	} else {
		p.Open = v
	}
	return nil
}

func (p *TrafficEnv) ReadField2(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadString(); err != nil {
		return err
	} else {
		p.Env = v
	}
	return nil
}

func (p *TrafficEnv) Write(oprot thrift.TProtocol) (err error) {
	var fieldId int16
	if err = oprot.WriteStructBegin("TrafficEnv"); err != nil {
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

func (p *TrafficEnv) writeField1(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("Open", thrift.BOOL, 1); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteBool(p.Open); err != nil {
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

func (p *TrafficEnv) writeField2(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("Env", thrift.STRING, 2); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteString(p.Env); err != nil {
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

func (p *TrafficEnv) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("TrafficEnv(%+v)", *p)
}

type Base struct {
	LogID string `thrift:"LogID,1" json:"LogID"`

	Caller string `thrift:"Caller,2" json:"Caller"`

	Addr string `thrift:"Addr,3" json:"Addr"`

	Client string `thrift:"Client,4" json:"Client"`

	TrafficEnv *TrafficEnv `thrift:"TrafficEnv,5" json:"TrafficEnv,omitempty"`

	Extra map[string]string `thrift:"Extra,6" json:"Extra,omitempty"`
}

func NewBase() *Base {
	return &Base{
		LogID:  "",
		Caller: "",
		Addr:   "",
		Client: "",
	}
}

func (p *Base) GetLogID() string {
	return p.LogID
}

func (p *Base) GetCaller() string {
	return p.Caller
}

func (p *Base) GetAddr() string {
	return p.Addr
}

func (p *Base) GetClient() string {
	return p.Client
}

var Base_TrafficEnv_DEFAULT *TrafficEnv

func (p *Base) GetTrafficEnv() *TrafficEnv {
	if !p.IsSetTrafficEnv() {
		return Base_TrafficEnv_DEFAULT
	}
	return p.TrafficEnv
}

var Base_Extra_DEFAULT map[string]string

func (p *Base) GetExtra() map[string]string {
	if !p.IsSetExtra() {
		return Base_Extra_DEFAULT
	}
	return p.Extra
}

func (p *Base) SetLogID(val string) {
	p.LogID = val
}

func (p *Base) SetCaller(val string) {
	p.Caller = val
}

func (p *Base) SetAddr(val string) {
	p.Addr = val
}

func (p *Base) SetClient(val string) {
	p.Client = val
}

func (p *Base) SetTrafficEnv(val *TrafficEnv) {
	p.TrafficEnv = val
}

func (p *Base) SetExtra(val map[string]string) {
	p.Extra = val
}

var fieldIDToName_Base = map[int16]string{
	1: "LogID",
	2: "Caller",
	3: "Addr",
	4: "Client",
	5: "TrafficEnv",
	6: "Extra",
}

func (p *Base) IsSetTrafficEnv() bool {
	return p.TrafficEnv != nil
}

func (p *Base) IsSetExtra() bool {
	return p.Extra != nil
}

func (p *Base) Read(iprot thrift.TProtocol) (err error) {
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
			if fieldTypeId == thrift.STRING {
				if err = p.ReadField2(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 3:
			if fieldTypeId == thrift.STRING {
				if err = p.ReadField3(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 4:
			if fieldTypeId == thrift.STRING {
				if err = p.ReadField4(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 5:
			if fieldTypeId == thrift.STRUCT {
				if err = p.ReadField5(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 6:
			if fieldTypeId == thrift.MAP {
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
	return thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_Base[fieldId]), err)
SkipFieldError:
	return thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)

ReadFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T read field end error", p), err)
ReadStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
}

func (p *Base) ReadField1(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadString(); err != nil {
		return err
	} else {
		p.LogID = v
	}
	return nil
}

func (p *Base) ReadField2(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadString(); err != nil {
		return err
	} else {
		p.Caller = v
	}
	return nil
}

func (p *Base) ReadField3(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadString(); err != nil {
		return err
	} else {
		p.Addr = v
	}
	return nil
}

func (p *Base) ReadField4(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadString(); err != nil {
		return err
	} else {
		p.Client = v
	}
	return nil
}

func (p *Base) ReadField5(iprot thrift.TProtocol) error {
	p.TrafficEnv = NewTrafficEnv()
	if err := p.TrafficEnv.Read(iprot); err != nil {
		return err
	}
	return nil
}

func (p *Base) ReadField6(iprot thrift.TProtocol) error {
	_, _, size, err := iprot.ReadMapBegin()
	if err != nil {
		return err
	}
	p.Extra = make(map[string]string, size)
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

		p.Extra[_key] = _val
	}
	if err := iprot.ReadMapEnd(); err != nil {
		return err
	}
	return nil
}

func (p *Base) Write(oprot thrift.TProtocol) (err error) {
	var fieldId int16
	if err = oprot.WriteStructBegin("Base"); err != nil {
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

func (p *Base) writeField1(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("LogID", thrift.STRING, 1); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteString(p.LogID); err != nil {
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

func (p *Base) writeField2(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("Caller", thrift.STRING, 2); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteString(p.Caller); err != nil {
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

func (p *Base) writeField3(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("Addr", thrift.STRING, 3); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteString(p.Addr); err != nil {
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

func (p *Base) writeField4(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("Client", thrift.STRING, 4); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteString(p.Client); err != nil {
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

func (p *Base) writeField5(oprot thrift.TProtocol) (err error) {
	if p.IsSetTrafficEnv() {
		if err = oprot.WriteFieldBegin("TrafficEnv", thrift.STRUCT, 5); err != nil {
			goto WriteFieldBeginError
		}
		if err := p.TrafficEnv.Write(oprot); err != nil {
			return err
		}
		if err = oprot.WriteFieldEnd(); err != nil {
			goto WriteFieldEndError
		}
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 5 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 5 end error: ", p), err)
}

func (p *Base) writeField6(oprot thrift.TProtocol) (err error) {
	if p.IsSetExtra() {
		if err = oprot.WriteFieldBegin("Extra", thrift.MAP, 6); err != nil {
			goto WriteFieldBeginError
		}
		if err := oprot.WriteMapBegin(thrift.STRING, thrift.STRING, len(p.Extra)); err != nil {
			return err
		}
		for k, v := range p.Extra {

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
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 6 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 6 end error: ", p), err)
}

func (p *Base) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("Base(%+v)", *p)
}

type BaseResp struct {
	StatusMessage string `thrift:"StatusMessage,1" json:"StatusMessage"`

	StatusCode int32 `thrift:"StatusCode,2" json:"StatusCode"`

	Extra map[string]string `thrift:"Extra,3" json:"Extra,omitempty"`
}

func NewBaseResp() *BaseResp {
	return &BaseResp{
		StatusMessage: "",
		StatusCode:    0,
	}
}

func (p *BaseResp) GetStatusMessage() string {
	return p.StatusMessage
}

func (p *BaseResp) GetStatusCode() int32 {
	return p.StatusCode
}

var BaseResp_Extra_DEFAULT map[string]string

func (p *BaseResp) GetExtra() map[string]string {
	if !p.IsSetExtra() {
		return BaseResp_Extra_DEFAULT
	}
	return p.Extra
}

func (p *BaseResp) SetStatusMessage(val string) {
	p.StatusMessage = val
}

func (p *BaseResp) SetStatusCode(val int32) {
	p.StatusCode = val
}

func (p *BaseResp) SetExtra(val map[string]string) {
	p.Extra = val
}

var fieldIDToName_BaseResp = map[int16]string{
	1: "StatusMessage",
	2: "StatusCode",
	3: "Extra",
}

func (p *BaseResp) IsSetExtra() bool {
	return p.Extra != nil
}

func (p *BaseResp) Read(iprot thrift.TProtocol) (err error) {
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
			if fieldTypeId == thrift.I32 {
				if err = p.ReadField2(iprot); err != nil {
					goto ReadFieldError
				}
			} else {
				if err = iprot.Skip(fieldTypeId); err != nil {
					goto SkipFieldError
				}
			}
		case 3:
			if fieldTypeId == thrift.MAP {
				if err = p.ReadField3(iprot); err != nil {
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
	return thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldIDToName_BaseResp[fieldId]), err)
SkipFieldError:
	return thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)

ReadFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T read field end error", p), err)
ReadStructEndError:
	return thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
}

func (p *BaseResp) ReadField1(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadString(); err != nil {
		return err
	} else {
		p.StatusMessage = v
	}
	return nil
}

func (p *BaseResp) ReadField2(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadI32(); err != nil {
		return err
	} else {
		p.StatusCode = v
	}
	return nil
}

func (p *BaseResp) ReadField3(iprot thrift.TProtocol) error {
	_, _, size, err := iprot.ReadMapBegin()
	if err != nil {
		return err
	}
	p.Extra = make(map[string]string, size)
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

		p.Extra[_key] = _val
	}
	if err := iprot.ReadMapEnd(); err != nil {
		return err
	}
	return nil
}

func (p *BaseResp) Write(oprot thrift.TProtocol) (err error) {
	var fieldId int16
	if err = oprot.WriteStructBegin("BaseResp"); err != nil {
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

func (p *BaseResp) writeField1(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("StatusMessage", thrift.STRING, 1); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteString(p.StatusMessage); err != nil {
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

func (p *BaseResp) writeField2(oprot thrift.TProtocol) (err error) {
	if err = oprot.WriteFieldBegin("StatusCode", thrift.I32, 2); err != nil {
		goto WriteFieldBeginError
	}
	if err := oprot.WriteI32(p.StatusCode); err != nil {
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

func (p *BaseResp) writeField3(oprot thrift.TProtocol) (err error) {
	if p.IsSetExtra() {
		if err = oprot.WriteFieldBegin("Extra", thrift.MAP, 3); err != nil {
			goto WriteFieldBeginError
		}
		if err := oprot.WriteMapBegin(thrift.STRING, thrift.STRING, len(p.Extra)); err != nil {
			return err
		}
		for k, v := range p.Extra {

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
	}
	return nil
WriteFieldBeginError:
	return thrift.PrependError(fmt.Sprintf("%T write field 3 begin error: ", p), err)
WriteFieldEndError:
	return thrift.PrependError(fmt.Sprintf("%T write field 3 end error: ", p), err)
}

func (p *BaseResp) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("BaseResp(%+v)", *p)
}
