#!/bin/sh
# Builds the viewer for the browser into one static folder.
#
# Usage: wasm/build.sh [--raw] [outdir] [feed.json]
#
#   --raw      keep the uncompressed mentions.wasm as well
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
	-o "$out/mentions.wasm" ./wasm
gzip -9 -n -c "$out/mentions.wasm" >"$out/mentions.wasm.gz"
brotli -q 11 -f -o "$out/mentions.wasm.br" "$out/mentions.wasm"
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$out/"

sip=$(go mod download -json "github.com/Gaurav-Gosain/sip@$sipver" | sed -n 's/^[[:space:]]*"Dir": "\(.*\)",$/\1/p')
if [ -z "$sip" ] || [ ! -f "$sip/static/webterm.js" ]; then
	echo "build.sh: cannot find sip's webterm bundle (sip $sipver)" >&2
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
rawsize=$(size "$out/mentions.wasm")
if [ "$raw" = 0 ]; then
	rm -f "$out/mentions.wasm"
fi
echo "mentions.wasm: raw $rawsize, gzip $(size "$out/mentions.wasm.gz"), brotli $(size "$out/mentions.wasm.br") bytes"
echo "wrote $out"
