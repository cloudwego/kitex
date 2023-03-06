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
	"net/url"

	"github.com/bytedance/sonic/ast"
)

// Cookies ...
type Cookies map[string]string

// MIMEType ...
type MIMEType string

const (
	MIMEApplicationJson     = "application/json"
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
	PostForm    url.Values
	Uri         string
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
func (hreq HTTPRequest) GetHeader(key string) string {
	return hreq.Header.Get(key)
}

// GetCookie implements RequestGetter.GetCookie of dynamicgo
func (hreq HTTPRequest) GetCookie(key string) string {
	return hreq.Cookies[key]
}

// GetQuery implements RequestGetter.GetQuery of dynamicgo
func (hreq HTTPRequest) GetQuery(key string) string {
	return hreq.Query.Get(key)
}

// GetBody implements RequestGetter.GetBody of dynamicgo
func (hreq HTTPRequest) GetBody() []byte {
	return hreq.RawBody
}

// GetMethod implements RequestGetter.GetMethod of dynamicgo
func (hreq HTTPRequest) GetMethod() string {
	return hreq.Method
}

// GetPath implements RequestGetter.GetPath of dynamicgo
func (hreq HTTPRequest) GetPath() string {
	return hreq.Path
}

// GetHost implements RequestGetter.GetHost of dynamicgo
func (hreq HTTPRequest) GetHost() string {
	return hreq.Host
}

// GetParam implements RequestGetter.GetParam of dynamicgo
func (hreq HTTPRequest) GetParam(key string) string {
	return hreq.Params.ByName(key)
}

// GetMapBody implements RequestGetter.GetMapBody.
func (hreq *HTTPRequest) GetMapBody(key string) string {
	switch t := hreq.GeneralBody.(type) {
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
	case map[string]string:
		return t[key]
	case url.Values:
		return t.Get(key)
	default:
		return ""
	}
}

// GetPostForm implements RequestGetter.GetPostForm of dynamicgo.
func (hreq HTTPRequest) GetPostForm(key string) string {
	if vs := hreq.PostForm[key]; len(vs) > 0 {
		return vs[0]
	}
	return ""
}

// GetUri implements RequestGetter.GetUri.
func (hreq HTTPRequest) GetUri() string {
	return hreq.Uri
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
func (hresp HTTPResponse) SetStatusCode(code int) error {
	hresp.StatusCode = int32(code)
	return nil
}

// SetHeader implements ResponseSetter.SetHeader of dynamicgo
func (hresp HTTPResponse) SetHeader(key string, val string) error {
	hresp.Header.Set(key, val)
	return nil
}

// SetCookie implements ResponseSetter.SetCookie of dynamicgo
func (hresp HTTPResponse) SetCookie(key string, val string) error {
	c := &http.Cookie{Name: key, Value: val}
	hresp.Header.Add("Set-Cookie", c.String())
	return nil
}

type wrapBody struct {
	io.Reader
}

func (wrapBody) Close() error { return nil }

func (self HTTPResponse) SetRawBody(body []byte) error {
	self.GeneralBody = wrapBody{bytes.NewReader(body)}
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
