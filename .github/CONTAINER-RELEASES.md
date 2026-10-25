# Container publishing

Flare publishes stable images to Docker Hub (`soulteary/flare`) and GitHub
Container Registry (`ghcr.io/soulteary/flare`). Both provide version tags such
as `0.6.1` and a rolling `latest` tag, with these platforms:

- `linux/amd64`
- `linux/arm64`
- `linux/arm/v7`
- `linux/arm/v6`

## Automatic releases

The existing `Release` workflow builds and publishes to Docker Hub with
GoReleaser. Once that job succeeds for a release tag, it calls
`publish-ghcr.yml` with the version and the exact commit that was built.
The GHCR job copies the existing multi-platform image; it does not rebuild
binaries, modify the Docker Hub tags, or recreate GitHub Release assets.

The publishing job has only `contents: read` and `packages: write`. It uses the
short-lived `GITHUB_TOKEN`, with no additional PAT or Docker Hub write secret.
Only trusted release-tag events and manual runs from `main` can publish.
Pull-request CI runs unit tests and actionlint with read-only permissions.

The image copy uses a pinned crane release with a fixed SHA-256 checksum. Before
any image is published, the script resolves the Git tag to its actual commit,
checks that its GitHub Release is published and stable, resolves the Docker Hub
version tag to one fixed digest, and verifies every platform's version, source
repository, and commit labels. All four expected platforms must be present.

Version tags are immutable in this workflow: an existing identical GHCR tag is
accepted, but a different digest is not overwritten. Destination contents and
anonymous access are verified after the copy. The workflow summary records the
source commit and shared manifest digest.

`latest` is promoted only after version verification, and only when GitHub's
current stable release and Docker Hub's current `latest` both match the candidate.
A higher version already on GHCR is not replaced by an older one. Publications
are serialized, with queued runs retained, so backfills do not race each other.
Version publication and `latest` promotion are separate operations; either can
be safely retried.

## Backfill or retry a released version

Run the new workflow from `main`, where the current publishing script exists.
There is no need to rerun GoReleaser on an old tag:

```sh
gh workflow run publish-ghcr.yml --repo soulteary/flare --ref main \
  -f version=0.6.1
```

Manual runs leave `latest` alone by default. To also promote the current stable
release, explicitly request it; all of the same latest-version checks still apply:

```sh
gh workflow run publish-ghcr.yml --repo soulteary/flare --ref main \
  -f version=0.6.1 -f update_latest=true
```

Only stable numeric versions (`major.minor.patch`, without a `v` prefix) are
accepted. Drafts, prereleases, missing releases, incomplete platform indexes,
and images whose labels disagree with the release commit are rejected.

For a read-only preflight, install the workflow's pinned crane version and run:

```sh
python3 .github/scripts/publish_ghcr.py --tag 0.6.1 --check-only
```

This needs a `gh` session that can read the repository and public registry access;
it performs no registry writes.

## First GHCR publication

GitHub creates new container packages as private, even for a public repository.
After the first upload, an owner must open the package's **Package settings**,
choose **Change visibility**, and make the `flare` package **Public**. Publishing
with this repository's `GITHUB_TOKEN` and retaining the source label links the
package to `soulteary/flare`.

The workflow intentionally fails its final anonymous check while the package is
private. Once visibility is public, rerun the same workflow; the existing version
is verified without replacing it. Do not distribute a private package URL as a
public installation source just because an authenticated push succeeded.

If a package with this name already exists but is not linked to this repository,
grant this repository Actions access in the package settings before retrying.

References:

- [GitHub Container Registry and package visibility](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
- [Reusable workflows](https://docs.github.com/en/actions/how-tos/reuse-automations/reuse-workflows)
- [crane copy v0.22.1](https://github.com/google/go-containerregistry/blob/v0.22.1/cmd/crane/doc/crane_copy.md)
