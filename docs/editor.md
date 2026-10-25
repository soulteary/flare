# Navigation editor

## State and persistence

The editor loads `/editor/document` with a revision covering the exact on-disk
bytes of both `apps.yml` and `bookmarks.yml`. All views share a working document;
search/filter operations must never remove hidden entries from that document.
Category IDs survive renaming and reordering. Session entry keys do not enter YAML.
Existing URLs (including intranet/custom schemes), icon values, descriptions and
private flags are preserved. Unknown category references are not silently moved.

A save uses JSON and `X-Flare-Editor: 1`, repeats validation on the server, and
rejects outdated revisions with HTTP 409. Validation errors use HTTP 422 and a
field path. An old CSV editor page cannot bypass revision protection: it receives
409 and must preserve its edits before refreshing. YAML storage remains unchanged.

Both files are staged and synced before replacement. Recovery copies live in a
private `.flare-editor-recovery/save-*` directory. Successful saves remove their
recovery copies; failed saves retain them for inspection. A replacement failure
attempts to restore replaced files and reports any recovery failure explicitly.
This is not a cross-file atomic transaction. Process/power loss and external file
writers are not coordinated by an OS-wide lock; compare-before-replace protects
observed external edits but cannot eliminate the final filesystem race window.
Keep normal filesystem backups and do not edit files concurrently during saves.

## Verification

`go test ./... -count=1` covers validation, stale/external revision conflicts,
stable IDs, preserved private flags, empty documents, prepared-write failure and
second-file failure with recovery. `node --test tests/editor/state.test.cjs` tests
the shared working state. The mobile browser acceptance test exercises the legacy
table UI against the versioned document interface.

This change is the persistence foundation; the list/form interface and integrated
batch view follow as separate dependent pull requests.

## List and form workflow

The default editor is a searchable list. Search matches name, address and
description; the location filter only affects display. A form edits one entry's
name, exact address, destination and optional icon/description. Completing a form
updates the working document; only the bottom Save changes button writes files.
Cancel leaves the document unchanged. Category deletion requires an explicit
migration destination for nonempty groups; entry deletion offers single-action
undo. Category ordering has up/down controls and never changes category IDs.

Actual data comparison clears dirty state when values are restored. Save errors
preserve the working document; conflicts expose a JSON draft download and an
explicit reload action. The JSON draft is an editor-state export, not a browser
bookmark import file. The standalone batch route remains available during this
intermediate PR and warns before leaving unsaved list edits; the next PR replaces
that navigation with an integrated adapter.

`node tests/editor/browser.cjs` runs the list/form acceptance suite with the same
FLARE_BINARY and CHROMIUM_EXECUTABLE variables as the mobile tests. Optional
EDITOR_SCREENSHOT_DIR captures mobile list/form and desktop list screenshots.


## Legacy placement compatibility

Application placement is separate from category IDs: a historical category with
ID `apps` remains a bookmark group. An omitted/empty bookmark category remains in
`bookmarks.yml`, following the homepage's first named group (or the ungrouped
bookmark list if no groups exist). The editor labels this default placement
explicitly and does not force users to reclassify it before unrelated saves.
Filtering the first named category includes default-placement links; the default
placement filter can also show just those links. Unknown nonempty category IDs
remain validation errors. `tests/editor/legacy.browser.cjs` covers both legacy
layouts, app-only saves, add/edit preservation and the special-ID collision.
