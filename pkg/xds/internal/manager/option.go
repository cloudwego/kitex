package manager

type Options struct {
	XDSSvrConfig *XDSServerConfig
	DumpPath string
}

func (o *Options) Apply(opts []Option) {
	for _, op := range opts {
		op.F(o)
	}
}

type Option struct {
	F func(o *Options)
}

func NewOptions(opts []Option) *Options {
	o := &Options{
		XDSSvrConfig: &XDSServerConfig{
			SvrAddr: ISTIOD_ADDR,
		},
		DumpPath: defaultDumpPath,
	}
	o.Apply(opts)
	return o
}