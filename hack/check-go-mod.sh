#!/bin/sh

STATUS=0

cp go.mod go.mod.orig
cp go.sum go.sum.orig

go mod tidy

mv go.mod go.mod.tidy
mv go.sum go.sum.tidy

mv go.mod.orig go.mod
mv go.sum.orig go.sum

for file in go.mod go.sum ; do
    if ! diff -u $file $file.tidy ; then
        STATUS=1
    fi
done

exit "$STATUS"