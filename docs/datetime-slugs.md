# Datetime slugs

BlueNote can generate post slugs from the saved publication time. Defaults:

```ini
[app]
datetime_slugs = false
datetime_slug_timezone = UTC
```

To generate dates in Japan time, set `datetime_slugs = true` and
`datetime_slug_timezone = Asia/Tokyo`. Any loadable IANA timezone is supported;
`Local` is rejected. An absent or empty timezone means UTC. An invalid timezone
prevents configuration loading when the feature is enabled; it is ignored when
disabled. Restart the server after changing the configuration.

Automatic slugs use the finalized `Created` instant converted to the configured
timezone, formatted as `YYYYMMDDHHMMSS`. Direct posts, API posts, imports and
drafts collected into a blog use the same setting. Drafts without a blog do not
receive an automatic slug until collected. Collection does not reset `Created`:
an old draft keeps its old publication time, and a scheduled post uses its
scheduled time. Explicit slugs on creation take precedence.

Saved slugs are retained on collection/recollection, even after editing the
publication date, changing timezone, or disabling the feature. Explicit slug
edits remain possible. A new automatic slug collision uses the existing unique
suffix mechanism, so the final slug may exceed 14 characters. Moving a saved
slug into a blog where it collides fails instead of automatically renaming it.
Moving between blogs still changes the blog portion of the URL.

When disabled, new automatic slugs use WriteFreely's title/body/ID rules.
Preserving already-saved slugs on recollection is intentional even when disabled.
Database timestamps, scheduling, and ActivityPub timestamps are not converted
or rewritten by this setting. Go's standard `time/tzdata` provides timezone data
when system timezone files are unavailable.

Regression tests (SQLite requires CGO and a C compiler):

```sh
go test -mod=readonly ./config
go test -mod=readonly -vet=off -tags sqlite -run TestBlueNote -count=1 .
```

`-vet=off` bypasses pre-existing format-string vet failures in the root package;
it does not suppress failing test assertions. Known upstream test failures are
not fixed by this change.
