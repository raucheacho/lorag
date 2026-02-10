# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc g++ libc-dev

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binaries
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o bin/lorag ./cmd/lorag
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o bin/loragctl ./cmd/loragctl

# UI build stage
FROM node:20-alpine AS ui-builder

WORKDIR /ui
COPY web/package*.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /build/bin/lorag /usr/local/bin/lorag
COPY --from=builder /build/bin/loragctl /usr/local/bin/loragctl

# Copy UI build
COPY --from=ui-builder /ui/dist ./ui/dist

# Create data directory
RUN mkdir -p /data

# Environment variables
ENV LORAG_DATA_DIR=/data
ENV LORAG_HOST=0.0.0.0
ENV LORAG_PORT=8080

# Expose port
EXPOSE 8080

# Volume for data
VOLUME ["/data"]

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run server
ENTRYPOINT ["lorag"]
CMD ["server"]
