#!/usr/bin/env python3
"""装备调适 CMD2258 第二轮：25 字节 body 的字段来源 + S2C handler 读取路径。

第一轮已闭环：
  - S2C handler 注册点 `sub_14000A060`：`sub_14599D450(qword_14E66C090, 2258, sub_140B899B0, 0)`
  - C2S 发送器 `sub_140B8AD40`：`sub_146D746E0(w, 2258)` → `sub_146D75B10(w, buf, 25)` → 发送
    ⇒ **body 恰 25 字节**（与外部文档一致），字段值由调用者填 `buf`

本轮目标：
  1. 反编译 S2C handler `sub_140B899B0`（它读什么、返回什么形状）
  2. 反编译 25 字节发送器 `sub_140B8AD40` 的**调用者**（谁填 buf ⇒ 字段顺序与来源）
  3. 反编译 `sub_140B915D0` / `sub_140B96DB0`（发包后的 UI 副作用）

产出：`analysis/dumps/awakening-2258b/`
"""

import json
import os
from datetime import datetime

import ida_funcs
import ida_hexrays
import ida_xref
import idc

TARGETS = [
    (0x140B899B0, "cmd2258_s2c_handler"),
    (0x140B8AD40, "cmd2258_c2s_sender"),
    (0x140B915D0, "after_send_ui_a"),
    (0x140B96DB0, "after_send_ui_b"),
]

MAX_CALLERS = 10
CALLER_SIZE_LIMIT = 0x40000

OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "awakening-2258b")


def callers_of(ea):
    out = []
    xr = ida_xref.get_first_cref_to(ea)
    while xr != idc.BADADDR:
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
    if size > CALLER_SIZE_LIMIT:
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
            "callers": [{"func": "0x%X" % c[0], "site": "0x%X" % c[1]} for c in callers_of(fea)][:20]}


def main():
    if not os.path.isdir(OUT_DIR):
        os.makedirs(OUT_DIR)
    if not ida_hexrays.init_hexrays_plugin():
        print("!! hexrays unavailable")
        return

    records = []
    print("== targets ==")
    for ea, tag in TARGETS:
        r = dump(ea, tag)
        if r:
            records.append(r)

    print("== callers of c2s sender ==")
    seen = set()
    for ea, site in callers_of(0x140B8AD40)[:MAX_CALLERS]:
        if ea in seen:
            continue
        seen.add(ea)
        r = dump(ea, "caller")
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
