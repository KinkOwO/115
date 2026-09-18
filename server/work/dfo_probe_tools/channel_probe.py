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
responses = out / "responses.json"
responses.write_text(
 json.dumps({"1554": str((project / "runtime/precheck_ok.bin").resolve())})
)
if tag.startswith("login_"):
 responses.write_text(
  json.dumps(
   {
    "1554": str((project / "runtime/precheck_ok.bin").resolve()),
    "1": str((project / "runtime/login_ok.bin").resolve()),
   }
  )
 )
 with bps.open("a") as f:
  f.write("52553b2 LOGIN_FIELDS_DONE\n5255a1f LOGIN_TAIL\n")
if tag.startswith("roles_"):
 responses.write_text(
  json.dumps(
   {
    str(k): str((project / ("runtime/" + v + "_ok.bin")).resolve())
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
  command += [
   "-character-storage",
   str(project / "runtime/storage/local.json"),
   "-character-catalog",
   str(project / "configs/characters.generated.json"),
   "-character-rules",
   str(project / "configs/character-probe.json"),
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
  command[command.index("-character-catalog") + 1] = str(
   project / "configs/characters.next25.json"
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
    mapping = json.loads(responses.read_text())
    mapping["1"] = str(pathlib.Path(os.environ["DFO_LOGIN_RESPONSE"]).resolve())
    responses.write_text(json.dumps(mapping))
   except Exception:
    pass
 # 允许用环境变量覆盖副本目录（本机用 dungeons.full.json：3200 副本/16042 地图，
 # 而默认的 dungeons.generated.json 只有 11 个）。
 if os.environ.get("DFO_DUNGEON_CATALOG") and "-dungeon-catalog" in command:
  command[command.index("-dungeon-catalog") + 1] = os.environ["DFO_DUNGEON_CATALOG"]
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
  probe = subprocess.Popen(
   [
    str(p / "probe.exe"),
    os.environ.get("DFO_CLIENT_DIR", str(p.parent / "dfo_probe_client")),
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
  probe.wait(timeout=None if (interactive or exception_trace) else 65)
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
