"""Exercise publication policy using registry responses, without registry writes."""

import copy
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest
from unittest import mock


SPEC = importlib.util.spec_from_file_location(
    "publish_ghcr", Path(__file__).with_name("publish_ghcr.py")
)
publish_ghcr = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(publish_ghcr)

TAG = "0.6.1"
RC_TAG = "0.6.2-rc.1"
COMMIT = "1" * 40
DIGEST = "sha256:" + "a" * 64
OTHER_DIGEST = "sha256:" + "b" * 64
SOURCE = "docker.io/soulteary/flare"
TARGET = "ghcr.io/soulteary/flare"


def image_fixture(tag=TAG):
    """Build independent registry metadata for the four supported platforms."""
    descriptors, configs = [], {}
    for number, (architecture, variant) in enumerate(
        [("amd64", ""), ("arm64", ""), ("arm", "v7"), ("arm", "v6")], start=1
    ):
        digest = "sha256:" + str(number) * 64
        platform = {"os": "linux", "architecture": architecture}
        if variant:
            platform["variant"] = variant
        descriptors.append({"digest": digest, "platform": platform})
        configs[digest] = {
            **platform,
            "config": {"Labels": {
                "org.opencontainers.image.version": tag,
                "org.opencontainers.image.revision": COMMIT,
                "org.opencontainers.image.source": "https://github.com/soulteary/flare",
            }},
        }
    return {"manifests": descriptors}, configs


class ImageValidationTests(unittest.TestCase):
    def verify(self, manifest, configs, tag=TAG):
        """Serve JSON fixtures at the CLI boundary while exercising validation."""
        def command(*args, env=None):
            if args[:2] == ("crane", "manifest"):
                return json.dumps(manifest)
            if args[:2] == ("crane", "config"):
                return json.dumps(configs[args[-1].split("@", 1)[1]])
            self.fail(f"Unexpected registry command: {args}")

        with mock.patch.object(publish_ghcr, "run", side_effect=command):
            publish_ghcr.verify_image(f"{SOURCE}@{DIGEST}", tag, COMMIT)

    def test_all_four_platforms_are_accepted(self):
        self.verify(*image_fixture())

    def test_candidate_image_is_accepted_only_under_its_own_version(self):
        self.verify(*image_fixture(RC_TAG), tag=RC_TAG)
        # A candidate build must not pass as the stable version it precedes.
        with self.assertRaises(ValueError):
            self.verify(*image_fixture(RC_TAG), tag="0.6.2")

    def test_missing_duplicate_wrong_platform_and_invalid_digest_are_rejected(self):
        for problem in ("missing", "duplicate", "wrong_os", "wrong_variant", "digest"):
            with self.subTest(problem=problem):
                manifest, configs = image_fixture()
                if problem == "missing":
                    manifest["manifests"].pop()
                elif problem == "duplicate":
                    manifest["manifests"].append(copy.deepcopy(manifest["manifests"][0]))
                elif problem == "wrong_os":
                    manifest["manifests"][0]["platform"]["os"] = "windows"
                elif problem == "wrong_variant":
                    manifest["manifests"][-1]["platform"]["variant"] = "v5"
                else:
                    manifest["manifests"][0]["digest"] = "sha256:not-a-digest"
                with self.assertRaises(ValueError):
                    self.verify(manifest, configs)

    def test_config_platform_and_release_labels_must_match(self):
        changes = (
            ("os", "windows"),
            ("architecture", "amd64"),
            ("variant", "v7"),
            ("org.opencontainers.image.version", "0.6.0"),
            ("org.opencontainers.image.revision", "2" * 40),
            ("org.opencontainers.image.source", "https://github.com/other/flare"),
        )
        for field, value in changes:
            with self.subTest(field=field):
                manifest, configs = image_fixture()
                config = configs[manifest["manifests"][-1]["digest"]]
                if field.startswith("org."):
                    config["config"]["Labels"][field] = value
                else:
                    config[field] = value
                with self.assertRaises(ValueError):
                    self.verify(manifest, configs)

    def test_optional_config_variant_can_be_absent(self):
        manifest, configs = image_fixture()
        for config in configs.values():
            config.pop("variant", None)
        self.verify(manifest, configs)


class RegistryResponses:
    """Model CLI success/failure, including crane's nonzero --no-clobber result."""

    def __init__(self, tag=TAG):
        self.tag = tag
        self.release = {"tag_name": tag, "draft": False,
                        "prerelease": publish_ghcr.is_candidate(tag)}
        self.commit = COMMIT
        self.version_digest = None
        self.latest_digest = OTHER_DIGEST
        self.latest_version = "0.6.0"
        self.latest_release = TAG
        self.source_latest = DIGEST
        self.copy_fails = False
        self.after_copy = lambda: None
        self.commands = []

    def api(self, path):
        if path == f"releases/tags/{self.tag}":
            return self.release
        if path == f"git/ref/tags/{self.tag}":
            return {"object": {"type": "commit", "sha": self.commit}}
        if path == "releases/latest":
            return {"tag_name": self.latest_release}
        raise AssertionError(f"Unexpected GitHub API read: {path}")

    def run(self, *args, env=None):
        self.commands.append(args)
        if args[:2] == ("crane", "digest"):
            values = {
                f"{SOURCE}:{self.tag}": DIGEST,
                f"{SOURCE}:latest": self.source_latest,
                f"{TARGET}:{self.tag}": self.version_digest,
                f"{TARGET}:latest": self.latest_digest,
            }
            value = values[args[-1]]
            if value is None:
                raise subprocess.CalledProcessError(1, args, stderr="MANIFEST_UNKNOWN")
            return value
        if args[:2] == ("crane", "ls"):
            return "\n".join(["latest"] + ([self.tag] if self.version_digest else []))
        if args[:2] == ("crane", "config"):
            return json.dumps({"config": {"Labels": {
                "org.opencontainers.image.version": self.latest_version,
            }}})
        if args[:2] == ("crane", "copy"):
            if args[-1] == f"{TARGET}:latest":
                self.latest_digest = DIGEST
                return ""
            if args[-1] != f"{TARGET}:{self.tag}":
                raise AssertionError(f"Unexpected write target: {args}")
            if self.copy_fails or ("--no-clobber" in args and self.version_digest):
                raise subprocess.CalledProcessError(1, args, stderr="copy refused")
            self.version_digest = DIGEST
            self.after_copy()
            return ""
        raise AssertionError(f"Unexpected registry command: {args}")

    def writes(self, target=None):
        return [args for args in self.commands if args[:2] == ("crane", "copy")
                and (target is None or args[-1] == target)]


class PublicationPolicyTests(unittest.TestCase):
    def setUp(self):
        self.registry = RegistryResponses()
        for name, options in (
            ("api", {"side_effect": self.registry.api}),
            ("run", {"side_effect": self.registry.run}),
            ("verify_image", {}),
            ("verify_public", {}),
            ("print", {"create": True}),
        ):
            patcher = mock.patch.object(publish_ghcr, name, **options)
            setattr(self, name, patcher.start())
            self.addCleanup(patcher.stop)
        environment = mock.patch.dict(os.environ, {"GITHUB_STEP_SUMMARY": ""})
        environment.start()
        self.addCleanup(environment.stop)

    def publish(self, **kwargs):
        publish_ghcr.publish(TAG, expected_commit=COMMIT, **kwargs)

    def use_registry(self, registry):
        self.registry = registry
        self.api.side_effect = registry.api
        self.run.side_effect = registry.run

    def result(self):
        """Return the machine-readable result printed by the last publication."""
        return json.loads(self.print.call_args_list[-1].args[0])

    def test_invalid_tag_is_rejected_before_external_commands(self):
        for tag in ("latest", "v0.6.1", "0.06.1", "0.6.1-rc1", "0.6.1\n", "0.6.1;id", "../0.6.1",
                    "0.6.1-rc", "0.6.1-rc.", "0.6.1-rc.0", "0.6.1-rc.01", "0.6.1-rc.1.2",
                    "0.6.1-RC.1", "0.6.1-beta.1", "0.6.1-rc.1\n", "0.6.1-rc.1+build", "0.06.1-rc.1"):
            with self.subTest(tag=tag), self.assertRaises(ValueError):
                publish_ghcr.publish(tag)
        self.api.assert_not_called()
        self.run.assert_not_called()

    def test_draft_prerelease_or_wrong_release_is_rejected_before_registry_access(self):
        for field, value in (("draft", True), ("prerelease", True), ("tag_name", "0.6.0")):
            with self.subTest(field=field):
                self.registry.release = {"tag_name": TAG, "draft": False, "prerelease": False}
                self.registry.release[field] = value
                with self.assertRaises(ValueError):
                    self.publish()
        self.run.assert_not_called()

    def test_moved_tag_is_rejected_before_registry_access(self):
        self.registry.commit = "2" * 40
        with self.assertRaises(ValueError):
            self.publish()
        self.run.assert_not_called()

    def test_annotated_tag_is_resolved_to_its_commit(self):
        self.api.side_effect = [
            {"object": {"type": "tag", "sha": "3" * 40}},
            {"object": {"type": "commit", "sha": COMMIT}},
        ]
        self.assertEqual(publish_ghcr.tag_commit(TAG), COMMIT)
        self.assertEqual(self.api.call_args_list, [
            mock.call(f"git/ref/tags/{TAG}"), mock.call("git/tags/" + "3" * 40),
        ])

    def test_check_only_validates_source_without_writes(self):
        self.publish(check_only=True, update_latest=True)
        self.assertEqual(self.registry.writes(), [])
        self.verify_image.assert_called_once_with(f"{SOURCE}@{DIGEST}", TAG, COMMIT)
        self.verify_public.assert_not_called()

    def test_existing_identical_version_can_resume_public_verification(self):
        self.registry.version_digest = DIGEST
        self.publish()
        self.verify_image.assert_any_call(f"{TARGET}@{DIGEST}", TAG, COMMIT)
        self.assertTrue(self.verify_public.called)
        self.assertEqual(self.registry.writes(f"{TARGET}:latest"), [])

    def test_different_existing_version_is_not_overwritten_or_promoted(self):
        self.registry.version_digest = OTHER_DIGEST
        with self.assertRaises((ValueError, subprocess.CalledProcessError)):
            self.publish(update_latest=True)
        self.assertEqual(self.registry.version_digest, OTHER_DIGEST)
        self.assertEqual(self.registry.writes(f"{TARGET}:latest"), [])
        self.verify_public.assert_not_called()

    def test_copy_failure_does_not_promote_latest(self):
        self.registry.copy_fails = True
        with self.assertRaises((ValueError, subprocess.CalledProcessError)):
            self.publish(update_latest=True)
        self.assertEqual(self.registry.writes(f"{TARGET}:latest"), [])

    def test_source_validation_failure_prevents_all_writes(self):
        self.verify_image.side_effect = ValueError("wrong source revision")
        with self.assertRaises(ValueError):
            self.publish(update_latest=True)
        self.assertEqual(self.registry.writes(), [])

    def test_destination_validation_failure_does_not_promote(self):
        def validate(reference, *_args):
            if reference.startswith(TARGET):
                raise ValueError("destination platform missing")

        self.verify_image.side_effect = validate
        with self.assertRaises(ValueError):
            self.publish(update_latest=True)
        self.assertEqual(self.registry.writes(f"{TARGET}:latest"), [])

    def test_anonymous_version_failure_does_not_promote(self):
        self.verify_public.side_effect = ValueError("package is private")
        with self.assertRaises(RuntimeError):
            self.publish(update_latest=True)
        self.assertEqual(self.registry.writes(f"{TARGET}:latest"), [])

    def test_backfill_and_latest_guards(self):
        for reason in ("historical", "newer_target", "source_changed", "release_changed_during_copy"):
            with self.subTest(reason=reason):
                self.registry = RegistryResponses()
                self.api.side_effect = self.registry.api
                self.run.side_effect = self.registry.run
                if reason == "historical":
                    self.registry.latest_release = "0.6.2"
                elif reason == "newer_target":
                    self.registry.latest_version = "0.6.2"
                elif reason == "source_changed":
                    self.registry.source_latest = OTHER_DIGEST
                else:
                    self.registry.after_copy = lambda: setattr(self.registry, "latest_release", "0.6.2")
                self.publish(update_latest=True)
                self.assertEqual(self.registry.version_digest, DIGEST)
                self.assertEqual(self.registry.writes(f"{TARGET}:latest"), [])

    def test_verified_current_release_promotes_the_pinned_digest(self):
        self.publish(update_latest=True)
        copies = self.registry.writes()
        self.assertEqual(copies[0][-2:], (f"{SOURCE}@{DIGEST}", f"{TARGET}:{TAG}"))
        self.assertEqual(copies[-1][-2:], (f"{TARGET}@{DIGEST}", f"{TARGET}:latest"))
        self.assertEqual(self.registry.latest_digest, DIGEST)
        self.assertEqual(self.result()["prerelease"], False)
        self.assertEqual(self.result()["latest_updated"], True)

    def test_tags_are_classified_as_stable_or_candidate(self):
        for tag in ("0.6.1", "0.10.0", "1.0.0"):
            with self.subTest(tag=tag):
                self.assertIs(publish_ghcr.is_candidate(tag), False)
        for tag in (RC_TAG, "0.10.0-rc.12"):
            with self.subTest(tag=tag):
                self.assertIs(publish_ghcr.is_candidate(tag), True)

    def test_release_candidate_is_mirrored_without_touching_latest(self):
        registry = RegistryResponses(RC_TAG)
        # Even if GitHub and Docker Hub both reported the candidate as current,
        # the script itself must leave latest alone.
        registry.latest_release = RC_TAG
        self.use_registry(registry)
        publish_ghcr.publish(RC_TAG, expected_commit=COMMIT, update_latest=True)
        self.assertEqual(registry.version_digest, DIGEST)
        self.assertEqual(
            [args[-2:] for args in registry.writes()],
            [(f"{SOURCE}@{DIGEST}", f"{TARGET}:{RC_TAG}")],
        )
        self.assertEqual(registry.latest_digest, OTHER_DIGEST)
        self.assertNotIn(mock.call("releases/latest"), self.api.call_args_list)
        self.assertFalse(any(args[-1].endswith(":latest") for args in registry.commands))
        self.verify_image.assert_any_call(f"{SOURCE}@{DIGEST}", RC_TAG, COMMIT)
        self.verify_image.assert_any_call(f"{TARGET}@{DIGEST}", RC_TAG, COMMIT)
        self.verify_public.assert_called_once_with(RC_TAG, DIGEST, COMMIT, latest=False)
        self.print.assert_any_call("Skipping latest: release candidates never move latest")
        self.assertEqual(self.result()["prerelease"], True)
        self.assertEqual(self.result()["latest_updated"], False)

    def test_release_candidate_without_latest_request_is_mirrored(self):
        self.use_registry(RegistryResponses(RC_TAG))
        publish_ghcr.publish(RC_TAG, expected_commit=COMMIT)
        self.assertEqual(self.registry.version_digest, DIGEST)
        self.assertEqual(self.registry.writes(f"{TARGET}:latest"), [])

    def test_release_candidate_check_only_performs_no_writes(self):
        self.use_registry(RegistryResponses(RC_TAG))
        publish_ghcr.publish(RC_TAG, expected_commit=COMMIT, update_latest=True, check_only=True)
        self.assertEqual(self.registry.writes(), [])
        self.verify_image.assert_called_once_with(f"{SOURCE}@{DIGEST}", RC_TAG, COMMIT)

    def test_candidate_tag_and_prerelease_flag_must_agree(self):
        for tag, prerelease in ((RC_TAG, False), (TAG, True)):
            with self.subTest(tag=tag, prerelease=prerelease):
                registry = RegistryResponses(tag)
                registry.release["prerelease"] = prerelease
                self.use_registry(registry)
                with self.assertRaises(ValueError):
                    publish_ghcr.publish(tag, expected_commit=COMMIT, update_latest=True)
        self.run.assert_not_called()

    def test_draft_release_candidate_is_rejected_before_registry_access(self):
        registry = RegistryResponses(RC_TAG)
        registry.release["draft"] = True
        self.use_registry(registry)
        with self.assertRaises(ValueError):
            publish_ghcr.publish(RC_TAG, expected_commit=COMMIT)
        self.run.assert_not_called()

    def test_latest_promotion_rejects_a_candidate_before_any_lookup(self):
        with self.assertRaises(ValueError):
            publish_ghcr.promote_latest(RC_TAG, DIGEST)
        self.api.assert_not_called()
        self.run.assert_not_called()

    def test_step_summary_states_candidate_and_latest_outcome(self):
        self.use_registry(RegistryResponses(RC_TAG))
        with tempfile.TemporaryDirectory() as directory:
            summary = Path(directory, "summary.md")
            with mock.patch.dict(os.environ, {"GITHUB_STEP_SUMMARY": str(summary)}):
                publish_ghcr.publish(RC_TAG, expected_commit=COMMIT, update_latest=True)
            written = summary.read_text(encoding="utf-8")
        self.assertIn(f"- Image: `{TARGET}:{RC_TAG}`", written)
        self.assertIn("- Release candidate: True", written)
        self.assertIn("- Updated `latest`: False", written)


class AnonymousAccessTests(unittest.TestCase):
    def test_verification_uses_empty_credentials_and_removes_token_environment(self):
        secret_keys = ("GH_TOKEN", "GITHUB_TOKEN", "DOCKER_AUTH_CONFIG", "REGISTRY_AUTH_FILE")
        environments = []

        def command(*args, env=None):
            self.assertIsNotNone(env)
            self.assertTrue(all(key not in env for key in secret_keys))
            config = Path(env["DOCKER_CONFIG"], "config.json")
            self.assertEqual(json.loads(config.read_text()), {"auths": {}})
            environments.append(env)
            return DIGEST

        with mock.patch.dict(os.environ, {key: "test-only-token" for key in secret_keys}), \
                mock.patch.object(publish_ghcr, "run", side_effect=command), \
                mock.patch.object(publish_ghcr, "verify_image") as verify:
            publish_ghcr.verify_public(TAG, DIGEST, COMMIT, latest=True)
            self.assertEqual(len(environments), 2)
            self.assertTrue(all(os.environ[key] == "test-only-token" for key in secret_keys))
            verify.assert_called_once_with(f"{TARGET}@{DIGEST}", TAG, COMMIT, env=environments[0])
        self.assertFalse(Path(environments[0]["DOCKER_CONFIG"]).exists())


class WorkflowSafetyTests(unittest.TestCase):
    def test_publication_queue_retains_waiting_runs(self):
        workflow = (Path(__file__).parent.parent / "workflows" / "publish-ghcr.yml").read_text()
        concurrency = re.search(r"(?m)^concurrency:\n((?:[ \t]+[^\n]*\n|\n)*)", workflow)
        self.assertIsNotNone(concurrency, "Publishing requires workflow-level concurrency")
        # actionlint does not yet recognize queue, so protect both literal
        # settings while its unsupported-property diagnostic is suppressed.
        self.assertRegex(concurrency.group(1), r"(?m)^  queue: max$")
        self.assertRegex(concurrency.group(1), r"(?m)^  cancel-in-progress: false$")


class ReleaseCandidateConfigurationTests(unittest.TestCase):
    """Pin the settings that keep a release candidate away from stable tags.

    Publication cannot run in CI, so these checks read the configuration text.
    """

    ROOT = Path(__file__).parent.parent.parent

    def test_release_workflow_accepts_stable_and_candidate_tags_only(self):
        workflow = (self.ROOT / ".github" / "workflows" / "release.yml").read_text()
        gate = re.search(r'\[\[ "\$GITHUB_REF_NAME" =~ (\S+) \]\]', workflow)
        self.assertIsNotNone(gate, "The release job must validate its tag")
        pattern = re.compile(gate.group(1))
        for tag in ("0.6.1", "0.10.0", RC_TAG, "0.10.0-rc.12"):
            with self.subTest(tag=tag):
                self.assertIsNotNone(pattern.fullmatch(tag))
                # Every tag the workflow publishes must be one the mirror accepts.
                publish_ghcr.is_candidate(tag)
        for tag in ("1.0.0", "v0.6.1", "0.6.1-rc", "0.6.1-rc1", "0.6.1-rc.0", "0.6.1-rc.01",
                    "0.6.1-rc.1.2", "0.6.1-RC.1", "0.6.1-beta.1", "0.6.1-rc.1+build"):
            with self.subTest(tag=tag):
                self.assertIsNone(pattern.fullmatch(tag))

    def test_release_workflow_requests_latest_for_stable_tags_only(self):
        workflow = (self.ROOT / ".github" / "workflows" / "release.yml").read_text()
        self.assertEqual(
            re.findall(r"(?m)^      update_latest: (.+)$", workflow),
            ["${{ !contains(github.ref_name, '-') }}"],
        )

    def test_release_job_names_the_pushed_tag_and_previous_stable_tag(self):
        # A stable tag normally shares its commit with its last candidate, and
        # GoReleaser alone would release the candidate tag on that commit.
        workflow = (self.ROOT / ".github" / "workflows" / "release.yml").read_text()
        self.assertIn("          GORELEASER_CURRENT_TAG: ${{ github.ref_name }}\n", workflow)
        self.assertIn("          GORELEASER_PREVIOUS_TAG: ${{ steps.tags.outputs.previous }}\n", workflow)
        self.assertIn(
            "git describe --tags --abbrev=0 --exclude='*-rc.*' \"refs/tags/$GITHUB_REF_NAME^\"",
            workflow,
        )

    def test_goreleaser_sorts_candidates_below_their_stable_tag(self):
        config = (self.ROOT / ".goreleaser.yaml").read_text()
        self.assertRegex(config, r'(?m)^git:\n  prerelease_suffix: "-"$')

    def test_goreleaser_marks_candidates_as_prereleases(self):
        config = (self.ROOT / ".goreleaser.yaml").read_text()
        self.assertRegex(config, r"(?m)^release:\n  prerelease: auto$")

    def test_goreleaser_moves_latest_for_stable_tags_only(self):
        config = (self.ROOT / ".goreleaser.yaml").read_text()
        manifests = re.split(r"(?m)^  - name_template: ", config.split("\ndocker_manifests:\n", 1)[1])[1:]
        skip_push = {}
        for manifest in manifests:
            name = manifest.split("\n", 1)[0].strip('"')
            values = re.findall(r"(?m)^    skip_push: (.+)$", manifest)
            self.assertEqual(len(values), 1, name)
            skip_push[name] = values[0]
        self.assertEqual(skip_push, {
            "soulteary/flare:{{ .Tag }}": '"false"',
            "soulteary/flare:latest": "auto",
        })

    def test_goreleaser_moves_floating_architecture_tags_for_stable_tags_only(self):
        config = (self.ROOT / ".goreleaser.yaml").read_text()
        dockers = config.split("\ndockers:\n", 1)[1].split("\ndocker_manifests:\n", 1)[0]
        images = re.findall(r'(?m)^      - "([^"]*soulteary/flare:[^"]*)"$', dockers)
        versioned = [image for image in images if "{{ .Tag }}" in image]
        floating = [image for image in images if "{{ .Tag }}" not in image]
        self.assertEqual(len(versioned), 4)
        self.assertEqual(len(floating), 4)
        for image in floating:
            with self.subTest(image=image):
                # GoReleaser skips an image template that renders to an empty string.
                self.assertRegex(
                    image,
                    r"\A\{\{ if not \.Prerelease \}\}soulteary/flare:linux-[a-z0-9]+\{\{ end \}\}\Z",
                )


if __name__ == "__main__":
    unittest.main()
