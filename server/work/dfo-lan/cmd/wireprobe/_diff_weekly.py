import json
import struct
import sys

DEC = r'D:\115us\analysis-tools\output\official_20261002-160349_decoded'
OPS = {637, 706, 781, 782, 537, 1336, 2254, 2255, 2256}


def parse_bin(path):
    data = open(path, 'rb').read()
    count = struct.unpack('<I', data[:4])[0]
    frames = []
    offset = 4
    for i in range(count):
        if offset + 2 > len(data):
            break
        mlen = struct.unpack('<H', data[offset:offset + 2])[0]
        rec = data[offset + 2: offset + 2 + mlen - 2]
        frames.append((i, rec))
        offset += 2 + mlen - 2 + 2
        if offset > len(data):
            break
    return frames


def frame_info(i, rec):
    if len(rec) < 8:
        return None
    op = struct.unpack('<I', rec[4:8])[0]
    body = rec[8:]
    return i, op, body


def summarize_781(body):
    return f"781 len={len(body)} state@4={body[4:8].hex()} 0xff={body[0xff]:02x} tail={body[-12:].hex()}"


def summarize_2254(body):
    return f"2254 len={len(body)} head16={body[0:16].hex()} off21={body[21]:02x}"


def summarize_generic(op, body):
    return f"op={op} len={len(body)} head={body[:24].hex()}"


for name in ('session_s1_s2c', 'session_s4_s2c'):
    print(f"===== {name} =====")
    frames = parse_bin(fr'{DEC}\{name}.bin')
    print('frames:', len(frames))
    for i, rec in frames:
        info = frame_info(i, rec)
        if not info:
            continue
        _, op, body = info
        if op not in OPS:
            continue
        if op == 781 and len(body) >= 272:
            print(f"  f{i}", summarize_781(body))
        elif op == 2254 and len(body) >= 32:
            print(f"  f{i}", summarize_2254(body))
        elif op == 706:
            print(f"  f{i} 706 len={len(body)} head={body[:40].hex()}")
        elif op in (537, 1336, 637):
            print(f"  f{i} op={op} len={len(body)} {body[:32].hex()}")
        elif op == 782:
            print(f"  f{i} 782 len={len(body)} state@4={body[4:8].hex() if len(body)>8 else ''}")
        elif op == 2255:
            print(f"  f{i} 2255 len={len(body)} {body[:32].hex()}")
