"""Prepare a read-only inner PVF copy using the existing pvf_archive wrapper chain.

Client files are inputs only. Existing outputs and destinations inside the client
directory are refused. The manifest records fingerprints, never key material.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import tempfile

from pvf_archive import aes, stream, wrapper_keys


def fingerprint(path):
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return {"path": str(path), "size": path.stat().st_size, "sha256": digest.hexdigest()}


def check_destination(client, output):
    if output == client or client in output.parents:
        raise ValueError("output must be outside the read-only client directory")
    if output.exists():
        raise FileExistsError(f"refusing existing output: {output}")


def prepare(client, output, manifest_path):
    client, output, manifest_path = (Path(p).resolve() for p in (client, output, manifest_path))
    if output == manifest_path:
        raise ValueError("archive and manifest destinations must differ")
    for path in (output, manifest_path):
        check_destination(client, path)
        if not path.parent.is_dir():
            raise FileNotFoundError(f"output directory does not exist: {path.parent}")
    inputs = [client / name for name in ("DFO.exe", "sk.dat", "Script.pvf")]
    state = [(p.stat().st_size, p.stat().st_mtime_ns) for p in inputs]
    manifest = {"format": "dfo_20260901_inner", "decoder": "pvf_archive.wrapper_keys/aes", "client_exe": fingerprint(inputs[0]), "sk_dat": fingerprint(inputs[1])}
    keys = wrapper_keys(client)
    outer_hash, inner_hash = hashlib.sha256(), hashlib.sha256()
    size = 0
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(dir=output.parent, prefix="pvf-prepare-", suffix=".partial", delete=False) as destination:
            temporary = Path(destination.name)
            with inputs[2].open("rb") as source:
                index = 0
                for block in iter(lambda: source.read(0xA00000), b""):
                    outer_hash.update(block)
                    if index < len(keys):
                        if len(block) < 0x2800:
                            raise ValueError("truncated protected outer PVF segment")
                        block = aes(keys[index], block[:0x2800]) + block[0x2800:]
                    if index == 0 and stream("iNfO", block[:48])[:4] != b"nkpi":
                        raise ValueError("unwrapped header is not the supported inner PVF")
                    destination.write(block)
                    inner_hash.update(block)
                    size += len(block)
                    index += 1
            destination.flush()
            os.fsync(destination.fileno())
        if size == 0:
            raise ValueError("empty client PVF")
        if state != [(p.stat().st_size, p.stat().st_mtime_ns) for p in inputs]:
            raise ValueError("client inputs changed during preparation; retry from a stable source")
        manifest["outer"] = {"path": str(inputs[2]), "size": size, "sha256": outer_hash.hexdigest()}
        manifest["inner"] = {"path": str(output), "size": size, "sha256": inner_hash.hexdigest()}
        # A hard link publishes without replacing a concurrently created file.
        os.link(temporary, output)
        with manifest_path.open("x", encoding="utf-8", newline="\n") as target:
            json.dump(manifest, target, ensure_ascii=False, indent=2)
            target.write("\n")
        return manifest
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--client-dir", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--manifest", required=True)
    args = parser.parse_args()
    manifest = prepare(args.client_dir, args.output, args.manifest)
    print(json.dumps(manifest, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
