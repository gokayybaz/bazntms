// @ts-check
const { themes } = require('prism-react-renderer');
// bazNTMS dokümantasyon sitesi (Faz 7.4, Docusaurus).
// Kurulum: cd docs-site && npm install && npm run dev
//
// GitHub Pages (proje sayfası): https://gokayybaz.github.io/bazntms/
// Deploy: .github/workflows/docs.yml — docs-site/** değişince otomatik.
// Kendi alan adınızı kullanmak için: url + baseUrl'i güncelleyin ve
// docs-site/static/CNAME dosyasına alan adınızı yazın.
//
// Tema: ürünün kendi arayüzüyle (frontend/DESIGN.md — "htop çok-panelli
// terminal") aynı dil. Tek koyu tema, tek mono aile, kare köşe. Açık tema
// dalı yok — DESIGN.md kuralı ("Don't açık tema dalı açma").
const SITE_URL = 'https://gokayybaz.github.io';
const BASE_URL = '/bazntms/';
// Canlı demo: Docusaurus router'ının DIŞINDA, docs.yml'in build/demo altına
// kopyaladığı statik frontend build'i. Mutlak URL — Docusaurus'un kırık-link
// denetçisi bunu harici sayar (yoksa build-anında build/demo yok diye uyarır).
const DEMO_URL = `${SITE_URL}${BASE_URL}demo/`;

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'bazNTMS',
  tagline: 'Ağ trafiği izleme — hub + agent + cihaz entegrasyonları',
  url: SITE_URL,
  baseUrl: BASE_URL,
  customFields: { demoUrl: DEMO_URL },
  favicon: 'img/bazntms.svg',
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
      // Tek aile: JetBrains Mono her yerde. Uzun makale prose'u sistem
      // sans-serif'e döner (DESIGN.md typography.prose — ayrı web-font yok).
      tagName: 'link',
      attributes: {
        rel: 'stylesheet',
        href: 'https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400..700&display=swap',
      },
    },
  ],
  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      colorMode: {
        defaultMode: 'dark',
        disableSwitch: true,
        respectPrefersColorScheme: false,
      },
      // Kod blokları koyu — landing'deki <pre> ve dashboard ile aynı okuma
      // deneyimi. Her iki slot da aynı tema (tek tema).
      prism: { theme: themes.palenight, darkTheme: themes.palenight },
      navbar: {
        title: 'bazNTMS',
        logo: { src: 'img/bazntms.svg' },
        items: [
          { to: '/docs/installation', label: 'Kurulum', position: 'left' },
          { to: '/docs/reference/api', label: 'API', position: 'left' },
          { to: '/docs/reference/upgrading', label: 'Güncelleme', position: 'left' },
          { to: '/docs/reference/troubleshooting', label: 'Sorun Giderme', position: 'left' },
          {
            href: DEMO_URL,
            label: 'Demo',
            position: 'right',
            className: 'navbar__item--demo',
            target: '_blank',
            rel: 'noreferrer',
          },
          {
            to: '/docs/reference/changelog',
            label: 'v1.3.0',
            position: 'right',
            className: 'navbar__item--version',
          },
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
              { label: 'Sorun Giderme', to: '/docs/reference/troubleshooting' },
            ],
          },
          {
            title: 'Proje',
            items: [
              {
                label: 'GitHub',
                href: 'https://github.com/gokayybaz/bazntms',
              },
              { label: 'Değişiklik Günlüğü', to: '/docs/reference/changelog' },
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
