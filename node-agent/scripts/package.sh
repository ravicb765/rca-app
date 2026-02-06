#!/bin/bash
# RCA Node Agent Packaging Orchestrator
set -e

echo "🔨 Building RCA Node Agent for Linux/amd64..."
make build-amd64

echo "📦 Generating Debian package..."
make package-deb

echo "📦 Generating RPM package..."
make package-rpm

echo "📦 Generating Legacy Tarball..."
make package

echo "✨ All packages generated successfully!"
ls -lh *.deb *.rpm *.tar.gz
