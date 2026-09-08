package store

import (
	"testing"
	"time"
)

func TestAIProviderCRUD(t *testing.T) {
	st := openTest(t)

	id, err := st.CreateAIProvider(AIProvider{
		Name: "yerel-ollama", Kind: "ollama", BaseURL: "http://localhost:11434/v1",
		DefaultModel: "qwen2.5:7b", Enabled: true, CreatedBy: "admin",
	})
	if err != nil {
		t.Fatalf("CreateAIProvider: %v", err)
	}

	// anahtarlı bulut sağlayıcı
	if _, err := st.CreateAIProvider(AIProvider{
		Name: "openai", Kind: "openai", BaseURL: "https://api.openai.com/v1",
		APIKeyEnc: "v1:sifreli", Enabled: false,
	}); err != nil {
		t.Fatalf("ikinci sağlayıcı: %v", err)
	}

	list, err := st.ListAIProviders()
	if err != nil || len(list) != 2 {
		t.Fatalf("ListAIProviders = %d (%v)", len(list), err)
	}
	if list[0].OptsJSON != "{}" {
		t.Errorf("opts_json varsayılanı boş obje olmalı: %q", list[0].OptsJSON)
	}

	// UpdateAIProvider: anahtar boş → değişmez
	p, _ := st.AIProviderByID(id)
	p.DefaultModel = "llama3.2"
	p.APIKeyEnc = ""
	if err := st.UpdateAIProvider(*p); err != nil {
		t.Fatalf("UpdateAIProvider: %v", err)
	}
	got, _ := st.AIProviderByID(id)
	if got.DefaultModel != "llama3.2" {
		t.Errorf("model güncellenmedi: %q", got.DefaultModel)
	}

	// anahtarı olan sağlayıcıda boş update anahtarı korumalı
	all, _ := st.ListAIProviders()
	var openai AIProvider
	for _, x := range all {
		if x.Name == "openai" {
			openai = x
		}
	}
	openai.Enabled = true
	openai.APIKeyEnc = ""
	if err := st.UpdateAIProvider(openai); err != nil {
		t.Fatalf("openai update: %v", err)
	}
	after, _ := st.AIProviderByID(openai.ID)
	if after.APIKeyEnc != "v1:sifreli" {
		t.Errorf("boş anahtar update'i mevcut anahtarı silmemeli: %q", after.APIKeyEnc)
	}

	if err := st.DeleteAIProvider(id); err != nil {
		t.Fatalf("DeleteAIProvider: %v", err)
	}
	if list, _ := st.ListAIProviders(); len(list) != 1 {
		t.Fatalf("silme sonrası %d sağlayıcı", len(list))
	}
}

func TestAIConversationRoundTrip(t *testing.T) {
	st := openTest(t)

	cid, err := st.CreateAIConversation(AIConversation{
		CreatedBy: "analyst", ScopeKind: "incident", ScopeRef: "42", Source: "user",
	})
	if err != nil {
		t.Fatalf("CreateAIConversation: %v", err)
	}

	if _, err := st.AppendAIMessage(AIMessage{ConversationID: cid, Role: "system", Content: SystemStub()}); err != nil {
		t.Fatalf("system mesajı: %v", err)
	}
	if _, err := st.AppendAIMessage(AIMessage{ConversationID: cid, Role: "user", Content: "bu olayı açıkla", ContextJSON: `{"incident":42}`}); err != nil {
		t.Fatalf("user mesajı: %v", err)
	}
	// streaming asistan mesajı: önce boş, sonra finalize
	amID, err := st.AppendAIMessage(AIMessage{ConversationID: cid, Role: "assistant"})
	if err != nil {
		t.Fatalf("asistan mesajı: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := st.FinalizeAIMessage(amID, "triyaj: gürültü", "", 120, 45); err != nil {
		t.Fatalf("FinalizeAIMessage: %v", err)
	}

	conv, msgs, err := st.AIConversationByID(cid)
	if err != nil {
		t.Fatalf("AIConversationByID: %v", err)
	}
	if conv.ScopeKind != "incident" || conv.ScopeRef != "42" {
		t.Errorf("kapsam kaybı: %+v", conv)
	}
	if len(msgs) != 3 || msgs[2].Role != "assistant" || msgs[2].Content != "triyaj: gürültü" || msgs[2].TokensOut != 45 {
		t.Fatalf("mesajlar: %+v", msgs)
	}

	// meta yaz
	if err := st.SetAIConversationMeta(cid, "Olay 42 triyajı", "qwen2.5:7b", 3); err != nil {
		t.Fatalf("SetAIConversationMeta: %v", err)
	}

	// aktif listede görünür, arşiv listesinde görünmez
	act, _ := st.ListAIConversations(AIConversationFilter{ScopeKind: "incident", ScopeRef: "42"})
	if len(act) != 1 || act[0].Title != "Olay 42 triyajı" {
		t.Fatalf("aktif liste: %+v", act)
	}
	if arch, _ := st.ListAIConversations(AIConversationFilter{Archived: true}); len(arch) != 0 {
		t.Fatalf("arşiv listesi boş olmalı: %+v", arch)
	}

	// arşivle → aktiften düşer
	if err := st.SetAIConversationArchived(cid, true); err != nil {
		t.Fatalf("SetAIConversationArchived: %v", err)
	}
	if act, _ := st.ListAIConversations(AIConversationFilter{}); len(act) != 0 {
		t.Fatalf("arşivden sonra aktif liste boş olmalı: %+v", act)
	}

	// prune: gelecekteki bir eşik → arşivlenmiş konuşma + mesajları silinir
	ids, err := st.PruneAIConversations(time.Now().Add(time.Hour).Unix())
	if err != nil {
		t.Fatalf("PruneAIConversations: %v", err)
	}
	if len(ids) != 1 || ids[0] != cid {
		t.Fatalf("prune edilen id'ler: %v", ids)
	}
	if c, _, _ := st.AIConversationByID(cid); c != nil {
		t.Fatalf("konuşma silinmeliydi")
	}
}

func TestAIConversationSiteScope(t *testing.T) {
	st := openTest(t)
	_, _ = st.CreateAIConversation(AIConversation{ScopeKind: "fleet", Site: "istanbul", CreatedBy: "a"})
	_, _ = st.CreateAIConversation(AIConversation{ScopeKind: "fleet", Site: "ankara", CreatedBy: "b"})
	_, _ = st.CreateAIConversation(AIConversation{ScopeKind: "fleet", Site: "", CreatedBy: "global"})

	// site-admin (istanbul): kendi sahası + saha-üstü, ankara görünmez
	ist, _ := st.ListAIConversations(AIConversationFilter{Site: "istanbul"})
	if len(ist) != 2 {
		t.Fatalf("istanbul kapsamı 2 konuşma görmeli (kendi + global): %d", len(ist))
	}
	// global admin: hepsi
	all, _ := st.ListAIConversations(AIConversationFilter{})
	if len(all) != 3 {
		t.Fatalf("global admin 3 görmeli: %d", len(all))
	}
}

// SystemStub, test için kısa bir sistem promptu (internal/ai import etmeden).
func SystemStub() string { return "sen bir analizcisin" }
