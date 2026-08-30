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

const FEATURES = [
  {
    icon: <IconTraffic />,
    title: 'Canlı Trafik İzleme',
    text: 'Paket bazlı ölçüm, yön tespiti ve protokol dağılımı; en yoğun uç noktalar, DNS görünürlüğü ve GeoIP/ASN zenginleştirme ile saniye saniye görünürlük.',
  },
  {
    icon: <IconFleet />,
    title: 'Merkezi Agent Filosu',
    text: 'Enrollment, toplu telemetri ve offline disk kuyruğu ile 5.000 agent’a kadar ölçek. Süreç bazlı trafik atfı ve çoklu-hub failover standart olarak gelir.',
  },
  {
    icon: <IconDevices />,
    title: 'Cihaz Entegrasyonları',
    text: 'SNMPv3, NetFlow v5 ve syslog alıcısının üzerine FortiGate REST API: VPN tünelleri, SD-WAN sağlık metrikleri, politika hit trendleri ve kaynak/oturum izleme.',
  },
  {
    icon: <IconTopo />,
    title: 'Canlı Ağ Topolojisi',
    text: 'LLDP/CDP/ARP keşfi ve agent subnet bildirimleriyle otomatik harita: hub–cihaz–agent ilişkileri ve port düzeyi komşuluklar tek bakışta.',
  },
  {
    icon: <IconScale />,
    title: 'Ölçek Altyapısı',
    text: 'PostgreSQL + TimescaleDB üzerinde katmanlı saklama (ham 7 gün, 1 dk 90 gün, 1 sa 2 yıl) ve NATS JetStream ile esnek ingest hattı; k8s/Helm dağıtımı hazır.',
  },
  {
    icon: <IconShield />,
    title: 'Güvenlik ve Denetim',
    text: 'Rol tabanlı erişim (admin/netops/analyst/viewer + site scope), OIDC SSO, entegrasyon token’ları ve hash-zincirli append-only denetim kaydı.',
  },
  {
    icon: <IconSpark />,
    title: 'Akıllı Operasyon',
    text: 'İstatistiksel anomali tespiti (saatlik baseline + z-skoru), SLA/kapasite/banding raporları ve Teams, Slack, SMTP, imzalı webhook bildirim kanalları.',
  },
  {
    icon: <IconRocket />,
    title: 'Dağıtım ve Operasyon',
    text: 'Docker/Helm ile k8s dağıtımı; deb/rpm/MSI/pkg installer’lar; ed25519 imza doğrulamalı otomatik agent güncelleme kanalı (stable/beta).',
  },
  {
    icon: <IconStamp />,
    title: '5651 Uyumluluk ve ISO 27001',
    text: 'Loglar Merkle checkpoint + RFC 3161 zaman damgası ile imzalanır, WORM depoda 2 yıl saklanır; delil paketi, offline doğrulama ve ISO kontrol haritası.',
  },
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

/* --- kopyala düğmesi --- */

function CopyButton({ code }) {
  const [done, setDone] = React.useState(false);
  return (
    <button
      type="button"
      className={styles.copy}
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(code);
          setDone(true);
          setTimeout(() => setDone(false), 1600);
        } catch {
          /* pano erişimi yoksa sessizce yut */
        }
      }}
    >
      {done ? '✓ kopyalandı' : 'kopyala'}
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
            <p className={`${styles.kicker} ${styles.fadeUp}`}>enterprise network traffic monitoring</p>
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

        {/* SAYILAR */}
        <section className={styles.statsBand}>
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
