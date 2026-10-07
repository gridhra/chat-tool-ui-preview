#!/bin/sh
# Tests for scripts/mcpb.sh and scripts/registry.sh without a release: builds
# a binary for darwin/arm64 and windows/amd64, packs both into .mcpb bundles,
# checks the bundles, then renders server.json from a checksums file that
# lists them. Run by CI; needs go, zip, unzip, jq. Does not open a browser
# and does not touch the network.
set -eu

die() { printf 'mcpb_test.sh: %s\n' "$1" >&2; exit 1; }
root=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d "${TMPDIR:-/tmp}/mcpb_test.XXXXXX")
trap 'rm -rf "$work"' EXIT
cd "$root"

version=9.9.9
epoch=1700000000
mkdir -p "$work/bin" "$work/out"
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -o "$work/bin/darwin" ./cmd/chat-preview
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o "$work/bin/windows.exe" ./cmd/chat-preview

# 1. pack + check for both OS families
sh scripts/mcpb.sh pack "$work/bin/darwin" darwin arm64 "$version" "$epoch" "$work/out" >/dev/null
sh scripts/mcpb.sh pack "$work/bin/windows.exe" windows amd64 "$version" "$epoch" "$work/out" >/dev/null
test -f "$work/out/chat-preview_${version}_darwin_arm64.mcpb" || die "darwin bundle missing"
test -f "$work/out/chat-preview_${version}_windows_amd64.mcpb" || die "windows bundle missing"
sh scripts/mcpb.sh check "$work/out/chat-preview_${version}_darwin_arm64.mcpb"
sh scripts/mcpb.sh check "$work/out/chat-preview_${version}_windows_amd64.mcpb"

# 2. the darwin manifest carries the exec-bit workaround, windows runs the .exe
m=$(unzip -p "$work/out/chat-preview_${version}_darwin_arm64.mcpb" manifest.json)
printf '%s' "$m" | jq -e '.server.mcp_config.command == "/bin/sh" and (.server.mcp_config.args[1] | contains("chmod u+x")) and .compatibility.platforms == ["darwin"]' >/dev/null || die "darwin manifest"
m=$(unzip -p "$work/out/chat-preview_${version}_windows_amd64.mcpb" manifest.json)
printf '%s' "$m" | jq -e '.server.mcp_config.command == "${__dirname}/server/chat-preview.exe" and .server.mcp_config.args == ["mcp"]' >/dev/null || die "windows manifest"

# 3. packing twice gives identical bytes (reproducible)
sh scripts/mcpb.sh pack "$work/bin/darwin" darwin arm64 "$version" "$epoch" "$work/out2" >/dev/null
cmp "$work/out/chat-preview_${version}_darwin_arm64.mcpb" "$work/out2/chat-preview_${version}_darwin_arm64.mcpb" || die "pack is not reproducible"

# 4. the extracted binary runs after the same chmod the darwin manifest does
#    (only on a mac; the windows binary is not run)
if [ "$(uname -s)" = Darwin ]; then
  mkdir -p "$work/x" && (cd "$work/x" && unzip -q "$work/out/chat-preview_${version}_darwin_arm64.mcpb")
  chmod 0600 "$work/x/server/chat-preview" # what Claude Desktop does (mcpb issue #294)
  out=$(/bin/sh -c 'chmod u+x "$0" && exec "$0" version' "$work/x/server/chat-preview")
  case "$out" in "chat-preview "*) ;; *) die "binary did not run after chmod: $out" ;; esac
fi

# 5. render server.json from a checksums file naming all six bundles
sums="$work/checksums.txt"
: > "$sums"
for p in darwin_arm64 darwin_amd64 windows_arm64 windows_amd64 linux_arm64 linux_amd64; do
  printf '%064d  chat-preview_%s_%s.mcpb\n' 1 "$version" "$p" >> "$sums"
done
printf '%064d  chat-preview_%s_darwin_arm64.tar.gz\n' 2 "$version" >> "$sums"
sh scripts/registry.sh render "$version" "$sums" > "$work/server.json"
sh scripts/registry.sh check "$work/server.json"
jq -e --arg v "$version" '.packages[0].identifier == "https://github.com/gridhra/chat-tool-ui-preview/releases/download/v\($v)/chat-preview_\($v)_darwin_arm64.mcpb" and .packages[0].version == $v' "$work/server.json" >/dev/null || die "package identifier"

# 6. a missing bundle in checksums is an error
printf '%064d  chat-preview_%s_darwin_arm64.mcpb\n' 1 "$version" > "$work/short.txt"
if sh scripts/registry.sh render "$version" "$work/short.txt" >/dev/null 2>&1; then die "render accepted an incomplete checksums file"; fi

echo "mcpb_test.sh: OK"
