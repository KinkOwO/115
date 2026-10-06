/*
 * AutoConfirmDelete.dll —— 去掉「删角色要手打确认短语」这一步
 *
 * 静态取证（analysis/tasks/chinese-input-probe-20261006.md §6）：
 *   删角色通知窗 CharacterDeleteNotiWindow（UI/CharacterDeleteNoti/Character_Delete.xui）
 *   里有一个编辑框 edit_delete（窗口成员 +0x690）。点"确定"时客户端跑
 *   RVA 0x11F7C90（本窗类虚表槽，vtable 0x140965A278 指向它）：
 *
 *     ctrl = *(void**)(window + 0x690);
 *     typed = ctrl->vtable[0x2A8/8](ctrl);          // 取编辑框文本
 *     phrase = *(0x14723C170)(0x5F73224);           // 取本地化确认短语
 *     if (typed != phrase) 不发送;
 *     ... writer(): opcode 6 + u16 slot(window+0x978) + wstring name(window+0x958) → 发送
 *
 *   ★ 要打的是**固定短语**（DSTR id 100086308 = "delete character"，汉化后是中文），
 *     **不是角色名**；发出去的名字取自窗口成员，跟输入框无关。
 *   ★ 所以只要让"取编辑框文本"这一步返回那句短语，校验就会通过 —— 不需要打字，
 *     也不会发错名字（客户端本来就用自己存的名字）。
 *
 * 本插件就做这一件事：
 *   1. inline 钩住窗口 init（RVA 0x11F3EC0）→ 拿到 window 与它的 edit 控件；
 *   2. 把**该控件实例所属 vtable 的 +0x2A8 槽**换成包装：只对这一个控件实例返回
 *      运行时取到的确认短语（走客户端自己的 0x14723C170），其它实例原样转发。
 *
 * 不改 EXE、不改报文、不自己发包；卸载即删除本 DLL。
 * 日志写本 DLL 自己所在目录（AGENTS §0 第 9 条）。
 */

#define WIN32_LEAN_AND_MEAN
#define _CRT_SECURE_NO_WARNINGS
#include <windows.h>
#include <stdio.h>
#include <stdarg.h>
#include <stdlib.h>
#include <string.h>

/* ---- 客户端地址（RVA）与偏移 ---- */
#define RVA_INIT 0x11F3EC0   /* CharacterDeleteNotiWindow 的初始化（绑定 edit_delete） */
#define RVA_CONFIRM 0x11F7C90 /* 校验 + 发送（本插件不调用，仅用于日志） */
#define RVA_UITEXT 0x723C170  /* 本地化 UI 文本查找：const wchar_t*(unsigned id) */
#define EXPECT_ID 0x5F73224u  /* DSTR 100086308 = "delete character" */
#define OFF_WINDOW_EDIT 0x690 /* 窗口里的编辑框成员 */
#define OFF_VT_GETTEXT 0x2A8  /* 控件虚表里"取文本"的槽 */

static HMODULE g_self;
static HMODULE g_exe;
static wchar_t g_dir[MAX_PATH];
static FILE *g_log;
static CRITICAL_SECTION g_lock;
static DWORD g_t0;
static int g_enable = 1;      /* auto_confirm_delete：0 关、1 打补丁（默认） */
static unsigned g_phraseId = EXPECT_ID;

static void w2u(const wchar_t *src, char *dst, size_t cap) {
    if (!src) { dst[0] = 0; return; }
    int n = WideCharToMultiByte(CP_UTF8, 0, src, -1, dst, (int)cap - 1, NULL, NULL);
    dst[n > 0 ? n : 0] = 0;
}

static void logf_(const char *fmt, ...) {
    char buf[2048];
    va_list ap;
    if (!g_log) return;
    va_start(ap, fmt);
    _vsnprintf_s(buf, sizeof(buf), _TRUNCATE, fmt, ap);
    va_end(ap);
    EnterCriticalSection(&g_lock);
    fprintf(g_log, "[+%6lums][t%5lu] %s\n", (unsigned long)(GetTickCount() - g_t0),
            (unsigned long)GetCurrentThreadId(), buf);
    fflush(g_log);
    LeaveCriticalSection(&g_lock);
}

/* ------------------------------------------------------------------ */
/* 近地址分配 + inline 跳转（与中文输入插件同一套，已被独立自测验证过）      */
/* ------------------------------------------------------------------ */
static unsigned char *alloc_near(HMODULE mod, SIZE_T size) {
    SYSTEM_INFO si;
    GetSystemInfo(&si);
    unsigned char *base = (unsigned char *)mod;
    ULONG_PTR gran = si.dwAllocationGranularity ? si.dwAllocationGranularity : 0x10000;
    for (ULONG_PTR delta = gran; delta < 0x70000000ULL; delta += gran) {
        unsigned char *cand = base + delta;
        MEMORY_BASIC_INFORMATION mbi;
        if (VirtualQuery(cand, &mbi, sizeof(mbi)) != sizeof(mbi)) break;
        if (mbi.State != MEM_FREE) continue;
        unsigned char *p =
            (unsigned char *)VirtualAlloc(cand, size, MEM_COMMIT | MEM_RESERVE,
                                          PAGE_EXECUTE_READWRITE);
        if (!p) continue;
        ULONG_PTR off = (ULONG_PTR)p - (ULONG_PTR)base;
        if (off > 0 && off < 0x7FFF0000ULL) return p;
        VirtualFree(p, 0, MEM_RELEASE);
    }
    return NULL;
}

static void write_rel_jmp(unsigned char *at, const void *to) {
    at[0] = 0xE9;
    *(int *)(at + 1) = (int)((const unsigned char *)to - (at + 5));
}

static void write_abs_jmp(unsigned char *at, const void *to) {
    at[0] = 0xFF;
    at[1] = 0x25;
    *(DWORD *)(at + 2) = 0;
    *(void **)(at + 6) = (void *)to;
}

static int range_readable(const void *addr, size_t len) {
    MEMORY_BASIC_INFORMATION mbi;
    if (VirtualQuery(addr, &mbi, sizeof(mbi)) != sizeof(mbi)) return 0;
    if (mbi.State != MEM_COMMIT) return 0;
    if (mbi.Protect & (PAGE_NOACCESS | PAGE_GUARD)) return 0;
    return (const unsigned char *)addr + len <=
           (const unsigned char *)mbi.BaseAddress + mbi.RegionSize;
}

static void *install_inline(const char *name, unsigned char *p, const unsigned char *expect,
                            size_t patchLen, void *hook) {
    if (!range_readable(p, patchLen)) {
        logf_("inline 钩子[%s] 跳过：%p 不在已提交内存里（客户端版本不同？）", name, p);
        return NULL;
    }
    if (memcmp(p, expect, patchLen) != 0) {
        logf_("inline 钩子[%s] 跳过：期望字节对不上", name);
        return NULL;
    }
    unsigned char *stub = alloc_near(g_exe, 0x1000);
    if (!stub) {
        logf_("inline 钩子[%s] 跳过：模块附近没有可分配内存", name);
        return NULL;
    }
    LONG_PTR dist = (LONG_PTR)(stub + 0x40) - (LONG_PTR)p;
    if (dist > 0x7FF00000LL || dist < -0x7FF00000LL) {
        logf_("inline 钩子[%s] 跳过：桥离目标 %lld 字节，超出 rel32 范围", name, (long long)dist);
        return NULL;
    }
    memcpy(stub, p, patchLen);
    write_rel_jmp(stub + patchLen, p + patchLen);
    write_abs_jmp(stub + 0x40, hook);

    DWORD old = 0;
    if (!VirtualProtect(p, patchLen, PAGE_EXECUTE_READWRITE, &old)) {
        logf_("inline 钩子[%s] 跳过：无法改页面保护", name);
        return NULL;
    }
    write_rel_jmp(p, stub + 0x40);
    for (size_t i = 5; i < patchLen; i++) p[i] = 0x90;
    DWORD tmp = 0;
    VirtualProtect(p, patchLen, old, &tmp);
    FlushInstructionCache(GetCurrentProcess(), p, patchLen);
    logf_("inline 钩子[%s] 已装：%p（覆盖 %zu 字节，跳板 %p）", name, p, patchLen, stub);
    return stub;
}

/* ------------------------------------------------------------------ */
/* 要打的两处                                                          */
/* ------------------------------------------------------------------ */
typedef void *(WINAPI *GetTextFn)(void *control);      /* 控件取文本（宽串） */
typedef const wchar_t *(WINAPI *UiTextFn)(unsigned int); /* 本地化文本查找 */

static GetTextFn g_origGetText;
static void *g_control;         /* 我们盯住的那个编辑框实例 */
static const wchar_t *g_phrase; /* 运行时取到的确认短语（客户端自己给的，汉化后是中文） */

static const wchar_t *WINAPI h_getText(void *control) {
    if (g_control && control == g_control && g_phrase) {
        static LONG once;
        if (InterlockedIncrement(&once) == 1) {
            char u8[256];
            w2u(g_phrase, u8, sizeof(u8));
            logf_("★ 已把编辑框文本替换为确认短语 \"%s\"（玩家不需要打字）", u8);
        }
        return g_phrase;
    }
    return g_origGetText(control);
}

static void patch_control(void *control) {
    if (!control || !range_readable(control, sizeof(void *))) return;
    void **vt = *(void ***)control;
    if (!vt || !range_readable(vt, OFF_VT_GETTEXT + sizeof(void *))) {
        logf_("控件 %p 的虚表读不到，放弃", control);
        return;
    }
    void *cur = vt[OFF_VT_GETTEXT / sizeof(void *)];
    if (cur == (void *)h_getText) return; /* 已经打过 */
    g_control = control;
    g_origGetText = (GetTextFn)cur;
    DWORD old = 0;
    if (!VirtualProtect(&vt[OFF_VT_GETTEXT / sizeof(void *)], sizeof(void *), PAGE_READWRITE,
                        &old)) {
        logf_("控件虚表不可写，放弃：%p", (void *)&vt[OFF_VT_GETTEXT / sizeof(void *)]);
        return;
    }
    vt[OFF_VT_GETTEXT / sizeof(void *)] = (void *)h_getText;
    DWORD tmp = 0;
    VirtualProtect(&vt[OFF_VT_GETTEXT / sizeof(void *)], sizeof(void *), old, &tmp);
    logf_("★ 控件 %p 的取文本槽（vtable+%#x）已换成包装：原 %p → 包装 %p", control,
          OFF_VT_GETTEXT, cur, (void *)h_getText);
}

/* 窗口 init 钩子：进入时 rcx = window；返回后 [window+0x690] 已绑定编辑框 */
static void *(WINAPI *g_origInit)(void *self);

static void *WINAPI h_init(void *self) {
    void *ret = g_origInit(self);
    if (g_enable && self && range_readable((char *)self + OFF_WINDOW_EDIT, sizeof(void *))) {
        void *ctrl = *(void **)((char *)self + OFF_WINDOW_EDIT);
        if (ctrl) {
            if (!g_phrase) {
                UiTextFn lookup = (UiTextFn)((unsigned char *)g_exe + RVA_UITEXT);
                g_phrase = lookup(g_phraseId);
                char u8[256] = "";
                if (g_phrase) w2u(g_phrase, u8, sizeof(u8));
                logf_("确认短语（id %#x）= \"%s\"", g_phraseId, u8);
            }
            patch_control(ctrl);
        } else {
            logf_("窗口 %p 的 +%#x 还没有编辑框（init 时机不对？）", self, OFF_WINDOW_EDIT);
        }
    }
    return ret;
}

/* ------------------------------------------------------------------ */
/* 配置（缺失时自生成）                                                 */
/* ------------------------------------------------------------------ */
static const char *kDefaultIni =
    "# auto-confirm.ini —— 自动确认插件开关（缺失时由插件生成，只生成一次）\n"
    "#\n"
    "# 背景：删角色要点\"确定\"前必须先手打一句确认短语（DSTR 100086308 =\n"
    "# \"delete character\"，汉化后是中文）。名字是客户端自己存的，跟输入框无关，\n"
    "# 所以把这个校验喂过去即可，不需要打字。\n"
    "#\n"
    "# 0 = 关（什么都不做）\n"
    "# 1 = 打补丁：让编辑框校验直接通过（默认；仍需点一次\"确定\"）\n"
    "auto_confirm_delete=1\n"
    "\n"
    "# 确认短语的 DSTR id（换客户端版本时可能要改）\n"
    "phrase_id=5F73224\n";

static int ini_int(const char *text, const char *key, int dflt) {
    const char *p = text;
    size_t klen = strlen(key);
    while (p && *p) {
        const char *eol = strchr(p, '\n');
        size_t len = eol ? (size_t)(eol - p) : strlen(p);
        if (len > klen && _strnicmp(p, key, klen) == 0 && p[klen] == '=') return atoi(p + klen + 1);
        p = eol ? eol + 1 : NULL;
    }
    return dflt;
}

static unsigned ini_hex(const char *text, const char *key, unsigned dflt) {
    const char *p = text;
    size_t klen = strlen(key);
    while (p && *p) {
        const char *eol = strchr(p, '\n');
        size_t len = eol ? (size_t)(eol - p) : strlen(p);
        if (len > klen && _strnicmp(p, key, klen) == 0 && p[klen] == '=') {
            const char *v = p + klen + 1;
            while (*v == ' ' || *v == '\t') v++;
            char *end = NULL;
            unsigned long value = strtoul(v, &end, 16);
            if (end == v || value == 0) return dflt;
            return (unsigned)value;
        }
        p = eol ? eol + 1 : NULL;
    }
    return dflt;
}

static void load_config(void) {
    char path[MAX_PATH * 2], dir8[MAX_PATH * 2];
    w2u(g_dir, dir8, sizeof(dir8));
    _snprintf_s(path, sizeof(path), _TRUNCATE, "%s\\auto-confirm.ini", dir8);
    FILE *f = fopen(path, "rb");
    if (!f) {
        FILE *w = fopen(path, "wb");
        if (w) { fputs(kDefaultIni, w); fclose(w); }
        logf_("没有 %s → 已生成默认开关（auto_confirm_delete=%d）", path, g_enable);
        return;
    }
    char text[4096];
    size_t n = fread(text, 1, sizeof(text) - 1, f);
    text[n] = 0;
    fclose(f);
    g_enable = ini_int(text, "auto_confirm_delete", 1);
    g_phraseId = ini_hex(text, "phrase_id", EXPECT_ID);
    logf_("配置 %s：auto_confirm_delete=%d phrase_id=%#x", path, g_enable, g_phraseId);
}

/* ------------------------------------------------------------------ */
static DWORD WINAPI worker(LPVOID unused) {
    (void)unused;
    char path[MAX_PATH * 2], dir8[MAX_PATH * 2];
    w2u(g_dir, dir8, sizeof(dir8));
    _snprintf_s(path, sizeof(path), _TRUNCATE, "%s\\auto-confirm.log", dir8);
    g_log = fopen(path, "ab");
    if (!g_log) return 1;

    logf_("==== 自动确认插件启动 ====");
    logf_("pid=%lu 目录=%s", (unsigned long)GetCurrentProcessId(), dir8);
    load_config();
    if (!g_enable) {
        logf_("auto_confirm_delete=0 → 什么都不做");
        return 0;
    }

    static const unsigned char expectInit[] = {0x48, 0x8B, 0xC4, 0x55, 0x57};
    g_origInit = (void *(WINAPI *)(void *))install_inline("delNotiInit",
                                                          (unsigned char *)g_exe + RVA_INIT,
                                                          expectInit, sizeof(expectInit),
                                                          (void *)h_init);
    if (!g_origInit) logf_("初始化钩子没装上：本插件不会生效（客户端版本不同？）");
    logf_("（发确定包的是 RVA %#x；本插件不调用它，只让它的校验通过）", RVA_CONFIRM);
    return 0;
}

extern __declspec(dllexport) DWORD WINAPI StartLocalization(void);
extern __declspec(dllexport) DWORD WINAPI ModStart(void);
extern __declspec(dllexport) const char *WINAPI ModName(void);

__declspec(dllexport) DWORD WINAPI StartLocalization(void) {
    InitializeCriticalSection(&g_lock);
    g_t0 = GetTickCount();
    g_exe = GetModuleHandleW(NULL);
    wchar_t path[MAX_PATH];
    DWORD n = GetModuleFileNameW(g_self, path, MAX_PATH);
    if (n) {
        wcscpy_s(g_dir, MAX_PATH, path);
        for (DWORD i = n; i > 0; i--)
            if (g_dir[i - 1] == L'\\') { g_dir[i - 1] = 0; break; }
    }
    HANDLE t = CreateThread(NULL, 0, worker, NULL, 0, NULL);
    if (t) CloseHandle(t);
    return 0;
}

__declspec(dllexport) DWORD WINAPI ModStart(void) { return StartLocalization(); }
__declspec(dllexport) const char *WINAPI ModName(void) { return "auto-confirm-delete"; }

BOOL WINAPI DllMain(HINSTANCE inst, DWORD reason, LPVOID reserved) {
    (void)reserved;
    if (reason == DLL_PROCESS_ATTACH) {
        g_self = inst;
        DisableThreadLibraryCalls(inst);
    }
    return TRUE;
}
