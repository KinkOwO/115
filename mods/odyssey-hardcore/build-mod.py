#!/usr/bin/env python3
"""打包 odyssey.hardcore（奥德赛模式强化：副本禁消耗品 + 死亡不可复活 + 怪物血量 ×10）。

与 mods/examples/*/build-mod.py 同一套路：收集 server 层 → 从源码推导钩子与权限 →
生成组织文件 mod.json → 打包（固定时间戳，字节可复现）→ 自证。

用法：
  python build-mod.py                        # 输出到 <脚本目录>/dist
  python build-mod.py --modkit <modkit.exe>  # 顺便跑一次 modkit verify
  python build-mod.py --rules-only           # 只出服务端两条规则，不碰 DFO.exe

退出码：0 成功；1 失败。
"""

import argparse
import hashlib
import io
import json
import os
import shutil
import struct
import subprocess
import sys
import zipfile

MOD_ID = "odyssey.hardcore"
VERSION = "1.0.0"
ZIP_DATE = (2026, 10, 6, 0, 0, 0)  # 固定时间戳 → 可复现
NAME = "奥德赛强化（副本禁消耗品 + 死亡不可复活 + 怪物血量 ×10）"
DESCRIPTION = (
    "奥德赛模式（DGN 声明 arad odyssey）内两条规则：① 副本内禁止使用消耗品（可携带，"
    "城镇不受影响）——服务端拒绝 CMD44，客户端本来就不发 NOTI1584、界面已灰；"
    "② 死亡不可复活——CMD41 的三级回退（奥德赛测试额度/背包复活币/CERA）在入口整条拒绝，"
    "随后走既有死亡超时流程判负回城。规则经 internal/modpolicy 挂在既有业务点上，"
    "不是第二套玩法表；管理器中启停，改动需重启服务端。"
    "怪物血量 ×10 由 client 层对 DFO.exe 的三处同长度字节补丁实现（HP getter 调用侧 trampoline，"
    "把返回值 ×10）：服务端不下发怪物血量（怪物包只有 entity/template/level/rank），"
    "血量表既不在 Script.pvf 也不在任何 NPK 里，所以只能走 EXE 补丁。"
    "该补丁是**全局**的（所有副本怪物都变厚），不是奥德赛专属；详见本 mod 的 README 与 client/PATCH-NOTES.md。"
)
HERE = os.path.dirname(os.path.abspath(__file__))

# ---------------------------------------------------------------------------
# client 层 exe.patch 的定点参数（现场地址全部是**文件偏移**，按 PE 节表换算：
# 本 EXE 的 .text/.rdata/.data 三个节里 file_off == VA - 0x140000000，已逐条核对）
# ---------------------------------------------------------------------------
SOURCE_SHA256 = "01c633dfda5c883ba126261246cb8067b0a0331500587d8062df368da175a836"
SOURCE_SIZE = 258972712
DEFAULT_EXE = r"C:\Game\dof\115us\DFO\DFO.exe"

GETTER_VA = 0x147220E40          # HP getter
CALLSITE1_VA = 0x145A06EB6       # flag==1 调用点 1（函数 0x145A06DB0 内）
CALLSITE2_VA = 0x145A06F56       # flag==1 调用点 2（同一函数）
CAVE_VA = 0x1486786AB            # 函数间对齐填充（RET 之后 21 字节 cc，全镜像零引用）
CAVE_LEN = 21                    # 可用 21 字节，本设计只用 16
TRAMPOLINE_LEN = 16

CALLSITE1_OFF = 0x5A06EB6
CALLSITE2_OFF = 0x5A06F56
CAVE_OFF = 0x86786AB
CALLSITE1_BEFORE = "e8859f8101"  # call 0x147220E40
CALLSITE2_BEFORE = "e8e59e8101"  # call 0x147220E40
CAVE_BEFORE = "cc" * CAVE_LEN    # 21 字节 cc


def rel32(site_va, instr_len, target):
    """rel32 = 目标 − (指令地址 + 指令长度)，越界即报错。"""
    d = target - (site_va + instr_len)
    if not (-0x80000000 <= d <= 0x7FFFFFFF):
        raise ValueError("rel32 越界：0x%X -> 0x%X" % (site_va, target))
    return struct.pack("<i", d)


def build_trampoline(multiplier, cave_va=CAVE_VA, getter_va=GETTER_VA):
    """16 字节 trampoline（放进 0x1486786AB 那个 21 字节 cc 洞）：

        4C 8B D1        mov  r10, rcx        ; 保住调用者传的描述块指针（getter 会打掉 rcx）
        E8 disp32       call getter          ; 原样调用 HP getter ⇒ rax = 血量
        48 6B C0 imm8   imul rax, rax, mul   ; 返回值 ×10
        49 8B CA        mov  rcx, r10        ; 按原契约把 rcx 交回调用者（本题里已是死值）
        C3              ret                  ; 回到 0x145A06EBB / 0x145A06F5B

    r10/r11 是易失寄存器：getter 本来就会打掉它们，6 个上层调用者都在 call 之后
    直接从 xmm0 读返回值，不依赖 rcx/r10/r11，所以这个写入不改变可见语义。
    """
    m = int(multiplier)
    if m != multiplier or not (1 <= m <= 127):
        raise ValueError("倍率必须是 1..127 的整数（imul r64,r64,imm8），当前 %r" % (multiplier,))
    if m == 1:
        raise ValueError("倍率 1 等于不打补丁，拒绝生成空补丁")
    b = bytearray()
    b += b"\x4C\x8B\xD1"                                   # mov r10, rcx
    b += b"\xE8" + rel32(cave_va + len(b), 5, getter_va)    # call getter
    b += b"\x48\x6B\xC0" + bytes([m])                       # imul rax, rax, imm8
    b += b"\x49\x8B\xCA"                                   # mov rcx, r10
    b += b"\xC3"                                           # ret
    if len(b) > TRAMPOLINE_LEN:
        raise ValueError("trampoline %d 字节 > %d" % (len(b), TRAMPOLINE_LEN))
    return bytes(b)


def verify_patch_sites(exe_path=DEFAULT_EXE):
    """打包前逐字节核对现场：before 必须与文件实际一致，否则停手。

    返回 (ok, lines)。不修改任何文件。
    """
    lines = []
    if not os.path.isfile(exe_path):
        return False, ["找不到 %s" % exe_path]
    with io.open(exe_path, "rb") as f:
        data = f.read()
    got_sha = hashlib.sha256(data).hexdigest()
    if len(data) != SOURCE_SIZE or got_sha != SOURCE_SHA256:
        return False, ["DFO.exe 不是清单认可的那一份：size=%d sha256=%s（期望 %d / %s）"
                       % (len(data), got_sha, SOURCE_SIZE, SOURCE_SHA256)]
    checks = [
        ("callsite1 0x145A06EB6 call getter", CALLSITE1_OFF, CALLSITE1_BEFORE),
        ("callsite2 0x145A06F56 call getter", CALLSITE2_OFF, CALLSITE2_BEFORE),
        ("代码洞 0x1486786AB 21 字节 cc", CAVE_OFF, CAVE_BEFORE),
        # 不得触碰的现场（顺带自证“旧补丁确实已还原”）
        ("getter flag 分支 test bl,bl", 0x7220ECD, "84db"),
        ("getter flag!=0 jne（不得动）", 0x7220ECF, "756f"),
        ("getter flag==0 入口 lea（不得动）", 0x7220ED1, "488d4f48"),
        ("getter addsd 旧补丁点（须已还原）", 0x7220F34, "f20f5805e46f8d04"),
        ("共享常量 1.0（不得动）", 0xBAF7F20, "000000000000f03f"),
        ("洞前 RET（不得动）", CAVE_OFF - 1, "c3"),
    ]
    ok = True
    for label, off, expect in checks:
        got = data[off:off + len(expect) // 2].hex()
        good = got == expect
        ok = ok and good
        lines.append("  [%s] %-40s off 0x%-9X 期望 %s 实测 %s"
                     % ("OK " if good else "BAD", label, off, expect, got))
    return ok, lines


def make_client_ops(multiplier):
    """生成 mod.json 里的 client 层 ops（3 处等长补丁）。"""
    tramp = build_trampoline(multiplier)
    tramp_padded = tramp + b"\xCC" * (CAVE_LEN - len(tramp))
    patches = [
        {"offset": CALLSITE1_OFF, "before": CALLSITE1_BEFORE,
         "after": (b"\xE9" + rel32(CALLSITE1_VA, 5, CAVE_VA)).hex()},
        {"offset": CALLSITE2_OFF, "before": CALLSITE2_BEFORE,
         "after": (b"\xE9" + rel32(CALLSITE2_VA, 5, CAVE_VA)).hex()},
        {"offset": CAVE_OFF, "before": CAVE_BEFORE, "after": tramp_padded.hex()},
    ]
    for p in patches:
        assert len(p["before"]) == len(p["after"]), "补丁长度不等：%r" % p
    return [{
        "kind": "exe.patch",
        "target": "DFO.exe",
        "sourceSHA256": SOURCE_SHA256,
        "patches": patches,
    }]



def sha256_of(path):
    h = hashlib.sha256()
    with io.open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def read_text(path):
    with io.open(path, "r", encoding="utf-8", errors="replace") as f:
        return f.read()


def stage_tree(src, dst):
    if not os.path.isdir(src):
        return 0
    n = 0
    for root, _dirs, files in os.walk(src):
        for name in files:
            full = os.path.join(root, name)
            rel = os.path.relpath(full, src)
            target = os.path.join(dst, rel)
            os.makedirs(os.path.dirname(target), exist_ok=True)
            shutil.copy2(full, target)
            n += 1
    return n


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", default=os.path.join(HERE, "dist"))
    ap.add_argument("--modkit", default=None)
    ap.add_argument("--hp-multiplier", type=int, default=10,
                    help="怪物血量倍率（写进调用侧 trampoline 的 imul 立即数，1..127，默认 10）")
    ap.add_argument("--exe", default=DEFAULT_EXE,
                    help="用于**打包前逐字节核对**的原始 DFO.exe（只读，不改）")
    ap.add_argument("--skip-verify", action="store_true",
                    help="跳过打包前的现场字节核对（不推荐；也要求本机存在 --exe）")
    ap.add_argument("--rules-only", action="store_true",
                    help="只出服务端两条规则（不打血量 EXE 补丁）——血量补丁连带影响不确定时的备用包")
    args = ap.parse_args()

    # ---- 0) 打包前逐字节核对现场（before 必须与文件实际一致，否则停手）----
    if not args.rules_only and not args.skip_verify:
        print("现场字节核对（%s）:" % args.exe)
        ok, lines = verify_patch_sites(args.exe)
        for ln in lines:
            print(ln)
        if not ok:
            print("FAIL: 现场字节与补丁 before 不符 —— 停手，不改任何东西。", file=sys.stderr)
            return 1
        print("  全部现场核对通过。")
        print("")

    staging = os.path.join(args.out, "staging")
    if os.path.isdir(staging):
        shutil.rmtree(staging)
    os.makedirs(staging, exist_ok=True)

    # ---- 1) server 层（Go 源码 + 内嵌 config.json）----
    server_src = os.path.join(HERE, "server")
    if not os.path.isdir(server_src):
        print("FAIL: missing server/ layer", file=sys.stderr)
        return 1
    if not os.path.isfile(os.path.join(server_src, "mod.go")):
        print("FAIL: server/mod.go missing", file=sys.stderr)
        return 1
    if not os.path.isfile(os.path.join(server_src, "config.json")):
        print("FAIL: server/config.json missing（规则开关是 mod 的一部分）", file=sys.stderr)
        return 1
    n_server = stage_tree(server_src, os.path.join(staging, "server"))
    print("server layer: %d file(s)（含 config.json）" % n_server)

    # ---- 1b) client 层（只有 exe.patch；框架要求声明了该层就得有 client/ 目录）----
    if not args.rules_only:
        client_src = os.path.join(HERE, "client")
        if not os.path.isdir(client_src):
            print("FAIL: missing client/ layer dir", file=sys.stderr)
            return 1
        n_client = stage_tree(client_src, os.path.join(staging, "client"))
        print("client layer: %d file(s)（exe.patch 说明）" % n_client)

    # ---- 2) 从源码推导钩子与权限（不手写第二份清单）----
    code = read_text(os.path.join(server_src, "mod.go"))
    hooks = []
    if "RegisterBoot(" in code:
        hooks.append({"name": "server.boot",
                      "note": "把奥德赛规则写进 internal/modpolicy（生效点）"})
    if "RegisterConsole(" in code:
        hooks.append({"name": "console.command",
                      "note": "一次性诊断命令：status / what"})
    if not hooks:
        print("FAIL: server/mod.go 没有登记任何钩子", file=sys.stderr)
        return 1
    # client 层：DFO.exe 三处**等长**字节补丁（HP getter 调用侧 trampoline，怪物血量 ×10）
    #
    # 设计（方案 B：只改调用侧，getter 一个字节都不碰）——完整论证见 client/PATCH-NOTES.md
    #   现场：getter 0x147220E40 的返回值在 RAX（尾部 0x147221000 `F2 48 0F 2C C6` = CVTTSD2SI RAX,X6）；
    #        全镜像只有 2 个 flag==1 调用点（0x145A06EB6 / 0x145A06F56），都在函数 0x145A06DB0 内，
    #        且都读 RAX（后继 `F3 48 0F 2A F0/C0` = CVTSI2SS X?,RAX）。
    #   补丁：把这两条 5 字节 `call <getter>` 换成同长度 `jmp 0x1486786AB`（函数间 21 字节 cc 填充，
    #        全镜像零引用），洞里放 16 字节 trampoline：
    #          mov r10,rcx / call <getter> / imul rax,rax,10 / mov rcx,r10 / ret
    #        即“调用原 getter，把返回值 ×10，再交回调用者”。
    #   为什么不碰 getter：`test bl,bl` + `jne` 只区分 flag，改它必然经由代码洞并牵动
    #        21 个 flag==0 调用点（角色面板/血条比例）；改调用侧则 flag==0 路径**逐字节不变**。
    #   为什么等长不覆盖相邻指令：两处 call→jmp 都是 5 字节原地替换，洞内只在 cc 上写，
    #        没有任何指令边界被移动。
    #   注意：补丁是**全局**的（凡走这两条取值口的怪物都变厚），不是奥德赛专属。
    client_ops = [] if args.rules_only else make_client_ops(args.hp_multiplier)
    permissions = sorted({"server.hook"} | ({"client.exe.patch"} if client_ops else set()))

    # ---- 3) 组织文件 ----
    manifest = {
        "schema": 2,
        "id": MOD_ID,
        "version": VERSION,
        "name": NAME,
        "author": "DFO 115us",
        "description": DESCRIPTION if not args.rules_only else (
            DESCRIPTION.replace(
                "怪物血量 ×10 由 client 层对 DFO.exe 的三处同长度字节补丁实现（HP getter 调用侧 trampoline，"
                "把返回值 ×10）：服务端不下发怪物血量（怪物包只有 entity/template/level/rank），"
                "血量表既不在 Script.pvf 也不在任何 NPK 里，所以只能走 EXE 补丁。"
                "该补丁是**全局**的（所有副本怪物都变厚），不是奥德赛专属；详见本 mod 的 README 与 client/PATCH-NOTES.md。",
                "本包（rules-only 变体）**不含**血量补丁：只启用服务端两条规则"
                "（副本内禁用消耗品 + 死亡不可复活立即回城）。血量补丁的连带影响未定，先出这一版备用。")),
        "permissions": permissions,
        "layers": dict(
            [("server", {"package": ".", "hooks": hooks})]
            + ([("client", {"ops": client_ops})] if client_ops else [])
        ),
    }
    manifest_path = os.path.join(staging, "mod.json")
    with io.open(manifest_path, "w", encoding="utf-8", newline="\n") as f:
        f.write(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
    # 源码指纹：用来回答"这个包到底是从哪份 mod.go 打出来的"
    print("mod.go sha256: %s" % sha256_of(os.path.join(server_src, "mod.go")))

    # ---- 4) 打包 ----
    suffix = "-rules-only" if args.rules_only else ""
    zip_path = os.path.join(args.out, "%s-%s%s.zip" % (MOD_ID, VERSION, suffix))
    if os.path.exists(zip_path):
        os.remove(zip_path)

    members = []
    for root, _dirs, files in os.walk(staging):
        for name in files:
            full = os.path.join(root, name)
            rel = os.path.relpath(full, staging).replace(os.sep, "/")
            members.append((rel, full))
    members.sort(key=lambda kv: (kv[0] != "mod.json", kv[0]))

    with zipfile.ZipFile(zip_path, "w", zipfile.ZIP_DEFLATED) as z:
        for rel, full in members:
            zi = zipfile.ZipInfo(rel, date_time=ZIP_DATE)
            zi.compress_type = zipfile.ZIP_DEFLATED
            zi.external_attr = 0o644 << 16
            with io.open(full, "rb") as f:
                z.writestr(zi, f.read())

    print("")
    print("built : %s" % zip_path)
    print("size  : %d bytes" % os.path.getsize(zip_path))
    print("sha256: %s" % sha256_of(zip_path))
    print("perms : %s" % ", ".join(permissions))
    print("hooks : %s" % ", ".join(h["name"] for h in hooks))
    if client_ops:
        print("client: exe.patch ×%d 处（怪物血量 ×%d：两条 call->jmp 调用侧改写 + 16 字节 trampoline；"
              "getter 与共享 1.0 常量零改动）"
              % (len(client_ops[0]["patches"]), args.hp_multiplier))
        for i, p in enumerate(client_ops[0]["patches"], 1):
            print("         #%d off 0x%X  before %s  after %s"
                  % (i, p["offset"], p["before"], p["after"]))
    else:
        print("client: 无（--rules-only：只出服务端两条规则，不碰 DFO.exe）")

    if args.modkit:
        mk = args.modkit
        if not os.path.isfile(mk):
            print("FAIL: modkit not found: %s" % mk, file=sys.stderr)
            return 1
        print("")
        print("verify:")
        rc = subprocess.call([mk, "verify", "--mod", zip_path])
        if rc != 0:
            print("FAIL: modkit verify exit=%d" % rc, file=sys.stderr)
            return 1

    print("")
    print("next:")
    print("  modkit plan    --client <客户端目录> --mod \"%s\" --root <启动器根>" % zip_path)
    print("  modkit install --client <客户端目录> --mod \"%s\" --root <启动器根>" % zip_path)
    print("  卸载（一键还原 EXE）：")
    print("  modkit uninstall --client <客户端目录> --id %s --root <启动器根>" % MOD_ID)
    print("  # 装完看启动日志：'[mod odyssey.hardcore] 策略已生效' 与 'odyssey mode rules: 开启：…'")
    return 0


if __name__ == "__main__":
    sys.exit(main())
