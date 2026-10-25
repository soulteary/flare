# Hybrid homepage search

> Availability: included in Flare 0.6.5 and later; not included in 0.6.4.

Hybrid search is an optional enhancement to the ordinary search form. Configure a
valid external default target and `Search.Mode: hybrid` in search settings. The
legacy/select mode remains script-free. If JavaScript is unavailable, the normal
form still submits to its selected target.

Typing displays at most eight local matches. Applications match name, resolved
address and description; bookmarks match name and resolved address, just like
existing server search. Hidden modules and orphan categories are not suggested.
Click a result to open it using that module's existing new-tab and encoded-link
settings. Use Up/Down then Enter to open a selected match. With no selected match,
Enter submits the normal form. When `OpenNewTab` is enabled, external targets
open in a new tab while local search stays in the current tab. The explicit new-tab
button always opens a new tab, including local search. Escape dismisses results. Input method composition
never triggers navigation. Explicit keyword commands bypass local suggestions.

Suggestions use a same-origin POST, with no external suggestion provider and no
search history. The endpoint inherits the homepage's PRIVATE authentication,
returns `Cache-Control: no-store`, and is unavailable when hybrid search is off.
No full bookmark database is embedded in the page. Queries are debounced and old
responses discarded. If loading fails, ordinary form search remains available.

The immutable embedded script is allowed by its SHA-256 CSP hash. The homepage
combines search and clock hashes when both are enabled; it does not enable
`unsafe-inline` or arbitrary script sources. CSP also limits suggestion connections
to the same origin. Dynamic addresses are resolved using request-scoped homepage
metadata rather than mutable global request state.

Browser acceptance must cover desktop keyboard navigation, iOS/Android touch,
Chinese IME confirmation, delayed/out-of-order responses, CSP enforcement with and
without the clock, and the script-free fallback. Server tests cover matching,
limits, authentication, disabled mode, no-store responses and production/debug
script hashes.

Changing the selected engine clears the active local match. Moving focus out of
the search input cancels pending suggestions; stale responses cannot reopen a
dismissed list. Mouse and touch activation still use the result's native anchor.
Known enabled commands such as `!local`, `!ex` and `!s` bypass suggestions and
override the selected target. Unknown commands remain ordinary searchable text.
