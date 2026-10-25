# Navigation editor

> Availability: development branch after 0.6.4; not included in the published 0.6.4 image.

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

The integrated editor shares one working document across persistence, list/forms
and the optional batch view.

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
bookmark import file. The batch route remains a compatible entry URL; the integrated adapter shares
all pending data with the list. Switching views does not navigate or discard edits.

`node tests/editor/browser.cjs` runs the list/form acceptance suite with the same
FLARE_BINARY and CHROMIUM_EXECUTABLE variables as the mobile tests. Optional
EDITOR_SCREENSHOT_DIR captures mobile list/form and desktop list screenshots.

## Integrated batch editing

More > Batch editing opens a lazy-loaded table over the same pending document.
No navigation, separate save button or second edit buffer survives a successful
mode switch. Active cells are committed before save/switch. Incomplete or invalid
rows block save/switch and identify the first problem; they are not silently
filtered out. Reset table changes restores the last synchronized working document;
Discard changes restores the last server snapshot. These actions are deliberately
labelled differently. Category management synchronizes valid table changes first.

The table retains row movement, clipboard paste, undo/redo and optional fields.
Application order and bookmark order are reconstructed from visual row order,
without renumbering category IDs or losing private flags. Application placement
uses a different token namespace from category IDs, including legacy IDs `apps`.
The table instance is reused and stops listening to keyboard input when hidden.

Conflicts offer a JSON draft export. More > Restore editor draft explicitly loads
one back into pending data, after confirmation; it retains the revision loaded by
the current page, does not auto-merge, and still requires Save changes. Invalid
rows in a recovered draft must be fixed before saving. Native HTML/browser import
formats remain available through the existing import/export page.

Acceptance includes shared unsaved edits across modes, incomplete rows, active
cell save, actual clipboard paste, visual row ordering, empty-table repopulation,
JSON draft recovery and private flag preservation. A 1000-entry list render is
measured in the acceptance suite rather than adding pagination prematurely.
Browser timings are environment-specific measurements, not performance promises.
CI retains editor screenshots as workflow artifacts. Physical Safari/Via keyboard,
font scaling and safe areas remain a manual device checklist.


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

## Language

The editor shell, entry forms, category controls, batch table, icon picker and
status/validation messages follow `config.yml`'s `Locale: zh` or `Locale: en`.
Messages use the same embedded locale bundles as the other settings pages. User
names, addresses, category labels and descriptions are never translated. The
HTML language attribute follows the selected locale. JSON recovery drafts have a
shared 64 MiB export/restore limit; see [recovery behavior](editor-recovery.md).

Run `FLARE_BINARY=/path/to/flare node tests/editor/reliability.browser.cjs` after
building production resources to verify the English UI, startup recovery notice,
and a draft larger than the historical 4 MiB restore limit.
