package errors

import (
	"github.com/cloudwego/kitex/pkg/kerrors"
)

var (
	ErrStream               = kerrors.ErrStreaming.WithChild("stream-level")
	ErrUnexpectedHeader     = ErrStream.WithChild("unexpected header frame")
	ErrUnexpectedTrailer    = ErrStream.WithChild("stream read a unexpect trailer")
	ErrApplicationException = ErrStream.WithChild("application exception")
	ErrIllegalBizErr        = ErrStream.WithChild("illegal bizErr")

	ErrConnection   = kerrors.ErrStreaming.WithChild("connection-level")
	ErrIllegalFrame = ErrConnection.WithChild("frame does not conform to the ttheader specification")
	ErrTransport    = ErrConnection.WithChild("transport is closing")
)
