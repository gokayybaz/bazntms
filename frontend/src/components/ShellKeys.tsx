import { useMemo, useState } from 'react'
import { useRegisterKeys } from '../lib/KeymapContext'
import type { KeyAction } from '../lib/KeymapContext'
import { useHotkeys } from '../lib/useHotkeys'
import { useDialog } from '../lib/dialog'
import { HelpOverlay } from './HelpOverlay'

// ShellKeys — kabuğun global F-tuşu eylemlerini kaydeder (FnKeyBar bunları
// çizer) ve F1/? yardım overlay'ini yönetir. Providerlar içinde render edilir.
export function ShellKeys({ onLogout, onRefresh }: { onLogout: () => void; onRefresh: () => void }) {
  const { confirm } = useDialog()
  const [help, setHelp] = useState(false)

  const actions = useMemo<KeyAction[]>(
    () => [
      { key: 'F1', label: 'Yardım', handler: () => setHelp(true), order: 100 },
      { key: 'F5', label: 'Yenile', handler: onRefresh, order: 101 },
      {
        key: 'F10',
        label: 'Çıkış',
        order: 110,
        handler: async () => {
          if (await confirm('Oturumu kapatmak istiyor musunuz?', { danger: true, confirmLabel: 'Çıkış' })) onLogout()
        },
      },
    ],
    [confirm, onLogout, onRefresh],
  )
  useRegisterKeys(actions)
  useHotkeys([{ key: '?', handler: () => setHelp(true) }])

  return help ? <HelpOverlay onClose={() => setHelp(false)} /> : null
}
