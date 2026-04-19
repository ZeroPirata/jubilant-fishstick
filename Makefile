.PHONY: help
help: ## Display this help screen.
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available Targets:"
	@grep -E '^[a-zA-Z\/_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2}'


.PHONY: sqlc run

sqlc: ## Generate the SQLc file
	@sqlc generate
	@echo "geração dos modelos SQLc"

air: ## Run golang with air instance
	@echo "rodando air"
	@air

deps: go.mod ## Update the dependencies of the project
	@echo "--> Running go mod tidy..."
	go mod tidy
