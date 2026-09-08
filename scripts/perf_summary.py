#!/usr/bin/env python3
"""Faz 21 yük koşusu özetleyici (S21.3).

İki /metrics anlık görüntüsü (koşu öncesi/sonrası) + loadgen JSON özeti +
profil eşiklerini alır; markdown rapor üretir ve eşik ihlallerinde çıkış
kodu 1 döndürür.

Kullanım:
  perf_summary.py --before b.prom --after a.prom --loadgen lg.json \
                  --profile loadtest/profiles/target.env --elapsed 600 \
                  --out docs/perf/runs/<utc>-target.md
"""
import argparse
import json
import re
import sys
import time

LINE = re.compile(r'^([a-zA-Z_:][a-zA-Z0-9_:]*)(\{[^}]*\})?\s+([-+0-9.eE]+|NaN|\+Inf|-Inf)\s*$')
LBL = re.compile(r'([a-zA-Z_][a-zA-Z0-9_]*)="((?:[^"\\]|\\.)*)"')


def parse_prom(path):
    """prom text → list of (name, {labels}, value)."""
    out = []
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            m = LINE.match(line)
            if not m:
                continue
            name, lblblob, val = m.group(1), m.group(2) or "", m.group(3)
            try:
                v = float(val)
            except ValueError:
                v = float("nan")
            labels = {k: vv for k, vv in LBL.findall(lblblob)}
            out.append((name, labels, v))
    return out


def total(samples, name, pred=None):
    s = 0.0
    for n, lbl, v in samples:
        if n == name and (pred is None or pred(lbl)):
            s += v
    return s


def one(samples, name, pred=None, default=0.0):
    for n, lbl, v in samples:
        if n == name and (pred is None or pred(lbl)):
            return v
    return default


def read_profile(path):
    cfg = {}
    with open(path) as f:
        for line in f:
            line = line.split("#", 1)[0].strip()
            if "=" in line:
                k, v = line.split("=", 1)
                cfg[k.strip()] = v.strip()
    return cfg


def num(cfg, key, default=None):
    if key not in cfg:
        return default
    try:
        return float(cfg[key])
    except ValueError:
        return default


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--before", required=True)
    ap.add_argument("--after", required=True)
    ap.add_argument("--loadgen", required=True)
    ap.add_argument("--profile", required=True)
    ap.add_argument("--elapsed", type=float, required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--stack", default="single-node")
    ap.add_argument("--label", default="")
    args = ap.parse_args()
    label = args.label or args.profile

    b = parse_prom(args.before)
    a = parse_prom(args.after)
    with open(args.loadgen) as f:
        lg = json.load(f)
    cfg = read_profile(args.profile)
    el = max(args.elapsed, 1.0)

    is_tel = lambda l: l.get("path", "").endswith("/api/v1/agent/telemetry")
    tel_reqs = total(a, "bazntms_http_requests_total", is_tel) - total(b, "bazntms_http_requests_total", is_tel)
    tel_2xx = total(a, "bazntms_http_requests_total", lambda l: is_tel(l) and l.get("status", "").startswith("2")) \
        - total(b, "bazntms_http_requests_total", lambda l: is_tel(l) and l.get("status", "").startswith("2"))
    tel_rps = tel_reqs / el
    tel_err_rate = 0.0 if tel_reqs == 0 else (tel_reqs - tel_2xx) / tel_reqs

    flow_recv = total(a, "bazntms_flows_received_total") - total(b, "bazntms_flows_received_total")
    flow_drop = total(a, "bazntms_flows_dropped_total") - total(b, "bazntms_flows_dropped_total")
    flow_rate = flow_recv / el
    flow_drop_rate = flow_drop / el

    q_pending = one(a, "bazntms_queue_pending", default=0.0)

    dp_cnt = one(a, "bazntms_devpoll_cycle_duration_seconds_count") - one(b, "bazntms_devpoll_cycle_duration_seconds_count")
    dp_sum = one(a, "bazntms_devpoll_cycle_duration_seconds_sum") - one(b, "bazntms_devpoll_cycle_duration_seconds_sum")
    dp_avg = dp_sum / dp_cnt if dp_cnt > 0 else 0.0

    db_wait = one(a, "bazntms_db_pool_wait_count_total") - one(b, "bazntms_db_pool_wait_count_total")
    db_inuse = one(a, "bazntms_db_pool_in_use", default=0.0)

    # store_write per table (ortalama süre ms)
    tables = sorted({l.get("table", "") for n, l, _ in a if n == "bazntms_store_write_duration_seconds_count"})
    writes = []
    for t in tables:
        p = lambda l, t=t: l.get("table") == t
        c = one(a, "bazntms_store_write_duration_seconds_count", p) - one(b, "bazntms_store_write_duration_seconds_count", p)
        s = one(a, "bazntms_store_write_duration_seconds_sum", p) - one(b, "bazntms_store_write_duration_seconds_sum", p)
        rows = total(a, "bazntms_store_write_rows_total", p) - total(b, "bazntms_store_write_rows_total", p)
        if c > 0:
            writes.append((t, c, (s / c) * 1000.0, rows))

    # --- eşik kontrolleri ---
    checks = []

    def chk(name, ok, got, want):
        checks.append((name, ok, got, want))

    ag = lg.get("agent")
    if ag:
        p95 = ag.get("p95_ms", 0.0)
        mx = num(cfg, "MAX_P95_MS")
        if mx is not None:
            chk("agent p95 gecikme", p95 <= mx, f"{p95:.0f} ms", f"≤ {mx:.0f} ms")
        mr = num(cfg, "MIN_TELEMETRY_RPS")
        if mr is not None:
            # loadgen'in kendi rps'i istemci-tarafı gerçek; /metrics paylaşımlı
            # yığında başka istemci gürültüsü taşıyabilir → yalnız bilgi amaçlı.
            chk("agent telemetri rps", ag.get("rps", 0.0) >= mr, f"{ag.get('rps', 0.0):.0f}/sn", f"≥ {mr:.0f}/sn")
        # hata oranı da loadgen sayaçlarından (dial + http); /metrics 2xx-dışı
        # oranı raporda ayrıca gösterilir ama eşik loadgen'e bakar.
        sent = ag.get("sent", 0)
        failed = ag.get("failed", 0)
        lg_err = 0.0 if (sent + failed) == 0 else failed / (sent + failed)
        chk("agent hata oranı", lg_err <= 0.001, f"{lg_err*100:.2f}% (loadgen)", "≤ 0.1%")

    mq = num(cfg, "MAX_QUEUE_PENDING")
    if mq is not None:
        chk("kuyruk birikimi (son)", q_pending <= mq, f"{q_pending:.0f}", f"≤ {mq:.0f}")

    fl = lg.get("flow")
    if fl or flow_recv > 0:
        md = num(cfg, "MAX_FLOW_DROP_RATE", 0.0)
        chk("flow düşürme oranı", flow_drop_rate <= md, f"{flow_drop_rate:.1f}/sn", f"≤ {md:.1f}/sn")
        want_fr = num(cfg, "FLOW_RATE")
        if want_fr:
            chk("flow alım hızı", flow_rate >= want_fr * 0.9, f"{flow_rate:.0f}/sn", f"≥ {want_fr*0.9:.0f}/sn (hedef %90)")

    mc = num(cfg, "MAX_DEVPOLL_CYCLE_S")
    if mc is not None and dp_cnt > 0:
        chk("cihaz poll döngüsü (ort)", dp_avg <= mc, f"{dp_avg:.1f} s", f"≤ {mc:.0f} s")

    passed = all(ok for _, ok, _, _ in checks)

    # --- markdown ---
    lines = []
    lines.append(f"# Yük koşusu — {label}")
    lines.append("")
    lines.append(f"- Tarih (UTC): {time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())}")
    lines.append(f"- Yığın: {args.stack}")
    lines.append(f"- Süre: {el:.0f} s")
    lines.append(f"- Sonuç: **{'PASS' if passed else 'FAIL'}**")
    lines.append("")
    lines.append("## Eşikler")
    lines.append("")
    lines.append("| Kontrol | Sonuç | Ölçülen | Hedef |")
    lines.append("|---|---|---|---|")
    for name, ok, got, want in checks:
        lines.append(f"| {name} | {'✅' if ok else '❌'} | {got} | {want} |")
    lines.append("")
    lines.append("## loadgen (istemci tarafı)")
    lines.append("")
    lines.append("```json")
    lines.append(json.dumps(lg, indent=2, ensure_ascii=False))
    lines.append("```")
    lines.append("")
    lines.append("## Hub /metrics deltaları")
    lines.append("")
    lines.append(f"- telemetri istekleri: {tel_reqs:.0f} ({tel_rps:.0f}/sn), 2xx-dışı oran {tel_err_rate*100:.2f}%")
    lines.append(f"- flow alınan: {flow_recv:.0f} ({flow_rate:.0f}/sn), düşürülen: {flow_drop:.0f}")
    lines.append(f"- kuyruk birikimi (son ölçüm): {q_pending:.0f}")
    lines.append(f"- DB havuzu: kullanımda {db_inuse:.0f}, bekleme sayısı deltası {db_wait:.0f}")
    if dp_cnt > 0:
        lines.append(f"- devpoll: {dp_cnt:.0f} döngü, ortalama {dp_avg:.2f} s")
    if writes:
        lines.append("")
        lines.append("| Tablo | Yazım | Ort. süre | Satır |")
        lines.append("|---|---|---|---|")
        for t, c, ms, rows in writes:
            lines.append(f"| {t} | {c:.0f} | {ms:.2f} ms | {rows:.0f} |")
    lines.append("")

    with open(args.out, "w") as f:
        f.write("\n".join(lines) + "\n")

    print(f"\n=== {label} — {'PASS' if passed else 'FAIL'} ===")
    for name, ok, got, want in checks:
        print(f"  [{'OK' if ok else 'FAIL'}] {name}: {got} (hedef {want})")
    print(f"\nrapor → {args.out}")
    sys.exit(0 if passed else 1)


if __name__ == "__main__":
    main()
