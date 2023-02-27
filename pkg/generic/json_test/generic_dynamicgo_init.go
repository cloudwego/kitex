//go:build amd64 && go1.16
// +build amd64,go1.16

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

// Package test ...
package test

import (
	"bytes"
	"context"
	"math"
	"strconv"
	"strings"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/genericclient"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/transport"
)

func getString() string {
	return strings.Repeat("你好,\b\n\r\t世界", 2)
}

func getBytes() []byte {
	return bytes.Repeat([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}, 2)
}

func GetSimpleValue() *Simple {
	return &Simple{
		ByteField:   math.MaxInt8,
		I64Field:    math.MaxInt64,
		DoubleField: math.MaxFloat64,
		I32Field:    math.MaxInt32,
		StringField: getString(),
		BinaryField: getBytes(),
	}
}

func GetNestingValue() *Nesting {
	ret := &Nesting{
		String_:         getString(),
		ListSimple:      []*Simple{},
		Double:          math.MaxFloat64,
		I32:             math.MaxInt32,
		ListI32:         []int32{},
		I64:             math.MaxInt64,
		MapStringString: map[string]string{},
		SimpleStruct:    GetSimpleValue(),
		MapI32I64:       map[int32]int64{},
		ListString:      []string{},
		Binary:          getBytes(),
		MapI64String:    map[int64]string{},
		ListI64:         []int64{},
		Byte:            math.MaxInt8,
		MapStringSimple: map[string]*Simple{},
	}

	for i := 0; i < 16; i++ {
		ret.ListSimple = append(ret.ListSimple, GetSimpleValue())
		ret.ListI32 = append(ret.ListI32, math.MinInt32)
		ret.ListI64 = append(ret.ListI64, math.MinInt64)
		ret.ListString = append(ret.ListString, getString())
	}

	for i := 0; i < 16; i++ {
		ret.MapStringString[strconv.Itoa(i)] = getString()
		ret.MapI32I64[int32(i)] = math.MinInt64
		ret.MapI64String[int64(i)] = getString()
		ret.MapStringSimple[strconv.Itoa(i)] = GetSimpleValue()
	}

	return ret
}

func newGenericDynamicgoClient(tp transport.Protocol, destService string, g generic.Generic, targetIPPort string) genericclient.Client {
	var opts []client.Option
	opts = append(opts, client.WithHostPorts(targetIPPort), client.WithTransportProtocol(tp))
	genericCli, _ := genericclient.NewClient(destService, g, opts...)
	return genericCli
}

// GenericServiceImpl ...
type GenericServiceBenchmarkImpl struct{}

// GenericCall ...
func (g *GenericServiceBenchmarkImpl) GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error) {
	return request, nil
}
