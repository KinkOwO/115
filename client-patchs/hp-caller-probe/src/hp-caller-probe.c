/*
 * hp-caller-probe -- DFO 115us runtime probe: which call site computes the
 * monster HP value?
 *
 * WHAT IT DOES (read-only probe, see ../README.md for the full argument)
 * ---------------------------------------------------------------------
 * It installs a single 5-byte rel32 jump over the `ret` that ends the HP
 * getter at VA 0x147221011:
 *
 *      VA          file        before              after
 *      0x147221011 0x7221011   C3 CC CC CC CC      E9 <disp32>
 *
 * The four bytes after the `ret` are 0xCC inter-function padding; the getter
 * never executes beyond the `ret`, and the next function starts at
 * 0x147221020, so the 5 patched bytes are exactly "ret + 4 filler bytes".
 *
 * The jump lands in a freshly VirtualAlloc'ed page (so no existing byte of
 * DFO.exe is reused and no static branch target can collide with it).  The
 * page holds the trampoline from src/probe-hook.s: it saves the registers a
 * Windows x64 call must preserve, hands {caller return address, rax, rcx,
 * rdx} to probe_hp_entry(), and jumps back to the instruction the original
 * `ret` would have returned to.
 *
 * The return value (rax) is never modified; the getter keeps returning
 * bit-for-bit what it computed before.  The only side effect is that the
 * probe writes its own ring buffer and its own log file inside the DLL's
 * directory.
 *
 * The cave must NOT call the probe's logger directly: the C wrapper returns
 * through the cave's frame and would clobber the registers the caller had in
 * flight.  probe_hp_entry() therefore only snapshots 64 bytes into the ring
 * and returns immediately; a separate consumer thread turns the ring into
 * the log.
 *
 * Log file: <DLL directory>\hp-caller-probe.log   (AGENTS hard rule)
 * Switch  : <DLL directory>\hp-caller-probe.ini   (created on first run)
 *           probe=0 disables the whole patch -> pure no-op plugin.
 */

#define WIN32_LEAN_AND_MEAN
#define _CRT_SECURE_NO_WARNINGS
#include <windows.h>
#include <stdio.h>
#include <stdarg.h>
#include <stdlib.h>
#include <string.h>
#include <intrin.h>

#include "probe-logic.h"
#include "probe-hook-meta.h"

/* ------------------------------------------------------------------ */
/* in-memory state                                                     */
/* ------------------------------------------------------------------ */
static HMODULE g_self;
static wchar_t g_dir[MAX_PATH];
static FILE *g_log;
static CRITICAL_SECTION g_lock;
static DWORD g_t0;
static HMODULE g_exe;
static BYTE *g_exeBase;
static unsigned long long g_imgEnd; /* exe base + SizeOfImage */

static probe_ring_t *volatile g_ring;   /* read by the producer */
static volatile LONG g_gate;            /* 0 = cave is a no-op   */
static probe_drain_state_t g_drain;

static int g_cfgEnable = 1;
static int g_cfgDrainMs = 250;
static int g_cfgSummaryMs = 5000;
static int g_cfgHeadSamples = 8;

/* ------------------------------------------------------------------ */
/* logging (writes only into our own directory)                        */
/* ------------------------------------------------------------------ */
static void w2u(const wchar_t *src, char *dst, size_t cap) {
    if (!src) { dst[0] = 0; return; }
    int n = WideCharToMultiByte(CP_UTF8, 0, src, -1, dst, (int)cap - 1, NULL, NULL);
    if (n <= 0) { dst[0] = 0; return; }
    dst[n] = 0;
}

static void logf_(const char *fmt, ...) {
    char buf[2048];
    va_list ap;
    if (!g_log) return;
    va_start(ap, fmt);
    _vsnprintf_s(buf, sizeof(buf), _TRUNCATE, fmt, ap);
    va_end(ap);
    EnterCriticalSection(&g_lock);
    fprintf(g_log, "[+%7lums][t%6lu] %s\n", (unsigned long)(GetTickCount() - g_t0),
            (unsigned long)GetCurrentThreadId(), buf);
    fflush(g_log);
    LeaveCriticalSection(&g_lock);
}

/* module+offset for an address, so the log is readable next to IDA */
static void where(void *addr, char *out, size_t cap) {
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

/* ------------------------------------------------------------------ */
/* the code cave                                                       */
/* ------------------------------------------------------------------ */
/*
 * The cave is written in assembly (src/probe-hook.s) and its only "work" is a
 * call to probe_hp_entry() below.  All the register discipline lives in the
 * .s file; the C function must be a plain extern "C" function so its address
 * can be dropped into the cave's absolute-jump stub.
 */
/*
 * The cave jumps here and this function returns *into the cave's frame*, which
 * then jumps back to the getter's caller.  Therefore it must not touch anything
 * the cave has already saved, and it must leave rax exactly as it found it.
 */
extern "C" void probe_hp_entry(probe_ring_t *ring, volatile LONG *gate, unsigned long long caller,
                               long long rax, long long rcx, long long rdx, long long rbx);

/* ------------------------------------------------------------------ */
/* ini                                                                 */
/* ------------------------------------------------------------------ */
static const char kDefaultIni[] =
    "# hp-caller-probe -- runtime probe for the DFO 115us HP getter.\r\n"
    "# This file is generated once by the plugin and never overwritten after.\r\n"
    "# Delete it (or set probe=0) to make the plugin a pure no-op.\r\n"
    "\r\n"
    "# 1 = install the read-only probe, 0 = do nothing at all.\r\n"
    "probe=1\r\n"
    "\r\n"
    "# How often (ms) the background thread drains the in-memory ring.\r\n"
    "drain_ms=250\r\n"
    "\r\n"
    "# How often (ms) the per-caller summary table is appended to the log.\r\n"
    "summary_ms=5000\r\n"
    "\r\n"
    "# How many of the *first* samples of each newly seen call site are logged\r\n"
    "# verbatim (caller + returned value + descriptor pointer).  0 = none.\r\n"
    "head_samples=8\r\n";

static int ini_int(const char *text, const char *key, int dflt) {
    size_t klen = strlen(key);
    const char *p = text;
    while (p && *p) {
        const char *eol = strchr(p, '\n');
        const char *line = p;
        while (*line == ' ' || *line == '\t') line++;
        if (_strnicmp(line, key, klen) == 0 && line[klen] == '=') {
            const char *v = line + klen + 1;
            while (*v == ' ' || *v == '\t') v++;
            int val = atoi(v);
            if (strstr(v, "0x") == v || strstr(v, "0X") == v) val = (int)strtol(v, NULL, 16);
            return val;
        }
        p = eol ? eol + 1 : NULL;
    }
    return dflt;
}

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
    _snprintf_s(path, sizeof(path), _TRUNCATE, "%s\\hp-caller-probe.ini", dir8);
    FILE *f = fopen(path, "rb");
    if (!f) {
        write_default_ini(path);
        logf_("没有 %s -> 已生成默认开关（probe=%d drain_ms=%d summary_ms=%d）；删除它或把 probe 改 0 即可完全停用",
              path, g_cfgEnable, g_cfgDrainMs, g_cfgSummaryMs);
        return;
    }
    char text[4096];
    size_t n = fread(text, 1, sizeof(text) - 1, f);
    text[n] = 0;
    fclose(f);
    g_cfgEnable = ini_int(text, "probe", 1);
    g_cfgDrainMs = ini_int(text, "drain_ms", 250);
    g_cfgSummaryMs = ini_int(text, "summary_ms", 5000);
    g_cfgHeadSamples = ini_int(text, "head_samples", 8);
    if (g_cfgDrainMs < 20) g_cfgDrainMs = 20;
    if (g_cfgDrainMs > 5000) g_cfgDrainMs = 5000;
    if (g_cfgSummaryMs < 200) g_cfgSummaryMs = 200;
    if (g_cfgHeadSamples < 0) g_cfgHeadSamples = 0;
    if (g_cfgHeadSamples > 4096) g_cfgHeadSamples = 4096;
    logf_("配置 %s：probe=%d drain_ms=%d summary_ms=%d head_samples=%d", path, g_cfgEnable,
          g_cfgDrainMs, g_cfgSummaryMs, g_cfgHeadSamples);
}

/* ------------------------------------------------------------------ */
/* memory helpers                                                      */
/* ------------------------------------------------------------------ */
static int range_readable(const void *addr, size_t len) {
    MEMORY_BASIC_INFORMATION mbi;
    if (VirtualQuery(addr, &mbi, sizeof(mbi)) != sizeof(mbi)) return 0;
    if (mbi.State != MEM_COMMIT) return 0;
    if (mbi.Protect & (PAGE_NOACCESS | PAGE_GUARD)) return 0;
    const unsigned char *end = (const unsigned char *)addr + len;
    const unsigned char *regionEnd = (const unsigned char *)mbi.BaseAddress + mbi.RegionSize;
    return end <= regionEnd;
}

/*
 * Reserve a fresh executable page *near the game image* so that a 5-byte
 * rel32 jump can reach it.  We do not reuse any 0xCC filler inside the
 * executable: the previous attempt at this problem picked a 25-byte filler
 * run that turned out to receive a real CALL from 0x143327B14, so a fresh
 * mapping is the only option that cannot be referenced by anything.
 */
static BYTE *alloc_cave_near(BYTE *site, size_t size) {
    SYSTEM_INFO si;
    unsigned long long lo, hi, addr;
    GetSystemInfo(&si);
    {
        unsigned long long gran = si.dwAllocationGranularity ? si.dwAllocationGranularity : 0x10000;
        /* keep the whole allocation inside +-1 GiB of the site: that is 2x
         * the rel32 reach of the 5-byte jump we are going to write. */
        unsigned long long loRaw = (unsigned long long)site - 0x40000000ULL;
        unsigned long long hiRaw = (unsigned long long)site + 0x40000000ULL;
        lo = (loRaw + gran - 1) & ~(gran - 1);
        hi = hiRaw & ~(gran - 1);
        if (lo < (unsigned long long)si.lpMinimumApplicationAddress)
            lo = (unsigned long long)si.lpMinimumApplicationAddress;
        if (hi > (unsigned long long)si.lpMaximumApplicationAddress)
            hi = (unsigned long long)si.lpMaximumApplicationAddress;
    }
    /* prefer starting right above the image and walking upwards */
    addr = (unsigned long long)site;
    if (g_imgEnd && g_imgEnd < hi) addr = (g_imgEnd + 0xFFFFFULL) & ~0xFFFFFULL;
    for (; addr < hi;) {
        MEMORY_BASIC_INFORMATION mbi;
        if (VirtualQuery((LPCVOID)addr, &mbi, sizeof(mbi)) != sizeof(mbi)) break;
        if (mbi.State == MEM_FREE && mbi.RegionSize >= size &&
            (unsigned long long)mbi.BaseAddress >= lo) {
            BYTE *p = (BYTE *)VirtualAlloc(mbi.BaseAddress, size, MEM_COMMIT | MEM_RESERVE,
                                           PAGE_EXECUTE_READWRITE);
            if (p) {
                long long d = (long long)p - (long long)site;
                if (d > -0x7FF00000LL && d < 0x7FF00000LL) return p;
                VirtualFree(p, 0, MEM_RELEASE);
            }
        }
        {
            unsigned long long next = (unsigned long long)mbi.BaseAddress + mbi.RegionSize;
            if (next <= addr) break;
            addr = next;
        }
    }
    /* second pass: walk downwards from the site in case the address space
     * above the image is saturated */
    addr = (unsigned long long)site;
    for (; addr > lo;) {
        MEMORY_BASIC_INFORMATION mbi;
        unsigned long long back = (addr > 0x100000ULL) ? addr - 0x100000ULL : 0;
        if (back < lo) back = lo;
        if (VirtualQuery((LPCVOID)back, &mbi, sizeof(mbi)) != sizeof(mbi)) break;
        if (mbi.State == MEM_FREE && mbi.RegionSize >= size) {
            BYTE *p = (BYTE *)VirtualAlloc(mbi.BaseAddress, size, MEM_COMMIT | MEM_RESERVE,
                                           PAGE_EXECUTE_READWRITE);
            if (p) {
                long long d = (long long)p - (long long)site;
                if (d > -0x7FF00000LL && d < 0x7FF00000LL) return p;
                VirtualFree(p, 0, MEM_RELEASE);
            }
        }
        addr = back;
    }
    return NULL;
}

/* ------------------------------------------------------------------ */
/* install                                                             */
/* ------------------------------------------------------------------ */
static const BYTE kCaveHead[PROBE_CAVEHEAD_LEN] = PROBE_CAVEHEAD_BYTES;

/*
 * Full integrity check of the embedded cave image.
 *
 * This is not paranoia: MSVC's linker folds identical data ("ICF") and the cave
 * image is mostly zeros, so without /OPT:NOICF the linker can alias runs of it
 * with other zero data -- in the offline harness that silently replaced the
 * stub's target slot with zeros.  The DLL is built with /OPT:NOICF, and this
 * check makes a future build that loses it fail loudly instead of jumping into
 * a zero address inside the game.
 */
static int cave_image_intact(void) {
    /* structural invariants of the generated image */
    if (PROBE_CAVE_IMAGE_LEN < 32) return 0;
    if (PROBE_CAVE_IMAGE[0] != 0xE9) return 0;                  /* entry jmp */
    if (PROBE_CAVE_IMAGE[PROBE_OFF_STUB + 0] != 0xFF) return 0;       /* stub */
    if (PROBE_CAVE_IMAGE[PROBE_OFF_STUB + 1] != 0x25) return 0;
    {
        size_t k;
        for (k = 0; k < 8; k++) {
            /* both patch slots must still be zero in the built image */
            if (PROBE_CAVE_IMAGE[PROBE_OFF_LOG_SLOT + k] != 0) return 0;
            if (PROBE_CAVE_IMAGE[PROBE_OFF_TAIL_SLOT + k] != 0) return 0;
            /* the stub's target slot must still be zero */
            if (PROBE_CAVE_IMAGE[PROBE_OFF_STUB + 6 + k] != 0) return 0;
        }
    }
    if (memcmp(PROBE_CAVE_IMAGE, kCaveHead, sizeof(kCaveHead)) != 0) return 0;
    return 1;
}

static int install_probe(void) {
    BYTE *site = g_exeBase + PROBE_SITE_RVA;
    BYTE *cave;
    BYTE *stub;
    BYTE before[PROBE_SITE_PATCHLEN];
    long long disp;
    DWORD old = 0;

    /* 1. the site must be readable and hold exactly the expected bytes.
     *    Anything unexpected and we do not touch the client at all. */
    {
        static const BYTE expect[PROBE_SITE_PATCHLEN] = PROBE_SITE_EXPECTED_BYTES;
        if (!range_readable(site, PROBE_SITE_PATCHLEN)) {
            logf_("跳过：目标 %p 不在已提交内存里（客户端版本不同？）", site);
            return 0;
        }
        memcpy(before, site, sizeof(before));
        if (memcmp(before, expect, PROBE_SITE_PATCHLEN) != 0) {
            logf_("跳过：现场字节不是期望的 %s（实测 %02x %02x %02x %02x %02x）",
                  PROBE_SITE_EXPECTED_HEX, before[0], before[1], before[2], before[3], before[4]);
            return 0;
        }
    }
    logf_("现场核对通过：VA %s RVA %s 字节 %s", PROBE_SITE_HEX, PROBE_SITE_RVA_HEX,
          PROBE_SITE_EXPECTED_HEX);

    /* 2. reserve a fresh page near the image */
    cave = alloc_cave_near(site, PROBE_TOTAL_SIZE);
    if (!cave) {
        logf_("跳过：模块附近找不到可分配的可用内存（保持客户端原样）");
        return 0;
    }
    stub = cave + PROBE_OFF_STUB;
    {
        char w[300];
        where(cave, w, sizeof(w));
        logf_("代码洞已分配：%p (%s)，大小 %d 字节", cave, w, (int)PROBE_TOTAL_SIZE);
    }

    /* 3. copy the assembled trampoline and check its head matches the header
     *    contract (metadata probe-hook-meta.h is generated from the .s) */
    if (!cave_image_intact()) {
        logf_("跳过：内嵌代码洞镜像完整性自检失败（构建时丢了 /OPT:NOICF，链接器把零字节段折叠了？）");
        return 0;
    }
    memset(cave, 0xCC, PROBE_TOTAL_SIZE);
    memcpy(cave, PROBE_CAVE_IMAGE, PROBE_CAVE_IMAGE_LEN);
    if (PROBE_CAVE_IMAGE_LEN < (size_t)PROBE_CAVE_LEN_BYTES) {
        logf_("注意：内嵌代码洞镜像只有 %d 字节，预期 %d 字节（继续，但请核对生成脚本）",
              (int)PROBE_CAVE_IMAGE_LEN, PROBE_CAVE_LEN_BYTES);
    }
    if (memcmp(cave, kCaveHead, sizeof(kCaveHead)) != 0) {
        logf_("跳过：内嵌代码洞头部与元数据不一致 -> 构建脚本生成的镜像不是预期的代码洞");
        VirtualFree(cave, 0, MEM_RELEASE);
        return 0;
    }

    /* 4. patch the fields inside the cave */
    disp = (long long)cave - (long long)site - 5;
    *(int *)(cave + PROBE_OFF_SITEJMP + 1) = (int)disp;
    {
        unsigned long long fn = (unsigned long long)(void *)&probe_hp_entry;
        memcpy(stub + 6, &fn, sizeof(fn));
        *(unsigned long long *)(cave + PROBE_OFF_LOG_SLOT) = fn;
        /* Return address: the patched `ret` is preceded by a `call <getter>`
         * whose disp32 we verified above, so the address the original `ret`
         * would have returned to is site + 5 + disp32.  Computed from the
         * verified bytes, never assumed. */
        *(unsigned long long *)(cave + PROBE_OFF_TAIL_SLOT) =
            (unsigned long long)(site + 5 + *(const int *)(before + 1));
        logf_("返回地址槽：VA %#llx（由现场 E8 %08x 还原）",
              *(unsigned long long *)(cave + PROBE_OFF_TAIL_SLOT),
              (unsigned)(*(const int *)(before + 1) & 0xFFFFFFFFu));
    }
    /* 5. the cave is inert until the consumer thread has the ring ready */
    InterlockedExchange(&g_gate, 0);

    FlushInstructionCache(GetCurrentProcess(), cave, PROBE_TOTAL_SIZE);

    /* 6. patch the getter's exit */
    {
        BYTE a[PROBE_SITE_PATCHLEN];
        char hexb[64], hexa[64];
        size_t k;
        DWORD tmp = 0;
        if (!VirtualProtect(site, PROBE_SITE_PATCHLEN, PAGE_EXECUTE_READWRITE, &old)) {
            logf_("跳过：无法修改目标页保护（error %lu）", (unsigned long)GetLastError());
            VirtualFree(cave, 0, MEM_RELEASE);
            return 0;
        }
        memcpy(a, before, PROBE_SITE_PATCHLEN);
        a[0] = 0xE9;
        *(int *)(a + 1) = (int)disp;
        memcpy(site, a, PROBE_SITE_PATCHLEN);
        VirtualProtect(site, PROBE_SITE_PATCHLEN, old, &tmp);
        FlushInstructionCache(GetCurrentProcess(), site, PROBE_SITE_PATCHLEN);
        for (k = 0; k < PROBE_SITE_PATCHLEN; k++) {
            _snprintf_s(hexb + k * 3, 4, _TRUNCATE, "%02x ", before[k]);
            _snprintf_s(hexa + k * 3, 4, _TRUNCATE, "%02x ", a[k]);
        }
        hexb[PROBE_SITE_PATCHLEN * 3] = 0;
        hexa[PROBE_SITE_PATCHLEN * 3] = 0;
        logf_("已装探针：VA %s (file %s) before=[%s] after=[%s] jmp -> %p (rel32 %+lld)",
              PROBE_SITE_HEX, PROBE_SITE_FILE_HEX, hexb, hexa, cave, disp);
    }
    logf_("覆盖范围：VA %s..%s（原 C3 + 4 个 CC 填充；其余 CC 填充与下一函数未动）", PROBE_SITE_HEX,
          PROBE_SITE_HEX_LAST);
    logf_("未改动：getter 本体、0x14BAF7F20 的共享 1.0 常量、flag==0 路径、DFO.exe 文件本身");
    logf_("已知覆盖缺口：getter 另有出口 VA %s，其后紧跟真实指令（无填充可借），本探针不碰它；"
          "从该出口返回的样本不会被记录（见 README「覆盖范围」）。",
          PROBE_SITE2_HEX);
    return 1;
}

/* ------------------------------------------------------------------ */
/* producer entry point (called from the cave)                         */
/* ------------------------------------------------------------------ */
extern "C" void probe_hp_entry(probe_ring_t *ring, volatile LONG *gate, unsigned long long caller,
                               long long rax, long long rcx, long long rdx, long long rbx) {
    if (gate && *gate > 0 && ring) probe_produce(ring, caller, rax, rcx, rdx, rbx);
}
/* ------------------------------------------------------------------ */
/* consumer: ring -> tally -> log                                      */
/* ------------------------------------------------------------------ */
/*
 * IMPORTANT: the consumer must never dereference the descriptor pointer it
 * captured.  By the time a sample is drained the game object may already have
 * been freed, and this is a probe -- it must not be able to take the client
 * down.  Everything we need to identify "which call site" is in the register
 * snapshot itself; the descriptor can be inspected offline from the values
 * printed here.
 */
#define MAX_TALLY 256

static probe_tally_t g_tally[MAX_TALLY];
static int g_tallyUsed;
static ULONGLONG g_loggedHits = 0;

static void lost_cb(ULONGLONG n, void *ctx) {
    (void)ctx;
    logf_("环形缓冲落后：%llu 个采样被覆盖（继续统计，丢失量会累计）", (unsigned long long)n);
}

typedef struct {
    int newCaller;
    int logged;
} drain_ctx;

static void drain_cb(const probe_entry_t *e, void *ctx) {
    drain_ctx *dc = (drain_ctx *)ctx;
    ULONGLONG caller = (ULONGLONG)e->e_caller;
    int isnew;
    int row;
    isnew = probe_tally_add(g_tally, MAX_TALLY, &g_tallyUsed, caller, (LONG64)e->e_rax,
                            (LONG64)e->e_rcx, (LONG64)e->e_rdx, (LONG64)e->e_rbx,
                            (LONGLONG)e->e_tick);
    for (row = 0; row < g_tallyUsed; row++)
        if (g_tally[row].caller == caller) break;
    if (isnew) {
        char w[300];
        where((void *)caller, w, sizeof(w));
        logf_("新的调用点：caller=%p (%s)  第一条样本 ret_rax=%lld rcx=%p rdx=%lld rbx=%lld",
              (void *)caller, w, (long long)e->e_rax, (void *)e->e_rcx, (long long)e->e_rdx,
              (long long)e->e_rbx);
        dc->newCaller++;
    }
    /* Log the first few samples of every call site verbatim: this is what lets
     * the reader see "this caller returned 12345 five times" right next to the
     * user's action. */
    if (row < g_tallyUsed && (int)g_tally[row].hits <= g_cfgHeadSamples) {
        logf_("样本 #%llu caller=%p ret_rax=%lld rcx(desc)=%p rdx=%lld rbx=%lld",
              (unsigned long long)g_tally[row].hits, (void *)caller, (long long)e->e_rax,
              (void *)e->e_rcx, (long long)e->e_rdx, (long long)e->e_rbx);
    }
    dc->logged++;
}

static void dump_tally(const char *why) {
    int i, j;
    probe_tally_t *t = g_tally;
    logf_("---- 调用点汇总（%s）：共 %d 个调用点，已消费 %llu 个采样，落后丢失 %llu，半写重试 %llu ----",
          why, g_tallyUsed, (unsigned long long)g_drain.total, (unsigned long long)g_drain.lost,
          (unsigned long long)g_drain.torn);
    /* sort by hits, descending (simple insertion sort on an index array) */
    {
        int idx[MAX_TALLY];
        for (i = 0; i < g_tallyUsed; i++) idx[i] = i;
        for (i = 1; i < g_tallyUsed; i++) {
            int k = idx[i];
            j = i - 1;
            while (j >= 0 && t[idx[j]].hits < t[k].hits) {
                idx[j + 1] = idx[j];
                j--;
            }
            idx[j + 1] = k;
        }
        for (i = 0; i < g_tallyUsed; i++) {
            probe_tally_t *r = &t[idx[i]];
            char w[300];
            where((void *)r->caller, w, sizeof(w));
            logf_("  caller=%p (%-28s) hits=%-9llu ret_rax[last=%lld min=%lld max=%lld] desc=%p rdx_evidence=%lld",
                  (void *)r->caller, w, (unsigned long long)r->hits, (long long)r->last_rax,
                  (long long)r->min_rax, (long long)r->max_rax, (void *)r->last_rcx,
                  (long long)r->last_rdx);
        }
    }
}

static void dump_callsite_table(void) {
    int i;
    logf_("已知直接调用点（来自离线取证 FINDINGS.md §2.1；表里的 rva 是 RVA）：");
    for (i = 0; i < PROBE_KNOWN_COUNT; i++)
        logf_("  %2d  RVA %-10s VA %s  flag=%d  %s", i + 1, PROBE_KNOWN[i].rva_hex,
              PROBE_KNOWN[i].va_hex, PROBE_KNOWN[i].flag, PROBE_KNOWN[i].note);
}

static DWORD WINAPI consumer_thread(LPVOID param) {
    DWORD lastSummary = GetTickCount();
    (void)param;
    for (;;) {
        Sleep((DWORD)g_cfgDrainMs);
        if (g_ring) {
            drain_ctx dc;
            dc.newCaller = 0;
            probe_drain(g_ring, &g_drain, drain_cb, &dc, lost_cb);
        }
        if (GetTickCount() - lastSummary >= (DWORD)g_cfgSummaryMs) {
            lastSummary = GetTickCount();
            if (g_drain.total != g_loggedHits) {
                g_loggedHits = g_drain.total;
                dump_tally("周期汇总");
            }
        }
    }
}

/* ------------------------------------------------------------------ */
/* startup                                                             */
/* ------------------------------------------------------------------ */
static DWORD WINAPI worker(LPVOID param) {
    char path[MAX_PATH * 2];
    char dir8[MAX_PATH * 2];
    (void)param;
    w2u(g_dir, dir8, sizeof(dir8));
    _snprintf_s(path, sizeof(path), _TRUNCATE, "%s\\hp-caller-probe.log", dir8);
    g_log = fopen(path, "ab");
    if (!g_log) return 1;

    logf_("==== hp-caller-probe 启动（只记录，不改游戏行为）====");
    logf_("pid=%lu dll 目录=%s", (unsigned long)GetCurrentProcessId(), dir8);
    logf_("日志=%s", path);
    {
        char exe[MAX_PATH * 2] = "";
        wchar_t wexe[MAX_PATH];
        if (GetModuleFileNameW(NULL, wexe, MAX_PATH)) w2u(wexe, exe, sizeof(exe));
        logf_("客户端 EXE=%s", exe);
    }
    load_config();
    dump_callsite_table();

    if (!g_cfgEnable) {
        logf_("配置 probe=0：不装探针，本插件不产生任何运行时开销。");
        return 0;
    }

    g_exe = GetModuleHandleW(NULL);
    g_exeBase = (BYTE *)g_exe;
    {
        /* SizeOfImage via the PE headers of the main module (no file access) */
        BYTE *p = g_exeBase;
        if (p && p[0] == 'M' && p[1] == 'Z') {
            LONG e_lfanew = *(LONG *)(p + 0x3C);
            BYTE *nt = p + e_lfanew;
            if (nt[0] == 'P' && nt[1] == 'E') {
                unsigned size = *(unsigned *)(nt + 0x18 + 0x38); /* OptionalHeader.SizeOfImage */
                g_imgEnd = (unsigned long long)g_exeBase + size;
                logf_("主模块基址=%p SizeOfImage=%#x -> 镜像末尾=%#llx", g_exeBase, size,
                      (unsigned long long)g_imgEnd);
            }
        }
    }
    if (!g_imgEnd) logf_("警告：读不到主模块 SizeOfImage，代码洞搜索将退化为全地址空间扫描。");

    {
        /* A page of our own is the safest possible code cave: nothing in the
         * image can reference it.  Reserve it before the ring so the ring
         * never has to be near anything. */
        probe_ring_t *ring = (probe_ring_t *)VirtualAlloc(NULL, sizeof(probe_ring_t),
                                                          MEM_COMMIT | MEM_RESERVE, PAGE_READWRITE);
        if (!ring) {
            logf_("无法分配环形缓冲（%llu 字节），放弃。", (unsigned long long)sizeof(probe_ring_t));
            return 1;
        }
        memset((void *)ring, 0, sizeof(probe_ring_t));
        g_ring = ring;
        logf_("环形缓冲 %d 槽 × %d 字节 = %llu 字节 @ %p", PROBE_RING_SLOTS, (int)sizeof(probe_entry_t),
              (unsigned long long)sizeof(probe_ring_t), ring);
    }

    if (!install_probe()) {
        logf_("探针未安装：本 DLL 现在什么都不做。把本日志发回即可。");
        return 1;
    }
    InterlockedExchange(&g_gate, 1);
    logf_("门控已打开（gate=1）：getter 的每一次返回现在都会被采一个样本。");

    {
        HANDLE t = CreateThread(NULL, 0, consumer_thread, NULL, 0, NULL);
        if (t) CloseHandle(t);
        else logf_("无法启动消费线程：样本会留在环形缓冲里、不会落盘。");
    }
    logf_("探针就绪。请按《操作说明》做：进副本站定 10 秒 -> 打两下怪 -> 开关一次角色面板 -> 退出游戏。");
    return 0;
}

__declspec(dllexport) DWORD WINAPI ModStart(void) {
    wchar_t path[MAX_PATH];
    DWORD n = GetModuleFileNameW(g_self, path, MAX_PATH);
    if (n) {
        wcscpy_s(g_dir, MAX_PATH, path);
        for (DWORD i = n; i > 0; i--) {
            if (g_dir[i - 1] == L'\\') { g_dir[i - 1] = 0; break; }
        }
    }
    InitializeCriticalSection(&g_lock);
    g_t0 = GetTickCount();
    {
        HANDLE t = CreateThread(NULL, 0, worker, NULL, 0, NULL);
        if (t) CloseHandle(t);
    }
    return 0;
}

__declspec(dllexport) const char *WINAPI ModName(void) { return "hp-caller-probe"; }

BOOL WINAPI DllMain(HINSTANCE inst, DWORD reason, LPVOID reserved) {
    (void)reserved;
    if (reason == DLL_PROCESS_ATTACH) {
        g_self = inst;
        DisableThreadLibraryCalls(inst);
    }
    return TRUE;
}
