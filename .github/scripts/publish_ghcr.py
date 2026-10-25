#!/usr/bin/env python3
"""Copy a published Flare image to GHCR without rebuilding or changing its digest."""

import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile


REPOSITORY = "soulteary/flare"
SOURCE = f"docker.io/{REPOSITORY}"
TARGET = f"ghcr.io/{REPOSITORY}"
PLATFORMS = {"linux/amd64", "linux/arm64", "linux/arm/v7", "linux/arm/v6"}
DIGEST_PATTERN = re.compile(r"sha256:[0-9a-f]{64}\Z")
TAG_PATTERN = re.compile(r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\Z")


def run(*args, env=None):
    return subprocess.run(
        args, check=True, text=True, stdout=subprocess.PIPE, env=env
    ).stdout.strip()


def api(path):
    return json.loads(run("gh", "api", f"repos/{REPOSITORY}/{path}"))


def version_tuple(tag):
    if not TAG_PATTERN.fullmatch(tag):
        raise ValueError(f"Expected a stable version such as 0.6.1, got {tag!r}")
    return tuple(int(part) for part in tag.split("."))


def require_digest(digest):
    if not DIGEST_PATTERN.fullmatch(digest):
        raise ValueError(f"Invalid image digest: {digest!r}")
    return digest


def tag_commit(tag):
    obj = api(f"git/ref/tags/{tag}")["object"]
    # Support both lightweight and annotated tags, without resolving a branch
    # with the same name or trusting release.target_commitish (often 'main').
    for _ in range(10):
        if obj["type"] == "commit":
            sha = obj["sha"]
            if not re.fullmatch(r"[0-9a-f]{40}", sha):
                raise ValueError("The release tag does not resolve to a commit SHA")
            return sha
        if obj["type"] != "tag":
            break
        obj = api(f"git/tags/{obj['sha']}")["object"]
    raise ValueError("The release tag does not resolve to a commit")


def verify_image(reference, tag, commit, env=None):
    manifest = json.loads(run("crane", "manifest", reference, env=env))
    descriptors = manifest.get("manifests", [])
    platforms = set()
    for descriptor in descriptors:
        platform = descriptor["platform"]
        name = "/".join(
            platform[key] for key in ("os", "architecture", "variant")
            if platform.get(key)
        )
        if name not in PLATFORMS or name in platforms:
            raise ValueError(f"Unexpected or duplicate image platform: {name}")
        platforms.add(name)
        digest = require_digest(descriptor["digest"])
        repository = reference.split("@", 1)[0]
        config = json.loads(run("crane", "config", f"{repository}@{digest}", env=env))
        if (config.get("os"), config.get("architecture")) != (
            platform["os"], platform["architecture"]
        ):
            raise ValueError(f"Image config does not match platform {name}")
        if platform["architecture"] == "arm" and config.get("variant") not in (
            None, "", platform["variant"]
        ):
            raise ValueError(f"Image config does not match ARM variant {name}")
        labels = config.get("config", {}).get("Labels", {})
        expected = {
            "org.opencontainers.image.version": tag,
            "org.opencontainers.image.revision": commit,
            "org.opencontainers.image.source": f"https://github.com/{REPOSITORY}",
        }
        for key, value in expected.items():
            if labels.get(key) != value:
                raise ValueError(f"{name}: {key} does not match the published release")
    if platforms != PLATFORMS:
        raise ValueError(f"Expected all four platforms; found {sorted(platforms)}")


def promote_latest(tag, digest):
    # Re-read the release and source latest immediately before promotion.
    # A historical backfill must not roll the rolling tag back.
    if api("releases/latest")["tag_name"] != tag:
        print("Skipping latest: this is not the current GitHub stable release")
        return False
    if run("crane", "digest", f"{SOURCE}:latest") != digest:
        print("Skipping latest: Docker Hub latest points to another image")
        return False
    tags = run("crane", "ls", TARGET).splitlines()
    if "latest" in tags:
        current = json.loads(run("crane", "config", "--platform", "linux/amd64", f"{TARGET}:latest"))
        current_tag = current["config"]["Labels"]["org.opencontainers.image.version"]
        if version_tuple(current_tag) > version_tuple(tag):
            print("Skipping latest: GHCR already contains a newer stable version")
            return False
    run("crane", "copy", f"{TARGET}@{digest}", f"{TARGET}:latest")
    if run("crane", "digest", f"{TARGET}:latest") != digest:
        raise ValueError("GHCR latest digest does not match the verified version")
    return True


def verify_public(tag, digest, commit, latest):
    # An empty Docker config and no token environment prove anonymous access.
    # GHCR packages are private by default on first publication.
    with tempfile.TemporaryDirectory(prefix="flare-ghcr-anonymous-") as directory:
        Path(directory, "config.json").write_text('{"auths": {}}\n', encoding="utf-8")
        env = dict(os.environ, DOCKER_CONFIG=directory)
        for key in ("GH_TOKEN", "GITHUB_TOKEN", "DOCKER_AUTH_CONFIG", "REGISTRY_AUTH_FILE"):
            env.pop(key, None)
        if run("crane", "digest", f"{TARGET}:{tag}", env=env) != digest:
            raise ValueError("The public GHCR version does not match the source digest")
        verify_image(f"{TARGET}@{digest}", tag, commit, env=env)
        if latest and run("crane", "digest", f"{TARGET}:latest", env=env) != digest:
            raise ValueError("The public GHCR latest does not match the source digest")


def publish(tag, expected_commit="", update_latest=False, check_only=False):
    version_tuple(tag)
    release = api(f"releases/tags/{tag}")
    if release["tag_name"] != tag or release["draft"] or release["prerelease"]:
        raise ValueError("Only published stable releases can be mirrored")
    commit = tag_commit(tag)
    if expected_commit and commit != expected_commit:
        raise ValueError("The release tag moved or does not match the triggering commit")

    digest = require_digest(run("crane", "digest", f"{SOURCE}:{tag}"))
    verify_image(f"{SOURCE}@{digest}", tag, commit)
    if check_only:
        print(json.dumps({"tag": tag, "commit": commit, "digest": digest, "check_only": True}))
        return

    # --no-clobber keeps existing version tags immutable. A different existing
    # digest fails the check below instead of silently replacing a release.
    try:
        run("crane", "copy", "--no-clobber", f"{SOURCE}@{digest}", f"{TARGET}:{tag}")
    except subprocess.CalledProcessError as error:
        # crane --no-clobber also returns an error for an identical existing tag.
        # Accept it only after reading the destination; all content is verified
        # below. A failed or partial copy is not treated as a successful retry.
        if run("crane", "digest", f"{TARGET}:{tag}") != digest:
            raise ValueError("GHCR version already exists with a different digest; refusing to overwrite it") from error
        print("GHCR version already has the verified digest; checking existing contents")
    if run("crane", "digest", f"{TARGET}:{tag}") != digest:
        raise ValueError("GHCR version already exists with a different digest; refusing to overwrite it")
    verify_image(f"{TARGET}@{digest}", tag, commit)
    try:
        verify_public(tag, digest, commit, latest=False)
    except (subprocess.CalledProcessError, ValueError) as error:
        raise RuntimeError(
            "GHCR upload completed, but anonymous verification failed. For the first "
            "publication, open https://github.com/users/soulteary/packages/container/flare/settings "
            "and set the package visibility to Public, then rerun this workflow."
        ) from error
    latest = update_latest and promote_latest(tag, digest)
    if latest:
        verify_public(tag, digest, commit, latest=True)
    result = {"tag": tag, "commit": commit, "digest": digest, "latest_updated": latest,
              "platforms": sorted(PLATFORMS), "public": True}
    print(json.dumps(result))
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a", encoding="utf-8") as summary:
            summary.write(
                f"## GHCR publication verified\n\n"
                f"- Image: `{TARGET}:{tag}`\n- Source commit: `{commit}`\n"
                f"- Source and destination digest: `{digest}`\n"
                f"- Platforms: {', '.join(sorted(PLATFORMS))}\n"
                f"- Updated `latest`: {latest}\n- Anonymous access: verified\n"
            )


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--tag", required=True)
    parser.add_argument("--expected-commit", default="")
    parser.add_argument("--update-latest", action="store_true")
    parser.add_argument("--check-only", action="store_true")
    args = parser.parse_args()
    publish(args.tag, args.expected_commit, args.update_latest, args.check_only)
