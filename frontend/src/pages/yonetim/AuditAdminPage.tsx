import { AdminPageShell } from '../../components/AdminPageShell'
import { Card } from '../../components/Card'
import { AuditCard } from '../../components/AuditCard'

export function AuditAdminPage() {
  return (
    <AdminPageShell
      title="Denetim Kaydı"
      hint="Her yönetim işlemi append-only hash-zincire yazılır (SHA-256, prev_hash → hash). Zincir bütünlüğü sunucuda doğrulanır."
    >
      <Card title="Denetim Olayları">
        <AuditCard />
      </Card>
    </AdminPageShell>
  )
}
