"""Read-only evidence for the local Ispins settlement crash; never edits EXE/IDB."""
import hashlib
import json
from pathlib import Path

import capstone
import pefile

ROOT = Path(__file__).resolve().parents[2]
MANIFEST = ROOT / "server/work/client-build/Script.inner.manifest.json"
manifest = json.loads(MANIFEST.read_text(encoding="utf-8"))
exe = Path(manifest["client_exe"]["path"])
pe = pefile.PE(str(exe), fast_load=True)
base = pe.OPTIONAL_HEADER.ImageBase
md = capstone.Cs(capstone.CS_ARCH_X86, capstone.CS_MODE_64)


def disassemble(va, size):
    data = pe.get_data(va - base, size)
    return [
        f"{ins.address:#x} {ins.mnemonic} {ins.op_str}"
        for ins in md.disasm(data, va)
    ]


result = {"exe": str(exe), "base": hex(base), "resources": {}, "readers": {}}
for name in ("client_exe", "sk_dat", "outer", "inner"):
    entry = manifest[name]
    path = Path(entry["path"])
    with path.open("rb") as stream:
        digest = hashlib.file_digest(stream, "sha256").hexdigest()
    result["resources"][name] = {
        "path": str(path), "sha256": digest,
        "matches_decode_manifest": digest == entry["sha256"],
    }
for name, va, size in (
    ("noti2252", 0x1424FDC30, 0x11D),
    ("noti2253", 0x1424FDB60, 0xC1),
    ("fixed_copy_reader", 0x146EA0BE0, 0x67),
    ("local_userinfo_equipment", 0x145639840, 0xDC),
    ("local_2047_reset_reader", 0x142530950, 0x2DB),
    ("local_2047_sender", 0x1425311F0, 0xAA),
    ("local_party_header", 0x1452F27A0, 0x88),
    ("local_party_action3_skips_details", 0x1452F2F05, 0x15),
    ("local_party_action3_clears_members", 0x1452F40FA, 0x3A),
    ("local_ispins_roster_gate", 0x1424E66A0, 0x137),
    ("local_ispins_prerequisite_sets_flag", 0x1424E71E0, 0xDA),
    ("local_timeout1474_registration", 0x1452BB715, 0x17),
    ("local_timeout1474_two_u32_reader", 0x1452AE370, 0xC6),
    ("gold_return_crash_actor_collection", 0x145C34D30, 0x149),
    ("native_cached_map_mode0_branch", 0x145B235B0, 0x50),
):
    result["readers"][name] = disassemble(va, size)
assert pe.get_data(0x1424FDCF7 - base, 5).hex() == "ba5c1e0000"
assert pe.get_data(0x1424FDBCB - base, 5).hex() == "ba65090000"
assert pe.get_data(0x146EA0C30 - base, 11).hex() == "c704250000000000000000"
assert pe.get_data(0x142530A1E - base, 3).hex() == "83fa04"
result["verified"] = {
    "2252_direct_copy_bytes": 7772,
    "2253_direct_copy_bytes": 2405,
    "short_read_deliberate_null_write": "0x146ea0c30",
}
print(json.dumps(result, ensure_ascii=False, indent=2))
