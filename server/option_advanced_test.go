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

package server

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/mocks"
	internal_server "github.com/cloudwego/kitex/internal/server"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/acl"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

func TestWithServerBasicInfo(t *testing.T) {
	svcName := "svcName" + time.Now().String()
	svr := NewServer(WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{
		ServiceName: svcName,
	}))
	iSvr := svr.(*server)
	test.Assert(t, iSvr.opt.Svr.ServiceName == svcName, iSvr.opt.Svr.ServiceName)
}

func TestACLRulesOption(t *testing.T) {
	var (
		rules []acl.RejectFunc
	)
	rules = append(rules, func(ctx context.Context, request interface{}) error {
		return nil
	})
	rules = append(rules, func(ctx context.Context, request interface{}) error {
		return nil
	})

	svr := NewServer(WithACLRules(rules...))
	err := svr.RegisterService(mocks.ServiceInfo(), mocks.MyServiceHandler())
	test.Assert(t, err == nil, err)
	time.AfterFunc(100*time.Millisecond, func() {
		err := svr.Stop()
		test.Assert(t, err == nil, err)
	})
	err = svr.Run()
	test.Assert(t, err == nil, err)
	iSvr := svr.(*server)
	test.Assert(t, len(iSvr.opt.ACLRules) == 2, len(iSvr.opt.ACLRules))
	test.Assert(t, iSvr.opt.ACLRules[0] != nil)
	test.Assert(t, iSvr.opt.ACLRules[1] != nil)
}

func TestProxyOptionPanic(t *testing.T) {
	o := internal_server.NewOptions(nil)
	o.Proxy = &proxyMock{}

	opts := []Option{
		WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{
			ServiceName: "svcName",
		}),
		WithProxy(&proxyMock{}),
	}

	test.Panic(t, func() {
		internal_server.ApplyOptions(opts, o)
	})
}

type myLimitReporter struct {
}

func (m *myLimitReporter) ConnOverloadReport() {

}

func (m *myLimitReporter) QPSOverloadReport() {

}

type otherLimitReporter struct {
}

func (m *otherLimitReporter) ConnOverloadReport() {

}

func (m *otherLimitReporter) QPSOverloadReport() {

}

func TestLimitReporterOption(t *testing.T) {
	my := &myLimitReporter{}
	other := &otherLimitReporter{}
	svr := NewServer(WithLimitReporter(my))
	err := svr.RegisterService(mocks.ServiceInfo(), mocks.MyServiceHandler())
	test.Assert(t, err == nil, err)
	time.AfterFunc(100*time.Millisecond, func() {
		err := svr.Stop()
		test.Assert(t, err == nil, err)
	})
	err = svr.Run()
	test.Assert(t, err == nil, err)
	iSvr := svr.(*server)
	test.Assert(t, iSvr.opt.LimitReporter != nil)
	test.Assert(t, iSvr.opt.LimitReporter == my)
	test.DeepEqual(t, iSvr.opt.LimitReporter, my)
	test.Assert(t, iSvr.opt.LimitReporter != other)
}

func TestGenericOption(t *testing.T) {

}
