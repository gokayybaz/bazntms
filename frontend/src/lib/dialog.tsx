import { createContext, useCallback, useContext, useState } from 'react'
import type { ReactNode } from 'react'
import { Modal } from '../components/Modal'

// TUI diyalog yardımcıları — native confirm()/prompt() yerine. Uygulama kökünde
// <DialogProvider>, ekranlar `const { confirm, prompt } = useDialog()` ile
// promise-tabanlı çağırır. Bkz. DESIGN.md → Dialog.

export interface ConfirmOptions {
  title?: string
  confirmLabel?: string
  cancelLabel?: string
  /** Onay eylemi yıkıcı mı (rose reverse-video buton). */
  danger?: boolean
}

export interface PromptOptions extends ConfirmOptions {
  defaultValue?: string
  placeholder?: string
}

interface DialogApi {
  confirm: (message: ReactNode, opts?: ConfirmOptions) => Promise<boolean>
  prompt: (message: ReactNode, opts?: PromptOptions) => Promise<string | null>
}

type Request =
  | { kind: 'confirm'; message: ReactNode; opts: ConfirmOptions; resolve: (v: boolean) => void }
  | { kind: 'prompt'; message: ReactNode; opts: PromptOptions; resolve: (v: string | null) => void }

const DialogCtx = createContext<DialogApi | null>(null)

const btnBase = 'border px-3 py-1 font-mono text-[11px] uppercase tracking-[0.04em] transition'
const cancelCls = `${btnBase} border-rule-hi text-tui-dim hover:text-ink-hi hover:border-ink-hi`
const okCls = `${btnBase} border-rx bg-rx text-ground hover:opacity-90`
const dangerCls = `${btnBase} border-rose-400 bg-rose-400 text-ground hover:opacity-90`

export function DialogProvider({ children }: { children: ReactNode }) {
  const [req, setReq] = useState<Request | null>(null)
  const [value, setValue] = useState('')

  const close = useCallback(() => setReq(null), [])

  const confirm = useCallback<DialogApi['confirm']>(
    (message, opts = {}) =>
      new Promise<boolean>((resolve) => {
        setReq({ kind: 'confirm', message, opts, resolve })
      }),
    [],
  )

  const prompt = useCallback<DialogApi['prompt']>(
    (message, opts = {}) =>
      new Promise<string | null>((resolve) => {
        setValue(opts.defaultValue ?? '')
        setReq({ kind: 'prompt', message, opts, resolve })
      }),
    [],
  )

  const settle = (result: boolean | string | null) => {
    if (!req) return
    if (req.kind === 'confirm') req.resolve(result === true || typeof result === 'string')
    else req.resolve(typeof result === 'string' ? result : null)
    close()
  }

  return (
    <DialogCtx.Provider value={{ confirm, prompt }}>
      {children}
      {req && (
        <Modal title={req.opts.title ?? (req.kind === 'prompt' ? 'Girdi' : 'Onay')} onClose={() => settle(null)}>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              settle(req.kind === 'prompt' ? value : true)
            }}
            className="space-y-3"
          >
            <div className="font-mono text-[11px] leading-relaxed text-ink">{req.message}</div>
            {req.kind === 'prompt' && (
              <input
                autoFocus
                value={value}
                onChange={(e) => setValue(e.target.value)}
                placeholder={req.opts.placeholder}
                aria-label={req.opts.title ?? 'Girdi'}
                className="w-full border border-rule-hi bg-ground px-2 py-1 font-mono text-[11px] text-ink outline-none placeholder:text-tui-dim focus:border-rx/60"
              />
            )}
            <div className="flex justify-end gap-2">
              <button type="button" className={cancelCls} onClick={() => settle(null)}>
                {req.opts.cancelLabel ?? 'İptal'}
              </button>
              <button type="submit" className={req.opts.danger ? dangerCls : okCls}>
                {req.opts.confirmLabel ?? (req.kind === 'prompt' ? 'Tamam' : 'Onayla')}
              </button>
            </div>
          </form>
        </Modal>
      )}
    </DialogCtx.Provider>
  )
}

export function useDialog(): DialogApi {
  const ctx = useContext(DialogCtx)
  if (!ctx) throw new Error('useDialog must be used within <DialogProvider>')
  return ctx
}
