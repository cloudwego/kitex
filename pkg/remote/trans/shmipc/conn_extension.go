package shmipc

import (
	"context"
	"errors"
	"net"
	"syscall"
	"time"

	"github.com/cloudwego/netpoll"
	"github.com/cloudwego/shmipc-go"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans"
	knetpoll "github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
)

const (
	shmIPCOn  = "1"
	shmIPCOff = "0"
)

func newShmIPCExtension() trans.Extension {
	return &shmipcExtension{fallbackExt: knetpoll.NewNetpollConnExtension()}
}

type shmipcExtension struct {
	fallbackExt trans.Extension
}

func (e *shmipcExtension) IsRemoteClosedErr(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, shmipc.ErrStreamClosed) || errors.Is(err, netpoll.ErrConnClosed) || errors.Is(err, syscall.EPIPE)
}

func (e *shmipcExtension) SetReadTimeout(ctx context.Context, conn net.Conn, cfg rpcinfo.RPCConfig, role remote.RPCRole) {
	if stream, ok := conn.(*shmipc.Stream); ok {
		readTimeout := trans.GetReadTimeout(cfg)
		// just set read timeout for client
		// block read in shmipc in server side cannot set read timeout, it will lead read timeout error for idle stream
		if role == remote.Client && readTimeout > 0 {
			stream.SetReadDeadline(time.Now().Add(readTimeout))
		}
	} else {
		e.fallbackExt.SetReadTimeout(ctx, conn, cfg, role)
	}
}

func (e *shmipcExtension) NewWriteByteBuffer(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer {
	if stream, ok := conn.(*shmipc.Stream); ok {
		setShmIPCTag(shmIPCOn, msg)
		return newWriterByteBuffer(stream)
	}
	setShmIPCTag(shmIPCOff, msg)
	return e.fallbackExt.NewWriteByteBuffer(ctx, conn, msg)
}

func (e *shmipcExtension) NewReadByteBuffer(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer {
	if stream, ok := conn.(*shmipc.Stream); ok {
		return newReaderByteBuffer(stream)
	}
	return e.fallbackExt.NewReadByteBuffer(ctx, conn, msg)
}

func (e *shmipcExtension) IsTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	tt := errors.Is(err, shmipc.ErrTimeout)
	if !tt {
		tt = e.fallbackExt.IsTimeoutErr(err)
	}
	return tt
}

func setShmIPCTag(shmIPCStat string, msg remote.Message) {
	if msg.RPCRole() == remote.Client {
		if remote := remoteinfo.AsRemoteInfo(msg.RPCInfo().To()); remote != nil {
			remote.SetTag(rpcinfo.ShmIPCTag, shmIPCStat)
		}
	} else {
		if remote := rpcinfo.AsMutableEndpointInfo(msg.RPCInfo().From()); remote != nil {
			remote.SetTag(rpcinfo.ShmIPCTag, shmIPCStat)
		}
	}
}

func (e *shmipcExtension) ReleaseBuffer(buffer remote.ByteBuffer, err error) error {
	if buffer != nil {
		return buffer.Release(err)
	}
	return nil
}
