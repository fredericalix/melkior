package data

import (
	"sync"
	"time"

	nodev1 "github.com/melkior/nodestatus/gen/go/api/proto"
)

// MetricsSnapshot represents a point-in-time view of metrics
type MetricsSnapshot struct {
	Timestamp time.Time

	// Status distribution
	StatusCounts map[nodev1.NodeStatus]int
	StatusRatios map[nodev1.NodeStatus]float64

	// Type distribution
	TypeCounts map[nodev1.NodeType]int
	TypeRatios map[nodev1.NodeType]float64

	// Time series (per-second buckets)
	StatusTimeSeries map[nodev1.NodeStatus][]int // Last N seconds
	TimeSeriesLabels []string                     // Time labels

	// Rates
	EventsPerSecond float64
	MutationRate    float64 // Creates + Updates + Deletes per second

	// Totals
	TotalNodes       int
	TotalEvents      int64
	ConnectedWatchers int
}

// Node represents a node in the system
type Node struct {
	ID       string
	Name     string
	Type     nodev1.NodeType
	Status   nodev1.NodeStatus
	Labels   map[string]string
	Metadata string
	LastSeen time.Time
}

// Event represents a change event
type Event struct {
	Type          nodev1.EventType
	Node          *Node
	ChangedFields []string
	Timestamp     time.Time
}

// RingBuffer is a circular buffer for storing time-series data
type RingBuffer struct {
	mu       sync.RWMutex
	data     []int
	capacity int
	head     int
	size     int
}

// NewRingBuffer creates a new ring buffer
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		data:     make([]int, capacity),
		capacity: capacity,
	}
}

// Push adds a value to the ring buffer
func (rb *RingBuffer) Push(value int) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.data[rb.head] = value
	rb.head = (rb.head + 1) % rb.capacity
	if rb.size < rb.capacity {
		rb.size++
	}
}

// GetAll returns all values in chronological order
func (rb *RingBuffer) GetAll() []int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.size == 0 {
		return []int{}
	}

	result := make([]int, rb.size)
	if rb.size < rb.capacity {
		// Buffer not full yet
		copy(result, rb.data[:rb.size])
	} else {
		// Buffer is full, need to reorder
		tail := rb.capacity - rb.head
		copy(result[:tail], rb.data[rb.head:])
		copy(result[tail:], rb.data[:rb.head])
	}
	return result
}

// Sum returns the sum of all values
func (rb *RingBuffer) Sum() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	sum := 0
	for i := 0; i < rb.size; i++ {
		sum += rb.data[i]
	}
	return sum
}

// Average returns the average of all values
func (rb *RingBuffer) Average() float64 {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.size == 0 {
		return 0
	}
	return float64(rb.Sum()) / float64(rb.size)
}

// SnapshotProvider provides metrics snapshots
type SnapshotProvider interface {
	Snapshot() MetricsSnapshot
	Subscribe() <-chan MetricsSnapshot
	Unsubscribe(<-chan MetricsSnapshot)
}