/*
 * Copyright 2025 CloudWeGo Authors
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

package pbtpl

import (
	"bytes"
	"go/format"
	"testing"
)

func TestRender(t *testing.T) {
	in := &Args{}
	in.Services = []*Service{
		{
			Name: "UnaryService",
			Methods: []*Method{{
				Name:    "UnaryFunc",
				ReqType: "Req",
				ResType: "Res",
			}},
		},
		{
			Name: "ClientStreamingService",
			Methods: []*Method{{
				Name:    "StreamingFunc",
				ReqType: "Req",
				ResType: "Res",

				ClientStream: true,
			}},
		},
		{
			Name: "ServerStreamingService",
			Methods: []*Method{{
				Name:    "StreamingFunc",
				ReqType: "Req",
				ResType: "Res",

				ServerStream: true,
			}},
		},
		{
			Name: "BidStreamingService",
			Methods: []*Method{{
				Name:    "StreamingFunc",
				ReqType: "Req",
				ResType: "Res",

				ClientStream: true,
				ServerStream: true,
			}},
		},
	}

	buf := &bytes.Buffer{}

	t.Log("StreamX=false")
	in.StreamX = false
	Render(buf, in)
	b, err := format.Source(buf.Bytes())
	if err != nil {
		t.Fatal(buf.String(), "\n", err)
	} else {
		t.Log(string(b))
	}

	buf.Reset()
	t.Log("StreamX=true")
	in.StreamX = true
	Render(buf, in)
	b, err = format.Source(buf.Bytes())
	if err != nil {
		t.Fatal(buf.String(), "\n", err)
	} else {
		t.Log(string(b))
	}
}
