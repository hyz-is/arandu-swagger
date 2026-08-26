# Security policy

## Supported versions

The latest released minor version of Arandu Swagger receives security fixes.
Published Go module versions are immutable; a fix is released under a new tag
rather than by moving an existing tag.

## Reporting a vulnerability

Report vulnerabilities privately through GitHub Security Advisories:

<https://github.com/arandu-io/swagger/security/advisories/new>

Do not open a public issue or pull request. Include the affected version,
configuration, route/documentation declaration, request, and the smallest
reproduction that demonstrates impact. Reports are acknowledged as soon as
practical, and confirmed fixes, releases, and advisories are coordinated.

## In scope

Security-sensitive behavior in this repository includes:

- an enabled documentation endpoint bypassing configured UI or specification
  middleware;
- a route excluded by package rules or `Hidden` appearing in a document;
- state or documentation leaking between independent module instances;
- registry or cache races that expose a partial or unrelated document;
- traversal or allowlist bypass in embedded asset delivery;
- HTML, JavaScript, header, or extension injection from documented metadata;
- a CSP regression that enables unintended script, style, frame, form, or
  network execution;
- accidental outbound calls, runtime file access, process execution, database
  access, or migration behavior;
- real credentials or request data being captured automatically by the package;
- a generation failure exposing internal route/schema diagnostics to an HTTP
  client.

Framework or Hesape vulnerabilities should be reported to the repository that
owns the affected behavior, unless Arandu Swagger makes the issue exploitable
through its own integration.

## Deployment responsibilities

Documentation is disabled by default, but `Enabled: true` makes the configured
routes reachable. The package does not implement its own authentication. The
application must attach appropriate `UIMiddleware` and `SpecMiddleware`, choose
route filters, and keep sensitive operations hidden.

`PersistAuthorization` can retain credentials in browser storage and is false
by default. `DisableTryItOut` removes interactive submit methods but does not
make route metadata non-sensitive. The built-in CSP allows only same-origin
connections; cross-origin request execution requires an explicit application
or proxy decision about CSP and CORS.

Never include passwords, tokens, API keys, captured headers, configuration
secrets, database records, or real user data in OpenAPI examples.

## Third-party assets

Swagger UI is vendored under its upstream license and isolated behind a fixed
asset allowlist. Provenance and hashes are in [THIRD_PARTY.md](THIRD_PARTY.md).
Report an upstream Swagger UI issue upstream as well as here when the embedded
version makes an Arandu application vulnerable.
