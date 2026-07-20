# syntax=docker/dockerfile:1
# Build context must be the PARENT directory (contains dailybot/ and all-minilm-l6-v2-go/).
# Local:  docker build -f dailybot/Dockerfile -t bubblepulse-backend:local .
# CI:     context: .  file: dailybot/Dockerfile  (GHA checks both repos out as siblings)

ARG ONNX_VERSION=1.21.0
ARG GO_VERSION=1.25

# ─── Stage 1: Download ONNX Runtime (cached independently of source changes) ──
FROM debian:bookworm-slim AS onnx-downloader

ARG ONNX_VERSION
RUN apt-get update && apt-get install -y --no-install-recommends wget ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN wget -q "https://github.com/microsoft/onnxruntime/releases/download/v${ONNX_VERSION}/onnxruntime-linux-x64-${ONNX_VERSION}.tgz" \
    && tar -xzf "onnxruntime-linux-x64-${ONNX_VERSION}.tgz" \
    && mkdir -p /onnx/lib \
    && cp "onnxruntime-linux-x64-${ONNX_VERSION}/lib"/libonnxruntime*.so* /onnx/lib/ \
    && rm -rf "onnxruntime-linux-x64-${ONNX_VERSION}.tgz" "onnxruntime-linux-x64-${ONNX_VERSION}"

# ─── Stage 2: Build the Go binary ─────────────────────────────────────────────
FROM golang:${GO_VERSION}-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev \
    && rm -rf /var/lib/apt/lists/*

# Make libonnxruntime available for CGO linking
COPY --from=onnx-downloader /onnx/lib/ /usr/local/lib/
RUN ldconfig

WORKDIR /build

# Cache dependency downloads separately from source
COPY dailybot/go.mod dailybot/go.sum ./dailybot/
COPY all-minilm-l6-v2-go/ ./all-minilm-l6-v2-go/
RUN cd dailybot && go mod download

# Build
COPY dailybot/ ./dailybot/
ENV CGO_ENABLED=1
RUN cd dailybot && go build -ldflags="-s -w" -o /app/bubblepulse ./cmd/bubblepulse

# ─── Stage 3: Minimal runtime image ───────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# ONNX Runtime shared library
COPY --from=onnx-downloader /onnx/lib/ /usr/local/lib/
RUN ldconfig

WORKDIR /app

COPY --from=builder /app/bubblepulse ./
# Goose reads migrations from internal/db/migrations relative to WORKDIR
COPY --from=builder /build/dailybot/internal/db/migrations ./internal/db/migrations/

RUN useradd --system --uid 1000 --no-create-home appuser \
    && chown -R appuser:appuser /app
USER appuser

ENV ONNX_RUNTIME_PATH=/usr/local/lib/libonnxruntime.so

EXPOSE 8080
ENTRYPOINT ["./bubblepulse"]
