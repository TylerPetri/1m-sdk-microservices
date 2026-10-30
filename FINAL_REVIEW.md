# Final engineering and merge review — Pass 3

Reviewed 2026-10-30 (America/Toronto), on the existing branch and PR #1.

**Engineering checks: GO. Final MERGE: NO-GO pending the owner decisions below.**
No additional broad hardening is proposed. No merge, deployment, visibility
change, external credential rotation, or published-history rewrite occurred.

## Review scope and result

Current main is `4deceaa7129a2aa75735a0ab181d109026b8de26`. The complete PR was
reviewed against that ref, including configuration/authentication, test harnesses,
CI, Docker/Compose, observability, generated code, and documentation. Reviewed
implementation head is `855dbd367ff170b0687090b29bf45f6f81b8b60e`; Pass 3 adds only
documentation. The final documentation commit and its corresponding successful
CI run are recorded in the PR description and final handoff.

No unintended runtime regression or further blocking code defect was found:

- Invalid, malformed, unsigned/non-HS256, expired, wrong-issuer, bad-signature,
  missing-expiry/subject, and refresh JWTs are rejected. Duplicate headers and
  route-bypass attempts are covered; the documented public endpoints remain public.
- Auth and gateway require explicit signing configuration. Staging/prod DB
  configuration is explicit; the historical default cannot silently issue tokens.
- Auth/Hello gRPC and the HTTP edge retain their service boundaries. Direct gRPC
  and admin listeners remain trusted local interfaces, as explicitly documented.
- Migration definitions and Protobuf source contracts are unchanged from main.
  Real migrations are tested on fresh databases, including repeat up, constraints,
  invalid SQL and dirty-state rejection; no force/repair hides negative failures.
- Buf/sqlc generation is synchronized. Generated descriptor source-path changes
  match actual repository paths; no wire contract was changed. The two broad Go
  formatting diffs (`cmd/hellod/main.go`, `internal/db/pool.go`) were independently
  confirmed to equal gofmt of main, with no semantic changes.
- Compose smoke tests use uniquely named stacks, generated ephemeral keys,
  random loopback ports and fresh volumes. They reference no production endpoint.
  The managed Docker daemon and hosted CI are test environments. This does not
  authorize targeting an arbitrary external Docker daemon.
- Secret scanning retains exactly the known historical JWT finding; the current
  working tree has no findings. Root/nested environment files are excluded from
  Docker contexts and the exclusion is negatively tested.

Pass 3 presentation corrections specify the tested Go toolchain, minimum Compose
version used by the smoke overrides, and Python smoke-test prerequisite. Migration
wording distinguishes an expand/contract policy from proven zero-downtime
operations. Existing design documentation and the compact Mermaid diagram remain.

## Historical credential: verified facts versus confirmation

Repository-verified: the root commit's example-domain JWT has a 15-minute lifetime,
is expired, and verifies with the committed development signing default. The
original Compose file called its full-stack profile `prod`, supplied that default,
and did not explicitly set APP_ENV. Therefore the token is a signed development
example, but **strictly synthetic / never deployed is not established**. Neither
example-domain claims nor expiry prove the key was never reused elsewhere.

To rule out reuse requires owner confirmation supported by historical deployment,
CI/CD, configuration and secret-manager records for every environment/project
where this code or key was copied. Repository inspection cannot establish those
external facts, and no external environment was accessed for this review.

If reuse is possible or records are incomplete, the owner must treat the old key
as compromised: rotate it on all issuers and validators, stop accepting it,
invalidate affected access/refresh tokens, require login again, and assess logs.
[SECURITY.md](SECURITY.md) contains the full runbook. History cleanup is a separate
decision and cannot substitute for revocation/rotation.

## Licensing: actual obligations and ownership

[THIRD_PARTY_REVIEW.md](THIRD_PARTY_REVIEW.md) documents all 47 external runtime
modules' inspected root notices, the pinned upstream component licenses, and
copyright/package records extracted from the local base images.

- Runtime module root texts are permissive MIT/BSD/Apache-2.0 and bundled notices.
  Binary redistribution needs the actual notices/licenses, including applicable
  NOTICE text, Go standard-library attribution and the included health probe.
- Grafana/Loki/Promtail are AGPL components. Redistribution and modified network
  versions have source obligations; independent Go services are not automatically
  relicensed just because they communicate with these separate services.
- Distroless is not an Apache-only image: Debian package records include GPL,
  LGPL and other terms. Image redistribution requires package-specific review
  and preservation of notices/source arrangements. This PR does not publish images.
- This is not a complete per-file/per-platform/container SBOM or redistribution
  bundle. Those are needed before a future binary/image distribution, rather than
  assumed complete by green CI or by the old module-name inventory.

LICENSE.txt remains byte-for-byte unchanged: all rights reserved and proprietary/
confidential wording. No permissive license was substituted. Dependency rights do
not override ownership of original contributions; conversely, the proprietary
claim does not override third-party rights. Employer/client restrictions and
contribution ownership cannot be verified from Git authorship alone.

## Owner decisions before the final merge recommendation

1. **Credential history:** confirm isolated local-only use based on the available
   records, or acknowledge possible reuse and complete the applicable rotation/
   invalidation actions. No secret values are needed in a public response.
2. **Ownership and public terms:** confirm rights to publicly display the original
   contributions. Decide whether to clarify LICENSE.txt's contradictory
   confidentiality/public-display wording while retaining all rights reserved.
   Suggested clarification is in THIRD_PARTY_REVIEW.md; it is not applied.

These are owner facts/terms decisions, not requests for another engineering pass.
Once closed, the engineering evidence supports MERGE GO for this scoped PR.
Production deployment remains outside scope and is not approved.

## Validation and exact final-head evidence

The existing successful implementation run is
[41139457813](https://github.com/TylerPetri/1m-sdk-microservices/actions/runs/41139457813)
for `855dbd367ff170b0687090b29bf45f6f81b8b60e`. Pass 3 repeats relevant checks and
records its exact final commit/run in the PR description after completion:

- Uncached unit and race tests; full race-enabled Postgres integration suite.
- Vet and Staticcheck (ordinary and integration tags) under Go 1.24.9.
- Clean-checkout Buf/sqlc generation with no drift, Protobuf lint and breaking
  comparison against current main; formatting and module-cache verification.
- Fresh positive/idempotent/negative migration tests.
- Actual Dockerfile builds for all three services, Docker-context negative tests,
  disposable full Compose readiness/auth flows, Prometheus rules/scrapes/metrics,
  Grafana dashboard provisioning, OTLP delivery, and Promtail/Loki log ingestion.
- Current-tree and full-history redacted secret scans; whitespace/shell checks.

Local actual Dockerfile builds remain subject to the managed network's denied Go
module redirect; hosted Actions performs the actual builds without circumventing
that policy. The pinned Staticcheck cannot read local Go 1.27 export data, so the
Go 1.24.9 container/hosted checks are authoritative for static analysis.

## GitHub presentation

Public metadata currently describes a "modular Go microservices SDK" with topics
including sdk/distributed-systems. A more precise description is:

> Go backend portfolio: HTTP-to-gRPC gateway, PostgreSQL authentication, generated contracts, and tested local observability.

Suggested topics: `go`, `golang`, `backend`, `grpc`, `grpc-gateway`, `postgresql`,
`protobuf`, `sqlc`, `docker-compose`, `opentelemetry`, `prometheus`, `grafana`,
`authentication`. Metadata was reviewed, not changed. The README distinguishes
production-style engineering mechanisms from an experimental local demo.
