import { AdminPageShell, Placeholder } from '../../components/AdminPageShell'

export function EnrollAdminPage() {
  return (
    <AdminPageShell
      title="Agent Ekle"
      hint="Enrollment token seç/üret → işletim sistemi → kopyalanabilir kurulum komutu."
    >
      <Placeholder note="Agent Ekle sihirbazı yakında (S12.4) — /api/v1/enroll-tokens." />
    </AdminPageShell>
  )
}
