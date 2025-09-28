# Node Status Platform

A production-ready gRPC-first platform for tracking operational status of infrastructure components (bare-metal servers, VMs, containers).

## Features

- **gRPC-Only Business Logic**: All operations via strongly-typed gRPC with streaming support
- **Real-time Event Streaming**: Watch create/update/delete events as they happen
- **Redis-Backed Storage**: Fast, indexed storage with append-only event stream
- **Secure by Default**: Admin token authentication for all mutations
- **Clean Architecture**: Domain-driven design with clear separation of concerns
- **Interactive TUI Dashboard**: Real-time terminal UI with charts and visualizations
- **CLI Tools**: Command-line interface for automation and scripting
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

## TUI Dashboard

The `nodectl` tool now defaults to an interactive Terminal UI dashboard with real-time monitoring capabilities.

### Launching the TUI

```bash
# Default: Launch interactive TUI
nodectl

# Or explicitly
nodectl tui

# With custom settings
nodectl tui --fps 10 --charts-refresh 100 --window-secs 600

# Test with mock data (no backend required)
BACKEND_ADDR=mock nodectl tui
```

### TUI Environment Variables

```bash
BACKEND_ADDR=localhost:50051  # gRPC backend address
BACKEND_TOKEN=your-token      # Authentication token
TUI_FPS=8                      # UI refresh rate (default: 8 FPS)
CHARTS_REFRESH=250             # Charts update interval in ms
WINDOW_SECS=300                # Time window for metrics (default: 5 min)
```

### TUI Views

```
┌─────────────────────────────────────────────────────────────────┐
│ List │ Details │ Logs                                          │
├─────────────────────────────────────────────────────────────────┤
│ ID          │ Name         │ Type      │ Status │ Last Seen    │
│─────────────┼──────────────┼───────────┼────────┼──────────────│
│ 550e8400... │ web-01       │ VM        │ UP     │ 10:15:30     │
│ 660f9511... │ db-master    │ BAREMETAL │ UP     │ 10:15:28     │
│ 770a0622... │ api-service  │ CONTAINER │ DOWN   │ 10:14:45     │
│ 880b1733... │ worker-02    │ VM        │ DEGRADED│ 10:15:25    │
├─────────────────────────────────────────────────────────────────┤
│ Total: 4 | UP: 2 | DOWN: 1 | DEGRADED: 1 | UNKNOWN: 0         │
│ [↑/↓] Navigate [Enter] Details [f] Filter [c] Charts [q] Quit  │
└─────────────────────────────────────────────────────────────────┘
```

The dashboard provides multiple views accessible via tabs:

1. **List View**: Table of all nodes with filtering
   - Shows ID, Name, Type, Status, Last Seen
   - Footer displays status distribution counts
   - Filterable by type and status

2. **Details View**: Detailed information for selected node
   - Full node properties
   - Labels and metadata
   - Scrollable for long content

3. **Logs View**: Real-time event stream
   - Shows CREATE, UPDATE, DELETE events
   - Timestamp and changed fields
   - Auto-scroll with manual override

### Charts View (Full-Screen)

Press `c` to open full-screen charts with real-time visualizations:

```
┌────────────────────────────────────────────────────────────────┐
│                        Status Distribution                     │
│     ╭─────╮                  ┌─────────────────────────┐      │
│    ╱       ╲    UP: 45%      │ Node Types              │      │
│   │   ●●●   │   DOWN: 20%    │ ████████ BAREMETAL (8)  │      │
│   │  ●   ●  │   DEGRADED: 25%│ ██████   VM (6)         │      │
│    ╲  ●●●  ╱    UNKNOWN: 10% │ ████     CONTAINER (4)  │      │
│     ╰─────╯                   └─────────────────────────┘      │
├────────────────────────────────────────────────────────────────┤
│                     Status Over Time (5 min)                   │
│  100│     ╱╲                                                   │
│   75│    ╱  ╲    ╱╲                                           │
│   50│───╯    ╲__╱  ╲___   UP ────                            │
│   25│                   ╲  DOWN ····                           │
│    0│________________________DEGRADED ----                     │
│      10:10  10:11  10:12  10:13  10:14  10:15                 │
├────────────────────────────────────────────────────────────────┤
│ Events/sec [████████░░] 8.5  │ Mutations/sec [██░░░░] 2.1     │
│ Press 'q' or 'esc' to return │                                 │
└────────────────────────────────────────────────────────────────┘
```

- **Status Donut**: Distribution of node statuses (UP/DOWN/DEGRADED/UNKNOWN)
- **Type Bar Chart**: Count by node type (BAREMETAL/VM/CONTAINER)
- **Time Series**: Historical status counts over time window
- **Gauges**: Events/sec and Mutations/sec metrics

### Keyboard Shortcuts

#### Global
- `q`, `Ctrl+C`: Quit application
- `?`: Toggle help
- `Tab`, `→`: Next tab
- `←`: Previous tab
- `c`: Open charts view

#### List View
- `↑/k`, `↓/j`: Navigate table
- `Enter`: Show selected node details
- `f`: Toggle filters
- `r`: Reset filters
- `PgUp/PgDn`: Page navigation

#### Details/Logs View
- `↑/k`, `↓/j`: Scroll content
- `PgUp/PgDn`: Page scroll
- `Home/End`: Jump to start/end
- `a`: Toggle auto-scroll (logs only)

#### Charts View
- `Esc`, `q`: Return to main dashboard
- Charts auto-update based on CHARTS_REFRESH setting

### Terminal Requirements

- **Minimum Size**: 80x24 characters
- **Recommended**: 120x40 or larger for best experience
- **Color Support**: 256 colors or true color terminal
- **Font**: Monospace font with Unicode support

### Troubleshooting TUI

**Issue: Characters appear garbled**
- Ensure terminal supports UTF-8
- Set `export LANG=en_US.UTF-8`

**Issue: Charts not rendering correctly**
- Check terminal size: `echo $COLUMNS $LINES`
- Try different terminal emulator (iTerm2, Windows Terminal, etc.)

**Issue: Slow performance**
- Reduce FPS: `nodectl tui --fps 4`
- Increase charts refresh: `nodectl tui --charts-refresh 500`

**Issue: Connection errors**
- Verify backend is running: `nc -zv localhost 50051`
- Check token: `echo $BACKEND_TOKEN`
- Try mock mode: `BACKEND_ADDR=mock nodectl tui`

## CLI Usage (Legacy Mode)

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