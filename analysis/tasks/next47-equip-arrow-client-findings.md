# next47 — 装备栏"更好的装备 ↑"箭头：客户端侧定位结论

> 承接 `analysis/tasks/next47-equip-arrow-handoff.md`（§6 两个待答问题、§7 期望产出、§8 硬约束）
> 分析日期：2026-09-21（第二轮已做**根源修正**，见 §9） · 手段：纯静态（PE 原始字节 + capstone + xorstr 表），未修改任何客户端文件
> 环境说明：本机无 IDA，`client/DFO.exe.i64` 无法打开，**未打开也因此无需关闭 IDB**（§8 末条已满足）

---

## 0. 一句话结论

那个 `↑` 是 **`UI/Inventory/Animation/ArrowUp.ani`**（背包格覆盖图标族之一），由两条客户端代码路径安装。
两条路径共有一个关键形状：**必须拿到"该部位当前已装备物品"的记录才能做强弱比较；拿不到时走退化分支，默认把箭头画出来。**

而这张"已装备记录表"**不是客户端本地算出来的，是被反序列化进来的**：它由三条报文处理器
（`0x141FA8CE0` / `0x141FA8CA0` / `0x141FA8D00`）分别从**当前网络报文**中读取
`0x590` / `0x120` / `0x1E0` 字节，写进对比控制器单例的连续三块区域。

**判定条件（自然语言）**：
> 该格子物品必须是**本角色能穿**的装备类物品（等级达标、部位/类别在白名单内、可用性谓词通过）；
> 若客户端**知道**该部位当前穿着什么 → 显示 ⇔ `Score(格内物品) > Score(身上那件)`；
> 若客户端**不知道**该部位穿着什么（`equippedTable[部位].part <= 0`）→ **直接显示**（退化分支，即当前 BUG 态）。

---

## 1. 箭头资源：`ArrowUp.ani`（并证伪交接文档 §5 的三个猜测）

`UI/Inventory/Animation/` 目录下全客户端只有 4 个资源，构成**背包格子覆盖图标族**：

| VA | 资源 | 语义 |
| --- | --- | --- |
| `0x1498CA450` | `UI/Inventory/Animation/ArrowUp.ani` | **← 目标箭头（更好的装备）** |
| `0x14A6CEA60` | `UI/Inventory/Animation/TimeLimit.ani` | 限时标记 |
| `0x14A6CEAC0` | `UI/Inventory/Animation/upgradeicondodge00.ani` | 升级标记 |
| `0x14A6CEB30` | `UI/Inventory/Animation/conversionicondodge00.ani` | 转换标记 |

配套贴图集：`Item/IconMark.img`、`Item/IconMark_univ.img`。

### 交接文档 §5「箭头资源」全部排除（邻居法，非猜测）

字符串表是 `.rdata` 中字面量的转储，**同一模块/同一 `.xui` 的字面量地址通常相邻**，所以打印目标地址前后若干条即可判断归属。实测：

| 猜测 | 实测邻居 | 判定 |
| --- | --- | --- |
| `spec_slot_arrow` `0x14974BBB0` | `specEntry_%d` / `spec_slot` / `recommendSlot` / `currentEquip` / `recommendEquip` / `equipItemSlotPanel` / `over_tooltip_*` | ❌ 属**地下城推荐装备面板** |
| `up_arrow` `0x1492F60D8`、`down_arrow` `0x1492F60F8` | `left_arrow`/`right_arrow`/`arrow_panel_back`/`arrow_timer_gauge`，紧邻 `Contents/2026/AradOdyssey/UI/Node/*` | ❌ 属 **2026 阿拉德历险记节点 UI** |
| `panel_arrow` `0x1492BD070` | `Contents/2026/AradOdyssey/UI/Node/ani_arrow_head.ani` … | ❌ 同上 |
| `upArrowText`/`downArrowText`/`equalText`/`gradeImg` | 同一片内出现 `Live/Event/Chn/2025/0612_AnniversaryContent/UI/InGame/matching.xui` | ❌ 属 **2025 周年庆匹配 UI** |

`is equipped` / `is my equipped item` / `is same my equipped item group` / `is equipped slot` 一簇：
邻居是 `rarity` / `item category` / `stackable type` / `is lock` / `is sealed` / `is on my inventory` /
`is on my cargo` / `is my character usable` / `is dimmed effect` / `is able rbclick item` …
→ 这是**物品属性/查询键注册表**（通用求值器），不是箭头逻辑本体。❌

### 本模块自身的字符串（本轮新增，用于定名）

```
"CNGridArrow"  "CNCustomDownArrow"  "CNDownArrow"      ← 箭头类 UI 控件名
"upgradeArrowImage"  "upgradeDetectionImage"  "upgradeItemText"
"basicAllEquipedItemChangeButton"  "basicItemControl"  "basicAcquisitionInformationButton"
"fame_txt"  "emblem_weight"  "gradeImg"
"stat_panel"  "option_%d"  "stat_%d"  "detectionImage"  "detectionImage2"  "checkImg"
"sy_item_compare"  "sy_rank_score"  "sy_scene_auto_skip"
```
→ 一整套**物品提示框「装备对比」面板**的元素命名，与 §3 的对比控制器一一对应。

---

## 2. `SetSlotArrow` —— 箭头唯一的写入入口

```
0x141F9C7B0  SetSlotArrow(Window* w /*rcx*/, int index /*edx*/, bool on /*r8b*/)
```

```
if (index > 0x3B) return;                        // 60 格上限
if (*(void**)(w + index*16 + 0x58) == NULL) return;   // 该格没有物品 -> 不动
auto& anim = *(void**)(w + index*16 + 0x7D8);         // 每格独立的箭头对象（stride 0x10）
if (anim) { Stop(anim, /*...) ; }
if (!on) return;
if (anim) { Play(anim, 1); return; }
// 否则新建：
const wchar_t* path = Decrypt(xorstr "UI/Inventory/Animation/ArrowUp.ani");
auto a = CreateAnimation(path, ...);
Assign(&(w + index*16 + 0x7D8), a);              // 存进这一格
```

即：**每格一个 `ArrowUp.ani` 实例，存在 `窗口 + 格号*16 + 0x7D8`；物品指针在 `窗口 + 格号*16 + 0x58`**（两个并行数组，60 项）。
该函数**只有一个调用方**：`0x141F9CF51`（见 §3）。

---

## 3. 对比控制器：对象布局（本轮完整解出）

单例全局 **`0x14E634248`**（懒构造于 `0x141FAB150`，构造器 `0x1456918F0`，`operator new(0x9C0)`）。
其内部有一整块被报文反序列化的结构：

```
obj + 0x048 : 查找表管理器指针（Score / MapEquipSlot 都用它）    ← 由 0x141FA9B40 处 alloc(0x340) 建
obj + 0x050 : 第二张表指针                                        ← alloc(0xB8)
obj + 0x0A0 : u8  flag[11]     ← 分析函数写（见 §5）
obj + 0x0AB : u8  flag[60]     ← 分析函数写（见 §5）
obj + 0x126 : int count                       ← 记录块头（0x141FAB1E0 读）
obj + 0x12A : Rec20 equippedTable[11]         ← ★ 已装备/参考表 ★（0x141FAB940 读，idx ≤ 10）
obj + 0x206 : Rec20 slotTable[60]             ← ★ 格子物品表 ★（0x141FAB1F0 读，idx ≤ 59）
obj + 0x6B6 : 第 2 块载荷，0x120 字节
obj + 0x7D6 : 第 3 块载荷，0x1E0 字节
obj + 0x9B6 : int（0x141FAB840 读）  → 对象总大小 0x9C0
```

**偏移自洽性校验**：`0x12A + 11*20 = 0x206`；`0x126 + 0x590 = 0x6B6`；`0x6B6 + 0x120 = 0x7D6`；
`0x7D6 + 0x1E0 = 0x9B6`；对象 `0x9C0`。**三块载荷首尾相接，拼成一个整体结构。**

`Rec20 = { int part; int key[4]; }`（20 字节，`part` = 装载部位序号）。

访问器一览：

| VA | 语义 |
| --- | --- |
| `0x141FAB1D0` | `return obj + 0x126;`（记录块基址） |
| `0x141FAB1E0` | `return *(int*)(obj + 0x126);`（count） |
| `0x141FAB1F0` | `GetSlotRec(i, out)`：`i ≤ 0x3B`，读 `obj + i*20 + 0x206`，16+4 字节 |
| `0x141FAB940` | `GetEquippedRec(i, out)`：`i ≤ 0x0A`，读 `obj + i*20 + 0x12A`，16+4 字节 |
| `0x141FAB220` | 统计 `slotTable[]` 中 `part != 0` 的条数 |
| `0x141FAA9D0` | **`MapEquipSlot(obj, itemKey) -> 部位序号`**（见下） |
| `0x141FAB260` | `Score(obj, Rec20*) -> int`（见 §4） |

### `MapEquipSlot`（`0x141FAA9D0`）—— 本轮重要修正

```c
int MapEquipSlot(Ctrl* self, int itemKey) {
    if (!self->[0x48]) return 11;
    ItemDef* def = Resolve(self->[0x48], itemKey);      // 0x140157B30
    if (!def) return 11;
    void* g = LookupGroup(def->[4]);                    // 0x145A70830，参数 = def+4 处那个 id
    if (!g) return 11;
    switch (g->[0x848] - 0x0E) {                        // 12 路跳转表 @0x1FAAA84
        case 0: return 1;   case 1: return 0;   case 2: return 2;
        case 3: return 4;   case 4: return 3;   case 5: return 6;
        case 6: return 5;   case 7: return 8;   case 8: return 7;
        case 9: return 0xA; case 10: return 9;
        default: return 11;                             // 哨兵：不适用
    }
}
```

**它不是"查找已装备物品"，而是"内部部位码（`0x0E`–`0x19` 共 12 种）→ 本表部位序号（0–10）的置换映射"**，
且这些内部码在别处被成对互换（`0↔1`、`3↔4`、`5↔6`、`7↔8`、`9↔0xA`）。
证据：`0x141FAD8E9 lea edx, [r15+0xe]` 用的就是同一个 `0x0E` 基址。

---

## 4. 比较量的语义 —— `Score`（`0x141FAB260`）

```c
struct Rec20 { int part; int key[4]; };            // 20 字节
int Ctrl::Score(const Rec20* r) {                  // 0x141FAB260
    int s = 0;
    if (this->[0x48])                              // 查表管理器
        for (int k = 0; k < 4; k++)
            s += Lookup(this->[0x48], r->key[k]);  // 0x14015CDB0，读 r->key[0..3]
    return s;
}
```

→ **比较量不是 `[grade]` 本身，不是名望，不是攻击/防御单值**，而是一条 20 字节记录里
**4 个键各自查表求和**得到的分数。

---

## 5. 路径 B（批量驱动，最贴合"背包格子"）—— `0x141F9C8B0`

`0x141F9C8B0` 是**虚函数**（无任何直接 `call` 方）。虚表回推结果：
**虚表基址 `0x1498CA220`，槽位 index 7（偏移 `+0x38`）**，`vtable[-1] = 0x14BBE8430`。

```asm
; --- 逐格判定（索引 i = 0..59）---
0x141F9CED0  mov  edx, [rdi]                  ; rdi = obj + i*20 + 0x206 -> slotTable[i].part
0x141F9CED2  test edx, edx
0x141F9CED4  je   0x141F9CF58                 ; part == 0 -> 另一支（关箭头）
0x141F9CEDD  call 0x141FAA9D0                 ; MapEquipSlot(obj, part) -> 部位序号
0x141F9CEE7  (zero 20B rec)
0x141F9CEF7  call 0x141FAB940                 ; GetEquippedRec(序位, &mine)  ← 读 +0x12A 表
0x141F9CEFC  xor   r15b, r15b                 ; show = false
0x141F9CEFF  cmp   dword [rbp-0x60], 0        ; mine.part
0x141F9CF03  jle   0x141F9CF44                ; ★ <=0 -> show = true（退化分支）★
0x141F9CF1D  call  0x141FAB260                ; Score(mine)  -> ebx
0x141F9CF33  lea   rdx, [rsp+0x60]            ; = slotTable[i] 的副本
0x141F9CF3B  call  0x141FAB260                ; Score(slot)  -> eax
0x141F9CF40  cmp   eax, ebx
0x141F9CF42  jle   0x141F9CF47
0x141F9CF44  mov   r15b, 1                    ; show = true
0x141F9CF4B  mov   edx, r14d                  ; 参数2 = 格号 i
0x141F9CF4E  mov   rcx, r13                   ; 参数1 = 窗口
0x141F9CF51  call  0x141F9C7B0                ; SetSlotArrow(窗口, 格号, show)
```

伪代码：

```c
void InventoryWindow::RefreshUpArrows() {
    Ctrl* c = GetCompareCtrl();                       // 0x141FAB150（单例 0x14E634248）
    if (!c) return;
    for (int i = 0; i < 60; i++) {
        Rec20 slot;  c->GetSlotRec(i, &slot);          // obj + 0x206 + i*20
        if (slot.part == 0) { TurnOffArrow(this, i); continue; }
        Rec20 mine = {};
        c->GetEquippedRec(c->MapEquipSlot(slot.part), &mine);   // obj + 0x12A + 序位*20
        bool show = (mine.part <= 0)                   // ← 退化：该部位没有记录
                  ? true
                  : (c->Score(&slot) > c->Score(&mine));
        SetSlotArrow(this, i, show);                   // 0x141F9C7B0 -> ArrowUp.ani
    }
}
```

---

## 6. 路径 A（格子控件自身虚函数）—— `0x145A66B10`

`0x145A66B10` 是格子控件的**虚函数**，在 5 个虚表里**一致地**落在 **slot index 77（偏移 `+0x268`）**：

```
0x149D38690 (vtable[-1]=0x14BC61B50)   0x14A924448 (0x14BD9A8F0)   0x14A9B29F8 (0x14BDA41B0)
0x14A9B31B0 (0x14BDA42A8)              0x14A9B38F0 (0x14BDA42D0)
```

```
0x145A66B10  ItemSlotControl::ShouldShowUpArrow(Ctx* ctx /*r9*/)
```

```asm
0x145A66C18  mov  rax, [r15+0x18]            ; r15 = ctx
0x145A66C1C  test rax, rax
0x145A66C1F  je   0x145A66C2D                ; 空 -> r14 = NULL
0x145A66C27  mov  r14, [r15+0x20]            ; ctx->equipped
0x145A66C2D  xor  r14d, r14d
0x145A66C50  test r14, r14
0x145A66C53  je   0x145A66D71                ; ★ equipped == NULL -> 退化路径 ★
0x145A66C59  mov  r8,  [rcx+0x5a8]           ; vtbl[0x5a8]
0x145A66C63  mov  rdx, r14                   ; equipped
0x145A66C69  call r8
0x145A66C6E  je   0x145A66C9E                ; false -> 不显示
0x145A66C70  cmp  ebx, 0x2d
0x145A66C79  movabs rcx, 0x200000001087
0x145A66C83  bt   rcx, rbx                   ; 类别 ∈ {0,1,2,7,12,37}
0x145A66CAB  mov  r8d, 1                     ; mode = 1
0x145A66CB9  call 0x14500A050                ; -> UI/Inventory/Animation/ArrowUp.ani
```

退化路径 `0x145A66D71` 与主路径**唯一差别就是缺 `vtbl[0x5a8](equipped)` 这一关**，其余白名单/谓词完全相同。

`0x14500A050` 的 mode 表：

| mode | 资源 |
| --- | --- |
| **1** | **`UI/Inventory/Animation/ArrowUp.ani`** |
| 2 | `UI/Inventory/Animation/upgradeicondodge00.ani` |
| 3 | `UI/Inventory/Animation/conversionicondodge00.ani` |

其余判定（自然语言）：等级要求 `myLevel ≥ reqLevel`；物品 `field0 != 0x32`；两个可用性谓词
`vtbl[0x1c8]()` / `this+0x1c2c` 均为假；类别 `≤ 0x2D` 且命中白名单位图；末谓词 `0x145A86C50`
（按物品类别做装备类判定）通过。

`0x14E6B5BD8` 是这一路径的一个总开关（非 0 则一律不显示），但**全客户端只有 5 处引用、全是读取，
没有任何写入** → 编译期常量/构建标记（值为 0），**不可作为运行时开关**。

---

## 7. ★ 关键突破：记录表是**报文反序列化**进来的（本轮新增）

### 7.1 报文入口：`CNGameCore::NetworkProc`（`0x1459A1BB0`）

```asm
0x1459A1BB0  push rbp; push rsi; ...                    ; 函数自证身份见下方断言串
0x1459A1BE6  r15 = r8                                   ; arg4 = 报文缓冲
0x1459A1BEF  if (rcx->[8] == 0) { eax = 0x80000001; ret; }
0x1459A1BFF  edx = *(int*)(r8 + 3)                      ; ★ 报文长度取自 packet+3 ★
0x1459A1C03  rcx = r15
0x1459A1C06  call 0x146EA2120                           ; ★ ResetSource(packet, len) ★
0x1459A1C0B  call qword ptr [0x149187188]               ; 导入函数（校验/解密）
0x1459A1C17  edx = *(u8*)r15                            ; packet[0] = 子类型
0x1459A1C1D  if (edx == 0) goto 0x1459A1FBC
0x1459A1C23  if (edx == 1) goto 0x1459A1C95
0x1459A1C28  ...
0x1459A1C34  xorstr "HACK>> Network Packet Type Error [ %d ]"
0x1459A1C43  xorstr "CNGameCore::NetworkProc"
```

**函数名与源文件由断言串自证**（同区还出现 `"INIT>> Ngs init fail"`、`"CNGameCore::waitNgsInitModule"`、
`"D:\Work\Jenkins5\src\DNFClient\RDAR\GameCoreInit.cpp"`）。

### 7.2 全局"顺序读"原语：`0x146EA0BE0(dst, n)`

```asm
0x146EA0BE0  if (n <= 0) return;
             if ([0x14F1BF878] < n)  *(int*)0 = 0;        ; 越界即崩溃
             src = [0x14F1BF870]
             memcpy(dst, src, n)                          ; 0x148AA1E60
             [0x14F1BF870] += n                           ; ★ 游标前进 ★
             jmp 0x146EA0340                              ; 其内 `sub [0x14F1BF878], n` 扣减剩余
```
同族还有读 1 / 2 / 4 字节的兄弟（`cmp [limit],1/2/4`）。
`ResetSource` = `0x146EA2120`：存 `owner → [0x14F1BF880]`、`len → [0x14F1BF87C]`，
`operator delete([0x14F1BF868])` 释放旧缓冲并置空。
→ **结论：`0x146EA0BE0` 就是"从当前报文里顺序取 n 字节"的原语。**

### 7.3 三条报文处理器（各自从报文里读定长块写进控制器）

```c
// 0x141FA8CE0 —— 报文 0x914
void Handle_914(void) {
    Ctrl* c = GetCompareCtrl();                  // 0x141FAB150
    if (!c) return;
    return RebuildRecordBlock(c);                // jmp 0x141FADE40
}

// 0x141FA8CA0 —— 报文 0x915
void Handle_915(void) {
    Ctrl* c = GetCompareCtrl();
    if (!c) return;
    ReadFromPacket(c + 0x6B6, 0x120);            // 0x146EA0BE0
    return RefreshArrows();                      // 0x141FAE300
}

// 0x141FA8D00 —— 报文 0x916
void Handle_916(void) {
    Ctrl* c = GetCompareCtrl();
    if (!c) return;
    ReadFromPacket(c + 0x7D6, 0x1E0);            // 0x146EA0BE0
    return RefreshArrows();
}
```

`RebuildRecordBlock`（`0x141FADE40`，唯一入口 `0x141FA8CE0`）：

```asm
0x141FADE40  ...
0x141FADE73  for (i = 0; i < 0x0B; i++) { buf2[i].part=0; buf2[i].key=0; }   ; buf2 = &buf[4]
0x141FADEA0  for (i = 0; i < 0x3C; i++) { buf3[i].part=0; buf3[i].key=0; }   ; buf3 = &buf[0xE0]
0x141FADEB2  *(int*)buf = 0xC                                               ; count 默认值
0x141FADEC5  memset(&buf[4], 0, 0x58C)                                      ; 0x148AA2510
0x141FADED4  ReadFromPacket(&buf[0], 0x590)                                 ; ★ 0x146EA0BE0 ★
0x141FADF97  memcpy(c + 0x126, &buf[0], 0x590)                              ; 0x148AA1E60
0x141FADFB1  RefreshArrows()                                                ; 0x141FAE300
; --- 之后填两张布尔标记表 ---
0x141FADF10  if (c->slotTable[i].part == 0 && buf_slot[i].part != 0 && i <= 0x3B)
0x141FADF25      c->[0xAB + i] = 1;
0x141FADF34  if (i < 0x0B && buf_equipped[i].part != 0)
0x141FADF60      for (k = 0; k < 4; k++) if (slot.key[k] > equipped.key[k]) break;
0x141FADF7D      if (i <= 0x0A) c->[0xA0 + i] = 1;
```

> ⚠️ **注意**：`buf` 长度为 `0x590`，其中 `buf[0..3]` = count、`buf[4..0xDF]` = `equippedTable[11]`、
> `buf[0xE0..]` = `slotTable[60]`。`memcpy` 目标是 `c + 0x126`，所以
> **`c+0x126` = count、`c+0x12A` = equippedTable、`c+0x206` = slotTable** —— 与 §3 完全吻合。

### 7.4 刷新入口：`RefreshArrows`（`0x141FAE300`）

```c
void RefreshArrows(void) {
    w1 = GetUIObject(0x8bb);  if (w1) w1->vtbl[0x228](w1);
    w2 = GetUIObject(0xaee);  if (w2) w2->vtbl[0x228](w2);
}
```
（`0x148aa307c` 按 id 取 UI 对象；`0x8bb`/`0xaee` 这两个 id 同时出现在 `0x141FAB850` / `0x141FAB2B0`。）
`RefreshArrows` 共有 **10 个调用点**（`0x141FAC4C8` 内 5 处 + `0x141FA8C06`/`0x141FA8CD7`/`0x141FAB964`/`0x141FADB29`），
其中多数宿主函数**没有直接调用方** → 走虚表分派 → 由**多种 UI/网络事件**触发。

### 7.5 报文 id 登记（`0x141FA9B40`，对比控制器初始化）

```asm
0x141FA9B40  mov edi, ecx                         ; arg1 = 控制器对象
0x141FA9B42  ecx = 0x340;  call 0x146e8ba20       ; operator new(0x340)
0x141FA9B5C  call 0x1401556d0                     ; 建查找表
0x141FA9B69  [rdi + 0x48] = rax                   ; ★ 就是 Score 用的那张表 ★
0x141FA9B87  ecx = 0xB8;   call 0x146e8ba20
0x141FA9BA1  call 0x141f8f860                     ; 第二张表
0x141FA9BAB  [rdi + 0x50] = rax
; --- 四条报文登记 ---
0x141FA9BB2  lea r8, = 0x141FA8C10    edx = 0x913   → Register(dispatcher, 0x913, fn, 0)
0x141FA9BCD  lea r8, = 0x141FA8CE0    edx = 0x914   → Register(dispatcher, 0x914, fn, 0)   ★ 0x590
0x141FA9BE8  lea r8, = 0x141FA8CA0    edx = 0x915   → Register(dispatcher, 0x915, fn, 0)   ★ 0x120
0x141FA9C03  lea r8, = 0x141FA8D00    edx = 0x916   → Register(dispatcher, 0x916, fn, 0)   ★ 0x1E0
; Register = 0x14599d5d0(rcx = [0x14E66C090], edx = id, r8 = fn, r9d = 0)
```

| 登记 id | 处理器 | 载荷 | 写入位置 |
| --- | --- | --- | --- |
| `0x913` | `0x141FA8C10` | ？ | ？ |
| **`0x914`** | `0x141FA8CE0` | **0x590** | **`c+0x126`（含 `equippedTable[11]`）** |
| `0x915` | `0x141FA8CA0` | 0x120 | `c+0x6B6` |
| `0x916` | `0x141FA8D00` | 0x1E0 | `c+0x7D6` |

> ⚠️ `0x913–0x916` 是**客户端内部登记 id**（`Register([0x14E66C090], id, fn, 0)`）。
> `NetworkProc` 里 `packet[0]` 只被断言限定为 `{0,1}`，所以 id ≠ 线上 opcode；
> **id 与线上报文的映射链尚未追出**（见 §10）。这四个 id 连续，且与 `0x8bb`/`0xaee`/`0xa66`/`0x2a9` 等
> UI 对象 id 同量级，需实测确认其含义。

---

## 8. 回答交接文档 §6 的两个问题

**问题 1：箭头是按什么算出来的？**

> 格子物品先要满足"本角色可穿戴"（等级 / 部位 / 类别白名单 / 可用性谓词）；
> 然后与该部位**当前已装备物品**比一个**由 4 个键查表求和得到的分数**（`Score`，§4），
> 分数更高才显示。**不是**档次 `[grade]` 直接比较，**不是**名望，**不是**攻击/防御单值。
> 两边记录的来源：`slotTable[格号]` 与 `equippedTable[MapEquipSlot(slot.part)]`（§3）。

**问题 2：重登路径下客户端缺哪一步，导致它退化成"只标可穿戴"？**

> 缺的是**"该部位当前已装备物品"所在的记录块**——也就是**报文 `0x914` 那次 0x590 字节的反序列化**（§7.3）。
> 该块把 `equippedTable[11]` 与 `slotTable[60]` 一起写进控制器；
> 若它没跑，`equippedTable[]` 全为 0 → `GetEquippedRec()` 取回的 `mine.part <= 0`
> → 两条路径**都**落入"无基线 ⇒ 直接显示"的退化分支（`0x141F9CF03` / `0x145A66C53`）
> → 所有可穿戴装备全部点亮。
> 真实换装 / 切图之所以能修好，是因为那条路由会让处理器重新跑一次（或另一次 `RefreshArrows` 配着已填充的表）。

---

## 9. §6 结论修正：服务端**有**杠杆（本轮推翻上一轮判断）

**上一轮结论**（已作废）：*"纯客户端状态机问题，服务端没有可用杠杆"* —— 该判断建立在
"记录表由客户端本地从装备槽算出"这一**错误前提**上（误导来源是把 `0x141FAA9D0` 读成了"查找已装备物品"）。

**本轮结论**：

> 记录表（含 `equippedTable[11]`）是**从网络报文中反序列化**得到的（`0x146EA0BE0` 从 `NetworkProc`
> 设好的全局报文游标顺序取字节）。**因此服务端确实有杠杆**——数据必须由服务端送来。
> 但杠杆**不是**"补发通知帧/确认帧"，而是**让承载块载荷的那条报文在重登路由上被送达并被派发**。

同时解释了交接文档 §3 三次尝试为何无效：`NOTI13 list0+list3` / `NOTI23+NOTI24` / `CMD19 确认帧`
**都不是承载 0x590/0x120/0x1E0 载荷的那条报文**，它们只是通知/确认类帧，
既不写入控制器、也不触发 `RebuildRecordBlock`。

**修复着力点（优先级从高到低）**：

| # | 方向 | 说明 |
| --- | --- | --- |
| 1 | **让 0x914/0x915/0x916 在重登路由上被派发** | 这最符合"从根源修正"：先查服务端在重登/进城时是否发送对应报文；若已发送而客户端不处理，则是**派发**问题（登记时机 / 报文被过滤 / 窗口尚未创建） |
| 2 | 客户端补一次重读 + 刷新 | 在进城/开背包时调用 `0x141FA8CE0` 或直接 `RefreshArrows` 之前，先确保载荷已读。**属修改 `DFO.exe` 本体**，按 §8 **未经你明确要求不得动手** |
| 3 | 改退化分支（**不推荐**） | 把"无基线 ⇒ 显示"改成"无基线 ⇒ 不显示"会把"刚换上更好装备但记录未到"的场景一起打哑，属于掩盖症状 |

---

## 10. 未闭环项（不作猜测，明确标出）

- **`0x913–0x916` 登记 id ↔ 线上报文 opcode 的映射链**未追出。`NetworkProc` 只对 `packet[0] ∈ {0,1}`
  做分支，其余报文字段由各处理器自己用 `0x146EA0BE0` 顺序读，故 opcode 应是报文内某字段，
  需要继续追 `packet[1..]` 的解析或 dispatcher `[0x14E66C090]` 的表结构。
- **两处路径的具体类名**：该构建已剥离/混淆 RTTI（全文件仅 81 处 `.?AV`），
  `pe_rtti_name.py` 解出的 TypeDescriptor 为乱码。已确认的只有：报文入口自称 `CNGameCore::NetworkProc`；
  箭头控件族字符串 `CNGridArrow` / `CNCustomDownArrow` / `CNDownArrow`。
- **`RefreshArrows` 的 `vtbl[0x228]` 具体落点**：`RefreshArrows` 对 id `0x8bb`/`0xaee` 的对象调 `+0x228`。
  已确认 `0x141F9C8B0`（路径 B 驱动）在其虚表 `0x1498CA220` 的 **index 7（+0x38）**，
  与 `+0x228` 不是同一槽 —— 即 `+0x228` 是上游方法，再由它间接触发驱动，这一段未逐步展开。
- `0x141FAC4C8`（5 处调用 `RefreshArrows`）的虚表归属未确认。

---

## 11. 建议的下一步（按性价比排序）

| # | 方案 | 说明 |
| --- | --- | --- |
| 1 | **服务端取证（最快、且与 §9 结论一致）** | 抓一份"重登进城"和一份"换装后"的收发包，比对是否出现承载 0x590/0x120/0x1E0 载荷的那几条报文。这是唯一能直接判定"服务端该不该补"的动作，且**不需要动客户端** |
| 2 | **静态追 id ↔ opcode 映射** | 用 `analysis/dumps/rip_xrefs.tsv` 追 dispatcher `[0x14E66C090]` 的表结构，以及 `Register` 入参 id 的生成式 |
| 3 | **动态验证** | 在 `0x141F9CF03`（退化分支）/ `0x141FA8CE0`（0x914 处理器）下断或 hook，分别读 `mine.part`、`c+0x126` 的 count、`equippedTable[0].part`。项目 Python 已装 frida；**是否注入由你决定**，且需你手动启动客户端（§8） |
| 4 | 若接受改本体 | 见 §9 方案 2/3，**需你明确授权**，交付 `.exe` 改动说明与回滚方式 |

**实机验证须由你操作**（§8：不得无人值守启动客户端）。需要我做哪一步，告诉我即可。

---

## 12. 产物清单

| 路径 | 说明 |
| --- | --- |
| **工具** | |
| `analysis/tools/xorstr_neighbors.py` | 按**地址邻居**读 xorstr 表，判断字面量归属模块（证伪 §1 三个猜测全靠它） |
| `analysis/tools/pe_ptr_scan.py` | 扫绝对指针 → 找函数所在虚表槽位 |
| `analysis/tools/pe_rtti_name.py` | 走 MSVC RTTI 链取类名（本构建 RTTI 被剥离，留作他用） |
| `analysis/tools/pe_build_xrefs.py` | 建全量 .text RIP 相对引用索引（带失步恢复） |
| **`analysis/tools/pe_disp_scan.py`** | **本轮新增**：按"指令形状"扫固定位移（`0F 11 <ModRM> <SIB> <disp32>`）——用它证明了**不存在**对 `+0x12A`/`+0x206` 的直接 store，从而把排查方向逼到 memcpy/报文读取上 |
| **索引产物** | |
| `analysis/dumps/rip_xrefs.tsv` | 全量 RIP 引用索引：**40,681,977 条指令 / 1,355,582 条引用 / 97 MB**。查询：`awk -F'\t' '$1=="0x<VA>"' rip_xrefs.tsv`（**必须带制表符，否则前缀误匹配**） |
| **反汇编** | |
| `analysis/tasks/next47/fn_145A66B09_mark_decide.asm` | 路径 A 判定函数 |
| `analysis/tasks/next47/fn_14500A045.asm` | ArrowUp/upgrade/conversion 三态图标选择器 |
| `analysis/tasks/next47/fn_141F9C8B0_arrow_driver.asm` | 路径 B 批量驱动 |
| `analysis/tasks/next47/fn_141FAB260_score.asm` | 分数函数 + 同区辅助 |
| `analysis/tasks/next47/writer/writer_fn.asm` | `0x141FADE40` 记录块重建（含 0x590 报文读 + memcpy + 两张标记表） |
| `analysis/tasks/next47/tbl/at_0x*.asm` | 6 个接触记录表的函数窗口 |
| `analysis/tasks/next47/mod/`、`analysis/tasks/next47/helper/` | 模块内函数与 helper 反汇编 |

未修改 `client/` 下任何文件；大资产未入库。

---

## 13. 附：本次踩坑（避免下次重复）

1. `pe_disasm.py --window` 是**十进制**解析（`type=int`），传 `0x140` 会直接报错退出；
   `--at X --context` 的窗口是 `±window` 字节，**必须传十进制**。
2. `grep -i "^0x14E634248"` 会**前缀误匹配** `0x14E6342480` 等地址（一次捞出 828 KB）。
   查索引必须用 `awk -F'\t' '$1=="0x..."'`。
3. `pe_disasm.py --start X --out` 的结束地址来自 `find_function_bounds()` 前向启发式，
   **会截短**（曾只吐出 7 行）。要看完整函数用 `--at X --context --window <大值>`，
   或显式 `--start/--end`。
4. `find_callers`/`--callers` 只扫 `E8`/`E9` rel32，**抓不到虚表分派**；
   而 `rip_xrefs.tsv` 只收 RIP **内存操作数**，**抓不到直接 `call rel32`**。
   两者互补，缺一不可。
5. 手写 PE 节头解析时字段顺序是
   `Name[8], VirtualSize, VirtualAddress, SizeOfRawData, PointerToRawData`
   （从 `h+8` 起 `unpack("<IIII")`）。顺序写错会得到**空结果**而非报错——曾因此误判"0x590 不存在"。
6. capstone 线性反汇编遇非法字节会**静默停止**，必须"从最后消费位置重启；零指令则 `pos += 1`"。

---

# 14. 追加取证（第二轮）：报文族边界、注册表机制、回调时序

> 本节是对 §7 的**补充与修正**。§7.3 说"三条处理器"，实际是**四条**，且 §7.4 的"刷新入口"在
> **每一条**处理器尾部都被调用——这直接**排除了"客户端刷新时序错"这一类假设**。

## 14.1 报文 id 登记（修正 §7.5）

`0x141FA9B40` 处依次登记 4 个处理器（`Register` 目标 `0x14599D5D0`）：

| id | 处理器 | 读取位置 | 长度 | 说明 |
|---|---|---|---|---|
| `0x913` | `0x141FA8C10` | `c + 0x0F4` | `0x8EF` | **全量快照**（覆盖下面三块） |
| `0x914` | `0x141FA8CE0` → `0x141FADE40` | `c + 0x126` | `0x590` | 块 1：count + equippedTable[11] + slotTable[60] |
| `0x915` | `0x141FA8CA0` | `c + 0x6B6` | `0x120` | 块 2 |
| `0x916` | `0x141FA8D00` | `c + 0x7D6` | `0x1E0` | 块 3 |

**四段偏移首尾相接、无缝隙**——这是"同一记录区的全量 + 分块"结构的铁证：

```
0x0F4 ──0x8EF──▶ 0x9E3        (0x913 全量)
0x126 ──0x590──▶ 0x6B6        (0x914 块1)
0x6B6 ──0x120──▶ 0x7D6        (0x915 块2)
0x7D6 ──0x1E0──▶ 0x9B6        (0x916 块3)
0x9B6 ──  int ──▶ 0x9BA  ⇒ 对象实际尺寸 ≥ 0x9E8（**修正 §3 的"对象 0x9C0"**）
```

⇒ **`equippedTable`（`c+0x12A`）与 `slotTable`（`c+0x206`）只能由 `0x913` 或 `0x914` 写入，二者必居其一。**

## 14.2 ★ 回调时序：四条处理器**全部**在读完立刻刷新箭头 ★

| 处理器 | 尾部行为 |
|---|---|
| `0x913` | `0x141FA8C3C  call 0x141FAE300`（`RefreshArrows`） |
| `0x914` | `0x141FADFB1  call 0x141FAE300`（在 `memcpy` 落表**之后**） |
| `0x915` | `0x141FA8CCC  jmp  0x141FAE300`（尾调用） |
| `0x916` | `0x141FA8D2C  jmp  0x141FAE300`（尾调用） |

**结论（重要）**：只要报文**到达**，箭头就会在**同一次调用内**被重算。因此"箭头显示错误"**不可能**
是"数据到了但没触发刷新"造成的。

⇒ 剩下只有两种可能：**(A) 报文没到达**、**(B) 报文到达了但载荷里的 `equippedTable` 就是 0**。
两者都指向**数据源**，不指向客户端渲染。

## 14.3 `0x141FADE40` 真身：`LoadEquipBlock`（修正 §7.3 的名称与语义）

之前叫它 `RebuildRecordBlock`，读完完整函数体后应更正为 **`LoadEquipBlock(ctrl)`**。
它**不是**做过滤重建，而是"读报文 → 落表 → 挖变更标记 → 刷新"：

```c
void LoadEquipBlock(Ctrl* c) {                        // 0x141FADE40 .. 0x141FAE02B
    Rec20 srcEquip[11], srcSlot[60];                  // rsp+0x24 / rsp+0x100
    memset(&srcEquip, 0, 0x58C);                      // 防御性清零（Read 会全覆盖）
    int blkCount = 12;  struct {int count; ...} blk;
    blk.count = 12;                                   // [rsp+0x20] = 12
    Read(&blk, 0x590);                                // 0x141FADED4 → 0x146EA0BE0，从报文游标整块读
    if (blk.count >= 1 && blk.count <= 60)            // 0x141FADEDD:  lea eax,[rcx-1]; cmp eax,0x3b; ja
        for (i = 0; i < blk.count; i++) {
            if (c->slotTable[i].part == 0 && srcSlot[i].part != 0 && i <= 59)
                c->[0xAB + i] = 1;                    // 格子"空→有"变更标记
            if (i < 11 && srcEquip[i].part != 0)
                for (k = 0; k < 4; k++)
                    if (srcEquip[i].key[k] > c->equippedTable[i].key[k]) {
                        c->[0xA0 + i] = 1;  break;    // 已装备项"变强"变更标记
                    }
        }
    memcpy(c + 0x126, &blk, 0x590);                   // 0x141FADF9E ★原样整块落表，零过滤★
    RefreshArrows(c);                                 // 0x141FADFB1
    if (countOf(slotTable[i].part != 0) >= c->count)  // 0x141FAE002
        Notify(0x141FAE1F0);                          // 满格额外通知
}
```

**三点关键含义**

1. **落表是 `memcpy` 原样搬运，没有任何按条件丢弃**（`§3` 曾怀疑的"过滤"不存在）。
   ⇒ `equippedTable` 全 0 **只可能**因为**报文本体里就是 0**。这是"服务端问题"的**直接证据**。
2. `srcEquip[i].key[k] > c->equippedTable[i].key[k]` 这条比较，**反证了 `key[4]` 是"可比大小的量"
   （物品 id 或权重）**，与 §4 `Score = Σ Lookup(key[k])` 自洽；也说明 `Rec20 = {part, key[4]}`
   里 `key[4]` 是 4 个并列条目（如 主装备 + 3 个附魔/镶嵌）。
3. `c->[0xA0 + i]`（11 项）与 `c->[0xAB + i]`（60 项）是**变更标记数组**，
   尺寸恰好等于 `equippedTable[11]` / `slotTable[60]`，仅供 UI 做"箭头动画/高亮提示"。

## 14.4 `0x913` 全量处理器额外逻辑（新解出）

```c
void OnEquipFullSnapshot(void) {                       // 0x141FA8C10
    Ctrl* c = GetCtrl();  if (!c) return;              // 0x141FAB150
    Read(c + 0xF4, 0x8EF);                             // 0x141FA8C34
    RefreshArrows(c);                                  // 0x141FA8C3C
    if (c->[0x70] == 0) {                              // 一次性初始化
        int v = c->[0xF4];                             // 块首字段
        c->[0x70] = 1;
        c->[0x60] = (v == -1) ? 0 : v;
    }
    if (c->[0x9E2] != 0 && g_14E683C78 != 0)           // 0x141FA8C5C
        SendEvent(0xB3B, ...);                         // 0x141FA8C80 tailcall 0x14668C520
}
```

## 14.5 注册表机制：`hash_map<u32, handler>`（新解出，用于否定性结论）

`Register` 目标 `0x14599D5D0` → 实插 `0x1459A3270`，后者对 **4 字节 id 做 FNV-1a 64**：

```
r10 = (id & 0xFF)     ^ 0xcbf29ce484222325 ; r10 *= 0x100000001b3
rdx = ((id>>8)  & 0xFF) ^ r10              ; rdx *= 0x100000001b3
rcx = ((id>>16) & 0xFF) ^ rdx              ; ... *= 0x100000001b3
rax = ((id>>24) & 0xFF) ^ ...              ; ... *= 0x100000001b3
```

- 表对象单例在 `0x14E6836F8`；表对象仅被 `0x14599xxxx–0x1459A3xxx` 这一小片代码引用
  （全文件仅 **12 处** RIP 引用），说明是一个**自封闭的全局命令注册表**。
- 全表约 **1166** 个登记项（`--callers 0x14599D5D0` = 1166，含 `jmp` 形式），
  与 DNF 报文种类量级一致。
- **否定性结论**：在 `.text`/`.rdata`/`.data` 全量搜索
  `13 09 00 00 14 09 00 00 15 09 00 00 16 09 00 00`（u32 连续）与 `13 09 14 09 15 09 16 09`（u16 连续）
  **均为 0 命中**。⇒ `0x913..0x916` 是**逐个字面量插入的哈希键**，
  客户端**不存在**一张"连续 id ↔ 线上 opcode"的映射表。
  ⇒ 该映射要么在**发送方**构造，要么在传输层（`NGClient64.aes` / `BlackCipher`，已加壳）内完成。

---

# 15. 取证路径评估与风险（本轮新增，重要）

## 15.1 环境核查结果

| 项 | 实测 | 影响 |
|---|---|---|
| `DllCharacteristics` | `0x8120` → **DYNAMICBASE = False** | **镜像固定加载在 `0x140000000`，无 ASLR**。绝对地址即运行时地址，hook 无需重定位计算 |
| `.themida` 节 | 存在，**12,083,200 字节** | 二进制带 **Themida 壳**（代码虚拟化 + 常见反调试/反注入） |
| 反作弊 | `BlackCipher/` 目录：`BlackCipher64.aes` 31.9 MB、`BlackCall64.aes` 23.4 MB、`BlackXchg.aes` 8.8 MB、`NGClient64.aes`、`CrashReporter_64.dll` | **nProtect GameGuard (BlackCipher) 反作弊在位** |
| 反作弊日志时间戳 | `BlackCipher64.log` / `BlackCall64.log` / `NGClient64.log` 均为 **2026-09-21 16:23** | **本次客户端运行时反作弊是激活的**，不是空壳 |
| `frida` | 项目 Python 内 `frida 17.18.0` + 全套 CLI（`frida.exe` 等） | 工具就绪 |
| 客户端日志 | `LagLog.txt` 仅含 FPS/延迟统计；`cef.log`、`Patch.trc` 无关 | **零注入的日志取证路径不可用** |

## 15.2 三条取证路径的可行性判定

| 路径 | 能否得到答案 | 主要风险 |
|---|---|---|
| **A. 网卡抓包（pcap/Npcap）** | ❌ **基本不可用** | 传输层经 `NGClient64`/BlackCipher 加解密 + 混淆；抓到的长度与载荷**无法与 `0x590` 对齐**，只能看包频/包量等弱信号 |
| **B. 进程内 hook（frida / 调试器）** | ✅ 能**直接**得到"A 未到达"还是"B 载荷为 0" | ⚠️ **需注入进程**，而进程内有 **BlackCipher（nProtect）+ Themida**。轻则被拒绝注入/进程退出，重则**触发反作弊记录**。**不建议在联机正式环境做** |
| **C. 服务端侧取证** | ✅ 能直接闭环，且**零客户端风险** | 需要服务端代码/发包表（你已有 `NOTI13`/`NOTI23+24`/`CMD19` 等命名，说明可及） |

**结论：优先走 C。** 且 §14.3 已给出**可直接用于服务端检索的指纹**：
"一条**载荷长度恰为 `0x590`** 的报文，其首 4 字节是 `count`（取值 1–60，满格时等于当前格数），
紧随其后是 11 个 `{int part; int key[4];}`（`part` 取内部部位码 `0x0E–0x19`）
再是 60 个 `{int part; int key[4];}`"。这个形状在服务端 `struct` 里很好认。

## 15.3 若仍要做 B，唯一相对安全的前提

仅在**满足全部三条**时才建议：① 单机/离线测试环境；② BlackCipher 已停用或未加载；
③ 账号为一次性测试号。此时可用的最小探针（**本文件不生成脚本，需你显式要求**）：

| 探针地址 | 读什么 | 判读 |
|---|---|---|
| `0x141FA8CE0`（0x914） | 是否被调用 | 到达=否 → 服务端未发（结论 A） |
| `0x141FADFB1` 后读 `c+0x126`/`c+0x12A` | `count`、`equippedTable[0].part` | `part==0` → 载荷为 0（结论 B） |
| `0x141FA8C10`（0x913 全量） | 是否被调用 | 判断全量快照是否随进城下发 |
| `0x141F9CF03`（退化分支） | `mine.part` | 直接看到"退化"发生的时刻与次数 |

---

# 16. 本节新增的未闭环项

1. **`0x913..0x916` ↔ 线上 opcode / 服务端 `NOTI_*`/`CMD_*` 命名** 仍未打通。
   已证明客户端侧**不存在**该映射表（§14.5），故该工作**必须在服务端侧或传输层做**。
2. `0x9E2`（`0x913` 处理器里的门控字节）语义未定；疑为"是否已进入村庄/是否完成初始同步"。
3. `0x141FAA700` / `0x141FAA2B0` / `0x141FAA4A0`（被日志函数写入 `c+0x68`/`c+0x6c`）语义未定，
   疑为"总攻击/总防御/名望"之类的汇总值。
4. 对象真实尺寸（≥ `0x9E8`）与 `0x9BA` 之后字段未展开。

## 16.1 本轮踩坑补充

7. `--callers` 会命中**函数中间的地址**（如 `0x14599D5D0` 落在 `0x14599D4E0` 函数体内），
   必须用 `--at` 复核函数边界；否则会把 1166 个调用点误当成"1166 次报文注册"。
8. 查全局变量引用时先确认列序：`rip_xrefs.tsv` 的列序是
   **`target_va, insn_va, bytes, insn`**（`$1` 是 target，`$3` 是字节码）。用错列会得到 0 命中。
9. 判定"表由报文写入"这类问题时，**"只对常量做字节搜索"比追函数调用链更快**
   （搜 `movabs 0x100000001b3` 得 122 处噪声；而搜连续 id 序列一次得到干净的**否定性结论**）。

---

# 第三轮（2026-09-21 17:xx–18:xx）：服务端取证 → opcode 闭环 + 模块身份存疑

## 17. opcode 映射闭环（关闭 §10 未闭环项）

`analysis/dumps/opcodes.tsv` 直接命中（该表来自客户端自带枚举名表）：

| id | hex | 枚举名 |
| --- | --- | --- |
| 2323 | 0x913 | `ENUM_NOTIPACKET_EVENT_EPIC_GROWTH_SIMULATOR` |
| 2324 | 0x914 | `ENUM_NOTIPACKET_EVENT_EPIC_GROWTH_SIMULATOR_INVENTORY`（← 0x590 载荷） |
| 2325 | 0x915 | `ENUM_NOTIPACKET_EVENT_EPIC_GROWTH_SIMULATOR_BOOK` |
| 2326 | 0x916 | `ENUM_NOTIPACKET_EVENT_EPIC_GROWTH_SIMULATOR_MISSION` |

与注册函数 `0x141FA9B40` 尾部的 xorstr `"epic growth simulator"`（0x141FA9C61）**独立互证**。
0x590 载荷自洽：`4 (count) + 11×20 (equipped) + 60×20 (slots) = 1424 = 0x590`。

## 18. 第二条数据通道：事件信封 681 + kind 0xA66

增量子命令分派器 `0x141FAC5E0`（19 路跳转，读包游标 `0x146EA0BA0`）的唯一调用链：

```
0x145252FA0  CMD_Etc 处理器（断言串 GameCoreNetwork_CMD_Etc.cpp）
             注册 id 0x2A9=681  ←→ cmd 681 = ENUM_CMDPACKET_EVENT_REQUEST
  └─ 体内首 u32 = kind（从游标读入 [rbp-0x39]，非顶层 opcode）
       └─ kind == 0xA66 → GetCompareCtrl(0x141FAB150) → 0x141FAC5E0
            子命令：单条写 equippedTable/slotTable（索引校验 ≤0xA / ≤0x3B）、
                    直接写 count(c+0x126)、汇总 equipped[11] 4 项属性到 c+0x74
```

由此前"未知"的写入点 `0x141FAC7BE` / `0x141FACA21` 等确认（`pe_lea_scan.py` 新工具扫出）。

## 19. 写入方全集闭环（三重扫描 + 访问器 9 调用方逐一核验）

记录块（`+0x126 / +0x12A / +0x206 / +0x6B6 / +0x7D6 / +0x9B6`）的全部写入方：

| 写入方 | 触发 |
| --- | --- |
| `0x141FA8C10`（2323，0x8EF 快照） | NOTI |
| `0x141FADE40`（2324，0x590，唯一调用方=0x914 处理器） | NOTI |
| `0x141FA8CA0` / `0x141FA8D00`（2325/2326） | NOTI |
| `0x141FAC5E0` 19 路子命令（kind 0xA66） | CMD_Etc 681 信封 |
| `0x141FA8500` ResetAll（仅清零，count=0xC） | 初始化 |

`0x141FAB1D0`（返回基址）的 9 个调用方逐一核验：除上述外全部是**读者**
（含 `0x141FA7F1C` 把 0x590 memcpy 出来供 UI 合成用，方向=读）。

## 20. ★ Go 服务端取证结果：三条通道全无 ★

| 检索 | 结果 |
| --- | --- |
| `2323|2324|2325|2326|EPIC_GROWTH|epic growth` | 0 命中（仅密表噪声） |
| `2662 / 0xA66` | 0 命中 |
| `0x02A9 / 681`（精确） | 0 命中 |

Go 服务端**从未**向客户端发送 2323–2326、681 信封、kind 0xA66。

## 21. ★★★ 由此产生的逻辑矛盾 → 模块身份存疑 ★★★

若本模块就是物品栏"↑ 更好装备"箭头的判定者：

- 表只能由 2323–2326 / 681+0xA66 写入（§19 已闭环）；
- Go 服务端从不发送（§20）；
- ⇒ 表永远全 0 ⇒ 箭头应当**永远"全亮"**；

但实机对照（交接文档）：**换装/切图后箭头立刻变正确**。
两者不可同真。

结合 681 = `EVENT_REQUEST`、模块字符串 = "epic growth simulator"（史诗成长模拟器**事件**），
最简洁的解释是：

> **本模块是"史诗成长模拟器"事件 UI 的对比系统，不是物品栏箭头的判定者。**
> 行为巧合相似（可穿戴前置 + Score 比较 + 空表退化全亮）使前两轮锚定了错误模块；
> 三次入口补帧尝试全部无效也因此完全说得通——补的全是无关通道。

**此前 §9"服务端有杠杆"的结论对本 BUG 而言再次存疑**（对该事件模块本身仍成立）。
§13/§14/§15 中所有"物品栏箭头 = 本模块"的表述暂按本节修正理解。

## 22. 下一步（修订 §11）

| 选项 | 内容 | 说明 |
| --- | --- | --- |
| **A（推荐）** | 回到交接文档 §5 原始锚点（`spec_slot_arrow` / `up_arrow` / `down_arrow`）重新定位**真正的物品栏箭头**绘制/判定函数 | 前两轮对 §5 的"证伪"是在本模块内做的，需复核是否被错误模块带偏 |
| B | 一次动态探针一锤定音：hook `0x141FA8CE0`（2324 处理器）入口 + `0x141F9CF03`（退化分支） | 若重登/换装时处理器从不触发 → 证实"错误模块"假设；需用户手动启动客户端，是否注入由用户决定 |
| C | 服务端实现 2324 快照或 681+0xA66 信封 | **暂缓**——按 AGENTS.md 禁止猜包 + C2S 三次上限已用完，且 A/B 未闭环前这修的很可能是错误系统 |

## 23. 第三轮踩坑补充

10. **枚举名冲突时查数据来源**：`0xA66` 先被当成 NOTI opcode（枚举表叫
    `SEMI_RAID_BIDDING_JOIN_STATE`，对不上）；追到 `[rbp-0x39]` 是**从包游标读出的
    体内 kind** 才解开——顶层 id 和体内 kind 是两个空间，数值撞车（0xA66 同时是
    noti 2662 的 id）纯属巧合。
11. **行为相似 ≠ 模块同源**：判定条件逐字节吻合（可穿戴 + Score + 空表退化）仍可能是
    另一个 UI 的巧合。用"服务端从不喂它，现象却会自愈"这类**运行时必然性推理**
    反证模块身份，比继续读汇编更省力。
12. 常量位移写入扫描要同时覆盖 SSE store（`pe_disp_scan.py`）与 GPR
    lea/mov/add（`pe_lea_scan.py`，capstone 逐命中校验解码）——漏掉后者就漏掉
    `lea rcx,[obj+0x12A]; call memcpy` 形态的写入方。

---

# 第四轮（2026-09-21 晚）：模块身份定案 + 真数据链 + 实机验证方案

## 24. ★ 模块身份存疑解除（§21 关闭）★

**方法论修正**：§1 对交接文档 §5 锚点的"证伪"用的是**邻居法**，它只能证明字符串
"属于哪个面板"，**不能证明该字符串不参与箭头判定**——逻辑缺口。本轮改用
`rip_xrefs.tsv` 直接找**代码引用方**：

| 锚点 | 引用方 | 判定 |
| --- | --- | --- |
| `ArrowUp.ani` `0x1498CA450` | `0x141F9C821`（= `SetSlotArrow` 内）**和** `0x14500A116` | ★ 两处都在我们已分析的模块 |
| `spec_slot_arrow` `0x14974BBB0` | 仅 `0x1418A4260`（推荐装备面板） | 排除维持 |
| `is equipped` 族 `0x1491ADEE0/ADF40/ADF70/AE1A8` | `0x14016C3xx`/`0x140171Axx`（物品查询键注册表） | 排除维持 |
| `fame_txt` `0x1491DC6C0` | 含 `0x141F8C51F`/`0x141F9B38A`/`0x141FA18C1`（本模块） | 印证 |

**决定性证据**：`0x14500A050`（`SetGridOverlay`）解出——格子覆盖图标设置器，
`kind` 1=ArrowUp.ani / 2=upgradeicondodge00 / 3=conversionicondodge00，
写 `slot+0x7B0`、状态缓存 `slot+0x7C0`。其**唯一调用方** `0x145A66CB9` 属于
`0x145A66B10` = **路径 A**。→ 路径 A 就是真格子箭头写入者，§21 的
"史诗成长模拟器巧合"假设**不成立**：该模块同时服务对比面板与格子箭头，
2323–2326 只是它的可选数据源之一。

## 25. ★ 真格子箭头完整数据链（本轮解出）★

```
格子重绘 0x145A66830
  ├─ 闸门: [0x14E66C090]+0x11B0 != 0 → 跳过 (0x1459AB5E0)
  ├─ mgr = GetManager()                    ; 0x145EFAFB0, 角色状态单例
  │        (全局 0x14EF2CA80/88/90, owner 字段 +0x110, 返回 ptr-0x30)
  ├─ obj = mgr->vtbl[0x1F08](格子物品id)   ; ★按物品 id 查"对应部位已装备对象"★
  │        (物品 id 来自 slot->vtbl[0x410], 空槽默认 0x1A)
  ├─ ctx = {1, …, shared_ptr 容器, obj}    ; 栈上构造 @rsp+0x28
  └─ slot->vtbl[77] = 0x145A66B10 (路径 A)
       ├─ 等级比较: mgr->vtbl[0xAB8]() vs 0x145008DB0(slot)
       ├─ item->field0 == 0x32 → 不显示; vtbl[0x1C8]/slot+0x1C2C 可用性谓词
       ├─ ctx->equipped == NULL 或 [容器+8]==0 → ★退化: 跳过比较直接放行★ (0x145A66D71)
       ├─ "更好"比较: slot->vtbl[0x5A8](格子物品, equipped)
       ├─ 类别白名单 bt 0x200000001087 (位 0,1,2,7,12,37), 类别 ≤ 0x2D
       ├─ 装备类判定 0x145A86C50
       └─ SetGridOverlay(0x14500A050, kind=1) → ArrowUp.ani
```

**关键修正（推翻第三轮的隐含假设）**：`ctx->equipped` 的容器 count 在 `+8`，
而对比控制器记录块 count 在 `+0x126` —— **对不上**。即路径 A 用的
"已装备"来自**角色状态 manager 的本地查询**（其数据由 NOTI13/14 等角色/背包
报文本地喂养），**不是**直接读 2323–2326 那张表。这解释了第三轮的核心矛盾：

> 服务端从不发 2323–2326/681，箭头却会"换装后自愈"——因为自愈动力来自
> NOTI13/14 刷新的本地装备数据，而非 epic-growth 报文。

## 26. 服务端侧实证（runtime 会话资产，无需抓包）

`server/work/dfo-lan/runtime/` 保留真实会话的 `events.jsonl`（解密帧级日志）：

| 会话时刻 (9/20) | 进城 `worn_equipment_restored`(id13) | `worn_equipment_visuals_restored`(id14) |
| --- | --- | --- |
| 20:36 – 22:54 | **3 字节 `030000`（空！）** | — |
| 23:14 起 | **1113 字节真数据**（`0306000c00…`，space=3 + 6 行） | 1089 字节 |

即 9/20 深夜服务端已修过"进城 worn 空载荷"一版（`inventory.WornPayload` /
`WornSpaceUpdate` → `EquipmentPayload(3, b.Worn, …)`）。但交接文档称 9/21 仍复现，
说明：要么 9/21 的复现观察早于修复生效，要么本地装备数据之外仍有缺口
（如 worn 行 `Slot uint16` 与客户端内部部位码 `0x0E–0x19` 的映射、或 id14 窗口
行格式），**须实机一次定案**。

## 27. 实机验证方案（一次会话定案，用户操作）

**最小方案（零注入，优先）**：跑一次会话并复现：
1. 重登 → 进城 → 开背包看箭头（记录哪些格子亮）；
2. 随便换一件装备 → 看箭头变化（应自愈）；
3. 结束后把 `runtime/` 新目录交给 AI——`events.jsonl` 会自动记录进城时序与
   id13/id14 载荷，比对 §26 表格即可判定进城 worn 是否仍异常。

**增强方案（团队已有 x64dbg 断点工作流，RVA = VA − 0x140000000）**：

| RVA | 位置 | 观察什么 |
| --- | --- | --- |
| `0x5A66A6E` | ctx 构造完的调用点 | `[rsp+0x48]`(equipped obj)、`[rsp+0x40]`(容器)；重登时是否为 0 |
| `0x5A66B10` | 路径 A 入口 | r15(ctx)；单步看 r14 是否为 0 |
| `0x500A050` | SetGridOverlay | r8d(kind)：重登 vs 换装后的取值序列 |
| `0x5A66D71` | ★退化分支★ | 命中即证明"equipped 缺失 → 全亮" |

判据：重登进城时 `0x5A66D71` 命中且 `[rsp+0x48]==0`、换装后两者翻转 → **定案**；
再据此回服务端补缺口（大概率是进城时序或 worn 行字段），客户端零改动。

## 28. 第四轮踩坑

13. **邻居法只能归属字符串，不能否定参与**："证伪"交接锚点时要补代码引用方
    （`rip_xrefs.tsv` 查 `.rdata` 地址的 `lea` 引用）才算闭环。
14. **手写 PE 解析脚本里 `tbase` 混用 RVA/VA**：节表里的 `VirtualAddress` 是 RVA，
    直接当 VA 用会得到"全部命中落在低地址"的假象（真 VA = 命中值 + ImageBase）。
    快速自查：命中值 < ImageBase 必然错了。
15. **间接虚调用定位法**：`call [reg+disp32]` 全文件命中太多时，用**调用约定参数
    指纹**过滤（如本例 r9=ctx 必须在调用前装载，`mov/lea r9`），8/143 一次筛出；
    注意 `lea` 也算装载、窗口放宽到 ~20 条指令，且 SIB（rsp 基址）形态要单列。
16. **编译器把调用方放在被调函数紧邻前面**：`0x145A66A6E` 与 `0x145A66B10`
    只差 0xA2——同类内虚函数的"调用方就在旁边"值得先试。
17. **私有服务端项目自带 runtime/ 会话资产**（fixture.json + events.jsonl +
    client_trace.txt，含解密帧 hex）——查"服务端到底发没发 X"先翻它，
    比读代码和抓包都快；且历史会话能直接看到"哪天修了什么"。

---

# 第五轮（2026-09-21 19:35）：实机验证定案 + 服务端修复落地

## 29. 实机验证结果（用户执行，零注入）

用户按 §27 最小方案操作：重登进城 → 开背包 → 更换青铜护腿。

| 时刻 | 观察到的箭头状态 |
| --- | --- |
| 进城后开背包 | **第一行装备全部显示 ↑ 箭头**（= equipped 状态缺失，退化分支全亮） |
| 更换第三格青铜护腿后 | **箭头全部消失**（= equipped 状态就位，比较正常执行，背包装备均不比身上强） |

与 §27 预判的判据完全吻合：重登时 equipped 缺失 → 全亮；一次装备变动后翻转。

## 30. runtime 会话取证（19:32 会话，定案证据）

`roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260921_193234_186109_next37/events.jsonl`：

- 进城时序：`inventory_restored`(行47) → **`worn_equipment_restored` id13 1668B(行49)** → `enter_gameworld_complete` id124(行53) → … → `worn_equipment_visuals_restored` id14 1632B(行61)。
  **载荷非空且结构正确**（头 `03 09 00` = space 3 + count 9，行 185B，slot 码 0x0C/0x0F/0x12/0x13/0x15…）——服务端确实发了，但客户端没吃。
- 换装时序（行215–223）：id19 move echo → id13 bag resync → **id13 worn resync（同一 1668B 载荷）** → id14 增量×2（`0001000b00…`/`0301001000…`，正好是被交换的两个槽位）→ id14 window refresh → id2 → id14 visuals。

**根因**：进城时 worn 快照发在 **entry/actor 初始化屏障之前**（`enter_gameworld_complete` 之前，全部事件 `client_acceptance: pending`），箭头对比管理器此时还未安装，早到的数据被丢弃/重置；换装时刻的所有报文都在屏障之后，客户端正常消化 → 自愈。
这与本项目时装列表踩过的坑**同构**：`entry_flow.go` 中 `avatar_inventory_restored` 的注释原文就写着 "The client accepts authoritative avatar rows only after the town actor/UserInfo graph has been installed"，因此时装列表早已被移到屏障后发送，而 worn 快照没有。

## 31. 修复（服务端单点，客户端零改动）

`cmd/wireprobe/entry_flow.go` `packets()`：在屏障后的 `actor_appearance_ready` 与 `worn_equipment_visuals_restored` 之间**补发一次 id13 worn 快照**（`worn_equipment_restored_after_barrier`，载荷复用 `p.Worn`），顺序镜像换装流（worn 在前、visuals 在后）。屏障前的原发送保留（供 avatar/外观管线消费）。

- 编译验证：`go build ./cmd/wireprobe/` 通过（exit=0）。
- 验证方法：重跑一次会话，重登进城开背包——若箭头直接呈现正确状态（不全亮），定案。

## 32. 第五轮踩坑

18. **"服务端发了" ≠ "客户端收到了"**：验证数据链时必须同时核对（a）载荷内容（b）**发送时机相对客户端初始化屏障的位置**。本项目已两次踩同一坑（avatar 列表、worn 快照），规律是"权威列表类数据必须在 enter_gameworld_complete 之后重发"。
19. **一次换装 = 天然的自愈实验**：对比"坏状态时刻"与"好状态时刻"的两批报文（runtime/events.jsonl 行级 diff），差集就是修复动力来源——比任何静态推理都快。

## 33. 第二次尝试：镜像换装全序列（19:47）

19:44 会话证实：`worn_equipment_restored_after_barrier` 已发出但箭头仍全亮 → id13 worn 单独重发不构成修复动力。
载荷尺寸对齐（19:32 会话）：`equipment_bag_resynced`(2358B) ≡ `inventory_restored`(2358B，同构建器)；`equipment_worn_window_refreshed`(1632B) ≡ `worn_equipment_visuals_restored`(1632B，同 `WornSpaceUpdate`)。
即换装时刻相对进城多出的要素 = **bag list0 重同步** + **id2 外观块在所有行之后**（equipment_flow.go C9 注释："rebuild 必须把 id13/id14 行当新对象看到，外观放最后"；进城原本外观在行之前）。
改动：`packets()` 屏障后改为 bag list0 → worn id13 → worn window id14 → `actor_appearance_ready` → 2827。已重编 `bin/wireprobe-handoff-source.exe` 并以新事件名字符串校验。
若仍失败，停止报文侧猜测，走 §27 增强：x64dbg 断 `0x5A66A6E`/`0x5A66D71`，直接观察 equipped 装载时刻与来源。

## 34. 第三次尝试失败 → 转入动态（19:53）

镜像换装全序列（bag list0 + worn + window + 外观后置）后 19:53 会话仍全亮。报文侧假设穷尽：
修复动力很可能是**客户端本地处理装备移动（CMD19 apply）时的自身状态重建**，而非这批报文内容。

转入 §27 增强方案。已解出关键调用点精确地址：

```
0x145A66883  call 0x145EFAFB0            ; GetManager
0x145A66906  mov  rsi, [rax+0x1F08]      ; manager 查询函数
0x145A66913  call [rax+0x410]            ; slot->vtbl[0x410] 取物品 id（NULL→edx=0x1A）
0x145A66936  call rsi                    ; ★断点A★ rax 返回=已装备对象；NULL→0x145A66940 ctx 清空
0x14500A050  SetGridOverlay              ; ★断点B★ r8d = kind（1=箭头）
```

用户观察项：RSI（查询函数地址）、进城 vs 换装后 RAX（0/非0）、SetGridOverlay r8d 序列。
拿到 RSI 后静态解剖查询函数，钉死其数据源由哪条报文写入。

## 35. 动态探针第一击：闪退 + 查询函数静态解剖（21:00–21:30）

**frida 探针**（`analysis/tools/arrow_probe.py`）：挂 `0x145A66933`（查询入参）+ `0x145A66938`（查询返回）+ `0x500A050`（SetGridOverlay）。
附加成功（注意：frida **按名字**找不到 DFO.exe，必须用 PID；进程枚举也看不到它，但按 PID 附加正常）。
用户开背包瞬间**客户端闪退**。最可能原因是 `0x5A66933` 与 `0x5A66938` 两个钩子仅相距 5 字节、
且 `0x145A6692C` 有 `jmp` 直指钩子地址——中段内联钩互相踩踏，未必是反作弊。

**静态收获（本轮核心）**——查不出 RSI 的问题解决了，直接解剖候选查询函数：

- 巨型虚表扫描（.rdata 中 ≥800 连续 .text 指针）：`[+0x1F08]` 收敛到两个候选
  `0x145CDCA30` / `0x145DB4840`；后者是浮点 getter，排除。
- **查询函数 = `0x145CDCA30`（零化 r8/r9 后 jmp `0x145CDC8E0`）**，语义：

```c
void* Query(void* self, int id) {              // id ∈ [0, 0x31]，≥0x32 → NULL
    flag = (id == 0x2F) || self->vtbl[0x90]() == 1;
    if (id != 0xD) call 0x145A89860(id);       // 0xC–0x30 id 跳转表
    if (!call 0x145A8A680(id)) return NULL;
    ...
    void* A = self->[0x5B98 + id*24];          // ★注册表（comparison 已注册对象）★
    void* B = self->[0x10F60 + id*24];         // ★当前表★
    if (A != B) return A;                      // 不同 → 返回注册对象（箭头隐藏路径）
    if (!flag)  return NULL;                   // 相同且无标记 → NULL（→ 箭头显示路径）
    return B;
}
```

- **机制精细化**：格子重绘传 r9=残留值（经 stub 清 0）⇒ flag 仅 id==0x2F 为真。
  重登时两表皆空（NULL==NULL）→ 返回 NULL → 全亮；换装后 A/B 指向不同对象 →
  返回 A → 箭头正确。与实机观察完全自洽，且证明判定在**客户端注册状态**。
- 两张表（+0x5B98 / +0x10F60）在 .text 内**均无常量位移直接写入**（`pe_lea_scan`
  disp 0x5B98/0x10F60 全量扫描）⇒ 注册走计算地址/memcpy/构造期写入，静态常量扫描不可见。

**下一步**：A) 安全重试动态——只挂**函数入口**（`0x145CDCA30` 查询入口 + `0x14500A050`
覆盖图标入口），不再用中段钩子；B) 升级静态工具做数据流归约找注册写入方。

## §36 动态探针时间线（第四~六轮，20260921 21:39–21:56，arrow_probe.py v3–v5）

探针：frida 按 PID 附加，只挂函数入口（0x145CDCA30 入口 + SetGridOverlay 入口 + v5 增 A/B 表 200ms 只读轮询）。
安全版不再闪退（前次闪退=中段钩子互踩）。坑：frida x64 上下文无 `edx`，须取 `rdx` 低 32 位；
`Memory.readPointer` 已移除，用 `ptr.readPointer()`；GetManager 入口钩不触发（疑似内联），manager 从 Query 的 RCX 拿。

**时间线（去重后）：**
- 修复态（换装后）：Query(cand1) id=12,14–21 → 非零；id=13 恒 0x0（任何状态）
- 退回选角：全部翻 0x0 → 重灌新指针（A 表清空重填，同一 manager 对象 0xac3f9740）
- 进游戏+开装备栏：8 格 OVERLAY kind=1 —— 此刻 Query 表非零、无 0x0 翻转 ⇒ 画箭头不走 cand1 的 NULL 分支，
  而是走路径 A 等级比较分支（mgr->vtbl[0xAB8] vs 0x145008DB0(slot)）
- 换装瞬间：id=16 → 0x0 → 新指针；TABLE[A] 仅 A[16] 翻转；B 表（+0x10F60）全程纹丝不动
- ⇒ 箭头消失由"换装触发客户端全局重估"造成，与 A/B 表翻转无直接因果

**服务端对账（events.jsonl 会话 213405）：**

| 报文 | 换装自愈序列 | 登录序列 |
|---|---|---|
| equipment_move_committed (id19) | ✅ | — |
| bag resync (id13) | ✅ | ✅ |
| worn resync (id13) | ✅ | ✅ after_barrier |
| **equipment_slots_updated ×2 (id14, 槽位格式)** | ✅ | ❌ 从未发 |
| worn_window_refreshed (id14) | ✅ | ✅ entry |
| appearance/visuals (id2/C9) | ✅ | ✅ |

slot-update payload 携带**物品等级**（实测字节 0x3C=60 / 0x32=50 / 0x28=40），
正是箭头等级比较分支依赖的槽位级数据。前三次镜像修复均漏掉此报文。

## §37 修复（third pass，20260921 22:08）

- `entry_flow.go`：entryPayloads 新增 `WornSlots`；屏障后、窗口刷新前插入
  `equipment_slots_updated_entry`（id14，space 3 全套 worn 行，
  `inventory.EquipmentPayload(3, bag.Worn, false)`——与换装路径同构建器同格式）
- `main.go`：wearService 块构建 plan.WornSlots
- `worn_display_handoff_test.go`：修复原本就已过时失败的断言（second pass 把
  actor_appearance_ready 挪后导致），并新增 slot-update 顺序断言；全部测试通过
- `bin/wireprobe-handoff-source.exe` 已重编（备份 .bak-20260921-2200），二进制含新事件名
- 待实机验证：重启服务端+游戏 → 进城看箭头；events.jsonl 应出现 equipment_slots_updated_entry

## §38 实机验证通过（20260921 22:12）——next47 关闭

重启服务端 + 游戏后实机结果：**进城装备栏仅青铜护腿 1 个箭头**（此前为第一行 8 格全亮）。

判定：剩余 1 个箭头为正确行为——青铜护腿是全身等级最低装备，↑箭头是"可换更好装备"的真实提示；
实机旁证：换下护腿后全部箭头消失（含该箭头），与"护腿箭头为真实语义"自洽。7 格全亮 bug 由
equipment_slots_updated 缺失导致，third pass 修复后消失。

**任务闭环：**
- 根因：登录序列从未下发 id14 槽位更新报文（携带物品等级），客户端箭头"等级比较"分支数据缺失 → 全画箭头；
  换装自愈正是因为换装处理器会补发该报文 ×2。
- 修复：entry_flow.go 屏障后插入 equipment_slots_updated_entry（全套 worn 行，EquipmentPayload(3, bag.Worn, false)）。
- 方法论沉淀：报文镜像法穷尽后转动态探针（安全版只挂函数入口）；探针证明箭头走等级比较分支而非 NULL 分支，
  再以服务端 events.jsonl 报文集差集（换装 vs 登录）锁定唯一缺失报文。
