/*
 * ChineseLocalization.dll —— 客户端 mod 宿主（115US）  版本 1.1.0（ABI 1.1）
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
 *   <客户端目录>\.115us-mods\plugins.json      ← 可选：显式加载清单（见下）
 *
 * ---------------------------------------------------------------------------
 * 插件 ABI（两个入口都可选；1.1 起"都没有"也算加载成功）
 *
 *   DWORD WINAPI ModStart(void);
 *       宿主在**独立线程**里调用它（不在 loader lock 下）。
 *       返回 0 视为成功。宿主只记结果，不因插件失败而中止其它插件。
 *   const char *WINAPI ModName(void);
 *       可选，日志里显示的可读名字。
 *
 *   StartLocalization 同样被认作入口点（同一个 DLL 也能直接占那个唯一槽位）。
 *
 * ---------------------------------------------------------------------------
 * 1.1.0 的四项增强
 *
 *   A 无入口点的插件不再被"跳过"：只 LoadLibrary、登记进表、日志写
 *     "已加载（无 ModStart/StartLocalization 入口点，仅 DllMain）"。靠 DllMain
 *     自举的第三方 DLL 因此不必去抢那个唯一槽位。注意这不改变执行时机——
 *     DllMain 在 LoadLibrary 时就已经跑过了，A 只是把状态从"失败"改成"已加载"。
 *
 *   B 递归子目录：.115us-mods\**\*.dll（深度上限 MAX_DEPTH）。子目录里的
 *     一律**只 LoadLibrary、不调用入口点**，当作依赖；且先于根层插件加载，
 *     这样根层插件起来时依赖已经在了。
 *
 *   C 显式清单 plugins.json：可以指定顺序、启停（enabled）、是否调用入口点
 *     （entrypoint）、依赖（deps）。没有清单时回退到旧行为（单层 + 文件名字典序）。
 *
 *   D 插件间通信：宿主导出 GetPlugin(name) / GetPluginCount / GetPluginNameAt /
 *     GetHostVersion，插件可以拿到彼此的 HMODULE，不再只能靠共享文件协作。
 *     要在自己的 ModStart 里**可靠**拿到对方，必须在清单里声明 deps：
 *     依赖是**同步**加载的，能保证先于依赖方就绪；没声明 deps 的两个插件
 *     各跑各的线程，互相之间没有顺序保证（GetPlugin 可能返回 NULL）。
 *
 * ---------------------------------------------------------------------------
 * 加载顺序（无清单时等价于旧版）：
 *     1) 子目录依赖（递归，只 LoadLibrary）
 *     2) 清单声明项，按 deps 拓扑 + 清单顺序
 *     3) 清单没声明、但扫描到的根层插件（按文件名字典序）
 *
 * 宿主只加载、不卸载：插件一旦加载就固定到进程结束（GetModuleHandleExW PIN）。
 * 日志写宿主自己所在目录（AGENTS §0 第 9 条）。
 */

#define WIN32_LEAN_AND_MEAN
#define _CRT_SECURE_NO_WARNINGS
#include <windows.h>
#include <stdio.h>
#include <stdarg.h>
#include <stdlib.h>
#include <string.h>

#define PLUGIN_DIR L".115us-mods"
#define MANIFEST_NAME "plugins.json"
#define MAX_PLUGINS 64
#define MAX_SPEC 64
#define MAX_DEPS 8
#define MAX_DEPTH 3 /* B：.115us-mods 自身算第 0 层 */
#define HOST_ABI_VERSION 0x00010001u /* 1.1 */

/* ---------------------------------------------------------------- 基础设施 */

static HMODULE g_self;
static wchar_t g_dir[MAX_PATH];
static FILE *g_log;
static CRITICAL_SECTION g_lock;
static DWORD g_t0;

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

static void u2w(const char *src, wchar_t *dst, size_t cap) {
    if (!src) { dst[0] = 0; return; }
    int n = MultiByteToWideChar(CP_UTF8, 0, src, -1, dst, (int)cap - 1);
    dst[n > 0 ? n : 0] = 0;
}

/* 取文件名主干（去目录、去扩展名），用于 D 的默认名 */
static void stem_of(const wchar_t *path, char *out, size_t cap) {
    const wchar_t *base = path;
    const wchar_t *p;
    size_t len;
    wchar_t tmp[MAX_PATH];
    for (p = path; *p; p++) {
        if (*p == L'\\' || *p == L'/') base = p + 1;
    }
    len = wcslen(base);
    for (p = base + len; p > base; p--) {
        if (*(p - 1) == L'.') { len = (size_t)(p - base) - 1; break; }
    }
    if (len >= MAX_PATH) len = MAX_PATH - 1;
    memcpy(tmp, base, len * sizeof(wchar_t));
    tmp[len] = 0;
    w2u(tmp, out, cap);
}

/* ------------------------------------------------------------ 插件登记表(D) */

typedef struct {
    char name[128];  /* 清单里的 name，或文件名主干 */
    char stem[128];  /* 文件名主干，用于按文件名查 */
    HMODULE handle;
    int called_entry;
} PluginRecord;

static PluginRecord g_records[MAX_PLUGINS];
static int g_nrecords;

static void reg_add(const char *name, const char *stem, HMODULE h, int called) {
    EnterCriticalSection(&g_lock);
    if (g_nrecords < MAX_PLUGINS) {
        PluginRecord *r = &g_records[g_nrecords++];
        _snprintf_s(r->name, sizeof(r->name), _TRUNCATE, "%s", name ? name : "");
        _snprintf_s(r->stem, sizeof(r->stem), _TRUNCATE, "%s", stem ? stem : "");
        r->handle = h;
        r->called_entry = called;
    }
    LeaveCriticalSection(&g_lock);
}

/* D：插件按名字取另一个插件的 HMODULE */
__declspec(dllexport) HMODULE WINAPI GetPlugin(const char *name) {
    HMODULE found = NULL;
    int i;
    if (!name || !*name) return NULL;
    EnterCriticalSection(&g_lock);
    for (i = 0; i < g_nrecords && !found; i++) {
        if (_stricmp(g_records[i].name, name) == 0 ||
            _stricmp(g_records[i].stem, name) == 0) {
            found = g_records[i].handle;
        }
    }
    LeaveCriticalSection(&g_lock);
    return found;
}

__declspec(dllexport) DWORD WINAPI GetPluginCount(void) {
    DWORD n;
    EnterCriticalSection(&g_lock);
    n = (DWORD)g_nrecords;
    LeaveCriticalSection(&g_lock);
    return n;
}

__declspec(dllexport) const char *WINAPI GetPluginNameAt(DWORD index) {
    static char slots[MAX_PLUGINS][128];
    const char *r = NULL;
    EnterCriticalSection(&g_lock);
    if (index < (DWORD)g_nrecords) {
        _snprintf_s(slots[index], sizeof(slots[index]), _TRUNCATE, "%s",
                    g_records[index].name);
        r = slots[index];
    }
    LeaveCriticalSection(&g_lock);
    return r;
}

/* D：插件据此判断宿主有没有某项能力，比猜导出是否存在可靠 */
__declspec(dllexport) DWORD WINAPI GetHostVersion(void) { return HOST_ABI_VERSION; }

/* ------------------------------------------------------------------ 清单(C) */

typedef struct {
    char name[128];
    char path[MAX_PATH]; /* 相对 .115us-mods 的路径，UTF-8，允许 / 或 \ */
    int enabled;
    int entrypoint; /* 是否调用 ModStart/StartLocalization */
    char deps[MAX_DEPS][128];
    int ndeps;
    int declared; /* 清单里确实出现过这一项 */
    int loaded;   /* 依赖解析用 */
} PluginSpec;

typedef struct {
    PluginSpec specs[MAX_SPEC];
    int n;
    int auto_discover;
    int recurse;
    int have; /* 是否读到了清单 */
} Manifest;

/* —— 极简 JSON 读取：只解析本宿主关心的形状，其它一律跳过 —— */

static const char *skip_ws(const char *p) {
    while (*p == ' ' || *p == '\t' || *p == '\r' || *p == '\n') p++;
    return p;
}

static const char *skip_string(const char *p) {
    if (*p != '"') return p;
    p++;
    while (*p && *p != '"') {
        if (*p == '\\' && p[1]) p += 2;
        else p++;
    }
    if (*p == '"') p++;
    return p;
}

static const char *skip_value(const char *p, int depth) {
    char open;
    char close;
    if (depth > 8) return p;
    p = skip_ws(p);
    if (*p == '"') return skip_string(p);
    if (*p == '{' || *p == '[') {
        open = *p;
        close = (open == '{') ? '}' : ']';
        p++;
        for (;;) {
            p = skip_ws(p);
            if (!*p) return p;
            if (*p == close) return p + 1;
            if (*p == ',') { p++; continue; }
            if (open == '{') {
                if (*p != '"') return p; /* 期望 key */
                p = skip_string(p);
                p = skip_ws(p);
                if (*p != ':') return p;
                p++;
                p = skip_value(p, depth + 1);
            } else {
                p = skip_value(p, depth + 1);
            }
        }
    }
    while (*p && *p != ',' && *p != '}' && *p != ']' && *p != ' ' && *p != '\t' &&
           *p != '\r' && *p != '\n')
        p++;
    return p;
}

static void put_utf8(char *out, size_t cap, size_t *n, unsigned cp) {
    if (cp < 0x80) {
        if (*n + 1 < cap) out[(*n)++] = (char)cp;
    } else if (cp < 0x800) {
        if (*n + 2 < cap) {
            out[(*n)++] = (char)(0xC0 | (cp >> 6));
            out[(*n)++] = (char)(0x80 | (cp & 0x3F));
        }
    } else {
        if (*n + 3 < cap) {
            out[(*n)++] = (char)(0xE0 | (cp >> 12));
            out[(*n)++] = (char)(0x80 | ((cp >> 6) & 0x3F));
            out[(*n)++] = (char)(0x80 | (cp & 0x3F));
        }
    }
}

static int read_string(const char *p, char *out, size_t cap, const char **end) {
    size_t n = 0;
    p = skip_ws(p);
    if (*p != '"') { *end = p; out[0] = 0; return 0; }
    p++;
    while (*p && *p != '"') {
        if (*p == '\\' && p[1]) {
            p++;
            switch (*p) {
                case 'n': put_utf8(out, cap, &n, '\n'); p++; break;
                case 't': put_utf8(out, cap, &n, '\t'); p++; break;
                case 'r': put_utf8(out, cap, &n, '\r'); p++; break;
                case 'b': put_utf8(out, cap, &n, '\b'); p++; break;
                case 'f': put_utf8(out, cap, &n, '\f'); p++; break;
                case 'u': {
                    unsigned cp = 0;
                    int i;
                    p++;
                    for (i = 0; i < 4 && *p; i++, p++) {
                        char c = *p;
                        unsigned d = 0;
                        if (c >= '0' && c <= '9') d = (unsigned)(c - '0');
                        else if (c >= 'a' && c <= 'f') d = (unsigned)(c - 'a' + 10);
                        else if (c >= 'A' && c <= 'F') d = (unsigned)(c - 'A' + 10);
                        else break;
                        cp = cp * 16 + d;
                    }
                    put_utf8(out, cap, &n, cp);
                    break;
                }
                default: put_utf8(out, cap, &n, (unsigned char)*p); p++; break;
            }
        } else {
            put_utf8(out, cap, &n, (unsigned char)*p);
            p++;
        }
    }
    if (*p == '"') p++;
    out[n] = 0;
    *end = p;
    return 1;
}

static int read_bool(const char *p, int deflt, const char **end) {
    p = skip_ws(p);
    if (strncmp(p, "true", 4) == 0) { *end = skip_value(p, 0); return 1; }
    if (strncmp(p, "false", 5) == 0) { *end = skip_value(p, 0); return 0; }
    *end = p;
    return deflt;
}

/* 读一个 key，命中返回 1 并把游标移到 value 起点 */
static int seek_key(const char **pp, const char *key) {
    const char *p = skip_ws(*pp);
    const char *after;
    char k[64];
    if (*p != '"') return 0;
    if (!read_string(p, k, sizeof(k), &after)) return 0;
    if (_stricmp(k, key) != 0) return 0;
    p = skip_ws(after);
    if (*p != ':') return 0;
    *pp = p + 1;
    return 1;
}

static void parse_spec(const char **pp, PluginSpec *s) {
    const char *p = skip_ws(*pp);
    s->enabled = 1;
    s->entrypoint = 1;
    s->declared = 1;
    s->ndeps = 0;
    s->loaded = 0;
    if (*p != '{') { *pp = skip_value(p, 0); return; }
    p++;
    for (;;) {
        const char *cur;
        p = skip_ws(p);
        if (!*p || *p == '}') { if (*p == '}') p++; break; }
        if (*p == ',') { p++; continue; }
        cur = p;
        if (seek_key(&cur, "name")) {
            read_string(cur, s->name, sizeof(s->name), &p);
        } else if (seek_key(&cur, "path")) {
            read_string(cur, s->path, sizeof(s->path), &p);
        } else if (seek_key(&cur, "enabled")) {
            s->enabled = read_bool(cur, 1, &p);
        } else if (seek_key(&cur, "entrypoint")) {
            s->entrypoint = read_bool(cur, 1, &p);
        } else if (seek_key(&cur, "deps")) {
            const char *q = skip_ws(cur);
            if (*q == '[') {
                q++;
                for (;;) {
                    q = skip_ws(q);
                    if (!*q || *q == ']') { if (*q == ']') q++; break; }
                    if (*q == ',') { q++; continue; }
                    if (s->ndeps < MAX_DEPS) {
                        read_string(q, s->deps[s->ndeps], sizeof(s->deps[0]), &q);
                        s->ndeps++;
                    } else {
                        q = skip_value(q, 0);
                    }
                }
            } else {
                q = skip_value(q, 0);
            }
            p = q;
        } else {
            /* 不认识的 key：整对跳掉 */
            const char *q = skip_string(p);
            q = skip_ws(q);
            if (*q == ':') q++;
            p = skip_value(q, 0);
        }
    }
    *pp = p;
}

static int load_manifest(const wchar_t *dir, Manifest *m) {
    wchar_t path[MAX_PATH];
    char *buf;
    long size = 0;
    FILE *f;

    m->n = 0;
    m->have = 0;
    m->auto_discover = 1;
    m->recurse = 1;

    _snwprintf_s(path, MAX_PATH, _TRUNCATE, L"%s\\%s\\%hs", dir, PLUGIN_DIR, MANIFEST_NAME);
    f = _wfopen(path, L"rb");
    if (!f) {
        logf_("清单 plugins.json 不存在，回退旧行为（单层自动发现 + 递归依赖）");
        return 0;
    }
    fseek(f, 0, SEEK_END);
    size = ftell(f);
    fseek(f, 0, SEEK_SET);
    if (size <= 0 || size > (1 << 20)) {
        fclose(f);
        logf_("清单 plugins.json 大小异常（%ld 字节），忽略", size);
        return 0;
    }
    buf = (char *)malloc((size_t)size + 1);
    if (!buf) { fclose(f); return 0; }
    if (fread(buf, 1, (size_t)size, f) != (size_t)size) {
        free(buf);
        fclose(f);
        logf_("清单 plugins.json 读取失败，忽略");
        return 0;
    }
    buf[size] = 0;
    fclose(f);

    {
        const char *p = skip_ws(buf);
        if (*p == '{') {
            p++;
            for (;;) {
                const char *cur;
                p = skip_ws(p);
                if (!*p || *p == '}') { if (*p == '}') p++; break; }
                if (*p == ',') { p++; continue; }
                cur = p;
                if (seek_key(&cur, "plugins")) {
                    const char *q = skip_ws(cur);
                    if (*q == '[') {
                        q++;
                        for (;;) {
                            q = skip_ws(q);
                            if (!*q || *q == ']') { if (*q == ']') q++; break; }
                            if (*q == ',') { q++; continue; }
                            if (m->n < MAX_SPEC) {
                                parse_spec(&q, &m->specs[m->n]);
                                m->n++;
                            } else {
                                q = skip_value(q, 0);
                            }
                        }
                    } else {
                        q = skip_value(q, 0);
                    }
                    p = q;
                } else if (seek_key(&cur, "autoDiscover")) {
                    m->auto_discover = read_bool(cur, 1, &p);
                } else if (seek_key(&cur, "recurse")) {
                    m->recurse = read_bool(cur, 1, &p);
                } else {
                    const char *q = skip_string(p);
                    q = skip_ws(q);
                    if (*q == ':') q++;
                    p = skip_value(q, 0);
                }
            }
        }
    }
    free(buf);
    m->have = 1;
    logf_("读到清单 plugins.json：%d 条声明，autoDiscover=%d，recurse=%d", m->n,
          m->auto_discover, m->recurse);
    return 1;
}

/* --------------------------------------------------------------- 加载插件 */

typedef struct {
    wchar_t path[MAX_PATH];
    char name[128];
    char stem[128];
    int call_entry;
} PluginJob;

typedef DWORD(WINAPI *ModStartFn)(void);
typedef const char *(WINAPI *ModNameFn)(void);

static DWORD WINAPI run_plugin(LPVOID param) {
    PluginJob *job = (PluginJob *)param;
    char u8[MAX_PATH];
    char disp[288];
    char pretty[256];
    HMODULE h;
    HMODULE pinned = NULL;
    ModNameFn namefn;
    ModStartFn start;

    w2u(job->path, u8, sizeof(u8));
    h = LoadLibraryW(job->path);
    if (!h) {
        logf_("插件加载失败（LoadLibrary 错误 %lu）：%s", (unsigned long)GetLastError(), u8);
        HeapFree(GetProcessHeap(), 0, job);
        return 1;
    }
    /* 固定生命周期：插件一旦进来就不允许被卸载。 */
    GetModuleHandleExW(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS |
                           GET_MODULE_HANDLE_EX_FLAG_PIN,
                       (LPCWSTR)h, &pinned);

    pretty[0] = 0;
    namefn = (ModNameFn)GetProcAddress(h, "ModName");
    if (namefn) {
        const char *n = namefn();
        if (n) _snprintf_s(pretty, sizeof(pretty), _TRUNCATE, "%s", n);
    }
    _snprintf_s(disp, sizeof(disp), _TRUNCATE, "%s%s%s%s", job->name,
                pretty[0] ? "（" : "", pretty[0] ? pretty : "", pretty[0] ? "）" : "");

    if (!job->call_entry) {
        /* B：子目录依赖 / 清单 entrypoint=false —— 只加载，不调用入口点 */
        logf_("依赖已加载（不调用入口点）：%s", disp);
        reg_add(job->name, job->stem, h, 0);
        HeapFree(GetProcessHeap(), 0, job);
        return 0;
    }

    start = (ModStartFn)GetProcAddress(h, "ModStart");
    if (!start) start = (ModStartFn)GetProcAddress(h, "StartLocalization");
    if (!start) {
        /* A：没有入口点也算加载成功。DllMain 已经在 LoadLibrary 里跑过了，
         * 这里只是把状态从"跳过/失败"改成"已加载"，不额外执行任何代码。 */
        logf_("已加载（无 ModStart/StartLocalization 入口点，仅 DllMain）：%s", disp);
        reg_add(job->name, job->stem, h, 0);
        HeapFree(GetProcessHeap(), 0, job);
        return 0;
    }
    logf_("插件启动：%s", disp);
    {
        DWORD rc = start();
        logf_("插件返回 %lu：%s", (unsigned long)rc, u8);
    }
    reg_add(job->name, job->stem, h, 1);
    HeapFree(GetProcessHeap(), 0, job);
    return 0;
}

static int cmp_plugins(const void *a, const void *b) {
    return _wcsicmp(((const WIN32_FIND_DATAW *)a)->cFileName,
                    ((const WIN32_FIND_DATAW *)b)->cFileName);
}

/*
 * sync=1 时同步加载（不另起线程）。依赖必须用同步方式加载，否则
 * "依赖已加载"这件事和"依赖方开始跑"之间没有顺序保证 —— 那样 D 的
 * GetPlugin 会时灵时不灵。普通插件仍走独立线程（不在 loader lock 下调用
 * ModStart，且一个插件卡住不影响其它插件）。
 */
static void spawn_plugin(const wchar_t *fullpath, const char *name, int call_entry,
                         int sync) {
    PluginJob *job = (PluginJob *)HeapAlloc(GetProcessHeap(), HEAP_ZERO_MEMORY, sizeof(*job));
    HANDLE t;
    if (!job) {
        char u8[MAX_PATH];
        w2u(fullpath, u8, sizeof(u8));
        logf_("内存不足，跳过：%s", u8);
        return;
    }
    _snwprintf_s(job->path, MAX_PATH, _TRUNCATE, L"%s", fullpath);
    _snprintf_s(job->name, sizeof(job->name), _TRUNCATE, "%s", name);
    stem_of(fullpath, job->stem, sizeof(job->stem));
    job->call_entry = call_entry;
    if (sync) {
        run_plugin(job); /* run_plugin 自己会释放 job */
        return;
    }
    t = CreateThread(NULL, 0, run_plugin, job, 0, NULL);
    if (!t) {
        char u8[MAX_PATH];
        w2u(fullpath, u8, sizeof(u8));
        logf_("无法为插件起线程：%s", u8);
        HeapFree(GetProcessHeap(), 0, job);
        return;
    }
    CloseHandle(t);
}

/* 扫描一层目录里的 *.dll，按文件名字典序 */
static int scan_dir_dlls(const wchar_t *pattern, WIN32_FIND_DATAW *out, int max) {
    WIN32_FIND_DATAW fd;
    HANDLE find;
    int n = 0;
    find = FindFirstFileW(pattern, &fd);
    if (find == INVALID_HANDLE_VALUE) return 0;
    do {
        if (fd.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY) continue;
        if (n < max) out[n++] = fd;
    } while (FindNextFileW(find, &fd));
    FindClose(find);
    if (n > 1) qsort(out, (size_t)n, sizeof(out[0]), cmp_plugins);
    return n;
}

static int ends_with_dll(const wchar_t *s) {
    size_t n = wcslen(s);
    return n >= 4 && _wcsicmp(s + n - 4, L".dll") == 0;
}

/* B：递归加载子目录依赖。只 LoadLibrary，不调用入口点。 */
static void load_dependencies(const wchar_t *base, int recurse) {
    wchar_t stack_dirs[64][MAX_PATH];
    int stack_depth[64];
    int top;
    int total = 0;

    if (!recurse) return;
    _snwprintf_s(stack_dirs[0], MAX_PATH, _TRUNCATE, L"%s", base);
    stack_depth[0] = 0;
    top = 1;

    while (top > 0) {
        wchar_t dir[MAX_PATH];
        wchar_t subpat[MAX_PATH];
        wchar_t dllpat[MAX_PATH];
        WIN32_FIND_DATAW fd;
        WIN32_FIND_DATAW found[MAX_PLUGINS];
        HANDLE find;
        int depth;
        int n;
        int i;

        top--;
        depth = stack_depth[top];
        _snwprintf_s(dir, MAX_PATH, _TRUNCATE, L"%s", stack_dirs[top]);
        if (depth >= MAX_DEPTH) continue;

        /* 先把子目录压栈 */
        _snwprintf_s(subpat, MAX_PATH, _TRUNCATE, L"%s\\*", dir);
        find = FindFirstFileW(subpat, &fd);
        if (find != INVALID_HANDLE_VALUE) {
            do {
                if (!(fd.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY)) continue;
                if (fd.cFileName[0] == L'.') continue; /* 跳过 . / .. / 隐藏目录 */
                if (top < 64) {
                    _snwprintf_s(stack_dirs[top], MAX_PATH, _TRUNCATE, L"%s\\%s", dir,
                                 fd.cFileName);
                    stack_depth[top] = depth + 1;
                    top++;
                }
            } while (FindNextFileW(find, &fd));
            FindClose(find);
        }

        /* 再加载这一层目录里的 dll（跳过 .115us-mods 根层本身） */
        if (depth == 0) continue;
        _snwprintf_s(dllpat, MAX_PATH, _TRUNCATE, L"%s\\*.dll", dir);
        n = scan_dir_dlls(dllpat, found, MAX_PLUGINS);
        for (i = 0; i < n; i++) {
            wchar_t full[MAX_PATH];
            char stem[128];
            if (!ends_with_dll(found[i].cFileName)) continue;
            _snwprintf_s(full, MAX_PATH, _TRUNCATE, L"%s\\%s", dir, found[i].cFileName);
            stem_of(full, stem, sizeof(stem));
            spawn_plugin(full, stem, 0, 1); /* 依赖：同步加载，不调用入口点 */
            total++;
            if (total >= MAX_PLUGINS) break;
        }
        if (total >= MAX_PLUGINS) break;
    }
    if (total) logf_("子目录依赖：共 %d 个，已全部 LoadLibrary（不调用入口点）", total);
}

static PluginSpec *find_spec(Manifest *m, const char *name) {
    int i;
    for (i = 0; i < m->n; i++) {
        if (_stricmp(m->specs[i].name, name) == 0) return &m->specs[i];
    }
    return NULL;
}

static void load_spec(Manifest *m, PluginSpec *s, int depth) {
    wchar_t full[MAX_PATH];
    wchar_t rel[MAX_PATH];
    char stem[128];
    int i;

    if (depth > 4 || !s || s->loaded) return;
    s->loaded = 1;

    /* 先按 deps 递归加载依赖 */
    for (i = 0; i < s->ndeps; i++) {
        PluginSpec *d = find_spec(m, s->deps[i]);
        if (d && !d->loaded) {
            logf_("按依赖先行加载：%s <- %s", s->name, d->name);
            load_spec(m, d, depth + 1);
        }
    }

    if (!s->enabled) {
        logf_("清单已禁用，跳过：%s", s->name[0] ? s->name : s->path);
        return;
    }
    if (!s->path[0]) {
        logf_("清单项缺少 path，跳过：%s", s->name);
        return;
    }
    u2w(s->path, rel, MAX_PATH);
    for (i = 0; rel[i]; i++) {
        if (rel[i] == L'/') rel[i] = L'\\';
    }
    _snwprintf_s(full, MAX_PATH, _TRUNCATE, L"%s\\%s\\%s", g_dir, PLUGIN_DIR, rel);
    stem_of(full, stem, sizeof(stem));
    if (!s->name[0]) _snprintf_s(s->name, sizeof(s->name), _TRUNCATE, "%s", stem);
    /* depth>0 表示本项是别人声明的依赖 ⇒ 同步加载，保证顺序 */
    spawn_plugin(full, s->name, s->entrypoint, depth > 0 ? 1 : 0);
}

static int already_declared(Manifest *m, const char *path8, const char *stem) {
    int k;
    for (k = 0; k < m->n; k++) {
        if (_stricmp(m->specs[k].path, path8) == 0) return 1;
        if (m->specs[k].name[0] && _stricmp(m->specs[k].name, stem) == 0) return 1;
    }
    return 0;
}

/* ------------------------------------------------------------------ 主流程 */

static void load_plugins(void) {
    Manifest m;
    wchar_t root[MAX_PATH];
    wchar_t pattern[MAX_PATH];
    WIN32_FIND_DATAW found[MAX_PLUGINS];
    int n;
    int extra = 0;
    int i;

    _snwprintf_s(root, MAX_PATH, _TRUNCATE, L"%s\\%s", g_dir, PLUGIN_DIR);
    load_manifest(g_dir, &m);

    /* 1) B：子目录依赖先加载（只 LoadLibrary） */
    load_dependencies(root, m.recurse);

    /* 2) C：清单声明项，按 deps 拓扑 + 清单顺序 */
    if (m.have) {
        for (i = 0; i < m.n; i++) load_spec(&m, &m.specs[i], 0);
    }

    /* 3) 自动发现：单层 *.dll，清单没声明过的才补 */
    _snwprintf_s(pattern, MAX_PATH, _TRUNCATE, L"%s\\%s\\*.dll", g_dir, PLUGIN_DIR);
    n = scan_dir_dlls(pattern, found, MAX_PLUGINS);
    for (i = 0; i < n; i++) {
        char stem[128];
        char path8[MAX_PATH];
        wchar_t full[MAX_PATH];
        if (!ends_with_dll(found[i].cFileName)) continue;
        _snwprintf_s(full, MAX_PATH, _TRUNCATE, L"%s\\%s\\%s", g_dir, PLUGIN_DIR,
                     found[i].cFileName);
        stem_of(full, stem, sizeof(stem));
        w2u(found[i].cFileName, path8, sizeof(path8));
        if (already_declared(&m, path8, stem)) continue;
        if (m.have && !m.auto_discover) {
            logf_("清单未声明且 autoDiscover=false，跳过：%s", path8);
            continue;
        }
        spawn_plugin(full, stem, 1, 0);
        extra++;
    }
    logf_("自动发现（清单外）：%d 个", extra);
}

__declspec(dllexport) DWORD WINAPI StartLocalization(void) {
    char dir8[MAX_PATH];
    wchar_t path[MAX_PATH];
    char logpath[MAX_PATH * 2];
    DWORD n;

    g_t0 = GetTickCount();

    n = GetModuleFileNameW(g_self, path, MAX_PATH);
    if (n) {
        DWORD i;
        wcscpy_s(g_dir, MAX_PATH, path);
        for (i = n; i > 0; i--) {
            if (g_dir[i - 1] == L'\\') { g_dir[i - 1] = 0; break; }
        }
    }
    w2u(g_dir, dir8, sizeof(dir8));
    _snprintf_s(logpath, sizeof(logpath), _TRUNCATE, "%s\\client-host.log", dir8);
    g_log = fopen(logpath, "ab");
    if (!g_log) return 1;

    logf_("==== 115US 客户端 mod 宿主启动（1.1.0 / ABI 0x%08x）====",
          (unsigned)HOST_ABI_VERSION);
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
        /* 插件的 DllMain 可能在宿主 StartLocalization 之前就回调 GetPlugin，
         * 所以锁要在 attach 阶段就可用。 */
        InitializeCriticalSection(&g_lock);
        DisableThreadLibraryCalls(inst);
    } else if (reason == DLL_PROCESS_DETACH) {
        DeleteCriticalSection(&g_lock);
    }
    return TRUE;
}
