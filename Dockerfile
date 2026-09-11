FROM golang:alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /nusaid-api ./apps/api/cmd/server

FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /nusaid-api /app/nusaid-api

EXPOSE 8080
USER nobody:nobody

ENTRYPOINT ["/app/nusaid-api"]
