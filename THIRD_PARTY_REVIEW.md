# Third-party licensing review — Pass 3

Reviewed 2026-10-30. [LICENSE.txt](LICENSE.txt) was **not changed**. Retaining
rights to original project code is compatible with the permissive runtime
licenses identified here; it does not transfer ownership of third-party code or
override third-party conditions. Public visibility is not an open-source grant.

## What is actually included

The Linux Go runtime import graph (`go list -deps -json ./cmd/...`) contains
**47 external modules**. All 47 have root license files in the verified module
cache. Those texts contain MIT, BSD, Apache-2.0, and bundled permissive notices;
no copyleft license was identified in these runtime module root texts. This is a
closer review of the code compiled into the services, distinct from the older
141-module selection inventory, which also contains optional/test dependencies.
The full runtime list and versions appear below. Nested file-specific notices
and future build targets must still be preserved where applicable.

No vendored dependency tree, application binary release, or published application
container is added by this PR. Compose pulls independently licensed upstream
images. CI builds disposable local images and uploads logs, not application
images or binaries. Build/test tools have their own licenses; using a compiler or
generator does not automatically apply its license to original application code.
The grpc-health-probe executable is copied into the runtime images and therefore
must be considered when those images are distributed.

## Actual notice and redistribution conditions

| License/material | Required handling when distributing the covered code/binaries |
| --- | --- |
| MIT | Include the copyright and permission notice in copies/substantial portions. Preserve the accompanying disclaimer. |
| BSD | Retain copyright, conditions, and disclaimer in source distributions; reproduce them in accompanying binary documentation/materials. Observe non-endorsement clauses where present. The exact upstream terms control; do not reduce every BSD notice to a generic label. |
| Apache-2.0 | Supply the license; retain applicable attribution/copyright/patent/trademark notices; preserve applicable NOTICE text if supplied. Mark modified upstream files as modified. Apache does not require releasing independent original application source under Apache. |
| PostgreSQL license and bundled notices | Retain the PostgreSQL copyright, permission grant, and disclaimers in copies. The inspected package also carries regex/Tcl/BSD and other per-file notices; preserve the package's complete copyright file. |
| AGPL-3.0 components | Keep their notices/license. Conveying binaries invokes corresponding-source requirements (section 6). If modifying the covered program and users interact with it remotely, offer corresponding source prominently to those users (section 13). |
| Debian components in redistributed images | Preserve package copyright/license material. GPL/LGPL components can require corresponding source/source-offer arrangements; LGPL conditions depend on how a library is linked/modified. Apache licensing of the Distroless project does not make every package in an image Apache-licensed. |

Runtime modules with explicit root NOTICE files: `google.golang.org/grpc`,
`github.com/prometheus/client_model`, `github.com/prometheus/common`,
`github.com/prometheus/client_golang`, `github.com/prometheus/procfs`, and
`go.yaml.in/yaml/v2`. OpenTelemetry LICENSE files also include Go/BSD attribution;
copy their complete text rather than dropping the appended notice. The Go
standard library carries its own BSD-style license, separate from module notices.

Before publishing standalone service binaries or application images, assemble a
notice/license bundle from the exact build inputs (including Go and the health
probe) and review the base-image source/notice obligations. This review report is
**not** that redistribution bundle. The current source-only portfolio does not
ship those binary artifacts.

## Container and build components checked

These are the application/component licenses, not a claim that each complete
image has only one license. Source links were read at the configured versions.

| Component used by this repository | Verified upstream terms / source |
| --- | --- |
| Go 1.24.9 build toolchain/stdlib | [BSD-style license](https://github.com/golang/go/blob/go1.24.9/LICENSE). Builder image also includes independently licensed OS/compiler packages; it is not shipped as the application's final image. |
| grpc-health-probe v0.4.38 | [Apache-2.0](https://github.com/grpc-ecosystem/grpc-health-probe/blob/v0.4.38/LICENSE); executable is copied into all three final images. |
| Distroless base-debian12 | [Distroless Apache-2.0](https://github.com/GoogleContainerTools/distroless/blob/main/LICENSE) plus Debian package licenses. Inspected image records include libc6 (LGPL-2.1-or-later), netbase (GPL-2), ca-certificates (MPL-2.0/GPL packaging), OpenSSL and additional package-specific notices. |
| PostgreSQL 16 | [PostgreSQL license](https://github.com/postgres/postgres/blob/REL_16_STABLE/COPYRIGHT); inspected `/usr/share/doc/postgresql-16/copyright` includes additional component terms. |
| migrate v4.17.1 | [MIT](https://github.com/golang-migrate/migrate/blob/v4.17.1/LICENSE). |
| Prometheus v2.55.1 | [Apache-2.0](https://github.com/prometheus/prometheus/blob/v2.55.1/LICENSE), plus upstream/dependency notices. |
| Grafana 11.2.2 (OSS image) | [AGPL-3.0](https://github.com/grafana/grafana/blob/v11.2.2/LICENSE), plus separately licensed dependencies/plugins. This repository uses `grafana/grafana`, not Grafana Enterprise. |
| Loki and Promtail 3.1.1 | [AGPL-3.0 root terms](https://github.com/grafana/loki/blob/v3.1.1/LICENSE); preserve per-file/dependency exceptions. |
| OTel Collector Contrib 0.110.0 | [Apache-2.0](https://github.com/open-telemetry/opentelemetry-collector-releases/blob/v0.110.0/LICENSE), also confirmed by the image's OCI license label; individual contrib/dependency notices still apply. |

Grafana/Loki/Promtail are separate, unmodified programs here, configured through
mounted files and ordinary network interfaces. Their AGPL licenses do **not by
that fact alone** require relicensing the independent Go services. This is not a
clearance for future combined/derived works or modified hosted AGPL software.
Running upstream images, modifying those programs, and redistributing images are
different cases. The service binaries use CGO_ENABLED=0; the image still carries
Debian libraries even when the Go binaries do not link them. Do not treat a
source URL alone as a universal source-offer
substitute when distributing binaries.

Image evidence: package copyright files and dpkg status records were read from
local Distroless digest
`sha256:fabbf1c0c357a3d42550111351daed089b20a2c954df13ee2fcff60602515e84`;
PostgreSQL package notices came from local digest
`sha256:1beaa3084eee2676f20a9cd9928f0bf2c5bf674f8e49f81017912183d377c61c`.
Other component texts were checked at the linked upstream tags; the collector
image additionally declares `Apache-2.0`. No container filesystem was executed or
modified for the notice extraction. Tags are not digest-pinned; these observations
do not certify every future image pulled under the same tag or constitute a
complete package-by-package SBOM for every image.

## Original-code ownership and license decision

The current LICENSE.txt says the entire Software is exclusive property and
"proprietary and confidential" while the repository is intentionally public.
That wording should not be read as ownership of upstream components. No evidence
of third-party source misappropriation was found, but authorship/commit metadata
cannot establish employment contracts, client restrictions, or contributor rights.
Those questions require the owner's confirmation, separately from dependency
license conditions.

Suggested clarification for owner approval, **not applied to LICENSE.txt**:
retain copyright/all rights reserved for original contributions; replace the
confidentiality claim with public display for evaluation/review; explicitly
exclude independently licensed third-party material from the proprietary claim.
Do not grant general reuse/modification/redistribution rights unless intended,
and do not purport to restrict rights provided by third-party licenses or the
hosting platform's terms. No switch to MIT, Apache, or another permissive project
license is proposed or performed.

## Runtime module notices inspected

License-family labels below summarize the actual root files, not an SPDX legal
classification of every file or a substitute for shipping the complete notices.

| Module | Selected version | Texts identified | Root notice files |
| --- | --- | --- | --- |
| `github.com/beorn7/perks` | `v1.0.1` | MIT | LICENSE |
| `github.com/cenkalti/backoff/v5` | `v5.0.3` | MIT | LICENSE |
| `github.com/cespare/xxhash/v2` | `v2.3.0` | MIT | LICENSE.txt |
| `github.com/felixge/httpsnoop` | `v1.0.4` | MIT | LICENSE.txt |
| `github.com/go-logr/logr` | `v1.4.3` | Apache-2.0 | LICENSE |
| `github.com/go-logr/stdr` | `v1.2.2` | Apache-2.0 | LICENSE |
| `github.com/golang-jwt/jwt/v5` | `v5.3.0` | MIT | LICENSE |
| `github.com/google/uuid` | `v1.6.0` | BSD (retain exact terms) | LICENSE |
| `github.com/grpc-ecosystem/grpc-gateway/v2` | `v2.27.3` | BSD (retain exact terms) | LICENSE |
| `github.com/jackc/pgpassfile` | `v1.0.0` | MIT | LICENSE |
| `github.com/jackc/pgservicefile` | `v0.0.0-20240606120523-5a60cdf6a761` | MIT | LICENSE |
| `github.com/jackc/pgx/v5` | `v5.8.0` | MIT | LICENSE |
| `github.com/jackc/puddle/v2` | `v2.2.2` | MIT | LICENSE |
| `github.com/munnerz/goautoneg` | `v0.0.0-20191010083416-a7dc8b61c822` | BSD (retain exact terms) | LICENSE |
| `github.com/prometheus/client_golang` | `v1.23.2` | Apache-2.0 | LICENSE, NOTICE |
| `github.com/prometheus/client_model` | `v0.6.2` | Apache-2.0 | LICENSE, NOTICE |
| `github.com/prometheus/common` | `v0.67.4` | Apache-2.0 | LICENSE, NOTICE |
| `github.com/prometheus/otlptranslator` | `v1.0.0` | Apache-2.0 | LICENSE |
| `github.com/prometheus/procfs` | `v0.19.2` | Apache-2.0 | LICENSE, NOTICE |
| `go.opentelemetry.io/auto/sdk` | `v1.2.1` | Apache-2.0 | LICENSE |
| `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc` | `v0.64.0` | Apache-2.0 + BSD (retain exact terms) | LICENSE |
| `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` | `v0.64.0` | Apache-2.0 + BSD (retain exact terms) | LICENSE |
| `go.opentelemetry.io/contrib/instrumentation/runtime` | `v0.64.0` | Apache-2.0 + BSD (retain exact terms) | LICENSE |
| `go.opentelemetry.io/otel` | `v1.39.0` | Apache-2.0 + BSD (retain exact terms) | LICENSE |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace` | `v1.39.0` | Apache-2.0 + BSD (retain exact terms) | LICENSE |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` | `v1.39.0` | Apache-2.0 + BSD (retain exact terms) | LICENSE |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` | `v1.39.0` | Apache-2.0 + BSD (retain exact terms) | LICENSE |
| `go.opentelemetry.io/otel/exporters/prometheus` | `v0.61.0` | Apache-2.0 + BSD (retain exact terms) | LICENSE |
| `go.opentelemetry.io/otel/exporters/stdout/stdouttrace` | `v1.39.0` | Apache-2.0 + BSD (retain exact terms) | LICENSE |
| `go.opentelemetry.io/otel/metric` | `v1.39.0` | Apache-2.0 + BSD (retain exact terms) | LICENSE |
| `go.opentelemetry.io/otel/sdk` | `v1.39.0` | Apache-2.0 + BSD (retain exact terms) | LICENSE |
| `go.opentelemetry.io/otel/sdk/metric` | `v1.39.0` | Apache-2.0 + BSD (retain exact terms) | LICENSE |
| `go.opentelemetry.io/otel/trace` | `v1.39.0` | Apache-2.0 + BSD (retain exact terms) | LICENSE |
| `go.opentelemetry.io/proto/otlp` | `v1.9.0` | Apache-2.0 | LICENSE |
| `go.uber.org/multierr` | `v1.11.0` | MIT | LICENSE.txt |
| `go.uber.org/zap` | `v1.27.1` | MIT | LICENSE |
| `go.yaml.in/yaml/v2` | `v2.4.3` | MIT + Apache-2.0 | LICENSE, LICENSE.libyaml, NOTICE |
| `golang.org/x/crypto` | `v0.44.0` | BSD (retain exact terms) | LICENSE |
| `golang.org/x/net` | `v0.47.0` | BSD (retain exact terms) | LICENSE |
| `golang.org/x/sync` | `v0.18.0` | BSD (retain exact terms) | LICENSE |
| `golang.org/x/sys` | `v0.39.0` | BSD (retain exact terms) | LICENSE |
| `golang.org/x/text` | `v0.31.0` | BSD (retain exact terms) | LICENSE |
| `golang.org/x/time` | `v0.9.0` | BSD (retain exact terms) | LICENSE |
| `google.golang.org/genproto/googleapis/api` | `v0.0.0-20251202230838-ff82c1b0f217` | Apache-2.0 | LICENSE |
| `google.golang.org/genproto/googleapis/rpc` | `v0.0.0-20251202230838-ff82c1b0f217` | Apache-2.0 | LICENSE |
| `google.golang.org/grpc` | `v1.77.0` | Apache-2.0 | LICENSE, NOTICE.txt |
| `google.golang.org/protobuf` | `v1.36.10` | BSD (retain exact terms) | LICENSE |
