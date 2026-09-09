import React from 'react';
import Layout from '@theme/Layout';
import useBaseUrl from '@docusaurus/useBaseUrl';
import styles from './index.module.css';

/* ============================================================================
   Landing page — ürünün kendi arayüzüyle (frontend/DESIGN.md, "htop
   çok-panelli terminal") aynı dil. Sayfa bir bazNTMS ekranı gibi okunur:
   üstte TuiHeader benzeri şerit, panel panel aşağı inen ekranlar, altta sabit
   numaralı ekran şeridi (1–7 tuşuyla gezilir, tıpkı dashboard'daki TabBar).
   Kare köşe, tek mono aile, gölge/blur/gradyan yok. 7 renk sabit-anlam.
   Dekoratif hiçbir renk yok — her renk panelde gördüğünüz anlamı taşır.
   ========================================================================== */

const REPO = 'https://github.com/gokayybaz/bazntms';
const QUICK_CMD = 'docker compose -f deploy/docker-compose.yml up --build';

/* alt şerit: numaralı ekranlar (dashboard TabBar karşılığı) */
const SCREENS = [
  { id: 'ust', label: 'ÜST' },
  { id: 'yetenekler', label: 'YETENEK' },
  { id: 'panel', label: 'PANEL' },
  { id: 'uyumluluk', label: '5651' },
  { id: 'mimari', label: 'MİMARİ' },
  { id: 'olcek', label: 'ÖLÇEK' },
  { id: 'kurulum', label: 'KURULUM' },
];

/* 12 yetenek — datasheet satırı: ALAN → YETENEK → MEKANİZMA. Grup başına
   sol-kenar anlam rengi (Triad Rule — frontend Overview stat tile ile aynı). */
const CAPABILITIES = [
  {
    group: 'Görünürlük',
    accent: 'rx',
    rows: [
      [
        'Trafik ölçümü',
        'Paket bazlı yön / protokol / port dağılımı; en yoğun uç noktalar GeoIP + ASN ile dünya haritasında hacme göre.',
      ],
      [
        'L7 + DNS görünürlüğü',
        'Süreç bazlı TLS ClientHello SNI + HTTP Host + DNS sorgu/yanıt — imza tabanlı DPI yok, "hangi süreç hangi alan adına".',
      ],
      [
        'Topoloji',
        'LLDP/CDP/ARP keşfi + agent subnet bildirimi → client → hub → cihaz → router → internet zinciri otomatik haritada.',
      ],
    ],
  },
  {
    group: 'Toplama & entegrasyon',
    accent: 'tx',
    rows: [
      [
        'Agent filosu',
        'Enrollment, toplu telemetri, offline disk kuyruğu — doğrulanmış 5 000 agent. Agent↔hub mTLS, sertifikalar kendini yeniler.',
      ],
      [
        'Akış toplama',
        'NetFlow v5/v9, IPFIX ve sFlow v5 tek toplayıcıda; şablon önbelleği + örnekleme oranına göre ölçekleme.',
      ],
      [
        'Ağ cihazları',
        'SNMPv3 arayüz/durum takibi, syslog alıcısı, FortiGate REST API — VPN tünel, SD-WAN sağlık, politika hit trendi.',
      ],
    ],
  },
  {
    group: 'Güvenlik & uyumluluk',
    accent: 'rose',
    rows: [
      [
        'Erişim denetimi',
        'Rol tabanlı erişim (admin / netops / analyst / viewer + site scope), OIDC SSO, hash-zincirli append-only denetim kaydı.',
      ],
      [
        'SIEM / IOC',
        'IOC kara listesiyle L7 + DNS eşleştirme; olaylar CEF / LEEF / JSON → Splunk HEC, QRadar, ServiceNow, ArcSight.',
      ],
      [
        '5651 + ISO 27001',
        'İmzalı log zinciri + RFC 3161 zaman damgası; risk defteri, SoA, iç denetim ve tek tıkla denetçi paketi.',
      ],
    ],
  },
  {
    group: 'Ölçek & operasyon',
    accent: 'emerald',
    rows: [
      [
        'Depo',
        'SQLite → PostgreSQL + TimescaleDB tek bayrakla; katmanlı saklama ham 7g · 1dk 90g · 1sa 2y. NATS JetStream ingest hattı.',
      ],
      [
        'Anomali & rapor',
        'Mevsimsel + EWMA baseline anomali tespiti; SLA / kapasite / PDF raporlar; zamanlı ve olay-tetikli çalışır.',
      ],
      [
        'AI analiz',
        'Çoklu sağlayıcı (OpenAI-uyumlu + Anthropic); /ai sohbet sekmesi, gecelik analiz, olay triyajı. Opt-in + egress kilidi.',
      ],
    ],
  },
];

/* 5651 zinciri — her halka bir öncekinin çıktısını mühürler */
const CHAIN = [
  { tag: 'OLAY', value: 'log kaydı' },
  { tag: 'ZİNCİR', value: 'SHA-256 hash' },
  { tag: 'SAATLİK', value: 'Merkle kök' },
  { tag: 'GÜNLÜK', value: 'RFC 3161 damga' },
  { tag: 'MANİFEST', value: 'ed25519 imza' },
  { tag: 'SAKLAMA', value: 'WORM · 2 yıl', done: true },
];

/* Tek düğüm → Ölçek: aynı binary'nin iki yapılandırması */
const SCALE_ROWS = [
  ['Depo', 'SQLite dosyası', 'PostgreSQL + TimescaleDB (pgx)'],
  ['Kuyruk', 'Doğrudan yazım', 'NATS JetStream — ingest / processor ayrışması'],
  ['Saklama', 'Otomatik temizlik', 'Hypertable + cagg: ham 7g → 1dk 90g → 1sa 2y'],
  ['Dağıtım', 'Tek binary · docker-compose', 'Helm — N × ingest + kontrolcü + yük dengeleyici'],
  ['Taşıma', 'HTTP', 'Opsiyonel mTLS — dahili CA, ECDSA P-256'],
  ['Güncelleme', 'Elle', 'stable / beta kanalı — SHA-256 + ed25519 doğrulamalı atomik değişim'],
];

/* üç birbirini dışlayan kurulum yolu (sıralı adım değil) */
const STEPS = [
  {
    badge: 'A',
    label: 'Tek-node demo',
    code: `git clone ${REPO}
cd bazntms
docker compose -f deploy/docker-compose.yml up --build
# → http://localhost:8080 · şifre: demo123`,
  },
  {
    badge: 'B',
    label: 'Elle derle ve agent bağla',
    code: `make                       # frontend + hub + agent + ctl
./bazntmsctl setup         # interaktif sihirbaz → bazntms-hub.yml
./bazntms-hub -config bazntms-hub.yml

# agent (deb · rpm · MSI · pkg release sayfasında):
./bazntms-agent -hub-url https://hub.example.com \\
  -enroll-token <hub-loglarındaki-token>`,
  },
  {
    badge: 'C',
    label: 'Ölçek mimarisi (k8s olmadan)',
    code: `docker compose -f deploy/docker-compose.scale.yml up --build
# 2 × ingest + kontrolcü + nginx LB + JetStream
# --scale hub-ingest=4 → yatay büyüt
# dashboard :8080 · agent API :8081`,
  },
];

/* v1.3.0 hattına kadar operatörün önemsediği başlıklar */
const NEW_IN = [
  'eBPF süreç atfı (Linux)',
  'ETW süreç atfı (Windows)',
  'çoklu-sağlayıcı AI analiz + /ai',
  'uyarı yaşam döngüsü + bakım penceresi',
  'Jira / ServiceNow connector',
  'zamanlı / PDF / SLA rapor',
  'olay motoru + tehdit istihbaratı',
  'sağlık skoru',
  'v1.0 GA + doğrulanmış kapasite',
];

/* --- deterministik RNG (SSR ↔ hidrasyon farkı çıkmasın) --- */
function makeRng(seed) {
  let s = seed >>> 0;
  return () => {
    s = (s * 1103515245 + 12345) & 0x7fffffff;
    return s / 0x7fffffff;
  };
}

function prefersReducedMotion() {
  return (
    typeof window !== 'undefined' &&
    window.matchMedia &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  );
}

/* ============================ TUI primitifleri ============================ */

function Panel({ title, right, children, className = '' }) {
  return (
    <section className={`${styles.panel} ${className}`}>
      {(title || right) && (
        <header className={styles.panelHead}>
          {title && (
            <h3 className={styles.panelTitle}>
              <span aria-hidden>┤</span>
              <span>{title}</span>
              <span aria-hidden>├</span>
            </h3>
          )}
          {right && <div className={styles.panelRight}>{right}</div>}
        </header>
      )}
      <div className={styles.panelBody}>{children}</div>
    </section>
  );
}

/* Meter — LABEL [███····] değer. accent rx/tx tek renk; threshold htop rampası. */
function Meter({ label, value, max, display, width = 18, accent = 'threshold' }) {
  const frac = max > 0 && Number.isFinite(value) ? Math.min(1, Math.max(0, value / max)) : 0;
  const filled = Math.round(frac * width);
  const empty = width - filled;
  const text = display ?? String(Math.round(value || 0));

  let bar;
  let valueCls;
  if (accent === 'rx' || accent === 'tx') {
    const cls = accent === 'rx' ? styles.fgRx : styles.fgTx;
    bar = <span className={cls}>{'█'.repeat(filled)}</span>;
    valueCls = cls;
  } else {
    const warnAt = Math.round(0.6 * width);
    const critAt = Math.round(0.85 * width);
    const nOf = (from, to) => Math.max(0, Math.min(filled, to) - Math.max(0, from));
    bar = (
      <>
        <span className={styles.fgEmerald}>{'█'.repeat(nOf(0, warnAt))}</span>
        <span className={styles.fgAmber}>{'█'.repeat(nOf(warnAt, critAt))}</span>
        <span className={styles.fgRose}>{'█'.repeat(nOf(critAt, width))}</span>
      </>
    );
    valueCls =
      frac < 0.6 ? styles.fgEmerald : frac < 0.85 ? styles.fgAmber : styles.fgRose;
  }

  return (
    <div className={styles.meter} role="meter" aria-label={label} aria-valuenow={Math.round(value || 0)} aria-valuemin={0} aria-valuemax={Math.round(max || 0)}>
      <span className={styles.meterLabel}>{label}</span>
      <span aria-hidden className={styles.meterBar}>
        <span className={styles.meterBracket}>[</span>
        <span className={styles.meterTrack}>
          {bar}
          <span className={styles.fgRule}>{'·'.repeat(empty)}</span>
        </span>
        <span className={styles.meterBracket}>]</span>
      </span>
      <span className={`${styles.meterValue} ${valueCls}`}>{text}</span>
    </div>
  );
}

/* Sparkline — 8 seviyeli blok rampası (▁▂▃▄▅▆▇█) */
const RAMP = ['▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'];
function sparkChars(data) {
  if (!data.length) return '';
  const min = Math.min(...data);
  const max = Math.max(...data);
  const span = max - min || 1;
  return data
    .map((v) => RAMP[Math.max(0, Math.min(7, Math.round(((v - min) / span) * 7)))])
    .join('');
}

/* Bracket — [ ETIKET ] link/buton. primary → reverse-video cyan. */
function Bracket({ href, to, children, primary = false, external = false }) {
  const baseUrl = useBaseUrl(to || '/');
  const url = href ?? baseUrl;
  return (
    <a
      className={`${styles.bracket} ${primary ? styles.bracketPrimary : ''}`}
      href={url}
      {...(external ? { target: '_blank', rel: 'noreferrer' } : {})}
    >
      <span aria-hidden className={styles.bracketEdge}>
        [
      </span>
      <span className={styles.bracketLabel}>{children}</span>
      <span aria-hidden className={styles.bracketEdge}>
        ]
      </span>
    </a>
  );
}

/* CopyButton — pano erişimi kurumsal politikayla engellenebilir → hata durumu */
function CopyButton({ code, className = '' }) {
  const [state, setState] = React.useState('idle');
  return (
    <button
      type="button"
      className={`${styles.copy} ${state === 'error' ? styles.copyError : ''} ${className}`}
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(code);
          setState('done');
        } catch {
          setState('error');
        } finally {
          window.setTimeout(() => setState('idle'), 1800);
        }
      }}
    >
      {state === 'done' ? '✓ kopyalandı' : state === 'error' ? '✗ kopyalanamadı' : 'kopyala'}
    </button>
  );
}

/* ThroughputTrace — elle SVG, basamaklı çizgi, mono tick, karakter-ızgara
   zemin, alan dolgusu / gradyan YOK. Tek rAF döngüsü ~4 fps; sentetik veri;
   prefers-reduced-motion → tek kare donar. (frontend ThroughputChart kalıbı) */
const TRACE_N = 96;
function initTrace(seed) {
  const rnd = makeRng(seed);
  const rx = [];
  const tx = [];
  let r = 0.18;
  let burst = 0;
  for (let i = 0; i < TRACE_N; i++) {
    if (burst > 0) burst--;
    else if (rnd() < 0.05) burst = 4 + Math.floor(rnd() * 10);
    const target = burst > 0 ? 0.55 + rnd() * 0.38 : 0.1 + rnd() * 0.22;
    r = Math.max(0.03, Math.min(0.97, r + (target - r) * 0.28 + (rnd() - 0.5) * 0.05));
    rx.push(r);
    tx.push(Math.max(0.02, Math.min(0.7, r * (0.22 + rnd() * 0.2))));
  }
  return { rx, tx };
}

function ThroughputTrace() {
  const [buf, setBuf] = React.useState(() => initTrace(20260909));
  const rxRef = React.useRef(null);
  const txRef = React.useRef(null);

  React.useEffect(() => {
    if (prefersReducedMotion()) return undefined;
    const rnd = makeRng(97531);
    let raf = 0;
    let last = 0;
    let burst = 0;
    const loop = (ts) => {
      if (ts - last > 240) {
        last = ts;
        setBuf((prev) => {
          const rx = prev.rx.slice(1);
          const tx = prev.tx.slice(1);
          if (burst > 0) burst--;
          else if (rnd() < 0.05) burst = 4 + Math.floor(rnd() * 10);
          const lastR = prev.rx[prev.rx.length - 1];
          const target = burst > 0 ? 0.55 + rnd() * 0.38 : 0.1 + rnd() * 0.22;
          const r = Math.max(
            0.03,
            Math.min(0.97, lastR + (target - lastR) * 0.28 + (rnd() - 0.5) * 0.06),
          );
          rx.push(r);
          tx.push(Math.max(0.02, Math.min(0.7, r * (0.22 + rnd() * 0.2))));
          return { rx, tx };
        });
      }
      raf = window.requestAnimationFrame(loop);
    };
    raf = window.requestAnimationFrame(loop);
    return () => window.cancelAnimationFrame(raf);
  }, []);

  const W = 520;
  const H = 150;
  const PAD = { t: 10, r: 8, b: 18, l: 40 };
  const iw = W - PAD.l - PAD.r;
  const ih = H - PAD.t - PAD.b;
  const peak = Math.max(0.35, ...buf.rx);
  const x = (i) => PAD.l + (i / (TRACE_N - 1)) * iw;
  const y = (v) => PAD.t + ih - (v / peak) * ih;

  const step = (series) => {
    let d = `M${x(0).toFixed(1)},${y(series[0]).toFixed(1)}`;
    for (let i = 1; i < series.length; i++) {
      const px = x(i).toFixed(1);
      d += ` L${px},${y(series[i - 1]).toFixed(1)} L${px},${y(series[i]).toFixed(1)}`;
    }
    return d;
  };

  const lastRx = buf.rx[buf.rx.length - 1];
  const lastTx = buf.tx[buf.tx.length - 1];
  if (rxRef.current) rxRef.current.textContent = (lastRx * 11.6).toFixed(2);
  if (txRef.current) txRef.current.textContent = (lastTx * 11.6).toFixed(2);

  const yTicks = [0, 0.5, 1];
  const xTicks = [0, 0.25, 0.5, 0.75, 1];

  return (
    <div className={styles.trace}>
      <div className={styles.traceHead}>
        <span>
          <i className={styles.swatchRx} /> indirilen{' '}
          <b ref={rxRef} className={styles.fgRx}>
            {(lastRx * 11.6).toFixed(2)}
          </b>{' '}
          Mb/s
        </span>
        <span>
          <i className={styles.swatchTx} /> gönderilen{' '}
          <b ref={txRef} className={styles.fgTx}>
            {(lastTx * 11.6).toFixed(2)}
          </b>{' '}
          Mb/s
        </span>
      </div>
      <svg viewBox={`0 0 ${W} ${H}`} className={styles.traceSvg} preserveAspectRatio="none" role="img" aria-label="Temsilî canlı verim izi">
        {yTicks.map((f, i) => (
          <line key={`y${i}`} x1={PAD.l} x2={W - PAD.r} y1={PAD.t + ih - f * ih} y2={PAD.t + ih - f * ih} stroke="#232b3a" strokeWidth="1" />
        ))}
        {xTicks.map((f, i) => (
          <line key={`x${i}`} x1={PAD.l + f * iw} x2={PAD.l + f * iw} y1={PAD.t} y2={PAD.t + ih} stroke="#232b3a" strokeWidth="1" />
        ))}
        {yTicks.map((f, i) => (
          <text key={`yt${i}`} x={PAD.l - 6} y={PAD.t + ih - f * ih + 3} textAnchor="end" fill="#8794a8" fontSize="8" fontFamily="ui-monospace, monospace">
            {(f * peak * 11.6).toFixed(0)}
          </text>
        ))}
        <text x={PAD.l} y={H - 4} fill="#8794a8" fontSize="8" fontFamily="ui-monospace, monospace">
          -96 sn
        </text>
        <text x={W - PAD.r} y={H - 4} textAnchor="end" fill="#8794a8" fontSize="8" fontFamily="ui-monospace, monospace">
          şimdi
        </text>
        <path d={step(buf.tx)} fill="none" stroke="#a78bfa" strokeWidth="1.4" vectorEffect="non-scaling-stroke" />
        <path d={step(buf.rx)} fill="none" stroke="#22d3ee" strokeWidth="1.4" vectorEffect="non-scaling-stroke" />
        <rect x={x(TRACE_N - 1) - 2.4} y={y(lastTx) - 2.4} width="4.8" height="4.8" fill="#a78bfa" />
        <rect x={x(TRACE_N - 1) - 2.4} y={y(lastRx) - 2.4} width="4.8" height="4.8" fill="#22d3ee" />
      </svg>
    </div>
  );
}

/* Hero üst şeridi — dashboard TuiHeader karşılığı */
function TopStrip() {
  const [now, setNow] = React.useState(null);
  React.useEffect(() => {
    setNow(new Date());
    const id = window.setInterval(() => setNow(new Date()), 1000);
    return () => window.clearInterval(id);
  }, []);
  return (
    <div className={styles.topStrip}>
      <span className={styles.brand}>
        <BrandGlyph />
        <b>bazNTMS</b>
      </span>
      <span className={styles.wsPill}>
        <i className={styles.wsDot} /> WS: CANLI
      </span>
      <span className={styles.stripMeters}>
        <Meter label="RX" value={0.62} max={1} width={7} accent="rx" display="7.2 Mb/s" />
        <Meter label="TX" value={0.2} max={1} width={7} accent="tx" display="2.3 Mb/s" />
        <Meter label="PPS" value={0.32} max={1} width={7} accent="threshold" display="3.2K" />
      </span>
      <span className={styles.stripRight}>
        <span className={styles.verTag}>v1.3.0</span>
        <span className={styles.clock}>
          {now ? now.toLocaleTimeString('tr-TR') : '--:--:--'}
          <i className={styles.cursor} />
        </span>
      </span>
    </div>
  );
}

function BrandGlyph() {
  return (
    <svg viewBox="0 0 24 24" className={styles.glyph} fill="none" stroke="currentColor" strokeWidth="1.9" aria-hidden>
      <circle cx="12" cy="12" r="2" fill="currentColor" stroke="none" />
      <circle cx="5" cy="5" r="1.6" />
      <circle cx="19" cy="5" r="1.6" />
      <circle cx="5" cy="19" r="1.6" />
      <circle cx="19" cy="19" r="1.6" />
      <path d="M6.2 6.2 10.6 10.6m6.8-4.4-4.4 4.4M6.2 17.8l4.4-4.4m6.8 4.4-4.4-4.4" strokeLinecap="round" />
    </svg>
  );
}

/* ============================ Panel vitrin (statik Dashboard alıntısı) ============================ */

const SHOW_SPARK = [
  0.42, 0.5, 0.38, 0.55, 0.7, 0.52, 0.6, 0.75, 0.58, 0.44, 0.5, 0.62, 0.8, 0.66, 0.52,
  0.48, 0.58, 0.72, 0.85, 0.7, 0.55, 0.6, 0.68, 0.5, 0.46, 0.58, 0.64, 0.78, 0.6, 0.52,
  0.5, 0.44, 0.56, 0.66, 0.6, 0.48,
];
const SHOW_ENDPOINTS = [
  ['cdn.example.net', 'AS13335 · US', 0.92],
  ['update.example.com', 'AS16509 · IE', 0.64],
  ['pkg.example.org', 'AS24940 · DE', 0.47],
  ['api.example.io', 'AS15169 · NL', 0.31],
  ['mail.example.net', 'AS8075 · TR', 0.19],
];
const SHOW_FEED = [
  ['proc', 'yeni süreç · agent-ofis-3 : curl'],
  ['warn', 'bant genişliği zirvesi · agent-dc1-07'],
  ['crit', 'IOC eşleşmesi · agent-sube-a'],
];
const SHOW_TILES = [
  ['AKTİF AGENT', '124 / 140', 'filo toplamı', 'emerald'],
  ['PAKET HIZI', '3.2K pps', 'anlık', 'rx'],
  ['OLAY HIZI', '4.8 / sn', 'son 60 sn', 'tx'],
  ['AÇIK UYARI', '2', 'kritik dahil', 'amber'],
];

function ShowcasePanel() {
  return (
    <Panel
      title="bazntms.local · Genel Bakış"
      right={
        <span className={styles.wsPill}>
          <i className={styles.wsDot} /> WS: CANLI
        </span>
      }
      className={styles.showcase}
    >
      <div className={styles.tiles}>
        {SHOW_TILES.map(([label, value, cap, tone]) => (
          <div key={label} className={`${styles.tile} ${styles[`edge_${tone}`]}`}>
            <span className={styles.tileLabel}>{label}</span>
            <b className={styles.tileValue}>{value}</b>
            <span className={styles.tileCap}>{cap}</span>
          </div>
        ))}
      </div>

      <div className={styles.showBody}>
        <div className={styles.showCol}>
          <div className={styles.showColHead}>
            <span>VERİM · 48 sn</span>
            <span className={styles.dimText}>Mb/s</span>
          </div>
          <div className={`${styles.sparkRow} ${styles.fgRx}`}>{sparkChars(SHOW_SPARK)}</div>
          <div className={styles.showColHead}>
            <span>EN YOĞUN UÇ NOKTALAR</span>
          </div>
          <ul className={styles.epList}>
            {SHOW_ENDPOINTS.map(([host, meta, v]) => (
              <li key={host}>
                <div className={styles.epRow}>
                  <span className={styles.epHost}>{host}</span>
                  <span className={styles.epMeta}>{meta}</span>
                </div>
                <div className={styles.epBar}>
                  <i style={{ width: `${Math.round(v * 100)}%` }} />
                </div>
              </li>
            ))}
          </ul>
        </div>
        <div className={styles.showCol}>
          <div className={styles.showColHead}>
            <span>UYARI AKIŞI</span>
          </div>
          <ul className={styles.feed}>
            {SHOW_FEED.map(([kind, text]) => (
              <li key={text} className={styles[`feed_${kind}`]}>
                <span className={styles.feedDot} />
                {text}
              </li>
            ))}
          </ul>
        </div>
      </div>
    </Panel>
  );
}

/* ============================ Mimari (elle SVG) ============================ */

function Architecture() {
  const agents = [
    { y: 62, name: 'agent · ofis-a' },
    { y: 146, name: 'agent · dc1' },
    { y: 230, name: 'agent · şube-3' },
  ];
  const devices = [
    { y: 62, name: 'firewall' },
    { y: 146, name: 'core-switch' },
    { y: 230, name: 'router' },
  ];
  return (
    <svg className={styles.archSvg} viewBox="0 0 960 390" role="img" aria-label="bazNTMS mimarisi: agent'lar ve ağ cihazları hub'a telemetri gönderir; hub PostgreSQL/TimescaleDB ve NATS JetStream üzerine yazar">
      <g className={styles.archFlow}>
        <path d="M232 85 C 320 85, 340 158, 392 172" />
        <path d="M232 169 L 392 186" />
        <path d="M232 253 C 320 253, 340 214, 392 200" />
        <path d="M728 85 C 640 85, 620 158, 568 172" />
        <path d="M728 169 L 568 186" />
        <path d="M728 253 C 640 253, 620 214, 568 200" />
        <path d="M436 246 L 436 300" />
        <path d="M524 246 L 524 300" />
      </g>

      <text x="40" y="34" className={styles.archGroup}>UÇLAR</text>
      {agents.map((a) => (
        <g key={a.name}>
          <rect x="40" y={a.y} width="192" height="44" className={styles.archNode} />
          <rect x="56" y={a.y + 20} width="6" height="6" className={styles.archDotRx} />
          <text x="74" y={a.y + 27} className={styles.archText}>{a.name}</text>
        </g>
      ))}

      <text x="920" y="34" className={styles.archGroup} textAnchor="end">AĞ CİHAZLARI</text>
      {devices.map((d) => (
        <g key={d.name}>
          <rect x="728" y={d.y} width="192" height="44" className={styles.archNode} />
          <rect x="744" y={d.y + 20} width="6" height="6" className={styles.archDotTx} />
          <text x="762" y={d.y + 27} className={styles.archText}>{d.name}</text>
        </g>
      ))}

      <rect x="392" y="136" width="176" height="110" className={styles.archHub} />
      <text x="480" y="178" textAnchor="middle" className={styles.archHubText}>bazntms-hub</text>
      <text x="480" y="200" textAnchor="middle" className={styles.archSub}>ingest · RBAC</text>
      <text x="480" y="220" textAnchor="middle" className={styles.archSub}>audit · uyarı motoru</text>

      <rect x="336" y="300" width="200" height="52" className={styles.archNode} />
      <text x="436" y="324" textAnchor="middle" className={styles.archText}>PostgreSQL + TimescaleDB</text>
      <text x="436" y="340" textAnchor="middle" className={styles.archSub}>hypertable · cagg · retention</text>

      <rect x="556" y="300" width="200" height="52" className={styles.archNode} />
      <text x="656" y="324" textAnchor="middle" className={styles.archText}>NATS JetStream</text>
      <text x="656" y="340" textAnchor="middle" className={styles.archSub}>ingest → processor</text>

      <text x="292" y="126" className={styles.archLabel}>telemetri ↑ (mTLS)</text>
      <text x="668" y="126" className={styles.archLabel}>SNMP · NetFlow · Syslog</text>
    </svg>
  );
}

/* ============================ Alt ekran şeridi (FnKeyBar karşılığı) ============================ */

function ScreenBar({ active, onJump }) {
  return (
    <div className={styles.screenBar}>
      <div className={styles.screenBarInner}>
        {SCREENS.map((s, i) => (
          <button
            key={s.id}
            type="button"
            className={`${styles.screenKey} ${active === s.id ? styles.screenKeyActive : ''}`}
            onClick={() => onJump(s.id)}
          >
            <span className={styles.screenNum}>{i + 1}</span>
            <span>{s.label}</span>
          </button>
        ))}
        <span className={styles.screenBarSpacer} />
        <a className={styles.screenAction} href="#kurulum">
          KURULUM ↵
        </a>
        <a className={styles.screenAction} href={REPO} target="_blank" rel="noreferrer">
          GITHUB ↗
        </a>
      </div>
    </div>
  );
}

/* ============================ Sayfa ============================ */

export default function Home() {
  const [active, setActive] = React.useState('ust');
  const docsUrl = useBaseUrl('/docs/installation');
  const apiUrl = useBaseUrl('/docs/reference/api');
  const configUrl = useBaseUrl('/docs/reference/configuration');

  const jump = React.useCallback((id) => {
    const el = document.getElementById(id);
    if (el) el.scrollIntoView({ behavior: prefersReducedMotion() ? 'auto' : 'smooth', block: 'start' });
  }, []);

  /* aktif ekranı en görünür bölümden izle */
  React.useEffect(() => {
    const ids = SCREENS.map((s) => s.id);
    const io = new IntersectionObserver(
      (entries) => {
        const vis = entries
          .filter((e) => e.isIntersecting)
          .sort((a, b) => b.intersectionRatio - a.intersectionRatio)[0];
        if (vis) setActive(vis.target.id);
      },
      { rootMargin: '-45% 0px -45% 0px', threshold: [0, 0.5, 1] },
    );
    ids.forEach((id) => {
      const el = document.getElementById(id);
      if (el) io.observe(el);
    });
    return () => io.disconnect();
  }, []);

  /* klavye: 1–7 ekran, g/G baş/son, ? kurulum (dashboard useHotkeys dili;
     tarayıcı/OS'a bağlı F-tuşları ele geçirilmez) */
  React.useEffect(() => {
    const onKey = (e) => {
      const t = e.target;
      if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)) return;
      if (e.metaKey || e.ctrlKey || e.altKey) return;
      const n = Number(e.key);
      if (n >= 1 && n <= SCREENS.length) {
        jump(SCREENS[n - 1].id);
      } else if (e.key === 'g') {
        window.scrollTo({ top: 0, behavior: prefersReducedMotion() ? 'auto' : 'smooth' });
      } else if (e.key === 'G') {
        window.scrollTo({ top: document.body.scrollHeight, behavior: prefersReducedMotion() ? 'auto' : 'smooth' });
      } else if (e.key === '?') {
        jump('kurulum');
      } else {
        return;
      }
      e.preventDefault();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [jump]);

  return (
    <Layout
      title="Ağ Trafiği İzleme Platformu"
      description="Hub + agent + cihaz entegrasyonları: canlı paket ölçümü, süreç bazlı L7/DNS görünürlüğü, TimescaleDB + NATS ölçek altyapısı, RBAC/SSO, 5651 uyumlu imzalı loglar. Açık kaynak, MIT, kendi altyapınızda."
    >
      <main className={`${styles.page} landing-root`}>
        {/* ---------- ÜST: Genel Bakış ekranı ---------- */}
        <section id="ust" className={styles.hero}>
          <div className={styles.shell}>
            <TopStrip />
            <div className={styles.heroGrid}>
              <div className={styles.heroMain}>
                <h1 className={styles.title}>
                  Paketten <span className={styles.fgRx}>imzalı kayda</span> kadar tek
                  platform.
                </h1>
                <p className={styles.lede}>
                  Paket seviyesinde izleme, akış toplama ve 5651 uyumlu imzalı loglar.
                  Tek makineden <span className={styles.fgRx}>5.000 agent</span>'a kadar
                  tek binary.
                </p>

                <div className={styles.cmd}>
                  <span aria-hidden className={styles.cmdSigil}>$</span>
                  <code className={styles.cmdText}>{QUICK_CMD}</code>
                  <CopyButton code={QUICK_CMD} />
                </div>

                <div className={styles.heroCtas}>
                  <Bracket to="/docs/installation" primary>
                    KURULUM
                  </Bracket>
                  <Bracket href={REPO} external>
                    GITHUB ↗
                  </Bracket>
                  <span className={styles.heroMeta}>MIT · vendor lock-in yok</span>
                </div>
              </div>

              <div className={styles.heroAside}>
                <Panel title="CANLI ÖZET">
                  <div className={styles.asideMeters}>
                    <Meter label="RX" value={0.62} max={1} accent="rx" display="7.2 Mb/s" />
                    <Meter label="TX" value={0.2} max={1} accent="tx" display="2.3 Mb/s" />
                    <Meter label="PPS" value={0.32} max={1} accent="threshold" display="3.2K pps" />
                    <Meter label="EVT" value={0.24} max={1} accent="threshold" display="4.8 / sn" />
                  </div>
                  <ThroughputTrace />
                  <p className={styles.asideNote}>
                    temsilî akış — gerçek veri değil, örnekleme penceresini gösterir
                  </p>
                </Panel>
              </div>
            </div>
          </div>
        </section>

        {/* ---------- YETENEK ---------- */}
        <section id="yetenekler" className={styles.section}>
          <div className={styles.shell}>
            <h2 className={styles.h2}>Uçtan uca görünürlük</h2>
            <p className={styles.sectionLede}>
              Dört alan, on iki yetenek. Her satır bir mekanizma anlatır — her rengin
              sabit bir okunuşu var, panelde gördüğünüzle aynı.
            </p>

            <div className={styles.tableWrap}>
              <table className={styles.tuiTable}>
                <colgroup>
                  <col className={styles.colAlan} />
                  <col className={styles.colTerm} />
                  <col className={styles.colMech} />
                </colgroup>
                <thead>
                  <tr>
                    <th>ALAN</th>
                    <th>YETENEK</th>
                    <th>MEKANİZMA</th>
                  </tr>
                </thead>
                <tbody>
                  {CAPABILITIES.map((g) =>
                    g.rows.map(([term, mech], i) => (
                      <tr key={term} className={styles[`grp_${g.accent}`]}>
                        {i === 0 && (
                          <td rowSpan={g.rows.length} className={styles.grpCell}>
                            {g.group}
                          </td>
                        )}
                        <td className={styles.termCell}>{term}</td>
                        <td className={styles.mechCell}>{mech}</td>
                      </tr>
                    )),
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </section>

        {/* ---------- PANEL ---------- */}
        <section id="panel" className={styles.sectionTint}>
          <div className={styles.shell}>
            <h2 className={styles.h2}>Kurulumdan sonra gördüğünüz ekran</h2>
            <p className={styles.sectionLede}>
              Filo sayaçları, canlı verim, en yoğun uç noktalar ve uyarı akışı — gerçek
              panelin birebir dili, sentetik veriyle sahnelenmiş.
            </p>
            <ShowcasePanel />
          </div>
        </section>

        {/* ---------- 5651 ---------- */}
        <section id="uyumluluk" className={styles.section}>
          <div className={styles.shell}>
            <h2 className={styles.h2}>Logun sonradan değişmediğini kanıtlarsınız</h2>
            <p className={styles.sectionLede}>
              Kayıt yazıldığı anda zincire eklenir. Her halka bir öncekinin özetini
              taşır; zincir saatlik köklerle mühürlenir, gün sonunda dış bir otoriteden
              zaman damgası alır.
            </p>

            <div className={styles.chain}>
              {CHAIN.map((c, i) => (
                <React.Fragment key={c.tag}>
                  <div className={`${styles.link} ${c.done ? styles.linkDone : ''}`}>
                    <span className={styles.linkTag}>{c.tag}</span>
                    <span className={styles.linkValue}>{c.value}</span>
                  </div>
                  {i < CHAIN.length - 1 && (
                    <span aria-hidden className={styles.linkArrow}>
                      ─▶
                    </span>
                  )}
                </React.Fragment>
              ))}
            </div>

            <div className={styles.notes}>
              <div>
                <h3 className={styles.noteH}>Delil paketi</h3>
                <p>
                  Tarih aralığıyla çıkarım, PII maskeleme ve <code>bazntmsctl verify</code>{' '}
                  ile çevrimdışı doğrulama — paketi teslim alan tarafın bazNTMS kurmasına
                  gerek yok.
                </p>
              </div>
              <div>
                <h3 className={styles.noteH}>ISO 27001</h3>
                <p>
                  Annex A kontrol haritası, risk defteri, SoA, iç denetim kayıtları ve
                  tek tıkla denetçi paketi. Zaman sapması alarmı (A.8.17).
                </p>
              </div>
            </div>
          </div>
        </section>

        {/* ---------- MİMARİ ---------- */}
        <section id="mimari" className={styles.sectionTint}>
          <div className={styles.shell}>
            <h2 className={styles.h2}>Nasıl çalışır?</h2>
            <p className={styles.sectionLede}>
              Hub stateless'tır; deploy replikaları arasında uyarı, poller ve yakalama
              rolleri bayraklarla ayrılır. Agent↔hub trafiği opsiyonel mTLS ile korunur.
            </p>
            <div className={styles.archScroll}>
              <Architecture />
            </div>
          </div>
        </section>

        {/* ---------- ÖLÇEK ---------- */}
        <section id="olcek" className={styles.section}>
          <div className={styles.shell}>
            <h2 className={styles.h2}>Tek makineden filoya, aynı binary</h2>
            <p className={styles.sectionLede}>
              Büyürken platform değiştirmezsiniz. Depo seçimi tek bayrakla değişir:{' '}
              <code>-db</code> bir dosya yolu alırsa SQLite, <code>postgres://</code> DSN
              alırsa PostgreSQL/TimescaleDB. Uygulama kodu ve arayüz aynı kalır.
            </p>

            <div className={styles.tableWrap}>
              <table className={styles.tuiTable}>
                <colgroup>
                  <col className={styles.colScaleKey} />
                  <col className={styles.colHalf} />
                  <col className={styles.colHalf} />
                </colgroup>
                <thead>
                  <tr>
                    <th></th>
                    <th>TEK DÜĞÜM</th>
                    <th>ÖLÇEK</th>
                  </tr>
                </thead>
                <tbody>
                  {SCALE_ROWS.map(([k, single, scaled]) => (
                    <tr key={k}>
                      <td className={styles.termCell}>{k}</td>
                      <td className={styles.mechCell}>{single}</td>
                      <td className={styles.mechCell}>{scaled}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <p className={styles.hint}>
              Ölçek hedefleri <code>bazntms-loadgen</code> + k6 ile doğrulanır (5 000
              agent @ 30 sn · p95 5 ms · ≥50 000 flow/sn kayıpsız) —{' '}
              <a href={`${REPO}/blob/main/docs/CAPACITY.md`} target="_blank" rel="noreferrer">
                docs/CAPACITY.md
              </a>
              . k8s olmadan denemek için <code>deploy/docker-compose.scale.yml</code>.
            </p>
          </div>
        </section>

        {/* ---------- KURULUM ---------- */}
        <section id="kurulum" className={styles.sectionTint}>
          <div className={styles.shell}>
            <h2 className={styles.h2}>Üç kurulum yolundan birini seçin</h2>
            <p className={styles.sectionLede}>
              Üçü birbirinin alternatifi — sıralı adım değil. Hepsi aynı binary'yi
              kullanır.
            </p>

            <div className={styles.steps}>
              {STEPS.map((s) => (
                <div key={s.badge} className={styles.step}>
                  <div className={styles.stepHead}>
                    <span className={styles.stepBadge}>{s.badge}</span>
                    <span className={styles.stepLabel}>{s.label}</span>
                    <CopyButton code={s.code} />
                  </div>
                  <pre className={styles.stepPre}>{s.code}</pre>
                </div>
              ))}
            </div>

            <p className={styles.hint}>
              Tüm bayraklar ve ortam değişkenleri için{' '}
              <a href={configUrl}>yapılandırma referansı</a>. Windows için MSI
              sihirbazı Npcap'i sessizce kurar.
            </p>
          </div>
        </section>

        {/* ---------- v1.3.0 ---------- */}
        <section id="surum" className={styles.section}>
          <div className={styles.shell}>
            <h2 className={styles.h2}>v1.3.0 hattında yeni</h2>
            <p className={styles.sectionLede}>
              v0.4.0'dan bu yana: süreç atfı artık Linux'ta eBPF, Windows'ta ETW —
              pcap/Npcap zorunlu değil. Derin toplama + L7 tüm kurulumlarda varsayılan
              açık.
            </p>
            <div className={styles.badges}>
              {NEW_IN.map((n) => (
                <span key={n} className={styles.badge}>
                  {n}
                </span>
              ))}
            </div>
          </div>
        </section>

        {/* ---------- ALT ---------- */}
        <section className={styles.bottom}>
          <div className={styles.shell}>
            <h2 className={styles.bottomTitle}>Ağınızı bugün görünür kılın.</h2>
            <p className={styles.bottomText}>
              Tek-node demo ile başlayın; aynı kurulumu TimescaleDB ve NATS arkasına
              taşıyarak filo ölçeğine çıkarın. Platform değişmez.
            </p>
            <div className={styles.heroCtas}>
              <Bracket to="/docs/installation" primary>
                KURULUM
              </Bracket>
              <Bracket href={apiUrl}>API REFERANSI</Bracket>
            </div>
            <p className={styles.bottomRisk}>
              MIT lisanslı · kendi altyapınızda çalışır · vendor lock-in yok
            </p>
          </div>
        </section>

        <ScreenBar active={active} onJump={jump} />
      </main>
    </Layout>
  );
}
