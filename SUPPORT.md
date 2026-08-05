# Support and compatibility

## Go versions

Version 1.0.0 is continuously tested with Go 1.22 through Go 1.26. The module's minimum language version is declared as Go 1.22.

## Runtime dependencies

The module has no third-party runtime dependencies. Swagger UI assets are browser-side and may be served from the pinned default CDN or from an application-controlled mirror.

## Compatibility promise

Within major version 1:

- exported identifiers will not be removed or changed incompatibly;
- existing struct fields and method signatures will remain source compatible;
- generated OpenAPI output may gain additional correct framework responses or schema metadata;
- bug fixes may tighten validation where the previous behavior was unsafe or contradicted documented behavior.

## Intended use

The package is production-oriented for typed JSON APIs built on `net/http`. Protocol endpoints requiring form encoding, browser redirects, streaming, WebSockets, or custom response representations should use standard `net/http` handlers alongside the generated API.
