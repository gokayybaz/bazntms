# Sorun Giderme

## Süreç trafiği / DNS / L7 panelleri boş

Bu üç panel agent'ın **süreç-atıf motorundan** gelir. Yöntem `collect.method`
ile seçilir (`auto` varsayılan) ve `/api/v1/agents` yanıtındaki `attr_method`
alanında + agent detay sayfasındaki rozet'te görünür.

| Platform | `auto` sırası | Gereksinim |
|----------|--------------|-----------|
| Linux    | **eBPF** (+ L7 için yardımcı pcap) → pcap | eBPF: kernel ≥ 5.8 + `/sys/kernel/btf/vmlinux` + `CAP_BPF`/`CAP_SYS_ADMIN` (veya root). pcap + L7 yardımcı: `CAP_NET_RAW` + `libpcap` |
| Windows  | **pcap** (MSI Npcap'i kurar) → ETW | Npcap ([npcap.com](https://npcap.com), MSI otomatik kurar). Npcap yoksa ETW: yükseltilmiş süreç — L7 akmaz |
| macOS    | pcap | `sudo` (`/dev/bpf*` root dışında kapalı) |

Kontrol listesi:

1. **Derin toplama v1.3.0'dan beri varsayılan açık** — `collect.pcap` /
   `-pcap` gerekmez. `agent.log`'da `surec atfi kapali — collect.method=off`
   görüyorsanız `agent.yml`'de `collect.method: off` var demektir; kaldırın.
2. **Hub politikası açık mı?** `-agent-pcap` v1.3.0'dan beri varsayılan açık.
   `agent.log`: `PCAP politikasi hub tarafinda kapali` → hub `-agent-pcap=false`
   ile başlatılmış (veya `hub.yaml`'de `agent_pcap: false`); kaldırın.
3. **`agent.log`'da hangi yöntem seçildi?** `süreç atfı aktif  yöntem=ebpf`
   gibi bir satır olmalı. `yöntem=pcap` ise eBPF/ETW ortamı uygun değildir —
   aynı log satırındaki `not=` alanı nedeni söyler (`kernel 5.4 < 5.8` vb.).
4. **eBPF yüklenmedi:** `eBPF nesnesi yüklenemedi` → kernel < 5.8 ya da BTF yok
   (`CONFIG_DEBUG_INFO_BTF=y` gerekir; RHEL/Debian ≥ 11 var, bazı minimal
   imajlarda yok). Seçici otomatik pcap'e düşer.
5. **Windows ETW başlamadı:** yükseltilmemiş süreç → `ETW atlandı: süreç
   yükseltilmemiş`. Servis olarak kurulduysa SYSTEM'dir; elle çalıştırıyorsanız
   "Yönetici olarak çalıştır".

### Windows'ta Npcap

**v1.3.0'dan beri MSI kurulumu Npcap'i sessizce indirip kurar**
(`deploy/msi/install-npcap.ps1` — `npcap.com`'dan, SHA-256 + Authenticode
imza doğrulamalı) ve agent `collect.method: pcap` ile gelir → **L7 (SNI/Host)
Windows'ta da çalışır.**

Npcap kurulamazsa (internet yok, imza/hash uyuşmadı) MSI yine başarıyla biter;
agent otomatik **ETW**'ye düşer → süreç trafiği + DNS akar, **L7 akmaz**.
`agent.log`: `pcap arka ucu (L7/SNI dahil) icin Npcap gerekir`. Elle kurup
servisi yeniden başlatın (`sc stop bazntms-agent && sc start bazntms-agent`),
ya da geçici olarak `-collect-method=etw`.

Hava boşluklu / internet erişimi olmayan Windows makineleri: Npcap'i önceden
kurun, MSI kurulumu onu tespit edip atlar.

**Derleme için Npcap SDK / mingw-w64 GEREKMEZ** (Windows agent `CGO_ENABLED=0`;
`wpcap.dll` çalışma anında yüklenir, gopacket Npcap dizinini otomatik bulur).

### Arayüz listesi boş / seçilen arayüz trafik göstermiyor

- Sadece `up` ve loopback olmayan arayüzler listelenir
- VPN/filtre sürücüleri trafiği başka sanal arayüze taşıyabilir; doğru arayüzü seçin

### "Error opening adapter" / "dosya adı veya birim etiketi söz dizimi hatalı" (Windows)

`-collect-method=pcap` seçildi, Npcap **kurulu** ama başlamıyor:

```
WARN surec atfi baslatilamadi — telemetri surecek  iface=Ethernet  err="Error opening adapter: ..."
```

libpcap Windows'ta arayüzü `\Device\NPF_{GUID}` biçiminde ister; `Ethernet` /
`Wi-Fi` gibi friendly ad çalışmaz (hata 123, ERROR_INVALID_NAME). Agent v0.2.3+
`collect.pcap_interface` boş/`auto` ise friendly adı otomatik NPF adına çevirir.
Elle vermek isterseniz:

```powershell
Get-NetAdapter | Select-Object Name, InterfaceGuid
```

sonra `C:\ProgramData\bazntms\agent.yml` (tek tırnak — YAML'da `\` literal kalır):

```yaml
collect:
  pcap: true
  pcap_interface: '\Device\NPF_{BULUNAN-GUID}'
```

`sc stop bazntms-agent && sc start bazntms-agent`, ardından `agent.log`'da
`surec atfi aktif` satırını bekleyin. L7 (SNI/Host) için yakalama başladıktan
**sonra** açılan HTTPS bağlantıları gerekir; mevcut bağlantılar sayılmaz.

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

### "hata 1603" + logda `SECUREREPAIR: SecureRepair Failed` / `ProcessComponents. Return value 3`

Aynı sürüm **zaten kuruluyken** `msiexec /i bazntms-agent-amd64.msi` çalıştırmak
kurulum değil, **bakım/onarım** işlemi başlatır. Windows Installer önbelleğindeki
orijinal paketi doğrulamaya çalışır (`SECUREREPAIR`); indirdiğiniz dosyanın adı
önbellektekiyle uyuşmadığı için (`bazntms-agent-amd64_3.msi` gibi sayı ekli
adlar bunun işaretidir) doğrulama başarısız olur → `ProcessComponents` geri
döner → rollback → **1603**. Dosya bozuk değildir, kurulum eksik değildir.

Doğrulama:

```powershell
Get-Package '*bazNTMS*'                                  # zaten kurulu mu?
Get-Service bazntms-agent                                # servis kayıtlı mı?
```

**Yalnızca yeniden yapılandırmak istiyorsanız — tekrar kurmayın:** ürün zaten
kuruludur, sadece kayıt defterini düzenleyip servisi başlatın:

```powershell
$k = 'HKLM:\SOFTWARE\bazNTMS\Agent'
Set-ItemProperty $k hub_url      'https://hub.example.com'
Set-ItemProperty $k enroll_token 'ent_...'
# saha: Set-ItemProperty $k site 'ofis-a'
Restart-Service bazntms-agent
Get-Content C:\ProgramData\bazntms\agent.log -Tail 20
```

**Gerçekten sıfırdan kurmak istiyorsanız — önce kaldırın, sonra kurun** (`/i`
üstüne `/i` değil):

```powershell
$p = Get-Package '*bazNTMS*'
msiexec /x $p.FastPackageReference /qn /l*v "$env:TEMP\baz-x.log"
msiexec /i "$env:TEMP\bazntms-agent.msi" /qn /l*v "$env:TEMP\baz-i.log" `
  HUBURL=https://hub.example.com ENROLLTOKEN=ent_...
```

Sihirbazın verdiği güncel kurulum komutu bu kaldır-sonra-kur sırasını zaten
uygular (S12.4 sonrası) — eski bir komut kopyaladıysanız panelden yenisini alın.

## Agent filosu

### Uzun hub kesintisi sonrası bir agent'ın verisinde boşluk

Hub'a ulaşamayan agent her telemetri batch'ini diskteki offline kuyruğa
(`<state-file>.queue.jsonl`, 0600) yazar ve hub dönünce hepsini sırayla
oynatır. Kuyruk tavanı **100 batch** (varsayılan 30 sn aralıkta ~50 dk); daha
uzun kesintide en eski batch'ler atılır ve agent logu bunu belirtir:

```
WARN offline kuyruk dolu — en eski batch'ler atildi (veri kaybi) atilan=N toplam_atilan=M
```

Bu, o pencerede kalıcı veri kaybıdır (agent tarafında tampon sınırlı). Kesinti
sırasında agent gönderim denemelerini **üstel olarak geri çeker** (aralık ×2,
×4, ×8; en fazla 5 dk) + ±%20 jitter — 5000 agent'ın hub dönünce onu aynı
anda dövmemesi için. Kuyruk (dolmadıysa) tam replay eder, boşluk oluşmaz.

### Agent logu "hub daha eski protokol konusuyor — agent degrade ediyor"

Agent, hub'dan yeni bir sürüm (daha yüksek `protocol_version`). Hub sert
reddetmek yerine (eski davranış: 401) agent'ı kabul eder ve `HubReply`'de
kendi sürümünü bildirir; agent o sürüme göre çalışır (telgraf JSON ileri/geri
uyumlu). Çözüm: hub'ı da güncelleyin — degrade yalnızca geçiş dönemi içindir.

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

### Agent detayında Süreçler / DNS / L7 panelleri boş

Bu üç panel de tek bir agent motorundan (`internal/agent/attr.go` — pcap +
soket→PID atfı) beslenir. Motor yalnızca **agent isteği** (`agent.yml`'de
`collect.pcap: true` ya da `-pcap` bayrağı) **ve hub politikası**
(`bazntms-hub -agent-pcap`) birlikte açıkken başlar.

Kontrol sırası:

1. `agent.log`'da başlangıçtan hemen sonra bir satır arayın:
   - `surec atfi aktif iface=…` → motor çalışıyor, sorun trafik/atıf tarafında.
   - `derin toplama kapali — surec trafigi / DNS / L7 gorunurlugu yok` →
     `agent.yml`'de `collect.pcap` kapalı. `true` yapıp servisi yeniden başlatın
     (`launchctl kickstart -k system/local.bazntms.agent` / `systemctl restart
     bazntms-agent` / `sc stop|start bazntms-agent`).
   - `PCAP politikasi hub tarafinda kapali` → hub'ı `-agent-pcap` ile başlatın.
   - Hiçbiri yoksa ve satır beklediğiniz gibi değilse: agent eski bir binary
     olabilir (v0.3.3 öncesi bu tanı satırını basmaz).
2. `.pkg` / MSI / deb / rpm **yeniden kurulumu** mevcut `agent.yml`'e dokunmaz;
   ama dosya yoksa sihirbaz yeni bir tane üretir. v0.3.3+ paketleri
   `collect.pcap: true` yazar, daha eskiler `false` — yeniden kurulumdan sonra
   panellerin boşaldığını görürseniz önce bu satırı kontrol edin.
3. Docker/Alpine agent'larında yalnızca kendi çıkış trafiği varsa (örn. yalnız
   hub'a konuşan sentetik agent) L7 boş kalabilir: `sanitizeHost` noktasız tek
   etiketli host adlarını (`lb` gibi) eler — gerçek FQDN hedeflerine giden
   trafik gerekir.

## AI analizi sorunları (Faz 26)

### "AI analiz kapalı" / `/ai` sekmesi görünmüyor

- Hub `-ai` bayrağıyla başlatılmalı.
- Sekme yalnız `PermAnalyze` olan rollere (admin / site-admin / netops /
  analyst) görünür — `viewer` göremez.

### "etkin AI sağlayıcısı yok"

Yönetim > AI Sağlayıcı'dan bir model ekleyin (yerel: Ollama/LM Studio; bulut:
OpenAI/Anthropic). Ya da `-llm-base-url` ile bootstrap seed edin.

### "ai.allow_cloud kapalı — yalnızca yerel model adresleri kabul edilir"

Egress kilidi açık (`-ai-allow-cloud=false`). Bulut sağlayıcı kullanmak için
`=true` yapın; yerel model için Ollama/LM Studio adresini kullanın.

### "AI boş yanıt döndü" / "token limitini aştı"

Reasoning modeller (Qwen3, DeepSeek-R1) cevaptan önce uzun düşünme üretir.
Sağlayıcı düzenleme dialog'unda `no_think=1` (Qwen3) ya da `max_tokens`
artırın. `reasoning_content` + `<think>` blokları otomatik temizlenir.

### "AI servisine ulaşılamadı" / "Test Et" başarısız

- Yerel sunucu çalışıyor mu? (`curl localhost:11434/v1/models`)
- Ollama'da model çekilmiş mi? (`ollama pull qwen2.5:7b`)
- Docker'da hub → host Ollama: `-llm-base-url http://host.docker.internal:11434/v1`

### Sohbet akışı "donuyor" (nginx LB arkasında)

`deploy/nginx/lb.conf`'ta `/api/v1/ai/` konumunda `proxy_buffering off` var mı?
Hub `X-Accel-Buffering: no` gönderir ama LB tamponu bunu ezebilir.

### Gecelik analiz / triyaj notu üretilmiyor

- `ai.nightly.enabled` / `ai.triage.enabled` YAML'de açık mı?
- Çoklu controller: yalnız **lider** replika çalıştırır (scheduler C1).
- Triyaj yalnız `min_severity` ve üstü **yeni** incident'lara; saatlik
  `max_per_hour` sınırı var (aşımda log satırı).

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
