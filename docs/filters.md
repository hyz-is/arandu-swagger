# Filters and exclusions

The safe default is an allowlist formed by explicit documentation calls: a
route appears only after `docs.Route(route)`. Set `IncludeUndocumented` only
when a basic inventory of the remaining eligible routes is intentional.
Undocumented operations contain inferred routing metadata and a generic
default response; they never receive invented bodies, status codes, or
security.

## Package exclusions

These rules are applied before caller filters:

- fallback routes are excluded;
- routes owned by the `swagger` module are excluded;
- the effective UI, initializer, asset, and specification paths are excluded;
- routes marked `Hidden` or `Ignore` are excluded;
- `/_arandu` is excluded unless `IncludeInternal` is true.

Caller filters cannot add back fallback, Swagger-owned, or explicitly hidden
routes. `IncludeInternal` only removes the package default ban on the internal
prefix; the route must still pass documentation and caller filter rules.

## Typed route filters

```go
config.Filter = swagger.RouteFilter{
	IncludePrefixes: []string{"/api"},
	ExcludePrefixes: []string{"/api/admin"},
	IncludeModules:  []string{"users", "billing"},
	ExcludeNames:    []string{"users.export"},
	IncludeMethods:  []string{http.MethodGet, http.MethodPost},
	ExcludeDomains:  []string{"internal.example.test"},
	ExcludePredicate: func(route *fhttp.Route) bool {
		return strings.HasSuffix(route.GetName(), ".debug")
	},
}
```

Every non-empty include list is an allowlist for that dimension. A route must
pass all active include dimensions. Exclusions run after inclusions and win.
`IncludePredicate` must return true; `ExcludePredicate` must return false.

| Dimension | Match behavior |
| --- | --- |
| prefixes | `strings.HasPrefix` on the full route URI |
| names | exact route name |
| modules | exact public module name |
| methods | exact method, case-insensitive |
| domains | exact public domain metadata |
| predicates | application-defined boolean decision |

Blank values and the same value in both include and exclude lists are rejected
when the enabled config is validated. Prefixes must begin with `/` and remain
static.

## Hiding one operation

```go
docs.Route(route).Hidden()
```

`Ignore` is an alias. Hidden is attached to the documented route pointer and
therefore remains instance-owned; it does not mutate Hesape route actions or
global state.
