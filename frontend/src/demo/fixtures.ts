// Demo katmanı — statik (veya yarı-statik) yanıt gövdeleri. Bunlar tick()
// ile değişmeyen kayıtlar: yönetim, uyumluluk/ISMS, rapor arşivi, AI sağlayıcı.

import { now } from './world'

const day = 86400

export const authStatus = {
  required: false,
  authenticated: false,
  multi_site: false,
  public_url: '',
}

export const health = () => ({
  score: 88,
  deductions: [
    { reason: '3 agent 24 saattir çevrimdışı (izmir)', points: 5 },
    { reason: 'core-sw-merkez-01 Gi1/0/1 kullanımı yüksek', points: 4 },
    { reason: 'dist-sw-merkez-02 son SNMP poll başarısız', points: 3 },
  ],
})

export const alertConfig = {
  enabled: true,
  cooldown_min: 10,
  bandwidth: { enabled: true, in_mbps: 800, out_mbps: 400, seconds: 120 },
  ports: { enabled: true, ports: [23, 4444, 1337, 3389] },
  new_proc: { enabled: true, ignore: ['chrome', 'firefox', 'slack'] },
  new_target: { enabled: true, min_total_mb: 500 },
  anomaly: { enabled: true, sensitivity: 3.5, min_samples: 12, window_min: 60 },
  forti: { vpn_down: true, sdwan_latency_ms: 150, sdwan_jitter_ms: 40, sdwan_loss_pct: 3, max_sessions: 500000 },
  iface: { enabled: true, warn_pct: 70, crit_pct: 90, sustain_sec: 300 },
  notifiers: {
    desktop: false,
    generic_url: '',
    discord_url: '',
    slack_url: 'https://hooks.slack.com/services/DEMO/XXXX/xxxx',
    telegram_token: '',
    telegram_chat_id: '',
    siem: { enabled: true, format: 'cef' as const, transport: 'syslog-udp' as const, target: 'siem.kurum.local:514', token: '', insecure: false },
    jira: { enabled: false, base_url: '', email: '', api_token: '', project: '', issue_type: '', resolve_transition: '' },
    servicenow: { enabled: false, base_url: '', user: '', password: '' },
  },
  notify_routes: [
    { severity: 'crit', kind: '', site: '', channels: ['slack', 'jira'], continue: false },
  ],
}

export const alertStatus = {
  channels: {
    slack: { last_attempt: now() - 3600, ok: true },
    siem: { last_attempt: now() - 900, ok: true },
  },
}

// ---- ISMS / ISO 27001 ----
export const ismsSummary = {
  soa: { total: 93, applicable: 84, implemented: 71, verified: 44, excluded: 9 },
  risks: { total: 18, open: 6, high: 3, medium: 8, low: 7 },
  open_findings: 4,
  policies_published: 11,
  assets: 14,
  suppliers_due: 1,
  last_continuity_test: { id: 3, kind: 'restore', title: 'PostgreSQL PITR tatbikatı', performed_at: now() - day * 21, result: 'basarili', evidence: 'runbook DR-02 · backup log 2026-08-19' },
}

export const ismsAssets = [
  ['db', 'PostgreSQL + TimescaleDB', 'DBA', 'yuksek'],
  ['app', 'bazntms-hub (controller)', 'Platform', 'yuksek'],
  ['app', 'bazntms-hub (ingest havuzu)', 'Platform', 'orta'],
  ['queue', 'NATS JetStream', 'Platform', 'orta'],
  ['secret', 'kimlik kasası (vault.key)', 'Güvenlik', 'kritik'],
  ['pki', 'dahili CA (agent mTLS)', 'Güvenlik', 'yuksek'],
  ['host', 'agent filosu (140 uç)', 'BT Operasyon', 'orta'],
  ['net', 'FortiGate-100F (fw-merkez-01)', 'Ağ', 'yuksek'],
  ['net', 'çekirdek anahtarlar (merkez)', 'Ağ', 'yuksek'],
  ['saas', 'Slack (uyarı bildirimi)', 'BT', 'dusuk'],
  ['saas', 'GitHub (kaynak + release)', 'Platform', 'orta'],
  ['data', '5651 imzalı log arşivi (WORM)', 'Hukuk/Uyum', 'kritik'],
  ['data', 'NetFlow / telemetri hypertable', 'BT Operasyon', 'orta'],
  ['doc', 'ISMS politika kütüphanesi', 'ISO Sorumlusu', 'orta'],
].map((a, i) => ({ id: i + 1, kind: a[0], name: a[1], owner: a[2], criticality: a[3], auto: i < 8 }))

export const ismsRisks = [
  ['Yetkisiz erişim', 'Zayıf/paylaşılan panel şifresi', 4, 3, 'mitigate', 'RBAC + OIDC SSO zorunlu, bootstrap şifre devre dışı', 'Güvenlik', 'in_progress'],
  ['Veri sızıntısı', 'Uyarı mesajında IOC/host sırları', 3, 2, 'mitigate', 'Uyarı gövdesi maskeleme (B1) uygulandı', 'Platform', 'closed'],
  ['Hizmet kesintisi', 'Tek hub düğümü', 4, 3, 'mitigate', 'HA: 2× controller + lider seçimi + paylaşımlı oturum', 'Platform', 'closed'],
  ['Kanıt bütünlüğü', 'Log zinciri çoklu-replika çatalı', 5, 2, 'mitigate', 'Tek-tx + pg advisory lock (audit + 5651)', 'Hukuk/Uyum', 'closed'],
  ['Tedarik zinciri', 'Bağımlılık/CVE', 3, 3, 'mitigate', 'govulncheck + Trivy imaj gate CI', 'Platform', 'open'],
  ['Fiziksel', 'DC erişim kontrolü (kiralık alan)', 3, 2, 'transfer', 'Sağlayıcı SOC 2 raporu yıllık gözden geçirme', 'Tesis', 'open'],
  ['Kötü amaçlı iç kullanıcı', 'Aşırı yetki', 3, 2, 'mitigate', 'En az yetki + hash-zincirli denetim kaydı', 'Güvenlik', 'in_progress'],
  ['Yedekten dönememe', 'Test edilmemiş yedek', 4, 2, 'mitigate', 'Çeyreklik PITR tatbikatı (BCDR kaydı)', 'DBA', 'closed'],
].map((r, i) => {
  const score = (r[2] as number) * (r[3] as number)
  return {
    id: i + 1, asset_id: ((i % 14) + 1), threat: r[0], vulnerability: r[1],
    impact: r[2], likelihood: r[3], score, treatment: r[4], plan: r[5],
    res_score: r[7] === 'closed' ? Math.max(2, Math.round(score / 3)) : 0,
    owner: r[6], status: r[7],
  }
})

const SOA_CATS = ['A.5 Örgütsel', 'A.6 İnsan', 'A.7 Fiziksel', 'A.8 Teknolojik']
export const ismsSoa = Array.from({ length: 40 }, (_, i) => {
  const cat = SOA_CATS[i % 4]
  const applicable = i % 11 !== 0
  const status = !applicable ? 'planned' : i % 3 === 0 ? 'verified' : i % 3 === 1 ? 'implemented' : 'planned'
  const num = 1 + i
  return {
    control_id: `A.${5 + (i % 4)}.${num}`,
    category: cat,
    title: [
      'Bilgi güvenliği politikaları', 'Roller ve sorumluluklar', 'Görevler ayrılığı', 'Yönetim taahhüdü',
      'Varlık envanteri', 'Kabul edilebilir kullanım', 'Erişim denetimi', 'Kimlik yönetimi',
      'Kimlik doğrulama bilgisi', 'Ayrıcalıklı erişim', 'Kriptografi', 'Anahtar yönetimi',
      'Loglama', 'İzleme', 'Zaman senkronizasyonu', 'Değişiklik yönetimi', 'Yedekleme',
      'Güvenlik açığı yönetimi', 'Ağ güvenliği', 'Ağ hizmetleri güvenliği', 'Uygulama güvenliği',
      'Güvenli geliştirme', 'Tedarikçi ilişkileri', 'Bulut hizmetleri', 'Olay yönetimi',
      'Delil toplama', 'İş sürekliliği', 'Yedeklilik', 'Yasal gereklilikler (5651)', 'Fikri mülkiyet',
      'Kayıt koruma', 'Gizlilik ve PII', 'Bağımsız gözden geçirme', 'Uyum gözden geçirmesi',
      'Belgelenmiş prosedürler', 'Fiziksel çevre güvenliği', 'Ekipman bakımı', 'Temiz masa',
      'Uzaktan çalışma', 'Depolama ortamı',
    ][i] ?? `Kontrol ${num}`,
    applicable,
    justification: applicable ? '' : 'Kuruluş bu kontrolü kapsam dışı bırakmıştır (ilgili varlık/süreç yok).',
    status,
    evidence: applicable && status !== 'planned' ? `bkz. politika POL-${(i % 11) + 1} / denetim kaydı` : '',
    owner: ['ISO Sorumlusu', 'Güvenlik', 'BT Operasyon', 'Platform'][i % 4],
  }
})

export const ismsPolicies = [
  'Bilgi Güvenliği Ana Politikası', 'Erişim Denetimi Politikası', 'Kriptografi Politikası',
  'Kabul Edilebilir Kullanım', 'Tedarikçi Güvenliği', 'Olay Müdahale Prosedürü',
  'İş Sürekliliği Planı', 'Değişiklik Yönetimi', 'Loglama ve İzleme (5651)',
  'Veri Sınıflandırma', 'Uzaktan Erişim Politikası', 'Yedekleme ve Kurtarma',
].map((title, i) => ({
  id: i + 1, ref: `POL-${String(i + 1).padStart(2, '0')}`, title,
  owner: ['ISO Sorumlusu', 'Güvenlik', 'Platform', 'BT'][i % 4],
  status: i < 9 ? 'published' : i === 9 ? 'approved' : i === 10 ? 'in_review' : 'draft',
  version: i < 9 ? `${1 + (i % 3)}.${i % 2}` : '0.9',
  approved_by: i < 10 ? 'CISO' : '',
  next_review: now() + day * (30 + i * 20),
}))

export const ismsAudits = [
  { id: 1, title: '2026 İç Denetim — Erişim Denetimi (A.8.2–A.8.5)', scope: 'RBAC, SSO, token yönetimi', planned_date: '2026-07-14', auditor: 'B. Yılmaz (iç)', status: 'closed' },
  { id: 2, title: '2026 İç Denetim — 5651 Log Bütünlüğü', scope: 'imza zinciri, TSA, WORM', planned_date: '2026-08-02', auditor: 'B. Yılmaz (iç)', status: 'done' },
  { id: 3, title: '2026-Q4 İç Denetim — Tedarikçi & BCDR', scope: 'A.5.19-22, A.5.29-30', planned_date: '2026-11-10', auditor: 'dış (planlı)', status: 'planned' },
]
export const ismsFindings: Record<number, any[]> = {
  1: [
    { id: 1, audit_id: 1, ref: 'F-2026-01', description: 'İki servis hesabı için MFA muafiyeti gerekçesiz', severity: 'orta', control_id: 'A.8.5', capa: 'API token’a geçiş + IP allowlist', capa_owner: 'Güvenlik', capa_due: '2026-09-30', status: 'in_progress', verified_by: '' },
    { id: 2, audit_id: 1, ref: 'F-2026-02', description: 'Ayrılan personel hesabı 6 gün aktif kaldı', severity: 'yuksek', control_id: 'A.8.3', capa: 'IK offboarding → otomatik devre dışı', capa_owner: 'BT', capa_due: '2026-09-15', status: 'verified', verified_by: 'B. Yılmaz' },
  ],
  2: [
    { id: 3, audit_id: 2, ref: 'F-2026-03', description: 'TSA yanıt gecikmesi bir günlük mühürü kaçırdı', severity: 'dusuk', control_id: 'A.8.15', capa: 'Yedek TSA endpoint + alarm', capa_owner: 'Platform', capa_due: '2026-10-01', status: 'open', verified_by: '' },
    { id: 4, audit_id: 2, ref: 'F-2026-04', description: 'WORM dizini disk doluluk alarmı yok', severity: 'orta', control_id: 'A.8.6', capa: 'Kapasite alarmı eklendi', capa_owner: 'Platform', capa_due: '2026-09-20', status: 'closed', verified_by: 'B. Yılmaz' },
  ],
  3: [],
}

export const ismsReviews = [
  { id: 1, ts: now() - day * 95, period: '2026-Q2', attendees: 'CISO, Platform Lideri, ISO Sorumlusu, BT Md.', decisions: 'HA mimarisi onaylandı; bootstrap şifre kaldırma hızlandırıldı', actions: 'B1–B8 sertleştirme planı' },
  { id: 2, ts: now() - day * 8, period: '2026-Q3', attendees: 'CISO, Platform Lideri, ISO Sorumlusu', decisions: 'AI analiz egress kilidi zorunlu; anomali v2 üretime', actions: 'Q4 dış denetim hazırlığı' },
]
export const ismsSuppliers = [
  { id: 1, name: 'Hetzner', service: 'Barındırma / IaaS', criticality: 'yuksek', next_review: now() + day * 40 },
  { id: 2, name: 'GitHub', service: 'Kaynak kod + CI + release', criticality: 'orta', next_review: now() + day * 120 },
  { id: 3, name: 'Slack', service: 'Uyarı bildirimi', criticality: 'dusuk', next_review: now() - day * 5 },
  { id: 4, name: 'DigiCert', service: 'RFC 3161 zaman damgası', criticality: 'yuksek', next_review: now() + day * 210 },
]
export const ismsContinuity = [
  { id: 1, kind: 'backup_check', title: 'Günlük yedek doğrulama (otomatik)', performed_at: now() - day * 1, result: 'basarili', evidence: 'backup log 2026-09-08' },
  { id: 2, kind: 'failover', title: 'hub-controller lider devir tatbikatı', performed_at: now() - day * 34, result: 'basarili', evidence: 'HA smoke 8/8' },
  { id: 3, kind: 'restore', title: 'PostgreSQL PITR tatbikatı', performed_at: now() - day * 21, result: 'kismen', evidence: 'runbook DR-02 — RTO hedefin 8 dk üstünde' },
]

export const complianceStatus = {
  config: { enabled: true, tsa_url: 'https://timestamp.digicert.com', sign_key: true, worm_dir: '/data/worm', mask_pii: true, retention_days: 730 },
  records: 4821903,
  last_record_ts: now() - 12,
  last_hourly: { bucket_start: now() - 1800, root: '9f2c4a8e17b3d05f2a6c9e14b7d38f0a4e2c6b91', record_count: 5122 },
  last_daily: { day: new Date(Date.now() - day * 1000).toISOString().slice(0, 10), root: 'a13b7c9e2f4d6081b3a5c7e90d2f4681a3c5e7b9', tsa_status: 'ok', signed: true, signed_at: now() - day + 3600, record_count: 118433 },
}
export const complianceReviews = [
  { id: 1, ts: now() - day * 30, username: 'netops', kind: 'log', period: '2026-08', notes: 'Ağustos log incelemesi — anomali yok, 2 flap olayı takip edildi', finding: '' },
  { id: 2, ts: now() - day * 3, username: 'admin', kind: 'access', period: '2026-09', notes: 'Çeyreklik erişim gözden geçirmesi — 1 fazla yetki düzeltildi', finding: 'F-2026-05' },
]

// ---- yönetim ----
export const users = [
  ['admin', 'admin', '', now() - day * 400, now() - 3600],
  ['netops', 'netops', '', now() - day * 210, now() - day],
  ['analyst-1', 'analyst', '', now() - day * 90, now() - day * 2],
  ['ankara-admin', 'site-admin', 'ankara', now() - day * 60, now() - day * 4],
  ['izmir-viewer', 'viewer', 'izmir', now() - day * 45, now() - day * 9],
  ['soc-bot', 'analyst', '', now() - day * 30, now() - 600],
].map((u, i) => ({ id: i + 1, username: u[0], role: u[1], site: u[2], enabled: i !== 5 ? true : true, created_at: u[3], last_login: u[4] }))

export const tokens = [
  ['grafana', 'viewer', '', now() - day * 120, now() - 300],
  ['ci-release', 'admin', '', now() - day * 200, now() - day * 3],
  ['ankara-poller', 'site-admin', 'ankara', now() - day * 50, now() - 900],
  ['siem-export', 'analyst', '', now() - day * 20, now() - 60],
].map((t, i) => ({ id: i + 1, name: t[0], role: t[1], site: t[2], created_at: t[3], last_used: t[4], revoked: i === 3 ? false : false }))

export const enrollTokens = [
  { id: 1, name: 'bootstrap-merkez', site: 'merkez', created_at: now() - day * 30, expires_at: now() + day * 60, last_used: now() - day, revoked: false, max_uses: 0, used_count: 62, allowed_cidrs: '10.0.0.0/16', created_by: 'admin' },
  { id: 2, name: 'ankara-rollout', site: 'ankara', created_at: now() - day * 14, expires_at: now() + day * 16, last_used: now() - day * 2, revoked: false, max_uses: 40, used_count: 28, allowed_cidrs: '10.10.0.0/16', created_by: 'ankara-admin' },
  { id: 3, name: 'dc1-batch (süresi doldu)', site: 'dc-1', created_at: now() - day * 90, expires_at: now() - day * 5, last_used: now() - day * 6, revoked: false, max_uses: 25, used_count: 25, allowed_cidrs: '', created_by: 'admin' },
]

export const aiStatus = {
  enabled: true,
  providers: [
    { id: 1, name: 'Yerel Ollama', kind: 'ollama', enabled: true, default_model: 'llama3.1:8b' },
    { id: 2, name: 'OpenAI', kind: 'openai', enabled: true, default_model: 'gpt-4o-mini' },
  ],
  default_provider: 1,
  default_model: 'llama3.1:8b',
  has_ready_provider: true,
}
export const aiProviders = [
  { id: 1, name: 'Yerel Ollama', kind: 'ollama', base_url: 'http://127.0.0.1:11434', default_model: 'llama3.1:8b', enabled: true, has_key: false, is_local: true },
  { id: 2, name: 'OpenAI', kind: 'openai', base_url: 'https://api.openai.com/v1', default_model: 'gpt-4o-mini', enabled: true, has_key: true, is_local: false },
  { id: 3, name: 'Anthropic', kind: 'anthropic', base_url: 'https://api.anthropic.com', default_model: 'claude-sonnet-5', enabled: false, has_key: false, is_local: false },
]
export const aiPresets = {
  presets: [
    { id: 'fleet_health', label: 'Filo sağlık özeti', scopes: ['fleet'], task: '' },
    { id: 'agent_review', label: 'Agent incele', scopes: ['agent'], task: '' },
    { id: 'incident_triage', label: 'Olay triyajı', scopes: ['incident'], task: '' },
    { id: 'anomaly_review', label: 'Sapmaları yorumla', scopes: ['anomaly'], task: '' },
    { id: 'device_review', label: 'Cihaz incele', scopes: ['device'], task: '' },
  ],
}

export const slaTargets = {
  targets: [
    { scope: 'global', site: '', agent_uptime_pct: 99, device_health_pct: 95, iface_err_ceiling: 5000 },
    { scope: 'site', site: 'merkez', agent_uptime_pct: 99.5, device_health_pct: 98, iface_err_ceiling: 2000 },
    { scope: 'site', site: 'vpn', agent_uptime_pct: 95, device_health_pct: 0, iface_err_ceiling: 0 },
  ],
}
export const reportSchedules = {
  schedules: [
    { id: 1, spec: 'weekly:mon:07:00', enabled: true, next_run_ts: now() + day * 2, last_run_ts: now() - day * 5, last_status: 'ok', payload: { type: 'enterprise', days: 30, site: '', format: 'pdf', email: ['ops@kurum.local'] } },
    { id: 2, spec: 'monthly:1:06:00', enabled: true, next_run_ts: now() + day * 12, last_run_ts: now() - day * 18, last_status: 'ok', payload: { type: 'compliance', days: 30, site: '', format: 'pdf', email: ['uyum@kurum.local', 'ciso@kurum.local'] } },
  ],
}
export const reportArchive = {
  archive: Array.from({ length: 6 }, (_, i) => ({
    id: i + 1,
    kind: ['enterprise', 'compliance', 'traffic'][i % 3],
    site: i % 3 === 2 ? 'ankara' : '',
    days: 30,
    format: i % 4 === 0 ? 'html' : 'pdf',
    size: 180000 + i * 42000,
    generated_ts: now() - day * (i * 7 + 1),
    delivered_to: i < 3 ? 'ops@kurum.local' : '',
    status: 'ok',
  })),
}

export const auditVerify = { ok: true, broken_at: 0, checked: 500 }
const AUDIT_ACTIONS: [string, string, string, string][] = [
  ['login', 'admin', 'admin', 'ok'],
  ['user.update', 'admin', 'analyst-1', 'ok'],
  ['token.create', 'admin', 'siem-export', 'ok'],
  ['agent.delete', 'netops', 'agent-izmir-014', 'ok'],
  ['alert.ack', 'netops', 'alert#1042', 'ok'],
  ['device.create', 'ankara-admin', 'sw-ankara-01', 'ok'],
  ['isms.risk.update', 'admin', 'risk#5', 'ok'],
  ['login', 'unknown', 'admin', 'denied'],
  ['report.generate', 'analyst-1', 'enterprise/30d', 'ok'],
  ['config.update', 'admin', 'alert-config', 'ok'],
  ['token.revoke', 'admin', 'token#3', 'ok'],
  ['incident.resolve', 'netops', 'incident#3', 'ok'],
]
export const auditEvents = (limit: number) =>
  Array.from({ length: Math.min(limit, 120) }, (_, i) => {
    const a = AUDIT_ACTIONS[i % AUDIT_ACTIONS.length]
    return {
      id: 5000 - i,
      ts: now() - i * 900 - 60,
      username: a[1],
      role: a[1] === 'unknown' ? '' : a[1] === 'admin' ? 'admin' : 'netops',
      action: a[0],
      target: a[2],
      detail: a[3] === 'denied' ? 'geçersiz kimlik' : '',
      ip: a[1] === 'unknown' ? '198.51.100.23' : `10.0.9.${5 + (i % 20)}`,
      hash: (0xabcdef + i * 7).toString(16).padStart(12, '0'),
      actor_type: a[1] === 'unknown' ? '-' : i % 5 === 0 ? 'token' : 'user',
      request_id: `req-${(0x1000 + i).toString(16)}`,
      user_agent: 'Mozilla/5.0 (X11; Linux x86_64) bazNTMS-web',
      result: a[3],
      before_json: a[0].endsWith('.update') ? '{"role":"viewer"}' : '',
      after_json: a[0].endsWith('.update') ? '{"role":"analyst"}' : '',
    }
  })
