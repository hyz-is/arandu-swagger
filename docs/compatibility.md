# Supported and unsupported behavior

This matrix describes the first public implementation, not planned behavior.

## OpenAPI surface

| Feature | Status | Notes |
| --- | --- | --- |
| OpenAPI 3.1.0 JSON | supported | canonical output format |
| JSON Schema 2020-12 dialect | supported | native Hesape schemas are snapshotted |
| YAML output | not supported | no YAML dependency is included |
| Info, contact, license, terms | supported | typed config and URL/email validation |
| Servers and variables | supported | template use/defaults are validated |
| Tags and external docs | supported | document and operation metadata |
| Paths and eight OpenAPI HTTP operations | supported | GET, PUT, POST, DELETE, OPTIONS, HEAD, PATCH, TRACE |
| Path, query, header, cookie parameters | supported | native schemas and component references |
| Request bodies and media types | supported | inline or referenced schemas, examples |
| Responses and response headers | supported | concrete/default responses and reusable responses |
| Schema, parameter, response, body, header, example components | supported | instance registry with conflict detection |
| Bearer, Basic, API key, mTLS | supported | typed security schemes |
| OAuth 2.0 and OpenID Connect model | supported | URLs, flows, scopes, and references validated |
| Package-owned OAuth callback/client setup | not supported | identity-provider integration remains application-owned |
| Security OR and AND requirements | supported | repeated `Security` for OR, `SecurityAll` for AND |
| Deprecation and `x-*` extensions | supported | Arandu metadata is preserved |
| Callbacks, webhooks, reusable path items | typed model only | no dedicated first-version registry/builder API |
| External `$ref` resolution | not supported | exact reference is emitted but never fetched |
| Full JSON Schema meta-schema validation | not supported | native builders are snapshotted as JSON objects; Hesape owns schema construction semantics |
| Document-level default security in `Config` | not supported | operation requirements are explicit; typed document can be adjusted after `Generate` |

## Routing behavior

| Arandu/Hesape route | Status | Result |
| --- | --- | --- |
| one concrete method | supported | one operation |
| `Match` with several concrete methods | supported | one operation per method |
| named route | supported | name becomes automatic `operationId` |
| unnamed route | supported | omit `operationId` or set it explicitly |
| `ANY` | explicit decision required | use `Methods` on a documented route |
| explicit `HEAD`, `OPTIONS`, `TRACE` | supported | represented as declared |
| implicit `HEAD` for `GET` | deliberately omitted | runtime convenience is not a declared route |
| fallback route | deliberately excluded | cannot be published through filters |
| hidden/ignored route | deliberately excluded | explicit per-instance decision |
| `/_arandu` route | excluded by default | `IncludeInternal` removes this one default exclusion |
| Swagger-owned endpoints | deliberately excluded | prevents recursive documentation |
| optional `{id?}` | rejected | OpenAPI path parameters cannot be optional |
| multi-segment `{path...}` | rejected | cannot preserve one-segment OpenAPI semantics |
| trailing `/{$}` anchor | supported | canonicalized to a trailing slash, not a `$` parameter |
| portable route constraint | supported | emitted conservatively as a string pattern |
| Go-only regex feature | rejected | avoids invalid ECMAScript output |
| domain metadata | extension only | emitted as `x-arandu-domain`; current router metadata does not prove enforcement |

The public constraint map retains the final regex but not whether a helper such
as `WhereNumber`, `WhereUuid`, `WhereUlid`, or `WhereIn` created it. Mapping the
pattern to integer, format, or enum would narrow the documented contract in
some cases, so the first version keeps the exact portable pattern on a string.

## Runtime and tooling

| Capability | Status |
| --- | --- |
| self-hosted Swagger UI | supported, pinned to 5.32.14 |
| CDN or runtime asset download | not used |
| same-origin CSP-safe initializer | supported |
| cross-origin Try it out under built-in CSP | not supported; proxy or explicit CSP/CORS decision required |
| programmatic and static generation | supported |
| deterministic JSON | supported |
| concurrent generation and registry updates | supported |
| registry- and route-aware production cache | supported |
| live reload / Joaju integration | not supported |
| Aru package CLI command | not supported; no extension contract exists |
| database, migrations, repository, policy, service | not used |
| runtime filesystem, outbound network, process execution | not used |

See [generation](generation.md) for diagnostics and
[assets/CSP](assets-csp.md) for browser boundaries.
