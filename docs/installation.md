# Installation

## Requirements

Arandu Swagger is a Go module for applications using Arandu Framework 0.45 or
newer. The released module currently targets Go 1.26 and uses the JSON Schema
types provided by Hesape 0.24.

Install it with the Go toolchain:

```bash
go get github.com/hyz-is/arandu-swagger
```

Import the package where the application is composed:

```go
import swagger "github.com/hyz-is/arandu-swagger"
```

## There is no install-time work

The package owns no database table and does not implement
`foundation.Migratable`. Do not add a migration command to the deployment for
this package. Swagger UI assets are already embedded in the compiled Go binary,
so there is no npm, Node.js, download, copy, or asset build at install or
runtime.

Its `arandu.mod.toml` declares one capability, the filesystem, and only the
`openapi:export` console command uses it:

| Capability | Declared | Why |
| --- | --- | --- |
| `network` | false | serving requests is not an outbound network call |
| `filesystem` | true | `openapi:export` writes the contract files and `--check` reads them; serving requests reads no file, and `go:embed` is compiled data |
| `exec` | false | no external process is started |
| `migrations` | false | the document is derived from in-memory route metadata |

Continue with [explicit bootstrap wiring](bootstrap.md), then document routes
through the small `swagger.Documenter` interface.
