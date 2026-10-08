#!/usr/bin/env python3
"""Patch the 115 client so CMD80 upgrade ACK accepts a new level up to 31.

What it changes
---------------
Inside sub_14529B2F0 (the single CMD80 reply handler):

  0x14529bb33  lea eax, [rdx-1]      ; rdx = reply byte [13] = NEW level
  0x14529bb36  cmp eax, 0xc  -> ja   ; new > 15 -> fallback branch
  0x14529bb71  lea eax, [rdx-1]
  0x14529bb74  cmp al, 0xa   -> ja   ; new-1 > 10 -> report ADD_HACKTYPE_CNT (253)

So a new level of 16 takes the fallback and then 16-1=15 > 10 -> the client
reports "unauthorized upgrade" and locks the amplify window. That is exactly
the observed "only +16 gets stuck".

The patch widens the single immediate 0x0a -> 0x1e, i.e. "new-1 <= 30",
which equals "new <= 31" -- matching the official cap (90US uses 30 -> +31).

Levels <= 15 never reach this branch (they go through the jump table), so this
patch cannot affect anything that already works.

Byte: file offset 0x529bb75, 0x0a -> 0x1e.

Usage
-----
  python patch_client_amplify.py           # apply (dry-run first if unsure)
  python patch_client_amplify.py --dry-run # verify only, write nothing
  python patch_client_amplify.py --revert  # restore 0x0a

Close DFO.exe before running -- Windows locks a running executable.
"""

import argparse
import hashlib
import pathlib
import shutil
import sys

CLIENT = pathlib.Path(r"E:/DNF/DNF-115US/DNF/DFO.exe")
BACKUP = pathlib.Path(r"E:/DNF/DNF-115US/.workbuddy/backup-client-20261007/DFO.exe")

OFFSET = 0x529BB75
ORIG = 0x0A
PATCHED = 0x1E

ORIG_MD5 = "c48cdd75afaa2ce5f9d4e20be22eecb2"


def md5(path: pathlib.Path) -> str:
    h = hashlib.md5()
    with path.open("rb") as fh:
        for chunk in iter(lambda: fh.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--dry-run", action="store_true")
    ap.add_argument("--revert", action="store_true")
    args = ap.parse_args()

    want_from, want_to = (PATCHED, ORIG) if args.revert else (ORIG, PATCHED)

    if not CLIENT.exists():
        print("FAIL client not found: %s" % CLIENT)
        return 1

    print("client: %s" % CLIENT)
    print("md5   : %s" % md5(CLIENT))
    if md5(CLIENT) != ORIG_MD5 and not args.revert:
        print("WARN  file md5 differs from the one analysed (%s)." % ORIG_MD5)
        print("      If this is already patched, md5 will differ -- continuing because")
        print("      the byte check below is the real guard.")

    # Reading works even while the client runs; only writing needs it closed.
    with CLIENT.open("rb") as fh:
        fh.seek(OFFSET)
        cur = fh.read(1)
        if not cur:
            print("FAIL  cannot read offset %#x" % OFFSET)
            return 1
        cur = cur[0]

        # Context: expect `3c <imm> 77`  ==  cmp al, imm ; ja rel8
        fh.seek(OFFSET - 1)
        ctx = fh.read(3)

    print("offset %#x: ctx=%s  cur=%#x" % (OFFSET, ctx.hex(" "), cur))
    if ctx[0] != 0x3C or ctx[2] != 0x77:
        print("FAIL  unexpected context (expected 3c <imm> 77). Aborting.")
        return 1

    if cur == want_to:
        print("SKIP  byte already %#x (nothing to do)." % want_to)
        return 0
    if cur != want_from:
        print("FAIL  expected %#x at offset, found %#x. Aborting." % (want_from, cur))
        return 1

    if args.dry_run:
        print("DRY-RUN would write %#x -> %#x at offset %#x" % (cur, want_to, OFFSET))
        return 0

    if not BACKUP.exists():
        print("backing up to %s ..." % BACKUP)
        BACKUP.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(CLIENT, BACKUP)

    with CLIENT.open("r+b") as fh:
        fh.seek(OFFSET)
        fh.write(bytes([want_to]))

    with CLIENT.open("rb") as fh:
        fh.seek(OFFSET - 1)
        after = fh.read(3)
    print("WROTE %#x -> %#x ; ctx now %s" % (cur, want_to, after.hex(" ")))
    print("md5   : %s" % md5(CLIENT))
    print("OK    %s" % ("reverted" if args.revert else "patched"))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
