#pragma once
#include "lifecycle-snapshot.h"

static wchar_t g_lifecyclePath[32768];
static SRWLOCK g_lifecycleLock = SRWLOCK_INIT;
static unsigned int g_lifecycleThread, g_lifecycleChannel, g_lifecycleOwner;
static LONG g_lifecycleSamples, g_lifecycleDropped, g_eliteRoomSerial;
static unsigned long long g_lifecycleRunTick;
static bool g_lifecycleReady, g_lifecycleArmed;
static constexpr LONG kEliteLifecycleLimit = 1024;

// Read the current player's native controller CRef without acquiring or retaining it.
static bool EliteCurrentPlayerReference(uintptr_t* out) {
    uintptr_t holder = 0, control = 0, controller = 0, binding[3] = {};
    int strong = 0;
    if (!EliteCopy(g_prepareImage+0xE683C08,&holder,sizeof(holder)) || !holder ||
        !EliteCopy(reinterpret_cast<void*>(holder+144),&control,sizeof(control)) || !control ||
        !EliteCopy(reinterpret_cast<void*>(control+8),&strong,sizeof(strong)) || strong <= 0 ||
        !EliteCopy(reinterpret_cast<void*>(holder+152),&controller,sizeof(controller)) || !controller ||
        !EliteCopy(reinterpret_cast<void*>(controller+2072),binding,sizeof(binding)) || !binding[1] || binding[2] < 48 ||
        !EliteCopy(reinterpret_cast<void*>(binding[1]+8),&strong,sizeof(strong)) || strong <= 0) return false;
    out[0] = binding[1]; out[1] = binding[2];
    return true;
}
static void EliteLifecycleSample(const char* cause, uintptr_t caller) {
    if (!g_lifecycleReady) return;
    if (!TryAcquireSRWLockExclusive(&g_lifecycleLock)) { InterlockedIncrement(&g_lifecycleDropped); return; }
    if (!g_lifecycleArmed || GetCurrentThreadId() != g_lifecycleThread) { ReleaseSRWLockExclusive(&g_lifecycleLock); return; }
    LONG ordinal = ++g_lifecycleSamples;
    if (ordinal > kEliteLifecycleLimit+1) { ReleaseSRWLockExclusive(&g_lifecycleLock); return; }
    uintptr_t manager = 0;
    EliteCopy(g_prepareImage+0xE638EF8, &manager, sizeof(manager));
    uintptr_t player[2] = {}, playerAfter[2] = {};
    const bool playerAvailable = EliteCurrentPlayerReference(player);
    auto s = EliteReadCompanions(manager, EliteCopy,playerAvailable ? player : nullptr);
    if (playerAvailable && (!EliteCurrentPlayerReference(playerAfter) || player[0] != playerAfter[0] || player[1] != playerAfter[1])) s.consistent = false;
    unsigned int channel = EliteNativeChannel(), owner = EliteCachedOwner();
    bool match = channel == g_lifecycleChannel && owner == g_lifecycleOwner;
    char line[2600] = {};
    int length = sprintf_s(line, sizeof(line),
        "{\"traceVersion\":2,\"processId\":%lu,\"runTick\":%llu,\"ordinal\":%ld,\"tick\":%llu,\"thread\":%lu,"
        "\"cause\":\"%s\",\"callerRva\":%llu,\"traceDropped\":%ld,\"traceLimit\":%d,\"currentChannel\":%u,\"currentOwner\":%u,"
        "\"identityMatches\":%d,\"available\":%d,\"consistent\":%d,\"weakAlive\":%d,\"weakStrong\":[%d,%d,%d],"
        "\"actorKind\":[%d,%d,%d],\"controllerId\":[%d,%d,%d],\"controllerWire\":[%d,%d,%d],\"controllerBound\":[%d,%d,%d],"
        "\"identityWords\":[[%u,%u],[%u,%u],[%u,%u]],\"identityValid\":[%d,%d,%d],\"objectId\":[%lld,%lld,%lld],"
        "\"ownerStrong\":[%d,%d,%d],\"ownerKind\":[%d,%d,%d],\"ownerObjectId\":[%lld,%lld,%lld],"
        "\"ownerControllerId\":[%d,%d,%d],\"ownerIdentityValid\":[%d,%d,%d],\"sceneAttached\":[%d,%d,%d],\"ownerMatchesCurrentPlayer\":[%d,%d,%d]}\r\n",
        GetCurrentProcessId(), g_lifecycleRunTick, ordinal, GetTickCount64(), GetCurrentThreadId(), cause,
        static_cast<unsigned long long>(caller), InterlockedCompareExchange(&g_lifecycleDropped,0,0), ordinal > kEliteLifecycleLimit,
        channel, owner, match, s.available, s.consistent, s.alive, s.strong[0], s.strong[1], s.strong[2],
        s.kind[0], s.kind[1], s.kind[2], s.controllerId[0], s.controllerId[1], s.controllerId[2],
        s.controllerWire[0], s.controllerWire[1], s.controllerWire[2], s.controllerBound[0], s.controllerBound[1], s.controllerBound[2],
        s.identityWords[0][0],s.identityWords[0][1],s.identityWords[1][0],s.identityWords[1][1],s.identityWords[2][0],s.identityWords[2][1],
        s.identityValid[0],s.identityValid[1],s.identityValid[2],s.objectId[0],s.objectId[1],s.objectId[2],
        s.ownerStrong[0],s.ownerStrong[1],s.ownerStrong[2],s.ownerKind[0],s.ownerKind[1],s.ownerKind[2],
        s.ownerObjectId[0],s.ownerObjectId[1],s.ownerObjectId[2],s.ownerControllerId[0],s.ownerControllerId[1],s.ownerControllerId[2],
        s.ownerIdentityValid[0],s.ownerIdentityValid[1],s.ownerIdentityValid[2],s.sceneAttached[0],s.sceneAttached[1],s.sceneAttached[2],s.ownerMatchesCurrentPlayer[0],s.ownerMatchesCurrentPlayer[1],s.ownerMatchesCurrentPlayer[2]);
    HANDLE file = length > 0 ? CreateFileW(g_lifecyclePath, FILE_APPEND_DATA, FILE_SHARE_READ|FILE_SHARE_WRITE,
        nullptr, OPEN_ALWAYS, FILE_ATTRIBUTE_NORMAL, nullptr) : INVALID_HANDLE_VALUE;
    DWORD written = 0;
    if (file == INVALID_HANDLE_VALUE || !WriteFile(file,line,static_cast<DWORD>(length),&written,nullptr) || written != static_cast<DWORD>(length))
        InterlockedIncrement(&g_lifecycleDropped);
    if (file != INVALID_HANDLE_VALUE) CloseHandle(file);
    if (!match) g_lifecycleArmed = false;
    ReleaseSRWLockExclusive(&g_lifecycleLock);
}
static void EliteObserveNativeFlush(uintptr_t caller) { EliteLifecycleSample("native-flush",caller); }
static void EliteArmLifecycle(unsigned int channel, unsigned int owner) {
    if (!g_lifecycleReady || !channel || !owner) return;
    AcquireSRWLockExclusive(&g_lifecycleLock);
    g_lifecycleThread = GetCurrentThreadId(); g_lifecycleChannel = channel; g_lifecycleOwner = owner;
    g_lifecycleArmed = true;
    ReleaseSRWLockExclusive(&g_lifecycleLock);
    EliteLifecycleSample("native-load-completed",0);
}
static bool EliteStartLifecycleTrace(unsigned char* image, const wchar_t* path, unsigned long long runTick) {
    const unsigned char setter[] = {0x89,0x91,0x24,0x21,0x01,0x00,0xc3};
    const unsigned char getter[] = {0x8b,0x81,0x24,0x21,0x01,0x00,0xc3};
    const unsigned char context[] = {0x48,0x8b,0x81,0x18,0x8a,0x01,0x00,0xc3};
    const unsigned char identityDecoder[] = {0x8b,0x01,0x35,0x5c,0x02,0x2a,0x1f,0x83,0xe8,0x04,0x89,0x02,0xc3};
    const unsigned char ownerSetter[] = {0x40,0x57,0x48,0x83,0xec,0x50,0x48,0xc7,0x44,0x24,0x20,0xfe,0xff,0xff,0xff,0x48};
    const unsigned char sceneGetter[] = {0x48,0x8b,0x81,0x90,0x01,0x00,0x00,0x48,0x85,0xc0,0x74,0x0e,0x83,0x78,0x08,0x00,0x74,0x08,0x48,0x8b,0x81,0x98,0x01,0x00,0x00,0xc3,0x33,0xc0,0xc3};
    const unsigned char uiFlushA[] = {0xe8,0xc6,0xf2,0x0f,0x03,0xe8,0xd1,0xd7,0x0f,0x03,0xba,0x73};
    const unsigned char uiFlushB[] = {0xe8,0x41,0xf2,0x0f,0x03,0x90,0x48,0x8b,0x4b,0x38,0x48,0x85};
    if (!EliteReadable(image+0x6E920A0,sizeof(identityDecoder)) || memcmp(image+0x6E920A0,identityDecoder,sizeof(identityDecoder)) ||
        !EliteReadable(image+0x5C77BB0,sizeof(ownerSetter)) || memcmp(image+0x5C77BB0,ownerSetter,sizeof(ownerSetter)) ||
        !EliteReadable(image+0x5B8B120,sizeof(sceneGetter)) || memcmp(image+0x5B8B120,sceneGetter,sizeof(sceneGetter)) ||
        !EliteReadable(image+0x5D369B0,sizeof(setter)) || memcmp(image+0x5D369B0,setter,sizeof(setter)) ||
        !EliteReadable(image+0x5CE2860,sizeof(getter)) || memcmp(image+0x5CE2860,getter,sizeof(getter)) ||
        !EliteReadable(image+0x5C6C080,sizeof(context)) || memcmp(image+0x5C6C080,context,sizeof(context)) ||
        !EliteReadable(image+0x3C76825,sizeof(uiFlushA)) || memcmp(image+0x3C76825,uiFlushA,sizeof(uiFlushA)) ||
        !EliteReadable(image+0x3C768AA,sizeof(uiFlushB)) || memcmp(image+0x3C768AA,uiFlushB,sizeof(uiFlushB))) return false;
    if (wcscpy_s(g_lifecyclePath,path)) return false;
    g_lifecycleRunTick = runTick;
    g_prepareFlushObservation = EliteObserveNativeFlush;
    g_lifecycleReady = true;
    return true;
}
