package jsonrpc

type ServerProviderOption func(pc *serverProvider)

func WithServerPayloadLimit(limit int) ServerProviderOption {
	return func(s *serverProvider) {
		s.payloadLimit = limit
	}
}
