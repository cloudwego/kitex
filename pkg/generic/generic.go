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

// Package generic ...
package generic

import (
	"fmt"

	"github.com/cloudwego/dynamicgo/conv"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/protobuf"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

// Generic ...
type Generic interface {
	Closer
	// PayloadCodec return codec implement
	// this is used for generic which does not need IDL
	PayloadCodec() remote.PayloadCodec
	// PayloadCodecType return the type of codec
	PayloadCodecType() serviceinfo.PayloadCodec
	// RawThriftBinaryGeneric must be framed
	Framed() bool
	// GetMethod is to get method name if needed
	GetMethod(req interface{}, method string) (*Method, error)
	// CodecInfo return codec info
	// this is used for generic which needs IDL
	CodecInfo() serviceinfo.CodecInfo
}

// Method information
type Method struct {
	Name   string
	Oneway bool
}

// BinaryThriftGeneric raw thrift binary Generic
func BinaryThriftGeneric() Generic {
	return &binaryThriftGeneric{}
}

// MapThriftGeneric map mapping generic
// Base64 codec for binary field is disabled by default. You can change this option with SetBinaryWithBase64.
// eg:
//
//	g, err := generic.MapThriftGeneric(p)
//	SetBinaryWithBase64(g, true)
//
// String value is returned for binary field by default. You can change the return value to []byte for binary field with SetBinaryWithByteSlice.
// eg:
//
//	SetBinaryWithByteSlice(g, true)
func MapThriftGeneric(p DescriptorProvider) (Generic, error) {
	codecInfo, err := newMapThriftCodec(p)
	if err != nil {
		return nil, err
	}
	return &mapThriftGeneric{
		codecInfo: codecInfo,
	}, nil
}

func MapThriftGenericForJSON(p DescriptorProvider) (Generic, error) {
	codecInfo, err := newMapThriftCodecForJSON(p)
	if err != nil {
		return nil, err
	}
	return &mapThriftGeneric{
		codecInfo: codecInfo,
	}, nil
}

// HTTPThriftGeneric http mapping Generic.
// Base64 codec for binary field is disabled by default. You can change this option with SetBinaryWithBase64.
// eg:
//
//	g, err := generic.HTTPThriftGeneric(p)
//	SetBinaryWithBase64(g, true)
func HTTPThriftGeneric(p DescriptorProvider, opts ...Option) (Generic, error) {
	gOpts := &Options{dynamicgoConvOpts: DefaultHTTPDynamicGoConvOpts}
	gOpts.apply(opts)
	return &httpThriftGeneric{codecInfo: newHTTPThriftCodec(p, thriftCodec, gOpts)}, nil
}

func HTTPPbThriftGeneric(p DescriptorProvider, pbp PbDescriptorProvider) (Generic, error) {
	codec, err := newHTTPPbThriftCodec(p, pbp, thriftCodec)
	if err != nil {
		return nil, err
	}
	return &httpPbThriftGeneric{
		codec: codec,
	}, nil
}

// JSONThriftGeneric json mapping generic.
// Base64 codec for binary field is enabled by default. You can change this option with SetBinaryWithBase64.
// eg:
//
//	g, err := generic.JSONThriftGeneric(p)
//	SetBinaryWithBase64(g, false)
func JSONThriftGeneric(p DescriptorProvider, opts ...Option) (Generic, error) {
	gOpts := &Options{dynamicgoConvOpts: DefaultJSONDynamicGoConvOpts}
	gOpts.apply(opts)
	return &jsonThriftGeneric{codecInfo: newJsonThriftCodec(p, gOpts)}, nil
}

// JSONPbGeneric json mapping generic.
// Uses dynamicgo for json to protobufs conversion, so DynamicGo field is true by default.
func JSONPbGeneric(p PbDescriptorProviderDynamicGo, opts ...Option) (Generic, error) {
	gOpts := &Options{dynamicgoConvOpts: conv.Options{}}
	gOpts.apply(opts)
	return &jsonPbGeneric{codecInfo: newJsonPbCodec(p, pbCodec, gOpts)}, nil
}

// SetBinaryWithBase64 enable/disable Base64 codec for binary field.
func SetBinaryWithBase64(g Generic, enable bool) error {
	switch c := g.(type) {
	case *httpThriftGeneric:
		if c.codecInfo == nil {
			return fmt.Errorf("empty codec for %#v", c)
		}
		c.codecInfo.binaryWithBase64 = enable
		if c.codecInfo.dynamicgoEnabled {
			c.codecInfo.convOpts.NoBase64Binary = !enable
			c.codecInfo.convOptsWithThriftBase.NoBase64Binary = !enable
		}
	case *jsonThriftGeneric:
		if c.codecInfo == nil {
			return fmt.Errorf("empty codec for %#v", c)
		}
		c.codecInfo.binaryWithBase64 = enable
		if c.codecInfo.dynamicgoEnabled {
			c.codecInfo.convOpts.NoBase64Binary = !enable
			c.codecInfo.convOptsWithThriftBase.NoBase64Binary = !enable
			c.codecInfo.convOptsWithException.NoBase64Binary = !enable
		}
	case *mapThriftGeneric:
		if c.codecInfo == nil {
			return fmt.Errorf("empty codec for %#v", c)
		}
		c.codecInfo.binaryWithBase64 = enable
	default:
		return fmt.Errorf("Base64Binary is unavailable for %#v", g)
	}
	return nil
}

// SetBinaryWithByteSlice enable/disable returning []byte for binary field.
func SetBinaryWithByteSlice(g Generic, enable bool) error {
	switch c := g.(type) {
	case *mapThriftGeneric:
		if c.codecInfo == nil {
			return fmt.Errorf("empty codec for %#v", c)
		}
		c.codecInfo.binaryWithByteSlice = enable
	default:
		return fmt.Errorf("returning []byte for binary fields is unavailable for %#v", g)
	}
	return nil
}

// SetFieldsForEmptyStructMode is a enum for EnableSetFieldsForEmptyStruct()
type SetFieldsForEmptyStructMode uint8

const (
	// NotSetFields means disable
	NotSetFields SetFieldsForEmptyStructMode = iota
	// SetNonOptiontionalFields means only set required and default fields
	SetNonOptiontionalFields
	// SetAllFields means set all fields
	SetAllFields
)

// EnableSetFieldsForEmptyStruct enable/disable set all fields of a struct even if it is empty.
// This option is only applicable to map-generic response (reading) now.
//
//	mode == 0 means disable
//	mode == 1 means only set required and default fields
//	mode == 2 means set all fields
func EnableSetFieldsForEmptyStruct(g Generic, mode SetFieldsForEmptyStructMode) error {
	switch c := g.(type) {
	case *mapThriftGeneric:
		if c.codecInfo == nil {
			return fmt.Errorf("empty codec for %#v", c)
		}
		c.codecInfo.setFieldsForEmptyStruct = uint8(mode)
	default:
		return fmt.Errorf("SetFieldsForEmptyStruct only supports map-generic at present")
	}
	return nil
}

var thriftCodec = thrift.NewThriftCodec()

var pbCodec = protobuf.NewProtobufCodec()

type binaryThriftGeneric struct{}

func (g *binaryThriftGeneric) Framed() bool {
	return true
}

func (g *binaryThriftGeneric) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Thrift
}

func (g *binaryThriftGeneric) PayloadCodec() remote.PayloadCodec {
	pc := &binaryThriftCodec{thriftCodec}
	return pc
}

func (g *binaryThriftGeneric) GetMethod(req interface{}, method string) (*Method, error) {
	return &Method{method, false}, nil
}

func (g *binaryThriftGeneric) Close() error {
	return nil
}

func (g *binaryThriftGeneric) CodecInfo() serviceinfo.CodecInfo {
	return nil
}

type mapThriftGeneric struct {
	codecInfo *mapThriftCodec
}

func (g *mapThriftGeneric) Framed() bool {
	return false
}

func (g *mapThriftGeneric) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Thrift
}

func (g *mapThriftGeneric) PayloadCodec() remote.PayloadCodec {
	return nil
}

func (g *mapThriftGeneric) GetMethod(req interface{}, method string) (*Method, error) {
	return g.codecInfo.getMethod(req, method)
}

func (g *mapThriftGeneric) Close() error {
	return g.codecInfo.Close()
}

func (g *mapThriftGeneric) CodecInfo() serviceinfo.CodecInfo {
	return g.codecInfo
}

type jsonThriftGeneric struct {
	codecInfo *jsonThriftCodec
}

func (g *jsonThriftGeneric) Framed() bool {
	return g.codecInfo.dynamicgoEnabled
}

func (g *jsonThriftGeneric) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Thrift
}

func (g *jsonThriftGeneric) PayloadCodec() remote.PayloadCodec {
	return nil
	//return g.codec
}

func (g *jsonThriftGeneric) GetMethod(req interface{}, method string) (*Method, error) {
	return g.codecInfo.getMethod(req, method)
}

func (g *jsonThriftGeneric) Close() error {
	return g.codecInfo.Close()
}

func (g *jsonThriftGeneric) CodecInfo() serviceinfo.CodecInfo {
	return g.codecInfo
}

type jsonPbGeneric struct {
	codecInfo *jsonPbCodec
}

func (g *jsonPbGeneric) Framed() bool {
	return false
}

func (g *jsonPbGeneric) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Protobuf
}

func (g *jsonPbGeneric) PayloadCodec() remote.PayloadCodec {
	return nil
}

func (g *jsonPbGeneric) GetMethod(req interface{}, method string) (*Method, error) {
	return g.codecInfo.getMethod(req, method)
}

func (g *jsonPbGeneric) Close() error {
	return g.codecInfo.Close()
}

func (g *jsonPbGeneric) CodecInfo() serviceinfo.CodecInfo {
	return g.codecInfo
}

type httpThriftGeneric struct {
	codecInfo *httpThriftCodec
}

func (g *httpThriftGeneric) Framed() bool {
	return g.codecInfo.dynamicgoEnabled
}

func (g *httpThriftGeneric) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Thrift
}

func (g *httpThriftGeneric) PayloadCodec() remote.PayloadCodec {
	return nil
}

func (g *httpThriftGeneric) GetMethod(req interface{}, method string) (*Method, error) {
	return g.codecInfo.getMethod(req)
}

func (g *httpThriftGeneric) Close() error {
	return g.codecInfo.Close()
}

func (g *httpThriftGeneric) CodecInfo() serviceinfo.CodecInfo {
	return g.codecInfo
}

type httpPbThriftGeneric struct {
	codec *httpPbThriftCodec
}

func (g *httpPbThriftGeneric) Framed() bool {
	return false
}

func (g *httpPbThriftGeneric) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Thrift
}

func (g *httpPbThriftGeneric) PayloadCodec() remote.PayloadCodec {
	return g.codec
}

func (g *httpPbThriftGeneric) GetMethod(req interface{}, method string) (*Method, error) {
	return g.codec.getMethod(req)
}

func (g *httpPbThriftGeneric) Close() error {
	return g.codec.Close()
}

func (g *httpPbThriftGeneric) CodecInfo() serviceinfo.CodecInfo {
	return nil
}
