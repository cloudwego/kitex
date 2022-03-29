package generic

import (
	"sync"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
)

type PbContentProvider struct {
	closeOnce sync.Once
	svcs      chan *desc.ServiceDescriptor
}

var _ PbDescriptorProvider = (*PbContentProvider)(nil)

func NewPbContentProvider(main string, includes map[string]string) (PbDescriptorProvider, error) {
	p := &PbContentProvider{
		svcs: make(chan *desc.ServiceDescriptor, 1),
	}

	var pbParser protoparse.Parser
	pbParser.Accessor = protoparse.FileContentsFromMap(includes)
	pbParser.IncludeSourceCodeInfo = true
	pbParser.ValidateUnlinkedFiles = true
	pbParser.InterpretOptionsInUnlinkedFiles = true
	fds, err := pbParser.ParseFiles(main)
	if err != nil {
		return nil, err
	}

	services := fds[0].GetServices()
	p.svcs <- services[0]

	return p, nil
}

func (p *PbContentProvider) UpdateIDL(main string, includes map[string]string) error {
	var pbParser protoparse.Parser
	pbParser.Accessor = protoparse.FileContentsFromMap(includes)
	pbParser.IncludeSourceCodeInfo = true
	pbParser.ValidateUnlinkedFiles = true
	pbParser.InterpretOptionsInUnlinkedFiles = true
	fds, err := pbParser.ParseFiles(main)
	if err != nil {
		return err
	}
	services := fds[0].GetServices()

	select {
	case <-p.svcs:
	default:
	}

	select {
	case p.svcs <- services[0]:
	default:
	}

	return nil
}

func (p *PbContentProvider) Provide() <-chan *desc.ServiceDescriptor {
	return p.svcs
}

func (p *PbContentProvider) Close() error {
	p.closeOnce.Do(func() {
		close(p.svcs)
	})
	return nil
}

