//go:build amd64
// +build amd64

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
	"net/http"
	"net/url"

	dhttp "github.com/cloudwego/dynamicgo/http"
)

// HTTPRequest ...
type HTTPRequest struct {
	Header       http.Header
	Query        url.Values
	Cookies      Cookies
	Method       string
	Host         string
	Path         string
	Params       *Params // path params
	RawBody      []byte
	Body         map[string]interface{}
	GeneralBody  interface{} // body of other representation, used with ContentType
	ContentType  MIMEType
	DHTTPRequest *dhttp.HTTPRequest
}
