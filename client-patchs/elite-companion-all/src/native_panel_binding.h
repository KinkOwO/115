#pragma once
#include <stdint.h>

// Compare native identities, never copy actors or own their weak references.
// An unchanged incomplete list must not rebind on every town frame.
struct PanelBindingSnapshot {
    uintptr_t window, roles, config;
    uint32_t slots[3];
    uintptr_t sources[3], clones[3];
};
inline bool PanelBindingChanged(const PanelBindingSnapshot& before,
                                const PanelBindingSnapshot& after) {
    if(before.window!=after.window||before.roles!=after.roles||before.config!=after.config)return true;
    for(int i=0;i<3;i++)
        if(before.slots[i]!=after.slots[i]||before.sources[i]!=after.sources[i]||before.clones[i]!=after.clones[i])return true;
    return false;
}
inline bool PilotRoomIdentity(bool scope,uintptr_t instance,uintptr_t source,
                              uintptr_t boundInstance,uintptr_t boundSource) {
    return scope&&instance&&source&&instance==boundInstance&&source==boundSource;
}
