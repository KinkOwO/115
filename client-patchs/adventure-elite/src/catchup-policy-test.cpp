#include "catchup-policy.h"
#include <stdio.h>
#include <initializer_list>
int main() {
    constexpr auto blocked=ElitePathResult::Unreachable;
    EliteCatchupKey key={1,2,3,4,5,6,0,1};EliteCatchupState state;
    for(uint64_t tick=0;tick<3000;tick+=250)if(EliteCatchupDue(state,key,tick,true,blocked,0,0,801,0))return 1;
    if(!EliteCatchupDue(state,key,3000,true,blocked,0,0,801,0) || EliteCatchupDue(state,key,3250,true,blocked,0,0,801,0))return 2;
    for(auto path: {ElitePathResult::Unknown,ElitePathResult::Reachable}) {state={};for(uint64_t tick=10000;tick<=14000;tick+=250)if(EliteCatchupDue(state,key,tick,true,path,0,0,100000,0))return 3;}
    state={};for(uint64_t tick=0;tick<3000;tick+=250)EliteCatchupDue(state,key,tick,true,blocked,0,0,1000,0);
    if(EliteCatchupDue(state,key,3000,false,blocked,0,0,1000,0) || state.tracking)return 4;
    for(int change=0;change<8;++change) {
        state={};auto replacement=key;
        for(uint64_t tick=0;tick<3000;tick+=250)EliteCatchupDue(state,key,tick,true,blocked,0,0,1000,0);
        switch(change) {case 0:++replacement.world;break;case 1:++replacement.scene;break;case 2:++replacement.playerControl;break;
            case 3:++replacement.playerAlias;break;case 4:++replacement.control;break;case 5:++replacement.alias;break;
            case 6:++replacement.objectId;break;case 7:++replacement.room;break;}
        if(EliteCatchupDue(state,replacement,3000,true,blocked,0,0,1000,0))return 5+change;
    }
    state={};EliteCatchupDue(state,key,10000,true,blocked,0,0,1000,0);
    if(EliteCatchupDue(state,key,9000,true,blocked,0,0,1000,0) || EliteCatchupDue(state,key,13000,true,blocked,0,0,1000,0))return 13;
    state={};for(uint64_t tick=0;tick<3000;tick+=250)EliteCatchupDue(state,key,tick,true,blocked,0,0,1000,0);
    if(EliteCatchupDue(state,key,3000,true,ElitePathResult::Reachable,0,0,0,0) || EliteCatchupDue(state,key,3250,true,blocked,0,0,1000,0))return 14;
    for(int extreme: {-2147483647,2147483647})if(EliteCatchupDue(state,key,3500,true,blocked,extreme,0,0,0))return 15;
    if(ElitePathClassify(false,false,false,false)!=ElitePathResult::Unknown ||
        ElitePathClassify(false,false,true,false)!=blocked ||
        ElitePathClassify(true,true,false,false)!=ElitePathResult::Reachable ||
        ElitePathClassify(true,false,false,false)!=ElitePathResult::Unknown ||
        ElitePathClassify(false,false,true,true)!=ElitePathResult::Unknown)return 16;
    puts("catchup policy: sustained native no-route, unknown/budget/reachable reset, cooldown, identity changes, invalid/dead/airborne input, clock rollback and interrupted polling passed");
    return 0;
}
