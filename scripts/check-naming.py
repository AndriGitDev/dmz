"""Reject retired product naming in current tracked source and paths."""
from pathlib import Path
import subprocess

root = Path(__file__).resolve().parents[1]
retired = "shield"
failures = []
for name in subprocess.check_output(["git", "ls-files", "-z"], cwd=root).decode().split("\0"):
    if not name:
        continue
    path = root / name
    if not path.is_file():
        continue
    if retired in name.lower():
        failures.append(name + ": retired source path")
    # Recorded measurement/provenance snapshots and upstream licenses stay verbatim.
    if (name.startswith("bench/results/") and path.suffix == ".json") or (
        "/licenses/" in name and path.suffix != ".md"
    ) or name == "scripts/check-naming.py":
        continue
    try:
        text = path.read_text()
    except UnicodeError:
        continue
    # This identifier retrieves an unchanged historical release, not a current build.
    text = text.replace("shield-alpha-0.1.1-alpha.1", "")
    if retired in text.lower():
        failures.append(name + ": retired product reference")
if failures:
    raise SystemExit("\n".join(failures))
print("Current source paths and product naming use dmz; historical evidence and upstream notices remain intact.")
