FROM golang:1.26.2-alpine3.23

WORKDIR /workspace

RUN apk add --no-cache \
    python3 \
    py3-pip \
    bash \
    git \
    ca-certificates \
    make \
    pango \
    cairo \
    gdk-pixbuf \
    fontconfig \
    ttf-liberation \
    py3-cffi \
    py3-brotli \
    libffi-dev \
    jpeg-dev \
    zlib-dev \
    && pip install weasyprint --break-system-packages

ARG UID=1000
RUN adduser -D -u ${UID} devuser

ENV GOMODCACHE=/go/pkg/mod
ENV GOCACHE=/home/devuser/.cache/go-build

RUN mkdir -p /go/pkg/mod /go/pkg/sumdb /home/devuser/.cache/go-build
RUN chown -R devuser:devuser /go /home/devuser

USER devuser

CMD ["sleep", "infinity"]