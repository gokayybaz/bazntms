// @ts-check
// bazNTMS dokümantasyon sitesi (Faz 7.4, Docusaurus).
// Kurulum: cd docs-site && npm install && npm run dev
//
// GitHub Pages (proje sayfası): https://gokayybaz.github.io/bazntms/
// Deploy: .github/workflows/docs.yml — docs-site/** değişince otomatik.
// Kendi alan adınızı kullanmak için: url + baseUrl'i güncelleyin ve
// docs-site/static/CNAME dosyasına alan adınızı yazın.
/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'bazNTMS',
  tagline: 'Ağ trafiği izleme — hub + agent + cihaz entegrasyonları',
  url: 'https://gokayybaz.github.io',
  baseUrl: '/bazntms/',
  trailingSlash: false,
  onBrokenLinks: 'warn',
  organizationName: 'gokayybaz',
  projectName: 'bazntms',
  i18n: { defaultLocale: 'tr', locales: ['tr'] },
  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: { sidebarPath: require.resolve('./sidebars.js'), routeBasePath: '/' },
        blog: false,
      }),
    ],
  ],
};

module.exports = config;
