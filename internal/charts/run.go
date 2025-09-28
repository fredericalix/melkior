package charts

import (
	"context"
	"fmt"
	"time"

	"github.com/melkior/nodestatus/internal/data"
	"github.com/mum4k/termdash"
	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/container"
	"github.com/mum4k/termdash/container/grid"
	"github.com/mum4k/termdash/keyboard"
	"github.com/mum4k/termdash/linestyle"
	"github.com/mum4k/termdash/terminal/terminalapi"
	"github.com/mum4k/termdash/terminal/tcell"
)

// Options configures the chart display
type Options struct {
	RefreshInterval time.Duration
	ShowHelp        bool
}

// RunTermdash runs the full-screen termdash charts
func RunTermdash(ctx context.Context, provider data.SnapshotProvider, opts Options) error {
	// Create terminal
	term, err := tcell.New()
	if err != nil {
		return fmt.Errorf("failed to create terminal: %w", err)
	}
	defer term.Close()

	// Create widgets
	widgets, err := NewWidgets()
	if err != nil {
		return fmt.Errorf("failed to create widgets: %w", err)
	}

	// Create container with grid layout
	c, err := createLayout(term, widgets)
	if err != nil {
		return fmt.Errorf("failed to create layout: %w", err)
	}

	// Create controller context with cancel
	controllerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Keyboard handler
	keyHandler := func(k *terminalapi.Keyboard) {
		switch k.Key {
		case keyboard.KeyEsc, 'q', 'Q':
			cancel()
		case '?':
			// Toggle help (optional)
		}
	}

	// Start update loop in background
	updateDone := make(chan struct{})
	go func() {
		updateLoop(controllerCtx, widgets, provider, opts.RefreshInterval)
		close(updateDone)
	}()

	// Run termdash with keyboard subscriber
	err = termdash.Run(controllerCtx, term, c,
		termdash.KeyboardSubscriber(keyHandler),
		termdash.RedrawInterval(opts.RefreshInterval),
	)
	if err != nil {
		return fmt.Errorf("failed to run termdash: %w", err)
	}

	// Wait for update loop to finish
	<-updateDone

	// Run until context is cancelled
	<-controllerCtx.Done()

	return nil
}

// createLayout creates the grid layout for widgets
func createLayout(term terminalapi.Terminal, w *Widgets) (*container.Container, error) {
	// Define grid layout
	builder := grid.New()
	builder.Add(
		// Row 1: Donut (left) | Bar Chart (right)
		grid.RowHeightPerc(40,
			grid.ColWidthPerc(50,
				grid.Widget(w.statusBar,
					container.Border(linestyle.Light),
					container.BorderTitle("Status Distribution"),
					container.BorderColor(cell.ColorWhite),
				),
			),
			grid.ColWidthPerc(50,
				grid.Widget(w.typeBar,
					container.Border(linestyle.Light),
					container.BorderTitle("Node Types"),
					container.BorderColor(cell.ColorWhite),
				),
			),
		),
		// Row 2: Time Series
		grid.RowHeightPerc(40,
			grid.Widget(w.timeSeries,
				container.Border(linestyle.Light),
				container.BorderTitle("Status Over Time"),
				container.BorderColor(cell.ColorWhite),
			),
		),
		// Row 3: Gauges and Help
		grid.RowHeightPerc(20,
			grid.ColWidthPerc(33,
				grid.Widget(w.epsGauge,
					container.Border(linestyle.Light),
					container.BorderTitle("Events/sec"),
					container.BorderColor(cell.ColorWhite),
				),
			),
			grid.ColWidthPerc(33,
				grid.Widget(w.mutationGauge,
					container.Border(linestyle.Light),
					container.BorderTitle("Mutations/sec"),
					container.BorderColor(cell.ColorWhite),
				),
			),
			grid.ColWidthPerc(34,
				grid.Widget(w.helpText,
					container.Border(linestyle.Light),
					container.BorderTitle("Help"),
					container.BorderColor(cell.ColorGray),
				),
			),
		),
	)

	// Create container
	gridOpts, err := builder.Build()
	if err != nil {
		return nil, err
	}

	c, err := container.New(term, gridOpts...)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// updateLoop continuously updates widgets
func updateLoop(ctx context.Context, w *Widgets, provider data.SnapshotProvider, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial update
	snap := provider.Snapshot()
	w.UpdateAll(ctx, snap)

	// Subscribe for push updates if available
	var updateChan <-chan data.MetricsSnapshot
	if sub, ok := provider.(interface {
		Subscribe() <-chan data.MetricsSnapshot
	}); ok {
		updateChan = sub.Subscribe()
		defer func() {
			if unsub, ok := provider.(interface {
				Unsubscribe(<-chan data.MetricsSnapshot)
			}); ok {
				unsub.Unsubscribe(updateChan)
			}
		}()
	}

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			// Pull latest snapshot
			snap := provider.Snapshot()
			if err := w.UpdateAll(ctx, snap); err != nil {
				// Log error but continue
				continue
			}

		case snap := <-updateChan:
			// Handle push update if available
			if err := w.UpdateAll(ctx, snap); err != nil {
				continue
			}
		}
	}
}