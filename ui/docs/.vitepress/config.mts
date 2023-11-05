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
        ]
      },
      {
        text: 'Usage',
        link: '/usage',
        items: [
          { text: 'User Interface', link: '/user-interface' },
          { text: 'Configuration', link: '/configuration' },
          { text: 'Maintenance', link: '/maintenance' },
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
