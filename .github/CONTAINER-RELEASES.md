# Container publishing

Flare publishes stable images to Docker Hub (`soulteary/flare`) and GitHub
Container Registry (`ghcr.io/soulteary/flare`). Both provide version tags such
as `0.6.1` and a rolling `latest` tag, with these platforms:

- `linux/amd64`
- `linux/arm64`
- `linux/arm/v7`
- `linux/arm/v6`

Release candidates (`0.x.y-rc.N`) are published under their own version tag
only and never move `latest`; see [Release candidates](#release-candidates).

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
checks that its GitHub Release is published and that its prerelease flag matches
the tag form (stable `0.x.y`, or a prerelease for `0.x.y-rc.N`), resolves the Docker Hub
version tag to one fixed digest, and verifies every platform's version, source
repository, and commit labels. All four expected platforms must be present.

Version tags are immutable in this workflow: an existing identical GHCR tag is
accepted, but a different digest is not overwritten. Destination contents and
anonymous access are verified after the copy. The workflow summary records the
source commit and shared manifest digest.

`latest` is promoted only after version verification, and only when GitHub's
current stable release and Docker Hub's current `latest` both match that version.
A higher version already on GHCR is not replaced by an older one. Publications
are serialized, with queued runs retained, so backfills do not race each other.
Version publication and `latest` promotion are separate operations; either can
be safely retried. A release candidate is never promoted to `latest`, whatever
the request.

## Release candidates

A release candidate lets a change run in real deployments before it reaches
`latest`. Use one when a release changes behaviour that existing deployments
depend on, such as defaults, access control, request validation, on-disk
formats or page layout. A dependency-only patch can be tagged as stable directly.

Tag the commit intended for release as `0.x.y-rc.N` (`N` starts at 1, without a
leading zero) and push the tag:

```sh
git tag 0.7.0-rc.1 <commit>
git push origin 0.7.0-rc.1
```

For that tag the `Release` workflow publishes:

- a GitHub Release marked as a prerelease, with the usual archives and checksums;
- `soulteary/flare:0.7.0-rc.1` on Docker Hub, with the per-architecture tags
  `linux-<arch>-0.7.0-rc.1`;
- `ghcr.io/soulteary/flare:0.7.0-rc.1`, copied by digest as for a stable version.

It does not move `latest` on either registry, the floating per-architecture tags
(`linux-amd64`, `linux-arm64`, `linux-armv7`, `linux-armv6`), or GitHub's latest
release. Deployments that follow `latest` are unaffected.

To try a candidate, pin its tag:

```sh
docker pull soulteary/flare:0.7.0-rc.1
docker pull ghcr.io/soulteary/flare:0.7.0-rc.1
```

Copy the data directory before starting a candidate. An older version refuses to
overwrite a `config.yml` last saved by a newer one, so after saving settings with
a candidate, restore the copy before returning to the previous stable image.

Leave the candidate available long enough to collect deployment feedback. If it
needs a fix, merge the fix and tag the next candidate (`0.7.0-rc.2`); do not
move an existing tag. Then publish the stable version from the same commit as
the last candidate:

```sh
git tag 0.7.0 "0.7.0-rc.2^{commit}"
git push origin 0.7.0
```

The release job releases the tag that was pushed, so a stable tag that shares its
commit with a candidate is published as the stable version. The generated
changelog of a candidate or a stable version lists the commits since the
previous stable version.

Name the candidate's commit explicitly: the report workflow can add a
report-only commit to `main` after a tag is pushed, so the branch head may no
longer be the tested commit. The stable release is a separate build of that
commit. It embeds the stable version string, so its image digest differs from
the candidate's. `latest` and the floating per-architecture tags move only with
the stable tag.

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

Accepted versions are stable `major.minor.patch` and release candidates
`major.minor.patch-rc.N`, without a `v` prefix. A candidate must be a GitHub
prerelease and a stable version must not be; `update_latest` has no effect for a
candidate. Drafts, missing releases, incomplete platform indexes, and images
whose labels disagree with the release commit are rejected.

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
