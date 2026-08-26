# Embedded assets and CSP

Swagger UI is vendored at version 5.32.14 and compiled into the Go binary with
`go:embed`. Opening `/docs` does not contact a CDN, download an asset, invoke
Node.js, or read an arbitrary runtime file.

## Served files

The UI uses a fixed allowlist under the effective UI path:

| Relative path | Content type |
| --- | --- |
| `assets/5.32.14/swagger-ui.css` | `text/css; charset=utf-8` |
| `assets/5.32.14/swagger-ui-bundle.js` | `text/javascript; charset=utf-8` |
| `assets/5.32.14/swagger-ui-bundle.js.LICENSE.txt` | `text/plain; charset=utf-8` |
| `assets/5.32.14/LICENSE` | `text/plain; charset=utf-8` |
| `assets/5.32.14/NOTICE` | `text/plain; charset=utf-8` |

No directory listing or arbitrary embedded path is exposed. Unknown assets
return 404. Versioned files carry:

```text
Cache-Control: public, max-age=31536000, immutable
X-Content-Type-Options: nosniff
```

The HTML and external initializer use `Cache-Control: no-store`. Their resource
URLs are same-origin and derived from `UIPath` and `SpecPath`. The initializer
also sets `validatorUrl: null`, preventing Swagger UI from sending the document
to its default remote validator.

## Content Security Policy

The HTML contains no inline script or style. The module sends:

```text
default-src 'none'; style-src 'self'; style-src-attr 'unsafe-inline'; script-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'
```

`script-src`, `style-src`, and `connect-src` require the same origin. Inline
scripts and inline style blocks remain forbidden. Swagger UI 5.x applies style
attributes at runtime, so the policy contains the narrow CSP3
`style-src-attr 'unsafe-inline'` exception required for the vendored UI to render;
it does not widen `script-src` or `style-src`. `frame-ancestors 'none'` prevents framing. `base-uri` and
`form-action` are disabled. Try it out therefore works only against a
same-origin server under the built-in policy. A cross-origin API requires an
explicit application or proxy decision about both CSP and CORS.

An application middleware may add stricter headers. If it replaces this CSP,
it must still allow the versioned same-origin stylesheet, bundle, initializer,
and the specification request. A reverse proxy mounting the application under
a prefix must preserve the configured absolute routes or configure `UIPath`
and `SpecPath` to their externally reachable paths.

## License provenance

The upstream distribution license, notice, source tag, and SHA-256 hashes are
recorded in [`THIRD_PARTY.md`](../THIRD_PARTY.md). Paths are version-pinned,
not content-addressed; changing any vendored bytes requires a version-path and
provenance review. Keep those files and update the record atomically when
changing the vendored version.
