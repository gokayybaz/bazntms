// Agent kurulum komutları — S12.4 sihirbazı. Release varlıkları:
// https://github.com/gokayybaz/bazntms/releases/latest/download/<asset>
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
        `curl -fsSL -o bazntms-agent ${REL}/bazntms-agent-linux-amd64`,
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
        seedYaml(p),
        `curl -fsSL -o /tmp/bazntms-agent.deb ${REL}/bazntms-agent-amd64.deb`,
        'sudo dpkg -i /tmp/bazntms-agent.deb',
      ].join('\n'),
  },
  {
    id: 'rpm',
    label: 'RHEL / Fedora (.rpm)',
    note: 'systemd servisi olarak kurulur ve otomatik başlar.',
    command: (p) =>
      [
        seedYaml(p),
        `curl -fsSL -o /tmp/bazntms-agent.rpm ${REL}/bazntms-agent-amd64.rpm`,
        'sudo rpm -i /tmp/bazntms-agent.rpm',
      ].join('\n'),
  },
  {
    id: 'macos',
    label: 'macOS (.pkg)',
    note: 'LaunchDaemon olarak kurulur. Config önceden yazıldığı için kurulum penceresi sormaz.',
    command: (p) =>
      [
        seedYaml(p),
        `curl -fsSL -o /tmp/bazntms-agent.pkg ${REL}/bazntms-agent-arm64.pkg`,
        'sudo installer -pkg /tmp/bazntms-agent.pkg -target /',
      ].join('\n'),
  },
  {
    id: 'windows',
    label: 'Windows (.msi)',
    note: 'Yönetici PowerShell. MSI özellikleriyle sessiz kurulum — servis otomatik başlar.',
    command: (p) =>
      [
        `curl.exe -fsSL -o "$env:TEMP\\bazntms-agent.msi" ${REL}/bazntms-agent-amd64.msi`,
        `msiexec /i "$env:TEMP\\bazntms-agent.msi" /qn HUBURL=${p.hubUrl} ENROLLTOKEN=${p.token}${p.site ? ` SITE=${p.site}` : ''}`,
      ].join('\n'),
  },
  {
    id: 'docker',
    label: 'Docker',
    note: 'Yayınlanmış imaj yok — repodan derlenir. Host ağı + NET_RAW/NET_ADMIN gerekir.',
    command: (p) =>
      [
        'docker build -f deploy/Dockerfile.agent -t bazntms-agent https://github.com/gokayybaz/bazntms.git',
        [
          'docker run -d --name bazntms-agent --restart=unless-stopped',
          '--network=host --cap-add=NET_RAW --cap-add=NET_ADMIN bazntms-agent',
          `-hub-url ${p.hubUrl} -enroll-token ${p.token}${p.site ? ` -site ${p.site}` : ''}`,
        ].join(' '),
      ].join('\n'),
  },
]
