"""Ensure the inner PVF archive matches the current client triple (four-state gate).

Why this exists
---------------
PVF direct mode makes the server read the *decrypted inner archive*
(`server/work/client-build/Script.inner.pvf`), not the client's encrypted
`Script.pvf`. That inner file:

  * is a local build artifact (`.gitignore` ignores it), so it is NOT shipped;
  * has no automatic producer upstream -- `prepare_inner_pvf.py` is a manual CLI
    with zero callers.

So a fresh player machine fails `open inner PVF` in the default profile. This
module closes that gap for the *server side*, so players who launch through
`启动服务端.cmd` / `launch_local.py` (without the launcher's update checkpoint)
also self-heal.

Why the gate key is the client TRIPLE, not just `Script.pvf`
-----------------------------------------------------------
`pvf_archive.wrapper_keys()` reads the RSA private key and the AES key from
`DFO.exe`, and uses `sk.dat` as the ciphertext. Changing `DFO.exe` alone (even
with `Script.pvf` untouched) yields a completely different inner archive.
Hence the fingerprint is sha256(DFO.exe) + sha256(sk.dat) + sha256(Script.pvf).

Four states
-----------
    inner | manifest | triple matches | action
    ------+----------+----------------+--------
    no    | -        | -              | build
    yes   | no       | -              | rebuild (cannot prove provenance)
    yes   | yes      | yes            | reuse
    yes   | yes      | no             | rebuild (client changed)

Kept in sync with the launcher's `internal/pvfprep` package; the manifest format
is the one `prepare_inner_pvf.py` already writes (`dfo_20260901_inner`), plus a
`cache` field this module adds for the cheap (size, mtime_ns) fast path.
"""

import hashlib
import json
import os
import time
from pathlib import Path

FORMAT = "dfo_20260901_inner"
CLIENT_INPUTS = ("DFO.exe", "sk.dat", "Script.pvf")


def fingerprint(path):
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return {"path": str(path), "size": path.stat().st_size, "sha256": digest.hexdigest()}


def stat_fingerprint(path):
    """Cheap fingerprint: size only (no content read)."""
    return {"path": str(path), "size": path.stat().st_size}


def client_states(client):
    """(size, mtime_ns) for each input -- the fast path's comparison key."""
    states = []
    for name in CLIENT_INPUTS:
        info = (client / name).stat()
        states.append({"name": name, "size": info.st_size, "mtime_ns": info.st_mtime_ns})
    return states


def load_manifest(path):
    if not path.is_file():
        return None
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return None
    if not isinstance(data, dict) or data.get("format") != FORMAT:
        return None
    return data


def decide(client, inner, manifest):
    """Return (needs_build, reason). Pure read-only inspection."""
    if not inner.is_file():
        return True, "内层 PVF 不存在"
    if manifest is None:
        return True, "缺少或无法解析清单，旧件不可信"
    # `load_manifest` already rejects a foreign format, but check again here so
    # `decide` stays correct when called with a manifest dict from elsewhere --
    # same defensive ordering as the launcher's internal/pvfprep.
    if manifest.get("format") != FORMAT:
        return True, "清单格式不符（%r）" % manifest.get("format")

    # The inner archive itself must match the manifest, regardless of the client.
    # Checked before the client comparison so a truncated inner is never accepted.
    recorded = manifest.get("inner") or {}
    got = inner.stat().st_size
    if recorded.get("size") not in (None, got):
        return True, f"内层 PVF 大小不符（盘上 {got}，清单 {recorded['size']}）"

    cached = manifest.get("cache")
    if isinstance(cached, list) and len(cached) == len(CLIENT_INPUTS):
        try:
            if client_states(client) == cached:
                return False, "客户端与内层 PVF 均未变化，复用现有产物"
        except OSError:
            return True, "无法读取客户端文件状态，按需重建"

    # Slow path: the client triple changed (or the fast path was unavailable),
    # so compare full fingerprints.
    exe = fingerprint(client / "DFO.exe")["sha256"]
    sk = fingerprint(client / "sk.dat")["sha256"]
    script = fingerprint(client / "Script.pvf")["sha256"]
    if (
        exe == (manifest.get("client_exe") or {}).get("sha256")
        and sk == (manifest.get("sk_dat") or {}).get("sha256")
        and script == (manifest.get("outer") or {}).get("sha256")
    ):
        return False, "客户端指纹与清单一致，复用现有产物"
    return True, "客户端 DFO.exe/sk.dat/Script.pvf 已变化"


def rotate_stale(paths):
    """Rename existing artifacts aside; never delete (keep a rollback path).

    `prepare_inner_pvf.py` refuses an existing output and publishes via os.link
    (atomic, but only when the target is absent). We therefore must not add a
    `--force` to it -- rotation is the caller's job.
    """
    stamp = time.strftime("%Y%m%d-%H%M%S")
    rotated = None
    for path in paths:
        if not path.is_file():
            continue
        target = Path(str(path) + ".stale-" + stamp)
        index = 1
        while target.exists():
            target = Path(f"{path}.stale-{stamp}-{index}")
            index += 1
        try:
            os.replace(path, target)
        except OSError:
            continue
        if rotated is None:
            rotated = target
    return rotated


def ensure(client, inner, manifest_path, prepare, log=print):
    """Four-state gate. `prepare(client, inner, manifest_path)` performs the build.

    Returns {"generated": bool, "reason": str, "elapsed": float}.
    """
    client = Path(client).resolve()
    inner = Path(inner).resolve()
    manifest_path = Path(manifest_path).resolve()

    missing = [name for name in CLIENT_INPUTS if not (client / name).is_file()]
    if missing:
        raise RuntimeError(
            "客户端缺少 %s，无法生成内层 PVF（需 DFO.exe + sk.dat + Script.pvf 三件套）"
            % "、".join(missing)
        )

    manifest = load_manifest(manifest_path)
    needs_build, reason = decide(client, inner, manifest)
    if not needs_build:
        return {"generated": False, "reason": reason, "elapsed": 0.0}

    rotated = rotate_stale([inner, manifest_path])
    if rotated is not None:
        log(f"内层 PVF 需要重建（{reason}）；旧件已备份为 {rotated.name}")

    log("正在从客户端 Script.pvf 生成内层 PVF（约 760 MB，请稍候）…")
    start = time.time()
    produced = prepare(str(client), str(inner), str(manifest_path))
    elapsed = time.time() - start

    # Cross-check the product against the manifest the prepare step wrote.
    if isinstance(produced, dict):
        expected = (produced.get("inner") or {}).get("sha256")
        if expected:
            actual = fingerprint(inner)["sha256"]
            if actual != expected:
                raise RuntimeError(
                    "生成的内层 PVF 哈希不符：期望 %s，实际 %s"
                    % (expected[:12], actual[:12])
                )
        produced["cache"] = client_states(client)
        manifest_path.write_text(
            json.dumps(produced, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
        )

    return {"generated": True, "reason": reason, "elapsed": elapsed}
