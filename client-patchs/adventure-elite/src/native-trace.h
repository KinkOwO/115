#pragma once
#include "notification-observer.h"
#include "notification-header.h"
#include "preparation-adapter.h"
#include "trace-record.h"
#include "lifecycle-trace.h"

static unsigned char* g_eliteImage;
static wchar_t g_eliteTracePath[32768];
static SRWLOCK g_eliteTraceLock = SRWLOCK_INIT;
static volatile LONG g_eliteTraceCount, g_eliteTraceDropped;
static volatile LONG g_eliteTraceOrdinals[3];
static unsigned long long g_eliteTraceRunTick;
static const char* g_eliteTraceFailure = "not-started";

static EliteTraceRecord EliteCaptureNative(unsigned int opcode, bool before,
        const ElitePreparationTLS& tls, bool success) {
    EliteTraceRecord s;
    s.entryChannel = tls.channel; s.entryOwner = tls.owner;
    if (g_prepareReady) { s.currentChannel = EliteNativeChannel(); s.currentOwner = EliteCachedOwner(); }
    s.active = tls.active; s.nonEmpty = tls.nonEmpty; s.sent = tls.sent; s.success = success;
    s.modeCalls = tls.modeCalls; s.predicateCalls = tls.predicateCalls;
    s.modeSeen = tls.modeSeen; s.predicateSeen = tls.predicateSeen; s.flushSeen = tls.flushSeen;
    s.reason = tls.reason ? tls.reason : "not-entered";
    s.exceptionCode = tls.exceptionCode; s.exceptionIp = tls.exceptionIp;
    memcpy(s.modeNative, tls.modeNative, sizeof(s.modeNative));
    memcpy(s.modeEffective, tls.modeEffective, sizeof(s.modeEffective));
    memcpy(s.predicateNative, tls.predicateNative, sizeof(s.predicateNative));
    memcpy(s.predicateEffective, tls.predicateEffective, sizeof(s.predicateEffective));
    AcquireSRWLockShared(&g_prepareLock);
    s.pendingThread = g_prepareState.thread; s.pendingChannel = g_prepareState.channel;
    s.pendingOwner = g_prepareState.owner; s.deadline = g_prepareState.deadline;
    s.waiting = g_prepareState.waiting; s.info = g_prepareState.info;
    ReleaseSRWLockShared(&g_prepareLock);
    uintptr_t cursor = 0, manager = 0;
    // Never advance the native reader.
    EliteCopy(g_eliteImage+0xF1BF878, &s.remaining, sizeof(s.remaining));
    if (before && s.remaining > 0 && EliteCopy(g_eliteImage+0xF1BF870, &cursor, sizeof(cursor))) {
        unsigned char prefix[16] = {};
        size_t length = s.remaining < 16 ? static_cast<size_t>(s.remaining) : 16;
        if (EliteCopy(reinterpret_cast<void*>(cursor), prefix, length)) {
            const auto h = ElitePeekNotificationHeader(opcode, prefix, length);
            s.first = h.count; s.second = h.mode; s.owner = h.owner;
            if (opcode == 1382 && length >= 4) s.companions = prefix[3];
        }
    }
    if (!EliteCopy(g_eliteImage+0xE638EF8, &manager, sizeof(manager)) || !manager) return s;
    EliteCopy(reinterpret_cast<void*>(manager+44), &s.preparedChannel, sizeof(s.preparedChannel));
    // Field +40 is logged without assigning an unverified gameplay meaning.
    EliteCopy(reinterpret_cast<void*>(manager+40), &s.managerField40, sizeof(s.managerField40));
    for (size_t i = 0; i < 3; ++i) {
        uintptr_t weak[2] = {};
        if (EliteCopy(reinterpret_cast<void*>(manager+72+24*i), weak, sizeof(weak)) && weak[0] && weak[1] &&
            EliteCopy(reinterpret_cast<void*>(weak[0]+8), &s.weakStrong[i], sizeof(s.weakStrong[i])) && s.weakStrong[i] > 0)
            ++s.weakAlive;
    }
    return s;
}

static void EliteTraceNotification(unsigned int opcode, const char* phase, LONG sequence, LONG ordinal,
        const EliteTraceRecord& s) {
    if (!TryAcquireSRWLockExclusive(&g_eliteTraceLock)) { InterlockedIncrement(&g_eliteTraceDropped); return; }
    char line[2048] = {};
    int length = EliteFormatTrace(line, sizeof(line), opcode, phase, sequence, ordinal,
        InterlockedCompareExchange(&g_eliteTraceDropped, 0, 0), g_eliteTraceRunTick, s);
    HANDLE file = length > 0 ? CreateFileW(g_eliteTracePath, FILE_APPEND_DATA, FILE_SHARE_READ|FILE_SHARE_WRITE,
        nullptr, OPEN_ALWAYS, FILE_ATTRIBUTE_NORMAL, nullptr) : INVALID_HANDLE_VALUE;
    DWORD written = 0;
    if (file == INVALID_HANDLE_VALUE || !WriteFile(file, line, static_cast<DWORD>(length), &written, nullptr) ||
        written != static_cast<DWORD>(length)) InterlockedIncrement(&g_eliteTraceDropped);
    if (file != INVALID_HANDLE_VALUE) CloseHandle(file);
    ReleaseSRWLockExclusive(&g_eliteTraceLock);
}

static void EliteObserve(unsigned int opcode, uintptr_t opaque, uintptr_t originalRva) {
    LONG sequence = InterlockedIncrement(&g_eliteTraceCount);
    LONG ordinal = InterlockedIncrement(&g_eliteTraceOrdinals[EliteTraceBucket(opcode)]);
    const ElitePreparationTLS previous = g_prepareTLS;
    bool success = false;
    bool record = ordinal <= EliteTraceBudget(opcode);
    ElitePreparationTLS arrival = {};
    arrival.reason = "arrival-before-identity";
    if (record) EliteTraceNotification(opcode, "arrival", sequence, ordinal, EliteCaptureNative(opcode, true, arrival, false));
    if (ordinal == EliteTraceBudget(opcode)+1)
        EliteTraceNotification(opcode, "trace-limit", sequence, ordinal, EliteCaptureNative(opcode, true, g_prepareTLS, false));
    // Preserve original ABI, body, ordering and exception propagation.
    __try {
        __try {
            EliteEnterPreparation(opcode);
            if (record) EliteTraceNotification(opcode, "before", sequence, ordinal, EliteCaptureNative(opcode, true, g_prepareTLS, false));
            reinterpret_cast<EliteNotification>(g_eliteImage+originalRva)(opcode, opaque);
            success = true;
        } __except (EliteRecordException(GetExceptionInformation(), g_prepareTLS.exceptionCode, g_prepareTLS.exceptionIp)) {
            // The filter always continues the original exception search.
        }
    } __finally {
        const ElitePreparationTLS completed = g_prepareTLS;
        // N1879's 142E5B2C1 existing-actor branch skips 1444FABF0 and
        // therefore its predicate entirely when all three slots are filled.
        // The active transaction already proves native N1754 -> N1382
        // (including all three info predicates) -> mode-2 done ordering.
        // Arm observations for either native branch; registration still
        // checks the current player, stable references and scene membership.
        if (success && opcode == 1879 && completed.active)
            EliteArmLifecycle(completed.channel,completed.owner);
        if (record) EliteTraceNotification(opcode, "after", sequence, ordinal, EliteCaptureNative(opcode, false, completed, success));
        EliteLeavePreparation(success, previous);
        if (record) EliteTraceNotification(opcode, "scope-closed", sequence, ordinal, EliteCaptureNative(opcode, false, completed, success));
    }
}
static void __fastcall EliteObserveSelections(unsigned int opcode, uintptr_t opaque) { EliteObserve(opcode, opaque, 0x2E5A4C0); }
static void __fastcall EliteObserveInfo(unsigned int opcode, uintptr_t opaque) { EliteObserve(opcode, opaque, 0x44FCF90); }
static void __fastcall EliteObserveDone(unsigned int opcode, uintptr_t opaque) { EliteObserve(opcode, opaque, 0x2E5B060); }

static bool EliteStartNativeTrace(unsigned char* image, const wchar_t* path) {
    const unsigned char selection[16] = {0x48,0x8b,0xc4,0x55,0x41,0x54,0x41,0x55,0x41,0x56,0x41,0x57,0x48,0x8d,0xa8,0xc8};
    const unsigned char info[16] = {0x48,0x8b,0xc4,0x55,0x41,0x54,0x41,0x55,0x41,0x56,0x41,0x57,0x48,0x8d,0xa8,0xf8};
    const uintptr_t rvas[] = {0x2E5A4C0, 0x44FCF90, 0x2E5B060};
    const unsigned char* signatures[] = {selection, info, selection};
    const char* failures[] = {"reader-signature-1754", "reader-signature-1382", "reader-signature-1879"};
    for (size_t i = 0; i < 3; ++i)
        if (!EliteReadable(image+rvas[i], 16) || memcmp(image+rvas[i], signatures[i], 16)) {
            g_eliteTraceFailure = failures[i]; return false;
        }
    g_eliteImage = image;
    if (wcscpy_s(g_eliteTracePath, path)) { g_eliteTraceFailure = "trace-path"; return false; }
    EliteNotificationBinding bindings[] = {
        {1754, reinterpret_cast<EliteNotification>(image+rvas[0]), EliteObserveSelections, nullptr},
        {1382, reinterpret_cast<EliteNotification>(image+rvas[1]), EliteObserveInfo, nullptr},
        {1879, reinterpret_cast<EliteNotification>(image+rvas[2]), EliteObserveDone, nullptr},
    };
    // Startup can precede native static registration. Bound the wait inside the
    // launcher's 30-second injection deadline, and never create missing entries.
    ULONGLONG deadline = GetTickCount64()+10000;
    g_eliteTraceFailure = "native-table-timeout-or-mismatch";
    do {
        uintptr_t table = 0;
        if (EliteCopy(image+0xE683700, &table, sizeof(table)) && table &&
            EliteNotificationSlots(reinterpret_cast<void*>(table), reinterpret_cast<uintptr_t>(image+0xA900660), bindings, 3)) {
            HMODULE pinned = nullptr;
            // Registered callbacks require the DLL to stay loaded until exit.
            if (!GetModuleHandleExW(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS|GET_MODULE_HANDLE_EX_FLAG_PIN,
                                    reinterpret_cast<LPCWSTR>(EliteObserveSelections), &pinned)) { g_eliteTraceFailure = "module-pin"; return false; }
            bool ready = EliteObserveNotifications(bindings, 3);
            g_eliteTraceFailure = ready ? "ready" : "callback-cas";
            return ready;
        }
        Sleep(25);
    } while (GetTickCount64() < deadline);
    return false;
}
