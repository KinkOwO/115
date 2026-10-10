#pragma once
#include <stdint.h>

// User-requested rescue policy, not PVF content or a native follow-mode rule.
static constexpr uint64_t kEliteCatchupDelay = 3000;
enum class ElitePathResult { Unknown, Reachable, Unreachable };
inline ElitePathResult ElitePathClassify(bool terminal,bool atGoal,bool openEmpty,bool poolLimited) {
    if(poolLimited)return ElitePathResult::Unknown;
    if(terminal)return atGoal?ElitePathResult::Reachable:ElitePathResult::Unknown;
    return openEmpty?ElitePathResult::Unreachable:ElitePathResult::Unknown;
}
struct EliteCatchupKey {
    uintptr_t world = 0, scene = 0, playerControl = 0, playerAlias = 0;
    uintptr_t control = 0, alias = 0;
    uint32_t objectId = 0, room = 0;
};
struct EliteCatchupState {
    EliteCatchupKey key;
    uint64_t last = 0, blockedSince = 0;
    bool tracking = false;
};
inline bool EliteCatchupSame(const EliteCatchupKey& a,const EliteCatchupKey& b) {
    return a.world==b.world && a.scene==b.scene && a.playerControl==b.playerControl && a.playerAlias==b.playerAlias &&
        a.control==b.control && a.alias==b.alias && a.objectId==b.objectId && a.room==b.room;
}
inline bool EliteCatchupCoordinate(int v) {return v>=-1048576 && v<=1048576;}
inline bool EliteCatchupDue(EliteCatchupState& state,const EliteCatchupKey& key,uint64_t now,
        bool eligible,ElitePathResult path,int px,int py,int ax,int ay) {
    if(!eligible || !key.scene || !key.playerControl || !key.control || !key.alias ||
        !EliteCatchupCoordinate(px) || !EliteCatchupCoordinate(py) || !EliteCatchupCoordinate(ax) || !EliteCatchupCoordinate(ay)) {
        state={};return false;
    }
    if(!EliteCatchupSame(state.key,key) || now<state.last || now-state.last>1000) {state={};state.key=key;}
    state.last=now;
    if(path!=ElitePathResult::Unreachable) {state.tracking=false;return false;}
    if(!state.tracking) {state.tracking=true;state.blockedSince=now;return false;}
    if(now-state.blockedSince<kEliteCatchupDelay)return false;
    // A failed native landing check waits another full interval before retrying.
    state.blockedSince=now;return true;
}
