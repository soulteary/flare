"""Replay the frozen baseline and current render benchmarks into a new directory."""
import argparse
import json
from pathlib import Path
import subprocess
import tempfile

BASE = "e9cadb6245cc8bc28bf5cda06a55cf5af79dd00f"
FILES = (
    "config/data/navigation_view.go",
    "config/data/parsed_cache.go",
    "internal/pages/home/application.go",
    "internal/pages/home/bookmark.go",
    "internal/pages/home/navigation_cache.go",
)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("output", type=Path, help="new output directory")
    args = parser.parse_args()
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    root = Path(__file__).resolve().parents[2]
    with tempfile.TemporaryDirectory(prefix="flare-read-baseline-") as directory:
        scratch = Path(directory)
        replacements = {}
        for name in FILES:
            target = scratch / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(subprocess.check_output(
                ["git", "show", f"{BASE}:{name}"], cwd=root))
            replacements[str(root / name)] = str(target)
        for name, package in (
            ("config/data/navigation_view_index_test.go", "data"),
            ("internal/pages/home/read_index_test.go", "home"),
        ):
            target = scratch / f"blank_{package}_test.go"
            target.write_text(f"package {package}\n")
            replacements[str(root / name)] = str(target)
        overlay = scratch / "overlay.json"
        overlay.write_text(json.dumps({"Replace": replacements}))
        for label, flags in (("baseline", [f"-overlay={overlay}"]), ("candidate", [])):
            with (output / f"{label}.txt").open("w") as stream:
                subprocess.run([
                    "go", "test", *flags, "./internal/pages/home", "-run", "^$",
                    "-bench", "^BenchmarkNavigationReadIndex$", "-benchmem",
                    "-benchtime=500ms", "-count=3",
                ], cwd=root, stdout=stream, check=True)


if __name__ == "__main__":
    main()
