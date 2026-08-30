package store

import (
	"path/filepath"
	"testing"
	"time"
)

// TestIsmsSoaSeed, 93 Annex A kontrolünün seed edildiğini ve karar
// güncellemesinin çalıştığını doğrular (10.2).
func TestIsmsSoaSeed(t *testing.T) {
	st := openTest(t)

	items, err := st.ListIsmsSoa()
	if err != nil {
		t.Fatalf("soa listesi: %v", err)
	}
	if len(items) != 93 {
		t.Fatalf("93 kontrol beklenirdi, gelen: %d", len(items))
	}
	// doğal sıra: A.5.1 < A.5.2 < A.5.10 < A.5.37 < A.6.1 < A.8.34
	if items[0].ControlID != "A.5.1" || items[36].ControlID != "A.5.37" ||
		items[37].ControlID != "A.6.1" || items[92].ControlID != "A.8.34" {
		t.Fatalf("sıralama hatalı: ilk=%s 37.=%s 38.=%s son=%s",
			items[0].ControlID, items[36].ControlID, items[37].ControlID, items[92].ControlID)
	}

	// tekrar açılışta seed çift kayıt eklememeli
	st2, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("ikinci acilis: %v", err)
	}
	defer st2.Close()
	if items, _ := st2.ListIsmsSoa(); len(items) != 93 {
		t.Fatalf("yeniden seed: 93 beklenirdi, gelen %d", len(items))
	}

	// güncelleme: hariç karar gerekçesiz reddedilir (store düzeyi serbest,
	// kural server'dadır); durum + kanıt yazımı
	item := items[0]
	item.Status = "implemented"
	item.Evidence = "test kanıtı"
	item.Owner = "gokay"
	if err := st.UpdateIsmsSoa(item); err != nil {
		t.Fatalf("soa guncelleme: %v", err)
	}
	fresh, _ := st.ListIsmsSoa()
	if fresh[0].Status != "implemented" || fresh[0].Evidence != "test kanıtı" {
		t.Fatalf("guncelleme kalici olmali: %+v", fresh[0])
	}

	total, applicable, implemented, verified, excluded, err := st.IsmsSoaCounts()
	if err != nil {
		t.Fatalf("sayim: %v", err)
	}
	if total != 93 || applicable != 93 || implemented != 1 || verified != 0 || excluded != 0 {
		t.Fatalf("sayim hatali: %d %d %d %d %d", total, applicable, implemented, verified, excluded)
	}
}

// TestIsmsAssetSync, filo senkronunun cihaz/agent/site'ları eklediğini ve
// ikinci çağrıda yeni kayıt açmadığını doğrular (10.1).
func TestIsmsAssetSync(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	if _, err := st.AddDevice(Device{Name: "fw-core", Host: "10.0.0.1", AddedAt: now}); err != nil {
		t.Fatalf("cihaz: %v", err)
	}
	if _, err := st.RegisterAgent(Agent{Name: "agt-1", Site: "merkez", TokenHash: "h1", FirstSeen: now, LastSeen: now}); err != nil {
		t.Fatalf("agent: %v", err)
	}

	added, err := st.SyncIsmsAssetsFromFleet()
	if err != nil {
		t.Fatalf("senkron: %v", err)
	}
	if added != 3 { // cihaz + agent + site
		t.Fatalf("3 varlık beklenirdi, gelen: %d", added)
	}
	added, err = st.SyncIsmsAssetsFromFleet()
	if err != nil || added != 0 {
		t.Fatalf("idempotent olmali: %d %v", added, err)
	}

	assets, _ := st.ListIsmsAssets()
	if len(assets) != 3 {
		t.Fatalf("3 varlık listelenmeli: %d", len(assets))
	}
	if assets[0].Kind != "agent" || !assets[0].Auto {
		t.Fatalf("ilk varlık agent olmali: %+v", assets[0])
	}

	// manuel güncelleme: kritiklik + sahip
	if err := st.UpdateIsmsAsset(IsmsAsset{ID: assets[0].ID, Owner: "netops", Criticality: "kritik"}); err != nil {
		t.Fatalf("guncelleme: %v", err)
	}
	assets, _ = st.ListIsmsAssets()
	if assets[0].Criticality != "kritik" || assets[0].Owner != "netops" {
		t.Fatalf("guncelleme kalici olmali: %+v", assets[0])
	}
	if err := st.DeleteIsmsAsset(assets[0].ID); err != nil {
		t.Fatalf("silme: %v", err)
	}
}

// TestIsmsRiskLedger, risk CRUD + skor hesabını doğrular (10.1).
func TestIsmsRiskLedger(t *testing.T) {
	st := openTest(t)

	id, err := st.AddIsmsRisk(IsmsRisk{
		AssetID: 7, Threat: "dış saldırı", Vulnerability: "yamalı olmayan firewall",
		Impact: 5, Likelihood: 4, Treatment: "mitigate", Plan: "yama + kural gözden geçirme",
		ResImpact: 5, ResLikelihood: 2, Owner: "gokay", Status: "in_progress",
	})
	if err != nil || id == 0 {
		t.Fatalf("risk ekleme: %v %d", err, id)
	}
	risks, _ := st.ListIsmsRisks()
	if len(risks) != 1 || risks[0].Score != 20 || risks[0].ResScore != 10 {
		t.Fatalf("skor hesabi hatali: %+v", risks[0])
	}

	// kalıntı muamele + kapanış
	risks[0].ResImpact, risks[0].ResLikelihood, risks[0].Status = 2, 2, "closed"
	risks[0].ReviewTs = time.Now().Unix()
	if err := st.UpdateIsmsRisk(risks[0]); err != nil {
		t.Fatalf("risk guncelleme: %v", err)
	}
	risks, _ = st.ListIsmsRisks()
	if risks[0].ResScore != 4 || risks[0].Status != "closed" {
		t.Fatalf("kalinti risk guncellenmedi: %+v", risks[0])
	}

	// seviye kilidi: 1-5 dışı değerler kırpılır
	if _, err := st.AddIsmsRisk(IsmsRisk{Threat: "x", Impact: 9, Likelihood: 0}); err != nil {
		t.Fatalf("risk ekleme2: %v", err)
	}
	risks, _ = st.ListIsmsRisks()
	var clamped *IsmsRisk
	for i := range risks {
		if risks[i].Threat == "x" {
			clamped = &risks[i]
		}
	}
	if clamped == nil || clamped.Impact != 5 || clamped.Likelihood != 1 {
		t.Fatalf("seviye kirpilmasi hatali: %+v", risks)
	}

	if err := st.DeleteIsmsRisk(clamped.ID); err != nil {
		t.Fatalf("risk silme: %v", err)
	}
}

// TestIsmsPolicies, politika sürümleme + durum akışını doğrular (10.3).
func TestIsmsPolicies(t *testing.T) {
	st := openTest(t)

	id, err := st.AddIsmsPolicy(IsmsPolicy{Ref: "POL-001", Title: "Erişim Kontrolü Politikası", Owner: "gokay"})
	if err != nil || id == 0 {
		t.Fatalf("politika: %v %d", err, id)
	}
	vid, err := st.AddIsmsPolicyVersion(IsmsPolicyVersion{
		PolicyID: id, Version: "1.0", Content: "amaç, kapsam, kurallar", ChangeNote: "ilk sürüm",
	})
	if err != nil || vid == 0 {
		t.Fatalf("surum: %v %d", err, vid)
	}
	if _, err := st.AddIsmsPolicyVersion(IsmsPolicyVersion{PolicyID: id, Version: "1.1", Content: "v1.1"}); err != nil {
		t.Fatalf("surum2: %v", err)
	}
	versions, _ := st.ListIsmsPolicyVersions(id)
	if len(versions) != 2 || versions[0].Version != "1.1" { // DESC
		t.Fatalf("surum listesi: %+v", versions)
	}

	// onay + yayın
	policies, _ := st.ListIsmsPolicies()
	p := policies[0]
	p.Status, p.ApprovedBy, p.ApprovedAt, p.PublishedAt = "published", "yonetim", 111, 222
	if err := st.UpdateIsmsPolicy(p); err != nil {
		t.Fatalf("politika guncelleme: %v", err)
	}
	policies, _ = st.ListIsmsPolicies()
	if policies[0].Status != "published" || policies[0].ApprovedBy != "yonetim" {
		t.Fatalf("politika durumu kalici olmali: %+v", policies[0])
	}
}

// TestIsmsAuditCAPA, denetim + bulgu + kapanış doğrulamasını doğrular (10.4).
func TestIsmsAuditCAPA(t *testing.T) {
	st := openTest(t)

	aid, err := st.AddIsmsAudit(IsmsAudit{Title: "Q3 iç denetim", Scope: "ağ izleme", PlannedDate: "2026-09-15"})
	if err != nil || aid == 0 {
		t.Fatalf("denetim: %v %d", err, aid)
	}
	fid, err := st.AddIsmsFinding(IsmsFinding{
		AuditID: aid, Description: "erişim incelemesi gecikmiş", Severity: "orta",
		ControlID: "A.9.2.5", CAPA: "inceleme takvimi otomatikleştirilecek", CAPAOwner: "gokay", CAPADue: "2026-10-01",
	})
	if err != nil || fid == 0 {
		t.Fatalf("bulgu: %v %d", err, fid)
	}
	findings, _ := st.ListIsmsFindings(aid)
	if len(findings) != 1 || findings[0].Status != "open" {
		t.Fatalf("bulgu listesi: %+v", findings)
	}

	// CAPA tamamlandı → doğrulandı → kapandı (kapanış zamanı otomatik)
	f := findings[0]
	f.Status, f.VerifiedBy = "closed", "denetci"
	if err := st.UpdateIsmsFinding(f); err != nil {
		t.Fatalf("bulgu guncelleme: %v", err)
	}
	findings, _ = st.ListIsmsFindings(aid)
	if findings[0].Status != "closed" || findings[0].ClosedAt == 0 || findings[0].VerifiedBy != "denetci" {
		t.Fatalf("kapanis dogrulamasi: %+v", findings[0])
	}

	// tüm bulgular (auditID=0)
	all, _ := st.ListIsmsFindings(0)
	if len(all) != 1 {
		t.Fatalf("tum bulgular: %d", len(all))
	}
}

// TestIsmsGovernance, yönetim incelemesi + tedarikçi + süreklilik testlerini
// doğrular (10.5-10.7).
func TestIsmsGovernance(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	if _, err := st.AddIsmsMgmtReview(IsmsMgmtReview{Period: "2026-Q3", Attendees: "yonetim, ciso", Decisions: "bütçe onayı"}); err != nil {
		t.Fatalf("yonetim incelemesi: %v", err)
	}
	reviews, _ := st.ListIsmsMgmtReviews(10)
	if len(reviews) != 1 || reviews[0].Ts == 0 {
		t.Fatalf("inceleme listesi: %+v", reviews)
	}

	sid, err := st.AddIsmsSupplier(IsmsSupplier{
		Name: "BulutTS", Service: "zaman damgası (TSA)", Criticality: "yuksek",
		DataAccess: "log hash'leri", NextReview: now - 100,
	})
	if err != nil || sid == 0 {
		t.Fatalf("tedarikci: %v %d", err, sid)
	}
	suppliers, _ := st.ListIsmsSuppliers()
	if len(suppliers) != 1 || suppliers[0].Name != "BulutTS" {
		t.Fatalf("tedarikci listesi: %+v", suppliers)
	}
	suppliers[0].Risk = "tek sağlayıcı bağımlılığı"
	if err := st.UpdateIsmsSupplier(suppliers[0]); err != nil {
		t.Fatalf("tedarikci guncelleme: %v", err)
	}
	if err := st.DeleteIsmsSupplier(suppliers[0].ID); err != nil {
		t.Fatalf("tedarikci silme: %v", err)
	}
	if suppliers, _ = st.ListIsmsSuppliers(); len(suppliers) != 0 {
		t.Fatalf("silinmeliydi: %d", len(suppliers))
	}

	if _, err := st.AddIsmsContinuityTest(IsmsContinuityTest{
		Kind: "restore", Title: "pg backup geri yükleme tatbikatı", Result: "basarili",
		Evidence: "docs/DR-RUNBOOK.md",
	}); err != nil {
		t.Fatalf("sureklilik testi: %v", err)
	}
	tests, _ := st.ListIsmsContinuityTests(10)
	if len(tests) != 1 || tests[0].Kind != "restore" || tests[0].PerformedAt == 0 {
		t.Fatalf("sureklilik listesi: %+v", tests)
	}
}
