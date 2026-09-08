-- 0020_enroll_token_hardening (PostgreSQL) — bkz. sqlite/0020_enroll_token_hardening.sql.
-- Faz 25-D: enrollment token sertleştirme. Kolonlar tümüyle yeni → her DB'de
-- tam bir kez. Sızmış bir token revoke/expire olana dek sınırsız agent'ı
-- herhangi bir IP'den kaydedebiliyordu.
--   max_uses      : izinli başarılı enrollment sayısı. 0 = sınırsız (mevcut
--                   token'lar geriye uyumlu); yeni token'larda handler varsayılanı 1.
--   used_count    : şu ana dek yapılan enrollment (ConsumeEnrollToken atomik artırır)
--   allowed_cidrs : virgülle ayrılmış CIDR listesi; boş = her IP
--   created_by    : token'ı üreten yönetici kullanıcı adı
--   revoked_at    : iptal zamanı (unix; 0 = iptal edilmedi)
ALTER TABLE enroll_tokens ADD COLUMN max_uses      BIGINT NOT NULL DEFAULT 0;
ALTER TABLE enroll_tokens ADD COLUMN used_count    BIGINT NOT NULL DEFAULT 0;
ALTER TABLE enroll_tokens ADD COLUMN allowed_cidrs TEXT   NOT NULL DEFAULT '';
ALTER TABLE enroll_tokens ADD COLUMN created_by    TEXT   NOT NULL DEFAULT '';
ALTER TABLE enroll_tokens ADD COLUMN revoked_at    BIGINT NOT NULL DEFAULT 0;
