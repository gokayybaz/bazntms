import React from 'react';
import Layout from '@theme/Layout';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import useBaseUrl from '@docusaurus/useBaseUrl';
import styles from './index.module.css';

/* --- SVG ikonlar (24px, stroke tabanlı) --- */

const IconTraffic = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
    <path d="M2 12h4l3-8 4 16 3-8h6" />
  </svg>
);

const IconFleet = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
    <rect x="9" y="9" width="6" height="6" rx="1.5" />
    <path d="M12 2v4M12 18v4M2 12h4M18 12h4M4.9 4.9l2.8 2.8M16.3 16.3l2.8 2.8M19.1 4.9l-2.8 2.8M7.7 16.3l-2.8 2.8" />
  </svg>
);

const IconDevices = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
    <rect x="2" y="4" width="20" height="7" rx="1.5" />
    <rect x="2" y="13" width="20" height="7" rx="1.5" />
    <path d="M6 7.5h.01M6 16.5h.01M10 7.5h.01M10 16.5h.01" />
  </svg>
);

const IconScale = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
    <ellipse cx="12" cy="5.5" rx="8" ry="3" />
    <path d="M4 5.5v6c0 1.66 3.58 3 8 3s8-1.34 8-3v-6" />
    <path d="M4 11.5v6c0 1.66 3.58 3 8 3s8-1.34 8-3v-6" />
  </svg>
);

const IconShield = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
    <path d="M12 2l8 3.5v5.2c0 5-3.4 9.3-8 11.3-4.6-2-8-6.3-8-11.3V5.5L12 2z" />
    <path d="M9 12l2 2 4-4.5" />
  </svg>
);

const IconSpark = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
    <path d="M3 20h18" />
    <path d="M4 16l4-5 3.5 3L16 7l4 4.5" />
    <circle cx="16" cy="7" r="1.4" />
  </svg>
);

const IconTopo = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="5" cy="5" r="2.2" />
    <circle cx="19" cy="5" r="2.2" />
    <circle cx="12" cy="12" r="2.6" />
    <circle cx="5" cy="19" r="2.2" />
    <circle cx="19" cy="19" r="2.2" />
    <path d="M6.6 6.6 10.2 10.2M17.4 6.6 13.8 10.2M6.6 17.4 10.2 13.8M17.4 17.4 13.8 13.8" />
  </svg>
);

const IconRocket = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
    <path d="M5 15c-1.5 1.5-2 5-2 5s3.5-.5 5-2c.8-.8.8-2.2 0-3-.8-.8-2.2-.8-3 0z" />
    <path d="M9 13 6.5 10.5C9 8 12 6.5 16 6c2-.2 3 .8 2.8 2.8-.5 4-2 7-4.5 9.5L12 15.5" />
    <path d="M14 10a1.5 1.5 0 1 0 .01 0z" />
  </svg>
);

const IconStamp = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
    <path d="M12 2l7.5 3.3v5c0 4.7-3.2 8.8-7.5 10.7-4.3-1.9-7.5-6-7.5-10.7v-5L12 2z" />
    <path d="M8.5 12.2l2.4 2.4 4.6-5" />
  </svg>
);

const IconLayers = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
    <path d="M12 2 2 7l10 5 10-5-10-5z" />
    <path d="M2 12l10 5 10-5" />
    <path d="M2 17l10 5 10-5" />
  </svg>
);

const IconFunnel = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
    <path d="M3 4h18l-7.2 8.4v6.1l-3.6 1.8v-7.9L3 4z" />
  </svg>
);

const IconLink = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
    <path d="M8 12.5l-2.6 2.6a3.5 3.5 0 0 0 5 5L13 17.5" />
    <path d="M16 11.5l2.6-2.6a3.5 3.5 0 0 0-5-5L11 6.5" />
    <path d="M9.5 14.5l5-5" />
  </svg>
);

const FEATURES = [
  {
    icon: <IconTraffic />,
    title: 'Canlı Trafik İzleme',
    text: 'Paket bazlı ölçüm, yön tespiti ve protokol dağılımı; en yoğun uç noktalar GeoIP/ASN ile zenginleştirilip dünya haritasında hacme göre görselleştirilir.',
  },
  {
    icon: <IconLayers />,
    title: 'Derin Uygulama Görünürlüğü',
    text: 'Süreç bazlı TLS ClientHello SNI + HTTP Host çıkarımı ve DNS sorgu/yanıt takibi — imza tabanlı DPI olmadan “hangi süreç, hangi alan adına” sorusunun cevabı.',
  },
  {
    icon: <IconFleet />,
    title: 'Merkezi Agent Filosu',
    text: 'Enrollment, toplu telemetri ve offline disk kuyruğu ile 5.000 agent’a kadar ölçek. Agent↔hub trafiği karşılıklı TLS (mTLS) ile korunur, sertifikalar kendini yeniler.',
  },
  {
    icon: <IconFunnel />,
    title: 'Çok Protokollü Akış Toplama',
    text: 'NetFlow v5/v9, IPFIX ve sFlow v5 tek toplayıcıda — şablon önbelleği ve örnekleme oranına göre otomatik ölçekleme, tamamı aynı akış tablosuna yazılır.',
  },
  {
    icon: <IconDevices />,
    title: 'Cihaz Entegrasyonları',
    text: 'SNMPv3 arayüz/durum takibi ve syslog alıcısının üzerine FortiGate REST API: VPN tünelleri, SD-WAN sağlık metrikleri, politika hit trendleri ve oturum izleme.',
  },
  {
    icon: <IconTopo />,
    title: 'Canlı Ağ Topolojisi',
    text: 'LLDP/CDP/ARP keşfi ve agent subnet bildirimleriyle otomatik harita: client → hub → cihaz → router → internet zinciri gerçek trafik akışıyla birlikte tek bakışta.',
  },
  {
    icon: <IconScale />,
    title: 'Ölçek Altyapısı',
    text: 'PostgreSQL + TimescaleDB üzerinde katmanlı saklama (ham 7 gün, 1 dk 90 gün, 1 sa 2 yıl) ve NATS JetStream ile esnek ingest hattı; k8s/Helm dağıtımı hazır.',
  },
  {
    icon: <IconShield />,
    title: 'Güvenlik ve Kimlik',
    text: 'Rol tabanlı erişim (admin/netops/analyst/viewer + site scope), OIDC SSO, entegrasyon token’ları ve hash-zincirli append-only denetim kaydı.',
  },
  {
    icon: <IconLink />,
    title: 'Tehdit İstihbaratı ve SIEM',
    text: 'IOC kara listesiyle L7/DNS eşleştirmesi; olaylar CEF, LEEF, JSON veya düz syslog olarak Splunk HEC, ServiceNow, QRadar, ArcSight gibi hedeflere aktarılır.',
  },
  {
    icon: <IconSpark />,
    title: 'Akıllı Operasyon',
    text: 'İstatistiksel anomali tespiti (saatlik baseline + z-skoru), SLA/kapasite/banding raporları ve Teams, Slack, SMTP, imzalı webhook bildirim kanalları.',
  },
  {
    icon: <IconRocket />,
    title: 'Dağıtım, API ve Operasyon',
    text: 'Docker/Helm ile k8s dağıtımı; deb/rpm/MSI/pkg installer’lar; imza doğrulamalı otomatik güncelleme; elle bakımlı OpenAPI 3.1 şeması + gömülü /api/docs gezgini.',
  },
  {
    icon: <IconStamp />,
    title: '5651 Uyumluluk ve ISO 27001',
    text: 'Loglar Merkle checkpoint + RFC 3161 zaman damgası ile imzalanır, WORM depoda 2 yıl saklanır; risk defteri, SoA, iç denetim ve tek tıkla denetçi paketi.',
  },
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

const STEPS = [
  {
    label: 'Tek-node demo',
    code: 'git clone https://github.com/gokayybaz/bazntms\ncd bazntms\ndocker compose -f deploy/docker-compose.yml up --build\n# → http://localhost:8080 · şifre: demo123',
  },
  {
    label: 'Hub’ı derleyip yapılandırın',
    code: 'make                       # frontend + hub + agent + ctl\n./bazntmsctl setup         # interaktif sihirbaz → bazntms-hub.yml\n./bazntms-hub -config bazntms-hub.yml',
  },
  {
    label: 'Agent’ları bağlayın',
    code: './bazntms-agent -hub-url https://hub.example.com \\\n  -enroll-token <hub-loglarındaki-token>\n# deb · rpm · MSI · pkg paketleri release sayfasında',
  },
  {
    label: 'Ölçek mimarisi (k8s olmadan)',
    code: 'docker compose -f deploy/docker-compose.scale.yml up --build\n# 2 × ingest replikası + kontrolcü + nginx LB + JetStream\n# --scale hub-ingest=4 → yatay büyüt · dashboard: :8080 · agent API: :8081',
  },
];

const STATS = [
  { value: '1.000', label: 'cihaz · 60 sn poll' },
  { value: '5.000', label: 'agent · 30 sn batch' },
  { value: '50K', label: 'flow/sn sürekli' },
  { value: '<1 sn', label: 'panel sorgusu p95' },
];

/* --- Animasyonlu mimari diyagramı --- */

function Architecture() {
  return (
    <svg
      className={styles.archSvg}
      viewBox="0 0 960 470"
      role="img"
      aria-label="bazNTMS mimarisi: agentlar ve ağ cihazları hub'a telemetri gönderir; hub TimescaleDB ve NATS üzerine yazar"
    >
      <defs>
        <linearGradient id="hubGlow" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#164e63" />
          <stop offset="100%" stopColor="#083344" />
        </linearGradient>
      </defs>

      {/* bağlantılar — akan paketler */}
      <g stroke="#155e75" strokeWidth="1.6" fill="none">
        <path d="M215 105 C 300 105, 330 190, 388 210" strokeDasharray="5 7" className={styles.flow} />
        <path d="M215 225 L 388 225" strokeDasharray="5 7" className={styles.flow} />
        <path d="M215 345 C 300 345, 330 260, 388 240" strokeDasharray="5 7" className={styles.flow} />
        <path d="M745 105 C 660 105, 630 190, 572 210" strokeDasharray="5 7" className={styles.flowSlow} />
        <path d="M745 225 L 572 225" strokeDasharray="5 7" className={styles.flowSlow} />
        <path d="M745 345 C 660 345, 630 260, 572 240" strokeDasharray="5 7" className={styles.flowSlow} />
        <path d="M440 296 L 440 356" strokeDasharray="4 6" className={styles.flow} />
        <path d="M520 296 L 520 356" strokeDasharray="4 6" className={styles.flowSlow} />
      </g>

      {/* uçlar */}
      {[
        { y: 80, name: 'agent · ofis-a' },
        { y: 200, name: 'agent · dc1' },
        { y: 320, name: 'agent · şube-3' },
      ].map((a) => (
        <g key={a.name}>
          <rect x="40" y={a.y} width="175" height="46" rx="9" className={styles.node} />
          <circle cx="64" cy={a.y + 23} r="4" className={styles.dot} />
          <text x="80" y={a.y + 27} className={styles.nodeText}>
            {a.name}
          </text>
        </g>
      ))}
      <text x="40" y="42" className={styles.groupText}>
        UÇLAR
      </text>

      {/* cihazlar */}
      {[
        { y: 80, name: 'firewall' },
        { y: 200, name: 'core-switch' },
        { y: 320, name: 'router' },
      ].map((d) => (
        <g key={d.name}>
          <rect x="745" y={d.y} width="175" height="46" rx="9" className={styles.node} />
          <circle cx="769" cy={d.y + 23} r="4" className={styles.dotAmber} />
          <text x="785" y={d.y + 27} className={styles.nodeText}>
            {d.name}
          </text>
        </g>
      ))}
      <text x="745" y="42" className={styles.groupText}>
        AĞ CİHAZLARI
      </text>

      {/* hub */}
      <rect x="388" y="168" width="184" height="114" rx="12" fill="url(#hubGlow)" stroke="#22d3ee" strokeWidth="1.6" />
      <circle cx="480" cy="200" r="5" className={styles.pulse} />
      <text x="480" y="234" textAnchor="middle" className={styles.hubText}>
        bazntms-hub
      </text>
      <text x="480" y="256" textAnchor="middle" className={styles.hubSub}>
        ingest · RBAC · audit · uyarı
      </text>

      {/* depo */}
      <rect x="300" y="356" width="200" height="50" rx="9" className={styles.node} />
      <text x="400" y="378" textAnchor="middle" className={styles.nodeText}>
        PostgreSQL + TimescaleDB
      </text>
      <text x="400" y="394" textAnchor="middle" className={styles.nodeSub}>
        hypertable · cagg · retention
      </text>

      <rect x="520" y="356" width="200" height="50" rx="9" className={styles.node} />
      <text x="620" y="378" textAnchor="middle" className={styles.nodeText}>
        NATS JetStream
      </text>
      <text x="620" y="394" textAnchor="middle" className={styles.nodeSub}>
        ingest → processor
      </text>

      {/* protokol etiketleri */}
      <text x="300" y="160" className={styles.labelText}>
        telemetri ↑
      </text>
      <text x="640" y="160" className={styles.labelText}>
        SNMP · NetFlow · Syslog
      </text>
    </svg>
  );
}

/* --- canlı önizleme: gerçek dashboard'un sahnelenmiş küçük hali --- */
/* Gerçek veri/IP göstermez — yalnızca ürünün "hissini" taşıyan sentetik bir sahne. */

const PREVIEW_AGENTS = [
  { y: 168, name: 'agent-ofis-3' },
  { y: 268, name: 'agent-dc1-07' },
  { y: 368, name: 'agent-sube-a' },
];

const PREVIEW_STATS = [
  { label: 'AKTİF AGENT', value: '124 / 140' },
  { label: 'PAKET HIZI', value: '3.2K pps' },
  { label: 'OLAY HIZI', value: '4.8 / sn' },
  { label: 'AÇIK UYARI', value: '2' },
];

const PREVIEW_ALERTS = ['yeni süreç · agent-ofis-3 : curl', 'bant genişliği zirvesi · agent-dc1-07', 'IOC eşleşmesi · agent-sube-a'];

const PREVIEW_GEO = [
  { x: 60, y: 24, r: 9 },
  { x: 118, y: 44, r: 5 },
  { x: 200, y: 30, r: 7 },
  { x: 260, y: 52, r: 4 },
  { x: 330, y: 20, r: 6 },
];

function LivePreview() {
  const HUB = { x: 500, y: 268 };
  const ROUTER = { x: 760, y: 268 };
  const NET = { x: 900, y: 268 };
  return (
    <svg
      className={styles.previewSvg}
      viewBox="0 0 1000 600"
      role="img"
      aria-label="bazNTMS gösterge paneli önizlemesi: agent filosu, hub, router ve internet arasında canlı paket akışı, üstte filo istatistikleri"
    >
      <defs>
        <linearGradient id="previewHubGlow" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#164e63" />
          <stop offset="100%" stopColor="#083344" />
        </linearGradient>
      </defs>

      {/* pencere çerçevesi */}
      <rect x="0.5" y="0.5" width="999" height="599" rx="16" className={styles.previewFrame} />

      {/* üst çubuk */}
      <g>
        <circle cx="24" cy="21" r="5" fill="#f87171" />
        <circle cx="42" cy="21" r="5" fill="#fbbf24" />
        <circle cx="60" cy="21" r="5" fill="#34d399" />
        <rect x="100" y="11" width="230" height="20" rx="10" className={styles.previewUrl} />
        <text x="215" y="25" textAnchor="middle" className={styles.previewUrlText}>
          bazntms.local · Genel Bakış
        </text>
        <circle cx="864" cy="21" r="3.5" className={styles.pulse} />
        <text x="876" y="25" className={styles.previewLive}>
          WS: CANLI
        </text>
      </g>
      <line x1="0" y1="42" x2="1000" y2="42" stroke="#1e293b" strokeWidth="1" />

      {/* sol nav şeridi */}
      <rect x="0" y="42" width="56" height="558" className={styles.previewSidebar} />
      {[70, 116, 162, 208, 254].map((y, i) => (
        <rect key={y} x="17" y={y} width="22" height="22" rx="6" className={i === 0 ? styles.previewNavActive : styles.previewNavItem} />
      ))}

      {/* istatistik şeridi */}
      {PREVIEW_STATS.map((s, i) => {
        const x = 78 + i * 216;
        return (
          <g key={s.label}>
            <rect x={x} y="60" width="200" height="58" rx="9" className={styles.node} />
            <text x={x + 16} y="82" className={styles.groupText}>
              {s.label}
            </text>
            <text x={x + 16} y="106" className={styles.previewStatValue}>
              {s.value}
            </text>
          </g>
        );
      })}

      {/* akış sahnesi: agent filosu → hub → router → internet */}
      <text x="78" y="150" className={styles.groupText}>
        AGENT FİLOSU
      </text>
      <g stroke="#155e75" strokeWidth="1.6" fill="none">
        {PREVIEW_AGENTS.map((a, i) => (
          <path
            key={a.name}
            d={`M254 ${a.y + 15} C 340 ${a.y + 15}, 360 ${HUB.y}, ${HUB.x - 84} ${HUB.y}`}
            strokeDasharray="5 7"
            className={i % 2 === 0 ? styles.flow : styles.flowSlow}
          />
        ))}
        <path d={`M${HUB.x + 84} ${HUB.y} L ${ROUTER.x - 44} ${ROUTER.y}`} strokeDasharray="5 7" className={styles.flow} strokeWidth="2.2" />
        <path d={`M${ROUTER.x + 44} ${ROUTER.y} L ${NET.x - 40} ${NET.y}`} strokeDasharray="5 7" className={styles.flowSlow} strokeWidth="2.2" />
      </g>

      {PREVIEW_AGENTS.map((a) => (
        <g key={a.name}>
          <rect x="78" y={a.y} width="176" height="46" rx="9" className={styles.node} />
          <circle cx="102" cy={a.y + 23} r="4" className={styles.dot} />
          <text x="118" y={a.y + 27} className={styles.nodeText}>
            {a.name}
          </text>
        </g>
      ))}

      <rect x={HUB.x - 84} y={HUB.y - 55} width="168" height="110" rx="12" className={styles.previewHub} />
      <circle cx={HUB.x} cy={HUB.y - 22} r="5" className={styles.pulse} />
      <text x={HUB.x} y={HUB.y + 8} textAnchor="middle" className={styles.hubText}>
        bazntms-hub
      </text>
      <text x={HUB.x} y={HUB.y + 28} textAnchor="middle" className={styles.hubSub}>
        3 agent · mTLS
      </text>

      <rect x={ROUTER.x - 44} y={ROUTER.y - 30} width="88" height="60" rx="9" className={styles.node} />
      <text x={ROUTER.x} y={ROUTER.y - 4} textAnchor="middle" className={styles.nodeSub}>
        ROUTER
      </text>
      <text x={ROUTER.x} y={ROUTER.y + 14} textAnchor="middle" className={styles.previewSmallLabel}>
        güvenlik duvarı
      </text>

      <circle cx={NET.x} cy={NET.y} r="34" className={styles.previewNet} />
      <text x={NET.x} y={NET.y + 4} textAnchor="middle" className={styles.previewSmallLabel}>
        İNTERNET
      </text>

      {/* alt paneller: uyarı akışı + coğrafi harita */}
      <rect x="78" y="470" width="410" height="106" rx="10" className={styles.node} />
      <text x="94" y="492" className={styles.groupText}>
        CANLI UYARI AKIŞI
      </text>
      {PREVIEW_ALERTS.map((a, i) => (
        <text key={a} x="94" y={514 + i * 20} className={styles.previewAlertLine}>
          {a}
        </text>
      ))}

      <rect x="512" y="470" width="410" height="106" rx="10" className={styles.node} />
      <text x="528" y="492" className={styles.groupText}>
        COĞRAFİ TRAFİK
      </text>
      <g transform="translate(536, 500)">
        {PREVIEW_GEO.map((p, i) => (
          <circle key={i} cx={p.x} cy={p.y} r={p.r} className={styles.geoDot} style={{ animationDelay: `${i * 0.3}s` }} />
        ))}
      </g>
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
          // pano erişimi engellenmiş olabilir (kurumsal tarayıcı politikası) —
          // kullanıcıya sessizce hiçbir şey olmamış gibi görünmesin
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

export default function Home() {
  const { siteConfig } = useDocusaurusContext();
  const docsUrl = useBaseUrl('/docs/installation');
  const apiUrl = useBaseUrl('/docs/reference/api');
  const configUrl = useBaseUrl('/docs/reference/configuration');
  const upgradeUrl = useBaseUrl('/docs/reference/upgrading');

  React.useEffect(() => {
    const els = document.querySelectorAll('[data-reveal]');
    const io = new IntersectionObserver(
      (entries) =>
        entries.forEach((e) => {
          if (e.isIntersecting) {
            e.target.classList.add(styles.revealed);
            io.unobserve(e.target);
          }
        }),
      { threshold: 0.12 },
    );
    els.forEach((el) => io.observe(el));
    return () => io.disconnect();
  }, []);

  return (
    <Layout
      title="Ağ Trafiği İzleme Platformu"
      description="Hub + agent + cihaz entegrasyonları: canlı trafik izleme, TimescaleDB + NATS ölçek altyapısı, RBAC/SSO, imza doğrulamalı agent güncellemesi."
    >
      <main className={styles.page}>
        {/* HERO */}
        <section className={styles.hero}>
          <div className={styles.heroInner}>
            <p className={`${styles.kicker} ${styles.fadeUp}`}>kurumsal ağ trafiği izleme platformu</p>
            <h1 className={`${styles.title} ${styles.fadeUp1}`}>
              baz<span>NTMS</span>
            </h1>
            <p className={`${styles.tagline} ${styles.fadeUp2}`}>
              {siteConfig.tagline}. Tek makinede başlayan bir izleme kurulumunu{' '}
              <b>5.000 agent</b> ölçeğine taşıyan uçtan uca platform.
            </p>
            <div className={`${styles.ctas} ${styles.fadeUp3}`}>
              <a className={styles.btnPrimary} href={docsUrl}>
                Kurulum dokümanı
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M5 12h14M13 6l6 6-6 6" />
                </svg>
              </a>
              <a
                className={styles.btnGhost}
                href="https://github.com/gokayybaz/bazntms"
                target="_blank"
                rel="noreferrer"
              >
                <svg viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 .5C5.65.5.5 5.65.5 12c0 5.08 3.29 9.39 7.86 10.91.58.11.79-.25.79-.55v-2.17c-3.2.7-3.87-1.36-3.87-1.36-.52-1.33-1.28-1.69-1.28-1.69-1.04-.71.08-.7.08-.7 1.15.08 1.76 1.19 1.76 1.19 1.03 1.75 2.69 1.25 3.34.95.1-.74.4-1.25.72-1.54-2.55-.29-5.23-1.28-5.23-5.68 0-1.26.45-2.28 1.19-3.09-.12-.29-.52-1.46.11-3.05 0 0 .97-.31 3.18 1.18a11.1 11.1 0 0 1 5.8 0c2.2-1.49 3.17-1.18 3.17-1.18.63 1.59.23 2.76.11 3.05.74.81 1.19 1.83 1.19 3.09 0 4.41-2.69 5.38-5.25 5.67.41.35.77 1.05.77 2.12v3.14c0 .3.21.67.8.55A11.51 11.51 0 0 0 23.5 12C23.5 5.65 18.35.5 12 .5z" />
                </svg>
                GitHub
              </a>
            </div>
            <div className={`${styles.meta} ${styles.fadeUp3}`}>
              <span>MIT Lisansı</span>
              <span>Go + React</span>
              <span>Sizin Altyapınızda</span>
            </div>
          </div>
        </section>

        {/* CANLI ÖNİZLEME */}
        <section className={styles.previewSection}>
          <div className={styles.previewWrap} data-reveal>
            <LivePreview />
          </div>
          <p className={styles.previewCaption}>
            Temsili görünüm — canlı bir demo değil, sahnelenmiş sentetik veri. Gerçek panel için{' '}
            <a href={docsUrl}>kurulum dokümanına</a> bakın.
          </p>
        </section>

        {/* v0.3.0'DA YENİ */}
        <section className={styles.whatsNew}>
          <div className={styles.whatsNewInner} data-reveal>
            <span className={styles.versionPill}>v0.3.0</span>
            <div className={styles.newBadges}>
              {NEW_IN.map((n) => (
                <span key={n} className={styles.badge}>
                  {n}
                </span>
              ))}
            </div>
          </div>
        </section>

        {/* SAYILAR */}
        <section className={styles.statsBand}>
          <p className={styles.statsCaption} data-reveal>
            Ölçek mimarisi tasarım hedefleri —{' '}
            <a href="https://github.com/gokayybaz/bazntms/tree/main/loadtest" target="_blank" rel="noreferrer">
              <code>bazntms-loadgen</code> ve k6
            </a>{' '}
            ile doğrulanır
          </p>
          <div className={styles.stats}>
            {STATS.map((s, i) => (
              <div key={s.label} data-reveal style={{ transitionDelay: `${i * 70}ms` }}>
                <b>{s.value}</b>
                <span>{s.label}</span>
              </div>
            ))}
          </div>
        </section>

        {/* YETENEKLER */}
        <section className={styles.section}>
          <p className={styles.overline} data-reveal>
            Platform
          </p>
          <h2 data-reveal>Uçtan uca görünürlük</h2>
          <div className={styles.grid}>
            {FEATURES.map((f, i) => (
              <article key={f.title} className={styles.feature} data-reveal style={{ transitionDelay: `${(i % 3) * 80}ms` }}>
                <div className={styles.featureIcon}>{f.icon}</div>
                <h3>{f.title}</h3>
                <p>{f.text}</p>
              </article>
            ))}
          </div>
        </section>

        {/* HIZLI BAŞLANGIÇ — alt alta */}
        <section className={styles.section}>
          <p className={styles.overline} data-reveal>
            Hızlı Başlangıç
          </p>
          <h2 data-reveal>Dört adımda çalışır durumda</h2>
          <div className={styles.steps}>
            {STEPS.map((s, i) => (
              <div key={s.label} className={styles.step} data-reveal style={{ transitionDelay: `${i * 80}ms` }}>
                <div className={styles.stepNo}>{i + 1}</div>
                <div className={styles.terminal}>
                  <div className={styles.terminalHead}>
                    <span className={styles.dotRed} />
                    <span className={styles.dotYellow} />
                    <span className={styles.dotGreen} />
                    <em>{s.label}</em>
                    <CopyButton code={s.code} />
                  </div>
                  <pre>{s.code}</pre>
                </div>
              </div>
            ))}
          </div>
          <p className={styles.hint} data-reveal>
            Tüm yapılandırma seçenekleri için{' '}
            <a href={configUrl}>yapılandırma referansına</a> bakın.
          </p>
        </section>

        {/* MİMARİ */}
        <section className={styles.section}>
          <p className={styles.overline} data-reveal>
            Mimari
          </p>
          <h2 data-reveal>Nasıl çalışır?</h2>
          <div data-reveal>
            <Architecture />
          </div>
          <p className={styles.hint} data-reveal>
            Hub stateless’tır; deploy replikaları arasında uyarı, poller ve
            yakalama rolleri bayraklarla ayrılır. Ayrıntılar için{' '}
            <a href={upgradeUrl}>operasyon dokümanlarına</a> göz atın.
          </p>
        </section>

        {/* ALT CTA */}
        <section className={styles.bottom}>
          <div data-reveal>
            <h2 className={styles.bottomTitle}>Ağınızı bugün görünür kılın.</h2>
            <p className={styles.bottomText}>
              Tek-node demo ile başlayın; aynı kurulumu TimescaleDB ve NATS
              arkasına taşıyarak filo ölçeğine çıkarın.
            </p>
            <div className={styles.ctas}>
              <a className={styles.btnPrimary} href={docsUrl}>
                Kurulum dokümanı
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M5 12h14M13 6l6 6-6 6" />
                </svg>
              </a>
              <a className={styles.btnGhost} href={apiUrl}>
                API referansı
              </a>
            </div>
          </div>
        </section>
      </main>
    </Layout>
  );
}
