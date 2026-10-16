# Native unary client

`WithNativeTransport(remote.NativeClientTransport)` selects a separate client endpoint. The bridge supplies governance hooks, its supported built-in metadata handlers, connection reservations and native-owned frame buffers. Kitex retains its middleware, tracer, default Fast codec and generated business interfaces. Streaming and oneway remain unsupported.

The endpoint performs WriteMeta, PrepareUnaryWire, ClientPrepare, Connect, connection metadata, Encode, Exchange, Decode, ClientFinish and ReadMeta in that order. The calling goroutine runs codec and Go hooks. A canceled CPU codec must finish its current invocation before the RPC returns. The absolute budget starts before middleware and can only shorten.

Exchange consumes the TX buffer, including cleanup when submission fails. The caller releases RX after decoding, governance and metadata processing. Connection.Release consumes the reservation and only reuses it when complete request sending and successful governance permit. Native cleanup outlives RPC results where needed. Client.Close does not unload a shared engine; the engine owner drains and closes it explicitly.

NativeIO.FailureStage carries the explicit native I/O failure stage. A full accepted byte count does not override a send error. Successful Exchange leaves subsequent codec and business error classification to Go.

NewNativeWriter and NewNativeReader are non-owning ByteBuffer views. Their Release invalidates the view; release the NativeBuffer separately. The writer never moves its backing bytes and does not implement NocopyWrite. FastWriteNocopy therefore copies fields into the acquired native frame, preserving TTHeader/Framed length patches. Capacity exhaustion returns an error. CRC validator scratch copies remain in the unchanged codec.

Explicit custom transport/metadata/codec options, Go pooling/routing and retry/backup are rejected in either option order. Per-call retry and Go address overrides are rejected before starting an attempt. The bridge is a trusted system boundary: its BuiltinMetaHandlers must contain only the supported built-ins, and Exchange must finish sending before a reusable reservation can return to its pool. Generated FastRead must return owned fields; arbitrary borrowed/custom FastRead implementations are outside this mode's contract.

The supported code generation is the standard Kitex Fast template using copying thrift.Binary field readers. FastCodec method presence alone does not prove ownership. The guard checks the default payload codec, standard per-field FastReadField methods throughout the actual argument/result type graph, and explicit frugal nocopy tags. This rejects the known -frugal-struct wrappers, including nested ones, without enabling Frugal. Reflection cannot certify a user-rewritten FastRead body that imitates these methods; such code is outside this mode's contract. Successful type-shape checks are cached, not repeated for every message.

The NativeTransport client tests use a transport boundary fixture and a real Go server; they validate the Go endpoint, framing, released-input ownership and configuration behavior. They are not evidence of native socket I/O. Real native acceptance must use the production bridge/SO and correlate the same RPC with native owner/socket events and zero old client transport hits.

```sh
go test ./client -count=1
go test -race ./client -run '^TestNativeTransport' -count=1
go vet ./client ./pkg/remote
```

Run these with the task workspace and locked patched gopkg dependency. Keep existing unary lifecycle tests unchanged for governance-only mode.
