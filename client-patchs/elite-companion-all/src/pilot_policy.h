#pragma once
#include <stdint.h>
#include <stddef.h>
inline bool PilotID(uint32_t id, const uint32_t* ids, size_t count) {
    for(size_t i=0;i<count;i++) if(ids[i]==id) return true;
    return false;
}
inline bool PilotScope(bool enabled, bool allowedID, bool pending, uint32_t elapsed,
                       bool validScene, uint32_t scene) {
    // N29 arrives in native scene 2; the native combat/GO_TO_TOWN path uses 3.
    // Town is 1. Never keep the pilot active on return to town.
    return enabled && allowedID && validScene && ((pending && elapsed<=5000) || scene==2 || scene==3);
}
inline bool PilotRetreat(bool scope, uint32_t scene, int action) {
    // GO_TO_TOWN=22, decoded from the native 83-entry registration table.
    // Loading/selection (2) must still obey the original menu lock.
    return scope && scene==3 && action==22;
}

// Build-time index is sorted and validated against the current read-only PVF.
inline bool PilotSortedID(uint32_t id,const uint32_t* ids,size_t count) {
    size_t lo=0,hi=count;
    while(lo<hi) {size_t mid=lo+(hi-lo)/2;if(ids[mid]<id)lo=mid+1;else hi=mid;}
    return lo<count&&ids[lo]==id;
}
inline bool PilotGenerationReady(bool all,bool pending,bool ready) { return !all||pending||ready; }
inline bool PilotRoomReady(bool all,bool ready) { return !all||ready; }
inline bool PilotNativeModeAllowed(bool all,int mode) { return !all||(mode!=3&&mode!=4); }

// 142E5F250 maps a character slot to a preset APC ID. All-dungeon membership
// cannot be applied to this namespace: small dungeon IDs overlap real slots.
// Keep the old narrow pilot behavior when the all-dungeon candidate is off.
inline int PilotRosterKey(bool all,bool enabled,bool allowed,int native) {
    return !all&&native==-1&&enabled&&allowed?2:native;
}
