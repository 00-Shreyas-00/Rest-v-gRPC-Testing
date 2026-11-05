#!/usr/bin/env bash
# Regenerates go-server/gen/productv1 from proto/product.proto.
#
# Runs inside the pinned Go toolchain container, so no local protoc or Go is
# needed. Generated code is committed: building the servers never requires
# this script, only changing the .proto does.
set -euo pipefail

cd "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

PROTOC_GEN_GO_VERSION=v1.36.11
PROTOC_GEN_GO_GRPC_VERSION=v1.6.1

docker compose run --rm --no-deps go sh -euc "
  apt-get update -qq >/dev/null
  apt-get install -y -qq --no-install-recommends protobuf-compiler libprotobuf-dev >/dev/null
  go install google.golang.org/protobuf/cmd/protoc-gen-go@${PROTOC_GEN_GO_VERSION}
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@${PROTOC_GEN_GO_GRPC_VERSION}
  echo \"\$(protoc --version), protoc-gen-go ${PROTOC_GEN_GO_VERSION}, protoc-gen-go-grpc ${PROTOC_GEN_GO_GRPC_VERSION}\"
  rm -rf gen/productv1 && mkdir -p gen/productv1
  protoc -I /proto -I /usr/include \
    --go_out=gen/productv1 --go_opt=paths=source_relative \
    --go-grpc_out=gen/productv1 --go-grpc_opt=paths=source_relative \
    /proto/product.proto
  ls gen/productv1
"
