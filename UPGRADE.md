# Upgrade Guide

## v0.2.0

Version 0.2.0 raises the supported platform. Upgrade Arandu Framework to
`v0.43.0` and Hesape to `v0.22.0` or newer before taking this release; an
application below either floor will not build against it.

```bash
go get github.com/arandu-io/framework@v0.43.0
go get github.com/arandu-io/hesape@v0.22.0
go get github.com/hyz-is/arandu-swagger
```

`arandu.mod.toml` now declares `framework = ">= 0.43"`.

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
