// Package core contains dependency-light contracts and request-scoped state
// shared by oashttp internal subsystems.
//
// Core must remain independent of sibling internal packages. Business logic,
// routing, binding, OpenAPI generation, authentication execution, and failure
// serialization belong in their dedicated packages.
package core
