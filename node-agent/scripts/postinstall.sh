#!/bin/bash
set -e

echo "Setting up RCA Node Agent..."
systemctl daemon-reload
systemctl enable rca-node-agent
echo "RCA Node Agent installed. Start it with: systemctl start rca-node-agent"
