#define WIN32_LEAN_AND_MEAN
#define _CRT_SECURE_NO_WARNINGS
#include <limits.h>
#include <string.h>
#include "trace-record.h"
int main() {
    if (EliteTraceBucket(1754) != 0 || EliteTraceBucket(1382) != 1 || EliteTraceBucket(1879) != 2 ||
        EliteTraceBudget(1754) != 128 || EliteTraceBudget(1382) != 64 || EliteTraceBudget(1879) != 64) return 1;
    DWORD escapedCode = 0;
    unsigned long long escapedIp = 0;
    bool propagated = false;
    __try {
        __try { RaiseException(0xe0424242, 0, 0, nullptr); }
        __except (EliteRecordException(GetExceptionInformation(), escapedCode, escapedIp)) { return 3; }
    } __except (EXCEPTION_EXECUTE_HANDLER) { propagated = true; }
    if (!propagated || escapedCode != 0xe0424242 || !escapedIp) return 4;
    EliteTraceRecord s;
    s.remaining = INT_MAX; s.currentChannel = UINT_MAX; s.entryOwner = 65535;
    s.modeSeen = 3; s.modeCalls = 3; s.modeNative[0] = 0; s.modeEffective[0] = 2;
    s.weakStrong[0] = INT_MAX; s.reason = "pending-owner-mismatch";
    s.waiting = true; s.deadline = ~0ull; s.success = false;
    char text[2048] = {};
    const char* phases[] = {"before", "after", "scope-closed", "trace-limit"};
    for (const char* phase : phases) {
        int n = EliteFormatTrace(text, sizeof(text), 1754, phase, LONG_MAX, 129, 2, ~0ull, s);
        if (n <= 0 || n >= static_cast<int>(sizeof(text)) || text[n-1] != '\n' ||
            !strstr(text, "\"traceVersion\":2") || !strstr(text, "\"traceDropped\":2") ||
            !strstr(text, "\"scopeReason\":\"pending-owner-mismatch\"") ||
            !strstr(text, "\"modeEffective\":[2,0]") || !strstr(text, "\"readerSuccess\":0") ||
            !strstr(text, "\"deadline\":18446744073709551615")) return 2;
        fputs(text, stdout); // Parsed by the offline collector test, no game.
    }
    return 0;
}
