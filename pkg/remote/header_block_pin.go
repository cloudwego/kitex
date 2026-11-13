//go:build go1.21

package remote

import "runtime"

type headerBlockPins struct{ pinner runtime.Pinner }

func newHeaderBlockPins() *headerBlockPins {
	p := new(headerBlockPins)
	runtime.SetFinalizer(p, (*headerBlockPins).release)
	return p
}

func (p *headerBlockPins) owned() bool     { return p != nil }
func (p *headerBlockPins) pin(data []byte) { p.pinner.Pin(&data[0]) }
func (p *headerBlockPins) release()        { p.pinner.Unpin() }
