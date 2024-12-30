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

package transmeta

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/transmeta"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/utils"
	"github.com/cloudwego/kitex/transport"
)

const (
	framedTransportType   = "framed"
	unframedTransportType = "unframed"

	// for biz error
	bizStatus  = "biz-status"
	bizMessage = "biz-message"
	bizExtra   = "biz-extra"
)

// TTHeader handlers.
var (
	ClientTTHeaderHandler remote.MetaHandler = &clientTTHeaderHandler{}
	ServerTTHeaderHandler remote.MetaHandler = &serverTTHeaderHandler{}
)

// clientTTHeaderHandler implement remote.MetaHandler
type clientTTHeaderHandler struct{}

// WriteMeta of clientTTHeaderHandler writes headers of TTHeader protocol to transport
func (ch *clientTTHeaderHandler) WriteMeta(ctx context.Context, msg remote.Message) (context.Context, error) {
	if !isTTHeader(msg) {
		return ctx, nil
	}
	ri := msg.RPCInfo()
	transInfo := msg.TransInfo()

	hd := map[uint16]string{
		transmeta.FromService: ri.From().ServiceName(),
		transmeta.FromMethod:  ri.From().Method(),
		transmeta.ToService:   ri.To().ServiceName(),
		transmeta.ToMethod:    ri.To().Method(),
		transmeta.MsgType:     strconv.Itoa(int(msg.MessageType())),
	}
	if msg.ProtocolInfo().TransProto&transport.Framed == transport.Framed {
		hd[transmeta.TransportType] = framedTransportType
	} else {
		hd[transmeta.TransportType] = unframedTransportType
	}

	cfg := rpcinfo.AsMutableRPCConfig(ri.Config())
	if cfg.IsRPCTimeoutLocked() {
		hd[transmeta.RPCTimeout] = strconv.Itoa(int(ri.Config().RPCTimeout().Milliseconds()))
	}
	if cfg.IsConnectTimeoutLocked() {
		hd[transmeta.ConnectTimeout] = strconv.Itoa(int(ri.Config().ConnectTimeout().Milliseconds()))
	}
	transInfo.PutTransIntInfo(hd)
	if idlSvcName := getIDLSvcName(msg.ServiceInfo(), ri); idlSvcName != "" {
		if strInfo := transInfo.TransStrInfo(); strInfo != nil {
			strInfo[transmeta.HeaderIDLServiceName] = idlSvcName
		} else {
			transInfo.PutTransStrInfo(map[string]string{transmeta.HeaderIDLServiceName: idlSvcName})
		}
	}
	return ctx, nil
}

func getIDLSvcName(svcInfo *serviceinfo.ServiceInfo, ri rpcinfo.RPCInfo) string {
	idlSvcName := ri.Invocation().ServiceName()
	// for combine service, idlSvcName may not be the same as server's service name
	var isCombineService bool
	if svcInfo != nil {
		val, exists := svcInfo.Extra["combine_service"]
		if exists {
			isCombineService, _ = val.(bool)
		}
	}
	// generic service name shouldn't be written to header
	if idlSvcName != serviceinfo.GenericService && !isCombineService {
		return idlSvcName
	}
	return ""
}

// ReadMeta of clientTTHeaderHandler reads headers of TTHeader protocol from transport
func (ch *clientTTHeaderHandler) ReadMeta(ctx context.Context, msg remote.Message) (context.Context, error) {
	if !isTTHeader(msg) {
		return ctx, nil
	}
	ri := msg.RPCInfo()
	transInfo := msg.TransInfo()
	strInfo := transInfo.TransStrInfo()

	bizErr, err := ParseBizStatusErr(strInfo)
	if err != nil {
		return ctx, err
	}
	if setter, ok := ri.Invocation().(rpcinfo.InvocationSetter); ok && bizErr != nil {
		setter.SetBizStatusErr(bizErr)
	}
	if val, ok := strInfo[transmeta.HeaderConnectionReadyToReset]; ok {
		if ei := rpcinfo.AsTaggable(ri.To()); ei != nil {
			ei.SetTag(rpcinfo.ConnResetTag, val)
		}
	}
	return ctx, nil
}

func ParseBizStatusErr(strInfo map[string]string) (kerrors.BizStatusErrorIface, error) {
	if code, err := strconv.ParseInt(strInfo[bizStatus], 10, 32); err == nil && code != 0 {
		if bizExtra := strInfo[bizExtra]; bizExtra != "" {
			extra, err := utils.JSONStr2Map(bizExtra)
			if err != nil {
				return nil, fmt.Errorf("malformed header info, extra: %s", bizExtra)
			}
			return kerrors.NewBizStatusErrorWithExtra(int32(code), strInfo[bizMessage], extra), nil
		} else {
			return kerrors.NewBizStatusError(int32(code), strInfo[bizMessage]), nil
		}
	}
	return nil, nil
}

// serverTTHeaderHandler implement remote.MetaHandler
type serverTTHeaderHandler struct{}

// ReadMeta of serverTTHeaderHandler reads headers of TTHeader protocol to transport
func (sh *serverTTHeaderHandler) ReadMeta(ctx context.Context, msg remote.Message) (context.Context, error) {
	if !isTTHeader(msg) {
		return ctx, nil
	}
	ri := msg.RPCInfo()
	transInfo := msg.TransInfo()
	intInfo := transInfo.TransIntInfo()

	ci := rpcinfo.AsMutableEndpointInfo(ri.From())
	if ci != nil {
		if v := intInfo[transmeta.FromService]; v != "" {
			ci.SetServiceName(v)
		}
		if v := intInfo[transmeta.FromMethod]; v != "" {
			ci.SetMethod(v)
		}
	}

	if cfg := rpcinfo.AsMutableRPCConfig(ri.Config()); cfg != nil {
		timeout := intInfo[transmeta.RPCTimeout]
		if timeoutMS, err := strconv.Atoi(timeout); err == nil {
			cfg.SetRPCTimeout(time.Duration(timeoutMS) * time.Millisecond)
		}
	}
	return ctx, nil
}

// WriteMeta of serverTTHeaderHandler writes headers of TTHeader protocol to transport
func (sh *serverTTHeaderHandler) WriteMeta(ctx context.Context, msg remote.Message) (context.Context, error) {
	if !isTTHeader(msg) {
		return ctx, nil
	}
	ri := msg.RPCInfo()
	transInfo := msg.TransInfo()
	intInfo := transInfo.TransIntInfo()
	strInfo := transInfo.TransStrInfo()

	intInfo[transmeta.MsgType] = strconv.Itoa(int(msg.MessageType()))

	if bizErr := ri.Invocation().BizStatusErr(); bizErr != nil {
		strInfo[bizStatus] = strconv.Itoa(int(bizErr.BizStatusCode()))
		strInfo[bizMessage] = bizErr.BizMessage()
		if len(bizErr.BizExtra()) != 0 {
			strInfo[bizExtra], _ = utils.Map2JSONStr(bizErr.BizExtra())
		}
	}
	if val, ok := ri.To().Tag(rpcinfo.ConnResetTag); ok {
		strInfo[transmeta.HeaderConnectionReadyToReset] = val
	}

	return ctx, nil
}

func isTTHeader(msg remote.Message) bool {
	transProto := msg.ProtocolInfo().TransProto
	return transProto&transport.TTHeader == transport.TTHeader
}
