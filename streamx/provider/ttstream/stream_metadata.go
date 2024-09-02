package ttstream

var _ ClientStreamMeta = (*clientStream)(nil)
var _ ServerStreamMeta = (*serverStream)(nil)

func (s *clientStream) RecvHeader() (Header, error) {
	<-s.headerSig
	return s.header, nil
}

func (s *clientStream) RecvTrailer() (Trailer, error) {
	<-s.trailerSig
	return s.trailer, nil
}

func (s *serverStream) SetHeader(hd Header) error {
	return s.writeHeader(hd)
}

func (s *serverStream) SendHeader(hd Header) error {
	err := s.writeHeader(hd)
	if err != nil {
		return err
	}
	return s.stream.sendHeader()
}

func (s *serverStream) SetTrailer(tl Trailer) error {
	return s.writeTrailer(tl)
}
