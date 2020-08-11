#!/bin/sh

upx -V >/dev/null && UPX="${UPX:-1}"

set -e

BUILDSTAMP="$(git rev-parse --short HEAD)"

# argumentsa are GOOS and GOARCH
build() {
	export GOOS="$1"
	export GOARCH="$2"

	GO_OUT_PATH="./release/$GOOS-$GOARCH"

	go build -o "$GO_OUT_PATH-no-upx" -ldflags="-w -s -X main.buildstamp=$BUILDSTAMP" .
	if [ "$UPX" = "1" ]; then
		upx -f --brute -o "$GO_OUT_PATH" "$GO_OUT_PATH-no-upx"
	fi
}

rm -rf ./release
mkdir -p ./release

build linux 386
build linux amd64
build linux arm
build linux arm64
