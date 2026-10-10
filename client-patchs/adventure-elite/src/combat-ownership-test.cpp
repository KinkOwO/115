#include "combat-ownership.h"
#include <stdio.h>
int main() {
    const uintptr_t image=0x140000000;
    EliteCombatSnapshot s;s.readable=true;s.stable=true;s.unique=true;s.bound=true;
    s.victimKind=3;s.victimId=4096;s.sourceKind=5;s.sourceId=0;s.sourceField=0;s.slot=0;s.sourceStrong=1;
    s.sourceController=65535;s.controllerWire=65535;s.combatGetter=image+0x14CC20;
    s.ownerMatches=1;s.ownerId=2;s.ownerController=2;s.playerRef[0]=3;s.playerRef[1]=4;
    auto decide=[&](const EliteCombatSnapshot& sample,int value=65535,uintptr_t caller=0x5DD022B,bool identity=true,bool assigning=true,bool same=true,unsigned int owner=2) {
        return EliteOwnedKiller(sample,value,caller,identity,assigning,same,owner,image);
    };
    if(decide(s)!=2 || decide(s,2)!=2 || decide(s,7)!=7 || decide(s,65535,0)!=65535 || decide(s,65535,0x5DD022B,false)!=65535 ||
        decide(s,65535,0x5DD022B,true,false)!=65535 || decide(s,65535,0x5DD022B,true,true,false)!=65535 ||
        decide(s,65535,0x5DD022B,true,true,true,0)!=65535 || decide(s,65535,0x5DD022B,true,true,true,65535)!=65535)return 1;
    for(int i=0;i<20;++i) {
        auto bad=s;
        switch(i) {
            case 0:bad.readable=false;break;case 1:bad.stable=false;break;case 2:bad.unique=false;break;case 3:bad.bound=false;break;
            case 4:bad.victimKind=5;break;case 5:bad.victimId=-1;break;case 6:bad.victimId=70000;break;case 7:bad.slot=-1;break;
            case 8:bad.sourceStrong=0;break;case 9:bad.sourceKind=3;break;case 10:bad.sourceId=-1;break;case 11:bad.sourceField=1;break;
            case 12:bad.sourceController=7;break;case 13:bad.controllerWire=7;break;case 14:bad.combatGetter=0;break;
            case 15:bad.ownerMatches=0;break;case 16:bad.ownerId=7;break;case 17:bad.ownerController=7;break;
            case 18:bad.victimId=bad.sourceId;break;case 19:bad.playerRef[1]=0;break;
        }
        if(decide(bad)!=65535)return i+2;
    }
    // Non-source native FFFF clears and foreign killers stay unchanged. Source
    // identity/binding are inputs; policy has no dependency on diagnostic budgets.
    if(s.sourceController!=65535 || s.controllerWire!=65535 || s.killer!=0)return 22;
    puts("elite owned killer: exact nested callsite/current-owner/source/binding/identity policy; unrelated FFFF/native/foreign values preserved; no diagnostic-budget dependency passed");
    return 0;
}
