package charts

import (
	"context"
	"fmt"

	"github.com/melkior/nodestatus/internal/data"
	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/widgets/barchart"
	"github.com/mum4k/termdash/widgets/gauge"
	"github.com/mum4k/termdash/widgets/linechart"
	"github.com/mum4k/termdash/widgets/text"
	nodev1 "github.com/melkior/nodestatus/gen/go/api/proto"
)

// Widgets holds all chart widgets
type Widgets struct {
	statusBar    *barchart.BarChart
	typeBar      *barchart.BarChart
	timeSeries   *linechart.LineChart
	epsGauge     *gauge.Gauge
	mutationGauge *gauge.Gauge
	helpText     *text.Text
}

// NewWidgets creates all chart widgets
func NewWidgets() (*Widgets, error) {
	// Create status bar chart (instead of donut)
	statusBar, err := barchart.New(
		barchart.ShowValues(),
		barchart.BarColors([]cell.Color{
			cell.ColorGreen,
			cell.ColorRed,
			cell.ColorYellow,
			cell.ColorNumber(8),
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create status bar chart: %w", err)
	}

	// Create type bar chart
	typeBar, err := barchart.New(
		barchart.ShowValues(),
		barchart.BarColors([]cell.Color{
			cell.ColorRed,
			cell.ColorCyan,
			cell.ColorGreen,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create bar chart: %w", err)
	}

	// Create time series line chart
	timeSeries, err := linechart.New(
		linechart.AxesCellOpts(cell.FgColor(cell.ColorGray)),
		linechart.YLabelCellOpts(cell.FgColor(cell.ColorWhite)),
		linechart.XLabelCellOpts(cell.FgColor(cell.ColorWhite)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create line chart: %w", err)
	}

	// Create EPS gauge
	epsGauge, err := gauge.New(
		gauge.Height(3),
		gauge.Color(cell.ColorGreen),
		gauge.BorderTitle("Events/sec"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create EPS gauge: %w", err)
	}

	// Create mutation rate gauge
	mutationGauge, err := gauge.New(
		gauge.Height(3),
		gauge.Color(cell.ColorYellow),
		gauge.BorderTitle("Mutations/sec"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create mutation gauge: %w", err)
	}

	// Create help text
	helpText, err := text.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create help text: %w", err)
	}

	// Write help text
	helpText.Write("Press 'q' or 'esc' to exit charts view | '?' for help", text.WriteCellOpts(cell.FgColor(cell.ColorGray)))

	return &Widgets{
		statusBar:     statusBar,
		typeBar:       typeBar,
		timeSeries:    timeSeries,
		epsGauge:      epsGauge,
		mutationGauge: mutationGauge,
		helpText:      helpText,
	}, nil
}

// UpdateStatusBar updates the status bar chart
func (w *Widgets) UpdateStatusBar(snap data.MetricsSnapshot) error {
	// Prepare data
	var categories []string
	var values []int
	var maxValue int

	statusOrder := []nodev1.NodeStatus{
		nodev1.NodeStatus_UP,
		nodev1.NodeStatus_DOWN,
		nodev1.NodeStatus_DEGRADED,
		nodev1.NodeStatus_UNKNOWN,
	}

	for _, status := range statusOrder {
		if count, ok := snap.StatusCounts[status]; ok {
			categories = append(categories, fmt.Sprintf("%s(%d)", statusName(status), count))
			values = append(values, count)
			if count > maxValue {
				maxValue = count
			}
		}
	}

	// Update bar chart
	if len(values) > 0 {
		return w.statusBar.Values(values, maxValue+1, barchart.Labels(categories))
	}

	return nil
}

// UpdateTypeBar updates the type bar chart
func (w *Widgets) UpdateTypeBar(snap data.MetricsSnapshot) error {
	// Prepare data
	var categories []string
	var values []int
	var maxValue int

	typeOrder := []nodev1.NodeType{
		nodev1.NodeType_BAREMETAL,
		nodev1.NodeType_VM,
		nodev1.NodeType_CONTAINER,
	}

	for _, nodeType := range typeOrder {
		if count, ok := snap.TypeCounts[nodeType]; ok && count > 0 {
			categories = append(categories, typeName(nodeType))
			values = append(values, count)
			if count > maxValue {
				maxValue = count
			}
		}
	}

	// Update bar chart
	if len(values) > 0 {
		return w.typeBar.Values(values, maxValue+1, barchart.Labels(categories))
	}

	return nil
}

// UpdateTimeSeries updates the time series line chart
func (w *Widgets) UpdateTimeSeries(snap data.MetricsSnapshot) error {
	// Clear existing series
	w.timeSeries.Series("UP", nil,
		linechart.SeriesCellOpts(cell.FgColor(cell.ColorGreen)),
	)
	w.timeSeries.Series("DOWN", nil,
		linechart.SeriesCellOpts(cell.FgColor(cell.ColorRed)),
	)
	w.timeSeries.Series("DEGRADED", nil,
		linechart.SeriesCellOpts(cell.FgColor(cell.ColorYellow)),
	)
	w.timeSeries.Series("UNKNOWN", nil,
		linechart.SeriesCellOpts(cell.FgColor(cell.ColorGray)),
	)

	// Add time series data
	statusConfigs := []struct {
		status nodev1.NodeStatus
		name   string
		color  cell.Color
	}{
		{nodev1.NodeStatus_UP, "UP", cell.ColorGreen},
		{nodev1.NodeStatus_DOWN, "DOWN", cell.ColorRed},
		{nodev1.NodeStatus_DEGRADED, "DEGRADED", cell.ColorYellow},
		{nodev1.NodeStatus_UNKNOWN, "UNKNOWN", cell.ColorGray},
	}

	for _, cfg := range statusConfigs {
		if series, ok := snap.StatusTimeSeries[cfg.status]; ok && len(series) > 0 {
			values := make([]float64, len(series))
			for i, v := range series {
				values[i] = float64(v)
			}

			err := w.timeSeries.Series(cfg.name, values,
				linechart.SeriesCellOpts(cell.FgColor(cfg.color)),
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// UpdateGauges updates the gauge widgets
func (w *Widgets) UpdateGauges(snap data.MetricsSnapshot) error {
	// Update EPS gauge (scale to 0-100 for visualization)
	epsPercent := int(snap.EventsPerSecond * 10) // Scale factor
	if epsPercent > 100 {
		epsPercent = 100
	}
	if err := w.epsGauge.Percent(epsPercent); err != nil {
		return err
	}

	// Update mutation gauge
	mutationPercent := int(snap.MutationRate * 20) // Scale factor
	if mutationPercent > 100 {
		mutationPercent = 100
	}
	if err := w.mutationGauge.Percent(mutationPercent); err != nil {
		return err
	}

	return nil
}

// UpdateAll updates all widgets with new snapshot
func (w *Widgets) UpdateAll(ctx context.Context, snap data.MetricsSnapshot) error {
	// Update each widget
	if err := w.UpdateStatusBar(snap); err != nil {
		return fmt.Errorf("failed to update status bar: %w", err)
	}

	if err := w.UpdateTypeBar(snap); err != nil {
		return fmt.Errorf("failed to update bar: %w", err)
	}

	if err := w.UpdateTimeSeries(snap); err != nil {
		return fmt.Errorf("failed to update time series: %w", err)
	}

	if err := w.UpdateGauges(snap); err != nil {
		return fmt.Errorf("failed to update gauges: %w", err)
	}

	return nil
}

// Helper functions
func statusName(status nodev1.NodeStatus) string {
	switch status {
	case nodev1.NodeStatus_UP:
		return "UP"
	case nodev1.NodeStatus_DOWN:
		return "DOWN"
	case nodev1.NodeStatus_DEGRADED:
		return "DEGRADED"
	case nodev1.NodeStatus_UNKNOWN:
		return "UNKNOWN"
	default:
		return "?"
	}
}

func typeName(nodeType nodev1.NodeType) string {
	switch nodeType {
	case nodev1.NodeType_BAREMETAL:
		return "BAREMETAL"
	case nodev1.NodeType_VM:
		return "VM"
	case nodev1.NodeType_CONTAINER:
		return "CONTAINER"
	default:
		return "?"
	}
}