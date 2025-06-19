package code

import (
	"errors"
	"flag"
	"fmt"
)

// ToolExitCode defines the enumeration type for tool exit codes.
type ToolExitCode int

const (
	// ToolSuccess indicates that the tool has run successfully and returned.
	ToolSuccess ToolExitCode = iota

	ToolExecuteFailed

	ToolArgsError
	// ToolVersionCheckFailed indicates that the tool's version check has failed.
	ToolVersionCheckFailed
	ToolInterrupted
	ToolInitFailed
)

var (
	ErrExitZeroInterrupted = fmt.Errorf("os.Exit(%d)", ToolInterrupted)
	ErrVersionCheckFailed  = fmt.Errorf("os.Exit(%d)", ToolVersionCheckFailed)
)

func WrapExitCode(ret int) (error, ToolExitCode) {
	switch ret {
	case 0:
		return nil, ToolSuccess
	case 1:
		return fmt.Errorf("tool run failed"), ToolExecuteFailed
	default:
		return fmt.Errorf("unexpected return value: %d", ret), ToolExecuteFailed
	}
}

func Good(ret ToolExitCode) bool {
	return ret == ToolSuccess || ret == ToolInterrupted
}

func IsInterrupted(err error) bool {
	return errors.Is(err, flag.ErrHelp) || errors.Is(err, ErrExitZeroInterrupted)
}

func IsVersionCheckFailed(err error) bool {
	return errors.Is(err, ErrVersionCheckFailed)
}
