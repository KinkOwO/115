/*
 * 探针自测宿主：完全按客户端 dinput8.dll 代理的方式调用 ChineseLocalization.dll。
 *   1. LoadLibraryExW(绝对路径, NULL, 0x900)
 *   2. GetProcAddress(h, "StartLocalization")
 *   3. 无参数调用它
 * 随后建一个 CS_IME 窗口、喂几条 IME/字符消息，跑 5 秒再退出。
 * 它只用于本机自测探针是否会崩、是否写日志，不参与游戏。
 */
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdio.h>

static LRESULT CALLBACK wnd(HWND h, UINT m, WPARAM w, LPARAM l) {
    return DefWindowProcW(h, m, w, l);
}

int wmain(int argc, wchar_t **argv) {
    if (argc < 2) {
        wprintf(L"用法: test-host.exe <ChineseLocalization.dll 绝对路径>\n");
        return 2;
    }
    /* 让 imm32 先装载，探针才会挂钩（真实客户端在启动期就装着 imm32） */
    LoadLibraryW(L"imm32.dll");

    HMODULE payload = LoadLibraryExW(argv[1], NULL, 0x900);
    wprintf(L"LoadLibraryExW -> %p (err=%lu)\n", (void *)payload, GetLastError());
    if (!payload) return 3;
    FARPROC start = GetProcAddress(payload, "StartLocalization");
    wprintf(L"GetProcAddress(StartLocalization) -> %p\n", (void *)start);
    if (!start) return 4;
    typedef DWORD(WINAPI * Fn)(void);
    DWORD rc = ((Fn)start)();
    wprintf(L"StartLocalization() -> %lu\n", (unsigned long)rc);

    WNDCLASSEXW wc = {0};
    wc.cbSize = sizeof(wc);
    wc.lpfnWndProc = wnd;
    wc.hInstance = GetModuleHandleW(NULL);
    wc.lpszClassName = L"ProbeTestWindow";
    wc.style = CS_IME;
    RegisterClassExW(&wc);
    HWND h = CreateWindowExW(0, L"ProbeTestWindow", L"probe test", WS_OVERLAPPEDWINDOW, 50, 50,
                             400, 300, NULL, NULL, wc.hInstance, NULL);
    wprintf(L"CreateWindowExW -> %p\n", (void *)h);
    ShowWindow(h, SW_SHOW);
    SetForegroundWindow(h);
    UpdateWindow(h);

    Sleep(1500);
    /* 用 GetProcAddress 取（而不是走本进程的导入表）：只有这样才能拿到
     * 插件装好的 EAT 钩子，验证"门禁修复"是不是真的生效。 */
    {
        HMODULE u32 = GetModuleHandleW(L"user32.dll");
        HMODULE imm = GetModuleHandleW(L"imm32.dll");
        typedef HKL(WINAPI * GetLayoutFn)(DWORD);
        typedef UINT(WINAPI * GetImeNameFn)(HKL, LPWSTR, UINT);
        GetLayoutFn gl = (GetLayoutFn)GetProcAddress(u32, "GetKeyboardLayout");
        GetImeNameFn gn = (GetImeNameFn)GetProcAddress(imm, "ImmGetIMEFileNameW");
        if (gl && gn) {
            HKL h = gl(0);
            WCHAR nbuf[128] = {0};
            UINT r = gn(h, nbuf, 128);
            wprintf(L"[门禁验证] GetKeyboardLayout -> %p ；ImmGetIMEFileNameW -> %u \"%ls\"\n",
                    (void *)h, r, nbuf);
        }
    }
    PostMessageW(h, WM_IME_STARTCOMPOSITION, 0, 0);
    PostMessageW(h, WM_IME_COMPOSITION, 0, 0x0008);
    PostMessageW(h, WM_IME_CHAR, 0x4E2D, 1);
    PostMessageW(h, WM_CHAR, 'A', 1);
    PostMessageW(h, WM_IME_ENDCOMPOSITION, 0, 0);
    PostMessageW(h, WM_INPUTLANGCHANGE, 0, 0);

    DWORD end = GetTickCount() + 4000;
    MSG msg;
    while (GetTickCount() < end) {
        while (PeekMessageW(&msg, NULL, 0, 0, PM_REMOVE)) {
            TranslateMessage(&msg);
            DispatchMessageW(&msg);
        }
        Sleep(50);
    }
    DestroyWindow(h);
    wprintf(L"宿主退出（探针日志应在 DLL 同目录）\n");
    return 0;
}
