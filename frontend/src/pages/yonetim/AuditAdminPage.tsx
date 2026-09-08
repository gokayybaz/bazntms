import { AdminPageShell } from '../../components/AdminPageShell'
import { Panel } from '../../components/Panel'
import { AuditCard } from '../../components/AuditCard'

export function AuditAdminPage() {
  return (
    <AdminPageShell
      title="Denetim Kaydı"
      hint="Her yönetim işlemi append-only hash-zincire yazılır (SHA-256, prev_hash → hash) — aktör türü, request-id, sonuç ve (yapılandırma değişikliklerinde) öncesi/sonrası durum farkı ile. Sır alanları maskelenir. Zincir bütünlüğü sunucuda doğrulanır."
    >
      <Panel title="Denetim Olayları">
        <AuditCard />
      </Panel>
    </AdminPageShell>
  )
}
