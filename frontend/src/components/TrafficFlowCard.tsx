import { useCallback, useEffect, useMemo, useState } from 'react'
import type { AgentWithRates } from '../types'
import { useDialog } from '../lib/dialog'
import { TrafficFlowDiagram } from './TrafficFlowDiagram'
import type { DiagramAgent, DiagramDevice, TrafficEvent } from './TrafficFlowDiagram'

// TrafficFlowCard — canlı trafik şemasının kendi kendine yeten sürümü.
// Dashboard'dan ayrı bir sekmeye taşındı (çok agent / çok trafik senaryosunda
// pano şişmesin — rAF animasyonu yalnız bu sayfa açıkken çalışır).
//
// Gruplama: yönetici (editable) agent'ları switch/AP cihazlarına atar; şema
// bunları uplink'e göre gruplar. Atama satır içi — agent düğümüne tıkla.

interface FlowRow {
  ts: number
  device: string
  src: string
  dst: string
  src_port: number
  dst_port: number
  proto: string
  packets: number
  octets: number
}
interface SyslogEvent {
  id: number
  ts: number
  host: string
  severity: number
  tag: string
  message: string
}
interface AgentConnSample {
  proto: string
  local_addr: string
  remote_addr?: string
  status?: string
}
interface DeviceRow {
  id: number
  name: string
  kind: string
  last_poll: number
  poll_seconds: number
  enabled: boolean
}

const GROUP_KINDS = new Set(['switch', 'ap', 'router', 'firewall'])
const DIRECT = '— Doğrudan (uplink yok) —'
const KIND_OPTS = ['switch', 'ap', 'router', 'firewall']
const SRC_VIRTUAL = 'Sanal düğüm (yalnız yerleşim, poll yok)'
const SRC_SNMP2 = 'SNMP v2c cihazı'
const SRC_SNMP3 = 'SNMP v3 cihazı'
const SRC_FORTI = 'FortiGate REST API'

export function TrafficFlowCard({ fill = false, editable = false }: { fill?: boolean; editable?: boolean }) {
  const { form, confirm } = useDialog()
  const [agents, setAgents] = useState<AgentWithRates[]>([])
  const [devices, setDevices] = useState<DeviceRow[]>([])
  const [flows, setFlows] = useState<FlowRow[]>([])
  const [syslog, setSyslog] = useState<SyslogEvent[]>([])
  const [agentConns, setAgentConns] = useState<{ agentName: string; ts: number; conns: AgentConnSample[] }[]>([])
  const [refreshTick, setRefreshTick] = useState(0)
  const bump = useCallback(() => setRefreshTick((n) => n + 1), [])

  useEffect(() => {
    let stop = false
    const load = async () => {
      try {
        const res = await fetch('/api/v1/agents')
        if (res.ok && !stop) setAgents(await res.json())
      } catch {
        /* yoksay */
      }
    }
    load()
    const id = window.setInterval(load, 5_000)
    return () => {
      stop = true
      window.clearInterval(id)
    }
  }, [refreshTick])

  useEffect(() => {
    let stop = false
    const load = async () => {
      try {
        const res = await fetch('/api/v1/devices')
        if (res.ok && !stop) setDevices(await res.json())
      } catch {
        /* yoksay */
      }
    }
    load()
    const id = window.setInterval(load, 10_000)
    return () => {
      stop = true
      window.clearInterval(id)
    }
  }, [refreshTick])

  useEffect(() => {
    let stop = false
    const load = async () => {
      try {
        const res = await fetch('/api/v1/flows?minutes=15&limit=20')
        if (res.ok && !stop) setFlows(await res.json())
      } catch {
        /* yoksay */
      }
    }
    load()
    const id = window.setInterval(load, 6_000)
    return () => {
      stop = true
      window.clearInterval(id)
    }
  }, [])

  useEffect(() => {
    let stop = false
    const load = async () => {
      try {
        const res = await fetch('/api/v1/syslog?limit=20')
        if (res.ok && !stop) setSyslog(await res.json())
      } catch {
        /* yoksay */
      }
    }
    load()
    const id = window.setInterval(load, 5_000)
    return () => {
      stop = true
      window.clearInterval(id)
    }
  }, [])

  const onlineAgentKey = agents.filter((a) => a.online).map((a) => a.id).join(',')
  useEffect(() => {
    if (!onlineAgentKey) {
      setAgentConns([])
      return
    }
    let stop = false
    const ids = onlineAgentKey.split(',').slice(0, fill ? 64 : 40) // aşırı büyük filoda istek patlamasın
    const load = async () => {
      try {
        const results = await Promise.all(
          ids.map(async (idStr) => {
            const res = await fetch(`/api/v1/agents/${idStr}`)
            if (!res.ok) return null
            const data: { agent: AgentWithRates; connections: AgentConnSample[] } = await res.json()
            return { agentName: data.agent.name, ts: data.agent.last_seen, conns: data.connections ?? [] }
          }),
        )
        if (!stop) setAgentConns(results.filter((r): r is { agentName: string; ts: number; conns: AgentConnSample[] } => r !== null))
      } catch {
        /* yoksay */
      }
    }
    load()
    const id = window.setInterval(load, 7_000)
    return () => {
      stop = true
      window.clearInterval(id)
    }
  }, [onlineAgentKey, fill])

  // switch/AP/router/firewall cihazları — gruplama düğümü olabilecekler
  const groupDevices = useMemo<DiagramDevice[]>(
    () =>
      devices
        .filter((d) => GROUP_KINDS.has(d.kind))
        .map((d) => ({
          id: d.id,
          name: d.name,
          kind: d.kind,
          online: d.enabled && d.last_poll > 0 && Date.now() / 1000 - d.last_poll < 3 * (d.poll_seconds || 60),
        })),
    [devices],
  )

  // sadece ÇEVRİMİÇİ agent'lar düğüm olur (süzme bileşen içinde); tüm filo
  // geçilir ki "N çevrimdışı gizli" ipucu gösterilebilsin.
  const agentByName = useMemo(() => {
    const m = new Map<string, AgentWithRates>()
    for (const a of agents) m.set(a.name, a)
    return m
  }, [agents])

  const diagramAgents = useMemo<DiagramAgent[]>(
    () =>
      [...agents]
        .sort((a, b) => Number(b.online) - Number(a.online) || a.name.localeCompare(b.name))
        .map((a) => {
          const busiest = [...(a.rates ?? [])].sort((x, y) => y.rx_bps + y.tx_bps - (x.rx_bps + x.tx_bps))[0]
          return {
            name: a.name,
            online: a.online,
            site: a.site || undefined,
            rxBps: busiest?.rx_bps ?? 0,
            txBps: busiest?.tx_bps ?? 0,
            uplinkId: a.uplink_device_id ?? undefined,
          }
        }),
    [agents],
  )

  const diagramEvents = useMemo<TrafficEvent[]>(() => {
    type Item =
      | { kind: 'flow'; ts: number; key: string; src: string; dst: string; dport: number; bytes: number }
      | { kind: 'syslog'; ts: number; key: string; source: string }
      | { kind: 'agent'; ts: number; key: string; source: string; local: string; remote?: string }
    const items: Item[] = [
      ...flows.map((f) => ({ kind: 'flow' as const, ts: f.ts, key: `f${f.ts}-${f.src}-${f.dst}-${f.src_port}`, src: f.src, dst: f.dst, dport: f.dst_port, bytes: f.octets ?? 0 })),
      ...syslog.map((e) => ({ kind: 'syslog' as const, ts: e.ts, key: `s${e.id}`, source: e.host })),
      ...agentConns.flatMap((a) =>
        a.conns.map((c, i) => ({ kind: 'agent' as const, ts: a.ts, key: `a${a.agentName}-${i}-${c.local_addr}-${c.remote_addr ?? ''}`, source: a.agentName, local: c.local_addr, remote: c.remote_addr })),
      ),
    ]
    return items
      .sort((a, b) => b.ts - a.ts)
      .slice(0, 80)
      .flatMap((it): TrafficEvent[] => {
        if (it.kind === 'flow') return [{ key: it.key, kind: 'flow', ts: it.ts, from: it.src, to: `${it.dst}:${it.dport}`, weight: it.bytes }]
        if (it.kind === 'agent') return it.remote ? [{ key: it.key, kind: 'agent', ts: it.ts, agent: it.source, from: it.local, to: it.remote }] : []
        return [{ key: it.key, kind: 'syslog', ts: it.ts, from: it.source }]
      })
  }, [flows, syslog, agentConns])

  // --- düzenleme eylemleri (yalnız editable) ---
  const assignUplink = useCallback(
    async (agentName: string) => {
      const a = agentByName.get(agentName)
      if (!a) return
      const opts = [DIRECT, ...groupDevices.map((d) => `${d.kind} · ${d.name}`)]
      const cur = a.uplink_device_id
        ? (() => {
            const d = groupDevices.find((g) => g.id === a.uplink_device_id)
            return d ? `${d.kind} · ${d.name}` : DIRECT
          })()
        : DIRECT
      const res = await form({
        title: `${agentName} · uplink`,
        fields: [{ key: 'uplink', label: 'Erişim katmanı (switch / AP / router)', type: 'select', options: opts, defaultValue: cur }],
        confirmLabel: 'Ata',
      })
      if (!res) return
      const pick = res.uplink
      const dev = pick === DIRECT ? null : groupDevices.find((d) => `${d.kind} · ${d.name}` === pick)
      const body = { device_id: dev ? dev.id : null }
      const r = await fetch(`/api/v1/agents/${a.id}/uplink`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      if (r.ok) bump()
      else await confirm(`Atama başarısız: ${await r.text()}`, { confirmLabel: 'Tamam' })
    },
    [agentByName, groupDevices, form, confirm, bump],
  )

  // switch/AP eklemek: önce kaynak (sanal / SNMP / REST), sonra ilgili alanlar.
  // Gerçek cihaz eklendiğinde Cihazlar sayfasındakiyle aynı: SNMP/REST ile
  // poll edilir. Sanal düğüm poll edilmez (yalnız şema yerleşimi).
  const post = useCallback(
    async (body: Record<string, unknown>) => {
      const r = await fetch('/api/v1/devices', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      if (r.ok) bump()
      else await confirm(`Eklenemedi: ${await r.text()}`, { confirmLabel: 'Tamam' })
    },
    [confirm, bump],
  )

  const addGroup = useCallback(async () => {
    const s1 = await form({
      title: 'Switch / AP ekle',
      fields: [
        {
          key: 'src',
          label: 'Kaynak',
          type: 'select',
          options: [SRC_VIRTUAL, SRC_SNMP2, SRC_SNMP3, SRC_FORTI],
          defaultValue: SRC_VIRTUAL,
        },
      ],
      confirmLabel: 'Devam',
    })
    if (!s1) return
    const src = s1.src

    if (src === SRC_VIRTUAL) {
      const r = await form({
        title: 'Sanal düğüm',
        fields: [
          { key: 'name', label: 'Ad (ör. kat1-sw)', type: 'text' },
          { key: 'kind', label: 'Tür', type: 'select', options: KIND_OPTS, defaultValue: 'switch' },
        ],
        confirmLabel: 'Ekle',
      })
      if (!r?.name?.trim()) return
      await post({ name: r.name.trim(), kind: r.kind, host: '' })
      return
    }

    if (src === SRC_SNMP2) {
      const r = await form({
        title: 'SNMP v2c cihazı',
        fields: [
          { key: 'name', label: 'Ad', type: 'text' },
          { key: 'kind', label: 'Tür', type: 'select', options: KIND_OPTS, defaultValue: 'switch' },
          { key: 'host', label: 'Host / IP', type: 'text', placeholder: '10.0.0.2' },
          { key: 'community', label: 'Community', type: 'password', placeholder: 'kasada şifrelenir' },
        ],
        confirmLabel: 'Ekle',
      })
      if (!r?.name?.trim() || !r.host?.trim()) return
      await post({ name: r.name.trim(), kind: r.kind, host: r.host.trim(), vendor: 'snmp', snmp_version: 2, community: r.community ?? '' })
      return
    }

    if (src === SRC_SNMP3) {
      const r = await form({
        title: 'SNMP v3 cihazı',
        fields: [
          { key: 'name', label: 'Ad', type: 'text' },
          { key: 'kind', label: 'Tür', type: 'select', options: KIND_OPTS, defaultValue: 'switch' },
          { key: 'host', label: 'Host / IP', type: 'text', placeholder: '10.0.0.2' },
          { key: 'v3_user', label: 'v3 kullanıcı', type: 'text' },
          { key: 'v3_auth_proto', label: 'Auth protokol', type: 'select', options: ['SHA', 'SHA256', 'SHA512', 'MD5'], defaultValue: 'SHA' },
          { key: 'v3_auth_pass', label: 'Auth parola', type: 'password' },
          { key: 'v3_priv_proto', label: 'Priv protokol', type: 'select', options: ['AES', 'AES256', 'DES'], defaultValue: 'AES' },
          { key: 'v3_priv_pass', label: 'Priv parola', type: 'password' },
        ],
        confirmLabel: 'Ekle',
      })
      if (!r?.name?.trim() || !r.host?.trim()) return
      await post({
        name: r.name.trim(),
        kind: r.kind,
        host: r.host.trim(),
        vendor: 'snmp',
        snmp_version: 3,
        v3_user: r.v3_user ?? '',
        v3_auth_proto: r.v3_auth_proto,
        v3_auth_pass: r.v3_auth_pass ?? '',
        v3_priv_proto: r.v3_priv_proto,
        v3_priv_pass: r.v3_priv_pass ?? '',
      })
      return
    }

    // FortiGate REST
    const r = await form({
      title: 'FortiGate REST API',
      fields: [
        { key: 'name', label: 'Ad', type: 'text' },
        { key: 'kind', label: 'Tür', type: 'select', options: KIND_OPTS, defaultValue: 'firewall' },
        { key: 'host', label: 'Host / IP', type: 'text' },
        { key: 'api_url', label: 'API URL', type: 'text', placeholder: 'https://10.0.0.1' },
        { key: 'api_token', label: 'API token', type: 'password', placeholder: 'read-only profil önerilir' },
      ],
      confirmLabel: 'Ekle',
    })
    if (!r?.name?.trim() || !r.api_url?.trim() || !r.api_token?.trim()) return
    await post({
      name: r.name.trim(),
      kind: r.kind,
      host: r.host?.trim() || r.api_url.trim(),
      vendor: 'fortigate',
      api_url: r.api_url.trim(),
      api_token: r.api_token,
      api_verify_tls: false,
    })
  }, [form, post])

  const editGroup = useCallback(
    async (deviceId: number) => {
      const d = groupDevices.find((g) => g.id === deviceId)
      if (!d) return
      const n = agents.filter((a) => a.uplink_device_id === deviceId).length
      if (
        await confirm(`“${d.name}” (${d.kind}) silinsin mi? Bağlı ${n} agent “Doğrudan”a döner.`, {
          danger: true,
          confirmLabel: 'Sil',
        })
      ) {
        const r = await fetch(`/api/v1/devices/${deviceId}`, { method: 'DELETE' })
        if (r.ok) bump()
        else await confirm(`Silinemedi: ${await r.text()}`, { confirmLabel: 'Tamam' })
      }
    },
    [groupDevices, agents, confirm, bump],
  )

  return (
    <TrafficFlowDiagram
      events={diagramEvents}
      agents={diagramAgents}
      devices={groupDevices}
      fill={fill}
      editable={editable}
      onAgentClick={editable ? assignUplink : undefined}
      onGroupClick={editable ? editGroup : undefined}
      onAddGroup={editable ? addGroup : undefined}
    />
  )
}
