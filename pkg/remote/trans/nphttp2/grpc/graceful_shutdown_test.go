/*
 * Copyright 2024 CloudWeGo Authors
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

package grpc

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
)

func TestGracefulShutdown(t *testing.T) {
	srv, cli := setUp(t, 0, math.MaxUint32, gracefulShutdown)
	defer cli.Close(errSelfCloseForTest)

	stream, err := cli.NewStream(context.Background(), &CallHdr{})
	test.Assert(t, err == nil, err)
	<-srv.srvReady
	go srv.gracefulShutdown()
	err = cli.Write(stream, nil, []byte("hello"), &Options{})
	test.Assert(t, err == nil, err)
	msg := make([]byte, 5)
	num, err := stream.Read(msg)
	test.Assert(t, err == nil, err)
	test.Assert(t, num == 5, num)
	_, err = cli.NewStream(context.Background(), &CallHdr{})
	test.Assert(t, err != nil, err)
	t.Logf("NewStream err: %v", err)
	time.Sleep(1 * time.Second)
	err = cli.Write(stream, nil, []byte("hello"), &Options{})
	test.Assert(t, err != nil, err)
	t.Logf("After timeout, Write err: %v", err)
	_, err = stream.Read(msg)
	test.Assert(t, err != nil, err)
	t.Logf("After timeout, Read err: %v", err)
}
