package proto

import (
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
)

type ServiceDescriptor = *desc.ServiceDescriptor
type MessageDescriptor = *desc.MessageDescriptor

type Message interface{
	Marshal() ([]byte, error)
	TryGetFieldByNumber(fieldNumber int) (interface{}, error)
	TrySetFieldByNumber(fieldNumber int, val interface{}) error
}

func NewMessage(descriptor MessageDescriptor) Message {
	return dynamic.NewMessage(descriptor)
}