FROM golang:1.25

WORKDIR /workspaces/hackton-treino

RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 \
    python3-pip \
    python3-venv \
    && rm -rf /var/lib/apt/lists/*

RUN go install github.com/air-verse/air@v1.61.5 && \
    go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 && \
    go install golang.org/x/tools/cmd/goimports@v0.28.0 && \
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8

CMD ["sleep", "infinity"]