package generic

import "github.com/cloudwego/kitex/pkg/generic/thrift"

type ThriftFileProviderOptions struct {
	includeDirs   []string
	commonOptions commonOptions
}

type ThriftContentProviderOptions struct {
	commonOptions commonOptions
}

type ThriftContentWithAbsIncludePathProviderOptions struct {
	commonOptions commonOptions
}

type commonOptions struct {
	parseMode thrift.ParseMode
}

type ThriftFileProviderOption struct {
	F func(opt *ThriftFileProviderOptions)
}

type ThriftContentProviderOption struct {
	F func(opt *ThriftContentProviderOptions)
}

type ThriftContentWithAbsIncludePathProviderOption struct {
	F func(opt *ThriftContentWithAbsIncludePathProviderOptions)
}

func (o *ThriftFileProviderOptions) apply(opts []ThriftFileProviderOption) {
	for _, op := range opts {
		op.F(o)
	}
}

func (o *ThriftContentProviderOptions) apply(opts []ThriftContentProviderOption) {
	for _, op := range opts {
		op.F(o)
	}
}

func (o *ThriftContentWithAbsIncludePathProviderOptions) apply(opts []ThriftContentWithAbsIncludePathProviderOption) {
	for _, op := range opts {
		op.F(o)
	}
}
