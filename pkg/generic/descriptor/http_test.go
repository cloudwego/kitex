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
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJSONBody(t *testing.T) {
	var data = []byte(`{"name":"foo","age":18}`)
	req, err := http.NewRequest("POST", "http://localhost:8080?id=123", bytes.NewBuffer(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	cookie := &http.Cookie{
		Name:  "token",
		Value: "some_token",
	}
	req.AddCookie(cookie)
	req.PostForm = url.Values{
		"key1": {"val1", "val2"},
	}
	param := Param{
		Key:   "param1",
		Value: "value1",
	}
	pslice := []Param{param}
	params := Params{
		params: pslice,
	}
	r := &HTTPRequest{
		Request: req,
		RawBody: data,
		Params:  &params,
	}
	require.Equal(t, "application/json", r.GetHeader("Content-Type"))
	require.Equal(t, "some_token", r.GetCookie("token"))
	require.Equal(t, "123", r.GetQuery("id"))
	require.Equal(t, "value1", r.GetParam("param1"))
	require.Equal(t, data, r.GetBody())
	require.Equal(t, "POST", r.GetMethod())
	require.Equal(t, "", r.GetPath())
	require.Equal(t, "localhost:8080", r.GetHost())
	require.Equal(t, `foo`, r.GetMapBody("name"))
	require.Equal(t, "18", r.GetMapBody("age"))
	require.Equal(t, "val1", r.GetPostForm("key1"))
	require.Equal(t, "http://localhost:8080?id=123", r.GetUri())
}
