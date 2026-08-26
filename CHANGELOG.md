# Changelog

All notable changes to Arandu Swagger are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and released versions follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Native `foundation.Module` integration with explicit bootstrap registration.
- Instance-owned, concurrency-safe route documentation and component registry.
- Typed OpenAPI 3.1 model and deterministic JSON generation.
- Route-local fluent documentation for operation metadata, parameters, request
  bodies, responses, examples, security, deprecation, visibility, and `x-*`
  extensions.
- Native Hesape JSON Schema snapshots and reusable OpenAPI components with
  conflict and reference validation.
- Conservative inference from public Arandu/Hesape route metadata, including
  multiple concrete methods, path constraints, module, domain, and deprecation.
- Safe route selection with explicit-documentation defaults, typed filters,
  internal-route control, and unconditional fallback/self exclusions.
- Bearer, Basic, API key, mutual TLS, OAuth 2.0, and OpenID Connect security
  scheme models.
- Lazy post-boot generation, registry- and route-aware production cache,
  programmatic generation, and `foundation.Diagnostic` reporting.
- Self-hosted Swagger UI 5.32.14 with embedded allowlisted assets, same-origin
  external initializer, strict CSP, MIME protection, and versioned cache policy.
- Compilable examples and focused guides for installation, configuration,
  wiring, route documentation, production operation, and compatibility.
- Context-aware JSON Schema validation, nested local schema references, and
  validated request-body property encodings.

### Security

- Documentation remains disabled by default.
- UI and specification accept independent application-owned middleware.
- Dynamic documentation responses are non-cacheable and generation failures do
  not expose internal diagnostics to HTTP clients.

### Limitations

- JSON is the only generated representation; YAML is not included.
- Optional path parameters, multi-segment wildcards, unnormalized qualified
  bindings, ambiguous `ANY` routes, unsupported methods, and non-portable route
  regexes are rejected rather than represented inaccurately.
- Cross-origin Try it out is blocked by the built-in same-origin CSP unless the
  application makes an explicit proxy/CSP/CORS decision.
- No Aru command is installed because there is no package CLI extension
  contract.
