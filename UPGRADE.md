# Upgrade Guide

## v0.4.9 — webhooks in the generated document

`apidiff` against `v0.4.8` reports compatible additions and nothing else:
`Webhook`, `Registry.Webhooks`, `Module.Webhooks` and `MediaSchema`. The
generated document of an application that does not call them is the same: it
has no `webhooks` key.

### Webhooks a static export added by hand can be registered

An application that wrote `Document.Webhooks` after `Generate` -- so its static
contract had the section and the served `/docs` page did not -- can register
the same values on the registry and the module, and both carry them:

```go
// Before
doc, err := swagger.Generate(router.Routes(), registry, config)
doc.Webhooks = swagger.Paths{"order.paid": &swagger.PathItem{Post: &swagger.Operation{ /* ... */ }}}

// After: one function of the application, called with the registry of the
// export and with the module bootstrap/app.go builds.
err := registry.Webhooks(swagger.Webhook{
	Name:      "order.paid",
	Body:      []*swagger.Media{swagger.JSONRef("OrderEvent")},
	Responses: swagger.Responses{"2XX": {Description: "Received."}},
})
```

Generation now validates what a hand-written section was never checked for:
each body, parameter and security reference must name a registered component,
each response key must be `default`, a status or a `1XX`-`5XX` range, and a
webhook's operationId must not repeat one a route or another webhook uses. A
schema assigned straight into `Document.Components.Schemas` must be registered
with `SchemaComponent` (or derived with `Models`) for those references to
resolve.

## v0.4.8 — components derived from the Go types an application answers with

`apidiff` against `v0.4.7` reports compatible additions and nothing else:
`Model`, `ModelOf`, `SchemasFromModels`, `Registry.Models` and
`Module.Models`. Nothing an application already registers changes, and the
generated document of an application that does not call them is the same.

### A payload described twice can be described once

An application that kept a schema for a payload in a string handed to
`SchemaFromJSON`, or rebuilt it with `jsonschema` builders beside the struct it
encodes, can derive it from the struct:

```go
// Before
schema, err := swagger.SchemaFromJSON([]byte(`{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}`))
err = docs.SchemaComponent("Product", schema)

// After
err := docs.Models(swagger.ModelOf[resources.Product]("Product"))
```

Compare the generated document before and after. The derivation writes what
`encoding/json` writes, so a difference is a place where the hand-written
schema and the answer already disagreed: a field without `omitempty` is
required, a pointer is nullable, and `int64` is `integer` with no `format`.
The `doc`, `format` and `enum` struct tags carry the description, the format
and the values a type cannot say.

## v0.4.7 — the docs page loads images only from the origins it is given

`apidiff` against `v0.4.6` reports one compatible addition and nothing else:
`Config.ImageOrigins`. What moved is the `img-src` directive of the policy
the UI route answers with, and what `New` accepts as a logo or favicon URL.

### `img-src` no longer admits every https origin

Since v0.4.0 both pages answered with `img-src 'self' data: https:`, so the
docs page could load an image from any https origin, and an application could
not narrow it. The directive is now built from the configuration: the page's
own origin and `data:` URIs, and beyond them only the origins it names. With
no image from another origin configured, both pages answer with:

```text
Content-Security-Policy: default-src 'none'; style-src 'self'; style-src-attr 'unsafe-inline'; script-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'
```

with `form-action 'none'` on the embedded page, as before. Every other
directive is unchanged.

The origin of an absolute `Theme.Logo.URL` and of an absolute `Theme.Favicon`
is added on its own, as scheme, host and port, without the path. A
configuration with `Theme.Logo.URL: "https://cdn.example.com/brand/logo.svg"`
answers with `img-src 'self' data: https://cdn.example.com`, so an
application whose logo or favicon is an https URL keeps working without a
change. A same-origin path or a `data:` URI adds nothing and needs nothing.

### Images from anywhere else are listed in `ImageOrigins`

An image the page draws from another origin that is not the logo or the
favicon is now blocked by the browser. That covers an image in an API or
operation description, a `url(...)` in `Theme.CustomCSS`, and an image in a
layout the `docs.swagger` view extends. An application relying on one lists
its exact origin:

```go
swagger.Config{
	// ...
	ImageOrigins: []string{"https://img.example.org", "https://cdn.example.com:8443"},
}
```

Each entry is an `https` origin and nothing more. There is no way to admit
every https origin again.

### `New` refuses an image URL the directive cannot name exactly

When documentation is enabled, `New` and `Config.Validate` now return an error
naming the field for:

- a `Theme.Logo.URL` or `Theme.Favicon` that is an absolute URL not using
  `https`, such as `http://…` or `javascript:…`, or a protocol-relative
  `//host/…`;
- one that carries credentials, has a wildcard or IPv6-literal host, an
  invalid port, or whitespace or control characters;
- an `ImageOrigins` entry that is not an `https` origin, or that carries a
  wildcard, a path, a query or a fragment.

An `http` logo or favicon on another origin was already blocked by the
browser under `https:`; one with credentials or an IPv6-literal host was not,
and is now refused at boot instead. `Generate` reports the same errors. A disabled `Config` stays valid whatever it names.

## v0.4.6 — the docs view works inside the application's layout

`apidiff` against `v0.4.5` reports no change at all. What moved is how the UI
route answers when a project's `docs.swagger` view extends its own layout,
and what it does when that view cannot be drawn.

### Forms in the layout submit to the application

The view now answers with `form-action 'self'` instead of `'none'`:

```text
Content-Security-Policy: default-src 'none'; style-src 'self'; style-src-attr 'unsafe-inline'; script-src 'self'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'
```

Every other directive is unchanged. Before, the browser refused a form in the
layout that it submitted natively, such as signing out. The embedded page
keeps `form-action 'none'`, because it draws no form a browser submits.

The route still sets `Content-Security-Policy`, `Referrer-Policy`,
`X-Content-Type-Options` and `Cache-Control` itself, so it replaces what a
middleware wrote before it, including the default security headers a new
application runs on every route. `docs/assets-csp.md` used to say a
middleware could replace the policy. It now says what the code does, and
that every other header such a middleware sets reaches the browser
unchanged.

### The view is handed the application's page

The `view.Page` inside `SwaggerViewData` now starts from `view.New`. It
carries the CSRF token the forms middleware issued, the login, logout and
register links the route table names, and whether somebody is signed in. The
route used to build that page by hand without any of these. A layout drawn
around the view then had no sign-out form, and the CSRF check refused every
form it did draw with 419.

The module reads no session. Signed-in state comes from the subject on the
request, so a project that wants the layout to show who is signed in mounts
the middleware that loads it on the UI route:

```go
swagger.Config{
	// ...
	UIMiddleware: []swagger.Middleware{middleware.LoadSubject(sessions)},
}
```

`middleware.RequireAuth` loads the subject too, and also keeps guests out.
Without either, the page is drawn for a guest. The fields the module decides
are as before: the title, the description, the brand (`Theme.Logo.Alt`, else
`Theme.Title`, else `Config.Title`, never `APP_NAME`), the home link, and the
canonical address and path, which are `UIPath`.

### A view that fails is no longer covered by the embedded page

Before, the route served the embedded page whenever `ctx.View` failed. It now
serves it only when no view named `ViewName` is registered. Every other error
goes to the router, which answers with the application's error page:

- a published view that fails to render, which used to answer 200 with the
  embedded page;
- a registered view with no view layer wired;
- a connection that breaks after the view started writing, after which the
  route used to write a second status and page.

A project that never published the views is answered as before.

### A project that published the views

The published views changed only in the comment at the top of
`swagger.kyse.go`, which is now in English. A project that published them has
nothing to publish again. To show the signed-in state in a layout around the
page, add the `UIMiddleware` line above.

## v0.4.5 — the Framework floor is 0.56

Nothing in this package's own API moved: `apidiff` against `v0.4.4` reports no
change at all. What moved is the minimum the package compiles against, which
`go.mod` and `arandu.mod.toml` now declare:

| | was | is |
|---|---|---|
| `github.com/arandu-io/framework` | `v0.55.1` | `v0.56.0` |
| `github.com/arandu-io/hesape` | `v0.52.0` | `v0.54.0` |
| `github.com/arandu-io/kyse` | `v0.33.0` | `v0.34.1` |

```bash
go get github.com/arandu-io/framework@v0.56.0
go get github.com/arandu-io/hesape@v0.54.0
go get github.com/hyz-is/arandu-swagger@v0.4.5
```

An application below those upgrades them first, following the Framework,
Hesape and Kyse upgrade guides between the two versions: Framework v0.56.0
removes `config.Config.SessionTTL`, so `config.Load` no longer reads
`SESSION_TTL`, and puts `APP_NAME` on every request, which Hesape v0.54.0's
`view.New` reads into `Page.AppName`. Hesape v0.53.0 and Kyse v0.34.0 change
the markup `OneTimeCode` and `Masked` draw and the script that mounts it.

The documentation page does not take that name. The UI route still hands the
`docs.swagger` view the brand it chose before, `Theme.Logo.Alt`, else
`Theme.Title`, else `Config.Title`, so a page that read "Billing API" keeps
reading it whatever `APP_NAME` says.

The published views are unchanged, and `aru view:build` compiles them to the
same code it did before. A project that published them has nothing to publish
again. The generated document, the routes, the configuration and the four
declared capabilities are unchanged.

## v0.4.4 — the published views write their targets, keep the page policy, and follow the locale

`apidiff` against `v0.4.3` reports two compatible additions and nothing else:
`SwaggerViewData.Translations` and `SwaggerViewData.Translate`. What moved is
what the UI route answers and what the published views draw.

### The view answers with the page's security headers

Since v0.4.0 only the embedded page set the headers `docs/assets-csp.md`
documents; a `docs.swagger` view answered with whatever the application's
middleware sent. The UI route now sets them before either page is drawn, so
the view answers with the same ones:

```text
Content-Security-Policy: default-src 'none'; style-src 'self'; style-src-attr 'unsafe-inline'; script-src 'self'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'
Referrer-Policy: no-referrer
Cache-Control: no-store
X-Content-Type-Options: nosniff
```

The handler sets them, so they replace a policy an application middleware set
before calling it. The published `docs.swagger` page, which includes the
topbar and the container, loads only same-origin files and renders unchanged.
A view an application changed to extend its own layout now runs under this
policy too: that layout's scripts, stylesheets and fonts must be same-origin
files, an inline `<script>` or `on…=` handler does not run, and a form in it
cannot submit. The `img-src` value is not new: the code has sent `https:`
since v0.4.0, and the document now says so.

### The topbar writes its link targets

The published topbar wrote `@if(.LogoTarget != "")target="…"@endif` inside
the logo and back links. A kyse directive takes a whole line, so that text
reached the browser as written and `Theme.Logo.Target` and `Theme.BackTarget`
never became attributes. The links now open across lines, each conditional
attribute in its own `@if` block:

```html
<a
	href="{{ .HomeURLOrDefault() }}"
	class="arandu-swagger-logo-link"
	@if(.LogoTarget != "")
		target="{{ .LogoTarget }}"
	@endif
>
```

### The header and the toggle label follow the locale

The published header drew its breadcrumb, eyebrow and Authorize button in
fixed Portuguese, and its breadcrumb read "Workspace"; the topbar's toggle
label was Portuguese as well. They now go through `SwaggerViewData.Translate`,
which reads `Config.Locale` and `Config.Translations`: English by default, the
package's Portuguese for `pt` and `pt-BR`, and an entry in
`Config.Translations` before either.

| key | `en` (default) | `pt-BR` |
|---|---|---|
| `Home` | Home | Início |
| `API documentation` | API documentation | Documentação da API |
| `Developer integration` | Developer integration | Integração e desenvolvedor |
| `Authorize` | Authorize | Autorizar |
| `Toggle theme` | Toggle theme | Alternar tema |

The eyebrow is uppercased by the `uppercase` class instead of being written in
capitals. The header's Authorize button lost its inline `onclick`, which the
policy above refuses to run; it carries `data-arandu-swagger-authorize`, and
the initializer binds it.

The embedded page's toggle label uses the same lookup. It was Portuguese for
any locale starting with `pt`; it is now Portuguese for `pt` and `pt-BR`, the
locales the Swagger UI dictionary already answered, so `pt-PT` draws
"Toggle theme" unless `Config.Translations` names it.

### A project that published the views

A project renders its own copies, and none of this reaches them until they
are published again. Under the new headers, a copied header's Authorize button
no longer opens the dialog, because its `onclick` does not run, and a copied
topbar keeps writing the `@if` text into its links. Publish the views again
with `aru vendor:publish` and carry over any edits, or make the same changes
by hand: the multi-line links above, `data-arandu-swagger-authorize` in place
of the `onclick`, and `{{ .Translate("…") }}` for each fixed string.

```bash
go get github.com/hyz-is/arandu-swagger@v0.4.4
```

## v0.4.3 — the documentation page names no product by default

Nothing in the package API moved: `apidiff` against `v0.4.2` reports no change
at all. What moved is what the UI route hands the `docs.swagger` view when
`Config.Theme.Logo` is nil. Since v0.4.0 it handed the defaults of the
application the theming was first written for, and every other application
showed them on its documentation page:

| `SwaggerViewData` field | was | is |
|---|---|---|
| `AppName` (the topbar brand) | `Peráta` | `Theme.Title`, else `Config.Title` |
| `HomeURL`, `LogoHref` | `/workspaces` | `/` |
| `LogoURL` | `/favicon.svg` | empty: no logo is drawn |

The two helpers that carried the same defaults follow them:
`BrandOrDefault` falls back to the page `Title` instead of `Peráta`, and
`LogoURLOrDefault` returns the empty string instead of `/favicon.svg`. The
published topbar draws the logo image only when `LogoURLOrDefault` is not
empty, and the published container's class is now `arandu-swagger-container`
instead of `perata-swagger-container`.

A configured `Theme.Logo` reaches the view exactly as before, so an
application that sets it sees no difference. The embedded page, served when
the view is not registered, already drew no logo without one and is unchanged.

An application that relied on the old defaults keeps them by naming them:

```go
Theme: swagger.Theme{
	Logo: &swagger.ThemeLogo{
		URL:  "/favicon.svg",
		Href: "/workspaces",
		Alt:  "Peráta",
	},
},
```

`Logo.URL` is required whenever `Logo` is set; a brand without an image is
`Theme.Title`.

A project that published the views keeps its own copies, and they are what it
renders. A copied topbar still draws `<img src="{{ .LogoURLOrDefault() }}">`
unconditionally, so with no `Theme.Logo` it now draws an image with an empty
source. Set `Theme.Logo`, or publish the views again with `aru vendor:publish`
and carry over any edits; a stylesheet that targeted
`.perata-swagger-container` targets `.arandu-swagger-container` after that.

```bash
go get github.com/hyz-is/arandu-swagger@v0.4.3
```

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

## v0.4.1 — skills only

Written after the fact, on 2026-10-09, with v0.4.3: the release shipped with
no entry here.

No Go file changed: the package API, the generated document, the routes and
the configuration are those of v0.4.0. The release changed only
`.agents/skills/`. It added the `arandu-ecosystem` skill, and it marked
`swagger-package` with `metadata.audience: app` in its frontmatter, which is
what `aru skills:sync` reads to copy that one skill into an application whose
`go.mod` requires this package. An application that runs `aru skills:sync`
after upgrading receives `swagger-package`; the release, module and security
procedures stay in this repository.

## v0.4.0 — theming, publishable views, and a different schema dialect

Written after the fact, on 2026-10-09, with v0.4.3: v0.4.0 was tagged on
2026-09-22 with no entry here, although it changed the generated document and
the UI route. The facts below are read from `git diff v0.3.1 v0.4.0`.

### The default `jsonSchemaDialect` changed

The document's `jsonSchemaDialect` was
`https://json-schema.org/draft/2020-12/schema` and is now
`https://spec.openapis.org/oas/3.1/dialect/base`, the OpenAPI 3.1 base
dialect. A document generated from identical input therefore differs in that
one field. A consumer that compared or pinned the old value sets it back
explicitly:

```go
swagger.Config{
	JSONSchemaDialect: "https://json-schema.org/draft/2020-12/schema",
}
```

### The UI route tries a view first

The UI endpoint became an action. It renders the view named by
`Config.ViewName` (else `Config.Theme.ViewName`, else `docs.swagger`) with a
`SwaggerViewData`, and falls back to the embedded page when the view is not
registered or fails to render. Three consequences are observable:

- An application with a view named `docs.swagger` serves that view at the UI
  path, whatever the view was written for.
- The `Content-Security-Policy` and `Referrer-Policy` headers documented for
  the UI are set by the embedded page only. A rendered view answers with the
  headers of the application's renderer and middleware, until v0.4.4; see
  that entry.
- A request that sends `Accept: application/vnd.arandu.view+json` receives
  the view name and the `SwaggerViewData` as JSON instead of markup, as every
  `ctx.View` does.

The module implements `foundation.Publishable` and offers four views,
`resources/views/docs/{swagger,topbar,container,header}.kyse.go`. Until v0.4.3,
the data handed to them named a product, a home path and a logo when no
`Theme.Logo` was configured; see the v0.4.3 entry.

### The page is themed by default

`Config` gained `Locale`, `Translations`, `ViewName`, `JSONSchemaDialect` and
`Theme` (with `ThemeLogo` and `HTMXConfig`). `Config.HasTheme` reports true for
the zero `Theme`, because the theme toggle is on unless
`Theme.DisableThemeToggle` is set. So with no theming configured the module
registers `GET <UIPath>/theme.css`, and the embedded page links it and draws a
topbar with a "Back to site" link to `/` and a light/dark toggle. Setting
`Theme.CustomJS` registers `GET <UIPath>/theme.js` as well. The UI
content-security policy's `img-src` gained `https:`, so a logo may be an
absolute HTTPS URL.

`New` now rejects a `SpecPath` equal to `<UIPath>/theme.css` or
`<UIPath>/theme.js`, and a `Theme.Logo` whose `URL` is blank.

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
