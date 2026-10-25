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
