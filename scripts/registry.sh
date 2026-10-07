#!/bin/sh
# Renders the server.json that is published to the official MCP registry
# (registry.modelcontextprotocol.io). Run by .github/workflows/release.yml
# after the GitHub Release is public; procedure in RELEASING.md "MCP registry".
#
#   sh scripts/registry.sh render VERSION CHECKSUMS_FILE   # server.json to stdout
#   sh scripts/registry.sh check FILE                        # verify a rendered file
#   sh scripts/registry.sh exists VERSION                    # 0 if the registry already has it
#
# The committed server.json is the template (name, description, repository).
# `render` sets the version and adds one `mcpb` package per OS/CPU, pointing
# at the .mcpb assets of the GitHub Release vVERSION with their SHA-256 taken
# from CHECKSUMS_FILE (the release's checksums.txt). Requires jq.
set -eu

die() { printf 'registry.sh: %s\n' "$1" >&2; exit 1; }

SERVER_NAME=io.github.gridhra/chat-tool-ui-preview
DOWNLOAD=https://github.com/gridhra/chat-tool-ui-preview/releases/download
REGISTRY=https://registry.modelcontextprotocol.io
PLATFORMS="darwin_arm64 darwin_amd64 windows_arm64 windows_amd64 linux_arm64 linux_amd64"

check_version() {
  printf '%s' "$1" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' || die "version must be X.Y.Z (no leading v): $1"
}

# render VERSION CHECKSUMS_FILE
render() {
  version=$1; sums=$2
  check_version "$version"
  [ -f "$sums" ] || die "checksums file not found: $sums"
  command -v jq >/dev/null || die "jq is required"
  template=$(dirname "$0")/../server.json
  pkgs='[]'
  for p in $PLATFORMS; do
    f="chat-preview_${version}_${p}.mcpb"
    sha=$(awk -v f="$f" '$2 == f {print $1}' "$sums")
    [ -n "$sha" ] || die "$f is not listed in $sums"
    printf '%s' "$sha" | grep -Eq '^[0-9a-f]{64}$' || die "bad sha256 for $f: $sha"
    pkgs=$(printf '%s' "$pkgs" | jq --arg id "$DOWNLOAD/v$version/$f" --arg sha "$sha" --arg v "$version" \
      '. + [{registryType: "mcpb", identifier: $id, version: $v, fileSha256: $sha, transport: {type: "stdio"}}]')
  done
  jq --arg v "$version" --argjson pkgs "$pkgs" '.version = $v | .packages = $pkgs' "$template"
}

# check FILE: name, version shape, six mcpb packages with hashes
check() {
  file=$1
  [ -f "$file" ] || die "not found: $file"
  command -v jq >/dev/null || die "jq is required"
  jq -e --arg n "$SERVER_NAME" '
    .name == $n
    and (.version | test("^[0-9]+\\.[0-9]+\\.[0-9]+$"))
    and (.packages | length == 6)
    and all(.packages[]; .registryType == "mcpb"
      and (.identifier | startswith("https://github.com/gridhra/chat-tool-ui-preview/releases/download/v"))
      and (.identifier | endswith(".mcpb"))
      and (.fileSha256 | test("^[0-9a-f]{64}$"))
      and .transport.type == "stdio")
  ' "$file" >/dev/null || die "$file is not a valid rendering"
  printf 'OK %s (%s)\n' "$file" "$(jq -r .version "$file")"
}

# exists VERSION: exit 0 when the registry already lists this version
exists() {
  check_version "$1"
  enc=$(printf '%s' "$SERVER_NAME" | sed 's,/,%2F,g')
  code=$(curl -sS -o /dev/null -w '%{http_code}' "$REGISTRY/v0.1/servers/$enc/versions/$1")
  case "$code" in
    200) printf 'registry has %s %s\n' "$SERVER_NAME" "$1"; return 0 ;;
    404) printf 'registry does not have %s %s\n' "$SERVER_NAME" "$1"; return 1 ;;
    *) die "unexpected status $code from the registry" ;;
  esac
}

cmd=${1:-}; [ $# -gt 0 ] && shift
case "$cmd" in
  render) [ $# -eq 2 ] || die "usage: render VERSION CHECKSUMS_FILE"; render "$@" ;;
  check) [ $# -eq 1 ] || die "usage: check FILE"; check "$1" ;;
  exists) [ $# -eq 1 ] || die "usage: exists VERSION"; exists "$1" ;;
  *) die "usage: registry.sh render|check|exists ..." ;;
esac
