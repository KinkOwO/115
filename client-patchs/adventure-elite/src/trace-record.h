#pragma once
#include <windows.h>
#include <stdio.h>

// Diagnostics only: independent budgets keep repeated selections from hiding
// the later info/done notifications. Never change native execution at the cap.
inline LONG EliteTraceBudget(unsigned int opcode) { return opcode == 1754 ? 128 : 64; }
inline size_t EliteTraceBucket(unsigned int opcode) { return opcode == 1754 ? 0 : opcode == 1382 ? 1 : 2; }
struct EliteTraceRecord {
    int remaining = -1, first = -1, second = -1, owner = -1, companions = -1;
    int preparedChannel = -1, managerField40 = -1, weakAlive = 0;
    int weakStrong[3] = {-1, -1, -1};
    unsigned int currentChannel = 0, currentOwner = 0, entryChannel = 0, entryOwner = 0;
    bool active = false, nonEmpty = false, sent = false, success = false;
    unsigned int modeCalls = 0, predicateCalls = 0, modeSeen = 0, predicateSeen = 0;
    int modeNative[2] = {}, modeEffective[2] = {};
    int predicateNative[4] = {}, predicateEffective[4] = {};
    bool flushSeen = false;
    const char* reason = "not-entered";
    unsigned int pendingThread = 0, pendingChannel = 0, pendingOwner = 0;
    unsigned long long deadline = 0;
    bool waiting = false, info = false;
    DWORD exceptionCode = 0;
    unsigned long long exceptionIp = 0;
};
inline LONG EliteRecordException(EXCEPTION_POINTERS* pointers, DWORD& code, unsigned long long& ip) {
    if (pointers && pointers->ExceptionRecord) {
        code = pointers->ExceptionRecord->ExceptionCode;
        ip = reinterpret_cast<unsigned long long>(pointers->ExceptionRecord->ExceptionAddress);
    }
    return EXCEPTION_CONTINUE_SEARCH; // Observe the escaping exception, never swallow it.
}
inline int EliteFormatTrace(char* out, size_t capacity, unsigned int opcode, const char* phase,
        LONG sequence, LONG ordinal, LONG dropped, unsigned long long runTick, const EliteTraceRecord& s) {
    return sprintf_s(out, capacity,
        "{\"traceVersion\":2,\"processId\":%lu,\"runTick\":%llu,\"sequence\":%ld,\"ordinal\":%ld,"
        "\"tick\":%llu,\"thread\":%lu,\"opcode\":%u,\"phase\":\"%s\",\"traceDropped\":%ld,"
        "\"remaining\":%d,\"headerCount\":%d,\"headerMode\":%d,\"ownerWireId\":%d,\"companionCount\":%d,"
        "\"currentChannel\":%u,\"currentOwner\":%u,\"entryChannel\":%u,\"entryOwner\":%u,\"preparedChannel\":%d,\"managerField40\":%d,"
        "\"weakAlive\":%d,\"weakStrong\":[%d,%d,%d],\"preparationScope\":%d,\"scopeReason\":\"%s\","
        "\"nonEmpty\":%d,\"modeSeen\":%u,\"modeCalls\":%u,\"modeNative\":[%d,%d],\"modeEffective\":[%d,%d],"
        "\"predicateSeen\":%u,\"predicateCalls\":%u,\"predicateNative\":[%d,%d,%d,%d],"
        "\"predicateEffective\":[%d,%d,%d,%d],\"flushSeen\":%d,\"requestObserved\":%d,\"readerSuccess\":%d,\"exceptionCode\":%lu,\"exceptionIp\":%llu,"
        "\"pending\":{\"thread\":%u,\"channel\":%u,\"owner\":%u,\"waiting\":%d,\"info\":%d,\"deadline\":%llu}}\r\n",
        GetCurrentProcessId(), runTick, sequence, ordinal, GetTickCount64(), GetCurrentThreadId(), opcode, phase, dropped,
        s.remaining, s.first, s.second, s.owner, s.companions, s.currentChannel, s.currentOwner, s.entryChannel, s.entryOwner, s.preparedChannel, s.managerField40,
        s.weakAlive, s.weakStrong[0], s.weakStrong[1], s.weakStrong[2], s.active, s.reason, s.nonEmpty,
        s.modeSeen, s.modeCalls, s.modeNative[0], s.modeNative[1], s.modeEffective[0], s.modeEffective[1],
        s.predicateSeen, s.predicateCalls, s.predicateNative[0], s.predicateNative[1], s.predicateNative[2], s.predicateNative[3],
        s.predicateEffective[0], s.predicateEffective[1], s.predicateEffective[2], s.predicateEffective[3],
        s.flushSeen, s.sent, s.success, s.exceptionCode, s.exceptionIp, s.pendingThread, s.pendingChannel, s.pendingOwner, s.waiting, s.info, s.deadline);
}
