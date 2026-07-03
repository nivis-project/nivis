#!/usr/bin/env bash
# Regenerate the tfplugin6 gRPC stubs from proto/tfplugin6.proto.
#
# The proto is a verbatim copy of terraform-plugin-go's
# tfprotov6/internal/tfplugin6/tfplugin6.proto (protocol v6.11), with go_package
# overridden to this module's internal path. The upstream stubs live in an
# internal package we cannot import (see beans nixform2-uu26), and the .proto
# explicitly instructs downstreams to copy + generate — which is what this does.
#
# Toolchain (verified 2026-06): protoc from nixpkgs; the two Go gen plugins via
# `go install`. Run from the repo root: bash proto/generate.sh
#
# NOTE: the .proto `go_package` tracks this module's path. If the module path
# changes (e.g. the 2026-07 move to github.com/nivis-project/nivis) without protoc
# on hand, the committed *.pb.go keep the OLD path embedded in their file
# descriptor (rawDesc) — harmless at runtime (go_package does not affect
# execution), and this script regenerates them to match on the next run. Do NOT
# hand-edit the rawDesc string: its length prefix is byte-encoded and a text
# replace corrupts the descriptor (a proto-init panic).
set -euo pipefail

cd "$(dirname "$0")/.."

# protoc from the nix store (no PATH assumption).
PROTOC="$(nix build nixpkgs#protobuf --no-link --print-out-paths)/bin/protoc"

# Ensure the Go codegen plugins are installed and on PATH.
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
export PATH="$(go env GOPATH)/bin:$PATH"

"$PROTOC" \
  --proto_path=proto \
  --go_out=. --go_opt=module=github.com/wearetechnative/nivis \
  --go-grpc_out=. --go-grpc_opt=module=github.com/wearetechnative/nivis \
  proto/tfplugin6.proto proto/tfplugin5.proto

echo "Generated internal/tfplugin6/{tfplugin6.pb.go,tfplugin6_grpc.pb.go}"
echo "Generated internal/tfplugin5/{tfplugin5.pb.go,tfplugin5_grpc.pb.go}"
