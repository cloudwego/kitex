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

	"github.com/bytedance/sonic/ast"
	dhttp "github.com/cloudwego/dynamicgo/http"

	"github.com/cloudwego/kitex/pkg/utils"
)

// MIMEType ...
type MIMEType string

const (
	MIMEApplicationJson     = "application/json"
	MIMEApplicationProtobuf = "application/x-protobuf"
)

var (
	_ dhttp.RequestGetter  = &HTTPRequest{}
	_ dhttp.ResponseSetter = &HTTPResponse{}
)

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

// GetHeader implements http.RequestGetter of dynamicgo
func (req *HTTPRequest) GetHeader(key string) string {
	return req.Request.Header.Get(key)
}

// GetCookie implements http.RequestGetter of dynamicgo
func (req *HTTPRequest) GetCookie(key string) string {
	if c, err := req.Request.Cookie(key); err == nil {
		return c.Value
	}
	return ""
}

// GetQuery implements http.RequestGetter of dynamicgo
func (req *HTTPRequest) GetQuery(key string) string {
	return req.Request.URL.Query().Get(key)
}

// GetBody implements http.RequestGetter of dynamicgo
func (req *HTTPRequest) GetBody() []byte {
	return req.RawBody
}

// GetMethod implements http.RequestGetter of dynamicgo
func (req *HTTPRequest) GetMethod() string {
	return req.Request.Method
}

// GetPath implements http.RequestGetter of dynamicgo
func (req *HTTPRequest) GetPath() string {
	return req.Request.URL.Path
}

// GetHost implements http.RequestGetter of dynamicgo
func (req *HTTPRequest) GetHost() string {
	return req.Request.Host
}

// GetParam implements http.RequestGetter of dynamicgo
func (req *HTTPRequest) GetParam(key string) string {
	return req.Params.ByName(key)
}

// GetMapBody implements http.RequestGetter of dynamicgo
func (req *HTTPRequest) GetMapBody(key string) string {
	if len(req.RawBody) == 0 {
		return ""
	}
	if req.GeneralBody == nil && req.Request != nil {
		body := req.RawBody
		s := utils.SliceByteToString(body)
		node := ast.NewRaw(s)
		cache := &jsonCache{
			root: node,
			m:    make(map[string]string),
		}
		req.GeneralBody = cache
	}
	switch t := req.GeneralBody.(type) {
	case *jsonCache:
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
	default:
		return ""
	}
}

// GetPostForm implements http.RequestGetter of dynamicgo
func (req *HTTPRequest) GetPostForm(key string) string {
	return req.Request.PostFormValue(key)
}

// GetUri implements http.RequestGetter of dynamicgo
func (req *HTTPRequest) GetUri() string {
	return req.Request.URL.String()
}

// HTTPResponse ...
type HTTPResponse struct {
	Header      http.Header
	StatusCode  int32
	RawBody     []byte
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

// SetStatusCode implements http.ResponseSetter of dynamicgo
func (resp *HTTPResponse) SetStatusCode(code int) error {
	resp.StatusCode = int32(code)
	return nil
}

// SetHeader implements http.ResponseSetter of dynamicgo
func (resp *HTTPResponse) SetHeader(key, val string) error {
	resp.Header.Set(key, val)
	return nil
}

// SetCookie implements http.ResponseSetter of dynamicgo
func (resp *HTTPResponse) SetCookie(key, val string) error {
	c := &http.Cookie{Name: key, Value: val}
	resp.Header.Add("Set-Cookie", c.String())
	return nil
}

type wrapBody struct {
	io.Reader
}

func (wrapBody) Close() error { return nil }

// SetRawBody implements http.ResponseSetter of dynamicgo
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
