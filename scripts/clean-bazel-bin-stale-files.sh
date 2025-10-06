#!/bin/bash
set -euo pipefail

echo "=== Cleaning bazel-bin/app of stale files ==="

rm -rf /home/sentio/.cache/bazel/_bazel_sentio/*/execroot/_main/bazel-out/*/bin/app/next-env.d.ts || true

BASEDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$BASEDIR/.."

BIN="bazel-bin/app"
SRC="app"

while IFS= read -r -d '' d; do
  name="$(basename "$d")"

  [[ "$name" == ".next" ]] && continue
  [[ "$name" == "node_modules" ]] && continue
  [[ -d "$SRC/$name" ]] || continue

  rsync -ai --dry-run --delete "$SRC/$name/" "$BIN/$name/" \
    | awk -v n="$name" '
        /^\*deleting/ {
          sub(/^\*deleting[[:space:]]+/, "", $0);  # Remove "*deleting" and any whitespace
          print n "/" $0
        }
      ' \
    | while IFS= read -r rel; do
      path="$BIN/$rel"
      if [[ -e "$path" || -L "$path" ]]; then
        echo "deleting $rel"
      fi
      rm -rf -- "$path"
    done
done < <(find "$BIN" -mindepth 1 -maxdepth 1 -type d -print0)

echo "=== Cleaning done ==="
