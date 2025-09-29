package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	nodev1 "github.com/melkior/nodestatus/gen/go/api/proto"
	"github.com/melkior/nodestatus/internal/data"
)

// ChartsView displays metrics charts using bubble tea
type ChartsView struct {
	width      int
	height     int
	aggregator *data.Aggregator
	snapshot   data.MetricsSnapshot
}

// NewChartsView creates a new charts view
func NewChartsView(aggregator *data.Aggregator) *ChartsView {
	return &ChartsView{
		aggregator: aggregator,
		snapshot:   aggregator.Snapshot(),
		width:      80,  // Default width
		height:     24,  // Default height
	}
}

// Init initializes the charts view
func (v *ChartsView) Init() tea.Cmd {
	return nil
}

// Update handles messages for the charts view
func (v *ChartsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
	case data.MetricsSnapshot:
		v.snapshot = msg
	}
	return nil
}

// View renders the charts
func (v *ChartsView) View() string {
	if v.width == 0 || v.height == 0 {
		return "Initializing charts..."
	}

	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("📊 Node Status Charts"))
	b.WriteString("\n\n")

	// Status distribution bar chart
	b.WriteString(v.renderStatusChart())
	b.WriteString("\n")

	// Event rate line chart
	b.WriteString(v.renderEventRateChart())
	b.WriteString("\n")

	// Node type distribution
	b.WriteString(v.renderTypeDistribution())
	b.WriteString("\n")

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))
	b.WriteString(helpStyle.Render("Press 'q' or 'ESC' to return to main view"))

	return b.String()
}

// renderStatusChart renders a horizontal bar chart of node statuses
func (v *ChartsView) renderStatusChart() string {
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().Bold(true).Underline(true)
	b.WriteString(headerStyle.Render("Node Status Distribution"))
	b.WriteString("\n\n")

	maxWidth := v.width - 20
	if maxWidth < 20 {
		maxWidth = 20
	}

	totalNodes := v.snapshot.TotalNodes
	if totalNodes == 0 {
		totalNodes = 1 // Avoid division by zero
	}

	statusColors := map[nodev1.NodeStatus]string{
		nodev1.NodeStatus_UP:       "42",  // Green
		nodev1.NodeStatus_DOWN:     "196", // Red
		nodev1.NodeStatus_DEGRADED: "214", // Orange
		nodev1.NodeStatus_UNKNOWN:  "241", // Gray
	}

	statusNames := map[nodev1.NodeStatus]string{
		nodev1.NodeStatus_UP:       "UP      ",
		nodev1.NodeStatus_DOWN:     "DOWN    ",
		nodev1.NodeStatus_DEGRADED: "DEGRADED",
		nodev1.NodeStatus_UNKNOWN:  "UNKNOWN ",
	}

	for status := nodev1.NodeStatus(0); status <= nodev1.NodeStatus_DEGRADED; status++ {
		count := v.snapshot.StatusCounts[status]
		ratio := float64(count) / float64(totalNodes)
		barWidth := int(ratio * float64(maxWidth))

		color := statusColors[status]
		barStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color))

		name := statusNames[status]
		bar := strings.Repeat("█", barWidth)
		if barWidth == 0 && count > 0 {
			bar = "▏" // Show minimal bar for non-zero counts
		}

		line := fmt.Sprintf("%s │ %s %d (%.1f%%)",
			name,
			barStyle.Render(bar),
			count,
			ratio*100)

		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

// renderEventRateChart renders a simple line chart for event rates
func (v *ChartsView) renderEventRateChart() string {
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().Bold(true).Underline(true)
	b.WriteString(headerStyle.Render("Event Rate (last 60 seconds)"))
	b.WriteString("\n\n")

	// Get event time series
	allEvents := make([]int, 0)
	for _, buffer := range v.snapshot.StatusTimeSeries {
		if len(buffer) > len(allEvents) {
			allEvents = buffer
			break
		}
	}

	if len(allEvents) == 0 {
		return b.String() + "No event data available\n"
	}

	// Find max value for scaling
	maxVal := 1
	for _, val := range allEvents {
		if val > maxVal {
			maxVal = val
		}
	}

	// Chart dimensions
	chartHeight := 10
	chartWidth := v.width - 10
	if chartWidth > len(allEvents) {
		chartWidth = len(allEvents)
	}

	// Create the chart
	for row := chartHeight; row > 0; row-- {
		threshold := float64(row) / float64(chartHeight) * float64(maxVal)

		// Y-axis label
		if row == chartHeight {
			b.WriteString(fmt.Sprintf("%3d │ ", maxVal))
		} else if row == 1 {
			b.WriteString("  0 │ ")
		} else {
			b.WriteString("    │ ")
		}

		// Plot points
		start := len(allEvents) - chartWidth
		if start < 0 {
			start = 0
		}

		for i := start; i < len(allEvents); i++ {
			val := allEvents[i]
			if float64(val) >= threshold {
				b.WriteString("▄")
			} else {
				b.WriteString(" ")
			}
		}
		b.WriteString("\n")
	}

	// X-axis
	b.WriteString("    └")
	b.WriteString(strings.Repeat("─", chartWidth))
	b.WriteString("\n")

	// Time labels
	b.WriteString("      ")
	if chartWidth > 20 {
		b.WriteString("60s ago")
		b.WriteString(strings.Repeat(" ", chartWidth-14))
		b.WriteString("now")
	} else {
		b.WriteString("time →")
	}
	b.WriteString("\n")

	return b.String()
}

// renderTypeDistribution renders node type distribution
func (v *ChartsView) renderTypeDistribution() string {
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().Bold(true).Underline(true)
	b.WriteString(headerStyle.Render("Node Type Distribution"))
	b.WriteString("\n\n")

	typeNames := map[nodev1.NodeType]string{
		nodev1.NodeType_BAREMETAL: "Bare Metal",
		nodev1.NodeType_VM:        "Virtual Machine",
		nodev1.NodeType_CONTAINER: "Container",
	}

	typeColors := map[nodev1.NodeType]string{
		nodev1.NodeType_BAREMETAL: "33",  // Blue
		nodev1.NodeType_VM:        "135", // Purple
		nodev1.NodeType_CONTAINER: "220", // Yellow
	}

	for nodeType := nodev1.NodeType(1); nodeType <= nodev1.NodeType_CONTAINER; nodeType++ {
		count := v.snapshot.TypeCounts[nodeType]
		ratio := v.snapshot.TypeRatios[nodeType]

		name := typeNames[nodeType]
		color := typeColors[nodeType]
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))

		// Simple pie chart representation using unicode
		blocks := int(ratio * 10)
		bar := strings.Repeat("●", blocks)

		line := fmt.Sprintf("%-15s: %s %d (%.1f%%)",
			name,
			style.Render(bar),
			count,
			ratio*100)

		b.WriteString(line)
		b.WriteString("\n")
	}

	// Summary stats
	b.WriteString("\n")
	summaryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	b.WriteString(summaryStyle.Render(fmt.Sprintf(
		"Total Nodes: %d | Events/sec: %.1f | Mutations/sec: %.1f",
		v.snapshot.TotalNodes,
		v.snapshot.EventsPerSecond,
		v.snapshot.MutationRate,
	)))

	return b.String()
}

// SetSnapshot updates the metrics snapshot
func (v *ChartsView) SetSnapshot(snapshot data.MetricsSnapshot) {
	v.snapshot = snapshot
}