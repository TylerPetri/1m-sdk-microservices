# Security boundaries and historical credential action

This is an experimental local service stack. Please report suspected issues to
the repository owner privately; do not attach tokens, keys, database dumps, or
personal data to public issues.

## Historical token assessment

The original root commit (`4deceaa7129a2aa75735a0ab181d109026b8de26`) includes a
JWT in `scripts/validate.sh`. Pass 2 examined it in memory without reproducing the
token, identity, or key values in this report:

- It is an HS256 access token with the project issuer and an example-domain email.
- Its 15-minute validity window was 2025-12-29, 20:24–20:39 America/Toronto. It is
  expired at review time.
- Offline HMAC verification **succeeds with the committed development default**.
  This is evidence of a development-issued example token, not just arbitrary
  placeholder text. A JWT is not itself an HMAC signing key; however, the matching
  key was also published as a configuration default.
- **Not established:** that the token was strictly synthetic or never used in a
  deployed service. Signature verification establishes the matching key, not the
  issuance path or the absence of external reuse. That requires owner confirmation
  and historical configuration/deployment records.
- The original reachable history has one commit. Its key references are the auth
  configuration and local Compose file. No real deployment manifests or external
  credential bindings were found. The old profile was called `prod`, did not set
  APP_ENV, and defaults could be used with APP_ENV unset. Therefore use outside
  isolated development **cannot be ruled out** by repository inspection.

Pass 2 removes the key fallback from auth and gateway configuration. Both require
an explicitly supplied signing secret of at least 32 characters even in dev.
The environment loader now reads APP_ENV and requires explicit staging/prod DB
configuration. No public history was rewritten; the historical token and default
remain discoverable in Git history.

## Required owner actions if reuse cannot be excluded

1. Inventory every running or previously deployed auth issuer and gateway/token
   validator using this code, including instances with APP_ENV unset. Confirm
   where the historical default or a copied key was used. Review deployment and
   secret-manager records; this review did not access production.
2. Wherever reuse is possible, treat the matching signing key as compromised.
   Generate a fresh high-entropy key (for example 32 random bytes encoded as hex)
   and coordinate replacement on **all issuers and validators**. Do not retain
   the old key as an accepted fallback. Do not put replacement keys in Git.
3. Invalidate all access and refresh tokens signed with the old key. With this
   single-key implementation, replacing the key at every validator invalidates
   those signatures immediately; require clients to log in again. Refresh tokens
   currently have no renewal/revocation API, so do not claim per-session logout or
   revocation. If a downstream system added sessions/refresh handling, revoke its
   affected sessions as well.
4. Assess issuance/access logs for suspicious use during the exposure window.
   Expiry of this one example token does **not** prevent forging new tokens with
   a published signing key. Record the assessment and rotation completion outside
   public reports without including secret values.
5. Decide separately whether historical identifiers warrant coordinated history
   cleanup. A rewrite alone cannot revoke tokens or recover disclosed keys, and
   is not authorized by this PR.

## HTTP authentication contract

- Public routes are exactly `/v1/auth/register`, `/v1/auth/login`,
  `/v1/auth/validate`, `/healthz`, and `/readyz`. Public means no bearer header is
  required; Validate still checks the token supplied in its JSON request body.
  Methods and request schemas are defined in Protobuf/gRPC-gateway.
- Other gateway paths require **one** Authorization header containing a Bearer
  access token. The scheme is case-insensitive. Multiple/combined headers,
  malformed tokens, unsigned/non-HS256 tokens, bad signatures, wrong/missing
  issuers, missing/expired expiration, future nbf, empty subjects, and refresh
  tokens are rejected.
- Rejection returns HTTP 401, a `WWW-Authenticate: Bearer` challenge, and the
  grpc-gateway-compatible JSON shape `{"code":16,"message":"unauthenticated"}`.
  No parser details or token values are echoed. Invalid authentication setup
  fails closed; process configuration validation rejects missing/short keys.
- Register/login issue access and refresh JWTs. Refresh JWTs cannot be used as
  access JWTs; refresh/rotation/logout endpoints are not implemented.
- The HTTP edge is the authentication boundary. Internal Hello gRPC is a trusted
  demo endpoint and does **not** validate caller JWTs independently. Direct gRPC
  access bypasses the HTTP edge by design; never expose those ports to untrusted
  clients. Admin endpoints also lack authentication. Incoming `x-user-id` metadata
  is not an authorization credential.
- No audience-based or domain/resource authorization is implemented. Do not add
  sensitive business operations without an explicit authorization design.

All local published ports bind to loopback. This is not TLS, service identity,
network isolation against other local users, production hardening, or a security
certification. Promtail reads Docker logs/socket and is scoped to this Compose
project; the socket mount still conveys powerful access and is a local example.
