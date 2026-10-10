#pragma once
#include <stdint.h>
#include <stddef.h>

// Operational bound for a town load, not a PVF gameplay rule.
static constexpr uint64_t kElitePreparationDeadline = 10000;
struct ElitePreparationState {
    unsigned int thread;
    unsigned int channel;
    unsigned short owner;
    uint64_t deadline;
    bool waiting;
    bool info;
};
inline void EliteArmPreparation(ElitePreparationState& s, unsigned int thread,
        unsigned int channel, unsigned short owner, uint64_t now) {
    s = {thread, channel, owner, now+kElitePreparationDeadline, true, false};
}
inline bool ElitePreparationMatches(const ElitePreparationState& s,
        unsigned int thread, unsigned int channel, unsigned short owner, uint64_t now) {
    return s.waiting && thread == s.thread && channel && channel == s.channel &&
           owner && owner != 65535 && owner == s.owner && now < s.deadline;
}
inline bool ElitePreparationCaller(unsigned int opcode, uintptr_t returnRva) {
    if (opcode == 1754) return returnRva == 0x2E5A5A2 || returnRva == 0x2E5AAEA;
    if (opcode == 1382) return returnRva == 0x44FD019 || returnRva == 0x44FD1CF || returnRva == 0x44FDB47;
    if (opcode == 1879) return returnRva == 0x44FAC6B;
    return false;
}

// Source permission is carried by the server's existing mode-2 row view.
// Empty rows still select mode 2 so native cleanup can run, but never arm a load.
inline bool ElitePreparationSelection(const unsigned char* body, size_t length, bool& nonEmpty) {
    nonEmpty = false;
    if (!body || length < 1 || body[0] > 4 || length < 1+531u*body[0]) return false;
    unsigned int seen = 0;
    bool mode2 = false;
    for (unsigned int i = 0; i < body[0]; ++i) {
        const unsigned char* row = body+1+531u*i;
        unsigned int mode = row[13] | (row[14] << 8);
        if (!mode || mode > 4 || (seen & (1u << mode))) return false;
        seen |= 1u << mode;
        if (mode != 2) continue;
        mode2 = true;
        for (size_t j = 0; j < 3; ++j) {
            const unsigned char* slot = row+39+4*j;
            uint32_t value = slot[0] | (uint32_t(slot[1]) << 8) | (uint32_t(slot[2]) << 16) | (uint32_t(slot[3]) << 24);
            if (value != UINT32_MAX && value > INT32_MAX) return false;
            nonEmpty |= value != UINT32_MAX;
        }
    }
    return mode2;
}
