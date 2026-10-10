#!/bin/sh
# Packages the plugin for every platform the server is built for into
# dist/, as the catalog offers it: a zip per platform holding manifest.json
# and the executable, and dist/catalog.json listing them.
# Usage: scripts/dist.sh [os/arch ...]
set -eu
cd "$(dirname "$0")/.."

targets=${*:-"linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64"}
version=$(sed -n 's/^  "version": "\(.*\)",$/\1/p' manifest.json)
api=$(sed -n 's/^  "apiVersion": "\(.*\)"$/\1/p' manifest.json)
now=$(date -u +%Y-%m-%dT%H:%M:%SZ)

rm -rf dist
mkdir -p dist
entries=
for target in $targets; do
	os=${target%/*}
	arch=${target#*/}
	exe=plugin
	[ "$os" = windows ] && exe=plugin.exe
	dir="dist/$os-$arch"
	mkdir -p "$dir"
	CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags "-s -w -X main.Version=$version" -o "$dir/$exe" .
	cp manifest.json "$dir/"
	zipfile="dlna-$version-$os-$arch.zip"
	(cd "$dir" && zip -q -X "../$zipfile" manifest.json "$exe")
	sum=$(shasum -a 256 "dist/$zipfile" | cut -d' ' -f1)
	entry="{\"version\": \"$version\", \"api_version\": \"$api\", \"runtime\": \"RUNTIME_PROCESS\", \"os\": \"$os\", \"arch\": \"$arch\", \"url\": \"$zipfile\", \"sha256\": \"$sum\", \"release_time\": \"$now\"}"
	entries="${entries:+$entries,
}      $entry"
	echo "dist/$zipfile"
done
cat > dist/catalog.json <<JSON
{
  "plugins": [{
    "id": "org.mavio.dlna",
    "name": "DLNA",
    "description": "A DLNA media server for televisions and other devices on the local network, and Play To, which plays on DLNA renderers.",
    "author": "Mavio",
    "homepage": "https://github.com/mavioai/mavio",
    "versions": [
$entries
    ]
  }]
}
JSON
