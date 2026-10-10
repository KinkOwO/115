#pragma once
#include <stddef.h>

struct EliteNotificationHeader { int count; int mode; int owner; };

// Read copied prefix bytes only. N1754 has count + 531-byte records;
// the first record's mode is at +0x0d, not at its beginning.
inline EliteNotificationHeader ElitePeekNotificationHeader(unsigned int opcode,
        const unsigned char* prefix, size_t length) {
    EliteNotificationHeader value = {-1, -1, -1};
    if (!prefix) return value;
    if (opcode == 1754) {
        if (length >= 1) value.count = prefix[0];
        if (value.count > 0 && length >= 16) value.mode = prefix[14] | (prefix[15] << 8);
    } else if (opcode == 1382) {
        if (length >= 1) value.count = prefix[0];
        if (length >= 3) value.owner = prefix[1] | (prefix[2] << 8);
    } else if (opcode == 1879 && length >= 2) {
        value.mode = prefix[0] | (prefix[1] << 8);
    }
    return value;
}
