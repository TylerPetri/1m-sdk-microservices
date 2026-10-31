# One local failure: Hello stalled, auth still usable

This experiment adds evidence about existing service boundaries. It adds no
service, fault endpoint, proxy, queue, or deployment component. It reuses the
normal disposable Compose smoke harness, including fresh Postgres, generated
local credentials, random loopback ports and cleanup of its own unique project.

```bash
make demo-failure
# Optional: preserve evidence in a chosen directory outside the source tree.
SDK_SMOKE_LOG_DIR=/tmp/sdk-failure-evidence make demo-failure
```

Use a local disposable Docker daemon, not a production Docker context. The
prerequisites are the same as `make smoke-compose`. The script creates its own
`sdkms-smoke-*` project; it does not stop an existing developer stack. Logs contain
no printed login token/password. Existing security and license decisions apply.

## Trigger, expected behavior, and evidence

| Phase | Trigger / assertion | Why it matters |
| --- | --- | --- |
| Baseline | Authenticate and call Hello: HTTP 200. | Proves the same route worked before the fault. |
| Failure | Pause only hellod in the disposable stack; confirm its paused state. | Suspends an established backend without adding a fault endpoint or relying on connection-refusal timing. |
| Boundary | Authenticated Hello carries a 500 ms Grpc-Timeout and returns HTTP 504 with gRPC code 4 (DeadlineExceeded). | The request fails within its RPC deadline even though the backend cannot respond. |
| Isolation | Login still returns HTTP 200 through authd and Postgres. | The Hello outage does not inherently take down the auth path. |
| Visibility | Prometheus observes a gateway duration/count sample labeled HTTP 504. | The failed request is measurable, even though the stopped backend cannot log it. |
| Limitation | Gateway admin `/readyz` still returns 200. | Current platform readiness does not prove downstream dependency availability. |
| Recovery | Unpause hellod; the original gateway serves HTTP 200 again. | Tests recovery through the existing client, not a restart of the entire system. |

The assertions are executable. `DEMO:` lines and `failure-demo.json` record actual
status, error code, measured failure duration, paused state, observed
metric count and recovery. The gateway container ID/restart count must remain
unchanged. `compose.log` captures the service/RPC logs, while `containers.txt`
and `cleanup.log` show the test stack and its teardown. No trace backend is
configured, so this experiment does not claim trace evidence.

The failing route is tried once after the pause: it must return 504/code 4 within
a two-second observation bound (500 ms RPC deadline plus CI scheduling margin).
Recovery is polled for at most 90 seconds to allow transport/scheduling recovery.
Metric collection has a separate 120-second observation budget; this is a test
polling budget, not a claim about production recovery time. The service is
unpaused in `finally`, and the shell cleanup trap removes the stack even when
an assertion fails. The default `make smoke-compose` remains a healthy-stack
check; CI runs `make demo-failure`, which includes that check first.

## Review of cancellation and failure behavior

| Concern | Existing behavior / trade-off |
| --- | --- |
| HTTP cancellation | Request context flows through the generated gateway to the RPC. Tests check cancellation reaches a waiting HTTP handler and traverses a real HTTP/gRPC connection to the RPC handler. This is cooperative cancellation, not forced termination of arbitrary work. |
| Deadlines | HTTP and unary gRPC policies supply defaults only when the caller has no deadline; an existing deadline is preserved, even if longer. The HTTP wrapper's standard `net/http.TimeoutHandler` returns **503**, despite its body text saying Gateway Timeout. A gRPC DeadlineExceeded that reaches grpc-gateway first maps to **504**. Tests distinguish wrapper behavior from RPC status mapping; there is no single universal timeout status across both races. |
| Service unavailable | A preliminary clean-stop experiment completed idle SIGTERM/exit 0, but the next request was still pending after the client's five-second timeout. A clean stop does not guarantee prompt connection-refusal semantics. The controlled pause/deadline experiment instead requires 504/code 4; it does not claim all outages fail identically. It adds no application retry loop/circuit breaker; reconnect timing belongs to the gRPC client. |
| Database failures | pgx operations receive request contexts. Initial connection has a retry budget; its synchronous backoff sleep is not immediately cancelable. Register maps most DB errors to Internal/500; Login maps lookup failures, including DB failures, to Unauthenticated/401 to avoid user enumeration. The readiness graph currently has no DB ping dependency. This demo does not inject a DB outage or certify those error classifications. |
| CPU work | Password hashing/verification is not context-interruptible. A client cancellation does not guarantee immediate termination of that CPU work. |
| Startup | Gateway waits for upstream transport readiness. Its wiring currently has no explicit startup deadline; signal handling in boot reaches its select loop after wiring. This experiment intentionally starts healthy before injecting the outage. |
| Shutdown | Hello/auth try GracefulStop, falling back to Stop on shutdown timeout. Gateway currently closes upstream connections before draining HTTP; in-flight requests can fail during that drain. Command mains discard boot's returned errors, and boot does not propagate every Serve error, so exit status is not a complete failure signal. The preliminary idle Hello exit is not an in-flight shutdown proof. |

These are explicit limits of a local demonstration, not production behavior
claims. There is no need to add infrastructure to make the portfolio credible.
If the project is extended later, the highest-value operational work would be a
small lifecycle change with tests: bounded/cancelable startup, drain ordering,
and reliable error exit status. That is preferable to adding more technologies.
