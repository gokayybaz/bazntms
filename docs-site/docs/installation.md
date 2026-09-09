---
sidebar_position: 1
---

# Kurulum

## Hızlı başlangıç (sihirbaz)

```bash
make                          # frontend + hub + agent + ctl tek komutla
./bazntmsctl setup            # interaktif: port, şifre, depo, NATS → bazntms-hub.yml
./bazntms-hub -config bazntms-hub.yml
```

`bazntmsctl` ayrıca GitHub Release sayfasından platformunuza uygun
ön-derli binary olarak indirilebilir (`bazntmsctl-<os>-<arch>`).

## Docker Compose (tek-node demo)

```bash
docker compose -f deploy/docker-compose.yml up --build
# → http://localhost:8080 (şifre: demo123)
```

## Ölçek mimarisi (Kubernetes olmadan)

`deploy/docker-compose.scale.yml`, Helm topolojisini compose ile aynalar:
**N × ingest replikası** (NATS JetStream durable tüketiciyi paylaşarak yazar)
+ **1 × kontrolcü** (uyarı motoru, SNMP poller, NetFlow/Syslog alıcıları) +
**nginx LB** (k8s Service karşılığı) + TimescaleDB + JetStream.

```bash
docker compose -f deploy/docker-compose.scale.yml up --build
# → dashboard: http://localhost:8080 (kontrolcü, şifre: demo123)
# → agent API: http://localhost:8081 (lb → ingest havuzu)

# yatay büyütme (durable kuyruk paylaşımı sayesinde anında ölçeklenir):
docker compose -f deploy/docker-compose.scale.yml up -d --scale hub-ingest=4

# sentetik filo yükü:
docker compose -f deploy/docker-compose.scale.yml --profile loadgen up -d loadgen
```

## Kubernetes

Yayınlanmış chart (OCI, ghcr.io) — `--version` sürüm etiketidir (`v` yok):

```bash
helm install bazntms oci://ghcr.io/gokayybaz/charts/bazntms --version 1.3.0 \
  --set config.database.path="postgres://..." \
  --set config.nats.url="nats://..." \
  --set auth.existingSecret=bazntms-auth
```

Hub imajı `ghcr.io/gokayybaz/bazntms-hub`, agent DaemonSet (`--set
agent.enabled=true`) `ghcr.io/gokayybaz/bazntms-agent` — ikisi de amd64 + arm64,
Chart `appVersion`'a sabitlenir. Depodan kurmak için chart yolunu
`deploy/helm/bazntms` verin.

## Windows (MSI)

Release sayfasındaki MSI'ı **çift tıklayınca gerçek bir kurulum sihirbazı**
açılır: karşılama ekranından sonra Hub adresi / kayıt (enroll) belirteci /
site alanlarını dolduran bir "Sunucu Ayarları" ekranı gelir — terminal
gerekmez.

MSI kurulum sırasında [Npcap](https://npcap.com)'i **sessizce indirip
kurar** (`npcap.com`, SHA-256 + Authenticode doğrulamalı) ve agent
`collect.method: pcap` ile gelir → süreç trafiği + DNS + L7/SNI Windows'ta
da çalışır. Npcap indirilemezse kurulum yine başarılı biter; agent ETW'ye
düşer (L7 hariç her şey akar).

Yönetilen/GPO/sessiz dağıtım için komut satırından da aynı property'ler
verilebilir (sihirbaz o zaman atlanır):

```bat
:: sunucu bilgisiyle sessiz kurulum
msiexec /i bazntms-agent-amd64.msi /qn HUBURL=https://hub.example.com ENROLLTOKEN=xxx SITE=ofis-a

:: servisi başlat (Start=auto olduğundan yeniden başlatmada kendiliğinden açılır)
sc start bazntms-agent
```

- Property'ler `HKLM\SOFTWARE\bazNTMS\Agent` altına yazılır; agent önceliği
  `flag > registry > config.yml` şeklindedir.
- Sihirbazdaki alanları (veya komut satırı property'lerini) boş bırakırsanız
  `C:\ProgramData\bazntms\agent.yml` kurulumla gelir; `hub.url` + `hub.token`
  doldurup servisi başlatabilirsiniz.
- MSI, servisi kurulum anında başlatmaz — hub bilgisi girilmeden başlayan
  servis hata verip kurulumu düşürür (hata 1920) diye tasarım gereği.

## AI analiz (opt-in)

AI analiz varsayılan **kapalıdır**. Hub'ı `-ai` ile başlatıp
`/yonetim/ai` ekranından bir sağlayıcı (OpenAI-uyumlu servis — Ollama,
LM Studio, llama.cpp, OpenAI… — veya Anthropic) tanımlayın; token vault'ta
şifreli saklanır. Egress kilidi açıksa hub yalnızca tanımlı sağlayıcı
adreslerine bağlanır.

Ayrıntı: [Yapılandırma referansı](/docs/reference/configuration).
