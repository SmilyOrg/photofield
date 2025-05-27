import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: "Photofield",
  description: "Self-Hosted Personal Photo Gallery",
  ignoreDeadLinks: [
    /^https?:\/\/localhost/,
  ],
  base: '/docs/',
  cleanUrls: true,
  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config

    logo: "/favicon-32x32.png",

    search: {
      provider: 'local',
    },

    editLink: {
      pattern: 'https://github.com/smilyorg/photofield/edit/main/docs/:path'
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
    ]

  },
})
