import { Modal } from './Modal'
import { KeyHint } from './KeyHint'

// HelpOverlay — F1 / ? ile açılan klavye kısayolu referansı.
const GROUPS: { title: string; rows: [string[], string][] }[] = [
  {
    title: 'Genel',
    rows: [
      [['1', '–', '9'], 'Sekme değiştir'],
      [['F1', '?'], 'Bu yardım'],
      [['F5'], 'Sayfayı yenile'],
      [['F10'], 'Oturumu kapat'],
    ],
  },
  {
    title: 'Liste & tablo',
    rows: [
      [['↑', '↓', 'j', 'k'], 'Satır seçimi'],
      [['g', 'G'], 'Başa / sona'],
      [['Enter'], 'Seçili satırı aç'],
      [['/'], 'Filtrele (Esc temizler)'],
      [['s', 'F6'], 'Sırala (kolon döngüsü)'],
    ],
  },
]

export function HelpOverlay({ onClose }: { onClose: () => void }) {
  return (
    <Modal title="Klavye" onClose={onClose} width="max-w-lg">
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
        {GROUPS.map((g) => (
          <div key={g.title}>
            <h3 className="mb-2 font-mono text-[10px] uppercase tracking-[0.08em] text-rx">{g.title}</h3>
            <dl className="space-y-1.5">
              {g.rows.map(([keys, desc]) => (
                <div key={desc} className="flex items-baseline justify-between gap-3">
                  <dt className="flex shrink-0 items-center gap-1">
                    {keys.map((k) => (k === '–' ? <span key={k} className="text-tui-dim">–</span> : <KeyHint key={k}>{k}</KeyHint>))}
                  </dt>
                  <dd className="text-right font-mono text-[11px] text-ink">{desc}</dd>
                </div>
              ))}
            </dl>
          </div>
        ))}
      </div>
      <p className="mt-4 border-t border-rule pt-2 font-mono text-[10px] text-tui-dim">
        Metin alanı odaktayken tek-harf kısayolları pasiftir; Esc alanı bırakır.
      </p>
    </Modal>
  )
}
