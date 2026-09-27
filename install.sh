#!/usr/bin/env bash
# Install ALcli as `anilist` and `al`.
#
#   curl -fsSL https://raw.githubusercontent.com/noyukii/ALcli/main/install.sh | bash
#
#   ./install.sh
#   ./install.sh --yes --prefix "$HOME/.local/bin"
#   ./install.sh --uninstall
set -euo pipefail

MODULE="github.com/noyukii/ALcli"
REPO="noyukii/ALcli"
BIN_NAME="anilist"
ALIAS_NAME="al"

YES=0
UNINSTALL=0
PREFIX="${ALCLI_PREFIX:-}"

usage() {
  cat <<EOF
Usage: install.sh [options]

Install ALcli so both \`anilist\` and \`al\` run it.

Options:
  --prefix DIR   Install directory (default: /usr/local/bin)
  --yes          Skip prompts
  --uninstall    Remove anilist and al from the prefix
  -h, --help     Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --prefix)
      if [[ $# -lt 2 || -z "${2:-}" ]]; then
        echo "Missing value for --prefix" >&2
        exit 1
      fi
      PREFIX="$2"
      shift 2
      ;;
    --prefix=*)
      PREFIX="${1#*=}"
      shift
      ;;
    -y|--yes)
      YES=1
      shift
      ;;
    --uninstall)
      UNINSTALL=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ "${ALCLI_YES:-}" == "1" ]]; then
  YES=1
fi

# Interactive gum prompts must read the terminal. The script itself may be
# arriving on stdin via `curl | bash`, so never redirect the script's stdin.
tty_in() {
  if [[ -r /dev/tty ]]; then
    "$@" < /dev/tty
  else
    "$@"
  fi
}

say() {
  if command -v gum >/dev/null 2>&1; then
    gum style "$@"
  else
    printf '%s\n' "$*"
  fi
}

die() {
  if command -v gum >/dev/null 2>&1; then
    gum style --foreground 196 "$*" >&2
  else
    echo "$*" >&2
  fi
  exit 1
}

confirm() {
  local prompt="$1"
  if [[ "$YES" -eq 1 ]]; then
    return 0
  fi
  tty_in gum confirm "$prompt"
}

script_dir() {
  if [[ -n "${BASH_SOURCE[0]:-}" && -f "${BASH_SOURCE[0]}" ]]; then
    cd "$(dirname "${BASH_SOURCE[0]}")" && pwd
    return
  fi
  return 1
}

ensure_gum() {
  if command -v gum >/dev/null 2>&1; then
    return
  fi

  echo "gum is not installed. Installing it first..."
  if command -v brew >/dev/null 2>&1; then
    brew install gum
    hash -r
    return
  fi

  local os arch version url tmp bin dest
  os="$(uname -s)"
  arch="$(uname -m)"
  case "$os" in
    Darwin|Linux) ;;
    *) die "Install gum manually, then rerun this script: https://github.com/charmbracelet/gum" ;;
  esac
  case "$arch" in
    arm64|aarch64) arch="arm64" ;;
    x86_64) arch="x86_64" ;;
    *) die "Unsupported architecture: $arch" ;;
  esac

  version="$(curl -fsSL "https://api.github.com/repos/charmbracelet/gum/releases/latest" \
    | sed -n 's/.*"tag_name": *"v\([^"]*\)".*/\1/p' \
    | head -n 1)"
  if [[ -z "$version" ]]; then
    die "Could not determine the latest gum release."
  fi

  url="https://github.com/charmbracelet/gum/releases/download/v${version}/gum_${version}_${os}_${arch}.tar.gz"
  tmp="$(mktemp -d)"
  curl -fsSL "$url" | tar -xz -C "$tmp"
  bin="$(find "$tmp" -type f -name gum -print -quit)"
  if [[ -z "$bin" ]]; then
    rm -rf "$tmp"
    die "The gum archive did not contain a gum binary."
  fi

  dest="${HOME}/.local/bin"
  mkdir -p "$dest"
  install -m 755 "$bin" "$dest/gum"
  rm -rf "$tmp"
  export PATH="${dest}:${PATH}"
  hash -r
}

ensure_go() {
  if command -v go >/dev/null 2>&1; then
    return
  fi
  say --foreground 196 "Go is required to build ALcli."
  if command -v brew >/dev/null 2>&1 && confirm "Install Go with Homebrew?"; then
    brew install go
    hash -r
    return
  fi
  die "Install Go from https://go.dev/dl/ and run this script again."
}

need_sudo() {
  local dir="$1"
  if [[ -d "$dir" && -w "$dir" ]]; then
    return 1
  fi
  if [[ ! -e "$dir" ]]; then
    local parent="$dir"
    while [[ ! -d "$parent" && "$parent" != "/" ]]; do
      parent="$(dirname "$parent")"
    done
    if [[ -w "$parent" ]]; then
      return 1
    fi
  fi
  return 0
}

run_priv() {
  if [[ "${USE_SUDO:-0}" -eq 1 ]]; then
    sudo "$@"
  else
    "$@"
  fi
}

prepare_prefix() {
  if [[ -z "$PREFIX" ]]; then
    if [[ "$YES" -eq 1 || ! -r /dev/tty ]]; then
      PREFIX="/usr/local/bin"
    else
      PREFIX="$(tty_in gum choose --header "Where should anilist and al be installed?" --label-delimiter "|" \
        "System-wide|/usr/local/bin" \
        "Just for me|${HOME}/.local/bin")" || {
        say "Cancelled."
        exit 0
      }
    fi
  fi

  if need_sudo "$PREFIX"; then
    if ! confirm "Writing to ${PREFIX} needs sudo. Continue?"; then
      say "Cancelled."
      exit 0
    fi
    USE_SUDO=1
    run_priv mkdir -p "$PREFIX"
  else
    USE_SUDO=0
    mkdir -p "$PREFIX"
  fi
}

local_source() {
  local dir=""
  dir="$(script_dir 2>/dev/null || true)"
  if [[ -n "$dir" && -f "${dir}/go.mod" ]] && grep -q "module ${MODULE}" "${dir}/go.mod"; then
    printf '%s\n' "$dir"
    return
  fi
  if [[ -f "./go.mod" ]] && grep -q "module ${MODULE}" "./go.mod"; then
    pwd
  fi
}

uninstall_cmds() {
  prepare_prefix
  local target
  if [[ -L "${PREFIX}/${ALIAS_NAME}" ]]; then
    target="$(readlink "${PREFIX}/${ALIAS_NAME}")"
    case "$target" in
      "${BIN_NAME}"|"${PREFIX}/${BIN_NAME}")
        run_priv rm -f "${PREFIX}/${ALIAS_NAME}"
        ;;
    esac
  fi
  if [[ -f "${PREFIX}/${BIN_NAME}" ]]; then
    run_priv rm -f "${PREFIX}/${BIN_NAME}"
  fi
  say --foreground 212 --bold "Removed ${BIN_NAME} and ${ALIAS_NAME} from ${PREFIX}."
}

install_cmds() {
  local source build_dir
  source="$(local_source || true)"
  build_dir="$(mktemp -d)"
  # shellcheck disable=SC2064
  trap "rm -rf '$build_dir'" EXIT

  ensure_go

  if [[ -n "$source" ]]; then
    gum spin --show-error --spinner dot --title "Building ALcli..." -- \
      bash -c 'cd "$1" && go build -trimpath -ldflags "-s -w" -o "$2/anilist" .' _ "$source" "$build_dir"
  else
    gum spin --show-error --spinner dot --title "Downloading ALcli..." -- \
      env GOBIN="$build_dir" go install "${MODULE}@latest"
    if [[ -f "${build_dir}/ALcli" ]]; then
      mv "${build_dir}/ALcli" "${build_dir}/anilist"
    fi
  fi

  if [[ ! -x "${build_dir}/anilist" ]]; then
    die "Build finished without an anilist binary."
  fi

  if [[ -e "${PREFIX}/${BIN_NAME}" && "$YES" -ne 1 ]]; then
    if ! confirm "Replace existing ${PREFIX}/${BIN_NAME}?"; then
      say "Cancelled."
      exit 0
    fi
  fi
  if [[ -e "${PREFIX}/${ALIAS_NAME}" && ! -L "${PREFIX}/${ALIAS_NAME}" && "$YES" -ne 1 ]]; then
    if ! confirm "${PREFIX}/${ALIAS_NAME} already exists and is not a symlink. Replace it?"; then
      say "Cancelled."
      exit 0
    fi
  fi

  run_priv install -m 755 "${build_dir}/anilist" "${PREFIX}/${BIN_NAME}"
  run_priv ln -sfn "$BIN_NAME" "${PREFIX}/${ALIAS_NAME}"

  say --foreground 212 --border double --border-foreground 212 --align center \
    --width 46 --margin "1 2" --padding "1 2" --bold \
    "ALcli installed" "anilist" "al"

  case ":${PATH}:" in
    *":${PREFIX}:"*) ;;
    *)
      say --foreground 214 "Add ${PREFIX} to your PATH:"
      say --bold "export PATH=\"${PREFIX}:\$PATH\""
      ;;
  esac
}

main() {
  ensure_gum

  say --foreground 212 --border-foreground 212 --border double --align center \
    --width 46 --margin "1 2" --padding "1 2" --bold \
    "ALcli" "AniList in the terminal"

  if [[ "$UNINSTALL" -eq 1 ]]; then
    uninstall_cmds
    exit 0
  fi

  if [[ "$YES" -ne 1 && ! -r /dev/tty ]]; then
    die "No terminal available for prompts. Re-run with --yes --prefix DIR."
  fi

  say "Installs ${BIN_NAME} and ${ALIAS_NAME} from github.com/${REPO}."
  if ! confirm "Install ALcli?"; then
    say "Cancelled."
    exit 0
  fi

  prepare_prefix
  install_cmds
}

main
