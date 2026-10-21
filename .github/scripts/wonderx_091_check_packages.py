#!/usr/bin/env python3
"""Check release packages against the v0.9.1 native bridge, not the legacy FFI."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import zipfile


def require(condition, message):
    if not condition:
        raise SystemExit(message)


def windows(archive):
    with zipfile.ZipFile(archive) as package:
        names = {name.replace(chr(92), "/").lower(): name for name in package.namelist()}
        required = {
            "flclash.exe", "flclashcore.exe", "flclashhelperservice.exe",
            "manifest.json", "flutter_windows.dll", "data/icudtl.dat",
            "protocol-helpers/naive.exe",
        }
        require(required <= names.keys(), f"Windows ZIP missing: {sorted(required - names.keys())}")
        for name in required:
            require(package.getinfo(names[name]).file_size > 0, f"Empty Windows file: {name}")
        require(any(name.startswith("data/flutter_assets/") for name in names), "Missing Flutter assets")
        require(not any(name.endswith("juicity-client.exe") for name in names), "Juicity must remain native")
        core_hash = hashlib.sha256(package.read(names["flclashcore.exe"])).hexdigest()
        manifest = json.loads(package.read(names["manifest.json"]))
        require(manifest.get("coreSha256") == core_hash, "Windows core manifest hash mismatch")
        return {"zip": archive.name, "core_manifest_verified": True, "naive_helper_present": True}


def android(apk):
    abi = "arm64-v8a"
    with zipfile.ZipFile(apk) as package:
        names = set(package.namelist())
        required = {f"lib/{abi}/{name}" for name in ("libclash.so", "libnaive.so", "libflutter.so", "libapp.so")}
        require(required <= names, f"Android APK missing: {sorted(required - names)}")
        require(not any(name.endswith("/libgojni.so") for name in names), "Unrenamed gomobile library")
        require(any(name.startswith(f"lib/{abi}/") and "rust_api" in name and name.endswith(".so") for name in names),
                "Missing Rust native-assets library")
        found_abis = {name.split("/")[1] for name in names if name.startswith("lib/") and name.endswith(".so")}
        require(found_abis == {abi}, f"Unexpected package ABIs: {sorted(found_abis)}")
        for name in required:
            data = package.read(name)
            require(len(data) > 64 and data[:4] == b"\x7fELF", f"Invalid ELF: {name}")
            require(data[4] == 2 and int.from_bytes(data[18:20], "little") == 183, f"Not AArch64 ELF: {name}")
            require(package.getinfo(name).compress_type == zipfile.ZIP_DEFLATED, f"Expected legacy extractable library: {name}")
        with tempfile.TemporaryDirectory() as temp:
            core = Path(temp) / "libclash.so"
            core.write_bytes(package.read(f"lib/{abi}/libclash.so"))
            symbols = subprocess.check_output(["readelf", "--dyn-syms", "--wide", str(core)], text=True)
            require("Java_com_follow_clash_core_mobile_Mobile" in symbols, "Missing gomobile Mobile JNI exports")
            require("Java_go_Seq" in symbols, "Missing gomobile Go Seq JNI exports")
    jars = list(Path("build/core/gomobile").glob("classes.jar"))
    require(len(jars) == 1, "Missing generated gomobile classes.jar")
    with zipfile.ZipFile(jars[0]) as jar:
        require("com/follow/clash/core/mobile/Mobile.class" in jar.namelist(), "Missing Mobile binding")
        mobile = jar.read("com/follow/clash/core/mobile/Mobile.class")
        for method in (b"invokeMethod", b"startTun", b"stopTun"):
            require(method in mobile, f"Missing Mobile binding method: {method.decode()}")
        seq = jar.read("go/Seq.class")
        require(b"\x01\x00\x05clash" in seq and b"\x01\x00\x05gojni" not in seq, "Seq must load libclash")
    build_tools = sorted((Path(os.environ["ANDROID_HOME"]) / "build-tools").glob("*/aapt"))
    require(build_tools, "Android aapt unavailable")
    badging = subprocess.check_output([str(build_tools[-1]), "dump", "badging", str(apk)], text=True)
    require("name='com.follow.clash.dev'" in badging, "Expected explicitly non-overwrite test application id")
    manifest = subprocess.check_output(
        [str(build_tools[-1]), "dump", "xmltree", str(apk), "AndroidManifest.xml"], text=True)
    require(re.search(r"extractNativeLibs[^\n]*(?:0xffffffff|true)", manifest), "Manifest must extract native helper")
    signers = sorted((Path(os.environ["ANDROID_HOME"]) / "build-tools").glob("*/apksigner"))
    require(signers, "Android apksigner unavailable")
    signature = subprocess.check_output([str(signers[-1]), "verify", "--verbose", "--print-certs", str(apk)], text=True)
    Path("dist/APK-SIGNATURES.txt").write_text(signature)
    return {"apk": apk.name, "abi": abi, "gomobile_bridge_verified": True,
            "naive_helper_present": True, "native_extraction_verified": True,
            "signing_mode": "debug-key-test-package-not-original-signature",
            "device_start_and_connection": "PENDING_DEVICE_TEST"}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("platform", choices=("windows", "android"))
    args = parser.parse_args()
    dist = Path("dist")
    packages = sorted(dist.glob("**/*.zip" if args.platform == "windows" else "**/*.apk"))
    require(len(packages) == 1, f"Expected exactly one {args.platform} package, got {len(packages)}")
    result = windows(packages[0]) if args.platform == "windows" else android(packages[0])
    (dist / "PACKAGE-CHECKS.json").write_text(json.dumps(result, indent=2) + "\n")
    print(json.dumps(result))


if __name__ == "__main__":
    main()
