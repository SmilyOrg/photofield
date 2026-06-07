# Quick Start

## Simple Executable

1. [Download] and unpack a release to a folder with folders of photos.
2. Run `./photofield` or double-click on `photofield.exe` to start the server.
3. Open http://localhost:8080 and you should see folders in the directory displayed as collections.
4. You're done 🥳

[Download]: https://github.com/SmilyOrg/photofield/releases

Check out [Dependencies](/dependencies) that can enhance your experience and [Configuration](/configuration) to add custom
collections and configure it to your liking.

## Docker

Make sure you create an empty `data` directory in the working directory and that
you put some photos in a `photos` directory.

```sh
docker run -p 8080:8080 -v "$PWD/data:/app/data" -v "$PWD/photos:/app/photos:ro" ghcr.io/smilyorg/photofield
```

The cache database will be persisted to the `data` dir and the app should be
accessible at http://localhost:8080. It should show the `photos` collection by
default.

When ready, create a `configuration.yaml` in the `data` dir and see [Configuration](/configuration) to fully configure it to your liking.

## Docker Compose
  
Minimal example to get you started with both Photofield and Photofield AI (for semantic search and face detection).
Set up custom collections and tweak all kinds of details using the [Configuration](/configuration).

::: code-group
```yaml [compose.yaml]
services:
  photofield:
    image: ghcr.io/smilyorg/photofield:latest
    container_name: photofield
    ports:
      - 8080:8080
    volumes:
      - ./data:/app/data # Cache and configuration
      - /path/to/your/photos:/app/photos:ro # Your photos, read-only
    restart: unless-stopped

  photofield-ai:
    image: ghcr.io/smilyorg/photofield-ai:latest
    container_name: photofield-ai
    restart: unless-stopped
```

```yaml [configuration.yaml]
ai:
  host: http://photofield-ai:8081
```
:::