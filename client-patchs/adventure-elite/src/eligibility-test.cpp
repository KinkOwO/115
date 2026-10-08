#include "eligibility-patch.h"
#include <stdio.h>

int main() {
    unsigned char* targets[5] = {};
    for (size_t i = 0; i < kEliteEligibilitySiteCount; ++i) {
        targets[i] = (unsigned char*)VirtualAlloc(nullptr, 4096, MEM_COMMIT|MEM_RESERVE, PAGE_READWRITE);
        if (!targets[i]) return 1;
        memcpy(targets[i], kEliteEligibilitySites[i].expected, kEliteEligibilitySites[i].length);
        DWORD old = 0;
        if (!VirtualProtect(targets[i], 4096, PAGE_EXECUTE_READ, &old)) return 2;
    }
    auto disabled = EliteEligibilityPatch(targets, false);
    if (!disabled.ready || disabled.enabled || disabled.changed) return 3;
    // Last-site mismatch must not leave any earlier site patched.
    unsigned char* mismatch[5];
    memcpy(mismatch, targets, sizeof(targets));
    mismatch[4] = targets[4]+1;
    auto reject = EliteEligibilityPatch(mismatch, true);
    if (reject.ready || reject.enabled || reject.changed) return 4;
    for (size_t i = 0; i < kEliteEligibilitySiteCount; ++i)
        if (memcmp(targets[i], kEliteEligibilitySites[i].expected, kEliteEligibilitySites[i].length)) return 5;
    auto applied = EliteEligibilityPatch(targets, true);
    if (!applied.ready || !applied.enabled || applied.changed != 10) return 6;
    for (size_t i = 0; i < kEliteEligibilitySiteCount; ++i) {
        const auto& s = kEliteEligibilitySites[i];
        for (size_t j = 0; j < s.length; ++j)
            if (targets[i][j] != ((j==s.levelOffset || j==s.awakeningOffset) ? 0 : s.expected[j])) return 7;
        MEMORY_BASIC_INFORMATION m = {};
        if (!VirtualQuery(targets[i], &m, sizeof(m)) || m.Protect != PAGE_EXECUTE_READ) return 8;
    }
    if (EliteEligibilityPatch(targets, true).ready) return 9;
    for (auto target : targets) VirtualFree(target, 0, MEM_RELEASE);
    // Unreadable site is rejected without reading it.
    targets[0] = nullptr;
    if (EliteEligibilityPatch(targets, true).ready) return 10;
    puts("eligibility mechanism: PASS (disabled, mismatch, all ten immediates, protections, repeat, unreadable)");
    return 0;
}
