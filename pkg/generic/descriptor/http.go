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

package descriptor

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/jhump/protoreflect/dynamic"
)

// Cookies ...
type Cookies map[string]string

// MIMEType ...
type MIMEType string

const (
	MIMEApplicationJson     = "application/json"
	MIMEApplicationForm     = "application/x-www-form-urlencoded"
	MIMEApplicationProtobuf = "application/x-protobuf"
)

// HTTPRequest ...
type HTTPRequest struct {
	Header      http.Header
	Query       url.Values
	Cookies     Cookies
	Method      string
	Host        string
	Path        string
	Params      *Params // path params
	RawBody     []byte
	Body        interface{}
	ContentType MIMEType
}

// HTTPResponse ...
type HTTPResponse struct {
	Header      http.Header
	StatusCode  int32
	Body        interface{}
	ContentType MIMEType
}

// NewHTTPJsonResponse ...
func NewHTTPJsonResponse() *HTTPResponse {
	return &HTTPResponse{
		Header:      http.Header{},
		ContentType: MIMEApplicationJson,
		Body:        map[string]interface{}{},
	}
}

// NewHTTPResponse init response with given MIMEType and body
func NewHTTPResponse(contentType MIMEType, initBody interface{}) *HTTPResponse {
	return &HTTPResponse{
		Header:      http.Header{},
		ContentType: contentType,
		Body:        initBody,
	}
}

// Write to ResponseWriter
func (resp *HTTPResponse) Write(w http.ResponseWriter) error {
	w.WriteHeader(int(resp.StatusCode))
	for k := range resp.Header {
		w.Header().Set(k, resp.Header.Get(k))
	}

	w.Header().Set("Content-Type", string(resp.ContentType))

	switch resp.ContentType {
	case MIMEApplicationJson:
		return json.NewEncoder(w).Encode(resp.Body)
	case MIMEApplicationProtobuf:
		bytes, err := resp.Body.(*dynamic.Message).Marshal()
		if err != nil {
			return err
		}
		_, err = w.Write(bytes)
		return err
	default:
		return nil
	}
}

// Param in request path
type Param struct {
	Key   string
	Value string
}

// Params and recyclable
type Params struct {
	params   []Param
	recycle  func(*Params)
	recycled bool
}

// Recycle the Params
func (ps *Params) Recycle() {
	if ps.recycled {
		return
	}
	ps.recycled = true
	ps.recycle(ps)
}

// ByName search Param by given name
func (ps *Params) ByName(name string) string {
	for _, p := range ps.params {
		if p.Key == name {
			return p.Value
		}
	}
	return ""
}
