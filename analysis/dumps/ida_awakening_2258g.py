#!/usr/bin/env python3
"""装备调适 CMD2258 第七轮：定位客户端**规则解析与选档**函数。

第六轮用 xorstr 表里的 VA 反查失败（那些是字段名注册表），改用本轮新找到的常量
（`analysis/dumps/xorstr_addr_to_text.json`，宽松搜索 "materials"/"awakening"）：

  0x14B2CF9C8  "[need materials]"                ← 调适/融合的成本段
  0x14B2CF9F8  "[refund materials]"              ← 返还段
  0x14B2CFA30  "[need amalgamation materials]"
  0x1491ADB90  "equipment awakening option"      ← 装备 .equ 里的字段名
  0x149483D30  "Etc/115LvAbility/EquipmentAwakeningOption.lst"
  0x149483C30  "EquipmentAwakeningOptionSystem::init"
  0x149483BD0  "EquipmentAwakeningOptionSystem Failed!"

做法：
  * 收集每个常量的引用函数；
  * **取交集**（同时引用多个调适常量的函数 = 调适系统的入口，而非通用解析器）；
  * 反编译交集函数与前若干引用者。

产出：analysis/dumps/awakening-2258g/
"""

import json
import os
from datetime import datetime

import ida_funcs
import ida_hexrays
import idautils
import idc

TARGETS = [
    (0x14B2CF9C8, "tag_need_materials"),
    (0x14B2CF9F8, "tag_refund_materials"),
    (0x14B2CFA30, "tag_need_amalgamation"),
    (0x1491ADB90, "field_equipment_awakening_option"),
    (0x149483D30, "path_equipmentawakeningoption_lst"),
    (0x149483C30, "log_system_init"),
]

FUNC_SIZE_LIMIT = 0x30000
MAX_PER_TARGET = 10
OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "awakening-2258g")


def dump(fea, tag):
    f = ida_funcs.get_func(fea)
    if not f:
        return None
    size = f.end_ea - f.start_ea
    name = ida_funcs.get_func_name(fea)
    path = os.path.join(OUT_DIR, "%s_%X_%s.c" % (tag, fea, name))
    if os.path.exists(path):
        return {"func": "0x%X" % fea, "name": name, "size": size, "file": os.path.basename(path), "cached": True}
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

    by_target = {}
    func_names = {}
    for va, tag in TARGETS:
        funcs = []
        for xref in idautils.XrefsTo(va, 0):
            f = ida_funcs.get_func(xref.frm)
            if not f:
                continue
            if f.start_ea not in funcs:
                funcs.append(f.start_ea)
        by_target[tag] = funcs
        for ea in funcs:
            func_names[ea] = ida_funcs.get_func_name(ea)
        print("== 0x%X %s: %d function(s)" % (va, tag, len(funcs)))
        for ea in funcs[:MAX_PER_TARGET]:
            print("   0x%X %s" % (ea, func_names.get(ea, "?")))

    # 交集：同时引用 ≥2 个调适常量的函数
    from collections import Counter
    counter = Counter()
    for funcs in by_target.values():
        for ea in funcs:
            counter[ea] += 1
    shared = [ea for ea, n in counter.items() if n >= 2]
    shared.sort(key=lambda ea: -counter[ea])
    print("== shared functions (%d) ==" % len(shared))
    for ea in shared:
        print("   0x%X %s  targets=%d" % (ea, func_names.get(ea, "?"), counter[ea]))

    records = []
    for ea in shared[:MAX_PER_TARGET]:
        rec = dump(ea, "shared")
        if rec:
            rec["shared_targets"] = counter[ea]
            records.append(rec)
    for tag, funcs in by_target.items():
        for ea in funcs[:3]:
            rec = dump(ea, tag)
            if rec:
                records.append(rec)

    jp = os.path.join(OUT_DIR, "index.json")
    with open(jp, "w", encoding="utf-8") as h:
        json.dump({
            "generated": datetime.now().isoformat(timespec="seconds"),
            "by_target": {k: ["0x%X" % e for e in v] for k, v in by_target.items()},
            "names": {"0x%X" % e: n for e, n in func_names.items()},
            "shared": ["0x%X" % e for e in shared],
            "decompiled": records,
        }, h, indent=1, ensure_ascii=False)
    print("wrote %s" % jp)


if __name__ == "__main__":
    main()
