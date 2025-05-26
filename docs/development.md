# Development

## Prerequisites

* [Go] - for the API server
* [Node.js] - for the frontend
* [just] - to conveniently run common commands
* sh-like shell (e.g. sh, bash, busybox) - required by `just`
* [watchexec] - to auto-reload the API server
* [ExifTool] - for metadata extraction
* [FFmpeg] - for format conversion

**[Scoop] (Windows)**: `scoop install busybox just exiftool watchexec`

## Installation

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

## Running

Run both the API server and the UI server in separate terminals. They are set
up to work with each other by default with the API server running at port `8080`
and the UI server on port `5173`.

`just` is [just] as defined in the [prerequisites](#prerequisites).

1. Run the API and watch for changes
    ```sh
    just watch
    ```

2. Run the UI server in a separate terminal and watch for changes
    ```sh
    cd ui
    npm run dev
    ```
3. Open http://localhost:5173

[Go]: https://golang.org/
[Node.js]: https://nodejs.org/
[just]: https://github.com/casey/just
[watchexec]: https://github.com/watchexec/watchexec
[ExifTool]: https://exiftool.org/
[FFmpeg]: https://ffmpeg.org/

[Scoop]: https://scoop.sh/