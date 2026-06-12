#!/bin/bash

set -e

BASEDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$BASEDIR/.." && pwd)"

# Path where the bazel target //sentio-sdk:write_gen expects the SDK checkout.
SDK_MOUNT="$REPO_ROOT/sentio-sdk"

# Source SDK checkout. Defaults to the sibling repo next to sentio-core
# (i.e. the workspace layout sentio-ws/{sentio-core,sentio-sdk}).
SDK_SRC="${SDK_SRC:-$REPO_ROOT/../sentio-sdk}"

CREATED_SYMLINK=0

cleanup() {
  if [ "$CREATED_SYMLINK" = "1" ] && [ -L "$SDK_MOUNT" ]; then
    echo "Removing temporary sentio-sdk symlink"
    rm -f "$SDK_MOUNT"
  fi
}
trap cleanup EXIT

if [ -d "$SDK_MOUNT" ]; then
  # A checkout (or a symlink the user set up themselves) is already mounted; use it as-is.
  echo "Using existing sentio-sdk at $SDK_MOUNT"
elif [ -d "$SDK_SRC" ]; then
  # Auto-mount the sibling SDK repo via a temporary symlink so bazel can resolve
  # //sentio-sdk:write_gen, then remove it on exit.
  rm -f "$SDK_MOUNT" # clear a dangling symlink from an interrupted run, if any
  ln -s "$(cd "$SDK_SRC" && pwd)" "$SDK_MOUNT"
  CREATED_SYMLINK=1
  echo "Linked sibling SDK $(readlink "$SDK_MOUNT") -> $SDK_MOUNT"
else
  echo "sentio-sdk not found. Expected a checkout at $SDK_MOUNT or a sibling repo at $SDK_SRC (override with SDK_SRC=/path)." >&2
  exit 1
fi

echo "Sync Proto and Gen TS to SDK folder"
bazel run //sentio-sdk:write_gen

# NOTE: common.proto imports the grpc-gateway openapiv2 options (used only as
# MethodOptions/JSONSchema extensions, never as field types). The es-proto-plugin
# (see //bazel/protoc-gen-es, driven by the es_proto rule's strip_imports
# attr) drops those options-only import dependencies from the generated descriptor at
# codegen time, so common_pb.ts has no openapiv2 import and no post-processing is needed.
#
# The generated *_pb.ts are listed in the sentio-sdk .prettierignore (they are
# machine-generated), so this script emits them verbatim — no formatting pass here.
