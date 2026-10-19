#!/bin/sh
# Prepares a module setup for the js/wasm build without touching go.mod, and
# prints the path of the modfile to pass to go build with -modfile.
#
# charm.land/bubbletea/v2 has TTY and signal files for unix and windows only,
# so it does not compile for js/wasm as released. This copies it out of the
# module cache into .wasm-build/ and adds stubs/bubbletea_tty_js.go.in as
# tty_js.go. Upstream: charmbracelet/bubbletea issue 1410. Drop this once a
# release builds for js.
set -e
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/.." && pwd)
out="$root/.wasm-build"
export GOWORK=off
cd "$root"
mkdir -p "$out"
cp go.mod "$out/go.wasm.mod"
cp go.sum "$out/go.wasm.sum"
go mod download -modfile "$out/go.wasm.mod" charm.land/bubbletea/v2
bt=$(go list -modfile "$out/go.wasm.mod" -m -f '{{.Dir}}' charm.land/bubbletea/v2)
rm -rf "$out/bubbletea"
cp -R "$bt" "$out/bubbletea"
chmod -R u+w "$out/bubbletea"
cp "$here/stubs/bubbletea_tty_js.go.in" "$out/bubbletea/tty_js.go"
# -modfile resolves a relative replace path against the main module root.
printf '\nreplace charm.land/bubbletea/v2 => ./.wasm-build/bubbletea\n' >>"$out/go.wasm.mod"
echo "$out/go.wasm.mod"
