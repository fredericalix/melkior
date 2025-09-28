package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/melkior/nodestatus/internal/data"
	nodev1 "github.com/melkior/nodestatus/gen/go/api/proto"
)

// LogsView displays a scrollable event log
type LogsView struct {
	events      []*data.Event
	maxEvents   int
	width       int
	height      int
	offset      int
	autoScroll  bool
}

// NewLogsView creates a new logs view
func NewLogsView(maxEvents int) *LogsView {
	return &LogsView{
		events:     make([]*data.Event, 0, maxEvents),
		maxEvents:  maxEvents,
		autoScroll: true,
	}
}

// Update handles messages
func (v *LogsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			v.autoScroll = false
			if v.offset > 0 {
				v.offset--
			}
		case "down", "j":
			v.offset++
			if v.offset >= len(v.events)-v.height+2 {
				v.autoScroll = true
			}
		case "pgup":
			v.autoScroll = false
			v.offset -= 10
			if v.offset < 0 {
				v.offset = 0
			}
		case "pgdown":
			v.offset += 10
		case "home":
			v.offset = 0
			v.autoScroll = false
		case "end":
			v.offset = len(v.events) - v.height + 2
			v.autoScroll = true
		case "a":
			v.autoScroll = !v.autoScroll
		}
	}

	return nil
}

// View renders the logs view
func (v *LogsView) View() string {
	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4"))

	header := "Event Log"
	if v.autoScroll {
		header += " [AUTO-SCROLL]"
	}
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n\n")

	// Calculate visible range
	visibleLines := v.height - 4
	if visibleLines < 1 {
		visibleLines = 1
	}

	// Apply auto-scroll
	if v.autoScroll && len(v.events) > visibleLines {
		v.offset = len(v.events) - visibleLines
	}

	// Ensure offset is valid
	if v.offset > len(v.events)-visibleLines {
		v.offset = len(v.events) - visibleLines
	}
	if v.offset < 0 {
		v.offset = 0
	}

	// Get visible events
	startIdx := v.offset
	endIdx := startIdx + visibleLines
	if endIdx > len(v.events) {
		endIdx = len(v.events)
	}

	// Render events
	if len(v.events) == 0 {
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render("No events yet..."))
	} else {
		for i := startIdx; i < endIdx; i++ {
			b.WriteString(v.formatEvent(v.events[i]))
			if i < endIdx-1 {
				b.WriteString("\n")
			}
		}
	}

	// Add scroll indicator
	if len(v.events) > visibleLines {
		scrollInfo := fmt.Sprintf("\n[%d-%d/%d]", startIdx+1, endIdx, len(v.events))
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render(scrollInfo))
	}

	return b.String()
}

// AddEvent adds a new event to the log
func (v *LogsView) AddEvent(event *data.Event) {
	// Defensive check
	if event == nil {
		return
	}

	v.events = append(v.events, event)

	// Trim to max size
	if len(v.events) > v.maxEvents {
		v.events = v.events[len(v.events)-v.maxEvents:]
	}
}

// Clear clears all events
func (v *LogsView) Clear() {
	v.events = v.events[:0]
	v.offset = 0
}

// formatEvent formats an event for display
func (v *LogsView) formatEvent(event *data.Event) string {
	// Timestamp
	timestampStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	timestamp := event.Timestamp.Format("15:04:05")

	// Event type
	eventTypeStyle := v.getEventTypeStyle(event.Type)
	eventType := v.getEventTypeName(event.Type)

	// Node info
	nodeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#C0CAF5"))

	nodeInfo := fmt.Sprintf("%s (%s)", event.Node.Name, event.Node.Type.String())

	// Status
	statusStyle := GetStatusStyle(event.Node.Status.String())
	status := event.Node.Status.String()

	// Build event line
	line := fmt.Sprintf("%s %s %s [%s]",
		timestampStyle.Render(timestamp),
		eventTypeStyle.Render(eventType),
		nodeStyle.Render(nodeInfo),
		statusStyle.Render(status))

	// Add changed fields if present
	if len(event.ChangedFields) > 0 {
		changedStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)
		line += " " + changedStyle.Render(fmt.Sprintf("(%s)", strings.Join(event.ChangedFields, ", ")))
	}

	return line
}

// getEventTypeStyle returns the style for an event type
func (v *LogsView) getEventTypeStyle(eventType nodev1.EventType) lipgloss.Style {
	var color lipgloss.Color
	switch eventType {
	case nodev1.EventType_CREATED:
		color = lipgloss.Color("#04B575")
	case nodev1.EventType_UPDATED:
		color = lipgloss.Color("#FFA500")
	case nodev1.EventType_DELETED:
		color = lipgloss.Color("#FF0000")
	default:
		color = lipgloss.Color("#626262")
	}
	return lipgloss.NewStyle().Foreground(color).Bold(true)
}

// getEventTypeName returns the display name for an event type
func (v *LogsView) getEventTypeName(eventType nodev1.EventType) string {
	switch eventType {
	case nodev1.EventType_CREATED:
		return "CREATED"
	case nodev1.EventType_UPDATED:
		return "UPDATED"
	case nodev1.EventType_DELETED:
		return "DELETED"
	default:
		return "UNKNOWN"
	}
}