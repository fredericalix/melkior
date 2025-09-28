# Node Status Platform

A production-ready gRPC-first platform for tracking operational status of infrastructure components (bare-metal servers, VMs, containers).

## Features

- **gRPC-Only Business Logic**: All operations via strongly-typed gRPC with streaming support
- **Real-time Event Streaming**: Watch create/update/delete events as they happen
- **Redis-Backed Storage**: Fast, indexed storage with append-only event stream
- **Secure by Default**: Admin token authentication for all mutations
- **Clean Architecture**: Domain-driven design with clear separation of concerns
- **CLI Dashboard**: Interactive command-line tool for management and monitoring
- **Swagger Documentation**: Auto-generated API docs from proto definitions

## Quick Start

### Prerequisites

- Go 1.22+
- Redis 7+
- Docker & Docker Compose (optional)
- buf CLI for proto generation

### Installation

```bash
# Clone the repository
git clone <repository-url>
cd nodestatus

# Install dependencies
make deps

# Generate proto files
make gen

# Build binaries
make build
```

### Running with Docker Compose

```bash
# Set admin token
export ADMIN_TOKEN=your-secure-token

# Start services
make docker-run
```

This starts:
- Redis on port 6379
- gRPC server on port 50051
- HTTP docs server on port 8080

### Running Locally

```bash
# Start Redis (required)
redis-server

# Start the server
ADMIN_TOKEN=your-secure-token make run-server

# In another terminal, use the CLI
BACKEND_TOKEN=your-secure-token ./nodectl list
```

## CLI Usage

### Watch Events

Monitor real-time events:

```bash
nodectl watch
nodectl watch --output json
```

### List Nodes

```bash
# List all nodes
nodectl list

# Filter by type
nodectl list --type VM

# Filter by status
nodectl list --status UP

# JSON output
nodectl list --output json
```

### Create Node

```bash
nodectl create \
  --type VM \
  --name my-server \
  --label env=prod \
  --label region=us-east \
  --metadata-json '{"cpu": 8, "ram": 32}'
```

### Update Node

```bash
nodectl update <node-id> \
  --name new-name \
  --label env=staging \
  --metadata-json '{"cpu": 16}'
```

### Update Status

```bash
nodectl set-status <node-id> --status UP
nodectl set-status <node-id> --status DEGRADED
```

### Delete Node

```bash
nodectl delete <node-id>
```

### Get Node Details

```bash
nodectl get <node-id>
```

## Architecture

### Domain Model

**Node Entity**:
- `id`: UUID v4 (auto-generated if empty)
- `type`: BAREMETAL, VM, or CONTAINER
- `name`: Unique per type
- `labels`: Key-value pairs for categorization
- `status`: UNKNOWN, UP, DOWN, or DEGRADED
- `last_seen`: Timestamp of last update
- `metadata_json`: Arbitrary JSON metadata

**Events**:
- `event_type`: CREATED, UPDATED, or DELETED
- `node`: Snapshot after change
- `changed_fields`: List of modified fields

### Redis Schema

```
node:{id}                    → HASH (node data)
node:byname:{type}:{name}    → STRING (node id)
nodes:all                    → SET (all node ids)
nodes:type:{type}            → SET (node ids by type)
nodes:status:{status}        → SET (node ids by status)
nodes:events                 → STREAM (append-only event log)
```

### API Endpoints

**gRPC** (port 50051):
- All business operations via gRPC
- Reflection enabled for debugging

**HTTP** (port 8080):
- `/healthz` - Health check
- `/readyz` - Readiness check
- `/openapi.json` - OpenAPI specification
- `/docs` - Swagger UI

## Configuration

All configuration via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_ADDR` | localhost:6379 | Redis server address |
| `REDIS_DB` | 0 | Redis database number |
| `REDIS_PASSWORD` | (empty) | Redis password |
| `GRPC_ADDR` | :50051 | gRPC server address |
| `PORT` | 8080 | HTTP server port (takes precedence over HTTP_ADDR) |
| `HTTP_ADDR` | :8080 | HTTP server address (use PORT for cloud deployments) |
| `ADMIN_TOKEN` | (required) | Admin authentication token |
| `LOG_LEVEL` | info | Log level (debug/info/warn/error) |

Note: The `PORT` environment variable takes precedence over `HTTP_ADDR` for the HTTP server. This is useful for cloud deployments (Heroku, Cloud Run, etc.) that set the PORT variable automatically.

CLI Configuration:

| Variable | Default | Description |
|----------|---------|-------------|
| `BACKEND_ADDR` | localhost:50051 | Backend gRPC address |
| `BACKEND_TOKEN` | (empty) | Admin token for mutations |

## Development

### Running Tests

```bash
make test
```

### Code Generation

After modifying proto files:

```bash
make gen
```

### Formatting

```bash
make fmt
```

### Building Docker Images

```bash
make docker-build
```

## Security

- All mutating operations require admin token authentication
- Token passed via `authorization: Bearer <token>` header
- Audit logging for all mutations
- TLS support ready (configure via gRPC options)

## Monitoring

The platform provides several monitoring capabilities:

1. **Real-time Event Stream**: Subscribe to all changes via `WatchEvents` RPC
2. **Health Endpoints**: HTTP endpoints for liveness and readiness probes
3. **Structured Logging**: JSON-formatted logs with correlation IDs
4. **Metrics Ready**: Easy to add Prometheus metrics via interceptors

## Performance

- Redis-backed for sub-millisecond operations
- Indexed queries for fast filtering
- Event streaming with fan-out to multiple subscribers
- Connection pooling and optimized Redis commands
- Graceful shutdown with context propagation

## License

[Your License Here]