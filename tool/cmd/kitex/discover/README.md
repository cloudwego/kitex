# Kitex Discover Command

The `kitex discover` command is a CLI tool for inspecting service registries and debugging service discovery issues.

## Features

- 🔍 Query service registries (Consul, Etcd, Nacos, ZooKeeper)
- 📊 Display services, instances, health status, and metadata
- 📝 Multiple output formats: table, JSON, YAML
- 🎯 Filter instances by health status and tags
- ⚡ Fast and efficient registry querying

## Installation

```bash
cd tool/cmd/kitex
go build -o kitex .
```

Or from the root of the kitex repository:

```bash
go build -o kitex ./tool/cmd/kitex
```

## Usage

### Basic Commands

**List all services:**
```bash
kitex discover --registry consul --addr localhost:8500
```

**Inspect a specific service:**
```bash
kitex discover user-service --registry consul --addr localhost:8500
```

**JSON output for automation:**
```bash
kitex discover user-service --registry consul --addr localhost:8500 --format json
```

**YAML output:**
```bash
kitex discover user-service --registry consul --addr localhost:8500 --format yaml
```

**Filter by health status:**
```bash
kitex discover user-service --registry consul --addr localhost:8500 --healthy-only
```

**Filter by tag:**
```bash
kitex discover user-service --registry consul --addr localhost:8500 --tag production
```

### Command Options

```
  --registry string
        Registry type: consul, etcd, nacos, zookeeper (default "consul")
  --addr string
        Registry address (e.g., localhost:8500) [REQUIRED]
  --format string
        Output format: table, json, yaml (default "table")
  --healthy-only
        Show only healthy instances
  --tag string
        Filter by tag
  --help
        Show help message
```

## Output Examples

### Table Format (Default)

```
Service: user-service
+-----------------+--------+----------+----------------------+
| Instance        | Status | Weight   | Metadata             |
+-----------------+--------+----------+----------------------+
| 10.0.1.10:8080  | UP     | 100      | version=v1.2.3      |
| 10.0.1.11:8080  | UP     | 100      | version=v1.2.3      |
| 10.0.1.12:8080  | DOWN   | 0        | version=v1.2.2      |
+-----------------+--------+----------+----------------------+
```

### JSON Format

```json
{
  "name": "user-service",
  "instances": [
    {
      "id": "user-service-1",
      "address": "10.0.1.10",
      "port": 8080,
      "status": "UP",
      "weight": 100,
      "metadata": {
        "version": "v1.2.3"
      },
      "tags": ["production", "api"]
    }
  ]
}
```

### YAML Format

```yaml
name: user-service
instances:
  - id: user-service-1
    address: 10.0.1.10
    port: 8080
    status: UP
    weight: 100
    metadata:
      version: v1.2.3
    tags:
      - production
      - api
```

## Supported Registries

### Currently Implemented
- ✅ **Consul** - Full support with health checks and metadata

### Planned Support
- ⏳ **Etcd** - Coming soon
- ⏳ **Nacos** - Coming soon
- ⏳ **ZooKeeper** - Coming soon

## Examples

### Debugging Service Mesh Issues

**Check if service is registered:**
```bash
kitex discover my-service --registry consul --addr localhost:8500
```

**Find unhealthy instances:**
```bash
kitex discover my-service --registry consul --addr localhost:8500 | grep DOWN
```

**Get machine-readable output:**
```bash
kitex discover my-service --registry consul --addr localhost:8500 --format json | jq '.instances[] | select(.status=="UP")'
```

**Check production instances:**
```bash
kitex discover my-service --registry consul --addr localhost:8500 --tag production
```

## Testing

### Running Tests

```bash
# Run all discover tests
go test ./tool/cmd/kitex/discover/... -v

# Run formatter tests only
go test ./tool/cmd/kitex/discover/formatter/... -v

# Run with coverage
go test ./tool/cmd/kitex/discover/... -cover
```

### Testing with Docker Consul

If you don't have Consul installed, you can quickly test with Docker:

```bash
# Start Consul
docker run -d -p 8500:8500 --name=consul consul agent -dev -ui -client=0.0.0.0

# Register a test service
curl -X PUT http://localhost:8500/v1/agent/service/register \
  -d '{
    "ID": "test-service-1",
    "Name": "test-service",
    "Address": "127.0.0.1",
    "Port": 8080,
    "Tags": ["production", "v1"]
  }'

# Test discover command
kitex discover --registry consul --addr localhost:8500
kitex discover test-service --registry consul --addr localhost:8500

# Cleanup
docker stop consul && docker rm consul
```

## Development

### Project Structure

```
tool/cmd/kitex/discover/
├── discover.go              # Main command logic
├── discover_test.go         # Command tests
├── registry/
│   ├── registry.go          # Registry interface
│   ├── consul.go            # Consul implementation
│   └── consul_test.go       # Consul tests (coming soon)
└── formatter/
    ├── formatter.go         # Formatter interface
    ├── formatter_test.go    # Formatter tests
    ├── table.go             # Table formatter
    ├── json.go              # JSON formatter
    └── yaml.go              # YAML formatter
```

### Adding New Registry Support

To add support for a new registry (e.g., Etcd):

1. Create `registry/etcd.go`:
```go
type EtcdRegistry struct {
    client *clientv3.Client
}

func NewEtcdRegistry(address string) (Registry, error) {
    // Implementation
}

func (e *EtcdRegistry) ListServices() ([]string, error) {
    // Implementation
}

func (e *EtcdRegistry) GetService(name string) (*Service, error) {
    // Implementation
}

func (e *EtcdRegistry) Close() error {
    // Implementation
}
```

2. Update `registry/registry.go` in the `NewRegistry` function:
```go
case "etcd":
    return NewEtcdRegistry(cfg.Address)
```

3. Add tests in `registry/etcd_test.go`

## Troubleshooting

**Error: "connection refused"**
- Check if the registry service is running
- Verify the address and port are correct
- Check firewall settings

**Error: "unsupported registry type"**
- Check the --registry flag value
- Currently only "consul" is supported

**Empty service list**
- Verify services are registered in the registry
- Check if you have permission to query the registry
- Try accessing the registry UI directly

## Contributing

Contributions are welcome! To contribute:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

Please ensure:
- Code follows Go best practices
- All tests pass
- New features have test coverage >70%
- Documentation is updated

## License

Apache License 2.0 - see LICENSE file for details
