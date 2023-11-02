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
      text: Documentation
      link: /docs
    - theme: alt
      text: Demo
      link: https://demo.photofield.dev/

features:
  - title: |
      <img class="bleed" src="assets/seamless-zoom.gif">
      Seamless zoomable interface
    details: |
      Every view is zoomable if you ever need to see just a little more detail.

  - title: |
      <img class="bleed" src="assets/progressive-load.gif">
      Progressive multi-resolution loading
    details: |
      The whole layout is progressively loaded from a low-res preview to a full quality photo.

  - title: |
      <img class="bleed" src="assets/downloads.png">
      Simple to run
    details: |
      All the dependencies are packed into a single executable for Windows, Linux, and macOS. Just download and run. Docker images are also available.

  - title: |
      Open Source (MIT)
    link: /contributing
    details: |
      The source code is available on GitHub. Contributions are welcome.
      
  - title: |
      <img class="bleed" src="assets/read-only.png">
      Non-destructive
    details: |
      Your files and directories are treated as an untouchable source of truth and are never modified. You can even use a read-only mount.

  - title: |
      <img class="bleed" src="assets/file-scroll.gif">
      Fast indexing
    details: |
      Files are indexed practically at the speed of the file system at up to 10000 files/sec. Additional details are extracted as follow-up operations at up to 1000 files/sec.

  - title: |
      <img class="bleed" src="assets/media-system.png">
      Flexible media system
    details: |
      Reuse hundreds of gigabytes of existing thumbnails or just let optimized versions be generated automatically to speed up display.

  - title: |
      <img class="bleed" src="assets/map.jpg">
      Different views
    details: |
      Collections of photos can be displayed with different layouts, like an album, a timeline, or a map.

  - title: |
      <img class="bleed" src="assets/slovenia.jpg">
      Reverse geolocation
    details: |
      Local, embedded reverse geolocation, negligible performance impact, no API calls needed. Supports ~50 thousand places powered by geoBoundaries.

  - title: |
      <img class="bleed" src="assets/cat-eyes.jpg">
      Semantic search (alpha)
    details: |
      You can search for photo contents using words like "beach sunset", "a couple kissing", or "cat eyes". Needs to be configured as it requires running a separate AI server.

  - title: |
      <img class="bleed" src="assets/tags.jpg">
      Tags (alpha)
    details: |
      You can tag photos with arbitrary tags, which are only stored in the database and not in the photos themselves. Needs to be enabled in the configuration.

  - title: |
      <img class="bleed" src="assets/llama.gif">
      Basic video support
    details: |
      Videos are supported, however there are some usability quirks. Previously transcoded resolutions are supported, but there is no support for on-the-fly transcoding right now.

---

<script setup>
import Background from './components/Background.vue'
</script>

<Background src="assets/background.jpeg" />
