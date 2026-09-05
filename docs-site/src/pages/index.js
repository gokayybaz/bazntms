import React from 'react';
import Layout from '@theme/Layout';
import useBaseUrl from '@docusaurus/useBaseUrl';
import styles from './index.module.css';

/* Landing page — "ölçüm aleti" yönü: koyu zemin, mono ray, datasheet.
   Renk sözleşmesi ürünün kendi panelinden geliyor (bkz. depo kökündeki
   DESIGN.md → Colors): cyan=rx/görünürlük, violet=tx/toplama,
   rose=kritik/uyumluluk, emerald=sağlıklı/ölçek. Dekoratif renk yok. */

/* 12 yetenek, 4 tematik grup — ikon kartı yerine datasheet satırı olarak
   okunuyor: mono terim → birincil tanım → dim ayrıntı. */
const CAPABILITY_GROUPS = [
  {
    label: 'Görünürlük',
    accent: 'cyan',
    items: [
      {
        term: 'Trafik',
        desc: 'Paket bazlı ölçüm, yön tespiti ve protokol dağılımı',
        note: 'En yoğun uç noktalar GeoIP/ASN ile zenginleştirilip dünya haritasında hacme göre görselleştirilir.',
      },
      {
        term: 'L7 görünürlük',
        desc: 'Süreç bazlı TLS ClientHello SNI + HTTP Host çıkarımı',
        note: 'DNS sorgu/yanıt takibiyle birlikte — imza tabanlı DPI olmadan “hangi süreç, hangi alan adına” sorusunun cevabı.',
      },
      {
        term: 'Topoloji',
        desc: 'LLDP/CDP/ARP keşfi ve agent subnet bildirimleriyle otomatik harita',
        note: 'client → hub → cihaz → router → internet zinciri, gerçek trafik akışıyla birlikte tek bakışta.',
      },
    ],
  },
  {
    label: 'Toplama & entegrasyon',
    accent: 'violet',
    items: [
      {
        term: 'Agent filosu',
        desc: 'Enrollment, toplu telemetri ve offline disk kuyruğu ile 5.000 agent’a kadar ölçek',
        note: 'Agent↔hub trafiği karşılıklı TLS (mTLS) ile korunur, sertifikalar kendini yeniler.',
      },
      {
        term: 'Akış toplama',
        desc: 'NetFlow v5/v9, IPFIX ve sFlow v5 tek toplayıcıda',
        note: 'Şablon önbelleği ve örnekleme oranına göre otomatik ölçekleme; tamamı aynı akış tablosuna yazılır.',
      },
      {
        term: 'Cihazlar',
        desc: 'SNMPv3 arayüz/durum takibi, syslog alıcısı ve FortiGate REST API',
        note: 'VPN tünelleri, SD-WAN sağlık metrikleri, politika hit trendleri ve oturum izleme.',
      },
    ],
  },
  {
    label: 'Güvenlik & uyumluluk',
    accent: 'rose',
    items: [
      {
        term: 'Erişim',
        desc: 'Rol tabanlı erişim (admin / netops / analyst / viewer + site scope) ve OIDC SSO',
        note: 'Entegrasyon token’ları ve hash-zincirli append-only denetim kaydı.',
      },
      {
        term: 'SIEM / IOC',
        desc: 'IOC kara listesiyle L7 ve DNS eşleştirmesi',
        note: 'Olaylar CEF, LEEF, JSON veya düz syslog olarak Splunk HEC, ServiceNow, QRadar, ArcSight gibi hedeflere aktarılır.',
      },
      {
        term: 'Uyumluluk',
        desc: '5651 için imzalı log zinciri, ISO 27001 için denetim kayıtları',
        note: 'Risk defteri, SoA, iç denetim ve tek tıkla denetçi paketi.',
      },
    ],
  },
  {
    label: 'Ölçek & operasyon',
    accent: 'emerald',
    items: [
      {
        term: 'Depo',
        desc: 'PostgreSQL + TimescaleDB üzerinde katmanlı saklama — ham 7g · 1dk 90g · 1sa 2y',
        note: 'NATS JetStream ile esnek ingest hattı; k8s/Helm dağıtımı hazır.',
      },
      {
        term: 'Anomali',
        desc: 'İstatistiksel anomali tespiti — saatlik baseline + z-skoru',
        note: 'SLA, kapasite ve banding raporları; Teams, Slack, SMTP ve imzalı webhook bildirim kanalları.',
      },
      {
        term: 'Dağıtım',
        desc: 'Docker/Helm, deb · rpm · MSI · pkg installer’ları, imza doğrulamalı otomatik güncelleme',
        note: 'Elle bakımlı OpenAPI 3.1 şeması + gömülü /api/docs gezgini.',
      },
    ],
  },
];

/* 5651 zinciri — her halka bir öncekinin çıktısını mühürler, sıra bilgi taşır */
const CHAIN = [
  { tag: 'Olay', value: 'log kaydı' },
  { tag: 'Zincir', value: 'SHA-256 hash' },
  { tag: 'Saatlik', value: 'Merkle checkpoint' },
  { tag: 'Günlük', value: 'RFC 3161 damgası' },
  { tag: 'Manifest', value: 'ed25519 imza' },
  { tag: 'Saklama', value: 'WORM · 2 yıl', done: true },
];

const NEW_IN = [
  'Karşılıklı TLS (mTLS)',
  'NetFlow v9 / IPFIX',
  'sFlow v5',
  'L7 uygulama görünürlüğü',
  'DNS görünürlüğü',
  'Coğrafi trafik haritası',
  'SIEM / ITSM connector',
  'IOC eşleştirme',
  'OpenAPI 3.1 + /api/docs',
];

const STATS = [
  { value: '1.000', label: 'cihaz · 60 sn poll' },
  { value: '5.000', label: 'agent · 30 sn batch' },
  { value: '50K', label: 'flow/sn sürekli' },
  { value: '<1 sn', label: 'panel sorgusu p95' },
];

/* üç birbirini dışlayan kurulum yolu (sıralı adım değil) — harf rozetli */
const STEPS = [
  {
    badge: 'A',
    label: 'Tek-node demo',
    code: 'git clone https://github.com/gokayybaz/bazntms\ncd bazntms\ndocker compose -f deploy/docker-compose.yml up --build\n# → http://localhost:8080 · şifre: demo123',
  },
  {
    badge: 'B',
    label: 'Elle derleyin ve agent bağlayın',
    code: 'make                       # frontend + hub + agent + ctl\n./bazntmsctl setup         # interaktif sihirbaz → bazntms-hub.yml\n./bazntms-hub -config bazntms-hub.yml\n\n# agent bağlamak için (deb · rpm · MSI · pkg release sayfasında):\n./bazntms-agent -hub-url https://hub.example.com \\\n  -enroll-token <hub-loglarındaki-token>',
  },
  {
    badge: 'C',
    label: 'Ölçek mimarisi (k8s olmadan)',
    code: 'docker compose -f deploy/docker-compose.scale.yml up --build\n# 2 × ingest replikası + kontrolcü + nginx LB + JetStream\n# --scale hub-ingest=4 → yatay büyüt · dashboard: :8080 · agent API: :8081',
  },
];

/* --- hero: ortam rx/tx izi ---------------------------------------------
   Gerçek veri değil, ürünün 120 sn'lik örnekleme penceresini temsil eden
   sentetik bir sinyal. Harici grafik kütüphanesi yok (DESIGN.md kuralı);
   üretken/dekoratif olduğu için elle SVG yerine canvas. */

function Wire() {
  const canvasRef = React.useRef(null);
  const rxRef = React.useRef(null);
  const txRef = React.useRef(null);

  React.useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || !canvas.getContext) return undefined;
    const ctx = canvas.getContext('2d');

    const N = 120; // 1 örnek = 1 sn
    const rx = [];
    const tx = [];
    let seed = 20260905;
    const rnd = () => {
      seed = (seed * 1103515245 + 12345) & 0x7fffffff;
      return seed / 0x7fffffff;
    };
    for (let k = 0; k < N; k++) {
      rx.push(0.16);
      tx.push(0.05);
    }

    let burst = 0;
    const step = () => {
      if (burst > 0) burst--;
      else if (rnd() < 0.05) burst = 4 + Math.floor(rnd() * 11);

      const target = burst > 0 ? 0.52 + rnd() * 0.4 : 0.1 + rnd() * 0.22;
      const prev = rx[N - 1];
      let v = prev + (target - prev) * 0.26 + (rnd() - 0.5) * 0.06;
      v = Math.max(0.03, Math.min(0.97, v));
      rx.push(v);
      rx.shift();

      const pt = tx[N - 1];
      const tTarget = v * (0.2 + rnd() * 0.22);
      const t = pt + (tTarget - pt) * 0.3 + (rnd() - 0.5) * 0.03;
      tx.push(Math.max(0.02, Math.min(0.72, t)));
      tx.shift();
    };
    for (let w = 0; w < 400; w++) step();

    let W = 0;
    let H = 0;
    const resize = () => {
      const dpr = Math.min(window.devicePixelRatio || 1, 2);
      const r = canvas.getBoundingClientRect();
      W = Math.max(1, Math.round(r.width));
      H = Math.max(1, Math.round(r.height));
      canvas.width = Math.round(W * dpr);
      canvas.height = Math.round(H * dpr);
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    };

    // rx ve tx aynı ölçekten okunur — "tx her zaman rx'ten küçük" korunur
    const peak = () => {
      let p = 0.25;
      for (let i = 0; i < N; i++) if (rx[i] > p) p = rx[i];
      return p;
    };

    const trace = (series, axis, dir, span, scale, color, aNear, aFar) => {
      const x = (i) => (i / (N - 1)) * W;
      const off = (i) => (series[i] / scale) * span * 0.94;
      const y = (i) => axis + dir * off(i);

      // gradyan eğrinin gerçek tepesine bağlanır — düşük sinyalde de görünür
      let maxOff = 8;
      for (let m = 0; m < N; m++) if (off(m) > maxOff) maxOff = off(m);
      const far = axis + dir * maxOff;

      const g = ctx.createLinearGradient(0, dir < 0 ? far : axis, 0, dir < 0 ? axis : far);
      g.addColorStop(0, dir < 0 ? aNear : aFar);
      g.addColorStop(1, dir < 0 ? aFar : aNear);

      ctx.beginPath();
      ctx.moveTo(0, axis);
      for (let i = 0; i < N; i++) ctx.lineTo(x(i), y(i));
      ctx.lineTo(W, axis);
      ctx.closePath();
      ctx.fillStyle = g;
      ctx.fill();

      ctx.beginPath();
      for (let j = 0; j < N; j++) {
        if (j === 0) ctx.moveTo(x(j), y(j));
        else ctx.lineTo(x(j), y(j));
      }
      ctx.strokeStyle = color;
      ctx.lineWidth = 1.25;
      ctx.lineJoin = 'round';
      ctx.stroke();

      ctx.beginPath();
      ctx.arc(x(N - 1), y(N - 1), 2.4, 0, Math.PI * 2);
      ctx.fillStyle = color;
      ctx.fill();
    };

    const draw = () => {
      ctx.clearRect(0, 0, W, H);
      const axis = Math.round(H * 0.62) + 0.5;
      const scale = peak();

      ctx.strokeStyle = 'rgba(148,163,184,0.055)';
      ctx.lineWidth = 1;
      for (let t = 0; t <= N; t += 20) {
        const gx = Math.round((t / (N - 1)) * W) + 0.5;
        ctx.beginPath();
        ctx.moveTo(gx, 0);
        ctx.lineTo(gx, H);
        ctx.stroke();
      }

      ctx.strokeStyle = 'rgba(148,163,184,0.16)';
      ctx.beginPath();
      ctx.moveTo(0, axis);
      ctx.lineTo(W, axis);
      ctx.stroke();

      trace(rx, axis, -1, axis, scale, '#22d3ee', 'rgba(34,211,238,0.26)', 'rgba(34,211,238,0.02)');
      trace(tx, axis, 1, H - axis, scale, '#a78bfa', 'rgba(167,139,250,0.30)', 'rgba(167,139,250,0.02)');
    };

    const readout = () => {
      if (rxRef.current) rxRef.current.textContent = (rx[N - 1] * 11.6).toFixed(2);
      if (txRef.current) txRef.current.textContent = (tx[N - 1] * 11.6).toFixed(2);
    };

    resize();
    draw();
    readout();

    const ro = typeof ResizeObserver !== 'undefined'
      ? new ResizeObserver(() => { resize(); draw(); })
      : null;
    if (ro) ro.observe(canvas);
    else window.addEventListener('resize', resize);

    const reduce = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    let raf = 0;
    if (!reduce) {
      let last = 0;
      const loop = (ts) => {
        if (ts - last > 220) {
          last = ts;
          step();
          draw();
          readout();
        }
        raf = requestAnimationFrame(loop);
      };
      raf = requestAnimationFrame(loop);
    }

    return () => {
      if (raf) cancelAnimationFrame(raf);
      if (ro) ro.disconnect();
      else window.removeEventListener('resize', resize);
    };
  }, []);

  return (
    <>
      <div className={styles.shell}>
        <div className={styles.wireHead}>
          <span className={styles.tag}>Canlı verim · 120 sn pencere</span>
          <div className={styles.readout}>
            <span className={styles.kRx}>
              <i className={`${styles.swatch} ${styles.swRx}`} />
              indirilen <b ref={rxRef}>0.00</b> Mb/s
            </span>
            <span className={styles.kTx}>
              <i className={`${styles.swatch} ${styles.swTx}`} />
              gönderilen <b ref={txRef}>0.00</b> Mb/s
            </span>
          </div>
        </div>
      </div>
      <div className={styles.wireFrame}>
        <div className={styles.shell}>
          <canvas ref={canvasRef} className={styles.wireCanvas} aria-hidden="true" />
        </div>
      </div>
      <div className={`${styles.shell} ${styles.wireFoot}`}>
        <span className={styles.tag}>Temsilî akış — gerçek veri değil, örnekleme penceresini gösterir</span>
        <span className={styles.tag}>WebSocket · 1 sn / örnek</span>
      </div>
    </>
  );
}

/* --- temsilî panel: ürünün kendi gösterge panelinin küçük hali --------
   Gerçek veri yok — sentetik, döngüsel bir sahne. Grafikler elle SVG
   (DESIGN.md: harici grafik kütüphanesi yok); stat kutuları panelin Triad
   kalıbını (Label → Data → Caption) ve sol kenar vurgu rengini birebir
   izler. Başlangıç durumu deterministik üretilir — SSR ile hidrasyon
   arasında fark çıkmasın. */

const PANEL_ENDPOINTS = [
  { host: 'cdn.example.net', meta: 'AS13335 · US', base: 0.92 },
  { host: 'update.example.com', meta: 'AS16509 · IE', base: 0.64 },
  { host: 'pkg.example.org', meta: 'AS24940 · DE', base: 0.47 },
  { host: 'api.example.io', meta: 'AS15169 · NL', base: 0.31 },
  { host: 'mail.example.net', meta: 'AS8075 · TR', base: 0.19 },
];

const PANEL_FEED = [
  { kind: 'proc', text: 'yeni süreç · agent-ofis-3 : curl' },
  { kind: 'warn', text: 'bant genişliği zirvesi · agent-dc1-07' },
  { kind: 'crit', text: 'IOC eşleşmesi · agent-sube-a' },
  { kind: 'proc', text: 'yeni hedef · agent-dc1-07 : pkg.example.org' },
  { kind: 'warn', text: 'şüpheli port · agent-ofis-3 : 4444/tcp' },
];

function makeRng(seed) {
  let s = seed;
  return () => {
    s = (s * 1103515245 + 12345) & 0x7fffffff;
    return s / 0x7fffffff;
  };
}

function initialPanel() {
  const rnd = makeRng(4242);
  const spark = [];
  let v = 0.42;
  for (let i = 0; i < 48; i++) {
    v = Math.max(0.1, Math.min(0.94, v + (rnd() - 0.5) * 0.2));
    spark.push(v);
  }
  return {
    spark,
    pps: 3.2,
    eps: 4.8,
    online: 124,
    alerts: 2,
    endpoints: PANEL_ENDPOINTS.map((e) => ({ ...e, v: e.base })),
    head: 0,
  };
}

function LivePanel() {
  const [snap, setSnap] = React.useState(initialPanel);

  React.useEffect(() => {
    const reduce = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (reduce) return undefined; // tek kare kalır

    const rnd = makeRng(97531);
    let t = 0;
    const id = setInterval(() => {
      t += 1;
      setSnap((prev) => {
        const spark = prev.spark.slice(1);
        const last = prev.spark[prev.spark.length - 1];
        const next = Math.max(0.1, Math.min(0.94, last + (rnd() - 0.5) * 0.26));
        spark.push(next);

        return {
          spark,
          pps: Math.max(0.6, Math.min(9.9, prev.pps + (rnd() - 0.5) * 0.7)),
          eps: Math.max(0.4, Math.min(19, prev.eps + (rnd() - 0.5) * 1.6)),
          online: Math.max(118, Math.min(140, prev.online + (rnd() < 0.86 ? 0 : rnd() < 0.5 ? -1 : 1))),
          // taban 1: akışta görünen uyarı satırlarıyla çelişmesin
          alerts: Math.max(1, Math.min(6, prev.alerts + (rnd() < 0.93 ? 0 : rnd() < 0.5 ? -1 : 1))),
          endpoints: prev.endpoints.map((e) => ({
            ...e,
            v: Math.max(0.08, Math.min(1, e.v + (rnd() - 0.5) * 0.13)),
          })),
          head: t % PANEL_FEED.length,
        };
      });
    }, 1000);
    return () => clearInterval(id);
  }, []);

  const W = 320;
  const H = 84;
  const pt = (v, i, n) => [(i / (n - 1)) * W, H - v * (H - 8) - 4];
  const line = snap.spark.map((v, i) => pt(v, i, snap.spark.length).join(',')).join(' ');
  const area = `M0,${H} L${snap.spark
    .map((v, i) => pt(v, i, snap.spark.length).join(','))
    .join(' L')} L${W},${H} Z`;
  const feed = Array.from({ length: 3 }, (_, i) => PANEL_FEED[(snap.head + i) % PANEL_FEED.length]);

  const stats = [
    { label: 'Aktif agent', value: `${snap.online} / 140`, caption: 'filo toplamı', accent: 'sEmerald' },
    { label: 'Paket hızı', value: `${snap.pps.toFixed(1)}K pps`, caption: 'anlık', accent: 'sCyan' },
    { label: 'Olay hızı', value: `${snap.eps.toFixed(1)} / sn`, caption: 'son 60 sn', accent: 'sViolet' },
    { label: 'Açık uyarı', value: `${snap.alerts}`, caption: 'kritik dahil', accent: 'sAmber' },
  ];

  return (
    <div className={styles.panel}>
      <div className={styles.panelBar}>
        <span className={styles.tag}>bazntms.local · Genel Bakış</span>
        <span className={styles.panelLive}>
          <i className={styles.pulse} /> WS: CANLI
        </span>
      </div>

      <div className={styles.panelStats}>
        {stats.map((s) => (
          <div key={s.label} className={`${styles.tile} ${styles[s.accent]}`}>
            <span className={styles.tileLabel}>{s.label}</span>
            <b className={styles.tileValue}>{s.value}</b>
            <span className={styles.tileCaption}>{s.caption}</span>
          </div>
        ))}
      </div>

      <div className={styles.panelBody}>
        <div className={styles.panelCard}>
          <div className={styles.panelCardHead}>
            <span className={styles.tag}>Verim · 48 sn</span>
            <span className={styles.panelUnit}>Mb/s</span>
          </div>
          <svg className={styles.spark} viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" aria-hidden="true">
            <path d={area} fill="rgba(34,211,238,0.14)" />
            <polyline points={line} fill="none" stroke="#22d3ee" strokeWidth="1.5" strokeLinejoin="round" />
          </svg>
        </div>

        <div className={styles.panelCard}>
          <div className={styles.panelCardHead}>
            <span className={styles.tag}>En yoğun uç noktalar</span>
          </div>
          <ul className={styles.epList}>
            {snap.endpoints.map((e) => (
              <li key={e.host}>
                <div className={styles.epRow}>
                  <span className={styles.epHost}>{e.host}</span>
                  <span className={styles.epMeta}>{e.meta}</span>
                </div>
                <div className={styles.epBar}>
                  <i style={{ width: `${Math.round(e.v * 100)}%` }} />
                </div>
              </li>
            ))}
          </ul>
        </div>
      </div>

      <ul className={styles.feed}>
        {feed.map((f, i) => (
          <li key={`${f.text}-${i}`} className={styles[f.kind]}>
            <span className={styles.feedDot} />
            {f.text}
          </li>
        ))}
      </ul>
    </div>
  );
}

/* --- mimari: uçlar → hub → depo -------------------------------------- */

function Architecture() {
  const agents = [
    { y: 64, name: 'agent · ofis-a' },
    { y: 150, name: 'agent · dc1' },
    { y: 236, name: 'agent · şube-3' },
  ];
  const devices = [
    { y: 64, name: 'firewall' },
    { y: 150, name: 'core-switch' },
    { y: 236, name: 'router' },
  ];
  return (
    <svg
      className={styles.archSvg}
      viewBox="0 0 960 400"
      role="img"
      aria-label="bazNTMS mimarisi: agent'lar ve ağ cihazları hub'a telemetri gönderir; hub PostgreSQL/TimescaleDB ve NATS JetStream üzerine yazar"
    >
      <g className={styles.archFlow}>
        <path d="M232 87 C 320 87, 340 160, 392 176" />
        <path d="M232 173 L 392 190" />
        <path d="M232 259 C 320 259, 340 220, 392 204" />
        <path d="M728 87 C 640 87, 620 160, 568 176" />
        <path d="M728 173 L 568 190" />
        <path d="M728 259 C 640 259, 620 220, 568 204" />
        <path d="M436 250 L 436 306" />
        <path d="M524 250 L 524 306" />
      </g>

      <text x="40" y="36" className={styles.archGroup}>UÇLAR</text>
      {agents.map((a) => (
        <g key={a.name}>
          <rect x="40" y={a.y} width="192" height="46" rx="3" className={styles.archNode} />
          <circle cx="64" cy={a.y + 23} r="3.5" className={styles.archDotRx} />
          <text x="80" y={a.y + 28} className={styles.archText}>{a.name}</text>
        </g>
      ))}

      <text x="920" y="36" className={styles.archGroup} textAnchor="end">AĞ CİHAZLARI</text>
      {devices.map((d) => (
        <g key={d.name}>
          <rect x="728" y={d.y} width="192" height="46" rx="3" className={styles.archNode} />
          <circle cx="752" cy={d.y + 23} r="3.5" className={styles.archDotTx} />
          <text x="768" y={d.y + 28} className={styles.archText}>{d.name}</text>
        </g>
      ))}

      <rect x="392" y="140" width="176" height="110" rx="3" className={styles.archHub} />
      <text x="480" y="184" textAnchor="middle" className={styles.archHubText}>bazntms-hub</text>
      <text x="480" y="206" textAnchor="middle" className={styles.archSub}>ingest · RBAC</text>
      <text x="480" y="224" textAnchor="middle" className={styles.archSub}>audit · uyarı</text>

      <rect x="336" y="306" width="200" height="52" rx="3" className={styles.archNode} />
      <text x="436" y="330" textAnchor="middle" className={styles.archText}>PostgreSQL + TimescaleDB</text>
      <text x="436" y="346" textAnchor="middle" className={styles.archSub}>hypertable · cagg · retention</text>

      <rect x="556" y="306" width="200" height="52" rx="3" className={styles.archNode} />
      <text x="656" y="330" textAnchor="middle" className={styles.archText}>NATS JetStream</text>
      <text x="656" y="346" textAnchor="middle" className={styles.archSub}>ingest → processor</text>

      <text x="292" y="128" className={styles.archLabel}>telemetri ↑</text>
      <text x="668" y="128" className={styles.archLabel}>SNMP · NetFlow · Syslog</text>
    </svg>
  );
}

/* --- kopyala düğmesi --- */

function CopyButton({ code }) {
  const [state, setState] = React.useState('idle'); // idle | done | error
  return (
    <button
      type="button"
      className={`${styles.copy} ${state === 'error' ? styles.copyError : ''}`}
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(code);
          setState('done');
        } catch {
          // pano erişimi kurumsal tarayıcı politikasıyla engellenmiş olabilir
          setState('error');
        } finally {
          setTimeout(() => setState('idle'), 1800);
        }
      }}
    >
      {state === 'done' ? '✓ kopyalandı' : state === 'error' ? '✗ kopyalanamadı' : 'kopyala'}
    </button>
  );
}

/* --- bölüm iskeleti: mono ray + gövde --- */

function Block({ rail, children }) {
  return (
    <div className={`${styles.shell} ${styles.block}`}>
      <div className={styles.blockRail}>
        <span className={styles.tag}>{rail}</span>
        <span className={styles.railLine} />
      </div>
      <div className={styles.blockBody}>{children}</div>
    </div>
  );
}

export default function Home() {
  const docsUrl = useBaseUrl('/docs/installation');
  const apiUrl = useBaseUrl('/docs/reference/api');
  const configUrl = useBaseUrl('/docs/reference/configuration');
  const upgradeUrl = useBaseUrl('/docs/reference/upgrading');

  return (
    <Layout
      title="Ağ Trafiği İzleme Platformu"
      description="Hub + agent + cihaz entegrasyonları: canlı trafik izleme, TimescaleDB + NATS ölçek altyapısı, RBAC/SSO, imza doğrulamalı agent güncellemesi."
    >
      <main className={`${styles.page} landing-root`}>
        {/* HERO */}
        <div className={`${styles.shell} ${styles.hero}`}>
          <span className={`${styles.tag} ${styles.heroEyebrow}`}>
            MIT · Go + React · sizin altyapınızda
          </span>
          <h1 className={styles.title}>
            Paketten <em>imzalı kayda</em> kadar tek bir platform.
          </h1>
          <p className={styles.tagline}>
            bazNTMS ağı paket seviyesinde izler, akışları ve cihaz telemetrisini tek
            toplayıcıda birleştirir, 5651’e uygun imzalı loglar üretir. Tek makinede
            başlayan kurulum <b>5.000 agent</b> ölçeğine aynı binary’yle çıkar.
          </p>
          <div className={styles.ctas}>
            <a className={styles.btnPrimary} href={docsUrl}>
              Kurulum dokümanı <span className={styles.arrow}>→</span>
            </a>
            <a
              className={styles.btnGhost}
              href="https://github.com/gokayybaz/bazntms"
              target="_blank"
              rel="noreferrer"
            >
              GitHub <span className={styles.arrow}>↗</span>
            </a>
          </div>
        </div>

        <Wire />

        {/* SAYILAR */}
        <section className={styles.section}>
          <Block rail="Hedefler">
            <div className={styles.stats}>
              {STATS.map((s) => (
                <div key={s.label} className={styles.stat}>
                  <b>{s.value}</b>
                  <span>{s.label}</span>
                </div>
              ))}
            </div>
            <p className={styles.hint}>
              Ölçek mimarisi tasarım hedefleri —{' '}
              <a href="https://github.com/gokayybaz/bazntms/tree/main/loadtest" target="_blank" rel="noreferrer">
                <code>bazntms-loadgen</code> ve k6
              </a>{' '}
              ile doğrulanır.
            </p>
          </Block>
        </section>

        {/* PANEL */}
        <section className={styles.section} id="panel">
          <Block rail="Panel">
            <h2>Panelin kendisi</h2>
            <p className={styles.lede}>
              Kurulumdan sonra karşınıza çıkan görünümün küçük bir hali — filo
              sayaçları, canlı verim, en yoğun uç noktalar ve uyarı akışı.
            </p>
            <LivePanel />
            <p className={styles.hint}>
              Temsilî panel — canlı bir demo değil, sentetik veriyle sahnelenmiş bir
              görünüm. Gerçek panel için <a href={docsUrl}>kurulum dokümanına</a> bakın.
            </p>
          </Block>
        </section>

        {/* YETENEKLER */}
        <section className={styles.section} id="yetenekler">
          <Block rail="Yetenekler">
            <h2>Uçtan uca görünürlük</h2>
            <p className={styles.lede}>
              Dört alan, on iki yetenek. Her rengin sabit bir okunuşu var — panelde
              gördüğünüz anlamın aynısı.
            </p>

            {CAPABILITY_GROUPS.map((group) => (
              <div key={group.label} className={styles.capGroup}>
                <div className={styles.capHead}>
                  <span className={`${styles.capDot} ${styles[group.accent]}`} />
                  <span className={styles.tag}>{group.label}</span>
                </div>
                <dl className={styles.sheet}>
                  {group.items.map((it) => (
                    <div key={it.term} className={styles.row}>
                      <dt>{it.term}</dt>
                      <dd>
                        {it.desc}
                        <small>{it.note}</small>
                      </dd>
                    </div>
                  ))}
                </dl>
              </div>
            ))}
          </Block>
        </section>

        {/* 5651 */}
        <section className={styles.section} id="uyumluluk">
          <Block rail="5651">
            <h2>Logun sonradan değişmediğini kanıtlayabilirsiniz</h2>
            <p className={styles.lede}>
              Kayıtlar yazıldığı anda zincire eklenir. Her halka bir öncekinin
              özetini taşır; zincir saatlik köklerle mühürlenir, gün sonunda dış bir
              otoriteden zaman damgası alır.
            </p>
            <div className={styles.chain}>
              {CHAIN.map((c) => (
                <div key={c.tag} className={styles.link}>
                  <span className={styles.tag}>{c.tag}</span>
                  <b className={c.done ? styles.chainDone : undefined}>{c.value}</b>
                </div>
              ))}
            </div>
            <div className={styles.note}>
              <div>
                <h3>Delil paketi</h3>
                <p>
                  Tarih aralığıyla çıkarım, PII maskeleme ve <code>bazntmsctl verify</code>{' '}
                  ile çevrimdışı doğrulama — paketi teslim alan tarafın bazNTMS
                  kurmasına gerek yok.
                </p>
              </div>
              <div>
                <h3>ISO 27001</h3>
                <p>
                  Annex A kontrol haritası, risk defteri, SoA, iç denetim kayıtları ve
                  tek tıkla denetçi paketi.
                </p>
              </div>
            </div>
          </Block>
        </section>

        {/* MİMARİ */}
        <section className={styles.section} id="mimari">
          <Block rail="Mimari">
            <h2>Nasıl çalışır?</h2>
            <p className={styles.lede}>
              Hub stateless’tır; deploy replikaları arasında uyarı, poller ve yakalama
              rolleri bayraklarla ayrılır.
            </p>
            <div className={styles.svgScroll}>
              <Architecture />
            </div>
            <p className={styles.hint}>
              Ayrıntılar için <a href={upgradeUrl}>operasyon dokümanlarına</a> göz atın.
            </p>
          </Block>
        </section>

        {/* KURULUM */}
        <section className={styles.section} id="kurulum">
          <Block rail="Kurulum">
            <h2>Üç kurulum yolundan birini seçin</h2>
            <p className={styles.lede}>
              Üçü birbirinin alternatifi — sıralı adım değil. Hepsi aynı binary’yi kullanır.
            </p>
            <div className={styles.steps}>
              {STEPS.map((s) => (
                <div key={s.badge} className={styles.step}>
                  <div className={styles.stepNo}>{s.badge}</div>
                  <div className={styles.terminal}>
                    <div className={styles.terminalHead}>
                      <span className={styles.tag}>{s.label}</span>
                      <CopyButton code={s.code} />
                    </div>
                    <pre>{s.code}</pre>
                  </div>
                </div>
              ))}
            </div>
            <p className={styles.hint}>
              Tüm yapılandırma seçenekleri için <a href={configUrl}>yapılandırma referansına</a> bakın.
            </p>
          </Block>
        </section>

        {/* v0.3.0 */}
        <section className={styles.section}>
          <Block rail="v0.3.0">
            <h2>Bu sürümde yeni</h2>
            <ul className={styles.newList}>
              {NEW_IN.map((n) => (
                <li key={n}>{n}</li>
              ))}
            </ul>
          </Block>
        </section>

        {/* ALT CTA */}
        <section className={styles.bottom}>
          <div className={styles.shell}>
            <h2 className={styles.bottomTitle}>Ağınızı bugün görünür kılın.</h2>
            <p className={styles.bottomText}>
              Tek-node demo ile başlayın; aynı kurulumu TimescaleDB ve NATS arkasına
              taşıyarak filo ölçeğine çıkarın.
            </p>
            <div className={styles.ctas}>
              <a className={styles.btnPrimary} href={docsUrl}>
                Kurulum dokümanı <span className={styles.arrow}>→</span>
              </a>
              <a className={styles.btnGhost} href={apiUrl}>
                API referansı
              </a>
            </div>
            <p className={`${styles.tag} ${styles.bottomRisk}`}>
              MIT lisanslı · kendi altyapınızda çalışır · vendor lock-in yok
            </p>
          </div>
        </section>
      </main>
    </Layout>
  );
}
