# Search targets

> Availability: development branch after 0.6.4; not included in the published 0.6.4 image.

The home page retains one search field. Existing installations and legacy POST
requests continue to search local bookmarks and applications. Add engines under
Settings → Search to enable the target selector. The basic selector works without
JavaScript and does not weaken the home page's Content Security Policy.

Add a preset or a custom engine with a stable unique ID, a display name, and an
absolute HTTP(S) URL containing exactly one `{query}` in a query parameter value.
For example, `https://www.bing.com/search?q={query}`. Search text is encoded once;
characters such as `+`, `&`, `%`, `#`, Chinese text and emoji remain part of the
query. URLs may point to internal services, but must not contain credentials.

Adding, removing or moving rows changes the working form only. Click **Save
changes** to persist the whole search configuration. Validation or write failures
retain the submitted form. **Test engine** opens a test query in a new tab without
saving. Do not put secrets in a search template: visitors can see enabled targets.

The default target must be local search or an enabled engine. Disable or remove an
engine only after choosing a replacement default. Engine names may change without
changing their stable IDs. Search settings remain separate from bookmark exports.

When keywords are enabled, `!g query` selects an engine whose keyword is `g`;
`!local query` forces local search. Unknown keywords are ordinary search text.
Searches accept up to 512 Unicode characters. Empty and invalid searches retain
input and display a message instead of navigating away.

In the script-free selector, Enter and the normal **Search** button use the current
tab. Enabling the new-tab setting adds an explicit **Search in new tab** button
for the selected target, including local search. This explicit action avoids
unreliable popup scripting and never makes ordinary local searches open a new tab.

Only submitted queries are sent to an external engine. Flare does not fetch search
results, query suggestions, engine icons, or test engine reachability. Browser
network access determines whether an external or internal target is reachable.

## Search settings request protection

Search settings forms now carry an eight-hour, instance-signed token paired with
an HttpOnly, host-only, SameSite=Strict cookie with an instance-specific name. Refresh any form opened before an
upgrade or application restart before saving. Programmatic clients must first GET
`/settings/search`, retain its cookie, and submit the hidden `csrf-token` field.
The compatibility handling for older search fields does not bypass this token.

The guard rejects cross-site and same-site sibling browser requests, opaque or
foreign Origin values, and foreign Referer values when present. It does not use
Forwarded or X-Forwarded headers to decide whether a request is trusted. A valid
token permits HTTPS termination at a reverse proxy while the backend uses HTTP;
keep the original Host (including a nondefault public port) and configure `FLARE_COOKIE_SECURE=true` for HTTPS-only
cookies in that deployment. Token-bearing clients without browser metadata remain
supported. Form responses use `Cache-Control: no-store`.

This protection covers the search settings endpoint, not every administrative
endpoint. Concurrent edits to the same search section still use last-writer-wins
behavior; separate processes writing the same configuration are not coordinated.

## Configuration example

Merge this block into `config.yml`; keep the existing unrelated settings. No
`Search` block is required for local-only search. Use `Mode: hybrid` and an enabled
external `DefaultTarget` (for example `bing`) to opt into local suggestions.

```yaml
ShowSearchComponent: true
DisabledSearchAutoFocus: false
Search:
  Mode: select
  DefaultTarget: local
  OpenNewTab: true
  EnableKeywords: true
  Engines:
    - ID: bing
      Name: Bing
      URLTemplate: "https://www.bing.com/search?q={query}"
      Keyword: b
      Enabled: true
    - ID: internal-docs
      Name: Internal documentation
      URLTemplate: "https://docs.example.com/search?q={query}"
      Keyword: docs
      Enabled: true
```

Restart after editing YAML directly: the running process caches configuration.
Only enabled homepage modules participate in local suggestions; a malformed file
for a hidden module does not disable matches from a visible module. Independent
settings sections are serialized within one Flare process so an appearance or
weather save cannot discard a simultaneous engine update.
