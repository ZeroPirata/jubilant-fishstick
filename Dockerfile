# syntax=docker/dockerfile:1
FROM golang:1.26.2-alpine3.23 AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/root/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o app ./cmd/server

# ── Runtime ───────────────────────────────────────────────────────────────────
FROM alpine:3.23

# Deps + pip + permissões de SO — camada pesada, só roda quando mudar dependências.
RUN --mount=type=cache,target=/root/.cache/pip \
    apk add --no-cache \
        python3 \
        py3-pip \
        ca-certificates \
        pango \
        cairo \
        gdk-pixbuf \
        fontconfig \
        ttf-liberation \
        font-noto \
        libffi \
        libjpeg-turbo \
        zlib \
        shared-mime-info \
        harfbuzz \
        fribidi \
        su-exec \
    && pip install --break-system-packages weasyprint \
    && fc-cache -fv \
    && chmod -R a+rX /etc/fonts /usr/share/fonts /usr/share/fontconfig 2>/dev/null || true \
    && mkdir -p /var/cache/fontconfig \
    && chmod a+rwx /var/cache/fontconfig \
    && chmod a+rx /usr/bin/python3.12 /usr/bin/python3 \
    && find /usr/lib/python3* -type d -exec chmod a+rx {} \; \
    && find /usr/lib/python3* -type f -exec chmod a+r {} \; \
    && find /usr/lib -name "*.so*" -exec chmod a+r {} \;

RUN adduser -D -u 1001 appuser && mkdir -p /app/output

WORKDIR /app

COPY --from=builder /app/app .
COPY --from=builder /app/static ./static
COPY --from=builder /app/scripts ./scripts
COPY --from=builder /app/internal/worker/prompts ./internal/worker/prompts
COPY entrypoint.sh ./entrypoint.sh

RUN chown -R appuser:appuser /app && chmod +x /app/entrypoint.sh

ENV PYTHONUNBUFFERED=1

ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["./app"]
