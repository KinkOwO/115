"""One bounded real-client run against a loopback Go wire experiment."""

import json
import os
import pathlib
import re
import struct
import subprocess
import sys
import time

p = pathlib.Path(__file__).parent
project = p.parent / "dfo-lan"
tag = sys.argv[1] if len(sys.argv) > 1 else "channel_01"
interactive = len(sys.argv) > 2 and sys.argv[2] == "interactive"
server_only = len(sys.argv) > 2 and sys.argv[2] == "server-only"
exception_trace = len(sys.argv) > 2 and sys.argv[2] == "exception-trace"
channel_check = len(sys.argv) > 2 and sys.argv[2] == "channel-check"
out = project / "runtime" / tag
out.mkdir(parents=True, exist_ok=True)
# The next35 investigation inherits the verified34 channel/login profile.
# Keep its output tag distinct and select its candidate binary explicitly.
candidate37 = tag.endswith("_next37")
if candidate37:
 tag = tag[:-7] + "_next36"
candidate36 = tag.endswith("_next36")
if candidate36:
 tag = tag[:-7] + "_next35"
candidate35 = tag.endswith("_next35")
if candidate35:
 tag = tag[:-7] + "_next34"


def varint(v):
 b = bytearray()
 while v >= 128:
  b.append(v & 127 | 128)
  v >>= 7
 b.append(v)
 return bytes(b)


def integer(n, v):
 return varint(n << 3) + varint(v)


def string(n, s):
 return varint(n << 3 | 2) + varint(len(s)) + s


# Fields are from the exact native PB parser: required status=2, key bytes=3.
# Remaining optional fields are left at protobuf defaults for this transport gate.
keys = bytes((i % 127) + 1 for i in range(1024))
pb = integer(2, 1) + string(3, keys) + string(4, b"LAN Local")
plain = struct.pack("<I", len(pb)) + pb
cipher = bytes((((x ^ 0xB5) >> 6) | ((x ^ 0xB5) << 2)) & 255 for x in plain)
table = []
for i in range(256):
 v = i
 for _ in range(8):
  v = (v >> 1) ^ (0x4DB89129 if v & 1 else 0)
 table.append(v)
crc = 0xFFFFFFFF
for x in cipher:
 crc = (crc >> 8) ^ table[(crc ^ x) & 255]
crc ^= 0xFFFFFFFF
fold = (
 (crc & 255) ^ ((crc >> 8) & 255) ^ ((crc >> 16) & 255) ^ ((crc >> 24) & 255) ^ 0x18
)
header = struct.pack("<B H I I I B", 0, 1, 16 + len(cipher), 0, fold, 0)
fixture = out / "channelinfo.bin"
fixture.write_bytes(header + cipher)
(out / "fixture.json").write_text(
 json.dumps(
  {
   "candidate": "channelinfo transport gate",
   "crc": "native polynomial 0x4db89129; folded low byte; measured in channel_02",
   "pb_hex": pb.hex(),
   "plain_hex": plain.hex(),
  },
  indent=2,
 )
)
bps = out / "breakpoints.txt"
bps.write_text(
 "6d76a47 PACKET_CRC\n52c7e28 CHANNEL_PB_RESULT\n6ca9740 SESSION_KEYS\n6d77447 SEND_RAW\n6ca9660 SEND_PLAIN\n59a1bb0 DISPATCH_PACKET\n5250350 NAME_CHECK_RESULT\n"
)
with bps.open("a") as f:
 f.write("5255c70 PRECHECK_RESULT\n52543d0 LOGIN_RESULT\n")
# 登录频道类型固定用 22（不再按 DFO_ODYSSEY_MODE 切换）：类型 22 是原生客户端的
# "自动入口"类型（旧 0/2/3 被拒，见 docs/protocol/next34-channel-login.md），且在它
# 底下客户端才会给出"奥德赛模式 / 剧情模式"双卡创建界面 —— 正是一个进程同时服务两种
# 角色所需的形态。模式该由角色存档决定（internal/character/odyssey.go），而非启动参数。
login_22 = project / "configs/login-normal22.bin"
login_normal = project / "runtime/login_ok.bin"
if login_22.exists():
 default_login_bin = login_22.resolve()
else:
 default_login_bin = login_normal.resolve()
responses = out / "responses.json"
responses.write_text(
 json.dumps({"1554": str((project / "runtime/precheck_ok.bin").resolve())})
)
if tag.startswith("login_"):
 responses.write_text(
  json.dumps(
   {
    "1554": str((project / "runtime/precheck_ok.bin").resolve()),
    "1": str(default_login_bin),
   }
  )
 )
 with bps.open("a") as f:
  f.write("52553b2 LOGIN_FIELDS_DONE\n5255a1f LOGIN_TAIL\n")
if tag.startswith("roles_"):
 responses.write_text(
  json.dumps(
   {
    str(k): (
     str(default_login_bin)
     if v == "login"
     else str((project / ("runtime/" + v + "_ok.bin")).resolve())
    )
    for k, v in [(1554, "precheck"), (1, "login"), (8, "characters"), (684, "name")]
   }
  )
 )
 with bps.open("a") as f:
  f.write(
   "5637a20 USERINFO_HANDLER\n5637dd8 USERINFO_ROWS_DONE\n5638781 USERINFO_TAIL\n5250d40 CREATE_RESULT\n"
  )
 if tag.startswith("roles_row"):
  if responses.exists():
   try:
    mapping = json.loads(responses.read_text())
    mapping["8"] = str((project / "runtime/characters_row_ok.bin").resolve())
    responses.write_text(json.dumps(mapping))
   except Exception:
    pass
  with bps.open("a") as f:
   f.write(
    "563e280 CHARACTER_ROW_BEGIN\n563ead2 CHARACTER_ROW_FIELDS_DONE\n563ec14 CHARACTER_ROW_TAIL\n"
   )
flags = subprocess.CREATE_NO_WINDOW


def exe_flags(exe):
 """列出服务端程序认识的参数名（解析 `exe -h` 打印的 usage）。

 Go 的 flag 包遇到 -h 就把 usage 打到 stderr 并以非零码退出，所有历史二进制都保留
 这个行为，所以能拿它判断"这个程序认不认某个参数"。探测失败（文件不在、超时、
 不是 Go 程序）返回 None，调用方据此保持原样，不改变既有行为。
 """
 try:
  probe = subprocess.run(
   [exe, "-h"],
   stdout=subprocess.PIPE,
   stderr=subprocess.STDOUT,
   creationflags=flags,
   timeout=15,
  )
 except (OSError, subprocess.SubprocessError):
  return None
 names = set()
 for line in probe.stdout.decode("utf-8", "replace").splitlines():
  match = re.match(r"^\s+-([A-Za-z0-9._-]+)", line)
  if match:
   names.add(match.group(1))
 return names or None


def prune_unsupported(command):
 """丢弃当前服务端程序不认识的参数（连同它的值），避免 flag 解析直接退出。

 2026-09-20 事故：b6d97c8 给源码加了 -booster-catalog / -item-index，本文件按
 "configs 里存在对应文件" 就追加参数，但归档基准的 exe 还是更早的构建；Go 的 flag
 包遇到未定义参数会打印 usage 并以非零码退出，玩家看到的是 "启动脚本异常退出:
 exit status 1"。下发前按 exe 自报的能力过滤一遍，旧程序也能照常起来（功能按旧版）。

 只在自身构造的命令串上工作，形状固定为 [-flag value -flag -flag value]；不认识的
 参数如果带值，按"下一个 token 不是参数名"判定并一并丢弃（参数值都是路径，不会以
 '-' 开头）。
 """
 supported = exe_flags(command[0])
 if not supported:
  return command
 kept = [command[0]]
 dropped = []
 index = 1
 while index < len(command):
  token = command[index]
  if not token.startswith("-") or token[1:] in supported:
   kept.append(token)
   index += 1
   continue
  dropped.append(token)
  index += 1
  if index < len(command) and not command[index].startswith("-"):
   dropped.append(command[index])
   index += 1
 if dropped:
  print(
   "WARNING: %s does not define %s; dropped them so this build can still start."
   % (command[0], ", ".join(x for x in dropped if x.startswith("-")))
  )
 return kept


with (
 (out / "gateway.out").open("w") as stdout,
 (out / "gateway.err").open("w") as stderr,
):
 persisted = tag.startswith("roles_persist")
 command = [
  str(project / ("bin/wireprobe-character.exe" if persisted else "bin/wireprobe.exe")),
  "-fixture",
  str(fixture.resolve()),
  "-output",
  str(out.resolve()),
 ]
 if persisted:
  cr_odyssey = project / "configs/character-rules.odyssey-release.json"
  cr_jobs = project / "configs/character-rules.jobs-release.json"
  cr_probe = project / "configs/character-probe.json"
  # 统一用带 odyssey_pilot 的那份：它允许创建界面同时提供两种模式（角色各自记下模式），
  # 而 jobs-release 缺这个字段，奥德赛角色在建号这一步就做不出来。
  if cr_odyssey.exists():
   cr_default = cr_odyssey
  elif cr_jobs.exists():
   cr_default = cr_jobs
  else:
   cr_default = cr_probe
  cat_skycastle = project / "configs/characters.skycastle-release.json"
  cat_generated = project / "configs/characters.generated.json"
  cat_default = cat_skycastle if cat_skycastle.exists() else cat_generated
  command += [
   "-character-storage",
   str(project / "runtime/storage/local.json"),
   "-character-catalog",
   str(cat_default),
   "-character-rules",
   str(cr_default),
  ]
 if tag.startswith("roles_persist_select"):
  command += ["-select-probe-config", str(project / "configs/select-parser-probe.json")]
  with bps.open("a") as f:
   f.write(
    "525a120 SELECT_RESULT\n525a2e3 SELECT_FIELDS_BEGIN\n525b409 SELECT_LAST_COUNT\n525b4c4 SELECT_FIELDS_DONE\n"
   )
 if tag.startswith("roles_persist_select_actor"):
  command += ["-entry-basic-probe"]
  with bps.open("a") as f:
   f.write(
    "563ec60 ENTRY_BASIC_BEGIN\n5641000 ENTRY_BASIC_DONE\n563d400 ENTRY_ADDITION_BEGIN\n"
   )
 if tag.startswith("roles_persist_select_actor_town"):
  command += [
   "-town-catalog",
   str(project / "configs/town.generated.json"),
   "-town-entry-probe",
   str(project / "configs/town-entry-probe.json"),
  ]
  with bps.open("a") as f:
   f.write("52fc5b0 AREA_USERS_BEGIN\n52fcf01 AREA_MAP_LOAD\n52fd9e3 AREA_USERS_DONE\n")
 if tag not in ["channel_01", "channel_02", "channel_03"]:
  command += ["-responses", str(responses.resolve())]
 if tag.startswith("roles_persist_select_actor_town_world_live"):
  command[command.index("-select-probe-config") + 1] = str(
   project / "configs/select-world-probe.json"
  )
  command += [
   "-world-catalog",
   str(project / "configs/world.generated.json"),
   "-world-rules",
   str(project / "configs/world-probe.json"),
   "-quest-catalog",
   str(project / "configs/quests.generated.json"),
   "-vault-rules",
   str(project / "configs/vault.generated.json"),
  ]
  if "_detail_" in tag:
   command += [
    "-entry-addition-probe",
    "-fatigue-rules",
    str(project / "configs/fatigue-probe.json"),
   ]
  if "_dungeon_" in tag:
   command += ["-dungeon-catalog", str(project / "configs/dungeons.full.json")]
 if tag.endswith(
  (
   "_next26",
   "_next27",
   "_next28",
   "_next29",
   "_next30",
   "_next31",
   "_next32",
   "_next33",
   "_next34",
  )
 ):
  if not persisted or "_dungeon_" not in tag:
   raise ValueError("next26 requires complete dungeon profile")
  command[0] = str(project / "bin/wireprobe-dungeon26.exe")
  # next25 代目录导出于 growtype 分段解析之前，没有 advancement_growth /
  # advancement_skills。用它时，建号请求 option[8] 选定的转职槽位无处落账，
  # 角色停在 advancement 0：全局按基础职业渲染、技能面板没有该分支起始技能。
  # skycastle-release 是同一份 PVF 快照的导出，17 个职业除这 4 个新增数据块
  # 外逐字段一致（checksum 与 raw_sha256 相同，已有存档无需迁移），只是补上缺失块。
  # 技能目录保持 next27：它已定义全部转职分支技能 ID，且 [required level] 同值。
  command[command.index("-character-catalog") + 1] = str(
   project / "configs/characters.skycastle-release.json"
  )
  command += [
   "-progression-catalog",
   str(project / "configs/progression.next25.json"),
   "-progression-rules",
   str(project / "configs/experience.compat90.json"),
   "-loot-catalog",
   str(project / "configs/loot.next25.json"),
   "-loot-rules",
   str(project / "configs/drop.compat90.json"),
   "-bag-rules",
   str(project / "configs/inventory.compat90.json"),
   "-card-rules",
   str(project / "configs/cards.compat90.json"),
  ]
  if tag.endswith(
   (
    "_next27",
    "_next28",
    "_next29",
    "_next30",
    "_next31",
    "_next32",
    "_next33",
    "_next34",
   )
  ):
   command[0] = str(project / "bin/wireprobe-dungeon27.exe")
   command += ["-skill-catalog", str(project / "configs/skills.next27.json")]
  if tag.endswith(
   ("_next28", "_next29", "_next30", "_next31", "_next32", "_next33", "_next34")
  ):
   command[0] = str(project / "bin/wireprobe-dungeon28.exe")
   command += ["-channel-refresh-config", str(project / "configs/channel.local28.json")]
   command[command.index("-dungeon-catalog") + 1] = str(
    project / "configs/dungeons.full.json"
   )
  if tag.endswith(("_next29", "_next30", "_next31", "_next32", "_next33", "_next34")):
   command[0] = str(project / "bin/wireprobe-dungeon29.exe")
   command[command.index("-bag-rules") + 1] = str(
    project / "configs/inventory.next29.json"
   )
   command += [
    "-quest-equipment-catalog",
    str(project / "configs/quest-equipment.next29.json"),
   ]
  if tag.endswith("_next30"):
   command[0] = str(project / "bin/wireprobe-dungeon30.exe")
  if tag.endswith("_next31"):
   command[0] = str(project / "bin/wireprobe-dungeon31.exe")
   command[command.index("-channel-refresh-config") + 1] = str(
    project / "configs/channel.local31.json"
   )
  if tag.endswith("_next32"):
   command[0] = str(project / "bin/wireprobe-dungeon32.exe")
   command[command.index("-channel-refresh-config") + 1] = str(
    project / "configs/channel.local32.json"
   )
  if tag.endswith("_next33"):
   command[0] = str(project / "bin/wireprobe-dungeon33.exe")
   command[command.index("-channel-refresh-config") + 1] = str(
    project / "configs/channel.local32.json"
   )
   command += ["-game-listen", "127.0.0.2:0"]
  if tag.endswith("_next34"):
   command[0] = str(project / "bin/wireprobe-dungeon33.exe")
   command[command.index("-channel-refresh-config") + 1] = str(
    project / "configs/channel.local34.json"
   )
   command += ["-game-listen", "127.0.0.2:0"]
 if candidate35:
  command[0] = str(project / "bin/wireprobe-dungeon35.exe")
  command[command.index("-quest-equipment-catalog") + 1] = str(
   project / "configs/equipment.current37.json"
  )
  command += [
   "-equipment-wear-rules",
   str(project / "configs/equipment-wear.current35.json"),
   "-account-options",
   str(project / "configs/account-options.current35.json"),
  ]
 if candidate36:
  # 36 keeps the verified34 channel/login profile and the35 equipment and
  # account-option wiring, then adds gameplay closure: equipment drops, the
  # per-job starting route, and the single-actor party roster the overhead
  # gold number is read from.
  command[0] = str(project / "bin/wireprobe-dungeon36.exe")
  command[command.index("-loot-rules") + 1] = str(
   project / "configs/drop.current36.json"
  )
  command += [
   "-tutorial-routes",
   str(project / "configs/tutorial-routes.current35.json"),
   "-tutorial-dungeons",
   str(project / "configs/tutorial-dungeons.current36.json"),
   "-solo-party-bootstrap",
  ]
 if candidate37:
  # 37 keeps every 36 flag, swaps the binary, and widens the equipment
  # catalog. Parameterless commands (the escape menu) are no longer discarded
  # before verification, a death the boss chain reports with killerFFFF is
  # confirmed so the room clears instead of trapping the character, and the
  # catalog now carries every template a quest can hand out - 1638 of them
  # were missing, which is why quest 21650 could never be handed in.
  # 39 is the current stable build. It keeps 37's safe (empty) userinfo
  # appearance block and refreshes worn gear through the NOTI14 equipment-space
  # path a live equip uses, sent after town entry. CONFIRMED in live play
  # 20260912: equipped gear renders on body + paper-doll and persists across
  # re-entry, no crash. (Do NOT use 38 - its userinfo-appearance block over-reads
  # and access-violates the client; that whole approach is abandoned.)
  command[0] = str(project / "bin/wireprobe-dungeon39.exe")
  command[command.index("-quest-equipment-catalog") + 1] = str(
   project / "configs/equipment.current37.json"
  )
  # The bag policy gains the quick-use belt (slots 0..8, the gap below the
  # equipment range) so a consumable can be dragged onto the hotkey bar.
  command[command.index("-bag-rules") + 1] = str(
   project / "configs/inventory.current37.json"
  )
  if (project / "configs/items.index.json").exists():
   command += ["-item-index", str(project / "configs/items.index.json")]
  if (project / "configs/booster-catalog.json").exists():
   command += ["-booster-catalog", str(project / "configs/booster-catalog.json")]
  # Pick-a-item boxes ([booster select category]) come from their own table. The
  # gateway starts with the project root as cwd, so the built-in relative paths
  # never resolve — pass the absolute catalog like every other config.
  selection_boxes = project / "configs/selection-boxes-candidate.json"
  if selection_boxes.exists():
   command += ["-selection-boxes", str(selection_boxes)]
  # Source item shops ([need material] prices — the Odyssey shop charges silver
  # coins). Without it the gateway charges a flat 1 gold for everything.
  item_shop = project / "configs/itemshop-candidate.json"
  if item_shop.exists():
   command += ["-item-shop", str(item_shop)]
  # The compiled apocalypse.ctp table (legion / apocalypse): the phase clock,
  # the four operation blocks, gate schedule, coin flag, rewards and duty
  # skills. Same cwd rule as the catalogs above - the gateway runs with the
  # project root as cwd, so the built-in relative default never resolves.
  # Deliberately NOT behind .exists(): when the file is missing the server
  # then logs the absolute path it tried, which separates a cwd problem from
  # a missing-file problem. Live run 20260923_163225 hit exactly this - the
  # relative default failed and every legion confirmation went unvalidated.
  command += [
   "-apocalypse-catalog",
   str(project / "configs/apocalypse.generated.json"),
  ]
  # Magic-seal unsealing (CMD393) rolls from the current random option rules;
  # the default relative path never resolves because the gateway's cwd is the
  # project root, so pass the absolute catalog like every other config.
  if (project / "configs/randomoption.current37.json").exists():
   command += [
    "-random-option-catalog",
    str(project / "configs/randomoption.current37.json"),
   ]
  shop_release = project / "configs/shop-vault-release.json"
  shop_pilot = project / "configs/shop-purchase-pilot.json"
  if os.environ.get("DFO_SHOP_PURCHASE_PILOT"):
   shop_override = pathlib.Path(os.environ["DFO_SHOP_PURCHASE_PILOT"])
   if not shop_override.is_absolute():
    shop_override = (project / shop_override).resolve()
   command += ["-shop-purchase-pilot", str(shop_override), "-shop-release"]
  elif shop_release.exists():
   command += ["-shop-purchase-pilot", str(shop_release), "-shop-release"]
  elif shop_pilot.exists():
   command += ["-shop-purchase-pilot", str(shop_pilot)]
 command[0] = os.environ.get("DFO_SERVER_BINARY", command[0])
 for flag, key in (
  ("-character-storage", "DFO_CHARACTER_STORAGE"),
  ("-character-catalog", "DFO_CHARACTER_CATALOG"),
  ("-character-rules", "DFO_CHARACTER_RULES"),
 ):
  if key in os.environ:
   command[command.index(flag) + 1] = os.environ[key]
 if os.environ.get("DFO_LOGIN_RESPONSE"):
  if responses.exists():
   try:
    override_path = pathlib.Path(os.environ["DFO_LOGIN_RESPONSE"])
    if not override_path.is_absolute():
     override_path = (project / override_path).resolve()
    if override_path.exists():
     mapping = json.loads(responses.read_text())
     mapping["1"] = str(override_path)
     responses.write_text(json.dumps(mapping))
   except Exception:
    pass
 # 角色目录必须带 growtype 分段数据（advancement_growth / advancement_skills）。
 # next25 那代导出于 growtype 解析之前：用它时建号请求 option[8] 选定的转职槽位
 # 无处落账，角色停在 advancement 0（全局按基础职业渲染、技能面板缺该分支起始技能），
 # 而且全程没有报错 —— 2026-09-20 的"新角色不转职"就是这么来的。这里显式告警。
 if "-character-catalog" in command:
  _catalog = pathlib.Path(command[command.index("-character-catalog") + 1])
  if not _catalog.is_file():
   print("WARNING: 角色目录不存在：%s" % _catalog, file=sys.stderr)
  else:
   try:
    _professions = json.loads(_catalog.read_text(encoding="utf-8-sig")).get("professions") or {}
    if not any((p or {}).get("advancement_growth") for p in _professions.values()):
     print(
      "WARNING: 角色目录 %s 不含 growtype 分段数据（advancement_growth/advancement_skills）："
      "建号选定的转职分支不会落账，角色会停在基础职业。"
      "请改用带该数据的目录（例如 configs/characters.skycastle-release.json）。" % _catalog,
      file=sys.stderr,
     )
   except Exception as exc:
    print("WARNING: 无法解析角色目录 %s：%s" % (_catalog, exc), file=sys.stderr)
 # 允许用环境变量覆盖副本目录（本机用 dungeons.full.json：3200 副本/16042 地图，
 # 而默认的 dungeons.generated.json 只有 11 个）。
 if os.environ.get("DFO_DUNGEON_CATALOG") and "-dungeon-catalog" in command:
  command[command.index("-dungeon-catalog") + 1] = os.environ["DFO_DUNGEON_CATALOG"]
 # ★ 显式校验：副本目录必须真实存在且非空。
 # 2026-09-18 事故：这里曾被指向 configs/dungeons.full.json，而那个 294MB 文件从没进仓库，
 # 于是服务端起不来、探针一直等不到 ready.json —— 正常玩家表现为"下载后启动不了游戏"。
 # 不要再让"文件不存在"静默落回默认值（那条 tag 降级链最终是只有 11 个副本的 dungeons.generated.json）。
 if "-dungeon-catalog" in command:
  dungeon_catalog = pathlib.Path(command[command.index("-dungeon-catalog") + 1])
  if not dungeon_catalog.is_file() or dungeon_catalog.stat().st_size == 0:
   raise RuntimeError(
    "副本目录不存在或为空：%s\n"
    "  当前 -dungeon-catalog 指向它，服务端会启动失败/超时。\n"
    "  生成：go run ./cmd/dungeonfull -output %s\n"
    "  或设 DFO_DUNGEON_CATALOG 指向已有目录；确实要用 11 个副本的默认表请显式指过去。"
    % (dungeon_catalog, dungeon_catalog)
   )
 # 奥德赛组件常驻挂载（不再看 DFO_ODYSSEY_MODE）：服务端只在环境变量存在时才挂载
 # 成长/货币/武器盒（cmd/wireprobe/main.go:369/455/579），而"按角色"要求同一进程同时
 # 服务两种角色 —— 少了这些，奥德赛角色进城后没有成长/货币/武器盒。挂载本身对所有
 # 角色无害：是否真的生效由服务端按角色判定（internal/character/odyssey.go 的 OdysseyRole）。
 coin_rules = project / "configs/odyssey-currency.json"
 if os.environ.get("DFO_ODYSSEY_COIN_RULES"):
  coin_override = pathlib.Path(os.environ["DFO_ODYSSEY_COIN_RULES"])
  if not coin_override.is_absolute():
   coin_override = (project / coin_override).resolve()
  if coin_override.exists():
   os.environ["DFO_ODYSSEY_COIN_RULES"] = str(coin_override)
 elif coin_rules.exists():
  os.environ["DFO_ODYSSEY_COIN_RULES"] = str(coin_rules.resolve())

 weapon_box = project / "configs/odyssey-weapon-box-release.json"
 if os.environ.get("DFO_ODYSSEY_WEAPON_BOX"):
  box_override = pathlib.Path(os.environ["DFO_ODYSSEY_WEAPON_BOX"])
  if not box_override.is_absolute():
   box_override = (project / box_override).resolve()
  if box_override.exists():
   os.environ["DFO_ODYSSEY_WEAPON_BOX"] = str(box_override)
   os.environ["DFO_ODYSSEY_REWARDS_RELEASE"] = "1"
 elif weapon_box.exists():
  os.environ["DFO_ODYSSEY_WEAPON_BOX"] = str(weapon_box.resolve())
  os.environ["DFO_ODYSSEY_REWARDS_RELEASE"] = "1"

 odyssey_growth = project / "configs/odyssey-growth-release.json"
 if os.environ.get("DFO_ODYSSEY_GROWTH"):
  growth_override = pathlib.Path(os.environ["DFO_ODYSSEY_GROWTH"])
  if not growth_override.is_absolute():
   growth_override = (project / growth_override).resolve()
  if growth_override.exists():
   os.environ["DFO_ODYSSEY_GROWTH"] = str(growth_override)
 elif odyssey_growth.exists():
  os.environ["DFO_ODYSSEY_GROWTH"] = str(odyssey_growth.resolve())

 # 七章目录（章节奖励按进度补发）。章节盒掉落表**不**在这里注入：那一项出厂
 # enabled=false，按手册要求由 profile 显式开启。
 odyssey_chapters = project / "configs/odyssey-chapters-release.json"
 if os.environ.get("DFO_ODYSSEY_CHAPTERS"):
  chapters_override = pathlib.Path(os.environ["DFO_ODYSSEY_CHAPTERS"])
  if not chapters_override.is_absolute():
   chapters_override = (project / chapters_override).resolve()
  if chapters_override.exists():
   os.environ["DFO_ODYSSEY_CHAPTERS"] = str(chapters_override)
 elif odyssey_chapters.exists():
  os.environ["DFO_ODYSSEY_CHAPTERS"] = str(odyssey_chapters.resolve())

 # 章节装备盒掉落（手册 P3 子项 3）。整表出厂 enabled=false，服务端只在环境变量存在
 # 时才挂载；此前没有任何入口注入它（probe 不注入、repair profile 也不认这个键），
 # 于是「章节最终领主掉装备盒」这条链永远是死的。现在 1/3/4/5/6 章已在 release 表
 # 开启，这里按与金币表相同的规则常驻挂载（2 章盒子 10419742 不在选择盒目录里、
 # 7 章手册没给装备盒，这两行保持关闭，开启会让网关启动即退出）。
 chapter_drop = project / "configs/odyssey-chapter-drop-release.json"
 if os.environ.get("DFO_ODYSSEY_CHAPTER_DROP"):
  drop_override = pathlib.Path(os.environ["DFO_ODYSSEY_CHAPTER_DROP"])
  if not drop_override.is_absolute():
   drop_override = (project / drop_override).resolve()
  if drop_override.exists():
   os.environ["DFO_ODYSSEY_CHAPTER_DROP"] = str(drop_override)
 elif chapter_drop.exists():
  os.environ["DFO_ODYSSEY_CHAPTER_DROP"] = str(chapter_drop.resolve())

 eq_full = project / "configs/equipment-full"
 if (project / "configs/equipment-full.index.json").exists() and (
  project / "configs/equipment-full.data"
 ).exists():
  os.environ["DFO_EQUIPMENT_FULL_CATALOG"] = str(eq_full.resolve())
  eq_wear_full = project / "configs/equipment-wear.full-candidate.json"
  if eq_wear_full.exists():
   os.environ["DFO_EQUIPMENT_WEAR_RULES"] = str(eq_wear_full.resolve())
 # ★ 下发前按当前服务端程序自报的能力过滤参数（见 prune_unsupported）。
 command = prune_unsupported(command)
 stdout.write(' '.join(command) + '\n')
 server = subprocess.Popen(command, stdout=stdout, stderr=stderr, creationflags=flags)
 try:
  ready = out / "ready.json"
  for _ in range(1000):
   if server.poll() is not None:
    raise RuntimeError(f"gateway exited on {command}")
   if ready.exists():
    break
   time.sleep(0.05)
  state = json.loads(ready.read_text())
  port = int(state["address"].split(":")[-1])
  if server_only:
   (out / "run.json").write_text(json.dumps({"server_pid": server.pid, "port": port}))
   print(f"Server listening on 127.0.0.1:7001 and {state['address']}")
   try:
    server.wait()
   except KeyboardInterrupt:
    pass
   sys.exit(0)
  payload = f"13?127.0.0.1?{port}?probe?00000000000000000000000000000000?0?0?30?0?0?0"
  if channel_check or tag.endswith(
   ("_next30", "_next31", "_next32", "_next33", "_next34")
  ):
   payload = "3?127.0.0.1?7001?probe?00000000000000000000000000000000?0?0?30?0?0?0"
  # probe.exe 依据这个目录判断"客户端是否存在"。先把 Python 侧的可见性打出来：
  # 万一 probe 在写 client.log 之前就退出（见下面的返回码 3），也能立刻分清是目录问题还是被拦。
  client_dir = os.environ.get("DFO_CLIENT_DIR", str(p.parent / "dfo_probe_client"))
  if not (pathlib.Path(client_dir) / "DFO.exe").is_file():
   print(f"WARNING: probe cannot see DFO.exe under client dir: {client_dir}")
  stdout.write(' '.join([
    str(p / "probe.exe"),
    client_dir,
    str(out / "client.log"),
    "55",
    "normal-ui"
    if channel_check
    else (
     "trace-owned-ui"
     if exception_trace
     else ("interactive-ui" if interactive else "trace-root-ui")
    ),
    str(bps),
    payload,
   ]) + '\n')
  probe = subprocess.Popen(
   [
    str(p / "probe.exe"),
    client_dir,
    str(out / "client.log"),
    "55",
    "normal-ui"
    if channel_check
    else (
     "trace-owned-ui"
     if exception_trace
     else ("interactive-ui" if interactive else "trace-root-ui")
    ),
    str(bps),
    payload,
   ],
   creationflags=flags,
  )
  (out / "run.json").write_text(
   json.dumps({"server_pid": server.pid, "probe_pid": probe.pid, "port": port})
  )
  if (
   os.environ.get("DFO_ENABLE_OBSERVER") == "1"
   and tag.endswith(("_next30", "_next31", "_next32", "_next33", "_next34"))
   and interactive
  ):
   with (
    (out / "observer.out").open("wb") as obsout,
    (out / "observer.err").open("wb") as obserr,
   ):
    # 36's observer adds the actor+0x6800 window: 145c08420 copies
    # actor+0x6808 into the HP descriptor's rate field, and 30's window
    # stopped short of it, so the source of the scale was never recorded.
    observer = "watch_monster_stats36.py" if candidate37 else "watch_monster_stats30.py"
    subprocess.Popen(
     [sys.executable, str(p / observer), str(out)],
     stdout=obsout,
     stderr=obserr,
     creationflags=flags,
    )
  probe_code = probe.wait(timeout=None if (interactive or exception_trace) else 65)
  # probe 的退出码是"客户端到底有没有被拉起"的第一手证据：
  # 它若判断 <client_dir>\DFO.exe 不存在就直接返回 3，而这一步发生在打开 client.log 之前 ——
  # 所以这种失败**不会留下 client.log**，只表现为"秒退 + 零日志"。
  (out / "probe.json").write_text(
   json.dumps(
    {
     "probe_pid": probe.pid,
     "probe_returncode": probe_code,
     "client_dir": client_dir,
     "payload": payload,
    }
   )
  )
  print(f"probe.exe exited with code {probe_code}")
  if probe_code:
   print("WARNING: the game client was not launched correctly.")
   print(
    f"  client dir passed to probe: {client_dir}"
    "  (return code 3 = probe could not see DFO.exe there and it exits before writing client.log;"
    " on real machines this usually means security software blocked probe.exe)"
   )
 except Exception as e:
  raise RuntimeError(command) from e
 finally:
  if server.poll() is None:
   server.terminate()
   server.wait(timeout=5)
trace = pathlib.Path.home() / "AppData/LocalLow/DNF/DFO.trc"
if trace.exists():
 text = bytes(((((x >> 6) | (x << 2)) & 255) ^ 118) for x in trace.read_bytes()).decode(
  "utf-8", "replace"
 )
 text = re.sub(r"MAC Address[^\r\n]*", "MAC Address [redacted]", text).replace(
  "\r\r\n", "\n"
 )
 (out / "client_trace.txt").write_text(text, encoding="utf-8")
 print(
  "\n".join(
   x
   for x in text.splitlines()
   if re.search(
    "CHANNELINFO|LOGIN|GET_USERINFO|CREATE_CHARACTER|SELECT_CHARACTER|CHECK_CHARACTER_NAME|Checksum|decrypt",
    x,
    re.I,
   )
  )[:5000]
 )
print(out.resolve())
