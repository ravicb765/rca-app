#!/bin/bash

# RCA Node Agent Installation Script for Linux VMs
# Usage: sudo ./install.sh

set -e

INSTALL_DIR="/opt/rca-app/node-agent"
SERVICE_FILE="rca-node-agent.service"

echo "🚀 Starting RCA Node Agent installation..."

# 1. Create directory structure
mkdir -p "$INSTALL_DIR"
mkdir -p "$INSTALL_DIR/ebpf"

# 2. Copy binary and eBPF objects
if [ -f "node-agent" ]; then
    cp node-agent "$INSTALL_DIR/"
    chmod +x "$INSTALL_DIR/node-agent"
else
    echo "❌ Error: node-agent binary not found in current directory."
    exit 1
fi

if [ -f "ebpf/network_tracer.o" ]; then
    cp ebpf/network_tracer.o "$INSTALL_DIR/ebpf/"
else
    echo "⚠️ Warning: ebpf/network_tracer.o not found. Tracer may not function."
fi

# 3. Install systemd service
if [ -f "$SERVICE_FILE" ]; then
    cp "$SERVICE_FILE" /etc/systemd/system/
    systemctl daemon-reload
    systemctl enable rca-node-agent
    echo "✅ Systemd service installed and enabled."
else
    echo "❌ Error: $SERVICE_FILE not found."
    exit 1
fi

echo "✨ Installation complete!"
echo "To start the agent: sudo systemctl start rca-node-agent"
echo "To view logs: journalctl -u rca-node-agent -f"
