#!/usr/bin/env bash
#MISE description="Run golangci-lint and shellcheck on the codebase"
#MISE depends=["dev:fmt"]
set -euo pipefail

golangci-lint run ./...
find . -name "*.sh" -type f -exec shellcheck {} +
