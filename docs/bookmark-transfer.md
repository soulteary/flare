# Bookmark import and export

> Scope: transfer is available since 0.6.2. The privacy and persistent-session
> requirements marked below apply to current main development, after 0.6.7.

Enable the existing editor (`--enable_editor`, enabled by default), open `/editor`,
and select **导入 / 导出浏览器书签与 Flare YAML**. Save any pending editor changes
before opening the tool; return to a fresh editor page after importing.

## Import

1. Export bookmarks as HTML from Chrome, Edge, Firefox, or another browser using
   the Netscape bookmark format. Flare also accepts its native `bookmarks.yml` or
   `apps.yml` schema (`categories` and `links`). Files must be UTF-8.
2. Select **分类书签** (`bookmarks.yml`) or **首页应用** (`apps.yml`). Importing a
   file changes only the selected collection. Applications have no categories.
3. Choose directory and duplicate handling, then select **解析并预览**.
4. Review the counts and paginated entries. Filter invalid or duplicate entries
   and optionally download the result as JSON. Change options and preview again
   if needed.
5. Confirm the import. Flare backs up the selected file before replacing it.

Directory paths become category titles joined by ` / `. Empty folders are
ignored. Optionally remove the outermost folder (for example, the bookmarks bar)
or put everything in one category. Top-level bookmarks go into **导入书签**, or
the specified default category. Order is preserved and new items are appended.

Duplicates use **target category + URL**. Only whitespace around the URL and the
case of its scheme/host are normalized for comparison. Paths, ports, query
parameters and fragments are retained. The saved URL is not rewritten beyond
trimming outer whitespace. The default skips duplicates; alternatives update
the name and non-empty icon/description or retain every entry. Updates never
remove an existing private flag. Importing an empty file never clears data.

HTTP(S), including LAN hosts and IPs, is allowed by default. Relative links and
FTP(S), mailto, tel, SSH, SMB, RDP, VNC, magnet, Steam and Obsidian links require
explicit opt-in. Executable/embedded protocols such as `javascript:`, `data:` and
`vbscript:` are always rejected. Invalid links appear in the preview. Native
icon identifiers and safe image URLs are retained; HTML embedded favicons are
ignored. Flare does not fetch links or icons during import.

Limits: **8 MiB per input file, 20,000 bookmarks/categories after merging, 32
folder levels**. A preview expires after 15 minutes or a server restart.

## Privacy choices (current main)

Authenticated imports default new resources to private. Review the preview's
visibility and inherited category restrictions. Choosing public import requires
recent password verification; changing the public option invalidates the old
preview and requires a fresh preview before confirmation. The server rechecks
recent authentication on public commit. Existing private flags are not erased by
duplicate updates. Trusted anonymous mode rejects private input/results.

A private category cannot be made public by importing a child with `private: false`.
YAML containing anchors/aliases can be read and exported as exact original YAML,
but web edits and merge imports into such documents are refused. See [accounts](accounts.md)
and [YAML upgrade boundaries](login-private-upgrade.md).

## Export and recovery

- **Browser HTML** exports names, URLs and categories, without Flare-only fields.
  Flattened directory paths remain category titles. Unsafe legacy URLs are
  omitted from HTML output. Browsers can re-import the resulting file.
- **Flare YAML** preserves names, links, category IDs/titles, icons, descriptions
  and private flags. Export both collections separately to migrate bookmarks
  and applications. Import merges into existing data; it is not a full restore.
- **下载最近一次导入前备份** downloads the latest pre-import file for the selected
  collection. Backups retain the exact original file bytes. Import it to merge
  older entries, or stop Flare and replace the corresponding YAML file to fully
  roll back that collection. Keep a copy of the current file before replacing it.

Backups are stored under `.flare-bookmark-backups/` in Flare's working directory,
with directory permissions `0700` and file permissions `0600` for new backups.
Mount/persist this directory alongside the YAML files when running containers.
Backups are not automatically deleted; operators should include them in their
backup retention policy. These exports/backups contain **bookmark data only**,
not account credentials or system settings. Bookmark URLs themselves may contain
sensitive information; protect exported files accordingly.

For current-main login-enabled deployments, both export and backup downloads
require password verification within 10 minutes and explicit download confirmation.
HTML exports do not retain Flare privacy fields; importing that HTML elsewhere
cannot preserve access rules. Treat every downloaded file as potentially private,
including backups of earlier versions of the collection.

## Consistency and access

All transfer routes reuse editor authentication and exist only when the editor
is enabled. If login is disabled, they follow the editor's existing public-access
behavior. POST requests require a custom same-origin header; current main also requires
a session/browser-bound CSRF token (`X-CSRF-Token` or the form field). Preview
credentials are signed and expire after 15 minutes; current-main tickets bind to
the active session/browser, so reauthentication can require a fresh preview. Downloads are
attachments and all data responses disable caching.

Preview does not commit imported data. Ordinary missing-file initialization is
subject to recovery and authorization guards; it must not repair a missing file
during tentative authenticated reads or when retained recovery copies exist. Confirmation checks the on-disk SHA-256 version again
under the shared in-process writer lock. An editor save or manual file change
invalidates the preview. The backup must succeed before the original file is
replaced using a same-directory staged write, file sync and atomic rename. A
failed import leaves the original data intact. Each import affects one file;
there is no cross-file transaction. Independent processes editing the same
volume are outside the in-process lock and must be coordinated by the operator.
