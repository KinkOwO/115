#!/usr/bin/env python3
"""装备调适 CMD2258 第五轮：面板为什么不刷新。

第四轮已闭环：handler `sub_140B899B0` 里
  - 状态字节 != 0 → `sub_141491360(ui)` → `sub_1414921F0(ui, 3)`
  - 状态字节 == 0 → 按 u16 码弹提示 → 末尾 `sub_1414921F0(ui, 4)`
其中 `ui = sub_14148BF80()` = `sub_14667BB90(qword_14E683C78, 3580, 0)`（窗口 3580 = 调适面板）。

本轮把这条链的**语义**做实：
  - `sub_1414928D0(window)`：state 3/4 共有的动作（怀疑=刷新/重建面板内容）
  - `sub_141494520(window, 0)`：state 4 独有（怀疑=复位等待态）
  - `sub_14667BB90(uiMgr, id, flag)`：窗口取用语义（返回 0 的条件）
  - `sub_14668C520(uiMgr, kind, arg, sub)`：提示/弹窗函数（handler 的 case 分支用它）

产出：analysis/dumps/awakening-2258e/
"""

import json
import os
from datetime import datetime

import ida_funcs
import ida_hexrays
import ida_xref
import idc

TARGETS = [
    (0x1414928D0, "ui_refresh_action"),
    (0x141494520, "ui_state_reset"),
    (0x14667BB90, "ui_window_lookup"),
    (0x14668C520, "ui_popup"),
]

SIZE_LIMIT = 0x40000
OUT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "awakening-2258e")


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
    # 调适窗口 3580 的取用点：谁在别处也取这个窗口（用来判断"刷新"入口有哪些）。
    print("== callers of window getter ==")
    seen = set()
    for fea, site in callers_of(0x14148BF80)[:8]:
        if fea in seen:
            continue
        seen.add(fea)
        r = dump(fea, "getter_caller")
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
