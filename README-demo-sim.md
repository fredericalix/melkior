# Demo-Sim: Node Service Simulation Tool

A comprehensive CLI tool for simulating long-running operations against the Node Service gRPC backend. This tool enables stress testing, performance validation, and demonstration of the platform's capabilities.

## Features

- **Seed Operation**: Create configurable datasets of nodes with realistic distributions
- **Continuous Simulation**: Run long-duration simulations with rate-limited operations
- **Cleanup**: Remove all simulator-created resources safely
- **Statistics**: Real-time monitoring of node distributions and statuses
- **Reproducible**: Seedable RNG for consistent test scenarios
- **Production-Ready**: Rate limiting, retry logic, graceful shutdown, and comprehensive error handling

## Installation

```bash
# Build the simulator
make build-sim

# Or install globally
go install ./cmd/demo-sim
```

## Quick Start

```bash
# 1. Seed 1000 nodes
BACKEND_ADDR=localhost:50051 BACKEND_TOKEN=secret \
  demo-sim seed --total 1000 --pct-baremetal 0.1 --pct-vm 0.6 --pct-container 0.3

# 2. Run simulation for 8 hours
BACKEND_ADDR=localhost:50051 BACKEND_TOKEN=secret \
  demo-sim run --duration 8h --update-qps 20 --max-concurrency 32

# 3. Check statistics
BACKEND_ADDR=localhost:50051 demo-sim stats

# 4. Cleanup when done
BACKEND_ADDR=localhost:50051 BACKEND_TOKEN=secret demo-sim cleanup --force
```

## Commands

### `seed` - Create Initial Dataset

Creates a configurable number of nodes with specified type distribution.

```bash
demo-sim seed [flags]
```

**Flags:**
- `--total` (default: 300) - Total number of nodes to create
- `--pct-baremetal` (default: 0.10) - Percentage of baremetal nodes
- `--pct-vm` (default: 0.50) - Percentage of VM nodes
- `--pct-container` (default: 0.40) - Percentage of container nodes
- `--labels` - Additional labels (repeatable, format: key=value)

**Example:**
```bash
demo-sim seed --total 500 \
  --pct-baremetal 0.05 \
  --pct-vm 0.65 \
  --pct-container 0.30 \
  --labels env=prod --labels region=us-east
```

### `run` - Continuous Simulation

Runs continuous operations against existing nodes.

```bash
demo-sim run [flags]
```

**Flags:**
- `--duration` (default: "0" = infinite) - How long to run (e.g., "6h", "30m")
- `--update-qps` (default: 15.0) - Target operations per second
- `--max-concurrency` (default: 32) - Maximum concurrent operations
- `--prob-status-flip` (default: 0.25) - Probability of status change
- `--prob-label-change` (default: 0.15) - Probability of label update
- `--prob-metadata-change` (default: 0.20) - Probability of metadata update
- `--prob-delete-and-recreate` (default: 0.02) - Probability of delete/recreate
- `--jitter` (default: true) - Add ±20% timing jitter
- `--batch-size` (default: 50) - Nodes per update tick
- `--names-pool` - Path to file with candidate names

**Example:**
```bash
demo-sim run \
  --duration 12h \
  --update-qps 25 \
  --max-concurrency 64 \
  --prob-status-flip 0.30 \
  --prob-delete-and-recreate 0.01
```

### `cleanup` - Remove Simulator Nodes

Removes all nodes created by the simulator (identified by labels).

```bash
demo-sim cleanup [flags]
```

**Flags:**
- `--force` - Skip confirmation prompt

**Example:**
```bash
demo-sim cleanup --force
```

### `stats` - Display Statistics

Shows current distribution of nodes by type and status.

```bash
demo-sim stats [flags]
```

**Flags:**
- `--json` - Output in JSON format

**Example:**
```bash
# Table format
demo-sim stats

# JSON format
demo-sim stats --json | jq .
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `BACKEND_ADDR` | localhost:50051 | gRPC backend address |
| `BACKEND_TOKEN` | (empty) | Admin token for mutations |
| `SIM_LABEL_PREFIX` | demo-sim/ | Prefix for simulator labels |
| `SIM_SEED` | random | RNG seed for reproducibility |

## Operation Probabilities

During the `run` command, operations are selected based on configured probabilities:

1. **Delete & Recreate** (`--prob-delete-and-recreate`): Complete node replacement
2. **Status Flip** (`--prob-status-flip`): Change node status (UP/DOWN/DEGRADED/UNKNOWN)
3. **Label Change** (`--prob-label-change`): Update node labels
4. **Metadata Change** (`--prob-metadata-change`): Update node metadata

Probabilities should sum to less than 1.0; remaining probability defaults to status flips.

## Rate Limiting

The simulator uses a token bucket algorithm for rate limiting:

- **QPS Target**: Set with `--update-qps`
- **Burst Capacity**: 2x the QPS rate
- **Concurrency**: Limited by `--max-concurrency`
- **Jitter**: Optional ±20% timing variation

## Error Handling

The simulator implements robust error handling:

- **Retry Logic**: Exponential backoff with jitter for transient errors
- **Auth Failures**: Fast fail with clear error messages
- **Name Conflicts**: Automatic retry with new names during seeding
- **Graceful Shutdown**: Completes in-flight operations on SIGINT/SIGTERM

## Monitoring

During execution, the simulator provides:

- **Live Stats**: Every 30 seconds during `run`
- **RPC Counts**: Creates, updates, deletes, errors
- **QPS Metrics**: Actual vs target throughput
- **Final Summary**: Complete statistics on shutdown

## Reproducible Testing

Use `SIM_SEED` for deterministic behavior:

```bash
# Run 1 - generates specific sequence
SIM_SEED=42 demo-sim seed --total 100

# Run 2 - same sequence
SIM_SEED=42 demo-sim seed --total 100
```

## Safety Features

- **Label-Based Identification**: All simulator nodes tagged with `demo=true` and `demo.owner=cli`
- **Batch Tracking**: Each seed operation gets unique `demo.batch` timestamp
- **Safe Cleanup**: Only removes nodes with simulator labels
- **Confirmation Prompts**: Cleanup requires confirmation (bypass with `--force`)

## Performance Considerations

- **Batch Operations**: Processes nodes in configurable batches
- **Connection Pooling**: Reuses gRPC connections
- **Concurrent Operations**: Bounded by `--max-concurrency`
- **Timeout Management**: 5-second timeout per RPC with retries

## Example Scenarios

### Load Testing
```bash
# Heavy load test
demo-sim seed --total 10000
demo-sim run --update-qps 100 --max-concurrency 128 --duration 1h
```

### Chaos Testing
```bash
# High churn simulation
demo-sim run --prob-delete-and-recreate 0.10 --prob-status-flip 0.40
```

### Long-Duration Stability
```bash
# 24-hour stability test
SIM_SEED=stable demo-sim seed --total 5000
demo-sim run --duration 24h --update-qps 10 --jitter true
```

## Troubleshooting

### Authentication Errors
```
Error: permission denied
```
Ensure `BACKEND_TOKEN` is set correctly.

### Connection Issues
```
Error: failed to create client
```
Verify `BACKEND_ADDR` and that the backend is running.

### Rate Limit Violations
```
Resource exhausted errors
```
Reduce `--update-qps` or increase backend capacity.

## Development

### Running Tests
```bash
go test ./internal/sim/...
```

### Adding New Operations
1. Add probability flag in `RunOptions`
2. Update `selectOperation()` in runner.go
3. Implement operation in `executeOperation()`

## License

Same as parent project