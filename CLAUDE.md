# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a production-ready gRPC-first infrastructure status tracking platform for monitoring bare-metal servers, VMs, and containers. The system uses Redis for storage and provides real-time event streaming capabilities.

## Essential Commands

### Development Workflow
```bash
# Generate code from proto definitions (MUST run after modifying .proto files)
make gen

# Build both server and CLI binaries
make build

# Run tests
make test

# Run a specific test
go test -v -run TestCreateNode ./internal/redisstore

# Format code
make fmt

# Run server locally (Redis must be running)
ADMIN_TOKEN=mysecret make run-server

# Test CLI commands
BACKEND_TOKEN=mysecret ./nodectl list
BACKEND_TOKEN=mysecret ./nodectl watch
```

### Docker Operations
```bash
# Run everything with Docker Compose
ADMIN_TOKEN=mysecret make docker-run

# Build Docker images
make docker-build

# Stop services
make docker-stop
```

## Architecture & Key Design Patterns

### gRPC-First Design
- ALL business logic goes through gRPC service (`/node.v1.NodeService/*`)
- No REST endpoints for business operations - HTTP server only serves health checks and Swagger docs
- Proto definitions in `api/proto/node.proto` are the single source of truth

### Authentication Pattern
- Admin token required for ALL mutating operations (Create, Update, Delete)
- Token passed via metadata header: `authorization: Bearer <token>`
- Auth interceptors in `internal/auth/interceptor.go` check mutating methods map
- Read operations (Get, List, Watch) do NOT require authentication

### Redis Storage Architecture
The system uses specific Redis patterns for performance:
- **Primary storage**: `node:{id}` → HASH containing all node fields
- **Name index**: `node:byname:{type}:{name}` → STRING (node ID) for uniqueness constraint
- **Set indexes** for fast filtering:
  - `nodes:all` → SET of all node IDs
  - `nodes:type:{type}` → SET of IDs by type
  - `nodes:status:{status}` → SET of IDs by status
- **Event stream**: `nodes:events` → Redis STREAM for append-only event log

### Event Streaming Pattern
Real-time events use a fan-out broker pattern:
1. Service operations publish to in-memory broker (`internal/events/broker.go`)
2. WatchEvents RPC subscribes clients to broker
3. Background goroutine polls Redis Stream for missed events
4. Events include snapshot + changed_fields for efficient updates

### Environment Variable Precedence
- `PORT` takes precedence over `HTTP_ADDR` for HTTP server (cloud-friendly)
- `ADMIN_TOKEN` is REQUIRED - server won't start without it
- Redis connection tested on startup with 5-second timeout

## Critical Implementation Details

### Proto Generation
- Uses buf (not protoc directly) for reproducible builds
- Generated code goes to `gen/go/` directory
- Must run `buf dep update` before `buf generate` when dependencies change
- Generated files use `paths=source_relative` option

### Index Management
When updating nodes:
1. Delete old indexes BEFORE updating node data
2. Save new node data with new indexes atomically using pipeline
3. Append event to stream AFTER successful save

### Testing Approach
- Uses miniredis for Redis unit tests (no real Redis needed)
- Auth interceptor tests verify all permission scenarios
- Store tests cover CRUD + index operations

### CLI Tool Design
- `nodectl` uses cobra for command structure
- All mutations require `BACKEND_TOKEN` env var or `--token` flag
- Watch command uses streaming RPC for real-time updates
- List/Get commands support JSON output format

## Common Pitfall Avoidances

### When Adding New Proto Fields
1. Update proto file
2. Run `make gen`
3. Update `nodeFromHash` and `saveNode` in redisstore
4. Update `getChangedFields` if field should trigger events
5. Add field to CLI create/update commands if user-settable

### When Adding New Status Values
1. Add to proto enum
2. Regenerate with `make gen`
3. Update CLI autocomplete in relevant commands
4. No Redis schema changes needed (stored as int32)

### When Modifying Authentication
- Add method to `mutatingMethods` map in auth interceptor
- Mutations MUST return appropriate gRPC status codes (InvalidArgument, NotFound, etc.)
- Always log mutations with zap structured logging