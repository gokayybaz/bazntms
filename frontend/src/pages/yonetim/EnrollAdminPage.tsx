import { AdminPageShell } from '../../components/AdminPageShell'
import { Card } from '../../components/Card'
import { EnrollWizard } from '../../components/EnrollWizard'

export function EnrollAdminPage() {
  return (
    <AdminPageShell
      title="Agent Ekle"
      hint="Enrollment token üret → işletim sistemi → hedef makinede çalıştırılacak kurulum komutu. Token yalnızca üretilirken bir kez görünür."
    >
      <Card title="Yeni Agent Sihirbazı">
        <EnrollWizard />
      </Card>
    </AdminPageShell>
  )
}
