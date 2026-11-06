package thrift

import "github.com/cloudwego/kitex/pkg/generic/descriptor"

// ServiceDescriptor returns the descriptor snapshot used by this reader.
// The returned descriptor is shared and must be treated as immutable.
func (m *ReadStruct) ServiceDescriptor() *descriptor.ServiceDescriptor { return m.svc }

// ServiceDescriptor returns the descriptor snapshot used by this reader.
// The returned descriptor is shared and must be treated as immutable.
func (m *ReadJSON) ServiceDescriptor() *descriptor.ServiceDescriptor { return m.svc }
