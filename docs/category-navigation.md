# Bookmark category navigation

> Availability: included in Flare 0.6.5 and later; not included in 0.6.4.

Appearance settings offer three bookmark category modes. All groups remains the
backward-compatible default. Category links filter the server-rendered response;
All restores every group. Links preserve the current search query and category
IDs are URL-encoded. Stable 0.6.7 falls back to All for unknown IDs. Current main development checks
category visibility first: unknown or inaccessible requested categories return
404. Category tabs are presentation controls; private-resource authorization is
separate and also applies to search and redirects. See [accounts](accounts.md).

Collapsible groups use native HTML details/summary controls and require no
JavaScript. Groups start closed; search results expand rather than hiding matches.
Expansion state is intentionally not persisted across reloads. Category order and
item order are unchanged. Default-placement bookmarks remain in the first group.

Category navigation applies to both the homepage and the bookmarks page. The
editor's location filter is independent and does not change homepage settings.
