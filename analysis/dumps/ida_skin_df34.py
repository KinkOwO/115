#!/usr/bin/env python3
"""df34: which int does the client store as a skin's family class, and where does it come from?

Closed before this round (df22-df33, all in analysis/dumps/skin-noti):
  * registry accessor sub_1444EBAB0(id) = sub_140283D60(qword_14E683BF8, id, 1);
    record+8 = family class, record+12 = subtype (df23_sub_1444EBAB0.c).
  * every panel grid reader filters rec+8 == the page it snapshots
    (df22_ui_1441E4180 -> page 1 / ==1, 1441E3510 -> page 2 / ==2, 1441E3990 -> 3,
     1444E36F0 -> 9, 1441E47B0 -> 4).
  * NOTI1546 page maps live at mgr+136+16*page (df14_page_lookup_1444ebd90.c), so
    select category N reads owned page N for N in 0,1,3,4,7,8 (6 is the second
    damage-font partition and also reads page 2).
  * CMD1565 echo case 1 keeps only ids whose record has rec+8==1 && rec+12==3
    (df23_sub_1444EE820.c).
Open question this round: the PVF [type]/[sub type] labels are plain strings
('party frame', 'skill cutscene', 'second awakening cutscene', ...) and none of
them occur in DFO.exe, yet the registry stores small ints. Find the builder that
inserts records into qword_14E683BF8 and show exactly how +8 / +12 are produced.
"""
import ida_bytes, ida_hexrays, ida_name, idautils, idc

REG = 0x14E683BF8
OUT = r"D:\115us\analysis\dumps\skin-noti"


def fn(ea):
    f = idc.get_func_attr(ea, idc.FUNCATTR_START)
    return f if f != idc.BADADDR else 0


def dump(tag, text):
    import re
    name = "df34_%s.c" % re.sub(r"\W+", "_", tag)
    with open("%s\\%s" % (OUT, name), "w", encoding="utf-8") as h:
        h.write("// %s\n\n%s\n" % (tag, text))
    return name


def body_of(ea):
    try:
        return str(ida_hexrays.decompile(ea))
    except Exception:
        return ""


print("=== 1. references to the static skin registry qword_14E683BF8 ===")
sites = []
for xr in idautils.XrefsTo(REG, 0):
    tgt = xr.to
    frm = xr.frm
    mnem = idc.print_insn_mnem(frm)
    line = idc.GetDisasm(frm)
    f = fn(frm)
    sites.append((frm, f, mnem, line))
    print("ref frm=%s fn=%s(%s) mnem=%s line=%s" % (hex(frm), hex(f),
          ida_name.get_name(f) if f else "-", mnem, line))
print("total refs=%d distinct functions=%d" % (len(sites), len(set(s[1] for s in sites if s[1]))))

print("=== 2. what each referencing function calls (builder candidates) ===")
cands = []
for f in sorted(set(s[1] for s in sites if s[1])):
    calls = []
    for h in idautils.FuncItems(f):
        if idc.print_insn_mnem(h) == "call":
            t = idc.get_operand_value(h, 0)
            calls.append(t)
    uniq = sorted(set(calls))
    flags = []
    if any(c == 0x140283D60 for c in uniq):
        flags.append("lookup140283D60")
    print("fn=%s(%s) size=%d calls=%d %s" % (hex(f), ida_name.get_name(f),
          idc.get_func_attr(f, idc.FUNCATTR_END) - f, len(uniq), " ".join(flags)))
    if not any(c == 0x140283D60 for c in uniq) or len(uniq) > 1:
        cands.append(f)
print("builder candidates=%d" % len(cands))

print("=== 3. decompile candidates that write +8/+12 ===")
import re
written = 0
for f in cands[:15]:
    b = body_of(f)
    if not b:
        continue
    dump("cand_0x%x" % f, b)
    hits = re.findall(r"\+\s*(?:8|12)\)\s*=\s*([^;]{1,48});", b)
    if not hits:
        continue
    print("fn=%s writes+8/+12 hits=%d" % (hex(f), len(hits)))
    for h in hits[:12]:
        print("   store:", h.strip())
    written += 1
print("done, candidates scanned=%d writers=%d" % (len(cands[:15]), written))
