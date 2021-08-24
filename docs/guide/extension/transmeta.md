# Extension of MetaInfo Transparent Transmission

MetaInfo transparently transmission transmits some additional RPC information to the downstream based on the transport protocol, and reads the upstream transparently transmitted information carried by transport protocol. The transparently transmitted field needs to be combined with the internal governance capability. It is recommended that users extend and implement it by themselves.

## Extension API

```go
// MetaHandler reads or writes metadata through certain protocol.
// The protocol will be a thrift.TProtocol usually.
type MetaHandler interface {
	WriteMeta(ctx context.Context, msg Message) (context.Context, error)
	ReadMeta(ctx context.Context, msg Message) (context.Context, error)
}
```

## Extension Example

- ClientMetaHandler

```go
package transmeta

var	ClientTTHeaderHandler remote.MetaHandler = &clientTTHeaderHandler{}

// clientTTHeaderHandler implement remote.MetaHandler
type clientTTHeaderHandler struct{}

// WriteMeta of clientTTHeaderHandler writes headers of TTHeader protocol to transport
func (ch *clientTTHeaderHandler) WriteMeta(ctx context.Context, msg remote.Message) (context.Context, error) {
	ri := msg.RPCInfo()
	transInfo := msg.TransInfo()

	hd := map[uint16]string{
		transmeta.FromService:    ri.From().ServiceName(),
		transmeta.FromIDC:        ri.From().DefaultTag(consts.IDC, ""),
		transmeta.FromMethod:     ri.From().Method(),
		transmeta.ToService:      ri.To().ServiceName(),
		transmeta.ToMethod:       ri.To().Method(),
		transmeta.MsgType:        strconv.Itoa(int(msg.MessageType())),
	}
	transInfo.PutTransIntInfo(hd)

	if metainfo.HasMetaInfo(ctx) {
		hd := make(map[string]string)
		metainfo.SaveMetaInfoToMap(ctx, hd)
		transInfo.PutTransStrInfo(hd)
	}
	return ctx, nil
}

// ReadMeta of clientTTHeaderHandler reads headers of TTHeader protocol from transport
func (ch *clientTTHeaderHandler) ReadMeta(ctx context.Context, msg remote.Message) (context.Context, error) {
	ri := msg.RPCInfo()
	remote := remoteinfo.AsRemoteInfo(ri.To())
	if remote == nil {
		return ctx, nil
	}

	transInfo := msg.TransInfo()
	strInfo := transInfo.TransStrInfo()
	ad := strInfo[transmeta.HeaderTransRemoteAddr]
	if len(ad) > 0 {
		// when proxy case to get the actual remote address
		_ = remote.SetRemoteAddr(utils.NewNetAddr("tcp", ad))
	}
	return ctx, nil
}
```

- Use Customized ClientMetaHandler

  ```go
  cli, err := xxxservice.NewClient(targetService, client.WithMetaHandler(transmeta.ClientTTHeaderHandler))
  ```

  