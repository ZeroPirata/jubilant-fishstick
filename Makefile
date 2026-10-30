.PHONY: help
help: ## Display this help screen.
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available Targets:"
	@grep -E '^[a-zA-Z\/_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2}'


.PHONY: sqlc run test test-race

sqlc: ## Generate the SQLc file
	@sqlc generate
	@echo "geração dos modelos SQLc"

air: ## Run golang with air instance
	@echo "rodando air"
	@air

# Pacotes Go listados um a um: ./... entra em data/pgdata (volume do Postgres, sem
# permissão de leitura) e falha.
GO_PKGS := ./cmd/... ./config/... ./database/... ./internal/... ./repository/...

test: ## Run the Go tests
	go test $(GO_PKGS) -count=1 -timeout 120s

test-race: ## Run the Go tests with the race detector (needs gcc + musl-dev / CGO_ENABLED=1)
	CGO_ENABLED=1 go test $(GO_PKGS) -race -count=1 -timeout 600s

deps: go.mod ## Update the dependencies of the project
	@echo "--> Running go mod tidy..."
	go mod tidy
