#!/usr/bin/env python3
"""Exercise real Dockerfile/Compose boundaries; never print credentials or tokens."""
import json
import os
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
eventually(lambda: bool(request("PROMETHEUS_URL", "/api/v1/query?query=portfolio_smoke")["data"]["result"]),
           "OTLP metric reaches Prometheus")
eventually(lambda: request("GRAFANA_URL", "/api/health")["database"] == "ok", "Grafana healthy")
eventually(lambda: any(d.get("uid") == "sdkms-overview" for d in request("GRAFANA_URL", "/api/search")),
           "Grafana provisions overview dashboard")
eventually(lambda: request("LOKI_URL", "/ready") is not None, "Loki ready")
query = '{container=~"' + os.environ["COMPOSE_PROJECT_NAME"] + '-gatewayd.*"}'
eventually(lambda: bool(request("LOKI_URL", "/loki/api/v1/query_range?" + urllib.parse.urlencode({"query": query}))["data"]["result"]),
           "Promtail delivers this stack's gateway logs to Loki")
