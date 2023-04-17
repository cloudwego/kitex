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

package test

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	dt "github.com/cloudwego/dynamicgo/thrift"
	dg "github.com/cloudwego/dynamicgo/thrift/generic"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/client/genericclient"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/server/genericserver"
)

func TestMain(m *testing.M)() {
	initDescriptor()
	initServer()
	initClient()
	m.Run()
}

var (
	ExampleReqDesc *dt.TypeDescriptor
	ExampleRespDesc *dt.TypeDescriptor
    StrDesc *dt.TypeDescriptor
    BaseLogidPath []dg.Path
    DynamicgoOptions = &dg.Options{}
)

func initDescriptor() {
	sdesc, err := dt.NewDescritorFromPath(context.Background(), "idl/example.thrift")
	if err != nil {
		panic(err)
	}
	ExampleReqDesc = sdesc.Functions()["ExampleMethod"].Request().Struct().FieldById(1).Type()
	ExampleRespDesc = sdesc.Functions()["ExampleMethod"].Response().Struct().FieldById(0).Type()
	StrDesc = ExampleReqDesc.Struct().FieldById(1).Type()
	BaseLogidPath = []dg.Path{dg.NewPathFieldName("Base"), dg.NewPathFieldName("LogID")}
}

func initServer() {
	// init special server
	addr, _ := net.ResolveTCPAddr("tcp", ":9009")
	g := generic.BinaryThriftGeneric()
	svr := genericserver.NewServer(new(ExampleValueServiceImpl), g, server.WithServiceAddr(addr))
	go func() {
		err := svr.Run()
		if err != nil {
			panic(err)
		}
	}()
	time.Sleep(500 * time.Millisecond)
}

var cli genericclient.Client

func initClient() {
	g := generic.BinaryThriftGeneric()
	genericCli, _ := genericclient.NewClient("destServiceName", g, client.WithHostPorts("127.0.0.1:9009"))
	cli = genericCli
}

// exampleRespPool gives the thrift DOM used for make thrift message and set its initial values
var exampleRespPool = sync.Pool{
	New: func() interface{} {
		return &dg.PathNode{
			Node: dg.NewTypedNode(thrift.STRUCT, 0, 0),
			Next: []dg.PathNode{
				{
					Path: dg.NewPathFieldId(1),
					Node: dg.NewNodeString(""),
				},
				{
					Path: dg.NewPathFieldId(2),
					Node: dg.NewNodeString(""),
				},
				{
					Path: dg.NewPathFieldId(255),
					Node: dg.NewTypedNode(thrift.STRUCT, 0, 0),
					Next: []dg.PathNode{
						{
							Path: dg.NewPathFieldId(1),
							Node: dg.NewNodeString(""),
						},
						{
							Path: dg.NewPathFieldId(2),
							Node: dg.NewNodeString("d.e.f"),
						},
						{
							Path: dg.NewPathFieldId(3),
							Node: dg.NewNodeString("127.0.0.1"),
						},
						{
							Path: dg.NewPathFieldId(4),
							Node: dg.NewNodeString("dynamicgo"),
						},
					},
				},
			},
		}
	},
}

// MakeExampleRespBinary make a Thrift-Binary-Encoding response using ExampleResp structed DOM
// Except msg, requre_field and logid, which are reset everytime, 
// all other fields keeps original values from exampleRespPool
func MakeExampleRespBinary(msg string, require_field string, logid string) ([]byte, error) {
	// get DOM from sync.Pool, which make sure no-reset fields keep same value
	dom := exampleRespPool.Get().(*dg.PathNode)

	// set biz-logic-related fields' values on the DOM
	dom.Field(1, DynamicgoOptions).Node = dg.NewNodeString(msg)
	dom.Field(2, DynamicgoOptions).Node = dg.NewNodeString(require_field)
	dom.Field(255, DynamicgoOptions).Field(1, DynamicgoOptions).Node = dg.NewNodeString(logid)

	// marshal DOM into thrift binary
	out, err := dom.Marshal(DynamicgoOptions)

	// recycle the DOM 
	exampleRespPool.Put(dom)
	return out, err
}

var exampleReqPool = sync.Pool{
	New: func() interface{} {
		return &dg.PathNode{
			Node: dg.NewTypedNode(thrift.STRUCT, 0, 0),
			Next: []dg.PathNode{
				{
					Path: dg.NewPathFieldId(1),
					Node: dg.NewNodeString("Hello"),
				},
				{
					Path: dg.NewPathFieldId(2),
					Node: dg.NewNodeInt32(1),
				},
				{
					Path: dg.NewPathFieldId(3),
					Node: dg.NewTypedNode(thrift.LIST, thrift.STRUCT, 0),
					Next: []dg.PathNode{
						{
							Path: dg.NewPathIndex(0),
							Node: dg.NewTypedNode(thrift.STRUCT, 0, 0),
							Next: []dg.PathNode{
								{
									Path: dg.NewPathFieldId(1),
									Node: dg.NewNodeString(""),
								},
							},
						},
					},
				},
				{
					Path: dg.NewPathFieldId(4),
					Node: dg.NewTypedNode(thrift.MAP, thrift.STRUCT, thrift.STRING),
					Next: []dg.PathNode{
						{
							Path: dg.NewPathStrKey("a"),
							Node:  dg.NewTypedNode(thrift.STRUCT, 0, 0),
							Next: []dg.PathNode{
								{
									Path: dg.NewPathFieldId(1),
									Node: dg.NewNodeString(""),
								},
							},
						},
					},
				},
				{
					Path: dg.NewPathFieldId(6),
					Node: dg.NewTypedNode(thrift.LIST,  thrift.I64, 0),
					Next: []dg.PathNode{
						{
							Path: dg.NewPathIndex(0),
							Node: dg.NewNodeInt64(1),
						},
						{
							Path: dg.NewPathIndex(1),
							Node: dg.NewNodeInt64(2),
						},
						{
							Path: dg.NewPathIndex(2),
							Node: dg.NewNodeInt64(3),
						},
					},
				},
				{
					Path: dg.NewPathFieldId(7),
					Node: dg.NewNodeBool(false),
				},
				{
					Path: dg.NewPathFieldId(255),
					Node: dg.NewTypedNode(thrift.STRUCT, 0, 0),
					Next: []dg.PathNode{
						{
							Path: dg.NewPathFieldId(1),
							Node: dg.NewNodeString(""),
						},
						{
							Path: dg.NewPathFieldId(2),
							Node: dg.NewNodeString("a.b.c"),
						},
						{
							Path: dg.NewPathFieldId(3),
							Node: dg.NewNodeString("127.0.0.1"),
						},
						{
							Path: dg.NewPathFieldId(4),
							Node: dg.NewNodeString("dynamicgo"),
						},
					},
				},
			},
		}
	},
}

// MakeExampleReqBinary make a Thrift-Binary-Encoding request using ExampleReq structed DOM
// Except B, A and logid, which are reset everytime, 
// all other fields keeps original values from exampleReqPool
func MakeExampleReqBinary(B bool, A string, logid string) ([]byte, error) {
	// get DOM from sync.Pool, which make sure no-reset fields keep same value
	dom := exampleReqPool.Get().(*dg.PathNode)

	// set biz-logic-related fields' values on the DOM
	dom.Next[2].Next[0].Next[0].Node = dg.NewNodeString(A)
	dom.Next[3].Next[0].Next[0].Node = dg.NewNodeString(A)
	dom.Next[5].Node = dg.NewNodeBool(B)
	dom.Next[6].Next[0].Node = dg.NewNodeString(logid)

	// marshal DOM into thrift binary
	out, err := dom.Marshal(DynamicgoOptions)

	// recycle the DOM 
	exampleReqPool.Put(dom)
	return out, err
}

const (
	Method = "ExampleMethod"
	ReqMsg = "pending"
	RespMsg = "ok"
)

// ExampleValueServiceImpl ...
type ExampleValueServiceImpl struct{}

// GenericCall ...
func (g *ExampleValueServiceImpl) GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error) {
	// get and unwrap body with message
	in := request.([]byte)

	// unwarp thrift message and get request body
	methodName, _, seqID, _, body, err := dt.UnwrapBinaryMessage(in)

	// biz logic
	resp, err := ExampleServerHandler(body)

	// wrap response as thrift REPLY message
	return dt.WrapBinaryBody(resp, methodName, dt.REPLY, 0, seqID)
}

// biz logic
func ExampleServerHandler(request []byte) (resp []byte, err error) {
	// wrap body as Value
	req := dg.NewValue(ExampleReqDesc, request)
	if err != nil {
		return nil, err
	}

	required_field := ""
	logid := ""
	// if B == true then get logid and required_field
	if b, err := req.FieldByName("B").Bool(); err == nil && b {
		if e := req.GetByPath(BaseLogidPath...); e.Error() != ""{
			return nil, e
		} else {
			logid, _ = e.String()
		}
		if a := req.FieldByName("TestMap").GetByStr("a"); a.Error() != "" {
			return nil, a
		} else {
			required_field, _ = a.FieldByName("Bar").String()
		}
	}

	// make response with checked values
	return MakeExampleRespBinary(RespMsg, required_field, logid)
}

func ThriftReflectExample(t *testing.T) {
	log_id := strconv.Itoa(rand.Int())

	// make a request body
	req, err := MakeExampleReqBinary(true, ReqMsg, log_id)
	test.Assert(t, err == nil, err)

	// wrap request as thrift CALL message
	buf, err := dt.WrapBinaryBody(req, Method, dt.CALL, 1, 0)
	test.Assert(t, err == nil, err)

	// generic call
	out, err := cli.GenericCall(context.Background(), Method, buf, callopt.WithRPCTimeout(1*time.Second))
	test.Assert(t, err == nil, err)

	// unwrap REPLY message and get resp body
	_, _, _, _, body, err := dt.UnwrapBinaryMessage(out.([]byte))
	test.Assert(t, err == nil, err)

	// biz logic...
	err = ExampleClientHandler(body, log_id)
	if err != nil {
		t.Fatal(err)
	}
}

var clientRespPool = sync.Pool{
	New: func() interface{} {
		return &dg.PathNode{}
	},
}

// biz logic...
func ExampleClientHandler(response []byte, log_id string) error {
	// make dynamicgo/generic.Node with body
	resp := dg.NewNode(dt.STRUCT, response)

	// check node values by Node APIs
	msg, err := resp.Field(1).String()
	if err != nil {
		return err
	}
	if  msg != RespMsg {
		return errors.New("msg does not match")
	}
	require_field, err := resp.Field(2).String()
	if err != nil {
		return err
	}
	if require_field != ReqMsg {
		return errors.New("require_field does not match")
	}

	// get dom from memory pool
	root := clientRespPool.Get().(*dg.PathNode)
	root.Node = resp

	// load **first layer** children
	err = root.Load(false, DynamicgoOptions)
	if err != nil {
		return err
	}
	// spew.Dump(root) // -- only root.Next is set 
	// check node values by PathNode APIs
	require_field2, err := root.Field(2, DynamicgoOptions).Node.String()
	if err != nil {
		return err
	}
	if require_field2 != ReqMsg {
		return errors.New("require_field2 does not match")
	}
	
	// // load **all layers** children
	// err = root.Load(true, DynamicgoOptions)
	// if err != nil {
	// 	return err
	// }

	// // spew.Dump(root) // -- every PathNode.Next will be set if it is a nesting-typed (LIST/SET/MAP/STRUCT)
	// // check node values by PathNode APIs
	// logid, err := root.Field(255, DynamicgoOptions).Field(1, DynamicgoOptions).Node.String()
	// if logid != log_id {
	// 	return errors.New("logid not match")
	// }

	// recycle DOM
	root.ResetValue()
	clientRespPool.Put(root)
	return nil
}


func BenchmarkThriftReflect(b *testing.B) {
	st := time.Now()
	for i := 0; i < 1000; i++ {
		log_id := strconv.Itoa(rand.Int())

		// make a request body
		req, err := MakeExampleReqBinary(true, ReqMsg, log_id)
		test.Assert(b, err == nil, err)

		// wrap request as thrift CALL message
		buf, err := dt.WrapBinaryBody(req, Method, dt.CALL, 1, 0)
		test.Assert(b, err == nil, err)

		// generic call
		out, err := cli.GenericCall(context.Background(), Method, buf, callopt.WithRPCTimeout(1*time.Second))
		test.Assert(b, err == nil, err)

		// unwrap REPLY message and get resp body
		_, _, _, _, body, err := dt.UnwrapBinaryMessage(out.([]byte))
		test.Assert(b, err == nil, err)

		// biz logic...
		err = ExampleClientHandler(body, log_id)
		test.Assert(b, err == nil, err)
	}
	et := time.Now()

	println("time cost: ", et.Sub(st).Microseconds(), "us")
}