#!/usr/bin/env python3
"""Export CeraShop catalog from pvfinspect-extracted etc/(r)cerashop.etc.txt.

Reproducible script to build configs/cerashop.json from pvfinspect output.
Matches the 1362 goods in DFO 115us client Script.pvf.
"""

import json
import pathlib
import re
import sys


def main():
    root = pathlib.Path(__file__).resolve().parent.parent
    workspace_root = root.parent.parent.parent

    # Candidate text paths
    txt_candidates = [
        workspace_root / "runtime" / "pvf" / "00-(r)cerashop.etc.txt",
        root.parent.parent / "runtime" / "pvf" / "00-(r)cerashop.etc.txt",
        root / "runtime" / "pvf" / "00-(r)cerashop.etc.txt",
        pathlib.Path("runtime/pvf/00-(r)cerashop.etc.txt").resolve(),
    ]

    txt_path = None
    for p in txt_candidates:
        if p.is_file():
            txt_path = p
            break

    if txt_path is None:
        sys.exit(f"Error: cerashop txt not found in candidates: {txt_candidates}")

    try:
        with open(txt_path, encoding="latin1") as f:
            text = f.read()
    except OSError as e:
        sys.exit(f"Error reading file {txt_path}: {e}")

    sections = re.findall(r"\[([^/\]]+)\](.*?)\[/\1\]", text, re.DOTALL)
    section_dict = dict(sections)

    valid_sections = [
        "avatar dye",
        "item event",
        "item second",
        "item",
        "item mod or ext",
        "item period or contract",
        "creature",
        "item etc",
        "package related",
    ]

    pattern = re.compile(
        r"(\d+)\s+(\d+)\s+(\d+)\s+(-?\d+)\s+(-?\d+)\s+(-?\d+)\s+(-?\d+)\s+(-?\d+)\s+`([^`]*)`"
    )

    goods = {}
    for s in valid_sections:
        body = section_dict.get(s, "")
        matches = pattern.findall(body)
        for m in matches:
            gid, tmpl, qty, f3, f4, price, f7, f8, name = m
            try:
                gid_val = int(gid)
                tmpl_val = int(tmpl)
                qty_val = int(qty)
                price_val = int(price)
            except ValueError as e:
                sys.exit(f"Error parsing number for good {gid}: {e}")
            goods[str(gid_val)] = {
                "template": tmpl_val,
                "qty": qty_val,
                "price": price_val,
                "name": name,
                "section": s,
            }

    if len(goods) != 1362:
        sys.exit(f"Unexpected goods count: {len(goods)} (expected 1362)")

    # Assert critical item
    item3000126 = goods.get("3000126")
    if not item3000126:
        sys.exit("Critical item 3000126 not found!")
    if (
        item3000126["template"] != 10000541
        or item3000126["price"] != 50
        or item3000126["qty"] != 1
    ):
        sys.exit(f"Critical item 3000126 mismatch: {item3000126}")

    catalog = {
        "source": "server/work/client-build/Script.inner.pvf:etc/(r)cerashop.etc",
        "source_sha256": "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80",
        "goods": goods,
    }

    out_file = root / "configs" / "cerashop.json"
    out_file.parent.mkdir(parents=True, exist_ok=True)
    try:
        with open(out_file, "w", encoding="utf-8") as f:
            json.dump(catalog, f, indent=2, ensure_ascii=False)
    except OSError as e:
        sys.exit(f"Error writing catalog {out_file}: {e}")

    print(f"Exported {len(goods)} cerashop items to {out_file}")


if __name__ == "__main__":
    main()
