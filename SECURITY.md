# Security policy

## Supported versions

The canonical `github.com/sevlumen/oashttp/v2` module is the fully supported release line. The legacy v1 module receives security fixes only so existing applications have a migration window.

| Version | Supported |
| --- | --- |
| `v2.x` | Yes |
| `v1.x` | Security fixes only |
| `< v1` and pre-release versions | No |

Users should update to the newest patch release in their supported major version before reporting an issue.

## Reporting a vulnerability

Do not open a public issue for a suspected vulnerability.

Use **Security → Report a vulnerability** in this GitHub repository when private vulnerability reporting is available. Otherwise, contact the repository owners privately and include:

- affected version and commit;
- a minimal reproduction;
- expected and observed impact;
- whether the issue is already being exploited;
- any proposed mitigation.

Reports will be acknowledged as soon as practical. A coordinated disclosure date will be agreed before public details are released.

## Security scope

`oashttp` provides HTTP binding, validation, Problem Details, OpenAPI generation, panic recovery, and authorization integration points. It does **not** implement JWT verification, OAuth 2.0, OpenID Connect, TLS termination, rate limiting, CSRF protection, or application-specific authorization policy. Applications are responsible for configuring those controls correctly.
