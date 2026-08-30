package compliance

// RFC 3161 (TimeStamp Protocol) istemcisi — Faz 9.2.
// Sağlayıcı bağımsızdır: KamuSM, e-Tugra, TurkTrust vb. TSA uçlarıyla çalışır.
// İstek DER (application/timestamp-query), yanıt DER (application/timestamp-reply).
// Yanıttan durum kodu ve ham TimeStampToken çıkarılır; tam CMS doğrulaması
// offline doğrulamada (bazntmsctl verify, openssl ts destekli) yapılır.

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var oidSHA256 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}

// TSAClient, RFC 3161 zaman damgası servisi istemcisi.
type TSAClient struct {
	URL     string
	Timeout time.Duration
	HTTP    *http.Client
}

// NewTSAClient, istemciyi hazırlar.
func NewTSAClient(url string, timeout time.Duration) *TSAClient {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &TSAClient{URL: url, Timeout: timeout, HTTP: &http.Client{Timeout: timeout}}
}

// tsReq DER yapısı (RFC 3161 §2.4.1)
type asn1AlgorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue `asn1:"optional"`
}

type asn1MessageImprint struct {
	HashAlgorithm asn1AlgorithmIdentifier
	HashedMessage []byte
}

type asn1TimeStampReq struct {
	Version        int
	MessageImprint asn1MessageImprint
	Nonce          int64 `asn1:"optional"`
	CertReq        bool  `asn1:"optional"`
}

// asn1TimeStampResp (RFC 3161 §2.4.2): SEQUENCE { status, token OPTIONAL }
type asn1TimeStampResp struct {
	Status asn1PKIStatusInfo
	Token  asn1.RawValue `asn1:"optional"`
}

type asn1PKIStatusInfo struct {
	Status int
	// statusString/failInfo yok sayılır
}

// Timestamp, hash'in (32 bayt, SHA-256) zaman damgasını ister.
// Dönüş: ham TimeStampToken (DER, CMS), TSA süresi (varsa unix), hata.
func (c *TSAClient) Timestamp(ctx context.Context, hash []byte) ([]byte, int64, error) {
	if len(hash) != sha256.Size {
		return nil, 0, fmt.Errorf("tsa: hash SHA-256 boyutunda olmalı")
	}
	var nonce [8]byte
	rand.Read(nonce[:])
	nonceVal := int64(beUint64(nonce))

	req := asn1TimeStampReq{
		Version: 1,
		MessageImprint: asn1MessageImprint{
			HashAlgorithm: asn1AlgorithmIdentifier{Algorithm: oidSHA256},
			HashedMessage: hash,
		},
		Nonce:   nonceVal,
		CertReq: true, // token içinde TSA sertifikası istenir (offline doğrulama için)
	}
	reqDER, err := asn1.Marshal(req)
	if err != nil {
		return nil, 0, fmt.Errorf("tsa istek derleme: %w", err)
	}

	httpCtx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(httpCtx, http.MethodPost, c.URL, bytes.NewReader(reqDER))
	if err != nil {
		return nil, 0, err
	}
	httpReq.Header.Set("Content-Type", "application/timestamp-query")
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("tsa erişim: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("tsa HTTP %d: %s", resp.StatusCode, truncateStr(string(body), 120))
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/timestamp-reply") {
		return nil, 0, fmt.Errorf("tsa beklenmeyen içerik tipi: %s", ct)
	}
	return ParseTimestampReply(body)
}

// ParseTimestampReply, TimeStampResp DER'ini ayrıştırır:
// durum 0 (granted) veya 1 (grantedWithMods) → token döner.
func ParseTimestampReply(der []byte) ([]byte, int64, error) {
	var parsed struct {
		Status struct {
			Status       int
			StatusString asn1.RawValue `asn1:"optional"`
			FailInfo     asn1.RawValue `asn1:"optional"`
		}
		Token asn1.RawValue `asn1:"optional"`
	}
	if _, err := asn1.Unmarshal(der, &parsed); err != nil {
		// bazı TSA'lar token'ı explicit etiketsiz verir: ikinci ham deneme
		var fallback struct {
			Status struct {
				Status int
			}
			Token asn1.RawValue `asn1:"optional"`
		}
		if _, err2 := asn1.Unmarshal(der, &fallback); err2 != nil {
			return nil, 0, fmt.Errorf("tsa yanıt ayrıştırma: %w", err)
		}
		return checkTSStatus(fallback.Status.Status, fallback.Token.FullBytes)
	}
	return checkTSStatus(parsed.Status.Status, parsed.Token.FullBytes)
}

func checkTSStatus(status int, token []byte) ([]byte, int64, error) {
	switch status {
	case 0, 1: // granted, grantedWithMods
		if len(token) == 0 {
			return nil, 0, fmt.Errorf("tsa: token boş")
		}
		return token, 0, nil
	case 2:
		return nil, 0, fmt.Errorf("tsa: reddedildi (rejection)")
	case 3:
		return nil, 0, fmt.Errorf("tsa: bekliyor (waiting)")
	case 5:
		return nil, 0, fmt.Errorf("tsa: CA sağlanamadı")
	default:
		return nil, 0, fmt.Errorf("tsa: durum %d", status)
	}
}

// TokenContainsHash, token DER baytları içinde mesaj özetini arar (hızlı
// sağlık kontrolü; tam CMS doğrulaması openssl ts ile yapılır).
func TokenContainsHash(token, hash []byte) bool {
	return bytes.Contains(token, hash)
}

// TokenHex, token'ı delil paketinde taşınabilir hex'e çevirir.
func TokenHex(token []byte) string { return hex.EncodeToString(token) }

// TokenFromHex, hex'ten geri yükler.
func TokenFromHex(s string) ([]byte, error) { return hex.DecodeString(s) }

func beUint64(b [8]byte) uint64 {
	var v uint64
	for _, x := range b {
		v = v<<8 | uint64(x)
	}
	return v
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
