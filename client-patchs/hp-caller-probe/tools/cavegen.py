#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
cavegen.py (v2) -- assemble src/probe-hook.s with ml64 and prove, byte by byte,
that the resulting machine code is the trampoline the plugin believes it is.

What it emits
-------------
  src/probe-hook-image.h   the byte image the plugin copies into its code cave,
                           the offsets the plugin patches/relocates, and the
                           frame map the C probe reads the evidence from.

What it refuses to do
---------------------
It writes nothing unless every one of these holds.  Each check exists because a
previous version got it wrong:

  * image[0x00..0x04] == E9 00 00 00 00   and image[0x16] is the FIRST byte of
    the trampoline.  v1's layout landed the trampoline at +0x13 because ml64
    sorts fragments inside .text$mn; the offsets were only ever validated as
    "somewhere in the dump".
  * image[0x08..0x15] == FF 25 00000000 0000000000000000 (the absolute stub).
  * every xmm access is MOVUPS.  Not one MOVAPS/VMOVAPS may appear: MOVAPS
    faults on an 8 (mod 16) address, which is exactly the state at the cave
    entry point.  That is the v1 crash.
  * the GPR save/restore pairs are exact mirrors off the same frame base.
  * no instruction writes rax except the restore of the saved rax and the
    push of the return address.
  * `sub rsp, N` and the frame map agree, N % 16 == 0, and the read of the
    caller's return address uses the frame map's asm_frame field.
  * the pool slots are still all zero and inside the image.

Usage:
    python cavegen.py            assemble + verify + write the header
    python cavegen.py --check    assemble + verify only (writes nothing)
"""

import argparse
import os
import re
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)          # client-patchs/hp-caller-probe
SRC = os.path.join(ROOT, "src")
DIST = os.path.join(ROOT, "dist")
ASM = os.path.join(SRC, "probe-hook.s")
VCVARS = r"C:\Program Files\Microsoft Visual Studio\2022\Enterprise\VC\Auxiliary\Build\vcvars64.bat"
GO = r"C:\Game\dof\115us\tools\go\bin\go.exe"
GOENV = {
    "GOPROXY": "off",
    "GOPATH": r"C:\Game\dof\115us\tools\gopath",
    "GOCACHE": r"C:\Game\dof\115us\tools\gocache",
    "GOTOOLCHAIN": "local",
    "GOFLAGS": "-mod=mod",
}

OA_SITEJMP = 0
OA_STUB = 8
OA_STUB_TARGET = 14
OA_CODE = 22
OA_POOL = 0x200
OA_TOTAL = 0x400

IMG_SITEJMP_BYTES = bytes([0xE9, 0, 0, 0, 0])
IMG_STUB_BYTES = bytes([0xFF, 0x25, 0, 0, 0, 0])
IMG_STUB_TARGET_BYTES = bytes(8)

DISASM_GO = r'''
package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

	"golang.org/x/arch/x86/x86asm"
)

func main() {
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	base, _ := strconv.ParseUint(os.Args[2], 0, 64)
	for i := 0; i < len(b); {
		inst, err := x86asm.Decode(b[i:], 64)
		if err != nil {
			fmt.Printf("%#x  db %02x\n", base+uint64(i), b[i])
			i++
			continue
		}
		fmt.Printf("%#x  %-44s ; %s\n", base+uint64(i), inst.String(),
			hex.EncodeToString(b[i:i+inst.Len]))
		i += inst.Len
	}
}
'''


def fail(problems, headline="VERIFY FAILED"):
    print("=== %s ===" % headline)
    for p in problems:
        print("  - " + p)
    sys.exit(2)


def run(cmd, cwd=None):
    p = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True)
    return p.returncode, p.stdout, p.stderr


def parse_equates():
    """Read `NAME equ VALUE` from the .s file: the numbers are a contract and
    the assertions below use these values, not duplicated literals."""
    out = {}
    with open(ASM, "r", encoding="ascii") as f:
        for line in f:
            m = re.match(r"^\s*([A-Za-z_][A-Za-z0-9_]*)\s+equ\s+(\S+)\s*(?:;.*)?$", line)
            if m:
                name, val = m.group(1), m.group(2)
                try:
                    out[name] = int(val, 0)
                except ValueError:
                    pass
    return out


def assemble():
    os.makedirs(DIST, exist_ok=True)
    obj = os.path.join(DIST, "probe-hook.obj")
    if os.path.exists(obj):
        os.remove(obj)
    bat = os.path.join(DIST, "_cavegen_asm.bat")
    with open(bat, "w", newline="\r\n") as f:
        f.write("@echo off\r\n")
        f.write('call "%s" >nul\r\n' % VCVARS)
        f.write('ml64 /nologo /c /Fo"%s" "%s"\r\n' % (obj, ASM))
    rc, out, err = run(["cmd", "/c", bat])
    if rc != 0 or not os.path.exists(obj):
        print("ml64 failed rc=%d\n%s\n%s" % (rc, out, err))
        sys.exit(1)
    return obj


def coff_sections(data):
    if data[:2] != b"\x64\x86":
        print("unexpected COFF machine 0x%04x" % int.from_bytes(data[:2], "little"))
        sys.exit(1)
    nsec = int.from_bytes(data[2:4], "little")
    optsize = int.from_bytes(data[16:18], "little")
    base = 20 + optsize
    secs = []
    for i in range(nsec):
        h = data[base + i * 40: base + (i + 1) * 40]
        secs.append({
            "name": h[:8].rstrip(b"\x00").decode("ascii", "replace"),
            "vsize": int.from_bytes(h[8:12], "little"),
            "rawsize": int.from_bytes(h[16:20], "little"),
            "rawptr": int.from_bytes(h[20:24], "little"),
            "relptr": int.from_bytes(h[24:28], "little"),
            "nrel": int.from_bytes(h[32:34], "little"),
            "chars": int.from_bytes(h[36:40], "little"),
        })
    return secs


def coff_symbols(data):
    symptr = int.from_bytes(data[8:12], "little")
    nsym = int.from_bytes(data[12:16], "little")
    strtab = symptr + nsym * 18
    syms = []
    for i in range(nsym):
        o = symptr + i * 18
        raw = data[o:o + 8]
        if raw[:4] == b"\x00\x00\x00\x00":
            so = int.from_bytes(raw[4:8], "little")
            end = data.index(b"\x00", strtab + so)
            name = data[strtab + so:end].decode("ascii", "replace")
        else:
            name = raw.rstrip(b"\x00").decode("ascii", "replace")
        syms.append({
            "name": name,
            "value": int.from_bytes(data[o + 8:o + 12], "little"),
            "sec": int.from_bytes(data[o + 12:o + 14], "little", signed=True),
        })
    return syms


def text_section(data):
    """Return (bytes, section_name, relocations) of the executable section."""
    secs = coff_sections(data)
    syms = coff_symbols(data)
    for s in secs:
        if s["name"].startswith(".text") and (s["chars"] & 0x20):
            body = data[s["rawptr"]: s["rawptr"] + s["rawsize"]]
            rels = []
            for r in range(s["nrel"]):
                o = s["relptr"] + r * 10
                symidx = int.from_bytes(data[o + 4:o + 8], "little")
                rels.append({
                    "va": int.from_bytes(data[o:o + 4], "little"),
                    "sym": syms[symidx]["name"] if symidx < len(syms) else "?",
                    "type": int.from_bytes(data[o + 8:o + 10], "little"),
                })
            return body, s["name"], rels
    print("no executable section in the object")
    sys.exit(1)


def disassemble(blob):
    tmp = tempfile.mkdtemp(prefix="cavegen-")
    with open(os.path.join(tmp, "go.mod"), "w") as f:
        f.write("module cavedis\n\ngo 1.21\n\nrequire golang.org/x/arch v0.8.0\n")
    with open(os.path.join(tmp, "main.go"), "w") as f:
        f.write(DISASM_GO)
    binp = os.path.join(tmp, "cavedis.exe")
    env = dict(os.environ)
    env.update(GOENV)
    p = subprocess.run([GO, "build", "-o", binp, "."], cwd=tmp, env=env,
                       capture_output=True, text=True)
    if p.returncode != 0:
        print("go build failed:\n%s\n%s" % (p.stdout, p.stderr))
        sys.exit(1)
    blobpath = os.path.join(tmp, "blob.bin")
    with open(blobpath, "wb") as f:
        f.write(blob)
    p = subprocess.run([binp, blobpath, "0x0"], capture_output=True, text=True)
    if p.returncode != 0:
        print("disasm failed: %s" % p.stderr)
        sys.exit(1)
    return p.stdout


def parse_ins(tout):
    """[(offset, mnemonic_lower, raw_hex, operand_text_upper)]"""
    ins = []
    for line in tout.strip().splitlines():
        if ";" not in line:
            continue
        head, raw = line.rsplit(";", 1)
        raw = raw.strip()
        parts = head.split(None, 2)
        if len(parts) < 2 or not parts[0].startswith("0x"):
            continue
        if not re.fullmatch(r"[0-9a-f]+", raw):
            continue
        try:
            off = int(parts[0], 16)
        except ValueError:
            continue
        operands = parts[2].strip().upper() if len(parts) > 2 else ""
        ins.append((off, parts[1].strip().lower(), raw, operands))
    return ins


def disp_of(operand_text):
    """Pull the (possibly huge, negative) displacement out of `[RBP-0x10]`
    style operand text.  x86asm prints negative displacements as unsigned
    64-bit values, and small ones as +0x.. / -0x..."""
    m = re.search(r"RBP\s*([+-])\s*(0x[0-9a-f]+|\d+)", operand_text)
    if m:
        v = int(m.group(2), 16) if m.group(2).startswith("0x") else int(m.group(2))
        return -v if m.group(1) == "-" else v
    m = re.search(r"RBP\+0x([0-9a-f]{8,})", operand_text)
    if m:
        return int(m.group(1), 16) - (1 << 64)
    if "RBP" in operand_text and "[" in operand_text:
        return 0
    return None


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--check", action="store_true",
                    help="assemble + verify only, write no header")
    args = ap.parse_args()

    eq = parse_equates()
    required = ["OA_CODE", "OA_STUB", "OA_STUB_TARGET", "OA_POOL", "OA_TOTAL",
                "FRM_SIZE", "FRM_FLAGS", "FRM_RAX", "FRM_RCX", "FRM_RDX",
                "FRM_RBX", "FRM_RSI", "FRM_RDI", "FRM_R8", "FRM_R9",
                "FRM_R10", "FRM_R11", "FRM_R12", "FRM_R13", "FRM_R14",
                "FRM_R15", "FRM_XMM0", "FRM_XMM_STEP", "FRM_XMM15",
                "FRM_LIMIT", "OA_ASM_FRAME"]
    missing = [r for r in required if r not in eq]
    if missing:
        fail(["src/probe-hook.s is missing equates: %s" % ", ".join(missing)])

    obj = assemble()
    data = open(obj, "rb").read()
    text, secname, rels = text_section(data)
    if not text:
        fail(["the assembled section is empty"])
    print("assembled section %s: %d bytes; relocations: %s"
          % (secname, len(text), rels if rels else "none"))

    tout = disassemble(text)
    ins = parse_ins(tout)
    if not ins:
        fail(["could not decode a single instruction"])

    problems = []

    # ---------------------------------------------------------------- bytes
    if text[:5] != IMG_SITEJMP_BYTES:
        problems.append("cave[0x00..0x04] must be E9 00000000, got %s"
                        % text[:5].hex())
    if text[OA_STUB:OA_STUB + 6] != IMG_STUB_BYTES:
        problems.append("cave[0x%02X..0x%02X] must be FF 25 00000000, got %s"
                        % (OA_STUB, OA_STUB + 5, text[OA_STUB:OA_STUB + 6].hex()))
    if text[OA_STUB_TARGET:OA_STUB_TARGET + 8] != IMG_STUB_TARGET_BYTES:
        problems.append("cave[0x%02X..0x%02X] (stub imm64) must still be zero, got %s"
                        % (OA_STUB_TARGET, OA_STUB_TARGET + 7,
                           text[OA_STUB_TARGET:OA_STUB_TARGET + 8].hex()))
    if text[OA_STUB + 6:OA_CODE] != bytes(OA_CODE - OA_STUB - 6):
        problems.append("cave[0x%02X..0x%02X] (filler before the trampoline) must be zero, got %s"
                        % (OA_STUB + 6, OA_CODE - 1, text[OA_STUB + 6:OA_CODE].hex()))

    # the trampoline must actually START at OA_CODE
    first_code = next((i for i in ins if i[0] >= OA_CODE), None)
    if first_code is None:
        problems.append("no instruction at or after OA_CODE")
    elif first_code[0] != OA_CODE:
        problems.append("the first instruction of the trampoline is at +0x%X, "
                        "but OA_CODE is +0x%X -- ml64 fragment ordering moved it"
                        % (first_code[0], OA_CODE))
    else:
        print("trampoline starts at +0x%X: %s" % (OA_CODE, first_code[1]))

    # pool must be all zero and inside the image
    if len(text) < OA_POOL + 32:
        problems.append("the image is only %d bytes; the pool at +0x%X is truncated"
                        % (len(text), OA_POOL))
    elif text[OA_POOL:OA_POOL + 32] != bytes(32):
        problems.append("the pool at +0x%X is not zero: %s"
                        % (OA_POOL, text[OA_POOL:OA_POOL + 32].hex()))
    if len(text) > OA_TOTAL:
        problems.append("the image is %d bytes, which exceeds OA_TOTAL 0x%X"
                        % (len(text), OA_TOTAL))

    # ------------------------------------------------------- no MOVAPS anywhere
    bad_align = [(o, m, t) for (o, m, _r, t) in ins
                 if m in ("movaps", "movapd", "vmovaps", "vmovapd")]
    if bad_align:
        problems.append("alignment-faulting moves present (MOVAPS on an 8(mod16) "
                        "rsp is the v1 crash): %s" % bad_align)

    # ------------------------------------------------------------ movups pairs
    saves = [(o, disp_of(t)) for (o, m, _r, t) in ins if m == "movups" and t.startswith("[RBP")]
    loads = [(o, disp_of(t)) for (o, m, _r, t) in ins
             if m == "movups" and re.match(r"^X\d+,", t)]
    if len(saves) != 16:
        problems.append("expected 16 `movups [rbp-..], xmmN` saves, got %d" % len(saves))
    if len(loads) != 16:
        problems.append("expected 16 `movups xmmN, [rbp-..]` restores, got %d" % len(loads))
    if len(saves) == 16 and len(loads) == 16:
        if [d for _o, d in saves] != [d for _o, d in loads]:
            problems.append("xmm save/restore displacements are not mirrors:\n"
                            "    save: %s\n    load: %s"
                            % ([hex(-d) for _o, d in saves],
                               [hex(-d) for _o, d in loads]))
        want_xmm = [-eq["FRM_XMM0"] - eq["FRM_XMM_STEP"] * i for i in range(16)]
        if [d for _o, d in saves] != want_xmm:
            problems.append("xmm slot map is %s, want %s"
                            % ([hex(-d) for _o, d in saves], [hex(-d) for d in want_xmm]))
        # same for the restores: must be in the same order (mirror off one base)
        if [d for _o, d in loads] != want_xmm:
            problems.append("xmm restore slot map is %s, want %s"
                            % ([hex(-d) for _o, d in loads], [hex(-d) for d in want_xmm]))

    # --------------------------------------------------------- GPR save/restore
    gpr_slots = {
        "RAX": eq["FRM_RAX"], "RCX": eq["FRM_RCX"], "RDX": eq["FRM_RDX"],
        "RBX": eq["FRM_RBX"], "RSI": eq["FRM_RSI"], "RDI": eq["FRM_RDI"],
        "R8": eq["FRM_R8"], "R9": eq["FRM_R9"], "R10": eq["FRM_R10"],
        "R11": eq["FRM_R11"], "R12": eq["FRM_R12"], "R13": eq["FRM_R13"],
        "R14": eq["FRM_R14"], "R15": eq["FRM_R15"],
    }
    stored = {}
    restored = {}
    for (o, m, raw, t) in ins:
        mm = re.match(r"^MOV \[RBP(?:-0X([0-9A-F]+))?\], ([A-Z0-9]+)$", t)
        if mm:
            off = int(mm.group(1), 16) if mm.group(1) else 0
            stored[mm.group(2)] = -off
            continue
        mm = re.match(r"^MOV ([A-Z0-9]+), \[RBP(?:-0X([0-9A-F]+))?\]$", t)
        if mm:
            off = int(mm.group(1), 16) if mm.group(2) else 0
            restored[mm.group(1)] = -off
    for reg, slot in gpr_slots.items():
        if stored.get(reg) != -slot:
            problems.append("register %s is saved to %s, frame says -0x%X"
                            % (reg, stored.get(reg), slot))
        if restored.get(reg) != -slot:
            problems.append("register %s is restored from %s, frame says -0x%X"
                            % (reg, restored.get(reg), slot))

    # ---------------------------------------------------------------- rsp/ret
    sub_m = [i for i in ins if i[1] == "sub" and i[3].startswith("RSP")]
    if not sub_m:
        problems.append("no `sub rsp, N`")
    else:
        m = re.match(r"SUB RSP, (0X[0-9A-F]+|\d+)$", sub_m[0][3])
        n = int(m.group(1), 16) if m and m.group(1).startswith("0X") else (int(m.group(1)) if m else -1)
        if n != eq["FRM_SIZE"]:
            problems.append("`sub rsp, 0x%X` does not match FRM_SIZE 0x%X"
                            % (n, eq["FRM_SIZE"]))
        if n % 16 != 0:
            problems.append("FRM_SIZE 0x%X is not a multiple of 16" % n)
        if n < eq["FRM_LIMIT"]:
            problems.append("FRM_SIZE 0x%X is smaller than FRM_LIMIT 0x%X: the save "
                            "area overlaps the caller's stack" % (n, eq["FRM_LIMIT"]))
    if not any(i[1] == "mov" and i[3] == "RSP, RBP" for i in ins):
        problems.append("no `mov rsp, rbp` (a 32-bit restore would be required if "
                        "the frame is larger than 2 GiB, but it also hides bugs)")
    if not any(i[1] == "popfq" for i in ins):
        problems.append("no `popfq` (the flags would not be restored)")
    if not any(i[1] == "pushfq" for i in ins):
        problems.append("no `pushfq`")
    if not any(i[1] == "ret" for i in ins):
        problems.append("no `ret`: the cave would never return")
    if [i[1] for i in ins].count("call") != 1:
        problems.append("expected exactly one `call` (probe_hp_entry), got %d"
                        % [i[1] for i in ins].count("call"))

    # the caller's return address must be read through the frame map, i.e. the
    # `push qword ptr [rbp-1]` idiom, not a hand-counted [rsp+..] offset
    push_mem = [i for i in ins if i[1] == "push" and "RBP" in i[3]]
    want_disp = -1 - eq["OA_ASM_FRAME"]
    if len(push_mem) != 1:
        problems.append("expected exactly one `push [rbp-0x%X]` (the caller return "
                        "address), got %s" % (-want_disp, push_mem))
    elif disp_of(push_mem[0][3]) != want_disp:
        problems.append("the return-address push uses displacement 0x%X, want 0x%X"
                        % (disp_of(push_mem[0][3]), want_disp))

    # the frame base must be published to the C probe exactly once
    pub = [i for i in ins if i[3] == "[RBP], RBP"]
    if len(pub) != 1:
        problems.append("expected exactly one `mov [rbp], rbp` (asm_frame publish), "
                        "got %d" % len(pub))
    lea_rcx = [i for i in ins if i[1] == "lea" and i[3].startswith("RCX")]
    if len(lea_rcx) != 1:
        problems.append("expected exactly one `lea rcx, [rbp..]` (frame base passed "
                        "to probe_hp_entry), got %d" % len(lea_rcx))
    elif disp_of(lea_rcx[0][3]) != -eq["OA_ASM_FRAME"]:
        problems.append("the frame base handed to the C probe is at %s, want [rbp-0x%X]"
                        % (lea_rcx[0][3], eq["OA_ASM_FRAME"]))

    # rax may only be written by the restore and read by the save
    rax_writes = []
    for (o, m, raw, t) in ins:
        if m == "mov" and (t.startswith("RAX,") or t == "RAX, [RBP-0X10]"):
            if t == "RAX, [RBP-0X10]":
                continue          # the restore
            rax_writes.append((hex(o), t))
        if m in ("xor", "add", "sub", "or", "and", "lea", "pop", "movzx",
                 "movsxd", "imul", "shl", "shr") and t.startswith("RAX,"):
            rax_writes.append((hex(o), t))
    if rax_writes:
        problems.append("something writes rax besides the restore: %s" % rax_writes)

    # ------------------------------------------ position independence + call
    # The trampoline must be POSITION INDEPENDENT: the plugin copies these bytes
    # to a VirtualAlloc'ed page, so any displacement relative to the instruction
    # pointer that the PE loader does not resolve would be wrong at the copy
    # destination.  v1 solved that by storing addresses in data slots and
    # relocating them by hand; every one of those hand-maintained fields was a
    # chance to get an offset wrong.  Here there are none: the single direct
    # `call probe_hp_entry` is resolved by the loader when the DLL is mapped.
    rip_any = [(o, m, t) for (o, m, _r, t) in ins if "RIP" in t]
    if rip_any:
        problems.append("RIP-relative operand(s) found -- the image would not be "
                        "position independent: %s" % rip_any)
    extern_rels = [r for r in rels if r["type"] == 0x0004]
    if len(extern_rels) != 1:
        problems.append("expected exactly one REL32 relocation (the probe_hp_entry "
                        "call), got %s" % extern_rels)
    elif extern_rels[0]["sym"] != "probe_hp_entry":
        problems.append("the REL32 relocation targets %s, expected probe_hp_entry"
                        % extern_rels[0]["sym"])
    if extern_rels and not (OA_CODE <= extern_rels[0]["va"] < OA_POOL):
        problems.append("the probe_hp_entry call at +0x%X is outside the trampoline"
                        % extern_rels[0]["va"])
    if rels and set(r["type"] for r in rels) - {0x0004}:
        problems.append("unexpected relocation types: %s" % rels)

    if problems:
        print()
        print(tout)
        fail(problems)

    print("=== VERIFY OK ===")
    print("  image            %d bytes (cave arena %d)" % (len(text), OA_TOTAL))
    print("  trampoline at    +0x%X" % OA_CODE)
    print("  pool at          +0x%X..+0x%X (all zero)" % (OA_POOL, OA_POOL + 31))
    print("  frame            rsp-0x%X via rbp; flags +0x%X, rax +0x%X, xmm0..15 "
          "+0x%X..+0x%X"
          % (eq["FRM_SIZE"], eq["FRM_FLAGS"], eq["FRM_RAX"], eq["FRM_XMM0"],
             eq["FRM_XMM15"]))
    print("  alignment        every xmm access is MOVUPS (0 MOVAPS)")
    print("  position indep.  no RIP-relative operands; 1 loader-resolved REL32")
    print("  call             `call probe_hp_entry` at +0x%X (resolved by the loader)"
          % extern_rels[0]["va"])

    if args.check:
        return

    # ------------------------------------------------------------------ header
    hdr = os.path.join(SRC, "probe-hook-image.h")
    with open(hdr, "w", newline="\n") as f:
        f.write("/* AUTO-GENERATED by tools/cavegen.py from src/probe-hook.s -- do not edit. */\n")
        f.write("#ifndef PROBE_HOOK_IMAGE_H\n#define PROBE_HOOK_IMAGE_H\n\n")
        f.write("#define PROBE_CAVE_IMAGE_LEN %d\n" % len(text))
        f.write("#define PROBE_TOTAL_SIZE     %d\n" % OA_TOTAL)
        f.write("#define PROBE_OFF_SITEJMP    %d\n" % OA_SITEJMP)
        f.write("#define PROBE_OFF_STUB       %d\n" % OA_STUB)
        f.write("#define PROBE_OFF_STUB_TARGET %d\n" % OA_STUB_TARGET)
        f.write("#define PROBE_OFF_CODE       %d\n" % OA_CODE)
        f.write("#define PROBE_OFF_POOL       %d\n" % OA_POOL)
        f.write("#define PROBE_OFF_RING_SLOT  %d\n" % OA_POOL)
        f.write("#define PROBE_OFF_TABLE_SLOT %d\n" % (OA_POOL + 8))
        f.write("#define PROBE_OFF_REL32      %d\n" % extern_rels[0]["va"])
        f.write("\n/* the frame the trampoline hands to probe_hp_entry() (rbp-relative) */\n")
        for name in ("FRM_SIZE", "FRM_FLAGS", "FRM_RAX", "FRM_RCX", "FRM_RDX",
                     "FRM_RBX", "FRM_RSI", "FRM_RDI", "FRM_R8", "FRM_R9",
                     "FRM_R10", "FRM_R11", "FRM_R12", "FRM_R13", "FRM_R14",
                     "FRM_R15", "FRM_XMM0", "FRM_XMM_STEP", "FRM_XMM15",
                     "FRM_LIMIT", "OA_ASM_FRAME"):
            f.write("#define PROBE_%s %d\n" % (name, eq[name]))
        f.write("\nstatic const unsigned char PROBE_CAVE_IMAGE[%d] = {\n" % len(text))
        for i in range(0, len(text), 12):
            f.write("    " + " ".join("0x%02X," % b for b in text[i:i + 12]) + "\n")
        f.write("};\n\n")
        f.write("#define PROBE_CAVEHEAD_BYTES {0xE9, 0x00, 0x00, 0x00, 0x00}\n")
        f.write("#define PROBE_CAVEHEAD_LEN 5\n")
        f.write("\n#endif\n")
    print("wrote %s (%d bytes of code)" % (hdr, len(text)))


if __name__ == "__main__":
    main()
