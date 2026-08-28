# Yapılandırma Referansı

## Komut Satırı Bayrakları

| Bayrak | Varsayılan | Açıklama |
|--------|-----------|----------|
| `-port` | `8080` | HTTP sunucu portu (0.0.0.0'a bağlanır) |
| `-dev` | `false` | Frontend embed'ini atla; Vite dev server ile geliştirme |
| `-db` | `bazntms.db` | SQLite veri tabanı dosyası (WAL modunda açılır) |
| `-retention-hours` | `168` (7 gün) | Veri saklama süresi; eski kayıtlar 10 dakikada bir silinir |
| `-auth-password` | — | Arayüz şifresi. Boşsa kimlik doğrulama kapalı. `AUTH_PASSWORD` ortam değişkeni de geçerli |
| `-llm-base-url` | — | OpenAI-uyumlu AI servisi adresi. Örn: `http://localhost:11434/v1` (Ollama), `http://localhost:1234/v1` (LM Studio) |
| `-llm-api-key` | — | AI API anahtarı. Yerel modeller için gerekmez |
| `-llm-model` | — | Varsayılan model. UI'dan da seçilebilir |
| `-llm-max-tokens` | `0` | İstek başına token limiti (0 = dahili varsayılanlar: parça 1500, final 2500, tek seferde 3000) |
| `-llm-no-think` | `false` | Qwen3 serisi modellerde düşünme modunu kapatır (sistem mesajına `/no_think` ekler) |
| `-record-dir` | `captures` | PCAP kayıt dosyalarının yazılacağı dizin |
| `-record-max-mb` | `100` | PCAP dosya başına üst boyut; aşıldığında otomatik yeni dosyaya geçer (rotasyon) |
| `-geoip-dir` | `geoip` | MaxMind GeoLite2 `.mmdb` dosyalarının aranacağı dizin |
| `-ip-api-lookup` | `true` | MMDB yoksa ip-api.com batch servisiyle IP çözümleme (internet kullanır) |

## Ortam Değişkenleri

| Değişken | Karşılığı | Not |
|----------|----------|-----|
| `AUTH_PASSWORD` | `-auth-password` | |
| `LLM_BASE_URL` / `OPENAI_BASE_URL` | `-llm-base-url` | |
| `LLM_API_KEY` / `OPENAI_API_KEY` | `-llm-api-key` | |
| `LLM_MODEL` | `-llm-model` | Varsayılan: `gpt-4o-mini` |
| `LLM_MAX_TOKENS` | `-llm-max-tokens` | |
| `LLM_NO_THINK` | `-llm-no-think` | `1` veya `true` |

Bayraklar ortam değişkenlerinden önceliklidir.

## AI Kurulumu

### Ollama (yerel)

```bash
ollama pull qwen2.5:7b
sudo ./bazntms -llm-base-url http://localhost:11434/v1
```

Yerel adres görüldüğünde API anahtarı zorunluluğu otomatik kalkar. Kurulu
modeller `/api/ai/models` üzerinden arayüze listelenir.

### LM Studio

Uygulamada "Local Server" sekmesinden sunucuyu başlatın:

```bash
sudo ./bazntms -llm-base-url http://localhost:1234/v1
```

### llama.cpp server / vLLM / OpenRouter / OpenAI

```bash
# llama.cpp
sudo ./bazntms -llm-base-url http://localhost:8080/v1 -llm-model model-adi

# Bulut servisler
LLM_API_KEY=sk-... ./bazntms
```

### Reasoning modelleri (Qwen3, DeepSeek-R1 vb.)

Bu modeller final cevaptan önce uzun düşünme metni üretir; token limiti
düşünmede biterse boş yanıt döner. Sunucu `reasoning_content` alanını ve
`<think>...</think>` bloklarını otomatik destekler. Ek ayarlar:

```bash
-llm-no-think            # Qwen3: düşünmeyi kapat (çok daha hızlı)
-llm-max-tokens 4000     # düşünmeye alan bırak
```

### Parça parça gönderme (chunked)

Analiz verisi 4 bölüme ayrılır: (1) trafik özeti + protokoller, (2) en yoğun
hedefler, (3) en aktif süreçler, (4) DNS sorguları. `chunked: true` iken her
bölüm ayrı istekle gider ve modelden yalnızca kısa not alınır; son istekte
yalnızca notlar birleştirilerek final analiz üretilir. Ham veri modele hiçbir
zaman ikinci kez gönderilmez — küçük modellerde (3B–7B) context şişmez.

## GeoIP Kurulumu

### 1) MaxMind GeoLite2 (offline, önerilen)

1. [maxmind.com](https://www.maxmind.com/en/geolite2/signup) üzerinden ücretsiz hesap açın
2. GeoLite2 **Country** ve GeoLite2 **ASN** veri tabanlarını indirin
3. `.mmdb` dosyalarını `geoip/` dizinine koyun:

```
geoip/
├── GeoLite2-Country.mmdb
└── GeoLite2-ASN.mmdb
```

Sunucu başlarken otomatik algılanır (`-geoip-dir` ile değiştirilebilir).
Kullanım tamamen offline'dur.

### 2) ip-api.com (anahtarsız fallback)

MMDB dosyaları yoksa devreye girer:

- Bilinmeyen genel IP'ler 3 saniyede bir ≤100'lük gruplar hâlinde sorgulanır
- Sonuçlar bellek önbelleğine yazılır (100.000 girdiye kadar)
- `429` yanıtında 1 dakika geri çekilme
- **Gizlilik**: uzak IP'leri üçüncü taraf servise gönderir; `-ip-api-lookup=false` ile kapatın

Özel, loopback ve link-local adresler (192.168.x.x, 127.x, fe80:: vb.) hiçbir
zaman çözümlenmez.

## Veri Saklama

| Tablo | Yazım periyodu | İçerik |
|-------|---------------|--------|
| `samples` | saniye | bps in/out/local, pps, düşen paket, protokol dağılımı (JSON) |
| `endpoint_stats` | dakika | uç nokta bazlı transfer farkları (delta) |
| `connection_events` | dakika | aktif bağlantılar (süreç adı + PID) |
| `dns_queries` | dakika | domain bazlı sorgu/yanıt sayaçları |
| `alert_events` | olay anında | uyarı geçmişi |
| `alert_seen` | kalıcı | yeni süreç/hedef kurallarının "görüldü" işaretleri |
| `alert_config` | PUT ile | uyarı ayarları (JSON, tek satır) |
| `insights` | analizle | AI analiz sonuçları |

Saklama süresi: `-retention-hours` (varsayılan 168 saat = 7 gün). DB dosyası
`-db` ile taşınabilir; boyut kontrolü için `ls -la <db>*` (WAL dahil).
