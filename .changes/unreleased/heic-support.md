---
kind: Added
body: Add HEIC/HEIF, GIF image and MOV video format support via FFmpeg
time: 2026-01-25T21:00:00Z
---

HEIC (High Efficiency Image Container), HEIF (High Efficiency Image Format), GIF, and MOV (QuickTime) files are now fully supported. These formats, commonly used by iOS devices and other platforms, are decoded using FFmpeg when available. The extensions `.heic`, `.heif`, `.gif`, and `.mov` are now included in the default configuration, so Photofield will automatically index and display these files alongside your other media.
