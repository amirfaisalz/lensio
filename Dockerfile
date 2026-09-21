# ==============================================================================
# Stage 1: Build binary with official Go compiler
# ==============================================================================
# Pinned to the toolchain in go.mod. An unpinned `golang:alpine` silently
# changes compiler version between builds, so the image is not reproducible.
FROM golang:1.26-alpine AS builder

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
# Stage 2: Distroless static non-root runtime (Zero shells, zero package managers)
# ==============================================================================
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder --chown=nonroot:nonroot /bin/lensio-api /app/lensio-api

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/app/lensio-api"]
