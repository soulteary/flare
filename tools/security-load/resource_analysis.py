#!/usr/bin/env python3
"""Equal offered-rate administrator resource experiment, disposable directories only.

The instrumented-wrapper control contract is optional; immutable production
binaries still report OS CPU-time deltas and RSS, never invented allocation data.
"""

import argparse
import concurrent.futures
import importlib.util
import json
import math
import pathlib
import subprocess
import threading
import time

_spec = importlib.util.spec_from_file_location(
    "scale_acceptance", pathlib.Path(__file__).with_name("acceptance.py")
)
acceptance = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(acceptance)


def cpu_seconds(value):
    """Parse POSIX ps cumulative CPU [days-][hours:]minutes:seconds.centis."""
    days = 0
    if "-" in value:
        day, value = value.split("-", 1)
        days = int(day)
    parts = [float(part) for part in value.strip().split(":")]
    if len(parts) not in (2, 3):
        raise ValueError(f"unsupported ps CPU time: {value}")
    return days * 86400 + sum(part * (60 ** index) for index, part in enumerate(reversed(parts)))


def process_sample(pid):
    raw = subprocess.check_output(
        ["ps", "-p", str(pid), "-o", "rss=,%cpu=,time="], text=True
    ).split()
    return {
        "rss_kib": int(raw[0]),
        "cpu_percent": float(raw[1]),
        "cpu_seconds": cpu_seconds(raw[2]),
    }


def bounded_offered_load(duration, rate, concurrency, operation):
    """Monotonic open loop with zero waiting queue and bounded running jobs."""
    started = time.monotonic()
    offered = math.ceil(duration * rate)
    permit = threading.BoundedSemaphore(concurrency)
    running = set()
    lock = threading.Lock()
    dropped, errors = [], []

    def job(index):
        try:
            operation(index)
        except Exception as error:
            with lock:
                errors.append({"index": index, "error": repr(error)})
        finally:
            permit.release()

    def completed(future):
        with lock:
            running.discard(future)

    with concurrent.futures.ThreadPoolExecutor(max_workers=concurrency) as executor:
        for index in range(offered):
            due = started + index / rate
            remaining = due - time.monotonic()
            if remaining > 0:
                time.sleep(remaining)
            # A late scheduler does not build an immediate catch-up burst.
            if time.monotonic() - due >= 1 / rate:
                dropped.append({"index": index, "reason": "scheduler-late"})
                continue
            if not permit.acquire(blocking=False):
                dropped.append({"index": index, "reason": "inflight-cap"})
                continue
            future = executor.submit(job, index)
            with lock:
                running.add(future)
            future.add_done_callback(completed)
        # Keep the offered observation window exactly duration even when the last
        # scheduled job finishes before it. In-flight jobs drain afterward.
        remaining = started + duration - time.monotonic()
        if remaining > 0:
            time.sleep(remaining)
        with lock:
            pending = list(running)
        concurrent.futures.wait(pending)
    return {
        "offered": offered,
        "admitted": offered - len(dropped),
        "drops": dropped,
        "operation_errors": errors,
        "actual_elapsed_seconds": time.monotonic() - started,
        "offered_rate": rate,
        "inflight_limit": concurrency,
        "waiting_queue_capacity": 0,
    }


def equal_rate_workload(port, duration, rate=6):
    administrator = acceptance.Client(port)
    code, _, _ = administrator.login()
    if code != 303:
        raise RuntimeError(f"administrator login failed: {code}")
    cookies = administrator.cookies.copy()
    reads = {"home": [], "search": []}
    probes = []
    payload_bytes = {"home": 0, "search": 0}
    violations = []
    lock = threading.Lock()
    search = "/?search=PRIVATE%20Entry%2019999"

    def read(index):
        kind, path = ("home", "/") if index % 2 == 0 else ("search", search)
        client = acceptance.Client(port)
        client.cookies = cookies.copy()
        code, body, milliseconds = client.request(path)
        with lock:
            reads[kind].append((code, milliseconds))
            payload_bytes[kind] += len(body)
            if code != 200 or not acceptance.contains_private_card(body):
                violations.append({"kind": kind, "status": code})

    def probe(index):
        client = acceptance.Client(port)
        began = time.monotonic()
        login, _, login_ms = client.login()
        private_code, private, _ = client.request(search)
        csrf_code, raw, _ = client.request("/editor/csrf")
        csrf = json.loads(raw)["csrfToken"]
        replay = acceptance.Client(port)
        replay.cookies = client.cookies.copy()
        logout, _, _ = client.request(
            "/logout", acceptance.urllib.parse.urlencode({"csrf-token": csrf}),
            {"Content-Type": "application/x-www-form-urlencoded", "Origin": f"http://127.0.0.1:{port}"},
        )
        replay_code, replay_body, _ = replay.request(search)
        valid = (
            login == 303 and private_code == 200 and csrf_code == 200
            and logout == 303 and replay_code == 200
            and acceptance.contains_private_card(private)
            and not acceptance.contains_private_card(replay_body)
        )
        with lock:
            probes.append({
                "login_status": login, "login_ms": login_ms,
                "duration_ms": (time.monotonic() - began) * 1000,
                "private_status": private_code, "csrf_status": csrf_code,
                "logout_status": logout, "replay_status": replay_code,
                "privacy_assertions_pass": valid,
            })
            if not valid:
                violations.append({"kind": "login-revoke-replay", "index": index})

    result = {"pressure_started_utc": acceptance.utc_now()}
    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
        reads_future = executor.submit(bounded_offered_load, duration, rate, 4, read)
        probe_future = executor.submit(bounded_offered_load, duration, .5, 1, probe)
        result["read_schedule"] = reads_future.result()
        result["probe_schedule"] = probe_future.result()
    result.update(
        pressure_completed_utc=acceptance.utc_now(),
        duration_seconds=duration,
        administrator_home=acceptance.summary(reads["home"]),
        administrator_search=acceptance.summary(reads["search"]),
        probes=probes,
        response_payload_bytes=payload_bytes,
        privacy_violations=violations,
    )
    return result


def control_get(port, path, timeout=30):
    connection = acceptance.http.client.HTTPConnection("127.0.0.1", port, timeout=timeout)
    try:
        connection.request("GET", path)
        response = connection.getresponse()
        body = response.read()
        if response.status != 200:
            raise RuntimeError(f"profile control {path}: HTTP {response.status}")
        return body
    finally:
        connection.close()


def run(args):
    import hashlib
    import os
    import resource
    root = pathlib.Path(args.directory).resolve()
    root.mkdir(parents=True, exist_ok=True)
    metadata = acceptance.fixture(root)
    environment = dict(
        os.environ, FLARE_PORT=str(args.port), FLARE_USER="scale-admin",
        FLARE_PASS="scale-test-password-2026", FLARE_DISABLE_LOGIN="false",
        FLARE_VISIBILITY="DEFAULT", FLARE_OFFLINE="true", FLARE_EDITOR="true",
    )
    if args.control_port:
        environment.update(FLARE_PROFILE_TEST_ENABLE="1", FLARE_PROFILE_TEST_PORT=str(args.control_port))
    output = pathlib.Path(args.output).resolve()
    profiles = output.parent / (output.stem + "-profiles")
    if args.control_port:
        profiles.mkdir(parents=True, exist_ok=True)
    log = open(root / "server.log", "wb")
    process = subprocess.Popen([args.binary], cwd=root, env=environment, stdout=log, stderr=subprocess.STDOUT)
    result = {
        "started_utc": acceptance.utc_now(), "fixture": metadata,
        "binary_sha256": hashlib.sha256(pathlib.Path(args.binary).read_bytes()).hexdigest(),
        "harness_sha256": hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest(),
        "acceptance_helper_sha256": hashlib.sha256(pathlib.Path(acceptance.__file__).read_bytes()).hexdigest(),
        "instrumented_wrapper": bool(args.control_port), "profile_directory": str(profiles) if args.control_port else None,
    }
    samples = []
    stop = threading.Event()
    monitor_thread = None
    profile_pool = None
    profile_future = None
    try:
        for _ in range(300):
            try:
                code, body, _ = acceptance.Client(args.port).request()
                if code == 200 and b"PUBLIC Entry 19998" in body:
                    break
            except OSError:
                pass
            if process.poll() is not None:
                raise RuntimeError("application exited before readiness")
            time.sleep(.1)
        else:
            raise RuntimeError("readiness timeout")
        # Warm both authenticated code paths before snapshots. This extra session
        # is identical in all runs; the timed scheduler uses its own session.
        warm = acceptance.Client(args.port)
        assert warm.login()[0] == 303
        assert acceptance.contains_private_card(warm.request()[1])
        assert acceptance.contains_private_card(warm.request("/?search=PRIVATE%20Entry%2019999")[1])
        if args.control_port:
            (profiles / "heap-before-gc.pb.gz").write_bytes(control_get(args.control_port, "/debug/pprof/heap?gc=1"))
            (profiles / "allocs-before.pb.gz").write_bytes(control_get(args.control_port, "/debug/pprof/allocs"))
            result["runtime_before"] = json.loads(control_get(args.control_port, "/runtime"))
            profile_pool = concurrent.futures.ThreadPoolExecutor(max_workers=1)
            profile_future = profile_pool.submit(
                control_get, args.control_port, f"/debug/pprof/profile?seconds={args.duration + 3}", args.duration + 20,
            )
            time.sleep(.1)
        result["os_before"] = process_sample(process.pid)
        began = time.monotonic()

        def monitor():
            while not stop.wait(2):
                try:
                    sample = process_sample(process.pid)
                    sample["t"] = time.monotonic() - began
                    samples.append(sample)
                except Exception:
                    break
        monitor_thread = threading.Thread(target=monitor, daemon=True)
        monitor_thread.start()
        result.update(equal_rate_workload(args.port, args.duration, args.rate))
        result["os_after"] = process_sample(process.pid)
        if args.control_port:
            result["runtime_after"] = json.loads(control_get(args.control_port, "/runtime"))
            (profiles / "allocs-after.pb.gz").write_bytes(control_get(args.control_port, "/debug/pprof/allocs"))
            (profiles / "heap-after-before-gc.pb.gz").write_bytes(control_get(args.control_port, "/debug/pprof/heap"))
        stop.set()
        monitor_thread.join(timeout=3)
        if profile_future is not None:
            (profiles / "cpu.pb.gz").write_bytes(profile_future.result())
            # Forced GC is outside both the timed load and the CPU profile window.
            (profiles / "heap-after-gc.pb.gz").write_bytes(control_get(args.control_port, "/debug/pprof/heap?gc=1"))
            result["runtime_retained_after_gc"] = json.loads(control_get(args.control_port, "/runtime"))
        completed = result["administrator_home"]["requests"] + result["administrator_search"]["requests"]
        cpu_delta = result["os_after"]["cpu_seconds"] - result["os_before"]["cpu_seconds"]
        result["normalized"] = {
            "completed_reads": completed, "cpu_seconds_os": cpu_delta,
            "cpu_seconds_per_completed_read_os": cpu_delta / completed if completed else None,
            "normalization_scope": "administrator reads plus identically scheduled login/revoke/replay probes; includes scheduler-session setup",
        }
        if args.control_port:
            before, after = result["runtime_before"], result["runtime_after"]
            for field in ("cpu_user_seconds", "cpu_system_seconds", "total_alloc_bytes", "mallocs", "frees", "num_gc", "pause_total_ns"):
                result["normalized"][field + "_delta"] = after[field] - before[field]
            result["normalized"]["alloc_bytes_per_completed_read"] = result["normalized"]["total_alloc_bytes_delta"] / completed
            result["normalized"]["cpu_seconds_per_completed_read_runtime"] = (result["normalized"]["cpu_user_seconds_delta"] + result["normalized"]["cpu_system_seconds_delta"]) / completed
    except Exception as error:
        result["error"] = repr(error)
        raise
    finally:
        stop.set()
        if monitor_thread is not None:
            monitor_thread.join(timeout=3)
        if profile_pool is not None:
            profile_pool.shutdown(wait=True)
        process.terminate()
        try:
            process.wait(timeout=15)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()
        log.close()
        result["resource_samples"] = samples
        result["kernel_children_peak_rss_kib"] = resource.getrusage(resource.RUSAGE_CHILDREN).ru_maxrss / (1024 if acceptance.platform.system() == "Darwin" else 1)
        result["completed_utc"] = acceptance.utc_now()
        output.write_text(json.dumps(result, indent=2))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", required=True)
    parser.add_argument("--directory", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--port", type=int, default=15300)
    parser.add_argument("--control-port", type=int, default=0)
    parser.add_argument("--duration", type=int, default=180)
    parser.add_argument("--rate", type=float, default=6)
    run(parser.parse_args())
