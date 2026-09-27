#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/release/skills/alcli" "$work/archive/skills/alcli" "$work/home"
cp "$root/plugin/skills/alcli/SKILL.md" "$work/archive/skills/alcli/SKILL.md"
go build -o "$work/archive/anilist" "$root"
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$(uname -m)" in arm64|aarch64) arch=arm64 ;; x86_64|amd64) arch=amd64 ;; esac
archive="alcli_${os}_${arch}.tar.gz"
tar -czf "$work/release/$archive" -C "$work/archive" anilist skills/alcli/SKILL.md
if command -v shasum >/dev/null; then hash="$(shasum -a 256 "$work/release/$archive" | awk '{print $1}')"; else hash="$(sha256sum "$work/release/$archive" | awk '{print $1}')"; fi
printf '%s  %s\n' "$hash" "$archive" > "$work/release/checksums.txt"
cp "$root/install.sh" "$work/install.sh"
export ALCLI_RELEASE_BASE_URL="file://$work/release"
export CODEX_HOME="$work/home/codex"
"$work/install.sh" --yes --prefix "$work/home/bin" --with-skill codex
"$work/home/bin/al" help >/dev/null
cmp "$root/plugin/skills/alcli/SKILL.md" "$CODEX_HOME/skills/alcli/SKILL.md"
"$work/install.sh" --yes --prefix "$work/home/bin" --with-skill codex
"$work/install.sh" --yes --prefix "$work/home/bin" --uninstall
[[ ! -e "$work/home/bin/anilist" && ! -e "$CODEX_HOME/skills/alcli/SKILL.md" ]]
"$work/install.sh" --yes --prefix "$work/home/bin" --with-skill codex >/dev/null
printf '%s\n' 'user modification' >> "$CODEX_HOME/skills/alcli/SKILL.md"
"$work/install.sh" --yes --prefix "$work/home/bin" --uninstall >/dev/null
grep -q 'user modification' "$CODEX_HOME/skills/alcli/SKILL.md"
rm -rf "$CODEX_HOME/skills/alcli"
mkdir -p "$CODEX_HOME/skills/alcli"
printf '%s\n' 'user-owned skill' > "$CODEX_HOME/skills/alcli/SKILL.md"
if "$work/install.sh" --yes --prefix "$work/home/bin" --with-skill codex >/dev/null 2>&1; then
  echo 'Installer replaced an unmanaged skill' >&2
  exit 1
fi
[[ "$(cat "$CODEX_HOME/skills/alcli/SKILL.md")" == 'user-owned skill' ]]
rm -rf "$CODEX_HOME/skills/alcli"
printf '%s  %s\n' "$(printf '0%.0s' {1..64})" "$archive" > "$work/release/checksums.txt"
if "$work/install.sh" --yes --prefix "$work/home/bin" >/dev/null 2>&1; then
  echo 'Installer accepted a bad checksum' >&2
  exit 1
fi
if (cd "$root" && cat install.sh | bash -s -- --yes --prefix "$work/home/bin") >/dev/null 2>&1; then
  echo 'Piped installer bypassed release verification inside a checkout' >&2
  exit 1
fi
[[ ! -e "$work/home/bin/anilist" ]]
printf '%s\n' 'Installer release checksum, repeat install, skill, and uninstall checks passed.'
