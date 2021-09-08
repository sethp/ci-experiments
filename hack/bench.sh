#!/bin/bash

set -euo pipefail

go build -o bin/build ./hack/build


# hyperfine -n "buildkit test" './bin/build --no-resolve --target test --progress plain' -n "go test" 'go test -v ./...'

sed_args=(-i "''")
if sed --version | grep -qi gnu ; then
    sed_args=(-i)
fi

last=$(grep var\ _ main_test.go | cut -d= -f 2)

hyperfine \
    -p "last=\$(grep var\\ _ main_test.go | cut -d= -f 2); sed ${sed_args[*]} 's/var _ = .*/var _ = '\"\$(( last + 1 ))\"'/' main_test.go" \
    -n "buildkit test (test change)" './bin/build --no-resolve --target test --progress plain' \
    -n "go test (test change)" 'go test -v ./...'

sed "${sed_args[@]}" 's/var _ = .*/var _ = '"$(( last ))"'/' main_test.go
