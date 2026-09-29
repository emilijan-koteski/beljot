# syntax=docker/dockerfile:1.7

FROM golang:1.26-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
# -tags nodynamic: the WebP encoder (gen2brain/webp) otherwise links purego to
# try a system libwebp first, which turns this CGO-free build into a
# dynamically linked binary. With the tag it stays static and always uses the
# encoder's pure-Go (transpiled) build.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -tags nodynamic -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 1000 beljot
WORKDIR /app
COPY --from=builder /out/api /app/api
USER beljot
ENV BELJOT_PORT=8080
# Tell the Go runtime about the 256 MiB container limit: past 200 MiB the GC
# works harder instead of letting garbage pile up to twice the live heap.
ENV GOMEMLIMIT=200MiB
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -qO- "http://127.0.0.1:${BELJOT_PORT}/healthz" >/dev/null || exit 1
ENTRYPOINT ["/app/api"]
