FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install certs if your app makes HTTPS requests
RUN apk add --no-cache ca-certificates

# Copy module files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source
COPY . .

# Build a static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/bidder ./bidder

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /app/server /app/server

EXPOSE 8080

USER 65532:65532

ENTRYPOINT ["/app/server"]