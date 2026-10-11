#include "pilot_policy.h"
#include "all_dungeon_evidence.h"
#include <assert.h>
#include <stdio.h>
#include <stdlib.h>
int main() {
    const uint32_t ids[]={1,17,23,100005068};
    for(uint32_t id:ids) assert(PilotSortedID(id,ids,4));
    assert(!PilotSortedID(0,ids,4)&&!PilotSortedID(18,ids,4)&&!PilotSortedID(100005069,ids,4));
    assert(!PilotSortedID(17,nullptr,0));
    for(size_t i=0;i<_countof(kAllDungeonIDs);i++) {
        assert(PilotSortedID(kAllDungeonIDs[i],kAllDungeonIDs,_countof(kAllDungeonIDs)));
        if(i)assert(kAllDungeonIDs[i]>kAllDungeonIDs[i-1]);
    }
    // Loading needs the native container gate before N1879, but an empty
    // roster must never enter room registration or retain the previous clone.
    assert(PilotGenerationReady(true,true,false));
    assert(!PilotRoomReady(true,false));
    assert(!PilotGenerationReady(true,false,false));
    assert(PilotGenerationReady(true,false,true)&&PilotRoomReady(true,true));
    assert(PilotGenerationReady(false,false,false)&&PilotRoomReady(false,false));
    assert(PilotNativeModeAllowed(true,0)&&PilotNativeModeAllowed(true,2));
    assert(!PilotNativeModeAllowed(true,3)&&!PilotNativeModeAllowed(true,4));
    assert(PilotNativeModeAllowed(false,3)&&PilotNativeModeAllowed(false,4));
    puts("PASS all source index, empty/stale generation isolation, native training/switch modes and r7 fallback");
}
