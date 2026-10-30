# Publication review — 2026-10-30

**PUBLICATION NO-GO pending owner review of historical token exposure, ownership,
and license language.** This is a portfolio preparation review, not a production
security certification. No visibility change, merge, deployment, or production
operation is authorized by this work.

## Scope and evidence

GitHub metadata reported the repository **already public** at inspection. Visibility
was not changed. Remote refs were fetched and enumerated: only `main` at
`4deceaa7129a2aa75735a0ab181d109026b8de26`, no tags. The checkout is not shallow;
that root commit is the entire reachable pre-review history (113 tracked files).
`git fsck --full --no-reflogs` passed. Deleted/unreachable server objects, private
forks, external artifacts, and credentials used outside this repository cannot be
certified by a Git audit.

Gitleaks 8.24.3 scanned all fetched history with redaction: **one finding**, a JWT
in `scripts/validate.sh:7` in the root commit. A separate working-tree scan after
removal found zero findings. Manual review covered configuration, scripts,
workflow, generated headers, docs, migrations, and tracked filenames. Scanners
can miss secrets; zero findings is not proof of absence.

## Findings and changes

| Finding | Change / remaining action |
| --- | --- |
| `gofmt -l ./...` fails (`lstat ./...`), but command substitution inside `test -z` masks the failure and returns success | Enumerate tracked Go files, propagate formatter failures with `pipefail`, and format existing differences. |
| Migration scripts are mode 100644 but invoked directly | Invoke with Bash consistently. |
| CI invokes missing Buf/sqlc/plugins | Add pinned tool installation; generation plugins match checked-in generator versions. |
| Buf lint rejects repository-level `proto/` prefix | Document a narrow `PACKAGE_DIRECTORY_MATCH` exception; retain package/import layout and other STANDARD rules. |
| Generated descriptors reference obsolete `api/proto/...` paths | Regenerate from actual `proto/...` inputs; SQL output remains unchanged. |
| Main push breaking check compared main to itself | CI uses PR base SHA or previous push SHA; manual target accepts `PROTO_BASE`. |
| Migration smoke prints a false boolean but exits zero on missing table | Query actual users columns and sessions table with `ON_ERROR_STOP`. Use unique resources, no fixed host port, and cleanup only resources this run created. |
| Compose auth starts without applying versioned migrations | Add a one-shot migration dependency before auth startup. Postgres readiness uses TCP to avoid observing the temporary initialization socket server. |
| Historical `prod` profile uses published development credentials, plaintext transport, anonymous Grafana, all-interface ports, and no APP_ENV override | Explicitly document local demo status, bind all published ports to localhost, set dev environment explicitly for auth/gateway, require a shared local JWT secret. Database credentials remain local demo defaults. Profile name retained for compatibility. |
| Gateway accepts any nonempty Authorization header | Wire existing JWT middleware into edge chain; validate signature, issuer, expiry and subject. Reject refresh JWTs in both token validators. Add rejection/acceptance unit tests and integration coverage through real edge middleware. |
| Hard-coded JWT and password-printing example scripts | Require a fresh token/password in environment, remove payload printing; historical JWT remains in the root commit. |
| Unpinned health probe and dynamically linked build risk | Pin probe v0.4.38 and build with CGO disabled; support optional build-secret CA trust for networked builds. |
| README overstates maturity and reproducibility | Describe demonstrated mechanisms and experimental limits; add diagram, prerequisites, migration-aware quickstart, explicit tests and destructive local teardown warning. Preserve useful DESIGN.md and migration/profile docs, correcting conflicting claims. |

## Sensitive content, ownership, and licensing

The JWT includes an email address and user identifier and is expired by review
time. Do not paste its value into issues or PRs. Removal from the branch does not
remove it from history. If its signing key or the published development secret
was ever reused in a real environment, the owner must rotate that key and assess
exposure. No real production key, cloud credential, private key, customer data,
or employer-specific material was identified in reviewed tracked content, but
provenance and external reuse cannot be established from code alone.

Git author metadata contains a personal email address. Owner consent to public
attribution and to historical identifiers needs review. No history rewrite was
performed; that requires a separate coordinated decision.

`LICENSE.txt` reserves all rights, restricts copying/distribution, and describes
the work as proprietary/confidential. These terms were preserved, not replaced
with an open-source license. Public engineering evaluation does not imply an
open-source grant. The owner must confirm ownership of all contributions and
resolve the confidentiality/public-display wording before publication GO.
A [Go dependency license inventory](DEPENDENCY_LICENSES.md) covers 141 selected
modules; 54 need manual classification or have no cached root notice. Automated
text matching does not determine legal compatibility. Third-party Go dependencies
and generated code retain their upstream notices;
this review does not relicense them. A complete transitive dependency/container
license and vulnerability assessment remains outstanding. `go mod verify` checks
cache integrity, not license compliance or vulnerability status.

## Verification

Toolchain used locally: Go 1.27.3 (prepared environment), Buf 1.47.2, sqlc 1.30.0,
protoc-gen-go 1.36.11, protoc-gen-go-grpc 1.6.0, grpc-gateway 2.27.3. CI and Docker
select Go 1.24.9 from the repository. Local Go-version coverage therefore differs
from CI until the remote workflow completes.

Passed locally:

- Explicit unit tests and uncached race suite (`go test -race -count=1 ./...`).
- `go build ./cmd/...`, `go vet ./...`, `go mod verify`.
- `make fmt-check`, `make lint-migrations`, Bash syntax, Compose profile config,
  `git diff --check`.
- Buf lint, breaking comparison against original main, and actual Protobuf/sqlc
  generation. Regenerated Protobuf artifacts are committed.
- Docker testcontainers integration suite, including real edge middleware,
  registration, login, authenticated Hello, arbitrary-header rejection, schema,
  and gRPC status checks.
- `make migrate-auth-smoke`: fresh Postgres, migrate CLI v4.17.1, both migrations,
  actual users/sessions queries, and disposable-resource cleanup.
- Gitleaks history scan identified the historical JWT; working-tree scan clean.

Additional verification:

- A fresh local clone of the committed branch passed `make verify`, Buf lint and
  breaking checks against the original main SHA, and migration policy lint;
  generation left the clone clean.
- A negative Buf check rejected removal of HelloResponse field 1.
- Generation verification also rejects untracked generated output.
- Negative formatter checks rejected an unformatted tracked file and propagated
  a formatter process failure instead of masking it.
- Full Docker image builds were attempted with the session CA mounted as a
  BuildKit secret. The build was **blocked** by network policy denying the Go
  proxy redirect to storage.googleapis.com for klauspost/compress v1.18.0.
  This is not a passing build. Full Compose startup and observability smoke
  checks remain unverified.
- An alternative Docker runtime smoke **passed**: CGO-disabled binaries compiled
  with local Go 1.27.3 were bind-mounted into disposable distroless containers,
  connected to fresh migrated Postgres. Registration, login, authenticated Hello,
  and forged-header rejection passed. Internal service names were excluded from
  the session HTTP proxy. This does not validate the Dockerfile build or Go 1.24.9
  images. The first attempt without the internal proxy exclusions timed out;
  the corrected run passed. All audit containers/networks were removed.
- Remote GitHub Actions was in progress when first checked; final status is
  recorded in the PR. No green CI result is assumed.

 No pre-existing
setup logs are counted as tests of these changes. Sandbox restrictions initially
blocked network/Docker; authorized local execution succeeded. The workspace's
SQL bind mounts initially had unreadable directory permissions; local permissions
were corrected without changing Git file modes. Source-based Buf installation
hit a network-policy denial on a Go module redirect; pinned upstream release
binaries were used locally instead. The CI installer remains source-based.

## Remaining product and operational limits

Refresh tokens are issued but no refresh/rotation/logout/revocation endpoints
exist. Internal gRPC has no TLS or service identity. Direct gRPC ports and admin
interfaces must not be exposed publicly. The local demo uses plaintext DB
connections. Rate limits are process-local. Availability, restore behavior,
performance, dashboards, alert semantics, and optional logs/OTel pipelines have
not been certified. Empty dashboard files and a misspelled empty provisioning
placeholder are existing scaffolding, not working observability features.

## Repository description

Proposed GitHub description:

> Experimental Go microservices portfolio: gRPC/HTTP gateway, Postgres auth, generated APIs and queries, and local observability tooling.

Repository metadata updates through the shell API were denied by the environment;
the available GitHub connector does not expose a repository-description update.
The text is prepared here for owner application. Visibility was not changed.

## Conditions for GO

1. Owner confirms rights to publicly display all code and resolves confidential
   license wording without accidentally granting unintended rights.
2. Owner assesses historical JWT/key reuse, any necessary rotation, and whether
   historical identifiers require coordinated removal.
3. Review and merge the PR separately; verify remote CI and complete any blocked
   Docker/generation checks. Complete dependency/container security and license
   review before making stronger security/compliance claims.

A GO for portfolio display would not be a GO for production deployment.
