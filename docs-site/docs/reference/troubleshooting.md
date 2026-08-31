---
title: Sorun Giderme
sidebar_position: 15
custom_edit_url: https://github.com/gokayybaz/bazntms/edit/main/docs/TROUBLESHOOTING.md
---

> Kaynak: [`docs/TROUBLESHOOTING.md`](https://github.com/gokayybaz/bazntms/blob/main/docs/TROUBLESHOOTING.md) — bu sayfa her build'de otomatik senkronize edilir.
# Sorun Giderme

## Yakalama başlamıyor

### "activate en0: Permission Denied (root/admin yetkisi gerekli olabilir)"

Paket yakalama ayrıcalıklı bir işlemdir:

- **macOS**: `sudo ./bazntms ...` ile çalıştırın. Xcode Command Line Tools kurulu olmalı
  (`xcode-select --install`). `/dev/bpf*` aygıtları root dışında kapalıdır.
- **Linux**: `sudo` ile çalıştırın ya da kalıcı yetki verin:
  ```bash
  sudo setcap cap_net_raw+ep ./bazntms
  ```
  Derleme için `libpcap-dev` (Debian/Ubuntu) veya `libpcap-devel` (RHEL) gerekir.
- **Windows**: [Npcap](https://npcap.com) kurulu olmalı ve uygulama **yönetici
  olarak** başlatılmalı — ama yalnızca gerçekten paket yakalıyorsanız: hub
  varsayılan olarak (`-capture=true`) başlangıçta yakalamayı dener, agent ise
  varsayılan olarak yakalamaz (`-pcap=false`, yalnızca süreç bazlı trafik
  atfı için opsiyonel). Npcap kurulu değilse uygulama çökmez, sadece
  yakalama çalışmaz.
  **Derleme için Npcap SDK/mingw-w64 GEREKMEZ** — `gopacket/pcap`, Windows'ta
  cgo kullanmaz; `wpcap.dll`'i yalnızca yakalama fiilen başladığında
  (syscall ile) çalışma zamanında yükler. Düz `go build -o bazntms.exe
  ./cmd/bazntms-hub` yeterlidir.
- **WSL2**: yakalama sanal ağda kalır; gerçek trafik için native Windows kullanın.

### Arayüz listesi boş / seçilen arayüz trafik göstermiyor

- Sadece `up` ve loopback olmayan arayüzler listelenir
- VPN/filtre sürücüleri trafiği başka sanal arayüze taşıyabilir; doğru arayüzü seçin

## Windows MSI / servis kurulumu

### "Service bazNTMS agent failed to start" (hata 1920)

MSI, servisi kurulum anında **başlatmaz** — config doldurulmadan başlayan servis
hata verip kurulumu düşürürdü. MSI'ı çift tıklarsanız sihirbaz Hub adresi/
enroll token/site alanlarını zaten sorar; komut satırından sessiz kurulumda
sıra:

```bat
:: 1. Kurulum — sunucu bilgisi opsiyonel property olarak verilebilir (/qn: sihirbazı atla)
msiexec /i bazntms-agent-amd64.msi /qn HUBURL=https://hub.example.com ENROLLTOKEN=xxx SITE=ofis-a

:: 2. Servisi başlat (config'e girmişseniz property vermenize gerek yok)
sc start bazntms-agent
```

- Property'ler `HKLM\SOFTWARE\bazNTMS\Agent` altına yazılır; agent önceliği
  `flag > registry > config.yml` şeklindedir. Yönetilen/GPO kurulumlarda
  property'lerin geçmesi için kurulumu yükseltilmiş (elevated) komut satırından
  çalıştırın.
- Config'i elle dolduracaksanız: `C:\ProgramData\bazntms\agent.yml` içindeki
  `hub.url` ve `hub.token` değerlerini girin, sonra servisi başlatın.
- Servis başlamıyor ama MSI kurulduysa: servis modunda loglar
  `C:\ProgramData\bazntms\agent.log` dosyasına yazılır (v0.2.2+) — en sık
  neden: hub adresine erişilemiyor (enrollment başarısız) veya token hatalı.
  Eski sürümde servis logları kaybolur; binary'yi interaktif çalıştırıp hatayı
  görün:
  ```bat
  "C:\Program Files\bazNTMS\bazntms-agent.exe" -config "C:\ProgramData\bazntms\agent.yml"
  ```
- Servis kurulumu hata 1053 veriyorsa (zaman aşımı) binary eski bir sürüm
  olabilir; SCM dispatcher desteği v0.2.1 ile geldi — release'ten güncel MSI'ı alın.

## Agent filosu

### Agent'lar sayfasında yanlış/beklenmedik IP adresi görünüyor

Hub, agent'ın IP'sini `X-Forwarded-For` başlığından (varsa) okur, yoksa
doğrudan bağlantının kaynağına düşer. `deploy/docker-compose.scale.yml`
gibi bir nginx LB arkasında (bkz. `deploy/nginx/lb.conf`) çalışırken LB
`X-Forwarded-For` eklemiyorsa ya da hub LB'nin arkasında değil de
doğrudan agent trafiğini alan farklı bir proxy'nin arkasındaysa, gösterilen
IP o ara katmanın (proxy/LB container'ı) kendi IP'si olarak görünür —
gerçek agent IP'si değil. Bu alan yalnızca **gösterim** amaçlıdır; erişim
kontrolü/rate-limit için kullanılmaz (kimlik doğrulama enroll/agent
token'larıyla yapılır).

## AI analizi sorunları

### "AI yapilandirilmamis"

`-llm-base-url` verin (ör. `http://localhost:11434/v1`) veya `LLM_API_KEY`
ayarlayın. Yerel adreslerde (localhost/127.0.0.1) anahtar gerekmez.

### "Bu dönem için veritabanında kayıt yok"

Veriler yalnızca **yakalama açıkken** birikir. Yakalamayı başlatıp birkaç dakika
bekleyin ya da daha uzun dönem (1 saat / 24 saat) seçin.

### "AI boş yanıt döndü" / "düşünme aşamasında token limitini aştı"

Reasoning modeller (Qwen3, DeepSeek-R1) cevaptan önce uzun düşünme üretir:

```bash
-llm-no-think            # düşünmeyi kapat (en hızlı)
-llm-max-tokens 4000     # veya limiti artır
```

`reasoning_content` ve `<think>` blokları otomatik desteklenir.

### "AI servisine ulasilamadi"

- LM Studio/Ollama'nın yerel sunucusu çalışıyor mu? (`curl localhost:11434/v1/models`)
- Ollama'da model çekilmiş mi? (`ollama pull qwen2.5:7b`)
- Anahtarsız uzak servis kullanılıyorsa `Enabled()` değildir; anahtar ekleyin

## GeoIP

- Bayraklar görünmüyorsa: `geoip/` altında `GeoLite2-Country.mmdb` +
  `GeoLite2-ASN.mmdb` var mı? Yoksa ip-api modu çalışır (internet ister,
  `-ip-api-lookup=false` ile kapanır)
- ip-api.com ücretsiz kotası dakikada 45 istektir; kota dolunca 1 dk bekler
- Özel ağ IP'leri (192.168.x.x) bilinçli olarak çözümlenmez

## Bildirimler

- **macOS masaüstü bildirimi çıkmıyor**: Sistem Ayarları → Bildirimler →
  (osascript'i kullanan terminal uygulaması) izinlerini kontrol edin
- **Windows**: `BurntToast` PowerShell modülü kurulu olmalı:
  `Install-Module BurntToast`
- **Linux**: `notify-send` paketi kurulu olmalı
- Webhook hataları sunucu log'una düşer; bildirim hatası izlemeyi durdurmaz

## Performans / veri

- **Veri tabanı çok büyüdü**: `-retention-hours` değerini düşürün
  (ör. 72). Temizlik 10 dakikada bir çalışır.
- **Paket düşüşü (dropped)**: UI'daki "Paket Hızı" kartında görürsünüz; ağır
  trafikte snaplen zaten 65535'tir, düşüş genelde disk/CPU tıkanıklığındandır
- **PCAP dosyaları**: `-record-max-mb` ile dosya boyutunu sınırlayın; kayıt
  yalnızca yakalama açıkken yazılır

## Kimlik doğrulama

- Şifrenizi unuttursanız: sunucuyu şifresiz başlatın (auth kapanır) ve yeni
  şifreyle yeniden başlatın. Şifre hiçbir yere yazılmaz.
- Oturumlar bellekte tutulur: her yeniden başlatmada yeniden giriş gerekir.
- `429 Too Many Requests`: IP başına 5 hatalı deneme sonrası 1 dk bekleme.

## Derleme

- **`go build` cgo hatası (macOS)**: Xcode CLT kurulu mu?
  `xcode-select --install`
- **frontend/dist embed hatası**: önce `cd frontend && npm install && npm run build`
  çalıştırın; `make` iki adımı sırayla yapar
- **Cross-compile**: hedef platformun (cgo kullanan) libpcap'i gerekir —
  Makefile'daki `cross-mac` / `cross-linux` hedefleri yalnızca darwin/linux
  içindir, Windows hedefi yok (CI'da doğrudan `windows-latest` runner'ında
  derlenir, cgo gerekmediği için ek SDK istemez)

## Log örnekleri

```
>> AI aktif: qwen2.5:7b (http://localhost:11434/v1)   # AI hazır
>> AI pasif: -llm-base-url ...                        # AI yapılmamış
>> UYARI: kimlik dogrulama kapali — ...               # -auth-password verin
UYARI [port] Şüpheli porta bağlantı: ...              # uyarı tetiklendi
```
