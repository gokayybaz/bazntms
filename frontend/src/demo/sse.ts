// Demo katmanı — AI mesaj akışı (SSE taklidi). lib/ai.ts `res.body.getReader()`
// ile `data: {...}\n\n` parçalarını okur; burada canned bir analiz metnini
// token token akıtan bir ReadableStream Response'u döndürürüz.

import { world, now } from './world'

const CANNED: Record<string, string> = {
  fleet_health: `## Filo Sağlık Özeti (demo)

**Genel durum: iyi.** 140 agent'ın ~118'i çevrimiçi, sağlık skoru 88/100.

### Öne çıkanlar
- **merkez uplink doygunluğu** — core-sw-merkez-01 Gi1/0/1 %90+ kullanım, yedekleme penceresiyle çakışıyor. Yedek işi 21:00'e kaydırıldı.
- **izmir sürüm sürüklenmesi** — 8 agent v1.1.0'da kalmış; otomatik güncelleme Npcap çakışması nedeniyle başarısız. Manuel MSI planlandı.
- **dc-1 DNS hacmi** — \`rclone\` sürecinden kaynaklı, beklenenin 5σ üstünde. Sahibi doğrulanıyor.

### Önerilen aksiyonlar
1. izmir agent'larına elle MSI dağıtımı (bugün).
2. rclone iş sahibini doğrula; meşruysa anomali baseline'ına ekle.
3. merkez uplink için 40G yükseltme kapasite planına alınmalı.

*Bu bir demo yanıtıdır — sentetik veriye dayanır.*`,
  agent_review: `## Agent İncelemesi (demo)

Bu agent son 24 saatte **stabil**: kesintisiz telemetri, arayüz hataları eşiğin altında.

- **En yoğun hedefler:** \`api.github.com\`, \`s3.eu-central-1.amazonaws.com\` (yedekleme), \`teams.microsoft.com\`.
- **Süreç trafiği:** \`backup-agent\` gece penceresinde baskın; gündüz \`chrome\`/\`teams\`.
- **Atıf yöntemi:** eBPF (CO-RE) — süreç→soket eşlemesi tam.

Anormal bir davranış yok. *(demo yanıtı)*`,
  incident_triage: `## Olay Triyajı (demo)

**Değerlendirme:** korelasyon motoru bu olayı 3 sinyalin 4 dakika içinde çakışmasıyla açtı — IOC eşleşmesi + yeni hedef + gece-dışı trafik.

**Risk skoru 86/100** — yüksek. Deterministik kurallar yetkilidir; bu not yalnızca yorum.

### Sonraki adımlar
1. Agent'ı ağdan **izole et** (uplink switch portunu kapat).
2. Süreç ağacını + \`connSamples\`'ı topla.
3. Hedef alan adını threat-intel beslemesinde doğrula.
4. Doğrulanırsa: kimlik bilgisi rotasyonu + forensik imaj.

*(demo yanıtı — gerçek bir olay değil)*`,
  anomaly_review: `## Sapma Yorumu (demo)

Üç aktif sapma var; ikisi ilişkili görünüyor:

- **fleet · dns_qps (z=4.8)** ve **fleet · bps (z=3.7)** aynı pencerede — muhtemelen tek bir toplu iş (yedek/senkron).
- **local · proc_bps · rclone (z=6.1)** bu hipotezi güçlendiriyor.

**Öneri:** rclone işini meşrulaştır ve mevsimsel baseline'a dahil et; aksi halde her gece aynı saatte tekrarlayacak. *(demo yanıtı)*`,
  device_review: `## Cihaz İncelemesi (demo)

FortiGate-100F sağlıklı: CPU ~%25, bellek ~%62, oturum ~320k (tavan 500k).

- **VPN:** 3 tünel up, \`dc1-transit\` **down** — ISP kaynaklı, izleniyor.
- **SD-WAN:** \`wan2\` health-check'lerinde aralıklı kayıp (%3–6) — failover eşiğinin altında.
- **Politika:** \`LAN→Internet\` baskın; \`Block-Tor\` son 3 saatte 0 hit.

*(demo yanıtı)*`,
}

function analysisFor(preset: string | undefined, content: string | undefined, scope: string): string {
  if (preset && CANNED[preset]) return CANNED[preset]
  if (content) {
    return `**Demo yanıtı.** Bu ortamda gerçek bir dil modeli çalışmıyor — sorunuz (\`${content.slice(0, 120)}\`) sentetik filo verisiyle yanıtlanamaz.\n\nGerçek kurulumda bu sohbet, seçtiğiniz sağlayıcıya (yerel Ollama / OpenAI / Anthropic) bağlanır ve **${scope}** kapsamının canlı verisini bağlam olarak alır.`
  }
  return CANNED.fleet_health
}

export function sseResponse(convID: number, body: { content?: string; preset?: string }): Response {
  const conv = world.conversations.find((c) => c.id === convID)
  const scope = conv?.scope_kind ?? 'fleet'
  const text = analysisFor(body.preset, body.content, scope)
  const tokens = text.match(/\S+\s*/g) ?? [text]
  let i = 0
  const enc = new TextEncoder()

  const stream = new ReadableStream({
    pull(controller) {
      if (i < tokens.length) {
        // birkaç token'ı birden yolla — akış hızlı ama görünür kalsın
        const chunk = tokens.slice(i, i + 3).join('')
        i += 3
        controller.enqueue(enc.encode(`data: ${JSON.stringify({ delta: chunk })}\n\n`))
        return new Promise((r) => setTimeout(r, 28))
      }
      // mesajları kalıcı hale getir
      const msgs = world.messages[convID] ?? (world.messages[convID] = [])
      const tIn = 400 + Math.floor(Math.random() * 900)
      const tOut = tokens.length
      msgs.push({ id: world.seq.msg++, role: 'user', content: body.content || `[${body.preset}]`, tokens_in: 0, tokens_out: 0, created_ts: now() - 1 })
      msgs.push({ id: world.seq.msg++, role: 'assistant', content: text, tokens_in: tIn, tokens_out: tOut, created_ts: now() })
      if (conv) {
        conv.updated_ts = now()
        if (!conv.title || conv.title === 'Yeni sohbet') conv.title = (body.preset ? body.preset.replace(/_/g, ' ') : (body.content || '').slice(0, 40)) || 'Sohbet'
      }
      controller.enqueue(enc.encode(`data: ${JSON.stringify({ done: true, tokens_in: tIn, tokens_out: tOut })}\n\n`))
      controller.close()
    },
  })

  return new Response(stream, { status: 200, headers: { 'content-type': 'text/event-stream' } })
}
