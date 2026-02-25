# CLAUDE.md — Kitex Codebase Guide for AI Assistants

## Project Overview

**Kitex** (`github.com/cloudwego/kitex`) is a high-performance, strongly-extensible Go RPC framework developed by CloudWeGo (ByteDance). Current version: **v0.16.1**. Supports Thrift, Protobuf, and gRPC protocols with first-class service governance features.

- **License:** Apache 2.0
- **Go Version:** 1.20+ (CI tests 1.21–1.26)
- **Module:** `github.com/cloudwego/kitex`

---

## Repository Structure

```
kitex/
├── client/           # Client-side RPC implementation
├── server/           # Server-side RPC implementation
├── pkg/              # Public packages (37+ sub-packages)
│   ├── remote/       # Transport layer, codecs, connection pooling
│   ├── endpoint/     # Endpoint and Middleware type definitions
│   ├── kerrors/      # Error type hierarchy
│   ├── rpcinfo/      # RPC call metadata (with object pooling)
│   ├── discovery/    # Service discovery interfaces (Resolver)
│   ├── registry/     # Service registry interfaces
│   ├── loadbalance/  # Load balancing implementations
│   ├── circuitbreak/ # Circuit breaker pattern
│   ├── retry/        # Retry logic
│   ├── limit/        # Rate limiting
│   ├── generic/      # Generic (reflection-based) RPC calls
│   ├── streaming/    # Streaming RPC support
│   ├── stats/        # Event/stats system for observability
│   ├── klog/         # Logging interface
│   ├── transmeta/    # Metadata transmission
│   ├── rpctimeout/   # Timeout management
│   ├── warmup/       # Service warmup
│   └── xds/          # xDS-based service discovery (Istio)
├── internal/         # Internal-only packages
│   ├── client/       # Internal client implementation
│   ├── server/       # Internal server implementation
│   ├── generic/      # Generic call internals
│   ├── mocks/        # Generated gomock files + update.sh script
│   └── utils/        # Internal utilities
├── tool/             # Kitex code generation CLI tool
│   └── cmd/kitex/    # Main entrypoint: `kitex` command
├── transport/        # Transport protocol bitmask definitions
├── scripts/          # Release and hotfix scripts
│   └── .utils/       # Version checking, go.mod validation helpers
└── .github/
    └── workflows/    # CI/CD pipelines (tests.yml, pr-check.yml, claude.yml)
```

---

## Development Workflow

### Prerequisites

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Install mockgen (for updating mocks)
go install github.com/golang/mock/mockgen@latest

# Clone netpoll alongside kitex (required for mocks)
# From kitex's parent directory:
git clone https://github.com/cloudwego/netpoll.git
```

### Running Tests

```bash
# Run all unit tests (standard)
go test ./...

# Run with race detector (required before submitting PRs)
go test -race ./...

# Run with coverage report
go test -race -coverprofile=coverage.out -covermode=atomic ./...

# Run benchmarks (short mode)
go test -bench=. -benchmem -run=none ./... -benchtime=100ms

# Run a specific package
go test -race ./pkg/retry/...

# Run a specific test
go test -race -run TestMyFunction ./pkg/retry/
```

### Linting

```bash
# Run linter (matches CI configuration)
golangci-lint run ./...
```

The project uses `.golangci.yaml` (version 2) with these linters enabled:
- `govet`, `ineffassign`, `staticcheck`, `unconvert`, `unused`

Formatters enforced:
- `gofumpt` (with extra-rules) — stricter than `gofmt`
- `goimports` — with `github.com/cloudwego/kitex` as local prefix

Excluded from linting: `kitex_gen/`, `third_party/`, `builtin/`, `examples/`

### Updating Mock Files

```bash
# From the internal/mocks directory
cd internal/mocks
sh update.sh
```

### Building the Code Generation Tool

```bash
go install github.com/cloudwego/kitex/tool/cmd/kitex@latest
```

---

## CI/CD Pipeline (`.github/workflows/tests.yml`)

All jobs run on pull requests:

| Job | Go Versions | Platform | Command |
|-----|-------------|----------|---------|
| `unit-test-x64` | 1.21–1.26 | Linux/X64 | `go test -race ./...` |
| `unit-test-arm` | 1.21–1.26 | ARM64 | `go test -race ./...` |
| `unit-scenario-test` | 1.21, 1.26 | Linux/X64 | Clones kitex-tests, runs `run.sh` |
| `benchmark-test` | stable | ubuntu-latest | `go test -bench=. -benchtime=100ms` |
| `codegen-test` | latest | ubuntu-latest | Tests thriftgo code generation |
| `windows-test` | — | Windows | Compatibility tests |

**Note:** `kitex-tests` (integration/scenario tests) lives in a separate repository: `https://github.com/cloudwego/kitex-tests`.

---

## Key Code Conventions

### License Header

Every `.go` source file must start with:

```go
/*
 * Copyright 2021 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * ...
 */
```

### Error Handling

Use the predefined error types from `pkg/kerrors` rather than raw `errors.New`:

```go
// Basic error types (pkg/kerrors/kerrors.go)
kerrors.ErrInternalException
kerrors.ErrServiceDiscovery
kerrors.ErrGetConnection
kerrors.ErrLoadbalance
kerrors.ErrNoMoreInstance
kerrors.ErrRPCTimeout
kerrors.ErrCanceledByBusiness
kerrors.ErrTimeoutByBusiness
kerrors.ErrACL
kerrors.ErrCircuitBreak
kerrors.ErrRemoteOrNetwork
kerrors.ErrOverlimit
kerrors.ErrPanic
kerrors.ErrBiz
kerrors.ErrRetry
kerrors.ErrRoute
kerrors.ErrPayloadValidation

// Wrapping with a cause:
kerrors.ErrInternalException.WithCause(err)
```

### Middleware / Endpoint Pattern

The core abstraction in `pkg/endpoint`:

```go
// An Endpoint is any RPC handler
type Endpoint func(ctx context.Context, req, resp interface{}) (err error)

// A Middleware wraps an Endpoint
type Middleware func(next Endpoint) Endpoint

// Compose middlewares (outermost first)
Chain(mw1, mw2, mw3)(finalEndpoint)

// Also available for streaming:
type UnaryEndpoint    // same signature, distinct type
type StreamMiddleware // for streaming RPCs
```

### Options Pattern

Client and server configuration uses functional options:

```go
// client.Option is the standard type
// Options are defined in client/option.go and server/option.go
client.WithXxx(...)  // returns client.Option
server.WithXxx(...)  // returns server.Option

// Options types:
//   client.Option, client.UnaryOption, client.StreamOption
//   server.Option, server.TTHeaderStreamingOption
```

### RPC Info

RPC metadata is context-propagated via `pkg/rpcinfo`:

```go
ri := rpcinfo.GetRPCInfo(ctx)     // retrieve from context
rpcinfo.PutRPCInfo(ri)             // return to pool when done
```

Object pooling is **enabled by default**. Disable for debugging:

```bash
KITEX_DISABLE_RPCINFO_POOL=1 go test ./...
```

### Transport Protocol Bitmask

Protocols are combined with bitwise OR (`transport/` package):

```
PurePayload | TTHeader | Framed | HTTP | GRPC | HESSIAN2
TTHeaderStreaming | GRPCStreaming
TTHeaderFramed = TTHeader | Framed  (convenience alias)
```

### Service Discovery / Registry Interfaces

- **Resolver** (`pkg/discovery`): Implement `Resolve(ctx, target) (Result, error)` and `Diff(key, prev, next Result) Change`
- **Registry** (`pkg/registry`): Implement `Register(info *Info) error` and `Deregister(info *Info) error`
- Results can be marked `Cacheable` — framework will cache and diff them automatically.

### Event / Stats System (`pkg/stats`)

```go
// Predefined events
stats.RPCStart, stats.RPCFinish
stats.ServerHandleStart, stats.ServerHandleFinish
stats.ClientConnStart, stats.ClientConnFinish
stats.ReadStart, stats.ReadFinish
stats.WriteStart, stats.WriteFinish
stats.StreamRecv, stats.StreamSend, stats.StreamStart, stats.StreamFinish

// Register custom events BEFORE initialization:
myEvent := stats.DefineNewEvent("MyEvent", stats.LevelDetailed)
```

---

## Testing Conventions

- Test files are co-located with the implementation (`foo.go` → `foo_test.go`).
- Use `github.com/stretchr/testify` (`require`, `assert`) for assertions.
- **Interface mocking:** Use `github.com/golang/mock/gomock` — generated mocks live in `internal/mocks/`.
- **Function mocking:** Use `github.com/bytedance/mockey` for patching concrete functions.
- Always run with `-race` before submitting — the CI enforces this.
- Table-driven tests are the preferred style for multiple input cases.

---

## Commit Message Format

Follow [AngularJS Git Commit Message Conventions](https://docs.google.com/document/d/1QrDFcIiPjSLDn3EL15IJygNPiHORgU1_OOAqWjiDU5Y/edit):

```
<type>(<scope>): <short summary>

Types: feat, fix, docs, style, refactor, perf, test, chore, ci
```

Examples from this repo:
```
feat: gRPC supports reusing write buffer for each connection
fix: remove streaming rpcstats Reset
fix(ttstream): add server-side information in ttstream error
chore: release version v0.16.1
chore: change tests workflow on go 1.21-1.26
```

---

## Key Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `cloudwego/netpoll` | v0.7.2 | High-performance async networking (core I/O) |
| `bytedance/sonic` | v1.15.0 | Fast JSON serialization |
| `cloudwego/thriftgo` | v0.4.3 | Thrift IDL parsing and code generation |
| `cloudwego/dynamicgo` | v0.8.0 | Dynamic/reflection-based message handling |
| `cloudwego/frugal` | v0.3.1 | JIT-based fast Thrift codec |
| `cloudwego/fastpb` | v0.0.5 | Fast Protobuf codec |
| `cloudwego/gopkg` | v0.1.8 | CloudWeGo shared utilities |
| `cloudwego/configmanager` | v0.2.3 | Dynamic configuration |
| `golang/mock` | v1.6.0 | Interface mocking for tests |
| `stretchr/testify` | v1.10.0 | Test assertions |
| `google.golang.org/protobuf` | v1.33.0 | Protocol Buffers runtime |

---

## Files to Avoid Modifying

- `kitex_gen/` — auto-generated; never edit by hand
- `third_party/` — vendored third-party code
- `builtin/` — framework built-in generated stubs
- `examples/` — example code, excluded from lint

---

## Code Style References

- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Effective Go](https://golang.org/doc/effective_go)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- [PingCAP General Advice](https://pingcap.github.io/style-guide/general.html)

---

## Common Patterns to Follow

1. **Always use `pkg/kerrors` error types** — never return raw `errors.New("rpc timeout")` from framework internals.
2. **Use functional options** for any new configurable component — follow the `WithXxx(val) Option` pattern.
3. **Pool allocation-heavy objects** — see `rpcinfo` object pool as a model; use `sync.Pool` for hot-path structs.
4. **Middleware = `func(next Endpoint) Endpoint`** — all cross-cutting concerns go through this.
5. **Context carries RPC info** — use `rpcinfo.GetRPCInfo(ctx)` rather than passing `rpcinfo` as explicit parameters.
6. **Interfaces for extensibility** — core subsystems (Resolver, Registry, LoadBalancer, Codec, etc.) are defined as interfaces; implementations live in separate extension repositories.
7. **Add the Apache 2.0 license header** to every new `.go` file.
