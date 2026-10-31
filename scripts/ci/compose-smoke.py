#!/usr/bin/env python3
"""Exercise real Dockerfile/Compose boundaries; never print credentials or tokens."""
import json
import os
import pathlib
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

# Every URL is a dynamically published localhost port on this disposable stack.
client = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def request(base, path, body=None, token=None, method=None):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = token
    req = urllib.request.Request(
        os.environ[base] + path,
        data=json.dumps(body).encode() if body is not None else None,
        headers=headers, method=method,
    )
    with client.open(req, timeout=5) as response:
        raw = response.read()
        return json.loads(raw) if raw and raw[:1] in (b"{", b"[") else raw


def eventually(check, label, timeout=120):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        try:
            if check():
                print("PASS:", label)
                return
        except (OSError, ValueError, KeyError, AssertionError):
            pass
        time.sleep(2)
    raise AssertionError("timed out: " + label)


for admin in ("GATEWAY_ADMIN_URL", "AUTH_ADMIN_URL", "HELLO_ADMIN_URL"):
    eventually(lambda: request(admin, "/readyz") is not None, admin + " ready")

credentials = {"email": "smoke@example.com", "password": "disposable-smoke-password"}
request("GATEWAY_URL", "/v1/auth/register", credentials)
login = request("GATEWAY_URL", "/v1/auth/login", credentials)
access = login["accessToken"]
identity = request("GATEWAY_URL", "/v1/auth/validate", {"access_token": access})
assert identity["userId"] == login["userId"]
hello = request("GATEWAY_URL", "/v1/hello/reviewer", token="Bearer " + access)
assert hello["message"] == "hello, reviewer"
for token in (None, "anything", "Bearer malformed", "Bearer " + login["refreshToken"]):
    try:
        request("GATEWAY_URL", "/v1/hello/reviewer", token=token)
    except urllib.error.HTTPError as error:
        assert error.code == 401
        assert error.headers.get("WWW-Authenticate")
        assert json.load(error)["code"] == 16
    else:
        raise AssertionError("invalid authorization reached protected route")
print("PASS: real HTTP/gRPC/Postgres registration, login, validation, Hello, and auth rejection")

# Send a real OTLP metric through the optional collector, then verify scraping.
metric = {"resourceMetrics": [{"scopeMetrics": [{"metrics": [{
    "name": "portfolio_smoke", "gauge": {"dataPoints": [{"asInt": "1"}]}
}]}]}]}
eventually(lambda: request("OTEL_URL", "/v1/metrics", metric) is not None, "collector accepts OTLP metric")


def targets_up():
    targets = request("PROMETHEUS_URL", "/api/v1/targets")["data"]["activeTargets"]
    required = {"gateway", "auth", "hello", "prometheus", "otel-collector"}
    healthy = {t["labels"]["job"] for t in targets if t["health"] == "up"}
    return required <= healthy


eventually(targets_up, "all five Prometheus scrape targets healthy")
for query in ('http_server_duration_seconds_count{job="gateway"}', 'rpc_server_duration_seconds_count{job="auth"}'):
    eventually(lambda: bool(request("PROMETHEUS_URL", "/api/v1/query?" + urllib.parse.urlencode({"query": query}))["data"]["result"]),
               "application metric exists: " + query)
eventually(lambda: bool(request("PROMETHEUS_URL", "/api/v1/query?query=portfolio_smoke")["data"]["result"]),
           "OTLP metric reaches Prometheus")
eventually(lambda: request("GRAFANA_URL", "/api/health")["database"] == "ok", "Grafana healthy")
eventually(lambda: any(d.get("uid") == "sdkms-overview" for d in request("GRAFANA_URL", "/api/search")),
           "Grafana provisions overview dashboard")
eventually(lambda: request("LOKI_URL", "/ready") is not None, "Loki ready")
query = '{container=~"' + os.environ["COMPOSE_PROJECT_NAME"] + '-gatewayd.*"}'
eventually(lambda: bool(request("LOKI_URL", "/loki/api/v1/query_range?" + urllib.parse.urlencode({"query": query}))["data"]["result"]),
           "Promtail delivers this stack's gateway logs to Loki")


if os.environ.get("SDK_FAILURE_DEMO") == "1":
    # Only the unique stack created by compose-smoke.sh may be disrupted.
    project = os.environ["COMPOSE_PROJECT_NAME"]
    compose = sys.argv[1:]
    if not project.startswith("sdkms-smoke-") or not compose or "-p" not in compose or compose[compose.index("-p") + 1] != project:
        raise RuntimeError("failure demo requires the disposable smoke project")
    evidence = []

    def record(stage, **values):
        row = {"stage": stage, **values}
        evidence.append(row)
        print("DEMO:", json.dumps(row, sort_keys=True), flush=True)

    def probe():
        start = time.monotonic()
        try:
            body = request("GATEWAY_URL", "/v1/hello/reviewer", token="Bearer " + access)
            return 200, body, time.monotonic() - start
        except urllib.error.HTTPError as error:
            return error.code, json.load(error), time.monotonic() - start

    gateway_id = subprocess.check_output(compose + ["ps", "-q", "gatewayd"], text=True).strip()
    gateway_restarts = subprocess.check_output(["docker", "inspect", "--format", "{{.RestartCount}}", gateway_id], text=True).strip()
    status, _, elapsed = probe()
    assert status == 200
    record("baseline", expected_http=200, actual_http=status)
    try:
        subprocess.run(compose + ["stop", "-t", "15", "hellod"], check=True)
        record("trigger", action="stop hellod", scope="disposable Compose project")
        hello_id = subprocess.check_output(compose + ["ps", "-a", "-q", "hellod"], text=True).strip()
        exit_code = int(subprocess.check_output(["docker", "inspect", "--format", "{{.State.ExitCode}}", hello_id], text=True))
        shutdown_logs = subprocess.check_output(compose + ["logs", "--no-color", "hellod"], text=True)
        assert exit_code == 0 and "shutdown signal" in shutdown_logs
        record("idle_shutdown", expected_exit=0, actual_exit=exit_code, signal_log_seen=True)
        status, body, elapsed = probe()
        assert status == 503 and body["code"] == 14, "expected gRPC Unavailable mapped to HTTP 503"
        record("failure", expected_http=503, actual_http=status, grpc_code=body["code"], elapsed_seconds=round(elapsed, 3))
        # This route needs authd/Postgres, but not Hello.
        still_logged_in = request("GATEWAY_URL", "/v1/auth/login", credentials)
        assert still_logged_in["userId"] == login["userId"]
        record("isolation", endpoint="auth/login", expected_http=200, actual_http=200)
        ready = request("GATEWAY_ADMIN_URL", "/readyz")
        assert ready is not None
        record("readiness_limit", expected_http=200, actual_http=200,
               explanation="platform readiness does not check Hello connectivity")
        query = 'sum(http_server_duration_seconds_count{job="gateway",http_status_code="503"})'
        metric_value = []

        def failure_metric():
            result = request("PROMETHEUS_URL", "/api/v1/query?" + urllib.parse.urlencode({"query": query}))["data"]["result"]
            if result and float(result[0]["value"][1]) >= 1:
                metric_value[:] = [float(result[0]["value"][1])]
                return True
            return False

        eventually(failure_metric, "injected HTTP 503 appears in gateway metrics")
        record("metric", series="http_server_duration_seconds_count", http_status_code="503", observed_count=metric_value[0])
    finally:
        # Restore the service even if an assertion fails; the shell trap then
        # captures logs and removes the entire disposable stack.
        subprocess.run(compose + ["start", "hellod"], check=True)

    def recovered():
        status, body, _ = probe()
        return status == 200 and body.get("message") == "hello, reviewer"

    eventually(recovered, "Hello recovers through the existing gateway connection", timeout=90)
    assert subprocess.check_output(compose + ["ps", "-q", "gatewayd"], text=True).strip() == gateway_id
    assert subprocess.check_output(["docker", "inspect", "--format", "{{.RestartCount}}", gateway_id], text=True).strip() == gateway_restarts
    record("recovery", action="start hellod", expected_http=200, actual_http=200,
           gateway_restarted=False)
    path = pathlib.Path(os.environ["SDK_SMOKE_LOG_DIR"]) / "failure-demo.json"
    path.write_text(json.dumps(evidence, indent=2) + "\n")
    print("PASS: controlled Hello outage, isolation, evidence, and recovery")
