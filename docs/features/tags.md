# Tags

You can tag photos with arbitrary tags. There is only basic support for tags
right now, but they form a foundation for many other features.

::: warning
Tags are currently in an alpha state and can be volatile. They are
not yet stored in the photos themselves, only in the "cache" database.
:::

Tags needs to be enabled in the `tags` section of the [configuration] the server
needs to be restarted.

```yaml
tags:
  enable: true
```

[configuration]: ../configuration

## Tagging Photos

If tags are enabled, the fullscreen photo view adds a # (hash) button for toggling the tag selection dropdown. It also adds a 🤍 (heart) button that toggles the `fav` tag to serve as simple "liking" functionality.

## Search

You can filter photos in the collection by searching for `tag:TAG`.

For example, you can search for `tag:fav` to only show favorited photos, or
`tag:hello tag:world` to only show photos with both `hello` and `world` tags.
This is an early version of filtering and should be more user-friendly in the
future.

See the [search documentation](search.md) for more on search.

## EXIF

Automatically add tags from EXIF data.

The only EXIF tags are the currently hardcoded `make` and `model`, and they are
added to the file as `exif:make:<make>` and `exif:model:<model>` tags
respectively.

To enable the automatic addition of these tags, you need to enable it in the config.
```yaml
tags:
  enable: true
  exif:
    enable: true
```
