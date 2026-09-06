# 0006 — Vault master anahtar kaynağı (B8 / Faz 16 S16.2–S16.3)

**Tarih:** 2026-09-06 · **Durum:** kabul edildi (kısmi — env sağlayıcı; age/KMS + zarf deferred)

## Sorun

`vault.Open` master anahtarı `-vault-key-file`'dan (32 bayt hex, `0600`) okur.
AES-256-GCM her sırrı doğrudan bu anahtarla şifreler. Anahtar veritabanının
yanında diskte duruyor: bir DB yedeği + dosya sistemi erişimi = tüm SNMP
community'leri, v3 parolaları, FortiGate API token'ları. Barındırılan senaryoda
harici KMS beklenir (B8).

## S16.2 — KMS soyutlaması kapsamı

| Seçenek | Değerlendirme |
|---|---|
| **Bulut KMS SDK'ları** (AWS KMS, GCP KMS) | Her biri büyük bir SDK bağımlılığı (AWS SDK v2 KMS istemcisi tek başına onlarca modül). Tek-binary / minimal-dep mimarisine aykırı. **Reddedildi.** |
| **age** (`filippo.io/age`) | Küçük, saf Go, iyi denetlenmiş. Plugin ekosistemi (age-plugin-yubikey/tpm/kms) KMS/donanım anahtarlarını *bizim kod yazmadan* kapsıyor. İyi bir sonraki adım ama şimdilik bir bağımlılık daha. **Ertelendi.** |
| **SOPS** | Bir araç/format; kütüphane olarak ağır, age/KMS/PGP sarmalıyor. Fazlalık. **Reddedildi.** |
| **HashiCorp Vault Transit** | HTTP API, bir istemci bağımlılığı, çalışan bir Vault gerektirir. Kurumsal ortamlarda mantıklı ama şimdilik dar bir kitle. **Ertelendi.** |
| **Ortam değişkeni** (`env`) | Sıfır bağımlılık. Master anahtar diske hiç yazılmaz. Anahtarı **herhangi bir** secret manager sağlayabilir: AWS Secrets Manager / GCP Secret Manager / k8s Secret / HashiCorp Vault agent → hepsi ortam değişkeni enjekte edebilir. **Seçildi (S16.3).** |

**Karar:** `vault.KeyProvider` arayüzü + iki sağlayıcı: `file` (varsayılan,
geriye uyum) ve `env` (`BAZNTMS_VAULT_MASTER_KEY`, hex veya base64, 32 bayt).
`-vault-key-source=file|env` (config `vault_key_source`). `age` ve
bulut-KMS sağlayıcıları arayüz sabit kaldığı için ileride eklenir.

## S16.3 — kapsam

**Bu fazda:** `KeyProvider` arayüzü + `file` + `env`. `vault.OpenWith(provider)`.
Doğrudan AES-256-GCM (master = şifreleme anahtarı). `env` modunda master disk'e
yazılmaz — pratik KMS hikayesi.

**Ertelenen — zarf şifrelemesi (veri anahtarı DB'de):** master rotasyonunu
re-encrypt'siz yapmak için gerekli. Bugün master rotasyonu tüm `v1:`
şifreli metinlerin yeniden şifrelenmesini ister (gelecekte
`bazntmsctl vault rotate`). Arayüz zarf eklemeye engel değil; `Vault` içinde
bir "veri anahtarı" katmanı + `vault_meta` tablosu ile eklenir.

## Sonuç

```
-vault-key-source=file   → -vault-key-file (bugünkü davranış, tek-node/dev)
-vault-key-source=env    → BAZNTMS_VAULT_MASTER_KEY (k8s Secret / bulut secret
                            manager / Vault agent enjeksiyonu — master diskte yok)
```
