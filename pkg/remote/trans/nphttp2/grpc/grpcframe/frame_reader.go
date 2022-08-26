/*
 * Copyright 2021 The Go Authors. All rights reserved.
 *
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2022 CloudWeGo Authors.
 *
 * Code forked and modified from golang v1.17.4
 */

package grpcframe

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/netpoll"
	"golang.org/x/net/http/httpguts"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

// A Framer reads and writes Frames.
type Framer struct {
	errDetail error

	// lastHeaderStream is non-zero if the last frame was an
	// unfinished HEADERS/CONTINUATION.
	lastHeaderStream uint32
	lastFrame        http2.Frame

	reader      netpoll.Reader
	maxReadSize uint32

	writer io.Writer
	wbuf   []byte
	// maxWriteSize uint32 // zero means unlimited; TODO: implement

	// AllowIllegalWrites permits the Framer's Write methods to
	// write frames that do not conform to the HTTP/2 spec. This
	// permits using the Framer to test other HTTP/2
	// implementations' conformance to the spec.
	// If false, the Write methods will prefer to return an error
	// rather than comply.
	AllowIllegalWrites bool

	// AllowIllegalReads permits the Framer's ReadFrame method
	// to return non-compliant frames or frame orders.
	// This is for testing and permits using the Framer to test
	// other HTTP/2 implementations' conformance to the spec.
	// It is not compatible with ReadMetaHeaders.
	AllowIllegalReads bool

	// ReadMetaHeaders if non-nil causes ReadFrame to merge
	// HEADERS and CONTINUATION frames together and return
	// MetaHeadersFrame instead.
	ReadMetaHeaders *hpack.Decoder

	// MaxHeaderListSize is the http2 MAX_HEADER_LIST_SIZE.
	// It's used only if ReadMetaHeaders is set; 0 means a sane default
	// (currently 16MB)
	// If the limit is hit, MetaHeadersFrame.Truncated is set true.
	MaxHeaderListSize uint32

	frameCache *frameCache // nil if frames aren't reused (default)
}

func (fr *Framer) maxHeaderListSize() uint32 {
	if fr.MaxHeaderListSize == 0 {
		return 16 << 20 // sane default, per docs
	}
	return fr.MaxHeaderListSize
}

const (
	minMaxFrameSize = 1 << 14
	maxFrameSize    = 1<<24 - 1
)

// SetReuseFrames allows the Framer to reuse Frames.
// If called on a Framer, Frames returned by calls to ReadFrame are only
// valid until the next call to ReadFrame.
func (fr *Framer) SetReuseFrames() {
	if fr.frameCache != nil {
		return
	}
	fr.frameCache = &frameCache{}
}

type frameCache struct {
	dataFrame DataFrame
}

func (fc *frameCache) getDataFrame() *DataFrame {
	if fc == nil {
		return &DataFrame{}
	}
	return &fc.dataFrame
}

// NewFramer returns a Framer that writes frames to w and reads them from r.
func NewFramer(w io.Writer, r netpoll.Reader) *Framer {
	fr := &Framer{
		writer: w,
		reader: r,
	}
	fr.SetMaxReadFrameSize(maxFrameSize)
	return fr
}

// SetMaxReadFrameSize sets the maximum size of a frame
// that will be read by a subsequent call to ReadFrame.
// It is the caller's responsibility to advertise this
// limit with a SETTINGS frame.
func (fr *Framer) SetMaxReadFrameSize(v uint32) {
	if v > maxFrameSize {
		v = maxFrameSize
	}
	fr.maxReadSize = v
}

// ErrorDetail returns a more detailed error of the last error
// returned by Framer.ReadFrame. For instance, if ReadFrame
// returns a StreamError with code PROTOCOL_ERROR, ErrorDetail
// will say exactly what was invalid. ErrorDetail is not guaranteed
// to return a non-nil value and like the rest of the http2 package,
// its return value is not protected by an API compatibility promise.
// ErrorDetail is reset after the next call to ReadFrame.
func (fr *Framer) ErrorDetail() error {
	return fr.errDetail
}

// ReadFrame reads a single frame. The returned Frame is only valid
// until the next call to ReadFrame.
//
// If the frame is larger than previously set with SetMaxReadFrameSize, the
// returned error is ErrFrameTooLarge. Other errors may be of type
// ConnectionError, StreamError, or anything else from the underlying
// reader.
func (fr *Framer) ReadFrame() (http2.Frame, error) {
	fr.errDetail = nil

	fh, err := readFrameHeader(fr.reader)
	if err != nil {
		return nil, err
	}
	if fh.Length > fr.maxReadSize {
		return nil, http2.ErrFrameTooLarge
	}
	// payload := fr.getReadBuf(fh.Length)
	var f http2.Frame
	if fh.Type == http2.FrameData {
		f, err = parseDataFrame(fr.frameCache, fh, fr.reader)
	} else {
		var payload []byte
		payload, err = fr.reader.Next(int(fh.Length))
		if err != nil {
			return nil, err
		}
		f, err = typeFrameParser(fh.Type)(fr.frameCache, fh, payload)
	}
	if err != nil {
		if ce, ok := err.(connError); ok {
			return nil, fr.connError(ce.Code, ce.Reason)
		}
		return nil, err
	}
	if err = fr.checkFrameOrder(f); err != nil {
		return nil, err
	}
	if fh.Type == http2.FrameHeaders && fr.ReadMetaHeaders != nil {
		return fr.readMetaFrame(f.(*HeadersFrame))
	}
	return f, nil
}

// connError returns ConnectionError(code) but first
// stashes away a public reason to the caller can optionally relay it
// to the peer before hanging up on them. This might help others debug
// their implementations.
func (fr *Framer) connError(code http2.ErrCode, reason string) error {
	fr.errDetail = errors.New(reason)
	return http2.ConnectionError(code)
}

// checkFrameOrder reports an error if f is an invalid frame to return
// next from ReadFrame. Mostly it checks whether HEADERS and
// CONTINUATION frames are contiguous.
func (fr *Framer) checkFrameOrder(f http2.Frame) error {
	last := fr.lastFrame
	fr.lastFrame = f
	if fr.AllowIllegalReads {
		return nil
	}

	fh := f.Header()
	if fr.lastHeaderStream != 0 {
		if fh.Type != http2.FrameContinuation {
			return fr.connError(http2.ErrCodeProtocol,
				fmt.Sprintf("got %s for stream %d; expected CONTINUATION following %s for stream %d",
					fh.Type, fh.StreamID,
					last.Header().Type, fr.lastHeaderStream))
		}
		if fh.StreamID != fr.lastHeaderStream {
			return fr.connError(http2.ErrCodeProtocol,
				fmt.Sprintf("got CONTINUATION for stream %d; expected stream %d",
					fh.StreamID, fr.lastHeaderStream))
		}
	} else if fh.Type == http2.FrameContinuation {
		return fr.connError(http2.ErrCodeProtocol, fmt.Sprintf("unexpected CONTINUATION for stream %d", fh.StreamID))
	}

	switch fh.Type {
	case http2.FrameHeaders, http2.FrameContinuation:
		if fh.Flags.Has(http2.FlagHeadersEndHeaders) {
			fr.lastHeaderStream = 0
		} else {
			fr.lastHeaderStream = fh.StreamID
		}
	}

	return nil
}

type headersEnder interface {
	HeadersEnded() bool
}

type headersOrContinuation interface {
	headersEnder
	HeaderBlockFragment() []byte
}

func (fr *Framer) maxHeaderStringLen() int {
	v := fr.maxHeaderListSize()
	if uint32(int(v)) == v {
		return int(v)
	}
	// They had a crazy big number for MaxHeaderBytes anyway,
	// so give them unlimited header lengths:
	return 0
}

// readMetaFrame returns 0 or more CONTINUATION frames from fr and
// merge them into the provided hf and returns a MetaHeadersFrame
// with the decoded hpack values.
func (fr *Framer) readMetaFrame(hf *HeadersFrame) (*MetaHeadersFrame, error) {
	if fr.AllowIllegalReads {
		return nil, errors.New("illegal use of AllowIllegalReads with ReadMetaHeaders")
	}
	mh := &MetaHeadersFrame{
		HeadersFrame: hf,
	}
	remainSize := fr.maxHeaderListSize()
	var sawRegular bool

	var invalid error // pseudo header field errors
	hdec := fr.ReadMetaHeaders
	hdec.SetEmitEnabled(true)
	hdec.SetMaxStringLength(fr.maxHeaderStringLen())
	hdec.SetEmitFunc(func(hf hpack.HeaderField) {
		if !httpguts.ValidHeaderFieldValue(hf.Value) {
			invalid = headerFieldValueError(hf.Value)
		}
		isPseudo := strings.HasPrefix(hf.Name, ":")
		if isPseudo {
			if sawRegular {
				invalid = errPseudoAfterRegular
			}
		} else {
			sawRegular = true
			if !validWireHeaderFieldName(hf.Name) {
				invalid = headerFieldNameError(hf.Name)
			}
		}

		if invalid != nil {
			hdec.SetEmitEnabled(false)
			return
		}

		size := hf.Size()
		if size > remainSize {
			hdec.SetEmitEnabled(false)
			mh.Truncated = true
			return
		}
		remainSize -= size

		mh.Fields = append(mh.Fields, hf)
	})
	// Lose reference to MetaHeadersFrame:
	defer hdec.SetEmitFunc(func(hf hpack.HeaderField) {})

	var hc headersOrContinuation = hf
	for {
		frag := hc.HeaderBlockFragment()
		if _, err := hdec.Write(frag); err != nil {
			return nil, http2.ConnectionError(http2.ErrCodeCompression)
		}

		if hc.HeadersEnded() {
			break
		}
		if f, err := fr.ReadFrame(); err != nil {
			return nil, err
		} else {
			hc = f.(*ContinuationFrame) // guaranteed by checkFrameOrder
		}
	}

	mh.HeadersFrame.headerFragBuf = nil
	// mh.HeadersFrame.invalidate()

	if err := hdec.Close(); err != nil {
		return nil, http2.ConnectionError(http2.ErrCodeCompression)
	}
	if invalid != nil {
		fr.errDetail = invalid
		return nil, http2.StreamError{StreamID: mh.StreamID, Code: http2.ErrCodeProtocol, Cause: invalid}
	}
	if err := mh.checkPseudos(); err != nil {
		fr.errDetail = err
		return nil, http2.StreamError{StreamID: mh.StreamID, Code: http2.ErrCodeProtocol, Cause: err}
	}
	return mh, nil
}

// validWireHeaderFieldName reports whether v is a valid header field
// name (key). See httpguts.ValidHeaderName for the base rules.
//
// Further, http2 says:
//
//	"Just as in HTTP/1.x, header field names are strings of ASCII
//	characters that are compared in a case-insensitive
//	fashion. However, header field names MUST be converted to
//	lowercase prior to their encoding in HTTP/2. "
func validWireHeaderFieldName(v string) bool {
	if len(v) == 0 {
		return false
	}
	for _, r := range v {
		if !httpguts.IsTokenRune(r) {
			return false
		}
		if 'A' <= r && r <= 'Z' {
			return false
		}
	}
	return true
}

func readByte(p []byte) (remain []byte, b byte, err error) {
	if len(p) == 0 {
		return nil, 0, io.ErrUnexpectedEOF
	}
	return p[1:], p[0], nil
}

func readUint32(p []byte) (remain []byte, v uint32, err error) {
	if len(p) < 4 {
		return nil, 0, io.ErrUnexpectedEOF
	}
	return p[4:], binary.BigEndian.Uint32(p[:4]), nil
}

func readFrameHeader(r netpoll.Reader) (http2.FrameHeader, error) {
	buf, err := r.Next(frameHeaderLen)
	if err != nil {
		return http2.FrameHeader{}, err
	}
	return http2.FrameHeader{
		Length:   uint32(buf[0])<<16 | uint32(buf[1])<<8 | uint32(buf[2]),
		Type:     http2.FrameType(buf[3]),
		Flags:    http2.Flags(buf[4]),
		StreamID: binary.BigEndian.Uint32(buf[5:]) & (1<<31 - 1),
		// valid:    true,
	}, nil
}
