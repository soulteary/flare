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
