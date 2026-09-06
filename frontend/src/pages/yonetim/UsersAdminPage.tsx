import { AdminPageShell } from '../../components/AdminPageShell'
import { Card } from '../../components/Card'
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
      <Card title="RBAC Kullanıcıları">
        <UsersCard lockedSite={lockedSite} />
      </Card>
    </AdminPageShell>
  )
}
