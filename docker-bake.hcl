group "ci" {
    targets = ["tidy", "shellcheck", "golangci-lint", "builder"]
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

target "builder" {
    target = "builder"
    # TODO: output = stdout?
}