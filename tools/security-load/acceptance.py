#!/usr/bin/env python3
"""Run local-only HTTP scale acceptance using a disposable working directory.

The fixture replaces apps.yml/bookmarks.yml in --directory. Never point this
command at a live deployment. Python's standard library is sufficient.
"""

import argparse
import collections
import concurrent.futures
import hashlib
import http.client
import json
import math
import os
import pathlib
import platform
import re
import resource
import subprocess
import threading
import time
import urllib.parse


def utc_now():
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def fixture(directory):
    directory = pathlib.Path(directory)
    directory.mkdir(parents=True, exist_ok=True)
    categories = "categories:\n" + "".join(
        f"  - id: c{i:03d}\n    title: Category {i:03d}\n" for i in range(100)
    )
    links = "links:\n" + "".join(
        f'  - name: {"PRIVATE" if i % 2 else "PUBLIC"} Entry {i:05d}\n'
        f"    link: https://service.example/{i:05d}\n"
        f"    category: c{i % 100:03d}\n"
        f"    private: {str(bool(i % 2)).lower()}\n"
        for i in range(20000)
    )
    raw = (categories + links).encode()
    (directory / "bookmarks.yml").write_bytes(raw)
    (directory / "apps.yml").write_text("links: []\n")
    return {
        "resources": 20000,
        "private": 10000,
        "categories": 100,
        "sha256": hashlib.sha256(raw).hexdigest(),
    }


class Client:
    def __init__(self, port):
        self.port = port
        self.cookies = {}

    def request(self, path="/", body=None, headers=None):
        connection = http.client.HTTPConnection("127.0.0.1", self.port, timeout=15)
        request_headers = {
            "Cookie": "; ".join(f"{k}={v}" for k, v in self.cookies.items())
        }
        request_headers.update(headers or {})
        started = time.monotonic()
        try:
            connection.request(
                "POST" if body is not None else "GET",
                path,
                body=body,
                headers=request_headers,
            )
            response = connection.getresponse()
            raw = response.read()
            for key, value in response.getheaders():
                if key.lower() == "set-cookie":
                    name, _, cookie = value.split(";")[0].partition("=")
                    self.cookies[name] = cookie
            return response.status, raw, (time.monotonic() - started) * 1000
        finally:
            connection.close()

    def login(self):
        status, raw, _ = self.request("/login")
        token = re.search(rb'name="csrf-token" value="([^"]+)"', raw)
        if not token:
            raise ValueError(f"login page missing CSRF field: HTTP {status}")
        return self.request(
            "/login",
            urllib.parse.urlencode(
                {
                    "username": "scale-admin",
                    "password": "scale-test-password-2026",
                    "csrf-token": token[1].decode(),
                }
            ),
            {
                "Content-Type": "application/x-www-form-urlencoded",
                "Origin": f"http://127.0.0.1:{self.port}",
            },
        )


def summary(rows):
    values = sorted(row[1] for row in rows)

    def quantile(fraction):
        return values[max(0, math.ceil(len(values) * fraction) - 1)] if values else None

    return {
        "requests": len(rows),
        "statuses": dict(collections.Counter(str(row[0]) for row in rows)),
        "p50_ms": quantile(0.5),
        "p95_ms": quantile(0.95),
    }


def measure_import(port, directory, baseline=False):
    # Real multipart HTTP import into an empty collection, including durable save.
    source = (directory / "bookmarks.yml").read_bytes()
    (directory / "bookmarks.yml").write_text("x-scale-sentinel: retain-me\nlinks: []\n")
    client = Client(port)
    token, login_ms = "", None
    if not baseline:
        status, _, login_ms = client.login()
        if status != 303:
            raise RuntimeError(f"import login failed: HTTP {status}")
        status, raw, _ = client.request("/editor/csrf")
        if status != 200:
            raise RuntimeError(f"import CSRF failed: HTTP {status}")
        token = json.loads(raw)["csrfToken"]
    boundary = "FlareScaleBoundary20261027"
    fields = {
        "format": "yaml", "collection": "bookmarks", "strategy": "skip", "public": "1"
    }
    parts = []
    for key, value in fields.items():
        parts.append(
            f'--{boundary}\r\nContent-Disposition: form-data; name="{key}"\r\n'
            f"\r\n{value}\r\n".encode()
        )
    parts.extend(
        [
            (
                f'--{boundary}\r\nContent-Disposition: form-data; name="file"; '
                'filename="scale.yml"\r\nContent-Type: application/yaml\r\n\r\n'
            ).encode(),
            source,
            f"\r\n--{boundary}--\r\n".encode(),
        ]
    )
    headers = {
        "Content-Type": f"multipart/form-data; boundary={boundary}",
        "X-Flare-Transfer": "1",
        "X-CSRF-Token": token,
        "Origin": f"http://127.0.0.1:{port}",
    }
    status, raw, preview_ms = client.request(
        "/editor/transfer/preview", b"".join(parts), headers
    )
    if status != 200:
        raise RuntimeError(f"import preview failed: HTTP {status}: {raw[:200]!r}")
    preview = json.loads(raw)
    headers["Content-Type"] = "application/json"
    status, raw, commit_ms = client.request(
        "/editor/transfer/commit", json.dumps({"token": preview["token"]}), headers
    )
    if status != 200:
        raise RuntimeError(f"import commit failed: HTTP {status}: {raw[:200]!r}")
    plan = json.loads(raw)
    _, body, _ = Client(port).request("/")
    return {
        "login_ms": login_ms,
        "preview_ms": preview_ms,
        "commit_ms": commit_ms,
        "added": plan["added"],
        "categories": plan["categories"],
        "anonymous_private_absent": b"PRIVATE Entry" not in body,
        "public_last_present": b"PUBLIC Entry 19998" in body,
        "source_bytes": len(source),
        "unknown_target_field_preserved": "x-scale-sentinel: retain-me" in (directory / "bookmarks.yml").read_text(),
        "private_true_count": (directory / "bookmarks.yml").read_text().count("private: true"),
    }


def pressure(port, duration):
    started = time.monotonic()
    finish = started + duration
    rows, probes = [], []
    lock = threading.Lock()

    def worker():
        client = Client(port)
        while time.monotonic() < finish:
            began = time.monotonic()
            try:
                status, _, milliseconds = client.login()
            except Exception:
                status, milliseconds = "exception", (time.monotonic() - began) * 1000
            with lock:
                rows.append((status, milliseconds))

    def probe():
        while time.monotonic() < finish:
            try:
                status, _, milliseconds = Client(port).request()
                probes.append((status, milliseconds))
            except Exception:
                probes.append(("exception", 15000))
            time.sleep(2)

    result = {"pressure_started_utc": utc_now()}
    # Fixed workers each issue one request at a time; no unbounded submission queue.
    with concurrent.futures.ThreadPoolExecutor(max_workers=5) as executor:
        futures = [executor.submit(worker) for _ in range(4)] + [executor.submit(probe)]
        for future in futures:
            future.result()
    result.update(
        pressure_completed_utc=utc_now(),
        duration_seconds=duration,
        actual_pressure_elapsed_seconds=time.monotonic() - started,
        login=summary(rows),
        accepted_login=summary([row for row in rows if row[0] == 303]),
        home_probes=summary(probes),
    )
    result["login"]["qps"] = len(rows) / (time.monotonic() - started)
    return result


def contains_private_card(body):
    # Search inputs echo the query even when the matching private card is hidden.
    # Match the actual fixture card, not the query string elsewhere in the page.
    return b"<span>PRIVATE Entry 19999</span>" in body


def administrator_pressure(port, duration):
    """Four bounded admin readers plus independent login/current-revoke probes."""
    administrator = Client(port)
    status, _, _ = administrator.login()
    if status != 303:
        raise RuntimeError(f"administrator pressure login failed: HTTP {status}")
    cookies = administrator.cookies.copy()
    paths = [("home", "/"), ("search", "/?search=PRIVATE%20Entry%2019999")]
    rows = {"home": [], "search": []}
    probes = []
    violations = []
    lock = threading.Lock()
    started = time.monotonic()
    finish = started + duration

    def reader(index):
        client = Client(port)
        client.cookies = cookies.copy()
        sequence = index
        while time.monotonic() < finish:
            name, path = paths[sequence % 2]
            sequence += 1
            began = time.monotonic()
            try:
                code, body, milliseconds = client.request(path)
                valid = code == 200 and contains_private_card(body)
            except Exception:
                code, milliseconds, valid = "exception", (time.monotonic() - began) * 1000, False
            with lock:
                rows[name].append((code, milliseconds))
                if not valid:
                    violations.append({"kind": name, "status": code})

    def login_revoke_probe():
        while time.monotonic() < finish:
            client = Client(port)
            began = time.monotonic()
            try:
                code, _, milliseconds = client.login()
                status, private, _ = client.request(paths[1][1])
                status, raw, _ = client.request("/editor/csrf")
                csrf = json.loads(raw)["csrfToken"]
                replay = Client(port)
                replay.cookies = client.cookies.copy()
                logout, _, _ = client.request(
                    "/logout", urllib.parse.urlencode({"csrf-token": csrf}),
                    {"Content-Type": "application/x-www-form-urlencoded", "Origin": f"http://127.0.0.1:{port}"},
                )
                replay_status, replay_body, _ = replay.request(paths[1][1])
                valid = code == 303 and logout == 303 and replay_status == 200 and contains_private_card(private) and not contains_private_card(replay_body)
            except Exception:
                code, milliseconds, valid = "exception", (time.monotonic() - began) * 1000, False
            with lock:
                probes.append((code, milliseconds))
                if not valid:
                    violations.append({"kind": "login-revoke-probe", "status": code})
            remaining = finish - time.monotonic()
            if remaining > 0:
                time.sleep(min(2, remaining))

    result = {"pressure_started_utc": utc_now(), "workload": "administrator-home-search-plus-login-revoke"}
    with concurrent.futures.ThreadPoolExecutor(max_workers=5) as executor:
        futures = [executor.submit(reader, i) for i in range(4)] + [executor.submit(login_revoke_probe)]
        for future in futures:
            future.result()
    actual = time.monotonic() - started
    result.update(
        pressure_completed_utc=utc_now(), duration_seconds=duration,
        actual_pressure_elapsed_seconds=actual,
        administrator_home=summary(rows["home"]),
        administrator_search=summary(rows["search"]),
        login_revoke_probes=summary(probes),
        privacy_violations=violations,
    )
    for name in ("home", "search"):
        result["administrator_" + name]["qps"] = len(rows[name]) / actual
    result["combined_read_qps"] = sum(len(value) for value in rows.values()) / actual
    return result


def run(args):
    directory = pathlib.Path(args.directory).resolve()
    metadata = fixture(directory)
    environment = dict(
        os.environ,
        FLARE_PORT=str(args.port),
        FLARE_USER="scale-admin",
        FLARE_PASS="scale-test-password-2026",
        FLARE_DISABLE_LOGIN="true" if args.baseline else "false",
        FLARE_VISIBILITY="DEFAULT",
        FLARE_OFFLINE="true",
        FLARE_EDITOR="true",
    )
    log = open(directory / "server.log", "wb")
    started = time.monotonic()
    process = subprocess.Popen(
        [args.binary], cwd=directory, env=environment, stdout=log, stderr=subprocess.STDOUT
    )
    result = {
        "started_utc": utc_now(),
        "host": platform.platform(),
        "fixture": metadata,
        "binary_sha256": hashlib.sha256(pathlib.Path(args.binary).read_bytes()).hexdigest(),
        "harness_sha256": hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest(),
    }
    samples = []
    stopped = threading.Event()

    def monitor():
        while not stopped.wait(2):
            try:
                values = subprocess.check_output(
                    ["ps", "-p", str(process.pid), "-o", "rss=,%cpu="], text=True
                ).split()
                samples.append(
                    {
                        "t": time.monotonic() - started,
                        "rss_kib": int(values[0]),
                        "cpu_percent": float(values[1]),
                    }
                )
            except Exception:
                break

    monitor_thread = threading.Thread(target=monitor, daemon=True)
    monitor_thread.start()
    try:
        for _ in range(300):
            try:
                status, body, elapsed = Client(args.port).request()
                if status == 200:
                    if process.poll() is not None:
                        raise RuntimeError("server exited; another process may own the port")
                    if b"PUBLIC Entry 19998" not in body:
                        raise RuntimeError("ready response does not contain this run's fixture")
                    break
            except OSError:
                pass
            if process.poll() is not None:
                raise RuntimeError("server exited before readiness")
            time.sleep(0.1)
        else:
            raise RuntimeError("startup timeout")
        result.update(startup_seconds=time.monotonic() - started, cold_home_ms=elapsed)
        if args.admin_pressure:
            result.update(administrator_pressure(args.port, args.duration))
        elif args.import_only:
            imported = []
            for _ in range(args.import_repeats):
                fixture(directory)
                imported.append(measure_import(args.port, directory, args.baseline))
            result["import_samples"] = imported
            result["import_preview"] = summary([(200, row["preview_ms"]) for row in imported])
            result["import_commit"] = summary([(200, row["commit_ms"]) for row in imported])
        elif args.duration:
            result.update(pressure(args.port, args.duration))
        else:
            for path, name in [("/", "home"), ("/?search=PUBLIC%20Entry%2019998", "search")]:
                began = time.monotonic()

                def one(_):
                    status, _, milliseconds = Client(args.port).request(path)
                    return status, milliseconds

                with concurrent.futures.ThreadPoolExecutor(max_workers=4) as executor:
                    rows = list(executor.map(one, range(80)))
                result[name] = summary(rows)
                result[name]["qps"] = len(rows) / (time.monotonic() - began)
            result["anonymous_private_absent"] = b"PRIVATE Entry" not in Client(args.port).request()[1]
            if not args.baseline:
                administrator = Client(args.port)
                status, _, _ = administrator.login()
                if status != 303:
                    raise RuntimeError(f"administrator benchmark login failed: HTTP {status}")
                result["administrator_private_present"] = b"PRIVATE Entry 19999" in administrator.request()[1]
                for path, name in [("/", "administrator_home"), ("/?search=PRIVATE%20Entry%2019999", "administrator_search")]:
                    began = time.monotonic()

                    def authenticated_one(_):
                        client = Client(args.port)
                        client.cookies = administrator.cookies.copy()
                        status, _, milliseconds = client.request(path)
                        return status, milliseconds

                    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as executor:
                        rows = list(executor.map(authenticated_one, range(80)))
                    result[name] = summary(rows)
                    result[name]["qps"] = len(rows) / (time.monotonic() - began)
            result["import"] = measure_import(args.port, directory, args.baseline)
    except Exception as error:
        result["error"] = str(error)
        raise
    finally:
        stopped.set()
        monitor_thread.join(timeout=3)
        process.terminate()
        try:
            process.wait(timeout=15)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()
        log.close()
        # Children here are the single server and tiny ps samples. Their maximum
        # is a kernel high-water mark, unlike the explicitly sampled RSS series.
        result["kernel_children_peak_rss_kib"] = resource.getrusage(resource.RUSAGE_CHILDREN).ru_maxrss / (1024 if platform.system() == "Darwin" else 1)
        result["resource_samples"] = samples
        result["peak_rss_kib"] = max((row["rss_kib"] for row in samples), default=0)
        result["completed_utc"] = utc_now()
        pathlib.Path(args.output).write_text(json.dumps(result, indent=2))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", required=True)
    parser.add_argument("--directory", required=True)
    parser.add_argument("--port", type=int, default=15180)
    parser.add_argument("--baseline", action="store_true")
    parser.add_argument("--admin-pressure", action="store_true")
    parser.add_argument("--import-only", action="store_true")
    parser.add_argument("--import-repeats", type=int, default=5)
    parser.add_argument("--duration", type=int, default=0)
    parser.add_argument("--output", required=True)
    run(parser.parse_args())
