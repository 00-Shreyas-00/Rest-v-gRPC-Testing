# Go REST and Go gRPC servers. One build stage, two runtime targets:
#   docker build --target rest -t go-rest .
#   docker build --target grpc -t go-grpc .
#
# Release build: CGO disabled (static binary), -trimpath, symbols stripped.
# Go binaries are always compiled with optimizations; there is no separate
# debug build to accidentally compare against.

FROM golang:1.26.3 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/rest-server ./cmd/rest-server && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/grpc-server ./cmd/grpc-server

# distroless/static-debian13:nonroot, pinned by digest.
FROM gcr.io/distroless/static-debian13:nonroot@sha256:25ac8f0c3b2334a9eb05452266bd4697864a8c8ef0ef71e5e6480d7f92868329 AS runtime-base
WORKDIR /app

# Health checks run every second only until the container is healthy, then
# once a minute, so they add no meaningful load during a benchmark.
FROM runtime-base AS rest
COPY --from=build /out/rest-server /app/rest-server
EXPOSE 8080
HEALTHCHECK --interval=60s --timeout=5s --start-period=60s --start-interval=1s --retries=3 \
    CMD ["/app/rest-server", "-healthcheck"]
ENTRYPOINT ["/app/rest-server"]

FROM runtime-base AS grpc
COPY --from=build /out/grpc-server /app/grpc-server
EXPOSE 9090
HEALTHCHECK --interval=60s --timeout=5s --start-period=60s --start-interval=1s --retries=3 \
    CMD ["/app/grpc-server", "-healthcheck"]
ENTRYPOINT ["/app/grpc-server"]
