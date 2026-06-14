// VitePress 2.x config (see https://vitepress.dev/guide/migration-guide)
export default {
  title: "Photofield",
  description: "Self-Hosted Personal Photo Gallery",
  outDir: 'dist', // VitePress 2.x default changed to .vitepress/dist/
  ignoreDeadLinks: [
    /^https?:\/\/localhost/,
  ],
  base: '/docs/',
  cleanUrls: true,
  theme: {
    logo: "/favicon-32x32.png",
    search: {
      provider: 'local',
    },
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Quick Start', link: '/quick-start' },
    ],
    sidebar: [
      {
        text: 'Install',
        items: [
          { text: 'Quick Start', link: '/quick-start' },
          { text: 'Dependencies', link: '/dependencies' },
        ]
      },
      {
        text: 'Features',
        link: '/features',
        items: [
          { text: 'Layouts', link: '/features/layouts' },
          { text: 'Search', link: '/features/search' },
          { text: 'Tags', link: '/features/tags' },
          { text: 'Reverse Geolocation', link: '/features/geolocation' },
        ]
      },
      {
        text: 'Usage',
        link: '/usage',
        items: [
          { text: 'User Interface', link: '/user-interface' },
          { text: 'Configuration', link: '/configuration' },
          { text: 'Maintenance', link: '/maintenance' },
          { text: 'Performance', link: '/performance' },
        ]
      },
      {
        text: 'Contributing',
        link: '/contributing',
        items: [
          { text: 'Development', link: '/development' },
        ]
      },
      {
        text: 'About',
        items: [
          { text: 'License', link: '/license' },
          { text: 'Credits', link: '/credits' },
        ]
      }
    ],
    socialLinks: [
      { icon: 'github', link: 'https://github.com/SmilyOrg/photofield' }
    ],
  },
}
