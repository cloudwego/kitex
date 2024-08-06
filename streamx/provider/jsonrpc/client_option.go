package jsonrpc

type ClientProviderOption func(cp *clientProvider)

func WithClientPayloadLimit(limit int) ClientProviderOption {
	return func(cp *clientProvider) {
		cp.payloadLimit = limit
	}
}
