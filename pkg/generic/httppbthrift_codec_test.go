/*
 * Copyright 2023 CloudWeGo Authors
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

package generic

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"

	"github.com/bytedance/mockey"

	"github.com/cloudwego/kitex/internal/test"
)

func TestFromHTTPPbRequest(t *testing.T) {
	mockey.PatchConvey("TestFromHTTPPbRequest", t, func() {
		req, err := http.NewRequest("POST", "/far/boo", bytes.NewBuffer([]byte("321")))
		test.Assert(t, err == nil)
		mockey.Mock(ioutil.ReadAll).Return([]byte("123"), nil).Build()
		hreq, err := FromHTTPPbRequest(req)
		test.Assert(t, err == nil)
		test.Assert(t, reflect.DeepEqual(hreq.RawBody, []byte("123")), string(hreq.RawBody))
		test.Assert(t, hreq.Method == "POST")
		test.Assert(t, hreq.Path == "/far/boo")
	})
}
