// @ts-check
// bazNTMS dokümantasyon sitesi (Faz 7.4, Docusaurus).
// Kurulum: cd docs-site && npm install && npm run dev
/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'bazNTMS',
  tagline: 'Ağ trafiği izleme — hub + agent + cihaz entegrasyonları',
  url: 'https://bazntms.example.com',
  baseUrl: '/',
  onBrokenLinks: 'warn',
  organizationName: 'gokayybaz',
  projectName: 'bazntms',
  i18n: { defaultLocale: 'tr', locales: ['tr'] },
  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: { sidebarPath: require.resolve('./sidebars.js') },
        blog: false,
        theme: { customCss: undefined },
      }),
    ],
  ],
};

module.exports = config;
