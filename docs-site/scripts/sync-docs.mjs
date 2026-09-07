// Repo kökündeki docs/*.md dosyalarını siteye senkron eder (Faz 7.4).
// Kullanım: node scripts/sync-docs.mjs  (npm run build öncesi otomatik)
//
// Kaynak → hedef:
//   docs/API.md            → docs/reference/api.md
//   docs/CONFIGURATION.md  → docs/reference/configuration.md
//   docs/ARCHITECTURE.md   → docs/reference/architecture.md
//   docs/UPGRADE-RUNBOOK.md → docs/reference/upgrading.md
//   docs/RELEASE-RUNBOOK.md → docs/reference/releasing.md
//   docs/DR-RUNBOOK.md     → docs/reference/dr.md
//   docs/TROUBLESHOOTING.md → docs/reference/troubleshooting.md
//   CHANGELOG.md (kök)     → docs/reference/changelog.md
//
// Kaynak dosyalar GitHub'da sürümlenir; site her build'de güncel içerik alır.
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const root = join(here, '..', '..');
const outDir = join(here, '..', 'docs', 'reference');
mkdirSync(outDir, { recursive: true });

// dir: kaynağın repo kökündeki dizini (varsayılan 'docs'); CHANGELOG kökte durur.
const docs = [
  { src: 'API.md', out: 'api.md', title: 'API Referansı', pos: 10 },
  { src: 'CONFIGURATION.md', out: 'configuration.md', title: 'Yapılandırma', pos: 11 },
  { src: 'ARCHITECTURE.md', out: 'architecture.md', title: 'Mimari', pos: 12 },
  { src: 'UPGRADE-RUNBOOK.md', out: 'upgrading.md', title: 'Güncelleme (Upgrade)', pos: 13 },
  { src: 'RELEASE-RUNBOOK.md', out: 'releasing.md', title: 'Sürüm Çıkarma (Release)', pos: 14 },
  { src: 'DR-RUNBOOK.md', out: 'dr.md', title: 'Felaket Kurtarma (DR)', pos: 15 },
  { src: 'TROUBLESHOOTING.md', out: 'troubleshooting.md', title: 'Sorun Giderme', pos: 16 },
  { src: 'CHANGELOG.md', dir: '.', out: 'changelog.md', title: 'Değişiklik Günlüğü', pos: 17 },
];

let copied = 0;
for (const d of docs) {
  const srcRel = d.dir === '.' ? d.src : `${d.dir ?? 'docs'}/${d.src}`;
  const body = readFileSync(join(root, srcRel), 'utf8');
  const fm = [
    '---',
    `title: ${d.title}`,
    `sidebar_position: ${d.pos}`,
    `custom_edit_url: https://github.com/gokayybaz/bazntms/edit/main/${srcRel}`,
    '---',
    '',
    `> Kaynak: [\`${srcRel}\`](https://github.com/gokayybaz/bazntms/blob/main/${srcRel}) — bu sayfa her build'de otomatik senkronize edilir.`,
    '',
  ].join('\n');
  writeFileSync(join(outDir, d.out), fm + body);
  copied++;
}
console.log(`[sync-docs] ${copied} referans sayfası senkronize edildi → docs/reference/`);
