"""IDAPython 自动化注释脚本：将 Dump 出的 xorstr 明文字符串直接标注到 IDA Pro IDB 中。

用法：
在 IDA Pro 中打开 client/DFO.exe.i64 后：
File -> Script file... -> 选择本脚本运行。
或者在 IDA 下方 Python 控制台中执行：
exec(open(r"F:/projects/agents/reverse/115us/analysis/dumps/ida_annotate_xorstr.py", encoding="utf-8").read())
"""

import json
import os
import sys

try:
    import ida_bytes  # type: ignore[import-not-found, import-untyped]

    IN_IDA = True
except ImportError:
    ida_bytes = None  # type: ignore
    IN_IDA = False


def run() -> None:
    if not IN_IDA or ida_bytes is None:
        print("[!] 本脚本专用于 IDA Pro 环境内运行。")
        return

    dump_dir = os.path.dirname(os.path.abspath(__file__))
    json_path = os.path.join(dump_dir, "xorstr_addr_to_text.json")
    if not os.path.exists(json_path):
        print(f"[!] 未找到映射文件: {json_path}")
        return

    print(f"[*] 读取 xorstr 映射表: {json_path} ...")
    try:
        with open(json_path, encoding="utf-8") as f:
            data = json.load(f)
    except (OSError, json.JSONDecodeError) as e:
        print(f"[!] 读取映射文件失败: {e}", file=sys.stderr)
        return

    if not isinstance(data, dict):
        print("[!] 映射表格式错误，预期为 dict", file=sys.stderr)
        return

    print(f"[*] 共载入 {len(data)} 条字符串映射，开始写入 IDB 注释...")
    count = 0
    for addr_str, text in data.items():
        try:
            va = int(addr_str, 16)
        except ValueError:
            continue

        # 设置可重复注释 (repeatable comment)
        ida_bytes.set_cmt(va, str(text), True)
        count += 1
        if count % 10000 == 0:
            print(f"[*] 已标注 {count} / {len(data)} ...")

    print(f"[+] 注释完成！共标注 {count} 处字符串。")


if __name__ == "__main__":
    if IN_IDA:
        run()
    else:
        print("[!] 本脚本专用于 IDA Pro 环境内运行。")
