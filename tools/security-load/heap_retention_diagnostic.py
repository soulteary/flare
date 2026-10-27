#!/usr/bin/env python3
"""Short, paired pool-retention diagnostic; never a timed CPU/performance headline."""
import argparse
import hashlib
import importlib.util
import json
import os
import pathlib
import subprocess
import time

_spec = importlib.util.spec_from_file_location("resource_analysis", pathlib.Path(__file__).with_name("resource_analysis.py"))
resource_runner = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(resource_runner)
acceptance = resource_runner.acceptance


def run(args):
    directory = pathlib.Path(args.directory).resolve()
    metadata = acceptance.fixture(directory)
    output = pathlib.Path(args.output).resolve()
    profiles = output.parent / (output.stem + "-profiles")
    profiles.mkdir(parents=True, exist_ok=True)
    environment = dict(os.environ, FLARE_PORT=str(args.port), FLARE_USER="scale-admin", FLARE_PASS="scale-test-password-2026", FLARE_DISABLE_LOGIN="false", FLARE_VISIBILITY="DEFAULT", FLARE_OFFLINE="true", FLARE_EDITOR="true", FLARE_PROFILE_TEST_ENABLE="1", FLARE_PROFILE_TEST_PORT=str(args.control_port))
    log = open(directory / "server.log", "wb")
    process = subprocess.Popen([args.binary], cwd=directory, env=environment, stdout=log, stderr=subprocess.STDOUT)
    result = {"started_utc": acceptance.utc_now(), "fixture": metadata, "binary_sha256": hashlib.sha256(pathlib.Path(args.binary).read_bytes()).hexdigest(), "harness_sha256": hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest(), "purpose": "short paired1/2/3forcedGCretention diagnostic, noCPUheadline"}
    try:
        for _ in range(300):
            try:
                status, body, _ = acceptance.Client(args.port).request()
                if status == 200 and b"PUBLIC Entry 19998" in body:
                    break
            except OSError:
                pass
            if process.poll() is not None:
                raise RuntimeError("wrapper exited")
            time.sleep(.1)
        else:
            raise RuntimeError("wrapper readiness timeout")
        client = acceptance.Client(args.port)
        assert client.login()[0] == 303
        assert acceptance.contains_private_card(client.request()[1])
        assert acceptance.contains_private_card(client.request("/?search=PRIVATE%20Entry%2019999")[1])
        (profiles / "warm-heap-gc.pb.gz").write_bytes(resource_runner.control_get(args.control_port, "/debug/pprof/heap?gc=1"))
        result["runtime_before"] = json.loads(resource_runner.control_get(args.control_port, "/runtime"))
        result["load"] = resource_runner.equal_rate_workload(args.port, args.duration, 3)
        result["runtime_after_before_gc"] = json.loads(resource_runner.control_get(args.control_port, "/runtime"))
        snapshots = []
        for index in range(1, 4):
            filename = f"heap-after-gc-{index}.pb.gz"
            (profiles / filename).write_bytes(resource_runner.control_get(args.control_port, "/debug/pprof/heap?gc=1"))
            snapshot = json.loads(resource_runner.control_get(args.control_port, "/runtime"))
            snapshot["forced_gc_number"] = index
            snapshots.append(snapshot)
            time.sleep(.1)
        result["runtime_after_gc_sequence"] = snapshots
    except Exception as error:
        result["error"] = repr(error)
        raise
    finally:
        process.terminate()
        try:
            process.wait(timeout=15)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()
        log.close()
        result["completed_utc"] = acceptance.utc_now()
        output.write_text(json.dumps(result, indent=2))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", required=True)
    parser.add_argument("--directory", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--port", type=int, required=True)
    parser.add_argument("--control-port", type=int, required=True)
    parser.add_argument("--duration", type=int, default=15)
    run(parser.parse_args())
