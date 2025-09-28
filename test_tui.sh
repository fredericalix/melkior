#!/bin/bash

# Test TUI with mock data
echo "Starting TUI with mock backend..."
echo "Press 'q' to quit, 'c' for charts, arrow keys to navigate"
echo ""

export BACKEND_ADDR=mock
export FPS=10
export CHARTS_REFRESH=1s
export WINDOW_SECS=60

./nodectl