package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/melkior/nodestatus/internal/sim"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger *zap.Logger
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var err error
	logger, err = setupLogger()
	if err != nil {
		return fmt.Errorf("failed to setup logger: %w", err)
	}
	defer logger.Sync()

	rootCmd := &cobra.Command{
		Use:   "demo-sim",
		Short: "Node service simulation tool",
		Long:  "A CLI tool for simulating node operations against the gRPC backend",
	}

	rootCmd.AddCommand(
		seedCmd(),
		runCmd(),
		cleanupCmd(),
		statsCmd(),
	)

	return rootCmd.Execute()
}

func seedCmd() *cobra.Command {
	var (
		total         int
		pctBaremetal  float64
		pctVM         float64
		pctContainer  float64
		labels        []string
	)

	cmd := &cobra.Command{
		Use:   "seed",
		Short: "Create initial dataset of nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := sim.LoadConfig()
			if err != nil {
				return err
			}

			if pctBaremetal+pctVM+pctContainer != 1.0 {
				return fmt.Errorf("percentages must sum to 1.0 (got %.2f)", pctBaremetal+pctVM+pctContainer)
			}

			seeder := sim.NewSeeder(cfg, logger)

			ctx, cancel := setupSignalHandler()
			defer cancel()

			return seeder.Seed(ctx, sim.SeedOptions{
				Total:        total,
				PctBaremetal: pctBaremetal,
				PctVM:        pctVM,
				PctContainer: pctContainer,
				Labels:       labels,
			})
		},
	}

	cmd.Flags().IntVar(&total, "total", 300, "Total number of nodes to create")
	cmd.Flags().Float64Var(&pctBaremetal, "pct-baremetal", 0.10, "Percentage of baremetal nodes")
	cmd.Flags().Float64Var(&pctVM, "pct-vm", 0.50, "Percentage of VM nodes")
	cmd.Flags().Float64Var(&pctContainer, "pct-container", 0.40, "Percentage of container nodes")
	cmd.Flags().StringSliceVar(&labels, "labels", []string{}, "Additional labels (key=value)")

	return cmd
}

func runCmd() *cobra.Command {
	var (
		duration              string
		updateQPS             float64
		maxConcurrency        int
		probStatusFlip        float64
		probLabelChange       float64
		probMetadataChange    float64
		probDeleteAndRecreate float64
		jitter                bool
		batchSize             int
		namesPool             string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start continuous simulation",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := sim.LoadConfig()
			if err != nil {
				return err
			}

			runner := sim.NewRunner(cfg, logger)

			ctx, cancel := setupSignalHandler()
			defer cancel()

			return runner.Run(ctx, sim.RunOptions{
				Duration:              duration,
				UpdateQPS:             updateQPS,
				MaxConcurrency:        maxConcurrency,
				ProbStatusFlip:        probStatusFlip,
				ProbLabelChange:       probLabelChange,
				ProbMetadataChange:    probMetadataChange,
				ProbDeleteAndRecreate: probDeleteAndRecreate,
				Jitter:                jitter,
				BatchSize:             batchSize,
				NamesPool:             namesPool,
			})
		},
	}

	cmd.Flags().StringVar(&duration, "duration", "0", "Duration to run (e.g., 6h, 0=infinite)")
	cmd.Flags().Float64Var(&updateQPS, "update-qps", 15.0, "Approximate updates per second")
	cmd.Flags().IntVar(&maxConcurrency, "max-concurrency", 32, "Maximum concurrent goroutines")
	cmd.Flags().Float64Var(&probStatusFlip, "prob-status-flip", 0.25, "Probability of status change")
	cmd.Flags().Float64Var(&probLabelChange, "prob-label-change", 0.15, "Probability of label change")
	cmd.Flags().Float64Var(&probMetadataChange, "prob-metadata-change", 0.20, "Probability of metadata change")
	cmd.Flags().Float64Var(&probDeleteAndRecreate, "prob-delete-and-recreate", 0.02, "Probability of delete and recreate")
	cmd.Flags().BoolVar(&jitter, "jitter", true, "Add ±20% jitter to sleep intervals")
	cmd.Flags().IntVar(&batchSize, "batch-size", 50, "Number of nodes per update tick")
	cmd.Flags().StringVar(&namesPool, "names-pool", "", "Path to file with candidate names")

	return cmd
}

func cleanupCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Remove all nodes created by the simulator",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := sim.LoadConfig()
			if err != nil {
				return err
			}

			cleaner := sim.NewCleaner(cfg, logger)

			ctx, cancel := setupSignalHandler()
			defer cancel()

			return cleaner.Cleanup(ctx, force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")

	return cmd
}

func statsCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Print current counts by type & status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := sim.LoadConfig()
			if err != nil {
				return err
			}

			stats := sim.NewStats(cfg, logger)

			ctx, cancel := setupSignalHandler()
			defer cancel()

			return stats.Print(ctx, jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	return cmd
}

func setupLogger() (*zap.Logger, error) {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.EncoderConfig.TimeKey = "time"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	return config.Build()
}

func setupSignalHandler() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal, shutting down gracefully...")
		cancel()
	}()

	return ctx, cancel
}