#pragma once
#include "eligibility-patch.h"
#include <stdint.h>

// Current 115 notification registrar/dispatcher: 1459A3DD0 / 1459A3BB0.
// Primary map node: next, prev, opcode at +16, callback object at +24.
// Callback object: function at +0, opaque argument at +8. No std::function map.
using EliteNotification = void (__fastcall *)(unsigned int, uintptr_t);
struct EliteNotificationBinding {
    unsigned int opcode;
    EliteNotification original;
    EliteNotification observer;
    void* volatile* slot;
};

inline bool EliteCopy(const void* address, void* out, size_t length) {
    if (!EliteReadable(static_cast<const unsigned char*>(address), length)) return false;
    __try { memcpy(out, address, length); return true; }
    __except (EXCEPTION_EXECUTE_HANDLER) { return false; }
}

inline bool EliteNotificationSlots(const void* table, uintptr_t expectedVtable,
                                   EliteNotificationBinding* bindings, size_t count) {
    uintptr_t header[17] = {};
    if (!count || count > 3 || !EliteCopy(table, header, sizeof(header)) ||
        header[0] != expectedVtable || !header[2] || header[3] > 8192) return false;
    uintptr_t sentinel[4] = {};
    if (!EliteCopy(reinterpret_cast<void*>(header[2]), sentinel, sizeof(sentinel))) return false;
    uintptr_t node = sentinel[0], previous = header[2];
    bool found[3] = {};
    size_t walked = 0;
    while (node != header[2]) {
        uintptr_t value[4] = {};
        if (walked++ >= header[3] || !EliteCopy(reinterpret_cast<void*>(node), value, sizeof(value)) ||
            value[1] != previous) return false;
        for (size_t i = 0; i < count; ++i) {
            if (static_cast<unsigned int>(value[2]) != bindings[i].opcode) continue;
            uintptr_t callback[2] = {};
            if (found[i] || (value[3] & (alignof(void*) - 1)) ||
                !EliteCopy(reinterpret_cast<void*>(value[3]), callback, sizeof(callback)) ||
                callback[0] != reinterpret_cast<uintptr_t>(bindings[i].original) || callback[1] != 0)
                return false;
            bindings[i].slot = reinterpret_cast<void* volatile*>(value[3]);
            found[i] = true;
        }
        previous = node;
        node = value[0];
    }
    if (walked != header[3] || sentinel[1] != previous) return false;
    for (size_t i = 0; i < count; ++i) if (!found[i]) return false;
    uintptr_t after[17] = {};
    return EliteCopy(table, after, sizeof(after)) && memcmp(header, after, sizeof(header)) == 0;
}

// Compare/exchange only aligned callback pointers, never native instructions.
// All slots are checked before writing. Rollback only restores pointers we own.
inline bool EliteObserveNotifications(EliteNotificationBinding* bindings, size_t count) {
    for (size_t i = 0; i < count; ++i) {
        uintptr_t value[2] = {};
        if (!bindings[i].slot || !bindings[i].observer ||
            !EliteCopy(const_cast<void* const*>(bindings[i].slot), value, sizeof(value)) ||
            value[0] != reinterpret_cast<uintptr_t>(bindings[i].original) || value[1] != 0) return false;
        MEMORY_BASIC_INFORMATION memory = {};
        if (!VirtualQuery(const_cast<void**>(bindings[i].slot), &memory, sizeof(memory)) ||
            (memory.Protect != PAGE_READWRITE && memory.Protect != PAGE_EXECUTE_READWRITE)) return false;
    }
    size_t changed = 0;
    for (; changed < count; ++changed) {
        auto& binding = bindings[changed];
        if (InterlockedCompareExchangePointer(binding.slot, reinterpret_cast<void*>(binding.observer),
                                              reinterpret_cast<void*>(binding.original)) !=
            reinterpret_cast<void*>(binding.original)) break;
    }
    if (changed == count) return true;
    while (changed) {
        auto& binding = bindings[--changed];
        InterlockedCompareExchangePointer(binding.slot, reinterpret_cast<void*>(binding.original),
                                           reinterpret_cast<void*>(binding.observer));
    }
    return false;
}
