// IsmsCard — ISMS yönetişim paneli (Faz 10): varlık envanteri, risk defteri,
// SoA (93 Annex A kontrolü), politika akışı, iç denetim + CAPA, yönetim
// incelemesi, tedarikçi güvenliği, BCDR testleri ve denetçi paketi.

import { useCallback, useEffect, useState } from 'react'

interface SoaCounts {
  total: number
  applicable: number
  implemented: number
  verified: number
  excluded: number
}
interface Summary {
  soa: SoaCounts
  risks: { total: number; open: number; high: number; medium: number; low: number }
  open_findings: number
  policies_published: number
  assets: number
  suppliers_due: number
  last_continuity_test?: ContinuityTest
}
interface Asset {
  id: number
  kind: string
  name: string
  owner: string
  criticality: string
  auto: boolean
}
interface Risk {
  id: number
  asset_id: number
  threat: string
  vulnerability: string
  impact: number
  likelihood: number
  score: number
  treatment: string
  plan: string
  res_score: number
  owner: string
  status: string
}
interface SoaItem {
  control_id: string
  category: string
  title: string
  applicable: boolean
  justification: string
  status: string
  evidence: string
  owner: string
}
interface Policy {
  id: number
  ref: string
  title: string
  owner: string
  status: string
  version: string
  approved_by: string
  next_review: number
}
interface Audit {
  id: number
  title: string
  scope: string
  planned_date: string
  auditor: string
  status: string
}
interface Finding {
  id: number
  audit_id: number
  ref: string
  description: string
  severity: string
  control_id: string
  capa: string
  capa_owner: string
  capa_due: string
  status: string
  verified_by: string
}
interface MgmtReview {
  id: number
  ts: number
  period: string
  attendees: string
  decisions: string
  actions: string
}
interface Supplier {
  id: number
  name: string
  service: string
  criticality: string
  next_review: number
}
interface ContinuityTest {
  id: number
  kind: string
  title: string
  performed_at: number
  result: string
  evidence: string
}

type Tone = 'ok' | 'warn' | 'bad' | 'muted'

function pill(text: string, tone: Tone = 'muted') {
  const tones: Record<Tone, string> = {
    ok: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-300',
    warn: 'border-amber-500/40 bg-amber-500/10 text-amber-300',
    bad: 'border-rose-500/40 bg-rose-500/10 text-rose-300',
    muted: 'border-slate-700 text-slate-400',
  }
  return <span className={`rounded px-1.5 py-0.5 font-mono text-[9px] uppercase ${tones[tone]}`}>{text}</span>
}

const riskTone = (score: number): Tone => (score >= 15 ? 'bad' : score >= 8 ? 'warn' : 'ok')
const statusTone = (s: string): Tone => {
  const map: Record<string, Tone> = {
    published: 'ok', verified: 'ok', closed: 'ok', implemented: 'ok', approved: 'ok', done: 'ok',
    in_review: 'warn', in_progress: 'warn', planned: 'muted', open: 'bad',
  }
  return map[s] ?? 'muted'
}

const btn = 'rounded border border-slate-700 px-2 py-0.5 text-[10px] text-slate-400 transition hover:border-slate-500 hover:text-slate-200'

export function IsmsCard({ refreshKey }: { refreshKey: number }) {
  const [tab, setTab] = useState<'risk' | 'soa' | 'politika' | 'denetim' | 'yonetisim'>('risk')
  const [summary, setSummary] = useState<Summary | null>(null)
  const [assets, setAssets] = useState<Asset[]>([])
  const [risks, setRisks] = useState<Risk[]>([])
  const [soa, setSoa] = useState<SoaItem[]>([])
  const [soaFilter, setSoaFilter] = useState<'hepsi' | 'planned' | 'uygulanan' | 'haric'>('hepsi')
  const [policies, setPolicies] = useState<Policy[]>([])
  const [audits, setAudits] = useState<Audit[]>([])
  const [selAudit, setSelAudit] = useState<number | null>(null)
  const [findings, setFindings] = useState<Finding[]>([])
  const [reviews, setReviews] = useState<MgmtReview[]>([])
  const [suppliers, setSuppliers] = useState<Supplier[]>([])
  const [continuity, setContinuity] = useState<ContinuityTest[]>([])
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    try {
      const [s, a, r, so, p, au, rv, sp, ct] = await Promise.all([
        fetch('/api/v1/isms/summary').then((x) => x.json()),
        fetch('/api/v1/isms/assets').then((x) => x.json()),
        fetch('/api/v1/isms/risks').then((x) => x.json()),
        fetch('/api/v1/isms/soa').then((x) => x.json()),
        fetch('/api/v1/isms/policies').then((x) => x.json()),
        fetch('/api/v1/isms/audits').then((x) => x.json()),
        fetch('/api/v1/isms/mgmt-reviews?limit=8').then((x) => x.json()),
        fetch('/api/v1/isms/suppliers').then((x) => x.json()),
        fetch('/api/v1/isms/continuity?limit=8').then((x) => x.json()),
      ])
      setSummary(s)
      setAssets(a)
      setRisks(r)
      setSoa(so)
      setPolicies(p)
      setAudits(au)
      setReviews(rv)
      setSuppliers(sp)
      setContinuity(ct)
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [])

  useEffect(() => {
    load()
  }, [load, refreshKey])

  const loadFindings = useCallback(async (auditId: number) => {
    setSelAudit(auditId)
    const res = await fetch(`/api/v1/isms/audits/${auditId}/findings`)
    if (res.ok) setFindings(await res.json())
  }, [])

  const post = async (url: string, body: unknown) => {
    await fetch(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }).catch(() => {})
    load()
  }
  const put = async (url: string, body: unknown) => {
    await fetch(url, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }).catch(() => {})
    load()
  }
  const del = async (url: string) => {
    await fetch(url, { method: 'DELETE' }).catch(() => {})
    load()
  }

  const ask = (q: string, def = '') => prompt(q, def) ?? ''

  const addRisk = () => {
    const threat = ask('Tehdit:')
    if (!threat) return
    post('/api/v1/isms/risks', {
      threat,
      vulnerability: ask('Zaafiyet:'),
      impact: Number(ask('Etki (1-5):', '3')),
      likelihood: Number(ask('Olasılık (1-5):', '3')),
      treatment: ask('Muamele (mitigate/accept/transfer/avoid):', 'mitigate'),
      plan: ask('Muamele planı:'),
      res_impact: Number(ask('Kalıntı etki (0=bilinmiyor):', '0')),
      res_likelihood: Number(ask('Kalıntı olasılık:', '0')),
      owner: ask('Sahip:'),
    })
  }

  const updateRisk = (rk: Risk) => {
    put(`/api/v1/isms/risks/${rk.id}`, { ...rk, status: ask('Durum (open/in_progress/closed):', rk.status) || rk.status })
  }

  const updateSoa = (c: SoaItem) => {
    const applicable = confirm(`${c.control_id} ${c.title}\n\nUygulanacak mı? (İptal = hariç, gerekçe istenir)`)
    const status = ask('Durum (planned/implemented/verified):', c.status) || c.status
    const justification = applicable ? c.justification : ask('Hariç gerekçesi:', c.justification)
    const evidence = ask('Kanıt notu:', c.evidence)
    const owner = ask('Sahip:', c.owner)
    put(`/api/v1/isms/soa/${c.control_id}`, { applicable, status, justification, evidence, owner })
  }

  const addPolicy = () => {
    const title = ask('Politika başlığı:')
    if (!title) return
    post('/api/v1/isms/policies', { title, owner: ask('Sahip:'), content: ask('İçerik (boş geçilebilir):') })
  }

  const addAudit = () => {
    const title = ask('Denetim başlığı:')
    if (!title) return
    post('/api/v1/isms/audits', { title, scope: ask('Kapsam:'), planned_date: ask('Planlanan tarih (YYYY-MM-DD):') })
  }

  const addFinding = (auditId: number) => {
    const description = ask('Bulgu açıklaması:')
    if (!description) return
    post(`/api/v1/isms/audits/${auditId}/findings`, {
      description,
      severity: ask('Şiddet (dusuk/orta/yuksek):', 'orta'),
      control_id: ask('İlgili kontrol (ör. A.8.15):'),
      capa: ask('CAPA aksiyonu:'),
      capa_owner: ask('CAPA sorumlusu:'),
      capa_due: ask('Vade (YYYY-MM-DD):'),
    })
    if (selAudit === auditId) loadFindings(auditId)
  }

  const tabs = [
    ['risk', `Risk (${risks.length})`],
    ['soa', `SoA ${summary ? summary.soa.implemented + '/' + summary.soa.applicable : ''}`],
    ['politika', `Politika (${policies.length})`],
    ['denetim', `Denetim (${audits.length})`],
    ['yonetisim', 'Yönetişim'],
  ] as const

  if (error) return <p className="text-xs text-rose-400">{error}</p>
  if (!summary) return <p className="text-xs text-slate-500">yükleniyor…</p>

  return (
    <div className="space-y-3">
      {/* olgunluk şeridi */}
      <div className="flex flex-wrap items-center gap-2">
        {pill(`soa ${summary.soa.implemented}/${summary.soa.applicable}`, summary.soa.verified > 0 ? 'ok' : 'muted')}
        {pill(`yüksek risk ${summary.risks.high}`, summary.risks.high > 0 ? 'bad' : 'ok')}
        {pill(`açık bulgu ${summary.open_findings}`, summary.open_findings > 0 ? 'warn' : 'ok')}
        {pill(`yayın ${summary.policies_published}`, 'muted')}
        {pill(`varlık ${summary.assets}`, 'muted')}
        {summary.suppliers_due > 0 && pill(`tedarikçi vadesi ${summary.suppliers_due}`, 'warn')}
        <a
          href="/api/v1/isms/auditor-package?format=html"
          className="ml-auto rounded-md border border-cyan-500/40 bg-cyan-500/10 px-3 py-1 text-xs font-medium text-cyan-300 transition hover:bg-cyan-500/20"
        >
          denetçi paketi ↓html
        </a>
        <a
          href="/api/v1/isms/auditor-package"
          className={btn}
        >
          ↓json
        </a>
        <button onClick={() => post('/api/v1/isms/assets/sync', {})} className={btn}>+ filo senkronu</button>
      </div>

      {/* sekmeler */}
      <div className="flex flex-wrap gap-1.5">
        {tabs.map(([k, label]) => (
          <button
            key={k}
            onClick={() => setTab(k)}
            className={`rounded px-2.5 py-1 text-[11px] transition ${
              tab === k ? 'bg-cyan-500/15 text-cyan-300' : 'text-slate-500 hover:text-slate-300'
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {tab === 'risk' && (
        <div className="space-y-1">
          <div className="mb-1.5 flex items-center gap-2">
            <span className="text-[10px] uppercase tracking-wider text-slate-500">risk defteri (etki×olasılık)</span>
            <button onClick={addRisk} className={btn}>+ risk</button>
          </div>
          {risks.length === 0 ? (
            <p className="text-xs text-slate-600">risk kaydı yok</p>
          ) : (
            risks.map((rk) => (
              <div key={rk.id} className="flex flex-wrap items-center gap-2 rounded border border-slate-800 bg-slate-900/50 px-2.5 py-1.5">
                {pill(String(rk.score), riskTone(rk.score))}
                <span className="text-xs text-slate-300">{rk.threat}</span>
                {rk.vulnerability && <span className="text-[10px] text-slate-500">{rk.vulnerability}</span>}
                <span className="text-[10px] text-slate-600">{rk.treatment}</span>
                {rk.res_score > 0 && pill(`kalıntı ${rk.res_score}`, riskTone(rk.res_score))}
                {pill(rk.status, statusTone(rk.status))}
                {rk.owner && <span className="font-mono text-[10px] text-slate-500">{rk.owner}</span>}
                <span className="ml-auto flex gap-1">
                  <button onClick={() => updateRisk(rk)} className={btn}>güncelle</button>
                  <button onClick={() => confirm(`${rk.threat} silinsin mi?`) && del(`/api/v1/isms/risks/${rk.id}`)} className={btn}>sil</button>
                </span>
                {rk.plan && <p className="w-full truncate text-[11px] text-slate-500" title={rk.plan}>plan: {rk.plan}</p>}
              </div>
            ))
          )}
        </div>
      )}

      {tab === 'soa' && (
        <div className="space-y-1">
          <div className="mb-1.5 flex items-center gap-2">
            <span className="text-[10px] uppercase tracking-wider text-slate-500">
              statement of applicability — {summary.soa.total} kontrol
            </span>
            <select
              value={soaFilter}
              onChange={(e) => setSoaFilter(e.target.value as typeof soaFilter)}
              className="rounded border border-slate-700 bg-slate-900 px-1.5 py-0.5 text-[10px] text-slate-300"
            >
              <option value="hepsi">hepsi</option>
              <option value="planned">planlanan</option>
              <option value="uygulanan">uygulanan</option>
              <option value="haric">hariç</option>
            </select>
          </div>
          <div className="max-h-96 space-y-0.5 overflow-y-auto pr-1">
            {soa
              .filter((c) =>
                soaFilter === 'hepsi' ? true
                : soaFilter === 'planned' ? c.applicable && c.status === 'planned'
                : soaFilter === 'uygulanan' ? c.applicable && (c.status === 'implemented' || c.status === 'verified')
                : !c.applicable,
              )
              .map((c) => (
                <button
                  key={c.control_id}
                  onClick={() => updateSoa(c)}
                  className="flex w-full flex-wrap items-center gap-2 rounded border border-slate-800/70 bg-slate-900/40 px-2.5 py-1 text-left transition hover:border-slate-600"
                >
                  <span className="font-mono text-[10px] text-cyan-300">{c.control_id}</span>
                  <span className="text-[11px] text-slate-300">{c.title}</span>
                  {!c.applicable && pill('hariç', 'bad')}
                  {pill(c.status, statusTone(c.status))}
                  {c.evidence && <span className="ml-auto max-w-[45%] truncate text-[10px] text-slate-600" title={c.evidence}>{c.evidence}</span>}
                </button>
              ))}
          </div>
          <p className="text-[10px] text-slate-600">satıra tıklayarak uygula/hariç kararı, gerekçe, kanıt ve sahip güncellenir</p>
        </div>
      )}

      {tab === 'politika' && (
        <div className="space-y-1">
          <div className="mb-1.5 flex items-center gap-2">
            <span className="text-[10px] uppercase tracking-wider text-slate-500">politika yaşam döngüsü</span>
            <button onClick={addPolicy} className={btn}>+ politika</button>
          </div>
          {policies.length === 0 ? (
            <p className="text-xs text-slate-600">politika yok — akış: taslak → inceleme → onay → yayın</p>
          ) : (
            policies.map((p) => (
              <div key={p.id} className="flex flex-wrap items-center gap-2 rounded border border-slate-800 bg-slate-900/50 px-2.5 py-1.5">
                <span className="font-mono text-[10px] text-cyan-300">{p.ref}</span>
                <span className="text-xs text-slate-300">{p.title}</span>
                <span className="font-mono text-[10px] text-slate-600">v{p.version}</span>
                {pill(p.status, statusTone(p.status))}
                {p.approved_by && <span className="font-mono text-[10px] text-slate-500">{p.approved_by}</span>}
                {p.next_review > 0 && (
                  <span className="text-[10px] text-slate-600">inceleme: {new Date(p.next_review * 1000).toLocaleDateString('tr-TR')}</span>
                )}
                <span className="ml-auto flex gap-1">
                  {p.status === 'draft' && <button onClick={() => post(`/api/v1/isms/policies/${p.id}/transition`, { status: 'in_review' })} className={btn}>incelemeye al</button>}
                  {p.status === 'in_review' && <button onClick={() => post(`/api/v1/isms/policies/${p.id}/transition`, { status: 'approved' })} className={btn}>onayla</button>}
                  {p.status === 'approved' && <button onClick={() => post(`/api/v1/isms/policies/${p.id}/transition`, { status: 'published' })} className={btn}>yayınla</button>}
                </span>
              </div>
            ))
          )}
        </div>
      )}

      {tab === 'denetim' && (
        <div className="space-y-1">
          <div className="mb-1.5 flex items-center gap-2">
            <span className="text-[10px] uppercase tracking-wider text-slate-500">iç denetim programı + CAPA</span>
            <button onClick={addAudit} className={btn}>+ denetim</button>
          </div>
          {audits.length === 0 ? (
            <p className="text-xs text-slate-600">denetim kaydı yok</p>
          ) : (
            audits.map((a) => (
              <div key={a.id} className="rounded border border-slate-800 bg-slate-900/50 px-2.5 py-1.5">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-xs text-slate-300">{a.title}</span>
                  {a.scope && <span className="text-[10px] text-slate-500">{a.scope}</span>}
                  {a.planned_date && <span className="font-mono text-[10px] text-slate-600">{a.planned_date}</span>}
                  {pill(a.status, statusTone(a.status))}
                  {a.auditor && <span className="font-mono text-[10px] text-slate-500">{a.auditor}</span>}
                  <span className="ml-auto flex gap-1">
                    <button onClick={() => loadFindings(a.id)} className={btn}>bulgular</button>
                    <button onClick={() => addFinding(a.id)} className={btn}>+ bulgu</button>
                    <button onClick={() => put(`/api/v1/isms/audits/${a.id}`, { ...a, status: ask('Durum (planned/done/closed):', a.status) || a.status })} className={btn}>güncelle</button>
                  </span>
                </div>
                {selAudit === a.id && (
                  <div className="mt-1.5 space-y-0.5 border-t border-slate-800 pt-1.5">
                    {findings.length === 0 ? (
                      <p className="text-[11px] text-slate-600">bulgu yok</p>
                    ) : (
                      findings.map((f) => (
                        <div key={f.id} className="flex flex-wrap items-center gap-2 px-1 py-0.5">
                          <span className="font-mono text-[10px] text-slate-500">{f.ref}</span>
                          <span className="text-[11px] text-slate-300">{f.description}</span>
                          {pill(f.severity, f.severity === 'yuksek' ? 'bad' : f.severity === 'orta' ? 'warn' : 'muted')}
                          {f.control_id && <span className="font-mono text-[10px] text-cyan-300">{f.control_id}</span>}
                          {pill(f.status, statusTone(f.status))}
                          {f.capa_owner && <span className="font-mono text-[10px] text-slate-500">{f.capa_owner}{f.capa_due && ` · ${f.capa_due}`}</span>}
                          <span className="ml-auto flex gap-1">
                            <button
                              onClick={() =>
                                put(`/api/v1/isms/findings/${f.id}`, {
                                  ...f,
                                  status: ask('Durum (open/in_progress/verified/closed):', f.status) || f.status,
                                  capa: ask('CAPA:', f.capa),
                                })
                              }
                              className={btn}
                            >
                              CAPA
                            </button>
                          </span>
                        </div>
                      ))
                    )}
                  </div>
                )}
              </div>
            ))
          )}
        </div>
      )}

      {tab === 'yonetisim' && (
        <div className="space-y-3">
          <div>
            <div className="mb-1.5 flex items-center gap-2">
              <span className="text-[10px] uppercase tracking-wider text-slate-500">yönetim incelemesi</span>
              <button
                onClick={() => {
                  const period = ask('Dönem (ör. 2026-Q3):')
                  if (!period) return
                  post('/api/v1/isms/mgmt-reviews', {
                    period,
                    attendees: ask('Katılımcılar:'),
                    inputs: ask('Girdiler (metrik/rapor özeti):'),
                    decisions: ask('Kararlar:'),
                    actions: ask('Aksiyonlar:'),
                  })
                }}
                className={btn}
              >
                + inceleme
              </button>
            </div>
            {reviews.map((r) => (
              <div key={r.id} className="flex flex-wrap items-center gap-2 rounded border border-slate-800 bg-slate-900/50 px-2.5 py-1">
                <span className="text-xs text-slate-300">{r.period}</span>
                <span className="text-[10px] text-slate-500">{r.attendees}</span>
                <span className="ml-auto font-mono text-[10px] text-slate-600">{new Date(r.ts * 1000).toLocaleDateString('tr-TR')}</span>
                {r.decisions && <p className="w-full truncate text-[11px] text-slate-500" title={r.decisions}>karar: {r.decisions}</p>}
              </div>
            ))}
          </div>

          <div>
            <div className="mb-1.5 flex items-center gap-2">
              <span className="text-[10px] uppercase tracking-wider text-slate-500">tedarikçiler (A.5.19-22)</span>
              <button
                onClick={() => {
                  const name = ask('Tedarikçi adı:')
                  if (!name) return
                  post('/api/v1/isms/suppliers', {
                    name,
                    service: ask('Hizmet:'),
                    criticality: ask('Kritiklik (dusuk/orta/yuksek/kritik):', 'orta'),
                    data_access: ask('Veri erişimi:'),
                    contract_ref: ask('Sözleşme referansı:'),
                    risk: ask('Risk notu:'),
                  })
                }}
                className={btn}
              >
                + tedarikçi
              </button>
            </div>
            {suppliers.length === 0 ? (
              <p className="text-xs text-slate-600">tedarikçi kaydı yok</p>
            ) : (
              suppliers.map((sp) => (
                <div key={sp.id} className="flex flex-wrap items-center gap-2 rounded border border-slate-800 bg-slate-900/50 px-2.5 py-1">
                  <span className="text-xs text-slate-300">{sp.name}</span>
                  <span className="text-[10px] text-slate-500">{sp.service}</span>
                  {pill(sp.criticality, sp.criticality === 'kritik' || sp.criticality === 'yuksek' ? 'warn' : 'muted')}
                  {sp.next_review > 0 && sp.next_review <= Date.now() / 1000 && pill('vadesi geçti', 'bad')}
                  <button onClick={() => confirm(`${sp.name} silinsin mi?`) && del(`/api/v1/isms/suppliers/${sp.id}`)} className={`${btn} ml-auto`}>sil</button>
                </div>
              ))
            )}
          </div>

          <div>
            <div className="mb-1.5 flex items-center gap-2">
              <span className="text-[10px] uppercase tracking-wider text-slate-500">süreklilik testleri (BCDR)</span>
              <button
                onClick={() => {
                  const title = ask('Test başlığı:')
                  if (!title) return
                  post('/api/v1/isms/continuity', {
                    title,
                    kind: ask('Tür (restore/failover/backup_check/tabletop):', 'restore'),
                    result: ask('Sonuç (basarili/kismen/basarisiz):', 'basarili'),
                    evidence: ask('Kanıt (runbook/backup log referansı):'),
                    notes: ask('Notlar:'),
                  })
                }}
                className={btn}
              >
                + test
              </button>
            </div>
            {continuity.length === 0 ? (
              <p className="text-xs text-slate-600">test kaydı yok — DR runbook ve backup scriptleri kanıt olarak bağlanır</p>
            ) : (
              continuity.map((t) => (
                <div key={t.id} className="flex flex-wrap items-center gap-2 rounded border border-slate-800 bg-slate-900/50 px-2.5 py-1">
                  <span className="text-xs text-slate-300">{t.title}</span>
                  {pill(t.kind, 'muted')}
                  {pill(t.result, t.result === 'basarili' ? 'ok' : t.result === 'basarisiz' ? 'bad' : 'warn')}
                  {t.evidence && <span className="text-[10px] text-slate-600">{t.evidence}</span>}
                  <span className="ml-auto font-mono text-[10px] text-slate-600">{new Date(t.performed_at * 1000).toLocaleDateString('tr-TR')}</span>
                </div>
              ))
            )}
          </div>
        </div>
      )}

      {/* varlık envanteri özeti (tüm sekmelerde görünür) */}
      <details className="rounded border border-slate-800 bg-slate-900/40 px-2.5 py-1.5">
        <summary className="cursor-pointer text-[10px] uppercase tracking-wider text-slate-500">
          varlık envanteri ({assets.length})
        </summary>
        <div className="mt-1.5 flex flex-wrap gap-1">
          {assets.map((a) => (
            <span key={a.id} className="rounded border border-slate-800 bg-slate-900 px-1.5 py-0.5 font-mono text-[10px] text-slate-400">
              {a.kind}:{a.name}
              {a.criticality !== 'orta' && <b className="text-slate-300"> · {a.criticality}</b>}
            </span>
          ))}
          {assets.length === 0 && <span className="text-[11px] text-slate-600">boş — filo senkronu ile doldurun</span>}
        </div>
      </details>
    </div>
  )
}
