package ktrace

import (
	"context"
	"runtime/trace"
)

type (
	Task      = trace.Task
	Region    = trace.Region
	TraceType = string
)

const (
	Server           = "Server"
	ServerConnect    = "ServerConnect"
	ServerDisconnect = "ServerDisconnect"
	ServerHandle     = "ServerHandle" // format: ServerHandle_Method
	ServerRead       = "ServerRead"
	ServerWaitRead   = "ServerWaitRead"
	ServerWrite      = "ServerWrite"
	ServerDecode     = "ServerDecode"
	ServerEncode     = "ServerEncode"

	Client           = "Client"
	ClientCall       = "ClientCall" // format: ClientCall_Method
	ClientConnect    = "ClientConnect"
	ClientDisconnect = "ClientDisconnect"
	ClientRead       = "ClientRead"
	ClientWaitRead   = "ClientWaitRead"
	ClientWrite      = "ClientWrite"
	ClientDecode     = "ClientDecode"
	ClientEncode     = "ClientEncode"
)

func NewTask(pctx context.Context, taskType TraceType) (ctx context.Context, task *Task) {
	return trace.NewTask(pctx, taskType)
}

func NewRegion(ctx context.Context, regionType TraceType) (region *Region) {
	return trace.StartRegion(ctx, regionType)
}
