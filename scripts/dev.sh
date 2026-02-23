#!/usr/bin/env bash
set -euo pipefail

echo "Recommended workflow"
echo "  make start"
echo "  make status"
echo "  make stop"
echo "  make restart"

echo "Manual fallback for frontend"
echo "  cd frontend"
echo "  npm install"
echo "  npm run dev"

echo "Manual fallback for backend"
echo "  cd backend"
echo "  go run ./cmd/plato"
