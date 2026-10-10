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

	headers := remote.AccessHeaders(transInfo)
	if headers.Primary() {
		writeClientIntHeaders(msg, headers.SetInt)
	} else {
		hd := make(map[uint16]string, 5)
		writeClientIntHeaders(msg, func(k uint16, v string) { hd[k] = v })
		transInfo.PutTransIntInfo(hd)
	}
	if idlSvcName := getIDLSvcName(ri); idlSvcName != "" {
		headers.SetString(transmeta.HeaderIDLServiceName, idlSvcName)
	}
	return ctx, nil
}

func getIDLSvcName(ri rpcinfo.RPCInfo) string {
	svcInfo, _ := ri.Invocation().Extra(rpcinfo.InvocationServiceInfoKey).(*serviceinfo.ServiceInfo)
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
	headers := remote.AccessHeaders(msg.TransInfo())

	bizErr, err := parseBizStatusErr(headers.String)
	if err != nil {
		return ctx, err
	}
	if setter, ok := ri.Invocation().(rpcinfo.InvocationSetter); ok && bizErr != nil {
		setter.SetBizStatusErr(bizErr)
	}
	if val, ok := headers.StringValue(transmeta.HeaderConnectionReadyToReset); ok {
		if ei := rpcinfo.AsTaggable(ri.To()); ei != nil {
			ei.SetTag(rpcinfo.ConnResetTag, val)
		}
	}
	return ctx, nil
}

func ParseBizStatusErr(strInfo map[string]string) (kerrors.BizStatusErrorIface, error) {
	return parseBizStatusErr(func(k string) string { return strInfo[k] })
}

func parseBizStatusErr(get func(string) string) (kerrors.BizStatusErrorIface, error) {
	if code, err := strconv.ParseInt(get(bizStatus), 10, 32); err == nil && code != 0 {
		if bizExtra := get(bizExtra); bizExtra != "" {
			extra, err := utils.JSONStr2Map(bizExtra)
			if err != nil {
				return nil, fmt.Errorf("malformed header info, extra: %s", bizExtra)
			}
			return kerrors.NewBizStatusErrorWithExtra(int32(code), get(bizMessage), extra), nil
		} else {
			return kerrors.NewBizStatusError(int32(code), get(bizMessage)), nil
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
	headers := remote.AccessHeaders(msg.TransInfo())

	ci := rpcinfo.AsMutableEndpointInfo(ri.From())
	if ci != nil {
		if v := headers.Int(transmeta.FromService); v != "" {
			ci.SetServiceName(v)
		}
		if v := headers.Int(transmeta.FromMethod); v != "" {
			ci.SetMethod(v)
		}
	}

	if cfg := rpcinfo.AsMutableRPCConfig(ri.Config()); cfg != nil {
		timeout := headers.Int(transmeta.RPCTimeout)
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
	h := remote.AccessHeaders(msg.TransInfo())
	h.SetInt(transmeta.MsgType, strconv.Itoa(int(msg.MessageType())))
	if bizErr := ri.Invocation().BizStatusErr(); bizErr != nil {
		h.SetString(bizStatus, strconv.Itoa(int(bizErr.BizStatusCode())))
		h.SetString(bizMessage, bizErr.BizMessage())
		if len(bizErr.BizExtra()) != 0 {
			extra, _ := utils.Map2JSONStr(bizErr.BizExtra())
			h.SetString(bizExtra, extra)
		}
	}
	if val, ok := ri.To().Tag(rpcinfo.ConnResetTag); ok {
		h.SetString(transmeta.HeaderConnectionReadyToReset, val)
	}

	return ctx, nil
}

func isTTHeader(msg remote.Message) bool {
	transProto := msg.RPCInfo().Config().TransportProtocol()
	return transProto&transport.TTHeader == transport.TTHeader
}

func writeClientIntHeaders(msg remote.Message, set func(uint16, string)) {
	ri := msg.RPCInfo()
	set(transmeta.FromService, ri.From().ServiceName())
	set(transmeta.FromMethod, ri.From().Method())
	set(transmeta.ToService, ri.To().ServiceName())
	set(transmeta.ToMethod, ri.To().Method())
	set(transmeta.MsgType, strconv.Itoa(int(msg.MessageType())))
	if ri.Config().TransportProtocol()&transport.Framed == transport.Framed {
		set(transmeta.TransportType, framedTransportType)
	} else {
		set(transmeta.TransportType, unframedTransportType)
	}

	cfg := rpcinfo.AsMutableRPCConfig(ri.Config())
	if cfg.IsRPCTimeoutLocked() {
		set(transmeta.RPCTimeout, strconv.Itoa(int(ri.Config().RPCTimeout().Milliseconds())))
	}
	if cfg.IsConnectTimeoutLocked() {
		set(transmeta.ConnectTimeout, strconv.Itoa(int(ri.Config().ConnectTimeout().Milliseconds())))
	}
}
