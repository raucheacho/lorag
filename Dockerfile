# VectixDB is a private Go module: pass a GitHub token that can read it as a
# build secret (it stays out of the image layers):
#   GITHUB_TOKEN=$(gh auth token) docker build --secret id=github_token,env=GITHUB_TOKEN -t lorag .

# UI
FROM node:20-alpine AS ui
WORKDIR /ui
COPY ui/package*.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

# Binaries (the VectixDB module carries its prebuilt static library)
FROM golang:1.24-bookworm AS builder
ENV GOPRIVATE=github.com/raucheacho/*
WORKDIR /src/lorag
COPY go.mod go.sum ./
RUN --mount=type=secret,id=github_token,required=true \
    GIT_CONFIG_COUNT=1 \
    GIT_CONFIG_KEY_0="url.https://x-access-token:$(cat /run/secrets/github_token)@github.com/.insteadOf" \
    GIT_CONFIG_VALUE_0=https://github.com/ \
    go mod download
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
