/*
 * inline 钩子机制自测：验证"5 字节相对跳转 + 近地址桥 + 跳板复制原指令"这一套
 * 在 x64 上真的能装、能转发、能返回原值。目标函数是本进程里手写的机器码，
 * 入口字节完全确定，不依赖编译器；不碰客户端。
 *
 * 与插件 install_inline 是同一套逻辑（去掉日志）。
 */
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdio.h>
#include <string.h>

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

static int install_inline(unsigned char *p, const unsigned char *expect, size_t patchLen,
                          void *hook, void **origOut) {
    if (memcmp(p, expect, patchLen) != 0) {
        printf("FAIL: 期望字节对不上\n");
        return 0;
    }
    unsigned char *stub = alloc_near(GetModuleHandleW(NULL), 0x1000);
    if (!stub) {
        printf("FAIL: 附近没有可分配内存\n");
        return 0;
    }
    unsigned char *tramp = stub;
    unsigned char *bridge = stub + 0x40;
    memcpy(tramp, p, patchLen);
    write_rel_jmp(tramp + patchLen, p + patchLen);
    write_abs_jmp(bridge, hook);

    DWORD old = 0;
    if (!VirtualProtect(p, patchLen, PAGE_EXECUTE_READWRITE, &old)) {
        printf("FAIL: VirtualProtect\n");
        return 0;
    }
    write_rel_jmp(p, bridge);
    for (size_t i = 5; i < patchLen; i++) p[i] = 0x90;
    DWORD tmp = 0;
    VirtualProtect(p, patchLen, old, &tmp);
    FlushInstructionCache(GetCurrentProcess(), p, patchLen);
    *origOut = tramp;
    printf("装上：目标 %p，桥 %p，跳板 %p（覆盖 %zu 字节）\n", p, bridge, tramp, patchLen);
    return 1;
}

/* ---------------- 被测目标：手写机器码 ---------------- */
static int g_calls = 0;
static void *orig_target = NULL;
typedef int(WINAPI *TargetFn)(int);

static int WINAPI hook_target(int x) {
    g_calls++;
    return ((TargetFn)orig_target)(x);
}

int main(void) {
    /* mov eax,ecx ; add eax,5 ; ret   → f(x) = x + 5 */
    static const unsigned char code[] = {0x8B, 0xC1, 0x83, 0xC0, 0x05, 0xC3};
    static const unsigned char expect[] = {0x8B, 0xC1, 0x83, 0xC0, 0x05}; /* 前 5 字节=完整指令 */
    unsigned char *fn = alloc_near(GetModuleHandleW(NULL), 0x1000);
    if (!fn) {
        printf("FAIL: 无法在主模块附近分配目标代码页\n");
        return 1;
    }
    DWORD prot = 0;
    VirtualProtect(fn, sizeof(code), PAGE_EXECUTE_READWRITE, &prot);
    memcpy(fn, code, sizeof(code));

    printf("目标入口字节：");
    for (size_t i = 0; i < sizeof(expect); i++) printf("%02x ", fn[i]);
    printf("\n");

    if (!install_inline(fn, expect, sizeof(expect), (void *)hook_target, &orig_target)) return 1;

    int r1 = ((TargetFn)fn)(10);
    int r2 = ((TargetFn)fn)(100);
    printf("f(10)=%d f(100)=%d 包装调用=%d 次\n", r1, r2, g_calls);
    if (r1 != 15 || r2 != 105 || g_calls != 2) {
        printf("FAIL：转发或返回值不对（期望 15/105/2）\n");
        return 1;
    }
    printf("PASS：inline 跳转 + 跳板转发 + 返回值都正确\n");
    return 0;
}
