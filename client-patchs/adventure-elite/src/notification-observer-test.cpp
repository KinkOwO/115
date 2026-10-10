#include "notification-observer.h"
#include "notification-header.h"
#include <stdio.h>

static unsigned int seenOpcode;
static uintptr_t seenOpaque;
static unsigned observed;
static void __fastcall Original(unsigned int opcode, uintptr_t opaque) {
    seenOpcode = opcode; seenOpaque = opaque;
}
static void __fastcall Observer(unsigned int opcode, uintptr_t opaque) {
    ++observed; Original(opcode, opaque);
}

int main() {
    // NOTI1754 uses the protocol encoder's fixed record layout. Nonzero
    // reserved prefix bytes must not be reported as a mode or consumed.
    unsigned char selection[16] = {1, 0x34, 0x12};
    selection[14] = 2;
    auto header = ElitePeekNotificationHeader(1754, selection, sizeof(selection));
    if (header.count != 1 || header.mode != 2 || header.owner != -1 || selection[1] != 0x34) return 12;
    if (ElitePeekNotificationHeader(1754, selection, 15).mode != -1) return 13;
    selection[0] = 0;
    if (ElitePeekNotificationHeader(1754, selection, sizeof(selection)).mode != -1) return 14;
    const unsigned char info[] = {1, 0x34, 0x12};
    if (ElitePeekNotificationHeader(1382, info, sizeof(info)).owner != 0x1234 ||
        ElitePeekNotificationHeader(1382, info, 2).owner != -1) return 15;
    const unsigned char done[] = {2, 0};
    if (ElitePeekNotificationHeader(1879, done, sizeof(done)).mode != 2 ||
        ElitePeekNotificationHeader(1879, done, 1).mode != -1 ||
        ElitePeekNotificationHeader(1754, nullptr, 16).count != -1) return 16;
    uintptr_t table[17] = {}, sentinel[4] = {}, nodes[3][4] = {}, callbacks[3][2] = {};
    table[0] = 123; table[2] = reinterpret_cast<uintptr_t>(sentinel); table[3] = 3;
    sentinel[0] = reinterpret_cast<uintptr_t>(nodes[0]);
    sentinel[1] = reinterpret_cast<uintptr_t>(nodes[2]);
    const unsigned int opcodes[] = {1754, 1382, 1879};
    EliteNotificationBinding bindings[3] = {};
    for (size_t i = 0; i < 3; ++i) {
        nodes[i][0] = reinterpret_cast<uintptr_t>(i == 2 ? sentinel : nodes[i+1]);
        nodes[i][1] = reinterpret_cast<uintptr_t>(i == 0 ? sentinel : nodes[i-1]);
        nodes[i][2] = opcodes[i]; nodes[i][3] = reinterpret_cast<uintptr_t>(callbacks[i]);
        callbacks[i][0] = reinterpret_cast<uintptr_t>(Original);
        bindings[i] = {opcodes[i], Original, Observer, nullptr};
    }
    if (EliteNotificationSlots(nullptr, 123, bindings, 3)) return 1;
    if (EliteNotificationSlots(table, 124, bindings, 3)) return 2;
    callbacks[2][1] = 1;
    if (EliteNotificationSlots(table, 123, bindings, 3)) return 3;
    callbacks[2][1] = 0;
    nodes[1][0] = reinterpret_cast<uintptr_t>(nodes[0]);
    if (EliteNotificationSlots(table, 123, bindings, 3)) return 4;
    nodes[1][0] = reinterpret_cast<uintptr_t>(nodes[2]);
    nodes[2][2] = 1382;
    if (EliteNotificationSlots(table, 123, bindings, 3)) return 5;
    nodes[2][2] = 1879;
    if (!EliteNotificationSlots(table, 123, bindings, 3)) return 6;
    // A later mismatch must leave earlier slots untouched.
    callbacks[2][1] = 2;
    if (EliteObserveNotifications(bindings, 3)) return 7;
    for (const auto& callback : callbacks) if (callback[0] != reinterpret_cast<uintptr_t>(Original)) return 8;
    callbacks[2][1] = 0;
    if (!EliteObserveNotifications(bindings, 3)) return 9;
    for (size_t i = 0; i < 3; ++i) {
        reinterpret_cast<EliteNotification>(callbacks[i][0])(opcodes[i], 987+i);
        if (seenOpcode != opcodes[i] || seenOpaque != 987+i || observed != i+1) return 10;
    }
    if (EliteObserveNotifications(bindings, 3)) return 11;
    puts("notification observer: native header offsets/bounds, bounded map, all-slot rejection, transparent ABI and idempotence passed");
    return 0;
}
