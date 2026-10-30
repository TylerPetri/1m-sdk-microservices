package httpmw

// Canonical request ID header used across the HTTP edge.
//
// We standardize on this value so all middleware, access logs, and downstream
// propagation use the same key.
const HeaderRequestID = "X-Request-Id"
