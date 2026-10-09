# client-patchs/tests — 机制与探针的离线自测

只在本进程里跑，**不碰 `DFO.exe`、不碰客户端目录、不需要游戏**。

| 文件 | 是什么 | 状态 |
| --- | --- | --- |
| `hp-probe-selftest.c` + `build-hp-probe-selftest.cmd` | `client-patchs/hp-caller-probe` 的环形缓冲/汇总逻辑（直接 include 插件用的同一份 `probe-logic.h`）：条目布局自检、4 生产者×20 万样本 + 1 消费者、落后丢样计数、顺序 1 万样本精确计数 | ✅ **通过**（`ALL SELF-TESTS PASSED`）|
| `cave-selftest.c` / `cave_test_entry.asm` + `build-cave-selftest.cmd` | 把 `hp-caller-probe` 的代码洞拷到私有可执行页、喂已知寄存器、校验全部非易失寄存器与 `RAX`/`RSP` | ⚠️ **实验性，未跑通**：异常报在洞内 `cave+0x22`，但该处字节实测为合法 `push rbx`、页属性 `PAGE_EXECUTE_READ`、`RSP` 正常；原因未定位。**不属于交付物**，失败即退出 |
| `inline_test.c` / `vtable_test.c` / `gate_test.c` / `slot_test.c`（及各自 `build-*.cmd`） | 既有共享机制自测（inline 跳转、虚表替换、IME 门禁、Themida 导入槽） | 既有内容，未改动 |

`cave-selftest` 卡点的价值：它已经暴露出一个**真实的构建陷阱** ——
MSVC 链接器会折叠相同数据（ICF），而代码洞镜像几乎全是零字节，实测会被别名成别的零数据，
从而把桩里的跳转槽清成 0。`hp-caller-probe` 因此用 `/OPT:NOICF` 构建，
并在插件里加了内嵌镜像的**构建期完整性自检**（不符就连洞都不装）。

构建：

```cmd
client-patchs\tests\build-hp-probe-selftest.cmd
client-patchs\tests\build-cave-selftest.cmd    :: 实验性，可能失败
```
