package sim

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"sync"

	nodev1 "github.com/melkior/nodestatus/gen/go/api/proto"
)

type Namer struct {
	rng       *rand.Rand
	pool      []string
	counters  map[nodev1.NodeType]int
	mu        sync.Mutex
}

func NewNamer(rng *rand.Rand, poolFile string) (*Namer, error) {
	n := &Namer{
		rng:      rng,
		counters: make(map[nodev1.NodeType]int),
	}

	if poolFile != "" {
		pool, err := loadNamesFromFile(poolFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load names pool: %w", err)
		}
		n.pool = pool
	}

	return n, nil
}

func (n *Namer) Generate(nodeType nodev1.NodeType) string {
	n.mu.Lock()
	defer n.mu.Unlock()

	if len(n.pool) > 0 {
		idx := n.rng.Intn(len(n.pool))
		return n.pool[idx]
	}

	n.counters[nodeType]++
	counter := n.counters[nodeType]

	prefix := n.getPrefix(nodeType)
	suffix := n.generateSuffix()

	return fmt.Sprintf("%s-%06d-%s", prefix, counter, suffix)
}

func (n *Namer) getPrefix(nodeType nodev1.NodeType) string {
	switch nodeType {
	case nodev1.NodeType_BAREMETAL:
		return "bm"
	case nodev1.NodeType_VM:
		return "vm"
	case nodev1.NodeType_CONTAINER:
		return "ctr"
	default:
		return "node"
	}
}

func (n *Namer) generateSuffix() string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = letters[n.rng.Intn(len(letters))]
	}
	return string(b)
}

func loadNamesFromFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var names []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		name := scanner.Text()
		if name != "" {
			names = append(names, name)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return names, nil
}