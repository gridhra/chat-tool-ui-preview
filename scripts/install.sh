#!/bin/sh
# Install a prebuilt chat-preview binary (macOS / Linux).
#
#   curl -fsSL https://raw.githubusercontent.com/gridhra/chat-tool-ui-preview/main/scripts/install.sh | sh
#
# Re-run it to update. It only ever writes the one binary; chat-preview keeps no state of its own.
#
# Environment:
#   CHAT_PREVIEW_VERSION      version to install, e.g. 0.1.0 (default: latest release)
#   CHAT_PREVIEW_INSTALL_DIR  install directory (default: $HOME/.local/bin). Never needs sudo.
set -eu

REPO="gridhra/chat-tool-ui-preview"
INSTALL_DIR="${CHAT_PREVIEW_INSTALL_DIR:-$HOME/.local/bin}"

die() { printf 'install.sh: %s\n' "$1" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"; }

need uname
need tar

# CHAT_PREVIEW_DOWNLOAD_BASE points at a local directory laid out like a release
# (scripts/install_test.sh uses it). It replaces the download, not the verification.
if [ -n "${CHAT_PREVIEW_DOWNLOAD_BASE:-}" ]; then
  fetch() { die "set CHAT_PREVIEW_VERSION when CHAT_PREVIEW_DOWNLOAD_BASE is set"; }
  fetch_to() { cp "$1" "$2"; }
# Nothing but HTTPS, redirects included.
elif command -v curl >/dev/null 2>&1; then
  fetch() { curl --proto '=https' --tlsv1.2 -fsSL "$1"; }
  fetch_to() { curl --proto '=https' --tlsv1.2 -fsSL -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
  fetch() { wget --https-only -qO- "$1"; }
  fetch_to() { wget --https-only -qO "$2" "$1"; }
else
  die "need curl or wget"
fi

os="$(uname -s)"
arch="$(uname -m)"
case "$os" in
  Darwin) goos=darwin ;;
  Linux) goos=linux ;;
  *) goos="" ;;
esac
case "$arch" in
  x86_64|amd64) goarch=amd64 ;;
  arm64|aarch64) goarch=arm64 ;;
  *) goarch="" ;;
esac
if [ -z "$goos" ] || [ -z "$goarch" ]; then
  die "no prebuilt binary for $os/$arch. With a Go toolchain: go install github.com/$REPO/cmd/chat-preview@latest"
fi

version="${CHAT_PREVIEW_VERSION:-}"
if [ -z "$version" ]; then
  # The tag_name of the latest release. (The unauthenticated API allows 60
  # requests an hour per address; set CHAT_PREVIEW_VERSION if you hit that.)
  version="$(fetch "https://api.github.com/repos/$REPO/releases/latest" \
    | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  [ -n "$version" ] || die "could not determine the latest release of $REPO; set CHAT_PREVIEW_VERSION"
fi
version="${version#v}"
printf '%s' "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' || die "invalid version: $version (expected e.g. 0.1.0)"

archive="chat-preview_${version}_${goos}_${goarch}.tar.gz"
base="${CHAT_PREVIEW_DOWNLOAD_BASE:-https://github.com/$REPO/releases/download/v$version}"

tmp="$(mktemp -d)"
staged=""
trap 'rm -rf "$tmp"; [ -z "$staged" ] || rm -f "$staged"' EXIT INT TERM

printf 'Downloading chat-preview %s (%s/%s)\n' "$version" "$goos" "$goarch" >&2
fetch_to "$base/$archive" "$tmp/$archive" || die "download failed: $base/$archive"
fetch_to "$base/checksums.txt" "$tmp/checksums.txt" || die "download failed: $base/checksums.txt"

# Verification is mandatory: without a hashing tool, or without a matching
# line in checksums.txt, nothing is installed.
if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmp/$archive" | cut -d' ' -f1)"
elif command -v shasum >/dev/null 2>&1; then
  actual="$(shasum -a 256 "$tmp/$archive" | cut -d' ' -f1)"
else
  die "need sha256sum or shasum to verify the download"
fi
expected="$(awk -v f="$archive" '{ n = $2; sub(/^\*/, "", n); if (n == f) { print $1; exit } }' "$tmp/checksums.txt")"
[ -n "$expected" ] || die "$archive is not listed in checksums.txt"
[ "$actual" = "$expected" ] || die "checksum mismatch for $archive (expected $expected, got $actual); nothing was installed"
printf 'Checksum OK\n' >&2

tar -C "$tmp" -xzf "$tmp/$archive" chat-preview
mkdir -p "$INSTALL_DIR"
# Stage next to the destination and rename: an interrupted install never
# leaves a half-written binary where the old one was.
staged="$INSTALL_DIR/.chat-preview.$$"
cp "$tmp/chat-preview" "$staged"
chmod 755 "$staged"
mv -f "$staged" "$INSTALL_DIR/chat-preview"
staged=""

printf 'Installed %s\n' "$INSTALL_DIR/chat-preview" >&2
"$INSTALL_DIR/chat-preview" --version >&2 || die "the installed binary does not run on this machine"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    # shellcheck disable=SC2016 # $PATH and the backticks are meant literally
    {
    printf '\nNote: %s is not on your PATH. Add it, e.g. in ~/.zshrc or ~/.bashrc:\n  export PATH="%s:$PATH"\n' "$INSTALL_DIR" "$INSTALL_DIR"
    printf 'Hooks and MCP client configs call `chat-preview` by name, so it has to be on PATH.\n'
    } >&2
    ;;
esac
