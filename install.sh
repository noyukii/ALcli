#!/usr/bin/env bash
# Install ALcli from a checksum-verified GitHub Release, or build a local checkout.
set -euo pipefail

REPO="noyukii/ALcli"
PREFIX="${ALCLI_PREFIX:-${HOME}/.local/bin}"
YES=0
UNINSTALL=0
WITH_SKILL=0
SKILL_DIR="${CODEX_HOME:-${HOME}/.codex}/skills/alcli"
SKILL_RECEIPT="$SKILL_DIR/.installer-sha256"
MANAGED_MARKER="ALCLI_INSTALLER_MANAGED"

usage() {
  cat <<'HELP'
Usage: install.sh [--prefix DIR] [--yes] [--with-skill codex] [--uninstall]

Install ALcli as anilist and al. Downloaded release archives are SHA256 verified.
When run from the source checkout with Go installed, builds the local source.
  --prefix DIR          Binary directory (default: ~/.local/bin)
  --with-skill codex    Install the ALcli Codex skill too
  --uninstall           Remove installed ALcli and installer-managed skill
  --yes                 Skip confirmation prompts
HELP
}

while (($#)); do
  case "$1" in
    --prefix) [[ $# -ge 2 ]] || { echo 'Missing --prefix value' >&2; exit 2; }; PREFIX="$2"; shift 2 ;;
    --prefix=*) PREFIX="${1#*=}"; shift ;;
    --with-skill) [[ "${2:-}" == codex ]] || { echo 'Only --with-skill codex is supported' >&2; exit 2; }; WITH_SKILL=1; shift 2 ;;
    --yes|-y) YES=1; shift ;;
    --uninstall) UNINSTALL=1; shift ;;
    --help|-h) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

confirm() {
  if [[ "$YES" == 1 || "${ALCLI_YES:-}" == 1 ]]; then return 0; fi
  if [[ ! -r /dev/tty ]]; then echo 'Pass --yes for noninteractive installation' >&2; return 1; fi
  local answer
  read -r -p "$1 [y/N] " answer < /dev/tty
  [[ "$answer" == y || "$answer" == Y ]]
}

script_path="${BASH_SOURCE[0]:-}"
script_dir="$(cd "$(dirname "$script_path")" && pwd)"
local_checkout=0
if [[ -n "$script_path" && -f "$script_path" && -f "$script_dir/go.mod" ]] && command -v go >/dev/null 2>&1; then
  if command -v rg >/dev/null 2>&1; then
    rg -q '^module github.com/noyukii/ALcli$' "$script_dir/go.mod" && local_checkout=1
  elif head -n 1 "$script_dir/go.mod" | grep -q '^module github.com/noyukii/ALcli$'; then
    local_checkout=1
  fi
fi

hash_file() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    echo 'shasum or sha256sum is required' >&2
    return 1
  fi
}

managed_skill() {
  [[ ! -L "$SKILL_DIR" && ! -L "$SKILL_DIR/SKILL.md" && ! -L "$SKILL_RECEIPT" ]] || return 1
  [[ -f "$SKILL_DIR/SKILL.md" && -f "$SKILL_RECEIPT" ]] || return 1
  grep -q "$MANAGED_MARKER" "$SKILL_DIR/SKILL.md" || return 1
  [[ "$(cat "$SKILL_RECEIPT")" == "$(hash_file "$SKILL_DIR/SKILL.md")" ]]
}

remove_skill() {
  if managed_skill; then
    rm -f "$SKILL_DIR/SKILL.md" "$SKILL_RECEIPT"
    rmdir "$SKILL_DIR" 2>/dev/null || true
  elif [[ -e "$SKILL_DIR" ]]; then
    echo "Preserved an unmanaged or modified skill at $SKILL_DIR." >&2
  fi
}

if [[ "$UNINSTALL" == 1 ]]; then
  confirm "Remove ALcli from $PREFIX?" || exit 0
  rm -f "$PREFIX/al"
  rm -f "$PREFIX/anilist"
  remove_skill
  echo 'ALcli removed.'
  exit 0
fi

confirm "Install ALcli to $PREFIX?" || exit 0
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

if [[ "$local_checkout" == 1 ]]; then
  (cd "$script_dir" && go build -o "$work/anilist" .)
  skill_source="$script_dir/plugin/skills/alcli/SKILL.md"
else
  command -v curl >/dev/null 2>&1 || { echo 'curl is required' >&2; exit 1; }
  command -v tar >/dev/null 2>&1 || { echo 'tar is required' >&2; exit 1; }
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in darwin|linux) ;; *) echo "Unsupported OS: $os" >&2; exit 1 ;; esac
  case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
  esac
  archive="alcli_${os}_${arch}.tar.gz"
  base="${ALCLI_RELEASE_BASE_URL:-https://github.com/${REPO}/releases/latest/download}"
  curl -fsSL "$base/checksums.txt" -o "$work/checksums.txt"
  curl -fsSL "$base/$archive" -o "$work/$archive"
  expected="$(awk -v name="$archive" '$2 == name {print $1}' "$work/checksums.txt")"
  [[ "$expected" =~ ^[[:xdigit:]]{64}$ ]] || { echo 'Release checksum is missing or invalid' >&2; exit 1; }
  actual="$(hash_file "$work/$archive")"
  [[ "$actual" == "$expected" ]] || { echo 'Release checksum mismatch' >&2; exit 1; }
  tar -xzf "$work/$archive" -C "$work" anilist skills/alcli/SKILL.md
  skill_source="$work/skills/alcli/SKILL.md"
fi

if [[ "$WITH_SKILL" == 1 ]]; then
  [[ -f "$skill_source" ]] || { echo 'ALcli skill is missing' >&2; exit 1; }
  if [[ -e "$SKILL_DIR" ]] && ! managed_skill; then
    echo "Cannot replace an unmanaged skill: $SKILL_DIR" >&2; exit 1
  fi
fi

[[ -x "$work/anilist" ]] || { echo 'Archive has no executable anilist' >&2; exit 1; }
mkdir -p "$PREFIX"
install -m 755 "$work/anilist" "$PREFIX/anilist"
ln -sfn anilist "$PREFIX/al"

if [[ "$WITH_SKILL" == 1 ]]; then
  mkdir -p "$SKILL_DIR"
  install -m 644 "$skill_source" "$SKILL_DIR/SKILL.md"
  hash_file "$SKILL_DIR/SKILL.md" > "$SKILL_RECEIPT"
  chmod 600 "$SKILL_RECEIPT"
fi

echo "Installed ALcli to $PREFIX/anilist (alias: al)."
if [[ "$WITH_SKILL" == 1 ]]; then echo "Installed Codex skill to $SKILL_DIR."; fi
