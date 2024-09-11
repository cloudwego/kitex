package ttstream

type ClientProviderOption func(cp *clientProvider)

func WithClientPayloadLimit(limit int) ClientProviderOption {
	return func(cp *clientProvider) {
		cp.payloadLimit = limit
	}
}

func WithClientMetaHandler(limit int) ClientProviderOption {
	return func(cp *clientProvider) {
		cp.payloadLimit = limit
	}
}
