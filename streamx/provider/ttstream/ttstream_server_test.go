package ttstream_test

import (
	"context"
	"io"
	"log"
)

type pingpongService struct{}
type streamingService struct{}

func (si *pingpongService) PingPong(ctx context.Context, req *Request) (*Response, error) {
	resp := &Response{Type: req.Type, Message: req.Message}
	log.Printf("Server PingPong: req={%v} resp={%v}", req, resp)
	return resp, nil
}

func (si *streamingService) Unary(ctx context.Context, req *Request) (*Response, error) {
	resp := &Response{Type: req.Type, Message: req.Message}
	log.Printf("Server Unary: req={%v} resp={%v}", req, resp)
	return resp, nil
}

func (si *streamingService) ClientStream(ctx context.Context, stream ClientStreamingServer[Request, Response]) (res *Response, err error) {
	var msg string
	defer log.Printf("Server ClientStream end")
	for {
		req, err := stream.Recv(ctx)
		if err == io.EOF {
			res = new(Response)
			res.Message = msg
			return res, nil
		}
		if err != nil {
			return nil, err
		}
		msg = req.Message
		log.Printf("Server ClientStream: req={%v}", req)
	}
}

func (si *streamingService) ServerStream(ctx context.Context, req *Request, stream ServerStreamingServer[Response]) error {
	log.Printf("Server ServerStream: req={%v}", req)

	_ = stream.SetHeader(map[string]string{"key1": "val1"})
	_ = stream.SendHeader(map[string]string{"key2": "val2"})
	_ = stream.SetTrailer(map[string]string{"key1": "val1"})
	_ = stream.SetTrailer(map[string]string{"key2": "val2"})

	for i := 0; i < 3; i++ {
		resp := new(Response)
		resp.Type = int32(i)
		resp.Message = req.Message
		err := stream.Send(ctx, resp)
		if err != nil {
			return err
		}
		log.Printf("Server ServerStream: send resp={%v}", resp)
	}
	return nil
}

func (si *streamingService) BidiStream(ctx context.Context, stream BidiStreamingServer[Request, Response]) error {
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
