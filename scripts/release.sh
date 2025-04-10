#!/bin/sh

set -e

BUILDSTAMP="$(git rev-parse --short HEAD)"

# argumentsa are GOOS and GOARCH
build() {
	export GOOS="$1"
	export GOARCH="$2"

	GO_OUT_PATH="./release/$GOOS-$GOARCH"

	go build -o "$GO_OUT_PATH" -ldflags="-w -s -X main.buildstamp=$BUILDSTAMP" .
}

rm -rf ./release
mkdir -p ./release

build linux 386
build linux amd64
build linux arm
build linux arm64
