#!/usr/bin/env python3
"""Summarize packaged file sizes without extracting or publishing packages."""

from collections import defaultdict
from pathlib import Path, PurePosixPath
import sys
import zipfile


def megabytes(value: int) -> str:
    return f"{value / 1_000_000:.2f}"


def group_name(name: str) -> str:
    normalized = name.replace("\\", "/")
    parts = PurePosixPath(normalized).parts
    lower = normalized.lower()

    if "/assets/data/" in lower or lower.startswith("assets/data/"):
        return "bundled geo data"
    if len(parts) >= 3 and parts[0] == "lib":
        return f"native library: {parts[1]}/{parts[-1]}"
    if lower.startswith("assets/flutter_assets/"):
        return "other Flutter assets"
    if parts:
        return f"{parts[0]}/"
    return "other"


def report_archive(path: Path) -> None:
    with zipfile.ZipFile(path) as archive:
        entries = [item for item in archive.infolist() if not item.is_dir()]

    grouped: dict[str, list[int]] = defaultdict(lambda: [0, 0, 0])
    for item in entries:
        row = grouped[group_name(item.filename)]
        row[0] += item.file_size
        row[1] += item.compress_size
        row[2] += 1

    expanded_total = sum(item.file_size for item in entries)
    compressed_total = sum(item.compress_size for item in entries)
    print(f"### {path.name}")
    print(
        f"Package bytes: {path.stat().st_size}; member bytes: "
        f"{expanded_total} expanded / {compressed_total} compressed; "
        f"{len(entries)} files."
    )
    print("\n| Group | Expanded MB | Compressed MB | Files |")
    print("|---|---:|---:|---:|")
    for group, (expanded, compressed, count) in sorted(
        grouped.items(), key=lambda pair: pair[1][0], reverse=True
    ):
        print(
            f"| `{group}` | {megabytes(expanded)} | "
            f"{megabytes(compressed)} | {count} |"
        )

    print("\nLargest members (expanded size):")
    for item in sorted(entries, key=lambda entry: entry.file_size, reverse=True)[:25]:
        print(
            f"- `{item.filename}` — {item.file_size} bytes expanded, "
            f"{item.compress_size} bytes compressed"
        )
    print()


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: report_package_footprint.py DIST_DIR", file=sys.stderr)
        return 2

    dist = Path(sys.argv[1])
    packages = sorted(
        path
        for path in dist.iterdir()
        if path.is_file() and path.suffix.lower() in {".apk", ".zip", ".exe"}
    )
    if not packages:
        print(f"No package files found in {dist}", file=sys.stderr)
        return 1

    print("## Package footprint audit")
    for package in packages:
        if package.suffix.lower() in {".apk", ".zip"}:
            report_archive(package)
        else:
            print(f"### {package.name}\nPackage bytes: {package.stat().st_size}.\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
