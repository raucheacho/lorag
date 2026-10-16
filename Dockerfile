# Build with the VectixDB checkout as a named context:
#   docker build --build-context vectixdb=../vectixdb -t lorag .

# UI
FROM node:20-alpine AS ui
WORKDIR /ui
COPY ui/package*.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

# Engine and binaries
FROM golang:1.24-bookworm AS builder
ARG ZIG_VERSION=0.16.0
RUN apt-get update && apt-get install -y --no-install-recommends xz-utils \
    && rm -rf /var/lib/apt/lists/* \
    && curl -fsSL "https://ziglang.org/download/${ZIG_VERSION}/zig-$(uname -m)-linux-${ZIG_VERSION}.tar.xz" \
       | tar -xJ -C /opt \
    && ln -s "/opt/zig-$(uname -m)-linux-${ZIG_VERSION}/zig" /usr/local/bin/zig

# go.mod points the binding at ../vectixdb/bindings/go.
COPY --from=vectixdb . /src/vectixdb
RUN cd /src/vectixdb && rm -rf zig-out .zig-cache bindings/go/lib && ./scripts/go-libs.sh

WORKDIR /src/lorag
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui /ui/dist ./ui/dist
ARG VERSION=dev
RUN CGO_ENABLED=1 go build -ldflags="-s -w -X main.Version=${VERSION}" -o /out/lorag ./cmd/lorag \
    && CGO_ENABLED=1 go build -ldflags="-s -w" -o /out/loragctl ./cmd/loragctl

# Runtime
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /data
COPY --from=builder /out/lorag /out/loragctl /usr/local/bin/

ENV LORAG_DATA_DIR=/data \
    LORAG_HOST=0.0.0.0 \
    LORAG_PORT=8080
EXPOSE 8080
VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["lorag"]
CMD ["server"]
