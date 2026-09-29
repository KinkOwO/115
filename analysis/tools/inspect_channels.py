"""只读导出当前客户端的频道规则，供频道目录和界面分类核对。"""

import argparse
import hashlib
import json
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "server/work/dfo-lan/scripts"))
from repair_item_names import Archive


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, default=ROOT / "client/Script.pvf")
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    archive = Archive(args.source)
    records = {}
    for index in range(archive.count):
        name, parent, _, _, _, kind = archive.record(index)
        if kind != 1:
            continue
        filename = archive.resolve(name).lower()
        if "channel" not in filename or not filename.endswith(".etc"):
            continue
        path = (archive.resolve(parent) + "/" + filename).lower().lstrip("/")
        archive.files[path] = index
        records[path] = {"sha256": hashlib.sha256(archive.read(path)).hexdigest(), "tokens": archive.tokens(path)}
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps({"source_sha256": archive.source_sha256, "files": records}, ensure_ascii=False, indent=2), encoding="utf-8")
    print("已只读导出频道规则：", "、".join(records))


if __name__ == "__main__":
    main()
