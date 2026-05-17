# ─── Stage 1: Build ──────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

# Security: run build as non-root
RUN addgroup -S aegis && adduser -S aegis -G aegis
WORKDIR /build

# Cache dependencies separately from source
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Build a fully static binary (CGO disabled — no libc required)
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags="-w -s -extldflags=-static" \
    -trimpath \
    -o aegis-ssh-mcp \
    ./main.go

# ─── Stage 2: Distroless runtime ─────────────────────────────────────────────
# gcr.io/distroless/static-debian12 contains:
#   - CA certificates  (needed for TLS)
#   - /etc/passwd      (needed for USER directive)
#   - No shell, no package manager, no utilities = minimal attack surface
FROM gcr.io/distroless/static-debian12:nonroot

# Copy binary
COPY --from=builder /build/aegis-ssh-mcp /usr/local/bin/aegis-ssh-mcp

# These directories must be provided via Docker volumes at runtime.
# They are created here so the image layer has the mount points.
# Permissions 0750 — readable by the aegis user (uid 65532 in distroless:nonroot)
COPY --from=builder --chown=65532:65532 /build/configs /configs
COPY --from=builder --chown=65532:65532 /build/rules   /rules

# Runtime environment
ENV AEGIS_CONFIGS_DIR=/configs \
    AEGIS_RULES_DIR=/rules

# MCP communicates over stdio — no ports need to be exposed.
# (Uncomment if adding an HTTP transport in a future iteration.)
# EXPOSE 8080

# Run as nonroot (uid 65532) — distroless nonroot image default
USER nonroot

ENTRYPOINT ["/usr/local/bin/aegis-ssh-mcp"]
