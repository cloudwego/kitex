package jsonrpc_test

import (
	"context"
	"github.com/cloudwego/kitex/streamx"
	"io"
	"log"
)

type serviceImpl struct{}

func (si *serviceImpl) Unary(ctx context.Context, req *Request) (*Response, error) {
	resp := &Response{Message: req.Message}
	log.Printf("Server Unary: req={%v} resp={%v}", req, resp)
	return resp, nil
}

func (si *serviceImpl) ClientStream(ctx context.Context, stream streamx.ClientStreamingServer[Request, Response]) error {
	for {
		req, err := stream.Recv(ctx)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		log.Printf("Server ClientStream: req={%v}", req)
	}
}

func (si *serviceImpl) ServerStream(ctx context.Context, req *Request, stream streamx.ServerStreamingServer[Response]) error {
	log.Printf("Server ServerStream: req={%v}", req)
	for i := 0; i < 3; i++ {
		resp := new(Response)
		resp.Message = req.Message
		err := stream.Send(ctx, resp)
		if err != nil {
			return err
		}
		log.Printf("Server ServerStream: resp={%v}", resp)
	}
	return nil
}

func (si *serviceImpl) BidiStream(ctx context.Context, stream streamx.BidiStreamingServer[Request, Response]) error {
	for {
		req, err := stream.Recv(ctx)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		resp := new(Response)
		resp.Message = req.Message
		err = stream.Send(ctx, resp)
		if err != nil {
			return err
		}
		log.Printf("Server BidiStream: req={%v} resp={%v}", req, resp)
	}
}
