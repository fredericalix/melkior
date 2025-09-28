package sim

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	nodev1 "github.com/melkior/nodestatus/gen/go/api/proto"
	"github.com/melkior/nodestatus/pkg/grpcclient"
	"go.uber.org/zap"
)

type RunOptions struct {
	Duration              string
	UpdateQPS             float64
	MaxConcurrency        int
	ProbStatusFlip        float64
	ProbLabelChange       float64
	ProbMetadataChange    float64
	ProbDeleteAndRecreate float64
	Jitter                bool
	BatchSize             int
	NamesPool             string
}

type Runner struct {
	config     *Config
	logger     *zap.Logger
	client     *grpcclient.Client
	namer      *Namer
	labelGen   *LabelGenerator
	metaGen    *MetadataGenerator
	rng        *rand.Rand
	rateLimiter *TokenBucket
	stats      *RunStats
}

type RunStats struct {
	TotalRPCs    atomic.Int64
	CreateCount  atomic.Int64
	UpdateCount  atomic.Int64
	DeleteCount  atomic.Int64
	StatusFlips  atomic.Int64
	ErrorCount   atomic.Int64
	StartTime    time.Time
}

func NewRunner(cfg *Config, logger *zap.Logger) *Runner {
	return &Runner{
		config: cfg,
		logger: logger,
		stats:  &RunStats{StartTime: time.Now()},
	}
}

func (r *Runner) Run(ctx context.Context, opts RunOptions) error {
	r.rng = r.config.NewRand()

	client, err := grpcclient.NewClient(r.config.BackendAddr, r.config.BackendToken)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()
	r.client = client

	namer, err := NewNamer(r.rng, opts.NamesPool)
	if err != nil {
		return err
	}
	r.namer = namer
	r.labelGen = NewLabelGenerator(r.rng, r.config.SimLabelPrefix)
	r.metaGen = NewMetadataGenerator(r.rng)

	r.rateLimiter = NewTokenBucket(opts.UpdateQPS, opts.UpdateQPS*2)

	duration, err := r.parseDuration(opts.Duration)
	if err != nil {
		return err
	}

	var endTime time.Time
	if duration > 0 {
		endTime = time.Now().Add(duration)
		r.logger.Info("Starting simulation",
			zap.Duration("duration", duration),
			zap.Float64("qps", opts.UpdateQPS),
			zap.Int("max_concurrency", opts.MaxConcurrency))
	} else {
		r.logger.Info("Starting infinite simulation",
			zap.Float64("qps", opts.UpdateQPS),
			zap.Int("max_concurrency", opts.MaxConcurrency))
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	reportTicker := time.NewTicker(30 * time.Second)
	defer reportTicker.Stop()

	semaphore := make(chan struct{}, opts.MaxConcurrency)
	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("Shutting down simulation...")
			wg.Wait()
			r.printFinalStats()
			return nil

		case <-reportTicker.C:
			r.printStats()

		case <-ticker.C:
			if duration > 0 && time.Now().After(endTime) {
				r.logger.Info("Duration reached, shutting down...")
				wg.Wait()
				r.printFinalStats()
				return nil
			}

			nodes, err := r.getSimulatorNodes(ctx)
			if err != nil {
				r.logger.Error("Failed to list nodes", zap.Error(err))
				continue
			}

			if len(nodes) == 0 {
				r.logger.Warn("No simulator nodes found, skipping tick")
				continue
			}

			batchSize := opts.BatchSize
			if batchSize > len(nodes) {
				batchSize = len(nodes)
			}

			for i := 0; i < batchSize; i++ {
				node := nodes[r.rng.Intn(len(nodes))]
				operation := r.selectOperation(opts)

				wg.Add(1)
				semaphore <- struct{}{}

				go func(n *nodev1.Node, op string) {
					defer wg.Done()
					defer func() { <-semaphore }()

					if err := r.rateLimiter.Take(ctx, 1); err != nil {
						return
					}

					r.executeOperation(ctx, n, op, opts)
				}(node, operation)
			}

			if opts.Jitter {
				jitterMs := int(200 * r.rng.Float64())
				time.Sleep(time.Duration(jitterMs) * time.Millisecond)
			}
		}
	}
}

func (r *Runner) selectOperation(opts RunOptions) string {
	roll := r.rng.Float64()

	if roll < opts.ProbDeleteAndRecreate {
		return "delete_recreate"
	}

	roll -= opts.ProbDeleteAndRecreate
	if roll < opts.ProbStatusFlip {
		return "status_flip"
	}

	roll -= opts.ProbStatusFlip
	if roll < opts.ProbLabelChange {
		return "label_change"
	}

	roll -= opts.ProbLabelChange
	if roll < opts.ProbMetadataChange {
		return "metadata_change"
	}

	return "status_flip"
}

func (r *Runner) executeOperation(ctx context.Context, node *nodev1.Node, operation string, opts RunOptions) {
	r.stats.TotalRPCs.Add(1)

	switch operation {
	case "delete_recreate":
		r.deleteAndRecreate(ctx, node)
	case "status_flip":
		r.flipStatus(ctx, node)
	case "label_change":
		r.updateLabels(ctx, node)
	case "metadata_change":
		r.updateMetadata(ctx, node)
	}
}

func (r *Runner) deleteAndRecreate(ctx context.Context, node *nodev1.Node) {
	err := RetryWithBackoff(ctx, DefaultRetryConfig(), func() error {
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return r.client.DeleteNode(ctxWithTimeout, node.Id)
	})

	if err != nil {
		r.stats.ErrorCount.Add(1)
		r.logger.Error("Failed to delete node", zap.String("id", node.Id), zap.Error(err))
		return
	}

	r.stats.DeleteCount.Add(1)

	newNode := &nodev1.Node{
		Name:         r.namer.Generate(node.Type),
		Type:         node.Type,
		Status:       node.Status,
		Labels:       node.Labels,
		MetadataJson: r.metaGen.Generate(node.Type.String()),
	}

	err = RetryWithBackoff(ctx, DefaultRetryConfig(), func() error {
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_, err := r.client.CreateNode(ctxWithTimeout, newNode)
		return err
	})

	if err != nil {
		r.stats.ErrorCount.Add(1)
		r.logger.Error("Failed to recreate node", zap.Error(err))
	} else {
		r.stats.CreateCount.Add(1)
	}
}

func (r *Runner) flipStatus(ctx context.Context, node *nodev1.Node) {
	statuses := []nodev1.NodeStatus{
		nodev1.NodeStatus_UP,
		nodev1.NodeStatus_DOWN,
		nodev1.NodeStatus_DEGRADED,
		nodev1.NodeStatus_UNKNOWN,
	}

	newStatus := statuses[r.rng.Intn(len(statuses))]
	for newStatus == node.Status {
		newStatus = statuses[r.rng.Intn(len(statuses))]
	}

	err := RetryWithBackoff(ctx, DefaultRetryConfig(), func() error {
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_, err := r.client.UpdateStatus(ctxWithTimeout, node.Id, newStatus)
		return err
	})

	if err != nil {
		r.stats.ErrorCount.Add(1)
		r.logger.Error("Failed to update status", zap.String("id", node.Id), zap.Error(err))
	} else {
		r.stats.StatusFlips.Add(1)
		r.stats.UpdateCount.Add(1)
	}
}

func (r *Runner) updateLabels(ctx context.Context, node *nodev1.Node) {
	node.Labels = r.labelGen.UpdateLabels(node.Labels)

	err := RetryWithBackoff(ctx, DefaultRetryConfig(), func() error {
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_, err := r.client.UpdateNode(ctxWithTimeout, node)
		return err
	})

	if err != nil {
		r.stats.ErrorCount.Add(1)
		r.logger.Error("Failed to update labels", zap.String("id", node.Id), zap.Error(err))
	} else {
		r.stats.UpdateCount.Add(1)
	}
}

func (r *Runner) updateMetadata(ctx context.Context, node *nodev1.Node) {
	node.MetadataJson = r.metaGen.Update(node.MetadataJson)

	err := RetryWithBackoff(ctx, DefaultRetryConfig(), func() error {
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_, err := r.client.UpdateNode(ctxWithTimeout, node)
		return err
	})

	if err != nil {
		r.stats.ErrorCount.Add(1)
		r.logger.Error("Failed to update metadata", zap.String("id", node.Id), zap.Error(err))
	} else {
		r.stats.UpdateCount.Add(1)
	}
}

func (r *Runner) getSimulatorNodes(ctx context.Context) ([]*nodev1.Node, error) {
	allNodes, err := r.client.ListNodes(ctx, 0, 0)
	if err != nil {
		return nil, err
	}

	var simNodes []*nodev1.Node
	for _, node := range allNodes {
		if FilterSimulatorLabels(node.Labels) {
			simNodes = append(simNodes, node)
		}
	}

	return simNodes, nil
}

func (r *Runner) parseDuration(durationStr string) (time.Duration, error) {
	if durationStr == "" || durationStr == "0" {
		return 0, nil
	}
	return time.ParseDuration(durationStr)
}

func (r *Runner) printStats() {
	elapsed := time.Since(r.stats.StartTime)
	totalRPCs := r.stats.TotalRPCs.Load()
	qps := float64(totalRPCs) / elapsed.Seconds()

	r.logger.Info("Simulation stats",
		zap.Int64("total_rpcs", totalRPCs),
		zap.Int64("creates", r.stats.CreateCount.Load()),
		zap.Int64("updates", r.stats.UpdateCount.Load()),
		zap.Int64("deletes", r.stats.DeleteCount.Load()),
		zap.Int64("status_flips", r.stats.StatusFlips.Load()),
		zap.Int64("errors", r.stats.ErrorCount.Load()),
		zap.Float64("qps", qps),
		zap.Duration("elapsed", elapsed))
}

func (r *Runner) printFinalStats() {
	fmt.Println("\n========== Final Statistics ==========")
	elapsed := time.Since(r.stats.StartTime)
	totalRPCs := r.stats.TotalRPCs.Load()

	fmt.Printf("Duration: %v\n", elapsed)
	fmt.Printf("Total RPCs: %d\n", totalRPCs)
	fmt.Printf("  - Creates: %d\n", r.stats.CreateCount.Load())
	fmt.Printf("  - Updates: %d\n", r.stats.UpdateCount.Load())
	fmt.Printf("  - Deletes: %d\n", r.stats.DeleteCount.Load())
	fmt.Printf("  - Status Flips: %d\n", r.stats.StatusFlips.Load())
	fmt.Printf("Errors: %d (%.2f%%)\n", r.stats.ErrorCount.Load(),
		float64(r.stats.ErrorCount.Load())*100/float64(totalRPCs+1))
	fmt.Printf("Average QPS: %.2f\n", float64(totalRPCs)/elapsed.Seconds())
	fmt.Println("======================================")
}