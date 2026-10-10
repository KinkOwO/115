/*
 * DifficultyRules.dll —— 副本难度（怪物血量 + 怪物伤害倍率）客户端宿主插件（115US）
 *
 * 定位
 *   把**启动器内嵌 GM**（115us-dfolauncher/gm）里那套"副本难度 → 怪物血量/伤害倍率"
 *   的字段执行逻辑搬进客户端进程：不碰 DFO.exe 文件、不落位 dinput8.dll、
 *   不改任何函数字节，只按已取证的偏移**读写**怪物实体上的字段。
 *
 * 宿主契约（client-patchs/client-host/HOST-README.md）
 *   宿主 = ChineseLocalization.dll（客户端自带 dinput8.dll 代理加载），枚举
 *   <客户端>\.115us-mods\*.dll → LoadLibraryW → ModName() → ModStart()；
 *   ModStart 返回 0 = 成功；调用发生在宿主自己的**工作线程**（不是游戏主线程，
 *   也不在 loader lock 里）；宿主 GetModuleHandleExW(PIN) 只加载不卸载。
 *   日志写**本 DLL 自己所在目录**（AGENTS §0 第 9 条）。
 *
 * 逐条照抄的来源（Go，115us-dfolauncher/gm/internal/mempatch，只读它）：
 *   monster_runtime.go   :82-121  指纹（14 段逐字节）
 *                        :110-121 旧钩子存在性检查
 *                        :124-132 地址换算 va(v) = v + base - 0x140000000
 *                        :143-166 identityValid（5 条）
 *                        :169-204 全局 → root → vtable[0xd8] → scene → 副本 id
 *                        :205-281 场景 → manager → 哨兵链（512 上限 + 环路检测，key>>16==3）
 *                        :248-267 血量四层 desc(layer)=actor+0x2368+layer*0xc38 / 当前血量 actor+0x5448
 *                        :290-299 scaleHP
 *                        :303-348 Reconcile（Original/Applied/Percent 三态，防复利）
 *                        :349-439 writeHP（写前写后 identityValid + 读回校验 + 失败回滚）
 *   monster_hp_formula.go :23-33  rescaleEffectiveHP（保住"已受伤比例"）
 *                         :49-66  NativeHPLayerAudit.maxima
 *                         :68-94  hpLayer（层内 4 个 rate / FinalRate / Addition / BonusRate）
 *   monster_combat.go      :16-40  怪物伤害 = 物攻(desc+0x398) + 魔攻(desc+0x3b8)
 *   monster_movement.go    :24-39  原生整数属性加解密（stored+guard 对）
 *                          :46-172 reconcileNativeStat（逐字节核对 + 回滚）
 *                          :50-53  偏移白名单（本版只用 0x398 / 0x3b8）
 *   session_windows.go     :33-40  属性加解密（64 位）
 *
 * 纪律（client-patchs/AGENTS.md §1）
 *   - 任何写入前逐字节核对期望字节；对不上就跳过 + 记日志（宁失效不崩）。
 *   - 指纹任何一段不符 → 拒绝接管（换客户端版本时唯一的保证）。
 *   - 规则文件损坏/删除 → 立刻还原已改过的怪并停止接管。
 *   - 默认规则 enabled=false（不偷偷加强）。
 *   - 全程只做内存读写，**不调用任何游戏代码**（与 Go 侧一致：绝不当"调用方"）。
 *
 * 2026-10-07 实机 bug 修复（重要，别改回去）
 *   血量写入路径原来在算"改后有效上限"时又调了一次 hp_layer(id, 3, new_base)。
 *   hp_layer 会核对 `DecryptAttr64(*desc) == base`，而那一刻内存里还是**旧** base，
 *   于是每只怪都在这里被静默丢弃 —— 现象是"实体链找到了怪（tracked 有值），
 *   但 applied 恒为 0、failed 也恒为 0、日志里一条写入记录都没有"。
 *   修法与 Go 一致：只读一次 hpLayer(old)，改后用同一个审计结果调 maxima(new)。
 *
 * 2026-10-07 多文件规则加载（rules.d，重要）
 *   规则文件从"只有 <插件目录>\rules.json"扩展为**两处**：
 *     1) <插件目录>\rules.d\*.json（可以不存在，按文件名**升序**遍历）
 *     2) <插件目录>\rules.json（最后读）
 *   每个文件内部仍是"第一条 enabled 且命中的规则生效"；跨文件是
 *   **先遍历到的文件里命中就赢** ⇒ rules.d 里 mod 自带的"只对某副本"规则
 *   会赢过玩家 rules.json 里的通用规则，其它副本仍按玩家规则（或原版）。
 *   rules.d 里单个文件解析失败**只跳过它自己**（醒目记日志），
 *   不让整份配置失效；rules.json 缺失/读失败/解析失败**保持旧语义**（整份不可用）。
 *   目录不存在 = 与旧版行为逐字节一致（只是少一次目录枚举）。
 */

#define WIN32_LEAN_AND_MEAN
#define _CRT_SECURE_NO_WARNINGS
#include <windows.h>
#include <stdio.h>
#include <stdarg.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <math.h>
#include <intrin.h>
#include <process.h>

/* ================================================================== */
/* 常量：逐条来自 Go 源，见文件头索引                                   */
/* ================================================================== */

/* session_windows.go:8 —— 客户端镜像基址。
 * 本机 DFO.exe 的 PE ImageBase 实测 = 0x140000000，且 DllCharacteristics(0xF6FDA37)
 * 未置 DYNAMIC_BASE(0x40)，不是 ASLR 随机基址。基址不符时插件直接拒绝接管。 */
#define EXPECTED_IMAGE_BASE 0x140000000ULL

/* monster_runtime.go:169-204 全局与场景链 */
#define VA_ROOT_PTR        0x14e6839f0ULL /* 全局：root 指针 */
#define VA_ROOT_GETTER     0x144ed9c60ULL /* root->vtable[0xd8] 必须等于它 */
#define OFF_ROOT_VT_GETTER 0xd8ULL
#define OFF_ROOT_SCENE     0x90ULL        /* scene = root + 0x90 */
#define VA_GAME_PTR        0x14e66c090ULL /* 全局：game 指针 */
#define OFF_GAME_CTX       0x120ULL       /* context = game + 0x120 */
#define OFF_CTX_DUNGEON    0x57ef58ULL    /* 副本 id = *(u32*)(context + 0x57ef58) */

/* monster_runtime.go:205-224 场景 → 实体链 */
#define OFF_SCENE_CONTROL 0x108ULL /* control = scene + 0x108（+8 = 计数，必须非 0） */
#define OFF_SCENE_MANAGER 0x110ULL /* manager = scene + 0x110 */
#define OFF_MANAGER_HEADS 0x78ULL  /* 哨兵节点 = manager + 0x78 */
#define ENTITY_CHAIN_MAX  512      /* 链长上限 */

/* monster_runtime.go:143-166 identityValid（5 条） */
#define OFF_NODE_KEY      0x10ULL  /* node + 0x10 低 32 位 = key */
#define OFF_NODE_CONTROL  0x20ULL  /* node + 0x20 = control 指针 */
#define OFF_NODE_REF      0x28ULL  /* node + 0x28 = actor + 0x30 */
#define OFF_CONTROL_COUNT 8ULL     /* control + 8 = 计数（必须非 0） */
#define RVA_ACTOR_DESC_L0 0x145c14140ULL /* vtable[0x12c8] 必须等于它 */
#define OFF_VT_DESC_L0    0x12c8ULL
#define ACTOR_REF_DELTA   0x30ULL
#define REF_MIN           0x10030ULL
#define KEY_SHIFT         16
#define KEY_WANT          3

/* monster_runtime.go:249,262,378,390 血量字段 */
#define OFF_ACTOR_DESC0 0x2368ULL /* desc(layer) = actor + 0x2368 + layer*0xc38 */
#define DESC_STRIDE     0xc38ULL
#define HP_LAYERS       4
#define OFF_ACTOR_HP    0x5448ULL /* 当前血量（64 位加密） */

/* monster_hp_formula.go:68-94 层内布局（相对 desc 起点，一次读 0x68） */
#define DESC_READ_LEN  0x68
#define OFF_DESC_BASE  0x00ULL
#define OFF_DESC_RATE0 0x10ULL /* 4 个 float32（各占 8 字节：stored + guard） */
#define OFF_DESC_FINAL 0x30ULL
#define OFF_DESC_ADD   0x48ULL /* int64 加密属性（加算） */
#define OFF_DESC_BONUS 0x60ULL
#define RATE_COUNT     4

/* monster_hp_formula.go:35-47 原生 float 解码 */
#define NATIVE_FLOAT_XOR       0x1f2a025cu
#define NATIVE_FLOAT_SUB       4
#define NATIVE_FLOAT_GUARD_ADD 0xc4u

/* session_windows.go:33-40 属性加解密 */
#define ATTR64_XOR 0x1f2a015c4bfa2b1cULL
#define ATTR64_ADD 8ULL

/* monster_runtime.go:256,267 —— base/current 的合理上界。
 * Go 原文是 `math.MaxInt64/1000000` = 9223372036854（**不是** 0x7fffffffffff：
 * 旧版这里放宽了 15 倍，会把 Go 侧明确丢弃的实体当成有效怪收进来）。 */
#define HP_SANE_MAX 9223372036854ULL

/* monster_movement.go:24-39 —— 受保护 32 位原生整数（stored + guard 对）
 *   编码：stored = (v + 4) ^ 0x1f2a025c；guard = stored + v + 0xc4
 *   解码：v = (stored ^ 0x1f2a025c) - 4；guard != 0 时必须等于 stored + v + 0xc4 */
#define NATIVE_INT_XOR       0x1f2a025cu
#define NATIVE_INT_SUB       4u
#define NATIVE_INT_GUARD_ADD 0xc4u
#define NATIVE_INT_MAX       0x7fffffffUL /* Go: math.MaxInt32 */

/* monster_movement.go:50-53 —— 只允许这 7 个"已取证"的原生整数属性偏移。
 * 本版**只用** 0x398 / 0x3b8 两项（monster_combat.go:22-23 的物理攻击 / 魔法攻击），
 * 其余偏移仍然保留为 TODO（见文末）。 */
#define OFF_STAT_MOVE      0x2e0ULL
#define OFF_STAT_ATKSPEED  0x300ULL
#define OFF_STAT_HITRECOV  0x348ULL
#define OFF_STAT_ATK_PHYS  0x398ULL
#define OFF_STAT_ATK_MAG   0x3b8ULL
#define OFF_STAT_DEF_PHYS  0x3a8ULL
#define OFF_STAT_DEF_MAG   0x3c8ULL

static int native_stat_whitelisted(uint64_t offset) {
    switch (offset) {
    case OFF_STAT_MOVE:
    case OFF_STAT_ATKSPEED:
    case OFF_STAT_HITRECOV:
    case OFF_STAT_ATK_PHYS:
    case OFF_STAT_ATK_MAG:
    case OFF_STAT_DEF_PHYS:
    case OFF_STAT_DEF_MAG:
        return 1;
    default:
        return 0;
    }
}

/* 场景快照上限（Go 侧没有硬上限，只受实体链 512 限制；这里给足余量） */
#define MAX_SCENE_MONSTERS 512

/* 轮询周期（gm/cmd/difficulty-dll/main.go:128 time.Sleep(200 * time.Millisecond)） */
#define TICK_MS 200

/* 状态文件（给启动器/GM 读；只写插件自己目录，遵循 AGENTS §0 第 9 条） */
#define INFO_HEARTBEAT_MS 5000

/* ================================================================== */
/* 指纹表：monster_runtime.go:82-121 逐字节照抄                        */
/* ================================================================== */

typedef struct {
    uint64_t va;
    const unsigned char *code;
    size_t len;
    const char *tag;
} CodeCheck;

/* 任务书列出的 7 段"真指纹"（0x145c14140 既是独立段又是 vtable+0x12c8 的目标） */
static const unsigned char kCode0[] = {0x48, 0x83, 0xec, 0x28, 0x48, 0x8b, 0x01, 0xba, 0x03, 0, 0, 0, 0xff, 0x90, 0xc8, 0x12, 0, 0, 0x48, 0x8b, 0xc8, 0x33, 0xd2, 0x48, 0x83, 0xc4, 0x28, 0xe9, 0x60, 0x74, 0x60, 0x01};
static const unsigned char kCode1[] = {0x40, 0x53, 0x56, 0x57, 0x48, 0x83, 0xec, 0x30, 0x0f, 0xb6, 0xda, 0x0f, 0x29, 0x74, 0x24, 0x20, 0x48, 0x8d, 0x54, 0x24, 0x60, 0x48, 0x8b, 0xf9, 0xe8, 0x53, 0x12, 0xc7, 0xff, 0x48, 0x8b, 0x74};
static const unsigned char kCode2[] = {0x48, 0x89, 0x5c, 0x24, 0x10, 0x57, 0x48, 0x83, 0xec, 0x40, 0x48, 0x8b, 0xda, 0x48, 0x8b, 0xf9, 0x41, 0x83, 0xf8, 0x04, 0x74, 0x1f, 0x4d, 0x63, 0xc0, 0x42, 0x8b, 0x04, 0xc2, 0x89, 0x01, 0x42};
static const unsigned char kCode3[] = {0x48, 0x63, 0xc2, 0x48, 0x69, 0xd0, 0x38, 0x0c, 0, 0, 0x48, 0x8d, 0x81, 0x68, 0x23, 0, 0, 0x48, 0x03, 0xc2, 0xc3};
static const unsigned char kCode4[] = {0x48, 0x8d, 0x81, 0x48, 0x54, 0, 0, 0xc3};
static const unsigned char kCode5[] = {0x48, 0x8b, 0x81, 0x20, 1, 0, 0, 0xc3};
static const unsigned char kCode6[] = {0x8b, 0x81, 0x58, 0xef, 0x57, 0, 0xc3};
/* 以下 7 段是 Go 侧同一次校验里的旁证（品级/夹取/修正系数访问器），一并照抄 */
static const unsigned char kCode7[] = {0x8b, 0x81, 0xc0, 0x66, 0, 0, 0xc3};
static const unsigned char kCode8[] = {0x33, 0xc0, 0x85, 0xd2, 0x0f, 0x49, 0xc2, 0xba, 0x64, 0, 0, 0, 0x3b, 0xc2, 0x0f, 0x4d, 0xc2, 0x89, 0x81, 0x70, 0x19, 0, 0, 0xc3};
static const unsigned char kCode9[] = {0x8b, 0x91, 0x70, 0x19, 0, 0, 0x48, 0x8b, 0x89, 0x78, 0x19, 0, 0, 0xe9, 0xce, 0xba, 0xb4, 0xfd};
static const unsigned char kCode10[] = {0x40, 0x53, 0x48, 0x83, 0xec, 0x20, 0x48, 0x8b, 0x1, 0xba, 0x3, 0x0, 0x0, 0x0, 0x8b, 0x99, 0xc0, 0x8d, 0x0, 0x0, 0xff, 0x90, 0xc8, 0x12, 0x0, 0x0, 0x8b, 0x80, 0x2c, 0xc, 0x0, 0x0, 0x3, 0xc3, 0x48, 0x83, 0xc4, 0x20, 0x5b, 0xc3};
static const unsigned char kCode11[] = {0x40, 0x53, 0x48, 0x83, 0xec, 0x20, 0x48, 0x8b, 0x1, 0xba, 0x3, 0x0, 0x0, 0x0, 0x8b, 0x99, 0xac, 0x75, 0x0, 0x0, 0xff, 0x90, 0xc8, 0x12, 0x0, 0x0, 0x8b, 0x80, 0x28, 0xc, 0x0, 0x0, 0x3, 0xc3, 0x48, 0x83, 0xc4, 0x20, 0x5b, 0xc3};
static const unsigned char kCode12[] = {0x40, 0x53, 0x55, 0x41, 0x56, 0x41, 0x57, 0x48, 0x83, 0xec, 0x38, 0x48, 0x8b, 0x99, 0xc8, 0x87, 0x0, 0x0, 0x48, 0x8b, 0xe9, 0x48, 0x8b, 0x81, 0xd0, 0x87, 0x0, 0x0, 0x48, 0x2b, 0xc3, 0x4c, 0x63, 0xfa};
static const unsigned char kCode13[] = {0x48, 0x8d, 0x81, 0x80, 0x89, 0, 0, 0xc3};

/* monster_runtime.go:110-121 —— 旧试验钩子存在性检查。本插件不做属性写入原语，
 * 但现场若已有旧钩子，说明另一个工具（旧 GM 实时执行器）正在改同一批字段，拒绝接管。 */
static const unsigned char kLegacy1[] = {0x48, 0x89, 0x6c, 0x24, 0x20};
/* session_windows.go:10 originalClamp */
static const unsigned char kLegacy2[] = {0xF2, 0x0F, 0x5F, 0x05, 0x32, 0x6F, 0x8D, 0x04};

/* ================================================================== */
/* 状态                                                                */
/* ================================================================== */

#define MAX_RULES       32 /* gmrules.MaxRules */
/* 单条规则的 dungeonIds 上限。2026-10-07 由 64 抬到 256。
 *
 * 依据：奥德赛 mod 的 rules.d\odyssey.hardcore.json 已经用掉 56 个副本 id，64 只剩 8 个坑位；
 * 而**规则条数**上限 MAX_RULES(32) 是跨文件共享的一个池子（见下面 MAX_RULE_FILES 的注释），
 * 为覆盖更多副本去拆规则很快会撞到 32 条。抬"单条规则的 id 容量"这一维更省，也不改任何语义：
 * 越界仍然按"整份不可用、不用一半"处理。
 *
 * 内存：Rule 由 408 B 涨到 1176 B，但它只出现在**堆**数组里 ——
 * ruleset_add_file_slot()/ruleset_ready() 用 calloc(MAX_RULES, sizeof(Rule)) 分配；
 * RuleSet 自己只存 RuleFile[]（名字 + 计数 + 指针），**不内嵌 Rule**，所以进函数栈的
 * RuleSet 仍是几 KiB，抬容量不改变栈占用（旧版内嵌数组曾把工作线程栈撑到 0xC00000FD）。
 * 最坏堆 = MAX_RULE_FILES(32) × MAX_RULES(32) × sizeof(Rule) ≈ 1.2 MiB。
 * 自测 test_rule_ids_capacity 用 sizeof 断言把"RuleSet 不内嵌 Rule 数组"这条钉住。 */
#define MAX_RULE_IDS    256
#define MAX_JSON_BYTES  (1u << 20)
#define MAX_LOG_LINES   2000000L

typedef struct {
    char id[64];
    char name[64];
    int enabled;
    int all;
    uint32_t percent;        /* 血量倍率（百分之一倍；100 = 不改） */
    uint32_t attack_percent; /* 怪物伤害（物攻+魔攻）倍率；100 = 不改 */
    uint32_t ids[MAX_RULE_IDS];
    int id_count;
    int id_overflow;
} Rule;

/* 规则来源文件数上限。规则**总条数**上限仍是 MAX_RULES（32），跨文件共享这一个池，
 * 所以多一份 rules.d 文件不会让总量膨胀 —— 超了就按"越界"处理（整份配置判不可用）。 */
#define MAX_RULE_FILES 32

/* 一份规则文件及其解析结果。
 *
 * rules 是**堆上**的紧凑数组，不是内嵌数组：Rule 本身约 350 字节，
 * RuleFile 内嵌 32 条就有 11 KiB，而 RuleSet 要放 32 份文件 —— 内嵌会得到
 * 一个 ~360 KiB 的 RuleSet，进了函数栈就是必然的 stack overflow
 * （本插件是在游戏进程的工作线程里跑，不能赌栈够大）。 */
typedef struct {
    char name[64]; /* 文件名（排序键 / 日志用；跨文件优先级就按这个字段升序） */
    int enabled;   /* 该文件顶层 enabled */
    int rule_count;
    Rule *rules;   /* 堆数组，容量 MAX_RULES；NULL = 没分配 */
} RuleFile;

/* 生效的规则集：**多个来源合并后的扁平列表**。
 *
 * 合并语义（2026-10-07 定，见文件头"多文件规则加载"）：
 *   先按**文件名升序**遍历 <插件目录>\rules.d\*.json，**再**读 <插件目录>\rules.json；
 *   每个文件内部仍是"第一条 enabled 且命中的规则生效"；
 *   跨文件是**先遍历到的文件里命中就赢**。
 * 为什么这样定优先级：mod 自带的规则（rules.d 里那份）必须赢过玩家的通用 rules.json ——
 * 例如 odyssey.hardcore 要"只在奥德赛副本内 ×10"，而玩家 rules.json 里可能是
 * "全副本 ×2"。若反过来让 rules.json 优先，玩家一写通用规则就把 mod 的范围打穿了，
 * 而 mod 作者无法预期、也无法修（rules.json 是玩家的文件）。
 * mod 自带文件先于玩家文件 = "更具体的规则优先"，且不依赖任何新字段。 */
typedef struct {
    int rule_file_count;
    RuleFile files[MAX_RULE_FILES];
    /* 匹配结果归属（日志与 status.json 用；match_rule 顺带填） */
    char matched_file[64];
} RuleSet;

/* rules.d 里一份 .json 的目录项（升序排列后逐个加载）。 */
typedef struct {
    wchar_t name[MAX_PATH];
    uint64_t size;
    DWORD lo, hi;
} DirRuleFile;

typedef struct {
    uint64_t scene, manager, node, control, actor, vtable;
    uint32_t key;
} MonsterId;

typedef struct {
    int used;
    MonsterId id;
    uint64_t original[HP_LAYERS];
    uint64_t applied[HP_LAYERS];
    uint32_t percent;
} Entry;

/* monster_movement.go:19-22 monsterStatIdentity —— 原生整数属性按 (身份, 偏移) 记账 */
typedef struct {
    int used;
    MonsterId id;
    uint64_t offset;
    uint32_t original[HP_LAYERS];
    uint32_t applied[HP_LAYERS];
    uint32_t percent;
} StatEntry;

/* monster_runtime.go:34-39 MonsterSnapshot —— 一次快照出来的合格怪（同一份给血量和伤害用） */
typedef struct {
    MonsterId id;
    uint64_t bases[HP_LAYERS];
    uint64_t current;
    uint32_t rank;
} MonsterSnap;

typedef struct {
    RuleSet set;
    Entry *entries;
    int entry_count;
    int entry_cap;
    StatEntry *stats;
    int stat_count;
    int stat_cap;
    DWORD mtime_lo, mtime_hi;  /* 上次取快照时的 rules.json 时间戳（0,0 = 不存在） */
    uint64_t fast_size;        /* 上次取快照时的 rules.json 文件大小 */
    uint64_t hash;             /* 上次取快照时的 rules.json 内容哈希（FNV-1a；0 = 不存在） */
    uint64_t dir_fingerprint;  /* 上次取快照时 rules.d 的目录指纹（0 = 不存在/空） */
    int rule_file_count;       /* 上次快照时 rules.d 里的 .json 文件数 */
} Engine;

static HMODULE g_self;
static wchar_t g_dir[MAX_PATH];
static FILE *g_log;
static CRITICAL_SECTION g_lock;
static DWORD g_t0;
static long g_log_lines;

static uint64_t g_base = EXPECTED_IMAGE_BASE;
static int g_ready; /* 指纹通过、开始接管 */
static int g_last_dungeon = -1;
static Engine g_engine;

/* ================================================================== */
/* 实体链诊断（任务书要求：状态变化时打一行，正常接管后不刷屏）           */
/* ================================================================== */
#define DIAG_NODES 3
#define ID_STEP_MAX 7

typedef struct {
    uint64_t node, control, ref, actor, vtable;
    uint32_t key;
    int verdict; /* 0=收下 1=key 过滤 2=control/ref 不合法 3=vtable 读失败 4=身份校验第 N 步 */
    int step;
} DiagNode;

typedef struct {
    uint64_t root, root_vt_d8, scene, control, count, manager, sentinel, first_node;
    int seen_root, seen_getter, seen_scene, seen_control, seen_manager, seen_sentinel;
    int stop_reason; /* 0=走完 1=无 root 2=getter 不符 3=scene 为空 4=control 为空
                        5=control 计数为 0 6=manager 为空 7=sentinel 读失败
                        8=首节点读失败 9=环路 10=超过 512 */
    int node_count;      /* 实际遍历到的节点数 */
    int accepted;        /* 身份校验通过（进入回调）的实体数 */
    int key_filtered;    /* key>>16 != 3 */
    int bad_ref;         /* control==0 或 ref<=0x10030 */
    int vt_fail;         /* actor 虚表读不出来 */
    int id_fail;         /* identity_valid 失败 */
    int step_fail[ID_STEP_MAX + 1]; /* 每一步各失败了多少个 */
    DiagNode nodes[DIAG_NODES];
} ChainDiag;

/* 交给主循环的"/info"内容（每轮抓一次） */
typedef struct {
    uint32_t dungeon_id;
    int has_dungeon;
    int matched;
    char rule_id[64];
    char rule_file[64]; /* 命中的规则来自哪一份文件（rules.d 里的文件名 / rules.json） */
    uint32_t percent;
    uint32_t attack_percent;
    int monsters;
    int applied;
    int failed;
    int tracked;
    int attack_applied;
    int attack_failed;
    int attack_tracked;
    int restored;
    int disabled;
    char last_error[192];
} TickInfo;

static TickInfo g_info;

/* ================================================================== */
/* 日志：只写本 DLL 自己所在目录                                        */
/* ================================================================== */
static void w2u(const wchar_t *src, char *dst, size_t cap) {
    if (!src) { dst[0] = 0; return; }
    {
        int n = WideCharToMultiByte(CP_UTF8, 0, src, -1, dst, (int)cap - 1, NULL, NULL);
        dst[n > 0 ? n : 0] = 0;
    }
}

static void logf_(const char *fmt, ...) {
    char buf[2048];
    va_list ap;
    if (!g_log) return;
    if (g_log_lines >= MAX_LOG_LINES) return;
    va_start(ap, fmt);
    _vsnprintf_s(buf, sizeof(buf), _TRUNCATE, fmt, ap);
    va_end(ap);
    EnterCriticalSection(&g_lock);
    fprintf(g_log, "[+%6lums][t%5lu] %s\n", (unsigned long)(GetTickCount() - g_t0),
            (unsigned long)GetCurrentThreadId(), buf);
    fflush(g_log);
    g_log_lines++;
    LeaveCriticalSection(&g_lock);
}

/* ================================================================== */
/* 内存访问                                                            */
/* ================================================================== */
static uint64_t va(uint64_t absolute) { return g_base + (absolute - EXPECTED_IMAGE_BASE); }

/* 目标地址必须是已提交且可读的内存，且整段落在同一个区域里。
 * 换客户端构建时 RVA 可能根本不在镜像里，这一步是必须的保险。 */
static int range_ok(uint64_t addr, size_t len) {
    MEMORY_BASIC_INFORMATION mbi;
    if (addr < 0x10000ULL) return 0;
    if (VirtualQuery((LPCVOID)(uintptr_t)addr, &mbi, sizeof(mbi)) != sizeof(mbi)) return 0;
    if (mbi.State != MEM_COMMIT) return 0;
    if (mbi.Protect & (PAGE_NOACCESS | PAGE_GUARD)) return 0;
    if (!(mbi.Protect & (PAGE_READONLY | PAGE_READWRITE | PAGE_WRITECOPY | PAGE_EXECUTE_READ |
                         PAGE_EXECUTE_READWRITE | PAGE_EXECUTE_WRITECOPY)))
        return 0;
    return addr + (uint64_t)len <= (uint64_t)(uintptr_t)mbi.BaseAddress + mbi.RegionSize;
}

static int rd(uint64_t addr, void *out, size_t len) {
    if (!range_ok(addr, len)) return 0;
    memcpy(out, (const void *)(uintptr_t)addr, len);
    return 1;
}

static int rd64(uint64_t addr, uint64_t *out) { return rd(addr, out, 8); }
static int rd32(uint64_t addr, uint32_t *out) { return rd(addr, out, 4); }

static uint64_t raw64(const unsigned char *b) {
    uint64_t v = 0;
    memcpy(&v, b, 8);
    return v;
}
static uint32_t raw32(const unsigned char *b) {
    uint32_t v = 0;
    memcpy(&v, b, 4);
    return v;
}
static void put64(unsigned char *b, uint64_t v) { memcpy(b, &v, 8); }
static void put32(unsigned char *b, uint32_t v) { memcpy(b, &v, 4); }

/* monster_movement.go:24-39 —— 受保护 32 位原生整数（8 字节：stored + guard）
 * 解码：v = (stored ^ 0x1f2a025c) - 4；guard 非 0 时必须 == stored + v + 0xc4。
 * 与 Go 的 decodeNativeInt 逐句一致（value > math.MaxInt32 视为非法）。 */
static int decode_native_int(const unsigned char *b, uint32_t *out) {
    uint32_t stored = raw32(b), guard = raw32(b + 4);
    uint32_t value = (stored ^ NATIVE_INT_XOR) - NATIVE_INT_SUB;
    if ((guard != 0 && guard != stored + value + NATIVE_INT_GUARD_ADD) ||
        (uint64_t)value > (uint64_t)NATIVE_INT_MAX)
        return 0;
    *out = value;
    return 1;
}

/* monster_movement.go:24-30 nativeIntPair：stored=(v+4)^K；guard=stored+v+0xc4 */
static void encode_native_int_pair(unsigned char *b, uint32_t value) {
    uint32_t stored = (value + NATIVE_INT_SUB) ^ NATIVE_INT_XOR;
    put32(b, stored);
    put32(b + 4, stored + value + NATIVE_INT_GUARD_ADD);
}

/* 写入：先放开页保护，写完立刻还原。任何失败都只记日志，不抛。 */
static int wr(uint64_t addr, const void *data, size_t len) {
    DWORD old = 0, tmp = 0;
    if (!range_ok(addr, len)) return 0;
    if (!VirtualProtect((LPVOID)(uintptr_t)addr, len, PAGE_READWRITE, &old)) return 0;
    memcpy((void *)(uintptr_t)addr, data, len);
    VirtualProtect((LPVOID)(uintptr_t)addr, len, old, &tmp);
    return 1;
}

/* session_windows.go:39-40 属性加解密（64 位） */
static uint64_t decrypt_attr64(uint64_t raw) { return (raw ^ ATTR64_XOR) - ATTR64_ADD; }
static uint64_t encrypt_attr64(uint64_t plain) { return (plain + ATTR64_ADD) ^ ATTR64_XOR; }

/* monster_hp_formula.go:35-47 原生 float32 解码 */
static int decode_native_float(const unsigned char *b, float *out) {
    uint32_t stored, guard, expected;
    int32_t bits;
    float f;
    memcpy(&stored, b, 4);
    memcpy(&guard, b + 4, 4);
    bits = (int32_t)((stored ^ NATIVE_FLOAT_XOR) - NATIVE_FLOAT_SUB);
    expected = stored + (uint32_t)bits + NATIVE_FLOAT_GUARD_ADD;
    if (guard != 0 && expected != 0 && guard != expected) return 0;
    memcpy(&f, &bits, 4);
    if (isnan(f) || isinf(f)) return 0;
    *out = f;
    return 1;
}

/* ================================================================== */
/* 指纹校验：monster_runtime.go:82-121                                 */
/* ================================================================== */
static int verify_code(uint64_t absolute_va, const unsigned char *expect, size_t len,
                       const char *tag) {
    unsigned char buf[64];
    int i;
    if (len > sizeof(buf)) return 0;
    if (!rd(va(absolute_va), buf, len)) {
        logf_("指纹[%s] @%#llx 不可读（客户端版本不同？）—— 拒绝接管", tag,
              (unsigned long long)absolute_va);
        return 0;
    }
    for (i = 0; i < (int)len; i++) {
        if (buf[i] != expect[i]) {
            char got[64 * 3 + 1];
            int j, n = 0;
            got[0] = 0;
            for (j = 0; j < (int)len && j < 11; j++)
                n += _snprintf_s(got + n, sizeof(got) - (size_t)n, _TRUNCATE, "%s%02x",
                                 j ? " " : "", buf[j]);
            logf_("指纹[%s] @%#llx 第 %d 字节不符：期望 %02x 现场 %02x —— 拒绝接管", tag,
                  (unsigned long long)absolute_va, i, expect[i], buf[i]);
            logf_("      现场前 11 字节：%s", got);
            return 0;
        }
    }
    return 1;
}

static int verify_all(void) {
    static const CodeCheck checks[] = {
        {0x145c199c0ULL, kCode0, sizeof(kCode0), "vfn-145c199c0"},
        {0x147220e40ULL, kCode1, sizeof(kCode1), "hp-getter-147220e40"},
        {0x14721faa0ULL, kCode2, sizeof(kCode2), "hp-mirror-14721faa0"},
        {0x145c14140ULL, kCode3, sizeof(kCode3), "hp-desc-a-145c14140"},
        {0x145c14120ULL, kCode4, sizeof(kCode4), "hp-cur-145c14120"},
        {0x142e0a3e0ULL, kCode5, sizeof(kCode5), "vt-slot-142e0a3e0"},
        {0x146d34370ULL, kCode6, sizeof(kCode6), "dungeon-146d34370"},
        {0x145c1a900ULL, kCode7, sizeof(kCode7), "rank-145c1a900"},
        {0x145d84000ULL, kCode8, sizeof(kCode8), "clamp-145d84000"},
        {0x145d6f090ULL, kCode9, sizeof(kCode9), "clamp2-145d6f090"},
        {0x145db4b40ULL, kCode10, sizeof(kCode10), "hp-bonus-145db4b40"},
        {0x145db2930ULL, kCode11, sizeof(kCode11), "hp-extra-145db2930"},
        {0x145db41d0ULL, kCode12, sizeof(kCode12), "hp-mod-145db41d0"},
        {0x145db2a60ULL, kCode13, sizeof(kCode13), "hp-desc-b-145db2a60"},
    };
    size_t i;
    for (i = 0; i < sizeof(checks) / sizeof(checks[0]); i++) {
        if (!verify_code(checks[i].va, checks[i].code, checks[i].len, checks[i].tag)) return 0;
    }
    /* 旧试验钩子存在性（monster_runtime.go:110-121） */
    if (!verify_code(0x146e922e0ULL, kLegacy1, sizeof(kLegacy1), "legacy-attr-writer")) {
        logf_("现场存在旧属性写入钩子（0x146e922e0）—— 本插件不与之并存，拒绝接管");
        return 0;
    }
    if (!verify_code(0x147220fe6ULL, kLegacy2, sizeof(kLegacy2), "legacy-hp-clamp")) {
        logf_("现场存在旧 HP 倍率补丁（0x147220fe6）—— 先还原旧补丁，拒绝接管");
        return 0;
    }
    return 1;
}

/* ================================================================== */
/* 定位：monster_runtime.go:143-281                                    */
/* ================================================================== */

/* :143-166 identityValid（写前写后都查）
 * 带 step 出参：0 = 通过，1..6 = 卡在第 N 条（只给诊断用，判据与 Go 完全一致）。 */
#define ID_OK              0
#define ID_NODE_KEY        1
#define ID_NODE_CONTROL    2
#define ID_CONTROL_COUNT   3
#define ID_NODE_REF        4
#define ID_ACTOR_VTABLE    5
#define ID_VT_DESC_L0      6

static int identity_valid_step(const MonsterId *id, int *step) {
    uint32_t node_key = 0, count = 0;
    uint64_t control = 0, ref = 0, vt = 0, fn = 0;
    if (step) *step = ID_OK;
    if (!rd32(id->node + OFF_NODE_KEY, &node_key) || node_key != id->key) {
        if (step) *step = ID_NODE_KEY;
        return 0;
    }
    if (!rd64(id->node + OFF_NODE_CONTROL, &control) || control != id->control) {
        if (step) *step = ID_NODE_CONTROL;
        return 0;
    }
    if (!rd32(control + OFF_CONTROL_COUNT, &count) || count == 0) {
        if (step) *step = ID_CONTROL_COUNT;
        return 0;
    }
    if (!rd64(id->node + OFF_NODE_REF, &ref) || ref != id->actor + ACTOR_REF_DELTA) {
        if (step) *step = ID_NODE_REF;
        return 0;
    }
    if (!rd64(id->actor, &vt) || vt != id->vtable) {
        if (step) *step = ID_ACTOR_VTABLE;
        return 0;
    }
    if (!rd64(vt + OFF_VT_DESC_L0, &fn) || fn != va(RVA_ACTOR_DESC_L0)) {
        if (step) *step = ID_VT_DESC_L0;
        return 0;
    }
    return 1;
}

static int identity_valid(const MonsterId *id) { return identity_valid_step(id, NULL); }

typedef struct {
    uint64_t scene;
    uint32_t dungeon_id;
    int has_dungeon;
} SceneInfo;

/* :169-204 全局 → root → vtable[0xd8] → scene → 副本 id
 * 返回 1 = 取到场景（has_dungeon 表示副本 id 是否可用）。
 * diag（可空）记录每一步的现场值，供"状态变化时打一行"用。 */
static int find_scene_diag(SceneInfo *out, ChainDiag *diag) {
    uint64_t root = 0, vt = 0, getter = 0, scene = 0, game = 0, ctx = 0;
    uint32_t dungeon = 0;
    memset(out, 0, sizeof(*out));
    if (diag) {
        /* 整份清零：否则"这一轮没走到遍历"时会把上一轮的节点快照当成本轮的打出来 */
        memset(diag, 0, sizeof(*diag));
        diag->stop_reason = 1;
    }
    if (!rd64(va(VA_ROOT_PTR), &root) || root == 0) return 0;
    if (diag) {
        diag->root = root;
        diag->seen_root = 1;
        diag->stop_reason = 2;
    }
    if (!rd64(root, &vt)) return 0;
    if (!rd64(vt + OFF_ROOT_VT_GETTER, &getter)) return 0;
    if (diag) {
        diag->root_vt_d8 = getter;
        diag->seen_getter = 1;
    }
    if (getter != va(VA_ROOT_GETTER)) return 0; /* 不是"场景根"（还在登录/选人等） */
    if (diag) diag->stop_reason = 3;
    if (!rd64(root + OFF_ROOT_SCENE, &scene) || scene == 0) return 0;
    out->scene = scene;
    if (diag) {
        diag->scene = scene;
        diag->seen_scene = 1;
        diag->stop_reason = 0;
    }
    if (!rd64(va(VA_GAME_PTR), &game) || game == 0) return 1;
    if (!rd64(game + OFF_GAME_CTX, &ctx) || ctx == 0) return 1;
    if (!rd32(ctx + OFF_CTX_DUNGEON, &dungeon)) return 1;
    out->dungeon_id = dungeon;
    out->has_dungeon = 1;
    return 1;
}

static int find_scene(SceneInfo *out) { return find_scene_diag(out, NULL); }

/* :205-281 场景 → manager → 哨兵链（512 上限 + 环路检测，只收 key>>16==3）
 * 回调返回 1 = 继续遍历；返回 0 = 立即停止。
 * 结构与 Go 逐句一致：control=scene+0x108（+8 计数非 0）→ manager=scene+0x110
 * → sentinel=*(manager+0x78) → node=*sentinel → while(node!=sentinel) node=*node。 */
typedef int (*MonsterFn)(const MonsterId *id, void *ctx);

static uint64_t g_seen[ENTITY_CHAIN_MAX];

static void walk_monsters_diag(const SceneInfo *sc, MonsterFn fn, void *ctx, ChainDiag *diag) {
    uint64_t control = 0, manager = 0, sentinel = 0, node = 0;
    uint32_t count = 0;
    int seen_n = 0;
    if (diag) {
        diag->stop_reason = 4;
        diag->control = diag->count = diag->manager = diag->sentinel = diag->first_node = 0;
        diag->seen_control = diag->seen_manager = diag->seen_sentinel = 0;
        diag->node_count = diag->accepted = diag->key_filtered = 0;
        diag->bad_ref = diag->vt_fail = diag->id_fail = 0;
    }
    if (!rd64(sc->scene + OFF_SCENE_CONTROL, &control) || control == 0) return;
    if (diag) {
        diag->control = control;
        diag->seen_control = 1;
        diag->stop_reason = 5;
    }
    if (!rd32(control + OFF_CONTROL_COUNT, &count) || count == 0) return;
    if (diag) diag->count = count;
    if (diag) diag->stop_reason = 6;
    if (!rd64(sc->scene + OFF_SCENE_MANAGER, &manager) || manager == 0) return;
    if (diag) {
        diag->manager = manager;
        diag->seen_manager = 1;
        diag->stop_reason = 7;
    }
    if (!rd64(manager + OFF_MANAGER_HEADS, &sentinel) || sentinel == 0) return;
    if (diag) {
        diag->sentinel = sentinel;
        diag->seen_sentinel = 1;
        diag->stop_reason = 8;
    }
    if (!rd64(sentinel, &node)) return;
    if (diag) {
        diag->first_node = node;
        diag->stop_reason = 0;
    }
    while (node != sentinel && seen_n < ENTITY_CHAIN_MAX) {
        int i;
        uint32_t key = 0;
        for (i = 0; i < seen_n; i++) {
            if (g_seen[i] == node) {
                if (diag) diag->stop_reason = 9;
                return;
            }
        }
        g_seen[seen_n++] = node;
        if (diag) diag->node_count = seen_n;
        if (!rd32(node + OFF_NODE_KEY, &key)) {
            if (diag) diag->stop_reason = 8;
            return;
        }
        if ((key >> KEY_SHIFT) != KEY_WANT) {
            if (diag) {
                diag->key_filtered++;
                if (seen_n <= DIAG_NODES) {
                    diag->nodes[seen_n - 1].node = node;
                    diag->nodes[seen_n - 1].key = key;
                    diag->nodes[seen_n - 1].verdict = 1;
                }
            }
        } else {
            uint64_t c = 0, ref = 0, vtable = 0;
            if (diag && seen_n <= DIAG_NODES) {
                diag->nodes[seen_n - 1].node = node;
                diag->nodes[seen_n - 1].key = key;
                diag->nodes[seen_n - 1].verdict = 2;
            }
            if (rd64(node + OFF_NODE_CONTROL, &c) && rd64(node + OFF_NODE_REF, &ref) && c != 0 &&
                ref > REF_MIN) {
                uint64_t actor = ref - ACTOR_REF_DELTA;
                if (diag && seen_n <= DIAG_NODES) {
                    diag->nodes[seen_n - 1].control = c;
                    diag->nodes[seen_n - 1].ref = ref;
                    diag->nodes[seen_n - 1].actor = actor;
                }
                if (rd64(actor, &vtable)) {
                    MonsterId id;
                    int step = ID_OK;
                    memset(&id, 0, sizeof(id));
                    id.scene = sc->scene;
                    id.manager = manager;
                    id.node = node;
                    id.control = c;
                    id.actor = actor;
                    id.vtable = vtable;
                    id.key = key;
                    if (diag && seen_n <= DIAG_NODES) {
                        diag->nodes[seen_n - 1].vtable = vtable;
                        diag->nodes[seen_n - 1].verdict = 3;
                    }
                    if (identity_valid_step(&id, &step)) {
                        if (diag) {
                            diag->accepted++;
                            if (seen_n <= DIAG_NODES) diag->nodes[seen_n - 1].verdict = 0;
                        }
                        if (!fn(&id, ctx)) return;
                    } else if (diag) {
                        if (step >= 1 && step <= ID_STEP_MAX) diag->step_fail[step]++;
                        diag->id_fail++;
                        if (seen_n <= DIAG_NODES) {
                            diag->nodes[seen_n - 1].verdict = 4;
                            diag->nodes[seen_n - 1].step = step;
                        }
                    }
                } else if (diag) {
                    diag->vt_fail++;
                    if (seen_n <= DIAG_NODES) diag->nodes[seen_n - 1].verdict = 3;
                }
            } else if (diag) {
                diag->bad_ref++;
                if (seen_n <= DIAG_NODES) diag->nodes[seen_n - 1].verdict = 2;
            }
        }
        {
            uint64_t next = 0;
            if (!rd64(node, &next)) {
                if (diag) diag->stop_reason = 8;
                return;
            }
            node = next;
        }
    }
    if (node != sentinel && seen_n >= ENTITY_CHAIN_MAX) {
        if (diag) diag->stop_reason = 10;
    }
}

static void walk_monsters(const SceneInfo *sc, MonsterFn fn, void *ctx) {
    walk_monsters_diag(sc, fn, ctx, NULL);
}

/* 关键校验步骤的中文名（诊断用） */
static const char *id_step_name(int step) {
    switch (step) {
    case ID_NODE_KEY: return "node+0x10 低32位 != key";
    case ID_NODE_CONTROL: return "node+0x20 != control";
    case ID_CONTROL_COUNT: return "control+8 计数为 0";
    case ID_NODE_REF: return "node+0x28 != actor+0x30";
    case ID_ACTOR_VTABLE: return "*actor != vtable";
    case ID_VT_DESC_L0: return "vtable+0x12c8 != 0x145c14140";
    default: return "?";
    }
}

static const char *chain_stop_name(int r) {
    switch (r) {
    case 0: return "走完（node == sentinel）";
    case 1: return "全局 root 指针为空/不可读";
    case 2: return "root->vtable[0xd8] 读失败";
    case 3: return "root->vtable[0xd8] != 0x144ed9c60（不是场景根）";
    case 4: return "scene+0x108(control) 为空/不可读";
    case 5: return "control+8 计数为 0";
    case 6: return "scene+0x110(manager) 为空/不可读";
    case 7: return "manager+0x78(sentinel) 读失败";
    case 8: return "节点不可读（key/next）";
    case 9: return "实体链出现环路";
    case 10: return "实体链超过 512 项";
    default: return "?";
    }
}

static const char *node_verdict_name(const DiagNode *n) {
    switch (n->verdict) {
    case 0: return "收下";
    case 1: return "key>>16 != 3 被过滤";
    case 2: return "control==0 或 ref<=0x10030 被过滤";
    case 3: return "actor 虚表读失败";
    case 4: return "身份校验失败";
    default: return "未走到";
    }
}

/* 状态变化时打一行完整链诊断（不变就不打，避免 200ms 刷屏） */
static void chain_diag_log(const ChainDiag *d, const SceneInfo *sc, int have_scene,
                           uint32_t dungeon, int have_dungeon, int monster_count,
                           int attack_count) {
    static uint64_t last_sig = 0;
    uint64_t sig;
    int i;
    sig = d->stop_reason;
    sig = sig * 1000003ULL + d->node_count;
    sig = sig * 1000003ULL + d->accepted;
    sig = sig * 1000003ULL + d->key_filtered;
    sig = sig * 1000003ULL + d->bad_ref;
    sig = sig * 1000003ULL + d->vt_fail;
    sig = sig * 1000003ULL + d->id_fail;
    sig = sig * 1000003ULL + (uint64_t)d->seen_control * 7 + (uint64_t)d->seen_manager * 11 +
          (uint64_t)d->seen_sentinel * 13;
    sig = sig * 1000003ULL + monster_count * 31 + attack_count;
    sig = sig * 1000003ULL + (have_scene ? (d->scene & 0xffffff) : 0) +
          (have_dungeon ? dungeon : 0);
    for (i = 1; i <= ID_STEP_MAX; i++) sig = sig * 1000003ULL + (uint64_t)d->step_fail[i];
    if (sig == last_sig) return;
    last_sig = sig;

    logf_("── 实体链诊断 ──");
    logf_("  root=%#llx root->vtable[0xd8]=%#llx（期望 %#llx）%s", (unsigned long long)d->root,
          (unsigned long long)d->root_vt_d8, (unsigned long long)va(VA_ROOT_GETTER),
          d->seen_getter && d->root_vt_d8 == va(VA_ROOT_GETTER) ? " [符合]" : " [不符/未取到]");
    logf_("  scene=%#llx%s  副本 id=%s", (unsigned long long)d->scene,
          have_scene ? "" : "（未取到场景根）",
          have_dungeon ? "" : "（不可用）");
    if (have_dungeon) logf_("  副本 id = %u", dungeon);
    logf_("  scene+0x108(control)=%#llx%s  control+8 计数=%llu%s", (unsigned long long)d->control,
          d->seen_control ? "" : "（未取到）", (unsigned long long)d->count,
          (d->seen_control && d->count == 0) ? " ← 计数为 0，遍历在这里就停了" : "");
    logf_("  scene+0x110(manager)=%#llx%s  manager+0x78(哨兵)=%#llx%s  首节点=*(哨兵)=%#llx",
          (unsigned long long)d->manager, d->seen_manager ? "" : "（未取到）",
          (unsigned long long)d->sentinel, d->seen_sentinel ? "" : "（未取到）",
          (unsigned long long)d->first_node);
    logf_("  遍历到节点 %d 个：收下(key>>16==3 且身份通过) %d；key 过滤 %d；control/ref 过滤 %d；"
          "虚表读失败 %d；身份校验失败 %d",
          d->node_count, d->accepted, d->key_filtered, d->bad_ref, d->vt_fail, d->id_fail);
    if (d->id_fail) {
        logf_("  身份校验失败分布：步骤1(node key)=%d 步骤2(node control)=%d 步骤3(control 计数)=%d "
              "步骤4(node ref)=%d 步骤5(actor 虚表)=%d 步骤6(vtable+0x12c8)=%d",
              d->step_fail[1], d->step_fail[2], d->step_fail[3], d->step_fail[4], d->step_fail[5],
              d->step_fail[6]);
    }
    for (i = 0; i < DIAG_NODES; i++) {
        const DiagNode *n = &d->nodes[i];
        if (!n->node) break;
        logf_("  节点%d: node=%#llx key=%#llx control=%#llx ref=%#llx actor=%#llx vtable=%#llx → %s%s",
              i, (unsigned long long)n->node, (unsigned long long)n->key,
              (unsigned long long)n->control, (unsigned long long)n->ref,
              (unsigned long long)n->actor, (unsigned long long)n->vtable,
              node_verdict_name(n), n->verdict == 4 ? "：" : "");
        if (n->verdict == 4)
            logf_("          卡在步骤 %d：%s", n->step, id_step_name(n->step));
    }
    logf_("  停止原因：%s；合格怪 %d 只，攻击属性目标 %d 只", chain_stop_name(d->stop_reason),
          monster_count, attack_count);
}

/* ================================================================== */
/* 血量计算：monster_hp_formula.go + monster_runtime.go:290-299        */
/* ================================================================== */
typedef struct {
    float rates[RATE_COUNT];
    float final_rate;
    float bonus_rate;
    int64_t addition;
    int64_t before_final;
    int64_t effective;
} HpLayerAudit;

/* monster_runtime.go:290-299 */
static int scale_hp(uint64_t value, uint32_t percent, uint64_t *out) {
    uint64_t next;
    if (percent == 0) return 0;
    if (value > 0x7fffffffffffffffULL / (uint64_t)percent) return 0; /* 溢出 */
    next = value * (uint64_t)percent / 100ULL;
    if (next == 0 && value > 0) next = 1;
    *out = next;
    return 1;
}

/* monster_hp_formula.go:49-66 maxima
 * 注意：Go 里 product 是 float32，逐次相乘都截回 float32，这里必须一致。 */
static int hp_maxima(const HpLayerAudit *h, uint64_t base, int64_t *before_out,
                     int64_t *effective_out) {
    float product = 1.0f;
    double value;
    int i;
    int64_t before;
    for (i = 0; i < RATE_COUNT; i++) product = (float)(product * h->rates[i]);
    value = ((double)product * (double)base + (double)h->addition) * (1.0 + (double)h->bonus_rate);
    if (isnan(value) || isinf(value) || value < 0 || value >= 9223372036854775807.0) return 0;
    before = (int64_t)value;
    if (value > 1.0 && h->final_rate > 0) {
        double v2 = value * (double)h->final_rate;
        if (v2 < 1.0) v2 = 1.0;
        value = v2;
    }
    if (isnan(value) || isinf(value) || value >= 9223372036854775807.0) return 0;
    *before_out = before;
    *effective_out = (int64_t)value;
    return 1;
}

/* monster_hp_formula.go:23-33 rescaleEffectiveHP —— 保住"已受伤比例"
 * Go: hi,lo := bits.Mul64(current,newMax); next,_ := bits.Div64(hi,lo,oldMax)
 * 这里用 MSVC x64 的 _umul128 / _udiv128，语义一致。 */
static int rescale_effective_hp(uint64_t current, int64_t old_max, int64_t new_max,
                                uint64_t *out) {
    uint64_t next;
    unsigned __int64 hi = 0, lo, rem = 0;
    if (old_max <= 0 || new_max <= 0 || current > (uint64_t)old_max) return 0;
    lo = _umul128(current, (unsigned __int64)new_max, &hi);
    /* current <= oldMax ⇒ 商必然放得下 64 位（bits.Div64 的 panic 条件不会触发） */
    next = _udiv128(hi, lo, (unsigned __int64)old_max, &rem);
    if (current > 0 && next == 0) next = 1;
    *out = next;
    return 1;
}

/* monster_hp_formula.go:68-94 hpLayer（只读，不调用游戏代码）
 * 注意：这里的 base 必须是**现场读到的那个值**（Go 的 hpLayer 会核对
 * `DecryptAttr64(*desc) == base`）。要算"改完之后"的上限，必须拿同一个
 * 审计结果去调 hp_maxima(new_base)（对应 Go 的 `h.maxima(bases[3])`），
 * **不能**再用新基础值去调本函数 —— 那时内存里还是旧值，必然对不上。 */
static int hp_layer(const MonsterId *id, int layer, uint64_t base, HpLayerAudit *out,
                    const char **why) {
    unsigned char b[DESC_READ_LEN];
    int i;
    memset(out, 0, sizeof(*out));
    if (why) *why = NULL;
    if (!rd(id->actor + OFF_ACTOR_DESC0 + (uint64_t)layer * DESC_STRIDE, b, sizeof(b))) {
        if (why) *why = "血量描述块不可读";
        return 0;
    }
    if (decrypt_attr64(raw64(b + OFF_DESC_BASE)) != base) {
        if (why) *why = "血量基础值已变化";
        return 0;
    }
    for (i = 0; i < RATE_COUNT; i++) {
        if (!decode_native_float(b + OFF_DESC_RATE0 + (size_t)i * 8, &out->rates[i])) {
            if (why) *why = "原生 float 系数校验失败（rate）";
            return 0;
        }
    }
    if (!decode_native_float(b + OFF_DESC_FINAL, &out->final_rate)) {
        if (why) *why = "原生 float 系数校验失败（finalRate）";
        return 0;
    }
    if (!decode_native_float(b + OFF_DESC_BONUS, &out->bonus_rate)) {
        if (why) *why = "原生 float 系数校验失败（bonusRate）";
        return 0;
    }
    out->addition = (int64_t)decrypt_attr64(raw64(b + OFF_DESC_ADD));
    if (!hp_maxima(out, base, &out->before_final, &out->effective)) {
        if (why) *why = "有效血量计算越界";
        return 0;
    }
    return 1;
}

/* ================================================================== */
/* 规则文件：<本 DLL 目录>\rules.d\*.json（先，按文件名升序）+          */
/*           <本 DLL 目录>\rules.json（后）                            */
/*                                                                    */
/* 合并语义与优先级理由见 RuleSet 的注释：更具体的（随 mod 分发的）文件 */
/* 先于玩家的通用 rules.json；文件内仍是"第一条 enabled 且命中生效"。  */
/* ================================================================== */
static const char kDefaultRules[] =
    "{\n"
    "  \"schema\": 1,\n"
    "  \"enabled\": false,\n"
    "  \"rules\": [\n"
    "    {\n"
    "      \"id\": \"example-mid-dungeon\",\n"
    "      \"name\": \"\xe7\xa4\xba\xe4\xbe\x8b\xef\xbc\x9a\xe6\x8c\x87\xe5\xae\x9a\xe5\x89\xaf"
    "\xe6\x9c\xac 10 \xe5\x80\x8d\xe8\xa1\x80\xe9\x87\x8f\",\n"
    "      \"enabled\": false,\n"
    "      \"dungeonIds\": [100005014],\n"
    "      \"percent\": 1000,\n"
    "      \"attackPercent\": 100\n"
    "    },\n"
    "    {\n"
    "      \"id\": \"example-all-dungeons\",\n"
    "      \"name\": \"\xe7\xa4\xba\xe4\xbe\x8b\xef\xbc\x9a\xe5\x85\xa8\xe9\x83\xa8\xe5"
    "\x89\xaf\xe6\x9c\xac 2 \xe5\x80\x8d\xe8\xa1\x80\xe9\x87\x8f 10 \xe5\x80\x8d\xe4\xbc"
    "\xa4\xe5\xae\xb3\",\n"
    "      \"enabled\": false,\n"
    "      \"all\": true,\n"
    "      \"dungeonIds\": [],\n"
    "      \"percent\": 200,\n"
    "      \"attackPercent\": 1000\n"
    "    }\n"
    "  ]\n"
    "}\n";

static void rules_path(wchar_t *out, size_t cap) { _snwprintf(out, cap, L"%s\\rules.json", g_dir); }

/* <本 DLL 目录>\rules.d —— mod 自带的"按范围"规则子目录（可以不存在）。 */
static void rules_dir_path(wchar_t *out, size_t cap) { _snwprintf(out, cap, L"%s\\rules.d", g_dir); }

static int write_default_rules(void) {
    wchar_t path[MAX_PATH];
    FILE *f;
    rules_path(path, MAX_PATH);
    f = _wfopen(path, L"wb");
    if (!f) return 0;
    fwrite(kDefaultRules, 1, sizeof(kDefaultRules) - 1, f);
    fclose(f);
    return 1;
}

/* 读任意路径的文本（与 read_rules_file 同一套上限/错误码）。
 * 返回：1 = 读到；0 = 文件不存在；-1 = 读失败/过大。 */
static int read_text_file(const wchar_t *path, char **text, size_t *len) {
    FILE *f;
    long n;
    char *buf;
    f = _wfopen(path, L"rb");
    if (!f) return 0;
    if (fseek(f, 0, SEEK_END) != 0) { fclose(f); return -1; }
    n = ftell(f);
    if (n < 0 || (unsigned long)n > MAX_JSON_BYTES) { fclose(f); return -1; }
    if (fseek(f, 0, SEEK_SET) != 0) { fclose(f); return -1; }
    buf = (char *)malloc((size_t)n + 1);
    if (!buf) { fclose(f); return -1; }
    if (n > 0 && fread(buf, 1, (size_t)n, f) != (size_t)n) {
        free(buf);
        fclose(f);
        return -1;
    }
    fclose(f);
    buf[n] = 0;
    *text = buf;
    *len = (size_t)n;
    return 1;
}

/* 返回：1 = 读到；0 = 文件不存在；-1 = 读失败/过大 */
static int read_rules_file(char **text, size_t *len) {
    wchar_t path[MAX_PATH];
    rules_path(path, MAX_PATH);
    return read_text_file(path, text, len);
}

/* --- 极简手写 JSON（不引第三方库） --------------------------------- */
typedef struct {
    const char *p;
    const char *end;
} JsonCur;

static void json_ws(JsonCur *c) {
    while (c->p < c->end && (*c->p == ' ' || *c->p == '\t' || *c->p == '\r' || *c->p == '\n'))
        c->p++;
}

static int json_str(JsonCur *c, char *out, size_t cap) {
    size_t n = 0;
    if (c->p >= c->end || *c->p != '"') return 0;
    c->p++;
    while (c->p < c->end && *c->p != '"') {
        char ch = *c->p++;
        if (ch == '\\') {
            if (c->p >= c->end) return 0;
            ch = *c->p++;
            switch (ch) {
            case 'n': ch = '\n'; break;
            case 't': ch = '\t'; break;
            case 'r': ch = '\r'; break;
            case 'b': ch = '\b'; break;
            case 'f': ch = '\f'; break;
            case 'u': {
                unsigned v = 0;
                int k;
                for (k = 0; k < 4; k++) {
                    int d;
                    char h;
                    if (c->p >= c->end) return 0;
                    h = *c->p++;
                    if (h >= '0' && h <= '9') d = h - '0';
                    else if (h >= 'a' && h <= 'f') d = h - 'a' + 10;
                    else if (h >= 'A' && h <= 'F') d = h - 'A' + 10;
                    else return 0;
                    v = v * 16 + (unsigned)d;
                }
                ch = (v < 0x80) ? (char)v : '?'; /* id/name 只用于日志 */
                break;
            }
            default: break;
            }
        }
        if (n + 1 < cap) out[n++] = ch;
    }
    if (c->p >= c->end) return 0;
    c->p++; /* 收尾引号 */
    out[n] = 0;
    return 1;
}

static int json_skip_value(JsonCur *c);

static int json_skip_container(JsonCur *c, char open, char close) {
    if (c->p >= c->end || *c->p != open) return 0;
    c->p++;
    json_ws(c);
    if (c->p < c->end && *c->p == close) { c->p++; return 1; }
    for (;;) {
        if (open == '{') {
            char key[64];
            json_ws(c);
            if (!json_str(c, key, sizeof(key))) return 0;
            json_ws(c);
            if (c->p >= c->end || *c->p != ':') return 0;
            c->p++;
        }
        if (!json_skip_value(c)) return 0;
        json_ws(c);
        if (c->p < c->end && *c->p == ',') { c->p++; continue; }
        if (c->p < c->end && *c->p == close) { c->p++; return 1; }
        return 0;
    }
}

static int json_skip_value(JsonCur *c) {
    json_ws(c);
    if (c->p >= c->end) return 0;
    if (*c->p == '{') return json_skip_container(c, '{', '}');
    if (*c->p == '[') return json_skip_container(c, '[', ']');
    if (*c->p == '"') {
        char tmp[2];
        return json_str(c, tmp, sizeof(tmp));
    }
    while (c->p < c->end && *c->p != ',' && *c->p != '}' && *c->p != ']') c->p++;
    return 1;
}

/* ["100005014","100005015"] 或 [100005014] → id 数组 */
static int json_id_array(JsonCur *c, Rule *r) {
    if (c->p >= c->end || *c->p != '[') return 0;
    c->p++;
    json_ws(c);
    if (c->p < c->end && *c->p == ']') { c->p++; return 1; }
    for (;;) {
        char tok[32];
        char *endp = NULL;
        unsigned long long v;
        json_ws(c);
        if (c->p < c->end && *c->p == '"') {
            if (!json_str(c, tok, sizeof(tok))) return 0;
        } else {
            size_t i = 0;
            while (c->p < c->end && *c->p != ',' && *c->p != ']' && i + 1 < sizeof(tok))
                tok[i++] = *c->p++;
            tok[i] = 0;
        }
        v = _strtoui64(tok, &endp, 10);
        if (endp != tok && v <= 0xffffffffULL) {
            if (r->id_count < MAX_RULE_IDS) r->ids[r->id_count] = (uint32_t)v;
            r->id_count++;
            if (r->id_count > MAX_RULE_IDS) r->id_overflow = 1;
        }
        json_ws(c);
        if (c->p < c->end && *c->p == ',') { c->p++; continue; }
        if (c->p < c->end && *c->p == ']') { c->p++; return 1; }
        return 0;
    }
}

static int json_one_rule(JsonCur *c, Rule *r) {
    if (c->p >= c->end || *c->p != '{') return 0;
    c->p++;
    _snprintf_s(r->id, sizeof(r->id), _TRUNCATE, "rule");
    r->percent = 100;
    r->attack_percent = 100; /* 省略 = 100 = 不改（向后兼容） */
    json_ws(c);
    if (c->p < c->end && *c->p == '}') { c->p++; return 1; }
    for (;;) {
        char key[64];
        json_ws(c);
        if (!json_str(c, key, sizeof(key))) return 0;
        json_ws(c);
        if (c->p >= c->end || *c->p != ':') return 0;
        c->p++;
        json_ws(c);
        if (_stricmp(key, "id") == 0) {
            if (!json_str(c, r->id, sizeof(r->id))) return 0;
        } else if (_stricmp(key, "name") == 0) {
            if (!json_str(c, r->name, sizeof(r->name))) return 0;
        } else if (_stricmp(key, "enabled") == 0 || _stricmp(key, "all") == 0) {
            int *dst = (_stricmp(key, "all") == 0) ? &r->all : &r->enabled;
            if (c->end - c->p >= 4 && strncmp(c->p, "true", 4) == 0) { *dst = 1; c->p += 4; }
            else if (c->end - c->p >= 5 && strncmp(c->p, "false", 5) == 0) { *dst = 0; c->p += 5; }
            else return 0;
        } else if (_stricmp(key, "percent") == 0 || _stricmp(key, "attackPercent") == 0) {
            char tok[32];
            size_t i = 0;
            uint32_t *dst = (_stricmp(key, "attackPercent") == 0) ? &r->attack_percent : &r->percent;
            while (c->p < c->end && ((*c->p >= '0' && *c->p <= '9') || *c->p == '-') &&
                   i + 1 < sizeof(tok))
                tok[i++] = *c->p++;
            tok[i] = 0;
            if (i == 0) return 0;
            {
                char *endp = NULL;
                unsigned long long v = _strtoui64(tok, &endp, 10);
                if (endp == tok || v > 0xffffffffULL) return 0;
                *dst = (uint32_t)v;
            }
        } else if (_stricmp(key, "dungeonIds") == 0) {
            if (!json_id_array(c, r)) return 0;
        } else {
            if (!json_skip_value(c)) return 0;
        }
        json_ws(c);
        if (c->p < c->end && *c->p == ',') { c->p++; continue; }
        if (c->p < c->end && *c->p == '}') { c->p++; return 1; }
        return 0;
    }
}

/* --- 规则集：多文件合并后的扁平视图（自测兼容访问器见 PARSED_* 宏） --- */

/* 取第 f 份文件的第 i 条规则（rules 为 NULL 时返回 NULL）。 */
static const Rule *rf_rule(const RuleFile *rf, int i) {
    if (rf == NULL || rf->rules == NULL || i < 0 || i >= rf->rule_count) return NULL;
    return &rf->rules[i];
}

static RuleFile *ruleset_add_file_slot(RuleSet *set, const char *name) {
    RuleFile *f;
    if (set->rule_file_count >= MAX_RULE_FILES) return NULL;
    f = &set->files[set->rule_file_count]; /* 失败时不递增计数 */
    memset(f, 0, sizeof(*f));
    _snprintf_s(f->name, sizeof(f->name), _TRUNCATE, "%s", name);
    f->rules = (Rule *)calloc(MAX_RULES, sizeof(Rule));
    if (f->rules == NULL) return NULL;
    set->rule_file_count++;
    return f;
}

/* 规则集是纯堆视图：清空 = 释放各文件数组 + 计数清零（可重复调用）。 */
static void ruleset_clear(RuleSet *set) {
    int i;
    for (i = 0; i < set->rule_file_count; i++) {
        free(set->files[i].rules);
        set->files[i].rules = NULL;
    }
    memset(set, 0, sizeof(*set));
}

static int ruleset_total(const RuleSet *set) {
    int i, n = 0;
    for (i = 0; i < set->rule_file_count; i++) n += set->files[i].rule_count;
    return n;
}

/* 把**一个文件**的解析结果并入总表（*one 的 rule_file_count 固定 = 1）。
 * 返回 0 = 越界（规则总数超 MAX_RULES 或文件数超 MAX_RULE_FILES）或分配失败，
 * 调用方据此把整份配置判不可用（与旧版"规则数超过 32 条 → 不可用"同义）。 */
static int add_rule_file(RuleSet *set, const char *name, const RuleSet *one) {
    RuleFile *f;
    const RuleFile *src = &one->files[0];
    if (set->rule_file_count + one->rule_file_count > MAX_RULE_FILES) return 0;
    if (ruleset_total(set) + src->rule_count > MAX_RULES) return 0;
    f = ruleset_add_file_slot(set, name);
    if (f == NULL) return 0;
    f->enabled = src->enabled;
    f->rule_count = src->rule_count;
    if (src->rule_count > 0) {
        if (src->rules == NULL) { f->rule_count = 0; return 0; }
        memcpy((void *)f->rules, (const void *)src->rules,
               (size_t)src->rule_count * sizeof(Rule));
    }
    return 1;
}

static int ruleset_ready(RuleSet *set) {
    /* 解析出参要能直接写规则，所以进入 parse_rules 前必须先建好 files[0]。 */
    if (set->rule_file_count == 0) set->rule_file_count = 1;
    if (set->files[0].rules == NULL) set->files[0].rules = (Rule *)calloc(MAX_RULES, sizeof(Rule));
    return set->files[0].rules != NULL;
}

/* 返回 1 = 解析成功；失败时 *why 给出原因。
 *
 * 出参约定：*out 表示**这一个文件**（rule_file_count == 1，文件内容全在 files[0] 里）。
 * 旧版 RuleSet 是扁平的单文件结构（loaded/enabled/rule_count/rules[]），
 * 上面的 PARSED_* 宏就是给自测保留那套读法的兼容访问器。 */
#define PARSED_RULES(set)      ((set).files[0].rules)
#define PARSED_RULE_COUNT(set) ((set).files[0].rule_count)
#define PARSED_ENABLED(set)    ((set).files[0].enabled)

static int parse_rules(const char *text, size_t len, RuleSet *out, char *why, size_t whycap) {
    JsonCur c;
    int have_schema = 0, have_enabled = 0, have_rules = 0;
    int overflow = 0, i;
    RuleFile *rf;
    ruleset_clear(out);
    if (!ruleset_ready(out)) { /* 建不出规则数组：按解析失败处理（宁失效不崩） */
        _snprintf_s(why, whycap, _TRUNCATE, "内存不足（规则数组分配失败）");
        return 0;
    }
    out->rule_file_count = 1; /* 解析出参固定表示"一个文件" */
    rf = &out->files[0];
    _snprintf_s(rf->name, sizeof(rf->name), _TRUNCATE, "rules.json");
    c.p = text;
    c.end = text + len;
    json_ws(&c);
    if (c.p >= c.end || *c.p != '{') {
        _snprintf_s(why, whycap, _TRUNCATE, "顶层不是 JSON 对象");
        return 0;
    }
    c.p++;
    json_ws(&c);
    if (c.p < c.end && *c.p == '}') c.p++;
    while (c.p < c.end && *c.p != '}') {
        char key[64];
        json_ws(&c);
        if (!json_str(&c, key, sizeof(key))) {
            _snprintf_s(why, whycap, _TRUNCATE, "键名解析失败");
            return 0;
        }
        json_ws(&c);
        if (c.p >= c.end || *c.p != ':') {
            _snprintf_s(why, whycap, _TRUNCATE, "键 %s 后缺冒号", key);
            return 0;
        }
        c.p++;
        json_ws(&c);
        if (_stricmp(key, "schema") == 0) {
            char tok[16];
            size_t n = 0;
            while (c.p < c.end && *c.p >= '0' && *c.p <= '9' && n + 1 < sizeof(tok))
                tok[n++] = *c.p++;
            tok[n] = 0;
            if (n == 0 || atoi(tok) != 1) {
                _snprintf_s(why, whycap, _TRUNCATE, "schema 必须是 1");
                return 0;
            }
            have_schema = 1;
        } else if (_stricmp(key, "enabled") == 0) {
            if (c.end - c.p >= 4 && strncmp(c.p, "true", 4) == 0) { rf->enabled = 1; c.p += 4; }
            else if (c.end - c.p >= 5 && strncmp(c.p, "false", 5) == 0) { rf->enabled = 0; c.p += 5; }
            else {
                _snprintf_s(why, whycap, _TRUNCATE, "enabled 不是 true/false");
                return 0;
            }
            have_enabled = 1;
        } else if (_stricmp(key, "rules") == 0) {
            if (c.p >= c.end || *c.p != '[') {
                _snprintf_s(why, whycap, _TRUNCATE, "rules 不是数组");
                return 0;
            }
            c.p++;
            json_ws(&c);
            if (c.p < c.end && *c.p == ']') c.p++;
            while (c.p < c.end && *c.p != ']') {
                Rule r;
                memset(&r, 0, sizeof(r));
                if (!json_one_rule(&c, &r)) {
                    _snprintf_s(why, whycap, _TRUNCATE, "第 %d 条规则解析失败", rf->rule_count + 1);
                    return 0;
                }
                if (rf->rule_count < MAX_RULES) {
                    rf->rules[rf->rule_count++] = r;
                    if (r.id_overflow) overflow = 1;
                } else {
                    overflow = 1; /* 超过 MAX_RULES（gmrules.MaxRules = 32） */
                }
                json_ws(&c);
                if (c.p < c.end && *c.p == ',') { c.p++; json_ws(&c); continue; }
                break;
            }
            json_ws(&c);
            if (c.p >= c.end || *c.p != ']') {
                _snprintf_s(why, whycap, _TRUNCATE, "rules 数组未正常闭合");
                return 0;
            }
            c.p++;
            have_rules = 1;
        } else {
            if (!json_skip_value(&c)) {
                _snprintf_s(why, whycap, _TRUNCATE, "未知键 %s 的值解析失败", key);
                return 0;
            }
        }
        json_ws(&c);
        if (c.p < c.end && *c.p == ',') { c.p++; json_ws(&c); continue; }
        break;
    }
    json_ws(&c);
    if (c.p >= c.end || *c.p != '}') {
        _snprintf_s(why, whycap, _TRUNCATE, "顶层对象未正常闭合（文件被截断？）");
        return 0;
    }
    if (!have_schema || !have_enabled || !have_rules) {
        _snprintf_s(why, whycap, _TRUNCATE, "缺少必需键：%s%s%s", have_schema ? "" : "schema ",
                    have_enabled ? "" : "enabled ", have_rules ? "" : "rules");
        return 0;
    }
    if (overflow) {
        _snprintf_s(why, whycap, _TRUNCATE, "规则数超过 %d 条，或单条 dungeonIds 超过 %d 项",
                    MAX_RULES, MAX_RULE_IDS);
        return 0;
    }
    for (i = 0; i < rf->rule_count; i++) {
        Rule *r = &rf->rules[i];
        if (!r->enabled) continue;
        if (r->percent < 1 || r->percent > 100000) {
            _snprintf_s(why, whycap, _TRUNCATE, "规则 %s 的 percent=%u 超出 1..100000", r->id,
                        r->percent);
            return 0;
        }
        /* 伤害倍率与血量同口径：越界（含 0）整份配置判不可用，
         * 绝不"用一半"（例如只放大血量、不放大攻击）。 */
        if (r->attack_percent < 1 || r->attack_percent > 100000) {
            _snprintf_s(why, whycap, _TRUNCATE,
                        "规则 %s 的 attackPercent=%u 超出 1..100000（100 = 不改伤害）", r->id,
                        r->attack_percent);
            return 0;
        }
        if (!r->all && r->id_count == 0) {
            _snprintf_s(why, whycap, _TRUNCATE,
                        "规则 %s 既没写 all:true 也没有 dungeonIds，无法判定作用范围", r->id);
            return 0;
        }
    }
    /* 解析成功：不回写 loaded（现在由 rule_file_count 表达） */
    return 1;
}

/* --- 规则集：多文件合并后的扁平视图 --------------------------------- */

/* 规则匹配：**先按文件顺序，文件内第一条 enabled 且命中的规则生效**
 * （跨文件"先遍历到的命中就赢"；gmrules.Rule.Matches 的 selected / all 子集）。 */
static const Rule *match_rule(uint32_t dungeon_id, int have_dungeon) {
    int fi, i, k;
    RuleSet *set = &g_engine.set;
    set->matched_file[0] = 0;
    for (fi = 0; fi < set->rule_file_count; fi++) {
        const RuleFile *rf = &set->files[fi];
        if (!rf->enabled) continue; /* 文件顶层 enabled=false → 该文件整份不参与 */
        for (i = 0; i < rf->rule_count; i++) {
            const Rule *r = rf_rule(rf, i);
            int hit = 0;
            if (!r->enabled) continue;
            if (r->all) {
                hit = 1;
            } else if (have_dungeon) {
                for (k = 0; k < r->id_count && k < MAX_RULE_IDS; k++) {
                    if (r->ids[k] == dungeon_id) { hit = 1; break; }
                }
            }
            if (hit) {
                _snprintf_s(set->matched_file, sizeof(set->matched_file), _TRUNCATE, "%s", rf->name);
                return r;
            }
        }
    }
    return NULL;
}

/* --- 配置加载 ------------------------------------------------------ */
static uint64_t fnv1a(const unsigned char *p, size_t n, uint64_t h) {
    size_t i;
    for (i = 0; i < n; i++) {
        h ^= p[i];
        h *= 1099511628211ULL;
    }
    return h;
}

typedef struct {
    int exists;
    uint64_t size;
    DWORD lo, hi;
} FileState;

static void rules_file_state(FileState *st) {
    wchar_t path[MAX_PATH];
    WIN32_FILE_ATTRIBUTE_DATA fad;
    memset(st, 0, sizeof(*st));
    rules_path(path, MAX_PATH);
    if (!GetFileAttributesExW(path, GetFileExInfoStandard, &fad)) return;
    st->exists = 1;
    st->size = ((uint64_t)fad.nFileSizeHigh << 32) | fad.nFileSizeLow;
    st->lo = fad.ftLastWriteTime.dwLowDateTime;
    st->hi = fad.ftLastWriteTime.dwHighDateTime;
}

/* 文件状态是否变过；变过就把快照更新为新状态并回答"变了"。
 * 注意语义：这里同时**消费**掉 st，调用方拿到 1 就说明该重读。 */
static int file_state_changed(FileState *st) {
    int changed;
    if (st->exists != (g_engine.mtime_lo != 0 || g_engine.mtime_hi != 0)) changed = 1;
    /* 文件不存在时**绝不能**回答"没变化"（2026-10-07 实机 bug）：
     * 首次运行必须走到 config_try() 去落地默认 rules.json；原来这里返回
     * `g_engine.hash == 0`（= "什么都没加载过，不必动"）会把生成路径**饿死** ——
     * 于是每 200 ms 都是"unchanged"，rules.json 永不生成、配置永远不可用
     * （现场：日志只有"配置不可用（…文件缺失）"，盘上没有 rules.json，
     *   status.json = enabled:false / lastError:configDisabled）。 */
    else if (!st->exists) changed = 1;
    else if (g_engine.hash == 0) changed = 1; /* 上次文件存在但读失败，重试 */
    else if (st->size != g_engine.fast_size || st->lo != g_engine.mtime_lo ||
             st->hi != g_engine.mtime_hi)
        changed = 1;
    else
        changed = 0;
    g_engine.mtime_lo = st->lo;
    g_engine.mtime_hi = st->hi;
    g_engine.fast_size = st->size;
    return changed;
}

/* --- rules.d 扫描 --------------------------------------------------- */

static int wcs_ends_with_json(const wchar_t *n) {
    size_t l = wcslen(n);
    if (l < 5) return 0;
    return _wcsicmp(n + l - 5, L".json") == 0;
}

static int wcs_ends_with(const wchar_t *n, const wchar_t *suffix) {
    size_t l = wcslen(n), m = wcslen(suffix);
    if (l < m) return 0;
    return _wcsicmp(n + l - m, suffix) == 0;
}

/* 把一条 DIRENT 的 cFileName 拷进出参（只取 basename，防御性）。 */
static void dir_entry_name(const wchar_t *cFileName, wchar_t *out, size_t cap) {
    const wchar_t *base = cFileName;
    size_t i, n;
    for (i = 0; cFileName[i]; i++)
        if (cFileName[i] == L'\\' || cFileName[i] == L'/') base = cFileName + i + 1;
    n = wcslen(base);
    if (n + 1 > cap) n = cap - 1;
    memcpy(out, base, n * sizeof(wchar_t));
    out[n] = 0;
}

/* 目录指纹：把（文件数, 每个文件的 size/mtime/首字符）折成一个 64 位值。
 * 只用来**快速判断"要不要重读"**；rules.json 那一侧仍以**内容哈希**为准
 * （所以哈希碰撞的后果只是"多做一次无用的重读"）。 */
static uint64_t dir_stat_fold(uint64_t h, const uint64_t *v, int n) {
    int i;
    for (i = 0; i < n; i++) {
        h ^= v[i];
        h *= 1099511628211ULL;
    }
    return h;
}

/* 枚举 <本 DLL 目录>\rules.d 下的 *.json，按**文件名升序**写入 out。
 * 目录不存在 → 返回 0（不算错误，按"mod 没带范围规则"处理）。
 * 超过 MAX_RULE_FILES 份 → 只保留排序后的前 MAX_RULE_FILES 份并返回负值（错误）。 */
static int scan_rules_dir(DirRuleFile *out, int cap, uint64_t *fingerprint) {
    wchar_t dir[MAX_PATH], pattern[MAX_PATH];
    WIN32_FIND_DATAW fd;
    HANDLE h;
    int n = 0, sorted, i, overflow = 0;
    uint64_t fold[1 + MAX_RULE_FILES * 3];
    int fn = 0;

    *fingerprint = 0;
    rules_dir_path(dir, MAX_PATH);
    _snwprintf(pattern, MAX_PATH, L"%s\\*.json", dir);
    h = FindFirstFileW(pattern, &fd);
    if (h == INVALID_HANDLE_VALUE) return 0; /* rules.d 不存在 / 没有 .json */
    do {
        if (fd.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY) continue;
        if (!wcs_ends_with_json(fd.cFileName)) continue;
        if (n >= cap) { overflow = 1; continue; }
        dir_entry_name(fd.cFileName, out[n].name, MAX_PATH);
        out[n].size = ((uint64_t)fd.nFileSizeHigh << 32) | fd.nFileSizeLow;
        out[n].lo = fd.ftLastWriteTime.dwLowDateTime;
        out[n].hi = fd.ftLastWriteTime.dwHighDateTime;
        n++;
    } while (FindNextFileW(h, &fd));
    FindClose(h);
    /* 文件名升序 —— 这就是**跨文件优先级**（先遍历到的命中就赢） */
    for (sorted = 0; !sorted;) {
        sorted = 1;
        for (i = 1; i < n; i++) {
            if (_wcsicmp(out[i - 1].name, out[i].name) > 0) {
                DirRuleFile t = out[i - 1];
                out[i - 1] = out[i];
                out[i] = t;
                sorted = 0;
            }
        }
    }
    fold[fn++] = (uint64_t)n;
    for (i = 0; i < n; i++) {
        fold[fn++] = out[i].size;
        fold[fn++] = ((uint64_t)out[i].hi << 32) | out[i].lo;
        fold[fn++] = (uint64_t)out[i].name[0];
    }
    *fingerprint = dir_stat_fold(14695981039346656037ULL, fold, fn);
    if (*fingerprint == 0) *fingerprint = 1; /* 0 保留给"目录不存在/空" */
    return overflow ? -n : n;
}

/* 解析并并入 rules.d 里的每一份文件。
 * **单个文件解析失败只跳过它自己**（记一条醒目日志），不影响其它文件、
 * 也不会让整份配置失效 —— 这正是 mod 自带规则要的行为：mod 的 rules.d 里
 * 塞了一份坏文件，玩家自己的 rules.json 仍要照常工作。
 * 返回 0 = 越界（规则总数/文件数超上限）→ 调用方判整份配置不可用。 */
static int load_rules_dir_files(RuleSet *set, const DirRuleFile *files, int count, int overflow) {
    int i, loaded = 0, skipped = 0;
    if (overflow > 0) {
        logf_("rules.d 里的 .json 超过 %d 份 —— 只加载文件名升序的前 %d 份，其余忽略",
              MAX_RULE_FILES, MAX_RULE_FILES);
    }
    for (i = 0; i < count; i++) {
        wchar_t path[MAX_PATH];
        char *text = NULL;
        size_t len = 0;
        int r;
        RuleSet one;
        char why[256];
        rules_dir_path(path, MAX_PATH);
        _snwprintf(path, MAX_PATH, L"%s\\rules.d\\%s", g_dir, files[i].name);
        memset(&one, 0, sizeof(one));
        r = read_text_file(path, &text, &len);
        if (r == 0) {
            skipped++;
            logf_("  [跳过] rules.d\\%ls：读取时文件已消失", files[i].name);
            continue;
        }
        if (r < 0) {
            skipped++;
            logf_("  [跳过] rules.d\\%ls：读取失败（文件过大或 I/O 错误）—— 只跳过这一份",
                  files[i].name);
            continue;
        }
        if (!parse_rules(text, len, &one, why, sizeof(why))) {
            skipped++;
            logf_("  [跳过] rules.d\\%ls：解析失败：%s —— **只跳过这一份**，其它规则文件照常",
                  files[i].name, why);
            free(text);
            continue;
        }
        free(text);
        {
            char name8[128];
            w2u(files[i].name, name8, sizeof(name8));
            if (!add_rule_file(set, name8, &one)) {
                ruleset_clear(&one);
                logf_("rules.d 规则越界（总条数上限 %d / 文件数上限 %d）—— 整份配置判不可用",
                      MAX_RULES, MAX_RULE_FILES);
                return 0;
            }
            ruleset_clear(&one); /* one 是临时出参，并进 set 后立刻释放自己的数组 */
        }
        loaded++;
    }
    if (loaded > 0 || skipped > 0) {
        logf_("rules.d：加载 %d 份、跳过 %d 份（升序优先级：先遍历到的文件命中就赢）", loaded,
              skipped);
    }
    return 1;
}

/* 读 rules.json + rules.d/*.json → 解析 → 与生效配置比**合并内容哈希**；不同就替换。
 * 返回 1 = 内容发生了变化（哪怕变化后不可用）。 */
static int config_try(const FileState *st) {
    char *text = NULL;
    size_t len = 0;
    uint64_t hash = 0, dir_fp = 0;
    RuleSet set, main_set;
    char why[256];
    int r, dir_count, dir_overflow = 0, parser_failed = 0;
    DirRuleFile dir_files[MAX_RULE_FILES];

    memset(&set, 0, sizeof(set));
    memset(&main_set, 0, sizeof(main_set));
    memset(dir_files, 0, sizeof(dir_files));
    dir_count = scan_rules_dir(dir_files, MAX_RULE_FILES, &dir_fp);
    if (dir_count < 0) {
        dir_overflow = 1;
        dir_count = -dir_count; /* 目录里的实际份数（超出的那些只记日志、不加载） */
    }

    if (!st->exists) {
        /* 没有 rules.json：落一份默认（enabled=false），下次快照就能读到它 */
        if (g_engine.hash != 0 || g_engine.fast_size != 0) {
            /* 之前有、现在没了：与旧版逐字一致 —— set 清零、按不可用处理。
             * rules.d 里的文件保持不动（但 set 已清零，所以它们也不再参与匹配）。 */
            g_engine.hash = 0;
            g_engine.dir_fingerprint = dir_fp;
            g_engine.rule_file_count = dir_count;
            ruleset_clear(&g_engine.set);
            logf_("rules.json 已被删除 —— 按不可用处理（保持/恢复原版）");
            return 1;
        }
        if (!write_default_rules()) {
            logf_("rules.json 不存在且写不出默认文件（目录只读？）—— 按不可用处理，稍后重试");
            return 0;
        }
        logf_("首次运行：已在本目录生成默认规则文件 rules.json（enabled=false，默认不加强）");
        return 0; /* 下一轮再读回来 */
    }

    /* rules.json 的"读取失败 / 解析失败"保持旧语义：整份配置不可用。 */
    r = read_rules_file(&text, &len);
    if (r != 1) {
        logf_("rules.json 读取失败（文件过大或 I/O 错误）—— 按不可用处理");
        g_engine.hash = 0;
        g_engine.dir_fingerprint = dir_fp;
        g_engine.rule_file_count = dir_count;
        ruleset_clear(&g_engine.set);
        return 1;
    }
    hash = fnv1a((const unsigned char *)text, len, 14695981039346656037ULL);
    if (hash == 0) hash = 1; /* 0 保留给"文件不存在" */
    if (!parse_rules(text, len, &main_set, why, sizeof(why))) {
        logf_("rules.json 解析失败：%s —— 本插件停止接管", why);
        parser_failed = 1;
    }
    free(text);
    /* 合并顺序 = 匹配优先级：**先 rules.d（文件名升序）**，最后才是 rules.json。
     * 为什么这样定见 RuleSet 的注释（mod 自带的"更具体"规则必须赢过玩家的通用规则）。 */
    if (!parser_failed && !load_rules_dir_files(&set, dir_files, dir_count, dir_overflow))
        parser_failed = 1;
    if (!parser_failed && !add_rule_file(&set, "rules.json", &main_set)) {
        logf_("规则越界（总条数上限 %d / 文件数上限 %d）—— 按不可用处理", MAX_RULES,
              MAX_RULE_FILES);
        parser_failed = 1;
    }
    ruleset_clear(&main_set); /* 临时出参，规则已并进 set */
    /* 先把"这一轮看到的是什么"落进状态，再判要不要替换生效配置：
     * 解析失败时也不该每 200 ms 重读同一份坏文件（那会刷屏），
     * 文件真的再被改动时目录指纹/内容哈希会变，自然会重试。
     * entry 表**不**清空 —— 改倍率时靠它保住 original/applied，防止复利。 */
    if (!parser_failed && hash == g_engine.hash && dir_fp == g_engine.dir_fingerprint &&
        set.rule_file_count == g_engine.rule_file_count) {
        ruleset_clear(&set); /* 内容没变（例如只是被 touch 了一下时间戳） */
        return 0;
    }
    g_engine.dir_fingerprint = dir_fp;
    g_engine.hash = hash;
    if (parser_failed) {
        ruleset_clear(&set);
        g_engine.rule_file_count = 0;
        ruleset_clear(&g_engine.set);
        return 1; /* 按不可用处理；下一轮不会因为"时间戳变了"重读 */
    }
    ruleset_clear(&g_engine.set); /* 先释放上一份，再接管新的一份 */
    g_engine.set = set;
    g_engine.rule_file_count = set.rule_file_count;
    {
        int i, k;
        logf_("规则已加载：共 %d 份文件、%d 条规则（rules.json 哈希 %#llx，%llu 字节；"
              "rules.d 指纹 %#llx、%d 个 .json）",
              set.rule_file_count, ruleset_total(&set), (unsigned long long)hash,
              (unsigned long long)st->size, (unsigned long long)dir_fp, dir_count);
        for (i = 0; i < set.rule_file_count; i++) {
            const RuleFile *rf = &set.files[i];
            logf_("  <%s> enabled=%d 共 %d 条", rf->name, rf->enabled, rf->rule_count);
            for (k = 0; k < rf->rule_count; k++) {
                const Rule *rr = rf_rule(rf, k);
                logf_("    [%d] id=%s%s%s percent=%u (%u.%02u 倍) attackPercent=%u (%u.%02u 倍) "
                      "副本数=%d %s",
                      k, rr->id, rr->enabled ? "" : " [停用]", rr->all ? " [全部副本]" : "",
                      rr->percent, rr->percent / 100, rr->percent % 100, rr->attack_percent,
                      rr->attack_percent / 100, rr->attack_percent % 100, rr->id_count, rr->name);
            }
        }
    }
    return 1;
}

static int config_usable(void) { return g_engine.set.rule_file_count > 0; }

/* ================================================================== */
/* 状态表：monster_runtime.go:303-348 的 Original/Applied/Percent      */
/* ================================================================== */

/* 只按**完整身份链**认领历史；地址复用一律当新怪
 * （Go: "Unknown/reused identities are never restored by raw address alone"） */
static Entry *entry_find(const MonsterId *id) {
    int i;
    for (i = 0; i < g_engine.entry_count; i++) {
        Entry *e = &g_engine.entries[i];
        if (!e->used) continue;
        if (e->id.actor == id->actor && e->id.node == id->node && e->id.manager == id->manager &&
            e->id.scene == id->scene && e->id.control == id->control && e->id.vtable == id->vtable &&
            e->id.key == id->key)
            return e;
    }
    return NULL;
}

static Entry *entry_new(const MonsterId *id) {
    int i;
    Entry *e = NULL;
    for (i = 0; i < g_engine.entry_count; i++) {
        if (!g_engine.entries[i].used) { e = &g_engine.entries[i]; break; }
    }
    if (!e) {
        if (g_engine.entry_count >= g_engine.entry_cap) {
            int cap = g_engine.entry_cap ? g_engine.entry_cap * 2 : 256;
            Entry *p = (Entry *)realloc(g_engine.entries, (size_t)cap * sizeof(Entry));
            if (!p) return NULL;
            memset(p + g_engine.entry_cap, 0, (size_t)(cap - g_engine.entry_cap) * sizeof(Entry));
            g_engine.entries = p;
            g_engine.entry_cap = cap;
        }
        e = &g_engine.entries[g_engine.entry_count++];
    }
    memset(e, 0, sizeof(*e));
    e->used = 1;
    e->id = *id;
    e->percent = 100;
    return e;
}

/* 本轮见过的身份（用于回收消失的怪） */
#define MAX_LIVE 1024
static MonsterId g_live[MAX_LIVE];
static int g_live_count;

static void live_add(const MonsterId *id) {
    if (g_live_count < MAX_LIVE) g_live[g_live_count++] = *id;
}
static int live_has(const MonsterId *id) {
    int i;
    for (i = 0; i < g_live_count; i++) {
        const MonsterId *l = &g_live[i];
        if (l->actor == id->actor && l->node == id->node && l->scene == id->scene) return 1;
    }
    return 0;
}

/* ================================================================== */
/* 写入：monster_runtime.go:349-439                                     */
/* ================================================================== */
#define MAX_EDITS (HP_LAYERS + 1)

static uint64_t desc_off(int layer) { return OFF_ACTOR_DESC0 + (uint64_t)layer * DESC_STRIDE; }

/* 写：四层 base + 当前血量。old_* 是"现场应有的值"（用来逐字节核对），
 * new_* 是要写进去的值。失败时回滚已写的部分。返回 1 = 全部写入并读回校验成功。 */
static int write_hp(const MonsterId *id, const uint64_t old_bases[HP_LAYERS],
                    const uint64_t new_bases[HP_LAYERS], uint64_t old_current, int64_t old_max,
                    int64_t new_max, uint64_t *new_current_out) {
    unsigned char before[MAX_EDITS][8], after[MAX_EDITS][8];
    uint64_t addr[MAX_EDITS];
    uint64_t next_current = 0;
    int layer, applied = 0;
    const char *err = NULL;

    if (!identity_valid(id)) {
        logf_("写前身份校验失败（对象已改变），本次跳过");
        return 0;
    }
    /* rescaleEffectiveHP：只乘 base 会让"当前血量/上限"脱钩（怪瞬死或无敌），
     * 所以当前血量按 (new_max/old_max) 同比例缩放，保住已受伤比例。 */
    if (!rescale_effective_hp(old_current, old_max, new_max, &next_current)) {
        logf_("当前血量(%llu)与有效上限(%lld)不一致，停止本次写入",
              (unsigned long long)old_current, (long long)old_max);
        return 0;
    }
    for (layer = 0; layer < HP_LAYERS; layer++) {
        addr[layer] = id->actor + desc_off(layer);
        if (!rd(addr[layer], before[layer], 8)) return 0;
        if (decrypt_attr64(raw64(before[layer])) != old_bases[layer]) {
            logf_("第 %d 层血量在写入前改变，本次跳过", layer);
            return 0;
        }
        put64(after[layer], encrypt_attr64(new_bases[layer]));
    }
    addr[HP_LAYERS] = id->actor + OFF_ACTOR_HP;
    if (!rd(addr[HP_LAYERS], before[HP_LAYERS], 8)) return 0;
    if (decrypt_attr64(raw64(before[HP_LAYERS])) != old_current) {
        logf_("当前血量在写入前改变，等下一轮");
        return 0;
    }
    put64(after[HP_LAYERS], encrypt_attr64(next_current));

    for (layer = 0; layer <= HP_LAYERS; layer++) {
        unsigned char now[8];
        if (!identity_valid(id)) { err = "写入中对象已销毁"; break; }
        if (!rd(addr[layer], now, 8) || memcmp(now, before[layer], 8) != 0) {
            err = "写入中血量改变";
            break;
        }
        applied++; /* 部分写入也要回滚 */
        if (!wr(addr[layer], after[layer], 8)) { err = "写入失败（页保护）"; break; }
        if (!rd(addr[layer], now, 8) || memcmp(now, after[layer], 8) != 0) {
            err = "血量写回校验失败";
            break;
        }
    }
    if (err) {
        int i;
        logf_("血量写入未完成（%s），回滚已写的 %d 项", err, applied);
        for (i = applied - 1; i >= 0; i--) {
            unsigned char now[8];
            if (!identity_valid(id)) break;
            if (rd(addr[i], now, 8) && memcmp(now, after[i], 8) == 0) wr(addr[i], before[i], 8);
        }
        return 0;
    }
    *new_current_out = next_current;
    return 1;
}

/* ================================================================== */
/* 场景快照：monster_runtime.go:167-283（只取"合格怪"这一段判据）        */
/* ================================================================== */
typedef struct {
    MonsterSnap mons[MAX_SCENE_MONSTERS];
    int count;
    int overflow;
    /* 诊断 */
    int raw;        /* 身份校验通过的实体数 */
    int rejected;   /* 基础值 / 当前血量不合格被丢掉的数 */
    int last_reject;/* 1=层描述块不可读 2=base guard 非 0 3=base 越界 4=当前血量不可读/越界 */
} SceneSnap;

static int collect_one(const MonsterId *id, void *vctx) {
    SceneSnap *snap = (SceneSnap *)vctx;
    MonsterSnap *m;
    uint64_t bases[HP_LAYERS];
    int layer;

    snap->raw++;
    if (snap->count >= MAX_SCENE_MONSTERS) {
        snap->overflow = 1;
        return 1;
    }
    /* :248-261 四层基础值 + guard + 上界（与 Go 的 snapshot 同一套判据） */
    for (layer = 0; layer < HP_LAYERS; layer++) {
        unsigned char b[12];
        if (!rd(id->actor + desc_off(layer), b, 12)) {
            snap->rejected++;
            snap->last_reject = 1;
            return 1;
        }
        if (raw32(b + 8) != 0) { /* guard 非 0 → 这个实体不是我们要的 */
            snap->rejected++;
            snap->last_reject = 2;
            return 1;
        }
        bases[layer] = decrypt_attr64(raw64(b));
        if (bases[layer] == 0 || bases[layer] > HP_SANE_MAX) {
            snap->rejected++;
            snap->last_reject = 3;
            return 1;
        }
    }
    /* :262-267 当前血量 */
    {
        uint64_t cur_raw = 0;
        if (!rd(id->actor + OFF_ACTOR_HP, &cur_raw, 8)) {
            snap->rejected++;
            snap->last_reject = 4;
            return 1;
        }
        m = &snap->mons[snap->count];
        m->current = decrypt_attr64(cur_raw);
        if (m->current == 0 || m->current > HP_SANE_MAX) {
            snap->rejected++;
            snap->last_reject = 4;
            return 1;
        }
    }
    m->id = *id;
    memcpy(m->bases, bases, sizeof(bases));
    m->rank = 0;
    {
        uint32_t rk = 0;
        if (rd32(id->actor + 0x66c0, &rk)) m->rank = rk; /* :244 品级（只做诊断） */
    }
    live_add(id); /* Go: live[id] = true（只有合格怪算"本轮见过"） */
    snap->count++;
    return 1;
}

static int id_equal(const MonsterId *a, const MonsterId *b) {
    return a->actor == b->actor && a->node == b->node && a->manager == b->manager &&
           a->scene == b->scene && a->control == b->control && a->vtable == b->vtable &&
           a->key == b->key;
}

static const MonsterSnap *snap_find(const SceneSnap *snap, const MonsterId *id) {
    int i;
    for (i = 0; i < snap->count; i++) {
        if (id_equal(&snap->mons[i].id, id)) return &snap->mons[i];
    }
    return NULL;
}

/* ================================================================== */
/* 一轮：Reconcile（monster_runtime.go:303-348）+ 应用规则              */
/* ================================================================== */
#define GATE_MAX 6
typedef struct {
    uint32_t percent;      /* 血量倍率（100 = 还原） */
    uint32_t attack_percent;
    int monsters;          /* 本轮合格怪数量 */
    int applied;
    int failed;
    int gate[GATE_MAX];    /* 血量路径各闸门被挡住的次数 */
    const char *gate_first[GATE_MAX];
} TickCtx;

static const char *gate_name(int g) {
    switch (g) {
    case 0: return "身份/现场值已被原生机制改变";
    case 1: return "血量层审计失败";
    case 2: return "写回失败/回滚";
    case 3: return "血量倍率溢出";
    case 4: return "当前血量与有效上限不一致";
    default: return "?";
    }
}

/* 单只怪的血量 Reconcile（Go: Reconcile 内层循环的一次迭代） */
static void hp_reconcile(const MonsterSnap *m, uint32_t percent, TickCtx *ctx) {
    const MonsterId *id = &m->id;
    Entry *e = entry_find(id);
    uint64_t new_bases[HP_LAYERS];
    int layer, changed, created = 0;

    if (e == NULL) {
        if (percent == 100) return; /* 保持原版：不建条目 */
        e = entry_new(id);
        if (!e) return;
        memcpy(e->original, m->bases, sizeof(m->bases));
        memcpy(e->applied, m->bases, sizeof(m->bases));
        e->percent = 100;
        created = 1;
    }
    if (!created && e->percent == percent) return; /* 已是目标倍率 */

    /* 现场必须还停在我们上次写入的值上，否则说明原生机制动过它 —— 不再碰这个怪 */
    for (layer = 0; layer < HP_LAYERS; layer++) {
        if (m->bases[layer] != e->applied[layer]) {
            ctx->gate[0]++;
            if (!ctx->gate_first[0]) ctx->gate_first[0] = "现场层基础值 ≠ 上次写入";
            logf_("实体 key=%llu 的第 %d 层基础值已被原生机制改变（现场 %llu ≠ 上次写入 %llu），"
                  "按 Go 侧策略放弃该怪",
                  (unsigned long long)(id->key & 0xffffULL), layer,
                  (unsigned long long)m->bases[layer], (unsigned long long)e->applied[layer]);
            return;
        }
    }
    changed = (percent != 100);
    for (layer = 0; layer < HP_LAYERS; layer++) {
        uint64_t v = 0;
        if (changed) {
            if (!scale_hp(e->original[layer], percent, &v)) {
                ctx->gate[3]++;
                if (!ctx->gate_first[3]) ctx->gate_first[3] = "scale_hp 溢出";
                logf_("血量倍率溢出（base=%llu percent=%u），跳过该怪",
                      (unsigned long long)e->original[layer], percent);
                return;
            }
        } else {
            v = e->original[layer];
        }
        new_bases[layer] = v;
    }
    {
        HpLayerAudit h_old, h_new;
        uint64_t new_cur = 0;
        const char *why = NULL;
        /* 有效上限必须用"改前/改后"两个基础值各算一次。
         * 改前：直接读现场（hpLayer(id, 3, 现场 base)）。
         * 改后：**只能**拿同一个审计结果去调 hp_maxima(new_base)
         *       —— 对应 Go 的 `h.maxima(bases[3])`。
         *       旧版这里错误地又调了一次 hp_layer(id, 3, new_base)，
         *       而那时内存里还是旧 base，`DecryptAttr64(*desc) == base` 必然失败，
         *       于是每只怪都在这里被静默丢掉：applied 永远 0、failed 也永远 0。 */
        if (!hp_layer(id, HP_LAYERS - 1, e->applied[HP_LAYERS - 1], &h_old, &why)) {
            ctx->gate[1]++;
            if (!ctx->gate_first[1]) ctx->gate_first[1] = why;
            return;
        }
        h_new = h_old;
        if (!hp_maxima(&h_new, new_bases[HP_LAYERS - 1], &h_new.before_final, &h_new.effective)) {
            ctx->gate[1]++;
            if (!ctx->gate_first[1]) ctx->gate_first[1] = "改后有效上限计算越界";
            return;
        }
        if (!write_hp(id, e->applied, new_bases, m->current, h_old.effective, h_new.effective,
                      &new_cur)) {
            ctx->failed++;
            ctx->gate[2]++;
            if (!ctx->gate_first[2]) ctx->gate_first[2] = "write_hp 失败";
            return;
        }
        memcpy(e->applied, new_bases, sizeof(new_bases));
        e->percent = percent;
        ctx->applied++;
        logf_("实体 key=%llu actor=%#llx 血量%s：%u.%02u 倍（层0 %llu→%llu，当前血量 %llu→%llu）",
              (unsigned long long)(id->key & 0xffffULL), (unsigned long long)id->actor,
              changed ? "已调整" : "已还原", percent / 100, percent % 100,
              (unsigned long long)e->original[0], (unsigned long long)new_bases[0],
              (unsigned long long)m->current, (unsigned long long)new_cur);
    }
}

/* ================================================================== */
/* 原生整数属性（怪物伤害）：monster_movement.go:41-172 + monster_combat */
/* ================================================================== */
typedef struct {
    uint32_t percent;
    int applied;
    int failed;
    int tracked;
} StatCtx;

static StatEntry *stat_find(const MonsterId *id, uint64_t offset) {
    int i;
    for (i = 0; i < g_engine.stat_count; i++) {
        StatEntry *s = &g_engine.stats[i];
        if (!s->used) continue;
        if (s->offset == offset && id_equal(&s->id, id)) return s;
    }
    return NULL;
}

static StatEntry *stat_new(const MonsterId *id, uint64_t offset) {
    int i;
    StatEntry *s = NULL;
    for (i = 0; i < g_engine.stat_count; i++) {
        if (!g_engine.stats[i].used) { s = &g_engine.stats[i]; break; }
    }
    if (!s) {
        if (g_engine.stat_count >= g_engine.stat_cap) {
            int cap = g_engine.stat_cap ? g_engine.stat_cap * 2 : 128;
            StatEntry *p = (StatEntry *)realloc(g_engine.stats, (size_t)cap * sizeof(StatEntry));
            if (!p) return NULL;
            memset(p + g_engine.stat_cap, 0, (size_t)(cap - g_engine.stat_cap) * sizeof(StatEntry));
            g_engine.stats = p;
            g_engine.stat_cap = cap;
        }
        s = &g_engine.stats[g_engine.stat_count++];
    }
    memset(s, 0, sizeof(*s));
    s->used = 1;
    s->id = *id;
    s->offset = offset;
    s->percent = 100;
    return s;
}

/* monster_movement.go:49-172 reconcileNativeStat 的单只怪版本。
 * 偏移必须先过白名单；写入前逐字节核对期望值，不符就跳过并记日志。 */
static void stat_reconcile(const MonsterSnap *m, uint64_t offset, uint32_t percent,
                           StatCtx *ctx) {
    const MonsterId *id = &m->id;
    StatEntry *s;
    unsigned char before[HP_LAYERS][8], after[HP_LAYERS][8];
    uint32_t values[HP_LAYERS], next[HP_LAYERS];
    uint64_t addr[HP_LAYERS];
    int layer, written = 0;
    const char *err = NULL;

    if (!native_stat_whitelisted(offset)) {
        logf_("未确认的原生属性偏移 %#llx —— 拒绝写入", (unsigned long long)offset);
        return;
    }
    s = stat_find(id, offset);
    if (s != NULL && s->percent == percent) return; /* 已是目标倍率 */

    for (layer = 0; layer < HP_LAYERS; layer++) {
        addr[layer] = id->actor + desc_off(layer) + offset;
        if (!rd(addr[layer], before[layer], 8)) {
            ctx->failed++;
            return;
        }
        if (!decode_native_int(before[layer], &values[layer])) {
            /* 逐字节核对不通过（stored/guard 对不上）→ 这个怪这项跳过 */
            if (!s) {
                logf_("实体 key=%llu 属性%#llx 层%d 原生整数校验失败（guard/stored 不符），跳过",
                      (unsigned long long)(id->key & 0xffffULL), (unsigned long long)offset, layer);
            }
            ctx->failed++;
            return;
        }
    }
    if (s == NULL) {
        if (percent == 100) return; /* 保持原版：不建条目 */
        s = stat_new(id, offset);
        if (!s) return;
        memcpy(s->original, values, sizeof(values));
        memcpy(s->applied, values, sizeof(values));
        s->percent = 100;
    }
    if (s->percent == percent) return;
    if (memcmp(values, s->applied, sizeof(values)) != 0) {
        logf_("实体 key=%llu 属性%#llx 已被原生机制改变（现场 ≠ 上次写入），按 Go 侧策略放弃",
              (unsigned long long)(id->key & 0xffffULL), (unsigned long long)offset);
        ctx->failed++;
        return;
    }
    for (layer = 0; layer < HP_LAYERS; layer++) {
        uint64_t v = (uint64_t)s->original[layer] * (uint64_t)percent / 100ULL;
        if (v > (uint64_t)NATIVE_INT_MAX) {
            logf_("原生整数属性%#llx 倍率溢出（层%d base=%u percent=%u），跳过",
                  (unsigned long long)offset, layer, s->original[layer], percent);
            ctx->failed++;
            return;
        }
        next[layer] = (uint32_t)v;
        encode_native_int_pair(after[layer], next[layer]);
    }
    for (layer = 0; layer < HP_LAYERS; layer++) {
        unsigned char now[8];
        if (!identity_valid(id)) { err = "写入中对象已销毁"; break; }
        if (!rd(addr[layer], now, 8) || memcmp(now, before[layer], 8) != 0) {
            err = "写入中属性改变";
            break;
        }
        written++; /* 部分写入也要回滚 */
        if (!wr(addr[layer], after[layer], 8)) { err = "写入失败（页保护）"; break; }
        if (!rd(addr[layer], now, 8) || memcmp(now, after[layer], 8) != 0) {
            err = "属性写回校验失败";
            break;
        }
    }
    if (err) {
        int i;
        logf_("属性%#llx 写入未完成（%s），回滚已写的 %d 项", (unsigned long long)offset, err,
              written);
        for (i = written - 1; i >= 0; i--) {
            unsigned char now[8];
            if (!identity_valid(id)) break;
            if (rd(addr[i], now, 8) && memcmp(now, after[i], 8) == 0) wr(addr[i], before[i], 8);
        }
        ctx->failed++;
        return;
    }
    memcpy(s->applied, next, sizeof(next));
    s->percent = percent;
    ctx->applied++;
    logf_("实体 key=%llu actor=%#llx 属性%#llx%s：%u.%02u 倍（层0 %u→%u）",
          (unsigned long long)(id->key & 0xffffULL), (unsigned long long)id->actor,
          (unsigned long long)offset, percent == 100 ? "已还原" : "已调整", percent / 100,
          percent % 100, s->original[0], next[0]);
}

/* 怪物伤害 = 物攻(0x398) + 魔攻(0x3b8)，两者同一倍率（monster_combat.go:16-40） */
static void attack_reconcile(const MonsterSnap *m, uint32_t percent, StatCtx *ctx) {
    stat_reconcile(m, OFF_STAT_ATK_PHYS, percent, ctx);
    stat_reconcile(m, OFF_STAT_ATK_MAG, percent, ctx);
}

/* 回收消失的怪（Go: for id := range r.entries { if !live[id] { delete } }） */
static void entries_reap(void) {
    int i;
    for (i = 0; i < g_engine.entry_count; i++) {
        Entry *e = &g_engine.entries[i];
        if (!e->used) continue;
        if (!live_has(&e->id)) e->used = 0;
    }
    for (i = 0; i < g_engine.stat_count; i++) {
        StatEntry *s = &g_engine.stats[i];
        if (!s->used) continue;
        if (!live_has(&s->id)) s->used = 0;
    }
}

static int entries_live_count(void) {
    int i, n = 0;
    for (i = 0; i < g_engine.entry_count; i++)
        if (g_engine.entries[i].used) n++;
    return n;
}
static int stats_live_count(void) {
    int i, n = 0;
    for (i = 0; i < g_engine.stat_count; i++)
        if (g_engine.stats[i].used) n++;
    return n;
}

/* 把"已被改过"的怪全部还原到 100%（停用 / 配置损坏 / 删除时用） */
static int restore_all(void) {
    SceneInfo sc;
    SceneSnap snap;
    TickCtx hp_ctx;
    StatCtx st_ctx;
    int i, n = 0, missed = 0;

    if (!g_ready) return 0;
    if (!find_scene(&sc)) {
        logf_("还原时找不到场景（已出图？）—— 内存里的倍率会留给本次会话，重启客户端即消失");
        return 0;
    }
    memset(&snap, 0, sizeof(snap));
    memset(&hp_ctx, 0, sizeof(hp_ctx));
    memset(&st_ctx, 0, sizeof(st_ctx));
    g_live_count = 0;
    walk_monsters(&sc, collect_one, &snap);
    hp_ctx.percent = 100;
    st_ctx.percent = 100;
    for (i = 0; i < g_engine.entry_count; i++) {
        Entry *e = &g_engine.entries[i];
        const MonsterSnap *m;
        if (!e->used || e->percent == 100) continue;
        m = snap_find(&snap, &e->id);
        n++;
        if (!m) { missed++; continue; }
        hp_reconcile(m, 100, &hp_ctx);
    }
    for (i = 0; i < g_engine.stat_count; i++) {
        StatEntry *s = &g_engine.stats[i];
        const MonsterSnap *m;
        if (!s->used || s->percent == 100) continue;
        m = snap_find(&snap, &s->id);
        n++;
        if (!m) { missed++; continue; }
        stat_reconcile(m, s->offset, 100, &st_ctx);
    }
    if (n > 0) {
        logf_("已尝试还原 %d 项曾被改过的字段（血量成功 %d/失败 %d；属性成功 %d/失败 %d；"
              "不在当前场景 %d）",
              n, hp_ctx.applied, hp_ctx.failed, st_ctx.applied, st_ctx.failed, missed);
    }
    return n;
}

/* ================================================================== */
/* 状态文件（只写插件自己目录）                                         */
/* ================================================================== */
static void json_escape(const char *src, char *dst, size_t cap) {
    size_t n = 0;
    for (; src && *src && n + 2 < cap; src++) {
        if (*src == '"' || *src == '\\') {
            dst[n++] = '\\';
            dst[n++] = *src;
        } else if ((unsigned char)*src < 0x20) {
            dst[n++] = ' ';
        } else {
            dst[n++] = *src;
        }
    }
    dst[n] = 0;
}

static void update_info(void) {
    wchar_t path[MAX_PATH];
    FILE *f;
    char err[400];
    char dungeon[32];
    if (!g_log) return;
    _snwprintf(path, MAX_PATH, L"%s\\difficulty-rules.status.json", g_dir);
    f = _wfopen(path, L"wb");
    if (!f) return;
    json_escape(g_info.last_error, err, sizeof(err));
    /* 旧版这里写的是 `g_info.has_dungeon ? "" : "null"`，于是有副本 id 时
     * 落盘成了 `"dungeonId": ,` —— 整份 status.json 不是合法 JSON。
     * 现在写真正的数字，取不到才写 null。 */
    if (g_info.has_dungeon)
        _snprintf_s(dungeon, sizeof(dungeon), _TRUNCATE, "%u", g_info.dungeon_id);
    else
        _snprintf_s(dungeon, sizeof(dungeon), _TRUNCATE, "null");
    fprintf(f,
            "{\n"
            "  \"schema\": 1,\n"
            "  \"mod\": \"difficulty.rules\",\n"
            "  \"pid\": %lu,\n"
            "  \"tickMs\": %d,\n"
            "  \"ready\": %s,\n"
            "  \"enabled\": %s,\n"
            "  \"rejectReason\": \"%s\",\n"
            "  \"dungeonId\": %s,\n"
            "  \"ruleId\": \"%s\",\n"
            "  \"ruleFile\": \"%s\",\n"
            "  \"ruleFiles\": %d,\n"
            "  \"rules\": %d,\n"
            "  \"percent\": %u,\n"
            "  \"attackPercent\": %u,\n"
            "  \"monsters\": %d,\n"
            "  \"applied\": %d,\n"
            "  \"failed\": %d,\n"
            "  \"tracked\": %d,\n"
            "  \"attackApplied\": %d,\n"
            "  \"attackFailed\": %d,\n"
            "  \"attackTracked\": %d,\n"
            "  \"restored\": %s,\n"
            "  \"lastError\": \"%s\"\n"
            "}\n",
            (unsigned long)GetCurrentProcessId(), TICK_MS, g_ready ? "true" : "false",
            config_usable() ? "true" : "false", g_ready ? "" : "clientFingerprintMismatch", dungeon,
            g_info.matched ? g_info.rule_id : "", g_info.matched ? g_info.rule_file : "",
            g_engine.set.rule_file_count, ruleset_total(&g_engine.set), (unsigned)g_info.percent,
            (unsigned)g_info.attack_percent, g_info.monsters, g_info.applied, g_info.failed,
            g_info.tracked, g_info.attack_applied, g_info.attack_failed, g_info.attack_tracked,
            g_info.restored ? "true" : "false", err);
    fclose(f);
}

/* ================================================================== */
/* 主循环                                                              */
/* ================================================================== */
static int module_is_client(void) {
    wchar_t path[MAX_PATH];
    DWORD n = GetModuleFileNameW(NULL, path, MAX_PATH);
    const wchar_t *base = path;
    DWORD i;
    if (n == 0) return 0;
    for (i = 0; i < n; i++)
        if (path[i] == L'\\' || path[i] == L'/') base = path + i + 1;
    return _wcsicmp(base, L"DFO.exe") == 0;
}

static void open_log(void) {
    char dir8[MAX_PATH];
    char logpath[MAX_PATH * 2];
    wchar_t wlogpath[MAX_PATH * 2];
    DWORD n = GetModuleFileNameW(g_self, g_dir, MAX_PATH);
    if (n == 0) return;
    {
        DWORD i;
        for (i = n; i > 0; i--) {
            if (g_dir[i - 1] == L'\\') { g_dir[i - 1] = 0; break; }
        }
    }
    w2u(g_dir, dir8, sizeof(dir8));
    _snprintf_s(logpath, sizeof(logpath), _TRUNCATE, "%s\\difficulty-rules.log", dir8);
    MultiByteToWideChar(CP_UTF8, 0, logpath, -1, wlogpath, MAX_PATH * 2);
    /* 简单轮转：超过 4 MB 就改名 .old（避免长时间跑图把日志撑爆） */
    {
        WIN32_FILE_ATTRIBUTE_DATA fad;
        if (GetFileAttributesExW(wlogpath, GetFileExInfoStandard, &fad) &&
            (((uint64_t)fad.nFileSizeHigh << 32) | fad.nFileSizeLow) > (4ULL << 20)) {
            wchar_t old[MAX_PATH * 2];
            _snwprintf(old, MAX_PATH * 2, L"%s.old", wlogpath);
            DeleteFileW(old);
            MoveFileW(wlogpath, old);
        }
    }
    g_log = _wfopen(wlogpath, L"ab");
}

static unsigned __stdcall worker(void *unused) {
    int waiting_logged = 0;
    int disabled = 0;       /* 配置不可用：已还原并停止接管 */
    int restored_once = 0;  /* 这一轮"不可用"期里已经还原过一次 */
    int rule_logged = 0;    /* 0 = 还没打过；1 = 上次"未命中"；2 = 上次"命中" */
    DWORD last_info = 0;
    (void)unused;

    /* 指纹只检一次：通过才接管；不通过就永久失效（宁失效不崩） */
    if (!verify_all()) {
        logf_("客户端原生指纹未全部通过 —— 本插件**不接管**（一个字节都不改）");
        snprintf(g_info.last_error, sizeof(g_info.last_error), "clientFingerprintMismatch");
        g_ready = 0;
        update_info();
    } else {
        logf_("客户端原生指纹校验通过（14 段逐字节 + 2 项旧钩子存在性检查）");
        g_ready = 1;
    }

    for (;;) {
        SceneInfo sc;
        TickCtx ctx;
        StatCtx sctx;
        SceneSnap snap;
        ChainDiag diag;
        FileState st;
        int changed, usable;

        Sleep(TICK_MS);
        if (!g_ready) continue;

        /* 1) 配置：rules.json 的时间戳/大小/哈希，或 rules.d 的目录指纹任一变化就重读
         *    （内容级检测，避免只改内容不改大小被漏掉） */
        rules_file_state(&st);
        changed = 0;
        if (file_state_changed(&st)) {
            changed = config_try(&st);
            if (changed) {
                logf_("配置发生变化，重新评估");
                restored_once = 0;
            }
        }
        usable = config_usable();
        if (!usable) {
            if (!disabled) {
                logf_("配置不可用（enabled=false / 解析失败 / 文件缺失）—— 还原已改过的怪并停止接管");
                disabled = 1;
                snprintf(g_info.last_error, sizeof(g_info.last_error), "configDisabled");
            }
            if (!restored_once) {
                g_info.restored = restore_all() > 0;
                restored_once = 1;
                g_last_dungeon = -1;
                rule_logged = 0;
            }
            g_info.disabled = 1;
            g_info.matched = 0;
            g_info.percent = 100;
            g_info.attack_percent = 100;
            g_info.monsters = 0;
            g_info.applied = 0;
            g_info.failed = 0;
            g_info.attack_applied = 0;
            g_info.attack_failed = 0;
            if (GetTickCount() - last_info > INFO_HEARTBEAT_MS) {
                update_info();
                last_info = GetTickCount();
            }
            continue;
        }
        if (disabled) {
            logf_("配置恢复可用 —— 重新开始接管");
            disabled = 0;
            restored_once = 0;
            g_info.restored = 0;
        }

        /* 2) 定位场景 */
        g_info.disabled = 0;
        if (!find_scene_diag(&sc, &diag)) {
            if (!waiting_logged) {
                logf_("未找到场景根（未进图或还在登录/选人）—— 保持原版");
                waiting_logged = 1;
            }
            /* 诊断：把 root / root->vtable[0xd8] / scene 的现场值和卡住的那一步打出来
             * （只在状态变化时打，不刷屏） */
            chain_diag_log(&diag, &sc, 0, 0, 0, 0, 0);
            g_info.has_dungeon = 0;
            g_info.matched = 0;
            g_info.percent = 100;
            g_info.attack_percent = 100;
            g_info.monsters = 0;
            g_info.applied = 0;
            g_info.failed = 0;
            g_info.attack_applied = 0;
            g_info.attack_failed = 0;
            if (GetTickCount() - last_info > INFO_HEARTBEAT_MS) {
                update_info();
                last_info = GetTickCount();
            }
            continue;
        }
        waiting_logged = 0;
        g_info.has_dungeon = sc.has_dungeon;
        g_info.dungeon_id = sc.dungeon_id;
        if (!sc.has_dungeon) continue;
        if ((int)sc.dungeon_id != g_last_dungeon) {
            logf_("副本 id = %u（原始值 %#x）", sc.dungeon_id, sc.dungeon_id);
            g_last_dungeon = (int)sc.dungeon_id;
        }

        /* 3) 匹配规则 → 快照 → 应用（血量 + 怪物伤害） */
        {
            const Rule *rule = match_rule(sc.dungeon_id, sc.has_dungeon);
            uint32_t percent = rule ? rule->percent : 100;
            uint32_t attack_percent = rule ? rule->attack_percent : 100;
            if (rule == NULL) {
                if (rule_logged != 1) {
                    logf_("副本 %u 未命中任何规则 —— 保持原版（100）", sc.dungeon_id);
                    rule_logged = 1;
                }
                g_info.matched = 0;
                g_info.rule_id[0] = 0;
                g_info.rule_file[0] = 0;
            } else {
                if (rule_logged != 2 || strcmp(g_info.rule_id, rule->id) != 0 ||
                    g_info.percent != percent || g_info.attack_percent != attack_percent) {
                    logf_("副本 %u 命中规则 %s（%s，来自 %s）→ 血量 %u.%02u 倍 / 伤害 %u.%02u 倍",
                          sc.dungeon_id, rule->id, rule->name[0] ? rule->name : "-",
                          g_engine.set.matched_file[0] ? g_engine.set.matched_file : "?",
                          percent / 100, percent % 100, attack_percent / 100, attack_percent % 100);
                    rule_logged = 2;
                }
                g_info.matched = 1;
                snprintf(g_info.rule_id, sizeof(g_info.rule_id), "%s", rule->id);
                snprintf(g_info.rule_file, sizeof(g_info.rule_file), "%s",
                         g_engine.set.matched_file);
            }
            g_info.percent = percent;
            g_info.attack_percent = attack_percent;

            /* 3a) 走实体链：一次快照，血量和伤害共用同一份"合格怪"名单 */
            memset(&ctx, 0, sizeof(ctx));
            memset(&sctx, 0, sizeof(sctx));
            memset(&snap, 0, sizeof(snap));
            ctx.percent = percent;
            sctx.percent = attack_percent;
            g_live_count = 0;
            walk_monsters_diag(&sc, collect_one, &snap, &diag);
            ctx.monsters = snap.count;

            /* 3b) 应用规则：血量 */
            {
                int i;
                for (i = 0; i < snap.count; i++) hp_reconcile(&snap.mons[i], percent, &ctx);
            }
            /* 3c) 应用规则：怪物伤害（物攻 0x398 + 魔攻 0x3b8 同一倍率） */
            {
                int i;
                for (i = 0; i < snap.count; i++)
                    attack_reconcile(&snap.mons[i], attack_percent, &sctx);
            }
            entries_reap();
            sctx.tracked = stats_live_count();
            g_info.monsters = snap.count;
            g_info.applied = ctx.applied;
            g_info.failed = ctx.failed;
            g_info.tracked = entries_live_count();
            g_info.attack_applied = sctx.applied;
            g_info.attack_failed = sctx.failed;
            g_info.attack_tracked = sctx.tracked;

            /* 诊断：链上发生了什么（状态变化时才打） */
            chain_diag_log(&diag, &sc, 1, sc.dungeon_id, sc.has_dungeon, snap.count, sctx.applied);
            if (snap.rejected > 0) {
                static int last_reject = -1;
                if (last_reject != snap.last_reject) {
                    last_reject = snap.last_reject;
                    logf_("血量为零/越界等被丢掉的实体 %d 只（最后一个原因：%s）", snap.rejected,
                          snap.last_reject == 1   ? "层描述块不可读"
                          : snap.last_reject == 2 ? "层 base 的 guard 非 0"
                          : snap.last_reject == 3 ? "层 base 为 0 或超过上界"
                                                  : "当前血量不可读/为 0/超过上界");
                }
            }

            if (ctx.applied > 0 || ctx.failed > 0 || sctx.applied > 0 || sctx.failed > 0) {
                logf_("本轮：合格怪 %d 只；血量写入 %d/失败 %d（跟踪 %d）；伤害写入 %d/失败 %d"
                      "（跟踪 %d）",
                      snap.count, ctx.applied, ctx.failed, entries_live_count(), sctx.applied,
                      sctx.failed, sctx.tracked);
            } else if (snap.count > 0 && (percent != 100 || attack_percent != 100)) {
                /* 有怪但一个字节都没写：把"卡在哪一步"按状态变化打出来 */
                static uint64_t last_gate_sig = 0;
                uint64_t sig = 0;
                int g;
                for (g = 0; g < GATE_MAX; g++) sig = sig * 1000003ULL + (uint64_t)ctx.gate[g];
                sig = sig * 1000003ULL + (uint64_t)sctx.failed + (uint64_t)snap.rejected;
                if (sig != last_gate_sig) {
                    last_gate_sig = sig;
                    logf_("本轮一个字节都没写：合格怪 %d 只（丢弃 %d 只）。血量闸门：", snap.count,
                          snap.rejected);
                    for (g = 0; g < GATE_MAX; g++) {
                        if (ctx.gate[g])
                            logf_("  闸门[%s] %d 次%s%s", gate_name(g), ctx.gate[g],
                                  ctx.gate_first[g] ? "：首个原因 " : "",
                                  ctx.gate_first[g] ? ctx.gate_first[g] : "");
                    }
                    if (sctx.failed)
                        logf_("  伤害路径失败 %d 次（多为原生整数 guard 校验不符，日志上有单条说明）",
                              sctx.failed);
                }
            }
            if (ctx.failed > 0 || sctx.failed > 0) {
                snprintf(g_info.last_error, sizeof(g_info.last_error),
                         "%d hp write failures, %d attack write failures", ctx.failed, sctx.failed);
            } else {
                g_info.last_error[0] = 0;
            }
        }
        if (GetTickCount() - last_info > INFO_HEARTBEAT_MS) {
            update_info();
            last_info = GetTickCount();
        }
    }
    return 0;
}

/* ================================================================== */
/* 入口                                                                */
/* ================================================================== */
extern __declspec(dllexport) DWORD WINAPI ModStart(void);

__declspec(dllexport) DWORD WINAPI ModStart(void) {
    uintptr_t base;
    InitializeCriticalSection(&g_lock);
    g_t0 = GetTickCount();

    base = (uintptr_t)GetModuleHandleW(NULL);
    g_base = (uint64_t)base;
    open_log();
    logf_("==== DifficultyRules（副本难度 · 怪物血量倍率）启动 ====");
    logf_("pid=%lu 本 DLL 目录=%ls", (unsigned long)GetCurrentProcessId(), g_dir);
    logf_("客户端基址 = %#llx（期望 %#llx；本机 DFO.exe 的 PE ImageBase 实测值）",
          (unsigned long long)g_base, (unsigned long long)EXPECTED_IMAGE_BASE);

    if (!module_is_client()) {
        logf_("当前进程不是 DFO.exe —— 只加载，不接管");
        return 0;
    }
    if (g_base != EXPECTED_IMAGE_BASE) {
        logf_("客户端镜像基址与取证不符（期望 %#llx，实测 %#llx）—— **拒绝接管**，"
              "所有写死的 RVA 都不可信",
              (unsigned long long)EXPECTED_IMAGE_BASE, (unsigned long long)g_base);
        snprintf(g_info.last_error, sizeof(g_info.last_error), "imageBaseMismatch");
        update_info();
        return 0;
    }
    {
        HANDLE t = (HANDLE)_beginthreadex(NULL, 0, worker, NULL, 0, NULL);
        if (t) CloseHandle(t);
    }
    return 0;
}

__declspec(dllexport) const char *WINAPI ModName(void) { return "difficulty-rules"; }

BOOL WINAPI DllMain(HINSTANCE inst, DWORD reason, LPVOID reserved) {
    (void)reserved;
    if (reason == DLL_PROCESS_ATTACH) {
        g_self = inst;
        DisableThreadLibraryCalls(inst);
    }
    return TRUE;
}

/* ================================================================== */
/* TODO（本版**刻意不做**，不要顺手开）                                 */
/*   本版只启用血量 + 怪物伤害（物攻 0x398 / 魔攻 0x3b8）两项。
 *   monster_movement.go:50-53 白名单里的其余偏移仍然不启用：
 *     0x2e0 移速 / 0x300 攻速 / 0x348 硬直恢复 / 0x3a8 物防 / 0x3c8 魔防
 *   以及 monster_recovery.go（硬直）、monster_sight.go（视野）、
 *   monster_warlike.go（好战）、monster_cooldown.go（技能冷却）、
 *   monster_attack_wait.go（攻击等待）、monster_resistance.go（命中抗性）、
 *   monster_plainstat.go（明文属性）。
 *   它们的写入路径与血量不同（原生整数是 stored+guard 对、明文偏移等），
 *   未在本版做逐字节证据复核，一律不启用。
 *
 *   如果要加下一项：把偏移加进上面的常量、确认它确实在
 *   monster_movement.go:50-53 白名单里、然后用 stat_reconcile 那一套
 *   （读 → decode_native_int 校 guard → 算 → 写 → 读回 → 回滚）即可，
 *   不要另起一条写入路径。
 * ================================================================== */
