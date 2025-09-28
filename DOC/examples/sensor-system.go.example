package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	nodev1 "github.com/melkior/nodestatus/gen/go/api/proto"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// SystemMetrics holds system resource information
type SystemMetrics struct {
	Hostname       string    `json:"hostname"`
	OS             string    `json:"os"`
	Platform       string    `json:"platform"`
	CPUCores       int       `json:"cpu_cores"`
	CPUUsagePercent float64  `json:"cpu_usage_percent"`
	MemoryTotalGB  float64   `json:"memory_total_gb"`
	MemoryUsedGB   float64   `json:"memory_used_gb"`
	MemoryPercent  float64   `json:"memory_percent"`
	DiskTotalGB    float64   `json:"disk_total_gb"`
	DiskUsedGB     float64   `json:"disk_used_gb"`
	DiskPercent    float64   `json:"disk_percent"`
	NetworkRxMbps  float64   `json:"network_rx_mbps"`
	NetworkTxMbps  float64   `json:"network_tx_mbps"`
	Uptime         uint64    `json:"uptime_seconds"`
	LoadAvg1       float64   `json:"load_avg_1"`
	LoadAvg5       float64   `json:"load_avg_5"`
	LoadAvg15      float64   `json:"load_avg_15"`
	Timestamp      time.Time `json:"timestamp"`
}

// SystemSensor monitors system resources
type SystemSensor struct {
	client         nodev1.NodeServiceClient
	conn           *grpc.ClientConn
	token          string
	nodeID         string
	checkInterval  time.Duration
	lastNetStats   *net.IOCountersStat
	lastCheckTime  time.Time
}

// Thresholds for status determination
type Thresholds struct {
	CPUCritical    float64
	CPUWarning     float64
	MemoryCritical float64
	MemoryWarning  float64
	DiskCritical   float64
	DiskWarning    float64
}

var defaultThresholds = Thresholds{
	CPUCritical:    90.0,
	CPUWarning:     70.0,
	MemoryCritical: 90.0,
	MemoryWarning:  80.0,
	DiskCritical:   90.0,
	DiskWarning:    80.0,
}

func NewSystemSensor(backendAddr, token string, interval time.Duration) (*SystemSensor, error) {
	conn, err := grpc.NewClient(backendAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return &SystemSensor{
		client:        nodev1.NewNodeServiceClient(conn),
		conn:          conn,
		token:         token,
		checkInterval: interval,
	}, nil
}

func (s *SystemSensor) Close() error {
	return s.conn.Close()
}

func (s *SystemSensor) authContext(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx,
		"authorization", fmt.Sprintf("Bearer %s", s.token))
}

func (s *SystemSensor) RegisterNode() error {
	ctx := s.authContext(context.Background())

	hostname, _ := os.Hostname()
	hostInfo, _ := host.Info()

	// Determine node type based on system
	nodeType := nodev1.NodeType_BAREMETAL
	if isContainer() {
		nodeType = nodev1.NodeType_CONTAINER
	} else if isVM() {
		nodeType = nodev1.NodeType_VM
	}

	node := &nodev1.Node{
		Name: hostname,
		Type: nodeType,
		Labels: map[string]string{
			"sensor":   "system",
			"os":       runtime.GOOS,
			"arch":     runtime.GOARCH,
			"platform": hostInfo.Platform,
			"monitor":  "true",
		},
		Status: nodev1.NodeStatus_UNKNOWN,
	}

	resp, err := s.client.CreateNode(ctx, &nodev1.CreateNodeRequest{Node: node})
	if err != nil {
		// Try to find existing node
		listResp, listErr := s.client.ListNodes(context.Background(), &nodev1.ListNodesRequest{
			PageSize: 1000,
		})
		if listErr != nil {
			return fmt.Errorf("failed to create or find node: %w", err)
		}

		for _, existingNode := range listResp.Nodes {
			if existingNode.Name == hostname {
				s.nodeID = existingNode.Id
				log.Printf("Found existing node %s with ID %s", hostname, existingNode.Id)
				return nil
			}
		}

		return fmt.Errorf("failed to create node: %w", err)
	}

	s.nodeID = resp.Node.Id
	log.Printf("Registered system as node %s", s.nodeID)
	return nil
}

func (s *SystemSensor) CollectMetrics() (*SystemMetrics, error) {
	metrics := &SystemMetrics{
		Timestamp: time.Now(),
	}

	// Host information
	hostname, _ := os.Hostname()
	metrics.Hostname = hostname

	hostInfo, err := host.Info()
	if err == nil {
		metrics.OS = hostInfo.OS
		metrics.Platform = hostInfo.Platform
		metrics.Uptime = hostInfo.Uptime
	}

	// CPU metrics
	metrics.CPUCores = runtime.NumCPU()
	cpuPercent, err := cpu.Percent(1*time.Second, false)
	if err == nil && len(cpuPercent) > 0 {
		metrics.CPUUsagePercent = cpuPercent[0]
	}

	// Load average would go here on Unix systems
	// For simplicity, we're skipping it in this example

	// Memory metrics
	vmStat, err := mem.VirtualMemory()
	if err == nil {
		metrics.MemoryTotalGB = float64(vmStat.Total) / (1024 * 1024 * 1024)
		metrics.MemoryUsedGB = float64(vmStat.Used) / (1024 * 1024 * 1024)
		metrics.MemoryPercent = vmStat.UsedPercent
	}

	// Disk metrics
	diskStat, err := disk.Usage("/")
	if err == nil {
		metrics.DiskTotalGB = float64(diskStat.Total) / (1024 * 1024 * 1024)
		metrics.DiskUsedGB = float64(diskStat.Used) / (1024 * 1024 * 1024)
		metrics.DiskPercent = diskStat.UsedPercent
	}

	// Network metrics
	netStats, err := net.IOCounters(false)
	if err == nil && len(netStats) > 0 {
		currentStats := &netStats[0]

		if s.lastNetStats != nil && !s.lastCheckTime.IsZero() {
			timeDiff := time.Since(s.lastCheckTime).Seconds()

			// Calculate network speed in Mbps
			rxBytes := float64(currentStats.BytesRecv - s.lastNetStats.BytesRecv)
			txBytes := float64(currentStats.BytesSent - s.lastNetStats.BytesSent)

			metrics.NetworkRxMbps = (rxBytes * 8) / (timeDiff * 1000000)
			metrics.NetworkTxMbps = (txBytes * 8) / (timeDiff * 1000000)
		}

		s.lastNetStats = currentStats
		s.lastCheckTime = time.Now()
	}

	return metrics, nil
}

func (s *SystemSensor) DetermineStatus(metrics *SystemMetrics) nodev1.NodeStatus {
	// Check critical thresholds
	if metrics.CPUUsagePercent >= defaultThresholds.CPUCritical ||
		metrics.MemoryPercent >= defaultThresholds.MemoryCritical ||
		metrics.DiskPercent >= defaultThresholds.DiskCritical {
		return nodev1.NodeStatus_DOWN
	}

	// Check warning thresholds
	if metrics.CPUUsagePercent >= defaultThresholds.CPUWarning ||
		metrics.MemoryPercent >= defaultThresholds.MemoryWarning ||
		metrics.DiskPercent >= defaultThresholds.DiskWarning {
		return nodev1.NodeStatus_DEGRADED
	}

	return nodev1.NodeStatus_UP
}

func (s *SystemSensor) UpdateNodeStatus(metrics *SystemMetrics) error {
	status := s.DetermineStatus(metrics)

	// Convert metrics to JSON for metadata
	metadataJSON, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	// Update both status and metadata
	ctx := s.authContext(context.Background())

	// First get the current node
	getResp, err := s.client.GetNode(context.Background(), &nodev1.GetNodeRequest{Id: s.nodeID})
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	// Update the node with new metadata
	node := getResp.Node
	node.MetadataJson = string(metadataJSON)
	node.Status = status

	_, err = s.client.UpdateNode(ctx, &nodev1.UpdateNodeRequest{Node: node})
	if err != nil {
		return fmt.Errorf("failed to update node: %w", err)
	}

	log.Printf("Updated system status to %s (CPU: %.1f%%, Mem: %.1f%%, Disk: %.1f%%)",
		status.String(),
		metrics.CPUUsagePercent,
		metrics.MemoryPercent,
		metrics.DiskPercent)

	return nil
}

func (s *SystemSensor) Start() error {
	// Register the node
	if err := s.RegisterNode(); err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}

	// Start monitoring loop
	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	log.Printf("System sensor started, reporting every %v", s.checkInterval)

	// Initial check
	s.performCheck()

	for range ticker.C {
		s.performCheck()
	}

	return nil
}

func (s *SystemSensor) performCheck() {
	metrics, err := s.CollectMetrics()
	if err != nil {
		log.Printf("Error collecting metrics: %v", err)
		return
	}

	if err := s.UpdateNodeStatus(metrics); err != nil {
		log.Printf("Error updating status: %v", err)
	}
}

// Helper functions to detect virtualization
func isContainer() bool {
	// Check for Docker
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// Check for Kubernetes
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}

	// Check cgroup for container signatures
	data, err := os.ReadFile("/proc/1/cgroup")
	if err == nil {
		content := string(data)
		if contains(content, "docker") || contains(content, "kubepods") {
			return true
		}
	}

	return false
}

func isVM() bool {
	// Check DMI for virtualization hints
	vendors := []string{
		"/sys/class/dmi/id/sys_vendor",
		"/sys/class/dmi/id/board_vendor",
		"/sys/class/dmi/id/chassis_vendor",
	}

	vmSignatures := []string{
		"VMware", "VirtualBox", "KVM", "QEMU",
		"Microsoft Corporation", "Xen", "Google",
	}

	for _, vendorFile := range vendors {
		data, err := os.ReadFile(vendorFile)
		if err == nil {
			content := string(data)
			for _, sig := range vmSignatures {
				if contains(content, sig) {
					return true
				}
			}
		}
	}

	return false
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr))
}

func main() {
	// Configuration from environment
	backendAddr := os.Getenv("BACKEND_ADDR")
	if backendAddr == "" {
		backendAddr = "localhost:50051"
	}

	token := os.Getenv("BACKEND_TOKEN")
	if token == "" {
		log.Fatal("BACKEND_TOKEN environment variable is required")
	}

	intervalStr := os.Getenv("CHECK_INTERVAL")
	if intervalStr == "" {
		intervalStr = "30s"
	}

	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		log.Fatalf("Invalid CHECK_INTERVAL: %v", err)
	}

	// Create and start sensor
	sensor, err := NewSystemSensor(backendAddr, token, interval)
	if err != nil {
		log.Fatal(err)
	}
	defer sensor.Close()

	if err := sensor.Start(); err != nil {
		log.Fatal(err)
	}
}

// Usage:
// BACKEND_ADDR=localhost:50051 BACKEND_TOKEN=my-token CHECK_INTERVAL=30s go run sensor-system.go

// To use this sensor, you'll need to install the gopsutil library:
// go get github.com/shirou/gopsutil/v3