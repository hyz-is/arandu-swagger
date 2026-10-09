# Upgrade Guide

## v0.4.2 — the Framework floor is 0.55

Nothing in this package's own API moved: `apidiff` against `v0.4.1` reports no
incompatible change. The one compatible change is inherited, because
`SwaggerViewData` embeds `hesape/view.Page`, which gained `FieldError`. What
moved is the minimum the package compiles against, which `go.mod` and
`arandu.mod.toml` now declare:

| | was | is |
|---|---|---|
| `github.com/arandu-io/framework` | `v0.47.1` | `v0.55.1` |
| `github.com/arandu-io/hesape` | `v0.41.1` | `v0.52.0` |
| `github.com/arandu-io/kyse` | not required | `v0.33.0` |

```bash
go get github.com/arandu-io/framework@v0.55.1
go get github.com/arandu-io/hesape@v0.52.0
go get github.com/hyz-is/arandu-swagger@v0.4.2
```

An application below those upgrades them first, following the Framework and
Hesape upgrade guides between the two versions: from Framework v0.55.0 the
session is configured only by what the session store reads and an unread
`SESSION_*` setting stops the boot, and from v0.54.0 a boolean setting that does
not read as one stops the boot. Hesape v0.52.0 removed the names it had
deprecated. `kyse` is required because the publishable topbar template imports
`kyse/icons`; an application that publishes the views already builds them with
`kyse`.

The generated document, the routes, the configuration and the four declared
capabilities are unchanged.

## v0.3.1

### The Framework floor is 0.47

`arandu.mod.toml` now requires `framework = ">= 0.47"`, and the module is built
against Framework `v0.47.0` and Hesape `v0.41.0`. A project on an older
framework upgrades before taking this release.

## v0.2.0

Version 0.2.0 raises the supported platform. Upgrade Arandu Framework to
`v0.45.0` and Hesape to `v0.24.0` or newer before taking this release; an
application below either floor will not build against it.

```bash
go get github.com/arandu-io/framework@v0.45.0
go get github.com/arandu-io/hesape@v0.24.0
go get github.com/hyz-is/arandu-swagger
```

`arandu.mod.toml` now declares `framework = ">= 0.45"`.

### The package API did not change

No exported symbol was added, removed, renamed, or given a different
signature. Configuration, the registry and its fluent builders, the typed
OpenAPI model, the security-scheme helpers, route names, `DefaultSpecPath`,
and `DefaultUIPath` are all unchanged, and a generated document is identical
for identical input. Existing wiring in `bootstrap/app.go` compiles and runs
as written.

### Nothing else moved

The four declared capabilities remain false: this package still performs no
outbound request, no runtime filesystem access, no process execution, and owns
no migration. Swagger UI stays embedded and version-pinned. The default
exclusions are unchanged, so internal, fallback, hidden, and Swagger-owned
routes do not become public through this upgrade.

Documentation endpoints are still authenticated by application middleware.
Security schemes describe authentication in the emitted document and never
enforce it.
