#!/bin/sh
# Builds the viewer for the browser into one static folder.
#
# Usage: wasm/build.sh [--raw] [outdir] [feed.json]
#
#   --raw      keep the uncompressed hearsay.wasm as well
#   outdir     default: dist/
#   feed.json  copied into the folder as mentions.json. Without it the page
#              shows the bundled demo.
#
# Needs go, gzip and brotli. The terminal renderer is sip's webterm bundle
# (xterm.js with the WebGL addon) and its JetBrains Mono Nerd Font files.
set -e
raw=0
if [ "$1" = "--raw" ]; then
	raw=1
	shift
fi
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/.." && pwd)
out=${1:-"$root/dist"}
feedfile=$2
sipver=${SIP_VERSION:-v0.9.3}
export GOWORK=off
modfile=$("$here/prepare.sh")
mkdir -p "$out/fonts"
out=$(cd "$out" && pwd)
cd "$root"

GOOS=js GOARCH=wasm go build -modfile "$modfile" -trimpath -ldflags "-s -w" \
	-o "$out/hearsay.wasm" ./wasm
gzip -9 -n -c "$out/hearsay.wasm" >"$out/hearsay.wasm.gz"
brotli -q 11 -f -o "$out/hearsay.wasm.br" "$out/hearsay.wasm"
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$out/"

# sip is not a dependency of hearsay, so a clean machine has no copy of it.
# go mod download fetches it. Its go.mod needs Go 1.26, which is why
# hearsay's go.mod asks for 1.26 too.
info=$(go mod download -json "github.com/Gaurav-Gosain/sip@$sipver" || true)
sip=$(printf '%s\n' "$info" | sed -n 's/^[[:space:]]*"Dir": "\(.*\)",$/\1/p')
if [ -z "$sip" ] || [ ! -f "$sip/static/webterm.js" ]; then
	echo "build.sh: cannot find sip's webterm bundle (sip $sipver)" >&2
	printf '%s\n' "$info" | sed -n 's/^[[:space:]]*"Error": "\(.*\)",*$/build.sh: \1/p' >&2
	exit 1
fi
cp "$sip/static/webterm.js" "$sip/static/webterm.css" "$sip/static/xterm.css" "$out/"
for face in Regular Bold; do
	cp "$sip/static/fonts/JetBrainsMonoNerdFontMono-$face.woff2" "$out/fonts/"
done
cp -R "$here/site/." "$out/"
if [ -n "$feedfile" ]; then
	cp "$feedfile" "$out/mentions.json"
fi
chmod -R u+w "$out"

size() { wc -c <"$1" | tr -d ' '; }
rawsize=$(size "$out/hearsay.wasm")
if [ "$raw" = 0 ]; then
	rm -f "$out/hearsay.wasm"
fi
echo "hearsay.wasm: raw $rawsize, gzip $(size "$out/hearsay.wasm.gz"), brotli $(size "$out/hearsay.wasm.br") bytes"
echo "wrote $out"
