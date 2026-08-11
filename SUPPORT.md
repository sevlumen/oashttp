# Support and compatibility

## Go versions

The current v2 module is continuously tested with Go 1.22 through Go 1.26. The module's minimum language version is declared as Go 1.22.

## Runtime dependencies

The module has no third-party runtime dependencies. Swagger UI assets are browser-side and may be served from the pinned default CDN or from an application-controlled mirror.

## Compatibility promise

Within major version 2:

- exported identifiers will not be removed or changed incompatibly;
- existing struct fields and method signatures will remain source compatible;
- generated OpenAPI output may gain additional correct framework responses or schema metadata;
- bug fixes may tighten validation where the previous behavior was unsafe or contradicted documented behavior.

The legacy v1 module receives security fixes only; new features target v2.

## Intended use

The package is production-oriented for typed JSON APIs built on `net/http`. Endpoints requiring form encoding, browser redirects, streaming, WebSockets, or custom response representations can use standard `net/http` handlers through `MapHandler` while remaining in the same router, middleware, security, and OpenAPI document. Application-specific protocol behavior remains the application's responsibility.
