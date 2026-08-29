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
  headTags: [
    {
      tagName: 'link',
      attributes: { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
    },
    {
      tagName: 'link',
      attributes: {
        rel: 'preconnect',
        href: 'https://fonts.gstatic.com',
        crossOrigin: 'anonymous',
      },
    },
    {
      tagName: 'link',
      attributes: {
        rel: 'stylesheet',
        href: 'https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&family=JetBrains+Mono:wght@400;600;700&display=swap',
      },
    },
  ],
  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      colorMode: { defaultMode: 'dark' },
      navbar: {
        title: 'bazNTMS',
        logo: { src: 'img/bazntms.svg' },
        items: [
          { to: '/docs/installation', label: 'Kurulum', position: 'left' },
          { to: '/docs/reference/api', label: 'API', position: 'left' },
          { to: '/docs/reference/upgrading', label: 'Güncelleme', position: 'left' },
          {
            href: 'https://github.com/gokayybaz/bazntms',
            label: 'GitHub',
            position: 'right',
          },
        ],
      },
      footer: {
        style: 'dark',
        links: [
          {
            title: 'Dokümanlar',
            items: [
              { label: 'Kurulum', to: '/docs/installation' },
              { label: 'API Referansı', to: '/docs/reference/api' },
              { label: 'Yapılandırma', to: '/docs/reference/configuration' },
            ],
          },
          {
            title: 'Operasyon',
            items: [
              { label: 'Upgrade Runbook', to: '/docs/reference/upgrading' },
              { label: 'Felaket Kurtarma', to: '/docs/reference/dr' },
              { label: 'Mimari', to: '/docs/reference/architecture' },
            ],
          },
          {
            title: 'Proje',
            items: [
              {
                label: 'GitHub',
                href: 'https://github.com/gokayybaz/bazntms',
              },
            ],
          },
        ],
        copyright: 'bazNTMS · MIT Lisansı',
      },
    }),
  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          sidebarPath: require.resolve('./sidebars.js'),
          routeBasePath: '/docs',
        },
        blog: false,
        theme: { customCss: require.resolve('./src/css/custom.css') },
      }),
    ],
  ],
};

module.exports = config;
