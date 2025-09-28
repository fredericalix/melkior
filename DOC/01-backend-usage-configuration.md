# Backend Usage and Configuration Guide

## Table of Contents
1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Installation](#installation)
4. [Configuration](#configuration)
5. [Running the Backend](#running-the-backend)
6. [API Documentation](#api-documentation)
7. [Health Monitoring](#health-monitoring)
8. [Security](#security)
9. [Troubleshooting](#troubleshooting)

## Overview

The Node Status Platform is a production-ready gRPC-first infrastructure monitoring system designed to track the operational status of bare-metal servers, virtual machines, and containers. It provides real-time event streaming, fast indexed queries, and a robust API for integration with monitoring tools.

### Key Features
- **gRPC-Only Business Logic**: All operations use strongly-typed Protocol Buffers
- **Real-time Event Streaming**: Watch infrastructure changes as they happen
- **Redis-Backed Storage**: Sub-millisecond operations with indexed queries
- **Token-Based Security**: Admin authentication for all mutations
- **Swagger Documentation**: Auto-generated API documentation from proto definitions

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Client Applications                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │ Sensors  │  │   CLI    │  │Dashboard │  │Monitoring│   │
│  │(Agents)  │  │(nodectl) │  │   (UI)   │  │  Tools   │   │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘   │
│       │             │             │             │           │
└───────┼─────────────┼─────────────┼─────────────┼───────────┘
        │             │             │             │
        └─────────────┴──────┬──────┴─────────────┘
                             │
                    ┌────────▼────────┐
                    │   gRPC API      │
                    │   Port: 50051   │
                    └────────┬────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                Backend                  │
        │                                          │
        │  ┌──────────────────────────────────┐   │
        │  │         Node Service             │   │
        │  │  • Create/Update/Delete Nodes    │   │
        │  │  • Status Management             │   │
        │  │  • Event Streaming               │   │
        │  └─────────────┬────────────────────┘   │
        │                │                        │
        │  ┌─────────────▼────────────────────┐   │
        │  │     Redis Storage Layer          │   │
        │  │  • Node Data (Hash)              │   │
        │  │  • Indexes (Sets)                │   │
        │  │  • Event Stream (Stream)         │   │
        │  └──────────────────────────────────┘   │
        │                                          │
        │  ┌──────────────────────────────────┐   │
        │  │      HTTP Documentation          │   │
        │  │        Port: 8080                │   │
        │  │  • /healthz  • /readyz           │   │
        │  │  • /docs     • /openapi.json     │   │
        │  └──────────────────────────────────┘   │
        └──────────────────────────────────────────┘
```

## Installation

### Prerequisites
- Go 1.22 or higher
- Redis 7.0 or higher
- Docker & Docker Compose (optional)
- buf CLI for proto generation

### Building from Source

```bash
# Clone the repository
git clone <repository-url>
cd nodestatus

# Install dependencies
make deps

# Generate protobuf code
make gen

# Build the backend
make build
```

### Using Docker

```bash
# Build Docker image
docker build -f Dockerfile.server -t nodestatus-server:latest .

# Or use docker-compose
docker-compose build
```

## Configuration

The backend is configured entirely through environment variables, following the 12-factor app methodology.

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ADMIN_TOKEN` | **Yes** | - | Authentication token for mutating operations |
| `REDIS_ADDR` | No | `localhost:6379` | Redis server address |
| `REDIS_DB` | No | `0` | Redis database number (0-15) |
| `REDIS_PASSWORD` | No | - | Redis authentication password |
| `GRPC_ADDR` | No | `:50051` | gRPC server listen address |
| `HTTP_ADDR` | No | `:8080` | HTTP server address for docs/health |
| `PORT` | No | - | HTTP port (overrides HTTP_ADDR for cloud deployments) |
| `LOG_LEVEL` | No | `info` | Logging level (debug/info/warn/error) |

### Configuration Examples

#### Development Configuration
```bash
# .env.development
ADMIN_TOKEN=dev-secret-token
REDIS_ADDR=localhost:6379
REDIS_DB=0
GRPC_ADDR=:50051
HTTP_ADDR=:8080
LOG_LEVEL=debug
```

#### Production Configuration
```bash
# .env.production
ADMIN_TOKEN=${SECRET_ADMIN_TOKEN}  # From secrets management
REDIS_ADDR=redis-cluster.internal:6379
REDIS_PASSWORD=${REDIS_PASSWORD}
REDIS_DB=0
GRPC_ADDR=:50051
HTTP_ADDR=:8080
LOG_LEVEL=info
```

#### Cloud Deployment (Heroku/Cloud Run)
```bash
# Automatically set by platform
PORT=8080

# Your configuration
ADMIN_TOKEN=${SECRET_ADMIN_TOKEN}
REDIS_ADDR=${REDIS_URL}
LOG_LEVEL=info
```

## Running the Backend

### Local Development

```bash
# Start Redis
redis-server

# Run the backend
ADMIN_TOKEN=your-secret-token make run-server

# Or directly
ADMIN_TOKEN=your-secret-token ./server
```

### Using Docker Compose

```bash
# Start all services
ADMIN_TOKEN=your-secret-token docker-compose up

# Run in background
ADMIN_TOKEN=your-secret-token docker-compose up -d

# View logs
docker-compose logs -f server

# Stop services
docker-compose down
```

### Production Deployment

```bash
# Using systemd service file
sudo systemctl start nodestatus

# Using Docker
docker run -d \
  --name nodestatus-server \
  -e ADMIN_TOKEN=${ADMIN_TOKEN} \
  -e REDIS_ADDR=redis:6379 \
  -p 50051:50051 \
  -p 8080:8080 \
  nodestatus-server:latest
```

## API Documentation

### Accessing Swagger UI

The backend automatically generates and serves OpenAPI/Swagger documentation from the proto definitions.

1. **Start the backend** with proper configuration
2. **Open your browser** and navigate to:
   ```
   http://localhost:8080/docs
   ```

3. **View the API specification** at:
   ```
   http://localhost:8080/openapi.json
   ```

### Swagger UI Features

The Swagger interface provides:
- **Interactive API Explorer**: Test endpoints directly from the browser
- **Request/Response Examples**: See expected payloads
- **Authentication Details**: Token requirements for each endpoint
- **Type Definitions**: Complete message schemas
- **Error Codes**: Possible error responses

### API Endpoints Overview

```
gRPC Service: NodeService (port 50051)
├── CreateNode    [Auth Required]
├── UpdateNode    [Auth Required]
├── UpdateStatus  [Auth Required]
├── DeleteNode    [Auth Required]
├── GetNode       [No Auth]
├── ListNodes     [No Auth]
└── WatchEvents   [No Auth] (Streaming)

HTTP Endpoints (port 8080)
├── /healthz      - Liveness probe
├── /readyz       - Readiness probe
├── /openapi.json - OpenAPI specification
└── /docs         - Swagger UI
```

## Health Monitoring

### Health Check Endpoints

#### Liveness Probe (`/healthz`)
Indicates if the server is running:
```bash
curl http://localhost:8080/healthz
# Response: {"status":"healthy"}
```

#### Readiness Probe (`/readyz`)
Indicates if the server is ready to handle requests:
```bash
curl http://localhost:8080/readyz
# Response: {"status":"ready"}
```

The readiness probe verifies:
- Redis connection is active
- Database queries are working
- Server is fully initialized

### Kubernetes Integration

```yaml
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: nodestatus
    image: nodestatus-server:latest
    livenessProbe:
      httpGet:
        path: /healthz
        port: 8080
      initialDelaySeconds: 10
      periodSeconds: 10
    readinessProbe:
      httpGet:
        path: /readyz
        port: 8080
      initialDelaySeconds: 5
      periodSeconds: 5
```

## Security

### Authentication

All mutating operations require authentication via Bearer token:

```
Authorization: Bearer <ADMIN_TOKEN>
```

Protected operations:
- `CreateNode`
- `UpdateNode`
- `UpdateStatus`
- `DeleteNode`

### Best Practices

1. **Token Management**
   - Use strong, randomly generated tokens (min 32 characters)
   - Rotate tokens regularly
   - Never commit tokens to version control
   - Use secrets management systems

2. **Network Security**
   - Use TLS for production deployments
   - Restrict network access to gRPC ports
   - Use firewalls and security groups

3. **Redis Security**
   - Enable Redis AUTH
   - Use Redis ACLs for fine-grained control
   - Enable TLS for Redis connections
   - Regular backups

### TLS Configuration (Production)

```go
// Example TLS setup for gRPC
creds, _ := credentials.NewServerTLSFromFile("server.crt", "server.key")
grpcServer := grpc.NewServer(grpc.Creds(creds))
```

## Troubleshooting

### Common Issues

#### 1. Authentication Errors
```
Error: rpc error: code = Unauthenticated desc = missing authorization header
```
**Solution**: Ensure `ADMIN_TOKEN` is set and included in requests

#### 2. Redis Connection Failed
```
Error: failed to connect to Redis: dial tcp localhost:6379: connection refused
```
**Solution**:
- Verify Redis is running: `redis-cli ping`
- Check `REDIS_ADDR` configuration
- Verify network connectivity

#### 3. Port Already in Use
```
Error: listen tcp :50051: bind: address already in use
```
**Solution**:
- Check for existing processes: `lsof -i :50051`
- Change port via `GRPC_ADDR` environment variable

#### 4. High Memory Usage
**Symptoms**: Increasing memory consumption over time
**Solution**:
- Check Redis memory usage: `redis-cli INFO memory`
- Implement event stream trimming
- Monitor connection leaks

### Debug Mode

Enable detailed logging for troubleshooting:

```bash
LOG_LEVEL=debug ADMIN_TOKEN=secret ./server
```

Debug logs include:
- All gRPC requests/responses
- Redis operations
- Authentication attempts
- Performance metrics

### Performance Tuning

#### Redis Optimization
```bash
# redis.conf
maxmemory 2gb
maxmemory-policy allkeys-lru
save ""  # Disable RDB snapshots for performance
```

#### Connection Pooling
```go
// Adjust in code if needed
redis.Options{
    PoolSize: 100,
    MinIdleConns: 10,
}
```

### Monitoring Metrics

Key metrics to monitor:
- **RPC Latency**: p50, p95, p99
- **Redis Operations/sec**
- **Active gRPC Connections**
- **Event Stream Lag**
- **Error Rates by RPC**

### Getting Help

1. **Check Logs**: Most issues are visible in server logs
2. **Enable Debug Mode**: `LOG_LEVEL=debug` for detailed output
3. **Test Connectivity**: Use `grpcurl` to test gRPC endpoints
4. **Verify Configuration**: Double-check all environment variables
5. **Review Documentation**: Check proto definitions for API details

## Next Steps

- [Tutorial: Getting Started](./02-tutorial.md) - Step-by-step guide
- [Sensor SDK Documentation](./03-sensor-sdk.md) - Building monitoring agents
- [API Reference](../gen/openapiv2/openapi.swagger.json) - Complete API specification