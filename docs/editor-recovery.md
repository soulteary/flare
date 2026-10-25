# Editor save failures and recovery

> Availability: development branch after 0.6.4; not included in the published 0.6.4 image.

The editor reads the exact bytes of `apps.yml` and `bookmarks.yml` into one
revision. Saving a stale revision returns HTTP 409 before either file is
replaced. Reload only after exporting any pending editor draft you need to keep.

An unchanged document keeps its revision and does not replace either file.
When only one collection changes, the other collection retains its original
bytes, including comments and formatting. Changed collections are encoded as
YAML and do not retain YAML comments. A no-op save still checks the revision.

## Failed saves

Both replacement files are prepared before writes begin. Original bytes are
retained in `.flare-editor-recovery/save-*` beneath the applications configuration
directory. Each recovery directory contains `apps.yml` and `bookmarks.yml`.

Authenticated save responses use these fields:

| HTTP status | `code` | Meaning | Next action |
| --- | --- | --- | --- |
| 409 | — | The loaded revision is stale; this request did not save | Export pending data, reload and review changes |
| 422 | — | The document failed validation | Correct the reported `field` and save again |
| 500 | `save_failed` | Save failed; when `recoveryPath` is present, replaced files were restored | Keep pending edits and resolve the reported failure before retrying |
| 500 | `recovery_required` | Replacement failed and rollback was incomplete | Stop saving and restore a consistent pair of files before reloading |

A replacement failure includes `recoveryPath` and `recoveryRequired`. A generic
failure before replacement may omit these fields. An unsuccessful response always
has `saved: false`; recovery details never report a successful save.

For `recovery_required`, first stop the Flare process and any external writer.
Preserve the current configuration files and the recovery directory separately.
Inspect the recovery pair at the exact path from the response and restore **both**
files to their configured locations, preserving deployment ownership and modes.
Restart Flare, verify the navigation and reload the editor. A downloaded editor
JSON draft contains pending edits; it is not the original on-disk recovery pair.
Review a restored draft before saving it over recovered files.

## Limits

A timeout or disconnected client cannot establish whether a request committed.
Keep the pending draft, inspect the current server document and use revision
checks rather than assuming the save failed. Do not blindly overwrite a newer
server revision.

The two YAML replacements are not a cross-file atomic transaction. Process or
power loss can interrupt the pair, and external processes do not share Flare's
in-process lock. Recovery copies are retained after a reported replacement
failure; they are removed after a successful save. Normal filesystem backups are
still needed. This feature does not automatically recover after a crash or merge
conflicting edits.
