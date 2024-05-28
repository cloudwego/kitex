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

// TODO: This is for generic server
//type StreamingService interface {
//	// ClientStreamingGenericCall handles the client streaming generic call
//	ClientStreamingGenericCall(stream genericserver.ClientStreaming) (err error)
//	// ServerStreamingGenericCall handles the server streaming generic call
//	ServerStreamingGenericCall(req interface{}, stream genericserver.ServerStreaming) (err error)
//	// BidirectionalStreamingGenericCall handles the bidirectional streaming generic call
//	BidirectionalStreamingGenericCall(stream genericserver.BidirectionalStreaming) (err error)
//}

func StreamingServiceInfo(pcType serviceinfo.PayloadCodec, codec serviceinfo.CodecInfo) *serviceinfo.ServiceInfo {
	return newClientStreamingServiceInfo(pcType, codec)
}

// TODO: refactor
func newClientStreamingServiceInfo(pcType serviceinfo.PayloadCodec, codec serviceinfo.CodecInfo) *serviceinfo.ServiceInfo {
	//handlerType := (*StreamingService)(nil)

	// TODO: codec == nil check for binary generic
	//var methods map[string]serviceinfo.MethodInfo
	//var svcName string
	//
	//if codec == nil {
	//	svcName = serviceinfo.GenericService
	//	methods = map
	//}

	methods := map[string]serviceinfo.MethodInfo{
		serviceinfo.GenericClientStreamingMethod: serviceinfo.NewMethodInfo(
			// TODO: create handler when generic server supports grpc streaming
			nil,
			//clientStreamingCallHandler,
			func() interface{} { return &Args{inner: codec.GetMessageReaderWriter()} },
			func() interface{} { return &Result{inner: codec.GetMessageReaderWriter()} },
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingClient),
		),
		serviceinfo.GenericServerStreamingMethod: serviceinfo.NewMethodInfo(
			nil,
			//serverStreamingCallHandler,
			func() interface{} { return &Args{inner: codec.GetMessageReaderWriter()} },
			func() interface{} { return &Result{inner: codec.GetMessageReaderWriter()} },
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingServer),
		),
		serviceinfo.GenericBidirectionalStreamingMethod: serviceinfo.NewMethodInfo(
			nil,
			//bidirectionalStreamingCallHandler,
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

// TODO: the followings are for generic server
//func clientStreamingCallHandler(ctx context.Context, handler, arg, result interface{}) error {
//	st := arg.(*streaming.Args).Stream
//	stream := genericserver.NewClientStreamingServer(st)
//	return handler.(StreamingService).ClientStreamingGenericCall(stream)
//}
//
//func serverStreamingCallHandler(ctx context.Context, handler, arg, result interface{}) error {
//	st := arg.(*streaming.Args).Stream
//	stream := genericserver.NewServerStreamingServer(st)
//	req := new(Args)
//	if err := st.RecvMsg(req); err != nil {
//		return err
//	}
//	return handler.(StreamingService).ServerStreamingGenericCall(req, stream)
//}
//
//func bidirectionalStreamingCallHandler(ctx context.Context, handler, arg, result interface{}) error {
//	st := arg.(*streaming.Args).Stream
//	stream := genericserver.NewBidirectionalStreamingServer(st)
//	return handler.(StreamingService).BidirectionalStreamingGenericCall(stream)
//}

/*
svr, err := genericserver.NewStreamServer()
svr.RegisterService("serviceName", handler)

func (gs *genericServer) RegisterService(idlServiceName, streamingType, methodName[], handler) {
	// TODO: create svcInfo
	svr.RegisterService(svcInfo, handler)
}

svr.RegisterService(svcInfo, handler)

/*
func serviceInfo() *kitex.ServiceInfo {
	return routeGuideServiceInfo
}

var routeGuideServiceInfo = NewServiceInfo()



func NewServiceInfo() *kitex.ServiceInfo {
	serviceName := "RouteGuide"
	handlerType := (*routeguide.RouteGuide)(nil)
	methods := map[string]kitex.MethodInfo{
		"GetFeature":   kitex.NewMethodInfo(getFeatureHandler, newGetFeatureArgs, newGetFeatureResult, false),
		"ListFeatures": kitex.NewMethodInfo(listFeaturesHandler, newListFeaturesArgs, newListFeaturesResult, false),
		"RecordRoute":  kitex.NewMethodInfo(recordRouteHandler, newRecordRouteArgs, newRecordRouteResult, false),
		"RouteChat":    kitex.NewMethodInfo(routeChatHandler, newRouteChatArgs, newRouteChatResult, false),
	}
	extra := map[string]interface{}{
		"PackageName":     "routeguide",
		"ServiceFilePath": ``,
	}
	extra["streaming"] = true
	svcInfo := &kitex.ServiceInfo{
		ServiceName:     serviceName,
		HandlerType:     handlerType,
		Methods:         methods,
		PayloadCodec:    kitex.Protobuf,
		KiteXGenVersion: "v1.13.3",
		Extra:           extra,
	}
	return svcInfo
}
*/
