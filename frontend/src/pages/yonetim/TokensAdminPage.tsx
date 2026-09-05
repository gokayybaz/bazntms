import { AdminPageShell, Placeholder } from '../../components/AdminPageShell'

export function TokensAdminPage() {
  return (
    <AdminPageShell title="API Token’ları" hint="Entegrasyonlar için Bearer token’ları — oluştur, listele, iptal et.">
      <Placeholder note="Token yönetimi yakında (S12.3) — /api/v1/tokens." />
    </AdminPageShell>
  )
}
