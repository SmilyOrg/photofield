# Search

Photofield offers powerful search capabilities to help you find photos quickly
and efficiently. The search feature supports various types of queries, including
tag-based searches, semantic searches, and more.

::: tip
Search requires [photofield-ai] to be configured and enabled in the `ai`
section of the [configuration].
:::

## Semantic Search

Semantic search allows you to search for photo contents using descriptive words
like "beach sunset", "a couple kissing", or "cat eyes".

![Semantic search example](../assets/semantic-search.jpg)

By default, the results are sorted by the semantic relevance to the query.

To filter the results instead of sorting them, you can use the `t` parameter in
the query. For example, `beach sunset t:0.25` will retain the original order of
the photos, but only keep photos very similar to beach sunsets. The value is
arbitrary, higher values are more strict and lower values are less strict. A
good range is usually between 0.1 and 0.3.

[photofield-ai]: https://github.com/smilyorg/photofield-ai
[configuration]: ../configuration

| Query | Description |
|-------|-------------|
| `beach sunset` | Sort photos by how much they look like a beach sunset. |
| `beach sunset t:0.25` | Filter photos to beach sunsets only. |
| `beach sunset t:0.15` | Same as above, but less strict. |
| `a couple kissing` | Find photos of couples kissing. |
| `rain` | Find the rainiest-looking photos. |
| `lake t:0.23` | Filter to photos containing a lake. |
| `upside down` | Filter to photos containing a lake. |
| `weird angle t:0.25` | Photos taken from strange angles only. |

## Tag Search

::: tip
Enable tags in the [configuration] to be able to add and search for tags.
:::

You can filter photos in the collection by searching for specific tags. For
example, you can search for `tag:fav` to only show favorited photos, or
`tag:hello tag:world` to only show photos with both `hello` and `world` tags.

| Query | Description |
|-------|-------------|
| `tag:fav` | Show all favorited photos. |
| `tag:vacation tag:beach` | Show photos tagged with both `vacation` and `beach`. |

See the [tags documentation](tags.md) for more on tags.

## Date Range

You can search for photos taken within a specific date range using the `created`
parameter. The date range should be specified in the format
`YYYY-MM-DD..YYYY-MM-DD`.

| Query | Description |
|-------|-------------|
| `created:2023-01-01..2023-12-31` | Find photos taken in the year 2023. |

## Deduplication

You can use the `dedup` parameter to filter out duplicate successive photos. The
value should be a threshold between 0 and 1 representing the similarity between
photos. For example, `dedup:0.9` will filter out photos that are 90% similar to
each other.

<!-- ### Example Query -->

| Query | Description |
|-------|-------------|
| `dedup:0.9` | Filter out photos that are 90% similar to each other. |
| `dedup:0.5` | Filter out photos that are even kind-of similar. |
| `dedup:0.3` | Only show very different photos. |
