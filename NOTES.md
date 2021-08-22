# Notes

- It's a little bit more tortured than writing a makefile / shell script, but not by too much so far
  - Invoking tools "locally" is a lot more tortured though

## todo

- read: https://github.com/moby/buildkit#exploring-llb
- registry cache vs gha?
  - is there an "and" in there that's useful?
- sharing the `mount=type=cache` stuff; that's where a lot of the partial intermediate work is being saved
- ideas for test cases (we're gonna need a bigger project):
  - source level change (single file vs multi-file?)
  - depenency change (add / remove / update)
  - CI system-level change (e.g. Dockerfile, docker-bake.hcl)
- cache-to mode=max ?
- how hard would it be to wrap this up in a github actions frontend that runs the bits concurrently but separates the output?
  - related: output mode = "stdout/err" (for what? the last container?)
  - output mode=none, too
- ooh, "canceled" when one target of many fails, that's kind of grim
  - see: https://github.com/sethp/ci-experiments/pull/1/commits/4cec11562fd3ac885c0405515fa66e7157546215
  - in ^ the "go build" saw the unused import before the linter was ready to run and the lint was canceled
  - related: https://github.com/earthly/earthly/issues/16 (but no idea how it was solved)
- debugging a failing target?
  - earthly has a good idea here: https://github.com/earthly/earthly/blob/main/docs/guides/debugging.md
  - related to ^: https://github.com/moby/moby/issues/40887
- (gha-specific) promote the output to the top-level somehow (so we can get e.g. annotations from golangci-lint)
