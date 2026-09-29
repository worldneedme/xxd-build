"""Verify the Windows portable zip carries a complete, runnable application.

The exe target hands the staging directory to Inno Setup and then removes it,
so the packaged zip is the only place left to check that flutter_windows.dll,
the Flutter data payload and the protocol helpers actually made it in. A zip
that misses any of them installs fine and then fails to start.
"""

import sys
import zipfile

REQUIRED = [
    "FlClash.exe",
    "FlClashCore.exe",
    "FlClashHelperService.exe",
    "manifest.json",
    "flutter_windows.dll",
    "data/icudtl.dat",
    "protocol-helpers/naive.exe",
]

# Juicity is native inside FlClashCore.exe; a bundled client would be a stale
# leftover that shadows it.
FORBIDDEN_SUFFIXES = ["juicity-client.exe"]


def main(path: str) -> int:
    with zipfile.ZipFile(path) as archive:
        names = archive.namelist()
    flat = {name.replace("\\", "/") for name in names}

    missing = [name for name in REQUIRED if name not in flat]
    if missing:
        print(f"ERROR: {path} is missing {', '.join(missing)}")
        for name in sorted(flat)[:60]:
            print(f"  {name}")
        return 1

    forbidden = [
        name for name in sorted(flat)
        if any(name.endswith(suffix) for suffix in FORBIDDEN_SUFFIXES)
    ]
    if forbidden:
        print(f"ERROR: {path} should not carry {', '.join(forbidden)}")
        return 1

    has_assets = any(name.startswith("data/flutter_assets/") for name in flat)
    if not has_assets:
        print(f"ERROR: {path} has no data/flutter_assets payload")
        return 1

    print(f"{path}: {len(flat)} entries, all required files present")
    return 0


if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("usage: check_windows_package.py <zip>")
        raise SystemExit(2)
    raise SystemExit(main(sys.argv[1]))
