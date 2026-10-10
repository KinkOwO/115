#include "pilot_policy.h"
#include <assert.h>
#include <stdio.h>
int main(){
    const uint32_t ids[]={17,23};
    assert(PilotID(17,ids,2)&&!PilotID(18,ids,2));
    assert(PilotScope(true,true,true,5000,true,0));
    assert(!PilotScope(true,true,true,5001,true,0));
    assert(!PilotScope(true,true,true,0,false,0));
    assert(PilotScope(true,true,false,99999,true,2));
    assert(PilotScope(true,true,false,99999,true,3));
    assert(!PilotScope(true,true,false,99999,true,1));
    assert(!PilotScope(false,true,true,0,true,1));
    assert(!PilotScope(true,false,true,0,true,1));
    assert(!PilotScope(true,true,false,0,true,0));
    assert(!PilotScope(true,true,false,0,false,1));
    for(int action=0;action<83;action++) {
        assert(PilotRetreat(true,3,action)==(action==22));
        assert(!PilotRetreat(false,3,action));
        assert(!PilotRetreat(true,2,action));
        assert(!PilotRetreat(true,1,action));
    }
    puts("PASS pilot ID, opt-in, pending timeout, unreadable scene, town and dungeon boundaries");
}
