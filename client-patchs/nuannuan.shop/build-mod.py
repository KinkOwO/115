#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""把「暖暖商城」v2 + v3.2 打成一个可一键安装 / 一键卸载的 modkit mod 包。

放在 client-patchs/ 而不是 mods/ 的原因（踩过的坑，别再挪回去）：
  启动器扫库是 mods/ 下 **两层** 且认「目录里有 mod.json」就算一个 mod
  （internal/modlib/store.go: DefaultScanDepth=2）。源码目录一旦叫 mods/nuannuan.shop
  且带 mod.json，导入时会被当成"覆盖同名 mod"——引擎先删掉整个目录再解压，
  源码和已打好的 zip 会一起被删光（2026-10-11 实测就是这么没的）。

打包要点：
  - 大文件**不落 staging**，直接从 Downloads 源目录流式读进 zip（省 5.7GB 二次拷贝）；
  - v3.2 的 size/sha256 直接用作者 _payload_manifest.json 的声明值（不重算）；
  - v2 的 15 个 NPK 现算 sha256，结果缓存到 dist 目录，重打包不重算；
  - zip64 + ZIP_STORED（PVF/NPK 本身已压缩，deflate 只白耗时间）；
  - PowerShell 7 便携版打进 tools/pwsh7/ —— 启动器安装时会把它作为包内解释器
    传给引擎，解决 npk.index 依赖 pwsh7 的硬阻塞；
  - 索引构建器 USLocalServer.*.dll 的目标框架是 **net10.0**，而 tools/pwsh7 那份
    pwsh 是**自包含 net8**（自包含应用忽略 roll-forward，永远跑在 8.0.10），加载不了它们。
    所以额外打进 tools/npkidx/NpkIndexTool.exe —— 一个自包含 net10 的单文件小工具
    （idxtool/ 的源码，跑 `publish.ps1` 重编），脚本复制它到 %TEMP% 后调用；
    目标机不需要装任何 .NET 运行时。

用法：
  python build-mod.py                 # 输出到 ../../.tmp/nuannuan-dist
  python build-mod.py --dry-run       # 只列清单，不打 zip
  python build-mod.py --skip-v2       # 只打 v3.2（约 1.75GB）
  python build-mod.py --out <目录>

退出码：0 成功；1 失败。
"""

import argparse
import hashlib
import io
import json
import os
import sys
import zipfile

MOD_ID = "nuannuan.shop"
VERSION = "3.2.0"
ZIP_DATE = (2026, 10, 10, 0, 0, 0)  # 固定时间戳 → 同样输入产出同样字节

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))

SRC_ROOT = os.path.join(os.path.expanduser("~"), "Downloads", "10月4日版暖暖商城mod")
SRC_V32 = os.path.join(SRC_ROOT, "暖暖商城_覆盖安装包_v3.2", "安装用文件", "客户端")
SRC_V2 = os.path.join(SRC_ROOT, "暖暖商城_覆盖安装包_v2", "安装用文件", "客户端")
IDX_TOOLS = os.path.join(SRC_ROOT, "暖暖商城_覆盖安装包_v3.2", "重建NPK索引", "PVF", "tools")
PWSH_SRC = os.path.join(REPO, ".tmp", "pwsh7", "extracted")
# 自包含 net10 的索引重建小工具（idxtool/ 编出来的单文件 exe）
NPKIDX_EXE = os.path.join(REPO, ".tmp", "npkidx", "NpkIndexTool.exe")
# 「扩展按钮 / 商城标签页」客户端代理 DLL（2026-10-08 编译，855,040 B；vendor/ 下不入库）。
# 它必须占客户端根的 dinput8.dll —— 那是客户端唯一会被自动加载的槽位。
# 实测它会自己转发系统 DirectInput 并 LoadLibrary("ChineseLocalization.dll")，
# 所以 qol.client-host 宿主与 .115us-mods 插件链不受影响。
EXTBTN_SRC = os.path.join(HERE, "vendor", "dinput8.dll")
EXTBTN_FALLBACK = os.path.join(os.path.expanduser("~"), "Downloads", "dinput8.dll")

DEFAULT_OUT = os.path.join(REPO, ".tmp", "nuannuan-dist")

# v3.2 里 5 个是「替换客户端已有 NPK」，2 个 dragonrobe4 是新增
V32_REPLACE_NPK = [
    "sprite_interface.NPK",
    "sprite_interface2_aura.NPK",
    "sprite_interface2_titlebook.NPK",
    "sprite_live_event_kor_2025_0109_boostup.NPK",
    "sprite_live_event_kor_2026_0326_boostup.NPK",
]
V32_ADD_NPK = [
    "sprite_avatar_dragonrobe4_20261006.NPK",
    "sprite_avatar_dragonrobe4_effects_20261006.NPK",
]
# v2 独有：神兽 / 第1期 avatar 15 个（v2 目录里还有一份 NpkIndex.etc，不收，索引我们自己重建）
V2_ADD_NPK = [
    "sprite_avatar_effect110_01.NPK",
    "sprite_avatar_import110_01.NPK",
    "sprite_avatar_import110_02.NPK",
    "sprite_avatar_import110_03.NPK",
    "sprite_avatar_import110_04.NPK",
    "sprite_avatar_import110_05.NPK",
    "sprite_avatar_import110_06.NPK",
    "sprite_avatar_import110_07.NPK",
    "sprite_avatar_import110_08.NPK",
    "sprite_avatar_import110_09.NPK",
    "sprite_avatar_import110_10.NPK",
    "sprite_avatar_misc110_01.NPK",
    "sprite_avatar_model110_01.NPK",
    "sprite_avatar_model110_02.NPK",
    "sprite_beast110_01.NPK",
]
FONTS = [
    "head-name-simsun.ttc",
    "hud-cn-digits.otf",
    "menu-cn-regular.otf",
    "menu-cn-serifbold.otf",
]

CHUNK = 1 << 22  # 4 MiB


def sha256_of(path):
    h = hashlib.sha256()
    with io.open(path, "rb") as f:
        for chunk in iter(lambda: f.read(CHUNK), b""):
            h.update(chunk)
    return h.hexdigest()


def load_v32_manifest():
    """v3.2 作者自带的 _payload_manifest.json（15 项 size+sha256），直接采信。"""
    # 注意：_payload_manifest.json 在 v3.2 包的**根**，不在 安装用文件\ 下
    path = os.path.join(SRC_ROOT, "暖暖商城_覆盖安装包_v3.2", "_payload_manifest.json")
    with io.open(path, "r", encoding="utf-8") as f:
        data = json.load(f)
    out = {}
    for item in data["files"]:
        out[item["path"].replace("\\", "/")] = (item["size"], item["sha256"])
    return out


def v2_hash_cache(out_dir):
    return os.path.join(out_dir, "v2-npk-hashes.json")


def resolve_v2_hashes(out_dir):
    """v2 的 15 个 NPK 现算 sha256（约 3.9GB，几分钟），缓存下来供重打包复用。"""
    cache_path = v2_hash_cache(out_dir)
    cache = {}
    if os.path.isfile(cache_path):
        try:
            with io.open(cache_path, "r", encoding="utf-8") as f:
                cache = json.load(f)
        except Exception:
            cache = {}
    result = {}
    for name in V2_ADD_NPK:
        src = os.path.join(SRC_V2, "ImagePacks2", name)
        if not os.path.isfile(src):
            print("FAIL: missing v2 NPK: %s" % src, file=sys.stderr)
            return None
        size = os.path.getsize(src)
        hit = cache.get(name)
        if hit and hit.get("size") == size and hit.get("sha256"):
            result[name] = (size, hit["sha256"])
            print("  %-42s %12d  (cached)" % (name, size))
        else:
            result[name] = (size, sha256_of(src))
            print("  %-42s %12d  %s" % (name, size, result[name][1][:16]))
    os.makedirs(out_dir, exist_ok=True)
    with io.open(cache_path, "w", encoding="utf-8") as f:
        json.dump({k: {"size": v[0], "sha256": v[1]} for k, v in result.items()},
                  f, ensure_ascii=False, indent=2)
    return result


def build_manifest(v32, v2, extbtn=None):
    def op(kind, target, source, size, sha, note=None):
        d = {"kind": kind, "target": target, "source": source,
             "size": size, "sha256": sha}
        if note:
            d["note"] = note
        return d

    pvf_ops = []
    pvf_ops.append(op("replace", "Script.pvf", "pvf/Script.pvf", *v32["Script.pvf"],
                      note="龙皮龙武国服特色 PVF；必须与 sk.dat 成对替换"))
    pvf_ops.append(op("replace", "sk.dat", "pvf/sk.dat", *v32["sk.dat"],
                      note="PVF 解密钥，与 Script.pvf 同源（2,560 → 2,816 字节）"))

    client_ops = []
    client_ops.append(op("file.replace", "dstr.dat", "client/dstr.dat", *v32["dstr.dat"],
                         note="客户端文本串（商城 / 道具名等）"))
    for f in FONTS:
        key = "Fonts/" + f
        client_ops.append(op("file.add", key, "client/Fonts/" + f, *v32[key],
                             note="新增字体，现场原本没有；卸载时删除"))
    if extbtn:
        client_ops.append(op(
            "file.replace", "dinput8.dll", "client/dinput8.dll", extbtn[0], extbtn[1],
            note="扩展按钮 / 商城标签页（115-shop-tabs）：顶替客户端根 dinput8.dll —— "
                 "客户端唯一会被自动加载的槽位。该 DLL 自己转发系统 DirectInput，"
                 "并主动加载 ChineseLocalization.dll（qol.client-host 宿主），"
                 "所以汉化代理与 .115us-mods 插件链不受影响；卸载逐字节还原原代理。"
                 "它只在 DFO.exe 镜像布局校验通过后才挂钩，不匹配就只记一行日志、不影响启动"))
    client_ops.append(op(
        "file.add", ".115us-mods/NuanNuanShop.dll", "client/NuanNuanShop.dll",
        *v32["dinput8.dll"],
        note="原包的 dinput8.dll 改名而来：不占 dinput8 槽位，由 qol.client-host "
             "宿主 LoadLibrary 触发 DllMain，保住汉化代理与它的两个插件"))

    res_ops = []
    for name in V32_ADD_NPK + V2_ADD_NPK:
        if name in V32_ADD_NPK:
            size, sha = v32["ImagePacks2/" + name]
        else:
            size, sha = v2[name]
        res_ops.append(op("npk.add", "ImagePacks2/" + name,
                          "resource/ImagePacks2/" + name, size, sha))
    for name in V32_REPLACE_NPK:
        size, sha = v32["ImagePacks2/" + name]
        res_ops.append(op("npk.replace", "ImagePacks2/" + name,
                          "resource/ImagePacks2/" + name, size, sha,
                          note="覆盖客户端已有的同名 NPK，卸载时逐字节还原"))
    res_ops.append({
        "kind": "npk.index",
        "target": "ImagePacks2/NpkIndex.etc",
        "script": "resource/rebuild-index.ps1",
        "work": "index",
        "args": ["-ClientDir", "{client}",
                 "-ToolsDir", "{pack}/resource/PVF/tools",
                 "-PackDir", "{pack}"],
        "note": "NPK 变动后必须重建索引，否则一启动就闪退。引擎只把 .ps1 抽到工作目录，"
                "所以工具目录/包根要用 {pack} 显式传进去；索引构建器是 net10.0，"
                "脚本会调用包内自包含 net10 的 tools/npkidx/NpkIndexTool.exe",
    })

    return {
        "schema": 2,
        "id": MOD_ID,
        "version": VERSION,
        "name": "暖暖商城（龙皮龙武国服特色 + 第四套龙袍 + 神兽第1期 15 套）",
        "author": "暖暖商城作者 / 打包：115us",
        "description": ("客户端资源整档：Script.pvf+sk.dat（pvf 层 replace）、dstr.dat、"
                        "4 个字体、龙袍变身插件 DLL；dinput8.dll 换成带「商城标签页 / "
                        "扩展按钮」的版本（顶替唯一槽位，卸载还原）；"
                        "NPK 新增 17 个 / 替换 5 个；装完自动重建 ImagePacks2/NpkIndex.etc。"
                        "不动 DFO.exe。"
                        "配套服务端体验开关（商城全开等 6 项）需在启动器 profile 里设置，不在本包内。"),
        "permissions": ["client.file.write", "pvf.merge", "resource.npk",
                        "resource.index", "exec.script"],
        "requires": ["qol.client-host"],
        "layers": {
            "pvf": {"ops": pvf_ops},
            "client": {"ops": client_ops},
            "resource": {"ops": res_ops},
        },
    }


def collect_members(manifest, want_v2, pwsh_src, extbtn_src=None):
    """(zip 内相对路径, 本地绝对路径) 列表；按清单顺序展开。"""
    members = []

    def add(arc, local):
        if not os.path.isfile(local):
            print("FAIL: missing source: %s" % local, file=sys.stderr)
            raise SystemExit(1)
        members.append((arc, local))

    for o in manifest["layers"]["pvf"]["ops"]:
        add(o["source"], os.path.join(SRC_V32, os.path.basename(o["target"])))
    for o in manifest["layers"]["client"]["ops"]:
        src = o["source"]
        if src.endswith("NuanNuanShop.dll"):
            # 原包自带的 dinput8.dll 改名而来（业主选择保留，无害：宿主只加载不调用）
            add(src, os.path.join(SRC_V32, "dinput8.dll"))
        elif src == "client/dinput8.dll":
            if not extbtn_src:
                continue
            add(src, extbtn_src)
        else:
            add(src, os.path.join(SRC_V32, o["target"].replace("\\", "/")))
    for o in manifest["layers"]["resource"]["ops"]:
        if o["kind"] == "npk.index":
            continue
        name = os.path.basename(o["target"])
        base = SRC_V32 if name in V32_ADD_NPK or name in V32_REPLACE_NPK else SRC_V2
        if not want_v2 and base is SRC_V2:
            continue
        add(o["source"], os.path.join(base, "ImagePacks2", name))

    # 索引重建脚本 + 它依赖的两个 dll
    add("resource/rebuild-index.ps1", os.path.join(HERE, "resource", "rebuild-index.ps1"))
    tools = os.path.join(HERE, "resource", "PVF", "tools")
    for name in sorted(os.listdir(tools)):
        p = os.path.join(tools, name)
        if os.path.isfile(p) and name.lower().endswith(".dll"):
            add("resource/PVF/tools/" + name, p)

    # 包内自带 pwsh7（启动器安装时探测 tools/pwsh7/pwsh.exe 并传给引擎）
    if os.path.isdir(pwsh_src):
        for root, _dirs, files in os.walk(pwsh_src):
            for name in files:
                full = os.path.join(root, name)
                rel = os.path.relpath(full, pwsh_src).replace(os.sep, "/")
                members.append(("tools/pwsh7/" + rel, full))
    else:
        print("WARN: pwsh7 便携版不在 %s —— npk.index 会被阻断" % pwsh_src, file=sys.stderr)

    # 索引脚本的 net10 运行现场：自包含小工具，复制到 %TEMP% 后直接跑
    if os.path.isfile(NPKIDX_EXE):
        members.append(("tools/npkidx/NpkIndexTool.exe", NPKIDX_EXE))
        print("  %-16s %5d 文件  <- %s" % ("tools/npkidx", 1, NPKIDX_EXE))
    else:
        print("WARN: 索引工具不在 %s —— npk.index 会在 .NET 8 上加载 net10 的 DLL 而失败"
              % NPKIDX_EXE, file=sys.stderr)
    return members


def write_zip(zip_path, manifest, members):
    total_bytes = 0
    os.makedirs(os.path.dirname(zip_path), exist_ok=True)
    if os.path.exists(zip_path):
        os.remove(zip_path)
    with zipfile.ZipFile(zip_path, "w", zipfile.ZIP_STORED, allowZip64=True) as z:
        zi = zipfile.ZipInfo("mod.json", date_time=ZIP_DATE)
        zi.compress_type = zipfile.ZIP_STORED
        zi.external_attr = 0o644 << 16
        z.writestr(zi, json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")

        for arc, local in members:
            size = os.path.getsize(local)
            total_bytes += size
            zi = zipfile.ZipInfo(arc, date_time=ZIP_DATE)
            zi.compress_type = zipfile.ZIP_STORED
            zi.external_attr = 0o644 << 16
            with z.open(zi, "w") as dst, io.open(local, "rb") as src:
                while True:
                    chunk = src.read(CHUNK)
                    if not chunk:
                        break
                    dst.write(chunk)
    return total_bytes


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", default=DEFAULT_OUT)
    ap.add_argument("--dry-run", action="store_true")
    ap.add_argument("--skip-v2", action="store_true", help="只打 v3.2（约 1.75GB）")
    ap.add_argument("--pwsh", default=PWSH_SRC)
    ap.add_argument("--extbtn", default=EXTBTN_SRC,
                    help="带扩展按钮的 dinput8.dll（默认 vendor/dinput8.dll）")
    args = ap.parse_args()

    for d in (SRC_V32, SRC_V2, IDX_TOOLS):
        if not os.path.isdir(d):
            print("FAIL: 源目录不存在：%s" % d, file=sys.stderr)
            return 1

    v32 = load_v32_manifest()
    print("v3.2 载荷声明：%d 项（来自作者 _payload_manifest.json）" % len(v32))

    if args.skip_v2:
        v2 = {n: (0, "") for n in V2_ADD_NPK}
        print("跳过 v2（--skip-v2）")
    else:
        print("计算 v2 的 15 个 NPK 的 sha256（约 3.9GB，几分钟）：")
        v2 = resolve_v2_hashes(args.out)
        if v2 is None:
            return 1

    extbtn_src = args.extbtn
    if not os.path.isfile(extbtn_src):
        extbtn_src = EXTBTN_FALLBACK if os.path.isfile(EXTBTN_FALLBACK) else None
    extbtn = None
    if extbtn_src:
        extbtn = (os.path.getsize(extbtn_src), sha256_of(extbtn_src))
        print("扩展按钮 DLL：%s（%d B，%s…）" % (extbtn_src, extbtn[0], extbtn[1][:16]))
    else:
        print("WARN: 扩展按钮 DLL 不在 %s，也不在 %s —— 本次不打 dinput8.dll 这条 op"
              % (args.extbtn, EXTBTN_FALLBACK), file=sys.stderr)

    manifest = build_manifest(v32, v2, extbtn)
    members = collect_members(manifest, not args.skip_v2, args.pwsh, extbtn_src)

    pvf_n = len(manifest["layers"]["pvf"]["ops"])
    cli_n = len(manifest["layers"]["client"]["ops"])
    res_n = len(manifest["layers"]["resource"]["ops"])
    payload = [(a, l) for a, l in members if not a.startswith("tools/")]
    pwsh_n = len(members) - len(payload)
    total = sum(os.path.getsize(l) for _a, l in members)

    print("")
    print("mod.json   : %s %s" % (manifest["id"], manifest["version"]))
    print("ops        : pvf %d / client %d / resource %d" % (pvf_n, cli_n, res_n))
    print("members    : %d（载荷 %d + 随包工具 %d：pwsh7/npkidx）"
          % (len(members), len(payload), pwsh_n))
    print("payload    : %.2f GB（含随包工具）" % (total / float(1 << 30)))

    if args.dry_run:
        print("")
        print("--dry-run：到此为止，未写任何文件")
        return 0

    zip_path = os.path.join(args.out, "%s-%s.zip" % (MOD_ID, VERSION))
    print("")
    print("打包 → %s" % zip_path)
    write_zip(zip_path, manifest, members)
    print("done       : %d bytes（%.2f GB）" % (os.path.getsize(zip_path),
                                                os.path.getsize(zip_path) / float(1 << 30)))
    print("")
    print("下一步：启动器 → MOD 工具 → 导入这个 zip（不要在资源管理器里停在 mods\\ 下选文件）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
