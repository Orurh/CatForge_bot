#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repo_dir}"

protoc -I api \
  --go_out=. --go_opt=module=catforge \
  --go-grpc_out=. --go-grpc_opt=module=catforge \
  api/gameengine/v1/game_engine.proto

gofmt -w internal/gen/gameengine/v1/*.go
