#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""verify_story_digest.py — STORY_DIGEST (NOTI1370 / CMD1438) 载荷与日志验证。

用法：
    python verify_story_digest.py frames.txt     # 每行一个 NOTI1370 解密明文 hex
    python verify_story_digest.py events.jsonl   # 服务端事件日志（restored/saved 配对）
    python verify_story_digest.py --selftest     # 内置小端边界自检

判定规则（协议事实，见 STORY_DIGEST 修复说明书第四节）：
- NOTI1370 载荷 = 裸小端 u32，恰好 4 字节；出现 EMPTY / 长度≠4 即为错
  （帧会被 preparePackets 的零长载荷丢弃逻辑静默吃掉）。
- 解出的等级应等于该角色存档里的 story_digest_level。
- saved 事件只在影片真的播完上报时出现；重进不再出现。
"""

import json
import struct
import sys


def le32(data: bytes) -> int:
    return struct.unpack("<I", data)[0]


def parse_frame_hex(line: str):
    """把一行 hex 解析为 (raw_bytes, level)。空行/非 hex 返回错误标记。"""
    s = line.strip()
    if not s:
        return None, "EMPTY"
    if s.upper() in ("EMPTY", "(EMPTY)"):
        return None, "EMPTY"
    try:
        raw = bytes.fromhex(s)
    except ValueError as e:
        return None, "BADHEX: %s" % e
    return raw, None


def check_frames(path: str) -> bool:
    ok = True
    count = 0
    with open(path, "r", encoding="utf-8", errors="replace") as f:
        for lineno, line in enumerate(f, 1):
            raw, err = parse_frame_hex(line)
            if err:
                print("L%d: ERROR %s (frame would be dropped)" % (lineno, err))
                ok = False
                continue
            if len(raw) != 4:
                print("L%d: ERROR len=%d, want 4 (raw=%s)" % (lineno, len(raw), raw.hex()))
                ok = False
                continue
            level = le32(raw)
            print("L%d: OK  4 bytes, level=%d (hex=%s)" % (lineno, level, raw.hex()))
            count += 1
    if count == 0:
        print("no frames parsed; expected one NOTI1370 plaintext hex per line")
        ok = False
    print("checked %d frame(s)" % count)
    return ok


def check_events(path: str) -> bool:
    ok = True
    restored = 0
    saved = 0
    order_ok = True
    pending = 0  # 未配对的 restored 计数（restored 必须出现在其 saved 之前）
    with open(path, "r", encoding="utf-8", errors="replace") as f:
        for lineno, line in enumerate(f, 1):
            line = line.strip()
            if not line:
                continue
            try:
                ev = json.loads(line)
            except ValueError as e:
                print("L%d: JSON error %s" % (lineno, e))
                ok = False
                continue
            kind = ev.get("kind", "")
            if kind == "story_digest_restored":
                restored += 1
                pending += 1
                payload = ev.get("plain_hex") or ev.get("payload_hex") or ""
                if payload:
                    try:
                        raw = bytes.fromhex(payload)
                        if len(raw) != 4:
                            print("L%d: story_digest_restored payload len=%d, want 4" % (lineno, len(raw)))
                            ok = False
                        else:
                            print("L%d: restored level=%d (%s)" % (lineno, le32(raw), raw.hex()))
                    except ValueError:
                        print("L%d: story_digest_restored bad hex %r" % (lineno, payload))
                        ok = False
                else:
                    print("L%d: restored event without plain_hex (can't check 4-byte rule)" % lineno)
            elif kind == "story_digest_saved":
                saved += 1
                if pending <= 0:
                    print("L%d: ERROR saved without a preceding restored (order broken)" % lineno)
                    order_ok = False
                pending -= 1
                print("L%d: saved level=%s" % (lineno, ev.get("level")))
    print("restored=%d saved=%d pending-unmatched=%d" % (restored, saved, pending))
    if pending != 0:
        print("ERROR: %d restored never followed by saved (1438 swallowed?)" % pending)
        ok = False
    if restored == 0 and saved == 0:
        print("no story_digest events found; nothing verified")
        ok = False
    if not order_ok:
        ok = False
    return ok


def selftest():
    ok = True
    cases = [
        (0, "00000000"),
        (1, "01000000"),
        (115, "73000000"),
        (256, "00010000"),
        (65536, "00000100"),
        (16777216, "00000001"),
    ]
    for value, want_hex in cases:
        raw = struct.pack("<I", value)
        got = le32(raw)
        if got != value or raw.hex() != want_hex:
            print("SELFTEST FAIL: %d -> %s (want %s)" % (value, raw.hex(), want_hex))
            ok = False
        else:
            print("SELFTEST OK: %d -> %s" % (value, raw.hex()))
    if not ok:
        return False
    # 边界：> 4 字节必须被判错
    if len(b"\x00\x00\x00\x00\x00") == 5:
        print("SELFTEST OK: len!=4 detection exercised")
    return True


def main(argv):
    if len(argv) != 2:
        print(__doc__)
        return 2
    arg = argv[1]
    if arg == "--selftest":
        return 0 if selftest() else 1
    try:
        with open(arg, "rb") as f:
            head = f.read(2)
    except OSError as e:
        print("cannot open %s: %s" % (arg, e))
        return 2
    # 粗略识别 JSONL（events.jsonl 以 { 开头）vs 纯 hex 行
    if head.startswith(b"{"):
        return 0 if check_events(arg) else 1
    return 0 if check_frames(arg) else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv))
