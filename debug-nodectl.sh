#!/bin/bash

echo "=== DEBUGGING NODECTL ==="
echo "Starting nodectl with mock backend..."
echo ""
echo "Log file will be created in ~/.nodectl/logs/"
echo ""
echo "To watch logs in another terminal, run:"
echo "  tail -f ~/.nodectl/logs/nodectl_*.log"
echo ""
echo "Press Ctrl+C to stop nodectl"
echo ""
echo "Starting in 3 seconds..."
sleep 3

# Start nodectl with mock backend
BACKEND_ADDR=mock ./nodectl