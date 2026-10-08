#pragma once
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <intrin.h>
#include <string.h>
#include "eligibility-sites.h"

struct ElitePatchResult {
    bool ready;
    bool enabled;
    unsigned changed;
    const char* reason;
};

inline bool EliteReadable(const unsigned char* p, size_t length) {
    MEMORY_BASIC_INFORMATION m = {};
    if (VirtualQuery(p, &m, sizeof(m)) != sizeof(m) || m.State != MEM_COMMIT ||
        (m.Protect & (PAGE_GUARD | PAGE_NOACCESS))) return false;
    return p >= m.BaseAddress && length <= (size_t)((unsigned char*)m.BaseAddress + m.RegionSize - p);
}

// Targets are immediate bytes, never opcodes/displacements. Each update is
// atomic; surrounding instructions and protected level getters stay intact.
inline ElitePatchResult EliteEligibilityPatch(unsigned char* const* targets, bool apply) {
    ElitePatchResult result = {false, false, 0, "context-mismatch"};
    DWORD protections[5] = {};
    size_t acquired = 0;
    for (size_t i = 0; i < kEliteEligibilitySiteCount; ++i) {
        const EliteEligibilitySite& s = kEliteEligibilitySites[i];
        if (!EliteReadable(targets[i], s.length) || memcmp(targets[i], s.expected, s.length)) return result;
    }
    result.ready = true;
    result.reason = "disabled";
    if (!apply) return result;
    for (; acquired < kEliteEligibilitySiteCount; ++acquired) {
        if (!VirtualProtect(targets[acquired], kEliteEligibilitySites[acquired].length,
                            PAGE_EXECUTE_READWRITE, &protections[acquired])) {
            result.reason = "protect-failed";
            break;
        }
    }
    bool failed = acquired != kEliteEligibilitySiteCount;
    if (!failed) {
        for (size_t i = 0; i < kEliteEligibilitySiteCount; ++i) {
            const EliteEligibilitySite& s = kEliteEligibilitySites[i];
            if (memcmp(targets[i], s.expected, s.length)) {
                failed = true;
                result.reason = "context-changed";
                break;
            }
        }
    }
    if (!failed) {
        for (size_t i = 0; i < kEliteEligibilitySiteCount && !failed; ++i) {
            const EliteEligibilitySite& s = kEliteEligibilitySites[i];
            const size_t offsets[] = {s.levelOffset, s.awakeningOffset};
            for (size_t off : offsets) {
                if ((unsigned char)_InterlockedCompareExchange8((volatile char*)(targets[i]+off),
                                                               0, (char)s.expected[off]) != s.expected[off]) {
                    failed = true;
                    result.reason = "compare-exchange-failed";
                    break;
                }
                ++result.changed;
            }
        }
    }
    if (failed) {
        // Only roll back bytes this transaction owns, in reverse order.
        unsigned remaining = result.changed;
        for (unsigned n = result.changed; n > 0; --n) {
            const size_t site = (n-1)/2;
            const EliteEligibilitySite& s = kEliteEligibilitySites[site];
            const size_t off = ((n-1)%2) ? s.awakeningOffset : s.levelOffset;
            if (_InterlockedCompareExchange8((volatile char*)(targets[site]+off), (char)s.expected[off], 0) == 0)
                --remaining;
        }
        result.changed = remaining;
        if (remaining) result.reason = "rollback-conflict";
    }
    bool restored = true;
    while (acquired) {
        --acquired;
        DWORD unused = 0;
        if (!VirtualProtect(targets[acquired], kEliteEligibilitySites[acquired].length,
                            protections[acquired], &unused)) restored = false;
        if (!FlushInstructionCache(GetCurrentProcess(), targets[acquired], kEliteEligibilitySites[acquired].length))
            restored = false;
    }
    result.enabled = result.changed != 0;
    result.ready = !failed && restored;
    if (!restored) result.reason = "protection-or-cache-restore-failed";
    else if (!failed) result.reason = "";
    return result;
}
