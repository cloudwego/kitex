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

package ttheaderstreaming

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/gopkg/cloud/metainfo"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/remote/transmeta"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	header_transmeta "github.com/cloudwego/kitex/pkg/transmeta"
)

func Test_clientTTHeaderFrameHandler_ServerReadHeader(t *testing.T) {
	defer func() {
		if p := recover(); p != nil {
			t.Logf("expected panic")
		} else {
			t.Errorf("missing expected panic")
		}
	}()
	h := NewClientTTHeaderFrameHandler()
	_, _ = h.ServerReadHeader(context.Background(), nil)
}

func Test_clientTTHeaderFrameHandler_WriteHeader(t *testing.T) {
	ivk := rpcinfo.NewInvocation("idl-service-name", "method-name")
	ri := rpcinfo.NewRPCInfo(nil, nil, ivk, nil, nil)
	msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)

	h := NewClientTTHeaderFrameHandler()
	_ = h.WriteHeader(context.Background(), msg)

	intInfo := msg.TransInfo().TransIntInfo()
	strInfo := msg.TransInfo().TransStrInfo()
	test.Assert(t, intInfo[transmeta.ToMethod] == ivk.MethodName(), intInfo)
	test.Assert(t, strInfo[transmeta.HeaderIDLServiceName] == ivk.ServiceName(), strInfo)
}

func Test_clientTTHeaderFrameHandler_ReadTrailer(t *testing.T) {
	t.Run("trans-error-parse-failure", func(t *testing.T) {
		msg := remote.NewMessage(nil, nil, nil, remote.Stream, remote.Client)
		msg.TransInfo().TransIntInfo()[transmeta.TransCode] = "X"

		h := NewClientTTHeaderFrameHandler()
		err := h.ReadTrailer(context.Background(), msg)
		test.Assert(t, err != nil, err)
		transErr, ok := err.(*remote.TransError)
		test.Assert(t, ok, err)
		test.Assert(t, transErr.TypeID() == remote.InternalError, transErr)
	})

	t.Run("trans-error", func(t *testing.T) {
		msg := remote.NewMessage(nil, nil, nil, remote.Stream, remote.Client)
		msg.TransInfo().TransIntInfo()[transmeta.TransCode] = "1204"
		msg.TransInfo().TransIntInfo()[transmeta.TransMessage] = "mesh timeout"

		h := NewClientTTHeaderFrameHandler()
		err := h.ReadTrailer(context.Background(), msg)
		test.Assert(t, err != nil, err)
		transErr, ok := err.(*remote.TransError)
		test.Assert(t, ok, err)
		test.Assert(t, transErr.TypeID() == 1204, transErr)
		test.Assert(t, transErr.Error() == "mesh timeout", transErr)
	})

	t.Run("biz-status-error-parse-failure", func(t *testing.T) {
		ivk := rpcinfo.NewInvocation("", "")
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, nil, nil)
		bizErr := kerrors.NewBizStatusError(100, "biz error")
		ri.Invocation().(rpcinfo.InvocationSetter).SetBizStatusErr(bizErr)
		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
		header_transmeta.InjectBizStatusError(msg.TransInfo().TransStrInfo(), ri.Invocation().BizStatusErr())
		msg.TransInfo().TransStrInfo()["biz-extra"] = "invalid biz extra"

		h := NewClientTTHeaderFrameHandler()
		err := h.ReadTrailer(context.Background(), msg)
		test.Assert(t, err != nil, err)
	})

	t.Run("biz-status-error", func(t *testing.T) {
		ivk := rpcinfo.NewInvocation("", "")
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, nil, nil)
		bizErr := kerrors.NewBizStatusError(100, "biz error")
		ri.Invocation().(rpcinfo.InvocationSetter).SetBizStatusErr(bizErr)
		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
		header_transmeta.InjectBizStatusError(msg.TransInfo().TransStrInfo(), ri.Invocation().BizStatusErr())

		h := NewClientTTHeaderFrameHandler()
		err := h.ReadTrailer(context.Background(), msg)
		test.Assert(t, err == nil, err)

		gotBizErr := ri.Invocation().BizStatusErr()
		test.Assert(t, gotBizErr.BizStatusCode() == bizErr.BizStatusCode(), gotBizErr)
		test.Assert(t, gotBizErr.BizMessage() == bizErr.BizMessage(), gotBizErr)
	})
}

func Test_parseTransErrorIfExists(t *testing.T) {
	t.Run("no-trans-code", func(t *testing.T) {
		test.Assert(t, parseTransErrorIfExists(map[uint16]string{}) == nil)
	})
	t.Run("invalid-trans-code", func(t *testing.T) {
		err := parseTransErrorIfExists(map[uint16]string{transmeta.TransCode: "X"})
		test.Assert(t, err != nil, err)
		transErr := err.(*remote.TransError)
		test.Assert(t, transErr.TypeID() == remote.InternalError, transErr)
	})
	t.Run("code=0", func(t *testing.T) {
		test.Assert(t, parseTransErrorIfExists(map[uint16]string{transmeta.TransCode: "0"}) == nil)
	})
	t.Run("non-zero code", func(t *testing.T) {
		intInfo := map[uint16]string{
			transmeta.TransCode:    "1204",
			transmeta.TransMessage: "mesh timeout",
		}
		err := parseTransErrorIfExists(intInfo)
		test.Assert(t, err != nil, err)
		transErr := err.(*remote.TransError)
		test.Assert(t, transErr.TypeID() == 1204, transErr)
		test.Assert(t, transErr.Error() == "mesh timeout", transErr)
	})
}

func Test_clientTTHeaderFrameHandler_WriteTrailer(t *testing.T) {
	t.Run("nil-trans-error", func(t *testing.T) {
		msg := remote.NewMessage(nil, nil, nil, remote.Stream, remote.Client)

		h := NewClientTTHeaderFrameHandler()
		err := h.WriteTrailer(context.Background(), msg)
		test.Assert(t, err == nil, err)

		intInfo := msg.TransInfo().TransIntInfo()
		test.Assert(t, len(intInfo) == 0)
	})

	t.Run("trans-error", func(t *testing.T) {
		transErr := remote.NewTransError(1204, errors.New("mesh timeout"))
		msg := remote.NewMessage(transErr, nil, nil, remote.Stream, remote.Client)

		h := NewClientTTHeaderFrameHandler()
		err := h.WriteTrailer(context.Background(), msg)
		test.Assert(t, err == nil, err)

		intInfo := msg.TransInfo().TransIntInfo()
		test.Assert(t, len(intInfo) == 2)
		test.Assert(t, intInfo[transmeta.TransCode] == "1204")
		test.Assert(t, intInfo[transmeta.TransMessage] == "mesh timeout")
	})
}

func Test_clientTTHeaderFrameHandler_WriteTrailer1(t *testing.T) {
	t.Run("not-trans-error", func(t *testing.T) {
		err := errors.New("xxx")
		msg := remote.NewMessage(err, nil, nil, remote.Stream, remote.Client)
		injectTransError(msg, err)
		intInfo := msg.TransInfo().TransIntInfo()
		test.Assert(t, intInfo[transmeta.TransCode] == strconv.Itoa(remote.InternalError), intInfo)
		test.Assert(t, intInfo[transmeta.TransMessage] == err.Error(), intInfo)
	})
	t.Run("trans-error", func(t *testing.T) {
		transErr := remote.NewTransError(1204, errors.New("mesh timeout"))
		msg := remote.NewMessage(transErr, nil, nil, remote.Stream, remote.Client)
		injectTransError(msg, transErr)
		intInfo := msg.TransInfo().TransIntInfo()
		test.Assert(t, intInfo[transmeta.TransCode] == strconv.Itoa(1204), intInfo)
		test.Assert(t, intInfo[transmeta.TransMessage] == "mesh timeout", intInfo)
	})
}

func Test_serverTTHeaderFrameHandler_ReadMeta(t *testing.T) {
	defer func() {
		err := recover()
		test.Assert(t, err != nil, err)
	}()
	msg := remote.NewMessage(nil, nil, nil, remote.Stream, remote.Server)
	h := NewServerTTHeaderFrameHandler()
	_ = h.ReadMeta(context.Background(), msg)
	t.Errorf("ReadMeta should not be called at server side")
}

func Test_serverTTHeaderFrameHandler_ClientReadHeader(t *testing.T) {
	defer func() {
		err := recover()
		test.Assert(t, err != nil, err)
	}()
	msg := remote.NewMessage(nil, nil, nil, remote.Stream, remote.Server)
	h := NewServerTTHeaderFrameHandler()
	_ = h.ClientReadHeader(context.Background(), msg)
	t.Errorf("ClientReadHeader should not be called at client side")
}

func Test_serverTTHeaderFrameHandler_ServerReadHeader(t *testing.T) {
	t.Run("missing-to-method", func(t *testing.T) {
		ivk := rpcinfo.NewInvocation("", "")
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, rpcinfo.NewRPCConfig(), nil)
		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
		h := NewServerTTHeaderFrameHandler()
		_, err := h.ServerReadHeader(context.Background(), msg)
		test.Assert(t, err != nil, err)
		test.Assert(t, strings.HasPrefix(err.Error(), "missing method"), err)
	})

	t.Run("normal:to_method", func(t *testing.T) {
		ivk := rpcinfo.NewInvocation("", "")
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, rpcinfo.NewRPCConfig(), nil)
		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
		intInfo := msg.TransInfo().TransIntInfo()
		intInfo[transmeta.ToMethod] = "to-method"
		h := NewServerTTHeaderFrameHandler()
		_, err := h.ServerReadHeader(context.Background(), msg)
		test.Assert(t, err == nil, err)
		test.Assert(t, ivk.MethodName() == "to-method", ivk.MethodName())
		test.Assert(t, ri.Config().RPCTimeout() == 0, ri.Config().RPCTimeout())
	})

	t.Run("normal:+rpcTimeout", func(t *testing.T) {
		ivk := rpcinfo.NewInvocation("", "")
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, rpcinfo.NewRPCConfig(), nil)
		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
		intInfo := msg.TransInfo().TransIntInfo()
		intInfo[transmeta.ToMethod] = "to-method"
		intInfo[transmeta.RPCTimeout] = "1000" // ms
		h := NewServerTTHeaderFrameHandler()
		_, err := h.ServerReadHeader(context.Background(), msg)
		test.Assert(t, err == nil, err)
		test.Assert(t, ri.Config().RPCTimeout() == 1000*time.Millisecond, ri.Config().RPCTimeout())
	})

	t.Run("normal:+isn", func(t *testing.T) {
		ivk := rpcinfo.NewInvocation("", "")
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, rpcinfo.NewRPCConfig(), nil)
		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
		intInfo := msg.TransInfo().TransIntInfo()
		intInfo[transmeta.ToMethod] = "to-method"
		msg.TransInfo().TransStrInfo()[transmeta.HeaderIDLServiceName] = "isn"
		h := NewServerTTHeaderFrameHandler()
		_, err := h.ServerReadHeader(context.Background(), msg)
		test.Assert(t, err == nil, err)
		test.Assert(t, ivk.ServiceName() == "isn", ivk.ServiceName())
	})

	t.Run("normal:+metainfo", func(t *testing.T) {
		ivk := rpcinfo.NewInvocation("", "")
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, rpcinfo.NewRPCConfig(), nil)
		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
		msg.TransInfo().TransIntInfo()[transmeta.ToMethod] = "to-method"
		strInfo := msg.TransInfo().TransStrInfo()
		strInfo[metainfo.PrefixTransient+"key"] = "value"
		strInfo[metainfo.PrefixPersistent+"key"] = "value"

		h := NewServerTTHeaderFrameHandler()
		ctx, err := h.ServerReadHeader(context.Background(), msg)
		test.Assert(t, err == nil, err)

		value, _ := metainfo.GetValue(ctx, "key")
		test.Assert(t, value == "value", value)

		pValue, _ := metainfo.GetPersistentValue(ctx, "key")
		test.Assert(t, pValue == "value", pValue)
	})

	t.Run("normal:+grpc_metadata", func(t *testing.T) {
		ivk := rpcinfo.NewInvocation("", "")
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, rpcinfo.NewRPCConfig(), nil)
		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
		msg.TransInfo().TransIntInfo()[transmeta.ToMethod] = "to-method"
		msg.TransInfo().TransStrInfo()[codec.StrKeyMetaData] = `{"key":["v1", "v2"]}`

		h := NewServerTTHeaderFrameHandler()
		ctx, err := h.ServerReadHeader(context.Background(), msg)
		test.Assert(t, err == nil, err)

		md, ok := metadata.FromIncomingContext(ctx)
		test.Assert(t, ok, ok)
		test.Assert(t, md["key"][0] == "v1", md["key"][0])
		test.Assert(t, md["key"][1] == "v2", md["key"][1])
	})
}

func Test_serverTTHeaderFrameHandler_ReadTrailer(t *testing.T) {
	t.Run("trans-error-parse-failure", func(t *testing.T) {
		msg := remote.NewMessage(nil, nil, nil, remote.Stream, remote.Server)
		msg.TransInfo().TransIntInfo()[transmeta.TransCode] = "X"

		h := NewServerTTHeaderFrameHandler()
		err := h.ReadTrailer(context.Background(), msg)
		test.Assert(t, err != nil, err)
		transErr, ok := err.(*remote.TransError)
		test.Assert(t, ok, err)
		test.Assert(t, transErr.TypeID() == remote.InternalError, transErr)
	})

	t.Run("trans-error", func(t *testing.T) {
		msg := remote.NewMessage(nil, nil, nil, remote.Stream, remote.Server)
		msg.TransInfo().TransIntInfo()[transmeta.TransCode] = "999"
		msg.TransInfo().TransIntInfo()[transmeta.TransMessage] = "client err"

		h := NewServerTTHeaderFrameHandler()
		err := h.ReadTrailer(context.Background(), msg)
		test.Assert(t, err != nil, err)
		transErr, ok := err.(*remote.TransError)
		test.Assert(t, ok, err)
		test.Assert(t, transErr.TypeID() == 999, transErr)
		test.Assert(t, transErr.Error() == "client err", transErr)
	})
}

func Test_serverTTHeaderFrameHandler_WriteTrailer(t *testing.T) {
	t.Run("trans-error", func(t *testing.T) {
		transErr := remote.NewTransError(999, errors.New("client err"))
		msg := remote.NewMessage(transErr, nil, nil, remote.Stream, remote.Server)

		h := NewClientTTHeaderFrameHandler()
		err := h.WriteTrailer(context.Background(), msg)
		test.Assert(t, err == nil, err)

		intInfo := msg.TransInfo().TransIntInfo()
		test.Assert(t, len(intInfo) == 2)
		test.Assert(t, intInfo[transmeta.TransCode] == strconv.Itoa(int(transErr.TypeID())))
		test.Assert(t, intInfo[transmeta.TransMessage] == transErr.Error())
	})
	t.Run("biz-err", func(t *testing.T) {
		ivk := rpcinfo.NewInvocation("", "")
		bizErr := kerrors.NewBizStatusErrorWithExtra(999, "biz err", map[string]string{"k": "v"})
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, nil, nil)
		ri.Invocation().(rpcinfo.InvocationSetter).SetBizStatusErr(bizErr)
		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
		h := NewServerTTHeaderFrameHandler()
		err := h.WriteTrailer(context.Background(), msg)
		test.Assert(t, err == nil, err)

		strInfo := msg.TransInfo().TransStrInfo()
		newIvk := rpcinfo.NewInvocation("", "")
		newRI := rpcinfo.NewRPCInfo(nil, nil, newIvk, rpcinfo.NewRPCConfig(), nil)
		err = header_transmeta.ParseBizStatusErrorToRPCInfo(strInfo, newRI)
		test.Assert(t, err == nil, err)

		gotBizErr := newRI.Invocation().BizStatusErr()
		test.Assert(t, gotBizErr != nil, gotBizErr)
		test.Assert(t, gotBizErr.BizStatusCode() == bizErr.BizStatusCode(), bizErr)
		test.Assert(t, gotBizErr.BizMessage() == bizErr.BizMessage(), bizErr)
		test.Assert(t, gotBizErr.BizExtra()["k"] == "v", bizErr)
	})
}
