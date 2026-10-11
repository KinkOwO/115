# 启动探测路径补丁 —— 取证与偏移推导

本文件随 mod 包分发（`client/PATCH-NOTES.md`，只是包内文档，不落位到客户端、不生成任何 op）。

## 1. 现象与结论

115 级客户端在启动早期拼出一条 UNC 路径 `\\toolserver02.neople.co.kr\CommonConfig`
（遗留的 Neople 开发环境共享目录），再调 `GetFileAttributesW` 判断"共享配置目录是否存在"。
该主机在本地/局域网不可达，SMB 连接/协商超时把**主线程**卡住，窗口迟迟不出现。

补丁把这条探测路径改成本地不存在的目录，`GetFileAttributesW` 立即返回
`INVALID_FILE_ATTRIBUTES`；代码把结果记成"没有共享配置"的布尔标志，与共享不可达时的取值
**完全一致**，因此初始化流程与分支结果都不变。

## 2. 字符串存放位置（xorstr）

客户端把这类常量字符串以自带的 xorstr 编码静态存放在 `.rdata`，运行时按需解密
（解密函数 `0x146E8C7D0`；算法与 IDA 复原记录见 `analysis/tools/pe_decode_xorstr.py`，
其文件头注明来自 `sub_146E8C490`）。`analysis/dumps/xorstr_addr_to_text.json` 已收录全量
VA→明文映射：

| 明文 | VA | 文件偏移 | 槽大小 | 密文长度 |
| --- | --- | --- | --- | --- |
| `\\toolserver02.neople.co.kr` | `0x14A8FF460` | `0xA8FF460` | 64 字节 | 56 字节（27 字符 + NUL，UTF-16LE） |
| `\CommonConfig` | `0x14A8FF4A0` | `0xA8FF4A0` | 40 字节 | 28 字节（13 字符 + NUL） |

- 偏移换算：`.rdata` 节表 `VirtualAddress=0x149186000` / `PointerToRawData=0x9186000`，
  两者相差 `0x1000`（= `.text` 的节对齐），所以本节的 **文件偏移 == RVA**：
  `0x14A8FF460 - 0x140000000 = 0xA8FF460`。
- 槽头 4 字节 `00 54 48 2A`：`byte0` 标志位、`byte1=0x54`（`seed8 = byte1 & 0xFE = 0x54`）、
  `u16 code=0x2A48`。长度 = `2 * (code ^ (seed8 | seed8 << 7))` = `2 * (0x2A48 ^ 0x2A54)` = `2 * 28` = 56 字节。
- 全文件中该 60 字节（header+密文）**只出现一次**（`0xA8FF460`），密文正文也只出现一次
  （`0xA8FF464`）——没有第二份副本要一起改。

## 3. 调用点（两处，共用同一对字符串）

在 `.text` 里按 RIP 相对寻址扫到引用这两个 VA 的代码，只有两处，且**形态相同**：

| # | 引用 `\\toolserver02.neople.co.kr` | 引用 `\CommonConfig` | 所属函数 |
| --- | --- | --- | --- |
| A | `0x1459970C6` | `0x14599712A` | `CNGameCore::Create`（`0x1459968C0..0x1459992EF`，源文件 `GameCoreInit.cpp`） |
| B | `0x1459A147D` | `0x1459A14D8` | 独立小函数 `0x1459A1450..0x1459A15F3`（紧邻 `CNGameCore::initNgsModule` `0x1459A0620..0x1459A0B8F`） |

两处的形状（以 A 为例，B 完全相同）：

```asm
lea  rcx, [rip+...]        ; '\\toolserver02.neople.co.kr'  (VA 0x14A8FF460)
call 0x146E8C7D0           ; xorstr 解密 -> rax
... std::wstring 构造 + wcslen ...
lea  rcx, [rip+...]        ; '\CommonConfig'               (VA 0x14A8FF4A0)
call 0x146E8C7D0
... wstring::append ...
call qword ptr [rip+0x37E4EC3]   ; GetFileAttributesW(c_str())
cmp  eax, -1                     ; INVALID_FILE_ATTRIBUTES
setne al
mov  byte ptr [rbp+0x14DA], al   ; 对象 +0x14DA = "有共享配置" 标志
```

- `0x1459970C6` 上方紧邻的日志串是 `'suite mask : %d'` / `'product type : %d'` +
  `'DNFClient\RDAR\GameCoreInit.cpp'` 行号 `0x3C6`/`0x3C7`，说明这是**很靠前**的初始化步骤。
- 两处都写同一个偏移 `+0x14DA` 的布尔标志；共享不可达时它本来就是 0，补丁后仍是 0。
- 该小函数（B）在全镜像里**没有**直接 `call/jmp rel32` 调用者，也没有指针落在文件内
  （`.data` 的 `VirtualSize 0x1760378 > SizeOfRawData 0x9D2000`，尾段是运行时才填的），
  所以它是由运行期填表间接调用的；这不影响结论：**两处用的是同一对字符串 VA**。

## 4. 为什么改字符串而不是改指令

- 两个调用点共用同一对字符串 VA，**重新加密一次**就把两处一起改掉，不用动任何一条指令；
- 只碰 `.rdata` 里 56 字节密文，**header 与槽外字节逐字节不变**（构建脚本会自证这一点），
  指令边界、`std::wstring` 布局、SMB 之外的任何逻辑都不受影响；
- 判定语义保持"没有共享配置"，与今天共享不可达的结果相同。

## 5. 新路径的选择

- 新明文 `C:\115us-no-commoncfg-share`（**恰好 27 字符**）→ 密文长度仍是 56 字节，
  header（`seed8` / `code`）可以原样保留，只有正文密文变化；
- 运行时拼出的路径是 `C:\115us-no-commoncfg-share\CommonConfig`：`C:` 存在、该目录不存在，
  `GetFileAttributesW` 立刻返回失败（不产生任何网络往返）；
- 选 `C:` 而不是某个盘符/UNC：`C:` 在任何 Windows 上都存在，判定确定性最好。

## 6. 构建脚本自证（`build-mod.py`）

打包前依次做：

1. **现场核对**：整份 EXE 的 size/sha256 与声明比对（只警告不阻断）；槽 header 必须等于
   `00 54 48 2A`；按 xorstr 算法解出的明文必须等于 `\\toolserver02.neople.co.kr`。
   任一不符即**停手，不改任何东西**。
2. **长度守恒**：新串长度必须等于原串长度（27），否则报错退出（改长度就得同步改 header，
   不在本补丁范围内）。
3. **补丁后自证**：把补丁打到内存副本上，再按同一算法解回 `C:\115us-no-commoncfg-share`；
   并断言 header 未变、槽外字节未变。
4. `modkit install` 时引擎还会**逐处 offset 比对 `before` 字节**（`apply2.go:191-232`），
   对不上就拒绝且不落盘。

## 7. 未闭环 / 风险

- **实机时延数字待业主复测**：本补丁只保证"探测立即失败、判定结果不变"；
  "进程创建到窗口出现"从约 12 秒降到约 1 秒需要业主在自己机器上复测（AI 不代跑客户端）。
- **Themida 完整性未知**：客户端带 Themida（`.themida` 节 RWX）。本次只改 `.rdata` 数据区
  56 字节，不改入口/不改代码节；但"打包器是否对 `.rdata` 做完整性校验"**没有验证过**。
  历史先例（`mods/odyssey-hardcore` 早期版本改过 `.text` 3 处并可用）说明改数据区风险更低，
  但仍属未闭环项；一旦出现异常，先 `modkit uninstall` 还原再复现，以区分"补丁问题"与"客户端问题"。
- **`+0x14DA` 标志的消费者未追**：只知道它是"有共享配置"的布尔标志、且补丁前后取值相同。
  该标志为真时会加载什么（如果会）没有进一步追；因为本地共享本来就不可达，标志一直是假，
  所以补丁不改变任何既有行为。
- 另一条完全不动 EXE 的等价路子是把 `toolserver02.neople.co.kr` 在 `hosts` 里指到 `127.0.0.1`
  （让 SMB 立即被拒）。本 mod 按业主要求走 EXE 补丁路线，`hosts` 方案未实现、未验证。
