#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdio.h>
#include <initializer_list>
#include "native-trace.h"

static unsigned int nativeCalls, predicateBits;
static void __fastcall DoneFixture(unsigned int, uintptr_t) {
    ++nativeCalls;
    g_prepareTLS.predicateCalls = predicateBits;
}
static uintptr_t __fastcall OwnerFixture(uintptr_t) { return 0; }
static bool CommitPage(unsigned char* image, size_t offset) {
    return VirtualAlloc(image + (offset & ~size_t(4095)), 4096,
        MEM_COMMIT, PAGE_EXECUTE_READWRITE) != nullptr;
}
template<typename T> static void Put(unsigned char* image, size_t offset, const T& value) {
    memcpy(image + offset, &value, sizeof(value));
}
static void Jump(unsigned char* site, uintptr_t target) {
    const unsigned char jump[] = {0x48,0xb8,0,0,0,0,0,0,0,0,0xff,0xe0};
    memcpy(site, jump, sizeof(jump)); memcpy(site + 2, &target, sizeof(target));
}
int main() {
    // Execute only synthetic callbacks in this process, never the client.
    auto own = reinterpret_cast<unsigned char*>(GetModuleHandleW(nullptr));
    if (EliteStartNativeTrace(own, L"unused") || EliteStartPreparationAdapter(own) ||
        EliteStartLifecycleTrace(own, L"unused", 0)) return 8;
    auto image = static_cast<unsigned char*>(VirtualAlloc(nullptr, 0xF1C0000,
        MEM_RESERVE, PAGE_NOACCESS));
    if (!image) return 1;
    const size_t pages[] = {0x2E5B060,0x5F0BA60,0xE638EF8,0xE66C090,0xE683C08,0xF1BF870};
    for (size_t offset : pages) if (!CommitPage(image, offset)) return 2;
    Jump(image + 0x2E5B060, reinterpret_cast<uintptr_t>(DoneFixture));
    Jump(image + 0x5F0BA60, reinterpret_cast<uintptr_t>(OwnerFixture));
    if (!FlushInstructionCache(GetCurrentProcess(), image, 0xF1C0000)) return 3;
    unsigned char channel[4540] = {}, holder[160] = {}, controller[120] = {}, control[16] = {}, manager[160] = {};
    const unsigned int channelId = 22; const unsigned short ownerId = 4; const int strong = 1;
    memcpy(channel + 4528, &channelId, sizeof(channelId));
    memcpy(controller + 112, &ownerId, sizeof(ownerId));
    memcpy(control + 8, &strong, sizeof(strong));
    const uintptr_t controlPtr = reinterpret_cast<uintptr_t>(control), controllerPtr = reinterpret_cast<uintptr_t>(controller);
    memcpy(holder + 144, &controlPtr, 8); memcpy(holder + 152, &controllerPtr, 8);
    Put(image, 0xE66C090, reinterpret_cast<uintptr_t>(channel));
    Put(image, 0xE683C08, reinterpret_cast<uintptr_t>(holder));
    Put(image, 0xE638EF8, reinterpret_cast<uintptr_t>(manager));
    unsigned char body[16] = {2,0};
    Put(image, 0xF1BF870, reinterpret_cast<uintptr_t>(body));
    g_eliteImage = g_prepareImage = image; g_prepareReady = 1;
    g_lifecycleReady = true; g_lifecycleRunTick = g_eliteTraceRunTick = 123;
    wchar_t directory[32768] = {};
    if (!GetModuleFileNameW(nullptr, directory, ARRAYSIZE(directory))) return 4;
    wchar_t* slash = wcsrchr(directory, L'\\'); if (!slash) return 4; *(slash + 1) = 0;
    if (swprintf_s(g_eliteTracePath, L"%snative-completion-notifications.jsonl", directory) < 0 ||
        swprintf_s(g_lifecyclePath, L"%snative-completion-lifecycle.jsonl", directory) < 0) return 4;
    // The actual N1879 takes its existing-actor branch for all three members.
    // That valid path has zero create-actor predicate calls; partial rosters
    // may instead hit the predicate while iterating an empty native slot.
    for (unsigned int bits : {0u,8u}) {
        predicateBits = bits; g_lifecycleArmed = false;
        EliteArmPreparation(g_prepareState, GetCurrentThreadId(), 22, 4, GetTickCount64());
        g_prepareState.info = true;
        Put(image, 0xF1BF878, int(sizeof(body)));
        const LONG before = g_lifecycleSamples;
        EliteObserveDone(1879, 0);
        if (!g_lifecycleArmed || g_lifecycleSamples != before + 1 || g_prepareState.waiting) return 5;
    }
    // Missing info, expired request and another owner's header must not arm.
    for (unsigned int invalid = 0; invalid < 3; ++invalid) {
        g_lifecycleArmed = false;
        EliteArmPreparation(g_prepareState, GetCurrentThreadId(), 22, 4, GetTickCount64());
        g_prepareState.info = invalid != 0;
        if (invalid == 1) g_prepareState.deadline = 0;
        body[0] = invalid == 2 ? 4 : 2;
        Put(image, 0xF1BF878, int(sizeof(body)));
        const LONG before = g_lifecycleSamples;
        EliteObserveDone(1879, 0);
        if (g_lifecycleArmed || g_lifecycleSamples != before) return 6;
    }
    if (nativeCalls != 5) return 7;
    VirtualFree(image, 0, MEM_RELEASE);
    puts("native completion: original callback once; zero/eight predicate paths arm; missing-info, expired and wrong-mode requests remain blocked");
    return 0;
}
