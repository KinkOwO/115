# 进度-20261004-wrapper_keys 多版本密钥提取修复（固定 VA + 全文件扫描回退）

## 问题
`pvf_archive.wrapper_keys()` 原实现从 `DFO.exe` 的固定 VA 槽位（0x14DC98110 → RSA PEM 指针、0x14DC98118 → AES hex 指针）取 wrapper 密钥。该布局只在 2.38.2.34 有效；**2.38.3.25 的 DFO.exe 里这两个槽位只剩运行期残值（指针 0x127）**，导致 `Unable to load PEM file ... MalformedFraming`，内层 PVF 生成中断（即昨天 `启动游戏.cmd` 报 `Missing repair profile dependency: Script.inner.pvf` 的根因链）。

## 修复设计
两段式取钥，任何一段成功即返回：

1. **固定 VA 快路径**（原逻辑，包 try/except）：2.38.2.34 直接命中，零额外开销。
2. **全文件扫描回退**（新增，`_scan_literals`）：
   - 正则扫描 DFO.exe 文件体中所有 `-----BEGIN PRIVATE KEY-----.*?-----END PRIVATE KEY-----`（re.S）；
   - 每个 PEM 块配对其**结束后 0x200 窗口内**首个 64 位大写十六进制串（`[0-9A-F]{64}(?![0-9A-F])`）作 AES 钥候选；
   - 候选真伪由 `_derive_keys` 用 **sk.dat 首块 RSA/PKCS1v15 试解**筛选——exe 里还有一把无关的 MIICeQ 钥（文件偏移 0xb1bf4a0），试解天然剔除；
   - 全部候选失败时显式 `RuntimeError`（原来是对残值指针抛晦涩的 pefile/PEM 异常）。

`_derive_keys` 把原函数体（RSA 分块解密 → prefix 对齐 256 → AES-CBC → 32B 分段）封装为"失败返回 None"的候选筛选器，两条路径共用。

## 变更文件
- `server/work/dfo-lan/scripts/pvf_archive.py` — 实现上述两段式。
- `server/work/dfo-lan/scripts/reinforcement_pvf_reader.py` — 同步相同修改（两文件本就是有意保持同步的副本）。
- `server/work/dfo-lan/scripts/test_pvf_wrapper_keys.py` — 新增单测（合成 RSA/AES fixture，**不含任何真实密钥**）：
  - `test_scan_fallback_recovers_keys`：非 PE 假 exe（固定 VA 必失败）+ 诱饵钥在前 + 真实钥在后，扫描回退取回正确钥；同时覆盖 prefix 非整除（480B 明文，prefix=256 只加密前 256B、其余透传）的边界；对两个模块分别验证。
  - `test_no_usable_key_raises`：sk.dat 换成不可解密的随机数据时必须 RuntimeError。

`server/reference/analysis-tools/` 下还有 4 个使用固定 VA 的参考脚本（unwrap_pvf / rewrap_pvf35/37/38），属只读参考件，未改。

## 验证
- 单测：`tools\python\python.exe -m unittest test_pvf_wrapper_keys -v` → 2 tests OK。
- 真实客户端：
  - `DFO_2.38.2.34` → 73 keys，0.0s（固定 VA 快路径，无回归）；
  - `DFO_2.38.3.25` → 73 keys，0.1s（扫描回退路径）。
- 端到端：`Archive('DFO_2.38.3.25/Script.pvf')` 完整解析成功——nkpi 头校验通过，5,650,303 条目、631,993 个 .stk/.equ/.str/.lst 文件索引，15.2s。

## 结论
wrapper 密钥提取已与客户端版本解耦；新客户端 PVF 解包链路全通。内层 PVF 生成（步骤 3）的阻塞已解除。

## 遗留问题
- AES 候选取"PEM 后 0x200 窗口内首个 64 位十六进制串"，若未来版本窗口内出现多个同形态字符串需扩为多候选枚举（当前两版均唯一命中）。
