#!/bin/sh

set -eu

STATUS=0

tmp=$(mktemp -d)

trap 'rm -rf -- "$tmp"' INT TERM HUP EXIT

cp go.mod go.sum "$tmp/"

go mod tidy

for file in go.mod go.sum ; do
    cp "$file" "$tmp/$file.tidy"
    cd "$tmp"
    if ! diff -u $file $file.tidy ; then
        STATUS=1
    fi
    cd -
done

exit "$STATUS"
