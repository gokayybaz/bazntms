import { AdminPageShell, Placeholder } from '../../components/AdminPageShell'

export function AuditAdminPage() {
  return (
    <AdminPageShell title="Denetim Kaydı" hint="Hash-zincirli denetim olayları + zincir bütünlüğü doğrulaması.">
      <Placeholder note="Denetim kaydı görünümü yakında (S12.5) — /api/v1/audit + /audit/verify." />
    </AdminPageShell>
  )
}
