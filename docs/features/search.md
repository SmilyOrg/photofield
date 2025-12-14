# Search

Photofield offers powerful search capabilities to help you find photos quickly
and efficiently. The search feature supports various types of queries, including
tag-based searches, semantic searches, date filtering with flexible wildcards, and more.

::: tip
Features marked with <Badge type="tip" text="AI" /> require capabilities
provided by [photofield-ai]. You can configure it in the `ai` section of the
[configuration].
:::

## Search Interface

The search interface includes interactive **search chips** that make it easy to build and
visualize your search queries. When you activate search, chips appear for common filters
like dates and similarity thresholds.

![Search chips example](../assets/search-chips.png)

::: tip
The chips only provide some common basic filters. The search syntax supports more
advanced queries as described below.
:::

## Semantic Search <Badge type="tip" text="AI" />

Semantic search allows you to search for photo contents using descriptive words
like "beach sunset", "a couple kissing", or "cat eyes".

![Semantic search example](../assets/semantic-search.jpg)

By default, the results are sorted by the semantic relevance to the query.

To filter the results instead of sorting them, you can use the `t` parameter in
the query. For example, `beach sunset t:0.25` will retain the original order of
the photos, but only keep photos very similar to beach sunsets. The value is
a threshold between 0.15 and 0.30, where higher values are more strict and lower 
values are less strict.

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
| `weird angle t:0.25` | Photos taken from strange angles only. |

## Tag Search

::: tip
Enable tags in the [configuration] to be able to add and search for tags.
:::

You can filter photos in the collection by searching for specific tags. For
example, you can search for `tag:fav` to only show favorited photos, or
`tag:vacation tag:beach` to only show photos with both `vacation` and `beach` tags.

| Query | Description |
|-------|-------------|
| `tag:fav` | Show all favorited photos. |
| `tag:vacation tag:beach` | Show photos tagged with both `vacation` and `beach`. |

See the [tags documentation](tags.md) for more on tags.

## Date Filtering

You can search for photos based on when they were taken using the `created`
qualifier. Photofield supports flexible date formats and wildcards for powerful
date-based searches.

### Date Formats

| Format | Example | Description |
|--------|---------|-------------|
| `YYYY-MM-DD` | `created:2023-06-15` | Exact date |
| `YYYY-MM` | `created:2023-06` | Entire month |
| `YYYY` | `created:2023` | Entire year |

### Date Ranges

Use `..` to specify a date range:

| Query | Description |
|-------|-------------|
| `created:2023-01-01..2023-12-31` | Photos from all of 2023 |
| `created:2023-06..2023-08` | Summer months of 2023 |
| `created:2020..2024` | Photos from 2020 through 2024 |

### Comparison Operators

Use comparison operators for open-ended date filters:

| Query | Description |
|-------|-------------|
| `created:>=2023-06-15` | Photos from June 15, 2023 onwards |
| `created:>2023-06-15` | Photos after June 15, 2023 |
| `created:<=2023-06-15` | Photos up to and including June 15, 2023 |
| `created:<2023-06-15` | Photos before June 15, 2023 |

### Date Wildcards

Use `*` as a wildcard to match specific days across multiple months or years:

| Query | Description |
|-------|-------------|
| `created:*-12-25` | All photos taken on December 25th (any year) |
| `created:*-01-01` | All photos taken on New Year's Day |
| `created:*-02-29` | All photos taken on leap day |
| `created:2024-*-01` | First day of every month in 2024 |
| `created:2024-*-15` | 15th of every month in 2024 |
| `created:*-05-*` | All photos from May (any year) |

::: tip
Wildcards are especially useful for finding photos from recurring events like birthdays,
holidays, or monthly patterns, without needing to specify every year.
:::

### Examples

| Query | Description |
|-------|-------------|
| `created:2023-01-01..2023-12-31` | All photos from 2023 |
| `created:2024-02` | All photos from February 2024 |
| `created:>=2024-01-01` | All photos from 2024 onwards |
| `created:*-12-25` | All Christmas photos (any year) |
| `created:2023-*-01` | First day of each month in 2023 |

## Deduplication <Badge type="tip" text="AI" />

You can use the `dedup` parameter to filter out duplicate successive photos. The
value should be a threshold between 0 and 1 representing the similarity between
photos. For example, `dedup:0.9` will filter out photos that are 90% similar to
each other.

| Query | Description |
|-------|-------------|
| `dedup:0.9` | Filter out photos that are 90% similar to each other. |
| `dedup:0.5` | Filter out photos that are even kind-of similar. |
| `dedup:0.3` | Only show very different photos. |

## Combining Filters

You can combine multiple search qualifiers in a single query:

| Query | Description |
|-------|-------------|
| `created:2023-06..2023-08 tag:vacation t:0.25 sunset` | Summer vacation sunset photos from 2023 |
| `created:>=2024-01-01 t:0.25 dedup:0.9 beach` | Distinct beach photos from 2024 onwards |
| `created:*-12-* tag:family` | All December family photos |
