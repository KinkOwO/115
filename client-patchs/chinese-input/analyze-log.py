#!/usr/bin/env python3
"""读 chinese-input-probe.log，输出一份可直接下结论的摘要。

用法：
    python analyze-log.py <chinese-input-probe.log>

它回答四个问题（对应四种修法）：
  Q1 客户端窗口到底收不收 WM_IME_* / WM_CHAR？          → 收/不收
  Q2 客户端有没有把窗口的输入法上下文关掉（NULL）？      → 有/没有
  Q3 操作系统侧在组字、客户端却毫无反应？                → 组字串与消息的时序
  Q4 客户端有没有自己去读输入法（ImmGetContext/LockIMC）？→ 有/没有，调用者是不是 DFO.exe
"""

import collections
import re
import sys

IMM_RE = re.compile(r"^\S+\s+(Imm\w+|CreateWindowEx\w|DestroyWindow|GetProcAddress)\(")
MSG_RE = re.compile(r"^(WM_\w+)\s+hwnd=(\S+)\[([^\]]*)\]")
BACKTRACE_RE = re.compile(r"\^(\w+)\s+#\d+\s+(\S+)")
COMP_RE = re.compile(r"输入法状态变化：hwnd=(\S+) 开启=(-?\d+) 组字串=\"(.*)\"")
FOCUS_RE = re.compile(r"前台：hwnd=(\S+)\[([^\]]*)\] pid=(\d+) tid=(\d+) 焦点=(\S+) hkl=(\S+) IsIME=(-?\d+)")
HIMC_RE = re.compile(r"该窗口 himc=(\S+) 输入法开启=(-?\d+) 组字串=\"(.*)\"")


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        return 2
    lines = open(sys.argv[1], encoding="utf-8", errors="replace").read().splitlines()

    api = collections.Counter()
    api_callers = collections.defaultdict(collections.Counter)
    msgs = collections.Counter()
    msg_hwnds = collections.Counter()
    msg_classes = collections.Counter()
    nonascii_chars = []
    associate_null = []
    associate_any = []
    comp_changes = []
    focus_lines = []
    himc_lines = []
    last_api = None

    for ln in lines:
        ln = ln.rstrip()
        if not ln or ln.lstrip().startswith("^"):
            m = BACKTRACE_RE.search(ln)
            if m and last_api:
                api_callers[last_api][m.group(2)] += 1
            continue
        m = COMP_RE.search(ln)
        if m:
            comp_changes.append((m.group(1), int(m.group(2)), m.group(3)))
            continue
        m = FOCUS_RE.search(ln)
        if m:
            focus_lines.append(m.groups())
            continue
        m = HIMC_RE.search(ln)
        if m:
            himc_lines.append(m.groups())
            continue
        m = MSG_RE.match(ln)
        if m:
            msgs[m.group(1)] += 1
            msg_hwnds[m.group(2)] += 1
            msg_classes[m.group(3)] += 1
            cm = re.search(r"char=U\+([0-9A-F]+)\((\w+)\)", ln)
            if cm and cm.group(2) != "ascii":
                nonascii_chars.append(cm.group(1))
            continue
        m = IMM_RE.match(ln)
        if m:
            name = m.group(1)
            api[name] += 1
            last_api = name
            if name == "ImmAssociateContext":
                associate_any.append(ln)
                if "himc=0000000000000000" in ln or "传入 NULL" in ln:
                    associate_null.append(ln)
            continue
        if "窗口[" in ln:
            continue
        last_api = None

    print("=== 1. 客户端窗口消息 ===")
    if not msgs:
        print("  一条 WM_/按键消息都没记到 → 客户端文本输入不走窗口消息（多半是 DirectInput 轮询）")
    for k, v in msgs.most_common():
        print("  %-28s %d" % (k, v))
    if msg_classes:
        print("  涉及窗口类：", ", ".join("%s×%d" % (c, n) for c, n in msg_classes.most_common(8)))
    ime_msgs = sum(v for k, v in msgs.items() if k.startswith("WM_IME_"))
    print("  WM_IME_ 系列合计：%d" % ime_msgs)
    print("  非 ASCII 字符：%s" % (", ".join("U+%s" % c for c in nonascii_chars[:20]) or "无"))

    print()
    print("=== 2. 输入法上下文关联（ImmAssociateContext）===")
    print("  调用次数：%d；其中传 NULL（关掉输入法）的：%d" % (len(associate_any), len(associate_null)))
    for ln in associate_null[:10]:
        print("   ", ln)
    if not associate_any:
        print("  客户端从未调用 ImmAssociateContext")

    print()
    print("=== 3. 探针自己观察到的组字状态 ===")
    if himc_lines:
        for h, o, s in himc_lines[:10]:
            print("  前台窗口 himc=%s 开启=%s 组字串=\"%s\"" % (h, o, s))
    for h, o, s in comp_changes[:30]:
        print("  变化 hwnd=%s 开启=%d 组字串=\"%s\"" % (h, o, s))
    if not himc_lines and not comp_changes:
        print("  没有任何组字状态记录（可能全程没切到中文输入法，或 IME 未装载）")

    print()
    print("=== 4. 客户端调用的 imm32 API（计数 + 调用者模块 top3）===")
    if not api:
        print("  没记到任何 imm32/user32 调用")
    for k, v in api.most_common(30):
        callers = ", ".join("%s×%d" % (c, n) for c, n in api_callers.get(k, {}).most_common(3))
        print("  %-28s %-6d %s" % (k, v, callers))

    print()
    print("=== 5. 前台窗口样本 ===")
    for f in focus_lines[:6]:
        print("  hwnd=%s[%s] pid=%s tid=%s focus=%s hkl=%s IsIME=%s" % f)

    print()
    print("=== 判读提示 ===")
    if ime_msgs == 0 and not api:
        print("  · 客户端既没收到 IME 消息、也没调 imm32 → 它的输入链路完全没接输入法：")
        print("    修法是主动给游戏窗口挂上输入法上下文，并把组字结果喂进客户端。")
    elif ime_msgs == 0 and api:
        print("  · 客户端自己调 imm32（在读输入法），但没有 WM_IME_* 消息 → 它走的是")
        print("    『直接读 IMC 组字缓冲』那条路；重点看第 4 节里 DFO.exe 调用者与第 3 节组字串。")
    elif ime_msgs > 0 and not nonascii_chars:
        print("  · IME 消息到了窗口，但没有非 ASCII 字符上屏 → 客户端收到组字却没消费/被过滤。")
    elif nonascii_chars:
        print("  · 已经有非 ASCII 字符到达该窗口，问题可能在更后面的渲染/提交环节。")
    return 0


if __name__ == "__main__":
    sys.exit(main())
