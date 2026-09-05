import { AdminPageShell } from '../../components/AdminPageShell'
import { Card } from '../../components/Card'
import { UsersCard } from '../../components/UsersCard'

export function UsersAdminPage() {
  return (
    <AdminPageShell
      title="Kullanıcılar"
      hint="RBAC hesapları — rol, site kapsamı, etkin/pasif, şifre sıfırlama. Şifreler bcrypt ile saklanır."
    >
      <Card title="RBAC Kullanıcıları">
        <UsersCard />
      </Card>
    </AdminPageShell>
  )
}
