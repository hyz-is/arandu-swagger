# Third-party notices

## Incorporated software

### Swagger UI

Arandu Swagger vendors the browser distribution of [Swagger UI](https://github.com/swagger-api/swagger-ui) at the immutable upstream tag [`v5.32.14`](https://github.com/swagger-api/swagger-ui/tree/v5.32.14). The files were downloaded directly from that tag and are distributed under the [Apache License 2.0](internal/ui/assets/5.32.14/LICENSE).

The upstream notice is retained verbatim in [NOTICE](internal/ui/assets/5.32.14/NOTICE):

> swagger-ui  
> Copyright 2020-2021 SmartBear Software Inc.

The minified bundle's extracted third-party license comments are retained in `swagger-ui-bundle.js.LICENSE.txt`. The bundle itself retains the comment that points to that file. The CSS retains its upstream license comments.

| Vendored file | SHA-256 |
| --- | --- |
| `internal/ui/assets/5.32.14/LICENSE` | `cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30` |
| `internal/ui/assets/5.32.14/NOTICE` | `0d20d1adef18aee3f40dd258172155521ce702ac445cb5f7b7d60ed32dad2fb2` |
| `internal/ui/assets/5.32.14/swagger-ui-bundle.js` | `16d93d5cc19e54c98fb0b81157dbb3bd90780aa36b914e128a643b31e54a93f4` |
| `internal/ui/assets/5.32.14/swagger-ui-bundle.js.LICENSE.txt` | `c07853f3704b510a864eb56561ca4f36e0347fdaefc5176611c57575e4b5593d` |
| `internal/ui/assets/5.32.14/swagger-ui.css` | `d7f39f764aa18c7b47dd05b9af5613e373e4ac0f3557c2693d52d0abc2464d76` |

This is the minimum self-hosted distribution used by Arandu Swagger: the bundle and stylesheet load without a CDN or a Node.js runtime. `swagger-ui-standalone-preset.js` is not needed because the integration uses the bundle's `BaseLayout`. The upstream OAuth redirect page is not incorporated because it relies on an inline script; OAuth security schemes remain representable in generated OpenAPI documents, but a redirect page is outside this vendored asset set.

## Reference-only investigations

The following projects informed ecosystem research only. No source code, generated code, assets, or documentation from them is incorporated into Arandu Swagger:

- [go-swagger/go-swagger](https://github.com/go-swagger/go-swagger), licensed under Apache-2.0.
- [swaggo/swag](https://github.com/swaggo/swag), licensed under MIT.
- An item described only as "Bolt Swagger". No sufficiently identified upstream project or license was established, so this repository makes no provenance or licensing claim for it.

The Arandu Swagger implementation is original work based on the OpenAPI 3.1 specification and the public Arandu and Hesape APIs.
