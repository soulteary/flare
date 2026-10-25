"""Validate server and custom zones in the scratch image (without OS tzdata)."""
import datetime
import os
import re
import time
import urllib.error
import urllib.request
import zoneinfo

base = os.environ["FLARE_CONTAINER_URL"]
for attempt in range(60):
    try:
        with urllib.request.urlopen(base + "/", timeout=2) as response:
            body = response.read().decode()
        break
    except (OSError, urllib.error.URLError):
        if attempt == 59:
            raise
        time.sleep(0.2)
assert 'data-zone="Asia/Kathmandu"' in body
actual = re.search(r'id=clock-time>(.*?)</span>', body).group(1)
# Avoid a flaky check when a request straddles the minute boundary.
now = datetime.datetime.now(zoneinfo.ZoneInfo("Asia/Kathmandu"))
expected = {(now - datetime.timedelta(seconds=i)).strftime("%H:%M:") for i in (0, 1, 2)}
assert any(actual.startswith(value) for value in expected), actual
with urllib.request.urlopen(base + "/settings/appearance", timeout=2) as response:
    settings = response.read().decode()
assert 'data-server-zone="America/New_York"' in settings
print("Scratch-container acceptance passed: embedded server New York and custom Kathmandu zones.")
