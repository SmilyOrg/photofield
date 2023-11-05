---
# https://vitepress.dev/reference/default-theme-home-page
layout: home

hero:
  name: "Photofield"
  text: Self-Hosted Personal Photo Gallery
  tagline: |
    A non-invasive local photo viewer with a focus on speed and simplicity.

  actions:
    - theme: brand
      text: Quick Start
      link: /quick-start
    - theme: alt
      text: Usage
      link: /usage
    - theme: alt
      text: Demo
      link: https://demo.photofield.dev/

features:
  - icon:
      src: /assets/features/seamless-zoom.gif
    title: |
      Seamless zoomable interface
    details: |
      Every view is zoomable if you ever need to see just a little more detail.

  - icon:
      src: /assets/features/progressive-load.gif
    title: |
      Progressive multi-resolution loading
    details: |
      The whole layout is progressively loaded from a low-res preview to a full quality photo.

  - icon:
      src: /assets/features/downloads.png
    title: |
      Simple to run
    link: https://github.com/SmilyOrg/photofield/releases
    details: |
      All the dependencies are packed into a single executable for Windows, Linux, and macOS. Just download and run. Docker images are also available.

  - icon:
      src: /assets/features/gource.jpeg
    title: |
      Open Source (MIT)
    link: /contributing
    details: |
      The source code is available on GitHub. Contributions are welcome.
      
  - icon:
      src: /assets/features/read-only.png
    title: |
      Non-destructive
    details: |
      Your files and directories are treated as an untouchable source of truth and are never modified. You can even use a read-only mount.

  - icon:
      src: /assets/features/file-scroll.gif
    title: |
      Fast indexing
    details: |
      Files are indexed practically at the speed of the file system at up to 10000 files/sec. Additional details are extracted as follow-up operations at up to 1000 files/sec.

  - icon:
      src: /assets/features/media-system.png
    title: |
      Flexible media system
    details: |
      Reuse hundreds of gigabytes of existing thumbnails or just let optimized versions be generated automatically to speed up display.

  - icon:
      src: /assets/features/map.jpg
    title: |
      Different views
    details: |
      Collections of photos can be displayed with different layouts, like an album, a timeline, or a map.

  - icon:
      src: /assets/features/slovenia.jpg
    title: |
      Reverse geolocation
    details: |
      Local, embedded reverse geolocation, negligible performance impact, no API calls needed. Supports ~50 thousand places powered by geoBoundaries.

  - icon:
      src: /assets/features/cat-eyes.jpg
    title: |
      Semantic search (alpha)
    details: |
      You can search for photo contents using words like "beach sunset", "a couple kissing", or "cat eyes". Needs to be configured as it requires running a separate AI server.

  - icon:
      src: /assets/features/tags.jpg
    title: |
      Tags (alpha)
    details: |
      You can tag photos with arbitrary tags, which are only stored in the database and not in the photos themselves. Needs to be enabled in the configuration.

  - icon:
      src: /assets/features/llama.gif
    title: |
      Basic video support
    details: |
      Videos are supported, however there are some usability quirks. Previously transcoded resolutions are supported, but there is no support for on-the-fly transcoding right now.

---

<script setup>
import Background from './components/Background.vue'
import bg from './assets/background.jpeg'
</script>

<Background :src="bg" />
