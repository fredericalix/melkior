package sim

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/melkior/nodestatus/pkg/grpcclient"
	"go.uber.org/zap"
)

type Cleaner struct {
	config *Config
	logger *zap.Logger
	client *grpcclient.Client
}

func NewCleaner(cfg *Config, logger *zap.Logger) *Cleaner {
	return &Cleaner{
		config: cfg,
		logger: logger,
	}
}

func (c *Cleaner) Cleanup(ctx context.Context, force bool) error {
	client, err := grpcclient.NewClient(c.config.BackendAddr, c.config.BackendToken)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()
	c.client = client

	c.logger.Info("Fetching simulator nodes...")
	nodes, err := c.client.ListNodes(ctx, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	var toDelete []string
	for _, node := range nodes {
		if FilterSimulatorLabels(node.Labels) {
			toDelete = append(toDelete, node.Id)
		}
	}

	if len(toDelete) == 0 {
		c.logger.Info("No simulator nodes found to cleanup")
		return nil
	}

	c.logger.Info("Found simulator nodes", zap.Int("count", len(toDelete)))

	if !force {
		fmt.Printf("About to delete %d nodes. Continue? (y/N): ", len(toDelete))
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			c.logger.Info("Cleanup cancelled by user")
			return nil
		}
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 32)
	var deleted atomic.Int32
	var failed atomic.Int32
	startTime := time.Now()

	for _, id := range toDelete {
		if ctx.Err() != nil {
			break
		}

		wg.Add(1)
		semaphore <- struct{}{}

		go func(nodeID string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			err := RetryWithBackoff(ctx, DefaultRetryConfig(), func() error {
				ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				return c.client.DeleteNode(ctxWithTimeout, nodeID)
			})

			if err != nil {
				failed.Add(1)
				c.logger.Error("Failed to delete node",
					zap.String("id", nodeID),
					zap.Error(err))
			} else {
				deleted.Add(1)
				if deleted.Load()%100 == 0 {
					c.logger.Info("Progress",
						zap.Int32("deleted", deleted.Load()),
						zap.Int("total", len(toDelete)))
				}
			}
		}(id)
	}

	wg.Wait()

	duration := time.Since(startTime)
	c.logger.Info("Cleanup completed",
		zap.Int32("deleted", deleted.Load()),
		zap.Int32("failed", failed.Load()),
		zap.Duration("duration", duration))

	return nil
}