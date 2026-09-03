package server

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gokayybaz/bazntms/internal/agent"
	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/pki"
	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// mtlsTestServer, agent CA'si + VerifyClientCertIfGiven ile TLS servisi kurar.
func mtlsTestServer(t *testing.T) (*httptest.Server, store.Store, *pki.CA) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	ca, err := pki.LoadOrCreateCA(t.TempDir())
	if err != nil {
		t.Fatalf("ca: %v", err)
	}
	srv := New(nil, capture.NewEngine(), st, "m.db",
		alert.NewManager(alert.DefaultConfig(), st, capture.NewEngine(), 30),
		nil, "", testEnrollToken, 30, false, nil, nil, nil)
	srv.SetAgentCA(ca)

	serverCert, err := ca.ServerTLSCertificate([]string{"127.0.0.1", "localhost", "::1"})
	if err != nil {
		t.Fatalf("server cert: %v", err)
	}
	ts := httptest.NewUnstartedServer(srv.Handler())
	ts.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.VerifyClientCertIfGiven,
		ClientCAs:    ca.Pool(),
	}
	ts.StartTLS()
	t.Cleanup(ts.Close)
	return ts, st, ca
}

// TestMTLSEnrollAndAuth, agent'in https hub'a enroll olup istemci sertifikasi
// aldigini, sonraki telemetri isteklerinin SADECE sertifikayla (Bearer'sız da
// gecerli olsa da) kabul edildigini, agent silinince kimligin reddedildigini
// dogrular.
func TestMTLSEnrollAndAuth(t *testing.T) {
	ts, st, _ := mtlsTestServer(t)
	state := filepath.Join(t.TempDir(), "agent.state.json")

	c := agent.New(agent.Options{
		HubURLs:     []string{ts.URL},
		EnrollToken: testEnrollToken,
		Name:        "mtls-agent",
		StateFile:   state,
	})
	agState, err := c.Enroll()
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	if agState.AgentID == 0 {
		t.Fatal("agent_id yok")
	}
	for _, ext := range []string{".crt", ".key", ".ca"} {
		if b, err := os.ReadFile(state + ext); err != nil || len(b) == 0 {
			t.Fatalf("mTLS dosyası %s eksik/boş: %v", ext, err)
		}
	}

	// telemetri: istemci sertifikası üzerinden (agent.Send c.http'yi kullanır,
	// reloadTLS ile mTLS transport'u kurulmuştur)
	if err := c.Send(agState, telemetry.TelemetryBatch{TS: 1}); err != nil {
		t.Fatalf("mTLS telemetri: %v", err)
	}
	ag, err := st.AgentByID(agState.AgentID)
	if err != nil || ag.LastSeen == 0 {
		t.Fatalf("agent last_seen güncellenmedi: %v", err)
	}

	// agent silinince sertifika tabanlı kimlik reddedilmeli
	if err := st.DeleteAgent(agState.AgentID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := c.Send(agState, telemetry.TelemetryBatch{TS: 2}); err == nil {
		t.Fatal("silinmiş agent'ın sertifikası hâlâ kabul edildi")
	}
}

// TestMTLSBrowserWithoutCert, istemci sertifikası SUNMAYAN bir istemcinin
// (tarayıcı) korumasız uçlara erişebildiğini doğrular.
func TestMTLSBrowserWithoutCert(t *testing.T) {
	ts, _, ca := mtlsTestServer(t)
	cl := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: ca.Pool()}}}
	resp, err := cl.Get(ts.URL + "/api/auth/status")
	if err != nil {
		t.Fatalf("sertifikasız GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("auth/status 200 dönmeliydi, %d", resp.StatusCode)
	}
}
