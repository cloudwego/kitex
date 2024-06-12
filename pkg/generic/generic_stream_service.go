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

package generic

import (
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

func StreamingServiceInfo(pcType serviceinfo.PayloadCodec, codec serviceinfo.CodecInfo) *serviceinfo.ServiceInfo {
	return newClientStreamingServiceInfo(pcType, codec)
}

func newClientStreamingServiceInfo(pcType serviceinfo.PayloadCodec, codec serviceinfo.CodecInfo) *serviceinfo.ServiceInfo {
	if codec == nil {
		// TODO: support grpc binary generic
		return nil
	}

	methods := map[string]serviceinfo.MethodInfo{
		serviceinfo.GenericClientStreamingMethod: serviceinfo.NewMethodInfo(
			nil,
			func() interface{} { return &Args{inner: codec.GetMessageReaderWriter()} },
			func() interface{} { return &Result{inner: codec.GetMessageReaderWriter()} },
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingClient),
		),
		serviceinfo.GenericServerStreamingMethod: serviceinfo.NewMethodInfo(
			nil,
			func() interface{} { return &Args{inner: codec.GetMessageReaderWriter()} },
			func() interface{} { return &Result{inner: codec.GetMessageReaderWriter()} },
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingServer),
		),
		serviceinfo.GenericBidirectionalStreamingMethod: serviceinfo.NewMethodInfo(
			nil,
			func() interface{} { return &Args{inner: codec.GetMessageReaderWriter()} },
			func() interface{} { return &Result{inner: codec.GetMessageReaderWriter()} },
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingBidirectional),
		),
		serviceinfo.GenericMethod: serviceinfo.NewMethodInfo(
			callHandler,
			func() interface{} { return &Args{inner: codec.GetMessageReaderWriter()} },
			func() interface{} { return &Result{inner: codec.GetMessageReaderWriter()} },
			false,
		),
	}
	svcInfo := &serviceinfo.ServiceInfo{
		ServiceName:  codec.GetIDLServiceName(),
		Methods:      methods,
		PayloadCodec: pcType,
		Extra:        make(map[string]interface{}),
	}
	svcInfo.Extra["generic"] = true
	return svcInfo
}
