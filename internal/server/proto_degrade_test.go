package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// TestHelloProtocolGracefulDegrade, agent hub'dan yeni bir protokol sürümü
// bildirdiğinde hub'ın sert 401 yerine kabul edip eskisini dayattığını
// doğrular (S21.15).
func TestHelloProtocolGracefulDegrade(t *testing.T) {
	ts := newTestServerWithEnroll(t)

	body, _ := json.Marshal(telemetry.AgentHello{
		Name: "future-agent", Site: "test", ProtocolVersion: 99,
	})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/agent/hello", bytes.NewReader(body))
	req.Header.Set("X-Enroll-Token", testEnrollToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("istek: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("yeni protokol agent'ı kabul edilmeliydi (nazik degrade), gelen: %d", resp.StatusCode)
	}
	var reply telemetry.HubReply
	if err := json.NewDecoder(resp.Body).Decode(&reply); err != nil {
		t.Fatalf("yanıt çözülemedi: %v", err)
	}
	if !reply.Accepted || reply.AgentToken == "" {
		t.Fatalf("agent kaydedilmeliydi: %+v", reply)
	}
	if reply.ProtocolVersion != maxProtocolVersion {
		t.Fatalf("hub protokol sürümü %d bildirmeliydi, gelen: %d", maxProtocolVersion, reply.ProtocolVersion)
	}
}
