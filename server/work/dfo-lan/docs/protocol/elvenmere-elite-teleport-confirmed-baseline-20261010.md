# 艾尔文泪湖换层入口确认基线

业主确认：“能进去了，提交这里吧”。确认仅为精锐启用时艾尔文泪湖换层入口恢复，不包括随后报告的 APC 跟随问题。

源码候选 `bin/wireprobe-handoff-source.exe`，版本 `0.3.19-elvenmere-attempt1`，31,107,072 字节，SHA256 `dda10d39b4d89528a6380365ebfa33b57452c0de475b7a64cfc4863c0e0dd947`；默认程序保持。源码修复依据原生 CMD2015 被精锐门禁拦截的日志，恢复 legacy 已有响应，不新增协议或客户端改动。

实机会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261010_150641_992519_next37/`，客户端 PID22348/runTick7840796：

- `events.jsonl:258–265` 15:08:45（北京时间）首次 CMD2015 成功，ACK2015 `01`，NOTI2193 前缀 `000102`（已过1/当前2）。
- 至 `events.jsonl:545–552` 15:11:05，第11次 CMD2015 成功，NOTI2193 前缀 `000b0c`（已过11/当前12）。
- 此过程没有新的 CMD37/CMD45/CMD1811；不能把通知层数等同服务端已同步所有原生场景对象。
- 业主指出 APC 留在首层；现有原生日志只记录首房三名登记成功，跟随问题独立调查。

Go1.26.0 执行 `go build ./...`、`go vet ./...`、`go test ./... -count=1 -json` 通过：4154通过/273跳过，前基线4140通过/273跳过，逐名失败集合为空。提交前fetch实际 `gud/main` 与HEAD同为d80919db（0/0），没有远端变更需要合并；本机不存在规范旧称origin/fork，未添加或更改远端。门禁与重新验证结果在提交时实际核验。

已有SQLite角色/物品/schema、PVF资源及默认程序保持；用户其它工作区文件与运行期数据不提交。奖励真源、经验、周期/幂等遗留边界见 [入口取证](elvenmere-elite-teleport-20261010.md) 与根目录PVF运行依赖台账。
