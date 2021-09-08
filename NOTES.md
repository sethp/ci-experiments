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
- debugging a failing target?
  - earthly has a good idea here: https://github.com/earthly/earthly/blob/main/docs/guides/debugging.md
  - related to ^: https://github.com/moby/moby/issues/40887
- (gha-specific) promote the output to the top-level somehow (so we can get e.g. annotations from golangci-lint)

## security

Caches are trusted implicitly; might might someone slip in an (overt) supply chain attack by:

- Sending a PR that does a --mount=type=cache for the go build cache
- Writes a "valid" go package that happens to include a backdoor
- Lets that get compiled into a layer that's exported to the --cache-to?

A possible mitigation would be to consider these things "advisory" and do a full rebuild before shipping artifacts? That's somewhat purpose-defeating, though, in a CD environment.
