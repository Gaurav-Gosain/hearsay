# The mentions feed, version 1

A mentions feed is one JSON file, usually called `mentions.json`. It lists the
places a project is mentioned: videos, posts, articles, issues, packages and
so on. Any collector in any language can write it. The viewer reads it and
never changes it.

The JSON Schema is [`feed/schema.json`](../feed/schema.json).

## Rules

- The file is UTF-8 JSON. One object at the top level.
- `version` is `1`. A reader rejects a file with a higher major version.
- A reader ignores fields it does not know. A writer can add fields with an
  `x_` prefix for its own use.
- Times are RFC 3339 (`2026-10-18T05:14:09Z`) or a plain date (`2026-10-18`).
- An item `id` is stable. The same mention gets the same `id` on every run.
  A good `id` is the first 16 hex characters of the SHA-1 of the canonical URL.
- Triage state does not live in the feed. It lives in a separate state file
  (see below), so a collector can rewrite the feed at any time.

## Top level

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `version` | integer | yes | Always `1` for this spec. |
| `project` | object | yes | The project the mentions are about. |
| `generated` | time | yes | When the collector wrote the file. |
| `groups` | array | no | Sidebar entries to add or change. See Groups. |
| `items` | array | yes | The mentions. Can be empty. |

### project

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `name` | string | yes | The project name. The viewer draws it as the wordmark. |
| `subtitle` | string | no | A short line under the name, such as `in the wild`. |
| `url` | string | no | The project home page. |
| `colors.accent` | `#rrggbb` | no | The main colour. |
| `colors.accent2` | `#rrggbb` | no | The second colour of the wordmark gradient. |

## Items

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `id` | string | yes | Stable, unique in the file. |
| `source` | string | yes | The platform, in lower case: `youtube`, `x`, `bluesky`, `reddit`, `hackernews`, `lobsters`, `lemmy`, `mastodon`, `stackexchange`, `github`, `web`, `devto`, `registry`, or your own. |
| `kind` | string | yes | What the item is: `video`, `post`, `reply`, `comment`, `story`, `question`, `answer`, `article`, `newsletter`, `directory`, `issue`, `pr`, `discussion`, `repo`, `code`, `dependent`, `package`, or your own. |
| `url` | string | yes | The canonical link to the item. |
| `title` | string | yes | One line. For a post with no title, use its first line. |
| `group` | string | no | The sidebar entry. If it is missing, the reader picks one from `source` and `kind` (table below). |
| `author` | string | no | A display name or a handle. |
| `published` | time | no | When the item was posted. |
| `excerpt` | string | no | The text around the mention. 1,000 characters at most. |
| `link_title` | string | no | The title of a page the item links to. |
| `files` | array of strings | no | For a code search hit: the matching file paths. |
| `metrics` | object | no | Counts, name to number. See Metrics. |
| `tags` | array of strings | no | Free labels, such as a language code or a query name. |
| `relevance` | string | no | `real`, `likely`, `unverified`, `noise`, `automation` or `own`. The default is `real`. |
| `reason` | string | no | One line that says why the item has its relevance. |
| `first_seen` | time | no | When a collector saw the item first. |
| `last_seen` | time | no | When a collector saw the item last. |
| `state` | string | no | The starting triage state: `new`, `seen`, `featured` or `ignored`. A state file overrides it. |

The viewer hides items with the relevance `noise`, `automation` or `own`
until the reader asks for them.

### Metrics

A metric is a number of zero or more. Use these names where they fit, so
every viewer reads them the same way:

`views`, `likes`, `reposts`, `replies`, `comments`, `reactions`, `stars`,
`forks`, `points`, `score`, `upvotes`, `votes`, `boosts`, `bookmarks`,
`downloads`, `installs_30d`, `installs_90d`, `installs_365d`, `popularity`.

A collector that does not know a count leaves the metric out. It does not
write `0` for an unknown count.

### Default groups

| source | kind | group |
| --- | --- | --- |
| `github` | `issue` | `issues` |
| `github` | `pr` | `prs` |
| `github` | `discussion` | `discussions` |
| `github` | `repo` | `repos` |
| `github` | `dependent` | `dependents` |
| `github` | `code` | `code` |
| `web`, `devto` | `newsletter` | `newsletters` |
| `web`, `devto` | `directory` | `directories` |
| `web`, `devto` | anything else | `articles` |
| `registry` | any | `registries` |
| `hn` | any | `hackernews` |
| any other | any | the `source` |

## Groups

The reader has built-in sidebar entries for the sources and groups above. A
feed can add an entry for its own source, or change a built-in one, with
`groups`:

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `id` | string | yes | The group id that items use. |
| `name` | string | yes | The label in the sidebar. |
| `section` | string | no | The heading the entry sits under. The default is `Other`. |
| `color` | `#rrggbb` | no | The colour of the icon. |
| `icon` | string | no | One Nerd Font glyph. |
| `ascii` | string | no | One ASCII character for terminals without a Nerd Font. |

## The state file

Triage is local. The viewer writes it to a state file, and never to the feed.

```json
{
  "version": 1,
  "items": {
    "a70022d02101a664": { "state": "featured", "at": "2026-10-19T09:12:00Z" },
    "bc2f6c6d31474279": { "state": "seen", "at": "2026-10-19T09:12:04Z" }
  }
}
```

- `state` is `new`, `seen`, `featured` or `ignored`.
- An id that is not in the file has the state `new`, or the item's own
  `state` when the feed sets one.
- Ids that are no longer in the feed stay in the file, so a mention that
  comes back keeps its state.

## A public feed

A public feed is a normal feed with the featured items only. Make it with
`mentions publish`. It keeps `id`, `source`, `kind`, `group`, `url`, `title`,
`author`, `published`, `excerpt`, `link_title`, `metrics` and `tags`. It drops
`relevance`, `reason`, `files`, `first_seen`, `last_seen` and `state`, so a
reader of the public feed starts with every item new.

## A small example

```json
{
  "version": 1,
  "project": { "name": "aurora", "subtitle": "in the wild", "colors": { "accent": "#7c6cff", "accent2": "#f08bd0" } },
  "generated": "2026-10-19T08:00:00Z",
  "items": [
    {
      "id": "3f1c9a0b2d4e5f60",
      "source": "youtube",
      "kind": "video",
      "url": "https://www.youtube.com/watch?v=xxxxxxxxxxx",
      "title": "A first look at aurora",
      "author": "Some Channel",
      "published": "2026-10-12",
      "metrics": { "views": 1520, "likes": 64, "comments": 9 }
    }
  ]
}
```
