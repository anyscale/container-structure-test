# Bazel smoke test

Verifies that the `container_structure_test` bazel rule exposed by this project
works properly, exercising HEAD's rule against a from-source build of HEAD's
binary (`//cmd/container-structure-test`) rather than a downloaded release.

## Running

```sh
cd bazel/test
bazel test //...                                   # bzlmod (default)
bazel test //... --enable_bzlmod=false --enable_workspace  # WORKSPACE mode
```

Both module systems register the from-source `structure_test_toolchain`
(`@container_structure_test//bazel:from_source_structure_test_toolchain`) ahead
of the downloaded release toolchains, so resolution prefers HEAD's binary. This
means changes to either the rule (`bazel/container_structure_test.bzl`) or the
Go binary are tested together, with no manual binary-swapping.

## How the from-source toolchain is wired

- **bzlmod**: `MODULE.bazel` brings `rules_go`/`gazelle` and registers the
  from-source toolchain. Go module deps come from the isolated `go_deps`
  extension reading the root `//:go.mod`.
- **WORKSPACE**: `WORKSPACE.bazel` reconstructs the same wiring (the repo itself
  is bzlmod-only, with an empty root `WORKSPACE.bazel`). Go module deps live in
  the generated `go_deps.bzl` macro; regenerate it after `go.mod` changes per
  the header comment in that file.

Real consumers do not use the from-source toolchain — they get the downloaded
release toolchain via `container_structure_test_register_toolchain`
(`repositories.bzl`) and need no `rules_go`.
