#!/bin/bash

set -e

BASEDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

SDK_DIR="$BASEDIR/../sentio-sdk"

if [ -d "$SDK_DIR" ]; then
  echo "Sync Proto and Gen TS to SDK folder"
  bazel run //sentio-sdk:write_gen

  sed -i '' -e 's/Function.fromPartial(base ?? {});/Function.fromPartial(base ?? {} as any);/g' $SDK_DIR/packages/protos/src/service/common/protos/common.ts
  sed -i '' -e 's/Function.fromPartial(base ?? {});/Function.fromPartial(base ?? {} as any);/g' $SDK_DIR/packages/runtime/src/gen/service/common/protos/common.ts

else
  echo "SDK directory not existed, please clone the sdk repository into this repo (sentio-core/sentio-sdk)"
fi
