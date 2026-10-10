#include <assert.h>
#include <stdio.h>
#include <vector>
#include <windows.h>
#include "pilot_policy.h"
#include "all_dungeon_evidence.h"

int main() {
    // Read-only live Window569 evidence: all 18 qualified account slots reach
    // the selector; the old hook suppresses overlapping low dungeon IDs.
    const uint32_t slots[]={0,4,6,7,1,2,3,5,8,9,11,12,13,14,15,16,18,19};
    std::vector<uint32_t> oldVisible;
    for(uint32_t slot:slots) {
        bool overlaps=PilotSortedID(slot,kAllDungeonIDs,_countof(kAllDungeonIDs));
        int old=overlaps?2:-1;
        if(old==-1)oldVisible.push_back(slot);
        // Native 142E61240 admits regular slots whose native key is -1;
        // fabricated preset ID 2 has no row in the actual preset map.
        assert(PilotRosterKey(true,true,overlaps,-1)==-1);
        assert(PilotRosterKey(true,true,overlaps,5952)==5952);
        assert(PilotRosterKey(true,false,overlaps,-1)==-1);
    }
    assert((oldVisible==std::vector<uint32_t>{0,16,18,19}));
    assert(PilotRosterKey(false,true,true,-1)==2);
    assert(PilotRosterKey(false,false,true,-1)==-1);
    assert(PilotRosterKey(false,true,false,-1)==-1);
    assert(PilotRosterKey(false,true,true,23002)==23002);
    puts("Native roster key: old visible slots exactly 0,16,18,19; all 18 native keys preserved; preset and legacy modes preserved.");
}
