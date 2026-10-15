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

The HTML contains no inline script, inline event handler or style block. The
UI route sends the same headers whichever page answers it, the `docs.swagger`
view an application published or the embedded page served when no view by
that name is registered:

```text
Cache-Control: no-store
X-Content-Type-Options: nosniff
Referrer-Policy: no-referrer
```

The two pages answer with one policy that differs in its last directive,
`form-action`, and in nothing else. The `docs.swagger` view answers with:

```text
default-src 'none'; style-src 'self'; style-src-attr 'unsafe-inline'; script-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'
```

and the embedded page with:

```text
default-src 'none'; style-src 'self'; style-src-attr 'unsafe-inline'; script-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'
```

`script-src`, `style-src`, and `connect-src` require the same origin. Inline
scripts and inline style blocks remain forbidden. Swagger UI 5.x applies style
attributes at runtime, so the policy contains the narrow CSP3
`style-src-attr 'unsafe-inline'` exception required for the vendored UI to render;
it does not widen `script-src` or `style-src`. `frame-ancestors 'none'` prevents
framing, and `base-uri` is disabled. Try it out sends its requests with
`fetch`, which `connect-src` governs, so it works only against a same-origin
server under the built-in policy. A cross-origin API requires an explicit
application or proxy decision about both CSP and CORS.

Those are the policies of a configuration that loads no image from another
origin. `img-src` admits the page's own origin and `data:` URIs, and beyond
them exactly the origins the configuration names, never a whole scheme such
as `https:`:

- the origin of `Theme.Logo.URL` when it is an absolute URL;
- the origin of `Theme.Favicon` when it is an absolute URL;
- every entry of `ImageOrigins`, for images the page draws from elsewhere,
  such as one in an API description, a custom stylesheet, or a layout the
  view extends.

Each is written once, as scheme, host and port, in that order, after
`'self' data:`. A logo at `https://cdn.example.com/brand/logo.svg` and
`ImageOrigins: []string{"https://img.example.org:8443"}` give:

```text
img-src 'self' data: https://cdn.example.com https://img.example.org:8443
```

The logo's path is not written into the directive. `New` refuses an absolute
logo or favicon URL that does not use `https`, a protocol-relative one, one
carrying credentials, and one whose host is a wildcard or an IPv6 literal. It
refuses an `ImageOrigins` entry that is not an `https` origin or that carries
a wildcard, a path, a query or a fragment. A same-origin path and a `data:`
URI add nothing to the directive. Any other image the page refers to is
blocked by the browser.

The view's policy covers the whole page the view draws. A published view
changed to extend an application layout therefore answers under it too: that
layout's scripts, stylesheets and fonts must be same-origin files, and an
inline `<script>` or `on…=` handler does not run. The published header's
Authorize button carries `data-arandu-swagger-authorize` and is bound by the
external initializer for that reason. A form in the layout may submit to the
application's own origin and to no other: that is what `form-action 'self'`
admits, so a sign-out form in the layout's header reaches the application
instead of being refused by the browser. The page the view is handed starts
from the one `view.New` builds for the application's own screens, so the
layout draws the same CSRF token, navigation links and signed-in state there
as everywhere else; the module sets only its title, description, brand, home
link, canonical address and path. The module reads no session: the
signed-in state is the subject the application's `UIMiddleware` loads onto
the request, `middleware.LoadSubject` or `middleware.RequireAuth`, and
without one the layout is drawn for a guest. The embedded page draws no form a
browser submits, because Swagger UI handles its Authorize dialog in script, so
its `form-action` stays `'none'`.

The UI route sets these four headers itself, so a value an application
middleware set for any of them before calling the route is replaced. That
includes the default security headers a new application runs on every route,
which write a policy of their own on every answer: that policy has no
`style-src-attr` exception for Swagger UI and admits every same-origin
resource type through `default-src 'self'`, so the route answers with its
own. Every other header such a middleware sets, `X-Frame-Options`,
`Permissions-Policy` or `Strict-Transport-Security` among them, reaches the
browser unchanged. A reverse proxy mounting the application under a prefix
must preserve the configured absolute routes or configure `UIPath` and
`SpecPath` to their externally reachable paths.

The embedded page answers only when no view named `ViewName` is registered.
Any other error drawing the view is returned to the router and answered with
the application's error page rather than covered by the embedded page: a
published view that fails to render, a registered view with no view layer
wired to draw it, and a connection that breaks after the view has started
writing. Nothing is written after the view has written its status.

## License provenance

The upstream distribution license, notice, source tag, and SHA-256 hashes are
recorded in [`THIRD_PARTY.md`](../THIRD_PARTY.md). Paths are version-pinned,
not content-addressed; changing any vendored bytes requires a version-path and
provenance review. Keep those files and update the record atomically when
changing the vendored version.
