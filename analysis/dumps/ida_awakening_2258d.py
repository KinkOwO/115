#!/usr/bin/env python3
"""装备调适 CMD2258 第四轮：ACK 结果状态机。

第三轮已闭环：`sub_140B899B0`（S2C handler）里
  - 状态字节 != 0 → `sub_141491360(ui)` → `sub_1414921F0(ui, 3)`
  - 状态字节 == 0 → 按 u16 码弹提示 → 末尾 `sub_1414921F0(ui, 4)`
本轮反编译 `sub_1414921F0` 与 `sub_14148BF80`，确认 3/4 各自的语义
（关闭等待界面 / 播放成功或失败表现），从而定下服务端 ACK 的字节。

产出：analysis/dumps/awakening-2258d/
"""

import json
import os
from datetime import datetime

import ida_funcs
import ida_hexrays
import ida_xref
import idc

TARGETS = [
    (0x1414921F0, "ui_result_state"),
    (0x14148BF80, "ui_singleton_getter"),
    (0x141491360, "fail_path_wrapper"),
]

SIZE_LIMIT = 0x40000
OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "awakening-2258d")


def callers_of(ea, limit=10):
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
    jp = os.path.join(OUT_DIR, "index.json")
    with open(jp, "w", encoding="utf-8") as h:
        json.dump({"generated": datetime.now().isoformat(timespec="seconds"), "records": records},
                  h, indent=1, ensure_ascii=False)
    print("wrote %s" % jp)


if __name__ == "__main__":
    main()
