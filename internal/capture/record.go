package capture

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

const recordSnaplen = 65535

// RecordInfo, kayit durumunun anlik gorunumu.
type RecordInfo struct {
	Recording bool   `json:"recording"`
	File      string `json:"file,omitempty"`
	Packets   uint64 `json:"packets"`
	Bytes     uint64 `json:"bytes"`
	Error     string `json:"error,omitempty"`
}

// SetRecordOptions, kayit dizinini ve dosya basina maksimum boyutu ayarlar.
func (e *Engine) SetRecordOptions(dir string, maxBytesPerFile uint64) {
	e.recMu.Lock()
	e.recDir = dir
	e.recMaxBytes = maxBytesPerFile
	e.recMu.Unlock()
}

// StartRecording, calisan yakalamayi .pcap dosyasina yazmaya baslar.
func (e *Engine) StartRecording() error {
	e.mu.Lock()
	running := e.running
	device := e.device
	linkType := e.linkType
	e.mu.Unlock()

	if !running {
		return fmt.Errorf("kayit için önce yakalama açık olmalı")
	}

	e.recMu.Lock()
	defer e.recMu.Unlock()
	if e.recFile != nil {
		return fmt.Errorf("kayıt zaten açık")
	}
	if err := e.openRecLocked(device, linkType); err != nil {
		return err
	}
	return nil
}

// StopRecording, kaydi kapatir ve oturum ozetini dondurur.
func (e *Engine) StopRecording() (RecordInfo, error) {
	e.recMu.Lock()
	defer e.recMu.Unlock()
	if e.recFile == nil {
		return e.recInfoLocked(), fmt.Errorf("kayıt açık değil")
	}
	info := e.recInfoLocked()
	e.closeRecLocked()
	info.Recording = false
	return info, nil
}

// RecordStatus, anlik kayit durumunu dondurur.
func (e *Engine) RecordStatus() RecordInfo {
	e.recMu.Lock()
	defer e.recMu.Unlock()
	return e.recInfoLocked()
}

// recordPacket, okuma dongusunden gelen ham paketi dosyaya yazar.
func (e *Engine) recordPacket(data []byte, ci gopacket.CaptureInfo) {
	e.recMu.Lock()
	defer e.recMu.Unlock()
	if e.recWriter == nil || e.recFile == nil {
		return
	}
	// dosya boyutu siniri: yeni dosyaya gec (rotasyon)
	if e.recMaxBytes > 0 && e.recBytes >= e.recMaxBytes {
		device := e.recDevice
		linkType := e.linkType
		e.closeRecLocked()
		if err := e.openRecLocked(device, linkType); err != nil {
			e.recErr = err.Error()
			return
		}
	}
	if err := e.recWriter.WritePacket(ci, data); err != nil {
		e.recErr = err.Error()
		e.closeRecLocked()
		return
	}
	e.recPackets++
	e.recBytes += uint64(ci.CaptureLength)
}

// openRecLocked, yeni .pcap dosyasi acar ve basligi yazar (recMu kilitli).
func (e *Engine) openRecLocked(device string, linkType layers.LinkType) error {
	if err := os.MkdirAll(e.recDir, 0o755); err != nil {
		return fmt.Errorf("kayıt dizini: %w", err)
	}
	name := time.Now().Format("2006-01-02_15-04-05") + "_" + sanitizeDevice(device) + ".pcap"
	path := filepath.Join(e.recDir, name)
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("dosya açma: %w", err)
	}
	w := pcapgo.NewWriter(f)
	if err := w.WriteFileHeader(uint32(recordSnaplen), linkType); err != nil {
		f.Close()
		return fmt.Errorf("pcap başlığı: %w", err)
	}
	e.recFile = f
	e.recWriter = w
	e.recName = name
	e.recDevice = device
	e.recBytes = 0
	e.recErr = ""
	return nil
}

func (e *Engine) closeRecLocked() {
	if e.recFile != nil {
		e.recFile.Close()
		e.recFile = nil
	}
	e.recWriter = nil
}

func (e *Engine) recInfoLocked() RecordInfo {
	return RecordInfo{
		Recording: e.recFile != nil,
		File:      e.recName,
		Packets:   e.recPackets,
		Bytes:     e.recBytes,
		Error:     e.recErr,
	}
}

func sanitizeDevice(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "dev"
	}
	return string(out)
}

// RecordDir, kayit dosyalarinin bulundugu dizini dondurur.
func (e *Engine) RecordDir() string {
	e.recMu.Lock()
	defer e.recMu.Unlock()
	return e.recDir
}

// ListRecordings, kayit dizinindeki .pcap dosyalarini listeler.
type RecordFile struct {
	Name    string `json:"name"`
	Bytes   int64  `json:"bytes"`
	ModTime int64  `json:"mod_time"`
}

func (e *Engine) ListRecordings() []RecordFile {
	e.recMu.Lock()
	dir := e.recDir
	e.recMu.Unlock()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return []RecordFile{}
	}
	var out []RecordFile
	for _, en := range entries {
		if en.IsDir() || !strings.HasSuffix(en.Name(), ".pcap") {
			continue
		}
		fi, err := en.Info()
		if err != nil {
			continue
		}
		out = append(out, RecordFile{Name: en.Name(), Bytes: fi.Size(), ModTime: fi.ModTime().Unix()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModTime > out[j].ModTime })
	if out == nil {
		out = []RecordFile{}
	}
	return out
}

// stopRecordingInternal, yakalama dururken kaydi temiz kapatir.
func (e *Engine) stopRecordingInternal() {
	e.recMu.Lock()
	defer e.recMu.Unlock()
	if e.recFile != nil {
		e.closeRecLocked()
	}
}
