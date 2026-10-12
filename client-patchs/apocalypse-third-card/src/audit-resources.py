"""Verify that existing third-card artwork matches the user's CN original.

Read only. No image generation, conversion, recompression or client writes.
CN Card_2 animations reference LevelSelect.img frame 3; the card XUI references
Symbol_Flat_L.img. Compiled ANI pool indices are not portable between PVFs.
"""
import argparse
import hashlib
import json
from pathlib import Path
import struct
import zlib


def digest(data):
    return hashlib.sha256(data).hexdigest()


def entry(path, wanted):
    data = path.read_bytes()
    if data[:16] != b'NeoplePack_Bill\0':
        raise ValueError('Unsupported NPK: ' + str(path))
    count = struct.unpack_from('<I', data, 16)[0]
    if 20 + count * 264 > len(data):
        raise ValueError('Truncated NPK directory')
    key = (b'puchikon@neople dungeon and fighter ' + b'DNF' * 100)[:256]
    found = []
    for index in range(count):
        at = 20 + index * 264
        offset, size = struct.unpack_from('<II', data, at)
        name = bytes(a ^ b for a, b in zip(data[at + 8:at + 264], key)).split(b'\0')[0].decode('ascii').lower()
        if name == wanted:
            if offset < 20 + count * 264 or offset + size > len(data):
                raise ValueError('NPK entry out of bounds')
            found.append(data[offset:offset + size])
    if len(found) != 1:
        raise ValueError('Missing or ambiguous native artwork: ' + wanted)
    return found[0]


def frame(data, wanted):
    if data[:16] != b'Neople Img File\0' or len(data) < 32:
        raise ValueError('Unsupported IMG header')
    index_size = struct.unpack_from('<I', data, 16)[0]
    version, count = struct.unpack_from('<II', data, 24)
    if version != 2 or wanted >= count or count > 100000:
        raise ValueError('Unreviewed IMG layout')
    at, cursor = 32, 32 + index_size
    for index in range(count):
        kind, compression = struct.unpack_from('<II', data, at)
        if kind == 17:
            if index == wanted:
                raise ValueError('Unexpected linked target artwork')
            at += 8
            continue
        kind, compression, width, height, size, x, y, canvas_w, canvas_h = struct.unpack_from('<9I', data, at)
        at += 36
        if kind not in (14, 15, 16) or compression not in (5, 6):
            raise ValueError('Unreviewed image encoding')
        if compression == 5:
            size = width * height * (4 if kind == 16 else 2)
        if cursor + size > len(data) or at > 32 + index_size:
            raise ValueError('Truncated image')
        if index == wanted:
            pixels = data[cursor:cursor + size]
            if compression == 6:
                pixels = zlib.decompress(pixels)
            if len(pixels) != width * height * (4 if kind == 16 else 2):
                raise ValueError('Wrong pixel count')
            return {'format': kind, 'width': width, 'height': height, 'x': x, 'y': y,
                    'canvas_width': canvas_w, 'canvas_height': canvas_h, 'pixels_sha256': digest(pixels)}
        cursor += size
    raise ValueError('Artwork frame absent')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--cn-client', type=Path, required=True)
    parser.add_argument('--us-client', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    result = {'generated_artwork': False, 'cn_client': str(args.cn_client.resolve()), 'us_client': str(args.us_client.resolve()), 'sources': {}}
    for name, directory in [('cn', args.cn_client), ('us', args.us_client)]:
        level = entry(directory / 'ImagePacks2/sprite_contents_2026_apocalypse_ui.NPK', 'sprite/contents/2026/apocalypse/ui/levelselect.img')
        symbol = entry(directory / 'ImagePacks2/sprite_interface2_legion_legioncommon.NPK', 'sprite/interface2/legion/legioncommon/symbol_flat_l.img')
        result['sources'][name] = {'levelselect_sha256': digest(level), 'third_card': frame(level, 3), 'symbol_flat_l_sha256': digest(symbol)}
    cn, us = result['sources']['cn'], result['sources']['us']
    if cn['third_card'] != us['third_card'] or cn['symbol_flat_l_sha256'] != us['symbol_flat_l_sha256']:
        raise ValueError('US third-card art differs from CN; a reviewed resource import is required')
    result['third_card_matches_cn_original'] = True
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    print('PASS: third-card pixels, placement and symbol match CN original; no generated artwork')


if __name__ == '__main__':
    main()
