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
	"context"
	"fmt"

	"github.com/cloudwego/dynamicgo/conv"

	igeneric "github.com/cloudwego/kitex/internal/generic"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/protobuf"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

// Generic ...
type Generic interface {
	Closer
	// PayloadCodecType return the type of codec
	PayloadCodecType() serviceinfo.PayloadCodec
	// GenericMethod return generic method func
	GenericMethod() serviceinfo.GenericMethodFunc
	// IDLServiceName returns idl service name
	IDLServiceName() string
	// GetExtra returns extra info by key
	GetExtra(key string) interface{}
}

// GetMethodNameByRequestFunc get method name by request for http generic.
type GetMethodNameByRequestFunc func(req interface{}) (string, error)

// define common interface to initialize method info
type messageReaderWriterGetter interface {
	getMessageReaderWriter() interface{}
}

// Method information of oneway and streaming mode
type Method struct {
	Oneway        bool
	StreamingMode serviceinfo.StreamingMode
}

// BinaryThriftGeneric raw thrift binary Generic.
// Deprecated: use BinaryThriftGenericV2 instead.
func BinaryThriftGeneric() Generic {
	return &binaryThriftGeneric{}
}

// BinaryThriftGenericV2 is raw thrift binary Generic specific to a single service.
// e.g.
//
//	g := generic.BinaryThriftGenericV2("EchoService")
//	cli, err := genericclient.NewClient("destService", g, cliOpts...)
//
// For kitex server, we suggest to use genericserver.RegisterUnknownServiceOrMethodHandler instead of this api
// to be compatible with upstream requests that do not carry the service name, such as raw thrift traffic.
// If you are sure that all upstream requests carry the service name, you can also use this interface for the server:
// e.g.
//
//	g := generic.BinaryThriftGenericV2("EchoService")
//	genericserver.NewServerV2(handler, g, svrOpts...)

func BinaryThriftGenericV2(serviceName string) Generic {
	return &binaryThriftGenericV2{
		codec: newBinaryThriftCodecV2(serviceName),
	}
}

// BinaryPbGeneric is raw protobuf binary payload Generic specific to a single service.
// e.g.
//
//	g := generic.BinaryPbGeneric("EchoService", "echo")
//	cli, err := genericclient.NewClient("destService", g, cliOpts...)
//
// For kitex server, we suggest to use genericserver.RegisterUnknownServiceOrMethodHandler instead of this api
// to be compatible with upstream requests that do not carry the service name.
// If you are sure that all upstream requests carry the service name, you can also use this interface for the server:
// e.g.
//
//	g := generic.BinaryPbGeneric("EchoService", "echo")  // packageName is optional for kitex server.
//	genericserver.NewServerV2(handler, g, svrOpts...)

func BinaryPbGeneric(svcName, packageName string) Generic {
	return &binaryPbGeneric{
		codec: newBinaryPbCodec(svcName, packageName),
	}
}

// MapThriftGeneric converts map[string]interface{} to thrift struct.
// Base64 codec for binary field is disabled by default. You can change this option with SetBinaryWithBase64.
// e.g.
//
//	g, err := generic.MapThriftGeneric(p)
//	SetBinaryWithBase64(g, true)
//
// String value is returned for binary field by default. You can change the return value to []byte for binary field with SetBinaryWithByteSlice.
// e.g.
//
//	SetBinaryWithByteSlice(g, true)
//
// For client usage, e.g.
//
//	cli, err := genericclient.NewClient("destService", g, cliOpts...)
//
// For server usage, e.g.
//
//	genericserver.NewServerV2(handler, g, svrOpts...)

func MapThriftGeneric(p DescriptorProvider) (Generic, error) {
	return &mapThriftGeneric{
		codec: newMapThriftCodec(p, false),
	}, nil
}

// MapThriftGenericForJSON converts map[string]interface{} to thrift struct.
// It converts key type of thrift map fields to string to adapt go json marshal.
// e.g.
//
//	g, err := generic.MapThriftGenericForJSON(p)
//
// For client usage, e.g.
//
//	cli, err := genericclient.NewClient("destService", g, cliOpts...)
//
// For server usage, e.g.
//
//	genericserver.NewServerV2(handler, g, svrOpts...)

func MapThriftGenericForJSON(p DescriptorProvider) (Generic, error) {
	return &mapThriftGeneric{
		codec: newMapThriftCodec(p, true),
	}, nil
}

// HTTPThriftGeneric http mapping Generic.
// Base64 codec for binary field is disabled by default. You can change this option with SetBinaryWithBase64.
// e.g.
//
//	g, err := generic.HTTPThriftGeneric(p)
//	SetBinaryWithBase64(g, true)
//
// For client usage, e.g.
//
//	cli, err := genericclient.NewClient("destService", g, cliOpts...)
//
// For server usage, e.g.
//
//	genericserver.NewServerV2(handler, g, svrOpts...)

func HTTPThriftGeneric(p DescriptorProvider, opts ...Option) (Generic, error) {
	gOpts := &Options{dynamicgoConvOpts: DefaultHTTPDynamicGoConvOpts}
	gOpts.apply(opts)
	return &httpThriftGeneric{codec: newHTTPThriftCodec(p, gOpts)}, nil
}

// HTTPPbThriftGeneric http mapping Generic with protobuf to thrift.
// e.g.
//
//	g, err := generic.HTTPPbThriftGeneric(p, pbp)
//
// For client usage, e.g.
//
//	cli, err := genericclient.NewClient("destService", g, cliOpts...)
//
// For server usage, e.g.
//
//	genericserver.NewServerV2(handler, g, svrOpts...)

func HTTPPbThriftGeneric(p DescriptorProvider, pbp PbDescriptorProvider) (Generic, error) {
	return &httpPbThriftGeneric{
		codec: newHTTPPbThriftCodec(p, pbp),
	}, nil
}

// JSONThriftGeneric json mapping generic.
// Base64 codec for binary field is enabled by default. You can change this option with SetBinaryWithBase64.
// e.g.
//
//	g, err := generic.JSONThriftGeneric(p)
//	SetBinaryWithBase64(g, false)
//
// For client usage, e.g.
//
//	cli, err := genericclient.NewClient("destService", g, cliOpts...)
//
// For server usage, e.g.
//
//	genericserver.NewServerV2(handler, g, svrOpts...)

func JSONThriftGeneric(p DescriptorProvider, opts ...Option) (Generic, error) {
	gOpts := &Options{dynamicgoConvOpts: DefaultJSONDynamicGoConvOpts}
	gOpts.apply(opts)
	return &jsonThriftGeneric{codec: newJsonThriftCodec(p, gOpts)}, nil
}

// JSONPbGeneric json mapping generic.
// Uses dynamicgo for json to protobufs conversion, so DynamicGo field is true by default.
// e.g.
//
//	g, err := generic.JSONPbGeneric(pbp)
//
// For client usage, e.g.
//
//	cli, err := genericclient.NewClient("destService", g, cliOpts...)
//
// For server usage, e.g.
//
//	genericserver.NewServerV2(handler, g, svrOpts...)

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

// PayloadCodec return codec implement
// this is used for generic which does not need IDL
func (g *binaryThriftGeneric) PayloadCodec() remote.PayloadCodec {
	pc := &binaryThriftCodec{thriftCodec}
	return pc
}

func (g *binaryThriftGeneric) GenericMethod() serviceinfo.GenericMethodFunc {
	// note: binary generic cannot be used with multi-service or streaming feature
	mi := serviceinfo.NewMethodInfo(callHandler, newGenericServiceCallArgs, newGenericServiceCallResult, false)
	return func(ctx context.Context, methodName string) serviceinfo.MethodInfo {
		return mi
	}
}

// GetExtra returns binary thrift PayloadCodec to replace the original one, called in WithGeneric.
func (g *binaryThriftGeneric) GetExtra(key string) interface{} {
	switch key {
	case igeneric.BinaryThriftGenericV1PayloadCodecKey:
		return g.PayloadCodec()
	}
	return nil
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

func (b *binaryThriftGenericV2) GenericMethod() serviceinfo.GenericMethodFunc {
	methods := newMethodsMap(b.codec)
	return func(ctx context.Context, methodName string) serviceinfo.MethodInfo {
		return methods[igeneric.GetGenericStreamingMode(ctx)]
	}
}

func (b *binaryThriftGenericV2) GetExtra(key string) interface{} {
	switch key {
	case igeneric.IsBinaryGeneric:
		return true
	}
	return nil
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

func (g *binaryPbGeneric) GenericMethod() serviceinfo.GenericMethodFunc {
	methods := newMethodsMap(g.codec)
	return func(ctx context.Context, methodName string) serviceinfo.MethodInfo {
		return methods[igeneric.GetGenericStreamingMode(ctx)]
	}
}

func (g *binaryPbGeneric) Close() error {
	return nil
}

func (g *binaryPbGeneric) IDLServiceName() string {
	return g.codec.svcName
}

func (g *binaryPbGeneric) GetExtra(key string) interface{} {
	switch key {
	case igeneric.IsBinaryGeneric:
		return true
	case serviceinfo.PackageName:
		return g.codec.packageName
	}
	return nil
}

type mapThriftGeneric struct {
	codec *mapThriftCodec
}

func (g *mapThriftGeneric) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Thrift
}

func (g *mapThriftGeneric) GenericMethod() serviceinfo.GenericMethodFunc {
	methodsFunc := newMethodsFunc(g.codec)
	return func(ctx context.Context, methodName string) serviceinfo.MethodInfo {
		m, err := g.codec.getMethod(methodName)
		if err != nil {
			return nil
		}
		return methodsFunc(m)
	}
}

func (g *mapThriftGeneric) Close() error {
	return g.codec.Close()
}

func (g *mapThriftGeneric) IDLServiceName() string {
	s, _ := g.codec.svcName.Load().(string)
	return s
}

func (g *mapThriftGeneric) GetExtra(key string) interface{} {
	if key == serviceinfo.CombineServiceKey {
		s, _ := g.codec.combineService.Load().(bool)
		return s
	}
	return nil
}

type jsonThriftGeneric struct {
	codec *jsonThriftCodec
}

func (g *jsonThriftGeneric) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Thrift
}

func (g *jsonThriftGeneric) GenericMethod() serviceinfo.GenericMethodFunc {
	methodsFunc := newMethodsFunc(g.codec)
	return func(ctx context.Context, methodName string) serviceinfo.MethodInfo {
		m, err := g.codec.getMethod(methodName)
		if err != nil {
			return nil
		}
		return methodsFunc(m)
	}
}

func (g *jsonThriftGeneric) Close() error {
	return g.codec.Close()
}

func (g *jsonThriftGeneric) IDLServiceName() string {
	s, _ := g.codec.svcName.Load().(string)
	return s
}

func (g *jsonThriftGeneric) GetExtra(key string) interface{} {
	if key == serviceinfo.CombineServiceKey {
		s, _ := g.codec.combineService.Load().(bool)
		return s
	}
	return nil
}

type jsonPbGeneric struct {
	codec *jsonPbCodec
}

func (g *jsonPbGeneric) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Protobuf
}

func (g *jsonPbGeneric) GenericMethod() serviceinfo.GenericMethodFunc {
	methodsFunc := newMethodsFunc(g.codec)
	return func(ctx context.Context, methodName string) serviceinfo.MethodInfo {
		m, err := g.codec.getMethod(methodName)
		if err != nil {
			return nil
		}
		return methodsFunc(m)
	}
}

func (g *jsonPbGeneric) Close() error {
	return g.codec.Close()
}

func (g *jsonPbGeneric) IDLServiceName() string {
	s, _ := g.codec.svcName.Load().(string)
	return s
}

func (g *jsonPbGeneric) GetExtra(key string) interface{} {
	switch key {
	case serviceinfo.PackageName:
		s, _ := g.codec.packageName.Load().(string)
		return s
	case serviceinfo.CombineServiceKey:
		s, _ := g.codec.combineService.Load().(bool)
		return s
	}
	return nil
}

type httpThriftGeneric struct {
	codec *httpThriftCodec
}

func (g *httpThriftGeneric) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Thrift
}

func (g *httpThriftGeneric) GenericMethod() serviceinfo.GenericMethodFunc {
	methodsFunc := newMethodsFunc(g.codec)
	return func(ctx context.Context, methodName string) serviceinfo.MethodInfo {
		m, err := g.codec.getMethod(methodName)
		if err != nil {
			return nil
		}
		return methodsFunc(m)
	}
}

func (g *httpThriftGeneric) Close() error {
	return g.codec.Close()
}

func (g *httpThriftGeneric) IDLServiceName() string {
	s, _ := g.codec.svcName.Load().(string)
	return s
}

func (g *httpThriftGeneric) GetExtra(key string) interface{} {
	switch key {
	case igeneric.GetMethodNameByRequestFuncKey:
		return GetMethodNameByRequestFunc(func(req interface{}) (string, error) {
			return g.codec.getMethodByReq(req)
		})
	case serviceinfo.CombineServiceKey:
		s, _ := g.codec.combineService.Load().(bool)
		return s
	}
	return nil
}

type httpPbThriftGeneric struct {
	codec *httpPbThriftCodec
}

func (g *httpPbThriftGeneric) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Thrift
}

func (g *httpPbThriftGeneric) GenericMethod() serviceinfo.GenericMethodFunc {
	methodsFunc := newMethodsFunc(g.codec)
	return func(ctx context.Context, methodName string) serviceinfo.MethodInfo {
		m, err := g.codec.getMethod(methodName)
		if err != nil {
			return nil
		}
		return methodsFunc(m)
	}
}

func (g *httpPbThriftGeneric) Close() error {
	return g.codec.Close()
}

func (g *httpPbThriftGeneric) IDLServiceName() string {
	return g.codec.svcName.Load().(string)
}

func (g *httpPbThriftGeneric) GetExtra(key string) interface{} {
	switch key {
	case igeneric.GetMethodNameByRequestFuncKey:
		return GetMethodNameByRequestFunc(func(req interface{}) (string, error) {
			return g.codec.getMethodByReq(req)
		})
	case serviceinfo.CombineServiceKey:
		s, _ := g.codec.combineService.Load().(bool)
		return s
	}
	return nil
}

func newMethodsFunc(readWriterGetter messageReaderWriterGetter) func(sm Method) serviceinfo.MethodInfo {
	methods := newMethodsMap(readWriterGetter)
	return func(m Method) serviceinfo.MethodInfo {
		mi := methods[m.StreamingMode]
		if m.Oneway {
			return &methodInfo{
				MethodInfo: mi,
				oneway:     true,
			}
		}
		return mi
	}
}

func newMethodsMap(readWriterGetter messageReaderWriterGetter) map[serviceinfo.StreamingMode]serviceinfo.MethodInfo {
	return map[serviceinfo.StreamingMode]serviceinfo.MethodInfo{
		serviceinfo.StreamingClient:        newMethodInfo(readWriterGetter, serviceinfo.StreamingClient, false),
		serviceinfo.StreamingServer:        newMethodInfo(readWriterGetter, serviceinfo.StreamingServer, false),
		serviceinfo.StreamingBidirectional: newMethodInfo(readWriterGetter, serviceinfo.StreamingBidirectional, false),
		// actually the bellow two are the same, but keep both of them for compatibility
		serviceinfo.StreamingUnary: newMethodInfo(readWriterGetter, serviceinfo.StreamingUnary, false),
		serviceinfo.StreamingNone:  newMethodInfo(readWriterGetter, serviceinfo.StreamingNone, false),
	}
}

func newMethodInfo(readWriterGetter messageReaderWriterGetter, sm serviceinfo.StreamingMode, oneway bool) serviceinfo.MethodInfo {
	methodInfoGetter := func(handler serviceinfo.MethodHandler) serviceinfo.MethodInfo {
		return serviceinfo.NewMethodInfo(
			handler,
			func() interface{} {
				args := &Args{}
				args.SetCodec(readWriterGetter.getMessageReaderWriter())
				return args
			},
			func() interface{} {
				result := &Result{}
				result.SetCodec(readWriterGetter.getMessageReaderWriter())
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

// methodInfo is a wrapper to update the oneway flag of a service.MethodInfo.
type methodInfo struct {
	serviceinfo.MethodInfo
	oneway bool
}

func (m *methodInfo) OneWay() bool {
	return m.oneway
}
