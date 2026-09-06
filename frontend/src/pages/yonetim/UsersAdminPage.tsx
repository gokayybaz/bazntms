import { AdminPageShell } from '../../components/AdminPageShell'
import { Panel } from '../../components/Panel'
import { UsersCard } from '../../components/UsersCard'

export function UsersAdminPage({ lockedSite = '' }: { lockedSite?: string; multiSite?: boolean }) {
  return (
    <AdminPageShell
      title="Kullanıcılar"
      hint={
        lockedSite
          ? `RBAC hesapları — “${lockedSite}” sahasına kilitli (saha yöneticisi). Bu sahanın kullanıcılarını yönetirsiniz.`
          : 'RBAC hesapları — rol, site kapsamı, etkin/pasif, şifre sıfırlama. Şifreler bcrypt ile saklanır.'
      }
    >
      <Panel title="RBAC Kullanıcıları">
        <UsersCard lockedSite={lockedSite} />
      </Panel>
    </AdminPageShell>
  )
}
