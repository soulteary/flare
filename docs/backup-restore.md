# Backups and restore

Stop Flare and other writers before taking a consistent full backup or replacing
configuration files. Keep the original volume/project identity during upgrades.

| Artifact | Contains | Restore/use | Limits |
| --- | --- | --- | --- |
| Full data-directory backup | `config.yml`, `apps.yml`, `bookmarks.yml`, application `.env`, cookie key and backup/recovery directories | Stop writers; preserve current data; restore directory with original ownership/modes; restart and verify | External environment, Compose file, proxy and externally configured secrets need separate backup |
| Browser HTML export | Names, links, categories | Preview and merge through import/export | No account, search settings or full Flare configuration |
| Flare YAML export | One navigation collection | Preview and merge, or replace that collection while stopped | Merge is not full replacement or full-site restore |
| `.flare-bookmark-backups/` | Import-before copies | Download recent backup or inspect filesystem copies | Import backups only; not all ordinary edits or `config.yml` |
| `.flare-editor-recovery/save-*` | Original pair before an editor replacement (since 0.6.5) | Follow [recovery instructions](editor-recovery.md) for the exact reported directory | Not a new draft; no automatic merge; do not blindly choose a directory |
| `.flare-search-backups/` (0.6.6) | Full `config.yml` before managed settings changes, including permanent first-upgrade backup, rotating `latest.bak` and ownership/hash `manifest.json` | Preserve the entire directory; follow the configuration recovery steps below | Contains secrets; shared by configuration settings pages, not only search; not a full deployment backup |
| Editor JSON draft | Unsaved editor document (since 0.6.5) | Restore into working state, inspect, then explicitly save | Not browser import, not full site/account backup |

For a default bind mount, preserve `./app` on the host, mounted as `/app` in the
container. Include the hidden cookie key and recovery/backup directories. Do not
replace the key unnecessarily: existing signed login sessions become invalid.
Use encrypted, access-controlled storage for backups containing credentials or
private URLs. Test restoration into a separate disposable deployment before
relying on a backup. A healthy container alone does not prove navigation restored.

A simple stopped bind-mount backup from the deployment directory:

```sh
docker compose stop flare
tar -czf "flare-data-$(date +%Y%m%d-%H%M%S).tar.gz" app
docker compose start flare
```

For named volumes, use the existing volume name and an appropriate volume backup
tool; do not create a different Compose project as a substitute for restoring.

## Configuration backups in 0.6.6

Managed settings writes use `.flare-search-backups/` alongside `config.yml`.
`first-upgrade-to-<target-version>.bak` retains the original pre-upgrade document;
`latest.bak` holds the preceding document on subsequent changes. Older managed
`first-upgrade.bak` files remain intact. The manifest records ownership, hashes
and interrupted-rotation state. Preserve this directory in full backups.

To recover, stop Flare and other writers, preserve the complete current data
directory, and inspect the intended backup. Restore the selected configuration
with the deployment's ownership and private permissions. Keep the managed backup
files and manifest together and unchanged; do not edit their hashes to bypass a
save failure. A missing, modified, symlinked or insecure managed backup can block
settings writes. Resolve the reported condition from a known-good full backup
before resuming writes. See [configuration backup safeguards](search-hybrid.md#legacy-configuration-safety).

For a version rollback, restore the entire pre-upgrade data directory and external
configuration with the old binary. Restoring only `config.yml` does not establish
compatibility of the rest of the deployment. These managed backups do not replace
full backups or the separate editor recovery pair.
