import { AdminPageShell, Placeholder } from '../../components/AdminPageShell'

export function UsersAdminPage() {
  return (
    <AdminPageShell title="Kullanıcılar" hint="RBAC hesapları: rol, site kapsamı, etkin/pasif, şifre sıfırlama.">
      <Placeholder note="Kullanıcı yönetimi yakında (S12.2) — /api/v1/users." />
    </AdminPageShell>
  )
}
