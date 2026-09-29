#!/usr/bin/env python3
import argparse
import os
import re
import shutil
import subprocess
import sys
import tempfile
import zipfile
from pathlib import Path


REQUIRED_LIBS = (
    "libcore.so",
    "libclash.so",
    "libnaive.so",
)

REQUIRED_BRIDGE_SYMBOL_MIN_SIZES = {
    "JNI_OnLoad": 8,
    "Java_com_follow_clash_core_Core_invokeMethod": 8,
    "Java_com_follow_clash_core_Core_quickSetup": 8,
    "Java_com_follow_clash_core_Core_setEventListener": 8,
    "Java_com_follow_clash_core_Core_startTun": 8,
}

REQUIRED_LIBCLASH_SYMBOL_MIN_SIZES = {
    "invokeMethod": 8,
    "quickSetup": 8,
    "setEventListener": 8,
    "startTUN": 8,
}

MIN_LIBCLASH_SIZE = 1_000_000


def fail(message):
    print(f"ERROR: {message}", file=sys.stderr)
    return 1


def find_readelf():
    for name in ("llvm-readelf", "llvm-readelf.exe"):
        found = shutil.which(name)
        if found:
            return found

    ndk_roots = [
        os.environ.get("ANDROID_NDK_HOME"),
        os.environ.get("ANDROID_NDK_ROOT"),
    ]
    sdk_root = os.environ.get("ANDROID_HOME") or os.environ.get("ANDROID_SDK_ROOT")
    if sdk_root:
        ndk_roots.extend(str(path) for path in Path(sdk_root, "ndk").glob("*"))

    for root in filter(None, ndk_roots):
        candidates = Path(root).glob("toolchains/llvm/prebuilt/*/bin/llvm-readelf*")
        for candidate in candidates:
            if candidate.is_file():
                return str(candidate)

    return None


def collect_apks(path):
    target = Path(path)
    if target.is_file():
        if target.suffix.lower() != ".apk":
            raise ValueError(f"not an APK file: {target}")
        return [target]
    if target.is_dir():
        return sorted(target.rglob("*.apk"))
    raise ValueError(f"path does not exist: {target}")


def run_readelf(readelf, args, so_path):
    result = subprocess.run(
        [readelf, *args, str(so_path)],
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        encoding="utf-8",
        errors="replace",
    )
    if result.returncode != 0:
        raise RuntimeError(result.stdout)
    return result.stdout


def parse_symbol_sizes(symbol_output):
    sizes = {}
    pattern = re.compile(r"^\s*\d+:\s+[0-9a-fA-F]+\s+(\d+)\s+\S+\s+\S+\s+\S+\s+\S+\s+(.+?)\s*$")
    for line in symbol_output.splitlines():
        match = pattern.match(line)
        if not match:
            continue
        size = int(match.group(1))
        name = match.group(2).split("@", 1)[0]
        sizes[name] = max(size, sizes.get(name, 0))
    return sizes


def parse_needed_libraries(dynamic_output):
    return re.findall(r"Shared library: \[(.+?)\]", dynamic_output)


def verify_apk(apk_path, abi, readelf):
    print(f"Checking {apk_path}")
    lib_prefix = f"lib/{abi}/"
    with zipfile.ZipFile(apk_path) as apk:
        names = set(apk.namelist())
        missing = [name for name in REQUIRED_LIBS if f"{lib_prefix}{name}" not in names]
        if missing:
            return fail(f"{apk_path} is missing native libs for {abi}: {', '.join(missing)}")

        with tempfile.TemporaryDirectory() as temp_dir:
            core_member = f"{lib_prefix}libcore.so"
            core_path = Path(temp_dir, "libcore.so")
            with apk.open(core_member) as source, core_path.open("wb") as target:
                shutil.copyfileobj(source, target)

            libclash_member = f"{lib_prefix}libclash.so"
            libclash_path = Path(temp_dir, "libclash.so")
            with apk.open(libclash_member) as source, libclash_path.open("wb") as target:
                shutil.copyfileobj(source, target)

            dynamic_output = run_readelf(readelf, ["-d"], core_path)
            needed_libraries = parse_needed_libraries(dynamic_output)
            print(f"libcore.so size: {core_path.stat().st_size} bytes")
            print(
                "libcore.so NEEDED: "
                + (", ".join(needed_libraries) if needed_libraries else "none")
            )
            if "libclash.so" not in needed_libraries:
                absolute_libclash = [
                    name for name in needed_libraries
                    if name.replace("\\", "/").endswith("/libclash.so")
                ]
                if absolute_libclash:
                    return fail(
                        f"{apk_path} libcore.so depends on absolute libclash path: "
                        + ", ".join(absolute_libclash)
                    )
                return fail(f"{apk_path} libcore.so does not depend on libclash.so")

            symbol_output = run_readelf(readelf, ["-Ws"], core_path)
            sizes = parse_symbol_sizes(symbol_output)
            bad_symbols = []
            for symbol, min_size in REQUIRED_BRIDGE_SYMBOL_MIN_SIZES.items():
                size = sizes.get(symbol)
                if size is None:
                    bad_symbols.append(f"{symbol}=missing")
                elif size < min_size:
                    bad_symbols.append(f"{symbol}={size}")
            if bad_symbols:
                return fail(
                    f"{apk_path} libcore.so bridge is incomplete: "
                    + ", ".join(bad_symbols)
                )

            libclash_size = libclash_path.stat().st_size
            print(f"libclash.so size: {libclash_size} bytes")
            if libclash_size < MIN_LIBCLASH_SIZE:
                return fail(
                    f"{apk_path} libclash.so is unexpectedly small: "
                    f"{libclash_size} bytes"
                )

            libclash_symbol_output = run_readelf(readelf, ["-Ws"], libclash_path)
            libclash_sizes = parse_symbol_sizes(libclash_symbol_output)
            bad_libclash_symbols = []
            for symbol, min_size in REQUIRED_LIBCLASH_SYMBOL_MIN_SIZES.items():
                size = libclash_sizes.get(symbol)
                if size is None:
                    bad_libclash_symbols.append(f"{symbol}=missing")
                elif size < min_size:
                    bad_libclash_symbols.append(f"{symbol}={size}")
            if bad_libclash_symbols:
                return fail(
                    f"{apk_path} libclash.so is missing Core entry points: "
                    + ", ".join(bad_libclash_symbols)
                )

    print(f"OK: {apk_path} JNI bridge links a complete libclash.so")
    return 0


def main():
    parser = argparse.ArgumentParser(
        description="Verify Android APK native core bridge links libclash.so."
    )
    parser.add_argument("apk_or_dir", help="APK file or directory containing APK artifacts")
    parser.add_argument("--abi", default="arm64-v8a", help="Android ABI to verify")
    args = parser.parse_args()

    try:
        apks = collect_apks(args.apk_or_dir)
    except ValueError as error:
        return fail(str(error))

    if not apks:
        return fail(f"no APK files found under {args.apk_or_dir}")

    readelf = find_readelf()
    if not readelf:
        return fail("llvm-readelf was not found in PATH, ANDROID_NDK_HOME, or ANDROID_HOME/ndk")

    print(f"Using readelf: {readelf}")
    failed = 0
    for apk in apks:
        failed |= verify_apk(apk, args.abi, readelf)
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
