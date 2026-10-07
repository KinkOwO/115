/*
 * ChineseLocalization.dll —— 中文输入探针（只观察，不改客户端行为）
 *
 * 契约（逆向自客户端自带的 dinput8.dll 代理 "DFO localization proxy v2"）：
 *   1. 代理在 DllMain 里起一个线程，取自己的模块路径，把文件名部分换成
 *      L"ChineseLocalization.dll"，即 <客户端目录>\ChineseLocalization.dll；
 *   2. LoadLibraryExW(该绝对路径, NULL, 0x900)；
 *   3. GetProcAddress(h, "StartLocalization") 并用**无参数**调用它，
 *      返回值即代理线程的退出码（1..5 是代理自己的失败码）。
 *   因此本文件必须导出无参数、未修饰的 StartLocalization。
 *
 * 本探针只做三件事：
 *   A. 记录进程/窗口/键盘布局/IME 环境；
 *   B. 用 EAT 钩子记录客户端对 imm32 / user32 的调用（包装后原样转发）；
 *   C. 子类化本进程的窗口，记录按键、WM_CHAR 与 WM_IME_ 系列消息。
 * 它不返回值、不改报文、不改窗口过程语义（原样 CallWindowProcW 转发）。
 * 日志只写本 DLL 自己所在目录（AGENTS §0 第 9 条）。
 */

#define WIN32_LEAN_AND_MEAN
#define _CRT_SECURE_NO_WARNINGS
#include <windows.h>
#include <stdio.h>
#include <stdarg.h>
#include <stdlib.h>
#include <string.h>
#include <intrin.h>

/* ------------------------------------------------------------------ */
/* IMM32 最小声明（不依赖 imm.h，避免 SDK 版本差异）                    */
/* ------------------------------------------------------------------ */
typedef void *HIMC_;
typedef void *HIMCC_;

#define GCS_COMPREADSTR 0x0001
#define GCS_COMPSTR 0x0008
#define GCS_CURSORPOS 0x0080
#define GCS_RESULTSTR 0x0800

#ifndef CS_IME
#define CS_IME 0x00010000
#endif

#ifndef WM_IME_STARTCOMPOSITION
#define WM_IME_STARTCOMPOSITION 0x010D
#define WM_IME_ENDCOMPOSITION 0x010E
#define WM_IME_COMPOSITION 0x010F
#endif
#ifndef WM_IME_SETCONTEXT
#define WM_IME_SETCONTEXT 0x0281
#define WM_IME_NOTIFY 0x0282
#define WM_IME_CONTROL 0x0283
#define WM_IME_COMPOSITIONFULL 0x0284
#define WM_IME_SELECT 0x0285
#define WM_IME_CHAR 0x0286
#define WM_IME_REQUEST 0x0288
#define WM_IME_KEYDOWN 0x0290
#define WM_IME_KEYUP 0x0291
#endif

typedef HIMC_ (WINAPI *PFN_ImmGetContext)(HWND);
typedef BOOL(WINAPI *PFN_ImmReleaseContext)(HWND, HIMC_);
typedef HIMC_(WINAPI *PFN_ImmAssociateContext)(HWND, HIMC_);
typedef HIMC_(WINAPI *PFN_ImmCreateContext)(void);
typedef BOOL(WINAPI *PFN_ImmDestroyContext)(HIMC_);
typedef LONG(WINAPI *PFN_ImmGetCompositionStringW)(HIMC_, DWORD, LPVOID, DWORD);
typedef BOOL(WINAPI *PFN_ImmSetCompositionStringW)(HIMC_, DWORD, LPCVOID, DWORD, LPCVOID, DWORD);
typedef BOOL(WINAPI *PFN_ImmSetCompositionWindow)(HIMC_, const void *);
typedef BOOL(WINAPI *PFN_ImmSetCandidateWindow)(HIMC_, const void *);
typedef BOOL(WINAPI *PFN_ImmNotifyIME)(HIMC_, DWORD, DWORD, DWORD);
typedef BOOL(WINAPI *PFN_ImmGetOpenStatus)(HIMC_);
typedef BOOL(WINAPI *PFN_ImmSetOpenStatus)(HIMC_, BOOL);
typedef BOOL(WINAPI *PFN_ImmGetConversionStatus)(HIMC_, LPDWORD, LPDWORD);
typedef BOOL(WINAPI *PFN_ImmSetConversionStatus)(HIMC_, DWORD, DWORD);
typedef LPVOID(WINAPI *PFN_ImmLockIMC)(HIMC_);
typedef BOOL(WINAPI *PFN_ImmUnlockIMC)(HIMC_);
typedef LPVOID(WINAPI *PFN_ImmLockIMCC)(HIMCC_);
typedef BOOL(WINAPI *PFN_ImmUnlockIMCC)(HIMCC_);
typedef HWND(WINAPI *PFN_ImmGetDefaultIMEWnd)(HWND);
typedef BOOL(WINAPI *PFN_ImmIsIME)(HKL);
typedef UINT(WINAPI *PFN_ImmGetDescriptionW)(HKL, LPWSTR, UINT);
typedef UINT(WINAPI *PFN_ImmGetIMEFileNameW)(HKL, LPWSTR, UINT);
typedef BOOL(WINAPI *PFN_ImmAssociateContextEx)(HWND, HIMC_, DWORD);
/* ------------------------------------------------------------------ */
/* 状态                                                                */
/* ------------------------------------------------------------------ */
static HMODULE g_self;
static wchar_t g_dir[MAX_PATH];
static FILE *g_log;
static CRITICAL_SECTION g_lock;
static DWORD g_t0;
static LONG g_lines;

static PFN_ImmGetContext pImmGetContext;
static PFN_ImmReleaseContext pImmReleaseContext;
static PFN_ImmGetCompositionStringW pImmGetCompositionStringW;
static PFN_ImmGetOpenStatus pImmGetOpenStatus;
static PFN_ImmGetDescriptionW pImmGetDescriptionW;
static PFN_ImmIsIME pImmIsIME;
static PFN_ImmGetIMEFileNameW pImmGetIMEFileNameW;

/* ------------------------------------------------------------------ */
/* 实验性修复开关（chinese-input.ini，缺省全 0 = 只观察）                */
/*                                                                     */
/* 静态取证（见 analysis/tasks）：客户端的 IME 子系统只在                  */
/*   [IME管理器+0x68] ∈ { E0080404, E0090404, E00E0804 }               */
/* 且 ImmGetIMEFileNameW 返回的名字 ∈ { TINTLGNT.IME, CINTLGNT.IME,     */
/*   MSTCIPHA.IME, PINTLGNT.IME, MSSCIPYA.IME } 时才启用。              */
/* 本机布局是 0x08040804、ImmGetIMEFileNameW 返回 0 ⇒ 永远进不去。        */
/* 下面两个钩子就是把这个门禁"喂"过去。                                   */
/* ------------------------------------------------------------------ */
static int g_fixLayout = 1;        /* GetKeyboardLayout 对中文布局改报传统 IME HKL（编译期默认开） */
static int g_fixImeName = 1;       /* ImmGetIMEFileNameW 改报白名单里的名字 */
static int g_postLayout = 1;       /* 给顶层窗口补一条 WM_INPUTLANGCHANGE（覆盖另一条取 HKL 的路径） */
static int g_fixSlots = 1;         /* 修正 Themida 导入槽：客户端实际是从那些槽调 API 的 */
static int g_fixGate = 1;          /* 钩住客户端门禁本体（RVA 0x6F22400），强制喂传统 HKL */
/* 下面两个是**备用手段**，默认关：只在日志证明"客户端自己不处理组字"时才打开 */
static int g_fixCancel = 1;        /* 吞掉客户端主动取消组字（ImmNotifyIME/CPS_CANCEL）——实机验证必需 */
static int g_fixIsIme = 1;           /* ImmIsIME 对假 HKL 改报「是输入法」（实机 16:32 证据要求） */
static int g_fixCharPos = 1;        /* v2.0.3：自己回答 IME 的位置询问（WM_IME_REQUEST） */
/* 客户端通过 ImmSetCompositionWindow 告诉输入法的组字位置（客户区坐标），
 * 用来回答 IME 的 IMR_QUERYCHARPOSITION / IMR_CANDIDATEWINDOW。 */
static int g_compPointValid = 0;
static POINT g_compPoint;
static int g_imeBridge = 1;        /* 兜底：客户端若不接收上屏文字，我们投 WM_CHAR（实机验证时它没触发，客户端自己处理了） */
static volatile LONG g_clientReadResult; /* 客户端本轮组字里有没有自己来读 GCS_RESULTSTR */
static wchar_t g_imeName[64] = L"CINTLGNT.IME";
static DWORD g_layoutZhCN = 0xE00E0804;
static DWORD g_layoutZhTW = 0xE0080404;



/* v2.0.3：IME 位置询问用到的结构/常量（布局与 SDK 一致，自己定义避免依赖 imm.h 的包含顺序）。 */
#ifndef IMR_CANDIDATEWINDOW
#define IMR_CANDIDATEWINDOW 0x0002
#endif
#ifndef IMR_QUERYCHARPOSITION
#define IMR_QUERYCHARPOSITION 0x0006
#endif
#ifndef CFS_RECT
#define CFS_RECT 0x0001
#endif
#ifndef CFS_POINT
#define CFS_POINT 0x0002
#endif
#ifndef CFS_CANDIDATEPOS
#define CFS_CANDIDATEPOS 0x0008
#endif
typedef struct {
    DWORD dwStyle;
    POINT ptCurrentPos;
    RECT  rcArea;
} probe_compositionform;
typedef struct {
    DWORD dwIndex;
    DWORD dwStyle;
    POINT ptCurrentPos;
    RECT  rcArea;
} probe_candidateform;
typedef struct {
    DWORD dwSize;
    DWORD dwCharPos;
    POINT pt;
    UINT  cLineHeight;
    RECT  rcDocument;
} probe_imecharposition;

/* 我们报给客户端的三个传统 IME 布局（门禁取证见 analysis/tasks/chinese-input-probe-20261006.md） */
static const DWORD kFakeHkl[3] = {0xE0080404, 0xE0090404, 0xE00E0804};

static const wchar_t *kImeWhiteList[] = {L"TINTLGNT.IME", L"CINTLGNT.IME", L"MSTCIPHA.IME",
                                         L"PINTLGNT.IME", L"MSSCIPYA.IME"};

static int name_is_whitelisted(const wchar_t *name) {
    if (!name || !name[0]) return 0;
    for (int i = 0; i < (int)(sizeof(kImeWhiteList) / sizeof(kImeWhiteList[0])); i++)
        if (_wcsicmp(name, kImeWhiteList[i]) == 0) return 1;
    return 0;
}

/* 只对**游戏主模块**说谎：MSCTF（TSF 输入法框架）在同一个进程里也走 imm32 导出，
 * 把它的 IME 文件名/键盘布局换掉会真的把输入法搞坏。 */
static HMODULE g_mainModule;
static int caller_is_main(void *ra) {
    HMODULE m = NULL;
    if (!GetModuleHandleExW(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS |
                                GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT,
                            (LPCWSTR)ra, &m))
        return 0;
    return m != NULL && m == g_mainModule;
}

#define MAX_WIN 512
typedef struct {
    HWND hwnd;
    WNDPROC orig;
    char cls[64];
} WinSlot;
static WinSlot g_wins[MAX_WIN];
static int g_winCount;

/* ------------------------------------------------------------------ */
/* 日志                                                                */
/* ------------------------------------------------------------------ */
static void log_line(const char *text) {
    if (!g_log) return;
    EnterCriticalSection(&g_lock);
    if (g_lines < 200000) {
        fprintf(g_log, "[+%6lums][t%5lu] %s\n", (unsigned long)(GetTickCount() - g_t0),
                (unsigned long)GetCurrentThreadId(), text);
        fflush(g_log);
        g_lines++;
    }
    LeaveCriticalSection(&g_lock);
}

static void logf_(const char *fmt, ...) {
    char buf[2048];
    va_list ap;
    va_start(ap, fmt);
    _vsnprintf_s(buf, sizeof(buf), _TRUNCATE, fmt, ap);
    va_end(ap);
    log_line(buf);
}

/* UTF-16 → UTF-8（日志按字节写，避免 CRT 的 locale 依赖） */
static void w2u(const wchar_t *src, char *dst, size_t cap) {
    if (!src) { dst[0] = 0; return; }
    int n = WideCharToMultiByte(CP_UTF8, 0, src, -1, dst, (int)cap - 1, NULL, NULL);
    if (n <= 0) { dst[0] = 0; return; }
    dst[n] = 0;
}

/* 地址归属：返回 "模块名+0x偏移" */
static void mod_off(void *addr, char *out, size_t cap) {
    HMODULE m = NULL;
    if (GetModuleHandleExW(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS |
                               GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT,
                           (LPCWSTR)addr, &m) &&
        m) {
        wchar_t path[MAX_PATH];
        DWORD n = GetModuleFileNameW(m, path, MAX_PATH);
        const wchar_t *base = path;
        for (DWORD i = 0; i < n; i++)
            if (path[i] == L'\\') base = path + i + 1;
        char nb[MAX_PATH];
        w2u(base, nb, sizeof(nb));
        _snprintf_s(out, cap, _TRUNCATE, "%s+0x%llx", nb,
                    (unsigned long long)((char *)addr - (char *)m));
        return;
    }
    _snprintf_s(out, cap, _TRUNCATE, "%p", addr);
}

static void log_backtrace(const char *tag) {
    void *frames[10];
    USHORT n = CaptureStackBackTrace(1, 10, frames, NULL);
    for (USHORT i = 0; i < n; i++) {
        char where[300];
        mod_off(frames[i], where, sizeof(where));
        logf_("    ^%s #%u %s", tag, (unsigned)i, where);
    }
}

/* ------------------------------------------------------------------ */
/* EAT（导出地址表）钩子：任何用 GetProcAddress 取函数的人都会拿到包装   */
/* ------------------------------------------------------------------ */
typedef struct {
    const char *name;
    void *hook;
    void *orig;
} HookSpec;

/*
 * 导出地址表（EAT）里存的是**32 位 RVA**，GetProcAddress 做的是 base+RVA。
 * 所以不能把 64 位函数指针直接写进去（会写坏相邻表项，而且 GetProcAddress
 * 算出来的地址是错的）。正确做法：在目标模块 ±2GB 内分配一小块可执行内存，
 * 放一段 14 字节的绝对跳转桩（FF 25 + 8 字节地址），再把 EAT 的 RVA 指向桩。
 */
static BYTE *alloc_near(HMODULE mod, SIZE_T size) {
    SYSTEM_INFO si;
    GetSystemInfo(&si);
    BYTE *base = (BYTE *)mod;
    ULONG_PTR gran = si.dwAllocationGranularity ? si.dwAllocationGranularity : 0x10000;
    for (ULONG_PTR delta = gran; delta < 0x70000000ULL; delta += gran) {
        BYTE *cand = base + delta;
        MEMORY_BASIC_INFORMATION mbi;
        if (VirtualQuery(cand, &mbi, sizeof(mbi)) != sizeof(mbi)) break;
        if (mbi.State != MEM_FREE) continue;
        BYTE *p = (BYTE *)VirtualAlloc(cand, size, MEM_COMMIT | MEM_RESERVE,
                                       PAGE_EXECUTE_READWRITE);
        if (!p) continue;
        ULONG_PTR off = (ULONG_PTR)p - (ULONG_PTR)base;
        if (off > 0 && off < 0x7FFF0000ULL) return p;
        VirtualFree(p, 0, MEM_RELEASE);
    }
    return NULL;
}

static void write_stub(BYTE *at, void *target) {
    at[0] = 0xFF;
    at[1] = 0x25;
    *(DWORD *)(at + 2) = 0; /* jmp qword ptr [rip+0] */
    *(void **)(at + 6) = target;
}

static int hook_exports(HMODULE mod, HookSpec *specs, int count, const char *tag) {
    if (!mod) return 0;
    BYTE *base = (BYTE *)mod;
    IMAGE_DOS_HEADER *dos = (IMAGE_DOS_HEADER *)base;
    if (dos->e_magic != IMAGE_DOS_SIGNATURE) return 0;
    IMAGE_NT_HEADERS *nt = (IMAGE_NT_HEADERS *)(base + dos->e_lfanew);
    if (nt->Signature != IMAGE_NT_SIGNATURE) return 0;
    IMAGE_DATA_DIRECTORY *ed = &nt->OptionalHeader.DataDirectory[IMAGE_DIRECTORY_ENTRY_EXPORT];
    if (!ed->VirtualAddress) return 0;
    IMAGE_EXPORT_DIRECTORY *dir = (IMAGE_EXPORT_DIRECTORY *)(base + ed->VirtualAddress);
    DWORD *names = (DWORD *)(base + dir->AddressOfNames);
    WORD *ords = (WORD *)(base + dir->AddressOfNameOrdinals);
    DWORD *funcs = (DWORD *)(base + dir->AddressOfFunctions);

    BYTE *stubs = NULL;
    SIZE_T stub_used = 0;
    int done = 0;
    for (DWORD i = 0; i < dir->NumberOfNames; i++) {
        const char *nm = (const char *)(base + names[i]);
        for (int k = 0; k < count; k++) {
            if (specs[k].orig || _stricmp(nm, specs[k].name) != 0) continue;
            DWORD rva = funcs[ords[i]];
            /* 转发导出（RVA 落在导出目录内）不动 */
            if (rva >= ed->VirtualAddress && rva < ed->VirtualAddress + ed->Size) break;
            if (!stubs) {
                stubs = alloc_near(mod, 0x10000);
                if (!stubs) {
                    logf_("EAT 钩子[%s]：无法在模块附近分配桩代码，跳过（不改内存，安全）", tag);
                    return done;
                }
            }
            if (stub_used + 16 > 0x10000) break;
            BYTE *stub = stubs + stub_used;
            stub_used += 16;
            write_stub(stub, specs[k].hook);

            void *original = (void *)(base + rva); /* 先记住原地址，再改表 */
            DWORD old = 0;
            if (!VirtualProtect(&funcs[ords[i]], sizeof(DWORD), PAGE_READWRITE, &old)) break;
            funcs[ords[i]] = (DWORD)(stub - base);
            DWORD tmp = 0;
            VirtualProtect(&funcs[ords[i]], sizeof(DWORD), old, &tmp);
            specs[k].orig = original;

            /* 自检：改完之后 GetProcAddress 必须真的返回我们的桩 */
            FARPROC check = GetProcAddress(mod, nm);
            logf_("  钩子 %s：原=%p 桩=%p 包装=%p GetProcAddress=%p %s", nm, original, stub,
                  specs[k].hook, (void *)check, check == (FARPROC)stub ? "OK" : "!!未生效");
            done++;
            break;
        }
    }
    logf_("EAT 钩子[%s]：命中 %d 项", tag, done);
    return done;
}

/* ------------------------------------------------------------------ */
/* imm32 包装                                                          */
/* ------------------------------------------------------------------ */
static PFN_ImmGetContext r_ImmGetContext;
static PFN_ImmReleaseContext r_ImmReleaseContext;
static PFN_ImmAssociateContext r_ImmAssociateContext;
static PFN_ImmAssociateContextEx r_ImmAssociateContextEx;
static PFN_ImmCreateContext r_ImmCreateContext;
static PFN_ImmDestroyContext r_ImmDestroyContext;
static PFN_ImmGetCompositionStringW r_ImmGetCompositionStringW;
static PFN_ImmSetCompositionStringW r_ImmSetCompositionStringW;
static PFN_ImmSetCompositionWindow r_ImmSetCompositionWindow;
static PFN_ImmSetCandidateWindow r_ImmSetCandidateWindow;
static PFN_ImmNotifyIME r_ImmNotifyIME;
static PFN_ImmGetOpenStatus r_ImmGetOpenStatus;
static PFN_ImmSetOpenStatus r_ImmSetOpenStatus;
static PFN_ImmGetConversionStatus r_ImmGetConversionStatus;
static PFN_ImmSetConversionStatus r_ImmSetConversionStatus;
static PFN_ImmLockIMC r_ImmLockIMC;
static PFN_ImmUnlockIMC r_ImmUnlockIMC;
static PFN_ImmLockIMCC r_ImmLockIMCC;
static PFN_ImmUnlockIMCC r_ImmUnlockIMCC;
static PFN_ImmGetDefaultIMEWnd r_ImmGetDefaultIMEWnd;
static PFN_ImmIsIME r_ImmIsIME;
static PFN_ImmGetDescriptionW r_ImmGetDescriptionW;
static PFN_ImmGetIMEFileNameW r_ImmGetIMEFileNameW;
typedef FARPROC(WINAPI *PFN_GPA)(HMODULE, LPCSTR);
static PFN_GPA r_GetProcAddress;

static HIMC_ WINAPI h_ImmGetContext(HWND h) {
    HIMC_ r = r_ImmGetContext ? r_ImmGetContext(h) : NULL;
    logf_("ImmGetContext(hwnd=%p) -> %p", (void *)h, r);
    log_backtrace("imm");
    return r;
}
static BOOL WINAPI h_ImmReleaseContext(HWND h, HIMC_ c) {
    BOOL r = r_ImmReleaseContext ? r_ImmReleaseContext(h, c) : FALSE;
    logf_("ImmReleaseContext(hwnd=%p, himc=%p) -> %d", (void *)h, c, (int)r);
    return r;
}
static HIMC_ WINAPI h_ImmAssociateContext(HWND h, HIMC_ c) {
    HIMC_ r = r_ImmAssociateContext ? r_ImmAssociateContext(h, c) : NULL;
    logf_("★ImmAssociateContext(hwnd=%p, himc=%p) -> %p%s", (void *)h, c, r,
          c == NULL ? "  ← 传入 NULL：等于关掉该窗口的输入法" : "");
    log_backtrace("imm");
    return r;
}
static BOOL WINAPI h_ImmAssociateContextEx(HWND h, HIMC_ c, DWORD f) {
    BOOL r = r_ImmAssociateContextEx ? r_ImmAssociateContextEx(h, c, f) : FALSE;
    logf_("ImmAssociateContextEx(hwnd=%p, himc=%p, flags=%#lx) -> %d", (void *)h, c,
          (unsigned long)f, (int)r);
    log_backtrace("imm");
    return r;
}
static HIMC_ WINAPI h_ImmCreateContext(void) {
    HIMC_ r = r_ImmCreateContext ? r_ImmCreateContext() : NULL;
    logf_("ImmCreateContext() -> %p", r);
    return r;
}
static BOOL WINAPI h_ImmDestroyContext(HIMC_ c) {
    BOOL r = r_ImmDestroyContext ? r_ImmDestroyContext(c) : FALSE;
    logf_("ImmDestroyContext(himc=%p) -> %d", c, (int)r);
    return r;
}
static LONG WINAPI h_ImmGetCompositionStringW(HIMC_ c, DWORD idx, LPVOID buf, DWORD cap) {
    LONG r = r_ImmGetCompositionStringW ? r_ImmGetCompositionStringW(c, idx, buf, cap) : 0;
    /* 客户端自己有没有来读"已上屏串"？用来决定 ime_bridge 要不要兜底插入，
     * 这样两边不会各插一遍（字不会重复）。 */
    if (idx == GCS_RESULTSTR) {
        InterlockedExchange(&g_clientReadResult, 1);
    }
    if (idx == GCS_COMPSTR || idx == GCS_RESULTSTR || idx == GCS_COMPREADSTR) {
        wchar_t tmp[512];
        int shown = 0;
        if (r > 0) {
            if (buf && r <= (LONG)sizeof(tmp) - 2 && cap >= (DWORD)r) {
                memcpy(tmp, buf, (size_t)r);
                tmp[r / 2] = 0;
                shown = 1;
            } else if (r <= (LONG)sizeof(tmp) - 2) {
                /* 只问长度：再要一次内容 */
                wchar_t tmp2[512];
                LONG r2 = r_ImmGetCompositionStringW(c, idx, tmp2, (DWORD)sizeof(tmp2));
                if (r2 > 0) {
                    tmp2[r2 / 2] = 0;
                    memcpy(tmp, tmp2, sizeof(tmp2) < sizeof(tmp) ? sizeof(tmp2) : sizeof(tmp));
                    shown = 1;
                }
            }
        }
        char u8[1024] = "";
        if (shown) w2u(tmp, u8, sizeof(u8));
        logf_("ImmGetCompositionStringW(himc=%p, idx=%#lx, cap=%lu) -> %ld  \"%s\"", c,
              (unsigned long)idx, (unsigned long)cap, r, u8);
    } else {
        logf_("ImmGetCompositionStringW(himc=%p, idx=%#lx, cap=%lu) -> %ld", c,
              (unsigned long)idx, (unsigned long)cap, r);
    }
    return r;
}
static BOOL WINAPI h_ImmSetCompositionStringW(HIMC_ c, DWORD idx, LPCVOID p1, DWORD n1,
                                             LPCVOID p2, DWORD n2) {
    BOOL r = r_ImmSetCompositionStringW ? r_ImmSetCompositionStringW(c, idx, p1, n1, p2, n2) : FALSE;
    logf_("ImmSetCompositionStringW(himc=%p, idx=%#lx, n1=%lu, n2=%lu) -> %d", c,
          (unsigned long)idx, (unsigned long)n1, (unsigned long)n2, (int)r);
    return r;
}
static BOOL WINAPI h_ImmSetCompositionWindow(HIMC_ c, const void *f) {
    BOOL r = r_ImmSetCompositionWindow ? r_ImmSetCompositionWindow(c, f) : FALSE;
    logf_("ImmSetCompositionWindow(himc=%p, form=%p) -> %d", c, f, (int)r);
    /* v2.0.3：记下客户端给输入法的位置，稍后用来回答 IME 的位置询问。 */
    if (f) {
        const probe_compositionform *cf = (const probe_compositionform *)f;
        if (cf->dwStyle & (CFS_POINT | CFS_CANDIDATEPOS)) {
            g_compPoint = cf->ptCurrentPos;
            g_compPointValid = 1;
        } else if (cf->dwStyle & CFS_RECT) {
            g_compPoint.x = cf->rcArea.left;
            g_compPoint.y = cf->rcArea.top;
            g_compPointValid = 1;
        }
    }
    return r;
}
static BOOL WINAPI h_ImmSetCandidateWindow(HIMC_ c, const void *f) {
    BOOL r = r_ImmSetCandidateWindow ? r_ImmSetCandidateWindow(c, f) : FALSE;
    logf_("ImmSetCandidateWindow(himc=%p, form=%p) -> %d", c, f, (int)r);
    return r;
}
static BOOL WINAPI h_ImmNotifyIME(HIMC_ c, DWORD a, DWORD b, DWORD d) {
    /* 备用：客户端若主动取消组字（NI_COMPOSITIONSTR + CPS_CANCEL），把它吞掉，
     * 让输入法有机会把这一笔组字走完。默认关（fix_cancel=1 才生效）。 */
    if (g_fixCancel && a == 0x0015 /* NI_COMPOSITIONSTR */ && b == 0x0004 /* CPS_CANCEL */) {
        logf_("★ 吞掉 ImmNotifyIME(CPS_CANCEL)：himc=%p（fix_cancel=1）", c);
        return TRUE;
    }
    BOOL r = r_ImmNotifyIME ? r_ImmNotifyIME(c, a, b, d) : FALSE;
    logf_("ImmNotifyIME(himc=%p, action=%#lx, idx=%#lx, val=%#lx) -> %d", c,
          (unsigned long)a, (unsigned long)b, (unsigned long)d, (int)r);
    return r;
}
static BOOL WINAPI h_ImmGetOpenStatus(HIMC_ c) {
    BOOL r = r_ImmGetOpenStatus ? r_ImmGetOpenStatus(c) : FALSE;
    logf_("ImmGetOpenStatus(himc=%p) -> %d", c, (int)r);
    return r;
}
static BOOL WINAPI h_ImmSetOpenStatus(HIMC_ c, BOOL open) {
    BOOL r = r_ImmSetOpenStatus ? r_ImmSetOpenStatus(c, open) : FALSE;
    logf_("ImmSetOpenStatus(himc=%p, open=%d) -> %d", c, (int)open, (int)r);
    return r;
}
static BOOL WINAPI h_ImmGetConversionStatus(HIMC_ c, LPDWORD conv, LPDWORD sent) {
    BOOL r = r_ImmGetConversionStatus ? r_ImmGetConversionStatus(c, conv, sent) : FALSE;
    logf_("ImmGetConversionStatus(himc=%p) -> %d conv=%#lx sent=%#lx", c, (int)r,
          conv ? (unsigned long)*conv : 0, sent ? (unsigned long)*sent : 0);
    return r;
}
static BOOL WINAPI h_ImmSetConversionStatus(HIMC_ c, DWORD conv, DWORD sent) {
    BOOL r = r_ImmSetConversionStatus ? r_ImmSetConversionStatus(c, conv, sent) : FALSE;
    logf_("ImmSetConversionStatus(himc=%p, conv=%#lx, sent=%#lx) -> %d", c,
          (unsigned long)conv, (unsigned long)sent, (int)r);
    return r;
}
static LPVOID WINAPI h_ImmLockIMC(HIMC_ c) {
    LPVOID r = r_ImmLockIMC ? r_ImmLockIMC(c) : NULL;
    logf_("ImmLockIMC(himc=%p) -> %p", c, r);
    log_backtrace("imc");
    return r;
}
static BOOL WINAPI h_ImmUnlockIMC(HIMC_ c) {
    BOOL r = r_ImmUnlockIMC ? r_ImmUnlockIMC(c) : FALSE;
    logf_("ImmUnlockIMC(himc=%p) -> %d", c, (int)r);
    return r;
}
static LPVOID WINAPI h_ImmLockIMCC(HIMCC_ c) {
    LPVOID r = r_ImmLockIMCC ? r_ImmLockIMCC(c) : NULL;
    logf_("ImmLockIMCC(himcc=%p) -> %p", c, r);
    log_backtrace("imcc");
    return r;
}
static BOOL WINAPI h_ImmUnlockIMCC(HIMCC_ c) {
    BOOL r = r_ImmUnlockIMCC ? r_ImmUnlockIMCC(c) : FALSE;
    logf_("ImmUnlockIMCC(himcc=%p) -> %d", c, (int)r);
    return r;
}
static HWND WINAPI h_ImmGetDefaultIMEWnd(HWND h) {
    HWND r = r_ImmGetDefaultIMEWnd ? r_ImmGetDefaultIMEWnd(h) : NULL;
    logf_("ImmGetDefaultIMEWnd(hwnd=%p) -> %p", (void *)h, (void *)r);
    return r;
}
static BOOL WINAPI h_ImmIsIME(HKL hkl) {
    BOOL r = r_ImmIsIME ? r_ImmIsIME(hkl) : FALSE;
    /* fix_isime：把「这个布局是不是输入法」也喂过去。
     * 实机（2026-10-06 16:32）证据：门禁过了、拼音也能累积（k→ku→kuang），但客户端问
     * ImmIsIME(0xE00E0804) 得到 0 —— 因为那个传统布局在本机并不存在。它会据此把输入
     * 当普通字母处理（结果 commit 出来的是 kuang 而不是汉字）。
     * 既然我们已经在假装机器上有传统中文 IME（HKL 与文件名都假装了），这里必须一致地
     * 回答「是」，否则客户端拿到的几个答案互相矛盾。只对**游戏主模块**生效。 */
    if (!r && g_fixIsIme && hkl && caller_is_main(_ReturnAddress())) {
        DWORD v = (DWORD)(ULONG_PTR)hkl;
        DWORD lang = v & 0xFFFF;
        int fake = 0;
        for (int i = 0; i < 3; i++)
            if (v == kFakeHkl[i]) fake = 1;
        if (v == g_layoutZhCN || v == g_layoutZhTW) fake = 1;
        if (lang == 0x0804 || lang == 0x0404) fake = 1;
        if (fake) {
            logf_("\u2605 ImmIsIME(hkl=%p) 本来 0 → 改报 1（fix_isime=1，主模块）", (void *)hkl);
            return TRUE;
        }
    }
    logf_("ImmIsIME(hkl=%p) -> %d", (void *)hkl, (int)r);
    return r;
}
static UINT WINAPI h_ImmGetDescriptionW(HKL hkl, LPWSTR buf, UINT cap) {
    UINT r = r_ImmGetDescriptionW ? r_ImmGetDescriptionW(hkl, buf, cap) : 0;
    char u8[512] = "";
    if (buf && r) { buf[r < cap ? r : cap - 1] = 0; w2u(buf, u8, sizeof(u8)); }
    logf_("ImmGetDescriptionW(hkl=%p) -> %u \"%s\"", (void *)hkl, r, u8);
    return r;
}
static UINT WINAPI h_ImmGetIMEFileNameW(HKL hkl, LPWSTR buf, UINT cap) {
    UINT r = r_ImmGetIMEFileNameW ? r_ImmGetIMEFileNameW(hkl, buf, cap) : 0;
    char u8[512] = "";
    if (buf && r) { buf[r < cap ? r : cap - 1] = 0; w2u(buf, u8, sizeof(u8)); }
    int faked = 0;
    int mine = caller_is_main(_ReturnAddress());
    if (g_fixImeName && mine && !name_is_whitelisted(buf ? buf : L"")) {
        /* 客户端的门禁只认 5 个 XP 时代的输入法文件名；本机（TSF 输入法）
         * 这个名字取不到或不在白名单里，于是 IME 通路整条不被启用。 */
        size_t len = wcslen(g_imeName);
        if (buf && cap > len) {
            wcscpy_s(buf, cap, g_imeName);
            r = (UINT)len;
            faked = 1;
        } else if (!buf || cap == 0) {
            r = (UINT)(len + 1);
            faked = 2;
        }
        w2u(g_imeName, u8, sizeof(u8));
    }
    logf_("ImmGetIMEFileNameW(hkl=%p, cap=%lu) -> %u \"%s\"%s%s", (void *)hkl, (unsigned long)cap,
          r, u8, mine ? " [主模块]" : " [系统模块:不改]",
          faked ? (faked == 1 ? "  ← 已替换成白名单名字" : "  ← 只改返回值") : "");
    return r;
}

static HKL(WINAPI *r_GetKeyboardLayout)(DWORD);

static HKL WINAPI h_GetKeyboardLayout(DWORD tid) {
    HKL real = r_GetKeyboardLayout ? r_GetKeyboardLayout(tid) : NULL;
    HKL out = real;
    int mine = caller_is_main(_ReturnAddress());
    if (g_fixLayout && mine && real) {
        WORD lang = (WORD)((ULONG_PTR)real & 0xFFFF);
        if (lang == 0x0804)
            out = (HKL)(ULONG_PTR)g_layoutZhCN;
        else if (lang == 0x0404)
            out = (HKL)(ULONG_PTR)g_layoutZhTW;
    }
    if (out != real)
        logf_("GetKeyboardLayout(tid=%lu) -> %p%s%p%s", (unsigned long)tid, (void *)real,
              "  ← 改报传统 IME HKL ", (void *)out, " [主模块]");
    else
        logf_("GetKeyboardLayout(tid=%lu) -> %p%s", (unsigned long)tid, (void *)real,
              mine ? " [主模块]" : " [系统模块]");
    return out;
}

/* GetProcAddress：只记 imm32 的 Imm* 解析（客户端靠它动态取 IME API） */
static FARPROC WINAPI h_GetProcAddress(HMODULE m, LPCSTR name) {
    FARPROC r = r_GetProcAddress ? r_GetProcAddress(m, name) : NULL;
    if (name && (name[0] == 'I') && strncmp(name, "Imm", 3) == 0) {
        logf_("GetProcAddress(module=%p, \"%s\") -> %p", (void *)m, name, (void *)r);
        log_backtrace("gpa");
    }
    return r;
}

/* ------------------------------------------------------------------ */
/* user32：CreateWindowEx 钩子（发现窗口 + 子类化）                      */
/* ------------------------------------------------------------------ */
static HWND(WINAPI *r_CreateWindowExW)(DWORD, LPCWSTR, LPCWSTR, DWORD, int, int, int, int,
                                       HWND, HMENU, HINSTANCE, LPVOID);
static HWND(WINAPI *r_CreateWindowExA)(DWORD, LPCSTR, LPCSTR, DWORD, int, int, int, int,
                                       HWND, HMENU, HINSTANCE, LPVOID);
static BOOL(WINAPI *r_DestroyWindow)(HWND);

static const char *skip_classes[] = {"Default IME", "IME", "MSCTFIME UI", "Chrome_", "Cef",
                                     "Awesomium", "Intermediate D3D Window", "tooltips_class32"};

static BOOL class_skipped(const char *cls) {
    for (int i = 0; i < (int)(sizeof(skip_classes) / sizeof(skip_classes[0])); i++)
        if (strstr(cls, skip_classes[i])) return TRUE;
    return FALSE;
}

static WNDPROC orig_of(HWND h) {
    WNDPROC r = NULL;
    EnterCriticalSection(&g_lock);
    for (int i = 0; i < g_winCount; i++)
        if (g_wins[i].hwnd == h) { r = g_wins[i].orig; break; }
    LeaveCriticalSection(&g_lock);
    return r;
}

static void log_window_msg(HWND h, UINT msg, WPARAM w, LPARAM l);

/* 备用手段：客户端自己不处理组字时，我们把 GCS_RESULTSTR（已上屏的那串字）
 * 逐字当成 WM_CHAR 投给同一个窗口 —— 客户端自己的文本输入是吃 WM_CHAR 的
 * （第一次实机里 ASCII 字母就是靠 WM_CHAR 进去的）。默认关（ime_bridge=1 才生效），
 * 因为如果客户端自己也处理组字，两边都会插一遍、字会重复。 */
static void bridge_result_string(HWND h) {
    if (!pImmGetContext || !pImmGetCompositionStringW) return;
    HIMC_ himc = pImmGetContext(h);
    if (!himc) return;
    LONG n = pImmGetCompositionStringW(himc, GCS_RESULTSTR, NULL, 0);
    if (n > 0 && n <= 512) {
        wchar_t buf[257];
        LONG got = pImmGetCompositionStringW(himc, GCS_RESULTSTR, buf, (DWORD)n);
        if (got > 0) {
            char u8[512] = "";
            int k = 0;
            for (LONG i = 0; i + 1 < got && k < (int)sizeof(u8) - 4; i += 2) {
                PostMessageW(h, WM_CHAR, (WPARAM)buf[i / 2], 1);
                k += WideCharToMultiByte(CP_UTF8, 0, &buf[i / 2], 1, u8 + k, 4, NULL, NULL);
            }
            u8[k] = 0;
            logf_("★ ime_bridge：把上屏串「%s」（%ld 字节）逐字投成 WM_CHAR 给 %p", u8, got,
                  (void *)h);
        }
    }
    pImmReleaseContext(h, himc);
}

static LRESULT CALLBACK probe_proc(HWND h, UINT msg, WPARAM w, LPARAM l) {
    switch (msg) {
    case WM_CHAR:
    case WM_SYSCHAR:
    case WM_KEYDOWN:
    case WM_KEYUP:
    case WM_SYSKEYDOWN:
    case WM_SYSKEYUP:
    case WM_INPUTLANGCHANGE:
    case WM_INPUTLANGCHANGEREQUEST:
    case WM_SETFOCUS:
    case WM_KILLFOCUS:
    case WM_PASTE:
    case WM_IME_STARTCOMPOSITION:
    case WM_IME_ENDCOMPOSITION:
    case WM_IME_COMPOSITION:
    case WM_IME_SETCONTEXT:
    case WM_IME_NOTIFY:
    case WM_IME_CHAR:
    case WM_IME_SELECT:
    case WM_IME_REQUEST:
    case WM_IME_KEYDOWN:
    case WM_IME_KEYUP:
        log_window_msg(h, msg, w, l);
        break;
    default:
        break;
    }
    /* v2.0.3：输入法问"字符画在哪 / 候选窗摆哪"时自己回答，并跳过客户端那套处理。
     *
     * 实机证据（业主 2026-10-06 17:40 日志）：候选窗不出字、中文角色名检查时客户端卡住，
     * 而日志里恰好出现 `WM_IME_REQUEST wparam=0x6`（IMR_QUERYCHARPOSITION）。这条老路径
     * 只在"客户端认为有传统 IME"时才会走（正是我们打开的那条），它的实现要么缺失要么卡住：
     * 输入法拿不到坐标就摆不出候选窗，严重时就在等这个回答。
     * 这里用客户端自己通过 ImmSetCompositionWindow 给的位置回答，风险面最小（只答两个纯查询）。 */
    if (g_fixCharPos && msg == WM_IME_REQUEST && l) {
        POINT pt = g_compPoint;
        if (!g_compPointValid || (pt.x == 0 && pt.y == 0)) {
            RECT rc;
            GetClientRect(h, &rc);
            pt.x = rc.left + 24;
            pt.y = rc.top + 24;
        }
        ClientToScreen(h, &pt);
        if (w == IMR_QUERYCHARPOSITION) {
            probe_imecharposition *p = (probe_imecharposition *)l;
            DWORD size = p->dwSize ? p->dwSize : (DWORD)sizeof(*p);
            p->pt = pt;
            if (size > 12) {
                p->cLineHeight = p->cLineHeight ? p->cLineHeight : 18;
            }
            if (size >= (DWORD)sizeof(*p)) {
                GetClientRect(h, &p->rcDocument);
                MapWindowPoints(h, NULL, (POINT *)&p->rcDocument, 2);
            }
            logf_("★ 回 IME 字符位置询问：char=%u -> (%ld,%ld) size=%u 行高=%u",
                  (unsigned)p->dwCharPos, (long)pt.x, (long)pt.y, (unsigned)size,
                  (unsigned)p->cLineHeight);
            return TRUE;
        }
        if (w == IMR_CANDIDATEWINDOW) {
            probe_candidateform *p = (probe_candidateform *)l;
            p->ptCurrentPos = pt;
            p->dwStyle |= CFS_CANDIDATEPOS;
            logf_("★ 回 IME 候选窗位置询问：-> (%ld,%ld)", (long)pt.x, (long)pt.y);
            return TRUE;
        }
    }
    /* 先让客户端自己处理这条消息；它如果自己读了 GCS_RESULTSTR，我们就不插手
     * （避免同一串字被插两遍）。 */
    int bridge = (g_imeBridge && msg == WM_IME_COMPOSITION && (l & GCS_RESULTSTR)) ? 1 : 0;
    if (bridge) InterlockedExchange(&g_clientReadResult, 0);
    WNDPROC orig = orig_of(h);
    LRESULT r = orig ? CallWindowProcW(orig, h, msg, w, l) : DefWindowProcW(h, msg, w, l);
    if (bridge && !g_clientReadResult) bridge_result_string(h);
    return r;
}

static void consider_window(HWND h, const char *why) {
    if (!h || !IsWindow(h)) return;
    DWORD pid = 0;
    GetWindowThreadProcessId(h, &pid);
    if (pid != GetCurrentProcessId()) return;

    wchar_t wcls[128] = L"";
    GetClassNameW(h, wcls, 128);
    char cls[256];
    w2u(wcls, cls, sizeof(cls));
    if (class_skipped(cls)) return;

    EnterCriticalSection(&g_lock);
    for (int i = 0; i < g_winCount; i++) {
        if (g_wins[i].hwnd != h) continue;
        /* 客户端可能在窗口创建后又自己 SetWindowLongPtr 一次，把我们的钩子顶掉。
         * 巡检时发现过程已经不是 probe_proc，就按"当前的过程"重新挂一次。 */
        WNDPROC now = (WNDPROC)GetWindowLongPtrW(h, GWLP_WNDPROC);
        if (now != probe_proc) {
            WNDPROC got = (WNDPROC)SetWindowLongPtrW(h, GWLP_WNDPROC, (LONG_PTR)probe_proc);
            if (got) g_wins[i].orig = got;
            LeaveCriticalSection(&g_lock);
            logf_("窗口 %p 的过程被改回（%p），已重新挂钩", (void *)h, (void *)now);
            return;
        }
        LeaveCriticalSection(&g_lock);
        return;
    }
    if (g_winCount < MAX_WIN) {
        LONG_PTR style = GetWindowLongPtrW(h, GWL_STYLE);
        LONG_PTR exstyle = GetWindowLongPtrW(h, GWL_EXSTYLE);
        DWORD clsStyle = (DWORD)GetClassLongPtrW(h, GCL_STYLE);
        WNDPROC orig = (WNDPROC)SetWindowLongPtrW(h, GWLP_WNDPROC, (LONG_PTR)probe_proc);
        if (orig) {
            g_wins[g_winCount].hwnd = h;
            g_wins[g_winCount].orig = orig;
            strncpy_s(g_wins[g_winCount].cls, sizeof(g_wins[g_winCount].cls), cls, _TRUNCATE);
            g_winCount++;
            LeaveCriticalSection(&g_lock);
            char parent[300] = "";
            HWND ph = GetParent(h);
            if (ph) {
                wchar_t pcls[128] = L"";
                GetClassNameW(ph, pcls, 128);
                char pc[256];
                w2u(pcls, pc, sizeof(pc));
                _snprintf_s(parent, sizeof(parent), _TRUNCATE, " parent=%p(%s)", (void *)ph, pc);
            }
            logf_("窗口[%s] hwnd=%p class=\"%s\" style=%#llx ex=%#llx clsStyle=%#lx CS_IME=%d%s",
                  why, (void *)h, cls, (unsigned long long)style, (unsigned long long)exstyle,
                  (unsigned long)clsStyle, (clsStyle & CS_IME) ? 1 : 0, parent);
            /* 每个窗口的输入法上下文状态：聊天框那种"没有 HIMC 的窗口"是重点怀疑对象，
             * 转换模式则直接回答"用户以为在打中文、其实输入法在英文模式"这个可能。 */
            if (pImmGetContext && !class_skipped(cls)) {
                HIMC_ wc = pImmGetContext(h);
                if (wc) {
                    DWORD conv = 0, sent = 0;
                    int haveConv = r_ImmGetConversionStatus
                                       ? (int)r_ImmGetConversionStatus(wc, &conv, &sent) : 0;
                    logf_("    └ 输入法上下文：himc=%p 开启=%d 转换模式=%#lx（%s）", (void *)wc,
                          pImmGetOpenStatus ? (int)pImmGetOpenStatus(wc) : -1,
                          (unsigned long)(haveConv ? conv : 0),
                          !haveConv ? "取不到" : ((conv & 0x1) ? "中文/原生" : "英文/字母数字"));
                    if (pImmReleaseContext) pImmReleaseContext(h, wc);
                } else {
                    logf_("    └ 输入法上下文：himc=NULL（这个窗口没有输入法上下文）");
                }
            }
            /* post_layout=1 时，主动给**每个新建的顶层窗口**补一条
             * WM_INPUTLANGCHANGE（wParam=传统 IME HKL）。微软 IME 样例就是在
             * 这条消息里更新自己的布局字段的；不知道游戏用哪个窗口收发，就都发一次
             * （每个窗口只发一次，量很小）。 */
            if (g_postLayout && GetParent(h) == NULL) {
                PostMessageW(h, WM_INPUTLANGCHANGE, (WPARAM)(ULONG_PTR)g_layoutZhCN, 0);
                logf_("已给顶层窗口 %p 补发 WM_INPUTLANGCHANGE(wparam=%#lx)", (void *)h,
                      (unsigned long)g_layoutZhCN);
            }
            return;
        }
    }
    LeaveCriticalSection(&g_lock);
}

static void log_window_msg(HWND h, UINT msg, WPARAM w, LPARAM l) {
    const char *name = "?";
    switch (msg) {
    case WM_CHAR: name = "WM_CHAR"; break;
    case WM_SYSCHAR: name = "WM_SYSCHAR"; break;
    case WM_KEYDOWN: name = "WM_KEYDOWN"; break;
    case WM_KEYUP: name = "WM_KEYUP"; break;
    case WM_SYSKEYDOWN: name = "WM_SYSKEYDOWN"; break;
    case WM_SYSKEYUP: name = "WM_SYSKEYUP"; break;
    case WM_INPUTLANGCHANGE: name = "WM_INPUTLANGCHANGE"; break;
    case WM_INPUTLANGCHANGEREQUEST: name = "WM_INPUTLANGCHANGEREQUEST"; break;
    case WM_SETFOCUS: name = "WM_SETFOCUS"; break;
    case WM_KILLFOCUS: name = "WM_KILLFOCUS"; break;
    case WM_PASTE: name = "WM_PASTE"; break;
    case WM_IME_STARTCOMPOSITION: name = "WM_IME_STARTCOMPOSITION"; break;
    case WM_IME_ENDCOMPOSITION: name = "WM_IME_ENDCOMPOSITION"; break;
    case WM_IME_COMPOSITION: name = "WM_IME_COMPOSITION"; break;
    case WM_IME_SETCONTEXT: name = "WM_IME_SETCONTEXT"; break;
    case WM_IME_NOTIFY: name = "WM_IME_NOTIFY"; break;
    case WM_IME_CHAR: name = "WM_IME_CHAR"; break;
    case WM_IME_SELECT: name = "WM_IME_SELECT"; break;
    case WM_IME_REQUEST: name = "WM_IME_REQUEST"; break;
    case WM_IME_KEYDOWN: name = "WM_IME_KEYDOWN"; break;
    case WM_IME_KEYUP: name = "WM_IME_KEYUP"; break;
    }
    WNDPROC orig = orig_of(h);
    char cls[256] = "";
    EnterCriticalSection(&g_lock);
    for (int i = 0; i < g_winCount; i++)
        if (g_wins[i].hwnd == h) { strncpy_s(cls, sizeof(cls), g_wins[i].cls, _TRUNCATE); break; }
    LeaveCriticalSection(&g_lock);

    char extra[512] = "";
    if (msg == WM_CHAR || msg == WM_SYSCHAR || msg == WM_IME_CHAR) {
        _snprintf_s(extra, sizeof(extra), _TRUNCATE, " char=U+%04llX(%s) rep=%d",
                    (unsigned long long)(w & 0xFFFF),
                    (w >= 0x20 && w < 0x7F) ? "ascii" : (w < 0x20 ? "ctrl" : "non-ascii"),
                    (int)(l & 0xFFFF));
    } else if (msg == WM_IME_COMPOSITION) {
        _snprintf_s(extra, sizeof(extra), _TRUNCATE, " flags=%#llx%s%s%s",
                    (unsigned long long)l, (l & GCS_COMPSTR) ? " COMPSTR" : "",
                    (l & GCS_RESULTSTR) ? " RESULTSTR" : "",
                    (l & GCS_CURSORPOS) ? " CURSOR" : "");
    } else if (msg == WM_IME_SETCONTEXT) {
        _snprintf_s(extra, sizeof(extra), _TRUNCATE, " active=%llu flags=%#llx",
                    (unsigned long long)w, (unsigned long long)l);
    }
    logf_("%s hwnd=%p[%s] wparam=%#llx lparam=%#llx orig=%p%s", name, (void *)h, cls,
          (unsigned long long)w, (unsigned long long)l, (void *)orig, extra);

    if (msg == WM_IME_COMPOSITION && pImmGetContext && pImmGetCompositionStringW) {
        HIMC_ c = pImmGetContext(h);
        if (c) {
            wchar_t buf[512];
            LONG n = pImmGetCompositionStringW(c, GCS_COMPSTR, buf, sizeof(buf) - 2);
            if (n > 0) {
                buf[n / 2] = 0;
                char u8[1024];
                w2u(buf, u8, sizeof(u8));
                logf_("    （探针自己读到的组字串：\"%s\"）", u8);
            }
            pImmReleaseContext(h, c);
        }
    }
}

static HWND WINAPI h_CreateWindowExW(DWORD ex, LPCWSTR cls, LPCWSTR title, DWORD style, int x,
                                     int y, int w, int h, HWND parent, HMENU menu, HINSTANCE inst,
                                     LPVOID param) {
    HWND r = r_CreateWindowExW ? r_CreateWindowExW(ex, cls, title, style, x, y, w, h, parent, menu,
                                                   inst, param)
                               : NULL;
    consider_window(r, "create-exW");
    return r;
}
static HWND WINAPI h_CreateWindowExA(DWORD ex, LPCSTR cls, LPCSTR title, DWORD style, int x, int y,
                                     int w, int h, HWND parent, HMENU menu, HINSTANCE inst,
                                     LPVOID param) {
    HWND r = r_CreateWindowExA ? r_CreateWindowExA(ex, cls, title, style, x, y, w, h, parent, menu,
                                                   inst, param)
                               : NULL;
    consider_window(r, "create-exA");
    return r;
}
static BOOL WINAPI h_DestroyWindow(HWND h) {
    logf_("DestroyWindow(hwnd=%p)", (void *)h);
    EnterCriticalSection(&g_lock);
    for (int i = 0; i < g_winCount; i++) {
        if (g_wins[i].hwnd == h) {
            g_wins[i] = g_wins[g_winCount - 1];
            g_winCount--;
            break;
        }
    }
    LeaveCriticalSection(&g_lock);
    return r_DestroyWindow ? r_DestroyWindow(h) : FALSE;
}

/* ------------------------------------------------------------------ */
/* 环境与巡检                                                          */
/* ------------------------------------------------------------------ */
static void log_layouts(void) {
    int n = GetKeyboardLayoutList(0, NULL);
    HKL *list = (HKL *)HeapAlloc(GetProcessHeap(), 0, sizeof(HKL) * (n > 0 ? n : 1));
    if (!list) return;
    n = GetKeyboardLayoutList(n, list);
    logf_("键盘布局共 %d 个：", n);
    for (int i = 0; i < n; i++) {
        char desc[512] = "", file[512] = "";
        wchar_t buf[512] = L"";
        UINT rd = 0, rf = 0;
        if (pImmGetDescriptionW) {
            rd = pImmGetDescriptionW(list[i], buf, 512);
            if (rd) { buf[rd < 511 ? rd : 511] = 0; w2u(buf, desc, sizeof(desc)); }
        }
        if (pImmGetIMEFileNameW) {
            rf = pImmGetIMEFileNameW(list[i], buf, 512);
            if (rf) { buf[rf < 511 ? rf : 511] = 0; w2u(buf, file, sizeof(file)); }
        }
        wchar_t name[64] = L"";
        GetKeyboardLayoutNameW(name);
        char n8[128];
        w2u(name, n8, sizeof(n8));
        logf_("  hkl=%p IsIME=%d 布局ID=%s desc(r=%u)=\"%s\" ime(r=%u)=\"%s\"", (void *)list[i],
              pImmIsIME ? (int)pImmIsIME(list[i]) : -1, n8, rd, desc, rf, file);
    }
    HeapFree(GetProcessHeap(), 0, list);
}

static void log_focus_state(void) {
    HWND fg = GetForegroundWindow();
    DWORD pid = 0;
    DWORD tid = GetWindowThreadProcessId(fg, &pid);
    HWND focus = NULL;
    if (tid) focus = GetFocus();
    wchar_t wcls[128] = L"";
    if (fg) GetClassNameW(fg, wcls, 128);
    char cls[256];
    w2u(wcls, cls, sizeof(cls));
    HKL hkl = tid ? GetKeyboardLayout(tid) : NULL;
    logf_("前台：hwnd=%p[%s] pid=%lu tid=%lu 焦点=%p hkl=%p IsIME=%d", (void *)fg, cls,
          (unsigned long)pid, (unsigned long)tid, (void *)focus, (void *)hkl,
          (pImmIsIME && hkl) ? (int)pImmIsIME(hkl) : -1);

    if (fg && pid == GetCurrentProcessId() && pImmGetContext) {
        HIMC_ c = pImmGetContext(fg);
        if (c) {
            int open = pImmGetOpenStatus ? (int)pImmGetOpenStatus(c) : -1;
            wchar_t buf[512] = L"";
            LONG n = pImmGetCompositionStringW ? pImmGetCompositionStringW(c, GCS_COMPSTR, buf,
                                                                          sizeof(buf) - 2)
                                               : 0;
            if (n > 0) buf[n / 2] = 0;
            char u8[1024];
            w2u(buf, u8, sizeof(u8));
            logf_("  该窗口 himc=%p 输入法开启=%d 组字串=\"%s\"", c, open, u8);
            pImmReleaseContext(fg, c);
        } else {
            logf_("  该窗口 ImmGetContext= NULL（窗口没有关联输入法上下文）");
        }
    }
}

/* ------------------------------------------------------------------ */
/* 配置：<DLL 目录>\chinese-input.ini（缺省全 0 = 只观察）                */
/* ------------------------------------------------------------------ */
static int ini_int(const char *text, const char *key, int dflt) {
    const char *p = text;
    size_t klen = strlen(key);
    while (p && *p) {
        const char *eol = strchr(p, '\n');
        size_t len = eol ? (size_t)(eol - p) : strlen(p);
        if (len > klen && _strnicmp(p, key, klen) == 0 && p[klen] == '=') {
            return atoi(p + klen + 1);
        }
        p = eol ? eol + 1 : NULL;
    }
    return dflt;
}

/* HKL 一类的值是十六进制（ini 里写 E00E0804），必须按 16 进制解析；
 * 解析成 0 会让 GetKeyboardLayout 返回 NULL，比不修还糟。 */
static DWORD ini_hex(const char *text, const char *key, DWORD dflt) {
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
            return (DWORD)value;
        }
        p = eol ? eol + 1 : NULL;
    }
    return dflt;
}

static void ini_wstr(const char *text, const char *key, wchar_t *out, size_t cap, const wchar_t *dflt) {
    const char *p = text;
    size_t klen = strlen(key);
    wcscpy_s(out, cap, dflt);
    while (p && *p) {
        const char *eol = strchr(p, '\n');
        size_t len = eol ? (size_t)(eol - p) : strlen(p);
        if (len > klen && _strnicmp(p, key, klen) == 0 && p[klen] == '=') {
            char tmp[128];
            size_t n = len - klen - 1;
            if (n >= sizeof(tmp)) n = sizeof(tmp) - 1;
            memcpy(tmp, p + klen + 1, n);
            tmp[n] = 0;
            MultiByteToWideChar(CP_UTF8, 0, tmp, -1, out, (int)cap);
            return;
        }
        p = eol ? eol + 1 : NULL;
    }
}

/* ------------------------------------------------------------------ */
/* 客户端 C2S 发包追踪（可选，默认关）：trace_c2s = 0/1/2                */
/*                                                                     */
/* 客户端发包链（analysis/dumps/CLIENT-MECHANICS.md §2）：                */
/*   RVA 0x6D746E0(writer, opcode)   写 opcode + 13 字节前导             */
/*   RVA 0x6D75B10(writer, buf, len) 追加 body                          */
/*   RVA 0x6D75AF0()                 发送                               */
/* 前两个函数开头是**位置无关的完整指令**，可以安全地做 5/6 字节 inline   */
/* 跳转（跳板先执行被搬走的原指令再跳回）；发送函数首字节含 rel32 call，   */
/* 搬不动，所以不钩它。所有跳转前都先逐字节核对期望字节，对不上就不装。    */
/* ------------------------------------------------------------------ */
static int g_traceC2S = 0;
static unsigned short g_lastOpcode = 0;

static BYTE *alloc_near(HMODULE mod, SIZE_T size); /* 定义在后面（EAT 钩子那节） */

typedef struct {
    const char *name;
    unsigned char *target;
    const unsigned char *expect;
    size_t patchLen;
    void *hook;
    void *orig;
} InlineHook;

static void *r_writeOpcode;
static void *r_appendBody;

static void write_rel_jmp(unsigned char *at, const void *to) {
    at[0] = 0xE9;
    *(int *)(at + 1) = (int)((const unsigned char *)to - (at + 5));
}

static void write_abs_jmp(unsigned char *at, const void *to) {
    at[0] = 0xFF;
    at[1] = 0x25;
    *(DWORD *)(at + 2) = 0; /* jmp qword ptr [rip+0] */
    *(void **)(at + 6) = (void *)to;
}

/* 目标地址必须是**已提交且可读**的内存，否则连 memcmp 都会崩。
 * 换一个客户端构建时 RVA 可能根本不在镜像里，这一步是必须的保险。 */
static int range_readable(const void *addr, size_t len) {
    MEMORY_BASIC_INFORMATION mbi;
    if (VirtualQuery(addr, &mbi, sizeof(mbi)) != sizeof(mbi)) return 0;
    if (mbi.State != MEM_COMMIT) return 0;
    if (mbi.Protect & (PAGE_NOACCESS | PAGE_GUARD)) return 0;
    const unsigned char *end = (const unsigned char *)addr + len;
    const unsigned char *regionEnd = (const unsigned char *)mbi.BaseAddress + mbi.RegionSize;
    return end <= regionEnd;
}

static int install_inline(HMODULE mod, InlineHook *h) {
    unsigned char *p = (unsigned char *)mod + (h->target - (unsigned char *)mod);
    if (!range_readable(p, h->patchLen)) {
        logf_("inline 钩子[%s] 跳过：目标 %p 不在已提交内存里（客户端版本不同？）", h->name, p);
        return 0;
    }
    if (memcmp(p, h->expect, h->patchLen) != 0) {
        logf_("inline 钩子[%s] 跳过：期望字节对不上（现场 %02x %02x %02x %02x %02x %02x）", h->name,
              p[0], p[1], p[2], p[3], p[4], p[5]);
        return 0;
    }
    unsigned char *stub = alloc_near(mod, 0x1000);
    if (!stub) {
        logf_("inline 钩子[%s] 跳过：模块附近没有可分配内存", h->name);
        return 0;
    }
    unsigned char *tramp = stub;             /* 跳板 */
    unsigned char *bridge = stub + 0x40;     /* 补丁 → 包装 的桥 */
    /* 补丁处用的是 rel32（±2GB），桥必须离目标够近；不在范围内宁可跳过。 */
    LONG_PTR dist = (LONG_PTR)bridge - (LONG_PTR)p;
    if (dist > 0x7FF00000LL || dist < -0x7FF00000LL) {
        logf_("inline 钩子[%s] 跳过：桥离目标 %lld 字节，超出 rel32 范围", h->name,
              (long long)dist);
        return 0;
    }
    memcpy(tramp, p, h->patchLen);           /* 先执行被搬走的原指令 */
    write_rel_jmp(tramp + h->patchLen, p + h->patchLen);
    write_abs_jmp(bridge, h->hook);

    DWORD old = 0;
    if (!VirtualProtect(p, h->patchLen, PAGE_EXECUTE_READWRITE, &old)) {
        logf_("inline 钩子[%s] 跳过：无法改页面保护", h->name);
        return 0;
    }
    write_rel_jmp(p, bridge);
    for (size_t i = 5; i < h->patchLen; i++) p[i] = 0x90; /* 余下补 NOP */
    DWORD tmp = 0;
    VirtualProtect(p, h->patchLen, old, &tmp);
    FlushInstructionCache(GetCurrentProcess(), p, h->patchLen);
    h->orig = tramp;
    logf_("inline 钩子[%s] 已装：%p → 桥 %p，跳板 %p（覆盖 %zu 字节）", h->name, p, bridge, tramp,
          h->patchLen);
    return 1;
}

static void WINAPI h_writeOpcode(void *writer, int opcode) {
    g_lastOpcode = (unsigned short)opcode;
    logf_("C2S opcode=%d（writer=%p）", opcode, writer);
    ((void(WINAPI *)(void *, int))r_writeOpcode)(writer, opcode);
}

static void WINAPI h_appendBody(void *writer, const void *buf, int len) {
    if (g_traceC2S >= 2 || len <= 0) {
        char hex[3 * 48 + 4];
        int n = len > 48 ? 48 : len;
        for (int i = 0; i < n; i++) _snprintf_s(hex + i * 3, 4, _TRUNCATE, "%02x ",
                                                ((const unsigned char *)buf)[i]);
        hex[n * 3] = 0;
        logf_("C2S body(opcode=%u) len=%d 前 %d 字节：%s", g_lastOpcode, len, n, hex);
    }
    ((void(WINAPI *)(void *, const void *, int))r_appendBody)(writer, buf, len);
}

static void install_c2s_trace(HMODULE exe) {
    static const unsigned char expectOpcode[] = {0x40, 0x55, 0x56, 0x57, 0x41, 0x56};
    static const unsigned char expectAppend[] = {0x48, 0x89, 0x5C, 0x24, 0x08};
    InlineHook hooks[2] = {
        {"writeOpcode", (unsigned char *)exe + 0x6D746E0, expectOpcode, sizeof(expectOpcode),
         (void *)h_writeOpcode, NULL},
        {"appendBody", (unsigned char *)exe + 0x6D75B10, expectAppend, sizeof(expectAppend),
         (void *)h_appendBody, NULL},
    };
    if (install_inline(exe, &hooks[0])) r_writeOpcode = hooks[0].orig;
    if (install_inline(exe, &hooks[1])) r_appendBody = hooks[1].orig;
}

/* ------------------------------------------------------------------ */
/* Themida 导入槽修正（第 12 轮：第一次实机找到的真正拦路虎）              */
/* ------------------------------------------------------------------ */
/*
 * 客户端是 Themida 加壳的，它调用 user32/imm32 走的是**它自己的导入桩**：
 *       FF 25 <disp32>        →   jmp qword ptr [slot]
 * 桩表集中在 .rdata 的 RVA 0x9186160..0x9187988（静态扫出 297 个槽），
 * 槽里是提前解析好的**真实函数地址**。所以我们改导出表（EAT）对客户端毫无作用 ——
 * 第一次实机日志里，客户端一次都没进钩子，只有 MSCTF 之类的系统模块进来了。
 *
 * 修法：只在**精确匹配**（槽值 == 我们的原函数地址）时把槽换成我们的包装。
 * 槽是 Themida 解包时才填的，所以周期性重扫；扫完就什么都不剩（幂等）。
 */
#define SLOT_LO 0x9186100
#define SLOT_HI 0x9187A00

/* 钩子表副本：主循环里要周期性重扫槽（Themida 解包后才填值），
 * 而 spec 是 worker 里的局部数组。 */
static HookSpec g_specUser32[8];
static int g_nUser32;
static HookSpec g_specImm32[32];
static int g_nImm32;
static unsigned g_exeImageSize; /* 主模块镜像大小（PE OptionalHeader.SizeOfImage） */

static int fixup_import_slots(HMODULE exe, HookSpec *spec, int n) {
    int fixed = 0;
    /* 只在主模块镜像范围内扫：换一个小 EXE（自测宿主）时这段 RVA 根本不存在，
     * 免得跑到别的模块映射区里去改东西。 */
    if (!g_exeImageSize || SLOT_HI > g_exeImageSize) return 0;
    for (size_t off = SLOT_LO; off + sizeof(void *) <= SLOT_HI; off += sizeof(void *)) {
        void **slot = (void **)((unsigned char *)exe + off);
        if (!range_readable(slot, sizeof(void *))) continue;
        void *cur = *slot;
        if (!cur) continue;
        for (int i = 0; i < n; i++) {
            if (!spec[i].orig || cur != spec[i].orig) continue;
            DWORD old = 0, tmp = 0;
            if (!VirtualProtect(slot, sizeof(void *), PAGE_READWRITE, &old)) break;
            *slot = spec[i].hook;
            VirtualProtect(slot, sizeof(void *), old, &tmp);
            logf_("    Themida 槽 rva %#llx ← %s：原 %p → 包装 %p", (unsigned long long)off,
                  spec[i].name, cur, spec[i].hook);
            fixed++;
            break;
        }
    }
    return fixed;
}

/* ------------------------------------------------------------------ */
/* 门禁本体：RVA 0x6F22400  __fastcall gate(Mgr *mgr)                   */
/* ------------------------------------------------------------------ */
/*
 * 反出来的完整逻辑（.pdata 边界 0x6F22400..0x6F226C6）：
 *
 *     mgr->[0x88] = 0;
 *     hkl = mgr->[0x68];
 *     if (hkl != E0080404 && hkl != E0090404 && hkl != E00E0804) goto REJECT;
 *     if (ImmGetIMEFileNameW(hkl, buf, 0x3FF) == 0)              goto REJECT;
 *     if (mgr->[0x98] != 0) goto ACCEPT;              // 已经拿过 IME 对象
 *     for (白名单 5 个名字) if (strcmp(buf, name) == 0) goto ACCEPT;
 *   REJECT: 直接返回（IME 通路不启用）
 *   ACCEPT: 转换文件名、建立 IME 对象…
 *
 * 所以只要在入口把 mgr->[0x68] 换成传统 IME 的 HKL，第一条就过了；
 * 文件名那条由 ImmGetIMEFileNameW 钩子负责（槽修正后客户端会走我们的包装）。
 */
typedef void(WINAPI *GateFn)(void *mgr);
static GateFn r_gate;
static int g_gateInstalled;

static void WINAPI h_gate(void *mgr) {
    if (mgr && range_readable((char *)mgr + 0x68, sizeof(unsigned long long))) {
        unsigned long long *hkl = (unsigned long long *)((char *)mgr + 0x68);
        unsigned long long ext = 0;
        if (range_readable((char *)mgr + 0x98, sizeof(unsigned long long)))
            ext = *(unsigned long long *)((char *)mgr + 0x98);
        if (*hkl != 0xE0080404ULL && *hkl != 0xE0090404ULL && *hkl != 0xE00E0804ULL) {
            logf_("★ 门禁[gate] mgr=%p：mgr+0x68 从 %#llx 改成 0xE00E0804（+0x98=%#llx）", mgr,
                  *hkl, ext);
            *hkl = 0xE00E0804ULL;
        } else {
            logf_("门禁[gate] mgr=%p：mgr+0x68 已是 %#llx（+0x98=%#llx），直接通过", mgr, *hkl,
                  ext);
        }
    }
    ((GateFn)r_gate)(mgr);
}

static int try_install_gate(HMODULE exe) {
    static const unsigned char expect[] = {0x40, 0x55, 0x48, 0x81, 0xEC,
                                           0x50, 0x04, 0x00, 0x00};
    unsigned char *p = (unsigned char *)exe + 0x6F22400;
    /* Themida 还没解包到这儿时字节对不上：静默等下一轮，不刷日志 */
    if (!range_readable(p, sizeof(expect))) return 0;
    if (memcmp(p, expect, sizeof(expect)) != 0) return 0;
    InlineHook h = {"imeGate", p, expect, sizeof(expect), (void *)h_gate, NULL};
    if (!install_inline(exe, &h)) return 0;
    r_gate = (GateFn)h.orig;
    g_gateInstalled = 1;
    return 1;
}

/* ------------------------------------------------------------------ */
/* 布局写入点：RVA 0x6F23070                                            */
/* ------------------------------------------------------------------ */
/*
 * 反出来的原始逻辑（.pdata 边界 0x6F23070..0x6F2336B）：
 *
 *     rbx = r9;                    // 第 4 个参数 = HKL
 *     rdi = rcx;                   // 第 1 个参数 = IME 管理器
 *     *(u64 *)(rcx + 0x68) = rbx;  // ★ 布局字段就是在这里被写进去的
 *     *(u16 *)(rcx + 0x70) = (u16)rbx;
 *     call 0x6F222F0(lang);        // 按语言 ID 分发
 *     switch (…) { … }
 *
 * 也就是说：门禁读的 `mgr+0x68` 来自这里。客户端拿 HKL 的路子（GetKeyboardLayout
 * 或 WM_INPUTLANGCHANGE 的 wParam）不管走哪条，最后都落在这个函数上。
 * 所以在这一层把**中文布局**换成传统 IME HKL，是最贴近源头的一道保险：
 * 只要它在插件装上之后被调用过一次，管理器的字段就是对的。
 */
typedef void(WINAPI *SetLayoutFn)(void *mgr, void *a2, void *a3, unsigned long long hkl);
static SetLayoutFn r_setLayout;
static int g_setLayoutInstalled;

static void WINAPI h_setLayout(void *mgr, void *a2, void *a3, unsigned long long hkl) {
    unsigned short lang = (unsigned short)(hkl & 0xFFFF);
    if (lang == 0x0804 || lang == 0x0404) {
        unsigned long long fake = (lang == 0x0404) ? (unsigned long long)g_layoutZhTW
                                                  : (unsigned long long)g_layoutZhCN;
        if (hkl != fake) {
            logf_("★ 布局写入[mgr=%p]：%#llx → %#llx（语言 %#x）", mgr, hkl, fake, lang);
            hkl = fake;
        }
    }
    ((SetLayoutFn)r_setLayout)(mgr, a2, a3, hkl);
}

static int try_install_setter(HMODULE exe) {
    static const unsigned char expect[] = {0x48, 0x89, 0x5C, 0x24, 0x18};
    unsigned char *p = (unsigned char *)exe + 0x6F23070;
    if (!range_readable(p, sizeof(expect))) return 0;
    if (memcmp(p, expect, sizeof(expect)) != 0) return 0;
    InlineHook h = {"imeSetLayout", p, expect, sizeof(expect), (void *)h_setLayout, NULL};
    if (!install_inline(exe, &h)) return 0;
    r_setLayout = (SetLayoutFn)h.orig;
    g_setLayoutInstalled = 1;
    return 1;
}


static const char *kDefaultIni =
    "# chinese-input.ini —— 中文输入插件开关（本文件由插件在缺失时自动生成，只生成一次）\n"
    "#\n"
    "# 背景：客户端的 IME 通路只在两个门禁都过时才启用——\n"
    "#   1) 键盘布局 HKL 属于 { E0080404, E0090404, E00E0804 }（XP 时代传统 IME）；\n"
    "#   2) ImmGetIMEFileNameW 返回 { TINTLGNT.IME, CINTLGNT.IME, MSTCIPHA.IME,\n"
    "#      PINTLGNT.IME, MSSCIPYA.IME } 之一。\n"
    "# 现代 Windows 的 TSF 输入法两个都不满足（本机 HKL=08040804、文件名取不到），\n"
    "# 于是中文输入整条链路不会被启用。下面两个开关就是把门禁喂过去。\n"
    "#\n"
    "# 只对游戏主模块生效；MSCTF 等系统模块照实回答，否则会把输入法框架本身搞坏。\n"
    "# 想退回纯观察：把所有开关改成 0，重启游戏即可（不需要重装）。\n"
    "\n"
    "# 中文布局改报传统 IME HKL（0x0804→E00E0804、0x0404→E0080404）\n"
    "fix_layout=1\n"
    "\n"
    "# ImmGetIMEFileNameW 返回白名单里的名字\n"
    "fix_ime_name=1\n"
    "\n"
    "# 给每个新建的顶层窗口补一条 WM_INPUTLANGCHANGE（覆盖「从消息取 HKL」那条路径）\n"
    "post_layout=1\n"
    "\n"
    "# 修正 Themida 导入槽：客户端是加壳的，它调 user32/imm32 走自己的导入桩\n"
    "# （FF 25 → 槽里存着提前解析好的真实地址），改导出表对它无效。\n"
    "# 打开后：精确匹配到我们的原函数地址就把槽换成我们的包装。\n"
    "fix_slots=1\n"
    "\n"
    "# 钩住客户端门禁本体（RVA 0x6F22400），入口直接把 mgr+0x68 改成传统 IME 的 HKL。\n"
    "# 这是「喂门禁」的最后一道保险：不管客户端从哪拿到真实 HKL，门禁都会过。\n"
    "fix_gate=1\n"
    "\n"
    "# —— 下面两个是备用手段，默认关，只在日志证明「客户端自己不处理组字」时才打开 ——\n"
    "#\n"
    "# 吞掉客户端主动取消组字（ImmNotifyIME：NI_COMPOSITIONSTR + CPS_CANCEL）。\n"
    "# 2026-10-06 实机：不打开它，客户端每打一个字就取消组字，拼音永远只有单个字母。\n"
    "# 如果日志里看到客户端一直在取消组字、导致组字串永远只有单个字母，就打开它。\n"
        "# ImmIsIME 对“传统布局”改报“是输入法”：实机 16:32 日志里客户端问 ImmIsIME(0xE00E0804) 得到 0，于是把拼音当普通字母上屏（kuang）。\n"
    "fix_isime=1\n"
"# 自己回答 IME 的位置询问（候选窗摆位）。实机若出现「候选字不显示 / 中文角色名检查卡住」，\n"
"# 先把它改成 0 对照一次：关掉就退回客户端自己处理那条老路径。\n"
"fix_charpos=1\n"
"fix_cancel=1\n"
    "\n"
    "# 客户端不处理组字时，把已上屏的结果串（GCS_RESULTSTR）逐字当 WM_CHAR 投给它。\n"
    "# 注意：如果客户端自己也会处理组字，两边都插一遍、字会重复 —— 所以默认关。\n"
    "ime_bridge=1\n"
    "\n"
    "# 客户端 C2S 发包追踪：0=关 1=只记 opcode 2=连 body 前 48 字节\n"
    "# （用 inline 跳转改客户端代码，装之前逐字节核对现场，对不上就不装；默认关）\n"
    "trace_c2s=0\n"
    "\n"
    "# 冒充哪个输入法文件名（必须是上面 5 个之一）\n"
    "ime_name=CINTLGNT.IME\n"
    "\n"
    "# 两个冒充用的 HKL\n"
    "layout_zhcn=E00E0804\n"
    "layout_zhtw=E0080404\n";

static void write_default_ini(const char *path) {
    FILE *f = fopen(path, "wb");
    if (!f) return;
    fputs(kDefaultIni, f);
    fclose(f);
}

static void load_config(void) {
    char path[MAX_PATH * 2];
    char dir8[MAX_PATH * 2];
    w2u(g_dir, dir8, sizeof(dir8));
    _snprintf_s(path, sizeof(path), _TRUNCATE, "%s\\chinese-input.ini", dir8);
    FILE *f = fopen(path, "rb");
    if (!f) {
        /* 自己生成一份：这样 mod 只落位 DLL，升级时不会撞上 "file.add 不能覆盖"。
         * 生成之后就不再动它——使用者改过的开关必须活下来。 */
        write_default_ini(path);
        logf_("没有 %s → 已生成默认开关（fix_layout=%d fix_ime_name=%d trace_c2s=%d）", path,
              g_fixLayout, g_fixImeName, g_traceC2S);
        return;
    }
    char text[4096];
    size_t n = fread(text, 1, sizeof(text) - 1, f);
    text[n] = 0;
    fclose(f);
    g_fixLayout = ini_int(text, "fix_layout", 1);
    g_fixImeName = ini_int(text, "fix_ime_name", 1);
    g_postLayout = ini_int(text, "post_layout", 1);
    g_fixSlots = ini_int(text, "fix_slots", 1);
    g_fixGate = ini_int(text, "fix_gate", 1);
    g_fixCancel = ini_int(text, "fix_cancel", 1);
    g_fixCharPos = ini_int(text, "fix_charpos", 1);
    g_fixIsIme = ini_int(text, "fix_isime", 1);
    g_imeBridge = ini_int(text, "ime_bridge", 0);
    g_traceC2S = ini_int(text, "trace_c2s", 0);
    ini_wstr(text, "ime_name", g_imeName, 64, L"CINTLGNT.IME");
    g_layoutZhCN = ini_hex(text, "layout_zhcn", 0xE00E0804);
    g_layoutZhTW = ini_hex(text, "layout_zhtw", 0xE0080404);
    char name8[128];
    w2u(g_imeName, name8, sizeof(name8));
    logf_("配置 %s：fix_layout=%d fix_ime_name=%d post_layout=%d fix_slots=%d fix_gate=%d fix_isime=%d "
          "fix_cancel=%d fix_charpos=%d ime_bridge=%d trace_c2s=%d ime_name=%s zhcn=%#lx zhtw=%#lx",
          path, g_fixLayout, g_fixImeName, g_postLayout, g_fixSlots, g_fixGate, g_fixIsIme,
          g_fixCancel, g_fixCharPos, g_imeBridge, g_traceC2S, name8, (unsigned long)g_layoutZhCN,
          (unsigned long)g_layoutZhTW);
}

static BOOL CALLBACK enum_child_cb(HWND h, LPARAM l) {
    (void)l;
    consider_window(h, "scan-child");
    return TRUE;
}

static BOOL CALLBACK enum_top_cb(HWND h, LPARAM l) {
    (void)l;
    DWORD pid = 0;
    GetWindowThreadProcessId(h, &pid);
    if (pid == GetCurrentProcessId()) {
        consider_window(h, "scan-top");
        EnumChildWindows(h, enum_child_cb, 0);
    }
    return TRUE;
}

static DWORD WINAPI probe_thread(LPVOID param) {    (void)param;
    char path[MAX_PATH * 2];
    char dir8[MAX_PATH * 2];
    w2u(g_dir, dir8, sizeof(dir8));
    _snprintf_s(path, sizeof(path), _TRUNCATE, "%s\\chinese-input-probe.log", dir8);
    g_log = fopen(path, "wb");
    if (!g_log) return 1;

    char exe[MAX_PATH * 2] = "";
    wchar_t wexe[MAX_PATH];
    if (GetModuleFileNameW(NULL, wexe, MAX_PATH)) w2u(wexe, exe, sizeof(exe));
    logf_("==== 中文输入探针启动 ====");
    logf_("pid=%lu dll目录=%s", (unsigned long)GetCurrentProcessId(), dir8);
    logf_("客户端 EXE=%s", exe);
    logf_("说明：本探针只观察。请在游戏里用中文输入法在聊天框打字，然后退出游戏把日志发回。");
    /* 我们自己是什么时候进场的：用来判断客户端的门禁是不是"在我们来之前就跑过了" */
    {
        FILETIME ftCreate, ftExit, ftKernel, ftUser, ftNow;
        if (GetProcessTimes(GetCurrentProcess(), &ftCreate, &ftExit, &ftKernel, &ftUser)) {
            ULARGE_INTEGER a, b;
            GetSystemTimeAsFileTime(&ftNow);
            a.LowPart = ftCreate.dwLowDateTime;
            a.HighPart = ftCreate.dwHighDateTime;
            b.LowPart = ftNow.dwLowDateTime;
            b.HighPart = ftNow.dwHighDateTime;
            logf_("插件进场时，客户端进程已运行 %.2f 秒", (double)(b.QuadPart - a.QuadPart) / 1e7);
        }
    }
    load_config();

    /* 主模块镜像大小：用来判断某个 RVA 是不是真的在这个模块里 */
    if (g_mainModule) {
        unsigned char *b = (unsigned char *)g_mainModule;
        int e_lfanew = *(int *)(b + 0x3C);
        /* PE32 与 PE32+ 的 SizeOfImage 都在（OptionalHeader + 56）处 */
        g_exeImageSize = *(unsigned *)(b + e_lfanew + 24 + 56);
        logf_("主模块基址=%p 镜像大小=%#x（门禁 RVA 0x6F22400、Themida 槽 0x9186100 都在里面？%s）",
              (void *)g_mainModule, g_exeImageSize,
              (0x6F22400u < g_exeImageSize && SLOT_HI < g_exeImageSize) ? "是" : "否");
    }

    /* user32 钩子先装（窗口创建时就要能看到） */
    HMODULE u32 = GetModuleHandleW(L"user32.dll");
    if (u32) {
        r_GetProcAddress = (PFN_GPA)GetProcAddress(GetModuleHandleW(L"kernel32.dll"),
                                                   "GetProcAddress");
        HookSpec spec[] = {
            {"CreateWindowExW", (void *)h_CreateWindowExW, NULL},
            {"CreateWindowExA", (void *)h_CreateWindowExA, NULL},
            {"DestroyWindow", (void *)h_DestroyWindow, NULL},
            {"GetKeyboardLayout", (void *)h_GetKeyboardLayout, NULL},
        };
        hook_exports(u32, spec, 4, "user32");
        memcpy(g_specUser32, spec, sizeof(spec));
        g_nUser32 = 4;
        if (g_fixSlots) {
            int n = fixup_import_slots(g_mainModule, g_specUser32, g_nUser32);
            logf_("  Themida 槽修正[user32]：%d 处（槽由 Themida 解包时才填，之后每 200ms 重扫）", n);
        }
        r_CreateWindowExW = (HWND(WINAPI *)(DWORD, LPCWSTR, LPCWSTR, DWORD, int, int, int, int, HWND,
                                            HMENU, HINSTANCE, LPVOID))spec[0].orig;
        r_CreateWindowExA = (HWND(WINAPI *)(DWORD, LPCSTR, LPCSTR, DWORD, int, int, int, int, HWND,
                                            HMENU, HINSTANCE, LPVOID))spec[1].orig;
        r_DestroyWindow = (BOOL(WINAPI *)(HWND))spec[2].orig;
        r_GetKeyboardLayout = (HKL(WINAPI *)(DWORD))spec[3].orig;
    }

    /* 等 imm32 装载（它可能比 dinput8 晚） */
    HMODULE imm = NULL;
    for (int i = 0; i < 600 && !imm; i++) {
        imm = GetModuleHandleW(L"imm32.dll");
        if (!imm) Sleep(50);
    }
    logf_("imm32.dll 基址=%p", (void *)imm);
    if (imm) {
        pImmGetContext = (PFN_ImmGetContext)GetProcAddress(imm, "ImmGetContext");
        pImmReleaseContext = (PFN_ImmReleaseContext)GetProcAddress(imm, "ImmReleaseContext");
        pImmGetCompositionStringW =
            (PFN_ImmGetCompositionStringW)GetProcAddress(imm, "ImmGetCompositionStringW");
        pImmGetOpenStatus = (PFN_ImmGetOpenStatus)GetProcAddress(imm, "ImmGetOpenStatus");
        pImmGetDescriptionW = (PFN_ImmGetDescriptionW)GetProcAddress(imm, "ImmGetDescriptionW");
        pImmIsIME = (PFN_ImmIsIME)GetProcAddress(imm, "ImmIsIME");
        pImmGetIMEFileNameW =
            (PFN_ImmGetIMEFileNameW)GetProcAddress(imm, "ImmGetIMEFileNameW");

        HookSpec spec[] = {
            {"ImmGetContext", (void *)h_ImmGetContext, NULL},
            {"ImmReleaseContext", (void *)h_ImmReleaseContext, NULL},
            {"ImmAssociateContext", (void *)h_ImmAssociateContext, NULL},
            {"ImmAssociateContextEx", (void *)h_ImmAssociateContextEx, NULL},
            {"ImmCreateContext", (void *)h_ImmCreateContext, NULL},
            {"ImmDestroyContext", (void *)h_ImmDestroyContext, NULL},
            {"ImmGetCompositionStringW", (void *)h_ImmGetCompositionStringW, NULL},
            {"ImmSetCompositionStringW", (void *)h_ImmSetCompositionStringW, NULL},
            {"ImmSetCompositionWindow", (void *)h_ImmSetCompositionWindow, NULL},
            {"ImmSetCandidateWindow", (void *)h_ImmSetCandidateWindow, NULL},
            {"ImmNotifyIME", (void *)h_ImmNotifyIME, NULL},
            {"ImmGetOpenStatus", (void *)h_ImmGetOpenStatus, NULL},
            {"ImmSetOpenStatus", (void *)h_ImmSetOpenStatus, NULL},
            {"ImmGetConversionStatus", (void *)h_ImmGetConversionStatus, NULL},
            {"ImmSetConversionStatus", (void *)h_ImmSetConversionStatus, NULL},
            {"ImmLockIMC", (void *)h_ImmLockIMC, NULL},
            {"ImmUnlockIMC", (void *)h_ImmUnlockIMC, NULL},
            {"ImmLockIMCC", (void *)h_ImmLockIMCC, NULL},
            {"ImmUnlockIMCC", (void *)h_ImmUnlockIMCC, NULL},
            {"ImmGetDefaultIMEWnd", (void *)h_ImmGetDefaultIMEWnd, NULL},
            {"ImmIsIME", (void *)h_ImmIsIME, NULL},
            {"ImmGetDescriptionW", (void *)h_ImmGetDescriptionW, NULL},
            {"ImmGetIMEFileNameW", (void *)h_ImmGetIMEFileNameW, NULL},
        };
        const int nspec = (int)(sizeof(spec) / sizeof(spec[0]));
        hook_exports(imm, spec, nspec, "imm32");
        if (nspec <= (int)(sizeof(g_specImm32) / sizeof(g_specImm32[0]))) {
            memcpy(g_specImm32, spec, sizeof(HookSpec) * nspec);
            g_nImm32 = nspec;
        }
        if (g_fixSlots) {
            int n = fixup_import_slots(g_mainModule, g_specImm32, g_nImm32);
            logf_("  Themida 槽修正[imm32]：%d 处（同上，周期性重扫）", n);
        }
        r_ImmGetContext = (PFN_ImmGetContext)spec[0].orig;
        r_ImmReleaseContext = (PFN_ImmReleaseContext)spec[1].orig;
        r_ImmAssociateContext = (PFN_ImmAssociateContext)spec[2].orig;
        r_ImmAssociateContextEx = (PFN_ImmAssociateContextEx)spec[3].orig;
        r_ImmCreateContext = (PFN_ImmCreateContext)spec[4].orig;
        r_ImmDestroyContext = (PFN_ImmDestroyContext)spec[5].orig;
        r_ImmGetCompositionStringW = (PFN_ImmGetCompositionStringW)spec[6].orig;
        r_ImmSetCompositionStringW = (PFN_ImmSetCompositionStringW)spec[7].orig;
        r_ImmSetCompositionWindow = (PFN_ImmSetCompositionWindow)spec[8].orig;
        r_ImmSetCandidateWindow = (PFN_ImmSetCandidateWindow)spec[9].orig;
        r_ImmNotifyIME = (PFN_ImmNotifyIME)spec[10].orig;
        r_ImmGetOpenStatus = (PFN_ImmGetOpenStatus)spec[11].orig;
        r_ImmSetOpenStatus = (PFN_ImmSetOpenStatus)spec[12].orig;
        r_ImmGetConversionStatus = (PFN_ImmGetConversionStatus)spec[13].orig;
        r_ImmSetConversionStatus = (PFN_ImmSetConversionStatus)spec[14].orig;
        r_ImmLockIMC = (PFN_ImmLockIMC)spec[15].orig;
        r_ImmUnlockIMC = (PFN_ImmUnlockIMC)spec[16].orig;
        r_ImmLockIMCC = (PFN_ImmLockIMCC)spec[17].orig;
        r_ImmUnlockIMCC = (PFN_ImmUnlockIMCC)spec[18].orig;
        r_ImmGetDefaultIMEWnd = (PFN_ImmGetDefaultIMEWnd)spec[19].orig;
        r_ImmIsIME = (PFN_ImmIsIME)spec[20].orig;
        r_ImmGetDescriptionW = (PFN_ImmGetDescriptionW)spec[21].orig;
        r_ImmGetIMEFileNameW = (PFN_ImmGetIMEFileNameW)spec[22].orig;
    }

    /* 顺带记录客户端从哪里解析 IME API（只记 imm32 的 Imm*） */
    if (r_GetProcAddress) {
        HMODULE k32 = GetModuleHandleW(L"kernel32.dll");
        HookSpec spec[] = {{"GetProcAddress", (void *)h_GetProcAddress, NULL}};
        hook_exports(k32, spec, 1, "kernel32");
    }

    log_layouts();

    if (g_traceC2S && g_mainModule) {
        /* 只观察客户端自己要发的包；不动内容。 */
        install_c2s_trace(g_mainModule);
    }

    DWORD start = GetTickCount();
    HWND lastFg = NULL;
    char lastComp[512] = "";
    int lastOpen = -1;
    int lastConv = -1;
    DWORD lastSlotScan = 0, lastGateTry = 0, lastGateNote = 0;
    while (GetTickCount() - start < 30 * 60 * 1000) {
        EnumWindows(enum_top_cb, 0);

        /* Themida 槽是解包时才填值的，所以周期性重扫；匹配完就没了（幂等）。 */
        if (g_fixSlots && g_mainModule && GetTickCount() - lastSlotScan > 200) {
            lastSlotScan = GetTickCount();
            int a = fixup_import_slots(g_mainModule, g_specUser32, g_nUser32);
            int b = fixup_import_slots(g_mainModule, g_specImm32, g_nImm32);
            if (a + b) logf_("★ Themida 导入槽修正：user32 %d 处、imm32 %d 处", a, b);
        }
        /* 门禁钩子：等客户端解包到那段代码（字节对得上）才装；最多试 10 分钟。 */
        if (g_fixGate && g_mainModule && !g_gateInstalled && GetTickCount() - lastGateTry > 500) {
            lastGateTry = GetTickCount();
            if (try_install_gate(g_mainModule)) {
                logf_("★ 门禁钩子已装（客户端代码已解包）");
            } else if (GetTickCount() - lastGateNote > 30000) {
                lastGateNote = GetTickCount();
                logf_("门禁钩子还没装上：0x6F22400 处的字节还不是期望值（Themida 未解包？）");
            }
        }
        /* 布局写入点钩子（门禁读的那个字段就是它写的） */
        if (g_fixGate && g_mainModule && !g_setLayoutInstalled &&
            GetTickCount() - lastSlotScan > 500) {
            if (try_install_setter(g_mainModule)) logf_("★ 布局写入钩子已装（0x6F23070）");
        }

        HWND fg = GetForegroundWindow();
        if (fg != lastFg) {
            lastFg = fg;
            log_focus_state();
            lastComp[0] = 0;
            lastOpen = -1;
        }
        /* 轮询前台窗口的输入法状态：组字串/开启状态一变就记一行。
         * 这是"操作系统侧明明在组字、客户端却什么都没发生"的关键证据。 */
        if (fg && pImmGetContext) {
            DWORD pid = 0;
            GetWindowThreadProcessId(fg, &pid);
            if (pid == GetCurrentProcessId()) {
                HIMC_ c = pImmGetContext(fg);
                if (c) {
                    int open = pImmGetOpenStatus ? (int)pImmGetOpenStatus(c) : -1;
                    DWORD conv = 0, sent = 0;
                    int haveConv = r_ImmGetConversionStatus
                                       ? (int)r_ImmGetConversionStatus(c, &conv, &sent) : 0;
                    wchar_t buf[512] = L"";
                    LONG n = pImmGetCompositionStringW
                                 ? pImmGetCompositionStringW(c, GCS_COMPSTR, buf, sizeof(buf) - 2)
                                 : 0;
                    if (n > 0) buf[n / 2] = 0;
                    char u8[1024];
                    w2u(buf, u8, sizeof(u8));
                    if (open != lastOpen || strcmp(u8, lastComp) != 0 ||
                        (haveConv && (int)conv != lastConv)) {
                        /* 转换模式 bit0（IME_CMODE_NATIVE）就是"中文/英文"开关：
                         * 用户以为在打中文、其实是英文模式的话，这里会显示"英文/字母数字"。 */
                        logf_("输入法状态变化：hwnd=%p 开启=%d 组字串=\"%s\" 转换模式=%#lx（%s）",
                              (void *)fg, open, u8, (unsigned long)(haveConv ? conv : 0),
                              !haveConv ? "取不到" : ((conv & 0x1) ? "中文/原生" : "英文/字母数字"));
                        lastOpen = open;
                        lastConv = haveConv ? (int)conv : -1;
                        strncpy_s(lastComp, sizeof(lastComp), u8, _TRUNCATE);
                    }
                    if (pImmReleaseContext) pImmReleaseContext(fg, c);
                }
            }
        }
        Sleep(200);
    }
    logf_("探针巡检结束");
    return 0;
}

/* ------------------------------------------------------------------ */
/* 入口                                                                */
/* ------------------------------------------------------------------ */
extern __declspec(dllexport) DWORD WINAPI StartLocalization(void);

__declspec(dllexport) DWORD WINAPI StartLocalization(void) {
    InitializeCriticalSection(&g_lock);
    g_t0 = GetTickCount();
    g_mainModule = GetModuleHandleW(NULL);
    wchar_t path[MAX_PATH];
    DWORD n = GetModuleFileNameW(g_self, path, MAX_PATH);
    if (n) {
        wcscpy_s(g_dir, MAX_PATH, path);
        for (DWORD i = n; i > 0; i--) {
            if (g_dir[i - 1] == L'\\') { g_dir[i - 1] = 0; break; }
        }
    }
    HANDLE t = CreateThread(NULL, 0, probe_thread, NULL, 0, NULL);
    if (t) CloseHandle(t);
    return 0;
}

/* 插件 ABI（见 client-patchs/client-host）：宿主 qol.client-host 用这两个导出加载本探针。
 * 同时保留 StartLocalization，因此本 DLL 也能直接占用 dinput8 代理那个唯一槽位。 */
__declspec(dllexport) DWORD WINAPI ModStart(void) { return StartLocalization(); }

__declspec(dllexport) const char *WINAPI ModName(void) { return "chinese-input-probe"; }

BOOL WINAPI DllMain(HINSTANCE inst, DWORD reason, LPVOID reserved) {
    (void)reserved;
    if (reason == DLL_PROCESS_ATTACH) {
        g_self = inst;
        DisableThreadLibraryCalls(inst);
    }
    return TRUE;
}
