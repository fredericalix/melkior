# Sensor SDK Documentation

## Table of Contents
1. [Overview](#overview)
2. [Architecture](#architecture)
3. [SDK Installation](#sdk-installation)
4. [Core Concepts](#core-concepts)
5. [Building Your First Sensor](#building-your-first-sensor)
6. [Advanced Sensor Patterns](#advanced-sensor-patterns)
7. [Language SDKs](#language-sdks)
8. [Best Practices](#best-practices)
9. [Example Sensors](#example-sensors)
10. [Troubleshooting](#troubleshooting)

## Overview

Sensors are autonomous agents that monitor infrastructure components and report their status to the Node Status Platform. They act as the eyes and ears of your monitoring system, collecting real-time data and updating node status automatically.

### What is a Sensor?

A sensor is a lightweight program that:
- **Monitors** a specific infrastructure component (server, VM, container, service)
- **Collects** status information and metrics
- **Reports** to the backend via gRPC
- **Updates** node status based on health checks
- **Streams** events for real-time monitoring

### Sensor Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     Infrastructure                       │
│                                                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────┐ │
│  │  Server  │  │    VM    │  │Container │  │Service │ │
│  │   #1     │  │   #2     │  │   #3     │  │  API   │ │
│  └─────┬────┘  └─────┬────┘  └────┬─────┘  └───┬────┘ │
│        │             │            │             │       │
└────────┼─────────────┼────────────┼─────────────┼───────┘
         │             │            │             │
    ┌────▼────┐  ┌────▼────┐  ┌───▼────┐  ┌────▼────┐
    │ Sensor  │  │ Sensor  │  │ Sensor │  │ Sensor  │
    │  Agent  │  │  Agent  │  │  Agent │  │  Agent  │
    └────┬────┘  └────┬────┘  └───┬────┘  └────┬────┘
         │             │            │             │
         └─────────────┴────────────┴─────────────┘
                             │
                    ┌────────▼────────┐
                    │   gRPC Client   │
                    │  (Sensor SDK)   │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │  Node Service   │
                    │  Backend API    │
                    └─────────────────┘
```

## Architecture

### Sensor Components

```
┌──────────────────────────────────────┐
│            Sensor Agent              │
│                                      │
│  ┌────────────────────────────────┐ │
│  │     Configuration Layer        │ │
│  │  • Environment Variables       │ │
│  │  • Config Files               │ │
│  │  • Service Discovery          │ │
│  └───────────┬────────────────────┘ │
│              │                      │
│  ┌───────────▼────────────────────┐ │
│  │     Health Check Engine        │ │
│  │  • HTTP/HTTPS Checks          │ │
│  │  • TCP Port Checks            │ │
│  │  • Process Monitoring         │ │
│  │  • Custom Health Logic        │ │
│  └───────────┬────────────────────┘ │
│              │                      │
│  ┌───────────▼────────────────────┐ │
│  │     Metrics Collector          │ │
│  │  • CPU/Memory/Disk            │ │
│  │  • Network Stats              │ │
│  │  • Application Metrics        │ │
│  └───────────┬────────────────────┘ │
│              │                      │
│  ┌───────────▼────────────────────┐ │
│  │     Status Evaluator           │ │
│  │  • Threshold Analysis         │ │
│  │  • Trend Detection            │ │
│  │  • Anomaly Detection          │ │
│  └───────────┬────────────────────┘ │
│              │                      │
│  ┌───────────▼────────────────────┐ │
│  │     gRPC Reporter              │ │
│  │  • Connection Management      │ │
│  │  • Authentication             │ │
│  │  • Retry Logic                │ │
│  │  • Batching                   │ │
│  └────────────────────────────────┘ │
└──────────────────────────────────────┘
```

### Communication Flow

```
Sensor                    Backend
  │                         │
  ├─[1. Connect]──────────► │
  │                         │
  ├─[2. Authenticate]─────► │
  │◄────────[Token OK]───── │
  │                         │
  ├─[3. Create/Get Node]──► │
  │◄────────[Node ID]────── │
  │                         │
  │    ┌─────────────┐      │
  │    │ Monitor     │      │
  │    │   Loop      │      │
  │    └──────┬──────┘      │
  │           │             │
  ├─[4. Check Health]       │
  │           │             │
  ├─[5. Update Status]────► │
  │◄────────[ACK]────────── │
  │           │             │
  │        [Sleep]          │
  │           │             │
  └───────[Repeat]          │
```

## SDK Installation

### Go SDK

```bash
go get github.com/melkior/nodestatus/pkg/sensor
```

### Python SDK

```bash
pip install nodestatus-sensor
```

### Node.js SDK

```bash
npm install @nodestatus/sensor
```

## Core Concepts

### 1. Node Identity

Every sensor manages one or more nodes. Each node has:
- **Unique ID**: Auto-generated UUID or custom identifier
- **Name**: Human-readable identifier
- **Type**: BAREMETAL, VM, or CONTAINER
- **Labels**: Key-value metadata for categorization
- **Metadata**: JSON blob for extended information

### 2. Status Model

Sensors report one of four statuses:
- **UP**: Component is healthy and operational
- **DOWN**: Component is not responding or failed
- **DEGRADED**: Component is operational but with issues
- **UNKNOWN**: Status cannot be determined

### 3. Health Checks

Types of health checks sensors can perform:
- **Liveness**: Is the component running?
- **Readiness**: Can it handle requests?
- **Performance**: Is it meeting SLAs?
- **Resource**: Are resources within limits?

### 4. Reporting Strategies

- **Periodic**: Report at fixed intervals
- **On-Change**: Report only when status changes
- **Heartbeat**: Regular "alive" signals with full updates on change
- **Threshold**: Report when metrics cross thresholds

## Building Your First Sensor

### Go Sensor Example

```go
package main

import (
    "context"
    "log"
    "os"
    "time"

    "github.com/melkior/nodestatus/pkg/sensor"
)

func main() {
    // Initialize sensor
    s, err := sensor.New(sensor.Config{
        BackendAddr: os.Getenv("BACKEND_ADDR"),
        Token:      os.Getenv("BACKEND_TOKEN"),
        NodeName:   "my-service",
        NodeType:   sensor.TypeContainer,
        Labels: map[string]string{
            "service": "api",
            "version": "1.0.0",
        },
    })
    if err != nil {
        log.Fatal(err)
    }
    defer s.Close()

    // Start monitoring
    s.StartMonitoring(30*time.Second, func() sensor.Status {
        // Custom health check logic
        if checkServiceHealth() {
            return sensor.StatusUp
        }
        return sensor.StatusDown
    })

    // Keep running
    select {}
}

func checkServiceHealth() bool {
    // Your health check logic here
    return true
}
```

### Python Sensor Example

```python
import os
import time
from nodestatus_sensor import Sensor, NodeType, Status

def check_health():
    """Custom health check logic"""
    # Check if service is responding
    try:
        response = requests.get("http://localhost:8080/health")
        return Status.UP if response.status_code == 200 else Status.DEGRADED
    except:
        return Status.DOWN

def main():
    # Initialize sensor
    sensor = Sensor(
        backend_addr=os.getenv("BACKEND_ADDR"),
        token=os.getenv("BACKEND_TOKEN"),
        node_name="python-service",
        node_type=NodeType.CONTAINER,
        labels={
            "service": "web",
            "language": "python"
        }
    )

    # Start monitoring
    sensor.start_monitoring(
        interval=30,
        health_check=check_health
    )

if __name__ == "__main__":
    main()
```

### Node.js Sensor Example

```javascript
const { Sensor, NodeType, Status } = require('@nodestatus/sensor');

async function checkHealth() {
    // Custom health check logic
    try {
        const response = await fetch('http://localhost:3000/health');
        return response.ok ? Status.UP : Status.DEGRADED;
    } catch (error) {
        return Status.DOWN;
    }
}

async function main() {
    // Initialize sensor
    const sensor = new Sensor({
        backendAddr: process.env.BACKEND_ADDR,
        token: process.env.BACKEND_TOKEN,
        nodeName: 'node-service',
        nodeType: NodeType.CONTAINER,
        labels: {
            service: 'api',
            runtime: 'node.js'
        }
    });

    // Start monitoring
    await sensor.startMonitoring({
        interval: 30000, // 30 seconds
        healthCheck: checkHealth
    });
}

main().catch(console.error);
```

## Advanced Sensor Patterns

### 1. Multi-Component Sensor

Monitor multiple related components with a single sensor:

```go
type MultiSensor struct {
    client   *grpc.ClientConn
    nodes    map[string]*Node
    interval time.Duration
}

func (ms *MultiSensor) MonitorAll() {
    for _, node := range ms.nodes {
        go ms.monitorNode(node)
    }
}

func (ms *MultiSensor) monitorNode(node *Node) {
    ticker := time.NewTicker(ms.interval)
    for range ticker.C {
        status := node.CheckHealth()
        ms.updateStatus(node.ID, status)
    }
}
```

### 2. Adaptive Health Checks

Adjust check frequency based on status:

```go
func AdaptiveMonitoring(sensor *Sensor) {
    normalInterval := 60 * time.Second
    degradedInterval := 30 * time.Second
    downInterval := 10 * time.Second

    currentInterval := normalInterval

    for {
        status := sensor.CheckHealth()
        sensor.UpdateStatus(status)

        // Adjust interval based on status
        switch status {
        case StatusUp:
            currentInterval = normalInterval
        case StatusDegraded:
            currentInterval = degradedInterval
        case StatusDown:
            currentInterval = downInterval
        }

        time.Sleep(currentInterval)
    }
}
```

### 3. Circuit Breaker Pattern

Prevent overwhelming failing services:

```go
type CircuitBreaker struct {
    maxFailures  int
    resetTimeout time.Duration
    failures     int
    lastFailTime time.Time
    state        string // "closed", "open", "half-open"
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    if cb.state == "open" {
        if time.Since(cb.lastFailTime) > cb.resetTimeout {
            cb.state = "half-open"
            cb.failures = 0
        } else {
            return fmt.Errorf("circuit breaker is open")
        }
    }

    err := fn()
    if err != nil {
        cb.failures++
        cb.lastFailTime = time.Now()

        if cb.failures >= cb.maxFailures {
            cb.state = "open"
        }
        return err
    }

    if cb.state == "half-open" {
        cb.state = "closed"
    }
    cb.failures = 0
    return nil
}
```

### 4. Metric Aggregation

Collect and report detailed metrics:

```go
type MetricCollector struct {
    metrics map[string][]float64
    mu      sync.Mutex
}

func (mc *MetricCollector) Record(metric string, value float64) {
    mc.mu.Lock()
    defer mc.mu.Unlock()
    mc.metrics[metric] = append(mc.metrics[metric], value)
}

func (mc *MetricCollector) GetStats(metric string) map[string]float64 {
    mc.mu.Lock()
    defer mc.mu.Unlock()

    values := mc.metrics[metric]
    if len(values) == 0 {
        return nil
    }

    // Calculate statistics
    sum := 0.0
    min := values[0]
    max := values[0]

    for _, v := range values {
        sum += v
        if v < min { min = v }
        if v > max { max = v }
    }

    return map[string]float64{
        "min":  min,
        "max":  max,
        "avg":  sum / float64(len(values)),
        "count": float64(len(values)),
    }
}
```

### 5. Event-Driven Sensor

React to system events instead of polling:

```go
func EventDrivenSensor(sensor *Sensor) {
    // Subscribe to system events
    events := make(chan Event)
    subscribeToSystemEvents(events)

    for event := range events {
        switch event.Type {
        case "service_started":
            sensor.UpdateStatus(StatusUp)
        case "service_stopped":
            sensor.UpdateStatus(StatusDown)
        case "error_threshold_exceeded":
            sensor.UpdateStatus(StatusDegraded)
        case "memory_pressure":
            sensor.UpdateMetadata(map[string]interface{}{
                "memory_alert": true,
                "timestamp": time.Now(),
            })
        }
    }
}
```

## Language SDKs

### Go SDK Reference

```go
package sensor

// Config for sensor initialization
type Config struct {
    BackendAddr string
    Token       string
    NodeName    string
    NodeType    NodeType
    Labels      map[string]string
    Metadata    string // JSON
}

// Sensor interface
type Sensor interface {
    // Initialize connection
    Connect() error

    // Create or update node
    RegisterNode() (*Node, error)

    // Update node status
    UpdateStatus(status Status) error

    // Update node metadata
    UpdateMetadata(metadata interface{}) error

    // Start monitoring loop
    StartMonitoring(interval time.Duration, check HealthCheck) error

    // Stop monitoring
    Stop() error

    // Close connection
    Close() error
}

// HealthCheck function type
type HealthCheck func() Status

// Status enumeration
type Status int
const (
    StatusUnknown Status = iota
    StatusUp
    StatusDown
    StatusDegraded
)
```

### Python SDK Reference

```python
from enum import Enum
from typing import Dict, Callable, Optional

class NodeType(Enum):
    BAREMETAL = 1
    VM = 2
    CONTAINER = 3

class Status(Enum):
    UNKNOWN = 0
    UP = 1
    DOWN = 2
    DEGRADED = 3

class Sensor:
    def __init__(
        self,
        backend_addr: str,
        token: str,
        node_name: str,
        node_type: NodeType,
        labels: Optional[Dict[str, str]] = None,
        metadata: Optional[dict] = None
    ):
        """Initialize sensor with configuration"""

    def connect(self) -> None:
        """Establish connection to backend"""

    def register_node(self) -> str:
        """Register node and return ID"""

    def update_status(self, status: Status) -> None:
        """Update node status"""

    def update_metadata(self, metadata: dict) -> None:
        """Update node metadata"""

    def start_monitoring(
        self,
        interval: int,
        health_check: Callable[[], Status]
    ) -> None:
        """Start monitoring loop"""

    def stop(self) -> None:
        """Stop monitoring"""

    def close(self) -> None:
        """Close connection"""
```

### REST API Bridge (Alternative)

For environments where gRPC is not suitable:

```yaml
# REST API Bridge Configuration
apiVersion: v1
kind: Service
metadata:
  name: rest-bridge
spec:
  endpoints:
    - path: /api/v1/nodes
      method: POST
      grpc: CreateNode
    - path: /api/v1/nodes/{id}/status
      method: PUT
      grpc: UpdateStatus
    - path: /api/v1/nodes/{id}
      method: GET
      grpc: GetNode
    - path: /api/v1/events
      method: GET
      grpc: WatchEvents
      websocket: true
```

## Best Practices

### 1. Error Handling

Always implement robust error handling:

```go
func (s *Sensor) UpdateWithRetry(status Status) error {
    maxRetries := 3
    backoff := 1 * time.Second

    for i := 0; i < maxRetries; i++ {
        err := s.UpdateStatus(status)
        if err == nil {
            return nil
        }

        if isRetryable(err) {
            time.Sleep(backoff)
            backoff *= 2
            continue
        }

        return err // Non-retryable error
    }

    return fmt.Errorf("max retries exceeded")
}
```

### 2. Resource Management

Prevent resource leaks:

```go
func (s *Sensor) Run(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            s.logger.Info("Shutting down sensor")
            return
        case <-ticker.C:
            s.performHealthCheck()
        }
    }
}
```

### 3. Logging

Implement structured logging:

```go
logger.Info("Health check completed",
    zap.String("node_id", nodeID),
    zap.String("status", status.String()),
    zap.Duration("check_duration", duration),
    zap.Int("retry_count", retries))
```

### 4. Configuration Management

Use environment variables and config files:

```yaml
# sensor-config.yaml
sensor:
  name: web-server-01
  type: VM
  backend:
    address: backend.example.com:50051
    token_file: /etc/sensor/token
  monitoring:
    interval: 30s
    timeout: 5s
  health_checks:
    - type: http
      url: http://localhost:80
      expected_status: 200
    - type: tcp
      port: 443
    - type: process
      name: nginx
```

### 5. Security

Implement secure communication:

```go
// Use TLS for production
creds, err := credentials.NewClientTLSFromFile("ca.crt", "")
if err != nil {
    log.Fatal(err)
}

conn, err := grpc.Dial(
    backendAddr,
    grpc.WithTransportCredentials(creds),
)
```

## Example Sensors

### 1. Docker Container Sensor

```go
// docker-sensor.go
package main

import (
    "context"
    "github.com/docker/docker/client"
)

func checkDockerContainers() map[string]Status {
    cli, err := client.NewClientWithOpts()
    if err != nil {
        return nil
    }

    containers, err := cli.ContainerList(context.Background(),
        types.ContainerListOptions{All: true})

    statuses := make(map[string]Status)
    for _, container := range containers {
        if container.State == "running" {
            statuses[container.ID[:12]] = StatusUp
        } else {
            statuses[container.ID[:12]] = StatusDown
        }
    }

    return statuses
}
```

### 2. Kubernetes Pod Sensor

```go
// k8s-sensor.go
func checkKubernetesPods(namespace string) {
    config, _ := rest.InClusterConfig()
    clientset, _ := kubernetes.NewForConfig(config)

    pods, _ := clientset.CoreV1().Pods(namespace).List(
        context.TODO(),
        metav1.ListOptions{})

    for _, pod := range pods.Items {
        status := StatusUnknown

        switch pod.Status.Phase {
        case v1.PodRunning:
            status = StatusUp
        case v1.PodPending:
            status = StatusDegraded
        case v1.PodFailed:
            status = StatusDown
        }

        updateNodeStatus(pod.Name, status)
    }
}
```

### 3. Database Health Sensor

```go
// db-sensor.go
func checkDatabaseHealth(dsn string) Status {
    db, err := sql.Open("postgres", dsn)
    if err != nil {
        return StatusDown
    }
    defer db.Close()

    // Check connection
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    if err := db.PingContext(ctx); err != nil {
        return StatusDown
    }

    // Check replication lag
    var lag int
    err = db.QueryRowContext(ctx,
        "SELECT EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp()))").
        Scan(&lag)

    if err != nil {
        return StatusDegraded
    }

    if lag > 10 {
        return StatusDegraded
    }

    return StatusUp
}
```

### 4. HTTP Endpoint Sensor

```go
// http-sensor.go
type HTTPSensor struct {
    endpoints []string
    timeout   time.Duration
}

func (h *HTTPSensor) CheckEndpoints() map[string]Status {
    client := &http.Client{
        Timeout: h.timeout,
    }

    results := make(map[string]Status)

    for _, endpoint := range h.endpoints {
        resp, err := client.Get(endpoint)
        if err != nil {
            results[endpoint] = StatusDown
            continue
        }
        defer resp.Body.Close()

        switch {
        case resp.StatusCode >= 200 && resp.StatusCode < 300:
            results[endpoint] = StatusUp
        case resp.StatusCode >= 500:
            results[endpoint] = StatusDown
        default:
            results[endpoint] = StatusDegraded
        }
    }

    return results
}
```

## Troubleshooting

### Common Issues

#### Connection Failed
```
Error: connection refused
```
**Solution**: Check backend address and network connectivity

#### Authentication Failed
```
Error: permission denied
```
**Solution**: Verify token is correct and included in metadata

#### High CPU Usage
**Solution**: Increase monitoring interval or optimize health checks

#### Memory Leaks
**Solution**: Ensure proper resource cleanup and connection pooling

### Debugging

Enable debug logging:
```go
logger := zap.NewDevelopment()
sensor.SetLogger(logger)
```

Test connection:
```bash
grpcurl -H "authorization: Bearer $TOKEN" \
  localhost:50051 node.v1.NodeService/GetNode
```

### Performance Tuning

1. **Batch Updates**: Group multiple status updates
2. **Connection Pooling**: Reuse gRPC connections
3. **Caching**: Cache health check results
4. **Async Operations**: Use goroutines for parallel checks

## Summary

Sensors are the foundation of the Node Status Platform's monitoring capabilities. By following this SDK documentation, you can build robust, efficient sensors that:

- ✅ Monitor any infrastructure component
- ✅ Report accurate status information
- ✅ Handle errors gracefully
- ✅ Scale to thousands of nodes
- ✅ Integrate with existing monitoring tools

### Next Steps

1. Choose your language SDK
2. Build a prototype sensor
3. Test with the demo backend
4. Deploy to production
5. Monitor and iterate

For more examples and the latest SDK updates, visit the [GitHub repository](https://github.com/melkior/nodestatus).