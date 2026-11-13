//go:build !go1.21

package remote

type headerBlockPins struct{}

func newHeaderBlockPins() *headerBlockPins { return nil }

func (*headerBlockPins) owned() bool { return false }
func (*headerBlockPins) pin([]byte)  {}
func (*headerBlockPins) release()    {}
