package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"os"

	"github.com/shirou/gopsutil/v3/host"
)

// machineID, agent'in kararli makine kimligini dondurur (C3, Faz 13):
//
//	hex( sha256( hostID + "\x00" + hostname )[:16] )
//
// hostID kaynagi (gopsutil): Linux /sys/.../product_uuid → /etc/machine-id →
// boot_id; macOS IOPlatformUUID; Windows registry MachineGuid. Hostname de
// karisir cunku ayni imajdan klonlanan VM'ler machine-id/product_uuid'i
// paylasabilir. Ikisi de bos ise "" doner → hub her hello'da yeni satir acar.
//
// Karar notu: docs/decisions/0003-agent-machine-id.md
func machineID() string {
	hid, _ := host.HostID()
	hn, _ := os.Hostname()
	if hid == "" && hn == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(hid + "\x00" + hn))
	return hex.EncodeToString(sum[:16])
}
