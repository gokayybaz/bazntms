import { AdminPageShell } from '../../components/AdminPageShell'
import { Panel } from '../../components/Panel'
import { EnrollWizard } from '../../components/EnrollWizard'

export function EnrollAdminPage({
  lockedSite = '',
  multiSite = false,
  publicUrl = '',
}: {
  lockedSite?: string
  multiSite?: boolean
  publicUrl?: string
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
      <Panel title="Yeni Agent Sihirbazı">
        <EnrollWizard lockedSite={lockedSite} multiSite={multiSite} publicUrl={publicUrl} />
      </Panel>
    </AdminPageShell>
  )
}
