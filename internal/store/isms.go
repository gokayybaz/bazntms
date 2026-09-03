package store

// ISMS yönetişim veri katmanı (Faz 10): varlık envanteri, risk defteri,
// Statement of Applicability (93 Annex A kontrolü), politika yönetimi,
// iç denetim + CAPA, yönetim incelemesi, tedarikçi güvenliği ve süreklilik
// testleri. Tüm tablolar normal Prune kapsamı DIŞINDADIR — yönetişim
// kayıtları kanıt niteliğindedir ve kullanıcı kararıyla silinir.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// --- varlık envanteri (10.1) ---

// IsmsAsset, risk defterinin konusunu oluşturan varlık kaydı. Fleet senkronu
// cihaz/agent/site'ları otomatik (auto=1) ekler; insan kaynakları, yazılım,
// bina gibi varlıklar elle tanımlanır.
type IsmsAsset struct {
	ID          int64  `json:"id"`
	Kind        string `json:"kind"` // device | agent | site | service | data | people | other
	Name        string `json:"name"`
	Owner       string `json:"owner"`
	Criticality string `json:"criticality"` // dusuk | orta | yuksek | kritik
	Auto        bool   `json:"auto"`
	Notes       string `json:"notes"`
	CreatedAt   int64  `json:"created_at"`
}

// SyncIsmsAssetsFromFleet, cihaz + agent + site envanterini isms_assets'e
// yansıtır (idempotent: var olanlar korunur, yeni eklenen sayısı döner).
func (s *sqlStore) SyncIsmsAssetsFromFleet() (int, error) {
	now := time.Now().Unix()
	type seed struct {
		kind, name string
	}
	var seeds []seed

	devices, err := s.ListDevices("")
	if err != nil {
		return 0, err
	}
	for _, d := range devices {
		seeds = append(seeds, seed{"device", d.Name})
	}
	agents, err := s.ListAgents(0, "")
	if err != nil {
		return 0, err
	}
	sites := map[string]bool{}
	for _, a := range agents {
		seeds = append(seeds, seed{"agent", a.Name})
		if a.Site != "" {
			sites[a.Site] = true
		}
	}
	for site := range sites {
		seeds = append(seeds, seed{"site", site})
	}

	added := 0
	for _, sd := range seeds {
		if sd.name == "" {
			continue
		}
		res, err := s.db.Exec(s.q(`INSERT INTO isms_assets (kind, name, owner, criticality, auto, notes, created_at)
			VALUES (?,?,?,'orta',1,'',?) ON CONFLICT (kind, name) DO NOTHING`),
			sd.kind, sd.name, "", now)
		if err != nil {
			return added, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			added++
		}
	}
	return added, nil
}

func (s *sqlStore) ListIsmsAssets() ([]IsmsAsset, error) {
	rows, err := s.db.Query(s.q(`SELECT id, kind, name, owner, criticality, auto, notes, created_at
		FROM isms_assets ORDER BY kind, name`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IsmsAsset{}
	for rows.Next() {
		var a IsmsAsset
		var auto int
		if err := rows.Scan(&a.ID, &a.Kind, &a.Name, &a.Owner, &a.Criticality, &auto, &a.Notes, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.Auto = auto == 1
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *sqlStore) UpdateIsmsAsset(a IsmsAsset) error {
	_, err := s.db.Exec(s.q(`UPDATE isms_assets SET owner = ?, criticality = ?, notes = ? WHERE id = ?`),
		a.Owner, a.Criticality, a.Notes, a.ID)
	return err
}

func (s *sqlStore) DeleteIsmsAsset(id int64) error {
	_, err := s.db.Exec(s.q(`DELETE FROM isms_assets WHERE id = ?`), id)
	return err
}

// --- risk defteri (10.1) ---

// IsmsRisk, etki × olasılık skoru taşıyan risk kaydı. Score alanları okuma
// anında hesaplanır (impact × likelihood, 1-25); kalıntı skorlar 0 ise
// henüz muamele planlanmamıştır.
type IsmsRisk struct {
	ID            int64  `json:"id"`
	AssetID       int64  `json:"asset_id"`
	Threat        string `json:"threat"`
	Vulnerability string `json:"vulnerability"`
	Impact        int    `json:"impact"`     // 1-5
	Likelihood    int    `json:"likelihood"` // 1-5
	Score         int    `json:"score"`      // hesaplanan
	Treatment     string `json:"treatment"`  // mitigate | accept | transfer | avoid
	Plan          string `json:"plan"`
	ResImpact     int    `json:"res_impact"`
	ResLikelihood int    `json:"res_likelihood"`
	ResScore      int    `json:"res_score"` // hesaplanan
	Owner         string `json:"owner"`
	Status        string `json:"status"` // open | in_progress | closed
	CreatedAt     int64  `json:"created_at"`
	ReviewTs      int64  `json:"review_ts"`
}

func clampLevel(n int) int {
	if n < 1 {
		return 1
	}
	if n > 5 {
		return 5
	}
	return n
}

func (s *sqlStore) AddIsmsRisk(r IsmsRisk) (int64, error) {
	if r.CreatedAt == 0 {
		r.CreatedAt = time.Now().Unix()
	}
	r.Impact = clampLevel(r.Impact)
	r.Likelihood = clampLevel(r.Likelihood)
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO isms_risks
		(asset_id, threat, vulnerability, impact, likelihood, treatment, plan,
		 res_impact, res_likelihood, owner, status, created_at, review_ts)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?) RETURNING id`),
		r.AssetID, r.Threat, r.Vulnerability, r.Impact, r.Likelihood, r.Treatment, r.Plan,
		clampLevel(r.ResImpact), clampLevel(r.ResLikelihood), r.Owner, r.Status,
		r.CreatedAt, r.ReviewTs).Scan(&id)
	return id, err
}

func (s *sqlStore) scanIsmsRisks(rows interface{ Scan(...any) error }) (IsmsRisk, error) {
	var r IsmsRisk
	err := rows.Scan(&r.ID, &r.AssetID, &r.Threat, &r.Vulnerability, &r.Impact, &r.Likelihood,
		&r.Treatment, &r.Plan, &r.ResImpact, &r.ResLikelihood, &r.Owner, &r.Status,
		&r.CreatedAt, &r.ReviewTs)
	if err == nil {
		r.Score = r.Impact * r.Likelihood
		r.ResScore = r.ResImpact * r.ResLikelihood
	}
	return r, err
}

func (s *sqlStore) ListIsmsRisks() ([]IsmsRisk, error) {
	rows, err := s.db.Query(s.q(`SELECT id, asset_id, threat, vulnerability, impact, likelihood,
		treatment, plan, res_impact, res_likelihood, owner, status, created_at, review_ts
		FROM isms_risks ORDER BY (impact * likelihood) DESC, id DESC`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IsmsRisk{}
	for rows.Next() {
		r, err := s.scanIsmsRisks(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *sqlStore) UpdateIsmsRisk(r IsmsRisk) error {
	_, err := s.db.Exec(s.q(`UPDATE isms_risks SET asset_id = ?, threat = ?, vulnerability = ?,
		impact = ?, likelihood = ?, treatment = ?, plan = ?, res_impact = ?, res_likelihood = ?,
		owner = ?, status = ?, review_ts = ? WHERE id = ?`),
		r.AssetID, r.Threat, r.Vulnerability, clampLevel(r.Impact), clampLevel(r.Likelihood),
		r.Treatment, r.Plan, clampLevel(r.ResImpact), clampLevel(r.ResLikelihood),
		r.Owner, r.Status, r.ReviewTs, r.ID)
	return err
}

func (s *sqlStore) DeleteIsmsRisk(id int64) error {
	_, err := s.db.Exec(s.q(`DELETE FROM isms_risks WHERE id = ?`), id)
	return err
}

// --- Statement of Applicability (10.2) ---

// IsmsSoaItem, tek Annex A kontrolünün uygulama kararı. Seed, ISO 27001:2022
// Annex A'daki 93 kontrolün tamamını 'planned' durumuyla ekler; platformun
// kutudan çıkan özellikleri (Faz 5/9) kanıt notuyla işaretlidir.
type IsmsSoaItem struct {
	ControlID     string `json:"control_id"`
	Category      string `json:"category"` // A.5 | A.6 | A.7 | A.8
	Title         string `json:"title"`
	Applicable    bool   `json:"applicable"`
	Justification string `json:"justification"`
	Status        string `json:"status"` // planned | implemented | verified
	Evidence      string `json:"evidence"`
	Owner         string `json:"owner"`
	UpdatedAt     int64  `json:"updated_at"`
}

// annexAControls, ISO 27001:2022 Annex A'nın 93 kontrolü (kısa Türkçe
// başlıklar). Kanıt notu dolu olanlar bazNTMS platformunun doğrudan
// karşıladığı kontrollerdir.
var annexAControls = []IsmsSoaItem{
	// A.5 — Organizasyonel kontroller (37)
	{ControlID: "A.5.1", Category: "A.5", Title: "Bilgi güvenliği politikaları", Evidence: "Politika yönetimi modülü: sürümleme + onay + yayın akışı"},
	{ControlID: "A.5.2", Category: "A.5", Title: "Bilgi güvenliği rolleri ve sorumlulukları"},
	{ControlID: "A.5.3", Category: "A.5", Title: "Görev ayrılığı", Evidence: "RBAC rolleri (admin/netops/analyst/viewer) + audit zinciri"},
	{ControlID: "A.5.4", Category: "A.5", Title: "Yönetim sorumlulukları"},
	{ControlID: "A.5.5", Category: "A.5", Title: "Otoritelerle temas"},
	{ControlID: "A.5.6", Category: "A.5", Title: "Özel ilgi gruplarıyla temas"},
	{ControlID: "A.5.7", Category: "A.5", Title: "Tehdit istihbaratı"},
	{ControlID: "A.5.8", Category: "A.5", Title: "Proje yönetiminde bilgi güvenliği"},
	{ControlID: "A.5.9", Category: "A.5", Title: "Bilgi ve diğer varlıkların envanteri", Evidence: "Varlık envanteri: cihaz/agent/site otomatik senkron + manuel kayıt"},
	{ControlID: "A.5.10", Category: "A.5", Title: "Kabul edilebilir kullanım"},
	{ControlID: "A.5.11", Category: "A.5", Title: "Varlıkların iadesi"},
	{ControlID: "A.5.12", Category: "A.5", Title: "Bilginin sınıflandırılması"},
	{ControlID: "A.5.13", Category: "A.5", Title: "Bilginin etiketlenmesi"},
	{ControlID: "A.5.14", Category: "A.5", Title: "Bilgi aktarımı", Evidence: "Agent↔hub karşılıklı TLS (-tls; hub CA'sı, enrollment'ta imzalı istemci sertifikası) veya enrollment/Bearer token + JSON-over-HTTPS; delil paketi imzalı dışa aktarım"},
	{ControlID: "A.5.15", Category: "A.5", Title: "Erişim kontrolü", Evidence: "RBAC yetki matrisi + oturum/API-token kimlik doğrulama"},
	{ControlID: "A.5.16", Category: "A.5", Title: "Kimlik yönetimi", Evidence: "Kullanıcı yönetimi + OIDC SSO + API token'ları"},
	{ControlID: "A.5.17", Category: "A.5", Title: "Kimlik doğrulama bilgisi", Evidence: "Hash'lenmiş token'lar; SNMP/REST gizli anahtarları AES-GCM kimlik kasasında"},
	{ControlID: "A.5.18", Category: "A.5", Title: "Erişim hakları", Evidence: "Rol bazlı erişim + periyodik erişim incelemesi tutanakları (A.8.2 ile)"},
	{ControlID: "A.5.19", Category: "A.5", Title: "Tedarikçi ilişkilerinde bilgi güvenliği", Evidence: "Tedarikçi kayıt/gözden geçirme defteri"},
	{ControlID: "A.5.20", Category: "A.5", Title: "Tedarikçi sözleşmelerinde bilgi güvenliği"},
	{ControlID: "A.5.21", Category: "A.5", Title: "ICT tedarik zincirinde bilgi güvenliği", Evidence: "SBOM (syft), govulncheck, trivy, cosign imzalı release"},
	{ControlID: "A.5.22", Category: "A.5", Title: "Tedarikçi servis izleme ve değişiklik yönetimi"},
	{ControlID: "A.5.23", Category: "A.5", Title: "Bulut servislerinde bilgi güvenliği"},
	{ControlID: "A.5.24", Category: "A.5", Title: "Olay yönetimi planlaması", Evidence: "Uyarı motoru + bildirim kanalları (Teams/SMTP/webhook HMAC)"},
	{ControlID: "A.5.25", Category: "A.5", Title: "Bilgi güvenliği olaylarının değerlendirilmesi", Evidence: "Uyarı olayları zincirli kayıt + log inceleme tutanakları"},
	{ControlID: "A.5.26", Category: "A.5", Title: "Bilgi güvenliği olaylarına müdahale"},
	{ControlID: "A.5.27", Category: "A.5", Title: "Olaylardan öğrenme"},
	{ControlID: "A.5.28", Category: "A.5", Title: "Delil toplama", Evidence: "Tarih aralıklı delil paketi: loglar + Merkle zinciri + RFC 3161 + manifest imzası + offline doğrulama"},
	{ControlID: "A.5.29", Category: "A.5", Title: "Kesinti sırasında bilgi güvenliği", Evidence: "DR runbook + agent çoklu-hub failover + offline disk kuyruğu"},
	{ControlID: "A.5.30", Category: "A.5", Title: "İş sürekliliği için ICT hazırlığı", Evidence: "pg backup/restore script'leri + BCDR test kayıtları"},
	{ControlID: "A.5.31", Category: "A.5", Title: "Yasal ve sözleşmesel gereksinimler", Evidence: "5651 uyum profilleri (yerleşim yeri, erişim sağlayıcı, kurum ağı)"},
	{ControlID: "A.5.32", Category: "A.5", Title: "Fikri mülkiyet hakları"},
	{ControlID: "A.5.33", Category: "A.5", Title: "Kayıtların korunması", Evidence: "Günlük imzalı paketler WORM depoda 730 gün saklanır"},
	{ControlID: "A.5.34", Category: "A.5", Title: "Mahremiyet ve KIŞV koruması", Evidence: "Delil paketinde KVKK-duyarlı PII maskeleme"},
	{ControlID: "A.5.35", Category: "A.5", Title: "Bilgi güvenliğinin bağımsız incelemesi", Evidence: "İç denetim programı: plan + bulgu + CAPA + kapanış doğrulaması"},
	{ControlID: "A.5.36", Category: "A.5", Title: "Politikalara ve standartlara uyum", Evidence: "SoA durum takibi + denetçi paketi"},
	{ControlID: "A.5.37", Category: "A.5", Title: "Dokümante işletim prosedürleri", Evidence: "docs/: DR-RUNBOOK, UPGRADE-RUNBOOK, CONFIGURATION, TROUBLESHOOTING"},

	// A.6 — İnsan kaynakları kontrolleri (8)
	{ControlID: "A.6.1", Category: "A.6", Title: "Eleme (screening)"},
	{ControlID: "A.6.2", Category: "A.6", Title: "Çalışma koşulları"},
	{ControlID: "A.6.3", Category: "A.6", Title: "Bilgi güvenliği farkındalığı ve eğitimi"},
	{ControlID: "A.6.4", Category: "A.6", Title: "Disiplin süreci"},
	{ControlID: "A.6.5", Category: "A.6", Title: "İstihdam değişikliği sonrası sorumluluklar"},
	{ControlID: "A.6.6", Category: "A.6", Title: "Gizlilik sözleşmeleri"},
	{ControlID: "A.6.7", Category: "A.6", Title: "Uzaktan çalışma"},
	{ControlID: "A.6.8", Category: "A.6", Title: "Bilgi güvenliği olay bildirimi", Evidence: "Uyarı kanalları + olay kategorili zincirli log"},

	// A.7 — Fiziksel kontroller (14)
	{ControlID: "A.7.1", Category: "A.7", Title: "Fiziksel güvenlik çevreleri"},
	{ControlID: "A.7.2", Category: "A.7", Title: "Fiziksel giriş"},
	{ControlID: "A.7.3", Category: "A.7", Title: "Ofis, oda ve tesis güvenliği"},
	{ControlID: "A.7.4", Category: "A.7", Title: "Fiziksel güvenlik izleme"},
	{ControlID: "A.7.5", Category: "A.7", Title: "Fiziksel ve çevresel tehditlere koruma"},
	{ControlID: "A.7.6", Category: "A.7", Title: "Güvenli alanlarda çalışma"},
	{ControlID: "A.7.7", Category: "A.7", Title: "Temiz masa ve temiz ekran"},
	{ControlID: "A.7.8", Category: "A.7", Title: "Ekipman yerleşimi ve koruması"},
	{ControlID: "A.7.9", Category: "A.7", Title: "Tesis dışı varlık güvenliği"},
	{ControlID: "A.7.10", Category: "A.7", Title: "Depolama ortamları"},
	{ControlID: "A.7.11", Category: "A.7", Title: "Destek hizmetleri (enerji/iklimlendirme)"},
	{ControlID: "A.7.12", Category: "A.7", Title: "Kablo güvenliği"},
	{ControlID: "A.7.13", Category: "A.7", Title: "Ekipman bakımı"},
	{ControlID: "A.7.14", Category: "A.7", Title: "Ekipmanın güvenli imhası veya yeniden kullanımı"},

	// A.8 — Teknolojik kontroller (34)
	{ControlID: "A.8.1", Category: "A.8", Title: "Kullanıcı uç cihazları", Evidence: "Imzalı agent yükleyicileri (MSI/deb/rpm/pkg) + self-update imza doğrulaması"},
	{ControlID: "A.8.2", Category: "A.8", Title: "Ayrıcalıklı erişim hakları", Evidence: "RBAC admin ayrıcalığı + periyodik erişim incelemesi tutanağı"},
	{ControlID: "A.8.3", Category: "A.8", Title: "Bilgi erişimi kısıtlama", Evidence: "Site-scope kimlik + rol matrisi"},
	{ControlID: "A.8.4", Category: "A.8", Title: "Kaynak koduna erişim"},
	{ControlID: "A.8.5", Category: "A.8", Title: "Güvenli kimlik doğrulama", Evidence: "Agent: enrollment token + istemci sertifikası (mTLS, -tls) / Bearer token; panel: bcrypt şifre + IP bazlı deneme sınırı + OIDC SSO; sabit-zamanlı karşılaştırma"},
	{ControlID: "A.8.6", Category: "A.8", Title: "Kapasite yönetimi", Evidence: "Kapasite hedefleri + k6/loadgen yük testleri + bant genişliği uyarıları"},
	{ControlID: "A.8.7", Category: "A.8", Title: "Kötücül yazılıma koruma"},
	{ControlID: "A.8.8", Category: "A.8", Title: "Teknik zaafiyet yönetimi", Evidence: "govulncheck CI + trivy image taraması"},
	{ControlID: "A.8.9", Category: "A.8", Title: "Yapılandırma yönetimi", Evidence: "Doğrulamalı YAML config + env overlay (koanf)"},
	{ControlID: "A.8.10", Category: "A.8", Title: "Bilgi silme", Evidence: "Retention politikaları (Prune + Timescale retention) + compliance saklama"},
	{ControlID: "A.8.11", Category: "A.8", Title: "Veri maskeleme", Evidence: "PII maskeleme (IP/MAC/kullanıcı) — delil paketi ve panel"},
	{ControlID: "A.8.12", Category: "A.8", Title: "Veri sızıntısı önleme"},
	{ControlID: "A.8.13", Category: "A.8", Title: "Bilgi yedekleme", Evidence: "pg_backup/restore script'leri + BCDR test doğrulaması"},
	{ControlID: "A.8.14", Category: "A.8", Title: "Bilgi işleme tesislerinin yedekliliği", Evidence: "Stateless hub çoğaltma + Postgres HA + agent çoklu-hub failover"},
	{ControlID: "A.8.15", Category: "A.8", Title: "Loglama", Evidence: "Hash-zincirli compliance_logs + syslog/agent/cihaz tek hat + imzalı inceleme tutanakları"},
	{ControlID: "A.8.16", Category: "A.8", Title: "İzleme etkinlikleri", Evidence: "Uyarı motoru + z-score anomal tespiti + Prometheus metrikleri"},
	{ControlID: "A.8.17", Category: "A.8", Title: "Saat senkronizasyonu", Evidence: "NTP zaman sapması alarmı; zincir kayıtları unix zaman damgalı"},
	{ControlID: "A.8.18", Category: "A.8", Title: "Ayrıcalıklı yardımcı programların kullanımı"},
	{ControlID: "A.8.19", Category: "A.8", Title: "İşletim sistemlerine yazılım kurulumu"},
	{ControlID: "A.8.20", Category: "A.8", Title: "Ağ güvenliği", Evidence: "SNMPv3/NetFlow/syslog toplama + vault ile şifreli kimlik kasası"},
	{ControlID: "A.8.21", Category: "A.8", Title: "Ağ servislerinin güvenliği"},
	{ControlID: "A.8.22", Category: "A.8", Title: "Ağların ayrımı", Evidence: "Site/site-group organizasyonu + agent site kapsamı"},
	{ControlID: "A.8.23", Category: "A.8", Title: "Web filtreleme"},
	{ControlID: "A.8.24", Category: "A.8", Title: "Kriptografi kullanımı", Evidence: "AES-256-GCM kimlik kasası, ed25519 imza (manifest/güncelleme), RFC 3161 TSA, ECDSA P-256 mTLS PKI (hub CA); anahtar dosyaları 0600, vault.key yönetimi"},
	{ControlID: "A.8.25", Category: "A.8", Title: "Güvenli geliştirme yaşam döngüsü", Evidence: "CI test matrisi + govulncheck + imzalı release"},
	{ControlID: "A.8.26", Category: "A.8", Title: "Uygulama güvenlik gereksinimleri"},
	{ControlID: "A.8.27", Category: "A.8", Title: "Güvenli sistem mimarisi prensipleri"},
	{ControlID: "A.8.28", Category: "A.8", Title: "Güvenli kodlama"},
	{ControlID: "A.8.29", Category: "A.8", Title: "Geliştirme ve kabulde güvenlik testi", Evidence: "testcontainers + 3-platform CI test matrisi"},
	{ControlID: "A.8.30", Category: "A.8", Title: "Dış kaynakla geliştirme"},
	{ControlID: "A.8.31", Category: "A.8", Title: "Geliştirme/test/üretim ortam ayrımı", Evidence: "SQLite dev modu vs PostgreSQL/TimescaleDB üretim profili"},
	{ControlID: "A.8.32", Category: "A.8", Title: "Değişiklik yönetimi", Evidence: "SemVer + protokol versiyon negosyasyonu + upgrade runbook"},
	{ControlID: "A.8.33", Category: "A.8", Title: "Test bilgisi"},
	{ControlID: "A.8.34", Category: "A.8", Title: "Denetim testlerinde bilgi sistemlerinin korunması"},
}

// seedIsmsSoa, SoA tablosunu ilk açılışta 93 kontrolle doldurur
// (idempotent — mevcut kararlar korunur).
func (s *sqlStore) seedIsmsSoa() error {
	for _, c := range annexAControls {
		if _, err := s.db.Exec(s.q(`INSERT INTO isms_soa
			(control_id, category, title, applicable, justification, status, evidence, owner, updated_at)
			VALUES (?,?,?,1,'','planned',?,'',0) ON CONFLICT (control_id) DO NOTHING`),
			c.ControlID, c.Category, c.Title, c.Evidence); err != nil {
			return fmt.Errorf("%s: %w", c.ControlID, err)
		}
	}
	return nil
}

// soaSortKey, "A.5.10" kontrolünü doğal sayı sırasına çevirir (lexicographic
// yerine 5.1 < 5.2 < ... < 5.37).
func soaSortKey(id string) [3]int {
	parts := strings.Split(strings.TrimPrefix(id, "A."), ".")
	var nums [3]int
	for i, p := range parts {
		if i > 2 {
			break
		}
		nums[i], _ = strconv.Atoi(p)
	}
	return nums
}

func (s *sqlStore) ListIsmsSoa() ([]IsmsSoaItem, error) {
	rows, err := s.db.Query(s.q(`SELECT control_id, category, title, applicable, justification,
		status, evidence, owner, updated_at FROM isms_soa`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IsmsSoaItem{}
	for rows.Next() {
		var c IsmsSoaItem
		var app int
		if err := rows.Scan(&c.ControlID, &c.Category, &c.Title, &app, &c.Justification,
			&c.Status, &c.Evidence, &c.Owner, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.Applicable = app == 1
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := soaSortKey(out[i].ControlID), soaSortKey(out[j].ControlID)
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		if a[1] != b[1] {
			return a[1] < b[1]
		}
		return a[2] < b[2]
	})
	return out, nil
}

func (s *sqlStore) UpdateIsmsSoa(c IsmsSoaItem) error {
	if c.UpdatedAt == 0 {
		c.UpdatedAt = time.Now().Unix()
	}
	_, err := s.db.Exec(s.q(`UPDATE isms_soa SET applicable = ?, justification = ?, status = ?,
		evidence = ?, owner = ?, updated_at = ? WHERE control_id = ?`),
		btoi(c.Applicable), c.Justification, c.Status, c.Evidence, c.Owner, c.UpdatedAt, c.ControlID)
	return err
}

// IsmsSoaCounts, SoA olgunluk sayıları (panel + denetçi paketi).
func (s *sqlStore) IsmsSoaCounts() (total, applicable, implemented, verified, excluded int, err error) {
	row := s.db.QueryRow(s.q(`SELECT COUNT(*),
		COALESCE(SUM(applicable),0),
		COALESCE(SUM(CASE WHEN applicable = 1 AND status IN ('implemented','verified') THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN applicable = 1 AND status = 'verified' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN applicable = 0 THEN 1 ELSE 0 END),0)
		FROM isms_soa`))
	err = row.Scan(&total, &applicable, &implemented, &verified, &excluded)
	return
}

// --- politika yönetimi (10.3) ---

// IsmsPolicy, ISMS dokümanı (politika/talimat/prosedür). Durum makinesi:
// draft → in_review → approved → published → archived.
type IsmsPolicy struct {
	ID          int64  `json:"id"`
	Ref         string `json:"ref"` // POL-001
	Title       string `json:"title"`
	Owner       string `json:"owner"`
	Status      string `json:"status"`
	Version     string `json:"version"`
	ApprovedBy  string `json:"approved_by"`
	ApprovedAt  int64  `json:"approved_at"`
	PublishedAt int64  `json:"published_at"`
	NextReview  int64  `json:"next_review"`
	CreatedAt   int64  `json:"created_at"`
}

func (s *sqlStore) AddIsmsPolicy(p IsmsPolicy) (int64, error) {
	if p.CreatedAt == 0 {
		p.CreatedAt = time.Now().Unix()
	}
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO isms_policies
		(ref, title, owner, status, version, approved_by, approved_at, published_at, next_review, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?) RETURNING id`),
		p.Ref, p.Title, p.Owner, p.Status, p.Version, p.ApprovedBy, p.ApprovedAt,
		p.PublishedAt, p.NextReview, p.CreatedAt).Scan(&id)
	return id, err
}

func (s *sqlStore) ListIsmsPolicies() ([]IsmsPolicy, error) {
	rows, err := s.db.Query(s.q(`SELECT id, ref, title, owner, status, version, approved_by,
		approved_at, published_at, next_review, created_at FROM isms_policies ORDER BY ref`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IsmsPolicy{}
	for rows.Next() {
		var p IsmsPolicy
		if err := rows.Scan(&p.ID, &p.Ref, &p.Title, &p.Owner, &p.Status, &p.Version, &p.ApprovedBy,
			&p.ApprovedAt, &p.PublishedAt, &p.NextReview, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *sqlStore) UpdateIsmsPolicy(p IsmsPolicy) error {
	_, err := s.db.Exec(s.q(`UPDATE isms_policies SET title = ?, owner = ?, status = ?, version = ?,
		approved_by = ?, approved_at = ?, published_at = ?, next_review = ? WHERE id = ?`),
		p.Title, p.Owner, p.Status, p.Version, p.ApprovedBy, p.ApprovedAt, p.PublishedAt, p.NextReview, p.ID)
	return err
}

// IsmsPolicyVersion, dokümanın tek sürümü (içerik markdown/düz metin).
type IsmsPolicyVersion struct {
	ID         int64  `json:"id"`
	PolicyID   int64  `json:"policy_id"`
	Version    string `json:"version"`
	Content    string `json:"content"`
	ChangeNote string `json:"change_note"`
	CreatedBy  string `json:"created_by"`
	CreatedAt  int64  `json:"created_at"`
}

func (s *sqlStore) AddIsmsPolicyVersion(v IsmsPolicyVersion) (int64, error) {
	if v.CreatedAt == 0 {
		v.CreatedAt = time.Now().Unix()
	}
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO isms_policy_versions
		(policy_id, version, content, change_note, created_by, created_at)
		VALUES (?,?,?,?,?,?) RETURNING id`),
		v.PolicyID, v.Version, v.Content, v.ChangeNote, v.CreatedBy, v.CreatedAt).Scan(&id)
	return id, err
}

func (s *sqlStore) ListIsmsPolicyVersions(policyID int64) ([]IsmsPolicyVersion, error) {
	rows, err := s.db.Query(s.q(`SELECT id, policy_id, version, content, change_note, created_by, created_at
		FROM isms_policy_versions WHERE policy_id = ? ORDER BY id DESC`), policyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IsmsPolicyVersion{}
	for rows.Next() {
		var v IsmsPolicyVersion
		if err := rows.Scan(&v.ID, &v.PolicyID, &v.Version, &v.Content, &v.ChangeNote, &v.CreatedBy, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// --- iç denetim programı (10.4) ---

// IsmsAudit, planlanmış/gerçekleşmiş iç denetim.
type IsmsAudit struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Scope       string `json:"scope"`
	PlannedDate string `json:"planned_date"` // YYYY-MM-DD (plan düzeyi)
	PerformedAt int64  `json:"performed_at"`
	Auditor     string `json:"auditor"`
	Status      string `json:"status"` // planned | done | closed
	Summary     string `json:"summary"`
	CreatedAt   int64  `json:"created_at"`
}

func (s *sqlStore) AddIsmsAudit(a IsmsAudit) (int64, error) {
	if a.CreatedAt == 0 {
		a.CreatedAt = time.Now().Unix()
	}
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO isms_audits
		(title, scope, planned_date, performed_at, auditor, status, summary, created_at)
		VALUES (?,?,?,?,?,?,?,?) RETURNING id`),
		a.Title, a.Scope, a.PlannedDate, a.PerformedAt, a.Auditor, a.Status, a.Summary, a.CreatedAt).Scan(&id)
	return id, err
}

func (s *sqlStore) ListIsmsAudits() ([]IsmsAudit, error) {
	rows, err := s.db.Query(s.q(`SELECT id, title, scope, planned_date, performed_at, auditor, status, summary, created_at
		FROM isms_audits ORDER BY id DESC`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IsmsAudit{}
	for rows.Next() {
		var a IsmsAudit
		if err := rows.Scan(&a.ID, &a.Title, &a.Scope, &a.PlannedDate, &a.PerformedAt, &a.Auditor,
			&a.Status, &a.Summary, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *sqlStore) UpdateIsmsAudit(a IsmsAudit) error {
	_, err := s.db.Exec(s.q(`UPDATE isms_audits SET title = ?, scope = ?, planned_date = ?,
		performed_at = ?, auditor = ?, status = ?, summary = ? WHERE id = ?`),
		a.Title, a.Scope, a.PlannedDate, a.PerformedAt, a.Auditor, a.Status, a.Summary, a.ID)
	return err
}

// IsmsFinding, denetim bulgusu + CAPA takibi. Durum makinesi:
// open → in_progress → verified (doğrulayan onayı) → closed.
type IsmsFinding struct {
	ID          int64  `json:"id"`
	AuditID     int64  `json:"audit_id"`
	Ref         string `json:"ref"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // dusuk | orta | yuksek
	ControlID   string `json:"control_id"`
	CAPA        string `json:"capa"`
	CAPAOwner   string `json:"capa_owner"`
	CAPADue     string `json:"capa_due"`
	Status      string `json:"status"`
	ClosedAt    int64  `json:"closed_at"`
	VerifiedBy  string `json:"verified_by"`
	CreatedAt   int64  `json:"created_at"`
}

func (s *sqlStore) AddIsmsFinding(f IsmsFinding) (int64, error) {
	if f.CreatedAt == 0 {
		f.CreatedAt = time.Now().Unix()
	}
	if f.Status == "" {
		f.Status = "open"
	}
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO isms_findings
		(audit_id, ref, description, severity, control_id, capa, capa_owner, capa_due, status, closed_at, verified_by, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?) RETURNING id`),
		f.AuditID, f.Ref, f.Description, f.Severity, f.ControlID, f.CAPA, f.CAPAOwner,
		f.CAPADue, f.Status, f.ClosedAt, f.VerifiedBy, f.CreatedAt).Scan(&id)
	return id, err
}

func (s *sqlStore) ListIsmsFindings(auditID int64) ([]IsmsFinding, error) {
	q := `SELECT id, audit_id, ref, description, severity, control_id, capa, capa_owner,
		capa_due, status, closed_at, verified_by, created_at FROM isms_findings`
	args := []any{}
	if auditID > 0 {
		q += ` WHERE audit_id = ?`
		args = append(args, auditID)
	}
	q += ` ORDER BY id DESC`
	rows, err := s.db.Query(s.q(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IsmsFinding{}
	for rows.Next() {
		var f IsmsFinding
		if err := rows.Scan(&f.ID, &f.AuditID, &f.Ref, &f.Description, &f.Severity, &f.ControlID,
			&f.CAPA, &f.CAPAOwner, &f.CAPADue, &f.Status, &f.ClosedAt, &f.VerifiedBy, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *sqlStore) UpdateIsmsFinding(f IsmsFinding) error {
	if f.Status == "closed" && f.ClosedAt == 0 {
		f.ClosedAt = time.Now().Unix()
	}
	_, err := s.db.Exec(s.q(`UPDATE isms_findings SET ref = ?, description = ?, severity = ?,
		control_id = ?, capa = ?, capa_owner = ?, capa_due = ?, status = ?, closed_at = ?, verified_by = ?
		WHERE id = ?`),
		f.Ref, f.Description, f.Severity, f.ControlID, f.CAPA, f.CAPAOwner, f.CAPADue,
		f.Status, f.ClosedAt, f.VerifiedBy, f.ID)
	return err
}

// --- yönetim incelemesi, tedarikçi, süreklilik (10.5-10.7) ---

// IsmsMgmtReview, yönetim incelemesi toplantı kaydı (girdiler, kararlar,
// aksiyonlar — ISO 27001 m.9.3).
type IsmsMgmtReview struct {
	ID        int64  `json:"id"`
	Ts        int64  `json:"ts"`
	Period    string `json:"period"`
	Attendees string `json:"attendees"`
	Inputs    string `json:"inputs"`
	Decisions string `json:"decisions"`
	Actions   string `json:"actions"`
	CreatedBy string `json:"created_by"`
}

func (s *sqlStore) AddIsmsMgmtReview(r IsmsMgmtReview) (int64, error) {
	if r.Ts == 0 {
		r.Ts = time.Now().Unix()
	}
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO isms_mgmt_reviews
		(ts, period, attendees, inputs, decisions, actions, created_by)
		VALUES (?,?,?,?,?,?,?) RETURNING id`),
		r.Ts, r.Period, r.Attendees, r.Inputs, r.Decisions, r.Actions, r.CreatedBy).Scan(&id)
	return id, err
}

func (s *sqlStore) ListIsmsMgmtReviews(limit int) ([]IsmsMgmtReview, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := s.db.Query(s.q(`SELECT id, ts, period, attendees, inputs, decisions, actions, created_by
		FROM isms_mgmt_reviews ORDER BY id DESC LIMIT ?`), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IsmsMgmtReview{}
	for rows.Next() {
		var r IsmsMgmtReview
		if err := rows.Scan(&r.ID, &r.Ts, &r.Period, &r.Attendees, &r.Inputs, &r.Decisions,
			&r.Actions, &r.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// IsmsSupplier, tedarikçi güvenlik kaydı (A.5.19-22).
type IsmsSupplier struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Service     string `json:"service"`
	Criticality string `json:"criticality"`
	DataAccess  string `json:"data_access"`
	ContractRef string `json:"contract_ref"`
	Risk        string `json:"risk"`
	LastReview  int64  `json:"last_review"`
	NextReview  int64  `json:"next_review"`
	Notes       string `json:"notes"`
	CreatedAt   int64  `json:"created_at"`
}

func (s *sqlStore) AddIsmsSupplier(sp IsmsSupplier) (int64, error) {
	if sp.CreatedAt == 0 {
		sp.CreatedAt = time.Now().Unix()
	}
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO isms_suppliers
		(name, service, criticality, data_access, contract_ref, risk, last_review, next_review, notes, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?) RETURNING id`),
		sp.Name, sp.Service, sp.Criticality, sp.DataAccess, sp.ContractRef, sp.Risk,
		sp.LastReview, sp.NextReview, sp.Notes, sp.CreatedAt).Scan(&id)
	return id, err
}

func (s *sqlStore) ListIsmsSuppliers() ([]IsmsSupplier, error) {
	rows, err := s.db.Query(s.q(`SELECT id, name, service, criticality, data_access, contract_ref,
		risk, last_review, next_review, notes, created_at FROM isms_suppliers ORDER BY name`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IsmsSupplier{}
	for rows.Next() {
		var sp IsmsSupplier
		if err := rows.Scan(&sp.ID, &sp.Name, &sp.Service, &sp.Criticality, &sp.DataAccess,
			&sp.ContractRef, &sp.Risk, &sp.LastReview, &sp.NextReview, &sp.Notes, &sp.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

func (s *sqlStore) UpdateIsmsSupplier(sp IsmsSupplier) error {
	_, err := s.db.Exec(s.q(`UPDATE isms_suppliers SET name = ?, service = ?, criticality = ?,
		data_access = ?, contract_ref = ?, risk = ?, last_review = ?, next_review = ?, notes = ?
		WHERE id = ?`),
		sp.Name, sp.Service, sp.Criticality, sp.DataAccess, sp.ContractRef, sp.Risk,
		sp.LastReview, sp.NextReview, sp.Notes, sp.ID)
	return err
}

func (s *sqlStore) DeleteIsmsSupplier(id int64) error {
	_, err := s.db.Exec(s.q(`DELETE FROM isms_suppliers WHERE id = ?`), id)
	return err
}

// IsmsContinuityTest, BCDR testi kaydı; Faz 5 DR kanıtlarına (runbook,
// backup scriptleri) referans taşır (A.5.29/A.5.30/A.8.13).
type IsmsContinuityTest struct {
	ID          int64  `json:"id"`
	Kind        string `json:"kind"` // restore | failover | backup_check | tabletop
	Title       string `json:"title"`
	PerformedAt int64  `json:"performed_at"`
	Result      string `json:"result"` // basarili | kismen | basarisiz
	Evidence    string `json:"evidence"`
	Notes       string `json:"notes"`
	CreatedBy   string `json:"created_by"`
}

func (s *sqlStore) AddIsmsContinuityTest(t IsmsContinuityTest) (int64, error) {
	if t.PerformedAt == 0 {
		t.PerformedAt = time.Now().Unix()
	}
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO isms_continuity_tests
		(kind, title, performed_at, result, evidence, notes, created_by)
		VALUES (?,?,?,?,?,?,?) RETURNING id`),
		t.Kind, t.Title, t.PerformedAt, t.Result, t.Evidence, t.Notes, t.CreatedBy).Scan(&id)
	return id, err
}

func (s *sqlStore) ListIsmsContinuityTests(limit int) ([]IsmsContinuityTest, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := s.db.Query(s.q(`SELECT id, kind, title, performed_at, result, evidence, notes, created_by
		FROM isms_continuity_tests ORDER BY performed_at DESC LIMIT ?`), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IsmsContinuityTest{}
	for rows.Next() {
		var t IsmsContinuityTest
		if err := rows.Scan(&t.ID, &t.Kind, &t.Title, &t.PerformedAt, &t.Result, &t.Evidence,
			&t.Notes, &t.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
