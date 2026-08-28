# LinkedIn Paylaşım Paketi

## 1) Ana Post (TR — kopyala/yapıştır)

---

🚀 Yeni açık kaynak projesi: **bazNTMS**

"Bu bilgisayar şu an ağa ne yapıyor?" sorusunu cevaplamak için yazdığım araç:
bazNTMS — baz Network Traffic Monitoring System.

Tek bir Go binary'si, sıfır kurulum, tamamen yerel:

▸ Canlı paket yakalama (gopacket/libpcap) — indirme/gönderme/yerel ayrımıyla
▸ Aktif bağlantı listesi — süreç adı ve PID ile
▸ DNS görünürlüğü — hangi domainlere kim sorgu atıyor
▸ GeoIP + ASN — uzak IP'lerin ülkesi ve ağ sahibi
▸ SQLite ile kalıcı kayıt + bugün/dün ve 7 günlük karşılaştırmalı grafikler
▸ Uyarı motoru — şüpheli port, yeni süreç, trafik zirvesi → masaüstü + Telegram/Discord/Slack
▸ Wireshark uyumlu PCAP kaydı, tarayıcıdan indirme
▸ Tek tıkla kapsamlı HTML/PDF rapor

En sevdiğim kısım: **yerel LLM ile trafik analizi**. Ollama veya LM Studio'daki
modelinize bağlanıyor, veriyi parça parça göndererek küçük modellerde bile
analiz üretiyor. Trafiğiniz ve veriniz makinenizden hiç çıkmıyor. 🔒

macOS, Linux ve Windows destekli. Web arayüzü (React + Vite) binary'nin içine
gömülü — indir, çalıştır, tarayıcıdan bak.

GitHub: https://github.com/gokayybaz/bazntms

Geri bildirimlere ve PR'lara açığım. Faydalıysa ⭐ bırakmayı unutmayın!

#opensource #golang #networksecurity #cybersecurity #observability #devops #sre #ollama #ai #yazilim

---

## 2) Kısa EN Varyantı (yorum/farklı kitle için)

---

📦 New open-source release: **bazNTMS** — a multiplatform network traffic
monitor in a single Go binary.

Live packet capture (libpcap), per-process connections, DNS visibility,
GeoIP/ASN, SQLite history with comparison charts, an alert engine with
desktop/Telegram/Discord notifications, Wireshark-compatible PCAP recording,
HTML/PDF reports — and local LLM analysis via Ollama/LM Studio with a
chunked prompting mode designed for small models.

Everything runs locally. Embedded React dashboard. macOS / Linux / Windows.

→ https://github.com/gokayybaz/bazntms

#opensource #golang #networksecurity #ollama

---

## 3) Görseller

| Dosya | Kullanım |
|-------|----------|
| `marketing/banner-1280x640.png` | GitHub: Settings → General → **Social preview** olarak yükle (link paylaşımlarında çıkar). LinkedIn'e ilk görsel olarak da yüklenebilir |
| `marketing/linkedin-1200x627.png` | LinkedIn link önizleme boyutu (1.91:1) |

**Gerçek ekran görüntüleri (en etkili olanlar)** — posttan önce şunları al:

1. **Dashboard canlı görünüm** — yakalama açıkken, grafik dolu ve uç noktalar bayraklı (ana görsel adayı #1)
2. **AI analiz sonucu** — karta yazılmış Türkçe analiz görünürken
3. **Uyarı ayarları + olay akışı** — kurumsal özellik vurgusu
4. **PDF rapor ilk sayfası** — "Rapor Üretimi" çıktısı açıkken
5. **Bağlantı tablosu filtreli** — süreç adları görünüyor

LinkedIn'de çoklu görsel: 1 + 2 + 4 sırası önerilir (banner + canlı ekran + rapor).

## 4) Yayın taktiği

- Post saati: Salı–Perşembe 09:00–11:00 (TR saati) en iyi erişim
- İlk yorumuna `github.com/gokayybaz/bazntms` linkini de ekle (LinkedIn linkli postları kısabiliyor)
- 1–2 gün sonra kendi yorumunda teknik detay (chunked AI mimarisi) anlatan bir ek paylaş
