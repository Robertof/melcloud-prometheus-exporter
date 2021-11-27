#!/usr/bin/env bash

cd "$( dirname "${BASH_SOURCE[0]}" )"

echo "building for linux armv7..."

pushd ../src/
env GOOS=linux GOARCH=arm GOARM=7 -ldflags="-s -w" go build -o ../melcloud-prometheus-exporter.bin
popd

pushd ../

echo "spawning subshell - please deploy '*.nix' and '*.bin' to your server."
echo "once done, press ctrl-D and the binary file will be deleted automatically."

$SHELL

rm melcloud-prometheus-exporter.bin

echo "cleanup complete."
