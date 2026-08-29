import React from 'react';
import Layout from '@theme/Layout';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import useBaseUrl from '@docusaurus/useBaseUrl';
import styles from './index.module.css';

const FEATURES = [
  {
    icon: '⚡',
    title: 'Canlı Trafik İzleme',
    text: 'gopacket/libpcap ile paket bazlı ölçüm: yön tespiti, protokol dağılımı, en yoğun uç noktalar, DNS görünürlüğü ve GeoIP/ASN zenginleştirme.',
  },
  {
    icon: '🛰️',
    title: 'Hub + Agent Filosu',
    text: '5.000 agent ölçeğinde merkezi filo: enrollment, protobuf-hazır telemetri, offline disk kuyruğu, çoklu-hub failover ve süreç bazlı trafik atfı.',
  },
  {
    icon: '🖧',
    title: 'Cihaz Entegrasyonları',
    text: 'SNMPv3 poller (IF-MIB), NetFlow v5 toplayıcı, RFC3164 syslog alıcı; LLDP/CDP/ARP keşfiyle canlı topoloji haritası. Kimlik kasası AES-256-GCM.',
  },
  {
    icon: '📈',
    title: 'Ölçek Altyapısı',
    text: 'PostgreSQL + TimescaleDB (hypertable, continuous aggregate, retention: ham 7g → 1dk 90g → 1sa 2y) ve NATS JetStream ile ingest → processor ayrışması.',
  },
  {
    icon: '🔐',
    title: 'Güvenlik ve Denetim',
    text: 'RBAC (admin/netops/analyst/viewer + site scope), OIDC SSO, entegrasyon API token’ları ve SHA-256 hash-zincirli append-only denetim kaydı.',
  },
  {
    icon: '🧠',
    title: 'Akıllı Operasyon',
    text: 'AI’sız z-skoru anomali tespiti (saatlik baseline), SLA/kapasite/banding raporları, Teams/Slack/SMTP/webhook v2 (HMAC) bildirim kanalları.',
  },
];

const PHASES = [
  ['FAZ 0', 'Zemin Hazırlığı'],
  ['FAZ 1', 'Hub + Agent MVP'],
  ['FAZ 2', 'Süreç Bazlı Trafik'],
  ['FAZ 3', 'Cihaz Entegrasyonları'],
  ['FAZ 4', 'Ölçek Altyapısı'],
  ['FAZ 5', 'Güvenlik / RBAC / HA'],
  ['FAZ 6', 'İleri Analiz'],
  ['FAZ 7', 'Dağıtım / Operasyon'],
];

const QUICKSTART = [
  { label: '1 · Tek-node demo (Docker)', code: 'git clone https://github.com/gokayybaz/bazntms\ncd bazntms\ndocker compose -f deploy/docker-compose.yml up --build\n# → http://localhost:8080 (şifre: demo123)' },
  { label: '2 · Kurulum sihirbazı', code: './bazntmsctl setup\n./bazntms-hub -config hub.yml' },
  { label: '3 · Agent bağla', code: './bazntms-agent -hub-url https://hub.example.com \\\n  -enroll-token <hub-loglarındaki-token>' },
];

function Feature({ icon, title, text }) {
  return (
    <div className={styles.feature}>
      <div className={styles.featureIcon}>{icon}</div>
      <h3>{title}</h3>
      <p>{text}</p>
    </div>
  );
}

export default function Home() {
  const { siteConfig } = useDocusaurusContext();
  const docsUrl = useBaseUrl('/docs/installation');
  const apiUrl = useBaseUrl('/docs/reference/api');
  const upgradeUrl = useBaseUrl('/docs/reference/upgrading');
  return (
    <Layout
      title="Ağ Trafiği İzleme Platformu"
      description="Hub + agent + cihaz entegrasyonları: canlı trafik izleme, TimescaleDB + NATS ölçek altyapısı, RBAC/SSO, imza doğrulamalı agent güncellemesi."
    >
      <main className={styles.page}>
        {/* HERO */}
        <section className={styles.hero}>
          <p className={styles.kicker}>enterprise network traffic monitoring</p>
          <h1 className={styles.title}>
            baz<span>NTMS</span>
          </h1>
          <p className={styles.tagline}>
            {siteConfig.tagline} — tek makineden <b>5.000 agent</b> ölçeğine.
          </p>
          <div className={styles.ctas}>
            <a className={styles.btnPrimary} href={docsUrl}>
              Hızlı Başlangıç →
            </a>
            <a
              className={styles.btnGhost}
              href="https://github.com/gokayybaz/bazntms"
              target="_blank"
              rel="noreferrer"
            >
              GitHub ↗
            </a>
          </div>
          <div className={styles.chips}>
            <span className={styles.chip}>FAZ 0–7 ✓ TAMAMLANDI</span>
            <span className={styles.chip}>MIT</span>
            <span className={styles.chip}>GO + REACT</span>
            <span className={styles.chip}>SİZİN ALTYAPINIZDA</span>
          </div>
        </section>

        {/* SAYILAR */}
        <section className={styles.stats}>
          <div>
            <b>1.000</b>
            <span>cihaz · 60 sn poll</span>
          </div>
          <div>
            <b>5.000</b>
            <span>agent · 30 sn batch</span>
          </div>
          <div>
            <b>50K</b>
            <span>flow/sn sürekli</span>
          </div>
          <div>
            <b>&lt;1 sn</b>
            <span>panel sorgusu p95</span>
          </div>
        </section>

        {/* OZELLİKLER */}
        <section className={styles.section}>
          <h2>Yetenekler</h2>
          <div className={styles.grid}>
            {FEATURES.map((f) => (
              <Feature key={f.title} {...f} />
            ))}
          </div>
        </section>

        {/* HIZLI BASLANGIC */}
        <section className={styles.section}>
          <h2>60 saniyede başla</h2>
          <div className={styles.terminalGrid}>
            {QUICKSTART.map((q) => (
              <div key={q.label} className={styles.terminal}>
                <div className={styles.terminalHead}>
                  <i /> <i /> <i /> <span>{q.label}</span>
                </div>
                <pre>{q.code}</pre>
              </div>
            ))}
          </div>
        </section>

        {/* MIMARI */}
        <section className={styles.section}>
          <h2>Mimari</h2>
          <pre className={styles.arch}>{`┌─ Uçlar ────────────────────┐       ┌─ Ağ Cihazları ──────────────┐
│  bazntms-agent (5.000)     │       │  Firewall / Router / Switch │
│  · bağlantı + PID envanteri│       │  · SNMPv3 (LLDP/CDP/ARP)    │
│  · süreç trafik atfı       │       │  · NetFlow v5 · Syslog      │
│  · offline disk kuyruğu    │       └──────────────┬──────────────┘
└─────────────┬──────────────┘         SNMP poll ◄──┘  flow/syslog ▼
              │  mTLS-hazır HTTPS · policy ↓ / telemetri ↑
┌─────────────▼────────────────────────────────────────────────┐
│                 bazntms-hub (stateless ingest)               │
│  Ingest API · NATS JetStream · SNMP Poller · Flow Collector  │
│  PostgreSQL + TimescaleDB · RBAC / SSO · Audit · AI · Rapor  │
└──────────────────────────────────────────────────────────────┘`}</pre>
        </section>

        {/* YOL HARITASI */}
        <section className={styles.section}>
          <h2>Yol Haritası</h2>
          <div className={styles.phases}>
            {PHASES.map(([no, name]) => (
              <div key={no} className={styles.phase}>
                <b>{no}</b>
                <span>{name}</span>
                <em>✓</em>
              </div>
            ))}
          </div>
        </section>

        {/* CTA */}
        <section className={styles.bottom}>
          <a className={styles.btnPrimary} href={docsUrl}>
            Kurulum dokümanı →
          </a>
          <a className={styles.btnGhost} href={apiUrl}>
            API referansı
          </a>
          <a className={styles.btnGhost} href={upgradeUrl}>
            Upgrade runbook
          </a>
        </section>
      </main>
    </Layout>
  );
}
