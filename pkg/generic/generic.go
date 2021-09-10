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
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

// Generic ...
type Generic interface {
	// PayloadCodec return codec implement
	PayloadCodec() remote.PayloadCodec
	// PayloadCodecType return the type of codec
	PayloadCodecType() serviceinfo.PayloadCodec
	// RawThriftBinaryGeneric must be framed
	Framed() bool
	// GetMethod to get method name if need
	GetMethod(req interface{}, method string) (*Method, error)
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
func MapThriftGeneric(p DescriptorProvider) (Generic, error) {
	codec, err := newMapThriftCodec(p, thriftCodec)
	if err != nil {
		return nil, err
	}
	return &mapThriftGeneric{
		codec: codec,
	}, nil
}

// HTTPThriftGeneric http mapping Generic
func HTTPThriftGeneric(p DescriptorProvider) (Generic, error) {
	codec, err := newHTTPThriftCodec(p, thriftCodec)
	if err != nil {
		return nil, err
	}
	return &httpThriftGeneric{
		codec: codec,
	}, nil
}

// JSONThriftGeneric json mapping generic
func JSONThriftGeneric(p DescriptorProvider) (Generic, error) {
	codec, err := newJsonThriftCodec(p, thriftCodec)
	if err != nil {
		return nil, err
	}
	return &jsonThriftGeneric{codec: codec}, nil
}

var thriftCodec = thrift.NewThriftCodec()

type binaryThriftGeneric struct {
}

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

type mapThriftGeneric struct {
	codec *mapThriftCodec
}

func (g *mapThriftGeneric) Framed() bool {
	return false
}

func (g *mapThriftGeneric) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Thrift
}

func (g *mapThriftGeneric) PayloadCodec() remote.PayloadCodec {
	return g.codec
}

func (g *mapThriftGeneric) GetMethod(req interface{}, method string) (*Method, error) {
	return g.codec.getMethod(req, method)
}

type jsonThriftGeneric struct {
	codec *jsonThriftCodec
}

func (g *jsonThriftGeneric) Framed() bool {
	return false
}

func (g *jsonThriftGeneric) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Thrift
}

func (g *jsonThriftGeneric) PayloadCodec() remote.PayloadCodec {
	return g.codec
}

func (g *jsonThriftGeneric) GetMethod(req interface{}, method string) (*Method, error) {
	return g.codec.getMethod(req, method)
}

type httpThriftGeneric struct {
	codec *httpThriftCodec
}

func (g *httpThriftGeneric) Framed() bool {
	return false
}

func (g *httpThriftGeneric) PayloadCodecType() serviceinfo.PayloadCodec {
	return serviceinfo.Thrift
}

func (g *httpThriftGeneric) PayloadCodec() remote.PayloadCodec {
	return g.codec
}

func (g *httpThriftGeneric) GetMethod(req interface{}, method string) (*Method, error) {
	return g.codec.getMethod(req)
}
