/*
 * ChineseLocalization.dll —— 客户端 mod 宿主（115US）
 *
 * 为什么需要它：
 *   客户端自带的 dinput8.dll 代理只会加载**一个**固定名字的 DLL
 *   （<客户端目录>\ChineseLocalization.dll）并调用它的 StartLocalization。
 *   而 modkit 的 client 层是"同一目标路径两个 mod 冲突即拒绝"，
 *   所以第二个客户端 mod 没有地方可落。
 *
 * 本宿主把那个唯一的位置变成**插件目录**：
 *
 *   <客户端目录>\ChineseLocalization.dll       ← 本宿主（由 qol.client-host 落位）
 *   <客户端目录>\.115us-mods\*.dll             ← 各客户端 mod 的插件（file.add 落位）
 *
 * 插件 ABI（两个导出都可选，但至少要有一个入口）：
 *
 *   DWORD WINAPI ModStart(void);
 *       宿主在**独立线程**里调用它（不在 loader lock 下）。
 *       返回 0 视为成功。宿主只记结果，不因插件失败而中止其它插件。
 *   const char *ModName(void);
 *       可选，日志里显示的可读名字。
 *
 * 插件按**文件名字典序**加载，顺序稳定可预期。
 * 宿主只加载、不卸载：插件一旦加载就固定到进程结束（GetModuleHandleExW PIN）。
 * 日志写宿主自己所在目录（AGENTS §0 第 9 条）。
 */

#define WIN32_LEAN_AND_MEAN
#define _CRT_SECURE_NO_WARNINGS
#include <windows.h>
#include <stdio.h>
#include <stdarg.h>
#include <stdlib.h>

#define PLUGIN_DIR L".115us-mods"
#define MAX_PLUGINS 64

static HMODULE g_self;
static wchar_t g_dir[MAX_PATH];
static FILE *g_log;
static CRITICAL_SECTION g_lock;
static DWORD g_t0;

typedef DWORD(WINAPI *ModStartFn)(void);
typedef const char *(WINAPI *ModNameFn)(void);

static void logf_(const char *fmt, ...) {
    char buf[1024];
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

static void w2u(const wchar_t *src, char *dst, size_t cap) {
    if (!src) { dst[0] = 0; return; }
    int n = WideCharToMultiByte(CP_UTF8, 0, src, -1, dst, (int)cap - 1, NULL, NULL);
    dst[n > 0 ? n : 0] = 0;
}

typedef struct {
    wchar_t path[MAX_PATH];
} PluginJob;

static DWORD WINAPI run_plugin(LPVOID param) {
    PluginJob *job = (PluginJob *)param;
    char u8[MAX_PATH];
    w2u(job->path, u8, sizeof(u8));
    HMODULE h = LoadLibraryW(job->path);
    if (!h) {
        logf_("插件加载失败（LoadLibrary 错误 %lu）：%s", (unsigned long)GetLastError(), u8);
        HeapFree(GetProcessHeap(), 0, job);
        return 1;
    }
    /* 固定生命周期：插件一旦进来就不允许被卸载。 */
    HMODULE pinned = NULL;
    GetModuleHandleExW(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS |
                           GET_MODULE_HANDLE_EX_FLAG_PIN,
                       (LPCWSTR)h, &pinned);
    ModNameFn name = (ModNameFn)GetProcAddress(h, "ModName");
    char pretty[256] = "";
    if (name) {
        const char *n = name();
        if (n) _snprintf_s(pretty, sizeof(pretty), _TRUNCATE, "%s", n);
    }
    ModStartFn start = (ModStartFn)GetProcAddress(h, "ModStart");
    if (!start) start = (ModStartFn)GetProcAddress(h, "StartLocalization");
    if (!start) {
        logf_("插件没有 ModStart 导出，已跳过：%s", u8);
        HeapFree(GetProcessHeap(), 0, job);
        return 1;
    }
    logf_("插件启动：%s%s%s%s", u8, pretty[0] ? "（" : "", pretty[0] ? pretty : "",
          pretty[0] ? "）" : "");
    DWORD rc = start();
    logf_("插件返回 %lu：%s", (unsigned long)rc, u8);
    HeapFree(GetProcessHeap(), 0, job);
    return 0;
}

static int cmp_plugins(const void *a, const void *b) {
    return _wcsicmp(((const WIN32_FIND_DATAW *)a)->cFileName,
                    ((const WIN32_FIND_DATAW *)b)->cFileName);
}

static void load_plugins(void) {
    wchar_t pattern[MAX_PATH];
    _snwprintf_s(pattern, MAX_PATH, _TRUNCATE, L"%s\\%s\\*.dll", g_dir, PLUGIN_DIR);
    char u8[MAX_PATH];
    w2u(pattern, u8, sizeof(u8));
    WIN32_FIND_DATAW fd;
    HANDLE find = FindFirstFileW(pattern, &fd);
    if (find == INVALID_HANDLE_VALUE) {
        logf_("插件目录为空或不存在（正常）：%s", u8);
        return;
    }
    WIN32_FIND_DATAW found[MAX_PLUGINS];
    int n = 0;
    do {
        if (fd.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY) continue;
        if (n < MAX_PLUGINS) found[n++] = fd;
    } while (FindNextFileW(find, &fd));
    FindClose(find);
    qsort(found, (size_t)n, sizeof(found[0]), cmp_plugins);

    logf_("发现 %d 个插件，按文件名顺序加载", n);
    for (int i = 0; i < n; i++) {
        PluginJob *job = (PluginJob *)HeapAlloc(GetProcessHeap(), HEAP_ZERO_MEMORY, sizeof(*job));
        if (!job) break;
        _snwprintf_s(job->path, MAX_PATH, _TRUNCATE, L"%s\\%s\\%s", g_dir, PLUGIN_DIR,
                     found[i].cFileName);
        HANDLE t = CreateThread(NULL, 0, run_plugin, job, 0, NULL);
        if (!t) {
            logf_("无法为插件起线程：%ls", found[i].cFileName);
            HeapFree(GetProcessHeap(), 0, job);
            continue;
        }
        CloseHandle(t);
    }
}

__declspec(dllexport) DWORD WINAPI StartLocalization(void) {
    g_t0 = GetTickCount();
    InitializeCriticalSection(&g_lock);

    char dir8[MAX_PATH];
    wchar_t path[MAX_PATH];
    DWORD n = GetModuleFileNameW(g_self, path, MAX_PATH);
    if (n) {
        wcscpy_s(g_dir, MAX_PATH, path);
        for (DWORD i = n; i > 0; i--) {
            if (g_dir[i - 1] == L'\\') { g_dir[i - 1] = 0; break; }
        }
    }
    w2u(g_dir, dir8, sizeof(dir8));
    char logpath[MAX_PATH * 2];
    _snprintf_s(logpath, sizeof(logpath), _TRUNCATE, "%s\\client-host.log", dir8);
    g_log = fopen(logpath, "ab");
    if (!g_log) return 1;

    logf_("==== 115US 客户端 mod 宿主启动 ====");
    logf_("pid=%lu 宿主目录=%s", (unsigned long)GetCurrentProcessId(), dir8);
    logf_("插件目录=%s\\%s", dir8, ".115us-mods");
    load_plugins();
    logf_("宿主初始化结束（插件各自在本目录写自己的日志）");
    return 0;
}

BOOL WINAPI DllMain(HINSTANCE inst, DWORD reason, LPVOID reserved) {
    (void)reserved;
    if (reason == DLL_PROCESS_ATTACH) {
        g_self = inst;
        DisableThreadLibraryCalls(inst);
    }
    return TRUE;
}
