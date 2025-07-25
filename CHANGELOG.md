# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html),
and is generated by [Changie](https://github.com/miniscruff/changie).



## [v0.19.0] - 2025-07-24 - Lotsa bug fixes and performance improvements

It's been a while. No big new features in this release, but a lot of bug fixes and performance improvements.

Internal database thumbnail generation is now more aggressive, so if you have slow rendering, try clicking `Reindex color` in the expanded collection settings to regenerate thumbnails.

### Added
* Make use of djpeg / libjpeg-turbo if installed to make image loading faster in the absence of better thumbnails. libjpeg-turbo can partially decode smaller resolutions of JPEGs, which can be many times faster than loading the full resolution and then resizing.
* Add support for custom paths to external tools like djpeg, exiftool, and ffmpeg in the config file
* Preload next photo in zoomed in view
* Collections with 100k+ items should now render faster, especially on slower servers
* Faster zoomed-in photo navigation with predictive range loading and improved caching

### Fixed
* Fast scrolling leading to cut off rendering at the edge
* Clicks/taps unintentionally zooming into photos
* Zooming being sometimes too fast
* Fixed the top right progress spinner being invisible in the light theme
* Fixed glitch with an animation navigating between some photos where there should be none

### Development

* Lots of dependencies updated, including Vite, Vue, and others
* Update development server port from 3000 to 5173 to align with Vite defaults
* Add photogen test code for generating test images for unit tests (finally!)

[v0.19.0]: https://github.com/SmilyOrg/photofield/compare/v0.18.0...v0.19.0




## [v0.18.0] - 2025-03-02 - Wider platform support and improved build process

There is no new functionality in this release, but the build and release process
has been refactored to use Taskfile and changie -- a nice learning experience.

The entire changelog has been extracted to CHANGELOG.md, which is now the
canonical source of release notes.

Releases are now built for many more platforms than before, let me know
if any of them come in handy or have issues :)

### Added
* Refactored the build and release process to use Taskfile and changie

[v0.18.0]: https://github.com/SmilyOrg/photofield/compare/v0.17.1...v0.18.0




## [v0.17.1] - 2024-11-02 - Scrolling fixes

### Fixed

* Lots of scrolling glitches with just a few photos in collection due to the height not being fully tested
* Max scrolling position not being always correctly handled
* The scrolling position sometimes changing 2s after scrolling (due to persistent scrolling glitches)
* Being able to scroll out of bounds

[v0.17.1]: https://github.com/SmilyOrg/photofield/compare/v0.17.0...v0.17.1



## [v0.17.0] - 2024-10-27 - Photo details, dark mode, scrollbar, open in album

### New

- **Photo details view** with a few basic details, like date, tags (if enabled), 
  photo name & dimensions, and location.
- **Dark mode**: By default, it uses the system preference, but it's possible to
  switch in the display settings.
- **Custom scrollbar**: Inspired by other galleries, it shows years/months/days
  and has better support for longer galleries with "precision mode".
- **Open Image in Album** context menu entry. This is useful to open an image in
  album/time-based context from e.g. a search or map view.

### Changed

- **Scroll persistence**: The scrollable layouts (album, timeline, highlights,
  flex) now persist the scroll position via a file-based anchor in the URL (`f`
  parameter). Refreshing or sharing the URL within a long album should therefore
  retain the viewed position. This supports the "Open Image in Album" feature by
  scrolling it into view on page load.
- **Settings menu**: The settings (cog wheel) now close on selection.
- **PWA installation**: Installing the website as a Progressive Web App (PWA)
  should look nicer now.

### Fixed

- **Search improvements**: Addressed issues with the search bar and input
  behavior to ensure smoother scrolling and more predictable focus handling.
- **Scene reload**: Do not reload the scene on height change for album/timeline
  as it doesn't affect the display.

[v0.17.0]: https://github.com/SmilyOrg/photofield/compare/v0.16.0...v0.17.0



## [v0.16.0] - 2024-09-09 - Batch edit tags and faster collections

### Added

- Add `skip_collection_counts` config option for faster startup in some cases
- Batch edit tags by selecting multiple photos with Ctrl/Cmd and clicking "#"
- Loading very large collections (100K .. 10M+ files) now works

### Changed

- Optimized initial loading of large collections
- Finally fixed "determinant of affine transformation matrix is zero" error
- Fixed unit in loading spinner
- Fixed the file counter not being updated while rescan was underway
- Search or other scenes using tags auto-update when tags are added or removed
- Search is now enabled even if AI is not enabled (e.g. for tag search)
- Fixed a bug where collections with dirs that are a prefix of other dirs would
  in some cases erroneously also list files in these other dirs (e.g. /vacation
  would include /vacation2 or /vacation-eu)

### Breaking changes

- There is a hard limit of 50000 source directories per loaded view for now.
  Please let me know if you encounter this limit as it can be increased.

[v0.16.0]: https://github.com/SmilyOrg/photofield/compare/v0.15.1...v0.16.0



## [v0.15.2] - 2024-08-19 - Timezone, auto-crop, filters, tweaks

### Added

- New documentation pages
- Added CHANGELOG.md
- New search filters for filtering by date, query similarity, deduplication (see the new docs)

### Changed

- Increase the default image memory cache size to 1 GiB
- Fixed default photo width and height across all layouts
- Later collections with the same name now override earlier collections,
  allowing configuration of specific collections of an expanded collection
- Cleaned up README now that there is better documentation

## Fixed

- Timezones are now accounted for in cases the camera writes both a date with
  and without a timezone without specifying it explicitly
- Black bars in letter/pillarboxed JPEG thumbnails generated by some cameras are
  now cropped automatically
- Image cache size configuration did not apply properly before even through
  defaults.yaml implied it was possible
- Fixed the default width and height for photos for all layouts in case metadata
  is not present, fixes broken albums in some cases
- Fixed excessive CPU usage and glitches due to file watches watching the
  database
- Fixed slow home page loading due to unnecessary database queries

[v0.15.2]: https://github.com/SmilyOrg/photofield/compare/v0.15.1...v0.15.2



## [v0.15.1] - 2024-05-28 - Timezone & build fixes, and experimental layouts

### Added

* New **Flex layout** using a variant of Knuth & Plass, which makes a smarter layout, especially for odd aspect ratios
* New experimental **Highlights layout** with a twist! It varies the layout row height based on the "sameness" of the photos. The idea is to make travel photo collections more skimmable by shrinking similar and repeating photos. Requires AI to be enabled.

### Changed

* Upgrade pyroscope-go for Go 1.22
* Fix `GPSDateTime` handling by not letting it override already-detected timezone-aware dates

[@Terrance](https://github.com/Terrance) made their first contribution in https://github.com/SmilyOrg/photofield/pull/101 🥳



## [v0.15.0] - 2024-02-18 - Polished interaction with more zoomy bits

### Changed
- Left/right/down interaction now works a little more smoothly similar to mobile galleries
- Clicking on a photo now zooms into it directly
- Lots of other tweaks

### Fixed
- Video is finally controllable
- Back button should now work a little more like expected, esp. on mobile
- Fix dates not showing while scrolling on mobile
- Selection works on map view now (but it's still useless)
- A bunch of map fixes



## [v0.14.2] - 2024-01-07

### Added

Added `PHOTOFIELD_DOCS_PATH` for rewriting paths in documentation so that the same deployment can host the docs at https://photofield.dev and the demo at https://demo.photofield.dev/

### Fixed

Fixed scene constantly loading while contents are being indexed.



## [v0.14.1] - 2024-01-06 - ARM Docker images

Docker images are now built as multiarch - x64 and arm.

Good for cheaper cloud servers, maybe also M1/M2/M3 Macs (let me know!)



## [v0.14.0] - 2024-01-06 - Logs, autoreload, errors, fixes

Various quality of life improvements and bug fixes.

- **Logs**: timestamp is not shown anymore as I didn't find it useful (logging systems usually provide their own). Let me know if it was useful to you. All exposed urls are now shown on startup, so it's more obvious how to access it.
    ```
    app running (api under /api)
      local    http://127.0.0.1:8080
      network  http://172.22.0.27:8080
    ```
- **Loading & errors**: there are more loading indicators, especially for loading collections, so that it doesn't just show a white screen. If the connection to the server drops, there is also a more obvious error message and status shown. Same if you run it without any collections configured. Fixes #84.

- **Autoreload**: the configuration should be automatically reloaded on yaml file change, so restarting is not required anymore. The same applies to collections with `expand_subdirs: true`, if you change the dirs the server is automatically reloaded.

- **Rescan applies faster**: previously the view was sometimes cached "too much" leading to it showing the same thing, even after a rescan and page reload. Switching back to the photos or refreshing the page should now always show the up to date view, resolving point 1 in #81 from the scanning point of view.

- **Cleaner exit**: if you Ctrl+C or otherwise "soft close" (`SIGTERM`) the app, it should close the database cleanly, so `-shm` and `-wal` files are not left laying around anymore. They are still left if you forcibly close the app (`SIGKILL`) as that cannot be handled.

- **Layout fixes**: timeline view shouldn't overlap with the app bar on top anymore. Small collections should now be top-left aligned and not centered. The "Wall" layout was bugged and should now work as expected again.

- **Add Playwright e2e tests**: this should make it easier to make sure that some of the features above keep working as expected in the future. Still a bit of a proof of concept though.

- **File reorganization**: the UI, docs, and e2e tests are now there as three independent nodejs projects so that you don't have to install all of them to work on just one.



## [v0.13.0] - 2023-11-06 - Embedded docs MVP

It's basically the same as the README right now, but more room for growth.

It's accessible by clicking on the question mark in the top right corner of the app in the collections / home page.

Eventually this should be hosted on https://photofield.dev as the main documentation source with the README being trimmed down to the basics.



## [v0.12.0] - 2023-10-26 - Map view 🗺

✨ It shows _ALL_ the photos in the collection/album on the map near to where they were taken ✨

### Considerations
- Works for photos with embedded exif GPS coordinates
- Since often there are many photos taken in close proximity, there is a balance struck between "not overlapping with other photos", "distance from taken location", and "displayed size".
- Uses OpenStreetMap for the background map for now, so it's not fully self-hosted.
  - The photos themselves are rendered locally, but the background map layer is loaded from OSM

It's still somewhat rough, you might need to refresh after it's done loading. If you haven't reindexed metadata since [v0.11.0](https://github.com/SmilyOrg/photofield/releases/tag/v0.11.0), you will need to do it for the GPS coordinates to be picked up

### Changes 

* Fixed gradual browser slowdown bugs that have persisted for a while, especially noticeable on low-powered devices.



## [v0.11.1] - 2023-10-09 - Reverse geolocation by default

Reverse geolocation is now enabled by default using the custom-built [tinygpkg package](https://github.com/SmilyOrg/tinygpkg).

### Changes

- Instant startup compared to ~10s before
- Increased number of places from 6018 to 49689, powered by https://www.geoboundaries.org/
- Adds only 16MB to build (uncompressed)
- Negligible impact on performance
- Still timeline-only


If you don't see any locations, you might need to reindex your metadata.



## [v0.11.0] - 2023-08-12 - Reverse geolocation

Location names shown in the timeline view via embedded local-only geolocation package rgeo!

Has to be enabled explicitly via `configuration.yaml` due to the relatively long startup time (10s or more) and higher memory usage (~1.5GB), see [defaults.yaml](defaults.yaml#L58-L66) for details.

Thanks to [@zach-capalbo](https://github.com/zach-capalbo) for the contribution and apologies for the delayed release notes!



## [v0.10.4] - 2023-06-27 - Better loading and blurry photos fix

Tuned image loading to be more efficient and less likely to show blurry photos.



## [v0.10.3] - 2023-05-18 - Tag search

You can filter photos in the collection by searching for `tag:TAG`.

For example, you can search for `tag:fav` to only show favorited photos, or `tag:hello tag:world` to only show photos with both `hello` and `world` tags. This is an early version of filtering and should be more user-friendly in the future.

Related image search #62, tag filtering, and semantic search #40 currently cannot be combined and only one will be applied (in that order).



## [v0.10.2] - 2023-05-16 - EXIF tags

### Added

Automatically add tags from EXIF data.

The only EXIF tags are the currently hardcoded `make` and `model`, and they are added to the file as `exif:make:<make>` and `exif:model:<model>` tags respectively.

To enable the automatic addition of these tags, you need to enable it in the config.
```yaml
tags:
  enable: true
  exif:
    enable: true
```

### Fixed

Fix `tags.enable` to now actually work, as only `enabled` worked before. `enabled` will keep working for now to not break existing configurations.



## [v0.10.1] - 2023-05-14 - Related image search

* Adds **Find Similar Images** to the context menu
* Adds a basic search query parser currently supporting `img:ID` for searching related images
* This can be extended later filtering for tags, arithmetic/boolean logic, etc.



## [v0.10.0] - 2023-05-07 - Tags

In their current form the tags can be a bit volatile, so consider them alpha-level and don't get too attached.

_**Note:** You need to enable tagging explicitly first in the `tags` section of the configuration._

They have some cool features in their current form. The intention here is to form a foundation on top of which many other features can be built. See [H_Q_'s comment thread from a while ago](https://old.reddit.com/r/selfhosted/comments/x601ql/photofield_v05_released_google_photos_alternative/incw9bp/) for details and ideas.

1. **Selection.** You can select photos now via Ctrl + Click or Ctrl + (Shift) + Drag. Selections are handled as "system tags" (tags with `sys:` prefix) and persist across refreshes, restarts, and across browsers (as long as you keep the link and don't delete the database). You cannot do anything else with the selection right now, so functionally they're more of a tech demo (i.e. useless). This will make it easier to implement #4 however.

2. **Tag picker.** You can click the # button to show a photo's tags (excluding "system tags"). You can add and remove tags as you please using the multiselect with auto-complete. The ΓÖÑ button toggles a `fav` tag as a simple "liking" functionality.

3. **Range tree tagging.** This is an interesting implementation detail that makes it so that in some cases tags can be stored in a compressed "id range tree" manner, so that you can theoretically select/tag thousands of photos all at once. This should make it a lot more efficient to also add e.g. location tags to large subsets of photos for example as part of #59 

I'm not 100% happy with the way tag IDs/revisions/etc. work right now, so that's something to look at in the future, but it's alright as a first draft.

Clearly the biggest missing part is search and/or filtering, which will actually make them a bit more useful than what is there. But let's see :)

Also bonus: fixes #21 in a hacky way (show the hand cursor for all canvas interaction regardless of photo or background)



## [v0.9.4] - 2023-04-23

* Add FFmpeg hack to make RAW files brighter



## [v0.9.3] - 2023-04-23

* Dependency updates
* Fix `panic: determinant of affine transformation matrix is zero` for some cases



## [v0.9.2] - 2023-04-11

* Disable AI indexing if there is no visual host
* Remove search input on collections view
* Tuning to pick higher resolution thumbnails more eagerly



## [v0.9.1] - 2023-04-10

These changes were intended to be part of v0.9.0, but were mistakenly left out.

* Adds pyroscope support for performance profiling the app
* More flexible image variant source configuration
* Fix timeline view + upgrade build



## [v0.9.0] - 2023-04-10 - Refactor image loading and rendering

Replaces previous image and thumbnail loading/rendering system with a more flexible one.

* New system supports all image formats that FFmpeg does, including AVIF and some RAW formats
* Added sqlite-based thumbnail generation system to resolve the limitation of pre-generated thumbnails
* Some bugs may still be present as testing was limited



## [v0.8.0] - 2023-01-08 - Strip Viewer

Clicking now opens the new strip viewer, but you can still zoom in the original layout with ctrl+zoom or with pinch-to-zoom.

As this is a big refactor it likely has some remaining bugs, oddities, and visual quirks.



## [v0.7.0] - 2022-10-12 - Remove sidebar

The sidebar has been removed in favor of a dropdown menu for collections.



## [v0.6.2] - 2022-10-11

* Disable loading AI if not available
* Split AI hosts support



## [v0.6.1] - 2022-10-09

Upgrade to golang 1.19 in GitHub workflow.



## [v0.6.0] - 2022-10-09 - Add semantic search using photofield-ai

If you set up a [photofield-ai](https://github.com/SmilyOrg/photofield-ai) server and configure it in the `ai` section of the configuration, you should be able to search for photo contents using words like "beach sunset", "a couple kissing", or "cat eyes".

## [v0.5.3] - 2022-09-18 - Context menu fixes

Fixes a bunch of bugs with the context menu closing at bad times and the focus not working right.



## [v0.5.2] - 2022-09-18

Better memory cache size estimation, potentially making images load slightly faster.



## [v0.5.1] - 2022-09-06

Fix some off-by-one navigation issues.



## [v0.5.0] - 2022-09-04 - OpenLayers

- Replace OpenSeadragon with OpenLayers for tile rendering
- Fix date and small scene handling

There might be some bugs from the reimplementation, but the rendering seems a lot faster (especially on low power devices) and it doesn't slow down over time as was the case with OpenSeadragon.



## [v0.4.5] - 2022-08-15

Fix the CORS configuration being too loose by default. This should improve security somewhat.



## [v0.4.4] - 2022-07-31

Logging error hotfix



## [v0.4.3] - 2022-07-31

- Reduce memory usage by pruning scenes
- Add more logs for easier debugging



## [v0.4.2] - 2022-07-26

Fix dates getting stuck sometimes



## [v0.4.1] - 2022-07-26 - Preload all scrollbar dates and UI update

The scrollbar dates don't need a server check to load anymore as they are all preloaded on scene load. The UI has been changed so that instead of the dates showing up next to the scrollbar, they are fixed to the top left corner and only show up while scrolling quickly (similar to Google Photos on mobile).



## [v0.4.0] - 2022-07-16 - Large collections

Loading large collections can be way faster now (loads at 100k-1000k or more photos / second).

Collections of more than 100M photos should also work, but as I don't have that many photos I've only been able to try it by adding a hack to repeat a 40k collection thousands of times.

There is a frontend-based loader now while a scene is loading. This makes it more obvious what's going on without checking the logs.
PhotofieldMillions2

Image/video thumbnails/variants have been refactored to fix some issues with the previous setup. The configuration has changed slightly to support this. Video thumbnails should also show up faster and throw fewer errors in the logs now.



## [v0.3.3] - 2022-07-09 -  Frontend optimization

The app wasn't scoring that well on Lighthouse due to a variety of small low-hanging fruit optimizations that were missing (like gzipping js and css) and cache policies. This change addresses most of those and updates frontend dependencies while we're at it.

The following examples are for an album with ~5000 photos on a DS418play (warm caches).

![lighthouse score before](https://user-images.githubusercontent.com/1451391/178107242-c1b97a8c-deb0-4536-9777-0b754c14aa76.png)

_Photofield Lighthouse performance score was 76 before the optimizations_

Adding these smaller optimizations makes it much better, even scoring 100 on the performance metric.

![lighthouse score after](https://user-images.githubusercontent.com/1451391/178106178-d9d930c3-3065-496a-9a35-263e2f57362d.png)

_Photofield Lighthouse performance score is 100 after optimizations_

For reference, here is the same album with Google Photos (yes, it's slower!).

![google photos lighthouse score](https://user-images.githubusercontent.com/1451391/178106326-8e5f005d-56c9-45dc-b7cc-380fd3c46d01.png)

_Google Photos Lighthouse performance score_



## [v0.3.2] - 2022-07-08 - Smoother scrolling on slow devices

Previously the whole canvas was redrawn every time while scrolling. Fast devices like PCs had no problem with this, but slower devices, like phones, tablets and TVs were not able to keep up the redraws while scrolling leading to a laggy experience sometimes nearing slideshow levels. With this release the canvas is scrolled natively by the browser, which can be a lot smoother.

### Technical Details

Since the canvas is virtual, the native scrolling needs to be combined with redrawing so that the photos never "scroll out of view". If the device is slow and is unable to keep up with the redraws, this now presents itself as "white space" where the redraw has not happened yet. This seems like a better tradeoff as it feels a lot better to have a smooth scroll with some edge artifacts than an always-stable, but laggy scrolling.

These artifacts could be overcome by overdrawing at the edges, so that there is some wiggle room before a redraw needs to happen. This is not implemented yet as a larger canvas can also lead to slower redraws in the first place, so a balance would need to be found.



## [v0.3.1] - 2022-06-16 - Make it Actually Work

### Added

* Environment variable to specify a custom server address by @luusl in https://github.com/SmilyOrg/photofield/pull/7

### Fixed

* App refusing to start if you don't already have a database
* App showing a blank white screen in browser (in some cases, especially Windows) 



## [v0.3.0] - 2022-06-06 - More Efficient Database

This release should be functionally equivalent to v0.2.1, however it reorganizes database storage and processing so that it both takes less space and runs faster (especially scene creation on slow disks).

As an example, in v0.2.1, it takes up 193MiB for my database of ~540k photos. In v0.3.0 the same database with the same data takes up 74MiB.

### More Detail

Both sizes are after vacuuming for a more accurate comparison. You can run a vacuum as a [maintenance task](https://github.com/SmilyOrg/photofield/#maintenance) through the app itself now too. Ideally in the future this would be automatic, but for now you can run it now and then if you'd like to reclaim some space after adding/removing a lot of files.

### Even More Detail

Previously all the file paths were stored as absolute paths, which worked pretty well, but if you have a lot of files, it can lead to a lot of duplication as the same long path is stored for each file. This release de-duplicates all the directories into a separate table, so each file only references this table. It almost sounds like I'm reimplementing file systems here, but I swear it makes sense

This leads to only the file names and a reference to a directory having to be stored. With the above real-world collection of 540k photos, they are only part of ~4600 directories, leading to the big storage savings. This complicates the queries a bit, but SQLite does a good job at executing them.

Due to internal reworking, file paths are also no longer needed when listing / opening a collection (album), so the query can just filter to a few specific directory ids, returning the file metadata in order. With the right indexes this can be pretty fast and efficient. At least that's the idea



## [v0.2.1] - 2022-05-21

* Add embedded jpeg docs (forgot)
* Build on new tags only
* Sort by UTC time



## [v0.2.0] - 2021-12-01 - Embedded JPEG thumbnails

Embedded JPEG thumbnails are now supported. Extracting them is slower than loading pregenerated thumbnails already on disk, but way faster than loading the original, so it's a nice middle ground if you don't already have thumbnails.
![Photofield8](https://user-images.githubusercontent.com/1451391/144293168-173845cf-e0bd-4d95-ab80-ceca55e8b6ef.gif)

### Debug Modes

Additionally, the two debug modes supported by the API are now accessible again. They exist to more easily debug how and when the thumbnails are being used.

#### Debug Overdraw

Shows how close to "perfect" is the thumbnail / image being loaded.
More red ≡ƒæë too high resolution / wasted loaded pixels / slow loading
More blue ≡ƒæë not enough resolution / blurry display

Using only embedded JPEG thumbnails shows that when the original images has to be used, it is way too high of a resolution for the currently displayed size, so it's slow.

https://user-images.githubusercontent.com/1451391/144292927-0e5e3d3b-84a0-4d36-bb83-7a8a8f7cd8d6.mp4

#### Debug Thumbnails

Shows the resolution of the thumbnail / image being used, the "distance" from the ideal resolution for the currently displayed size and the name of the thumbnail (or `original` for the original photo).

https://user-images.githubusercontent.com/1451391/144292940-f298474f-2924-42b2-83a0-b5646b00d186.mp4

### Changelog

* e95e1c3 Merge pull request #5 from SmilyOrg/embedded
* 9aa66f8 Support embedded jpeg thumbs + fix debug modes


### Docker images

- `docker pull ghcr.io/smilyorg/photofield:v0.2.0`
- `docker pull ghcr.io/smilyorg/photofield:v0`
- `docker pull ghcr.io/smilyorg/photofield:v0.2`



## [v0.1.10] - 2021-11-08

Fixed default Docker data dir `PHOTOFIELD_DATA_DIR` in actual container releases and not just `Dockerfile`.



## [v0.1.9] - 2021-10-03

GitHub workflow debugging and improved README.



## [v0.1.8] - 2021-10-02

### Added

- README.md
- `defaults.yaml` documentation



## [v0.1.7] - 2021-09-27

GitHub workflow debugging.



## [v0.1.6] - 2021-09-27

GitHub workflow debugging.



## [v0.1.5] - 2021-09-27

Fixed `PHOTOFIELD_DATA_DIR` in Docker image to point to the right place.



## [v0.1.4] - 2021-09-26

GitHub workflow debugging.

### Added

- `jpeg` and `png` to default extensions



## [v0.1.3] - 2021-09-25

Identical to v0.1.2.



## [v0.1.2] - 2021-09-25

GitHub workflow debugging.



## [v0.1.1] - 2021-09-25

GitHub templates and workflow debugging.



## [v0.1.0] - 2021-09-25

First release





[unreleased]: https://github.com/SmilyOrg/photofield/compare/v0.15.1...HEAD
[v0.15.1]: https://github.com/SmilyOrg/photofield/compare/v0.15.0...v0.15.1
[v0.15.0]: https://github.com/SmilyOrg/photofield/compare/v0.14.2...v0.15.0
[v0.14.2]: https://github.com/SmilyOrg/photofield/compare/v0.14.1...v0.14.2
[v0.14.1]: https://github.com/SmilyOrg/photofield/compare/v0.14.0...v0.14.1
[v0.14.0]: https://github.com/SmilyOrg/photofield/compare/v0.13.0...v0.14.0
[v0.13.0]: https://github.com/SmilyOrg/photofield/compare/v0.12.0...v0.13.0
[v0.12.0]: https://github.com/SmilyOrg/photofield/compare/v0.11.1...v0.12.0
[v0.11.1]: https://github.com/SmilyOrg/photofield/compare/v0.11.0...v0.11.1
[v0.11.0]: https://github.com/SmilyOrg/photofield/compare/v0.10.4...v0.11.0
[v0.10.4]: https://github.com/SmilyOrg/photofield/compare/v0.10.3...v0.10.4
[v0.10.3]: https://github.com/SmilyOrg/photofield/compare/v0.10.2...v0.10.3
[v0.10.2]: https://github.com/SmilyOrg/photofield/compare/v0.10.1...v0.10.2
[v0.10.1]: https://github.com/SmilyOrg/photofield/compare/v0.10.0...v0.10.1
[v0.10.0]: https://github.com/SmilyOrg/photofield/compare/v0.9.4...v0.10.0
[v0.9.4]: https://github.com/SmilyOrg/photofield/compare/v0.9.3...v0.9.4
[v0.9.3]: https://github.com/SmilyOrg/photofield/compare/v0.9.2...v0.9.3
[v0.9.2]: https://github.com/SmilyOrg/photofield/compare/v0.9.1...v0.9.2
[v0.9.1]: https://github.com/SmilyOrg/photofield/compare/v0.9.0...v0.9.1
[v0.9.0]: https://github.com/SmilyOrg/photofield/compare/v0.8.0...v0.9.0
[v0.8.0]: https://github.com/SmilyOrg/photofield/compare/v0.7.0...v0.8.0
[v0.7.0]: https://github.com/SmilyOrg/photofield/compare/v0.6.2...v0.7.0
[v0.6.2]: https://github.com/SmilyOrg/photofield/compare/v0.6.1...v0.6.2
[v0.6.1]: https://github.com/SmilyOrg/photofield/compare/v0.6.0...v0.6.1
[v0.6.0]: https://github.com/SmilyOrg/photofield/compare/v0.5.3...v0.6.0
[v0.5.3]: https://github.com/SmilyOrg/photofield/compare/v0.5.2...v0.5.3
[v0.5.2]: https://github.com/SmilyOrg/photofield/compare/v0.5.1...v0.5.2
[v0.5.1]: https://github.com/SmilyOrg/photofield/compare/v0.5.0...v0.5.1
[v0.5.0]: https://github.com/SmilyOrg/photofield/compare/v0.4.5...v0.5.0
[v0.4.5]: https://github.com/SmilyOrg/photofield/compare/v0.4.4...v0.4.5
[v0.4.4]: https://github.com/SmilyOrg/photofield/compare/v0.4.3...v0.4.4
[v0.4.3]: https://github.com/SmilyOrg/photofield/compare/v0.4.2...v0.4.3
[v0.4.2]: https://github.com/SmilyOrg/photofield/compare/v0.4.1...v0.4.2
[v0.4.1]: https://github.com/SmilyOrg/photofield/compare/v0.4.0...v0.4.1
[v0.4.0]: https://github.com/SmilyOrg/photofield/compare/v0.3.3...v0.4.0
[v0.3.3]: https://github.com/SmilyOrg/photofield/compare/v0.3.2...v0.3.3
[v0.3.2]: https://github.com/SmilyOrg/photofield/compare/v0.3.1...v0.3.2
[v0.3.1]: https://github.com/SmilyOrg/photofield/compare/v0.3.0...v0.3.1
[v0.3.0]: https://github.com/SmilyOrg/photofield/compare/v0.2.1...v0.3.0
[v0.2.1]: https://github.com/SmilyOrg/photofield/compare/v0.2.0...v0.2.1
[v0.2.0]: https://github.com/SmilyOrg/photofield/compare/v0.1.10...v0.2.0
[v0.1.10]: https://github.com/SmilyOrg/photofield/compare/v0.1.9...v0.1.10
[v0.1.9]: https://github.com/SmilyOrg/photofield/compare/v0.1.8...v0.1.9
[v0.1.8]: https://github.com/SmilyOrg/photofield/compare/v0.1.7...v0.1.8
[v0.1.7]: https://github.com/SmilyOrg/photofield/compare/v0.1.6...v0.1.7
[v0.1.6]: https://github.com/SmilyOrg/photofield/compare/v0.1.5...v0.1.6
[v0.1.5]: https://github.com/SmilyOrg/photofield/compare/v0.1.4...v0.1.5
[v0.1.4]: https://github.com/SmilyOrg/photofield/compare/v0.1.3...v0.1.4
[v0.1.3]: https://github.com/SmilyOrg/photofield/compare/v0.1.2...v0.1.3
[v0.1.2]: https://github.com/SmilyOrg/photofield/compare/v0.1.1...v0.1.2
[v0.1.1]: https://github.com/SmilyOrg/photofield/compare/v0.1.0...v0.1.1
[v0.1.0]: https://github.com/SmilyOrg/photofield/releases/tag/v0.1.0
