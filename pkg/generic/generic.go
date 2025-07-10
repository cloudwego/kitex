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

type DeprecatedBinaryThriftGeneric interface {
	Generic
	// PayloadCodec return codec implement
	// this is used for generic which does not need IDL
	PayloadCodec() remote.PayloadCodec
}

// Generic ...
type Generic interface {
	Closer
	// PayloadCodecType return the type of codec
	PayloadCodecType() serviceinfo.PayloadCodec
	// Methods return all methods info parsed from idl
	Methods() (map[string]serviceinfo.MethodInfo, serviceinfo.GenericMethodFunc)
	// IDLServiceName returns idl service name
	IDLServiceName() string
}

// Method information
type Method struct {
	Name          string
	Oneway        bool
	StreamingMode serviceinfo.StreamingMode
}

// ExtraProvider provides extra info for generic
type ExtraProvider interface {
	GetExtra(key string) string
}

const (
	CombineServiceKey = "combine_service"
	packageNameKey    = "PackageName"
)

// BinaryThriftGeneric raw thrift binary Generic
func BinaryThriftGeneric() Generic {
	return &binaryThriftGeneric{}
}

func BinaryThriftGenericV2(serviceName string) Generic {
	return &binaryThriftGenericV2{
		codec: newBinaryThriftCodecV2(serviceName),
	}
}

// BinaryPbGeneric raw protobuf binary payload Generic
func BinaryPbGeneric(svcName, packageName string) Generic {
	return &binaryPbGeneric{
		codec: newBinaryPbCodec(svcName, packageName),
	}
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
	return &mapThriftGeneric{
		codec: newMapThriftCodec(p, false),
	}, nil
}

func MapThriftGenericForJSON(p DescriptorProvider) (Generic, error) {
	return &mapThriftGeneric{
		codec: newMapThriftCodec(p, true),
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
	return &httpThriftGeneric{codec: newHTTPThriftCodec(p, gOpts)}, nil
}

func HTTPPbThriftGeneric(p DescriptorProvider, pbp PbDescriptorProvider) (Generic, error) {
	return &httpPbThriftGeneric{
		codec: newHTTPPbThriftCodec(p, pbp),
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
	return &jsonThriftGeneric{codec: newJsonThriftCodec(p, gOpts)}, nil
}

// JSONPbGeneric json mapping generic.
// Uses dynamicgo for json to protobufs conversion, so DynamicGo field is true by default.
func JSONPbGeneric(p PbDescriptorProviderDynamicGo, opts ...Option) (Generic, error) {
	gOpts := &Options{dynamicgoConvOpts: conv.Options{}}
	gOpts.apply(opts)
	return &jsonPbGeneric{codec: newJsonPbCodec(p, gOpts)}, nil
}

// SetBinaryWithBase64 enable/disable Base64 codec for binary field.
func SetBinaryWithBase64(g Generic, enable bool) error {
	switch c := g.(type) {
	case *httpThriftGeneric:
		if c.codec == nil {
			return fmt.Errorf("empty codec for %#v", c)
		}
		c.codec.binaryWithBase64 = enable
		if c.codec.dynamicgoEnabled {
			c.codec.convOpts.NoBase64Binary = !enable
			c.codec.convOptsWithThriftBase.NoBase64Binary = !enable
		}
		return c.codec.updateMessageReaderWriter()
	case *jsonThriftGeneric:
		if c.codec == nil {
			return fmt.Errorf("empty codec for %#v", c)
		}
		c.codec.binaryWithBase64 = enable
		if c.codec.dynamicgoEnabled {
			c.codec.convOpts.NoBase64Binary = !enable
			c.codec.convOptsWithThriftBase.NoBase64Binary = !enable
			c.codec.convOptsWithException.NoBase64Binary = !enable
		}
		return c.codec.updateMessageReaderWriter()
	case *mapThriftGeneric:
		if c.codec == nil {
			return fmt.Errorf("empty codec for %#v", c)
		}
		c.codec.binaryWithBase64 = enable
		return c.codec.updateMessageReaderWriter()
	default:
		return fmt.Errorf("Base64Binary is unavailable for %#v", g)
	}
}

// SetBinaryWithByteSlice enable/disable returning []byte for binary field.
func SetBinaryWithByteSlice(g Generic, enable bool) error {
	switch c := g.(type) {
	case *mapThriftGeneric:
		if c.codec == nil {
			return fmt.Errorf("empty codec for %#v", c)
		}
		c.codec.binaryWithByteSlice = enable
		return c.codec.updateMessageReaderWriter()
	default:
		return fmt.Errorf("returning []byte for binary fields is unavailable for %#v", g)
	}
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
		if c.codec == nil {
			return fmt.Errorf("empty codec for %#v", c)
		}
		c.codec.setFieldsForEmptyStruct = uint8(mode)
		return c.codec.updateMessageReaderWriter()
	default:
		return fmt.Errorf("SetFieldsForEmptyStruct only supports map-generic at present")
	}
}

var thriftCodec = thrift.NewThriftCodec()

var pbCodec = protobuf.NewProtobufCodec()

type binaryThriftGeneric struct{}

func (g *binaryThriftGeneric) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Thrift
}

func (g *binaryThriftGeneric) PayloadCodec() remote.PayloadCodec {
	pc := &binaryThriftCodec{thriftCodec}
	return pc
}

func (g *binaryThriftGeneric) Methods() (map[string]serviceinfo.MethodInfo, serviceinfo.GenericMethodFunc) {
	// note: binary generic cannot be used with multi-service or streaming feature
	mi := serviceinfo.NewMethodInfo(callHandler, newGenericServiceCallArgs, newGenericServiceCallResult, false)
	genericMethod := func(streamMode serviceinfo.StreamingMode) serviceinfo.MethodInfo {
		return mi
	}
	return make(map[string]serviceinfo.MethodInfo), genericMethod
}

func (g *binaryThriftGeneric) Close() error {
	return nil
}

func (g *binaryThriftGeneric) IDLServiceName() string {
	return serviceinfo.GenericService
}

type binaryThriftGenericV2 struct {
	codec *binaryThriftCodecV2
}

func (b *binaryThriftGenericV2) IDLServiceName() string {
	return b.codec.svcName
}

func (b *binaryThriftGenericV2) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Thrift
}

func (b *binaryThriftGenericV2) Methods() (map[string]serviceinfo.MethodInfo, serviceinfo.GenericMethodFunc) {
	rw := b.codec.getMessageReaderWriter()
	methods := map[string]serviceinfo.MethodInfo{
		serviceinfo.GenericClientStreamingMethod:        newMethodInfo(rw, serviceinfo.StreamingClient, false),
		serviceinfo.GenericServerStreamingMethod:        newMethodInfo(rw, serviceinfo.StreamingServer, false),
		serviceinfo.GenericBidirectionalStreamingMethod: newMethodInfo(rw, serviceinfo.StreamingBidirectional, false),
		serviceinfo.GenericMethod:                       newMethodInfo(rw, serviceinfo.StreamingNone, false),
	}
	genericMethod := func(streamMode serviceinfo.StreamingMode) serviceinfo.MethodInfo {
		key := getGenericStreamingMethodInfoKey(streamMode)
		return methods[key]
	}
	return make(map[string]serviceinfo.MethodInfo), genericMethod
}

func (b *binaryThriftGenericV2) Close() error {
	return nil
}

type binaryPbGeneric struct {
	codec *binaryPbCodec
}

func (g *binaryPbGeneric) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Protobuf
}

func (g *binaryPbGeneric) Methods() (map[string]serviceinfo.MethodInfo, serviceinfo.GenericMethodFunc) {
	rw := g.codec.getMessageReaderWriter()
	methods := map[string]serviceinfo.MethodInfo{
		serviceinfo.GenericClientStreamingMethod:        newMethodInfo(rw, serviceinfo.StreamingClient, false),
		serviceinfo.GenericServerStreamingMethod:        newMethodInfo(rw, serviceinfo.StreamingServer, false),
		serviceinfo.GenericBidirectionalStreamingMethod: newMethodInfo(rw, serviceinfo.StreamingBidirectional, false),
		serviceinfo.GenericMethod:                       newMethodInfo(rw, serviceinfo.StreamingNone, false),
	}
	genericMethod := func(streamMode serviceinfo.StreamingMode) serviceinfo.MethodInfo {
		key := getGenericStreamingMethodInfoKey(streamMode)
		return methods[key]
	}
	return make(map[string]serviceinfo.MethodInfo), genericMethod
}

func (g *binaryPbGeneric) Close() error {
	return nil
}

func (g *binaryPbGeneric) IDLServiceName() string {
	return g.codec.svcName
}

func (g *binaryPbGeneric) MessageReaderWriter() interface{} {
	return g.codec.getMessageReaderWriter()
}

func (g *binaryPbGeneric) GetExtra(key string) string {
	return g.codec.extra[key]
}

type mapThriftGeneric struct {
	codec *mapThriftCodec
}

func (g *mapThriftGeneric) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Thrift
}

func (g *mapThriftGeneric) Methods() (map[string]serviceinfo.MethodInfo, serviceinfo.GenericMethodFunc) {
	functions := g.codec.getServiceDescriptor().Functions
	res := make(map[string]serviceinfo.MethodInfo, len(functions))
	for name, fn := range functions {
		res[name] = newMethodInfo(g.codec.getMessageReaderWriter(), fn.StreamingMode, fn.Oneway)
	}
	return res, nil
}

func (g *mapThriftGeneric) Close() error {
	return g.codec.Close()
}

func (g *mapThriftGeneric) IDLServiceName() string {
	return g.codec.svcName
}

func (g *mapThriftGeneric) GetExtra(key string) string {
	return g.codec.extra[key]
}

type jsonThriftGeneric struct {
	codec *jsonThriftCodec
}

func (g *jsonThriftGeneric) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Thrift
}

func (g *jsonThriftGeneric) Methods() (map[string]serviceinfo.MethodInfo, serviceinfo.GenericMethodFunc) {
	functions := g.codec.getServiceDescriptor().Functions
	res := make(map[string]serviceinfo.MethodInfo, len(functions))
	for name, fn := range functions {
		res[name] = newMethodInfo(g.codec.getMessageReaderWriter(), fn.StreamingMode, fn.Oneway)
	}
	return res, nil
}

func (g *jsonThriftGeneric) Close() error {
	return g.codec.Close()
}

func (g *jsonThriftGeneric) IDLServiceName() string {
	return g.codec.svcName
}

func (g *jsonThriftGeneric) GetExtra(key string) string {
	return g.codec.extra[key]
}

type jsonPbGeneric struct {
	codec *jsonPbCodec
}

func (g *jsonPbGeneric) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Protobuf
}

func (g *jsonPbGeneric) Methods() (map[string]serviceinfo.MethodInfo, serviceinfo.GenericMethodFunc) {
	functions := g.codec.getServiceDescriptor().Methods()
	res := make(map[string]serviceinfo.MethodInfo, len(functions))
	for name, fn := range functions {
		res[name] = newMethodInfo(g.codec.getMessageReaderWriter(), getStreamingMode(fn), false)
	}
	return res, nil
}

func (g *jsonPbGeneric) Close() error {
	return g.codec.Close()
}

func (g *jsonPbGeneric) IDLServiceName() string {
	return g.codec.svcName
}

func (g *jsonPbGeneric) GetExtra(key string) string {
	return g.codec.extra[key]
}

type httpThriftGeneric struct {
	codec *httpThriftCodec
}

func (g *httpThriftGeneric) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Thrift
}

func (g *httpThriftGeneric) Methods() (map[string]serviceinfo.MethodInfo, serviceinfo.GenericMethodFunc) {
	functions := g.codec.getServiceDescriptor().Functions
	res := make(map[string]serviceinfo.MethodInfo, len(functions))
	for name, fn := range functions {
		res[name] = newMethodInfo(g.codec.getMessageReaderWriter(), fn.StreamingMode, fn.Oneway)
	}
	return res, nil
}

func (g *httpThriftGeneric) Close() error {
	return g.codec.Close()
}

func (g *httpThriftGeneric) IDLServiceName() string {
	return g.codec.svcName
}

func (g *httpThriftGeneric) GetExtra(key string) string {
	return g.codec.extra[key]
}

type httpPbThriftGeneric struct {
	codec *httpPbThriftCodec
}

func (g *httpPbThriftGeneric) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Thrift
}

func (g *httpPbThriftGeneric) Methods() (map[string]serviceinfo.MethodInfo, serviceinfo.GenericMethodFunc) {
	functions := g.codec.getServiceDescriptor().Functions
	res := make(map[string]serviceinfo.MethodInfo, len(functions))
	for name, fn := range functions {
		res[name] = newMethodInfo(g.codec.getMessageReaderWriter(), fn.StreamingMode, fn.Oneway)
	}
	return res, nil
}

func (g *httpPbThriftGeneric) Close() error {
	return g.codec.Close()
}

func (g *httpPbThriftGeneric) IDLServiceName() string {
	return g.codec.svcName
}

func (g *httpPbThriftGeneric) GetExtra(key string) string {
	return g.codec.extra[key]
}

func HasIDLInfo(g Generic) bool {
	switch g.(type) {
	case *binaryThriftGeneric, *binaryPbGeneric:
		return false
	default:
		return true
	}
}

func newMethodInfo(readWriter interface{}, sm serviceinfo.StreamingMode, oneway bool) serviceinfo.MethodInfo {
	methodInfoGetter := func(handler serviceinfo.MethodHandler) serviceinfo.MethodInfo {
		return serviceinfo.NewMethodInfo(
			handler,
			func() interface{} {
				args := &Args{}
				args.SetCodec(readWriter)
				return args
			},
			func() interface{} {
				result := &Result{}
				result.SetCodec(readWriter)
				return result
			},
			oneway,
			serviceinfo.WithStreamingMode(sm),
		)
	}
	var streamHandlerGetter func(mi serviceinfo.MethodInfo) serviceinfo.MethodHandler
	switch sm {
	case serviceinfo.StreamingClient:
		streamHandlerGetter = clientStreamingHandlerGetter
	case serviceinfo.StreamingServer:
		streamHandlerGetter = serverStreamingHandlerGetter
	case serviceinfo.StreamingBidirectional:
		streamHandlerGetter = bidiStreamingHandlerGetter
	}
	if streamHandlerGetter != nil {
		// note: construct method info twice to make the method handler obtain the args/results constructor.
		return methodInfoGetter(streamHandlerGetter(methodInfoGetter(nil)))
	}
	return methodInfoGetter(callHandler)
}
