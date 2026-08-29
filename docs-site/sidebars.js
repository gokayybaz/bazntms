/** @type {import('@docusaurus/types').SidebarsConfig} */
const sidebars = {
  docsSidebar: [
    'intro',
    'installation',
    'agent-deployment',
    {
      type: 'category',
      label: 'Referans',
      link: { type: 'generated-index', title: 'Referans Dokümanları' },
      items: [
        'reference/api',
        'reference/configuration',
        'reference/architecture',
        'reference/upgrading',
        'reference/dr',
      ],
    },
  ],
};

module.exports = sidebars;
