group "ci" {
    targets = ["tidy", "shellcheck", "golangci-lint", "app", "test-ci"]
}

target "tidy" {
    target = "tidy"
    # TODO: output = stdout?
}

target "shellcheck" {
    target = "shellcheck"
    # TODO: output = stdout?
}

target "golangci-lint" {
    target = "golangci-lint"
    # TODO: output = stdout?
}

target "app" {
    target = "app"
    # tags = ["app"]
    output = ["type=image"] # or type=push
}

/* This doesn't work for us; see the comment in this stage in the Dockerfile
target "test-ci" {
    target = "test"
    args = {
        CI = "1"
    }
}
*/

group "test-ci" {
    targets = ["test-ci-out", "test-ci-check"]
}

target "test-ci-check" {
    target = "test-ci-check"
}

target "test-ci-out" {
    target = "test-ci-out"
    output = ["type=local,dest=out"]
}
