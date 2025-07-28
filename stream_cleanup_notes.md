# Kitex 流清理机制设计与实现笔记

## 1. 问题背景与方案演进

### 1.1 问题背景

**Kitex (< v1.18.2)**
在 Kitex 早期版本(< v1.18.2)，Client Stream 在使用 cancel 强制关闭流后，必须要依靠主动调用 Recv() 来感知 cancel 信号，将 Client Stream 关闭，并发送 rst_stream 给 Server Stream。

以 ServerStreaming 为例，如果在使用 Client Stream 时 Recv 到某个特殊的 resp 后(e.g. 某个字段表示业务流程结束) 就不再 Recv 了，这个 Client Stream 在 Kitex 的视角生命周期依然没有结束，仍能正常接受对端传来的数据，造成 Stream 泄漏。

此时必须使用以下的 Hack 写法：
```go
func SolveServerStreamingLeak() {
    ctx, cancel := context.WithCancel(ctx)
    st, err := cli.EchoServerStreaming(ctx, req)
    if err != nil {
        cancel()
        return
    }
    defer func() {
        cancel() // 此时流无法被关闭
        st.Recv() // 感知到 ctx 被 cancel，关闭流并发送 rst_stream
    }
    resp, err := st.Recv()
    if shouldStop(resp) {
        return
    } 
}
```

**grpc-go**
官方 gRPC-go 为每个流式 Stream 都分配了一个 goroutine，专门用来监听 ctx.Done()，虽然时效性好，但会大幅增加 goroutine 数量：
```go
func newClientStreamWithParams() {
    // ...
    // 只对流式(非 unary)的 Stream 生效
    if desc != unaryStreamDesc {
        go func() {
           select {
           // 连接级别的 ctx
           case <-cc.ctx.Done():
              cs.finish(ErrClientConnClosing)
           // 流级别的 ctx   
           case <-ctx.Done():
              cs.finish(toRPCErr(ctx.Err()))
           }
        }()
    }
}
```

### 1.2 方案演进

**Kitex ( >= v1.18.2)**
为了解决 cancel() 之后必须调用 stream.Recv() 来感知 cancel 信号的情况，同时避免 grpc-go 的解法带来的 goroutine 数量大幅增加的弊端，选择使用一个全局的 SharedTicker 来定期扫描已经被 cancel 的 Client Stream，释放资源并发送 rst_stream:

- 触发周期为 5s 的 SyncTicker，所有任务串行执行
```go
var ticker = utils.NewSyncSharedTicker(5 * time.Second)
```

- 每个 HTTP2Client 都分配一个 closeStreamTask，判断所有活跃流是否被 cancel 了
```go
func (task *closeStreamTask) Tick() {
    trans := task.t
    trans.mu.Lock()
    for _, stream := range trans.activeStreams {
       select {
       // 判断该连接上所有活跃的流是否被 cancel 了
       case <-stream.Context().Done():
          task.toCloseStreams = append(task.toCloseStreams, stream)
       default:
       }
    }
    trans.mu.Unlock()

    for i, stream := range task.toCloseStreams {
       sErr := ContextErr(stream.Context().Err())
       // 关闭被 cancel 的流，并发送 rst_stream
       // 其中 HTTP2 Error Code 为 8(ErrCodeCancel)
       trans.closeStream(stream, sErr, true, http2.ErrCodeCancel, status.Convert(sErr), nil, false)
       task.toCloseStreams[i] = nil
    }
    task.toCloseStreams = task.toCloseStreams[:0]
}
```

但这个功能被主动开启了，且并没有暴露开关和配置给用户，导致在 v1.18.2，v1.18.3，v1.19.0 因实现有 bug，必须封禁版本Kitex v1.18.2、v1.18.3、v1.19.0 版本召回。

**当前改进**
- 可配置开启/关闭：用户可以选择是否启用流清理功能
- 可配置检查间隔：用户可以自定义清理检查的时间间隔
- 保持向后兼容：如果不配置，仍然使用默认值（开启，5秒间隔）

---

## 2. 数据结构定义

### StreamCleanupConfig 结构体

**位置：** `/pkg/streaming/stream_cleanup.go`

```go
type StreamCleanupConfig struct {
    Disable        bool          // 是否禁用流清理，默认 false（即启用）
    CleanInterval  time.Duration // 清理任务间隔，默认 5s
}
```

**字段说明：**
- `Disable`：控制是否禁用流清理功能，默认为 `false`（即启用清理）
- `CleanInterval`：清理任务的执行间隔，默认为 5 秒

**位置：** `internal/client/option.go`
```go
// StreamOptions 定义了流式调用相关的所有可选项。
// - StreamCleanupConfig：流清理配置，允许用户自定义是否禁用流清理功能及清理间隔，
//   以防止流因未及时关闭而泄漏资源，并支持运行时观测和调优。
type StreamOptions struct {
    // ... 其他字段 ...
    StreamCleanupConfig *streaming.StreamCleanupConfig // 流清理配置
}
```

---

## 3. 配置与传递链路

配置传递完整链路：
- 用户接口 → 客户端选项 → GRPC 连接选项 → 传输层配置 → HTTP2 客户端
  - 用户通过 `WithStreamCleanupConfig` 配置流清理功能。
  - 配置会层层传递到传输层（GRPCStreaming/TTHeaderStreaming），最终影响 HTTP2 客户端的流清理行为。
  - 默认值：未配置时，自动启用流清理，间隔为 5 秒。

HTTP2 传输层实现：
- 包含完整的数据结构、统计记录、清理任务
- 实现了定时器共享机制，避免多连接场景下的资源浪费

### 3.1 配置接口

**位置：** `client/option_stream.go`
**说明：** 这是用户配置流清理功能的主要接口，提供了统一的配置方式。

```go
// WithStreamCleanupConfig 配置流清理功能
func WithStreamCleanupConfig(cfg streaming.StreamCleanupConfig) StreamOption {
    return StreamOption{F: func(o *StreamOptions, di *utils.Slice) {
        di.Push(fmt.Sprintf("WithStreamCleanupConfig(%+v)", cfg))
        if !cfg.Disable && cfg.CleanInterval <= 0 {
            panic("WithStreamCleanupConfig: CleanInterval must be greater than 0 when enabled")
        }
        // 禁用时自动归零
        if cfg.Disable && cfg.CleanInterval != 0 {
            cfg.CleanInterval = 0
        }
        o.StreamCleanupConfig = &cfg
    }}
}
```

### 3.2 初始化与配置传递

#### GRPCStreaming

**位置：** `internal/client/option.go` → `initRemoteOpt()` 
**说明：** GRPCStreaming 协议的流清理配置初始化。

```go
// 配置 grpc streaming
if o.Configs.TransportProtocol()&(transport.GRPC|transport.GRPCStreaming) == transport.GRPCStreaming {
    // 应用流清理配置到 GRPC 传输层
    // 若未配置，自动填充默认值（启用，5s）
    if o.StreamOptions.StreamCleanupConfig == nil {
        o.StreamOptions.StreamCleanupConfig = &streaming.StreamCleanupConfig{
            Disable:       false,
            CleanInterval: 5 * time.Second,
        }
    }
    		// 关键步骤：将配置传递给 GRPC 连接选项
		o.GRPCConnectOpts.StreamCleanupDisabled = o.StreamOptions.StreamCleanupConfig.Disable
		o.GRPCConnectOpts.StreamCleanupInterval = o.StreamOptions.StreamCleanupConfig.CleanInterval
    // 创建连接池时传入配置
    o.RemoteOpt.GRPCStreamingConnPool = nphttp2.NewConnPool(o.Svr.ServiceName, o.GRPCConnPoolSize, *o.GRPCConnectOpts)
    o.RemoteOpt.GRPCStreamingCliHandlerFactory = nphttp2.NewCliTransHandlerFactory()
}
```

#### 传输层配置结构 (Transport Configuration Structure)
**位置：** `pkg/remote/trans/nphttp2/grpc/transport.go`
```go
type ConnectOptions struct {
	// ... 其他字段
	StreamCleanupDisabled bool          // 是否禁用流清理
	StreamCleanupInterval time.Duration // 清理间隔
}
```

#### 连接池层 (Connection Pool Layer)
**位置：** `pkg/remote/trans/nphttp2/conn_pool.go`
```go
// NewConnPool 创建连接池时接收 ConnectOptions
func NewConnPool(remoteService string, size uint32, connOpts grpc.ConnectOptions) *connPool {
    return &connPool{
        remoteService: remoteService,
        size:          size,
        connOpts:      connOpts,  // 保存配置
    }
}

// 创建新传输时传递配置
func (p *connPool) newTransport(ctx context.Context, dialer remote.Dialer, network, address string,
    connectTimeout time.Duration, opts grpc.ConnectOptions,
) (grpc.ClientTransport, error) {
    // ... 连接建立逻辑
    
    // 关键步骤：创建 HTTP2 客户端传输时传入配置
    return grpc.NewClientTransport(
        ctx,
        conn,
        opts,  // 传递 ConnectOptions
        p.remoteService,
        // ... 回调函数
    )
}
```

#### TTHeaderStreaming

**位置：** `internal/client/option.go` → `initRemoteOpt()`
**说明：** TTHeaderStreaming 协议的流清理配置初始化。

```go
// 配置 TTHeaderStreaming
if o.Configs.TransportProtocol()&transport.TTHeaderStreaming == transport.TTHeaderStreaming {
    if o.PoolCfg != nil && *o.PoolCfg == zero {
        // configure short conn pool
        o.TTHeaderStreamingOptions.TransportOptions = append(o.TTHeaderStreamingOptions.TransportOptions, ttstream.WithClientShortConnPool())
    }
    
    // Apply stream cleanup configuration from StreamOptions to TTHeader transport
    if o.StreamOptions.StreamCleanupConfig == nil {
        o.StreamOptions.StreamCleanupConfig = &streaming.StreamCleanupConfig{
            Disable:       false,
            CleanInterval: 5 * time.Second,
        }
    }
    
    o.RemoteOpt.TTHeaderStreamingProvider = ttstream.NewClientProvider(o.TTHeaderStreamingOptions.TransportOptions...)
}
```

TTHeaderStreaming 的流清理配置链路已经打通，和 gRPCStreaming 一样支持配置和默认值传递，但底层清理待未实现。

### 3.3 HTTP2 传输层的流清理实现（gRPCStreaming）

#### 数据结构
**位置：** `pkg/remote/trans/nphttp2/grpc/http2_client.go`
```go
type closeStreamTask struct {
    t              *http2Client
    toCloseStreams []*Stream
    ticker         *utils.SharedTicker
}
```

#### 清理任务调度
- 定期遍历所有活跃流，关闭已 cancel 的流，释放资源并发送 RST_STREAM。
```go
func (task *closeStreamTask) Tick() {
    trans := task.t
    trans.mu.Lock()
    for _, stream := range trans.activeStreams {
        select {
        case <-stream.Context().Done():
            task.toCloseStreams = append(task.toCloseStreams, stream)
        default:
        }
    }
    trans.mu.Unlock()
    for i, stream := range task.toCloseStreams {
        sErr := ContextErr(stream.Context().Err())
        trans.closeStream(stream, sErr, true, http2.ErrCodeCancel, status.Convert(sErr), nil, false)
        task.toCloseStreams[i] = nil
    }
    task.toCloseStreams = task.toCloseStreams[:0]
}
```

#### 全局共享 ticker 管理
- 多连接共享定时器，节省资源。
```go
var (
    streamCleanupTickers    sync.Map
    streamCleanupTickersSfg singleflight.Group
)

func getStreamCleanupTicker(opts ConnectOptions) *utils.SharedTicker {
    if opts.StreamCleanupDisabled {
        return nil
    }
    interval := opts.StreamCleanupInterval
    if interval <= 0 {
        interval = 5 * time.Second
    }
    // 使用全局 map 共享相同间隔的 ticker
    return getSharedTickerForInterval(interval)
}
```

#### 启用流清理任务
- 由于配置链路打通，创建 HTTP2 客户端时，用户可灵活控制是否启用及清理间隔。
```go
func newHTTP2Client(ctx context.Context, conn net.Conn, opts ConnectOptions,
    remoteService string, onGoAway func(GoAwayReason), onClose func(),
) (_ *http2Client, err error) {
    // ... 其他初始化逻辑
    t := &http2Client{
        // ... 其他字段
        streamCleanupDisabled: opts.StreamCleanupDisabled,  // 保存配置
        streamCleanupInterval: opts.StreamCleanupInterval,  // 保存配置
    }
    // 设置流清理任务
    streamCleanupTicker := getStreamCleanupTicker(opts)
    if streamCleanupTicker != nil {
        t.streamCleanupTask = &closeStreamTask{
            t:      t,
            ticker: streamCleanupTicker,
        }
        streamCleanupTicker.Add(t.streamCleanupTask)  // 注册清理任务
    }
}
```

---

## 4. 使用方式与场景

### 之前的固定配置
```go
StreamCleanupDisabled = false  // 固定开启
StreamCleanupInterval = 5 * time.Second  // 固定5秒
```

### 现在的配置方式

**场景1：保持默认行为（无需配置）**
```go
// 不配置 WithStreamCleanupConfig，系统使用默认值
client.NewClient("service.name")
// 结果：启用清理，5秒间隔（适用于 GRPCStreaming 和 TTHeaderStreaming）
```

**场景2：禁用流清理功能**
```go
// 某些特殊场景下，用户可能不需要自动清理
client.NewClient("service.name",
    client.WithStreamOptions(
        client.WithStreamCleanupConfig(streaming.StreamCleanupConfig{
            Disable: true,
        }),
    ),
)
```

**场景3：自定义清理间隔**
```go
// 对于高频场景，可能需要更频繁的清理
client.NewClient("service.name",
    client.WithStreamOptions(
        client.WithStreamCleanupConfig(streaming.StreamCleanupConfig{
            Disable:       false,
            CleanInterval: 1 * time.Second,
        }),
    ),
)

// 对于低频场景，可以降低清理频率以节省资源
client.NewClient("service.name",
    client.WithStreamOptions(
        client.WithStreamCleanupConfig(streaming.StreamCleanupConfig{
            Disable:       false,
            CleanInterval: 10 * time.Second,
        }),
    ),
)
```

**场景4：用户只传一个参数**
```go
// 只设置 Disable（CleanInterval 自动设为 0）
client.NewClient("service.name",
    client.WithStreamOptions(
        client.WithStreamCleanupConfig(streaming.StreamCleanupConfig{
            Disable: true,
        }),
    ),
)

// 只设置 CleanInterval（Disable 默认为 false）
client.NewClient("service.name",
    client.WithStreamOptions(
        client.WithStreamCleanupConfig(streaming.StreamCleanupConfig{
            CleanInterval: 3 * time.Second,
        }),
    ),
)
```

---

## 5. 可观测性支持

可以通过 DEBUG 端口查询 client_info 。Kitex - 运行时 Debug 端口页面

---

## 6. 待讨论事项：gRPC Unary 行为设计

Kitex 在 gRPC Streaming 场景下，为了防止客户端主动 cancel 后服务端 goroutine 泄漏，引入了 stream cleanup 机制（定期清理已 cancel 的流）。目前该机制主要针对 streaming 类型（如 ClientStreaming、ServerStreaming、BidiStreaming）。现需讨论：gRPC Unary 是否也需要支持相同的流清理机制？

### gRPC Unary 行为设计的两种选择

**方案一：gRPC Unary 行为与 pingpong（一次性请求-响应）一致**
- 不启用 stream cleanup 机制。
- 依赖 gRPC 协议本身的 cancel 传播，server 端 handler 通过 ctx.Done() 感知 cancel，及时退出。
- 优点：实现简单，无额外资源消耗。
- 风险：如果 handler 内部有阻塞操作（非 ctx 控制），仍有泄漏风险，但这属于业务代码问题。
  
**方案二：gRPC Unary 行为与 Streaming 一致**
- 统一所有流类型的 cleanup 行为，即使是 Unary 也定期检查并清理。
- 优点：行为一致，极端情况下可兜底清理所有类型的流。
- 风险：流的超时时间难以合理设置，存在流尚未自然结束就被提前关闭的风险。

---

## 7. 设计优势总结

### 7.1 用户体验优化
- **默认开启**：符合大多数用户需求，无需额外配置
- **灵活配置**：支持禁用和自定义间隔，满足特殊场景需求
- **参数可选**：用户可以选择只设置一个参数，其他参数使用合理默认值

### 7.2 技术实现优势
- **向后兼容**：不破坏现有代码，默认行为保持一致
- **资源优化**：共享定时器机制，避免多连接场景下的资源浪费
- **代码简洁**：使用 `Disable` 字段，逻辑判断更直观（`!cfg.Disable`）

### 7.3 维护性提升
- **统一接口**：所有传输协议使用相同的配置接口
- **可观测性**：支持运行时查询配置状态
- **测试覆盖**：完整的测试用例覆盖各种配置场景 