import { useCallback, useEffect, useMemo, useState } from 'react'
import { formatBytes, formatNum } from '../lib/format'

interface GeoCountry {
  country: string
  name: string
  lat: number
  lon: number
  bytes: number
  sessions: number
}

const W = 800
const H = 400
// equirectangular: lon/lat → viewBox pikseli
const px = (lon: number) => ((lon + 180) / 360) * W
const py = (lat: number) => ((90 - lat) / 180) * H

// çok kaba kıta blobları (lon,lat çiftleri) — kesin coğrafya değil, "burada
// kara var" hissi için. Bubble'lar anlamı taşır.
const CONTINENTS: [number, number][][] = [
  // Kuzey Amerika
  [[-168, 68], [-95, 72], [-60, 60], [-55, 48], [-80, 25], [-105, 20], [-125, 40], [-160, 55]],
  // Güney Amerika
  [[-80, 10], [-60, 8], [-35, -5], [-40, -25], [-55, -50], [-72, -52], [-80, -18]],
  // Avrupa
  [[-10, 60], [30, 70], [40, 55], [28, 40], [-5, 36], [-10, 44]],
  // Afrika
  [[-17, 33], [12, 35], [33, 30], [43, 12], [40, -12], [20, -35], [12, -18], [-16, 10]],
  // Asya
  [[30, 75], [180, 72], [170, 60], [140, 35], [122, 22], [105, 8], [78, 6], [45, 12], [33, 30], [40, 55], [30, 70]],
  // Avustralya
  [[113, -12], [143, -12], [153, -28], [147, -39], [128, -33], [114, -22]],
]

const RANGES = [
  { label: '1 saat', minutes: 60 },
  { label: '6 saat', minutes: 360 },
  { label: '24 saat', minutes: 1440 },
]

export function GeoMapCard() {
  const [rows, setRows] = useState<GeoCountry[]>([])
  const [minutes, setMinutes] = useState(60)
  const [loaded, setLoaded] = useState(false)
  const [fetchError, setFetchError] = useState(false)
  const [hover, setHover] = useState<GeoCountry | null>(null)
  // imlecin yaninda yüzen ipucu penceresi için ekran (viewport) koordinati
  const [tip, setTip] = useState<{ x: number; y: number } | null>(null)

  const enter = (r: GeoCountry, e: { clientX: number; clientY: number }) => {
    setHover(r)
    setTip({ x: e.clientX, y: e.clientY })
  }
  // klavye odağı: elemanın viewport konumundan ipucu koordinatı türet (mouse yok)
  const focusEnter = (r: GeoCountry, target: SVGGElement) => {
    const rect = target.getBoundingClientRect()
    setHover(r)
    setTip({ x: rect.left + rect.width / 2, y: rect.top })
  }
  const leave = () => {
    setHover(null)
    setTip(null)
  }

  useEffect(() => {
    let stop = false
    const load = async () => {
      try {
        const res = await fetch(`/api/v1/geo?minutes=${minutes}`)
        if (res.status === 401) return
        if (!res.ok) {
          if (!stop) setFetchError(true)
          return
        }
        const data = await res.json()
        if (!stop) {
          setRows(Array.isArray(data) ? data : [])
          setLoaded(true)
          setFetchError(false)
        }
      } catch {
        if (!stop) setFetchError(true)
      }
    }
    load()
    const id = window.setInterval(load, 20_000)
    return () => {
      stop = true
      window.clearInterval(id)
    }
  }, [minutes])

  const maxBytes = useMemo(() => Math.max(1, ...rows.map((r) => r.bytes)), [rows])
  const radius = useCallback((b: number) => 4 + Math.sqrt(b / maxBytes) * 22, [maxBytes])

  // çakışma-önleme: komşu ülke etiketleri (bubble konumu sabit kalır, yalnızca
  // metin dikeyde ayrılır) — birbirine yakın balonlar (ör. TR/BG) aksi halde
  // okunamaz üst üste biner.
  const labelPos = useMemo(() => {
    const items = rows.map((r) => ({
      country: r.country,
      x: px(r.lon),
      y: py(r.lat) - radius(r.bytes) - 3,
      w: r.country.length * 6.2 + 4,
    }))
    for (let iter = 0; iter < 6; iter++) {
      let moved = false
      for (let i = 0; i < items.length; i++) {
        for (let j = i + 1; j < items.length; j++) {
          const a = items[i]
          const b = items[j]
          const dx = Math.abs(a.x - b.x)
          const dy = Math.abs(a.y - b.y)
          if (dx < (a.w + b.w) / 2 && dy < 11) {
            a.y -= 5
            b.y += 5
            moved = true
          }
        }
      }
      if (!moved) break
    }
    return new Map(items.map((it) => [it.country, { x: it.x, y: it.y }]))
  }, [rows, radius])

  return (
    <div>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <div className="flex border border-rule" role="group" aria-label="Zaman aralığı">
          {RANGES.map((r) => (
            <button
              key={r.minutes}
              onClick={() => setMinutes(r.minutes)}
              aria-pressed={minutes === r.minutes}
              className={`px-2 py-0.5 font-mono text-[11px] transition ${
                minutes === r.minutes ? 'bg-rx text-ground' : 'text-tui-dim hover:text-ink-hi'
              }`}
            >
              {r.label}
            </button>
          ))}
        </div>
        {fetchError && (
          <span className="ml-auto font-mono text-[11px] text-rose-400">⚠ veri alınamadı, yeniden deneniyor…</span>
        )}
      </div>

      {!loaded ? (
        <p className="py-8 text-center font-mono text-[11px] text-tui-dim">Yükleniyor…</p>
      ) : rows.length === 0 ? (
        <p className="py-8 text-center font-mono text-[11px] text-tui-dim">
          Coğrafi veri yok — MaxMind MMDB veya <code className="text-tui-dim">-ip-api-lookup</code> gerekir ve
          uzak trafik (NetFlow/agent) görülmüş olmalı.
        </p>
      ) : (
        <div className="overflow-x-auto">
          <svg viewBox={`0 0 ${W} ${H}`} className="w-full min-w-[560px]" role="group" aria-label="Uzak trafiğin ülke bazlı dünya haritası">
            <g aria-hidden="true">
              <rect x={0} y={0} width={W} height={H} fill="#0a0d13" />
              {/* graticule */}
              {[-120, -60, 0, 60, 120].map((lon) => (
                <line key={`v${lon}`} x1={px(lon)} y1={0} x2={px(lon)} y2={H} stroke="#16233a" strokeWidth={1} />
              ))}
              {[-60, -30, 0, 30, 60].map((lat) => (
                <line key={`h${lat}`} x1={0} y1={py(lat)} x2={W} y2={py(lat)} stroke="#16233a" strokeWidth={1} />
              ))}
              {/* kaba kıtalar */}
              {CONTINENTS.map((poly, i) => (
                <polygon
                  key={i}
                  points={poly.map(([lon, lat]) => `${px(lon).toFixed(0)},${py(lat).toFixed(0)}`).join(' ')}
                  fill="#13233a"
                  stroke="#1e3350"
                  strokeWidth={1}
                />
              ))}
            </g>
            {/* trafik balonları — hem mouse hem klavye (Tab/focus) ile erişilebilir */}
            {rows.map((r) => {
              const active = hover?.country === r.country
              const lp = labelPos.get(r.country) ?? { x: px(r.lon), y: py(r.lat) - radius(r.bytes) - 3 }
              const labelW = r.country.length * 6.2 + 4
              return (
                <g
                  key={r.country}
                  role="button"
                  tabIndex={0}
                  aria-label={`${r.name} (${r.country}), ${formatBytes(r.bytes)} trafik, ${formatNum(r.sessions)} uç nokta oturumu`}
                  onMouseEnter={(e) => enter(r, e)}
                  onMouseMove={(e) => tip && setTip({ x: e.clientX, y: e.clientY })}
                  onMouseLeave={leave}
                  onFocus={(e) => focusEnter(r, e.currentTarget)}
                  onBlur={leave}
                  onKeyDown={(e) => e.key === 'Escape' && leave()}
                  style={{ cursor: 'pointer' }}
                >
                  <circle
                    cx={px(r.lon)}
                    cy={py(r.lat)}
                    r={radius(r.bytes)}
                    fill="#22d3ee"
                    fillOpacity={active ? 0.34 : 0.18}
                    stroke={active ? '#67e8f9' : 'none'}
                    strokeWidth={1}
                  />
                  <circle cx={px(r.lon)} cy={py(r.lat)} r={3} fill="#22d3ee" />
                  {/* etiket zemin çipi — çakışma-önlemeyle taşınan etiketler komşu balonun üzerine denk gelebilir, okunurluk için */}
                  <rect
                    x={lp.x - labelW / 2}
                    y={lp.y - 8}
                    width={labelW}
                    height={10}
                   
                    fill="#0a0d13"
                    fillOpacity={0.75}
                  />
                  <text
                    x={lp.x}
                    y={lp.y}
                    textAnchor="middle"
                    fontSize={9}
                    fill={active ? '#e2e8f0' : '#94a3b8'}
                    fontFamily="ui-monospace, monospace"
                  >
                    {r.country}
                  </text>
                </g>
              )
            })}
          </svg>
        </div>
      )}

      {/* en yoğun ülkeler — sabit özet şeridi */}
      <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-[11px]">
        {rows.slice(0, 6).map((r) => (
          <span
            key={r.country}
            className={`font-mono ${hover?.country === r.country ? 'text-cyan-300' : 'text-tui-dim'}`}
          >
            {r.country} {formatBytes(r.bytes)}
          </span>
        ))}
      </div>

      {/* imlecin yanında yüzen ipucu penceresi — sabit genişlik: shrink-to-fit +
          transform kombinasyonu köşede kutuyu sıkıştırıp taşırıyordu, artık
          konum doğrudan sabit kutu boyutuna göre viewport'a kenetleniyor */}
      {hover && tip && (
        <div
          className="pointer-events-none fixed z-50 w-[190px] border border-rule-hi bg-panel px-2.5 py-1.5 text-[11px] "
          style={{
            left: Math.max(8, Math.min(tip.x + 14, window.innerWidth - 190 - 8)),
            top: Math.max(8, Math.min(tip.y + 16, window.innerHeight - 80 - 8)),
          }}
        >
          <div className="font-mono font-semibold text-cyan-300">
            {hover.name} <span className="text-tui-dim">({hover.country})</span>
          </div>
          <div className="mt-0.5 text-ink">{formatBytes(hover.bytes)} trafik</div>
          <div className="text-tui-dim">{formatNum(hover.sessions)} uç nokta oturumu</div>
        </div>
      )}
    </div>
  )
}
