# Installation

## Requirements

Arandu Swagger is a Go module for applications using Arandu Framework 0.43 or
newer. The released module currently targets Go 1.26 and uses the JSON Schema
types provided by Hesape 0.22.

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

Its `arandu.mod.toml` declares all four capabilities false:

| Capability | Why it is not needed |
| --- | --- |
| `network` | serving requests is not an outbound network call |
| `filesystem` | `go:embed` is compiled data; the package does not read or write runtime files |
| `exec` | no external process is started |
| `migrations` | the document is derived from in-memory route metadata |

Continue with [explicit bootstrap wiring](bootstrap.md), then document routes
through the small `swagger.Documenter` interface.
