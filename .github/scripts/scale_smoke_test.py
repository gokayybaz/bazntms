#!/usr/bin/env python3
"""docker-compose.scale.yml (N x hub-ingest + 1 x hub-controller + nginx lb +
TimescaleDB + NATS JetStream + gercek agent'lar) icin uctan uca duman testi.

installer_smoke_test.py'den ayri bir amaci var: paketleme degil, MIMARI
regresyonlari yakalamak — LB'nin agent trafigini ingest havuzuna dogru
yonlendirdigini, NATS JetStream'in "store-writer" tuketicisini ingest
replikalari arasinda paylastigini, ve nihayetinde agent verisinin gercekten
hub'in panosuna (dashboard API) ulastigini dogrular. Yigin zaten
`docker compose up --wait` ile ayaga kaldirilmis olmali (bkz. ci.yml).

Kullanim: HUB_URL (varsayilan http://localhost:8080) ve AUTH_PASSWORD
(varsayilan demo123) ortam degiskenleriyle calisir.
"""
import json
import os
import sys
import time
import urllib.error
import urllib.request

HUB_URL = os.environ.get("HUB_URL", "http://localhost:8080")
AUTH_PASSWORD = os.environ.get("AUTH_PASSWORD", "demo123")

FAILURES = []


def check(name, cond, detail=""):
    status = "OK" if cond else "FAIL"
    print(f"[{status}] {name}" + (f" — {detail}" if detail and not cond else ""))
    if not cond:
        FAILURES.append(name)


def http_json(method, path, body=None, cookie=None):
    url = HUB_URL + path
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    if data is not None:
        req.add_header("Content-Type", "application/json")
    if cookie:
        req.add_header("Cookie", cookie)
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            raw = resp.read()
            return resp.status, resp.headers, (json.loads(raw) if raw else None)
    except urllib.error.HTTPError as e:
        raw = e.read()
        try:
            return e.code, e.headers, json.loads(raw)
        except Exception:
            return e.code, e.headers, None


def main():
    # 1) hub-controller (dashboard) ve lb (agent API havuzu) healthz
    for name, url in [("hub-controller /healthz", HUB_URL + "/healthz"), ("lb /healthz", "http://localhost:8081/healthz")]:
        try:
            with urllib.request.urlopen(url, timeout=10) as resp:
                check(name, resp.status == 200)
        except Exception as e:
            check(name, False, str(e))

    # 2) giris (session cookie)
    status, headers, out = http_json("POST", "/api/login", {"password": AUTH_PASSWORD})
    check("giris (demo şifre)", status == 200, f"status={status} out={out}")
    cookie = None
    if status == 200:
        set_cookie = headers.get("Set-Cookie", "")
        cookie = set_cookie.split(";")[0] if set_cookie else None
    if not cookie:
        print("cookie alinamadi, kalan kontroller atlaniyor")
        sys.exit(1)

    # 3) agent filosu — LB uzerinden enroll olup telemetri gonderen 2
    # gercek agent (docker-compose.scale.yml: agent servisi replicas=2)
    # dashboard API'sinde online gorunmeli. "online" alani enrollment
    # hemen sonrasi (ilk telemetriden once bile) true olabiliyor — asil
    # kanit "rates" doluluğu, cunku o ArDIŞIK IKI ornek arasindaki
    # delta'dan hesaplaniyor (bkz. store.ListAgents), yani en az bir
    # agent telemetri araligi (varsayilan 30sn) + bir sonraki oraninca
    # gecmesini gerektirir. Ikisi de saglanana kadar (NATS JetStream
    # uzerinden asenkron yazim dahil) yeniden denenir.
    online = []
    deadline = time.time() + 150
    while time.time() < deadline:
        status, _, agents = http_json("GET", "/api/v1/agents", cookie=cookie)
        if status == 200 and agents:
            online = [a for a in agents if a.get("online")]
            if len(online) >= 2 and any(a.get("rates") for a in online):
                break
        time.sleep(5)

    check("en az 2 agent online (LB → ingest havuzu → NATS → hub okuma yolu)", len(online) >= 2,
          f"gelen online sayisi: {len(online)}")

    has_rates = any(a.get("rates") for a in online)
    check("online agent'larda telemetri verisi (rates) var", has_rates,
          f"agents: {json.dumps(online)[:500]}")

    docker_scale_sites = [a.get("site") for a in online if a.get("site") == "docker-scale"]
    check("agent'lar dogru site etiketiyle (docker-scale) kayitli", len(docker_scale_sites) >= 2)

    if FAILURES:
        print(f"\n{len(FAILURES)} kontrol basarisiz:")
        for f in FAILURES:
            print(f"  - {f}")
        sys.exit(1)
    print("\nTum kontroller basarili.")


if __name__ == "__main__":
    main()
