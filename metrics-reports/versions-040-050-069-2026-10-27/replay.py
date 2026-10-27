#!/usr/bin/env python3
"""Replay the frozen experiment in a new directory; never replace recorded data."""
import argparse
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import uuid

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[1]
BASE = "docker.io/arm64v8/alpine@sha256:4bdb4ac63839546daabfe0a267a363b3effa17ce02ac5f42d222174484c5686c"


def run(args, **kwargs):
    subprocess.run(args, check=True, **kwargs)


def captured_source(name):
    """Load the sanitized collector; working directories come from the environment."""
    return (HERE / "capture" / (name + ".txt")).read_text()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("destination", type=Path, help="new output directory, which must not exist")
    parser.add_argument("--probe-only", action="store_true", help="build and verify the 24 inputs without timed load")
    args = parser.parse_args()
    if sys.version_info < (3, 12):
        raise SystemExit("Replay requires Python 3.12+ for safe archive extraction.")
    destination = args.destination.resolve()
    destination.mkdir(parents=True, exist_ok=False)
    root, output = destination / "work", destination / "results"
    root.mkdir()
    output.mkdir()
    env = os.environ.copy()
    env["GOTOOLCHAIN"] = "go1.27.3"
    env["FLARE_BENCH_ROOT"] = str(root)
    env["FLARE_BENCH_OUTPUT"] = str(output)
    system = json.loads(subprocess.check_output(["docker", "info", "--format", "{{json .}}"], text=True))
    if system.get("Architecture") not in ("arm64", "aarch64") or system.get("CgroupVersion") != "2":
        raise SystemExit("Replay requires a native Linux ARM64 container engine with cgroup v2.")
    frozen = json.loads((HERE / "builds.json").read_text())
    for version, build in frozen.items():
        resolved = subprocess.check_output(["git", "rev-parse", version + "^{commit}"], cwd=REPO, text=True).strip()
        if resolved != build["revision"]:
            raise SystemExit("Tag does not match frozen revision: " + version)
        source = root / ("source-" + version)
        source.mkdir()
        archive = subprocess.check_output(["git", "archive", build["revision"]], cwd=REPO)
        with tarfile.open(fileobj=io.BytesIO(archive)) as packed:
            packed.extractall(source, filter="data")
        run(["go", "run", "build/build.go"], cwd=source, env=env)
        build_env = {**env, "GOOS": "linux", "GOARCH": "arm64", "CGO_ENABLED": "0"}
        run(["go", "build", "-trimpath", "-buildvcs=false", "-o", str(root / ("built-" + version)), "."],
            cwd=source, env=build_env)
    token = uuid.uuid4().hex[:12]
    image_prefix = "flare-report-" + token
    for name in ("fixtures.py", "build-images.py"):
        script = captured_source(name)
        if name == "build-images.py":
            script = script.replace("arm64v8/alpine:3.16", BASE).replace("flare-source-bench:", image_prefix + ":")
        path = root / name
        path.write_text(script)
        run([sys.executable, str(path)], cwd=REPO, env=env)
    runner_source = root / "runner.go"
    runner_source.write_text((HERE / "capture" / "runner.go.txt").read_text())
    image = root / "runner-image"
    image.mkdir()
    run(["go", "build", "-trimpath", "-o", str(image / "runner"), str(runner_source)],
        env={**env, "GOOS": "linux", "GOARCH": "arm64", "CGO_ENABLED": "0"})
    (image / "Dockerfile").write_text('FROM ' + BASE + '\nCOPY runner /runner\nENTRYPOINT ["/runner"]\n')
    runner_name = image_prefix + ":runner"
    run(["docker", "build", "--network", "none", "-t", runner_name, str(image)])
    compare = captured_source("compare.py").replace("flare-release-benchmark-runner:20261027", runner_name)
    compare = compare.replace("flare-controlled-server", image_prefix + "-server").replace(
        "flare-controlled-observer", image_prefix + "-observer")
    # Podman can leave the server behind when both dependent containers are
    # removed in one call; remove the observer first without changing the load.
    compare = compare.replace("run(['rm','-f',obs,name],False)",
                              "run(['rm','-f',obs],False);run(['rm','-f',name],False)")
    script = root / "compare.py"
    script.write_text(compare)
    run([sys.executable, str(script), *(["--probe"] if args.probe_only else [])], env=env)
    if not args.probe_only:
        analysis = root / "analyze.py"
        analysis.write_text(captured_source("analyze.py"))
        run([sys.executable, str(analysis), "--complete"], env=env)
    print("New run saved in", output)
    print("Temporary images use prefix", image_prefix)


if __name__ == "__main__":
    main()
