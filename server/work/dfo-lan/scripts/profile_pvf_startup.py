"""Measure the read-only PVF preparation path; never start game/storage/client."""
import argparse
import ctypes
from ctypes import wintypes
import datetime
import json
import os
from pathlib import Path
import re
import subprocess
import time


class ProcessMemory(ctypes.Structure):
    _fields_ = [
        ("cb", wintypes.DWORD), ("PageFaultCount", wintypes.DWORD),
        ("PeakWorkingSetSize", ctypes.c_size_t), ("WorkingSetSize", ctypes.c_size_t),
        ("QuotaPeakPagedPoolUsage", ctypes.c_size_t), ("QuotaPagedPoolUsage", ctypes.c_size_t),
        ("QuotaPeakNonPagedPoolUsage", ctypes.c_size_t), ("QuotaNonPagedPoolUsage", ctypes.c_size_t),
        ("PagefileUsage", ctypes.c_size_t), ("PeakPagefileUsage", ctypes.c_size_t),
        ("PrivateUsage", ctypes.c_size_t),
    ]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", default="bin/wireprobe-handoff-source.exe")
    parser.add_argument("--profile", default="configs/pvf-default.json")
    parser.add_argument("--output", help="new directory for logs and measurements")
    parser.add_argument("--timeout", type=float, default=180)
    parser.add_argument("--heap-profile", action="store_true", help="write an isolated post-GC Go heap profile")
    args = parser.parse_args()
    if os.name != "nt":
        parser.error("process memory sampling requires Windows")
    if args.timeout <= 0:
        parser.error("timeout must be positive")
    root = Path(__file__).resolve().parents[1]
    binary = (root / args.binary).resolve()
    profile = json.loads((root / args.profile).read_text(encoding="utf-8-sig"))
    env = {k: v for k, v in os.environ.items() if not k.startswith("DFO_PVF_")}
    env.update(profile["environment"])
    output = (root / args.output).resolve() if args.output else root / ".tmp" / (
        "pvf-perf-" + datetime.datetime.now(datetime.timezone(datetime.timedelta(hours=8))).strftime("%Y%m%d-%H%M%S"))
    output.mkdir(parents=True, exist_ok=False)
    kernel = ctypes.WinDLL("kernel32", use_last_error=True)
    kernel.OpenProcess.argtypes = [wintypes.DWORD, wintypes.BOOL, wintypes.DWORD]
    kernel.OpenProcess.restype = wintypes.HANDLE
    kernel.CloseHandle.argtypes = [wintypes.HANDLE]
    psapi = ctypes.WinDLL("psapi", use_last_error=True)
    psapi.GetProcessMemoryInfo.argtypes = [wintypes.HANDLE, ctypes.POINTER(ProcessMemory), wintypes.DWORD]
    psapi.GetProcessMemoryInfo.restype = wintypes.BOOL
    samples = []
    begin = time.monotonic()
    with (output / "report.json").open("w", encoding="utf-8") as out, (
            output / "prepare.log").open("w", encoding="utf-8") as err:
        command = [
            str(binary), "-pvf-check-catalogs", "-equipment-wear-rules",
            "configs/equipment-wear.current35.json",
        ]
        if args.heap_profile:
            command.extend(["-pvf-check-heap-profile", str(output / "heap.pprof")])
        process = subprocess.Popen(command, cwd=root, env=env, stdout=out, stderr=err, creationflags=subprocess.CREATE_NO_WINDOW)
        handle = kernel.OpenProcess(0x0400 | 0x0010, False, process.pid)
        try:
            if not handle:
                raise ctypes.WinError(ctypes.get_last_error())
            while process.poll() is None:
                now = time.monotonic() - begin
                if now > args.timeout:
                    raise TimeoutError("PVF preparation exceeded timeout")
                counters = ProcessMemory()
                counters.cb = ctypes.sizeof(counters)
                if psapi.GetProcessMemoryInfo(handle, ctypes.byref(counters), counters.cb):
                    samples.append({"seconds": round(now, 3),
                                    "working_mib": round(counters.WorkingSetSize / 2**20, 2),
                                    "private_mib": round(counters.PrivateUsage / 2**20, 2),
                                    "os_peak_working_mib": round(counters.PeakWorkingSetSize / 2**20, 2)})
                time.sleep(0.2)
            process.wait()
        finally:
            if handle:
                kernel.CloseHandle(handle)
            if process.poll() is None:
                process.kill()
                process.wait()
    summary = {"binary": str(binary), "profile": args.profile, "exit_code": process.returncode,
               "seconds": round(time.monotonic() - begin, 3),
               "peak_working_mib": max((v["os_peak_working_mib"] for v in samples), default=0),
               "peak_private_mib": max((v["private_mib"] for v in samples), default=0),
               "output": str(output)}
    log = (output / "prepare.log").read_text(encoding="utf-8")
    match = re.search(r"catalogs prepared in ([0-9.hms]+);", log)
    if match:
        summary["prepare_duration"] = match.group(1)
    if process.returncode == 0:
        report = json.loads((output / "report.json").read_text(encoding="utf-8"))
        if report.get("storage_accessed") is not False or report.get("runtime_started") is not False:
            raise RuntimeError("catalog check did not confirm isolation")
        summary["memory_after_gc"] = report.get("memory")
        summary["domain_count"] = report["domain_count"]
    (output / "samples.json").write_text(json.dumps(samples, indent=2), encoding="utf-8")
    (output / "summary.json").write_text(json.dumps(summary, indent=2), encoding="utf-8")
    print(json.dumps(summary, indent=2))
    return process.returncode


if __name__ == "__main__":
    raise SystemExit(main())
