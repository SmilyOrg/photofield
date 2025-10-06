# Dependencies

These tools are not strictly required, but if they are installed in your system, Photofield will use them to improve performance, metadata extraction, thumbnail generation, and video previews.

- [ExifTool]: Extracts metadata from many more formats than the embedded [goexif].
- [FFmpeg]: Generates video thumbnails and previews and adds support for more image formats (even basic RAW).
- [djpeg (libjpeg-turbo)]: Accelerates JPEG decoding of big images in cases where there are no other appropriate thumbnails available.
- [libwebp]: Enables high-performance WebP encoding via dynamic library loading. When available, the [go-libwebp] encoder can use the native libwebp dynamic shared library for faster encoding (`webp-jackdyn`) compared to pure Go implementations (`webp-jacktra`).

## Quick Install

### Docker

All dependencies are included in the [Docker image](/quick-start#docker) by default.

### Windows (scoop)
```sh
scoop install exiftool ffmpeg libjpeg-turbo libwebp
```

### macOS (brew)
```sh
brew install exiftool ffmpeg libjpeg-turbo webp
```

### Ubuntu/Debian
```sh
sudo apt install exiftool ffmpeg libjpeg-turbo-progs libwebp-dev
```

### CentOS/RHEL/Fedora
```sh
sudo dnf install exiftool ffmpeg libjpeg-turbo-utils libwebp
```

[djpeg (libjpeg-turbo)]: https://libjpeg-turbo.org/
[ExifTool]: https://exiftool.org/
[FFmpeg]: https://ffmpeg.org/
[goexif]: https://github.com/rwcarlsen/goexif
[go-libwebp]: https://git.sr.ht/~jackmordaunt/go-libwebp/
[libwebp]: https://developers.google.com/speed/webp
