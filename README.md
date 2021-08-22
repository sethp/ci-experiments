# ci-experiments

Goal: avoid duplicating work between builds on different systems

Motivating questions:

- If I run the linter before pushing (or even integrated into my ide), why does CI also have to run it?
- Is there a way to get the goodness of a shared bazel remote build cache without needing the bazel "total build" system?
- Can we eliminate the "local vs CI" dichotomy?
