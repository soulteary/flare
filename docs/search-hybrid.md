# Homepage search modes

> Hybrid search is available in Flare 0.6.5 and later. The additional modes and configuration protections described below require this change.

Search settings provide three modes, saved with the rest of the search configuration:

- `hybrid`: typing suggests local bookmarks and apps; Enter searches the selected web engine. Requires an enabled external default target. Use a local suggestion to open a bookmark or app.
- `local-first`: typing suggests local matches; Enter filters bookmarks and apps. The resulting page offers explicit web engine buttons. No automatic external fallback occurs. Works without external engines.
- `select`: choose bookmarks/apps or an external engine before submitting. Starts with `DefaultTarget` and works without JavaScript.

Existing mode values and configurations retain their meaning. The default remains `select` with local search. Switching modes in settings does not silently add external engines. In local-first mode, `DefaultTarget` does not change the initial local behavior.

Typing displays at most eight local matches. Applications match name, resolved
address and description; bookmarks match name and resolved address, just like
existing server search. Hidden modules and orphan categories are not suggested.
Click a result to open it using that module's existing new-tab and encoded-link
settings. Use Up/Down then Enter to open a selected match. With no selected match,
Enter submits the normal form. When `OpenNewTab` is enabled, external targets
open in a new tab while local search stays in the current tab. The explicit new-tab
button always opens a new tab, including local search. Escape dismisses results. Input method composition
never triggers navigation.

Suggestions use a same-origin POST, with no external suggestion provider and no
search history. The endpoint inherits the homepage's PRIVATE authentication,
returns `Cache-Control: no-store`, and is unavailable outside hybrid and local-first modes.
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
All queries are treated as ordinary search text and use the selected target.

## Legacy configuration safety

Compatibility is applied in memory when rendering and using search. A missing
Search block retains legacy local search. Missing mode/target fields use the
legacy defaults. A hybrid configuration whose default is local or unavailable
runs as local-first, preserving local intent. No migration adds, enables, deletes
or rewrites custom engines. Malformed engines and unknown modes are not guessed
at; validation continues to protect the local fallback.

Opening search settings or searching does not persist an upgrade. The settings
page explains when compatibility is active. When the user explicitly saves search
settings, unchanged effective settings cause no write or backup (even if YAML
formatting differs). Changed settings are backed up in the private
`.flare-search-backups/` directory before atomic replacement. `first-upgrade-to-<target-version>.bak`
retains the original pre-upgrade document permanently; `latest.bak` retains only
the immediately preceding document for subsequent changes. `manifest.json`
records ownership and SHA-256 hashes. Rotation only replaces a regular file whose
recorded hash still matches. A missing, unrecognized, externally modified or
symlinked managed backup stops the save. Old `config.yml.pre-search-*.bak` files
and other user backups are never scanned or deleted, since ownership cannot be
reliably established. Search fields are updated through a YAML tree. Unknown configuration/search fields and engine
extensions with unchanged IDs are retained. Bookmarks and apps are never written
by this path. Backup failure, invalid/ambiguous YAML, or a detected concurrent
change aborts the save. Keep the backups private: they contain the original full
configuration. This is a single-process update guard; administrators should avoid
editing config.yml concurrently from another process.

Selecting hybrid explicitly and saving with a local default selects the first
already-enabled engine. Without an enabled engine the settings page asks the user
to add one; it does not silently add a preset or send a query externally.

## Application version tracking

`AppVersion` records the application version in newly initialized configurations
and on actual settings writes, using the existing version-kit build version.
Read-only access and unchanged search saves do not stamp or rewrite old files.
`ConfigVersionState` is a runtime-only classification: legacy (no version), same,
upgrade, downgrade, or unknown. Comparison follows numeric semantic-version and
prerelease ordering; build metadata does not affect ordering. Development and
unrecognized versions cannot be reliably ordered, so an existing version marker
is retained rather than overwritten with a speculative upgrade/downgrade.

Known downgrades refuse configuration writes, including search and other settings.
The search settings page explains why saving is blocked. The first upgrade backup
filename includes its target version and its manifest records that version too.
The first backup remains permanent across later releases. Previously created
unversioned `first-upgrade.bak` files remain intact; their historical target version
is not guessed or retroactively relabeled. Later changes still rotate `latest.bak`.

## Save-path audit protections

All configuration settings pages now preserve unknown YAML fields and use the
same version guard and backup policy. The first write after an upgrade creates a
permanent backup even when it originates from the theme or appearance page.
Configuration reads bypass the old byte cache so manual version changes are
visible immediately. Search forms carry a revision of the document they displayed;
a stale form is rejected with its submitted values retained. Old forms without a
revision remain compatible. Backups are not committed to Git.

Backup rotation records a pending filename, digest and target version in the
manifest before replacing a backup. On retry, the recorded old or new digest
allows safe recovery of an interrupted rotation. Any other contents, unknown
manifest fields, symlinks or public backup permissions stop the save. Atomic
replacement syncs both file contents and the containing directory.

Changed saves of YAML with aliases/merge keys are refused because a shared anchor
could otherwise alter an unrelated section. Multiple YAML documents and duplicate
known keys are rejected rather than silently truncating the file. Symbolic-link
configuration files can be read but cannot be replaced by settings saves. A rename
of an extension-bearing engine is rejected when its identity cannot be preserved
unambiguously. These cases retain the original file for manual correction.

The revision check, process-local update mutex and repeated disk comparison reduce
lost updates, but do not provide an operating-system compare-and-swap against
another process writing in the final rename interval. Run one writer per config
file. Failure-injection tests cover process-interruption states; they do not
replace power-loss testing on the deployment filesystem.
