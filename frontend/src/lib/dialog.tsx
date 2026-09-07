import { createContext, useCallback, useContext, useState } from 'react'
import type { ReactNode } from 'react'
import { Modal } from '../components/Modal'

// TUI diyalog yardımcıları — native confirm()/prompt() yerine. Uygulama kökünde
// <DialogProvider>, ekranlar `const { confirm, prompt, form } = useDialog()` ile
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

export interface FormField {
  key: string
  label: string
  type?: 'text' | 'number' | 'password' | 'textarea' | 'select'
  defaultValue?: string
  options?: string[]
  placeholder?: string
}

export interface FormOptions extends ConfirmOptions {
  fields: FormField[]
}

interface DialogApi {
  confirm: (message: ReactNode, opts?: ConfirmOptions) => Promise<boolean>
  prompt: (message: ReactNode, opts?: PromptOptions) => Promise<string | null>
  /** Çok-alanlı TUI form dialog'u — art arda ask() zincirlerinin yerine. */
  form: (opts: FormOptions) => Promise<Record<string, string> | null>
}

type Request =
  | { kind: 'confirm'; message: ReactNode; opts: ConfirmOptions; resolve: (v: boolean) => void }
  | { kind: 'prompt'; message: ReactNode; opts: PromptOptions; resolve: (v: string | null) => void }
  | { kind: 'form'; opts: FormOptions; resolve: (v: Record<string, string> | null) => void }

const DialogCtx = createContext<DialogApi | null>(null)

const btnBase = 'border px-3 py-1 font-mono text-[11px] uppercase tracking-[0.04em] transition'
const cancelCls = `${btnBase} border-rule-hi text-tui-dim hover:text-ink-hi hover:border-ink-hi`
const okCls = `${btnBase} border-rx bg-rx text-ground hover:opacity-90`
const dangerCls = `${btnBase} border-rose-400 bg-rose-400 text-ground hover:opacity-90`
const fieldCls = 'w-full border border-rule-hi bg-ground px-2 py-1 font-mono text-[11px] text-ink outline-none placeholder:text-tui-dim focus:border-rx/60'

export function DialogProvider({ children }: { children: ReactNode }) {
  const [req, setReq] = useState<Request | null>(null)
  const [value, setValue] = useState('')
  const [formVals, setFormVals] = useState<Record<string, string>>({})

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

  const form = useCallback<DialogApi['form']>(
    (opts) =>
      new Promise<Record<string, string> | null>((resolve) => {
        setFormVals(Object.fromEntries(opts.fields.map((f) => [f.key, f.defaultValue ?? (f.type === 'select' ? f.options?.[0] ?? '' : '')])))
        setReq({ kind: 'form', opts, resolve })
      }),
    [],
  )

  const cancel = () => {
    if (!req) return
    if (req.kind === 'confirm') req.resolve(false)
    else req.resolve(null)
    close()
  }
  const submit = () => {
    if (!req) return
    if (req.kind === 'confirm') req.resolve(true)
    else if (req.kind === 'prompt') req.resolve(value)
    else req.resolve(formVals)
    close()
  }

  const title = req?.opts.title ?? (req?.kind === 'confirm' ? 'Onay' : req?.kind === 'form' ? 'Form' : 'Girdi')

  return (
    <DialogCtx.Provider value={{ confirm, prompt, form }}>
      {children}
      {req && (
        <Modal title={title} onClose={cancel} width={req.kind === 'form' ? 'max-w-lg' : 'max-w-md'}>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              submit()
            }}
            className="space-y-3"
          >
            {req.kind !== 'form' && <div className="font-mono text-[11px] leading-relaxed text-ink">{req.message}</div>}

            {req.kind === 'prompt' && (
              <input
                autoFocus
                value={value}
                onChange={(e) => setValue(e.target.value)}
                placeholder={req.opts.placeholder}
                aria-label={title}
                className={fieldCls}
              />
            )}

            {req.kind === 'form' && (
              <div className="space-y-2">
                {req.opts.fields.map((f, i) => (
                  <label key={f.key} className="block space-y-0.5 font-mono text-[10px] uppercase tracking-[0.04em] text-tui-dim">
                    {f.label}
                    {f.type === 'select' ? (
                      <select
                        autoFocus={i === 0}
                        value={formVals[f.key] ?? ''}
                        onChange={(e) => setFormVals((v) => ({ ...v, [f.key]: e.target.value }))}
                        className={fieldCls}
                      >
                        {(f.options ?? []).map((o) => (
                          <option key={o} value={o}>
                            {o}
                          </option>
                        ))}
                      </select>
                    ) : f.type === 'textarea' ? (
                      <textarea
                        autoFocus={i === 0}
                        rows={2}
                        value={formVals[f.key] ?? ''}
                        onChange={(e) => setFormVals((v) => ({ ...v, [f.key]: e.target.value }))}
                        placeholder={f.placeholder}
                        className={fieldCls}
                      />
                    ) : (
                      <input
                        autoFocus={i === 0}
                        type={f.type === 'number' ? 'number' : f.type === 'password' ? 'password' : 'text'}
                        value={formVals[f.key] ?? ''}
                        onChange={(e) => setFormVals((v) => ({ ...v, [f.key]: e.target.value }))}
                        placeholder={f.placeholder}
                        className={fieldCls}
                      />
                    )}
                  </label>
                ))}
              </div>
            )}

            <div className="flex justify-end gap-2">
              <button type="button" className={cancelCls} onClick={cancel}>
                {req.opts.cancelLabel ?? 'İptal'}
              </button>
              <button type="submit" className={req.opts.danger ? dangerCls : okCls}>
                {req.opts.confirmLabel ?? (req.kind === 'confirm' ? 'Onayla' : req.kind === 'form' ? 'Kaydet' : 'Tamam')}
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
