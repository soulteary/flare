# Search targets

> Availability: external engines, the selector and hybrid suggestions shipped in
> 0.6.5. Revision-protected settings writes and retirement of keyword shortcuts
> shipped in 0.6.6. Use the documentation at your release
> tag for 0.6.5 behavior. See [search modes and upgrade safety](search-hybrid.md).

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

In stable 0.6.7, search settings use an eight-hour instance-signed token paired
with an HttpOnly, host-only, SameSite=Strict cookie. Current main development uses
the unified account-session/preauthentication-browser-bound CSRF mechanism instead;
the old dedicated cookie's lifetime is not the current authentication lifetime.
See [accounts](accounts.md) and [proxy trust](reverse-proxy.md).

For programmatic settings writes:

1. Sign in when required and GET `/settings/search` with the same cookie jar.
2. Retain cookies and submit that form's hidden `csrf-token` and `config-revision`
   with the requested settings to the same origin. Do not put CSRF tokens in URLs.
3. After restart, session rotation or reauthentication, GET a fresh form. Preserve
   intended changes before refreshing. Treat 401/reauthentication requirements,
   source/CSRF 403 responses and stale configuration conflicts separately; some
   route-local 403 responses use a generic message rather than a machine code.

Source checks reject cross-site/sibling requests and invalid Origin/Referer when
present. Supported forwarding headers only affect current main when explicit
trusted proxies are configured. Keep public Host/port and HTTPS cookie policy
consistent. All form responses disable caching. Legacy search-field compatibility
does not bypass authentication, source or CSRF protection.

The applicable CSRF mechanism protects the search settings endpoint. Since 0.6.6, forms also
carry `config-revision`: a stale revision is rejected and
submitted values are retained. Preserve those values, reload the current settings
and review the differences before saving again. Programmatic clients should
submit both hidden fields. Legacy clients without a revision remain accepted and
cannot rely on stale-form detection. In 0.6.5, same-section saves use
last-writer-wins behavior. Separate processes writing the same configuration are
not coordinated; use one writer. See [configuration save protections](search-hybrid.md#save-path-audit-protections).

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
  Engines:
    - ID: bing
      Name: Bing
      URLTemplate: "https://www.bing.com/search?q={query}"
      Enabled: true
    - ID: internal-docs
      Name: Internal documentation
      URLTemplate: "https://docs.example.com/search?q={query}"
      Enabled: true
```

In 0.6.6, `config.yml` is checked on each configuration read, so
external search-setting edits are observed without a restart. Stop the process
before external edits to avoid racing a settings save. On 0.6.5, restart after
editing YAML directly.
Only enabled homepage modules participate in local suggestions; a malformed file
for a hidden module does not disable matches from a visible module. Independent
settings sections are serialized within one Flare process so an appearance or
weather save cannot discard a simultaneous engine update.
