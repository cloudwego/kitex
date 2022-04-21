package grpc

import (
	"bytes"
	"sync"
)

//fixme
// todo 后续先通过改大小写的方式实现
func MockStreamRecv(s *Stream) {
	hdr := []byte{0, 0, 0, 0, 10}
	body := []byte("1234567890")
	hdr = append(hdr, body...)
	buffer := new(bytes.Buffer)
	buffer.Reset()
	buffer.Write(hdr)
	s.write(recvMsg{buffer: buffer})
}

func MockStreamRecvHelloRequest(s *Stream) {
	hdr := []byte{0, 0, 0, 0, 6, 10, 4, 116, 101, 115, 116, 0}
	buffer := new(bytes.Buffer)
	buffer.Reset()
	buffer.Write(hdr)
	s.write(recvMsg{buffer: buffer})
}

func DontWaitHeader(s *Stream) {
	s.headerChan = nil
}

// mock a stream with recvBuffer
func MockNewServerSideStream() *Stream {

	recvBuffer := newRecvBuffer()
	trReader := &transportReader{
		reader: &recvBufferReader{
			ctx:     nil,
			ctxDone: nil,
			recv:    recvBuffer,
			freeBuffer: func(buffer *bytes.Buffer) {
				buffer.Reset()
			},
		},
		windowHandler: func(i int) {

		},
	}

	stream := &Stream{
		id:           2,
		st:           nil,
		ct:           nil,
		ctx:          nil,
		cancel:       nil,
		done:         nil,
		ctxDone:      nil,
		method:       "",
		recvCompress: "",
		sendCompress: "",
		buf:          recvBuffer,
		trReader:     trReader,
		fc:           nil,
		wq:           newWriteQuota(defaultWriteQuota, nil),
		requestRead: func(i int) {

		},
		headerChan:       nil,
		headerChanClosed: 0,
		headerValid:      false,
		hdrMu:            sync.Mutex{},
		header:           nil,
		trailer:          nil,
		noHeaders:        false,
		headerSent:       0,
		state:            0,
		status:           nil,
		bytesReceived:    0,
		unprocessed:      0,
		contentSubtype:   "",
	}

	return stream
}
