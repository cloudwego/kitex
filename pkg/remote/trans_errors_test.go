package remote

import (
	"errors"
	"io"
	"testing"

	"github.com/jackedelic/kitex/internal/test"
)

func TestTransError(t *testing.T) {
	errMsg := "mock err"
	transErr := NewTransError(InternalError, io.ErrShortWrite)
	test.Assert(t, errors.Is(transErr, io.ErrShortWrite))

	transErr = NewTransError(InternalError, NewTransErrorWithMsg(100, errMsg))
	uwErr, ok := errors.Unwrap(transErr).(*TransError)
	test.Assert(t, ok)
	test.Assert(t, uwErr.TypeID() == 100)
	test.Assert(t, transErr.Error() == errMsg, transErr.Error())
}
