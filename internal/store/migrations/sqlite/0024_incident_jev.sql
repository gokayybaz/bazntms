-- 0024_incident_jev (SQLite) — Faz 27 S27.7: Jev on-filtre kararini incident'a
-- yazar (arayuzde gorunur). jev_noul=-1 = degerlendirilmedi (Jev kapali/hata
-- → fail-open); gercek bir skor her zaman [0,1] araliginda oldugundan -1
-- guvenli sentinel'dir. jev_worth 0/1 (BIGINT) — proje genelindeki bool-as-int
-- konvansiyonu (bkz. ai_conversations.archived).
ALTER TABLE incidents ADD COLUMN jev_noul  REAL   NOT NULL DEFAULT -1;
ALTER TABLE incidents ADD COLUMN jev_worth BIGINT NOT NULL DEFAULT 0;
