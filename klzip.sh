#!/bin/bash

if ! command -v goreleaser &>/dev/null; then
  set -e
  go build -o ./dist/klzip ./
  time ./dist/klzip "$@"
  exit
fi

set -e
goreleaser --snapshot --skip-publish --skip-sign --rm-dist &>/dev/null
time ./dist/klzip_linux_amd64/klzip "$@"
