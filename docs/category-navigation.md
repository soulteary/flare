# Bookmark category navigation

Appearance settings offer three bookmark category modes. All groups remains the
backward-compatible default. Category links filter the server-rendered response;
All restores every group. Links preserve the current search query and category
IDs are URL-encoded. Unknown IDs fall back to All. This is presentation only, not
an authorization boundary.

Collapsible groups use native HTML details/summary controls and require no
JavaScript. Groups start closed; search results expand rather than hiding matches.
Expansion state is intentionally not persisted across reloads. Category order and
item order are unchanged. Default-placement bookmarks remain in the first group.

Category navigation applies to both the homepage and the bookmarks page. The
editor's location filter is independent and does not change homepage settings.
