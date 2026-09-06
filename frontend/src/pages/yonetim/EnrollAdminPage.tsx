import { AdminPageShell } from '../../components/AdminPageShell'
import { Card } from '../../components/Card'
import { EnrollWizard } from '../../components/EnrollWizard'

export function EnrollAdminPage({
  lockedSite = '',
  multiSite = false,
}: {
  lockedSite?: string
  multiSite?: boolean
}) {
  return (
    <AdminPageShell
      title="Agent Ekle"
      hint={
        lockedSite
          ? `Enrollment token → kurulum komutu. Token “${lockedSite}” sahasına kilitli.`
          : multiSite
            ? 'Enrollment token → kurulum komutu. Çoklu-saha modu: token için site zorunlu.'
            : 'Enrollment token üret → işletim sistemi → hedef makinede çalıştırılacak kurulum komutu. Token yalnızca üretilirken bir kez görünür.'
      }
    >
      <Card title="Yeni Agent Sihirbazı">
        <EnrollWizard lockedSite={lockedSite} multiSite={multiSite} />
      </Card>
    </AdminPageShell>
  )
}
