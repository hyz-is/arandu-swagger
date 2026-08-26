---
name: swagger-release
description: Validate and release Arandu Swagger, including gates, module metadata, capabilities, dependencies, Swagger UI provenance, changelog, and tags.
license: MIT
---

# Releasing Arandu Swagger

## Required gates

Run from this repository with the surrounding Go workspace disabled:

```sh
export GOWORK=off
gofmt -l $(find . -name '*.go' -not -path '*/testdata/*' -not -name '*.kyse.go')
go build ./...
go vet ./...
go test -race -count=1 ./...
```

All four commands must exit zero, and `gofmt -l` must print nothing. `aru doctor`
is not a package gate; it expects an Arandu application containing `main.go` and
`arandu.toml`.

## Manifest and dependencies

The normal manifest is:

```toml
[permissions]
network = false
filesystem = false
exec = false
migrations = false
```

Embedded build-time assets do not imply runtime filesystem access, and serving
HTTP does not imply outbound network access. Keep direct dependencies limited to
the pinned Arandu `framework` and `hesape` modules unless a reviewed feature
cannot be implemented with the standard library or existing ecosystem.

## Third-party assets

Before changing Swagger UI:

1. Select an immutable upstream release tag.
2. Verify the upstream license and NOTICE.
3. Vendor only the minimum runtime files.
4. Retain the license, notice, and extracted bundle license comments.
5. Update the versioned embed path and `internal/ui.Version`.
6. Recompute every SHA-256 in `THIRD_PARTY.md`.
7. Run the UI asset and feature tests.

Do not claim provenance for an unidentified project. The item reported as
“Bolt Swagger” was not sufficiently identifiable and contributed no code or
assets; preserve that explicit statement unless verifiable source evidence is
added.

## Publication

Update `CHANGELOG.md`, confirm README examples compile, and verify that every
exported symbol has Go documentation. Go module versions and vendored release
paths are immutable once published; correct a release with a new tag rather than
moving an existing one.
