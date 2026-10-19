# Native unary server transport

`WithNativeTransport` selects a separate `remotesvr.Server` implementation at `Run`. It uses the registered service searcher and the complete business endpoint chain. The existing listener, transport factory and pipeline remain the default path. Native mode supplies no synthetic `net.Conn` or `net.Listener`.

The bridge must finish listener readiness before returning from `Listen`; registration then uses its actual `Address`. `Accept` transfers one request task to an ordinary Go goroutine. A bounded 64-task dispatcher can wait without occupying the native owner. Response/control completion and buffer release continue independently of this dispatcher. Connection credit belongs to `NativeServerRequest.Release`, rather than the RX buffer release.

Each request runs Go Decode and built-in ReadMeta once before `ServerBegin`. The bridge admits it on the frame's owner. `ProjectIngress` rebuilds supported derived metadata from the final header state and the clean context saved before ReadMeta. It must implement deletion, retain one-time backward values and fallback IDs, and preserve the wire invocation. TransferForward and the complete business middleware/ACL/ErrorHandler chain run after that projection. The earliest absolute deadline, measured from native Begin, is attached before business middleware.

Only default generated Fast/owned Thrift unary with TTHeaderFramed is supported. The registered codec and generated args/results are checked before listen; the codec is checked again before decoding a frame. Explicit payload/transport codecs, metadata, generic, listener/proxy, BoundHandler/factories, transport limits, mux/reuse-port and transport profiler hooks are rejected in either option order. Manually modified FastRead implementations are outside the ownership contract.

WriteMeta runs once. ServerResponse applies governance changes before the ordinary Go codec writes one continuous native TX lease. Send consumes that lease on success or failure and returns only after complete local send or confirmed failure. CompleteServer runs once for an entered governance attempt; TX memory release is an independent native event. Decode/ReadMeta failures never enter that completion phase. Exceptions use the Go codec when the decoded invocation is reliable; otherwise the connection closes.

Stop closes the listener and cancels task contexts, then waits for actual Go task exit up to ExitWaitTime. A timeout reports draining and keeps live handler objects, governance and native references reachable. Stop does not close a shared engine. Business code may keep decoded Go-owned fields after RX release. Native Begin/read/write observations populate normal RPCStats through RecordAt; Go codec events keep their own execution times.

Boundary regression commands:

```sh
go test ./server ./client ./pkg/rpcinfo ./pkg/remote ./pkg/remote/codec ./pkg/remote/codec/thrift ./internal/server -count=1
go test -race ./server -run '^TestNativeServer' -count=1
go vet ./server ./client ./pkg/rpcinfo ./pkg/remote ./pkg/remote/codec ./pkg/remote/codec/thrift ./internal/server
```

The public fixture tests use a Go TCP peer behind the bridge interface and poison their own RX storage. They prove adapter ordering and ownership, not native sockets. The Byted tagged ingress test uses a fresh ingress-enabled SO, a separate native server PID and an ordinary Go client; it is the native network acceptance path.

A transport may implement `remote.NativeServerRequestCleanup`. When `RetainFrameForCleanup` is true, the server keeps RX through handler execution and response send, invokes CompleteIO, then calls `ReleaseWithBuffer` to release RX and request credit together. Buffer and request errors remain independent; the transport must disable reuse if RX release fails. This optional path trades longer RX retention for fewer owner wake cycles. Other requests keep the existing early RX release after Begin and metadata projection.

A listener may implement `remote.NativeConcurrentListener` and return true from `ConcurrentAccept`. This promises concurrent Accept calls are safe: the server starts 64 request workers that accept and execute directly, preserving the existing request concurrency bound while avoiding a central accept-to-handler scheduling hop. Stop cancels accepts and waits for all workers, including retained business handlers. Other listeners retain one acceptor. Peer cancellation is detached after CompleteIO and before normal request release.
