<!-- HEADER -->
<br />
<p align="center">
  <a href="https://github.com/smilyorg/photofield">
    <img src="ui/public/android-chrome-192x192.png" alt="Logo" width="80" height="80">
  </a>

  <h3 align="center">Photofield</h3>

  <p align="center">
    A self-hosted non-invasive single-binary photo gallery with a focus on speed and simplicity.
    <br />
    <br />
    <a href="https://demo.photofield.dev">Demo</a> ·
    <a href="https://photofield.dev/quick-start">Quick Start</a> ·
    <a href="https://photofield.dev">Docs</a>
    <br />
    <br />
    <a href="https://github.com/SmilyOrg/photofield/releases"><img alt="GitHub Release" src="https://img.shields.io/github/v/release/smilyorg/photofield?sort=date&display_name=release&style=flat-square&labelColor=%23dd8888&color=%23eee"></a>
    <br />
    <a href="https://github.com/SmilyOrg/photofield/actions"><img alt="GitHub Actions Workflow Status" src="https://img.shields.io/github/actions/workflow/status/SmilyOrg/photofield/.github%2Fworkflows%2Fci.yml?style=flat-square&labelColor=%23dd8888&color=%23eee"></a>
    <a href="#built-with"><img alt="Tech stack: Golang and Vue 3" src="https://img.shields.io/badge/tech-Go%20%26%20Vue-white?style=flat-square&labelColor=%23dd8888&color=%23eee&icon=go"></a>
    <a href="https://github.com/SmilyOrg/photofield/stargazers"><img alt="GitHub Repo stars" src="https://img.shields.io/github/stars/smilyorg/photofield?style=flat-square&labelColor=%23dd8888&color=%23eee"></a>
    <a href="https://github.com/SmilyOrg/photofield/blob/main/LICENSE"><img alt="License" src="https://img.shields.io/github/license/SmilyOrg/photofield?style=flat-square&logoColor=white&logoSize=auto&labelColor=%23dd8888&color=%23eee"></a>
    <a href="https://discord.gg/qjMxfCMVqM"><img alt="Discord" src="https://img.shields.io/discord/1210642013997764619?style=flat-square&logo=discord&logoColor=white&logoSize=auto&label=chat&labelColor=%23dd8888&color=%23eee"></a>
  </p>
</p>

<!-- TABLE OF CONTENTS -->
<details open="open">
  <summary>Table of Contents</summary>
  <ol>
    <li>
      <a href="#about">About</a>
      <ul>
        <li><a href="#features">Features</a></li>
        <li><a href="#limitations">Limitations</a></li>
        <li><a href="#built-with">Built With</a></li>
      </ul>
    </li>
    <li><a href="#getting-started">Getting Started</a></li>
    <li><a href="#configuration">Configuration</a></li>
    <li><a href="#usage">Usage</a></li>
    <li><a href="#maintenance">Maintenance</a></li>
    <li><a href="#development-setup">Development Setup</a></li>
    <li><a href="#contributing">Contributing</a></li>
    <li><a href="#license">License</a></li>
    <li><a href="#acknowledgements">Acknowledgements</a></li>
  </ol>
</details>



## About

![Zoom to logo within a 43k images](docs/assets/logo-zoom.gif)

_Zoom to logo within a sample of 43k images from [open-images-dataset], i7-5820K 6-Core CPU, NVMe SSD_

Photofield is a photo viewer built to mainly push the limits of what is possible
in terms of the number of photos visible at the same time and at the speed at
which they are displayed. The goal is to be as fast or faster than Google Photos
on commodity hardware while displaying more photos at the same time. It is
non-invasive and can be used either completely standalone or complementing other
photo gallery software.


### Features

* **Seamless zoomable interface**. Every view is zoomable if you ever need to see just a little more detail.

  ![Seamless zoom to giraffe face](docs/assets/seamless-zoom.gif)

* **Progressive multi-resolution loading**. The whole layout is progressively loaded from a low-res preview to a full quality photo.

  ![Progressive load of a deer](docs/assets/progressive-load.gif)

* **Different layouts**. Collections of photos can be displayed with different
layouts.
![layout examples](docs/assets/layouts.png)
* **Semantic search using [photofield-ai]**. If enabled, you can search
  for photo contents using words like "beach sunset", "a couple kissing", or
  "cat eyes". ![semantic search for "cat eyes"](docs/assets/semantic-search.jpg)
* **Tagging (alpha)**. You can tag and search photos with arbitrary tags. If
  enabled, tags are stored in the cache database and can be used to filter
  photos.
* **Reverse geolocation**. Local, embedded reverse geolocation of ~50 thousand
  places via [tinygpkg] with negligible overhead supported in the Timeline and
  Flex layouts.
* **Flexible media/thumbnail system**. Stores small thumbnails using SQLite,
  uses FFmpeg for on-the-fly format conversion, extracts embedded thumbnails
  from JPEG files, re-uses Synology Moments / Photo Station thumbnails, and
  uses djpeg (libjpeg-turbo) to efficiently decode lower resolutions.
* **Single file binary**. The server is a single static binary with optional
  dependencies for easy and flexible deployment (Docker images also available).
* **Read-only file system based collections**. The original files are not
  touched. You are encouraged to even mount your photos as read-only to ensure
  this. The file system is the source of truth, everything else is just a more
  or less stale cache.
* **Fast indexing**. Thanks to [godirwalk], file indexing practically runs at
  the speed of the file system 1000-10000 files/sec on fast SSD and hot cache.
  EXIF metadata and [prominent color] are extracted as separate follow-up
  operations and run at up to ~200 files/sec and ~1000 files/sec on a fast system.
* **Video support**. Videos are supported along with multiple resolutions
  (if as pre-generated by e.g. Synology Moments), however on-the-fly transcoding
  is not supported.

### Limitations

* **Not optimized for many clients**. As a lot of the normally client-side
  state is kept on the server, you will likely run into CPU or Memory problems
  with more than a few simultaneous users.
* **No user accounts**. Not the focus right now. You can define separate
  collections for separate users based on the directory structure, but there is
  no authentication or authorization support.
* **Initial load can be slow**. All the photos need to be laid out when you
  first load a page in a specific window size and configuration, which can take
  some time with a slow CPU and cold HDD cache.
* **No permalinks**. Deep linking to images works, however if you remove the
  database or move the files around, the links may break. 

See the [documentation] for more information.

### Built With

* [Go] - API and server-side tile rendering
* [Canvas (tdewolff)](https://github.com/tdewolff/canvas) - vector rendering in
  Go
* [SQLite 3 (zombiezen)](https://github.com/zombiezen/go-sqlite) -
  fast single-file database/cache
* [Vue 3] - frontend framework
* [BalmUI] - Material UI components
* [OpenLayers] - in-browser tiled image rendering
* [OpenSeadragon] (honorary mention) - tiled image rendering library used in the past
* [+ more Go libraries](go.mod)
* [+ more npm libraries](ui/package.json)




## Getting Started

### Docker

Make sure you create an empty `data` directory in the working directory and that
you put some photos in a `photos` directory.

```sh
docker run -p 8080:8080 -v "$PWD/data:/app/data" -v "$PWD/photos:/app/photos:ro" ghcr.io/smilyorg/photofield
```

The cache database will be persisted to the `data` dir and the app should be
accessible at http://localhost:8080. It should show the `photos` collection by
default. For further configuration, create a `configuration.yaml` in the
`data` dir.

<details>
  <summary><code>docker-compose.yaml</code> example</summary>
  
  This example binds the usual Synology Moments photo directories and assumes
  a certain path structure, modify to your needs graciously. It also assumes you
  have configured the `/photo` and `/user` directories as collections in
  the `configuration.yaml`.
  ```yaml
  version: '3.3'
  services:

    photofield:
      image: ghcr.io/smilyorg/photofield:latest
      ports:
        - 8080:8080
      volumes:
        - /volume1/docker/photofield/data:/app/data
        - /volume1/photo/:/photo:ro
        - /volume1/homes/ExampleUser/Drive/Moments:/exampleuser:ro
  ```
</details>

### Binaries

1. [Download and unpack a release].
2. Run `./photofield` or double-click on `photofield.exe` to start the server.
3. Open http://localhost:8080, folders in the working directory will be
displayed as collections. 🎉

* 📝 Create a `configuration.yaml` in the working dir to configure the app
* 🕵️‍♀️ Install [exiftool] and add it to PATH for better metadata support
(esp. for video)
* ⚡ Install [djpeg (libjpeg-turbo)] for faster JPEG processing (optional but recommended)
* ⚪ Set the `PHOTOFIELD_DATA_DIR` environment variable to change the path where
the app looks for the `configuration.yaml` and cache database

[Download and unpack a release]: https://github.com/SmilyOrg/photofield/releases
[exiftool]: https://exiftool.org/



## Configuration

You can configure the app via `configuration.yaml`.

The location of the file depends on the installation method, see
[Getting Started].

The following is a minimal `configuration.yaml` example, see [`defaults.yaml`]
for all options.

```yaml
collections:
  # Normal Album-type collection
  - name: Vacation Photos
    dirs:
      - /photo/vacation-photos

  # Timeline collection (similar to Google Photos)
  - name: My Timeline
    layout: timeline
    dirs:
      - /photo/myphotos
      - /exampleuser

  # Create collections from sub-directories based on their name
  - expand_subdirs: true
    expand_sort: desc
    dirs:
      - /photo
```

## Development Setup

### Prerequisites

* [Go] - for the backend / API server
* [Node.js] - for the frontend
* [Task] - to run common commands conveniently via Taskfile
* [watchexec] - for auto-reloading the Go server
* [exiftool] - for testing metadata extraction
* [djpeg (libjpeg-turbo)] - for optimized JPEG decoding (optional but recommended for better performance)

**[Scoop] (Windows)**: `scoop install go-task exiftool watchexec`

### Installation

1. Clone the repo
   ```sh
   git clone https://github.com/smilyorg/photofield.git
   ```
2. Install Go dependencies
   ```sh
   go get
   ```
3. Install NPM packages
   ```sh
   cd ui
   npm install
   ```

### Running

Run both the API server and the UI server in separate terminals. They are set
up to work with each other by default with the API server running at port `8080`
and the UI server on port `5173`.

`task` is [Task] as defined in the [prerequisites](#prerequisites).

#### API

* `task watch` the source files and auto-reload the server using [watchexec]
* or `task run` the server

#### UI

* `task ui` to start a hot-reloading development server
* or run from within the `ui` folder
  ```sh
  cd ui
  npm run dev
  ```



## Contributing

Pull requests are welcome. For major changes, please open an issue first to
discuss what you would like to change.



## License

Distributed under the MIT License. See `LICENSE` for more information.



## Acknowledgements
* [Open Images Dataset][open-images-dataset]
* [geoBoundaries](https://www.geoboundaries.org/) for geographic boundary data used for reverse geolocation
* [sams96/rgeo](https://github.com/sams96/rgeo) for previous reverse geolocation implementation and inspiration
* [Best-README-Template](https://github.com/othneildrew/Best-README-Template)
* [readme.so](https://readme.so/)

[Configuration]: #configuration
[documentation]: https://photofield.dev

[open an issue]: https://github.com/SmilyOrg/photofield/issues
[Getting Started]: #getting-started
[`defaults.yaml`]: defaults.yaml

[open-images-dataset]: https://opensource.google/projects/open-images-dataset

[Scoop]: https://scoop.sh/
[Task]: https://taskfile.dev/
[watchexec]: https://github.com/watchexec/watchexec

[Go]: https://golang.org/
[godirwalk]: https://github.com/karrick/godirwalk
[prominent color]: https://github.com/EdlinOrg/prominentcolor

[OpenLayers]: https://openlayers.org/
[OpenSeadragon]: https://openseadragon.github.io/
[Node.js]: https://nodejs.org/
[Vue 3]: https://v3.vuejs.org/
[BalmUI]: https://next-material.balmjs.com/
[photofield-ai]: https://github.com/smilyorg/photofield-ai
[tinygpkg]: https://github.com/smilyorg/tinygpkg
[djpeg (libjpeg-turbo)]: https://libjpeg-turbo.org/
