#!/usr/bin/env python3
"""打包 qol.fast-startup —— 客户端启动加速（跳过遗留开发环境共享目录探测）。

改什么
------
115 级客户端在启动早期（CNGameCore::Create 与 CNGameCore::initNgsModule 两条路径）
会拼出 UNC 路径 ``\\\\toolserver02.neople.co.kr\\CommonConfig`` 并调 GetFileAttributesW
判断"开发环境共享配置目录是否存在"。该主机在本地/局域网不可达，SMB 超时让主线程空等，
窗口迟迟不出现。

本 mod 把这条探测路径改成一个**本地不存在**的目录 ``C:\\115us-no-commoncfg-share``：
GetFileAttributesW 立刻返回 INVALID_FILE_ATTRIBUTES，判定结果与今天（共享不可达）**完全一致**
（都是"没有共享配置"），初始化流程原样继续。

怎么做到的（只改 56 字节密文，不动代码）
----------------------------------------
两个字符串在 .rdata 里以客户端自带的 xorstr 编码静态存储；解密算法见
``analysis/tools/pe_decode_xorstr.py``（IDA 复原：sub_146E8C490 / 0x146e8c7d0）。
本脚本用同一算法**重新加密**新路径，写在原密文位置（同长度、同 header、同密钥流），
所以两个调用点（同一对字符串 VA）同时被改掉，且不需要改任何一条指令。

用法
----
  python build-mod.py                    # 校验现场 + 生成 dist/qol.fast-startup-<ver>.zip
  python build-mod.py --modkit <modkit>  # 顺便跑一次 modkit verify
  python build-mod.py --exe <DFO.exe>    # 指定用于现场核对的客户端（只读）

退出码：0 成功；1 失败（现场字节不符时**不改任何东西**）。
"""

import argparse
import hashlib
import json
import os
import struct
import subprocess
import sys
import zipfile

MOD_ID = "qol.fast-startup"
VERSION = "1.0.0"
NAME = "客户端启动加速（跳过遗留共享目录探测）"
AUTHOR = "DFO 115us"
DESCRIPTION = (
    "客户端启动早期会探测遗留开发环境共享目录 \\\\toolserver02.neople.co.kr\\CommonConfig"
    "（CNGameCore::Create 与 initNgsModule 各一处），该主机不可达导致 SMB 超时、主线程空等、"
    "窗口迟迟不出现。本 mod 用客户端自带的 xorstr 算法重新加密一个新路径，把探测目标改成"
    "本地不存在的 C:\\115us-no-commoncfg-share：GetFileAttributesW 立即返回失败，"
    "判定结果与共享不可达时完全一致（都是没有共享配置），初始化流程原样继续。"
    "只改 DFO.exe 里 56 字节密文，不动任何指令、不动 PVF/NPK/存档；卸载从备份整文件还原。"
)
HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_EXE = r"C:\Game\dof\115us\DFO\DFO.exe"

# ---------------------------------------------------------------------------
# 定点参数：全部是**文件偏移**。本 EXE 的 .rdata 满足 file_off == RVA
# （节表 raw=0x9186000 / va=0x149186000，两者相减为 0x1000，即 .text 的节对齐）。
# VA 0x14A8FF460 -> 文件偏移 0xA8FF460，已按解密结果逐条核对（见 PATCH-NOTES.md）。
# ---------------------------------------------------------------------------
SLOT_OFF = 0xA8FF460      # 字符串槽起点（header 4 字节 + 密文 56 字节 + 4 字节对齐填充）
SLOT_LEN = 64             # 到下一个字符串（VA 0x14A8FF4A0）的距离
HEADER = "0054482a"       # 原 header：byte0 flag=0x00 / byte1 seed=0x54 / u16 code=0x2A48
BODY_OFF = SLOT_OFF + 4   # 只补丁密文正文，header 原样保留
BODY_LEN = 56             # = 2 * (27 字符 + NUL)，与原文长度一致
ORIG_TEXT = "\\\\toolserver02.neople.co.kr"
NEW_TEXT = "C:\\115us-no-commoncfg-share"   # 恰好 27 字符 → 密文长度不变
SOURCE_SHA256 = "01c633dfda5c883ba126261246cb8067b0a0331500587d8062df368da175a836"
SOURCE_SIZE = 258972712

# ---------------------------------------------------------------------------
# xorstr 算法（与 analysis/tools/pe_decode_xorstr.py 同一份实现；此处额外提供 encode）
# ---------------------------------------------------------------------------
KEY_SEED = 0x9A714CA0
MUL_DWORD = 65599
MUL_BYTE = 263


def xorstr_header(raw, multiplier=2):
    seed8 = raw[1] & 0xFE
    code = struct.unpack_from("<H", raw, 2)[0]
    length = (multiplier * (code ^ (seed8 | (seed8 << 7)))) & 0xFFFFFFFF
    return seed8, code, length, (seed8 | KEY_SEED) & 0xFFFFFFFF


def xorstr_decode(raw, multiplier=2):
    seed8, code, length, key = xorstr_header(raw, multiplier)
    if length == 0 or length > 4096:
        raise ValueError("implausible length %d" % length)
    body = raw[4:4 + length]
    out = bytearray()
    dwords = length // 4
    for i in range(dwords):
        v = struct.unpack_from("<I", body, i * 4)[0] ^ key
        out += struct.pack("<I", v)
        key = (v + MUL_DWORD * key) & 0xFFFFFFFF
    for i in range(length % 4):
        v = body[dwords * 4 + i] ^ (key & 0xFF)
        out.append(v)
        key = (v + MUL_BYTE * key) & 0xFFFFFFFF
    return out, length


def xorstr_encode(plain, seed8, length):
    """密文 = 明文 ^ 密钥流；密钥流的递推用**明文**驱动，所以与解码严格互逆。"""
    if len(plain) != length:
        raise ValueError("plaintext %d bytes != declared %d" % (len(plain), length))
    key = (seed8 | KEY_SEED) & 0xFFFFFFFF
    out = bytearray()
    dwords = length // 4
    for i in range(dwords):
        p = struct.unpack_from("<I", plain, i * 4)[0]
        out += struct.pack("<I", p ^ key)
        key = (p + MUL_DWORD * key) & 0xFFFFFFFF
    for i in range(length % 4):
        p = plain[dwords * 4 + i]
        out.append(p ^ (key & 0xFF))
        key = (p + MUL_BYTE * key) & 0xFFFFFFFF
    return bytes(out)


# ---------------------------------------------------------------------------
# 现场核对：before 必须与文件实际一致，否则停手
# ---------------------------------------------------------------------------
def verify_site(path):
    lines = []
    ok = True
    with open(path, "rb") as fh:
        data = fh.read()
    size = len(data)
    sha = hashlib.sha256(data).hexdigest()
    good = size == SOURCE_SIZE and sha == SOURCE_SHA256
    lines.append("  [%s] %-34s size=%d sha256=%s" %
                 ("OK " if good else "WARN", "整份 EXE 身份", size, sha[:16] + "…"))
    if not good:
        lines.append("       声明：size=%d sha256=%s（仅声明，不阻断；下面逐字节核对才是判据）"
                     % (SOURCE_SIZE, SOURCE_SHA256[:16] + "…"))

    slot = data[SLOT_OFF:SLOT_OFF + SLOT_LEN]
    if len(slot) != SLOT_LEN:
        lines.append("  [BAD] 偏移 0x%X 越界（文件只有 %d 字节）" % (SLOT_OFF, size))
        return False, lines

    got_header = slot[:4].hex()
    head_ok = got_header == HEADER
    ok = ok and head_ok
    lines.append("  [%s] %-34s off 0x%-9X 期望 %s 实测 %s"
                 % ("OK " if head_ok else "BAD", "字符串槽 header", SLOT_OFF, HEADER, got_header))

    plain, length = xorstr_decode(slot, 2)
    text = plain.decode("utf-16-le", "replace").split("\x00")[0]
    txt_ok = length == BODY_LEN and text == ORIG_TEXT
    ok = ok and txt_ok
    lines.append("  [%s] %-34s 期望 %r 实测 %r（len=%d）"
                 % ("OK " if txt_ok else "BAD", "解密原文", ORIG_TEXT, text, length))

    nxt = data[SLOT_OFF + SLOT_LEN - 8:SLOT_OFF + SLOT_LEN]
    lines.append("  [i ] %-34s %s（仅提示，不参与判定）" % ("槽尾 8 字节", nxt.hex()))
    return ok, lines


def build_patch(path):
    with open(path, "rb") as fh:
        data = fh.read()
    slot = data[SLOT_OFF:SLOT_OFF + SLOT_LEN]
    seed8, code, length, _ = xorstr_header(slot, 2)
    if len(NEW_TEXT) != len(ORIG_TEXT):
        raise SystemExit("FAIL: 替换串长度 %d != 原串 %d（会改变密文长度，需同步改 header）"
                         % (len(NEW_TEXT), len(ORIG_TEXT)))
    plain_new = (NEW_TEXT + "\x00").encode("utf-16-le")
    body_after = xorstr_encode(plain_new, seed8, length)
    if len(body_after) != BODY_LEN:
        raise SystemExit("FAIL: 新密文 %d 字节 != %d" % (len(body_after), BODY_LEN))

    # 自证：把补丁打到内存副本上，再按同一算法解回来
    probe = bytearray(data)
    probe[BODY_OFF:BODY_OFF + BODY_LEN] = body_after
    back, _ = xorstr_decode(probe[SLOT_OFF:SLOT_OFF + SLOT_LEN], 2)
    got = back.decode("utf-16-le").split("\x00")[0]
    if got != NEW_TEXT:
        raise SystemExit("FAIL: 自证失败——补丁后解出 %r" % got)
    if probe[SLOT_OFF:SLOT_OFF + 4] != bytes.fromhex(HEADER):
        raise SystemExit("FAIL: 自证失败——header 被改动")
    if probe[:SLOT_OFF] != data[:SLOT_OFF] or probe[SLOT_OFF + SLOT_LEN:] != data[SLOT_OFF + SLOT_LEN:]:
        raise SystemExit("FAIL: 自证失败——槽外字节被改动")

    return {
        "offset": BODY_OFF,
        "before": data[BODY_OFF:BODY_OFF + BODY_LEN].hex(),
        "after": body_after.hex(),
    }, ("%s\\CommonConfig" % NEW_TEXT)


def make_manifest(sha, patch, note):
    return {
        "schema": 2,
        "id": MOD_ID,
        "version": VERSION,
        "name": NAME,
        "author": AUTHOR,
        "description": DESCRIPTION,
        "permissions": ["client.exe.patch"],
        "layers": {
            "client": {
                "ops": [
                    {
                        "kind": "exe.patch",
                        "target": "DFO.exe",
                        "sourceSHA256": sha,
                        "note": note,
                        "patches": [patch],
                    }
                ]
            }
        },
    }


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--exe", default=DEFAULT_EXE,
                    help="用于现场核对的原始 DFO.exe（只读，不改）")
    ap.add_argument("--out", default=os.path.join(HERE, "dist"))
    ap.add_argument("--modkit", default="", help="可选：打包后跑一次 modkit verify")
    ap.add_argument("--skip-verify", action="store_true", help="跳过现场核对（不推荐）")
    args = ap.parse_args()

    if not os.path.isfile(args.exe):
        print("FAIL: 找不到客户端 EXE：%s" % args.exe, file=sys.stderr)
        return 1

    if not args.skip_verify:
        print("现场字节核对（%s）:" % args.exe)
        ok, lines = verify_site(args.exe)
        for ln in lines:
            print(ln)
        if not ok:
            print("FAIL: 现场字节与补丁 before 不符 —— 停手，不改任何东西。", file=sys.stderr)
            return 1
        print("  全部现场核对通过。\n")

    patch, new_path = build_patch(args.exe)
    assert len(patch["before"]) == len(patch["after"]), "before/after 长度不等"

    with open(args.exe, "rb") as fh:
        data = fh.read()
    sha = hashlib.sha256(data).hexdigest()
    note = ("把 xorstr 字符串 %r（VA 0x14A8FF460，两个调用点共用）重新加密为本地不存在的目录 "
            "%r：运行时拼出的路径变成 %r，GetFileAttributesW 立即失败，"
            "判定与共享不可达时一致。只改 56 字节密文，header/指令/PVF/NPK 均不动。"
            % (ORIG_TEXT, NEW_TEXT, new_path))
    manifest = make_manifest(sha, patch, note)

    staging = os.path.join(args.out, "staging")
    if os.path.isdir(staging):
        import shutil
        shutil.rmtree(staging)
    os.makedirs(staging, exist_ok=True)

    notes_src = os.path.join(HERE, "PATCH-NOTES.md")
    if os.path.isfile(notes_src):
        client_dir = os.path.join(staging, "client")
        os.makedirs(client_dir, exist_ok=True)
        with open(notes_src, "rb") as fh:
            notes = fh.read()
        with open(os.path.join(client_dir, "PATCH-NOTES.md"), "wb") as fh:
            fh.write(notes)

    with open(os.path.join(staging, "mod.json"), "w", encoding="utf-8", newline="\n") as fh:
        json.dump(manifest, fh, ensure_ascii=False, indent=2)
        fh.write("\n")

    zip_name = "%s-%s.zip" % (MOD_ID, VERSION)
    zip_path = os.path.join(args.out, zip_name)
    if os.path.isfile(zip_path):
        os.remove(zip_path)
    with zipfile.ZipFile(zip_path, "w", zipfile.ZIP_DEFLATED) as zf:
        for root, _dirs, files in os.walk(staging):
            for fn in sorted(files):
                full = os.path.join(root, fn)
                rel = os.path.relpath(full, staging).replace(os.sep, "/")
                info = zipfile.ZipInfo(rel, date_time=(2026, 10, 11, 0, 0, 0))
                info.compress_type = zipfile.ZIP_DEFLATED
                with open(full, "rb") as fh:
                    zf.writestr(info, fh.read())

    print("client : 1 条 exe.patch（DFO.exe @ 0x%X，%d 字节密文）" % (patch["offset"], BODY_LEN))
    print("  现场 before : %s" % patch["before"])
    print("  补丁 after  : %s" % patch["after"])
    print("  运行时路径  : %r" % new_path)
    print("权限   : client.exe.patch")
    print("产物   : %s（%d 字节）" % (zip_path, os.path.getsize(zip_path)))
    print("")
    print("安装（先退出游戏）:")
    print("  <modkit.exe> verify  --client <客户端> --mod \"%s\"" % zip_path)
    print("  <modkit.exe> plan    --client <客户端> --mod \"%s\" --root <整合包根>" % zip_path)
    print("  <modkit.exe> install --client <客户端> --mod \"%s\" --root <整合包根>" % zip_path)
    print("卸载（逐字节还原 DFO.exe）:")
    print("  <modkit.exe> uninstall --client <客户端> --id %s --root <整合包根>" % MOD_ID)

    if args.modkit:
        print("\nmodkit verify:")
        rc = subprocess.call([args.modkit, "verify", "--client", os.path.dirname(args.exe),
                              "--mod", zip_path])
        if rc != 0:
            print("FAIL: modkit verify 退出码 %d" % rc, file=sys.stderr)
            return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
