package data

import (
	"context"
	"sync"
	"time"

	nodev1 "github.com/melkior/nodestatus/gen/go/api/proto"
)

// Aggregator maintains rolling metrics and time-series data
type Aggregator struct {
	mu             sync.RWMutex
	windowSecs     int
	nodes          map[string]*Node
	statusCounts   map[nodev1.NodeStatus]int
	typeCounts     map[nodev1.NodeType]int

	// Time series ring buffers (one per status)
	statusTimeSeries map[nodev1.NodeStatus]*RingBuffer
	eventBuffer      *RingBuffer
	mutationBuffer   *RingBuffer

	// Event counters
	totalEvents    int64
	eventsLastSec  int
	mutationsLastSec int

	// Subscribers for push updates
	subscribers    map[chan MetricsSnapshot]struct{}
	subscribersMu  sync.RWMutex

	// Ticker for sampling
	ticker         *time.Ticker
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewAggregator creates a new metrics aggregator
func NewAggregator(windowSecs int) *Aggregator {
	ctx, cancel := context.WithCancel(context.Background())

	agg := &Aggregator{
		windowSecs:       windowSecs,
		nodes:            make(map[string]*Node),
		statusCounts:     make(map[nodev1.NodeStatus]int),
		typeCounts:       make(map[nodev1.NodeType]int),
		statusTimeSeries: make(map[nodev1.NodeStatus]*RingBuffer),
		eventBuffer:      NewRingBuffer(windowSecs),
		mutationBuffer:   NewRingBuffer(windowSecs),
		subscribers:      make(map[chan MetricsSnapshot]struct{}),
		ticker:           time.NewTicker(1 * time.Second),
		ctx:              ctx,
		cancel:           cancel,
	}

	// Initialize time series buffers for each status
	for _, status := range []nodev1.NodeStatus{
		nodev1.NodeStatus_UNKNOWN,
		nodev1.NodeStatus_UP,
		nodev1.NodeStatus_DOWN,
		nodev1.NodeStatus_DEGRADED,
	} {
		agg.statusTimeSeries[status] = NewRingBuffer(windowSecs)
	}

	// Start the sampling loop
	go agg.sampleLoop()

	return agg
}

// Close stops the aggregator
func (agg *Aggregator) Close() {
	agg.cancel()
	agg.ticker.Stop()

	agg.subscribersMu.Lock()
	for ch := range agg.subscribers {
		close(ch)
	}
	agg.subscribers = make(map[chan MetricsSnapshot]struct{})
	agg.subscribersMu.Unlock()
}

// HandleEvent processes an incoming event
func (agg *Aggregator) HandleEvent(event *Event) {
	agg.mu.Lock()
	defer agg.mu.Unlock()

	agg.totalEvents++
	agg.eventsLastSec++

	switch event.Type {
	case nodev1.EventType_CREATED:
		agg.nodes[event.Node.ID] = event.Node
		agg.statusCounts[event.Node.Status]++
		agg.typeCounts[event.Node.Type]++
		agg.mutationsLastSec++

	case nodev1.EventType_UPDATED:
		if existing, ok := agg.nodes[event.Node.ID]; ok {
			// Update status counts if status changed
			if existing.Status != event.Node.Status {
				agg.statusCounts[existing.Status]--
				agg.statusCounts[event.Node.Status]++
			}
			// Update type counts if type changed (unlikely but possible)
			if existing.Type != event.Node.Type {
				agg.typeCounts[existing.Type]--
				agg.typeCounts[event.Node.Type]++
			}
		}
		agg.nodes[event.Node.ID] = event.Node
		agg.mutationsLastSec++

	case nodev1.EventType_DELETED:
		if existing, ok := agg.nodes[event.Node.ID]; ok {
			agg.statusCounts[existing.Status]--
			agg.typeCounts[existing.Type]--
			delete(agg.nodes, event.Node.ID)
		}
		agg.mutationsLastSec++
	}
}

// SetNodes initializes the node set (for initial load)
func (agg *Aggregator) SetNodes(nodes []*Node) {
	agg.mu.Lock()
	defer agg.mu.Unlock()

	// Clear existing counts
	agg.nodes = make(map[string]*Node)
	agg.statusCounts = make(map[nodev1.NodeStatus]int)
	agg.typeCounts = make(map[nodev1.NodeType]int)

	// Populate from nodes
	for _, node := range nodes {
		agg.nodes[node.ID] = node
		agg.statusCounts[node.Status]++
		agg.typeCounts[node.Type]++
	}
}

// GetNodes returns a copy of current nodes
func (agg *Aggregator) GetNodes() []*Node {
	agg.mu.RLock()
	defer agg.mu.RUnlock()

	nodes := make([]*Node, 0, len(agg.nodes))
	for _, node := range agg.nodes {
		nodeCopy := *node
		nodes = append(nodes, &nodeCopy)
	}
	return nodes
}

// Snapshot returns current metrics snapshot
func (agg *Aggregator) Snapshot() MetricsSnapshot {
	agg.mu.RLock()
	defer agg.mu.RUnlock()

	snap := MetricsSnapshot{
		Timestamp:    time.Now(),
		StatusCounts: make(map[nodev1.NodeStatus]int),
		StatusRatios: make(map[nodev1.NodeStatus]float64),
		TypeCounts:   make(map[nodev1.NodeType]int),
		TypeRatios:   make(map[nodev1.NodeType]float64),
		StatusTimeSeries: make(map[nodev1.NodeStatus][]int),
		TotalNodes:   len(agg.nodes),
		TotalEvents:  agg.totalEvents,
	}

	// Copy status counts and calculate ratios
	totalNodes := float64(len(agg.nodes))
	if totalNodes == 0 {
		totalNodes = 1 // Avoid division by zero
	}

	for status, count := range agg.statusCounts {
		snap.StatusCounts[status] = count
		snap.StatusRatios[status] = float64(count) / totalNodes
	}

	// Copy type counts and calculate ratios
	for nodeType, count := range agg.typeCounts {
		snap.TypeCounts[nodeType] = count
		snap.TypeRatios[nodeType] = float64(count) / totalNodes
	}

	// Copy time series data
	for status, buffer := range agg.statusTimeSeries {
		snap.StatusTimeSeries[status] = buffer.GetAll()
	}

	// Calculate rates
	eventHistory := agg.eventBuffer.GetAll()
	if len(eventHistory) > 0 {
		snap.EventsPerSecond = agg.eventBuffer.Average()
	}

	mutationHistory := agg.mutationBuffer.GetAll()
	if len(mutationHistory) > 0 {
		snap.MutationRate = agg.mutationBuffer.Average()
	}

	// Generate time labels (last N seconds)
	now := time.Now()
	labels := make([]string, 0, agg.windowSecs)
	for i := agg.windowSecs - 1; i >= 0; i-- {
		t := now.Add(-time.Duration(i) * time.Second)
		labels = append(labels, t.Format("15:04:05"))
	}
	snap.TimeSeriesLabels = labels

	return snap
}

// Subscribe creates a channel for receiving push updates
func (agg *Aggregator) Subscribe() <-chan MetricsSnapshot {
	ch := make(chan MetricsSnapshot, 1)

	agg.subscribersMu.Lock()
	agg.subscribers[ch] = struct{}{}
	agg.subscribersMu.Unlock()

	// Send initial snapshot
	ch <- agg.Snapshot()

	return ch
}

// Unsubscribe removes a subscriber
func (agg *Aggregator) Unsubscribe(ch <-chan MetricsSnapshot) {
	agg.subscribersMu.Lock()
	// We need to match the actual channel
	for subCh := range agg.subscribers {
		if subCh == ch {
			delete(agg.subscribers, subCh)
			close(subCh)
			break
		}
	}
	agg.subscribersMu.Unlock()
}

// sampleLoop runs every second to update time series
func (agg *Aggregator) sampleLoop() {
	for {
		select {
		case <-agg.ticker.C:
			agg.sample()
		case <-agg.ctx.Done():
			return
		}
	}
}

// sample captures current metrics into time series
func (agg *Aggregator) sample() {
	agg.mu.Lock()

	// Push current status counts to time series
	for status, buffer := range agg.statusTimeSeries {
		buffer.Push(agg.statusCounts[status])
	}

	// Push event and mutation rates
	agg.eventBuffer.Push(agg.eventsLastSec)
	agg.mutationBuffer.Push(agg.mutationsLastSec)

	// Reset per-second counters
	agg.eventsLastSec = 0
	agg.mutationsLastSec = 0

	// Create snapshot for subscribers
	snap := agg.Snapshot()

	agg.mu.Unlock()

	// Notify subscribers (non-blocking)
	agg.subscribersMu.RLock()
	for ch := range agg.subscribers {
		select {
		case ch <- snap:
		default:
			// Skip if channel is full
		}
	}
	agg.subscribersMu.RUnlock()
}