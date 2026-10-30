#!/bin/bash
set -e

echo "Instalando ferramentas Go..."

tools=(
    "golang.org/x/tools/cmd/goimports@latest"
    "github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
    "github.com/air-verse/air@latest"
    "github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1"
    "github.com/go-delve/delve/cmd/dlv@latest"
)

for tool in "${tools[@]}"; do
    echo "→ $tool"
    go install "$tool"
done

echo "✅ Ferramentas instaladas com sucesso!"