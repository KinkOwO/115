#pragma once
#include "combat-snapshot.h"
// Explicit ordinary-squad policy, backed by v0.3.7 native source/setter hits.
// Neither global FFFF fallback nor rewriting the APC's controller/binding.
inline int EliteOwnedKiller(const EliteCombatSnapshot& s,int native,uintptr_t caller,
        bool identity,bool assigning,bool sameVictim,unsigned int owner,uintptr_t image) {
    if(native!=65535 || caller!=0x5DD022B || !identity || !assigning || !sameVictim || !owner || owner>=65535 ||
        !s.readable || !s.stable || s.victimKind!=3 || s.victimId<0 || s.victimId>65535 ||
        s.slot<0 || s.slot>=3 || !s.unique || !s.bound || s.sourceStrong<=0 || s.sourceKind!=5 ||
        s.sourceId<0 || s.sourceId>65535 || s.sourceField!=s.sourceId || s.sourceController!=65535 ||
        s.controllerWire!=65535 || s.combatGetter!=image+0x14CC20 || s.ownerMatches!=1 ||
        s.ownerId!=owner || s.ownerController!=static_cast<int>(owner) ||
        s.victimId==s.sourceId || s.victimId==s.ownerId || s.sourceId==s.ownerId || !s.playerRef[0] || !s.playerRef[1])return native;
    return static_cast<int>(owner);
}
