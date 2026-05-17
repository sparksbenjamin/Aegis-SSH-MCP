# syntax=docker/dockerfile:1.7

# Stage 1: build
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

# Security: run build as non-root
RUN addgroup -S aegis && adduser -S aegis -G aegis
WORKDIR /build

# Cache dependencies separately from source
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download && go mod verify

# Build a fully static binary for the requested target platform.
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build \
    -ldflags="-w -s -extldflags=-static" \
    -trimpath \
    -o /out/aegis-ssh-mcp \
    .

# Stage 2: distroless runtime
FROM gcr.io/distroless/static-debian12:nonroot

# Copy binary
COPY --from=builder /out/aegis-ssh-mcp /usr/local/bin/aegis-ssh-mcp

# These directories are expected at runtime and are usually bind-mounted.
COPY --from=builder --chown=65532:65532 /build/configs /configs
COPY --from=builder --chown=65532:65532 /build/rules /rules

ENV AEGIS_CONFIGS_DIR=/configs \
    AEGIS_RULES_DIR=/rules

USER nonroot

ENTRYPOINT ["/usr/local/bin/aegis-ssh-mcp"]
