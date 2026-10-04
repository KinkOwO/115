# Python 启动编排 → Go 移植规格（2026-10-04 只读取证）

状态：**取证规格**。为 [runtime-without-tools-plan.md](runtime-without-tools-plan.md) 的 **W2（去 Python）** 提供逐项可观察契约。
本文件不含实现，也**未改任何代码**。

> ⚠️ 使用纪律：本文件是**只读逆向**所得，凡涉及**字节级/时序级**的断言（§5 的 `channelinfo.bin` 布局、
> §7 的 `ready.json`、§6 的退出码、§3 的超时数值），Go 实现必须用**现 Python 产物做黄金样本实测对照**后才可下结论
> （根 `AGENTS.md` §0.3 不猜包）。发现不符即改本文件，不要改测试去迁就实现。

图例：**L**=`scripts/launch_local.py`，**P**=`dfo_probe_tools/channel_probe.py`，**E**=`scripts/ensure_inner_pvf.py`，
**PR**=`scripts/prepare_inner_pvf.py`，**RP**=`scripts/repair_profile.py`，**PA**=`scripts/pvf_archive.py`，
**CS**=探针目录内 `catalog_startup.py`，**C**=`dfo_probe_tools/probe.cpp`，**CMD**=`cmd/wireprobe`。
`PROJECT=server/work/dfo-lan`，`ROOT=server/`（L:17-18,22-26），`STORAGE=PROJECT/runtime/storage`。

**两套相对路径基准且彼此不一致（是移植的头号陷阱）**：`L.resolved()` 以 `ROOT=server/` 为基准（L:31-33），
而 `RP.load_profile` 以 `PROJECT=dfo-lan` 为基准（RP:34-40）；`INNER_PVF/INNER_MANIFEST` 又取 `PROJECT.parent`
（L:27-28，= `server/work/client-build/`）。

---

## 1. CLI 面

### `launch_local.py`（L:164-187）

| 参数 | 行为 |
| --- | --- |
| `--check` | 在 Windows 门禁之前返回（L:223-230）。**并非只读**：`_ensure_inner_pvf` 在 L:218-219 先跑，可能重建 760 MB 内层归档 |
| `--storage-only` | profile=None（L:101）；只需 L:200-209 的交互必需项；起库后打印 "Existing storage ready."（L:240-242） |
| `--repair-profile PATH` | 与 `--json-mode` **互斥**（L:169-171） |
| `--json-mode` | 不套默认 profile（L:101）；删除全部 `DFO_PVF_*`（L:150-153）；用 `launcher.local.json` 的 `server_binary` |
| `--server-only` | L:198-200；给探针的模式串 `"server-only"` |
| `--source-build` | 用 `bin/wireprobe-handoff-source.exe`（L:96-99）；**与 `--repair-profile` 同用时被静默忽略**（L:106-111） |
| `--client-only` | 跳过起库（L:238）与内层 PVF（L:127-128）；必需项改为客户端三件套（L:196-197） |

- 默认 profile = `PROJECT/configs/pvf-default.json`（L:21,100-102）。
- 退出：`ERROR: <exc>` 到 stderr + exit 1（L:290-294）。
- 帮助文本提到 `DFO_LAN_HOST`（L:185），但**全仓无任何代码读取它**（只有这一处帮助串）。

### `channel_probe.py`（P:15-19）

- `argv[1]`=tag（默认 `channel_01`）→ `out=PROJECT/runtime/<tag>`（P:20）；`argv[2]`=模式，**精确匹配**：
  `interactive` / `server-only` / `exception-trace` / `channel-check`；都不匹配 ⇒ 全 false ⇒ 探针模式 `trace-root-ui`。
- **tag 降级链**（P:24-32）：`_next37→_next36→_next35→_next34`（仅这四个）。**`out` 在降级前算出**：
  目录名保留 `_next37`，而所有决策看到的是 `_next34`。

---

## 2. 配置输入

| 文件 | 键 | 缺失行为 |
| --- | --- | --- |
| `server/launcher.local.json`（以 ROOT 为基准，`utf-8-sig`，L:44-50） | `client_dir`(L:192)、`server_binary`(L:98，仅无 profile 时)、`channel_identity`(L:189，可选，默认 False) | 缺文件 ⇒ `Copy launcher.example.json to launcher.local.json and set client_dir.`（L:46-49）；缺键 ⇒ 裸 `KeyError`；非布尔 ⇒ `channel_identity must be a JSON boolean`（L:191） |
| `PROJECT/runtime/storage/local.json`（`utf-8-sig`，L:51-56） | `postgres_dsn`（必需）、`postgres_bin`(L:70)、`postgres_data`(L:65) | 缺文件 ⇒ `Storage missing. Follow README first-time setup; no database was changed.`（L:52-55） |
| `PROJECT/configs/pvf-default.json`（经 `RP.load_profile`，L:105） | 顶层键**必须恰好** `{binary, environment}`，否则 `Expected binary and environment fields`（RP:32-33）；未知 env 键 ⇒ `Unknown profile key or invalid value: <key>`（RP:90）；`DFO_PVF_CATALOGS` 白名单必须等于 `gamedata.SupportedDomains`（`internal/gamedata/catalogs.go:136`），否则 `Invalid PVF candidate domains`（RP:79-88） | 返回 `(binary, required, env)`（RP:91）；`required` = binary + 每个 `PATH_KEYS` 路径 + `DFO_EQUIPMENT_FULL_CATALOG{,.data,.index.json}` |

- `max_connections` **两个脚本都不读**，由 Go 读（`internal/database/store.go:18`）。
- profile 把 `DFO_PVF_ARCHIVE=../client-build/Script.inner.pvf`（以 PROJECT 解析）放进 `required`
  ⇒ **这就是 `_ensure_inner_pvf` 必须早于 profile 必需项检查的原因**（L:215-222）。
- 必需项检查：L:195-222 的 helper/probe.exe/catalog_startup.py/binary/客户端三件套 ⇒ `Missing dependency: <p>`；
  profile 路径 ⇒ `Missing repair profile dependency: <p>`。
- 探针按 `.exists()` 条件读取的配置（P:264 等）：`configs/{character-rules.odyssey-release,character-probe,
  characters.skycastle-release,characters.generated,itemshop-candidate,randomoption.current37,odyssey-currency,
  odyssey-weapon-box-release,odyssey-growth-release,odyssey-chapters-release,odyssey-chapter-drop-release,
  equipment-wear.full-candidate}.json` 与 `cmd/wireprobe/testdata/login-normal22.bin`。
  **注**：`configs/town.generated.json` 已于 2026-10-03 删除（`server/AGENTS.md`），但 **P:264 仍在传它**；
  因 `TownPath` 无生产读取者（`catalogs.go:124`，仅 `catalogs_core_test.go:338`）**当前是惰性的**。
- `CS.validate_json_catalogs` 在默认路径上是 **no-op**（characters 已在 `DFO_PVF_CATALOGS`；`-dungeon-catalog` 从不传）——CS:12-29。
- **BOM 不一致**：launcher/storage/profile/characters 用 `utf-8-sig`；`run.json`(L:275) 与 `ready.json`(P:593)
  用**本地默认编码** `read_text()`；manifest 用纯 `utf-8`(E:75)。

---

## 3. 进程与端口编排（顺序）

1 `configuration` L:188（回环校验 L:57-59 → `This development profile requires local loopback storage.`）
2 `gateway_configuration` L:193　3 文件检查 L:212-222　4 `_ensure_inner_pvf` L:218-219（仅当 `profile_env["DFO_PVF_CATALOGS"]`；失败只 **WARNING** L:137-141）
5 `--check` 返回　6 Windows 门禁 L:231-232　7 端口 7001 预检 L:235-236（storage-only/client-only 跳过）　8 `start_storage` L:239
9 建 tag 目录 L:243-249（`mkdir` **无** `exist_ok`）　10 环境 L:250-254
11 **启动 helper**：`Popen([sys.executable, helper, tag, mode], cwd=ROOT, stdin=DEVNULL, stdout=helper.out, stderr=helper.err, CREATE_NO_WINDOW)` L:256-268
12 轮询 `out/run.json` L:271-286：有 `DFO_PVF_CATALOGS` 时 **2100×0.1s=210s**，否则 **300×0.1s=30s**；
    读到即打印返回、**不回收 helper**；`poll()!=None` ⇒ `Startup stopped; inspect <helper.err>`；
    超时 ⇒ `Startup timeout; inspect logs before retry: <out>`。

**PostgreSQL**（L:63-92）：先 `listening()`（0.5s 连接超时，L:36-41）。未监听时：
`data=Path(cfg["postgres_data"]).resolve()`；`data != STORAGE/pgdata` ⇒ `Database offline; external data directories must be started by their owner.`（L:65-69，**外部数据目录拒绝**）；
缺 `PG_VERSION` 或 `pg_ctl.exe` ⇒ `Existing PostgreSQL data or pg_ctl missing.`（L:71-72）；
argv=`[pg_ctl.exe,"-D",data,"-l",STORAGE/postgres.log,"-w","-t","30","start"]`，stdout=stderr=`STORAGE/launcher-postgres.log`（`"ab"`），`timeout=40`（L:73-90）；
后检失败 ⇒ `PostgreSQL did not start; inspect launcher-postgres.log.`（L:91-92）。**两个不同的日志文件**。

**网关**：argv 构造 P:227-449 → `prune_unsupported` P:548（会跑 `<exe> -h` **两次**、各 15s，P:148-219）
→ 追加 P:551-579 → argv 记录一行 P:580 → `Popen`（cwd 继承 = `server\`，CREATE_NO_WINDOW，stdout/stderr→`gateway.out/err`）P:581。
`ready.json` 轮询 P:583-592：`poll()!=None` ⇒ `gateway exited on <command>`；
**3600×0.05s=180s**（有 `DFO_PVF_CATALOGS`）否则 **1000×0.05s=50s**；超时后 `json.loads(ready...)` 抛 `FileNotFoundError`
→ 包装成 `RuntimeError(command)` P:593/690。**只读 `state["address"]`**（P:594）。

`server-only`：写 `run.json`，打印 `Server listening on 127.0.0.1:7001 and {address}`，`server.wait()`，`sys.exit(0)`（P:595-602）。

**payload** P:603-607：`"13?127.0.0.1?{port}?probe?00000000000000000000000000000000?0?0?30?0?0?0"`；
当 `channel-check` 或**降级后** tag 以 `_next30.._next34` 结尾时换成 `"3?127.0.0.1?7001?..."`（默认 launcher tag 走 7001 变体）。
实际 `ready.json`：`{"address":"127.0.0.2:51289",...}`，argv 带 `-game-listen 127.0.0.2:0`。

**probe 启动** P:628-645：`[probe.exe, client_dir, out/client.log, "55", mode, breakpoints.txt, payload]`，CREATE_NO_WINDOW；
`client_dir = DFO_CLIENT_DIR or dfo_probe_tools/dfo_probe_client`（P:610）；缺 `DFO.exe` 仅警告（P:611-612）。
**`run.json` 第二次写入在 `probe.wait` 之前**（P:646-648）⇒ launcher 可能对"瞬间失败的探针"报成功。
observer 分支 P:649-667 已死（launcher 设 `DFO_ENABLE_OBSERVER=0`）。`probe.wait(timeout=None if interactive/exception-trace else 65)` P:668。
`finally: server.terminate(); server.wait(timeout=5)` P:692-695。trace 解码 P:696-716。

**客户端进程由 `probe.exe` 起，不是 Python**：`CreateProcessW(target="<root>\DFO.exe", "\"<root>\DFO.exe\" <payload>",
flags=(normal?0:(root_debug?DEBUG_ONLY_THIS_PROCESS:DEBUG_PROCESS))|CREATE_SUSPENDED, cwd=root)`
→ `AssignProcessToJobObject` → `ResumeThread`（C:142-144）；默认 `interactive-ui` ⇒ `normal=1` ⇒ 无调试器、`SW_SHOWNORMAL`、
循环到客户端退出（C:145-154）。**此处不用 CREATE_NO_WINDOW**。

**超时汇总**：`listening()`0.5s；`pg_ctl` 40s（`-t 30`）；`exe -h` 15s×2；ready 轮询 180/50s；`probe.wait` 65s；
`server.wait` 5s；launcher 轮询 210/30s；探针内客户端窗口 `clamp(55,1,55)`s（C:108,146）。
**进程收尾**：只有 `channel_probe` 杀网关（P:692-695）；`launch_local` 从不杀/等。
`scripts/stop_environment.py` 对 8 个镜像 `taskkill /F /IM` + `pg_ctl stop -m fast -w -t 15`（:29-99）。

---

## 4. 环境变量契约

**L 设置**（L:250-254）：`DFO_CLIENT_DIR`、`DFO_SERVER_BINARY`（**总是**，含 json-mode/client-only）、
`DFO_CHANNEL_IDENTITY` `"1"/"0"`、`DFO_ENABLE_OBSERVER` `"0"`。

**`launch_environment` 顺序**（L:148-160）：复制 `os.environ` → 若 `--json-mode` **删除所有 `DFO_PVF_` 前缀键**（L:150-153）
→ 若 `profile_env` 非空**且**其中无 `DFO_PVF_CATALOGS`，则 `pop DFO_SKILL_RELEASE`、`pop DFO_ODYSSEY_REWARDS_PILOT`（L:156-158）
→ `env.update(profile_env)`（**profile 覆盖继承值**）L:159。

profile 提供 19 个键（`pvf-default.json`），含 `DFO_PVF_CATALOGS`（56 域）、`DFO_PVF_ARCHIVE`、`DFO_PVF_SHA256=""`、
10× `DFO_PVF_*_POLICY`、`DFO_SHOP_RELEASE`、`DFO_ODYSSEY_REWARDS_RELEASE`、`DFO_ATTUNEMENT_REBALANCE`、
`DFO_FATIGUE_FREE`、`DFO_ISPINS_MODE`。

**P 读取**：`DFO_SERVER_BINARY`(P:450，**覆盖** tag 选出的 `command[0]`)、`DFO_CHANNEL_IDENTITY`(P:451)、
`DFO_CHARACTER_STORAGE/CATALOG/RULES`(P:453-459，`command.index(flag)` ⇒ 非 roles tag 会 `ValueError`)、
`DFO_LOGIN_RESPONSE`(P:460-471)、`DFO_ODYSSEY_*` 覆盖(P:486-542)、`DFO_HISTORICAL_EQUIPMENT_CATALOG`(P:556-561)、
`DFO_PVF_CATALOGS`(P:196,586)。

**P 设置（网关继承）**：`DFO_ODYSSEY_COIN_RULES/WEAPON_BOX/GROWTH/CHAPTERS/CHAPTER_DROP` 绝对默认（P:485-542）；
`DFO_ODYSSEY_REWARDS_RELEASE="1"`（P:502,505）；`equipment-wear.full-candidate.json` 存在时**无条件覆盖**
`DFO_EQUIPMENT_WEAR_RULES`（P:544-546）；`DFO_EQUIPMENT_CATALOG="pvf"`（P:552-553）；
`DFO_ATTUNEMENT_REWARDS`、`DFO_EQUIPMENT_JOURNAL_RULES`、`DFO_EQUIPMENT_CREATE_COST` 恒为绝对值（P:566-579）。

**根 `.cmd` 传入**：`DFO_SHOP_OPEN_ALL=1`、`DFO_ODYSSEY_MODE`(0/1)、`DFO_CONTRACT_PURCHASE_CRASH_FIX=1`、
`DFO_MAX_ITEM_PERIOD=1`、`DFO_QUEST_VISIBLE_NPC_RELAX=1`、`DFO_QUEST_NPC_DISTANCE_MULTIPLIER=5`。
**`DFO_ODYSSEY_MODE` 现已无人读取**。

**网关别名**：约 58 个 `env:"DFO_*"` 在 `CMD/config.go:24-123` 声明一次。
语义：字符串原样；布尔**仅**字面 `"1"` 为真，除 `envmode:"not-zero"`（`DFO_BOOSTER_GAGE`）；
整数非法/越字节范围则回落默认（`config.go:197-215`）。
**关键：argv 里从不出现任何 `-pvf-*` 开关——全部 PVF 策略/归档接线只走环境变量**（已用实际 argv 验证）。

---

## 5. 产物

**L 创建**：`runtime/<tag>/`（`mkdir`，无 `exist_ok`）、`helper.out`、`helper.err`（`"wb"`，成功时为空）、
`runtime/storage/launcher-postgres.log`（追加）。

**P 创建**：`channelinfo.bin`(1060 B)、`fixture.json{candidate,crc,pb_hex,plain_hex}`（`indent=2`）、
`breakpoints.txt`（ASCII `"hexRVA LABEL\n"`，按 tag 追加）、`responses.json`（`json.dumps` 无缩进；
`{"1554":precheck_ok.bin}` / login_ 加 `"1"` / roles_ 为 `{1554,1,8,684}` 且 roles_row 改写 `"8"`）、
`gateway.out`（网关 stdout + 一行 `' '.join(command)`，**顺序是竞态** P:222-225,580-581）、`gateway.err`、
`run.json`（server-only `{server_pid,port}`；interactive `{server_pid,probe_pid,port}`）、
`probe.json{probe_pid,probe_returncode,client_dir,payload}`（**仅客户端退出后**）、
`client.log`（探针写，UTF-8，返回 3 时不存在）、`client_trace.txt`、`observer.{out,err}`（不可达）、
cwd=`server\` 下的 `runtime/pvf-cache`。网关另写 `ready.json`、`events.jsonl`。
**E/PR 写**：`server/work/client-build/Script.inner.pvf`（实测 761,764,363 B）+ `Script.inner.manifest.json`(1127 B)
及 `<inner>.stale-<stamp>` 轮转。

### `channelinfo.bin` 二进制布局（必须逐字节一致；实测 `ready.json` 的 `fixture_bytes=1060 == 16+1044`）

```
varint (P:35-41)  base-128，LSB 续位
integer(n,v) = varint(n<<3) + varint(v)            # wire type 0
string(n,s)  = varint(n<<3|2) + varint(len(s)) + s # wire type 2
pb    = integer(2,1) + string(3,keys) + string(4,b"LAN Local")
        keys = bytes((i % 127) + 1 for i in range(1024))   # P:54-55；pb = 1040 B (2 + 1027 + 11)
plain = struct.pack("<I", len(pb)) + pb                    # 4 字节 LE 长度前缀
cipher[i] = rol2(plain[i] ^ 0xB5)                          # P:57
table[i]: v=i; 8× v=(v>>1)^(0x4DB89129 if v&1 else 0)      # P:58-63 ⇒ 等价 Go crc32.MakeTable(0x4DB89129)
crc: init 0xFFFFFFFF; crc=(crc>>8)^table[(crc^x)&255]; final crc^=0xFFFFFFFF   # P:64-67 ⇒ crc32.Update
fold = (crc&255)^((crc>>8)&255)^((crc>>16)&255)^((crc>>24)&255)^0x18            # P:68-70
header = struct.pack("<B H I I I B", 0, 1, 16+len(cipher), 0, fold, 0)          # 恰好 16 B，P:71
偏移: 0:u8=0  1:u16=1  3:u32=1060  7:u32=0  11:u32=fold  15:u8=0  16..:1044 密文
```
**trace 解码是另一个变换，必须照抄**：`bytes(((((x>>6)|(x<<2))&255)^118) for x in trace)`（P:698）。

---

## 6. `probe.exe` 接口（`probe.cpp`）

- `argv[1]`=客户端根（须存在 `root\DFO.exe`，C:106-107）；`argv[2]`=日志（`wofstream`+UTF-8 codecvt，
  **不要预写 BOM**，C:108）；`argv[3]`=秒数 `clamp(1,55)`，`interactive` 时忽略（C:108,146）；
  `argv[4]`=模式 `interactive-ui|normal-ui|trace-owned-ui|trace-root-ui`（未知⇒`DEBUG_PROCESS`）C:137-142；
  `argv[5]`=断点文件，按 `while(spec>>hex>>rva>>label)` 读（**保持 ASCII、无 BOM**）C:109；
  `argv[6]`=payload，**原样**追加到客户端命令行，探针从不解析它 C:136。无 stdin、不读环境变量。
- 运行在 `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` 内（C:121-122）；根目录外的子进程会被杀，
  除无参 `systeminfo.exe`（C:163-166）。WFP 动态会话对所有非回环流量封堵（自身 + root 下每个 `.exe/.aes`），
  无管理员权限时降级为 `WFP_NOT_AVAILABLE_RUNNING_WITHOUT_ISOLATION`（C:112-119）。
- **退出码**：2=argc<4（C:106）；**3=`root\DFO.exe` 缺失且在开日志之前** ⇒ "瞬间退出、零日志"，
  常见于安全软件隔离 `probe.exe`（C:107；P:669-689）；7=job 建立失败（C:122）；11=客户端 `CreateProcessW` 失败（C:142）；
  12=job 指派失败（C:143）；21/22/23/24=`--net-check` 自检（C:65-72）；0=正常（客户端自身码只记为 `SUMMARY exit=0x…`）C:154,221。
- 生命周期：`Popen` P:628 → `run.json` P:646 → `wait(65s 或 None)` P:668；异常（含 `TimeoutExpired`）⇒ `RuntimeError(command)`
  P:690-691，`finally` 杀网关 P:692-695。

---

## 7. `ready.json` 契约

写入方 `CMD/main.go:157`：`json.Marshal(map[string]any{"address": l.Addr().String(), "advertise": advertised,
"pid": os.Getpid(), "fixture_bytes": len(raw), "channels": len(listeners)})`，`os.WriteFile` 0600（main.go:158），
并回显到 stdout（main.go:175）。实测：`{"address":"127.0.0.2:51289","advertise":"127.0.0.2:51289","channels":18,
"fixture_bytes":1060,"pid":3084}`。
**Python 只读 `state["address"]`**（P:594）⇒ 缺该键即 `KeyError`（包装成 `RuntimeError(command)`）；
`advertise/pid/fixture_bytes/channels` 及其它键缺失/多余都被容忍。`map[string]any` 编组会**按键排序**——不要依赖顺序。
Python **从不**用 `pid` 与 `server.pid` 交叉校验。
网关自己 `MkdirAll` 输出目录（`bootstrap.go:1518`）并用 `wire.ValidateServer` 校验 `-fixture`（`bootstrap.go:1523-1531`）
⇒ **`channelinfo.bin` 错 1 字节，`ready.json` 永不出现**。

---

## 8. 失败模式与文案（除标注外均为致命）

1 `Copy launcher.example.json to launcher.local.json and set client_dir.` L:47-49
2 裸 `KeyError` ⇒ `ERROR: 'client_dir'` L:192/293
3 `channel_identity must be a JSON boolean` L:191
4 `Storage missing. Follow README first-time setup; no database was changed.` L:53-55
5 `This development profile requires local loopback storage.` L:58-59
6 `Database offline; external data directories must be started by their owner.` L:67-69
7 `Existing PostgreSQL data or pg_ctl missing.` L:72
8 `PostgreSQL did not start; inspect launcher-postgres.log.` L:92
9 `Missing dependency: <p>` / `Missing repair profile dependency: <p>` L:214/222
10 **WARNING** `WARNING: 内层 PVF 未就绪：<exc>` L:140
11 `The game launcher requires Windows x64.` L:232
12 `Port7001 in use; inspect existing session before retrying.` L:236
13 `Startup stopped; inspect <helper.err>` L:284
14 `Startup timeout; inspect logs before retry: <out>` L:286
15 `Selected gateway does not advertise PVF direct support; rebuild it or explicitly select --json-mode.` P:197
16 **WARNING** `<exe> does not define <flags>; dropped them so this build can still start.` P:215-218
17 `next26 requires complete dungeon profile` P:303
18 `Historical binaries require DFO_HISTORICAL_EQUIPMENT_CATALOG and matching historical configs; use DFO_SERVER_BINARY
   with current PVF-capable source for this configuration tree` P:561
19 **WARNING** `角色目录不存在/不含 growtype…` CS:15-27
20 `副本内容已退休 JSON 入口…` CS:29（此处不可达）
21 `gateway exited on <argv>` P:589
22 `FileNotFoundError`→`RuntimeError(<argv>)` P:593/690
23 **WARNING** `probe cannot see DFO.exe under client dir: <dir>` P:612
24 `probe.exe exited with code N` + `WARNING: the game client was not launched correctly.` + rc3/安全软件文案 P:682-689
   （**helper 仍 exit 0，且 launcher 早已报成功**）
25 缺 trace ⇒ 静默
26 未捕获 ⇒ `ERROR: <exc>` exit 1 L:292-294

---

## 9. PVF 准备依赖

链路：`L._ensure_inner_pvf`(L:115-145) → `E.ensure` → `PR.prepare` → `PA.{aes,stream,wrapper_keys}`。
`pvf_archive` 导入 `pefile` + `cryptography`（PA:5-8）；两者都在 `tools\python\Lib\site-packages`。

- **它做什么**：`PA.wrapper_keys`（PA:26-41）把 `DFO.exe` 当 PE 解析，读 VA `0x14dc98110` 处的 PEM RSA 私钥，
  对 `sk.dat` 按 key_size 分块做 RSA-PKCS1v15 解密，再用 VA `0x14dc98118` 处的十六进制密钥做 AES-256-CBC（零 IV）
  解密前 `len//256*256` 字节，切成 32 字节密钥。`PR.prepare`（PR:31-79）按 0xA00000(10 MiB) 块遍历 `Script.pvf`，
  对前 `len(keys)` 块各自解密前 0x2800 字节，校验 `stream("iNfO", block[:48])[:4]==b"nkpi"`（PR:57），
  对内外层取哈希；先用临时 `pvf-prepare-*.partial`，再以 `os.link` 原子发布（**拒绝已存在输出**，PR:27-28），
  清单以 `"x"` 模式创建（PR:73）。
  拒绝条件：输出位于客户端目录内 PR:25-26、输出已存在、父目录缺失 PR:37-38、输出==清单 PR:33、
  首段被截断 PR:54-55、空 PVF PR:65-66、运行中客户端输入变化 PR:67-68。
- **写**：`server/work/client-build/Script.inner.pvf`(761,764,363 B) + `Script.inner.manifest.json`(1127 B)：
  `format "dfo_20260901_inner"`、`decoder`、client_exe/sk_dat/outer/inner 指纹，另有 `E.ensure` 追加的
  `cache` 列表 `{name,size,mtime_ns}`（E:190-193）。`INNER_PVF/INNER_MANIFEST` 取 `PROJECT.parent`（L:27-28），**不是 ROOT**。
- **四态门禁**（`E.decide` E:83-121，含精确原因串）：内层缺失 → 生成"内层 PVF 不存在"；
  清单缺失/不可解析/格式不符 → 重建"缺少或无法解析清单，旧件不可信"/"清单格式不符（%r）"；
  内层 size 不符 → 重建"内层 PVF 大小不符（盘上 N，清单 M）"；
  `cache` 长度==3 且 `(size,mtime_ns)` 相同 → **复用**"客户端与内层 PVF 均未变化，复用现有产物"；
  `DFO.exe+sk.dat+Script.pvf` 全量 sha256 相同 → **复用**"客户端指纹与清单一致，复用现有产物"；
  否则重建"客户端 DFO.exe/sk.dat/Script.pvf 已变化"；stat 抛 OSError → "无法读取客户端文件状态，按需重建"。
- **正常启动路径能否避开它？能。** 热机上 `decide()` 走 `(size,mtime_ns)` 快速路径返回 False（E:102-106）
  或只做哈希；`PR.prepare`（唯一做 RSA/AES/760 MB 的地方）只在 `needs_build` 时跑（E:167-177）。
  所以 Go 移植可以把 `pefile`/`cryptography` 从**热路径**移除，但冷路径仍需这套解包链——
  除非 launcher 侧的 `internal/pvfprep` 已覆盖；**注意该包不在本 checkout**（只有 L:118 与 E:34/91 的注释），
  它位于另一个仓库 `115us-dfolauncher`。

---

## 10. 移植风险清单（按严重度）

1. **`channelinfo.bin` 逐字节一致**：错 1 bit ⇒ fixture 校验失败、`ready.json` 不出现（P:52-73；1060 == 实测 `fixture_bytes`；bootstrap.go:1528）。
2. **两个相似但不同的旋转/混淆变换**：P:57（先 `^0xB5` 再 rol2）vs P:698（先 ror2 再 `^0x76`）。
3. **CRC 不是 IEEE**：`crc32.MakeTable(0x4DB89129)` + 折叠 8 位后再 `^0x18`（P:58-70）。
4. **tag 降级链 + `out` 用原 tag**：目录是 `_next37`，而 payload 端口、`-game-listen 127.0.0.2:0`、
   `channel.local34.json` 都来自 `_next34`（P:20-32,289-376）。
5. `DFO_SERVER_BINARY` **只覆盖** tag 选出的二进制，**不覆盖**该 tag 追加的额外开关（P:450 vs P:227-449；实际 argv 已确认）。
6. **相对路径基准混乱**：`ROOT=server\`（launcher.local.json / `resolved()`）、`PROJECT=dfo-lan`（profile 路径）、
   `PROJECT.parent`（client-build）；网关 cwd=`server\`，所以相对开关默认值（`configs/*.json`、`runtime/pvf-cache`）
   并不按字面解析（L:17-28；P:427-429,570-579；实际存在 `server\runtime\pvf-cache`）。
7. 每次脚本 spawn 都带 `CREATE_NO_WINDOW`，但 **probe 起客户端时不得带**（L:20/88/267；P:145/644；C:123/141-142）。
8. JSON 读取编码不一致（`utf-8-sig` vs L:275/P:593 的本地默认 vs E:75 的纯 utf-8）。
9. `gateway.out` 里 argv 行与 ready 行**顺序是缓冲竞态**（P:222-225,580-581；两种顺序都在盘上出现过）。
10. `run.json` 在 `probe.wait` **之前**写 ⇒ launcher 会对失败的探针报成功（P:646-648 vs P:668，L:273-282）。
11. 轮询次数由环境变量选择而非常量（L:271；P:586），还有 `poll()` 中止分支。
12. `prune_unsupported` 跑 `<exe> -h` 两次，并把未知开关**连同其值一起丢弃**（下一个不以 `-` 开头的 token）（P:148-219,548,551）。
13. 祖孙/Job 语义：探针持有 kill-on-close job + 动态 WFP 会话；interactive 模式会阻塞整个客户端会话（C:121-122,143,145-154；P:692-695）。
14. `launch_local` 不回收 helper 就返回，而 `channel_probe` 在 `finally` 里杀网关。
15. `-character-*` 以**开关**形式由 tag+env 注入（P:453-459），而全部 `-pvf-*` 接线只走**环境变量**。
16. `launch_environment` 的合并/删除优先级（L:148-160）。
17. `DFO_EQUIPMENT_WEAR_RULES` 被静默覆盖、`DFO_ODYSSEY_REWARDS_RELEASE` 被强制置位（P:502/505,544-546）。
18. `--check` 声称 "starts nothing"，实际可能触发 760 MB 内层 PVF 构建（L:165-167 vs 218-223）。
19. `breakpoints.txt` 必须保持 ASCII/无 BOM（C:109 用 `std::wifstream`）。
20. `-quest-equipment-catalog` 是**追加**的（P:554），而 `-loot-catalog "pvf"` 是模式标记而非路径。
21. `town.generated.json` 仍在被传，尽管已删除（P:264；因 `TownPath` 无生产读取者而惰性）。
22. `--client-only` 在帮助里宣传 `DFO_LAN_HOST`，但无人读取，且 helper 仍会起本地网关（L:185）。
