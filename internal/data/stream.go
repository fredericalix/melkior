package data

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"time"

	nodev1 "github.com/melkior/nodestatus/gen/go/api/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StreamConsumer consumes events from gRPC stream
type StreamConsumer struct {
	client       nodev1.NodeServiceClient
	aggregator   *Aggregator
	eventChan    chan *Event
	errorChan    chan error
	ctx          context.Context
	cancel       context.CancelFunc
	maxRetries   int
	baseDelay    time.Duration
	maxDelay     time.Duration
}

// NewStreamConsumer creates a new stream consumer
func NewStreamConsumer(client nodev1.NodeServiceClient, aggregator *Aggregator) *StreamConsumer {
	ctx, cancel := context.WithCancel(context.Background())
	return &StreamConsumer{
		client:     client,
		aggregator: aggregator,
		eventChan:  make(chan *Event, 100),
		errorChan:  make(chan error, 10),
		ctx:        ctx,
		cancel:     cancel,
		maxRetries: 10,
		baseDelay:  100 * time.Millisecond,
		maxDelay:   30 * time.Second,
	}
}

// Start begins consuming events
func (sc *StreamConsumer) Start(ctx context.Context) error {
	// First, load initial state
	if err := sc.loadInitialState(ctx); err != nil {
		return fmt.Errorf("failed to load initial state: %w", err)
	}

	// Start the stream consumer
	go sc.consumeLoop(ctx)

	// Start the event processor
	go sc.processEvents()

	return nil
}

// Stop stops the stream consumer
func (sc *StreamConsumer) Stop() {
	sc.cancel()
	close(sc.eventChan)
	close(sc.errorChan)
}

// Events returns the event channel
func (sc *StreamConsumer) Events() <-chan *Event {
	return sc.eventChan
}

// Errors returns the error channel
func (sc *StreamConsumer) Errors() <-chan error {
	return sc.errorChan
}

// loadInitialState loads all current nodes
func (sc *StreamConsumer) loadInitialState(ctx context.Context) error {
	resp, err := sc.client.ListNodes(ctx, &nodev1.ListNodesRequest{
		PageSize: 1000,
	})
	if err != nil {
		return err
	}

	nodes := make([]*Node, 0, len(resp.Nodes))
	for _, n := range resp.Nodes {
		nodes = append(nodes, convertNode(n))
	}

	sc.aggregator.SetNodes(nodes)
	return nil
}

// consumeLoop continuously consumes events with reconnection
func (sc *StreamConsumer) consumeLoop(ctx context.Context) {
	retries := 0

	for {
		select {
		case <-sc.ctx.Done():
			return
		default:
		}

		// Try to establish stream
		stream, err := sc.client.WatchEvents(ctx, &nodev1.WatchEventsRequest{})
		if err != nil {
			sc.handleStreamError(err, &retries)
			continue
		}

		// Reset retries on successful connection
		retries = 0

		// Consume from stream
		for {
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					// Stream ended normally
					break
				}

				// Handle error and decide whether to retry
				if !sc.handleStreamError(err, &retries) {
					return
				}
				break
			}

			// Convert and send event
			event := &Event{
				Type:          resp.EventType,
				Node:          convertNode(resp.Node),
				ChangedFields: resp.ChangedFields,
				Timestamp:     time.Now(),
			}

			select {
			case sc.eventChan <- event:
			case <-sc.ctx.Done():
				return
			}
		}
	}
}

// processEvents processes events from the channel
func (sc *StreamConsumer) processEvents() {
	for {
		select {
		case event, ok := <-sc.eventChan:
			if !ok {
				return
			}
			if event != nil {
				sc.aggregator.HandleEvent(event)
			}
		case <-sc.ctx.Done():
			return
		}
	}
}

// handleStreamError handles stream errors with exponential backoff
func (sc *StreamConsumer) handleStreamError(err error, retries *int) bool {
	st, ok := status.FromError(err)
	if !ok {
		// Not a gRPC error
		select {
		case sc.errorChan <- err:
		default:
		}
		return false
	}

	// Check if error is retryable
	switch st.Code() {
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
		// Retryable errors
		if *retries >= sc.maxRetries {
			select {
			case sc.errorChan <- fmt.Errorf("max retries exceeded: %w", err):
			default:
			}
			return false
		}

		// Calculate backoff with jitter
		delay := sc.calculateBackoff(*retries)
		*retries++

		select {
		case <-time.After(delay):
		case <-sc.ctx.Done():
			return false
		}
		return true

	default:
		// Non-retryable error
		select {
		case sc.errorChan <- err:
		default:
		}
		return false
	}
}

// calculateBackoff calculates exponential backoff with jitter
func (sc *StreamConsumer) calculateBackoff(retry int) time.Duration {
	delay := sc.baseDelay * (1 << uint(retry))
	if delay > sc.maxDelay {
		delay = sc.maxDelay
	}

	// Add jitter (±25%)
	jitter := time.Duration(rand.Float64() * float64(delay) * 0.5)
	if rand.Float32() < 0.5 {
		delay -= jitter
	} else {
		delay += jitter
	}

	return delay
}

// convertNode converts protobuf node to internal representation
func convertNode(n *nodev1.Node) *Node {
	if n == nil {
		return nil
	}

	var lastSeen time.Time
	if n.LastSeen != nil {
		lastSeen = n.LastSeen.AsTime()
	}

	return &Node{
		ID:       n.Id,
		Name:     n.Name,
		Type:     n.Type,
		Status:   n.Status,
		Labels:   n.Labels,
		Metadata: n.MetadataJson,
		LastSeen: lastSeen,
	}
}

// MockStreamConsumer for testing without gRPC connection
type MockStreamConsumer struct {
	*StreamConsumer
	ticker *time.Ticker
}

// NewMockStreamConsumer creates a mock stream consumer for testing
func NewMockStreamConsumer(aggregator *Aggregator) *MockStreamConsumer {
	ctx, cancel := context.WithCancel(context.Background())
	sc := &StreamConsumer{
		aggregator: aggregator,
		eventChan:  make(chan *Event, 100),
		errorChan:  make(chan error, 10),
		ctx:        ctx,
		cancel:     cancel,
	}

	return &MockStreamConsumer{
		StreamConsumer: sc,
		ticker:         time.NewTicker(500 * time.Millisecond),
	}
}

// Start begins generating mock events
func (msc *MockStreamConsumer) Start(ctx context.Context) error {
	// Generate initial nodes
	nodes := make([]*Node, 0, 10)
	for i := 0; i < 10; i++ {
		nodes = append(nodes, &Node{
			ID:       fmt.Sprintf("node-%d", i),
			Name:     fmt.Sprintf("test-node-%d", i),
			Type:     nodev1.NodeType(rand.Intn(3) + 1),
			Status:   nodev1.NodeStatus(rand.Intn(4) + 1),
			Labels:   map[string]string{"env": "test", "version": "1.0"},
			Metadata: `{"cpu": 4, "ram": 16}`,
			LastSeen: time.Now(),
		})
	}
	msc.aggregator.SetNodes(nodes)

	// Start generating events
	go msc.generateEvents()

	return nil
}

// generateEvents generates mock events
func (msc *MockStreamConsumer) generateEvents() {
	eventTypes := []nodev1.EventType{
		nodev1.EventType_CREATED,
		nodev1.EventType_UPDATED,
		nodev1.EventType_UPDATED,
		nodev1.EventType_UPDATED,
		nodev1.EventType_DELETED,
	}

	nodeID := 10
	for {
		select {
		case <-msc.ticker.C:
			// Generate random event
			eventType := eventTypes[rand.Intn(len(eventTypes))]

			var event *Event
			switch eventType {
			case nodev1.EventType_CREATED:
				event = &Event{
					Type: eventType,
					Node: &Node{
						ID:       fmt.Sprintf("node-%d", nodeID),
						Name:     fmt.Sprintf("test-node-%d", nodeID),
						Type:     nodev1.NodeType(rand.Intn(3) + 1),
						Status:   nodev1.NodeStatus(rand.Intn(4) + 1),
						Labels:   map[string]string{"env": "test"},
						Metadata: `{"cpu": 4, "ram": 16}`,
						LastSeen: time.Now(),
					},
					Timestamp: time.Now(),
				}
				nodeID++

			case nodev1.EventType_UPDATED:
				nodes := msc.aggregator.GetNodes()
				if len(nodes) > 0 {
					node := nodes[rand.Intn(len(nodes))]
					node.Status = nodev1.NodeStatus(rand.Intn(4) + 1)
					node.LastSeen = time.Now()
					event = &Event{
						Type:          eventType,
						Node:          node,
						ChangedFields: []string{"status", "last_seen"},
						Timestamp:     time.Now(),
					}
				}

			case nodev1.EventType_DELETED:
				nodes := msc.aggregator.GetNodes()
				if len(nodes) > 1 {
					node := nodes[rand.Intn(len(nodes))]
					event = &Event{
						Type:      eventType,
						Node:      node,
						Timestamp: time.Now(),
					}
				}
			}

			if event != nil {
				select {
				case msc.eventChan <- event:
					msc.aggregator.HandleEvent(event)
				case <-msc.ctx.Done():
					return
				}
			}

		case <-msc.ctx.Done():
			return
		}
	}
}

// Stop stops the mock stream consumer
func (msc *MockStreamConsumer) Stop() {
	msc.ticker.Stop()
	msc.StreamConsumer.Stop()
}