# Node Status Platform - Complete Tutorial

## Table of Contents
1. [Introduction](#introduction)
2. [Setting Up the Environment](#setting-up-the-environment)
3. [Starting the Backend](#starting-the-backend)
4. [Your First Node](#your-first-node)
5. [Managing Node Status](#managing-node-status)
6. [Working with Labels and Metadata](#working-with-labels-and-metadata)
7. [Real-time Event Monitoring](#real-time-event-monitoring)
8. [Building a Simple Sensor](#building-a-simple-sensor)
9. [Advanced Scenarios](#advanced-scenarios)
10. [Production Deployment](#production-deployment)

## Introduction

This tutorial will guide you through using the Node Status Platform from basic operations to advanced monitoring scenarios. By the end, you'll be able to:

- Deploy and configure the backend
- Create and manage infrastructure nodes
- Monitor real-time events
- Build custom sensors for automatic reporting
- Deploy the system in production

## Setting Up the Environment

### Step 1: Install Prerequisites

```bash
# Check Go version (need 1.22+)
go version

# Install Redis (macOS)
brew install redis
brew services start redis

# Install Redis (Ubuntu/Debian)
sudo apt update
sudo apt install redis-server
sudo systemctl start redis

# Install buf for proto generation
go install github.com/bufbuild/buf/cmd/buf@latest
```

### Step 2: Clone and Build the Project

```bash
# Clone the repository
git clone <repository-url>
cd nodestatus

# Install Go dependencies
make deps

# Generate proto code
make gen

# Build the binaries
make build
```

You should now have two binaries:
- `server` - The backend service
- `nodectl` - The CLI client

### Step 3: Prepare Configuration

Create a `.env` file for development:

```bash
cat > .env.development << EOF
ADMIN_TOKEN=my-dev-token-change-this
REDIS_ADDR=localhost:6379
REDIS_DB=0
GRPC_ADDR=:50051
HTTP_ADDR=:8080
LOG_LEVEL=info
EOF
```

## Starting the Backend

### Step 1: Start the Backend Server

```bash
# Load environment variables and start
source .env.development
./server
```

You should see output like:
```
2024-01-15T10:00:00.123Z  INFO  Starting gRPC server  {"addr": ":50051"}
2024-01-15T10:00:00.124Z  INFO  Starting HTTP server  {"addr": ":8080"}
```

### Step 2: Verify the Backend is Running

Open a new terminal:

```bash
# Check health endpoint
curl http://localhost:8080/healthz
# Expected: {"status":"healthy"}

# Check readiness
curl http://localhost:8080/readyz
# Expected: {"status":"ready"}
```

### Step 3: Access Swagger Documentation

Open your browser and navigate to:
```
http://localhost:8080/docs
```

You'll see the interactive Swagger UI with all available API operations.

## Your First Node

Let's create and manage your first infrastructure node using the CLI.

### Step 1: Set Up CLI Environment

```bash
# Export credentials for CLI
export BACKEND_ADDR=localhost:50051
export BACKEND_TOKEN=my-dev-token-change-this
```

### Step 2: Create a Virtual Machine Node

```bash
./nodectl create \
  --type VM \
  --name production-web-01 \
  --label env=production \
  --label service=web \
  --label region=us-east-1 \
  --metadata-json '{"cpu_cores": 8, "ram_gb": 32, "disk_gb": 500}'
```

Output:
```
Node created: 550e8400-e29b-41d4-a716-446655440000
```

### Step 3: List All Nodes

```bash
./nodectl list
```

Output:
```
ID                                    NAME               TYPE  STATUS   LAST_SEEN
550e8400-e29b-41d4-a716-446655440000  production-web-01  VM    UNKNOWN  2024-01-15T10:05:00Z
```

### Step 4: Get Node Details

```bash
./nodectl get 550e8400-e29b-41d4-a716-446655440000
```

Output (JSON):
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "VM",
  "name": "production-web-01",
  "labels": {
    "env": "production",
    "service": "web",
    "region": "us-east-1"
  },
  "status": "UNKNOWN",
  "last_seen": "2024-01-15T10:05:00Z",
  "metadata_json": "{\"cpu_cores\": 8, \"ram_gb\": 32, \"disk_gb\": 500}"
}
```

## Managing Node Status

### Step 1: Update Node Status to UP

```bash
./nodectl set-status 550e8400-e29b-41d4-a716-446655440000 --status UP
```

Output:
```
Status updated for node: 550e8400-e29b-41d4-a716-446655440000
```

### Step 2: Simulate a Problem - Set Status to DEGRADED

```bash
./nodectl set-status 550e8400-e29b-41d4-a716-446655440000 --status DEGRADED
```

### Step 3: Filter Nodes by Status

```bash
# List only degraded nodes
./nodectl list --status DEGRADED

# List only UP nodes
./nodectl list --status UP
```

## Working with Labels and Metadata

### Step 1: Create Multiple Nodes with Different Labels

```bash
# Create a database server
./nodectl create \
  --type BAREMETAL \
  --name db-master-01 \
  --label env=production \
  --label service=database \
  --label role=master

# Create a container node
./nodectl create \
  --type CONTAINER \
  --name api-service-01 \
  --label env=production \
  --label service=api \
  --label version=v2.1.0
```

### Step 2: Update Node Labels

```bash
# Get the node first
NODE_ID=$(./nodectl list --output json | jq -r '.[] | select(.name=="api-service-01") | .id')

# Update with new labels
./nodectl update $NODE_ID \
  --label version=v2.2.0 \
  --label deployment=blue
```

### Step 3: Filter by Type

```bash
# List only VMs
./nodectl list --type VM

# List only containers
./nodectl list --type CONTAINER
```

## Real-time Event Monitoring

One of the most powerful features is real-time event streaming.

### Step 1: Start Watching Events

Open a new terminal and run:

```bash
./nodectl watch
```

This will show:
```
Watching events (Ctrl+C to stop)...
--------------------------------------------------------------------------------
```

### Step 2: Generate Events in Another Terminal

```bash
# Create a new node
./nodectl create --type VM --name test-vm-02

# Update its status
./nodectl set-status <node-id> --status UP

# Delete it
./nodectl delete <node-id>
```

### Step 3: Observe Events in Watch Terminal

You'll see real-time updates:
```
[10:15:30] CREATED test-vm-02 (VM) - Status: UNKNOWN
[10:15:35] UPDATED test-vm-02 (VM) - Status: UP
  Changed fields: [status]
[10:15:40] DELETED test-vm-02 (VM) - Status: UP
```

### Step 4: Watch Events in JSON Format

```bash
./nodectl watch --output json
```

This outputs structured JSON for each event, perfect for processing by other tools.

## Building a Simple Sensor

Let's create a simple sensor that monitors a local service and reports its status.

### Step 1: Create the Sensor Script

Create `sensor-nginx.go`:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/metadata"

    nodev1 "github.com/melkior/nodestatus/gen/go/api/proto"
)

func main() {
    // Configuration
    backendAddr := os.Getenv("BACKEND_ADDR")
    token := os.Getenv("BACKEND_TOKEN")
    nodeName := "nginx-server-01"

    // Connect to backend
    conn, err := grpc.NewClient(backendAddr,
        grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer conn.Close()

    client := nodev1.NewNodeServiceClient(conn)
    ctx := metadata.AppendToOutgoingContext(context.Background(),
        "authorization", fmt.Sprintf("Bearer %s", token))

    // Create or update node
    node := &nodev1.Node{
        Name: nodeName,
        Type: nodev1.NodeType_CONTAINER,
        Labels: map[string]string{
            "service": "nginx",
            "sensor": "true",
        },
        MetadataJson: `{"port": 80}`,
    }

    resp, err := client.CreateNode(ctx, &nodev1.CreateNodeRequest{Node: node})
    if err != nil {
        // Node might already exist, try to get it
        getResp, _ := client.GetNode(context.Background(),
            &nodev1.GetNodeRequest{Id: nodeName})
        if getResp != nil {
            node = getResp.Node
        }
    } else {
        node = resp.Node
    }

    // Monitor loop
    for {
        status := checkNginx()

        _, err := client.UpdateStatus(ctx, &nodev1.UpdateStatusRequest{
            Id:     node.Id,
            Status: status,
        })

        if err != nil {
            log.Printf("Failed to update status: %v", err)
        } else {
            log.Printf("Updated %s status to %s", nodeName, status)
        }

        time.Sleep(30 * time.Second)
    }
}

func checkNginx() nodev1.NodeStatus {
    resp, err := http.Get("http://localhost:80")
    if err != nil {
        return nodev1.NodeStatus_DOWN
    }
    defer resp.Body.Close()

    if resp.StatusCode == 200 {
        return nodev1.NodeStatus_UP
    }
    return nodev1.NodeStatus_DEGRADED
}
```

### Step 2: Run the Sensor

```bash
# Build the sensor
go build -o nginx-sensor sensor-nginx.go

# Run it
BACKEND_ADDR=localhost:50051 BACKEND_TOKEN=my-dev-token-change-this ./nginx-sensor
```

The sensor will now continuously monitor nginx and update its status every 30 seconds.

## Advanced Scenarios

### Scenario 1: High Availability Monitoring

Monitor multiple database replicas:

```bash
# Create master node
./nodectl create --type BAREMETAL --name db-master \
  --label role=master --label cluster=main

# Create replica nodes
for i in {1..3}; do
  ./nodectl create --type VM --name db-replica-$i \
    --label role=replica --label cluster=main
done

# List all database nodes
./nodectl list --output json | jq '.[] | select(.labels.cluster=="main")'
```

### Scenario 2: Capacity Planning

Track resource utilization:

```bash
# Create nodes with capacity metadata
./nodectl create --type VM --name compute-01 \
  --metadata-json '{
    "total_cpu": 32,
    "used_cpu": 24,
    "total_ram_gb": 128,
    "used_ram_gb": 96,
    "utilization_pct": 75
  }'

# Query high utilization nodes
./nodectl list --output json | \
  jq '.[] | select(.metadata_json | fromjson.utilization_pct > 70)'
```

### Scenario 3: Deployment Tracking

Track deployments across environments:

```bash
# Tag nodes with deployment info
./nodectl update $NODE_ID \
  --label deployment_id=deploy-2024-01-15 \
  --label version=v2.5.0 \
  --label rollout_status=in_progress

# Monitor deployment progress
./nodectl list --output json | \
  jq '.[] | select(.labels.deployment_id=="deploy-2024-01-15") |
  {name, status: .status, rollout: .labels.rollout_status}'
```

### Scenario 4: Automated Remediation

Create a script that watches for issues and takes action:

```bash
#!/bin/bash
# auto-remediate.sh

while true; do
  # Get all DOWN nodes
  DOWN_NODES=$(./nodectl list --status DOWN --output json | \
    jq -r '.[].id')

  for node_id in $DOWN_NODES; do
    echo "Found DOWN node: $node_id"
    # Attempt restart (example action)
    echo "Attempting to restart node $node_id..."

    # Update status to UNKNOWN during restart
    ./nodectl set-status $node_id --status UNKNOWN

    # Simulate restart action
    sleep 5

    # Check if restart succeeded (example logic)
    if [ $((RANDOM % 2)) -eq 0 ]; then
      ./nodectl set-status $node_id --status UP
      echo "Node $node_id recovered"
    else
      ./nodectl set-status $node_id --status DOWN
      echo "Node $node_id still down, escalating..."
    fi
  done

  sleep 10
done
```

## Production Deployment

### Step 1: Prepare Production Configuration

```yaml
# docker-compose.production.yml
version: '3.8'

services:
  redis:
    image: redis:7-alpine
    command: redis-server --requirepass ${REDIS_PASSWORD}
    volumes:
      - redis-data:/data
    networks:
      - backend

  server:
    image: nodestatus-server:latest
    environment:
      ADMIN_TOKEN: ${ADMIN_TOKEN}
      REDIS_ADDR: redis:6379
      REDIS_PASSWORD: ${REDIS_PASSWORD}
      LOG_LEVEL: info
    ports:
      - "50051:50051"
      - "8080:8080"
    depends_on:
      - redis
    networks:
      - backend
    restart: always

volumes:
  redis-data:

networks:
  backend:
```

### Step 2: Deploy with Docker Compose

```bash
# Create production env file
cat > .env.production << EOF
ADMIN_TOKEN=$(openssl rand -hex 32)
REDIS_PASSWORD=$(openssl rand -hex 16)
EOF

# Start production stack
docker-compose -f docker-compose.production.yml up -d

# Check logs
docker-compose -f docker-compose.production.yml logs -f
```

### Step 3: Set Up Monitoring

```bash
# Create a monitoring dashboard
./nodectl watch --output json | while read event; do
  # Send to monitoring system
  echo $event | curl -X POST https://monitoring.example.com/events \
    -H "Content-Type: application/json" \
    -d @-
done
```

### Step 4: Configure Backup

```bash
#!/bin/bash
# backup.sh - Run daily via cron

# Backup Redis data
docker-compose exec redis redis-cli --rdb /data/backup.rdb

# Copy to backup location
docker cp $(docker-compose ps -q redis):/data/backup.rdb \
  /backup/redis-$(date +%Y%m%d).rdb

# Keep last 7 days
find /backup -name "redis-*.rdb" -mtime +7 -delete
```

## Summary

You've learned how to:
- ✅ Set up and configure the Node Status Platform
- ✅ Create and manage infrastructure nodes
- ✅ Monitor real-time events
- ✅ Build custom sensors
- ✅ Deploy in production

### Next Steps

1. **Explore the Sensor SDK**: Read the [Sensor SDK Documentation](./03-sensor-sdk.md)
2. **Integrate with Monitoring**: Connect to Prometheus, Grafana, or DataDog
3. **Build Custom Dashboards**: Create web UIs using the gRPC API
4. **Scale the System**: Deploy multiple backend instances with Redis Cluster

### Useful Commands Reference

```bash
# Create nodes
./nodectl create --type VM --name <name> --label key=value

# Update status
./nodectl set-status <id> --status UP|DOWN|DEGRADED|UNKNOWN

# Watch events
./nodectl watch [--output json]

# List with filters
./nodectl list [--type VM|BAREMETAL|CONTAINER] [--status UP|DOWN|...]

# Get details
./nodectl get <id>

# Update node
./nodectl update <id> --label key=value --metadata-json '{...}'

# Delete node
./nodectl delete <id>

# Show stats (with demo-sim)
./demo-sim stats [--json]
```