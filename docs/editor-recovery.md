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

## Recovery copies detected after restart

At startup Flare scans `.flare-editor-recovery/save-*` beneath the applications
configuration directory and logs each retained directory with an actionable
warning. The editor document includes `recoveryPaths`, and the authenticated editor
shows a persistent notice. Detection never restores, deletes, or merges files.
Symlink directories are not followed. If a live collection is missing while a
recovery directory exists, loading stops instead of creating example bookmarks.

A retained directory does not prove that the live pair is inconsistent: it can
also come from a completed rollback or failed cleanup. Stop Flare and external
writers, preserve both live files and recovery copies, inspect both collections,
and restore a consistent pair if needed. Once you have verified the live pair and
saved an independent backup, archive or remove the inspected recovery directory.
Do not remove a directory merely to silence the notice before checking its files.

## Editor draft capacity

Downloaded JSON drafts use compact UTF-8 JSON. Export and restore both accept up
to **64 MiB**, including optional fields and private flags; exceeding that limit
reports an error without replacing pending edits. The server's separate 32 MiB
request-body limit still applies when saving. A large restored draft may require
splitting before it can be saved; restoring does not bypass validation or revision
checks. Draft export remains distinct from the original YAML recovery pair.
