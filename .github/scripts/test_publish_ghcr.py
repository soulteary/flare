"""Exercise publication policy using registry responses, without registry writes."""

import copy
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import unittest
from unittest import mock


SPEC = importlib.util.spec_from_file_location(
    "publish_ghcr", Path(__file__).with_name("publish_ghcr.py")
)
publish_ghcr = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(publish_ghcr)

TAG = "0.6.1"
COMMIT = "1" * 40
DIGEST = "sha256:" + "a" * 64
OTHER_DIGEST = "sha256:" + "b" * 64
SOURCE = "docker.io/soulteary/flare"
TARGET = "ghcr.io/soulteary/flare"


def image_fixture():
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
                "org.opencontainers.image.version": TAG,
                "org.opencontainers.image.revision": COMMIT,
                "org.opencontainers.image.source": "https://github.com/soulteary/flare",
            }},
        }
    return {"manifests": descriptors}, configs


class ImageValidationTests(unittest.TestCase):
    def verify(self, manifest, configs):
        """Serve JSON fixtures at the CLI boundary while exercising validation."""
        def command(*args, env=None):
            if args[:2] == ("crane", "manifest"):
                return json.dumps(manifest)
            if args[:2] == ("crane", "config"):
                return json.dumps(configs[args[-1].split("@", 1)[1]])
            self.fail(f"Unexpected registry command: {args}")

        with mock.patch.object(publish_ghcr, "run", side_effect=command):
            publish_ghcr.verify_image(f"{SOURCE}@{DIGEST}", TAG, COMMIT)

    def test_all_four_platforms_are_accepted(self):
        self.verify(*image_fixture())

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

    def __init__(self):
        self.release = {"tag_name": TAG, "draft": False, "prerelease": False}
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
        if path == f"releases/tags/{TAG}":
            return self.release
        if path == f"git/ref/tags/{TAG}":
            return {"object": {"type": "commit", "sha": self.commit}}
        if path == "releases/latest":
            return {"tag_name": self.latest_release}
        raise AssertionError(f"Unexpected GitHub API read: {path}")

    def run(self, *args, env=None):
        self.commands.append(args)
        if args[:2] == ("crane", "digest"):
            values = {
                f"{SOURCE}:{TAG}": DIGEST,
                f"{SOURCE}:latest": self.source_latest,
                f"{TARGET}:{TAG}": self.version_digest,
                f"{TARGET}:latest": self.latest_digest,
            }
            value = values[args[-1]]
            if value is None:
                raise subprocess.CalledProcessError(1, args, stderr="MANIFEST_UNKNOWN")
            return value
        if args[:2] == ("crane", "ls"):
            return "\n".join(["latest"] + ([TAG] if self.version_digest else []))
        if args[:2] == ("crane", "config"):
            return json.dumps({"config": {"Labels": {
                "org.opencontainers.image.version": self.latest_version,
            }}})
        if args[:2] == ("crane", "copy"):
            if args[-1] == f"{TARGET}:latest":
                self.latest_digest = DIGEST
                return ""
            if args[-1] != f"{TARGET}:{TAG}":
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

    def test_invalid_tag_is_rejected_before_external_commands(self):
        for tag in ("latest", "v0.6.1", "0.06.1", "0.6.1-rc1", "0.6.1\n", "0.6.1;id", "../0.6.1"):
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


if __name__ == "__main__":
    unittest.main()
