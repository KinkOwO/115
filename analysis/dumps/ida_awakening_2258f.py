#!/usr/bin/env python3
"""装备调适 CMD2258 第六轮：客户端如何选 `[condition]` 档位。

要回答的两个玩家现象（服务端当前实现可能不一致）：
  1. 「调适 8 次后无法继续」—— 阶/档的取值域与推进规则到底是什么；
  2. 「金币扣了但光既灵魂没扣」—— 客户端按哪个 `[condition] 115 <rarity> <n>` 块算成本。

线索（`analysis/dumps/xorstr_addr_to_text.json`，明文与 VA）：
  0x149684E80  "equipmentAwakening"                  ← 装备 .equ 里的字段名（`[equipment awakening option]`）
  0x149684168  "[condition]"                         ← 规则表里筛选用的标签
  0x149483C30  "EquipmentAwakeningOptionSystem::init"
  0x149483C90  "D:\\Work\\Jenkins5\\...\\EquipmentAwakeningOptionSystem.cpp"
  0x149483D30  "Etc/115LvAbility/EquipmentAwakeningOption.lst"
  0x149483BD0  "EquipmentAwakeningOptionSystem Failed!"

做法：对这些**文件内地址**找数据引用（lea rcx, [rip+x]），再取所属函数并反编译。
产出：analysis/dumps/awakening-2258f/
"""

import json
import os
from datetime import datetime

import ida_funcs
import ida_hexrays
import idautils
import idc

TARGETS = [
    (0x149684E80, "field_equipmentAwakening"),
    (0x149684168, "tag_condition"),
    (0x149483C30, "log_EquipmentAwakeningOptionSystem_init"),
    (0x149483C90, "src_EquipmentAwakeningOptionSystem_cpp"),
    (0x149483D30, "path_EquipmentAwakeningOption_lst"),
    (0x149483BD0, "log_EquipmentAwakeningOptionSystem_Failed"),
]

FUNC_SIZE_LIMIT = 0x30000
MAX_FUNCS_PER_TARGET = 8
OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "awakening-2258f")


def dump(fea, tag):
    f = ida_funcs.get_func(fea)
    if not f:
        return None
    size = f.end_ea - f.start_ea
    name = ida_funcs.get_func_name(fea)
    path = os.path.join(OUT_DIR, "%s_%X_%s.c" % (tag, fea, name))
    if size > FUNC_SIZE_LIMIT:
        body = "// skipped: %d bytes > limit\n" % size
    else:
        try:
            cf = ida_hexrays.decompile(fea)
            body = str(cf) if cf else "// decompile returned None\n"
        except Exception as e:  # noqa: BLE001
            body = "// decompile failed: %s\n" % e
    with open(path, "w", encoding="utf-8") as h:
        h.write("// %s  va=0x%X  size=%d\n\n" % (name, fea, size))
        h.write(body)
    print("   wrote %s (%d bytes)" % (path, size))
    return {"func": "0x%X" % fea, "name": name, "size": size, "file": os.path.basename(path)}


def main():
    if not os.path.isdir(OUT_DIR):
        os.makedirs(OUT_DIR)
    if not ida_hexrays.init_hexrays_plugin():
        print("!! hexrays unavailable")
        return

    report = []
    for va, tag in TARGETS:
        print("== 0x%X %s ==" % (va, tag))
        seen = set()
        entries = []
        for xref in idautils.XrefsTo(va, 0):
            frm = xref.frm
            f = ida_funcs.get_func(frm)
            if not f or f.start_ea in seen:
                continue
            seen.add(f.start_ea)
            if len(seen) > MAX_FUNCS_PER_TARGET:
                print("   …more refs skipped")
                break
            print("   ref from 0x%X (%s)" % (frm, idc.GetDisasm(frm)[:60]))
            rec = dump(f.start_ea, tag)
            if rec:
                rec["ref_site"] = "0x%X" % frm
                entries.append(rec)
        report.append({"va": "0x%X" % va, "tag": tag, "functions": entries})

    jp = os.path.join(OUT_DIR, "index.json")
    with open(jp, "w", encoding="utf-8") as h:
        json.dump({"generated": datetime.now().isoformat(timespec="seconds"), "targets": report},
                  h, indent=1, ensure_ascii=False)
    print("wrote %s" % jp)


if __name__ == "__main__":
    main()
