/*
 * 插件 ABI 自测宿主：完全按 client-patchs/client-host（ChineseLocalization.dll）
 * 的方式加载并调用一个 .115us-mods 插件：
 *   1. LoadLibraryW(绝对路径)
 *   2. GetModuleHandleExW(PIN)   ← 宿主只加载不卸载
 *   3. GetProcAddress(h, "ModName")（可选）
 *   4. GetProcAddress(h, "ModStart")（找不到才退 StartLocalization）
 *   5. 调用它，0 = 成功
 * 只在本机自测插件"能不能被装载、导出对不对、会不会崩"，不参与游戏。
 */
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdio.h>

typedef DWORD(WINAPI *ModStartFn)(void);
typedef const char *(WINAPI *ModNameFn)(void);

int wmain(int argc, wchar_t **argv) {
    HMODULE h;
    HMODULE pinned = NULL;
    ModNameFn name;
    ModStartFn start;
    DWORD rc;
    if (argc < 2) {
        wprintf(L"用法: host-smoke.exe <插件 DLL 绝对路径>\n");
        return 2;
    }
    h = LoadLibraryW(argv[1]);
    wprintf(L"LoadLibraryW -> %p (err=%lu)\n", (void *)h, (unsigned long)GetLastError());
    if (!h) return 3;
    if (!GetModuleHandleExW(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS |
                                GET_MODULE_HANDLE_EX_FLAG_PIN,
                            (LPCWSTR)h, &pinned))
        wprintf(L"GetModuleHandleExW(PIN) 失败（err=%lu）\n", (unsigned long)GetLastError());
    else
        wprintf(L"GetModuleHandleExW(PIN) -> %p\n", (void *)pinned);

    name = (ModNameFn)GetProcAddress(h, "ModName");
    wprintf(L"GetProcAddress(ModName) -> %p", (void *)name);
    if (name) wprintf(L"  名字=\"%hs\"", name());
    wprintf(L"\n");

    start = (ModStartFn)GetProcAddress(h, "ModStart");
    if (!start) start = (ModStartFn)GetProcAddress(h, "StartLocalization");
    wprintf(L"GetProcAddress(ModStart) -> %p\n", (void *)start);
    if (!start) {
        wprintf(L"[FAIL] 没有 ModStart / StartLocalization 导出\n");
        return 4;
    }
    rc = start();
    wprintf(L"ModStart() -> %lu  %s\n", (unsigned long)rc, rc == 0 ? L"[OK]" : L"[FAIL]");
    /* 让插件的工作线程有机会跑一两轮、把日志刷出来 */
    Sleep(1500);
    wprintf(L"宿主退出（插件日志应在插件自己所在目录）\n");
    return rc == 0 ? 0 : 5;
}
