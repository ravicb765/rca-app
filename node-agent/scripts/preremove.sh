#!/bin/bash
set -e

echo "Stopping RCA Node Agent..."
systemctl stop rca-node-agent || true
systemctl disable rca-node-agent || true
