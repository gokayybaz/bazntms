# 0001 — Agent otomatik güncelleme kimlik doğrulaması (B2 / Faz 11 S11.5)

**Tarih:** 2026-09-05 · **Durum:** kabul edildi

## Sorun

`update.NewClient(hubURL, channel, publicKey)` yalnızca URL alıyor; ürettiği
`*http.Client` düz — ne `Authorization` başlığı ne de TLS yapılandırması var.
Güncelleme uçları (`GET /api/v1/agent/update/manifest`, `.../file/{channel}/{name}`)
`agentAuth` arkasında. Sonuç: gerçek bir hub'a karşı her güncelleme kontrolü
**401**. Yalnızca auth'suz sahte hub kullanan `update_test.go` yeşil kaldığı
için fark edilmemişti.

`agentAuth` iki yol kabul eder:
1. mTLS: CA'ya karşı doğrulanmış istemci sertifikası (CN = `bazntms-agent-<id>`)
2. `Authorization: Bearer <agent_token>`

## Karar

**Seçenek A:** İmzayı genişlet —
`update.NewClient(hubURL, channel, publicKey, token string, httpClient *http.Client)`.

- Agent, enrollment sonrası elindeki `*http.Client`'ı (pinli CA + varsa istemci
  sertifikası taşıyan transport) ve `state.Token`'ı enjekte eder.
- `update.Client` her istekte `Authorization: Bearer <token>` ekler ve verilen
  client'ı kullanır. `httpClient == nil` ise 10 dk timeout'lu düz client'a düşer
  (geriye uyum / testler).
- mTLS hub: transport zaten sertifika taşıdığı için `agentAuth` cert yoluyla
  kabul eder; Bearer de gider, zararsız. Düz HTTP / pinli-CA HTTPS hub: Bearer
  ile kimliklenilir. Tek kod yolu tüm dağıtım modlarını kapsar.
- İndirme timeout'u: agent transport'unun kısa (15 sn) timeout'u büyük binary
  indirmesini keser → `update.Client` verilen transport'u **uzun timeout'lu**
  yeni bir `http.Client`'a sarar (`agent.Client.UpdateHTTPClient()`).

## Reddedilen

**Seçenek B:** Orkestrasyon `internal/agent`'a taşınır (`agent.Client.CheckUpdate()`),
`internal/update` saf kütüphaneye iner (parse + verify + install). Mimari olarak
daha temiz (güncelleme "başka bir kimlikli hub çağrısı" olur, failover'ı da
paylaşır) ama `internal/update`'in indir→doğrula→kur akışını bölmek bu hatanın
gerektirdiğinden büyük bir refactor. İleride değerlendirilebilir.
