#!/usr/bin/env python3
"""deb/rpm kurulum sihirbazi (deploy/nfpm/postinstall.sh) icin uctan uca duman
testi — Docker + gercek pty (pexpect) uzerinden.

Bu script, 2026-09-01'de bu kod tabaninda bulunan iki gercek hatayi (RPM
%post'un stdin'i asla terminale bagli gostermemesi + /dev/tty duzeltmesinin
ilk halinin otomasyon senaryosunu bozmasi) elle test ederken ortaya cikardigi
senaryolarin otomatiklestirilmis halidir — bkz. commit 3f0b24e. Amac:
bir sonraki benzer regresyonun ilk push'ta CI'da yakalanmasi, tekrar elle
kesfedilmesi degil.

Kullanim: CI'da (ci.yml, "Installer smoke test" job'u) DEB_PKG ve RPM_PKG
ortam degiskenleriyle cagrilir; yerel calistirmak icin de ayni degiskenleri
export edip dogrudan calistirilabilir.
"""
import os
import subprocess
import sys

import pexpect

DEB_PKG = os.environ["DEB_PKG"]
RPM_PKG = os.environ["RPM_PKG"]

FAILURES = []


def check(name, cond, detail=""):
    status = "OK" if cond else "FAIL"
    print(f"[{status}] {name}" + (f" — {detail}" if detail and not cond else ""))
    if not cond:
        FAILURES.append(name)


def run_noninteractive(image, pkg_path, install_cmd):
    """TTY olmadan (otomasyon senaryosu) kurulum — dpkg/rpm'in tek basina
    ASLA basarisiz OLMAMASI (exit 0) ve bos bir agent.yml uretmesi beklenir."""
    out = subprocess.run(
        ["docker", "run", "--rm", "-v", f"{pkg_path}:/pkg",
         image, "bash", "-c",
         f"{install_cmd} /pkg >/tmp/install.log 2>&1; "
         f"echo EXIT:$?; cat /tmp/install.log; "
         f"echo ---; cat /etc/bazntms/agent.yml 2>&1"],
        capture_output=True, text=True, timeout=120,
    )
    return out.stdout + out.stderr


def run_interactive(image, pkg_path, install_cmd, hub_url, token, site):
    """Gercek pty uzerinden (pexpect) prompt'lari cevaplayarak kurulum.

    Prompt hic gorunmezse (ornegin bir regresyon RPM'de sorguyu sessizce
    atlarsa) pexpect EOF/TIMEOUT firlatir — bunu burada yakalayip o ana
    kadarki ciktiyla birlikte cagirana donduruyoruz, boylece diger
    senaryolar calismaya devam eder ve hata temiz bir FAIL olarak
    raporlanir (yakalanmamis bir traceback yerine).
    """
    log = []
    child = pexpect.spawn(
        "docker", ["run", "--rm", "-i", "-t", "-v", f"{pkg_path}:/pkg",
                   image, "bash", "-c",
                   f"{install_cmd} /pkg; echo EXIT:$?; cat /etc/bazntms/agent.yml"],
        timeout=60, encoding="utf-8",
    )
    child.logfile_read = Writer(log)
    try:
        child.expect("Hub adresi", timeout=30)
        child.sendline(hub_url)
        child.expect("Kayit")
        child.sendline(token)
        child.expect("Site")
        child.sendline(site)
        child.expect(pexpect.EOF, timeout=30)
    except (pexpect.exceptions.TIMEOUT, pexpect.exceptions.EOF) as e:
        log.append(f"\n[pexpect] prompt beklenirken kesildi: {type(e).__name__} — sihirbaz hic gorunmemis olabilir\n")
    finally:
        try:
            child.close(force=True)
        except Exception:
            pass
    return "".join(log)


class Writer:
    def __init__(self, buf):
        self.buf = buf

    def write(self, s):
        self.buf.append(s)

    def flush(self):
        pass


def main():
    scenarios = [
        ("DEB", "debian:12-slim", DEB_PKG, "dpkg -i"),
        ("RPM", "fedora:40", RPM_PKG, "rpm -ivh"),
    ]

    for fmt, image, pkg, install_cmd in scenarios:
        print(f"\n=== {fmt} / otomasyon (TTY yok) ===")
        out = run_noninteractive(image, pkg, install_cmd)
        print(out)
        check(f"{fmt} otomasyon: kurulum basarili (EXIT:0)", "EXIT:0" in out)
        check(f"{fmt} otomasyon: bos agent.yml uretildi",
              "url: ''" in out and "token: ''" in out)
        check(f"{fmt} otomasyon: /dev/tty hatasi TUM kurulumu dusurmedi",
              "cannot create /dev/tty" not in out and "No such device" not in out)

        print(f"\n=== {fmt} / interaktif (gercek pty) ===")
        out = run_interactive(image, pkg, install_cmd,
                               "http://smoketest.local:8081", "smoketest-token", "ci-smoketest")
        print(out)
        check(f"{fmt} interaktif: kurulum basarili (EXIT:0)", "EXIT:0" in out)
        check(f"{fmt} interaktif: girilen deger dogru yazildi",
              "http://smoketest.local:8081" in out and "smoketest-token" in out and "ci-smoketest" in out)

    if FAILURES:
        print(f"\n{len(FAILURES)} kontrol basarisiz:")
        for f in FAILURES:
            print(f"  - {f}")
        sys.exit(1)
    print(f"\nTum kontroller basarili.")


if __name__ == "__main__":
    main()
