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
	"bytes"
	"io"
	"net/http"
	"unsafe"

	"github.com/bytedance/sonic/ast"
)

// MIMEType ...
type MIMEType string

const (
	MIMEApplicationJson     = "application/json"
	MIMEApplicationProtobuf = "application/x-protobuf"
)

type GoSlice struct {
	Ptr unsafe.Pointer
	Len int
	Cap int
}

type GoString struct {
	Ptr unsafe.Pointer
	Len int
}

// HTTPRequest ...
type HTTPRequest struct {
	Params      *Params // path params
	Request     *http.Request
	RawBody     []byte
	Body        map[string]interface{}
	GeneralBody interface{} // body of other representation, used with ContentType
	ContentType MIMEType
}

type jsonCache struct {
	root ast.Node
	m    map[string]string
}

// GetHeader implements RequestGetter.GetHeader of dynamicgo
func (req HTTPRequest) GetHeader(key string) string {
	return req.Request.Header.Get(key)
}

// GetCookie implements RequestGetter.GetCookie of dynamicgo
func (req HTTPRequest) GetCookie(key string) string {
	if c, err := req.Request.Cookie(key); err == nil {
		return c.Value
	}
	return ""
}

// GetQuery implements RequestGetter.GetQuery of dynamicgo
func (req HTTPRequest) GetQuery(key string) string {
	return req.Request.URL.Query().Get(key)
}

// GetBody implements RequestGetter.GetBody of dynamicgo
func (req HTTPRequest) GetBody() []byte {
	return req.RawBody
}

// GetMethod implements RequestGetter.GetMethod of dynamicgo
func (req HTTPRequest) GetMethod() string {
	return req.Request.Method
}

// GetPath implements RequestGetter.GetPath of dynamicgo
func (req HTTPRequest) GetPath() string {
	return req.Request.URL.Path
}

// GetHost implements RequestGetter.GetHost of dynamicgo
func (req HTTPRequest) GetHost() string {
	return req.Request.Host
}

// GetParam implements RequestGetter.GetParam of dynamicgo
func (req HTTPRequest) GetParam(key string) string {
	return req.Params.ByName(key)
}

// GetMapBody implements RequestGetter.GetMapBody of dynamicgo
func (req HTTPRequest) GetMapBody(key string) string {
	if req.GeneralBody == nil && req.Request != nil {
		body := req.RawBody
		var s string
		(*GoString)(unsafe.Pointer(&s)).Len = (*GoSlice)(unsafe.Pointer(&body)).Len
		(*GoString)(unsafe.Pointer(&s)).Ptr = (*GoSlice)(unsafe.Pointer(&body)).Ptr
		node := ast.NewRaw(s)
		cache := &jsonCache{
			root: node,
			m:    make(map[string]string),
		}
		req.GeneralBody = cache
	}
	t, ok := req.GeneralBody.(*jsonCache)
	if !ok {
		return ""
	}
	// fast path
	if v, ok := t.m[key]; ok {
		return v
	}
	// slow path
	v := t.root.Get(key)
	if v.Check() != nil {
		return ""
	}
	j, e := v.Raw()
	if e != nil {
		return ""
	}
	if v.Type() == ast.V_STRING {
		j, e = v.String()
		if e != nil {
			return ""
		}
	}
	t.m[key] = j
	return j
}

// GetPostForm implements RequestGetter.GetPostForm of dynamicgo.
func (req HTTPRequest) GetPostForm(key string) string {
	return req.Request.PostFormValue(key)
}

// GetUri implements RequestGetter.GetUri.
func (req HTTPRequest) GetUri() string {
	return req.Request.URL.String()
}

// HTTPResponse ...
type HTTPResponse struct {
	Header      http.Header
	StatusCode  int32
	Body        map[string]interface{}
	GeneralBody interface{} // body of other representation, used with ContentType
	ContentType MIMEType
	Renderer    Renderer
}

// NewHTTPResponse HTTP response for JSON body
func NewHTTPResponse() *HTTPResponse {
	return &HTTPResponse{
		Header:      http.Header{},
		ContentType: MIMEApplicationJson,
		Body:        map[string]interface{}{},
		Renderer:    JsonRenderer{},
	}
}

// SetStatusCode implements ResponseSetter.SetStatusCode of dynamicgo
func (resp *HTTPResponse) SetStatusCode(code int) error {
	resp.StatusCode = int32(code)
	return nil
}

// SetHeader implements ResponseSetter.SetHeader of dynamicgo
func (resp *HTTPResponse) SetHeader(key, val string) error {
	resp.Header.Set(key, val)
	return nil
}

// SetCookie implements ResponseSetter.SetCookie of dynamicgo
func (resp *HTTPResponse) SetCookie(key, val string) error {
	c := &http.Cookie{Name: key, Value: val}
	resp.Header.Add("Set-Cookie", c.String())
	return nil
}

type wrapBody struct {
	io.Reader
}

func (wrapBody) Close() error { return nil }

func (resp *HTTPResponse) SetRawBody(body []byte) error {
	resp.GeneralBody = wrapBody{bytes.NewReader(body)}
	return nil
}

func NewHTTPPbResponse(initBody interface{}) *HTTPResponse {
	return &HTTPResponse{
		Header:      http.Header{},
		ContentType: MIMEApplicationProtobuf,
		GeneralBody: initBody,
		Renderer:    PbRenderer{},
	}
}

// NewGeneralHTTPResponse init response with given MIMEType and body
func NewGeneralHTTPResponse(contentType MIMEType, initBody interface{}, renderer Renderer) *HTTPResponse {
	return &HTTPResponse{
		Header:      http.Header{},
		ContentType: contentType,
		GeneralBody: initBody,
		Renderer:    renderer,
	}
}

// Write to ResponseWriter
func (resp *HTTPResponse) Write(w http.ResponseWriter) error {
	w.WriteHeader(int(resp.StatusCode))
	for k := range resp.Header {
		w.Header().Set(k, resp.Header.Get(k))
	}

	resp.Renderer.WriteContentType(w)

	if resp.Body != nil {
		return resp.Renderer.Render(w, resp.Body)
	}

	return resp.Renderer.Render(w, resp.GeneralBody)
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
