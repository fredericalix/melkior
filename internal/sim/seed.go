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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SeedOptions struct {
	Total        int
	PctBaremetal float64
	PctVM        float64
	PctContainer float64
	Labels       []string
}

type Seeder struct {
	config   *Config
	logger   *zap.Logger
	client   *grpcclient.Client
	namer    *Namer
	labelGen *LabelGenerator
	metaGen  *MetadataGenerator
	rng      *rand.Rand
}

func NewSeeder(cfg *Config, logger *zap.Logger) *Seeder {
	return &Seeder{
		config: cfg,
		logger: logger,
	}
}

func (s *Seeder) Seed(ctx context.Context, opts SeedOptions) error {
	s.rng = s.config.NewRand()

	client, err := grpcclient.NewClient(s.config.BackendAddr, s.config.BackendToken)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()
	s.client = client

	namer, err := NewNamer(s.rng, "")
	if err != nil {
		return err
	}
	s.namer = namer
	s.labelGen = NewLabelGenerator(s.rng, s.config.SimLabelPrefix)
	s.metaGen = NewMetadataGenerator(s.rng)

	s.logger.Info("Starting seed operation",
		zap.Int("total", opts.Total),
		zap.Float64("pct_baremetal", opts.PctBaremetal),
		zap.Float64("pct_vm", opts.PctVM),
		zap.Float64("pct_container", opts.PctContainer),
		zap.Int64("seed", s.config.SimSeed))

	numBaremetal := int(float64(opts.Total) * opts.PctBaremetal)
	numVM := int(float64(opts.Total) * opts.PctVM)
	numContainer := opts.Total - numBaremetal - numVM

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 32)
	errorChan := make(chan error, opts.Total)

	var created atomic.Int32
	var failed atomic.Int32
	startTime := time.Now()

	createNodes := func(nodeType nodev1.NodeType, count int) {
		for i := 0; i < count; i++ {
			if ctx.Err() != nil {
				return
			}

			wg.Add(1)
			semaphore <- struct{}{}

			go func() {
				defer wg.Done()
				defer func() { <-semaphore }()

				node := s.generateNode(nodeType, opts.Labels)

				err := RetryWithBackoff(ctx, DefaultRetryConfig(), func() error {
					ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
					defer cancel()

					_, err := s.client.CreateNode(ctxWithTimeout, node)
					if err != nil {
						if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
							node.Name = s.namer.Generate(nodeType)
							return err
						}
						return err
					}
					return nil
				})

				if err != nil {
					failed.Add(1)
					errorChan <- err
					s.logger.Error("Failed to create node",
						zap.String("name", node.Name),
						zap.Error(err))
				} else {
					created.Add(1)
					if created.Load()%100 == 0 {
						s.logger.Info("Progress",
							zap.Int32("created", created.Load()),
							zap.Int("total", opts.Total))
					}
				}
			}()
		}
	}

	createNodes(nodev1.NodeType_BAREMETAL, numBaremetal)
	createNodes(nodev1.NodeType_VM, numVM)
	createNodes(nodev1.NodeType_CONTAINER, numContainer)

	wg.Wait()
	close(errorChan)

	duration := time.Since(startTime)
	s.logger.Info("Seed operation completed",
		zap.Int32("created", created.Load()),
		zap.Int32("failed", failed.Load()),
		zap.Duration("duration", duration),
		zap.Float64("rate", float64(created.Load())/duration.Seconds()))

	return nil
}

func (s *Seeder) generateNode(nodeType nodev1.NodeType, extraLabels []string) *nodev1.Node {
	name := s.namer.Generate(nodeType)
	labels := s.labelGen.Generate(extraLabels)
	metadata := s.metaGen.Generate(nodeType.String())

	statuses := []nodev1.NodeStatus{
		nodev1.NodeStatus_UP,
		nodev1.NodeStatus_UP,
		nodev1.NodeStatus_UP,
		nodev1.NodeStatus_UP,
		nodev1.NodeStatus_UP,
		nodev1.NodeStatus_UP,
		nodev1.NodeStatus_UP,
		nodev1.NodeStatus_UP,
		nodev1.NodeStatus_DOWN,
		nodev1.NodeStatus_DEGRADED,
		nodev1.NodeStatus_UNKNOWN,
	}

	status := statuses[s.rng.Intn(len(statuses))]

	return &nodev1.Node{
		Name:         name,
		Type:         nodeType,
		Status:       status,
		Labels:       labels,
		MetadataJson: metadata,
	}
}