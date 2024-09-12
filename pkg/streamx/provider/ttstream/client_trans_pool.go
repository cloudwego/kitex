package ttstream

// TODO: it's to complex for users implement idle check
// so let's implement it in netpoll

// TODO: make transPoll configurable
type transPool interface {
	Get(network string, addr string) (trans *transport, err error)
	Put(trans *transport)
}
