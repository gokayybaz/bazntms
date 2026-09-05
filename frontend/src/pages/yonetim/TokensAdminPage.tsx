import { AdminPageShell } from '../../components/AdminPageShell'
import { Card } from '../../components/Card'
import { TokensCard } from '../../components/TokensCard'

export function TokensAdminPage() {
  return (
    <AdminPageShell
      title="API Token’ları"
      hint="Entegrasyonlar için Bearer token’ları (Grafana, CI, script). Düz değer yalnızca oluşturulurken bir kez gösterilir; hash saklanır."
    >
      <Card title="API Token’ları">
        <TokensCard />
      </Card>
    </AdminPageShell>
  )
}
