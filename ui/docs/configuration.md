# Configuration

You can configure the app via `configuration.yaml`.

The location of the file depends on your installation method, see
[Quick Start](/quick-start).

## Minimal Example

The following is a minimal `configuration.yaml` example, see [Defaults](#defaults) for all options.

::: code-group
```yaml [configuration.yaml]
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

:::

## Defaults

<<< @/../../defaults.yaml

