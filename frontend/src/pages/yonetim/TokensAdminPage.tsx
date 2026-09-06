import { AdminPageShell } from '../../components/AdminPageShell'
import { Card } from '../../components/Card'
import { TokensCard } from '../../components/TokensCard'

export function TokensAdminPage({ lockedSite = '' }: { lockedSite?: string; multiSite?: boolean }) {
  return (
    <AdminPageShell
      title="API Token’ları"
      hint={
        lockedSite
          ? `Entegrasyon Bearer token’ları — “${lockedSite}” sahasına kilitli. Düz değer yalnızca oluşturulurken bir kez gösterilir.`
          : 'Entegrasyonlar için Bearer token’ları (Grafana, CI, script). Düz değer yalnızca oluşturulurken bir kez gösterilir; hash saklanır.'
      }
    >
      <Card title="API Token’ları">
        <TokensCard lockedSite={lockedSite} />
      </Card>
    </AdminPageShell>
  )
}
