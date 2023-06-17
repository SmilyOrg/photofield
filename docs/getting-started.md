## Install

Photofield is a single binary that can be run on most popular platforms. You can also
run it in a Docker container.

### Docker

The following command assumes you have two directories in the working directory:

* `data` - for the cache database and configuration
* `photos` - for the photos

```sh
.
├───data # for the cache database and configuration
└───photos # for the photos

.
├── data
│   └── configuration.yaml
└── photos
    └── selfie.jpeg
```

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
* ⚪ Set the `PHOTOFIELD_DATA_DIR` environment variable to change the path where
the app looks for the `configuration.yaml` and cache database

[Download and unpack a release]: https://github.com/SmilyOrg/photofield/releases
[exiftool]: https://exiftool.org/
