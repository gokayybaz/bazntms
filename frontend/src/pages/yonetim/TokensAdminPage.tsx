import { AdminPageShell } from '../../components/AdminPageShell'
import { Panel } from '../../components/Panel'
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
      <Panel title="API Token’ları">
        <TokensCard lockedSite={lockedSite} />
      </Panel>
    </AdminPageShell>
  )
}
