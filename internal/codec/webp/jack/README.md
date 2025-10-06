# WebP Encoder Architecture Support

This directory contains the WebP encoder implementation using the `go-libwebp` library.

## Architecture Support

The transpiled WebP encoder (`transpiled/`) only supports the following architectures:
- linux: amd64, arm64
- darwin: amd64, arm64  
- windows: amd64, arm64

The dynamic WebP encoder (`dynamic/`) attempts to use the system's libwebp library at runtime and falls back to the transpiled version if unavailable.

On **unsupported architectures** (e.g., 32-bit systems like linux/386, windows/386, or platforms like openbsd), the WebP encoders will return an error. In these cases:

1. The dynamic encoder will return `ErrNotSupported` (library not found)
2. The transpiled encoder will return `ErrNotSupported` (architecture not supported)
3. The main `Encode` function will return a clear error message

Applications using this encoder should handle these errors and fall back to alternative formats like JPEG or PNG on unsupported architectures.

## Build Tags

The implementation uses Go build tags to conditionally compile the transpiled encoder only on supported platforms:

- `webp.go` (supported): Compiles on 64-bit linux, darwin, and windows
- `webp_unsupported.go`: Compiles on all other platforms and returns errors

This allows the project to build successfully on all architectures in the build matrix while gracefully degrading WebP support where it's not available.
