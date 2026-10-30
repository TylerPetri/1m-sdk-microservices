# Portfolio preparation — Pass 2

Reviewed 2026-10-30 (America/Toronto), continuing PR #1 and its existing branch.
No merge, public-history rewrite, visibility change, production access, or
publishing/deployment operation was performed. Docker/Compose operations were
limited to disposable test stacks and hosted CI runners.

## Assessment

**MERGE GO for the scoped remediation/documentation PR after its latest required
checks are green.** This recommendation is not approval to merge automatically.
Owner security and licensing actions below remain necessary before treating the
public portfolio as cleared for broader promotion. Production deployment remains
**NO-GO**; the project is explicitly an experimental local demonstration.

The architecture, generated APIs, working registration/login/Hello flows, design
documentation, and local observability features are preserved. A developer can
follow README.md to start the application and run meaningful tests. The existing
Mermaid diagram adequately shows the HTTP, gRPC, Postgres, migration, and metrics
boundaries; no competing diagram or architecture was introduced.

## Remaining findings, ordered by severity

1. **High if reused: historically published signing key.** Offline HMAC checking
   proves the historical JWT was signed with the committed development default.
   It has the project issuer, an example-domain identity, and an expired 15-minute
   validity window. This supports classification as a development-issued example,
   not an unknown external credential. It does **not** establish isolation: the
   historical `prod` profile and unset APP_ENV path could use the same default.
   No external deployment or production-key reuse was found in repository history,
   but actual reuse cannot be ruled out. The key can sign fresh tokens regardless
   of the old example's expiry. **Owner action:** inventory issuers/validators;
   rotate any potentially reused key everywhere, stop accepting the old key,
   invalidate affected access/refresh tokens, require reauthentication, and assess
   historical access. See [SECURITY.md](SECURITY.md) for the precise runbook.
2. **Medium, intentional demo boundary:** direct gRPC and admin listeners do not
   independently enforce HTTP bearer authentication; local DB credentials,
   plaintext transport, anonymous Grafana, process-local limits, and Docker socket
   access remain unsuitable for untrusted exposure. All published ports use
   loopback. Internal gRPC identity/resource authorization, refresh/logout APIs,
   TLS, production secrets, backups, and production SLO validation are not claimed.
3. **Medium, owner decision:** LICENSE.txt still reserves all rights and calls the
   work proprietary/confidential. Confirm public-display rights and reconcile
   that wording; this PR does not grant an open-source license. Confirm consent
   for historical personal attribution/identifiers. No history rewrite occurred.
4. **Low / incomplete assurance:** the Pass 1 dependency inventory covers 141
   selected Go modules, with 54 entries needing manual notice/classification
   review. It is not legal clearance. Complete transitive/container license and
   vulnerability assessment remains outstanding. Alert thresholds and operational
   tuning remain examples; smoke tests do not establish SLOs or scalability.

No JWT, signing key, private identity, or replacement credential is reproduced in
this review. Gitleaks 8.24.3 still finds exactly one historical JWT at root commit
`4deceaa7129a2aa75735a0ab181d109026b8de26`, `scripts/validate.sh:7`. The final
working-tree scan has zero findings. History has not been rewritten.

## Pass 2 fixes

- Removed auth/gateway's fallback signing key; both require an explicit key of at
  least 32 characters in **every** environment. Fixed APP_ENV lookup and enforced
  required staging/prod DSN settings rather than silently accepting defaults.
- Unified the access-token rejection contract across gateway and auth-service
  validators. Added malformed/base64/unsigned/wrong-algorithm, signature, issuer,
  expiry, nbf, subject, refresh-token, and cross-issuer/gateway-format checks.
- Limited public-route exemptions to the actual public endpoints. Accepted
  case-insensitive Bearer schemes, rejected duplicate/combined authorization
  headers, and added a bearer challenge plus grpc-gateway-compatible JSON 401
  responses. Removed the unused header-presence-only authentication helper.
- Migrated deprecated gRPC client setup to NewClient with an explicit readiness
  wait, preserving gateway startup dependency behavior. Added readiness and
  cancellation tests. Static analysis also identified and fixed ignored admin
  environment selection and a boolean simplification. The deliberate nil-context
  guard test retains its assertion and has a narrowly explained lint directive.
- Made Postgres integration startup wait for full readiness. One local race run
  exposed a premature connection/reset failure before this change; three
  consecutive race-enabled runs then passed all three integration tests.
- Added migration idempotence and negative checks: unique email, foreign-key
  enforcement, absent schema, invalid migration SQL, persisted dirty state, and
  rejection of an automatic retry against a dirty database. No force/repair step
  hides failed migrations.
- Corrected Grafana's dashboard file (it incorrectly used the HTTP import
  envelope), removed empty provisioning placeholders, opened collector OTLP
  receivers on container interfaces, replaced its deprecated logging exporter,
  and scoped Promtail discovery to the Compose project. Optional collector scraping
  now uses Docker DNS discovery, so disabling that profile does not create a
  permanently down scrape target.
- Wired the existing gateway HTTP metric middleware. Matched dashboard/recording/
  alert queries to actual emitted metric names and labels, and added synthetic
  Prometheus rule tests. Runtime smoke checks assert the HTTP/gRPC series exist.
- Excluded root/nested `.env` files from Docker contexts. A negative Docker build
  test proves they cannot be copied while `.env.example` remains available.
- Added a reproducible `make smoke-compose` gate using actual Dockerfiles, random
  loopback ports, fresh volumes, a generated ephemeral key, optional CA mounts,
  saved diagnostics, and cleanup restricted to its unique project. CI now runs
  this alongside generation, static analysis, race tests, and migrations.

## Exact verification and evidence

Verification code commit:
`df0033839e51285a3625e905f94f632ba931620f`.
A subsequent optional-collector discovery correction avoids a false down target
when the OTLP profile is disabled; the final head run is recorded in the PR.

| Check | Result / scope |
| --- | --- |
| `go test -count=1 ./...` | PASS locally, Go 1.27.3. |
| `go test -race -count=1 ./...` | PASS locally; also PASS with Go 1.24.9 in a disposable toolchain container. |
| `go test -race -tags=integration -count=3 ./internal/integration` | PASS after the explicit Postgres readiness correction: all three tests, three runs. |
| Full tagged integration suite with race detector | Required by CI through `make test-integration`; linked final run is authoritative. |
| `go vet ./...` | PASS locally and under Go 1.24.9. |
| Staticcheck 2025.1.1, normal and integration tags | PASS under Go 1.24.9; no global checks excluded. Local Go 1.27's export format is unsupported by that pinned analyzer, so that combination is unavailable. |
| Clean-checkout `make verify` | PASS: actual Buf/sqlc regeneration left no tracked/untracked output changes. |
| Buf lint and breaking check | PASS against original main SHA `4deceaa7129a2aa75735a0ab181d109026b8de26`. Earlier negative field-removal and formatter-error gates remain documented in the Pass 1 review. |
| `make migrate-auth-smoke` | PASS: migrate v4.17.1, fresh Postgres 16, both migrations, repeat up, and all listed negative cases. |
| Promtool config / rule tests | PASS: recording and error-rate expressions tested against synthetic series using actual exported metric names. |
| Docker context negative build test | PASS: both environment files excluded, example retained. |
| Actual Dockerfile builds | PASS on hosted GitHub Actions: gatewayd-run, authd-run, hellod-run, Go 1.24.9. Not replaced by locally built binary mounts. |
| Full disposable Compose smoke | PASS on hosted Actions: real HTTP/gRPC/Postgres flows and authentication rejection, three readiness endpoints, five Prometheus targets, application metric series, OTLP metric delivery, Grafana health/dashboard provisioning, Loki readiness, and Promtail gateway-log ingestion. |
| Local observability containers | PASS: Grafana provisions the corrected dashboard, collector accepts OTLP, Loki ready. Test stack/volumes removed. |
| `go mod verify`, shell syntax, whitespace, secret scans | PASS, except the explicitly retained historical JWT finding. |

CI evidence:

- [Pass 1 final run](https://github.com/TylerPetri/1m-sdk-microservices/actions/runs/41137017737):
  **success**, commit `e8e4caf126710da0a85f45d3b8512cccc82a89e2`.
- [Pass 2 full build and observability smoke](https://github.com/TylerPetri/1m-sdk-microservices/actions/runs/41138074945):
  **success**, commit `d2bacf6c378357f90e77c06ebfd70ec7921d6896`.
- [Static analysis, negative migrations, application metrics and rule checks](https://github.com/TylerPetri/1m-sdk-microservices/actions/runs/41138798159):
  **success**, commit `3f833ba3e5565e62ffe6adae5949f15f4a5e7c52`.
- [Verification code commit run](https://github.com/TylerPetri/1m-sdk-microservices/actions/runs/41139108363):
  commit `df0033839e51285a3625e905f94f632ba931620f`; see the linked run and PR
  checks for the final result. The PR description records the latest head run.

### Network blocker: resolved verification, unchanged policy

The managed environment still denies the Go proxy redirect to
storage.googleapis.com. The actual local Dockerfile attempt remains **blocked**;
TLS and proxy checks were not disabled, and no network rule was bypassed.
A read-only local Buildx activity directory was independently handled by placing
Buildx activity state in /tmp while preserving Docker credential configuration.

The supported alternative was the repository's hosted Actions runner: it builds
and tests the actual Dockerfiles with ordinary networking. This closes the
Dockerfile verification gap without changing the managed environment's policy.
A Go 1.24.9 toolchain container also ran vet/Staticcheck/race checks offline using
the existing verified module cache. This is distinct from the actual image build.

## Suggested GitHub metadata

Description:

> Experimental Go microservices: authenticated HTTP-to-gRPC gateway, PostgreSQL auth, generated APIs/queries, and reproducible local observability.

Topics: `go`, `golang`, `grpc`, `grpc-gateway`, `postgresql`, `protobuf`, `sqlc`,
`docker-compose`, `opentelemetry`, `prometheus`, `grafana`, `authentication`.

These are suggestions only; repository metadata/visibility was not changed.

## Portfolio readiness

Engineering presentation and reproducible local demonstration are ready for
review. The README describes service boundaries, setup, tests, and limits without
production-proof claims. Historical-key investigation/rotation and licensing/
public-attribution decisions still need explicit owner closure. They are not
resolved by green CI, by removing the token from the working tree, or by expiry
of the historical token. No production-readiness claim is warranted.
