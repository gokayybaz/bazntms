# 0003 — Agent makine kimliği (C3 / Faz 13 S13.5–S13.6)

**Tarih:** 2026-09-05 · **Durum:** kabul edildi

## Sorun

`handleAgentHello` her `hello`'da koşulsuz `RegisterAgent` → yeni satır. Agent
state dosyasını kaybederse (disk temizliği, yeniden imaj, k8s node değişimi) ya
da yanlış yapılandırma döngüsüne girerse `agents` tablosu şişer; eski kayıtlar
"offline" olarak asılı kalır (C3, [[2026-09-05-mimari-degerlendirme]]).

Çözüm: `hello`'da kararlı bir **makine kimliği** taşı; hub eşleşen bir
**çevrimdışı** kayıt bulursa yeni satır açmak yerine o satırı yeni token'la
günceller.

## Karar — kimlik kaynağı

**`github.com/shirou/gopsutil/v3/host.HostID()`** — proje zaten `gopsutil/v3`'e
doğrudan bağımlı (`internal/sysmon`), `host` alt paketi yeni modül getirmiyor.
Platform başına:

| Platform | Kaynak |
|----------|--------|
| Linux    | `/sys/class/dmi/id/product_uuid` (root) → `/etc/machine-id` → `/proc/sys/kernel/random/boot_id` |
| macOS    | `ioreg -rd1 -c IOPlatformExpertDevice` → `IOPlatformUUID` |
| Windows  | Registry `HKLM\SOFTWARE\Microsoft\Cryptography\MachineGuid` |

Elle platform kodu yazmaya gerek yok. `HOST_ETC`/`HOST_SYS` env override'ları
gopsutil'de var (container senaryosu).

## Karar — türetme

```
machine_id = hex( sha256( hostID + "\x00" + hostname )[:16] )   // 128-bit
```

- **Neden hostname de karışıyor:** aynı imajdan klonlanan VM'ler `machine-id` /
  `product_uuid` değerini (regenerate edilene dek) paylaşır. Hostname bunları
  ayırır. Hostname değişirse (DHCP) agent "yeni" bir kimlik üretir → yeni satır
  açılır; bu bugünkü davranışın aynısı (regresyon değil, yalnızca kaçırılan
  dedup).
- **Ham `hostID` asla gönderilmez** — opak, gizlilik. Hash yeterli.
- `hostID` ve hostname ikisi de boşsa `machine_id = ""` → hub her zaman yeni
  satır açar (bugünkü davranış).

## Karar — hub tarafı eşleştirme

`RegisterOrReuseAgent(a Agent, offlineBefore int64) (id int64, reused bool, err error)`:

- `a.MachineID != ""` VE aynı `machine_id` + `site`'lı bir agent VARSA VE o
  agent'ın `last_seen < offlineBefore` (yani çevrimdışı) ise → o satır güncellenir
  (yeni `token_hash`, `name`, `version`, `remote_ip`, `last_seen`), `reused=true`.
- Eşleşen agent **çevrimiçi** ise → yeni satır (gerçekten iki agent, ya da
  yanlış yapılandırma; operatör iki kaydı görüp fark eder).
- `offlineBefore`: sunucu `now - 2*telemetryInterval` geçirir (sistemin geri
  kalanıyla tutarlı "online" penceresi).
- `machine_id` şemaya `0003_agent_machine_id` migrasyonuyla eklenir (her iki
  dialect'te düz `.sql` — kolon tümüyle yeni, çakışma yok).

Eski `RegisterAgent(a Agent) (int64, error)` testler için kalır.

## Reddedilen

- **MAC adresi:** sanal/bonded arayüzlerde kararsız, docker'da rastgele.
- **Yalnız hostname:** çok kırılgan (çok agent aynı "localhost").
- **Elle platform dosyaları okumak:** gopsutil zaten doğru sırayı + fallback'i
  ve env override'ları yapıyor.
