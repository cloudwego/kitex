package test

import "github.com/cloudwego/thriftgo/generator/golang/extension/unknown"

func (p *Local) GetUnknown() unknown.Fields {
	return p._unknownFields
}

func (p *Local) SetUnknown(fs unknown.Fields) {
	p._unknownFields = fs
}

func (p *MixedStruct) GetUnknown() unknown.Fields {
	return p._unknownFields
}
