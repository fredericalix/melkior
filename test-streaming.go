package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/melkior/nodestatus/internal/data"
	"github.com/melkior/nodestatus/internal/logging"
)

func main() {
	// Initialize logging
	homeDir, _ := os.UserHomeDir()
	logDir := filepath.Join(homeDir, ".nodectl", "logs")
	if err := logging.Init(logDir, true); err != nil {
		fmt.Printf("Warning: Failed to initialize logging: %v\n", err)
	}
	defer logging.Close()

	logging.Info("=== TEST STREAMING STARTED ===")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create aggregator
	aggregator := data.NewAggregator(300)

	// Create mock stream consumer
	logging.Info("Creating MockStreamConsumer...")
	consumer := data.NewMockStreamConsumer(aggregator)

	// Start consumer
	logging.Info("Starting MockStreamConsumer...")
	if err := consumer.Start(ctx); err != nil {
		log.Fatal("Failed to start consumer:", err)
	}

	// Get channels
	logging.Info("Getting event and error channels...")
	eventChan := consumer.Events()
	errorChan := consumer.Errors()

	logging.Info("Event channel: %v", eventChan)
	logging.Info("Error channel: %v", errorChan)

	// Process events for 5 seconds
	logging.Info("Processing events for 5 seconds...")
	timeout := time.After(5 * time.Second)
	eventCount := 0

	for {
		select {
		case <-timeout:
			logging.Info("Timeout reached, received %d events", eventCount)
			return

		case event, ok := <-eventChan:
			if !ok {
				logging.Info("Event channel closed")
				return
			}
			if event != nil {
				eventCount++
				logging.Info("Received event #%d: Type=%v, Node=%s",
					eventCount, event.Type, event.Node.ID)
			}

		case err, ok := <-errorChan:
			if !ok {
				logging.Info("Error channel closed")
				return
			}
			if err != nil {
				logging.Error("Received error: %v", err)
			}

		default:
			// Non-blocking
			time.Sleep(10 * time.Millisecond)
		}
	}
}