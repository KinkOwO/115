#!/usr/bin/env python3
"""装备调适 CMD2258 第三轮：ACK 语义与 UI 侧字段。

第二轮已闭环 25 字节布局（sub_141491D10 / sub_141491BE0 两个发包点）：
  +13 u8      模式（0 调适 / 1 初始化返还）
  +14 u32     材料组（D10 来自 a1+996）
  +18 u8      空间（来自 a1+56）
  +19 u16     槽位（来自 a1+60）
  +21 u32     目标模板（来自对象虚调用 +24）

本轮目标（把"成功/失败回包怎么写"坐实）：
  - sub_141491360  ACK 状态非 0 时的失败路径
  - sub_146ADFC80  ACK 成功时的提示（码 → 消息 id）
  - sub_1480A6620  目标模板 id 的生成
  - sub_140B8D280  UI 对象（1360B）构造：空间/槽位/材料组字段来源

产出：analysis/dumps/awakening-2258c/
"""

import json
import os
from datetime import datetime

import ida_funcs
import ida_hexrays
import ida_xref
import idc

TARGETS = [
    (0x141491360, "ack_status_fail_path"),
    (0x146ADFC80, "ack_success_message"),
    (0x1480A6620, "target_template_gen"),
    (0x140B8D280, "ui_object_ctor"),
]

SIZE_LIMIT = 0x40000
OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "awakening-2258c")


def callers_of(ea, limit=12):
    out = []
    xr = ida_xref.get_first_cref_to(ea)
    while xr != idc.BADADDR and len(out) < limit:
        f = ida_funcs.get_func(xr)
        if f:
            out.append((f.start_ea, xr))
        xr = ida_xref.get_next_cref_to(ea, xr)
    return out


def dump(fea, tag):
    f = ida_funcs.get_func(fea)
    if not f:
        print("   no func at 0x%X" % fea)
        return None
    size = f.end_ea - f.start_ea
    name = ida_funcs.get_func_name(fea)
    path = os.path.join(OUT_DIR, "%s_%X_%s.c" % (tag, fea, name))
    if size > SIZE_LIMIT:
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
    return {"va": "0x%X" % fea, "name": name, "size": size, "file": os.path.basename(path),
            "callers": [{"func": "0x%X" % c[0], "site": "0x%X" % c[1]} for c in callers_of(fea)]}


def main():
    if not os.path.isdir(OUT_DIR):
        os.makedirs(OUT_DIR)
    if not ida_hexrays.init_hexrays_plugin():
        print("!! hexrays unavailable")
        return

    records = []
    for ea, tag in TARGETS:
        r = dump(ea, tag)
        if r:
            records.append(r)

    print("== callers of ui object ctor ==")
    seen = set()
    for fea, site in callers_of(0x140B8D280)[:8]:
        if fea in seen:
            continue
        seen.add(fea)
        r = dump(fea, "uictor_caller")
        if r:
            r["call_site"] = "0x%X" % site
            records.append(r)

    jp = os.path.join(OUT_DIR, "index.json")
    with open(jp, "w", encoding="utf-8") as h:
        json.dump({"generated": datetime.now().isoformat(timespec="seconds"), "records": records},
                  h, indent=1, ensure_ascii=False)
    print("wrote %s" % jp)


if __name__ == "__main__":
    main()
