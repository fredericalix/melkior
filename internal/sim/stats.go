package sim

import (
	"context"
	"encoding/json"
	"fmt"
	"text/tabwriter"
	"os"

	"github.com/melkior/nodestatus/pkg/grpcclient"
	"go.uber.org/zap"
)

type Stats struct {
	config *Config
	logger *zap.Logger
	client *grpcclient.Client
}

type StatsData struct {
	Total      int                       `json:"total"`
	ByType     map[string]int            `json:"by_type"`
	ByStatus   map[string]int            `json:"by_status"`
	ByTypeAndStatus map[string]map[string]int `json:"by_type_and_status"`
	SimulatorNodes int                      `json:"simulator_nodes"`
}

func NewStats(cfg *Config, logger *zap.Logger) *Stats {
	return &Stats{
		config: cfg,
		logger: logger,
	}
}

func (s *Stats) Print(ctx context.Context, jsonOutput bool) error {
	client, err := grpcclient.NewClient(s.config.BackendAddr, s.config.BackendToken)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()
	s.client = client

	stats, err := s.collect(ctx)
	if err != nil {
		return err
	}

	if jsonOutput {
		return s.printJSON(stats)
	}
	return s.printTable(stats)
}

func (s *Stats) collect(ctx context.Context) (*StatsData, error) {
	allNodes, err := s.client.ListNodes(ctx, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list all nodes: %w", err)
	}

	stats := &StatsData{
		Total:           len(allNodes),
		ByType:          make(map[string]int),
		ByStatus:        make(map[string]int),
		ByTypeAndStatus: make(map[string]map[string]int),
	}

	for _, node := range allNodes {
		typeStr := node.Type.String()
		statusStr := node.Status.String()

		stats.ByType[typeStr]++
		stats.ByStatus[statusStr]++

		if _, ok := stats.ByTypeAndStatus[typeStr]; !ok {
			stats.ByTypeAndStatus[typeStr] = make(map[string]int)
		}
		stats.ByTypeAndStatus[typeStr][statusStr]++

		if FilterSimulatorLabels(node.Labels) {
			stats.SimulatorNodes++
		}
	}

	return stats, nil
}

func (s *Stats) printJSON(stats *StatsData) error {
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func (s *Stats) printTable(stats *StatsData) error {
	fmt.Println("\n===== Node Statistics =====")
	fmt.Printf("Total Nodes: %d\n", stats.Total)
	fmt.Printf("Simulator Nodes: %d\n", stats.SimulatorNodes)
	fmt.Println()

	fmt.Println("By Type:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TYPE\tCOUNT\tPERCENT")
	for _, nodeType := range []string{"BAREMETAL", "VM", "CONTAINER"} {
		count := stats.ByType[nodeType]
		pct := float64(count) * 100 / float64(stats.Total)
		fmt.Fprintf(w, "%s\t%d\t%.1f%%\n", nodeType, count, pct)
	}
	w.Flush()
	fmt.Println()

	fmt.Println("By Status:")
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tCOUNT\tPERCENT")
	for _, status := range []string{"UP", "DOWN", "DEGRADED", "UNKNOWN"} {
		count := stats.ByStatus[status]
		pct := float64(count) * 100 / float64(stats.Total)
		fmt.Fprintf(w, "%s\t%d\t%.1f%%\n", status, count, pct)
	}
	w.Flush()
	fmt.Println()

	fmt.Println("By Type and Status:")
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TYPE\tUP\tDOWN\tDEGRADED\tUNKNOWN")
	for _, nodeType := range []string{"BAREMETAL", "VM", "CONTAINER"} {
		fmt.Fprintf(w, "%s\t", nodeType)
		for _, status := range []string{"UP", "DOWN", "DEGRADED", "UNKNOWN"} {
			count := 0
			if typeMap, ok := stats.ByTypeAndStatus[nodeType]; ok {
				count = typeMap[status]
			}
			fmt.Fprintf(w, "%d\t", count)
		}
		fmt.Fprintln(w)
	}
	w.Flush()
	fmt.Println("===========================")

	return nil
}