/*
 * 门禁自测：在本进程里复刻客户端那道 IME 门禁（形状与 DFO.exe RVA 0x6F22400 处一致），
 * 然后加载中文输入插件，验证门禁从"不过"变成"过"。它只碰本进程，不启动客户端。
 *
 * 客户端门禁（静态取证见 analysis/tasks/chinese-input-probe-20261006.md §4）：
 *     hkl = 键盘布局;  若 hkl ∈ { E0080404, E0090404, E00E0804 } 且
 *     ImmGetIMEFileNameW(hkl) 返回 { TINTLGNT.IME, CINTLGNT.IME, MSTCIPHA.IME,
 *                                    PINTLGNT.IME, MSSCIPYA.IME } 之一 → 启用 IME 通路
 *
 * 关键：所有 API 都用 GetProcAddress 现取 —— 只有这样才会拿到插件装在导出表里的桩。
 */
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdio.h>
#include <string.h>
#include <wchar.h>

typedef HKL(WINAPI *GetLayoutFn)(DWORD);
typedef UINT(WINAPI *GetImeNameFn)(HKL, LPWSTR, UINT);
typedef DWORD(WINAPI *StartFn)(void);

static const DWORD kAcceptedHkl[] = {0xE0080404, 0xE0090404, 0xE00E0804};
static const wchar_t *kAcceptedIme[] = {L"TINTLGNT.IME", L"CINTLGNT.IME", L"MSTCIPHA.IME",
                                        L"PINTLGNT.IME", L"MSSCIPYA.IME"};

/* 复刻客户端门禁；verbose 时打印每一步取到的值 */
static int gate(int verbose) {
    GetLayoutFn gl = (GetLayoutFn)GetProcAddress(GetModuleHandleW(L"user32.dll"),
                                                 "GetKeyboardLayout");
    GetImeNameFn gn = (GetImeNameFn)GetProcAddress(GetModuleHandleW(L"imm32.dll"),
                                                   "ImmGetIMEFileNameW");
    if (!gl || !gn) {
        printf("  取不到 API\n");
        return -1;
    }
    HKL hkl = gl(0);
    int hklOk = 0;
    for (int i = 0; i < 3; i++)
        if ((DWORD)(ULONG_PTR)hkl == kAcceptedHkl[i]) hklOk = 1;
    if (verbose) printf("  第一步 键盘布局 = %#010llx → %s\n", (unsigned long long)(ULONG_PTR)hkl,
                        hklOk ? "通过" : "不过（客户端会直接放弃 IME 通路）");
    if (!hklOk) return 0;

    WCHAR buf[512] = {0};
    UINT r = gn(hkl, buf, 512);
    if (verbose) {
        char u8[512] = "";
        WideCharToMultiByte(CP_UTF8, 0, buf, -1, u8, sizeof(u8) - 1, NULL, NULL);
        printf("  第二步 ImmGetIMEFileNameW → %u \"%s\"\n", r, u8);
    }
    if (!r) return 0;
    for (int i = 0; i < 5; i++)
        if (_wcsicmp(buf, kAcceptedIme[i]) == 0) return 1;
    if (verbose) printf("  文件名不在白名单里 → 不启用\n");
    return 0;
}

int wmain(int argc, wchar_t **argv) {
    if (argc < 2) {
        printf("用法: gate_test.exe <ChineseLocalization.dll 绝对路径>\n");
        return 2;
    }
    /* 客户端的 imm32 是静态导入的，这里也先 LoadLibrary，让插件能钩到 */
    LoadLibraryW(L"imm32.dll");

    printf("=== 装插件之前 ===\n");
    int before = gate(1);
    printf("门禁 = %s\n\n", before ? "启用" : "不启用");

    HMODULE payload = LoadLibraryExW(argv[1], NULL, 0x900);
    if (!payload) {
        printf("LoadLibraryExW 失败：%lu\n", GetLastError());
        return 3;
    }
    StartFn start = (StartFn)GetProcAddress(payload, "StartLocalization");
    if (!start) {
        printf("插件没有 StartLocalization 导出\n");
        return 4;
    }
    start();
    Sleep(600); /* 插件在自己的线程里装钩子 */

    printf("=== 装插件之后 ===\n");
    int after = gate(1);
    printf("门禁 = %s\n\n", after ? "启用" : "不启用");

    if (before == 0 && after == 1) {
        printf("PASS：插件把客户端形状的 IME 门禁从未通过变成通过\n");
        return 0;
    }
    printf("FAIL：期望 前=不启用 后=启用，实际 前=%d 后=%d\n", before, after);
    return 1;
}
