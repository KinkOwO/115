#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdio.h>
#include <string.h>
#include "lifecycle-snapshot.h"
static unsigned char manager[2704], actor[101000], context[5500], controller[2200], control[24];
static unsigned char owner[75000], ownerControl[24], sceneControl[24];
static bool race, unavailable, ownerRace, sceneRace;
static int ownerReads, sceneReads;
static int slotReads;
static bool Read(const void* p,void* out,size_t n) {
    const unsigned char* blocks[] = {manager,actor,context,controller,control,owner,ownerControl,sceneControl};
    const size_t sizes[] = {sizeof(manager),sizeof(actor),sizeof(context),sizeof(controller),sizeof(control),sizeof(owner),sizeof(ownerControl),sizeof(sceneControl)};
    if (unavailable && p == actor+74020) return false;
    if (p == manager+72 && ++slotReads == 2 && race) memset(manager+72,0,16);
    if (p == actor+25264 && ++ownerReads == 2 && ownerRace) memset(actor+25264,0,24);
    if (p == actor+400 && ++sceneReads == 2 && sceneRace) memset(actor+400,0,16);
    for(size_t i=0;i<8;++i) {
        auto start = reinterpret_cast<uintptr_t>(blocks[i]), addr = reinterpret_cast<uintptr_t>(p);
        if (addr >= start && addr-start <= sizes[i] && n <= sizes[i]-(addr-start)) { memcpy(out,p,n); return true; }
    }
    return false;
}
template<class T> static void Put(unsigned char* p,T value) { memcpy(p,&value,sizeof(value)); }
static void Identity(unsigned char* p,uint32_t value) {
    // IDA integer encoding and checksum; these are mechanism fixtures, not live vectors.
    const uint32_t encoded = (value + UINT32_C(4)) ^ UINT32_C(0x1F2A025C);
    Put(p+356,encoded); Put(p+360,encoded+value+UINT32_C(196));
}
static void OwnerAndScene() {
    Identity(actor,17); Identity(owner,2);
    Put(owner+352,3); Put(owner+74020,2); Put(ownerControl+8,1); Put(sceneControl+8,1);
    Put(actor+25272,reinterpret_cast<uintptr_t>(ownerControl)); Put(actor+25280,reinterpret_cast<uintptr_t>(owner+48));
    Put(actor+400,reinterpret_cast<uintptr_t>(sceneControl)); Put(actor+408,reinterpret_cast<uintptr_t>(manager));
}
static void Setup() {
    memset(manager,0,sizeof(manager)); memset(actor,0,sizeof(actor)); memset(context,0,sizeof(context));
    memset(controller,0,sizeof(controller)); memset(control,0,sizeof(control));
    memset(owner,0,sizeof(owner)); memset(ownerControl,0,sizeof(ownerControl)); memset(sceneControl,0,sizeof(sceneControl));
    race=false; unavailable=false; ownerRace=false; sceneRace=false; slotReads=0; ownerReads=0; sceneReads=0;
    Put(manager+72,reinterpret_cast<uintptr_t>(control)); Put(manager+80,reinterpret_cast<uintptr_t>(actor+48));
    Put(control+8,1); Put(actor+352,5); Put(actor+74020,7);
    Put(actor+100888,reinterpret_cast<uintptr_t>(context)); Put(context+5424,reinterpret_cast<uintptr_t>(controller));
    Put(controller+112,static_cast<unsigned short>(7)); Put(controller+2072,reinterpret_cast<uintptr_t>(control+16));
    Put(controller+2080,reinterpret_cast<uintptr_t>(control)); Put(controller+2088,reinterpret_cast<uintptr_t>(actor+48));
}
int main() {
    if(EliteReadCompanions(0,Read).available || EliteReadCompanions(1,nullptr).available) return 1;
    Setup(); unsigned char before[sizeof(manager)]; memcpy(before,manager,sizeof(manager));
    auto s=EliteReadCompanions(reinterpret_cast<uintptr_t>(manager),Read);
    if(!s.available || !s.consistent || s.alive!=1 || s.kind[0]!=5 || s.controllerId[0]!=7 || s.controllerWire[0]!=7 || s.controllerBound[0]!=1 || memcmp(before,manager,sizeof(manager))) return 2;
    Setup(); Put(control+8,0); s=EliteReadCompanions(reinterpret_cast<uintptr_t>(manager),Read);
    if(!s.consistent || s.alive || s.controllerWire[0]!=-1) return 3;
    Setup(); race=true; s=EliteReadCompanions(reinterpret_cast<uintptr_t>(manager),Read);
    if(s.consistent || s.alive) return 4;
    Setup(); unavailable=true; s=EliteReadCompanions(reinterpret_cast<uintptr_t>(manager),Read);
    if(s.consistent || s.alive!=1 || s.controllerWire[0]!=-1) return 5;
    Setup(); Put(controller+2088,reinterpret_cast<uintptr_t>(actor+64)); s=EliteReadCompanions(reinterpret_cast<uintptr_t>(manager),Read);
    if(!s.consistent || s.controllerBound[0]!=0) return 6;
    Setup(); Put(manager+80,static_cast<uintptr_t>(3)); s=EliteReadCompanions(reinterpret_cast<uintptr_t>(manager),Read);
    if(s.consistent || s.alive) return 7;
    Setup(); OwnerAndScene();
    unsigned char actorBefore[sizeof(actor)],ownerBefore[sizeof(owner)];
    memcpy(actorBefore,actor,sizeof(actor)); memcpy(ownerBefore,owner,sizeof(owner));
    s=EliteReadCompanions(reinterpret_cast<uintptr_t>(manager),Read);
    if(!s.consistent || s.objectId[0]!=17 || s.identityValid[0]!=1 || s.ownerStrong[0]!=1 || s.ownerObjectId[0]!=2 ||
        s.ownerIdentityValid[0]!=1 || s.ownerKind[0]!=3 || s.ownerControllerId[0]!=2 || s.sceneAttached[0]!=1 ||
        memcmp(actorBefore,actor,sizeof(actor)) || memcmp(ownerBefore,owner,sizeof(owner))) return 8;
    Setup(); OwnerAndScene(); Put(actor+360,UINT32_C(1));
    s=EliteReadCompanions(reinterpret_cast<uintptr_t>(manager),Read);
    if(!s.consistent || s.identityValid[0]!=0 || s.objectId[0]!=-1) return 9;
    Setup(); OwnerAndScene(); Put(ownerControl+8,0);
    s=EliteReadCompanions(reinterpret_cast<uintptr_t>(manager),Read);
    if(!s.consistent || s.ownerStrong[0]!=0 || s.ownerObjectId[0]!=-1) return 10;
    Setup(); OwnerAndScene(); ownerRace=true;
    s=EliteReadCompanions(reinterpret_cast<uintptr_t>(manager),Read);
    if(s.consistent) return 11;
    Setup(); OwnerAndScene(); sceneRace=true;
    s=EliteReadCompanions(reinterpret_cast<uintptr_t>(manager),Read);
    if(s.consistent) return 12;
    Setup(); OwnerAndScene(); Put(actor+25280,static_cast<uintptr_t>(3));
    s=EliteReadCompanions(reinterpret_cast<uintptr_t>(manager),Read);
    if(s.consistent || s.ownerObjectId[0]!=-1) return 13;
    Setup(); OwnerAndScene(); Put(sceneControl+8,0);
    s=EliteReadCompanions(reinterpret_cast<uintptr_t>(manager),Read);
    if(!s.consistent || s.sceneAttached[0]!=0) return 14;
    Setup(); Identity(actor,UINT32_C(0xFFFFFFFE));
    s=EliteReadCompanions(reinterpret_cast<uintptr_t>(manager),Read);
    if(!s.consistent || s.objectId[0]!=INT64_C(4294967294)) return 15;
    Setup(); OwnerAndScene(); uintptr_t currentPlayer[2] = {reinterpret_cast<uintptr_t>(ownerControl),reinterpret_cast<uintptr_t>(owner+48)};
    s=EliteReadCompanions(reinterpret_cast<uintptr_t>(manager),Read,currentPlayer);
    if(!s.consistent || s.ownerMatchesCurrentPlayer[0]!=1) return 16;
    currentPlayer[1]+=16; s=EliteReadCompanions(reinterpret_cast<uintptr_t>(manager),Read,currentPlayer);
    if(!s.consistent || s.ownerMatchesCurrentPlayer[0]!=0) return 17;
    puts("lifecycle snapshots: native alias, controller binding, object IDs/checksums, owner/scene live/dead/racing refs and no mutation passed");
    return 0;
}
