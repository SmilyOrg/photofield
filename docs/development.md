# Development

## Prerequisites

### Core

* [Go] - for the API server
* [Node.js] - for the frontend
* [Task] - for running common commands (replaces `just`)

### Recommended

* **[watchexec]** - auto-reloads the API server during development for faster iteration (required for `task watch`)
* **[ExifTool]** - better metadata extraction, [goexif] is used otherwise
* **[FFmpeg]** - for video thumbnail creation and extended image format support
* **[djpeg (libjpeg-turbo)]** - optimized JPEG decoding for better performance

## Installation

### Core

1. **Install Go**: https://go.dev/doc/install
2. **Install Node.js**: https://nodejs.org/en/download/
3. **Install Task**: `go install github.com/go-task/task/v3/cmd/task@latest` or https://taskfile.dev/installation/

### Recommended

- **Windows (scoop)**: `scoop install watchexec exiftool ffmpeg libjpeg-turbo`
- **macOS (brew)**: `brew install watchexec exiftool ffmpeg libjpeg-turbo`
- **Ubuntu/Debian**: `sudo apt install watchexec-cli exiftool ffmpeg libjpeg-turbo-progs`
- **CentOS/RHEL/Fedora**: `sudo dnf install exiftool ffmpeg libjpeg-turbo-utils` and [watchexec install](https://github.com/watchexec/watchexec#install)

### Project Setup

1. Clone the repository
   ```sh
   git clone https://github.com/smilyorg/photofield.git
   cd photofield
   ```

2. Install dependencies (geo data, ui, docs)
   ```sh
   task deps
   ```

## Running

Both the API server and UI server run in development mode with hot reloading. The API server runs on port `8080` and the UI server on port `3000`.

### Using Task (Recommended)

1. Start the API server with auto-reload
   ```sh
   task watch
   ```

2. In a separate terminal, start the UI server
   ```sh
   task ui
   ```

3. Open http://localhost:3000

### Manual Commands

1. Start the API server
   ```sh
   go run .
   ```

2. In a separate terminal, start the UI server
   ```sh
   cd ui
   npm run dev
   ```

3. Open http://localhost:3000

### Migration from `just`

If you were previously using `just`, replace:
- `just watch` → `task watch`
- `just build` → `task build`
- `just ui` → `task ui`

Run `task` to see all available commands.

[Go]: https://golang.org/
[Node.js]: https://nodejs.org/
[Task]: https://taskfile.dev/
[watchexec]: https://github.com/watchexec/watchexec
[ExifTool]: https://exiftool.org/
[FFmpeg]: https://ffmpeg.org/
[djpeg (libjpeg-turbo)]: https://libjpeg-turbo.org/
[goexif]: https://github.com/rwcarlsen/goexif