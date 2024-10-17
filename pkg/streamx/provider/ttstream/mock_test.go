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

package ttstream

import (
	"encoding/json"
	"fmt"

	"github.com/cloudwego/kitex/pkg/protocol/bthrift"
	kutils "github.com/cloudwego/kitex/pkg/utils"
)

type TestRequest struct {
	A int32  `thrift:"A,1" frugal:"1,default,i32" json:"A"`
	B string `thrift:"B,2" frugal:"2,default,string" json:"B"`
}

func (p *TestRequest) FastRead(buf []byte) (int, error) {
	err := json.Unmarshal(buf, p)
	if err != nil {
		return 0, err
	}
	return len(buf), nil
}

func (p *TestRequest) FastWriteNocopy(buf []byte, binaryWriter bthrift.BinaryWriter) int {
	data, _ := json.Marshal(p)
	copy(buf, data)
	return len(data)
}

func (p *TestRequest) BLength() int {
	data, _ := json.Marshal(p)
	return len(data)
}

func (p *TestRequest) DeepCopy(s interface{}) error {
	src, ok := s.(*TestRequest)
	if !ok {
		return fmt.Errorf("%T's type not matched %T", s, p)
	}

	p.A = src.A

	if src.B != "" {
		p.B = kutils.StringDeepCopy(src.B)
	}

	return nil
}

type TestResponse = TestRequest
