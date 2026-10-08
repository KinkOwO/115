#pragma once
#include "notification-observer.h"
#include "preparation-state.h"
#include "../vendor/minhook/include/MinHook.h"
#include <intrin.h>

using EliteNativeMode = __int64 (__fastcall *)();
using EliteNativePredicate = char (__fastcall *)();
static unsigned char* g_prepareImage;
static volatile LONG g_prepareReady;
static EliteNativeMode g_originalMode;
static EliteNativePredicate g_originalPredicate;
// Registration extends this owned hook; never hook 142E60CF0 a second time.
static char (*g_preparePredicateRegistration)(uintptr_t,char);
static EliteNativeMode g_originalFlush;
static void (*g_prepareFlushObservation)(uintptr_t);
static SRWLOCK g_prepareLock = SRWLOCK_INIT;
static ElitePreparationState g_prepareState;
static const char* g_prepareFailure = "not-started";
static int g_prepareHookStatus = 0;
static bool ElitePreparationReject(const char* reason) { g_prepareFailure = reason; return false; }
struct ElitePreparationTLS {
    unsigned int opcode;
    unsigned int channel;
    unsigned short owner;
    bool active;
    bool nonEmpty;
    unsigned int modeCalls;
    unsigned int predicateCalls;
    bool sent;
    unsigned int modeSeen, predicateSeen;
    int modeNative[2], modeEffective[2];
    int predicateNative[4], predicateEffective[4];
    bool flushSeen;
    const char* reason;
    DWORD exceptionCode;
    unsigned long long exceptionIp;
};
static __declspec(thread) ElitePreparationTLS g_prepareTLS;

static unsigned int EliteNativeChannel() {
    uintptr_t manager = 0;
    unsigned int channel = 0;
    if (EliteCopy(g_prepareImage+0xE66C090, &manager, sizeof(manager)) && manager)
        EliteCopy(reinterpret_cast<void*>(manager+4528), &channel, sizeof(channel));
    return channel;
}
static unsigned short EliteCachedOwner(uintptr_t holder = 0) {
    uintptr_t control = 0, controller = 0;
    int strong = 0;
    unsigned short owner = 0;
    if (!holder && (!EliteCopy(g_prepareImage+0xE683C08, &holder, sizeof(holder)) || !holder)) return 0;
    if (EliteCopy(reinterpret_cast<void*>(holder+144), &control, sizeof(control)) && control &&
        EliteCopy(reinterpret_cast<void*>(control+8), &strong, sizeof(strong)) && strong > 0 &&
        EliteCopy(reinterpret_cast<void*>(holder+152), &controller, sizeof(controller)) && controller)
        EliteCopy(reinterpret_cast<void*>(controller+112), &owner, sizeof(owner));
    return owner == 65535 ? 0 : owner;
}
static unsigned short EliteNativeOwner() {
    // Same native getter as v0.3.0. Diagnostics only peek the cache and never
    // invoke this getter a second time to manufacture an identity.
    uintptr_t holder = 0;
    if (!EliteCopy(g_prepareImage+0xE683C08, &holder, sizeof(holder)) || !holder) return 0;
    using CurrentController = uintptr_t (__fastcall *)(uintptr_t);
    reinterpret_cast<CurrentController>(g_prepareImage+0x5F0BA60)(holder);
    return EliteCachedOwner(holder);
}

static __declspec(noinline) __int64 __fastcall ElitePreparedMode() {
    uintptr_t caller = reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage);
    __int64 result = g_originalMode();
    bool observed = g_prepareTLS.opcode == 1754 && ElitePreparationCaller(1754, caller);
    size_t index = caller == 0x2E5A5A2 ? 0 : 1;
    if (observed) {
        g_prepareTLS.modeSeen |= 1u << index;
        g_prepareTLS.modeNative[index] = static_cast<int>(result);
        g_prepareTLS.modeEffective[index] = static_cast<int>(result);
    }
    if (g_prepareReady && result == 0 && g_prepareTLS.active &&
        ElitePreparationCaller(g_prepareTLS.opcode, caller) && g_prepareTLS.opcode == 1754 &&
        EliteNativeChannel() == g_prepareTLS.channel && EliteNativeOwner() == g_prepareTLS.owner) {
        g_prepareTLS.modeCalls |= caller == 0x2E5A5A2 ? 1u : 2u;
        g_prepareTLS.modeEffective[index] = 2;
        return 2;
    }
    return result;
}
static __declspec(noinline) char __fastcall ElitePreparedPredicate() {
    uintptr_t caller = reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage);
    char result = g_originalPredicate();
    bool observed = g_prepareTLS.opcode != 1754 && ElitePreparationCaller(g_prepareTLS.opcode, caller);
    size_t index = caller == 0x44FD019 ? 0 : caller == 0x44FD1CF ? 1 : caller == 0x44FDB47 ? 2 : 3;
    if (observed) {
        g_prepareTLS.predicateSeen |= 1u << index;
        g_prepareTLS.predicateNative[index] = result;
        g_prepareTLS.predicateEffective[index] = result;
    }
    if (g_prepareReady && g_prepareTLS.active && g_prepareTLS.opcode != 1754 &&
        ElitePreparationCaller(g_prepareTLS.opcode, caller) &&
        EliteNativeChannel() == g_prepareTLS.channel && EliteNativeOwner() == g_prepareTLS.owner) {
        g_prepareTLS.predicateCalls |= caller == 0x44FD019 ? 1u : caller == 0x44FD1CF ? 2u : caller == 0x44FDB47 ? 4u : 8u;
        g_prepareTLS.predicateEffective[index] = 1;
        return 1;
    }
    return g_preparePredicateRegistration ? g_preparePredicateRegistration(caller,result) : result;
}
static __declspec(noinline) __int64 __fastcall ElitePreparedFlush() {
    uintptr_t caller = reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage);
    if (g_prepareTLS.opcode == 1754 && caller == 0x2E5AEF0) g_prepareTLS.flushSeen = true;
    // This exact native flush follows CMD1811 + u16(mode) in N1754.
    // Merely receiving settings or reusing a valid snapshot does not arm a load.
    if (g_prepareReady && caller == 0x2E5AEF0 && g_prepareTLS.active &&
        g_prepareTLS.opcode == 1754 && g_prepareTLS.modeCalls == 3 && g_prepareTLS.nonEmpty &&
        EliteNativeChannel() == g_prepareTLS.channel && EliteNativeOwner() == g_prepareTLS.owner) {
        AcquireSRWLockExclusive(&g_prepareLock);
        EliteArmPreparation(g_prepareState, GetCurrentThreadId(), g_prepareTLS.channel,
                            g_prepareTLS.owner, GetTickCount64());
        ReleaseSRWLockExclusive(&g_prepareLock);
        g_prepareTLS.sent = true;
    }
    __int64 result = g_originalFlush();
    // Observe only after the unmodified native request was flushed. No packet
    // inspection or sending; caller RVA is evidence, not an opcode guess.
    DWORD nativeError = GetLastError();
    if (g_prepareFlushObservation) g_prepareFlushObservation(caller);
    SetLastError(nativeError);
    return result;
}

static void EliteEnterPreparation(unsigned int opcode) {
    g_prepareTLS = {};
    g_prepareTLS.reason = "adapter-not-ready";
    if (!g_prepareReady) return;
    g_prepareTLS.opcode = opcode;
    g_prepareTLS.channel = EliteNativeChannel();
    g_prepareTLS.owner = EliteNativeOwner();
    uintptr_t cursor = 0;
    int remaining = 0;
    unsigned char body[1+4*531] = {};
    g_prepareTLS.reason = "identity-unavailable";
    if (!g_prepareTLS.channel || !g_prepareTLS.owner) return;
    g_prepareTLS.reason = "reader-unavailable";
    if (!EliteCopy(g_prepareImage+0xF1BF870, &cursor, sizeof(cursor)) ||
        !EliteCopy(g_prepareImage+0xF1BF878, &remaining, sizeof(remaining)) || remaining <= 0) return;
    size_t length = remaining < static_cast<int>(sizeof(body)) ? static_cast<size_t>(remaining) : sizeof(body);
    g_prepareTLS.reason = "body-copy-failed";
    if (!EliteCopy(reinterpret_cast<void*>(cursor), body, length)) return;
    AcquireSRWLockExclusive(&g_prepareLock);
    if (opcode == 1754) {
        g_prepareState = {};
        g_prepareTLS.active = ElitePreparationSelection(body, length, g_prepareTLS.nonEmpty);
        g_prepareTLS.reason = g_prepareTLS.active ? (g_prepareTLS.nonEmpty ? "selection-nonempty" : "selection-empty") : "selection-invalid-or-no-mode2";
    } else if (ElitePreparationMatches(g_prepareState, GetCurrentThreadId(), g_prepareTLS.channel,
                                       g_prepareTLS.owner, GetTickCount64())) {
        g_prepareTLS.reason = "transaction-header-or-order-mismatch";
        if (opcode == 1382 && !g_prepareState.info && length >= 4 && body[0] == 1 &&
            (body[1] | (body[2] << 8)) == g_prepareTLS.owner && body[3] > 0 && body[3] <= 3)
            g_prepareTLS.active = true;
        if (opcode == 1879 && g_prepareState.info && length >= 2 && body[0] == 2 && body[1] == 0)
            g_prepareTLS.active = true;
        if (g_prepareTLS.active) g_prepareTLS.reason = "transaction-matched";
    } else {
        g_prepareTLS.reason = !g_prepareState.waiting ? "no-pending-native-request" :
            GetCurrentThreadId() != g_prepareState.thread ? "pending-thread-mismatch" :
            g_prepareTLS.channel != g_prepareState.channel ? "pending-channel-mismatch" :
            g_prepareTLS.owner != g_prepareState.owner ? "pending-owner-mismatch" : "pending-expired";
        g_prepareState = {};
    }
    ReleaseSRWLockExclusive(&g_prepareLock);
}
static void EliteLeavePreparation(bool success, const ElitePreparationTLS& previous) {
    if (g_prepareTLS.active && g_prepareTLS.opcode != 1754) {
        AcquireSRWLockExclusive(&g_prepareLock);
        if (success && g_prepareTLS.opcode == 1382 && g_prepareTLS.predicateCalls == 7 &&
            ElitePreparationMatches(g_prepareState, GetCurrentThreadId(), g_prepareTLS.channel,
                                    g_prepareTLS.owner, GetTickCount64()))
            g_prepareState.info = true;
        else g_prepareState = {};
        ReleaseSRWLockExclusive(&g_prepareLock);
    }
    if (!success && g_prepareTLS.opcode == 1754) {
        AcquireSRWLockExclusive(&g_prepareLock);
        g_prepareState = {};
        ReleaseSRWLockExclusive(&g_prepareLock);
    }
    g_prepareTLS = previous;
}


static bool EliteStartPreparationAdapter(unsigned char* image) {
    static const unsigned char expected0[] = {0x40,0x57,0x48,0x83,0xec,0x30,0x48,0xc7,0x44,0x24,0x20,0xfe,0xff,0xff,0xff,0x48,0x89,0x5c,0x24,0x40,0x33,0xdb,0x8b,0xfb,0x48,0x8b,0xd,0xd1,0xd2,0x80,0xb};
    static const unsigned char expected1[] = {0x48,0x83,0xec,0x28,0x48,0x8b,0xd,0x95,0xb3,0x80,0xb,0xe8,0xf0,0xe,0x3e,0x2,0x83,0xe8,0x44,0x74,0x2a,0x83,0xe8,0x5,0x74,0x25,0x83,0xe8,0x1,0x74,0x20,0x83,0xe8,0x2,0x74,0x1b,0x83,0xf8,0x2,0x74,0x16,0xe8,0xe2,0x94,0x3b,0xfd,0x48,0x8b,0xc8,0xe8,0xaa,0xbf,0x8e,0xfd,0x84,0xc0,0x75,0x5,0x48,0x83,0xc4,0x28,0xc3,0xb0,0x1,0x48,0x83,0xc4,0x28,0xc3};
    static const unsigned char expected2[] = {0xe8,0xfe,0x47,0x0,0x0,0x8b,0xf8,0x89,0xb5,0xad,0x1,0x0};
    static const unsigned char expected3[] = {0xe8,0xb6,0x42,0x0,0x0,0x8b,0xf8,0x4d,0x8b,0x7,0x49,0x8b};
    static const unsigned char expected4[] = {0xe8,0xd7,0x3c,0x96,0xfe,0x44,0x8b,0xff,0xb9,0x2,0x0,0x0};
    static const unsigned char expected5[] = {0xe8,0x21,0x3b,0x96,0xfe,0x84,0xc0,0xf,0x85,0xff,0x6,0x0};
    static const unsigned char expected6[] = {0xe8,0xa9,0x31,0x96,0xfe,0x84,0xc0,0xf,0x84,0xf1,0x0,0x0};
    static const unsigned char expected7[] = {0xe8,0x85,0x60,0x96,0xfe,0x84,0xc0,0xf,0x84,0x45,0x6,0x0};
    static const unsigned char expected8[] = {0x48,0x83,0xec,0x28,0xe8,0x7,0xe5,0xff,0xff,0x48,0x8b,0xc8,0x48,0x83,0xc4,0x28,0xe9,0x6b,0xdf,0xff,0xff,0xcc,0xcc,0xcc};
    static const unsigned char expected9[] = {0xe8,0x0,0xac,0xf1,0x3,0x48,0x8b,0x8d,0x0,0x8,0x0,0x0};
    if (!EliteReadable(image+0x2e5eda0, sizeof(expected0)) ||
        memcmp(image+0x2e5eda0, expected0, sizeof(expected0))) return ElitePreparationReject("bytes-0x2e5eda0");
    if (!EliteReadable(image+0x2e60cf0, sizeof(expected1)) ||
        memcmp(image+0x2e60cf0, expected1, sizeof(expected1))) return ElitePreparationReject("bytes-0x2e60cf0");
    if (!EliteReadable(image+0x2e5a59d, sizeof(expected2)) ||
        memcmp(image+0x2e5a59d, expected2, sizeof(expected2))) return ElitePreparationReject("bytes-0x2e5a59d");
    if (!EliteReadable(image+0x2e5aae5, sizeof(expected3)) ||
        memcmp(image+0x2e5aae5, expected3, sizeof(expected3))) return ElitePreparationReject("bytes-0x2e5aae5");
    if (!EliteReadable(image+0x44fd014, sizeof(expected4)) ||
        memcmp(image+0x44fd014, expected4, sizeof(expected4))) return ElitePreparationReject("bytes-0x44fd014");
    if (!EliteReadable(image+0x44fd1ca, sizeof(expected5)) ||
        memcmp(image+0x44fd1ca, expected5, sizeof(expected5))) return ElitePreparationReject("bytes-0x44fd1ca");
    if (!EliteReadable(image+0x44fdb42, sizeof(expected6)) ||
        memcmp(image+0x44fdb42, expected6, sizeof(expected6))) return ElitePreparationReject("bytes-0x44fdb42");
    if (!EliteReadable(image+0x44fac66, sizeof(expected7)) ||
        memcmp(image+0x44fac66, expected7, sizeof(expected7))) return ElitePreparationReject("bytes-0x44fac66");
    if (!EliteReadable(image+0x6d75af0, sizeof(expected8)) ||
        memcmp(image+0x6d75af0, expected8, sizeof(expected8))) return ElitePreparationReject("bytes-0x6d75af0");
    if (!EliteReadable(image+0x2e5aeeb, sizeof(expected9)) ||
        memcmp(image+0x2e5aeeb, expected9, sizeof(expected9))) return ElitePreparationReject("bytes-0x2e5aeeb");
    static const unsigned char ownerGetter[] = {0x40,0x57,0x48,0x83,0xec,0x50,0x48,0xc7,0x44,0x24,0x20,0xfe,0xff,0xff,0xff,0x48,0x89,0x5c,0x24,0x68,0x48,0x8b,0xd9,0x48,0x8b,0x81,0x90,0x00,0x00,0x00};
    if (!EliteReadable(image+0x5F0BA60, sizeof(ownerGetter)) ||
        memcmp(image+0x5F0BA60, ownerGetter, sizeof(ownerGetter))) return ElitePreparationReject("bytes-0x5F0BA60");
    g_prepareImage = image;
    g_prepareHookStatus = MH_Initialize();
    if (g_prepareHookStatus != MH_OK) return ElitePreparationReject("hook-initialize");
    void* targets[] = {image+0x2E5EDA0, image+0x2E60CF0, image+0x6D75AF0};
    void* replacements[] = {reinterpret_cast<void*>(ElitePreparedMode), reinterpret_cast<void*>(ElitePreparedPredicate), reinterpret_cast<void*>(ElitePreparedFlush)};
    void** originals[] = {reinterpret_cast<void**>(&g_originalMode), reinterpret_cast<void**>(&g_originalPredicate), reinterpret_cast<void**>(&g_originalFlush)};
    bool ready = true;
    for (size_t i = 0; i < 3; ++i) {
        g_prepareHookStatus = MH_CreateHook(targets[i], replacements[i], originals[i]);
        if (g_prepareHookStatus != MH_OK) { g_prepareFailure = "hook-create"; ready = false; break; }
        g_prepareHookStatus = MH_QueueEnableHook(targets[i]);
        if (g_prepareHookStatus != MH_OK) { g_prepareFailure = "hook-queue"; ready = false; break; }
    }
    if (ready) { g_prepareHookStatus = MH_ApplyQueued(); ready = g_prepareHookStatus == MH_OK;
        if (!ready) g_prepareFailure = "hook-apply"; }
    if (!ready) { MH_DisableHook(MH_ALL_HOOKS); MH_Uninitialize(); return false; }
    g_prepareFailure = "ready";
    InterlockedExchange(&g_prepareReady, 1);
    return true;
}
