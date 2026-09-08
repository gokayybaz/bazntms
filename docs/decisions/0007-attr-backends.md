# 0007 — Süreç-atıf arka uçları: eBPF / ETW / pcap (Faz 20)

**Tarih:** 2026-09-07 · **Durum:** kabul edildi ve **canlı doğrulandı** — Linux
eBPF (scale compose), Windows ETW (gerçek makine, `attr_method=etw`, süreç +
DNS panelleri dolu). Doğrulama sırasında bulunan 3 hata düzeltildi:
`traceLogfileHeader` eksik union alanı (callback hiç çağrılmıyordu),
Kernel-Network keyword maskesi (0x10/0x20, 0xF değil), TCP recv uzak-uç
ayrıştırması. Bkz. commit `6e89910`, `434ccb1`, `d0764d6`.

## Sorun

Süreç bazlı trafik atfı (+ L7 + DNS görünürlüğü) tek yöntemle çalışıyordu:
**nethogs yöntemi** — agent pcap ile her paketi yakalar, `/proc/net/*` (veya
`lsof`/`netstat`) soket-tablosuyla PID'e eşler. Sorunlar:

- Linux'ta `CAP_NET_RAW`, Windows'ta **Npcap kurulumu** ister — kurulumun
  "sadece agent'ı çalıştır" olması hedefine aykırı.
- Her *pakette* gopacket parse + map lookup → yoğun host'ta ölçülebilir CPU;
  NIC offload'ında (GRO/LRO/TSO) byte sayımı şişer.
- Windows'ta Npcap bir NDIS filtre sürücüsü kurar (her zaman açık kernel yükü).

Orijinal yol haritası (Modül 2) bunu "eBPF (Linux) / ETW (Windows) ile
platform başına en derin toplama" olarak işaretlemişti.

## Karar

`internal/agent/attrsource.go` `AttrSource` arayüzü — üç delta akışı
(`Deltas` / `L7Deltas` / `DNSDeltas`) + `Stop` + `Method`. Seçici
`newAttrSource(cfg, caps)` platform + `collect.method` + çalışma-zamanı
yeteneğine göre bir arka uç kurar ve **kurulamayan her adımda bir öncekine
zarif düşer**.

| Arka uç | Platform | Mekanizma | Gereksinim |
|---|---|---|---|
| **eBPF** | Linux | CO-RE **fentry** (`tcp/udp_sendmsg`, `tcp_cleanup_rbuf`, `skb_consume_udp`) → `LRU_HASH` + `RINGBUF` (DNS) | kernel ≥ 5.8 + BTF + `CAP_BPF`/`CAP_SYS_ADMIN` |
| **ETW** | Windows | elle sarılmış advapi32 tüketicisi; `Microsoft-Windows-Kernel-Network` + `Microsoft-Windows-DNS-Client` | yükseltilmiş süreç (SYSTEM / yönetici) |
| **pcap** | tümü | bugünkü nethogs motoru (`internal/agent/attr.go`) | `CAP_NET_RAW` / Npcap |

`auto` (varsayılan): Linux `eBPF → pcap`, Windows `ETW → pcap`, macOS `pcap`.
`collect.method` (`ebpf`|`etw`|`pcap`) bir arka ucu **zorlar** (kurulamazsa
hata, düşme yok); `off` atfı tamamen kapatır.

### Neden eBPF/ETW daha ucuz

Her *pakette* değil her *soket send/recv işleminde* çalışır (64 KB'lik bir
`sendmsg` = 1 çağrı ↔ ~44 paket). Payload kopyalanmaz (yalnız DNS için
~512 B / QueryName). Byte sayımı soket düzeyinde — offload'dan bağımsız,
kesin. `CAP_NET_RAW` ve Windows'ta Npcap gerekmez.

## Alt kararlar

### 1. eBPF programları: `fentry`, kprobe değil

`fentry` + `BPF_PROG` makrosu `pt_regs`'e dokunmaz → derlenen nesne
**mimariden bağımsız**: tek `attrprog_bpfel.o` amd64 + arm64. (kprobe olsaydı
`PT_REGS_PARM*` mimariye göre değişir, per-arch derleme gerekirdi.) Bedeli:
arm64'te fentry kernel ≥ 6.0 ister; yetersiz ortam pcap'e düşer.

### 2. Üretilen BPF çıktısı repoya commit'li

`bpf2go` ile C'den derlenen `attrprog_bpfel.{go,o}` + tam `vmlinux.h`
repoya girer → `go build` ve normal CI **clang gerektirmez**. `make
generate-bpf` (sabit `golang:1.26-bookworm` + `clang-14`, Docker) yalnız
BPF C'si değişince; CI `ebpf-generate` işi `.o` sha256'nın byte-tekrarlanabilir
olduğunu doğrular. `.gitignore` yerine `.gitattributes linguist-generated`.

### 3. Windows ETW: elle yazıldı, kütüphane yok

| Aday | Sonuç |
|---|---|
| `github.com/0xrawsec/golang-etw` | **GPL-3.0** — MIT + open-core hedefiyle uyumsuz (agent binary'si GPL'e tabi olurdu). Reddedildi. |
| `github.com/bi-zone/etw` | MIT ama **cgo** (`session.c`). Windows agent `CGO_ENABLED=0`; cgo mingw-w64 zincirini geri getirir (Faz 19'da kaldırılmıştı). Reddedildi. |
| `golang.org/x/sys/windows` | ETW *provider* tarafı var, *consumer* (`StartTrace`/`ProcessTrace`/…) **yok**. |
| **Elle** (`advapi32` syscall + SDK struct'ları) | Kernel-Network olay düzeni sabit + belgeli → TDH gerekmez, `UserData` ofsetle çözülür. Saf Go, yeni dep yok, MIT-temiz. **Seçildi.** |

Struct düzenleri `checkLayout()` ile çalışma anında + `TestETWLayout` ile
Windows CI'da doğrulanır — uyuşmazlıkta oturum açılmaz (bellek bozulması yerine
pcap fallback).

### 4. L7 (SNI/Host) pcap-gated kalır

Payload gerektirir; eBPF/ETW taşımaz. eBPF modunda dar filtreli bir yardımcı
pcap handle (`l7helper.go`) ile alınır (`CAP_NET_RAW` ister — açılamazsa L7
sessiz boş, sayım + DNS aksamaz). ETW modunda L7 **yoktur** —
`-collect-method=pcap` + Npcap gerekir. Standart-dışı portlar (9443 vb.)
yardımcı handle filtresinde yok — kabul edilmiş sınır.

**Sonuç (2026-09-07 canlı doğrulama):** "Npcap bağımlılığını kaldır" hedefi
**kısmen** karşılandı — çekirdek görünürlük (süreç trafiği + per-süreç DNS)
Windows'ta Npcap'siz çalışıyor, ama ayrı **L7/SNI paneli Npcap gerektirmeye
devam ediyor**. Per-süreç DNS pratikte uygulama görünürlüğünün büyük kısmını
verdiği için varsayılan dağıtım önerisi "tam özellik = Npcap kur" olarak
kalıyor; ETW `collect.method=auto` fallback'i Npcap'siz kurulumlarda çekirdeği
korur. ETW ile kısmi L7 (`Microsoft-Windows-WinINet`/`WinHTTP`/`Schannel` —
OS HTTP yığını app'leri kapsar, Chromium/Firefox/Electron hariç) mümkün ama
descope edildi (ayrı faz).

**Güncelleme (v1.3.0, 2026-09-08):** "Npcap'i kullanıcı elle kursun" kararı
**geri alındı**. Windows MSI kurulumu artık Npcap'i sessizce indirip kurar
(`deploy/msi/install-npcap.ps1` — deferred/SYSTEM CustomAction, sabit sürüm +
SHA-256 + Authenticode "Nmap Software LLC" doğrulaması, `exit 0` garantili) ve
Windows agent `collect.method: pcap` ile gelir → **L7/SNI tüm platformlarda
varsayılan çalışır.** Npcap indirilemezse (hava boşluğu, imza/hash hatası) MSI
yine başarılı biter ve agent ETW'ye düşer. Gerekçe: kullanıcı geri bildirimi —
"tam görünürlük varsayılan olmalı, Npcap engeli kabul edilemez"; NSIS `/S`
sessiz kurulumu + gopacket'in Npcap-dizini otomatik bulması bunu güvenilir
kıldı. Aynı sürümde agent derin toplama + hub `-agent-pcap` politikası da
varsayılan açığa çekildi.

### 5. eBPF DNS: yalnız yanıtlar

`skb_consume_udp`'de (recv) yakalanır; yanıt soru bölümünü taşıdığından her
domain görünür. Outbound sorgu yakalamak `msg_iter` okumayı gerektirir
(kernel-sürüm kırılgan) → ertelendi. Sorgu/yanıt sayı ayrımı yaklaşık,
domain listesi tam. (ETW'de 3006+3008 ile ayrım tam.)

### 6. macOS pcap-only

Endpoint Security `NEFilterDataProvider` süreç-atıflı akış verebilir ama
notarization + `com.apple.developer.networking` entitlement maliyeti var →
**kapsam dışı** (20-G).

## Kalan

- ETW ile kısmi L7 (`WinINet`/`WinHTTP`/`Schannel` sağlayıcıları) — OS HTTP
  yığını app'leri için Npcap'siz SNI/Host; tarayıcılar yine Npcap ister.
- eBPF L7 (`uprobe/SSL_write` ile şifresiz SNI), Windows NDIS-PacketCapture
  ile L7/`-record` paritesi (20-F), macOS ES (20-G) — ayrı fazlar.
