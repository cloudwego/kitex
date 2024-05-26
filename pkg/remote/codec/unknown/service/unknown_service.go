package unknown

import (
	"context"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

const (
	// UnknownService name
	UnknownService = "$UnknownService" // private as "$"
	// UnknownCall name
	UnknownMethod = "$UnknownCall"
)

type Args struct {
	Request     []byte
	Method      string
	ServiceName string
}

type Result struct {
	Success     []byte
	Method      string
	ServiceName string
}

type UnknownMethodService interface {
	UnknownMethodHandler(ctx context.Context, method string, request []byte) ([]byte, error)
}

func NewServiceInfo(pcType serviceinfo.PayloadCodec, service, method string) *serviceinfo.ServiceInfo {
	methods := map[string]serviceinfo.MethodInfo{
		method: serviceinfo.NewMethodInfo(callHandler, newServiceArgs, newServiceResult, false),
	}
	handlerType := (*UnknownMethodService)(nil)

	svcInfo := &serviceinfo.ServiceInfo{
		ServiceName:  service,
		HandlerType:  handlerType,
		Methods:      methods,
		PayloadCodec: pcType,
		Extra:        make(map[string]interface{}),
	}

	return svcInfo
}

func SetServiceInfo(svcInfo *serviceinfo.ServiceInfo, pcType serviceinfo.PayloadCodec, service, method string) {
	svcInfo.ServiceName = service
	svcInfo.PayloadCodec = pcType
	svcInfo.HandlerType = (*UnknownMethodService)(nil)
	methods := map[string]serviceinfo.MethodInfo{
		method: serviceinfo.NewMethodInfo(callHandler, newServiceArgs, newServiceResult, false),
	}
	svcInfo.Methods = methods

}

func callHandler(ctx context.Context, handler, arg, result interface{}) error {
	realArg := arg.(*Args)
	realResult := result.(*Result)
	realResult.Method = realArg.Method
	realResult.ServiceName = realArg.ServiceName
	success, err := handler.(UnknownMethodService).UnknownMethodHandler(ctx, realArg.Method, realArg.Request)
	if err != nil {
		return err
	}
	realResult.Success = success
	return nil
}

func newServiceArgs() interface{} {
	return &Args{}
}

func newServiceResult() interface{} {
	return &Result{}
}
