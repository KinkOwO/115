"""List literal references to every top-level config JSON without running services.

This is an inventory of source references, not a proof of runtime use. Dynamic
paths, external tools and old binaries require a separate execution-path review.
The report contains filenames and source locations, never configuration values.
"""

import argparse
from collections import defaultdict
from pathlib import Path
import re
import sys


def source_files(root):
    for directory in ("cmd", "internal", "scripts"):
        for path in sorted((root / directory).rglob("*")):
            if path.is_file() and path.suffix in (".go", ".py", ".ps1", ".cmd"):
                yield path
    # Small profiles can refer to policy files. Large game catalogs are data,
    # and searching their content would mix item text with dependency evidence.
    for path in sorted(list((root / "configs").glob("*.json")) + list((root / "docs").glob("*.json"))):
        if path.stat().st_size <= 1024 * 1024:
            yield path


def group(path):
    parts = path.parts
    if path.name.endswith("_test.go") or path.name.startswith("test_"):
        return "tests"
    if parts[0] in ("configs", "docs"):
        return "profiles"
    if parts[:2] == ("cmd", "wireprobe"):
        return "gateway"
    if parts[0] == "internal":
        return "internal"
    if parts[0] == "cmd":
        return "tools"
    return "scripts"


def inventory(root):
    configs = sorted((root / "configs").glob("*.json"))
    if not configs:
        raise ValueError("no top-level JSON configs found")
    pattern = re.compile("|".join(re.escape(path.name) for path in configs))
    references = {path.name: defaultdict(set) for path in configs}
    scanned = 0
    for path in source_files(root):
        relative = path.relative_to(root)
        category = group(relative)
        scanned += 1
        for line_number, line in enumerate(path.read_text(encoding="utf-8-sig").splitlines(), 1):
            for match in pattern.finditer(line):
                references[match.group()][category].add((relative.as_posix(), line_number))
    lines = [
        "# configs JSON 字面引用清单",
        "",
        "由 `scripts/audit_config_references.py` 生成。扫描范围为模块的 cmd、internal、scripts，以及不超过 1 MiB 的 configs JSON 和 docs 根目录的 JSON 示例。",
        "",
        "引用包含注释和历史分支；数字是不同引用位置数，不表示正式运行必读。零引用也不能作为删除依据：动态拼路径、模块外启动器、GM 代理及历史二进制未由本清单证明。",
        "",
        f"共 {len(configs)} 个顶层 JSON，扫描 {scanned} 个文件；只输出文件名、大小和引用位置，不输出配置值。",
        "",
        "| JSON | MiB | 网关 | internal | 工具 | 测试 | 脚本 | 配置引用 | 非测试引用示例 |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---|",
    ]
    for path in configs:
        refs = references[path.name]
        examples = sorted(set().union(*(refs[key] for key in ("gateway", "internal", "tools", "scripts", "profiles"))))[:2]
        example_text = "<br>".join(f"`{name}:{line}`" for name, line in examples) or "需追踪动态路径或外部入口"
        counts = " | ".join(str(len(refs[key])) for key in ("gateway", "internal", "tools", "tests", "scripts", "profiles"))
        lines.append(f"| `{path.name}` | {path.stat().st_size / (1024 * 1024):.3f} | {counts} | {example_text} |")
    return "\n".join(lines) + "\n"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument("--output", type=Path, help="new Markdown report; existing files are refused")
    args = parser.parse_args()
    report = inventory(args.root.resolve())
    if args.output:
        with args.output.open("x", encoding="utf-8", newline="\n") as target:
            target.write(report)
    else:
        sys.stdout.write(report)


if __name__ == "__main__":
    main()
