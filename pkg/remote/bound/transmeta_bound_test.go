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

package bound

import (
	"context"
	"github.com/cloudwego/kitex/pkg/consts"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
	"testing"

	"github.com/cloudwego/kitex/pkg/remote/mocks"
	"github.com/cloudwego/kitex/pkg/remote/trans/invoke"
	"github.com/stretchr/testify/assert"
)

// TestNewTransMetaHandler test NewTransMetaHandler function and assert the result not nil.
func TestNewTransMetaHandler(t *testing.T) {
	metaHandler1 := &mocks.MetaHandler{}
	metaHandler2 := &mocks.MetaHandler{}
	metaHandler3 := &mocks.MetaHandler{}
	mhs := []remote.MetaHandler{
		metaHandler1, metaHandler2, metaHandler3,
	}

	handler := NewTransMetaHandler(mhs)

	assert.NotNil(t, handler)
}

// TestTransMetaHandlerWrite test Write function of transMetaHandler.
func TestTransMetaHandlerWrite(t *testing.T) {
	t.Run("Test transMetahandler Write success", func(t *testing.T) {
		metaHandler1 := &mocks.MetaHandler{}
		metaHandler2 := &mocks.MetaHandler{}
		metaHandler3 := &mocks.MetaHandler{}
		mhs := []remote.MetaHandler{
			metaHandler1, metaHandler2, metaHandler3,
		}

		invokeMessage := invoke.NewMessage(nil, nil)
		remoteMessage := remote.NewMessage(
			nil, nil, nil, remote.Call, remote.Client)
		ctx := context.Background()

		handler := NewTransMetaHandler(mhs)

		metaHandler1.On("WriteMeta", ctx, remoteMessage).Return(context.Background(), nil)
		metaHandler2.On("WriteMeta", ctx, remoteMessage).Return(context.Background(), nil)
		metaHandler3.On("WriteMeta", ctx, remoteMessage).Return(context.Background(), nil)

		assert.NotNil(t, handler)
		ctx, err := handler.Write(ctx, invokeMessage, remoteMessage)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)
	})

	t.Run("Test TransMetahandler Write with error", func(t *testing.T) {
		metaHandler1 := &mocks.MetaHandler{}
		metaHandler2 := &mocks.MetaHandler{}
		metaHandler3 := &mocks.MetaHandler{}
		mhs := []remote.MetaHandler{
			metaHandler1, metaHandler2, metaHandler3,
		}

		invokeMessage := invoke.NewMessage(nil, nil)
		remoteMessage := remote.NewMessage(
			nil, nil, nil, remote.Call, remote.Client)
		ctx := context.Background()

		metaHandler1.On("WriteMeta", ctx, remoteMessage).Return(context.Background(), errFoo)
		metaHandler2.On("WriteMeta", ctx, remoteMessage).Return(context.Background(), nil)
		metaHandler3.On("WriteMeta", ctx, remoteMessage).Return(context.Background(), nil)

		handler := NewTransMetaHandler(mhs)
		assert.NotNil(t, handler)

		ctx, err := handler.Write(context.Background(), invokeMessage, remoteMessage)
		assert.NotNil(t, ctx)
		assert.Equal(t, errFoo, err)
	})
}

// TestTransMetaHandlerOnMessage test OnMessage function of transMetaHandler, on client side and on server side.
func TestTransMetaHandlerOnMessage(t *testing.T) {
	t.Run("Test TransMetaHandler OnMessage success client side", func(t *testing.T) {
		metaHandler1 := &mocks.MetaHandler{}
		metaHandler2 := &mocks.MetaHandler{}
		metaHandler3 := &mocks.MetaHandler{}
		mhs := []remote.MetaHandler{
			metaHandler1, metaHandler2, metaHandler3,
		}

		args := remote.NewMessage(
			nil, nil, nil, remote.Call, remote.Client)
		result := remote.NewMessage(
			nil, nil, nil, remote.Reply, remote.Client)
		ctx := context.Background()

		metaHandler1.On("ReadMeta", ctx, result).Return(context.Background(), nil)
		metaHandler2.On("ReadMeta", ctx, result).Return(context.Background(), nil)
		metaHandler3.On("ReadMeta", ctx, result).Return(context.Background(), nil)

		handler := NewTransMetaHandler(mhs)

		assert.NotNil(t, handler)
		ctx, err := handler.OnMessage(ctx, args, result)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)
	})

	t.Run("Test TransMetaHandler OnMessage success server side", func(t *testing.T) {
		metaHandler1 := &mocks.MetaHandler{}
		metaHandler2 := &mocks.MetaHandler{}
		metaHandler3 := &mocks.MetaHandler{}
		mhs := []remote.MetaHandler{
			metaHandler1, metaHandler2, metaHandler3,
		}
		ink := rpcinfo.NewInvocation("", "mock")
		to := remoteinfo.NewRemoteInfo(&rpcinfo.EndpointBasicInfo{}, "")
		ri := rpcinfo.NewRPCInfo(nil, to, ink, nil, nil)

		args := remote.NewMessage(
			nil, nil, ri, remote.Call, remote.Server)
		result := remote.NewMessage(
			nil, nil, nil, remote.Reply, remote.Server)
		ctx := context.Background()

		metaHandler1.On("ReadMeta", ctx, args).Return(context.Background(), nil)
		metaHandler2.On("ReadMeta", ctx, args).Return(context.Background(), nil)
		metaHandler3.On("ReadMeta", ctx, args).Return(context.Background(), nil)

		handler := NewTransMetaHandler(mhs)

		assert.NotNil(t, handler)
		ctx, err := handler.OnMessage(ctx, args, result)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)
		assert.Equal(t, args.RPCInfo().To().Method(), ctx.Value(consts.CtxKeyMethod))
	})
}

// TestGetValidMsg test getValidMsg function with message of server side and client side.
func TestGetValidMsg(t *testing.T) {
	t.Run("Test getValidMsg server side read args", func(t *testing.T) {
		args := remote.NewMessage(nil, nil, nil, remote.Call, remote.Server)
		result := remote.NewMessage(nil, nil, nil, remote.Reply, remote.Server)
		msg, isServer := getValidMsg(args, result)
		assert.Equal(t, true, isServer)
		assert.Equal(t, args, msg)
	})

	t.Run("Test getValidMsg client side read result", func(t *testing.T) {
		args := remote.NewMessage(nil, nil, nil, remote.Call, remote.Client)
		result := remote.NewMessage(nil, nil, nil, remote.Reply, remote.Client)
		msg, isServer := getValidMsg(args, result)
		assert.Equal(t, false, isServer)
		assert.Equal(t, result, msg)
	})
}

// TestTransMetaHandlerOnActive test OnActive function of transMetaHandler
func TestTransMetaHandlerOnActive(t *testing.T) {
	mhs := []remote.MetaHandler{
		&mocks.MetaHandler{},
	}
	ctx := context.Background()
	handler := NewTransMetaHandler(mhs)
	ctx, err := handler.OnActive(ctx, invoke.NewMessage(nil, nil))
	assert.Nil(t, err)
	assert.NotNil(t, ctx)
}

// TestTransMetaHandlerOnRead test OnRead function of transMetaHandler
func TestTransMetaHandlerOnRead(t *testing.T) {
	mhs := []remote.MetaHandler{
		&mocks.MetaHandler{},
	}
	ctx := context.Background()
	handler := NewTransMetaHandler(mhs)
	ctx, err := handler.OnRead(ctx, invoke.NewMessage(nil, nil))
	assert.Nil(t, err)
	assert.NotNil(t, ctx)
}

// TestTransMetaHandlerOnInactive test OnInactive function of transMetaHandler
func TestTransMetaHandlerOnInactive(t *testing.T) {
	mhs := []remote.MetaHandler{
		&mocks.MetaHandler{},
	}
	ctx := context.Background()
	handler := NewTransMetaHandler(mhs)
	ctx = handler.OnInactive(ctx, invoke.NewMessage(nil, nil))
	assert.NotNil(t, ctx)
}
