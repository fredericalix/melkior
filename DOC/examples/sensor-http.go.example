package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	nodev1 "github.com/melkior/nodestatus/gen/go/api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// HTTPEndpoint represents an HTTP endpoint to monitor
type HTTPEndpoint struct {
	Name            string
	URL             string
	ExpectedStatus  int
	TimeoutSeconds  int
	CheckInterval   time.Duration
}

// HTTPSensor monitors multiple HTTP endpoints
type HTTPSensor struct {
	client    nodev1.NodeServiceClient
	conn      *grpc.ClientConn
	token     string
	endpoints []HTTPEndpoint
	nodes     map[string]string // endpoint name -> node ID
}

func NewHTTPSensor(backendAddr, token string) (*HTTPSensor, error) {
	conn, err := grpc.NewClient(backendAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return &HTTPSensor{
		client: nodev1.NewNodeServiceClient(conn),
		conn:   conn,
		token:  token,
		nodes:  make(map[string]string),
	}, nil
}

func (s *HTTPSensor) Close() error {
	return s.conn.Close()
}

func (s *HTTPSensor) authContext(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx,
		"authorization", fmt.Sprintf("Bearer %s", s.token))
}

func (s *HTTPSensor) RegisterEndpoint(endpoint HTTPEndpoint) error {
	ctx := s.authContext(context.Background())

	// Try to create the node
	node := &nodev1.Node{
		Name: endpoint.Name,
		Type: nodev1.NodeType_CONTAINER,
		Labels: map[string]string{
			"sensor":  "http",
			"url":     endpoint.URL,
			"monitor": "true",
		},
		MetadataJson: s.getMetadataJSON(endpoint),
		Status:       nodev1.NodeStatus_UNKNOWN,
	}

	resp, err := s.client.CreateNode(ctx, &nodev1.CreateNodeRequest{Node: node})
	if err != nil {
		// Node might already exist, try to list and find it
		listResp, listErr := s.client.ListNodes(context.Background(), &nodev1.ListNodesRequest{
			PageSize: 1000,
		})
		if listErr != nil {
			return fmt.Errorf("failed to create or find node: %w", err)
		}

		// Find the node by name
		for _, existingNode := range listResp.Nodes {
			if existingNode.Name == endpoint.Name {
				s.nodes[endpoint.Name] = existingNode.Id
				log.Printf("Found existing node %s with ID %s", endpoint.Name, existingNode.Id)
				return nil
			}
		}

		return fmt.Errorf("failed to create node: %w", err)
	}

	s.nodes[endpoint.Name] = resp.Node.Id
	log.Printf("Registered endpoint %s as node %s", endpoint.Name, resp.Node.Id)
	return nil
}

func (s *HTTPSensor) getMetadataJSON(endpoint HTTPEndpoint) string {
	metadata := map[string]interface{}{
		"url":             endpoint.URL,
		"expected_status": endpoint.ExpectedStatus,
		"timeout_seconds": endpoint.TimeoutSeconds,
		"check_interval":  endpoint.CheckInterval.String(),
	}
	data, _ := json.Marshal(metadata)
	return string(data)
}

func (s *HTTPSensor) CheckEndpoint(endpoint HTTPEndpoint) nodev1.NodeStatus {
	client := &http.Client{
		Timeout: time.Duration(endpoint.TimeoutSeconds) * time.Second,
	}

	start := time.Now()
	resp, err := client.Get(endpoint.URL)
	latency := time.Since(start)

	if err != nil {
		log.Printf("Endpoint %s is DOWN: %v", endpoint.Name, err)
		return nodev1.NodeStatus_DOWN
	}
	defer resp.Body.Close()

	log.Printf("Endpoint %s responded with status %d in %v",
		endpoint.Name, resp.StatusCode, latency)

	if endpoint.ExpectedStatus > 0 && resp.StatusCode != endpoint.ExpectedStatus {
		log.Printf("Endpoint %s is DEGRADED: expected status %d, got %d",
			endpoint.Name, endpoint.ExpectedStatus, resp.StatusCode)
		return nodev1.NodeStatus_DEGRADED
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nodev1.NodeStatus_UP
	case resp.StatusCode >= 500:
		return nodev1.NodeStatus_DOWN
	default:
		return nodev1.NodeStatus_DEGRADED
	}
}

func (s *HTTPSensor) UpdateEndpointStatus(endpoint HTTPEndpoint, status nodev1.NodeStatus) error {
	nodeID, ok := s.nodes[endpoint.Name]
	if !ok {
		return fmt.Errorf("node ID not found for endpoint %s", endpoint.Name)
	}

	ctx := s.authContext(context.Background())
	_, err := s.client.UpdateStatus(ctx, &nodev1.UpdateStatusRequest{
		Id:     nodeID,
		Status: status,
	})

	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	log.Printf("Updated %s status to %s", endpoint.Name, status.String())
	return nil
}

func (s *HTTPSensor) MonitorEndpoint(endpoint HTTPEndpoint) {
	ticker := time.NewTicker(endpoint.CheckInterval)
	defer ticker.Stop()

	log.Printf("Starting monitoring for %s every %v", endpoint.Name, endpoint.CheckInterval)

	for {
		status := s.CheckEndpoint(endpoint)
		if err := s.UpdateEndpointStatus(endpoint, status); err != nil {
			log.Printf("Error updating status for %s: %v", endpoint.Name, err)
		}

		<-ticker.C
	}
}

func (s *HTTPSensor) Start(endpoints []HTTPEndpoint) error {
	s.endpoints = endpoints

	// Register all endpoints
	for _, endpoint := range endpoints {
		if err := s.RegisterEndpoint(endpoint); err != nil {
			return fmt.Errorf("failed to register endpoint %s: %w", endpoint.Name, err)
		}
	}

	// Start monitoring each endpoint in a goroutine
	for _, endpoint := range endpoints {
		go s.MonitorEndpoint(endpoint)
	}

	return nil
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

	// Create sensor
	sensor, err := NewHTTPSensor(backendAddr, token)
	if err != nil {
		log.Fatal(err)
	}
	defer sensor.Close()

	// Define endpoints to monitor
	endpoints := []HTTPEndpoint{
		{
			Name:           "google-homepage",
			URL:            "https://www.google.com",
			ExpectedStatus: 200,
			TimeoutSeconds: 5,
			CheckInterval:  30 * time.Second,
		},
		{
			Name:           "github-api",
			URL:            "https://api.github.com",
			ExpectedStatus: 200,
			TimeoutSeconds: 5,
			CheckInterval:  30 * time.Second,
		},
		{
			Name:           "local-service",
			URL:            "http://localhost:8080/healthz",
			ExpectedStatus: 200,
			TimeoutSeconds: 2,
			CheckInterval:  10 * time.Second,
		},
	}

	// Start monitoring
	if err := sensor.Start(endpoints); err != nil {
		log.Fatal(err)
	}

	log.Printf("HTTP Sensor started, monitoring %d endpoints", len(endpoints))

	// Keep running
	select {}
}

// Usage:
// BACKEND_ADDR=localhost:50051 BACKEND_TOKEN=my-token go run sensor-http.go