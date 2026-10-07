# AGENTS.md — analysis/

> 本文件是逆向分析与协议取证规则的领域索引。先读根 `AGENTS.md`，再读本文件。
> 逆向主目标为 115 级客户端 `client/DFO.exe` 及权威 IDB `client/DFO.exe.i64`。
>
> **强制继承根规范**（2026-10-04）：根 [`AGENTS.md`](../AGENTS.md) §0.3 提交规范、§0.4 代码目录规范、
> §0.5 开发规范对本目录**完全适用**。提交前必须先跑
> `pwsh -NoProfile -File scripts/check-commit-hygiene.ps1`；报出缓存/产物或规范违规时**立即停止提交**，
> 逐条报告业主并取得**明确二次确认**后才可继续。
> 分析产物落位：任务记录写 `analysis/tasks/`，权威 Dump 写 `analysis/dumps/`（新增 Dump 需同步 `dumps/README.md`）。

## 1. 证据优先级

1. **当前客户端实际动态命中**：`client/DFO.exe` 运行时的原生输入/输出（通过 `probe.exe` 配合网络抓包与服务端会话日志）。
2. **权威 IDB 静态逆向**：`client/DFO.exe.i64` 的控制流、数据结构、函数签名与调用约定。
3. **已验证测试向量与日志**：`server/work/dfo-lan/internal/game/protocol/testdata/` 中的原生测试向量，以及 `runtime/roles_*/events.jsonl` 会话日志。
4. **历史脚本与文档对照**：`server/reference/analysis-tools/` 与 `server/work/dfo-lan/docs/protocol/`，仅作线索参考，不是事实标准。

### 硬规则

- **协议闭环前不盲目实现**：实现新协议处理前，必须确认客户端 reader、字段宽度、字节序、flag 分支与消费时序；严禁仅凭“外形相似”直接编写代码。
- **动态观测默认依靠服务端日志与抓包**：本地动态观测优先分析服务端产生的 `events.jsonl` 与协议数据；不随意挂接 x64dbg 或下硬断点打断客户端网络心跳。
- **IDA 分析规范**：以 `client/DFO.exe.i64` 为准。
- **记录失败假设**：尝试失败时记录简要 checkpoint（日期、测试输入、未闭环点、下一步），避免重复试错。
- **异常先查资源边界**：客户端闪退或表现异常时，先对比 `client/Script.pvf`、`client/sk.dat` 与配置文件，区分数据缺失与协议错误。

## 2. 新加密与协议解析门禁（最高优先级硬规则）

> 新的协议封包或加密算法，未经完整 IDA 逆向链与客户端原生向量验证，严禁臆断算法名称或格式。

**A. IDA 逆向链闭环**：

| 检查点    | 必须确认                                               |
| --------- | ------------------------------------------------------ |
| 选择路径  | opcode/cmd/flag 如何分发到 handler/codec；调用点与方向 |
| wrapper   | 包装层、缓冲区指针、长度、原地/异地变换                 |
| key 来源  | 密钥材料来源、长度、协商流程、会话密钥派生时机         |
| 核心变换  | 算法变体、block 大小、查表、轮常量、字节序             |
| transport | 对齐、padding、校验和、包头长度与字段排布               |

> 常量局部相似、参考服函数名、密文长度一致、自回环可解密 —— 均**不构成证据**。

**B. 客户端原生向量逐字节验证**：

- 从**实际客户端原生调用**提取至少一组 `(plain, cipher)` 向量，固化为 `internal/game/protocol/` 的 Go 单元测试。
- **S2C**：服务端密文经客户端原生解析须逐字节还原明文并被 reader 消费；**C2S**：客户端密文须被服务端正确解密且字段符合业务语义。
- `encode -> decode` 自回环只证内部互逆，**不证与客户端兼容**。

## 3. IDA 与分析工具资产

- **权威 IDB**：`client/DFO.exe.i64` 是本项目的静态分析真源。
- **沉淀命名**：在确认未命名函数（`sub_xxxxx`）或数据结构的功能后，及时在 IDB 中规范重命名（如 `PacketReader_*`、`Handler_*`），节省后续逆向时间。
- **字符串提取**：DFO 客户端存在大量加密或运行时动态解密字符串，优先利用 `server/reference/analysis-tools/` 下已有的分析脚本（如 `scan_literals.py`、`decode_literals.py` 等）辅助定位。
- **专用探针脚本**：`server/reference/analysis-tools/` 积累了大量历史分析脚本（如 `channel_crypto_oracle.py`、`packed_stats_oracle.py`、`dungeon_map_oracle.py` 等），分析对应子系统前建议先检索相关脚本。
- **现成分析 Dump 资产（优先查阅，避免重复逆向）**：`analysis/dumps/`
  - `opcode_name_to_hex.json` / `opcodes.tsv`：全量 5,330 条 CMD/NOTI 数据包 opcode 原生名称与十六进制编号映射。
  - `xorstr_addr_to_text.json` / `ida_annotate_xorstr.py`：全量 114,614 条静态加密字符串与 VA 地址映射，支持一键载入 IDB 注释。
  - `dstr_id_to_text.json` / `dstr_raw.txt`：34,894 条 Neople 本地化翻译 ID 与明文表，解决 PVF Type 8 本地化引用及报错信息排查。
  - `pvf_file_paths.txt` / `pvf_file_index.tsv`：全量 565 万 PVF 文件路径与元数据索引，支持秒级 `grep` 定位目标文件。
  - 
  - `idb_funcs.tsv`：权威 `idb` 已确认函数，用于快速定位函数。
  - `idb_globals.tsv`：权威 `idb` 已确认全局变量，用于快速定位全局实例。
  - `idb_struct.json`：权威 `idb` 已确认结构，用于快速确认结构体属性偏移。
  - 详见 `analysis/dumps/README.md`。

## 4. 实机调试与网络安全隔离

- 客户端测试必须经 `scripts\启动游戏-SQLite.cmd` / `scripts\启动游戏-PostgreSQL.cmd`（内部走仓库内 Go 启动器 `dfolauncher launch`）启动；WFP 规则由 Go 隔离实现安装，强制客户端只连回环（127.0.0.1 / 127.0.0.2）。
- **严禁无人值守**代替用户操作客户端；流程见 `server/AGENTS.md` §4。

## 5. 分析复用原则

在完成分析后，需要将已确认的函数、全局变量和结构体写入到资料库中，以改善后续分析速度。

- `idb_funcs.tsv`：已确认函数
- `idb_globals.tsv`：已确认全局变量
- `idb_struct.json`：已确认结构、属性和偏移

结构体规范：

```json
{
    "结构体名": {
        "属性名": {
            "type": "类型名",
            "offset": 偏移量
        }
    }
}
```

> 可以添加其他自定义字段，但以上提到的字段必须存在，结构体必须至少提供一个属性，属性必须提供`type`和`offset`
>
> `type`可以指向其他结构体或者数组，例如`uint32`、`int8[8]`、`void(int, bool)`

函数和全局变量规范：

```
address	name	description
地址	名字	说明/描述(可空)
```

