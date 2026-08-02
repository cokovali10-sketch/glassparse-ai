# Multi-stage build for the GlassParse AI Google engine.
#
# Stage 1 — compile a static binary (mirrors go.mod's `go 1.22` language level).
#   CGO_ENABLED=0 → fully static, runs on plain alpine / scratch.
# Stage 2 — copy only the binary into a tiny, non-root alpine image (<20 MB).
#
# Build:    docker build -t glassparse-engine:latest ./engine
# Run:      docker run --rm -p 8080:8080 glassparse-engine:latest

# ---------- builder ----------
FROM golang:1.22-alpine AS builder

# ca-certificates are linked into the final image from here (HTTPS fetches).
RUN apk add --no-cache ca-certificates

WORKDIR /src

# Cache modules independently of source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/glassparse-server ./cmd/server

# ---------- runtime ----------
FROM alpine:latest AS runtime

# Non-root user; a numbered UID works on any runner (OpenShift-safe).
RUN adduser -D -u 10001 glassparse \
    && apk add --no-cache ca-certificates \
    && rm -rf /var/cache/apk/*

WORKDIR /app

COPY --from=builder /out/glassparse-server /app/glassparse-server

USER glassparse

# HTTP API exposed by the server.
EXPOSE 8080

ENV GLASSPARSE_ADDR="0.0.0.0:8080" \
    GLASSPARSE_TIMEOUT_S="45"

# Signals are forwarded; run the server directly (PID 1 sane init).
ENTRYPOINT ["/app/glassparse-server"]
CMD ["-addr", "0.0.0.0:8080"]