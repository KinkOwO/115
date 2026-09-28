"""把皮肤仓库（伤害字体 / 觉醒插图 / 边框）第八~十一轮的修复重放到源码树上。

背景：一键启动器的「同步上游」会整棵替换 server 目录，本任务未合进上游的改动因此每次同步
都会退回上游版（2026-09-28 已发生三次：01:26:50 抹掉 42 个文件、02:37:20 抹掉 10 个、
03:09:59 抹掉 14 个并且把本任务新增的那 4 个未跟踪源文件从磁盘上删掉了）。它弹窗里那 4 个
「上游已删除、本机仍存在」的文件其实是本任务新增的源文件，删掉才是真的编译失败。
03:18:12 启动前还发生第二类事故：启动器把 configs 下 4 份本机配置（items.index.json、
booster-catalog.json、shop-vault-release.json、shop-purchase-pilot.json）搬进
runtime/disabled-configs，物品索引因此没被补全，服务端在 attunement 校验处 log.Fatal，
启动脚本 exit status 1。本脚本让这两种恢复都变成一条命令，并且机械地保证源码侧只加不删：

  * 目标文件缺行（上游退回旧版）：用载荷里的确认版整文件写回；
  * 目标文件有载荷里没有的行（别人后来改过）：判为冲突，一个字节都不动，报出来让人先看；
  * 目标文件与载荷逐字节相同：跳过，所以重复运行无害（幂等）。

用法：
    python port_skin_family_fixes.py --dry      # 只看会动哪些文件
    python port_skin_family_fixes.py            # 实际重放（源码 + 被隔离的本机配置）
    python port_skin_family_fixes.py --target D:/somewhere/115us
    python port_skin_family_fixes.py --force    # 冲突文件也按载荷覆盖，判据见下
    python port_skin_family_fixes.py --configs-only  # 只搬回被隔离的 4 份本机配置，不动源码
    python port_skin_family_fixes.py --refresh  # 用当前工作树刷新载荷（确认版前滚）

冲突文件要逐条判一次再决定加不加 --force。03:09:59 那次同步之后，main.go / skin_cargo.go /
catalog/skin_storage.go / configs/skin-storage-items.json 等 8 个文件「目标多出的行」全是
被本任务取代的上游旧版（单 id 的 SelectSkinRequest、skin-storage-items-v2、slot-26 那段旧
注释），一条别人的新改动都没有，所以那一次用 --force 整文件覆盖是对的；若列出的行里混进了
别人的新功能，就只手工合并自己那段，再跑 --refresh 把合并结果前滚进载荷。

重放完请跑窄口径门禁：
    cd <target>/server/work/dfo-lan
    go build ./... && go vet ./... && go test -count=1 ./cmd/... ./internal/...
"""
import argparse
import io
import os
import shutil
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
PAYLOAD = os.path.join(HERE, "skin-family-20260928")
MODULE = "server/work/dfo-lan"
LINE_END = chr(10)

# 本任务独占的文件：上游从来没有它们，别人的改动不会落在里面。
FILES = [
    "cmd/wireprobe/skin_family_flow.go",
    "cmd/wireprobe/skin_family_flow_test.go",
    "cmd/wireprobe/skin_selection_flow.go",
    "cmd/wireprobe/skin_storage_flow.go",
    "cmd/wireprobe/skin_storage_flow_test.go",
    "cmd/wireprobe/entry_flow.go",
    "cmd/wireprobe/main.go",
    "internal/game/protocol/skin_cargo.go",
    "internal/game/protocol/skin_cargo_test.go",
    "internal/catalog/skin_storage.go",
    "internal/storage/skin_selection_list.go",
    "internal/storage/skin_selection_list_test.go",
    "configs/skin-storage-items.json",
    "docs/protocol/skin-cargo-scaffolding-20260926.md",
]

# 启动器在 03:18 那次启动前把这几份本机配置移进了 runtime/disabled-configs，
# 于是命令行里 -booster-catalog / -shop-vault 等参数消失，物品索引不再被补全，
# 服务端在 attunement 校验处 log.Fatal 退出。这里只把「configs 里没有、隔离区里有」
# 的那份搬回去；两边都有时不动，免得覆盖别人的选择。
LOCAL_CONFIGS = [
    "items.index.json",
    "booster-catalog.json",
    "shop-vault-release.json",
    "shop-purchase-pilot.json",
]
QUARANTINE = "runtime/disabled-configs"


def restore_configs(root, dry):
    moved = 0
    for name in LOCAL_CONFIGS:
        live = os.path.join(root, "configs", name)
        parked = os.path.join(root, *QUARANTINE.split("/"), name)
        if os.path.exists(live):
            print("  配置就位  configs/" + name)
        elif not os.path.exists(parked):
            print("  配置缺失  configs/" + name + "（隔离区也没有，需重新导出）")
            continue
        else:
            print(("  将搬回  " if dry else "  搬回    ") + "configs/" + name)
            if not dry:
                shutil.move(parked, live)
            moved += 1
    return moved


# 更新器留在 runtime/update-backup 下的整文件副本带 go:embed，副本树里没有可嵌资源，
# 于是 go build ./... 常年红灯。放一枚 module 边界标记让 Go 工具整棵跳过该子树。
BACKUP_MARKER = "runtime/update-backup/go.mod"
BACKUP_MARKER_BODY = "".join(line + LINE_END for line in [
    "// Update backups are whole-file copies of this module's sources, taken by the",
    "// runtime updater. Their embed patterns are relative to the copy's own tree,",
    "// which never holds the embedded assets, so compiling them in place fails.",
    "// This marker makes the directory a nested module boundary, which is how the",
    "// Go tool is told to skip a subtree. Nothing here is ever built or imported.",
    "module dfolan/runtime/update-backup",
    "",
    "go 1.26.0",
])


def read(path):
    with io.open(path, encoding="utf-8", newline="") as handle:
        return handle.read()


def key(text):
    """按“去掉所有空白后的行”做比较：同步器会把别人的行退回到未格式化的缩进，
    那不是新增内容，不该判成冲突。"""
    return [line.strip() for line in text.splitlines() if line.strip()]


def classify(src, dst):
    want = set(key(read(src)))
    if not os.path.exists(dst):
        return "missing", []
    extra = [line for line in key(read(dst)) if line not in want]
    if extra:
        return "conflict", extra
    if read(dst) == read(src):
        return "applied", []
    return "regression", []



def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--target", default="D:/115us", help="工作区根目录")
    ap.add_argument("--dry", action="store_true", help="只报告，不写盘")
    ap.add_argument("--refresh", action="store_true", help="用目标树刷新载荷")
    ap.add_argument("--configs-only", action="store_true",
                    help="只把被隔离区搬回 runtime/disabled-configs 的本机配置放回 configs，不动任何源码")
    ap.add_argument("--force", action="store_true",
                    help="把冲突文件也按载荷覆盖（仅在逐条核对过、确认目标多出的行是被取代的旧版时用）")
    args = ap.parse_args()

    root = os.path.join(os.path.normpath(args.target), *MODULE.split("/"))
    if not os.path.isdir(root):
        print("找不到模块目录: " + root)
        return 2

    if args.configs_only:
        moved = restore_configs(root, args.dry)
        print(("预演：可搬回 " if args.dry else "已搬回 ") + str(moved) + " 份本机配置")
        return 0

    if args.refresh:
        for rel in FILES:
            src = os.path.join(root, *rel.split("/"))
            dst = os.path.join(PAYLOAD, *MODULE.split("/"), *rel.split("/"))
            if not os.path.exists(src):
                print("跳过（目标没有该文件）: " + rel)
                continue
            os.makedirs(os.path.dirname(dst), exist_ok=True)
            shutil.copyfile(src, dst)
            print("载荷已刷新: " + rel)
        return 0

    changed = 0
    blocked = 0
    for rel in FILES:
        src = os.path.join(PAYLOAD, *MODULE.split("/"), *rel.split("/"))
        dst = os.path.join(root, *rel.split("/"))
        if not os.path.exists(src):
            print("载荷缺失，先跑 --refresh: " + rel)
            blocked += 1
            continue
        state, extra = classify(src, dst)
        if state == "applied":
            print("  已就位  " + rel)
        elif state == "missing":
            print(("  将新建  " if args.dry else "  新建    ") + rel)
            changed += 1
        elif state == "regression":
            print(("  将回写  " if args.dry else "  回写    ") + rel)
            changed += 1
        else:
            tag = "冲突（已按 --force 覆盖）" if args.force else "冲突"
            print(("  %s  " % tag) + rel + "（目标有载荷里没有的行）")
            for line in extra[:8]:
                print("            | " + line[:96])
            if args.force:
                changed += 1
            else:
                blocked += 1
        write = state in ("missing", "regression") or (args.force and state == "conflict")
        if write and not args.dry:
            os.makedirs(os.path.dirname(dst), exist_ok=True)
            shutil.copyfile(src, dst)

    marker = os.path.join(root, *BACKUP_MARKER.split("/"))
    if os.path.isdir(os.path.dirname(marker)) and not os.path.exists(marker):
        print(("  将写入  " if args.dry else "  写入    ") + BACKUP_MARKER)
        if not args.dry:
            with io.open(marker, "w", encoding="utf-8", newline=LINE_END) as handle:
                handle.write(BACKUP_MARKER_BODY)
        changed += 1
    else:
        print("  已就位  " + BACKUP_MARKER)

    configs_moved = restore_configs(root, args.dry)

    print("")
    if args.dry:
        print("预演：可回写/新建 %d 个文件，可搬回 %d 份本机配置" % (changed, configs_moved))
    else:
        print("已重放 %d 个文件，已搬回 %d 份本机配置" % (changed, configs_moved))
    if blocked:
        print("有 %d 个文件被判为冲突，未改动 —— 先看上面列出的行是谁的新改动：" % blocked)
        print("  是被本任务取代的旧版 ⇒ 加 --force 按载荷覆盖；")
        print("  是别人的新改动 ⇒ 手工把两边合起来，再跑 --refresh 前滚载荷。")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
