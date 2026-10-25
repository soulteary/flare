# Search targets

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
