// Agent kurulum komutları — S12.4 sihirbazı. Release varlıkları:
// https://github.com/gokayybaz/bazntms/releases/latest/download/<asset>
//
// Her komut "sıfırdan kurulum" mantığıyla yazılır: agent zaten kuruluysa
// önce temizlenir (paket kaldır / servis durdur / MSI uninstall), sonra
// kurulur. Böylece aynı sürümü tekrar "kur" demek 1603 (Windows
// SECUREREPAIR) veya "already installed" (rpm) hatası vermez.
const REL = 'https://github.com/gokayybaz/bazntms/releases/latest/download'

export interface InstallParams {
  hubUrl: string // -hub-url (enrollment hedefi) — genelde location.origin
  token: string // enrollment token (ent_… veya statik bootstrap sırrı)
  site: string // opsiyonel site etiketi
}

export interface OSOption {
  id: string
  label: string
  note: string
  command: (p: InstallParams) => string
}

const seedYaml = (p: InstallParams) =>
  [
    "sudo mkdir -p /etc/bazntms",
    "sudo tee /etc/bazntms/agent.yml >/dev/null <<'EOF'",
    'hub:',
    `  url: ${p.hubUrl}`,
    `  token: ${p.token}`,
    ...(p.site ? ['agent:', `  site: ${p.site}`] : []),
    'EOF',
  ].join('\n')

export const OS_OPTIONS: OSOption[] = [
  {
    id: 'linux-bin',
    label: 'Linux (binary)',
    note: 'systemd gerektirmez — hızlı test / konteyner-dışı. Süreç ön planda çalışır.',
    command: (p) =>
      [
        // çalışan bir kopya varsa dur (curl -o "Text file busy" vermesin)
        'sudo pkill -x bazntms-agent 2>/dev/null || true',
        `curl -fL --retry 3 -o bazntms-agent ${REL}/bazntms-agent-linux-amd64`,
        'chmod +x bazntms-agent',
        `sudo ./bazntms-agent -hub-url ${p.hubUrl} -enroll-token ${p.token}${p.site ? ` -site ${p.site}` : ''}`,
      ].join('\n'),
  },
  {
    id: 'deb',
    label: 'Debian / Ubuntu (.deb)',
    note: 'systemd servisi olarak kurulur ve otomatik başlar. Config önceden yazıldığı için sihirbaz sormaz.',
    command: (p) =>
      [
        // varsa eski paketi tümüyle kaldır (servisi durdurur), sonra sıfırdan kur
        'sudo dpkg -P bazntms-agent 2>/dev/null || true',
        seedYaml(p),
        `curl -fL --retry 3 -o /tmp/bazntms-agent.deb ${REL}/bazntms-agent-amd64.deb`,
        'sudo dpkg -i /tmp/bazntms-agent.deb',
      ].join('\n'),
  },
  {
    id: 'rpm',
    label: 'RHEL / Fedora (.rpm)',
    note: 'systemd servisi olarak kurulur ve otomatik başlar.',
    command: (p) =>
      [
        // rpm -i zaten kuruluysa hata verir → önce erase et
        'sudo rpm -e bazntms-agent 2>/dev/null || true',
        seedYaml(p),
        `curl -fL --retry 3 -o /tmp/bazntms-agent.rpm ${REL}/bazntms-agent-amd64.rpm`,
        'sudo rpm -i /tmp/bazntms-agent.rpm',
      ].join('\n'),
  },
  {
    id: 'macos',
    label: 'macOS (.pkg)',
    note: 'LaunchDaemon olarak kurulur. Config önceden yazıldığı için kurulum penceresi sormaz.',
    command: (p) =>
      [
        // varsa çalışan daemon'u boşalt (pkg postinstall yeniden yükler)
        'sudo launchctl unload /Library/LaunchDaemons/local.bazntms.agent.plist 2>/dev/null || true',
        seedYaml(p),
        `curl -fL --retry 3 -o /tmp/bazntms-agent.pkg ${REL}/bazntms-agent-arm64.pkg`,
        'sudo installer -pkg /tmp/bazntms-agent.pkg -target /',
      ].join('\n'),
  },
  {
    id: 'windows',
    label: 'Windows (.msi)',
    note: 'Yönetici olarak açılmış PowerShell (cmd.exe DEĞİL). Proxy arkasındaysanız önce: $env:HTTPS_PROXY="http://proxy:port". Zaten kuruluysa önce kaldırılır — sessiz kurulum, servis otomatik başlar.',
    command: (p) =>
      [
        // -fsSL yerine -fL --retry: -s (silent) indirme hatasını gizliyordu.
        '$msi = "$env:TEMP\\bazntms-agent.msi"',
        `curl.exe -fL --retry 3 -o $msi "${REL}/bazntms-agent-amd64.msi"`,
        // aynı sürümü /i üstüne /i yapmak 1603/SECUREREPAIR verir → varsa önce uninstall
        "$old = Get-Package '*bazNTMS*' -ErrorAction SilentlyContinue",
        "if ($old) { Start-Process msiexec.exe -Wait -ArgumentList '/x', $old.FastPackageReference, '/qn', '/norestart' }",
        `Start-Process msiexec.exe -Wait -ArgumentList '/i', $msi, '/qn', 'HUBURL=${p.hubUrl}', 'ENROLLTOKEN=${p.token}'${p.site ? `, 'SITE=${p.site}'` : ''}`,
      ].join('\n'),
  },
  {
    id: 'docker',
    label: 'Docker',
    note: 'Yayınlanmış imaj yok — repodan derlenir. Host ağı + NET_RAW/NET_ADMIN gerekir.',
    command: (p) =>
      [
        'docker build -f deploy/Dockerfile.agent -t bazntms-agent https://github.com/gokayybaz/bazntms.git',
        // aynı isimli eski konteyner varsa "run" hata verir → önce sil
        'docker rm -f bazntms-agent 2>/dev/null || true',
        [
          'docker run -d --name bazntms-agent --restart=unless-stopped',
          '--network=host --cap-add=NET_RAW --cap-add=NET_ADMIN bazntms-agent',
          `-hub-url ${p.hubUrl} -enroll-token ${p.token}${p.site ? ` -site ${p.site}` : ''}`,
        ].join(' '),
      ].join('\n'),
  },
]
