import type { ReactNode } from 'react'

// Mini markdown — AI yanıtları için. Harici kütüphane YOK (CLAUDE.md: elle
// yazılır). Desteklenen: # başlık, - / * / 1. liste, ``` fenced kod,
// `satır içi kod`, **kalın**, *italik*. Tablo/link yok (yeter).

function renderInline(text: string, keyPrefix: string): ReactNode[] {
  const nodes: ReactNode[] = []
  // **kalın**, *italik*, `kod` — sırayla tara
  const re = /(\*\*([^*]+)\*\*|\*([^*]+)\*|`([^`]+)`)/g
  let last = 0
  let m: RegExpExecArray | null
  let i = 0
  while ((m = re.exec(text)) !== null) {
    if (m.index > last) nodes.push(text.slice(last, m.index))
    if (m[2] !== undefined) {
      nodes.push(<strong key={`${keyPrefix}-b${i}`} className="font-bold text-ink-hi">{m[2]}</strong>)
    } else if (m[3] !== undefined) {
      nodes.push(<em key={`${keyPrefix}-i${i}`} className="italic">{m[3]}</em>)
    } else if (m[4] !== undefined) {
      nodes.push(
        <code key={`${keyPrefix}-c${i}`} className="rounded-sm bg-panel-2 px-1 text-rx">{m[4]}</code>,
      )
    }
    last = m.index + m[0].length
    i++
  }
  if (last < text.length) nodes.push(text.slice(last))
  return nodes
}

export function Markdown({ text }: { text: string }) {
  const lines = text.replace(/\r\n/g, '\n').split('\n')
  const blocks: ReactNode[] = []
  let list: { ordered: boolean; items: string[] } | null = null
  let code: string[] | null = null
  let key = 0

  const flushList = () => {
    if (!list) return
    const L = list
    blocks.push(
      L.ordered ? (
        <ol key={key++} className="my-1 list-decimal space-y-0.5 pl-5">
          {L.items.map((it, i) => (
            <li key={i}>{renderInline(it, `l${key}-${i}`)}</li>
          ))}
        </ol>
      ) : (
        <ul key={key++} className="my-1 list-disc space-y-0.5 pl-5">
          {L.items.map((it, i) => (
            <li key={i}>{renderInline(it, `l${key}-${i}`)}</li>
          ))}
        </ul>
      ),
    )
    list = null
  }

  for (const raw of lines) {
    const line = raw

    if (line.trim().startsWith('```')) {
      if (code === null) {
        flushList()
        code = []
      } else {
        blocks.push(
          <pre key={key++} className="my-1.5 overflow-x-auto border border-rule bg-panel-2 p-2 text-[11px] text-ink">
            <code>{code.join('\n')}</code>
          </pre>,
        )
        code = null
      }
      continue
    }
    if (code !== null) {
      code.push(line)
      continue
    }

    const h = line.match(/^(#{1,4})\s+(.*)$/)
    if (h) {
      flushList()
      const lvl = h[1].length
      blocks.push(
        <p
          key={key++}
          className={`mt-2 font-bold uppercase tracking-[0.04em] ${lvl <= 2 ? 'text-ink-hi' : 'text-ink'}`}
        >
          {renderInline(h[2], `h${key}`)}
        </p>,
      )
      continue
    }

    const ol = line.match(/^\s*(\d+)[.)]\s+(.*)$/)
    const ul = line.match(/^\s*[-*]\s+(.*)$/)
    if (ol) {
      if (!list || !list.ordered) {
        flushList()
        list = { ordered: true, items: [] }
      }
      list.items.push(ol[2])
      continue
    }
    if (ul) {
      if (!list || list.ordered) {
        flushList()
        list = { ordered: false, items: [] }
      }
      list.items.push(ul[1])
      continue
    }

    if (line.trim() === '') {
      flushList()
      continue
    }
    flushList()
    blocks.push(
      <p key={key++} className="my-1 leading-relaxed">
        {renderInline(line, `p${key}`)}
      </p>,
    )
  }
  flushList()
  if (code !== null) {
    blocks.push(
      <pre key={key++} className="my-1.5 overflow-x-auto border border-rule bg-panel-2 p-2 text-[11px] text-ink">
        <code>{code.join('\n')}</code>
      </pre>,
    )
  }
  return <div className="text-[13px] text-ink">{blocks}</div>
}
