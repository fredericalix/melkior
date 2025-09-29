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
	"github.com/melkior/nodestatus/internal/data"
	"github.com/melkior/nodestatus/internal/logging"
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
	TabCharts
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
	chartsView  *views.ChartsView

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


// NewModel creates a new TUI model
func NewModel(config Config) (*Model, error) {
	logging.Debug("Creating new TUI model with config: %+v", config)
	ctx, cancel := context.WithCancel(context.Background())

	// Create aggregator
	logging.Debug("Creating data aggregator with %d seconds window", config.WindowSecs)
	aggregator := data.NewAggregator(config.WindowSecs)

	// Create views
	logging.Debug("Creating TUI views...")
	listView := views.NewListView()
	detailsView := views.NewDetailsView()
	logsView := views.NewLogsView(1000)
	chartsView := views.NewChartsView(aggregator)

	// Create model
	m := &Model{
		config:      config,
		ctx:         ctx,
		cancel:      cancel,
		listView:    listView,
		detailsView: detailsView,
		logsView:    logsView,
		chartsView:  chartsView,
		aggregator:  aggregator,
		activeTab:   TabList,
		tabs:        []string{"List", "Details", "Logs", "Charts"},
		help:        help.New(),
		keys:        defaultKeys,
	}

	logging.Debug("TUI model created successfully")
	return m, nil
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	logging.Debug("Initializing TUI model...")

	// Start streaming in background
	logging.Debug("Starting streaming...")
	m.startStreaming()

	// Return initial commands
	logging.Debug("Setting up initial commands...")
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
		logging.Debug("Key pressed: %s", msg.String())

		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			m.cancel()
			return m, tea.Quit

		case key.Matches(msg, m.keys.Charts):
			logging.Debug("Charts key pressed, switching to charts tab")
			m.activeTab = TabCharts

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
		// Pass window size to all views
		m.listView.Update(msg)
		m.detailsView.Update(msg)
		m.logsView.Update(msg)
		m.chartsView.Update(msg)

	case tickMsg:
		// Update views with latest data
		nodes := m.aggregator.GetNodes()
		m.listView.SetNodes(nodes)
		// Update charts with latest snapshot
		snapshot := m.aggregator.Snapshot()
		m.chartsView.SetSnapshot(snapshot)

		// Continue ticking
		cmds = append(cmds, m.tick())

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
	case TabCharts:
		cmd := m.chartsView.Update(msg)
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
	case TabCharts:
		b.WriteString(m.chartsView.View())
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


// startStreaming starts the data streaming
func (m *Model) startStreaming() {
	logging.Debug("StartStreaming called with backend: %s", m.config.BackendAddr)

	// Check if we should use mock data
	if m.config.BackendAddr == "mock" {
		logging.Info("Using mock data stream consumer")
		// Use mock stream consumer for testing
		mockConsumer := data.NewMockStreamConsumer(m.aggregator)
		m.streamConsumer = mockConsumer

		if err := mockConsumer.Start(m.ctx); err != nil {
			logging.Error("Failed to start mock consumer: %v", err)
			m.err = err
		} else {
			logging.Debug("Mock consumer started successfully")
		}
	} else {
		// Connect to real backend
		logging.Info("Connecting to real backend at %s", m.config.BackendAddr)
		client, err := grpcclient.New(m.config.BackendAddr, m.config.BackendToken)
		if err != nil {
			logging.Error("Failed to create gRPC client: %v", err)
			m.err = err
			return
		}
		logging.Debug("gRPC client created successfully")

		// Create stream consumer
		logging.Debug("Creating stream consumer...")
		consumer := data.NewStreamConsumer(client.NodeService(), m.aggregator)
		m.streamConsumer = consumer

		logging.Debug("Starting stream consumer...")
		if err := consumer.Start(m.ctx); err != nil {
			logging.Error("Failed to start stream consumer: %v", err)
			m.err = err
			return
		}
		logging.Debug("Stream consumer started successfully")
	}

	// Start background event processor
	logging.Debug("Starting background event processor...")
	go m.processEventsBackground()
}

// processEventsBackground processes events in background
func (m *Model) processEventsBackground() {
	logging.Debug("ProcessEventsBackground goroutine started")

	if m.streamConsumer == nil {
		logging.Error("streamConsumer is nil, exiting processEventsBackground")
		return
	}

	// Create separate context for event processing
	eventCtx := m.ctx

	// Get channels once to avoid potential nil issues
	logging.Debug("Getting event and error channels from stream consumer")
	eventChan := m.streamConsumer.Events()
	errorChan := m.streamConsumer.Errors()

	logging.Debug("Event channel: %v, Error channel: %v", eventChan, errorChan)

	logging.Debug("Starting event processing loop...")
	eventCount := 0
	lastLog := time.Now()

	for {
		select {
		case <-eventCtx.Done():
			// Context cancelled, clean shutdown
			logging.Info("Event processing context cancelled, shutting down")
			return

		case event, ok := <-eventChan:
			if !ok {
				// Channel closed, exit cleanly
				logging.Info("Event channel closed, exiting event processor")
				return
			}
			logging.Debug("Received event from channel: %v", event)
			if event != nil {
				eventCount++
				logging.Debug("Processing event #%d, type=%v", eventCount, event.Type)
				if time.Since(lastLog) > 5*time.Second {
					logging.Debug("Processed %d events so far", eventCount)
					lastLog = time.Now()
				}

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
				logging.Info("Error channel closed")
				return
			}
			if err != nil {
				// Store error without blocking
				logging.Error("Stream error received: %v", err)
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
	logging.Info("Starting TUI application...")
	logging.Debug("Creating TUI model...")

	model, err := NewModel(config)
	if err != nil {
		logging.Error("Failed to create TUI model: %v", err)
		return err
	}
	defer func() {
		logging.Debug("Cleaning up TUI resources...")
		model.Cleanup()
	}()

	logging.Debug("Creating tea program...")
	p := tea.NewProgram(model, tea.WithAltScreen())

	logging.Info("Running tea program (this will block until exit)...")
	if _, err := p.Run(); err != nil {
		logging.Error("Tea program failed: %v", err)
		return err
	}

	logging.Info("TUI application finished successfully")
	return nil
}