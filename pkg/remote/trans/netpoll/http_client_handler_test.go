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

package netpoll

import (
	"strings"
	"testing"

	"github.com/jackedelic/kitex/internal/pkg/remote"
)

func TestReadLine(t *testing.T) {
	wantHead := "HTTP/1.1 200 OK"
	body := "{\"code\":0,\"data\":[\"mobile\",\"xxxxxxx\"],\"msg\":\"ok\"}"
	resp := []byte(wantHead + "\r\nDate: Thu, 16 Aug 2018 03:10:03 GMT\r\nKeep-Alive: timeout=5, max=100\r\nConnection: Keep-Alive\r\nTransfer-Encoding: chunked\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n" + body)
	reader := remote.NewReaderBuffer(resp)
	getHead, _ := readLine(reader)
	if strings.Compare(string(getHead), wantHead) != 0 {
		t.Fatal("readLine wrong")
	}
}

func TestSkipToBody(t *testing.T) {
	head := "HTTP/1.1 200 OK"
	wantBody := "{\"code\":0,\"data\":[\"mobile\",\"xxxxxxx\"],\"msg\":\"ok\"}"
	resp := []byte(head + "\r\nDate: Thu, 16 Aug 2018 03:10:03 GMT\r\nKeep-Alive: timeout=5, max=100\r\nConnection: Keep-Alive\r\nTransfer-Encoding: chunked\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n" + wantBody)
	reader := remote.NewReaderBuffer(resp)
	err := skipToBody(reader)
	if err != nil {
		t.Fatal(err)
	}
	getBody, err := reader.ReadBinary(reader.ReadableLen())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Compare(string(getBody), wantBody) != 0 {
		t.Fatal("skipToBody wrong")
	}
}

func TestParseHTTPResonseHead(t *testing.T) {
	head := "HTTP/1.1 200 OK"
	major, minor, statusCode, err := parseHTTPResposneHead(head)
	if err != nil {
		t.Fatal(err)
	}
	if major != 1 || minor != 1 || statusCode != 200 {
		t.Fatal("ParseHTTPResponseHead wrong")
	}
}
