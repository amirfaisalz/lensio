# ==============================================================================
# Stage 1: Build binary with official Go compiler
# ==============================================================================
FROM golang:alpine AS builder

WORKDIR /src

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-w -s -extldflags '-static'" \
    -o /bin/lensio-api \
    ./apps/api/cmd/server

# ==============================================================================
# Stage 2: Minimal hardened non-root runtime
# ==============================================================================
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata && \
    apk upgrade --no-cache && \
    addgroup -g 65532 -S appgroup && \
    adduser -u 65532 -S appuser -G appgroup -s /sbin/nologin

WORKDIR /app

COPY --from=builder --chown=65532:65532 /bin/lensio-api /app/lensio-api

EXPOSE 8080

USER 65532:65532

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/lensio-api"]
