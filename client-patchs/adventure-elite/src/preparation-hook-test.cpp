#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdio.h>
#include <string.h>
#include "../vendor/minhook/include/MinHook.h"
using Target = int (__fastcall *)(int);
static Target originalRip, originalCall, targetRip, targetCall;
static __declspec(thread) bool scoped;
static volatile LONG running = 1, failed;
static int __fastcall RipHook(int n) { return originalRip(n)+(scoped ? 100 : 0); }
static int __fastcall CallHook(int n) { return originalCall(n)+(scoped ? 100 : 0); }
static DWORD WINAPI Worker(void*) {
    while (InterlockedCompareExchange(&running, 0, 0))
        if (targetRip(7) != 307 || targetCall(7) != 12) InterlockedExchange(&failed, 1);
    return 0;
}
int main() {
    auto page = static_cast<unsigned char*>(VirtualAlloc(nullptr, 4096, MEM_RESERVE|MEM_COMMIT, PAGE_READWRITE));
    if (!page) return 1;
    // Native predicate needs RIP relocation; native flush needs CALL relocation.
    const unsigned char rip[] = {0x8b,0x05,0xfa,0x00,0x00,0x00,0x03,0xc1,0xc3};
    const unsigned char call[] = {0x48,0x83,0xec,0x28,0xe8,0x57,0x00,0x00,0x00,0x48,0x83,0xc4,0x28,0xc3};
    const unsigned char helper[] = {0x8b,0xc1,0x83,0xc0,0x05,0xc3};
    memcpy(page, rip, sizeof(rip)); memcpy(page+32, call, sizeof(call));
    memcpy(page+128, helper, sizeof(helper));
    const int value = 300; memcpy(page+256, &value, sizeof(value));
    DWORD old = 0;
    if (!VirtualProtect(page, 4096, PAGE_EXECUTE_READ, &old) || !FlushInstructionCache(GetCurrentProcess(), page, 4096)) return 2;
    targetRip = reinterpret_cast<Target>(page); targetCall = reinterpret_cast<Target>(page+32);
    if (targetRip(7) != 307 || targetCall(7) != 12 || MH_Initialize() != MH_OK) return 3;
    if (MH_CreateHook(page, reinterpret_cast<void*>(RipHook), reinterpret_cast<void**>(&originalRip)) != MH_OK ||
        MH_CreateHook(page+32, reinterpret_cast<void*>(CallHook), reinterpret_cast<void**>(&originalCall)) != MH_OK) return 4;
    HANDLE worker = CreateThread(nullptr, 0, Worker, nullptr, 0, nullptr);
    if (!worker) return 5;
    for (int i = 0; i < 8; ++i) {
        if (MH_QueueEnableHook(page) != MH_OK || MH_QueueEnableHook(page+32) != MH_OK || MH_ApplyQueued() != MH_OK) return 6;
        scoped = true;
        if (targetRip(7) != 407 || targetCall(7) != 112) return 7;
        scoped = false;
        if (targetRip(7) != 307 || targetCall(7) != 12) return 8;
        if (MH_QueueDisableHook(page) != MH_OK || MH_QueueDisableHook(page+32) != MH_OK || MH_ApplyQueued() != MH_OK) return 9;
    }
    InterlockedExchange(&running, 0);
    if (WaitForSingleObject(worker, 5000) != WAIT_OBJECT_0 || failed) return 10;
    CloseHandle(worker);
    if (MH_Uninitialize() != MH_OK || memcmp(page, rip, sizeof(rip)) || memcmp(page+32, call, sizeof(call))) return 11;
    VirtualFree(page, 0, MEM_RELEASE);
    puts("preparation hooks: RIP/CALL relocation, native forwarding, TLS separation, concurrent enable/disable and restoration passed");
    return 0;
}
