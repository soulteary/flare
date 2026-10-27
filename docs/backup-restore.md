# Backups and restore

> Scope: stable 0.6.7 content backups and current-main development. Persistent
> security-directory and session-revocation steps apply only to current main.

Stop Flare and other writers before taking a consistent full backup or replacing
configuration files. Keep the original volume/project identity during upgrades.

| Artifact | Contains | Restore/use | Limits |
| --- | --- | --- | --- |
| Full data-directory backup | `config.yml`, `apps.yml`, `bookmarks.yml`, application `.env`, legacy cookie key, current-main security state and backup/recovery directories | Stop writers; preserve current data; restore directory with original ownership/modes; restart and verify | External environment, Compose file, proxy and externally configured secrets need separate backup |
| `.flare-security/` or custom `FLARE_STATE_DIR` (current main) | Instance identity, password hash/ownership, sessions, transaction and migration state | Restore the complete consistent directory while stopped; revoke sessions before starting | Required even with read-only YAML; separate volumes and external directories need separate backup |
| Browser HTML export | Names, links, categories | Preview and merge through import/export | No account, search settings or full Flare configuration |
| Flare YAML export | One navigation collection | Preview and merge, or replace that collection while stopped | Merge is not full replacement or full-site restore |
| `.flare-bookmark-backups/` | Import-before copies | Download recent backup or inspect filesystem copies | Import backups only; not all ordinary edits or `config.yml` |
| `.flare-editor-recovery/save-*` | Original pair before an editor replacement (since 0.6.5) | Follow [recovery instructions](editor-recovery.md) for the exact reported directory | Not a new draft; no automatic merge; do not blindly choose a directory |
| `.flare-search-backups/` (0.6.6) | Full `config.yml` before managed settings changes, including permanent first-upgrade backup, rotating `latest.bak` and ownership/hash `manifest.json` | Preserve the entire directory; follow the configuration recovery steps below | Contains secrets; shared by configuration settings pages, not only search; not a full deployment backup |
| Editor JSON draft | Unsaved editor document (since 0.6.5) | Restore into working state, inspect, then explicitly save | Not browser import, not full site/account backup |

For a default bind mount, preserve `./app` on the host, mounted as `/app` in the
container. Include hidden directories. For 0.6.7 preserve the cookie key to retain signed
login sessions. For current main preserve the entire security directory; a cookie
key alone cannot restore accounts or sessions. Back up a custom `FLARE_STATE_DIR`,
separate security volume, external `FLARE_PASS_FILE`, Compose and proxy settings
with the same stopped instance. Do not combine security files from different backups.
Use encrypted, access-controlled storage for backups containing credentials or
private URLs. Test restoration into a separate disposable deployment before
relying on a backup. A healthy container alone does not prove navigation restored.

A simple stopped bind-mount backup from the deployment directory:

```sh
docker compose stop flare
tar -czf "flare-data-$(date +%Y%m%d-%H%M%S).tar.gz" app
docker compose start flare
```

This example covers only files inside `app`. Add separate state volumes and
external credential/deployment sources to the stopped backup before restarting.

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

## Complete current-main instance restore

1. Stop the service and all writers; preserve the current directory, separate
   state volume and deployment inputs before replacing anything.
2. Restore the matching complete data/security backup and external inputs with
   their original ownership and private modes. Restore the same state path;
   do not initialize a replacement account or delete faulted state.
3. With the service still stopped, run the restored binary's maintenance command
   from its working directory and effective environment:

   ```sh
   flare security revoke-all
   ```

   Use `--state-dir PATH` if selecting the security directory explicitly. In a
   container, run a one-off maintenance container with the same mounts, working
   directory and environment, without starting a second daemon. A running daemon's
   exclusive lock prevents maintenance. Require the command to succeed.
4. Start the service, confirm `ready`, verify the restored password/source and
   navigation, and confirm a pre-restore Cookie cannot access private content.
   Test anonymous homepage, search, redirects and management separately.

Complete backups may contain old valid sessions. Revoking them is required for
this restore procedure; ordinary restarts alone do not revoke them. Replacing
only one YAML file is a content restore: stop writers, retain current files and
restart, without overwriting security state.

## Rollback across the security change

To return from current main to 0.6.7 or earlier, restore the pre-upgrade directory
and deployment inputs with the old binary while isolated from public traffic.
Old releases do not enforce per-resource privacy. Require whole-site login plus
PRIVATE visibility, or remove confidential content before exposing an old public
site. Restoring an old cookie key can reactivate old signed cookies. Follow the
[full upgrade and rollback procedure](login-private-upgrade.md); changing only an
image tag is not a complete rollback.
