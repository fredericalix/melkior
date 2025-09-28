package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/melkior/nodestatus/internal/charts"
	"github.com/melkior/nodestatus/internal/data"
	"github.com/melkior/nodestatus/internal/tui/views"
	"github.com/melkior/nodestatus/pkg/grpcclient"
)

// Config holds the TUI configuration
type Config struct {
	BackendAddr   string
	BackendToken  string
	FPS           int
	ChartsRefresh time.Duration
	WindowSecs    int
}

// Tab represents a view tab
type Tab int

const (
	TabList Tab = iota
	TabDetails
	TabLogs
)

// Model represents the main TUI application model
type Model struct {
	config        Config
	ctx           context.Context
	cancel        context.CancelFunc

	// Views
	listView    *views.ListView
	detailsView *views.DetailsView
	logsView    *views.LogsView

	// Data
	aggregator     *data.Aggregator
	streamConsumer interface {
		Start(context.Context) error
		Stop()
		Events() <-chan *data.Event
		Errors() <-chan error
	}

	// UI state
	activeTab    Tab
	tabs         []string
	help         help.Model
	keys         keyMap
	width        int
	height       int
	chartsActive bool
	err          error
	quitting     bool
}

// keyMap defines all key bindings
type keyMap struct {
	Up        key.Binding
	Down      key.Binding
	Left      key.Binding
	Right     key.Binding
	Charts    key.Binding
	Filter    key.Binding
	Reset     key.Binding
	Tab       key.Binding
	Enter     key.Binding
	Help      key.Binding
	Quit      key.Binding
}

// ShortHelp returns short help
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// FullHelp returns full help
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Tab, k.Enter, k.Charts},
		{k.Filter, k.Reset},
		{k.Help, k.Quit},
	}
}

var defaultKeys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "prev tab"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "next tab"),
	),
	Charts: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "charts"),
	),
	Filter: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "filter"),
	),
	Reset: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "reset filters"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next tab"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

// Messages

type tickMsg time.Time

type eventMsg struct {
	event *data.Event
}

type errorMsg struct {
	err error
}

type chartsFinishedMsg struct{}

// NewModel creates a new TUI model
func NewModel(config Config) (*Model, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create aggregator
	aggregator := data.NewAggregator(config.WindowSecs)

	// Create views
	listView := views.NewListView()
	detailsView := views.NewDetailsView()
	logsView := views.NewLogsView(1000)

	// Create model
	m := &Model{
		config:      config,
		ctx:         ctx,
		cancel:      cancel,
		listView:    listView,
		detailsView: detailsView,
		logsView:    logsView,
		aggregator:  aggregator,
		activeTab:   TabList,
		tabs:        []string{"List", "Details", "Logs"},
		help:        help.New(),
		keys:        defaultKeys,
	}

	return m, nil
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	// Start streaming in background
	m.startStreaming()

	// Return initial commands
	return tea.Batch(
		m.tick(),
		tea.EnterAltScreen,
	)
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.chartsActive {
			// Ignore keys when charts are active
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			m.cancel()
			return m, tea.Quit

		case key.Matches(msg, m.keys.Charts):
			// Launch charts view
			return m, m.showCharts()

		case key.Matches(msg, m.keys.Tab), key.Matches(msg, m.keys.Right):
			m.activeTab = (m.activeTab + 1) % Tab(len(m.tabs))

		case key.Matches(msg, m.keys.Left):
			m.activeTab = (m.activeTab - 1 + Tab(len(m.tabs))) % Tab(len(m.tabs))

		case key.Matches(msg, m.keys.Enter):
			if m.activeTab == TabList {
				// Show selected node in details
				if node := m.listView.GetSelectedNode(); node != nil {
					m.detailsView.SetNode(node)
					m.activeTab = TabDetails
				}
			}

		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width

	case tickMsg:
		// Update views with latest data
		nodes := m.aggregator.GetNodes()
		m.listView.SetNodes(nodes)

		// Continue ticking only
		cmds = append(cmds, m.tick())


	case chartsFinishedMsg:
		m.chartsActive = false
		return m, tea.EnterAltScreen
	}

	// Update active view
	switch m.activeTab {
	case TabList:
		cmd := m.listView.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	case TabDetails:
		cmd := m.detailsView.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	case TabLogs:
		cmd := m.logsView.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m *Model) View() string {
	if m.quitting {
		return ""
	}

	if m.chartsActive {
		return "Charts view active (press ESC or q to return)..."
	}

	var b strings.Builder

	// Render tabs
	b.WriteString(m.renderTabs())
	b.WriteString("\n\n")

	// Render active view
	switch m.activeTab {
	case TabList:
		b.WriteString(m.listView.View())
	case TabDetails:
		b.WriteString(m.detailsView.View())
	case TabLogs:
		b.WriteString(m.logsView.View())
	}

	// Error display
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	// Help
	helpView := m.help.View(m.keys)
	b.WriteString("\n")
	b.WriteString(helpView)

	return b.String()
}

// renderTabs renders the tab bar
func (m *Model) renderTabs() string {
	var tabs []string

	for i, tab := range m.tabs {
		if Tab(i) == m.activeTab {
			tabs = append(tabs, activeTabStyle.Render(tab))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(tab))
		}
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		tabs...,
	)
}

// tick returns a tick command
func (m *Model) tick() tea.Cmd {
	// Ensure reasonable tick rate (max 30 FPS)
	fps := m.config.FPS
	if fps > 30 {
		fps = 30
	}
	if fps < 1 {
		fps = 1
	}

	return tea.Tick(time.Second/time.Duration(fps), func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}


// showCharts launches the charts view
func (m *Model) showCharts() tea.Cmd {
	m.chartsActive = true

	return func() tea.Msg {
		// Run charts in blocking mode
		err := m.runCharts()
		if err != nil {
			return errorMsg{err: err}
		}
		return chartsFinishedMsg{}
	}
}

// runCharts runs the termdash charts
func (m *Model) runCharts() error {
	// Create a new context for charts
	ctx, cancel := context.WithCancel(m.ctx)
	defer cancel()

	// Run termdash
	opts := charts.Options{
		RefreshInterval: m.config.ChartsRefresh,
		ShowHelp:        true,
	}

	return charts.RunTermdash(ctx, m.aggregator, opts)
}

// startStreaming starts the data streaming
func (m *Model) startStreaming() {
	// Check if we should use mock data
	if m.config.BackendAddr == "mock" {
		// Use mock stream consumer for testing
		mockConsumer := data.NewMockStreamConsumer(m.aggregator)
		m.streamConsumer = mockConsumer

		if err := mockConsumer.Start(m.ctx); err != nil {
			m.err = err
		}
	} else {
		// Connect to real backend
		client, err := grpcclient.New(m.config.BackendAddr, m.config.BackendToken)
		if err != nil {
			m.err = err
			return
		}

		// Create stream consumer
		consumer := data.NewStreamConsumer(client.NodeService(), m.aggregator)
		m.streamConsumer = consumer

		if err := consumer.Start(m.ctx); err != nil {
			m.err = err
			return
		}
	}

	// Start background event processor
	go m.processEventsBackground()
}

// processEventsBackground processes events in background
func (m *Model) processEventsBackground() {
	if m.streamConsumer == nil {
		return
	}

	// Create separate context for event processing
	eventCtx := m.ctx

	// Get channels once to avoid potential nil issues
	eventChan := m.streamConsumer.Events()
	errorChan := m.streamConsumer.Errors()

	for {
		select {
		case <-eventCtx.Done():
			// Context cancelled, clean shutdown
			return

		case event, ok := <-eventChan:
			if !ok {
				// Channel closed, exit cleanly
				return
			}
			if event != nil {
				// Non-blocking updates
				go func(e *data.Event) {
					m.aggregator.HandleEvent(e)
				}(event)

				// Update logs view in a non-blocking way
				go func(e *data.Event) {
					m.logsView.AddEvent(e)
				}(event)
			}

		case err, ok := <-errorChan:
			if !ok {
				// Channel closed
				return
			}
			if err != nil {
				// Store error without blocking
				m.err = err
			}

		default:
			// Non-blocking check - prevent tight loop
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// Cleanup cleans up resources
func (m *Model) Cleanup() {
	m.cancel()
	if m.aggregator != nil {
		m.aggregator.Close()
	}
	if m.streamConsumer != nil {
		m.streamConsumer.Stop()
	}
}

// Run starts the TUI application
func Run(ctx context.Context, config Config) error {
	model, err := NewModel(config)
	if err != nil {
		return err
	}
	defer model.Cleanup()

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}