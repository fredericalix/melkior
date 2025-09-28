package views

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/melkior/nodestatus/internal/data"
)

// DetailsView displays detailed information about a selected node
type DetailsView struct {
	node   *data.Node
	width  int
	height int
	offset int // For scrolling
}

// NewDetailsView creates a new details view
func NewDetailsView() *DetailsView {
	return &DetailsView{}
}

// Update handles messages
func (v *DetailsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width / 2 // Details takes half screen
		v.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if v.offset > 0 {
				v.offset--
			}
		case "down", "j":
			v.offset++
		case "pgup":
			v.offset -= 10
			if v.offset < 0 {
				v.offset = 0
			}
		case "pgdown":
			v.offset += 10
		}
	}

	return nil
}

// View renders the details view
func (v *DetailsView) View() string {
	if v.node == nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render("No node selected")
	}

	// Build content
	var lines []string

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4"))

	lines = append(lines, headerStyle.Render("Node Details"))
	lines = append(lines, "")

	// Basic info
	lines = append(lines, v.renderField("ID", v.node.ID))
	lines = append(lines, v.renderField("Name", v.node.Name))
	lines = append(lines, v.renderField("Type", v.node.Type.String()))
	lines = append(lines, v.renderField("Status", v.node.Status.String()))
	lines = append(lines, v.renderField("Last Seen", v.node.LastSeen.Format("2006-01-02 15:04:05")))
	lines = append(lines, "")

	// Labels
	if len(v.node.Labels) > 0 {
		lines = append(lines, headerStyle.Render("Labels"))
		for k, val := range v.node.Labels {
			lines = append(lines, v.renderField("  "+k, val))
		}
		lines = append(lines, "")
	}

	// Metadata
	if v.node.Metadata != "" {
		lines = append(lines, headerStyle.Render("Metadata"))

		// Try to parse as JSON for pretty printing
		var jsonData interface{}
		if err := json.Unmarshal([]byte(v.node.Metadata), &jsonData); err == nil {
			prettyJSON, _ := json.MarshalIndent(jsonData, "", "  ")
			metadataLines := strings.Split(string(prettyJSON), "\n")
			lines = append(lines, metadataLines...)
		} else {
			// Fallback to raw string
			lines = append(lines, v.node.Metadata)
		}
	}

	// Apply scrolling
	visibleLines := v.height - 2
	if v.offset > len(lines)-visibleLines {
		v.offset = len(lines) - visibleLines
	}
	if v.offset < 0 {
		v.offset = 0
	}

	endIdx := v.offset + visibleLines
	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	// Render visible lines
	content := strings.Join(lines[v.offset:endIdx], "\n")

	// Add scroll indicator
	if len(lines) > visibleLines {
		scrollInfo := fmt.Sprintf("[%d-%d/%d]", v.offset+1, endIdx, len(lines))
		content += "\n" + lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render(scrollInfo)
	}

	// Apply border and padding
	return lipgloss.NewStyle().
		Width(v.width).
		Height(v.height).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("241")).
		Padding(1, 2).
		Render(content)
}

// SetNode sets the node to display
func (v *DetailsView) SetNode(node *data.Node) {
	v.node = node
	v.offset = 0
}

// renderField renders a field with label and value
func (v *DetailsView) renderField(label, value string) string {
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Bold(true)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#C0CAF5"))

	// Handle status coloring
	if label == "Status" {
		valueStyle = GetStatusStyle(value)
	}

	return fmt.Sprintf("%s: %s",
		labelStyle.Render(label),
		valueStyle.Render(value))
}

// GetStatusStyle returns the style for a status value
func GetStatusStyle(status string) lipgloss.Style {
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
	return lipgloss.NewStyle().Foreground(color).Bold(true)
}