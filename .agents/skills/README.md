# Skills

These are the procedures an assistant follows when changing this package or
installing it in an Arandu application. Each procedure lives at
`.agents/skills/<name>/SKILL.md`; its frontmatter `name` must exactly match the
directory name so compatible coding assistants can discover it.

| Skill | Use it when |
| --- | --- |
| `swagger-package` | installing, wiring, configuring, or using the package in an application |
| `swagger-module` | changing configuration, route interpretation, builders, generation, cache, handlers, or embedded UI |
| `swagger-security` | changing exposure defaults, filters, middleware, examples, public errors, CSP, or interactive UI behavior |
| `swagger-release` | checking gates, dependencies, capabilities, vendored assets, licenses, changelog, or release tags |

The package procedure has an application-facing audience. It travels with the
module so a caller sees the explicit bootstrap wiring, instance registry, native
Hesape schema usage, endpoint controls, and programmatic generation API instead
of inventing a provider or discovery mechanism.

## Why these exist

Arandu Swagger sits beside the framework's route declaration surface without
changing it. Generic conventions are especially risky here: runtime handler
inspection, global documentation state, inferred authorization, remote UI
assets, and an implicit filesystem export would all contradict the package's
actual contracts and capability manifest.

The procedures keep those boundaries executable. Generation is deterministic,
route and registry snapshots are validated, public failures are sanitized,
third-party assets are versioned and hashed, and the final suite runs under the
race detector. A change should preserve those properties rather than merely
produce JSON that looks plausible.

## Adding a procedure

Keep a skill operational: state the situation that triggers it, the invariants
it must preserve, the files it should inspect, and the commands that prove the
result. A file that only says “read the documentation” does not change behavior.
