# Notes

- It's a little bit more tortured than writing a makefile / shell script, but not by too much so far
  - Invoking tools "locally" is a lot more tortured though

## todo

- read: https://github.com/moby/buildkit#exploring-llb
- registry cache vs gha?
  - is there an "and" in there that's useful?
- ideas for test cases (we're gonna need a bigger project):
  - source level change (single file vs multi-file?)
  - depenency change (add / remove / update)
  - CI system-level change (e.g. Dockerfile, docker-bake.hcl)
- cache-to mode=max ?
- how hard would it be to wrap this up in a github actions frontend that runs the bits concurrently but separates the output?
  - related: output mode = "stdout/err" (for what? the last container?)
  - output mode=none, too
