package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/melkior/nodestatus/internal/data"
	nodev1 "github.com/melkior/nodestatus/gen/go/api/proto"
)

// ListView displays a table of nodes
type ListView struct {
	table        table.Model
	nodes        []*data.Node
	filteredNodes []*data.Node
	typeFilter   nodev1.NodeType
	statusFilter nodev1.NodeStatus
	showFilters  bool
	width        int
	height       int
	focused      bool
}

// NewListView creates a new list view
func NewListView() *ListView {
	columns := []table.Column{
		{Title: "ID", Width: 20},
		{Title: "Name", Width: 25},
		{Title: "Type", Width: 12},
		{Title: "Status", Width: 10},
		{Title: "Last Seen", Width: 20},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	// Set styles
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return &ListView{
		table:        t,
		nodes:        []*data.Node{},
		filteredNodes: []*data.Node{},
		showFilters:  false,
		focused:      true,
	}
}

// Update handles messages
func (v *ListView) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		v.table.SetHeight(msg.Height - 4) // Leave room for header and footer

	case tea.KeyMsg:
		if v.focused {
			switch msg.String() {
			case "f":
				v.showFilters = !v.showFilters
				return nil
			case "r":
				v.resetFilters()
				return nil
			}
		}
	}

	// Update table
	if v.focused {
		v.table, cmd = v.table.Update(msg)
	}

	return cmd
}

// View renders the list view
func (v *ListView) View() string {
	var b strings.Builder

	// Filters section
	if v.showFilters {
		filterText := "Filters: "
		if v.typeFilter != 0 {
			filterText += fmt.Sprintf("Type=%s ", v.typeFilter.String())
		}
		if v.statusFilter != 0 {
			filterText += fmt.Sprintf("Status=%s ", v.statusFilter.String())
		}
		if v.typeFilter == 0 && v.statusFilter == 0 {
			filterText += "None"
		}
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render(filterText))
		b.WriteString("\n\n")
	}

	// Table
	b.WriteString(v.table.View())
	b.WriteString("\n")

	// Footer with counts
	statusCounts := v.getStatusCounts()
	footer := fmt.Sprintf(
		"Total: %d | UP: %d | DOWN: %d | DEGRADED: %d | UNKNOWN: %d",
		len(v.filteredNodes),
		statusCounts[nodev1.NodeStatus_UP],
		statusCounts[nodev1.NodeStatus_DOWN],
		statusCounts[nodev1.NodeStatus_DEGRADED],
		statusCounts[nodev1.NodeStatus_UNKNOWN],
	)
	b.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render(footer))

	return b.String()
}

// SetNodes updates the node list
func (v *ListView) SetNodes(nodes []*data.Node) {
	v.nodes = nodes
	v.applyFilters()
	v.updateTable()
}

// SetTypeFilter sets the type filter
func (v *ListView) SetTypeFilter(nodeType nodev1.NodeType) {
	v.typeFilter = nodeType
	v.applyFilters()
	v.updateTable()
}

// SetStatusFilter sets the status filter
func (v *ListView) SetStatusFilter(status nodev1.NodeStatus) {
	v.statusFilter = status
	v.applyFilters()
	v.updateTable()
}

// resetFilters clears all filters
func (v *ListView) resetFilters() {
	v.typeFilter = 0
	v.statusFilter = 0
	v.applyFilters()
	v.updateTable()
}

// applyFilters applies current filters to the node list
func (v *ListView) applyFilters() {
	v.filteredNodes = make([]*data.Node, 0, len(v.nodes))

	for _, node := range v.nodes {
		// Apply type filter
		if v.typeFilter != 0 && node.Type != v.typeFilter {
			continue
		}

		// Apply status filter
		if v.statusFilter != 0 && node.Status != v.statusFilter {
			continue
		}

		v.filteredNodes = append(v.filteredNodes, node)
	}

	// Sort by name
	sort.Slice(v.filteredNodes, func(i, j int) bool {
		return v.filteredNodes[i].Name < v.filteredNodes[j].Name
	})
}

// updateTable updates the table with filtered nodes
func (v *ListView) updateTable() {
	rows := make([]table.Row, 0, len(v.filteredNodes))

	for _, node := range v.filteredNodes {
		rows = append(rows, table.Row{
			truncateID(node.ID),
			node.Name,
			node.Type.String(),
			colorizeStatus(node.Status.String()),
			node.LastSeen.Format("2006-01-02 15:04:05"),
		})
	}

	v.table.SetRows(rows)
}

// getStatusCounts returns counts by status
func (v *ListView) getStatusCounts() map[nodev1.NodeStatus]int {
	counts := make(map[nodev1.NodeStatus]int)
	for _, node := range v.filteredNodes {
		counts[node.Status]++
	}
	return counts
}

// SetFocused sets the focus state
func (v *ListView) SetFocused(focused bool) {
	v.focused = focused
	v.table.SetCursor(0)
}

// GetSelectedNode returns the currently selected node
func (v *ListView) GetSelectedNode() *data.Node {
	if len(v.filteredNodes) == 0 {
		return nil
	}

	cursor := v.table.Cursor()
	if cursor >= 0 && cursor < len(v.filteredNodes) {
		return v.filteredNodes[cursor]
	}

	return nil
}

// Helper functions
func truncateID(id string) string {
	if len(id) > 18 {
		return id[:18] + "..."
	}
	return id
}

func colorizeStatus(status string) string {
	var color lipgloss.Color
	switch status {
	case "UP":
		color = lipgloss.Color("#04B575")
	case "DOWN":
		color = lipgloss.Color("#FF0000")
	case "DEGRADED":
		color = lipgloss.Color("#FFA500")
	default:
		color = lipgloss.Color("#626262")
	}
	return lipgloss.NewStyle().Foreground(color).Render(status)
}