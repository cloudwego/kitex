package ttstream

type ClientProviderOption func(cp *clientProvider)

func WithClientMetaHandler(metaHandler MetaFrameHandler) ClientProviderOption {
	return func(cp *clientProvider) {
		cp.metaHandler = metaHandler
	}
}
