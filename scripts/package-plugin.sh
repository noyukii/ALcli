#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output="${1:-$root/dist/alcli-plugin.zip}"
mkdir -p "$(dirname "$output")"
(cd "$root" && zip -q -r "$output" plugin)
echo "$output"
