/*
 * 假客户端：专门用来端到端验证「Themida 导入槽修正」这条路。
 *
 * 做法：
 *   1. 用一个 0x9187000 字节的 .bss 占位数组，把镜像 SizeOfImage 撑到
 *      插件扫描区间（RVA 0x9186100..0x9187A00）之内（.bss 不占文件体积）；
 *   2. 在区间里 RVA 0x9186200 处放一个"槽"，槽值 = 加载插件**之前**取到的
 *      真 GetKeyboardLayout 地址 —— 这正是 Themida 桩表里存的东西；
 *   3. 按 dinput8 代理的方式加载插件（LoadLibraryExW + StartLocalization）；
 *   4. 等一个扫描周期后，再看槽：
 *        - 槽值应当已经被换成插件的包装；
 *        - 通过槽调用应当拿到传统 IME HKL（0xE00E0804）。
 *
 * 它只碰本进程，不启动客户端。
 */
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdio.h>

/* 撑大镜像：.bss（未初始化）不写文件，但计入 SizeOfImage。
 * 必须真的被引用，否则 /O2 会把这个没人用的数组整个消掉、镜像又变回很小。 */
static volatile char g_pad[0x9187000];

#define SLOT_RVA 0x9186200 /* 落在插件的扫描区间 0x9186100..0x9187A00 内 */

typedef HKL(WINAPI *GetLayoutFn)(DWORD);

int wmain(int argc, wchar_t **argv) {
    setvbuf(stdout, NULL, _IONBF, 0); /* 崩了也要看到已经打到哪一步 */
    if (argc < 2) {
        printf("用法: slot_test.exe <ChineseLocalization.dll 绝对路径>\n");
        return 2;
    }
    g_pad[0] = 1; /* 别让链接器把它丢掉 */
    unsigned char *base = (unsigned char *)GetModuleHandleW(NULL);
    void **slot = (void **)(base + SLOT_RVA);

    /* 先确认这段 RVA 真的在本进程镜像里、而且可写 —— 不成立就别硬写 */
    int e_lfanew = *(int *)(base + 0x3C);
    unsigned imageSize = *(unsigned *)(base + e_lfanew + 24 + 56);
    MEMORY_BASIC_INFORMATION mbi;
    if (imageSize < SLOT_RVA + 8 || !VirtualQuery(slot, &mbi, sizeof(mbi)) ||
        mbi.State != MEM_COMMIT || (mbi.Protect & (PAGE_NOACCESS | PAGE_GUARD))) {
        printf("FAIL：RVA %#x 不在镜像里或不可访问（镜像大小 %#x）——撑大镜像的 .bss 被优化掉了？\n",
               SLOT_RVA, imageSize);
        return 5;
    }
    printf("镜像大小=%#x，槽 %p 落在 [%p, %p) 区间内\n", imageSize, (void *)slot,
           (void *)mbi.BaseAddress, (void *)((unsigned char *)mbi.BaseAddress + mbi.RegionSize));

    HMODULE u32 = LoadLibraryW(L"user32.dll");
    void *real = (void *)GetProcAddress(u32, "GetKeyboardLayout");
    DWORD old = 0, tmp = 0;
    VirtualProtect(slot, sizeof(void *), PAGE_READWRITE, &old);
    *slot = real;
    VirtualProtect(slot, sizeof(void *), old, &tmp);

    printf("镜像基址=%p 槽=%p（RVA %#x）槽值=真 GetKeyboardLayout %p\n", (void *)base, (void *)slot,
           SLOT_RVA, real);
    printf("装插件前，通过槽调用 -> %#010llx\n",
           (unsigned long long)((GetLayoutFn)*slot)(0));

    HMODULE payload = LoadLibraryExW(argv[1], NULL, 0x900);
    if (!payload) {
        printf("LoadLibraryExW 失败：%lu\n", GetLastError());
        return 3;
    }
    typedef DWORD(WINAPI * StartFn)(void);
    StartFn start = (StartFn)GetProcAddress(payload, "StartLocalization");
    if (!start) {
        printf("插件没有 StartLocalization 导出\n");
        return 4;
    }
    start();
    Sleep(900); /* 插件每 200ms 扫一次槽 */

    void *now = *slot;
    HKL via = ((GetLayoutFn)now)(0);
    printf("装插件后，槽值=%p（%s）\n", now, (now == real) ? "没被改动" : "已被换成包装");
    printf("装插件后，通过槽调用 -> %#010llx\n", (unsigned long long)via);

    if (now != real && (DWORD)(ULONG_PTR)via == 0xE00E0804) {
        printf("PASS：客户端那条导入桩被接管，中文布局拿到了传统 IME HKL\n");
        return 0;
    }
    printf("FAIL：期望槽被换掉且返回 0xE00E0804（槽=%p 真值=%p，返回=%#llx）\n", now, real,
           (unsigned long long)via);
    return 1;
}
