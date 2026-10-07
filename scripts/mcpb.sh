#!/bin/sh
# Builds MCP Bundles (.mcpb: a zip with manifest.json at the root, installable
# by Claude Desktop) from the release binaries. Run by GoReleaser's post-build
# hook (.goreleaser.yaml); procedure and rationale in RELEASING.md "MCP
# registry". One bundle per OS/CPU, named like the archives:
#
#   sh scripts/mcpb.sh pack BINARY OS ARCH VERSION EPOCH OUTDIR
#       -> OUTDIR/chat-preview_VERSION_OS_ARCH.mcpb
#   sh scripts/mcpb.sh manifest OS ARCH VERSION      # print manifest.json
#   sh scripts/mcpb.sh check FILE.mcpb               # verify a bundle's layout
#
# EPOCH (unix seconds) is stamped on every file so the zip is reproducible.
# `check` needs unzip and jq. No Node: the bundle is a plain zip, and the
# official `mcpb pack` would add a Node dependency to the release for nothing.
# shellcheck disable=SC2016 # ${__dirname} and $0 are for the host app, not this shell
set -eu

die() { printf 'mcpb.sh: %s\n' "$1" >&2; exit 1; }

NAME=chat-preview
REPO=https://github.com/gridhra/chat-tool-ui-preview

# manifest OS ARCH VERSION
manifest() {
  os=$1; arch=$2; version=$3
  case "$os" in
    darwin)
      # Claude Desktop on macOS extracts bundles with mode 0600 and drops the
      # execute bit (modelcontextprotocol/mcpb issue #294, open 2026-10-06), so a
      # `type: binary` server cannot be exec()ed directly. The command is
      # therefore /bin/sh, which restores u+x on the bundled binary and execs it.
      # $0 is the binary path passed as the first positional argument.
      bin='server/chat-preview'
      cmd='"command": "/bin/sh",
      "args": ["-c", "chmod u+x \"$0\" && exec \"$0\" mcp", "${__dirname}/server/chat-preview"]' ;;
    windows)
      bin='server/chat-preview.exe'
      cmd='"command": "${__dirname}/server/chat-preview.exe",
      "args": ["mcp"]' ;;
    linux)
      bin='server/chat-preview'
      cmd='"command": "${__dirname}/server/chat-preview",
      "args": ["mcp"]' ;;
    *) die "unsupported os: $os" ;;
  esac
  # The description avoids the service's name on purpose (docs/DESIGN.md §2.2).
  cat <<JSON
{
  "manifest_version": "0.3",
  "name": "chat-tool-ui-preview",
  "display_name": "chat-tool-ui-preview",
  "version": "$version",
  "description": "Shows a chat post in your browser before an agent sends it: wording, mentions, destination, thread placement, warnings. Never sends.",
  "long_description": "Local preview for posts an AI agent drafts for a team chat. The agent calls preview_message before sending; the post opens in your browser as a neutral, original chat-like page with the destination, thread placement, sender identity and a list of points to check (@here/@channel, unresolved mentions, length, unsupported blocks). Nothing is sent and no chat service is contacted. Bundle for $os/$arch.",
  "author": { "name": "gridhra", "url": "https://github.com/gridhra" },
  "repository": { "type": "git", "url": "$REPO" },
  "homepage": "$REPO",
  "documentation": "$REPO#readme",
  "support": "$REPO/issues",
  "license": "MIT",
  "keywords": ["preview", "chat", "review", "human-in-the-loop", "agent"],
  "server": {
    "type": "binary",
    "entry_point": "$bin",
    "mcp_config": {
      $cmd,
      "env": {}
    }
  },
  "tools": [
    { "name": "preview_message", "description": "Open a preview of a chat post and return its path and warnings. Call before sending; never sends." }
  ],
  "compatibility": { "platforms": ["$os"] }
}
JSON
}

# pack BINARY OS ARCH VERSION EPOCH OUTDIR
pack() {
  binary=$1; os=$2; arch=$3; version=$4; epoch=$5; outdir=$6
  [ -f "$binary" ] || die "binary not found: $binary"
  printf '%s' "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$' || die "version must be X.Y.Z: $version"
  printf '%s' "$epoch" | grep -Eq '^[0-9]+$' || die "epoch must be unix seconds: $epoch"
  command -v zip >/dev/null || die "zip is required"
  case "$os" in windows) ext=.exe ;; *) ext= ;; esac
  out="$outdir/${NAME}_${version}_${os}_${arch}.mcpb"
  mkdir -p "$outdir"
  stage=$(mktemp -d "${TMPDIR:-/tmp}/mcpb.XXXXXX")
  trap 'rm -rf "$stage"' EXIT
  mkdir -p "$stage/server"
  manifest "$os" "$arch" "$version" > "$stage/manifest.json"
  cp "$binary" "$stage/server/$NAME$ext"
  cp LICENSE "$stage/LICENSE"
  chmod 0755 "$stage/server/$NAME$ext"
  chmod 0644 "$stage/manifest.json" "$stage/LICENSE"
  # Fixed mtime and no extra fields (-X): same inputs give the same bytes.
  stamp=$(TZ=UTC date -u -r "$epoch" +%Y%m%d%H%M.%S 2>/dev/null || TZ=UTC date -u -d "@$epoch" +%Y%m%d%H%M.%S)
  touch -t "$stamp" "$stage/manifest.json" "$stage/server/$NAME$ext" "$stage/LICENSE"
  rm -f "$out"
  out_abs=$(cd "$(dirname "$out")" && pwd)/$(basename "$out")
  (cd "$stage" && TZ=UTC zip -X -q -D "$out_abs" manifest.json "server/$NAME$ext" LICENSE)
  printf '%s\n' "$out"
}

# check FILE: the bundle has exactly the expected entries and a parsable manifest
check() {
  file=$1
  [ -f "$file" ] || die "not found: $file"
  command -v unzip >/dev/null || die "unzip is required"
  command -v jq >/dev/null || die "jq is required"
  entries=$(unzip -Z1 "$file" | sort | tr '\n' ' ')
  case "$entries" in
    "LICENSE manifest.json server/chat-preview "|"LICENSE manifest.json server/chat-preview.exe ") ;;
    *) die "unexpected entries in $file: $entries" ;;
  esac
  m=$(unzip -p "$file" manifest.json)
  printf '%s' "$m" | jq -e '.manifest_version == "0.3" and .server.type == "binary" and (.version | test("^[0-9]+\\.[0-9]+\\.[0-9]+")) and (.tools[0].name == "preview_message")' >/dev/null \
    || die "manifest of $file is not what pack writes"
  entry=$(printf '%s' "$m" | jq -r .server.entry_point)
  unzip -Z1 "$file" | grep -qx "$entry" || die "entry_point $entry is not in $file"
  # The binary must carry the execute bit in the zip's unix attributes.
  mode=$(unzip -Z "$file" "$entry" | awk '{print $1}')
  case "$mode" in -rwxr-xr-x) ;; *) die "$entry has mode $mode, want -rwxr-xr-x" ;; esac
  printf 'OK %s (%s)\n' "$file" "$(printf '%s' "$m" | jq -r '.compatibility.platforms[0] + " " + .version')"
}

cmd=${1:-}; [ $# -gt 0 ] && shift
case "$cmd" in
  pack) [ $# -eq 6 ] || die "usage: pack BINARY OS ARCH VERSION EPOCH OUTDIR"; pack "$@" ;;
  manifest) [ $# -eq 3 ] || die "usage: manifest OS ARCH VERSION"; manifest "$@" ;;
  check) [ $# -eq 1 ] || die "usage: check FILE"; check "$1" ;;
  *) die "usage: mcpb.sh pack|manifest|check ..." ;;
esac
