#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdio.h>
#include "entry-trace.h"
#include "registration-adapter.h"
#include "combat-trace.h"
#include "catchup-adapter.h"
#include "birth-position-adapter.h"
#include "position-write-trace.h"
#include "first-chance-trace.h"
static unsigned char permissionSavedGate[128],permissionSavedEffects[512];
static int originalCalls,combatCalls;
static unsigned char combatVictim[36000];
static void __fastcall CombatAssignStub(uintptr_t actor,uintptr_t source) {
    if(actor!=11 || source!=22)RaiseException(0xE1234001,0,0,nullptr);
    ++combatCalls;SetLastError(ERROR_ACCESS_DENIED);
}
static uintptr_t __fastcall CombatDeathStub(uintptr_t actor,char a2,unsigned char a3) {
    if(actor!=11 || a2!=-3 || a3!=241)RaiseException(0xE1234002,0,0,nullptr);
    ++combatCalls;SetLastError(ERROR_ACCESS_DENIED);return UINT64_C(0xfedcba9876543210);
}
static void __fastcall CombatThrowStub(uintptr_t,uintptr_t) {RaiseException(0xE1234567,0,0,nullptr);}
static bool CombatExceptionForwarded(EliteAttackAssign call) {
    bool caught=false;
    __try {call(11,22);} __except(GetExceptionCode()==0xE1234567?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {caught=true;}
    return caught;
}
static unsigned char ownedVictim[36000],ownedActor[101000],ownedPlayer[101000],ownedManager[600],ownedContext[5500],ownedController[2200],playerController[2200],ownedStrong[32],playerStrong[32],ownedHolder[160],ownedChannel[4560],ownedVtable[9000];
static void FixturePtr(unsigned char* p,size_t offset,uintptr_t value) {memcpy(p+offset,&value,8);}
static void FixtureInt(unsigned char* p,size_t offset,int value) {memcpy(p+offset,&value,4);}
static void FixtureIdentity(unsigned char* p,uint32_t value,int kind) {
    uint32_t words[2]={(value+4)^UINT32_C(0x1f2a025c),0};words[1]=words[0]+value+196;
    FixtureInt(p,352,kind);memcpy(p+356,words,8);
}
static bool OwnedSetterFixture(unsigned char* image) {
    FixtureIdentity(ownedVictim,4096,3);FixtureIdentity(ownedActor,0,5);FixtureIdentity(ownedPlayer,2,5);
    FixtureInt(ownedVictim,26784,65535);FixtureInt(ownedActor,74020,65535);FixtureInt(ownedPlayer,74020,2);
    FixtureInt(ownedStrong,8,1);FixtureInt(playerStrong,8,1);
    FixturePtr(ownedVictim,29496,reinterpret_cast<uintptr_t>(ownedStrong));FixturePtr(ownedVictim,29504,reinterpret_cast<uintptr_t>(ownedActor+48));
    FixturePtr(ownedActor,25272,reinterpret_cast<uintptr_t>(playerStrong));FixturePtr(ownedActor,25280,reinterpret_cast<uintptr_t>(ownedPlayer+48));
    FixturePtr(ownedActor,100888,reinterpret_cast<uintptr_t>(ownedContext));FixturePtr(ownedContext,5424,reinterpret_cast<uintptr_t>(ownedController));
    unsigned short sentinel=65535,owner=2;memcpy(ownedController+112,&sentinel,2);memcpy(playerController+112,&owner,2);
    FixturePtr(ownedController,2080,reinterpret_cast<uintptr_t>(ownedStrong));FixturePtr(ownedController,2088,reinterpret_cast<uintptr_t>(ownedActor+48));
    FixturePtr(playerController,2080,reinterpret_cast<uintptr_t>(playerStrong));FixturePtr(playerController,2088,reinterpret_cast<uintptr_t>(ownedPlayer+48));
    FixturePtr(ownedManager,72,reinterpret_cast<uintptr_t>(ownedStrong));FixturePtr(ownedManager,80,reinterpret_cast<uintptr_t>(ownedActor+48));
    FixturePtr(ownedActor,0,reinterpret_cast<uintptr_t>(ownedVtable));FixturePtr(ownedVtable,1984,reinterpret_cast<uintptr_t>(image+0x14CC20));
    FixturePtr(ownedHolder,144,reinterpret_cast<uintptr_t>(playerStrong));FixturePtr(ownedHolder,152,reinterpret_cast<uintptr_t>(playerController));
    FixturePtr(image,0xE638EF8,reinterpret_cast<uintptr_t>(ownedManager));FixturePtr(image,0xE683C08,reinterpret_cast<uintptr_t>(ownedHolder));
    FixturePtr(image,0xE66C090,reinterpret_cast<uintptr_t>(ownedChannel));FixtureInt(ownedChannel,4528,22);
    // This test-only caller provides x64 shadow space around the copied original
    // E8 setter instruction. Return address remains exactly 145DD022B.
    const unsigned char prolog[]={0x48,0x83,0xec,0x28},epilog[]={0x48,0x83,0xc4,0x28,0xc3};
    memcpy(image+0x5DD0222,prolog,4);memcpy(image+0x5DD022B,epilog,5);
    if(!FlushInstructionCache(GetCurrentProcess(),image+0x5DD0222,14))return false;
    g_lifecycleArmed=true;g_lifecycleThread=GetCurrentThreadId();g_lifecycleChannel=22;g_lifecycleOwner=2;
    g_combatContext={reinterpret_cast<uintptr_t>(ownedVictim),reinterpret_cast<uintptr_t>(ownedActor),99,true};
    // Ownership must work after the observer has exhausted its file budget.
    g_combatCount=kEliteCombatLimit+1;
    auto caller=reinterpret_cast<EliteKillerSet>(image+0x5DD0222);SetLastError(ERROR_ACCESS_DENIED);
    caller(reinterpret_cast<uintptr_t>(ownedVictim),65535);
    int killer=0,controller=0;unsigned short wire=0;memcpy(&killer,ownedVictim+26784,4);memcpy(&controller,ownedActor+74020,4);memcpy(&wire,ownedController+112,2);
    bool success=killer==2 && controller==65535 && wire==65535 && GetLastError()==ERROR_ACCESS_DENIED;
    FixtureInt(ownedVictim,26784,65535);g_combatContext.assigning=false;caller(reinterpret_cast<uintptr_t>(ownedVictim),65535);
    memcpy(&killer,ownedVictim+26784,4);success=success && killer==65535;
    g_lifecycleArmed=false;g_combatCount=0;g_combatContext={};return success;
}

static unsigned char entryWorld[6900],entryDungeon[8100],entryScene[160];
static unsigned char actorBefore[sizeof(ownedActor)],managerBefore[sizeof(ownedManager)],controllerBefore[sizeof(ownedController)];
static uintptr_t __fastcall ActiveStub(uintptr_t) {SetLastError(ERROR_ACCESS_DENIED);return 1;}
static uintptr_t __fastcall ActiveOther(uintptr_t) {SetLastError(ERROR_ACCESS_DENIED);return 2;}
static uintptr_t __fastcall ActiveThrow(uintptr_t) {RaiseException(0xE1234567,0,0,nullptr);return 0;}
static bool ActiveExceptionForwarded(EliteManagerActive call,uintptr_t manager) {
    bool caught=false;
    __try {call(manager);} __except(GetExceptionCode()==0xE1234567?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {caught=true;}
    return caught;
}
static int ReentryRegistrationFixture(unsigned char* image) {
    auto manager=reinterpret_cast<uintptr_t>(ownedManager),dungeon=reinterpret_cast<uintptr_t>(entryDungeon);
    FixturePtr(image,0xEF2CA88,reinterpret_cast<uintptr_t>(playerStrong));FixturePtr(image,0xEF2CA90,reinterpret_cast<uintptr_t>(ownedPlayer+48));
    FixturePtr(entryWorld,6792,dungeon);g_entryWorld=reinterpret_cast<uintptr_t>(entryWorld);g_entryScene=reinterpret_cast<uintptr_t>(entryScene);
    g_lifecycleArmed=true;g_lifecycleThread=GetCurrentThreadId();g_lifecycleChannel=22;g_lifecycleOwner=2;
    g_entryLoaderCaller=0x6D2A1E7;g_entryFreshDungeon=true;
    // Execute the copied E8 instruction with its exact return address 145B251D8.
    const unsigned char prolog[]={0x48,0x83,0xec,0x28},epilog[]={0x48,0x83,0xc4,0x28,0xc3};
    memcpy(image+0x5B251CF,prolog,4);memcpy(image+0x5B251D8,epilog,5);
    if(!FlushInstructionCache(GetCurrentProcess(),image+0x5B251CF,14))return 1;
    auto precise=reinterpret_cast<EliteManagerActive>(image+0x5B251CF);
    auto other=reinterpret_cast<EliteManagerActive>(image+0x2E60CE0);
    const auto conditionCaller=reinterpret_cast<uintptr_t>(image)+0x5B251AE;
    ownedManager[576]=0;
    if(EliteConditionDecision(dungeon,conditionCaller)!=1 || !g_registrationPermitted || g_registrationReentry || precise(manager)!=0)return 2;
    ownedManager[576]=1;g_entryFreshDungeon=false;g_entryLoaderCaller=0x6CE14C1;
    if(EliteConditionDecision(dungeon,conditionCaller)!=0 || g_registrationPermitted || precise(manager)!=1)return 3;
    g_entryLoaderCaller=0x6D2A1E7;g_entryFreshDungeon=true;
    if(EliteConditionDecision(dungeon,conditionCaller)!=1 || !g_registrationReentry)return 4;
    memcpy(actorBefore,ownedActor,sizeof(ownedActor));memcpy(managerBefore,ownedManager,sizeof(ownedManager));memcpy(controllerBefore,ownedController,sizeof(ownedController));
    // Exhausted logging budget cannot disable the actual registration adapter.
    g_entryCount=514;SetLastError(ERROR_ACCESS_DENIED);
    if(precise(manager)!=0 || GetLastError()!=ERROR_ACCESS_DENIED || other(manager)!=1)return 5;
    if(memcmp(actorBefore,ownedActor,sizeof(ownedActor)) || memcmp(managerBefore,ownedManager,sizeof(ownedManager)) || memcmp(controllerBefore,ownedController,sizeof(ownedController)))return 6;
    unsigned char foreignManager[600]={};foreignManager[576]=1;
    if(precise(reinterpret_cast<uintptr_t>(foreignManager))!=1)return 7;
    FixturePtr(image,0xE638EF8,reinterpret_cast<uintptr_t>(foreignManager));
    if(precise(manager)!=1)return 8;
    FixturePtr(image,0xE638EF8,manager);
    g_entryFreshDungeon=false;if(precise(manager)!=1)return 9;g_entryFreshDungeon=true;
    g_entryLoaderCaller=0x6CE14C1;if(precise(manager)!=1)return 10;g_entryLoaderCaller=0x6D2A1E7;
    g_lifecycleOwner=99;if(precise(manager)!=1)return 11;g_lifecycleOwner=2;
    uintptr_t sceneRef[3]={0,reinterpret_cast<uintptr_t>(ownedStrong),reinterpret_cast<uintptr_t>(ownedActor+48)};
    FixturePtr(entryScene,88,reinterpret_cast<uintptr_t>(sceneRef));FixturePtr(entryScene,96,reinterpret_cast<uintptr_t>(sceneRef+3));FixturePtr(entryScene,104,reinterpret_cast<uintptr_t>(sceneRef+3));
    if(precise(manager)!=1 || EliteConditionDecision(dungeon,conditionCaller)!=0)return 12;
    memset(entryScene,0,sizeof(entryScene));
    FixturePtr(ownedManager,96,reinterpret_cast<uintptr_t>(ownedStrong));FixturePtr(ownedManager,104,reinterpret_cast<uintptr_t>(ownedActor+48));
    if(EliteConditionDecision(dungeon,conditionCaller)!=0)return 13;
    memset(ownedManager+96,0,24);
    FixturePtr(image,0xEF2CA90,reinterpret_cast<uintptr_t>(ownedActor+48));
    if(EliteConditionDecision(dungeon,conditionCaller)!=0)return 14;
    FixturePtr(image,0xEF2CA90,reinterpret_cast<uintptr_t>(ownedPlayer+48));
    if(EliteConditionDecision(dungeon,conditionCaller)!=1)return 15;
    auto original=g_registrationActiveOriginal;g_registrationActiveOriginal=ActiveStub;
    if(precise(manager)!=0 || GetLastError()!=ERROR_ACCESS_DENIED)return 16;
    g_registrationActiveOriginal=ActiveOther;if(precise(manager)!=2 || GetLastError()!=ERROR_ACCESS_DENIED)return 17;
    g_registrationActiveOriginal=ActiveThrow;if(!ActiveExceptionForwarded(precise,manager))return 18;
    g_registrationActiveOriginal=original;
    EliteRegistrationReset();if(precise(manager)!=1 || g_registrationReentry || g_registrationManager)return 19;
    if(EliteConditionDecision(dungeon,conditionCaller+1)!=0 || g_registrationPermitted)return 20;
    ownedManager[576]=2;if(EliteConditionDecision(dungeon,conditionCaller)!=0 || precise(manager)!=2)return 21;
    ownedManager[576]=1;g_lifecycleArmed=false;g_entryWorld=0;g_entryScene=0;g_entryLoaderCaller=0;g_entryFreshDungeon=false;g_entryCount=0;EliteRegistrationReset();
    return 0;
}

static char __fastcall NativeStub() {++originalCalls;SetLastError(ERROR_ACCESS_DENIED);return 37;}
static int catchupCalls,catchupMoves,catchupRegistrations;
static unsigned char catchupPosition[640],catchupPlayerPosition[640],catchupWorld[4600],catchupReplacementWorld[4600];
static ElitePathResult catchupPath=ElitePathResult::Unreachable;
static int catchupQueryCalls;
static ElitePathResult CatchupQuery(uintptr_t,int,int,int,int) {++catchupQueryCalls;return catchupPath;}
static int64_t __fastcall CatchupComponent(uintptr_t,int axis) {++catchupCalls;SetLastError(ERROR_ACCESS_DENIED);return INT64_C(0x1234567800000064)+axis;}
static int __fastcall CatchupX(uintptr_t actor) {++catchupCalls;SetLastError(ERROR_ACCESS_DENIED);int x=0;memcpy(&x,reinterpret_cast<void*>(actor+1100),4);return x;}
static int __fastcall CatchupY(uintptr_t) {return 100;}
static int __fastcall CatchupZ(uintptr_t) {return 0;}
static void CatchupMode(int mode) {
    uint32_t encoded=(static_cast<uint32_t>(mode)+4)^UINT32_C(0x1f2a025c);
    FixtureInt(catchupWorld,260,static_cast<int>(encoded));FixtureInt(catchupWorld,264,static_cast<int>(encoded+mode+196));
}
static uintptr_t __fastcall CatchupScene(uintptr_t) {return reinterpret_cast<uintptr_t>(entryScene);}
static unsigned char __fastcall CatchupLand(uintptr_t,int,int,uintptr_t) {return 1;}
static bool landingSpecial=true,landingBaseAllowed=true;
static int landingState=5,landingBaseCalls;
static uintptr_t landingSeenActor,landingSeenContext;
static int landingSeenX,landingSeenY;
static unsigned char __fastcall LandingSpecial(uintptr_t,int flag) {return flag==16 && landingSpecial;}
static int __fastcall LandingState(uintptr_t) {return landingState;}
static unsigned char __fastcall LandingBaseGate(uintptr_t) {return landingBaseAllowed;}
static unsigned char __fastcall LandingBasePoint(uintptr_t actor,int x,int y,uintptr_t context) {
    ++landingBaseCalls;landingSeenActor=actor;landingSeenX=x;landingSeenY=y;landingSeenContext=context;return 1;
}
static void LandingJump(unsigned char* site,uintptr_t target) {
    const unsigned char jump[]={0x48,0xb8,0,0,0,0,0,0,0,0,0xff,0xe0};
    memcpy(site,jump,sizeof(jump));memcpy(site+2,&target,8);FlushInstructionCache(GetCurrentProcess(),site,sizeof(jump));
}
static uintptr_t __fastcall CatchupSet(uintptr_t actor,int x,int y,int z) {
    if(actor!=reinterpret_cast<uintptr_t>(ownedActor) || y!=100 || z!=0)RaiseException(0xE1234568,0,0,nullptr);
    ++catchupMoves;FixtureInt(ownedActor,1100,x);return 0;
}
static int64_t __fastcall CatchupThrow(uintptr_t,int) {RaiseException(0xE1234567,0,0,nullptr);return 0;}
static char __fastcall CatchupRegister(uintptr_t scene,uintptr_t parent,unsigned int kind,unsigned int id,uintptr_t actor,uintptr_t a6,char a7,char a8) {
    if(scene!=reinterpret_cast<uintptr_t>(entryScene) || parent || kind!=5 || id!=0 || actor!=reinterpret_cast<uintptr_t>(ownedActor) || a6 || a7 || a8!=1)
        RaiseException(0xE1234569,0,0,nullptr);
    ++catchupRegistrations;
    uintptr_t begin=0;memcpy(&begin,entryScene+88,8);FixturePtr(entryScene,96,begin+24);return 1;
}
static bool CatchupExceptionForwarded(EliteCatchupPositionGet call) {
    bool caught=false;
    __try {call(11,0);} __except(GetExceptionCode()==0xE1234567?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {caught=true;}
    return caught;
}
static int NativePathFixture(unsigned char* image) {
    // Execute copied current-EXE A* code, not a replacement path algorithm.
    unsigned char tiles[1000]={};int width=16,height=16;
    uintptr_t tilePointer=reinterpret_cast<uintptr_t>(tiles);
    auto reset=[&]() {
        memset(g_nativePathArena,0,sizeof(g_nativePathArena));
        memcpy(g_nativePathArena+114924,&width,4);memcpy(g_nativePathArena+114928,&height,4);
        memcpy(g_nativePathArena+114936,&tilePointer,8);
    };
    reset();
    if(EliteNativePathSearch(1,1,10,1,0,0,0)!=ElitePathResult::Reachable)return 1;
    // Full-height wall: open list exhausts without a route.
    for(int y=0;y<16;++y)tiles[y*16+8]=2;
    reset();
    if(EliteNativePathSearch(1,1,10,1,0,0,0)!=ElitePathResult::Unreachable)return 2;
    // Tile 1's native pass flag changes the result without a DLL tile table.
    for(int y=0;y<16;++y)tiles[y*16+8]=1;
    reset();if(EliteNativePathSearch(1,1,10,1,0,0,0)!=ElitePathResult::Unreachable)return 3;
    reset();if(EliteNativePathSearch(1,1,10,1,0,1,0)!=ElitePathResult::Reachable)return 4;
    memset(tiles,0,sizeof(tiles));width=1000;height=1;reset();
    if(EliteNativePathSearch(0,0,999,0,0,0,0)!=ElitePathResult::Unknown)return 5;
    if(EliteNativePathSearch(0,0,0,0,0,0,0)!=ElitePathResult::Reachable)return 6;
    // Our query stops before pool wrap; ordinary native allocator stays native.
    reset();g_nativePathActive=true;g_nativePathAllocations=0;bool limited=false;
    auto allocate=reinterpret_cast<ElitePathAllocate>(image+0x6D642C0);
    __try {for(int n=0;n<1025;++n)allocate(reinterpret_cast<uintptr_t>(g_nativePathArena));}
    __except(GetExceptionCode()==kElitePathPoolLimit?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {limited=true;}
    g_nativePathActive=false;
    if(!limited || g_nativePathAllocations!=1025)return 7;
    allocate(reinterpret_cast<uintptr_t>(g_nativePathArena));
    int allocated=0;memcpy(&allocated,g_nativePathArena+114920,4);
    if(allocated!=1 || g_nativePathAllocations!=1025)return 8;
    puts("actual EXE A*: reachable, disconnected wall, native pass flag, 500-search unknown, same cell, isolated node-pool limit and native allocator forwarding passed");
    return 0;
}
static int CatchupFixture(unsigned char* image) {
    auto original=g_catchupOriginal;auto native=reinterpret_cast<EliteCatchupPositionGet>(image+0x5E08940);
    const bool originalPathEnabled=g_catchupPathEnabled;
    g_catchupPathEnabled=true;
    g_catchupOriginal=CatchupThrow;if(!CatchupExceptionForwarded(native) || g_catchupBusy)return 1;
    g_catchupOriginal=CatchupComponent;g_catchupX=CatchupX;g_catchupQuery=CatchupQuery;g_catchupY=CatchupY;g_catchupZ=CatchupZ;
    // PID8232: loader context and global world are distinct native objects.
    FixturePtr(image,0xE66C090,reinterpret_cast<uintptr_t>(catchupWorld));FixtureInt(catchupWorld,4528,22);
    g_catchupMode=reinterpret_cast<EliteCatchupGet>(image+0x59A90F0);
    // Actual EXE getter and decoder exercise the two observed live states.
    CatchupMode(3);if(g_catchupMode(reinterpret_cast<uintptr_t>(catchupWorld))!=3)return 14;
    CatchupMode(1);if(g_catchupMode(reinterpret_cast<uintptr_t>(catchupWorld))!=1)return 15;CatchupMode(3);
    g_catchupScene=CatchupScene;g_catchupLand=CatchupLand;g_catchupSet=CatchupSet;
    FixturePtr(ownedVtable,432,reinterpret_cast<uintptr_t>(image+0x5B960D0));
    FixturePtr(ownedVtable,320,reinterpret_cast<uintptr_t>(image+0x1854970));
    FixturePtr(ownedVtable,328,reinterpret_cast<uintptr_t>(image+0x449BF20));
    auto retained=reinterpret_cast<EliteCatchupGet>(image+0x449BF20);
    auto enable=reinterpret_cast<EliteCatchupEnable>(image+0x1854970);
    memcpy(actorBefore,ownedActor,sizeof(ownedActor));enable(reinterpret_cast<uintptr_t>(ownedActor),1);
    actorBefore[720]=1;
    if(retained(reinterpret_cast<uintptr_t>(ownedActor))!=1 || memcmp(actorBefore,ownedActor,sizeof(ownedActor)))return 25;
    FixturePtr(ownedVtable,632,reinterpret_cast<uintptr_t>(image+0x5C70EE0));
    FixturePtr(ownedVtable,8464,reinterpret_cast<uintptr_t>(image+0x5CF6C20));
    FixturePtr(ownedPlayer,0,reinterpret_cast<uintptr_t>(ownedVtable));
    FixturePtr(ownedActor,368,reinterpret_cast<uintptr_t>(catchupPosition));FixturePtr(ownedPlayer,368,reinterpret_cast<uintptr_t>(catchupPlayerPosition));
    // PID23648 slot0 is alive but has the native derived virtual632 function.
    // A valid companion must not be dropped for overriding this native slot.
    FixturePtr(ownedVtable,632,reinterpret_cast<uintptr_t>(image+0x63DE5D0));
    if(!EliteCatchupActor(reinterpret_cast<uintptr_t>(ownedActor),true))return 28;
    // Execute the complete derived function and its real common-function fallback
    // from this EXE. Complex external state/map dependencies are ABI stubs only.
    FixturePtr(ownedVtable,5432,reinterpret_cast<uintptr_t>(LandingSpecial));
    FixturePtr(ownedVtable,536,reinterpret_cast<uintptr_t>(LandingState));
    LandingJump(image+0x5C70E00,reinterpret_cast<uintptr_t>(LandingBaseGate));
    LandingJump(image+0x5CF6910,reinterpret_cast<uintptr_t>(LandingBasePoint));
    const auto owned=reinterpret_cast<uintptr_t>(ownedActor);
    landingBaseAllowed=false;
    if(!EliteCatchupLanding(owned,37,91,23) || landingBaseCalls)return 29;
    landingState=6;
    if(EliteCatchupLanding(owned,37,91,23) || landingBaseCalls)return 30;
    landingState=5;landingSpecial=false;landingBaseAllowed=true;
    if(!EliteCatchupLanding(owned,37,91,23) || landingBaseCalls!=1 ||
        landingSeenActor!=owned || landingSeenX!=37 || landingSeenY!=91 || landingSeenContext!=23)return 31;
    landingBaseAllowed=false;
    if(EliteCatchupLanding(owned,37,91,23) || landingBaseCalls!=1)return 32;
    FixturePtr(ownedVtable,632,reinterpret_cast<uintptr_t>(image+0x63DE5D1));
    if(EliteCatchupActor(owned,true) || EliteCatchupLanding(owned,37,91,23) || landingBaseCalls!=1)return 33;
    landingSpecial=true;landingBaseAllowed=true;
    FixturePtr(ownedVtable,632,reinterpret_cast<uintptr_t>(image+0x5C70EE0));
    if(!EliteCatchupLanding(owned,42,73,19) || landingBaseCalls!=2 || landingSeenX!=42 || landingSeenY!=73 || landingSeenContext!=19)return 34;
    FixtureInt(ownedActor,1100,2000);FixtureInt(ownedPlayer,1100,100);
    uintptr_t refs[6]={0,reinterpret_cast<uintptr_t>(ownedStrong),reinterpret_cast<uintptr_t>(ownedActor+48),0,reinterpret_cast<uintptr_t>(ownedStrong),reinterpret_cast<uintptr_t>(ownedActor+48)};
    FixturePtr(entryScene,88,reinterpret_cast<uintptr_t>(refs));FixturePtr(entryScene,96,reinterpret_cast<uintptr_t>(refs+3));FixturePtr(entryScene,104,reinterpret_cast<uintptr_t>(refs+6));
    g_lifecycleArmed=true;g_lifecycleThread=GetCurrentThreadId();g_lifecycleChannel=22;g_lifecycleOwner=2;
    g_catchupPoll=0;g_catchupRecords=1024;EliteCatchupReset();
    const uintptr_t player=reinterpret_cast<uintptr_t>(catchupPlayerPosition),actor=reinterpret_cast<uintptr_t>(catchupPosition);
    for(uint64_t tick=10000;tick<=13000;tick+=250) {EliteCatchupSample(player,tick);EliteCatchupSample(actor,tick);}
    int x=0;memcpy(&x,ownedActor+1100,4);
    if(catchupMoves!=1 || x!=36 || g_catchupRecords!=1024)return 2;
    for(int reject=0;reject<11;++reject) {
        EliteCatchupReset();FixtureInt(ownedActor,1100,2000);CatchupMode(3);catchupPath=ElitePathResult::Unreachable;ownedActor[70914]=0;
        FixturePtr(entryScene,96,reinterpret_cast<uintptr_t>(refs+3));g_lifecycleOwner=2;
        FixturePtr(ownedActor,25280,reinterpret_cast<uintptr_t>(ownedPlayer+48));
        FixturePtr(ownedController,2088,reinterpret_cast<uintptr_t>(ownedActor+48));
        FixturePtr(ownedVtable,432,reinterpret_cast<uintptr_t>(image+0x5B960D0));
        FixturePtr(ownedVtable,320,reinterpret_cast<uintptr_t>(image+0x1854970));
        FixturePtr(ownedVtable,328,reinterpret_cast<uintptr_t>(image+0x449BF20));
        switch(reject) {case 0:CatchupMode(1);break;case 1:ownedActor[70914]=1;break;
            case 2:FixturePtr(entryScene,96,reinterpret_cast<uintptr_t>(refs));break;
            case 3:FixturePtr(entryScene,96,reinterpret_cast<uintptr_t>(refs+6));break;
            case 4:FixturePtr(ownedActor,25280,reinterpret_cast<uintptr_t>(ownedVictim+48));break;
            case 5:FixturePtr(ownedController,2088,reinterpret_cast<uintptr_t>(ownedPlayer+48));break;
            case 6:FixturePtr(ownedVtable,432,reinterpret_cast<uintptr_t>(image+0x5B98210));break;
            case 7:catchupPath=ElitePathResult::Reachable;break;case 8:catchupPath=ElitePathResult::Unknown;break;
            case 9:FixturePtr(ownedVtable,320,0);break;case 10:FixturePtr(ownedVtable,328,0);break;}
        for(uint64_t tick=20000+10000*reject;tick<=24000+10000*reject;tick+=250)EliteCatchupSample(player,tick);
        if(catchupMoves!=1)return 3+reject;
    }
    CatchupMode(3);FixturePtr(entryScene,96,reinterpret_cast<uintptr_t>(refs+3));
    // Default isolation: even an unreachable path stub must never be invoked,
    // nor may an already registered actor be moved or have any bytes changed.
    g_catchupPathEnabled=false;catchupPath=ElitePathResult::Unreachable;EliteCatchupReset();
    memcpy(actorBefore,ownedActor,sizeof(ownedActor));int queryCalls=catchupQueryCalls;
    for(uint64_t tick=115000;tick<=119000;tick+=500)EliteCatchupSample(player,tick);
    if(catchupQueryCalls!=queryCalls || catchupMoves!=1 || memcmp(actorBefore,ownedActor,sizeof(ownedActor)))return 37;
    FixturePtr(ownedVtable,320,reinterpret_cast<uintptr_t>(image+0x1854970));
    FixturePtr(ownedVtable,328,reinterpret_cast<uintptr_t>(image+0x449BF20));
    auto registerOriginal=g_entryRegisterOriginal;g_entryRegisterOriginal=CatchupRegister;
    if(EliteCatchupRegistrationFree(reinterpret_cast<uintptr_t>(entryScene),0))return 21;
    if(!EliteCatchupRegistrationFree(reinterpret_cast<uintptr_t>(entryScene),99))return 22;
    g_catchupAnchors[0]={};FixturePtr(image,0xE66C090,0);
    EliteCatchupRegistered(reinterpret_cast<uintptr_t>(entryWorld),reinterpret_cast<uintptr_t>(entryScene),reinterpret_cast<uintptr_t>(ownedActor),0);
    if(g_catchupAnchors[0].scene)return 23;
    FixturePtr(image,0xE66C090,reinterpret_cast<uintptr_t>(catchupWorld));
    EliteCatchupRegistered(reinterpret_cast<uintptr_t>(entryWorld),reinterpret_cast<uintptr_t>(entryScene),reinterpret_cast<uintptr_t>(ownedActor),0);
    if(!g_catchupAnchors[0].scene || g_catchupAnchors[0].world!=reinterpret_cast<uintptr_t>(catchupWorld) ||
        g_catchupAnchors[0].world==reinterpret_cast<uintptr_t>(entryWorld))return 16;
    // PID15920: registration alone leaves the original virtual328 return zero.
    // Use the real EXE getter/setter bytes to require native post-registration
    // activation; copying membership alone must fail this regression.
    enable(reinterpret_cast<uintptr_t>(ownedActor),0);
    FixturePtr(entryScene,96,reinterpret_cast<uintptr_t>(refs));catchupPath=ElitePathResult::Reachable;EliteCatchupReset();
    for(uint64_t tick=120000;tick<123000;tick+=500)EliteCatchupSample(player,tick);
    if(catchupRegistrations || catchupMoves!=1 || retained(reinterpret_cast<uintptr_t>(ownedActor))!=0)return 17;
    EliteCatchupSample(player,123000);
    if(catchupRegistrations!=1 || catchupMoves!=2 || EliteCatchupMembership(reinterpret_cast<uintptr_t>(entryScene),refs+1)!=1)return 18;
    if(retained(reinterpret_cast<uintptr_t>(ownedActor))!=1 || catchupQueryCalls!=queryCalls)return 26;
    for(uint64_t tick=123500;tick<=127000;tick+=500)EliteCatchupSample(player,tick);
    if(catchupRegistrations!=1 || catchupMoves!=2)return 19;
    // A replaced world still refuses recovery, even with unchanged CRefs/scene.
    FixturePtr(entryScene,96,reinterpret_cast<uintptr_t>(refs));
    memcpy(catchupReplacementWorld,catchupWorld,sizeof(catchupWorld));
    FixturePtr(image,0xE66C090,reinterpret_cast<uintptr_t>(catchupReplacementWorld));EliteCatchupReset();
    for(uint64_t tick=128000;tick<=132000;tick+=500)EliteCatchupSample(player,tick);
    if(catchupRegistrations!=1 || catchupMoves!=2)return 24;
    FixturePtr(image,0xE66C090,reinterpret_cast<uintptr_t>(catchupWorld));
    FixturePtr(entryScene,96,reinterpret_cast<uintptr_t>(refs));g_catchupAnchors[0].scene++;EliteCatchupReset();
    for(uint64_t tick=133000;tick<=137000;tick+=500)EliteCatchupSample(player,tick);
    if(catchupRegistrations!=1 || catchupMoves!=2)return 20;
    // Already enabled but temporarily missing: restore without changing its
    // other actor state. Native retention remains1 after the same 3s delay.
    g_catchupAnchors[0].scene--;EliteCatchupReset();
    for(uint64_t tick=140000;tick<=143000;tick+=500)EliteCatchupSample(player,tick);
    if(catchupRegistrations!=2 || catchupMoves!=3 || retained(reinterpret_cast<uintptr_t>(ownedActor))!=1)return 27;
    // The actual missing-registration path must retain the derived dispatch.
    FixturePtr(ownedVtable,632,reinterpret_cast<uintptr_t>(image+0x63DE5D0));
    g_catchupLand=EliteCatchupLanding;FixturePtr(entryScene,96,reinterpret_cast<uintptr_t>(refs));EliteCatchupReset();
    for(uint64_t tick=150000;tick<=153000;tick+=500)EliteCatchupSample(player,tick);
    if(catchupRegistrations!=3 || catchupMoves!=4 || retained(owned)!=1)return 35;
    // Inactive special actor permits the first check, but activation adds its
    // native state restrictions. Do not bypass a denial after reactivation.
    enable(owned,0);FixturePtr(ownedActor,28504,1);landingBaseAllowed=false;
    FixturePtr(entryScene,96,reinterpret_cast<uintptr_t>(refs));EliteCatchupReset();
    for(uint64_t tick=160000;tick<=163000;tick+=500)EliteCatchupSample(player,tick);
    if(catchupRegistrations!=4 || catchupMoves!=4 || retained(owned)!=1)return 36;
    FixturePtr(ownedActor,28504,0);landingBaseAllowed=true;
    FixturePtr(ownedVtable,632,reinterpret_cast<uintptr_t>(image+0x5C70EE0));g_catchupLand=CatchupLand;
    g_entryRegisterOriginal=registerOriginal;
    g_lifecycleArmed=false;SetLastError(ERROR_ACCESS_DENIED);int calls=catchupCalls;
    if(native(player,0)!=INT64_C(0x1234567800000064) || GetLastError()!=ERROR_ACCESS_DENIED || catchupCalls!=calls+1)return 12;
    if(native(player,1)!=INT64_C(0x1234567800000065) || native(player,2)!=INT64_C(0x1234567800000066) || GetLastError()!=ERROR_ACCESS_DENIED || catchupCalls!=calls+3)return 13;
    g_catchupOriginal=original;g_catchupPathEnabled=originalPathEnabled;EliteCatchupReset();return 0;
}

static int birthMoves,birthCalls;
static int __fastcall BirthX(uintptr_t a) {int v=0;memcpy(&v,reinterpret_cast<void*>(a+1100),4);return v;}
static int __fastcall BirthY(uintptr_t a) {int v=0;memcpy(&v,reinterpret_cast<void*>(a+1104),4);return v;}
static int __fastcall BirthZ(uintptr_t a) {int v=0;memcpy(&v,reinterpret_cast<void*>(a+1108),4);return v;}
static int64_t __fastcall BirthComponent(uintptr_t,int axis) {const auto a=reinterpret_cast<uintptr_t>(ownedActor);return axis==0?BirthX(a):axis==1?BirthY(a):BirthZ(a);}
static uintptr_t __fastcall BirthForce(uintptr_t a,int x,int y,int z) {
    ++birthMoves;FixtureInt(reinterpret_cast<unsigned char*>(a),1100,x);FixtureInt(reinterpret_cast<unsigned char*>(a),1104,y);FixtureInt(reinterpret_cast<unsigned char*>(a),1108,z);return 0;
}
static void __fastcall BirthHonored(uintptr_t a,int x,int y,int z) {
    ++birthCalls;BirthForce(a,x,y,z);--birthMoves;SetLastError(ERROR_ACCESS_DENIED);
}
static void __fastcall BirthThrow(uintptr_t,int,int,int) {RaiseException(0xE1234567,0,0,nullptr);}
static int birthLandingCalls,birthLandingMode;static bool birthBusyInvalid;
static unsigned char __fastcall BirthLanding(uintptr_t a,int x,int y,uintptr_t context) {
    ++birthLandingCalls;
    if(!g_catchupBusy || a!=reinterpret_cast<uintptr_t>(ownedActor) || context)birthBusyInvalid=true;
    if(birthLandingMode==1)return 0;
    if(birthLandingMode==2 && birthLandingCalls==3)return 0;
    return x>=0 && x<1120 && y==242?1:0;
}
static bool BirthExceptionForwarded(EliteBirthSet call,uintptr_t actor) {
    bool caught=false;
    __try {call(actor,900,100,0);} __except(GetExceptionCode()==0xE1234567?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {caught=true;}
    return caught;
}
static int BirthFixture(unsigned char* image) {
    const uintptr_t actor=reinterpret_cast<uintptr_t>(ownedActor),manager=reinterpret_cast<uintptr_t>(ownedManager);
    // Refuse altered bytes before creating a hook; guard the full wrapper and call.
    image[0x5B25312]^=1;if(EliteStartBirthPosition(image) || g_birthReady)return 1;image[0x5B25312]^=1;
    image[0x5B98210]^=1;if(EliteStartBirthPosition(image) || g_birthReady)return 2;image[0x5B98210]^=1;
    image[0x5C6AE80]^=1;if(EliteStartBirthPosition(image) || g_birthReady)return 18;image[0x5C6AE80]^=1;
    image[0x5C0FB80]^=1;if(EliteStartBirthPosition(image) || g_birthReady)return 19;image[0x5C0FB80]^=1;
    if(!EliteStartBirthPosition(image))return 3;
    g_birthLandingEnabled=false; // Verify prior cache behavior separately.
    FixturePtr(ownedVtable,408,reinterpret_cast<uintptr_t>(image+0x5B98210));
    FixturePtr(ownedVtable,5688,reinterpret_cast<uintptr_t>(image+0x5C6AE80));
    g_lifecycleArmed=true;g_lifecycleThread=GetCurrentThreadId();g_lifecycleChannel=22;g_lifecycleOwner=2;
    g_entryWorld=reinterpret_cast<uintptr_t>(entryWorld);g_entryScene=reinterpret_cast<uintptr_t>(entryScene);
    g_entryFreshDungeon=true;g_entryLoaderCaller=0x6D2A1E7;g_registrationPermitted=true;
    g_registrationDungeon=reinterpret_cast<uintptr_t>(entryDungeon);g_registrationManager=manager;
    FixturePtr(entryWorld,6792,g_registrationDungeon);
    uintptr_t refs[6]={0,reinterpret_cast<uintptr_t>(ownedStrong),reinterpret_cast<uintptr_t>(ownedActor+48),0,reinterpret_cast<uintptr_t>(ownedStrong),reinterpret_cast<uintptr_t>(ownedActor+48)};
    FixturePtr(entryScene,88,reinterpret_cast<uintptr_t>(refs));FixturePtr(entryScene,96,reinterpret_cast<uintptr_t>(refs+3));FixturePtr(entryScene,104,reinterpret_cast<uintptr_t>(refs+6));
    g_catchupX=BirthX;g_catchupY=BirthY;g_catchupZ=BirthZ;g_catchupSet=BirthForce;
    // Execute the actual EXE conditional birth setter with an unbound position
    // owner. It sets dirty312 but leaves all XYZ untouched: the native early exit.
    memset(catchupPosition,0,sizeof(catchupPosition));FixtureInt(ownedActor,1100,0);FixtureInt(ownedActor,1104,0);FixtureInt(ownedActor,1108,71);
    g_birthOriginal(actor,900,100,0);
    if(BirthX(actor) || BirthY(actor) || BirthZ(actor)!=71 || catchupPosition[312]!=1)return 4;
    const auto original=g_birthOriginal;
    // Native virtual call's exact return address, test-process shadow space only.
    const unsigned char prolog[]={0x48,0x83,0xec,0x28},epilog[]={0x48,0x83,0xc4,0x28,0xc3};
    memcpy(image+0x5B2531C,prolog,4);image[0x5B25320]=0xE8;
    int32_t rel=static_cast<int32_t>(0x5B98210-0x5B25325);memcpy(image+0x5B25321,&rel,4);memcpy(image+0x5B25325,epilog,5);
    FlushInstructionCache(GetCurrentProcess(),image+0x5B2531C,14);
    auto precise=reinterpret_cast<EliteBirthSet>(image+0x5B2531C);
    auto other=reinterpret_cast<EliteBirthSet>(image+0x5B98210);
    const uint64_t queries=g_catchupPathQueries;
    precise(actor,900,100,0);
    if(birthMoves || BirthX(actor) || BirthY(actor) || BirthZ(actor)!=71 || g_catchupPathQueries!=queries)return 5;
    // Execute the actual EXE cache function, not a reimplementation. Its native
    // getter dependencies already use the coordinate fixture through the hook.
    const auto originalGet=g_catchupOriginal;g_catchupOriginal=BirthComponent;
    FixtureInt(ownedContext,5408,0);FixtureInt(ownedContext,5412,0);
    FixtureInt(ownedActor,25180,42);
    g_birthOriginal=BirthHonored;SetLastError(ERROR_ACCESS_DENIED);precise(actor,800,120,0);
    if(birthCalls!=1 || birthMoves!=0 || BirthX(actor)!=800 || BirthY(actor)!=120 || GetLastError()!=ERROR_ACCESS_DENIED)return 6;
    int cache[2]={};memcpy(cache,ownedContext+5408,8);
    if(cache[0]!=800 || cache[1]!=120 || *reinterpret_cast<int*>(ownedActor+25180)!=-1 || g_catchupPathQueries!=queries)return 20;
    // Native rollback now requests the accepted birth point instead of origin.
    // Preserve valid/sentinel caches and reject a foreign native virtual slot.
    for(int reject=0;reject<4;++reject) {
        const int bx=reject==0?700:reject==1?-1:0,by=reject==0?90:reject==1?-1:0;
        FixtureInt(ownedContext,5408,bx);FixtureInt(ownedContext,5412,by);FixtureInt(ownedActor,25180,42);
        if(reject==2)FixturePtr(ownedVtable,5688,0);
        if(reject==3)g_registrationPermitted=false;
        precise(actor,900,100,0);memcpy(cache,ownedContext+5408,8);
        if(cache[0]!=bx || cache[1]!=by || *reinterpret_cast<int*>(ownedActor+25180)!=42 || birthMoves)return 21+reject;
        FixturePtr(ownedVtable,5688,reinterpret_cast<uintptr_t>(image+0x5C6AE80));g_registrationPermitted=true;
    }
    g_birthOriginal=original;
    for(int reject=0;reject<9;++reject) {
        FixtureInt(ownedActor,1100,0);FixtureInt(ownedActor,1104,0);FixtureInt(ownedActor,1108,71);
        switch(reject) {
        case 0:g_registrationPermitted=false;break;case 1:g_entryFreshDungeon=false;break;
        case 2:FixturePtr(entryScene,96,reinterpret_cast<uintptr_t>(refs));break;
        case 3:FixturePtr(entryScene,96,reinterpret_cast<uintptr_t>(refs+6));break;
        case 4:g_registrationManager++;break;case 5:g_lifecycleOwner++;break;
        case 6:ownedActor[70914]=1;break;case 7:FixturePtr(ownedVtable,408,0);break;
        }
        if(reject==8)other(actor,900,100,0);else precise(actor,900,100,0);
        if(birthMoves!=0 || BirthX(actor) || BirthY(actor) || BirthZ(actor)!=71)return 7+reject;
        // Repeat the same scope refusal with a successful native birth: a
        // rejected scope must not synchronize cache despite correct XYZ.
        FixtureInt(ownedContext,5408,0);FixtureInt(ownedContext,5412,0);FixtureInt(ownedActor,25180,42);
        g_birthOriginal=BirthHonored;
        if(reject==8)other(actor,900,100,0);else precise(actor,900,100,0);
        memcpy(cache,ownedContext+5408,8);
        if(cache[0] || cache[1] || *reinterpret_cast<int*>(ownedActor+25180)!=42 || birthMoves)return 25+reject;
        g_birthOriginal=original;
        g_registrationPermitted=true;g_entryFreshDungeon=true;g_registrationManager=manager;g_lifecycleOwner=2;
        FixturePtr(entryScene,96,reinterpret_cast<uintptr_t>(refs+3));ownedActor[70914]=0;FixturePtr(ownedVtable,408,reinterpret_cast<uintptr_t>(image+0x5B98210));
    }
    FixtureInt(ownedActor,1100,0);FixtureInt(ownedActor,1104,0);FixtureInt(ownedActor,1108,71);
    precise(actor,900,100,17);if(birthMoves!=0 || BirthZ(actor)!=71)return 16;
    g_birthOriginal=BirthThrow;if(!BirthExceptionForwarded(precise,actor) || g_birthFault)return 17;
    // Reproduce the live rejection using the real EXE map check: flags273,
    // X1193, map width1120. It must return before the viewport/terrain calls.
    static unsigned char birthMap[24000]={};FixtureInt(birthMap,16472,1120);FixtureInt(ownedActor,348,273);
    FixtureInt(ownedActor,1100,1193);FixtureInt(ownedActor,1104,242);FixtureInt(ownedActor,1108,0);
    if(reinterpret_cast<unsigned char (__fastcall *)(uintptr_t,uintptr_t,char)>(image+0x4D27990)(reinterpret_cast<uintptr_t>(birthMap),actor,1)!=0)return 34;
    FixtureInt(ownedActor,348,0);
    const auto savedLand=g_catchupLand;g_catchupLand=BirthLanding;g_birthLandingEnabled=true;
    FixturePtr(ownedPlayer,0,reinterpret_cast<uintptr_t>(ownedVtable));FixturePtr(ownedPlayer,368,reinterpret_cast<uintptr_t>(catchupPosition));
    FixtureInt(ownedPlayer,1100,1093);FixtureInt(ownedPlayer,1104,242);FixtureInt(ownedPlayer,1108,0);
    refs[0]=0;refs[1]=reinterpret_cast<uintptr_t>(playerStrong);refs[2]=reinterpret_cast<uintptr_t>(ownedPlayer+48);
    FixturePtr(entryScene,96,reinterpret_cast<uintptr_t>(refs+6));
    FixturePtr(ownedActor,400,reinterpret_cast<uintptr_t>(ownedStrong));FixturePtr(ownedActor,408,g_entryScene);
    g_birthOriginal=BirthHonored;
    const uintptr_t world=reinterpret_cast<uintptr_t>(catchupWorld),scene=g_entryScene;
    const uintptr_t ownerComponent=reinterpret_cast<uintptr_t>(catchupPlayerPosition);
    FixturePtr(ownedPlayer,368,ownerComponent);
    const auto roomBefore=g_eliteRoomSerial;
    g_eliteRoomSerial=42;
    EliteCatchupKey birthKey={world,scene,reinterpret_cast<uintptr_t>(playerStrong),reinterpret_cast<uintptr_t>(ownedPlayer+48),reinterpret_cast<uintptr_t>(ownedStrong),actor+48,0,42};
    auto anchor=birthKey;anchor.room=0;
    g_catchupAnchors[0]=anchor;
    const bool savedPath=g_catchupPathEnabled;g_catchupPathEnabled=false;
    for(int scenario=0;scenario<16;++scenario) {
        birthCalls=0;birthLandingCalls=0;birthBusyInvalid=false;birthLandingMode=scenario==2?1:scenario==3?2:0;
        g_birthPending[0]={};g_eliteRoomSerial=42;g_catchupAnchors[0]=anchor;
        FixtureInt(ownedContext,5408,0);FixtureInt(ownedContext,5412,0);
        FixtureInt(ownedPlayer,1108,scenario==4?17:0);
        FixturePtr(entryScene,88,reinterpret_cast<uintptr_t>(refs));
        if(scenario==5)FixturePtr(entryScene,88,reinterpret_cast<uintptr_t>(refs+3)); // APC present, owner absent.
        if(scenario==6)FixtureInt(ownedContext,5408,-1);
        if(scenario==7)g_registrationPermitted=false;
        if(scenario==8)FixturePtr(ownedActor,408,0);
        g_entryWorld=reinterpret_cast<uintptr_t>(entryWorld);g_entryScene=scene;CatchupMode(2);
        const int nativeX=scenario==1?1093:1193;
        precise(actor,nativeX,242,0);memcpy(cache,ownedContext+5408,8);
        // During loading there are NO landing queries or fallback setters,
        // even when the owner is absent, airborne, or the APC sceneRef is empty.
        const bool queued=scenario!=6 && scenario!=7;
        const int expectedCache=scenario==6?-1:scenario==7?0:nativeX;
        if(birthCalls!=1 || birthLandingCalls || BirthX(actor)!=nativeX || BirthY(actor)!=242 || BirthZ(actor)!=0 ||
           cache[0]!=expectedCache || cache[1]!=(scenario==6 || scenario==7?0:242) || g_birthPending[0].active!=queued ||
           GetLastError()!=ERROR_ACCESS_DENIED || g_catchupBusy || g_catchupPathQueries!=queries)return 60+scenario;
        g_entryWorld=0;CatchupMode(3);
        if(scenario==9)++g_eliteRoomSerial;
        if(scenario==11)FixtureInt(ownedActor,1100,800);
        if(scenario==12)g_birthPending[0].context++;
        if(scenario==13)g_birthPending[0].manager++;
        if(scenario==14)g_entryWorld=reinterpret_cast<uintptr_t>(entryWorld);
        if(scenario==15)CatchupMode(2);
        g_catchupPoll=0;g_catchupLog=0;g_catchupBusy=true;
        if(scenario==10) {auto foreign=birthKey;foreign.playerAlias++;EliteBirthSample(0,foreign,1093,242,0);}
        else EliteCatchupSample(ownerComponent,200000+static_cast<uint64_t>(scenario)*10000);
        g_catchupBusy=false;
        // Loading/missing-owner/airborne checks preserve the pending birth
        // until the same valid generation becomes eligible, without native calls.
        if(scenario==4 || scenario==5 || scenario==13 || scenario==14 || scenario==15) {
            memcpy(cache,ownedContext+5408,8);
            if(birthCalls!=1 || birthLandingCalls || cache[0]!=1193 || cache[1]!=242 || !g_birthPending[0].active)return 80+scenario;
            FixtureInt(ownedPlayer,1108,0);FixturePtr(entryScene,88,reinterpret_cast<uintptr_t>(refs));
            g_birthPending[0].manager=manager;g_entryWorld=0;CatchupMode(3);
            g_catchupBusy=true;g_catchupPoll=0;EliteCatchupSample(ownerComponent,205000+static_cast<uint64_t>(scenario)*10000);g_catchupBusy=false;
        }
        memcpy(cache,ownedContext+5408,8);
        const bool moved=scenario==0 || scenario==4 || scenario==5 || scenario==13 || scenario==14 || scenario==15;
        const int expectedCalls=scenario==3?3:moved?2:1;
        const int expectedGates=scenario==3 || moved?3:scenario==2?2:scenario==1?1:0;
        const int actualX=scenario==11?800:moved?1093:nativeX;
        if(birthCalls!=expectedCalls || birthLandingCalls!=expectedGates || BirthX(actor)!=actualX || BirthY(actor)!=242 || BirthZ(actor)!=0 ||
           cache[0]!=(moved?1093:expectedCache) || cache[1]!=(scenario==6 || scenario==7?0:242) ||
           g_birthPending[0].active || birthBusyInvalid || g_catchupPathQueries!=queries) {
            printf("deferred birth scenario%d: calls%d gates%d xyz%d/%d/%d cache%d/%d pending%d invalid%d\n",scenario,birthCalls,birthLandingCalls,BirthX(actor),BirthY(actor),BirthZ(actor),cache[0],cache[1],g_birthPending[0].active,birthBusyInvalid);return 100+scenario;
        }
        // No repeat fallback on later native owner samples, including rejection.
        g_catchupBusy=true;g_catchupPoll=0;EliteCatchupSample(ownerComponent,208000+static_cast<uint64_t>(scenario)*10000);g_catchupBusy=false;
        if(birthCalls!=expectedCalls || birthLandingCalls!=expectedGates)return 120+scenario;
        FixturePtr(entryScene,88,reinterpret_cast<uintptr_t>(refs));g_registrationPermitted=true;FixturePtr(ownedActor,408,scene);
    }
    g_catchupPathEnabled=savedPath;g_eliteRoomSerial=roomBefore;g_birthPending[0]={};
    g_catchupLand=savedLand;g_birthLandingEnabled=false;
    g_birthOriginal=original;g_catchupOriginal=originalGet;g_lifecycleArmed=false;g_entryWorld=0;g_entryScene=0;EliteRegistrationReset();
    puts("actual EXE birth/cache/map rejection; deferred birth16 scenarios: no landing during loading, zero-cache always synchronized, guarded mode3 owner seam, single native-approved correction/cache update, final refusal restore, airborne/missing-owner/manager/loading wait, scene/room/identity/sentinel refusals; LastError/SEH preserved; no A* calls");
    return 0;
}


static int positionCalls,positionAxis,positionValue;
static uintptr_t positionComponent;
static int64_t __fastcall PositionGet(uintptr_t p,int axis) {
    if(p!=reinterpret_cast<uintptr_t>(catchupPosition) || axis<0 || axis>2)RaiseException(0xE1234567,0,0,nullptr);
    return axis==0?BirthX(reinterpret_cast<uintptr_t>(ownedActor)):axis==1?BirthY(reinterpret_cast<uintptr_t>(ownedActor)):BirthZ(reinterpret_cast<uintptr_t>(ownedActor));
}
static uintptr_t __fastcall PositionForce(uintptr_t p,int axis,int value) {
    if(GetLastError()!=ERROR_ACCESS_DENIED)RaiseException(0xE1234567,0,0,nullptr);
    ++positionCalls;positionComponent=p;positionAxis=axis;positionValue=value;
    FixtureInt(ownedActor,1100+4*static_cast<size_t>(axis),value);SetLastError(ERROR_INVALID_DATA);
    return UINT64_C(0xfedcba9876543210);
}
static void __fastcall PositionReject(uintptr_t p,int axis,int value) {
    if(GetLastError()!=ERROR_ACCESS_DENIED)RaiseException(0xE1234567,0,0,nullptr);
    ++positionCalls;positionComponent=p;positionAxis=axis;positionValue=value;SetLastError(ERROR_INVALID_DATA);
}
static uintptr_t __fastcall PositionThrow(uintptr_t,int,int) {RaiseException(0xE1234567,0,0,nullptr);return 0;}
static bool PositionExceptionForwarded(ElitePositionForce call,uintptr_t component) {
    bool caught=false;
    __try {call(component,0,3);} __except(GetExceptionCode()==0xE1234567?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {caught=true;}
    return caught;
}
static int PositionFixture(unsigned char* image) {
    image[0x5E0CD40]^=1;if(EliteStartPositionWrites(image,L"unused-position") || g_positionReady)return 1;image[0x5E0CD40]^=1;
    image[0x5E0EC50]^=1;if(EliteStartPositionWrites(image,L"unused-position") || g_positionReady)return 2;image[0x5E0EC50]^=1;
    if(!EliteStartPositionWrites(image,L"unused-position"))return 3;
    auto originalForce=g_positionForceOriginal;auto originalConditional=g_positionConditionalOriginal;auto originalGet=g_catchupOriginal;
    auto force=reinterpret_cast<ElitePositionForce>(image+0x5E0CD40);
    auto conditional=reinterpret_cast<ElitePositionConditional>(image+0x5E0EC50);
    g_positionForceOriginal=PositionForce;g_positionConditionalOriginal=PositionReject;g_catchupOriginal=PositionGet;
    g_lifecycleArmed=true;g_lifecycleThread=GetCurrentThreadId();g_lifecycleOwner=2;g_lifecycleChannel=22;
    uintptr_t refs[3]={0,reinterpret_cast<uintptr_t>(ownedStrong),reinterpret_cast<uintptr_t>(ownedActor+48)};
    FixturePtr(entryScene,88,reinterpret_cast<uintptr_t>(refs));FixturePtr(entryScene,96,reinterpret_cast<uintptr_t>(refs+3));FixturePtr(entryScene,104,reinterpret_cast<uintptr_t>(refs+3));
    FixturePtr(image,0xE66C090,reinterpret_cast<uintptr_t>(catchupWorld));
    g_catchupAnchors[0]={reinterpret_cast<uintptr_t>(catchupWorld),reinterpret_cast<uintptr_t>(entryScene),reinterpret_cast<uintptr_t>(playerStrong),reinterpret_cast<uintptr_t>(ownedPlayer+48),reinterpret_cast<uintptr_t>(ownedStrong),reinterpret_cast<uintptr_t>(ownedActor+48),0,0};
    g_catchupAnchors[1]={};g_catchupAnchors[2]={};
    const uintptr_t component=reinterpret_cast<uintptr_t>(catchupPosition);const uint64_t queries=g_catchupPathQueries;
    FixtureInt(ownedActor,1100,900);FixtureInt(ownedActor,1104,100);FixtureInt(ownedActor,1108,0);
    SetLastError(ERROR_ACCESS_DENIED);
    auto forceResult=force(component,0,0);auto forceError=GetLastError();
    if(forceResult!=UINT64_C(0xfedcba9876543210) || forceError!=ERROR_INVALID_DATA || positionCalls!=1 ||
       positionComponent!=component || positionAxis!=0 || positionValue!=0 || g_positionRecords!=1 || BirthX(reinterpret_cast<uintptr_t>(ownedActor))!=0) {
        uintptr_t player[2]={};EliteCurrentPlayerReference(player);auto snapshot=EliteReadCompanions(reinterpret_cast<uintptr_t>(ownedManager),EliteCopy,player);
        printf("position result=%llu error=%lu calls=%d records=%u fault=%d identity=%d available=%d consistent=%d id=%lld bound=%d owner=%d component=%llu expected=%llu\n",
            static_cast<unsigned long long>(forceResult),forceError,positionCalls,g_positionRecords,g_positionFault,EliteEntryIdentity(),snapshot.available,snapshot.consistent,snapshot.objectId[0],snapshot.controllerBound[0],snapshot.ownerControllerId[0],static_cast<unsigned long long>(positionComponent),static_cast<unsigned long long>(component));return 4;
    }
    // A rejected native conditional request must stay rejected, with no completion.
    SetLastError(ERROR_ACCESS_DENIED);conditional(component,1,0);
    if(positionCalls!=2 || BirthY(reinterpret_cast<uintptr_t>(ownedActor))!=100 || g_positionRecords!=2 || GetLastError()!=ERROR_INVALID_DATA)return 5;
    unsigned records=g_positionRecords;
    for(int reject=0;reject<5;++reject) {
        switch(reject) {
        case 0:g_lifecycleArmed=false;break;case 1:g_lifecycleOwner++;break;
        case 2:FixtureInt(ownedStrong,8,0);break;case 3:g_catchupAnchors[0].alias++;break;
        case 4:g_lifecycleThread++;break;
        }
        SetLastError(ERROR_ACCESS_DENIED);conditional(component,2,0);
        if(g_positionRecords!=records || GetLastError()!=ERROR_INVALID_DATA)return 6+reject;
        g_lifecycleArmed=true;g_lifecycleOwner=2;FixtureInt(ownedStrong,8,1);g_catchupAnchors[0].alias=reinterpret_cast<uintptr_t>(ownedActor+48);g_lifecycleThread=GetCurrentThreadId();
    }
    // Sampling budget cannot hide a later nonzero->zero reset.
    g_positionSamples[0]=1000;FixtureInt(ownedActor,1100,900);SetLastError(ERROR_ACCESS_DENIED);force(component,0,0);
    if(g_positionRecords!=records+1 || g_catchupPathQueries!=queries)return 11;
    g_positionForceOriginal=PositionThrow;if(!PositionExceptionForwarded(force,component) || g_positionBusy || g_positionFault)return 12;
    g_positionForceOriginal=originalForce;g_positionConditionalOriginal=originalConditional;g_catchupOriginal=originalGet;
    g_lifecycleArmed=false;
    puts("position write guards, axis/value/64-bit return/error/SEH forwarding, native rejection, 5 identity refusals and late zero reset capture passed; no A* calls");
    return 0;
}

static char __fastcall ThrowStub() {RaiseException(0xE1234567,0,0,nullptr);return 0;}
static bool ExceptionForwarded(EliteNativePredicate call) {
    bool caught=false;
    __try {call();} __except(GetExceptionCode()==0xE1234567?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {caught=true;}
    return caught;
}

static uintptr_t permissionGridType=2;
static int permissionGridCalls,permissionAbiCalls;
static unsigned char permissionTree[64];
static unsigned char __fastcall PermissionKind(uintptr_t) {return 1;}
static uintptr_t __fastcall PermissionGrid(uintptr_t,int x,int y) {
    if(x!=900/16 || y!=100/12)RaiseException(0xE1234567,0,0,nullptr);
    ++permissionGridCalls;return permissionGridType;
}
static uintptr_t __fastcall PermissionAbi(uintptr_t a,int x,int y) {
    if(a!=reinterpret_cast<uintptr_t>(ownedActor) || x!=900 || y!=100 || GetLastError()!=ERROR_ACCESS_DENIED)RaiseException(0xE1234567,0,0,nullptr);
    ++permissionAbiCalls;SetLastError(ERROR_INVALID_DATA);return UINT64_C(0xfedcba9876543201);
}
static uintptr_t __fastcall PermissionThrow(uintptr_t,int,int) {RaiseException(0xE1234567,0,0,nullptr);return 0;}
static bool PermissionExceptionForwarded(ElitePermission3 call,uintptr_t a) {
    bool caught=false;__try {call(a,900,100);} __except(GetExceptionCode()==0xE1234567?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {caught=true;}
    return caught;
}
static int permissionBirthCalls;
static unsigned char __fastcall PermissionBirthAbi(uintptr_t a,int x,int y,uintptr_t auxiliary) {
    if(a!=reinterpret_cast<uintptr_t>(ownedActor) || x!=900 || y!=100 || auxiliary!=123 || GetLastError()!=ERROR_ACCESS_DENIED)RaiseException(0xE1234567,0,0,nullptr);
    ++permissionBirthCalls;SetLastError(ERROR_INVALID_DATA);return 0xab;
}
static unsigned char __fastcall PermissionBirthThrow(uintptr_t,int,int,uintptr_t) {RaiseException(0xE1234567,0,0,nullptr);return 0;}
static bool PermissionBirthException(uintptr_t actor) {
    bool caught=false;__try {ElitePermissionBirthObserve(actor,900,100,123);} __except(GetExceptionCode()==0xE1234567?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {caught=true;}return caught;
}
static int PermissionFixture(unsigned char* image) {
    // Earlier derived-landing tests intentionally substituted dependencies.
    // Restore the complete current EXE leaves before validating/installing hooks.
    memcpy(image+0x5C70E00,permissionSavedGate,128);memcpy(image+0x5CF6910,permissionSavedEffects,512);
    FlushInstructionCache(GetCurrentProcess(),image+0x5C70E00,128);FlushInstructionCache(GetCurrentProcess(),image+0x5CF6910,512);
    for(const auto& guard:kElitePermissionGuards) {
        image[guard.rva]^=1;bool accepted=EliteStartPermissionTrace(image,L"unused-permission.jsonl");image[guard.rva]^=1;
        if(accepted || g_permissionReady)return 1;
    }
    if(!EliteStartPermissionTrace(image,L"unused-permission.jsonl")) {printf("permission start: %s\n",g_permissionFailure);return 2;}
    const uintptr_t a=reinterpret_cast<uintptr_t>(ownedActor);
    const auto originalPoint=g_permissionPoint,originalGrid=g_permissionGrid;const auto originalGet=g_catchupOriginal;
    auto call=reinterpret_cast<ElitePermission3>(image+0x5C70E90);
    FixturePtr(ownedVtable,5680,reinterpret_cast<uintptr_t>(image+0x5C70E90));
    FixturePtr(ownedVtable,144,reinterpret_cast<uintptr_t>(PermissionKind));
    FixturePtr(ownedVtable,632,reinterpret_cast<uintptr_t>(image+0x5C70EE0));
    FixturePtr(ownedVtable,328,reinterpret_cast<uintptr_t>(image+0x449BF20));
    FixturePtr(ownedVtable,5432,reinterpret_cast<uintptr_t>(LandingSpecial));landingSpecial=false;
    FixturePtr(ownedActor,400,0);FixturePtr(ownedActor,408,0);
    FixturePtr(ownedActor,74144,reinterpret_cast<uintptr_t>(permissionTree));FixturePtr(permissionTree,0,reinterpret_cast<uintptr_t>(permissionTree));
    FixturePtr(ownedActor,28488,0);FixturePtr(ownedActor,28504,0);FixturePtr(ownedActor,28520,0);
    for(size_t i=0;i<4;++i)FixtureInt(ownedActor,74372+4*i,-1);
    ownedActor[27859]=0;ownedActor[720]=1;image[0xE6839A4]=0;image[0xE6BA760]=0;
    g_lifecycleArmed=true;g_lifecycleThread=GetCurrentThreadId();g_lifecycleOwner=2;g_lifecycleChannel=22;
    uintptr_t refs[3]={0,reinterpret_cast<uintptr_t>(ownedStrong),reinterpret_cast<uintptr_t>(ownedActor+48)};
    FixturePtr(entryScene,88,reinterpret_cast<uintptr_t>(refs));FixturePtr(entryScene,96,reinterpret_cast<uintptr_t>(refs+3));FixturePtr(entryScene,104,reinterpret_cast<uintptr_t>(refs+3));
    FixturePtr(image,0xE66C090,reinterpret_cast<uintptr_t>(catchupWorld));
    g_catchupAnchors[0]={reinterpret_cast<uintptr_t>(catchupWorld),reinterpret_cast<uintptr_t>(entryScene),reinterpret_cast<uintptr_t>(playerStrong),reinterpret_cast<uintptr_t>(ownedPlayer+48),reinterpret_cast<uintptr_t>(ownedStrong),reinterpret_cast<uintptr_t>(ownedActor+48),0,0};
    g_catchupAnchors[1]={};g_catchupAnchors[2]={};
    g_catchupOriginal=PositionGet;g_permissionGrid=PermissionGrid;
    FixtureInt(ownedActor,1100,900);FixtureInt(ownedActor,1104,100);FixtureInt(ownedActor,1108,0);
    const auto queries=g_catchupPathQueries;unsigned records=g_permissionRecords;
    // Execute current EXE top/map/bounds/virtual632/effect/motion/collision
    // functions; only the external map grid and virtual kind/state are stubs.
    SetLastError(ERROR_ACCESS_DENIED);auto result=call(a,900,100);
    if((result&255)!=0 || permissionGridCalls!=1 || g_permissionRecords!=records+3 || g_permissionContext || GetLastError()!=ERROR_ACCESS_DENIED)return 3;
    records=g_permissionRecords;permissionGridType=0;ownedActor[27859]=1;
    SetLastError(ERROR_ACCESS_DENIED);result=call(a,900,100);
    if((result&255)!=0 || g_permissionRecords<=records+3 || g_permissionContext || BirthX(a)!=900 || BirthY(a)!=100)return 4;
    records=g_permissionRecords;ownedActor[27859]=0;
    SetLastError(ERROR_ACCESS_DENIED);result=call(a,900,100);
    if((result&255)!=1 || g_permissionRecords<=records+3 || g_permissionContext || GetLastError()!=ERROR_ACCESS_DENIED || g_catchupPathQueries!=queries)return 5;
    const auto savedLand=g_catchupLand;g_catchupLand=PermissionBirthAbi;
    records=g_permissionRecords;SetLastError(ERROR_ACCESS_DENIED);
    if(ElitePermissionBirthObserve(a,900,100,123)!=0xab || permissionBirthCalls!=1 || g_permissionRecords!=records+1 ||
       GetLastError()!=ERROR_INVALID_DATA || g_permissionContext)return 10;
    g_lifecycleOwner=3;SetLastError(ERROR_ACCESS_DENIED);
    if(ElitePermissionBirthObserve(a,900,100,123)!=0xab || permissionBirthCalls!=2 || g_permissionRecords!=records+1 ||
       GetLastError()!=ERROR_INVALID_DATA || g_permissionContext)return 11;
    g_lifecycleOwner=2;g_catchupLand=PermissionBirthThrow;
    if(!PermissionBirthException(a) || g_permissionContext || g_permissionFault)return 12;
    g_catchupLand=savedLand;
    g_permissionPoint=PermissionAbi;SetLastError(ERROR_ACCESS_DENIED);result=call(a,900,100);
    if(result!=UINT64_C(0xfedcba9876543201) || permissionAbiCalls!=1 || GetLastError()!=ERROR_INVALID_DATA || g_permissionContext)return 6;
    records=g_permissionRecords;g_lifecycleOwner=3;SetLastError(ERROR_ACCESS_DENIED);call(a,900,100);
    if(g_permissionRecords!=records || permissionAbiCalls!=2 || GetLastError()!=ERROR_INVALID_DATA)return 7;
    g_lifecycleOwner=2;g_permissionRecords=2048;SetLastError(ERROR_ACCESS_DENIED);call(a,900,100);
    if(g_permissionRecords!=2048 || permissionAbiCalls!=3 || GetLastError()!=ERROR_INVALID_DATA)return 8;
    g_permissionPoint=PermissionThrow;g_permissionSamples[0]=0;
    if(!PermissionExceptionForwarded(call,a) || g_permissionContext || g_permissionFault)return 9;
    g_permissionPoint=originalPoint;g_permissionGrid=originalGrid;g_catchupOriginal=originalGet;g_lifecycleArmed=false;
    puts("permission observation: 8 current EXE guards, native map rejection / blocked flag / allowed point, full RAX and entry/native errors, foreign owner/budget refusal, TLS cleanup on native exception; no extra native checks or A* passed");
    return 0;
}


static unsigned char collisionMap[24000];
static LONG WINAPI StartupFixtureException(EXCEPTION_POINTERS* e) {
    printf("offline fixture unhandled exception=%08lx RIP=%llx collision-image-rva=%llx address=%llx\n",e->ExceptionRecord->ExceptionCode,e->ContextRecord->Rip,e->ContextRecord->Rip-reinterpret_cast<uintptr_t>(g_prepareImage),e->ExceptionRecord->NumberParameters>1?static_cast<unsigned long long>(e->ExceptionRecord->ExceptionInformation[1]):0ULL);
    return EXCEPTION_EXECUTE_HANDLER;
}
static uintptr_t collisionTestImage;static int collisionNativeCalls;
static uintptr_t __fastcall CollisionZero(uintptr_t) {return 0;}
static uintptr_t __fastcall CollisionCoordinate(uintptr_t a,uintptr_t axis) {return static_cast<uint32_t>(axis?BirthY(a):BirthX(a));}
static uintptr_t __fastcall CollisionX(uintptr_t a) {return CollisionCoordinate(a,0);}
static uintptr_t __fastcall CollisionY(uintptr_t a) {return CollisionCoordinate(a,1);}
static uintptr_t __fastcall CollisionAbi(uintptr_t a,int x,int y,uintptr_t aux) {
    if(a!=reinterpret_cast<uintptr_t>(ownedActor) || x!=900 || y!=100 || aux!=123 || GetLastError()!=ERROR_ACCESS_DENIED)RaiseException(0xE1234567,0,0,nullptr);
    ++collisionNativeCalls;
    const auto image=reinterpret_cast<unsigned char*>(collisionTestImage);const uintptr_t map=reinterpret_cast<uintptr_t>(collisionMap);
    int rectangle[4]={800,50,200,100};
    if(reinterpret_cast<EliteCollision1>(image+0x5B8C7D0)(a)!=900 ||
       reinterpret_cast<EliteCollision1>(image+0x5B8C890)(a)!=100 ||
       reinterpret_cast<EliteCollision1>(image+0x5B8C7E0)(a)!=900 ||
       reinterpret_cast<EliteCollision1>(image+0x5B8C8B0)(a)!=100 ||
       reinterpret_cast<EliteCollision1>(image+0x4D24C40)(map)!=1300 ||
       (reinterpret_cast<EliteCollision3>(image+0x7211920)(900,100,reinterpret_cast<uintptr_t>(rectangle))&255)!=1 ||
       reinterpret_cast<EliteCollision3>(image+0x4D25A80)(map,1,0)!=2 ||
       (reinterpret_cast<EliteCollision3>(image+0x4D27990)(map,a,1)&255)!=1 ||
       reinterpret_cast<EliteCollision2>(image+0x5DF2660)(reinterpret_cast<uintptr_t>(entryScene),a)!=0)RaiseException(0xE1234567,0,0,nullptr);
    SetLastError(ERROR_INVALID_DATA);return UINT64_C(0xfedcba9876543200);
}
static uintptr_t __fastcall CollisionThrow(uintptr_t,int,int,uintptr_t) {RaiseException(0xE1234567,0,0,nullptr);return 0;}
static bool CollisionExceptionForwarded(ElitePermission4 call,uintptr_t actor) {
    bool caught=false;__try {call(actor,900,100,123);} __except(GetExceptionCode()==0xE1234567?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {caught=true;}
    return caught;
}
static constexpr unsigned kCollisionFixtureArity[]={3,2,2,2,1,1,1,1,1,3,3,2,3,0,0,0,0,3,1,1,1,1};
static uintptr_t collisionForwardedArguments[3];static unsigned collisionForwardedCount;static DWORD collisionForwardedError;static bool collisionForwardedThrow;
static uintptr_t CollisionForwardedResult(uintptr_t a,uintptr_t b,uintptr_t c) {
    collisionForwardedArguments[0]=a;collisionForwardedArguments[1]=b;collisionForwardedArguments[2]=c;
    ++collisionForwardedCount;collisionForwardedError=GetLastError();SetLastError(ERROR_INVALID_DATA);
    if(collisionForwardedThrow)RaiseException(0xE1234567,0,0,nullptr);
    return UINT64_C(0xfedcba9876543200);
}
static uintptr_t __fastcall CollisionForwarded0() {return CollisionForwardedResult(0,0,0);}
static uintptr_t __fastcall CollisionForwarded1(uintptr_t a) {return CollisionForwardedResult(a,0,0);}
static uintptr_t __fastcall CollisionForwarded2(uintptr_t a,uintptr_t b) {return CollisionForwardedResult(a,b,0);}
static uintptr_t __fastcall CollisionForwarded3(uintptr_t a,uintptr_t b,uintptr_t c) {return CollisionForwardedResult(a,b,c);}
static bool CollisionThirdException(EliteCollision3 call) {
    bool caught=false;__try {call(11,22,UINT64_C(0x123456789abcde));}
    __except(GetExceptionCode()==0xE1234567?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {caught=true;}
    return caught;
}
static int CollisionForwardingMatrix(unsigned char* image,ElitePermissionContext* permission) {
    void* saved[22]={};memcpy(saved,g_collisionOriginals,sizeof(saved));
    void* fixtures[]={reinterpret_cast<void*>(CollisionForwarded0),reinterpret_cast<void*>(CollisionForwarded1),reinterpret_cast<void*>(CollisionForwarded2),reinterpret_cast<void*>(CollisionForwarded3)};
    const uintptr_t a=11,b=22,c=UINT64_C(0x123456789abcde);
    EliteCollisionContext context={};context.permission=permission;
    for(unsigned scoped=0;scoped<2;++scoped) {
        g_collisionContext=scoped?&context:nullptr;
        for(size_t i=0;i<ARRAYSIZE(kCollisionFixtureArity);++i) {
            const unsigned arity=kCollisionFixtureArity[i];g_collisionOriginals[i]=fixtures[arity];
            auto native=image+kEliteCollisionGuards[i].rva;const auto before=collisionForwardedCount;
            SetLastError(ERROR_ACCESS_DENIED);uintptr_t result=0;
            if(arity==0)result=reinterpret_cast<uintptr_t (__fastcall *)()>(native)();
            if(arity==1)result=reinterpret_cast<EliteCollision1>(native)(a);
            if(arity==2)result=reinterpret_cast<EliteCollision2>(native)(a,b);
            if(arity==3)result=reinterpret_cast<EliteCollision3>(native)(a,b,c);
            if(result!=UINT64_C(0xfedcba9876543200) || GetLastError()!=ERROR_INVALID_DATA || collisionForwardedCount!=before+1 || collisionForwardedError!=ERROR_ACCESS_DENIED ||
               collisionForwardedArguments[0]!=(arity?a:0) || collisionForwardedArguments[1]!=(arity>=2?b:0) || collisionForwardedArguments[2]!=(arity>=3?c:0) ||
               g_collisionContext!=(scoped?&context:nullptr) || g_collisionFault || g_collisionRoots || g_collisionLeaves!=(scoped?i+1:0)) {
                printf("collision forwarding matrix mismatch: index=%zu phase=%s scoped=%u arity=%u third=%llx expected=%llx\n",i,kEliteCollisionPhases[i],scoped,arity,static_cast<unsigned long long>(collisionForwardedArguments[2]),static_cast<unsigned long long>(arity>=3?c:0));
                g_collisionContext=nullptr;memcpy(g_collisionOriginals,saved,sizeof(saved));return 1;
            }
        }
        auto viewport=reinterpret_cast<EliteCollision3>(image+kEliteCollisionGuards[12].rva);
        const auto count=collisionForwardedCount,leaves=g_collisionLeaves;
        SetLastError(ERROR_ACCESS_DENIED);auto result=viewport(a,b,0);
        if(result!=UINT64_C(0xfedcba9876543200) || collisionForwardedArguments[2] || collisionForwardedCount!=count+1 || GetLastError()!=ERROR_INVALID_DATA || g_collisionLeaves!=leaves+scoped)return 2;
        collisionForwardedThrow=true;SetLastError(ERROR_ACCESS_DENIED);const bool caught=CollisionThirdException(viewport);collisionForwardedThrow=false;
        if(!caught || collisionForwardedCount!=count+2 || collisionForwardedArguments[2]!=c || collisionForwardedError!=ERROR_ACCESS_DENIED || GetLastError()!=ERROR_INVALID_DATA ||
           g_collisionLeaves!=leaves+scoped || g_collisionContext!=(scoped?&context:nullptr))return 3;
        // Each scope starts at zero; exception forwarding must not log a result.
        if(!scoped)g_collisionLeaves=0;
    }
    g_collisionContext=nullptr;g_collisionLeaves=0;memcpy(g_collisionOriginals,saved,sizeof(saved));
    puts("collision ABI matrix: all 22 native signatures, scoped and unrelated paths; exact arguments including null/64-bit third pointer, full RAX, entry/native last-error and exactly one original call; native exceptions forwarded without fake result passed");
    return 0;
}
static int CollisionFixture(unsigned char* image) {
    for(const auto& g:kEliteCollisionGuards) {
        image[g.rva]^=1;bool accepted=EliteStartCollisionTrace(image,L"unused-collision.jsonl");image[g.rva]^=1;
        if(accepted || g_collisionReady)return 1;
    }
    if(!EliteStartCollisionTrace(image,L"unused-collision.jsonl")) {printf("collision start: %s\n",g_collisionFailure);return 2;}
    g_collisionOriginals[18]=reinterpret_cast<void*>(CollisionX);g_collisionOriginals[19]=reinterpret_cast<void*>(CollisionY);
    FixturePtr(ownedVtable,456,reinterpret_cast<uintptr_t>(CollisionCoordinate));
    collisionTestImage=reinterpret_cast<uintptr_t>(image);const uintptr_t a=reinterpret_cast<uintptr_t>(ownedActor);
    FixturePtr(ownedVtable,2856,reinterpret_cast<uintptr_t>(CollisionZero));FixturePtr(ownedVtable,2864,reinterpret_cast<uintptr_t>(CollisionZero));FixtureInt(ownedActor,348,0);
    FixtureInt(collisionMap,16472,1300);FixturePtr(collisionMap,23080,0);
    int tiles[2]={0,2};uintptr_t tileRow[3]={reinterpret_cast<uintptr_t>(tiles),reinterpret_cast<uintptr_t>(tiles+2),reinterpret_cast<uintptr_t>(tiles+2)};
    FixturePtr(collisionMap,15152,reinterpret_cast<uintptr_t>(tileRow));FixturePtr(collisionMap,15160,reinterpret_cast<uintptr_t>(tileRow+3));
    auto originalCollision=g_permissionCollision;auto originalGet=g_catchupOriginal;const auto queries=g_catchupPathQueries;
    ElitePermissionContext permission={};permission.position.slot=0;permission.position.actor=a;permission.position.component=reinterpret_cast<uintptr_t>(catchupPosition);permission.position.scene=reinterpret_cast<uintptr_t>(entryScene);permission.ordinal=777;permission.caller=0x5C77106;
    g_catchupOriginal=PositionGet;if(CollisionForwardingMatrix(image,&permission))return 7;
    g_permissionContext=&permission;g_catchupOriginal=PositionGet;g_permissionCollision=CollisionAbi;
    auto call=reinterpret_cast<ElitePermission4>(image+0x5D749A0);
    SetLastError(ERROR_ACCESS_DENIED);auto result=call(a,900,100,123);
    if(result!=UINT64_C(0xfedcba9876543200) || GetLastError()!=ERROR_INVALID_DATA || collisionNativeCalls!=1 || g_collisionLeaves!=9 || g_collisionRoots!=1 || g_collisionContext || g_collisionFault || g_permissionContext!=&permission || g_catchupPathQueries!=queries)return 3;
    g_permissionContext=nullptr;SetLastError(ERROR_ACCESS_DENIED);call(a,900,100,123);
    if(g_collisionLeaves!=9 || g_collisionRoots!=1 || collisionNativeCalls!=2 || GetLastError()!=ERROR_INVALID_DATA)return 4;
    g_permissionContext=&permission;g_collisionLeaves=15360;SetLastError(ERROR_ACCESS_DENIED);call(a,900,100,123);
    if(g_collisionLeaves!=15360 || g_collisionRoots!=2 || collisionNativeCalls!=3 || GetLastError()!=ERROR_INVALID_DATA)return 5;
    g_permissionCollision=CollisionThrow;
    if(!CollisionExceptionForwarded(call,a) || g_collisionContext || g_permissionContext!=&permission || g_collisionFault)return 6;
    g_permissionContext=nullptr;g_permissionCollision=originalCollision;g_catchupOriginal=originalGet;
    puts("collision observation: 22 current EXE guards, integer coordinate forwarding / native bbox getters / width/rectangle/tile/map/object leaves, full RAX/entry and native errors, no-root/budget transparency, native exception TLS restoration, no extra collision or A* passed");
    return 0;
}

static bool FirstChanceSynthetic(DWORD code) {
    bool caught=false;ULONG_PTR information[]={0,UINT64_C(0x123456789abcde)};
    __try {RaiseException(code,0,2,information);}
    __except(GetExceptionCode()==code?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {caught=true;}
    return caught;
}
static int FirstChanceFixture(unsigned char* image) {
    if(EliteStartFirstChanceTrace(nullptr,1,L"unused-exception.jsonl") || EliteStartFirstChanceTrace(image,0,L"unused-exception.jsonl"))return 1;
    if(!EliteStartFirstChanceTrace(image,reinterpret_cast<uintptr_t>(&FirstChanceFixture),L"unused-exception.jsonl"))return 2;
    SetLastError(ERROR_ACCESS_DENIED);
    if(!FirstChanceSynthetic(EXCEPTION_ACCESS_VIOLATION) || GetLastError()!=ERROR_ACCESS_DENIED || g_firstChanceRecords!=1 || g_firstChanceBusy)return 3;
    HANDLE file=CreateFileW(g_firstChancePath,GENERIC_READ,FILE_SHARE_READ|FILE_SHARE_WRITE,nullptr,OPEN_EXISTING,FILE_ATTRIBUTE_NORMAL,nullptr);
    if(file==INVALID_HANDLE_VALUE)return 4;
    char row[3201]={};DWORD bytes=0;bool read=ReadFile(file,row,3200,&bytes,nullptr)!=FALSE;CloseHandle(file);
    if(!read || bytes<100 || !strstr(row,"\"firstChanceOnly\":true") || !strstr(row,"\"continueSearch\":true") || !strstr(row,"\"faultAddress\":5124095576030430") || !strstr(row,"\"registers\":["))return 5;
    g_firstChanceRecords=64;
    if(!FirstChanceSynthetic(EXCEPTION_ACCESS_VIOLATION) || g_firstChanceRecords!=64)return 6;
    g_firstChanceRecords=1;g_firstChanceBusy=1;
    if(!FirstChanceSynthetic(EXCEPTION_ACCESS_VIOLATION) || g_firstChanceRecords!=1 || g_firstChanceBusy!=1)return 7;
    g_firstChanceBusy=0;
    if(!FirstChanceSynthetic(0xE1234567) || g_firstChanceRecords!=1)return 8;
    if(!RemoveVectoredExceptionHandler(g_firstChanceHandler))return 9;g_firstChanceHandler=nullptr;
    puts("first-chance observation: exact fault/register/stack schema, continue-search and last-error preservation, 64-record/recursion limits, unrelated exception forwarding and handler removal passed; no client launched");
    return 0;
}

static unsigned char* CopySites(const wchar_t* path) {
    HANDLE file=CreateFileW(path,GENERIC_READ,FILE_SHARE_READ,nullptr,OPEN_EXISTING,FILE_ATTRIBUTE_NORMAL,nullptr);
    if(file==INVALID_HANDLE_VALUE)return nullptr;
    unsigned char header[4096]={};DWORD read=0;
    if(!ReadFile(file,header,sizeof(header),&read,nullptr) || read!=sizeof(header)) {CloseHandle(file);return nullptr;}
    auto dos=reinterpret_cast<IMAGE_DOS_HEADER*>(header);
    if(dos->e_magic!=IMAGE_DOS_SIGNATURE || dos->e_lfanew<0 || static_cast<size_t>(dos->e_lfanew)>sizeof(header)-sizeof(IMAGE_NT_HEADERS64)) {CloseHandle(file);return nullptr;}
    auto nt=reinterpret_cast<IMAGE_NT_HEADERS64*>(header+dos->e_lfanew);
    auto sections=IMAGE_FIRST_SECTION(nt);
    if(nt->Signature!=IMAGE_NT_SIGNATURE || nt->FileHeader.Machine!=IMAGE_FILE_MACHINE_AMD64 || nt->OptionalHeader.SizeOfImage<0xEF2CAA0 || nt->OptionalHeader.SizeOfImage>0x30000000 || reinterpret_cast<unsigned char*>(sections+nt->FileHeader.NumberOfSections)>header+sizeof(header)) {CloseHandle(file);return nullptr;}
    auto image=static_cast<unsigned char*>(VirtualAlloc(nullptr,nt->OptionalHeader.SizeOfImage,MEM_RESERVE|MEM_COMMIT,PAGE_READWRITE));
    if(!image) {CloseHandle(file);return nullptr;}
    const DWORD sites[]={0x2E5EDA0,0x2E60CF0,0x2E5A59D,0x2E5AAE5,0x44FD014,0x44FD1CA,0x44FDB42,0x44FAC66,0x6D75AF0,0x2E5AEEB,0x5F0BA60,0x5B22F50,0x5F0C980,0x5B2518C,0x5DE4360,0x5B25298,0x5DD0070,0x5C3E8A0,0x5DC86D0,0x5DD01EC,0x5DC9BCD,0x14CC20,0x2E60CE0,0x6D2A1BB,0x5B8C7D0,0x5B8C890,0x5B8C8C0,0x5B960D0,0x59A9260,0x59A90F0,0x6E920A0,0x5CF6C20,0x5C70EE0,0x5E08940,0x6D64820,0x6D64330,0x6D64B70,0x6D642C0,0x6D64E00,0x6D6491B,0x5D714E0,0x5D70740,0x1854970,0x449C130,0x449BF20,0x5B253B7,0x63DE5D0,0x5C70E00,0x5CF6910,0x5B98210,0x5B25312,0x5E0EC50,0x5E0CD40,0x5C6AE80,0x5C0FB80,0x5C70E90,0x6D64A50,0x5C232C0,0x5D749A0,0x48F1840,0x59AC4C0,0x5CF68D0,0x4D27990,0x5DF2660,0x5B32610,0x5B2F130,0x4D24C40,0x4D23F20,0x4D22B70,0x4D26B50,0x4D25BF0,0x4D25A80,0x7211920,0x4D28250,0xEEB380,0x5494680,0x4D26D70,0x4D26DE0,0xE0CEC0,0x5D5EFA0,0x5B8C7E0,0x5B8C8B0};
    bool ok=true;
    for(DWORD rva:sites) {
        DWORD byteCount=rva==0x4D27990?4096:rva==0x5DF2660?512:(rva==0x5C232C0 || rva==0x5D749A0 || rva==0x5CF6910)?1024:rva==0x5E0EC50?320:(rva==0x6D64820 || rva==0x6D64330 || rva==0x6D64B70 || rva==0x6D64E00)?4096:rva==0x63DE5D0?512:rva==0x59A90F0?256:128;
        for(const auto& guard:kEliteCollisionGuards)if(guard.rva==rva && byteCount<512)byteCount=512;
        bool found=false;
        for(WORD i=0;i<nt->FileHeader.NumberOfSections;++i) {
            auto& section=sections[i];
            if(rva>=section.VirtualAddress && static_cast<ULONGLONG>(rva)+byteCount<=static_cast<ULONGLONG>(section.VirtualAddress)+section.SizeOfRawData) {
                LARGE_INTEGER offset={};offset.QuadPart=static_cast<LONGLONG>(section.PointerToRawData)+rva-section.VirtualAddress;
                found=SetFilePointerEx(file,offset,nullptr,FILE_BEGIN) && ReadFile(file,image+rva,byteCount,&read,nullptr) && read==byteCount;
                break;
            }
        }
        if(!found) {ok=false;break;}
    }
    memcpy(permissionSavedGate,image+0x5C70E00,128);memcpy(permissionSavedEffects,image+0x5CF6910,512);
    DWORD old=0;
    if(ok)ok=VirtualProtect(image,nt->OptionalHeader.SizeOfImage,PAGE_EXECUTE_READWRITE,&old)!=0;
    CloseHandle(file);
    if(!ok) {VirtualFree(image,0,MEM_RELEASE);return nullptr;}
    return image;
}
int wmain(int argc,wchar_t** argv) {
    setvbuf(stdout,nullptr,_IONBF,0);
    SetUnhandledExceptionFilter(StartupFixtureException);
    if(argc!=2)return 1;
    auto nonClient=reinterpret_cast<unsigned char*>(GetModuleHandleW(nullptr));
    if(EliteStartNativeTrace(nonClient,L"unused") || EliteStartLifecycleTrace(nonClient,L"unused",0))return 16;
    auto image=CopySites(argv[1]);if(!image)return 2;
    unsigned char originalCondition[7]={},sharedSite[16]={};memcpy(originalCondition,image+0x5B251A9,7);
    if(EliteStartCombatTrace(image,L"unused-combat") || g_combatReady)return 17;
    if(EliteStartRegistration(image) || memcmp(originalCondition,image+0x5B251A9,7))return 3;
    if(!EliteStartPreparationAdapter(image)) {printf("prepare: %s\n",g_prepareFailure);return 4;}
    memcpy(sharedSite,image+0x2E60CF0,sizeof(sharedSite));
    if(EliteStartRegistration(image) || memcmp(sharedSite,image+0x2E60CF0,sizeof(sharedSite)))return 5;
    if(!EliteStartEntryTrace(image,L"unused-startup-fixture")) {printf("entry: %s\n",g_entryFailure);return 6;}
    image[0x5B251A9]=0xCC;
    if(EliteStartRegistration(image) || g_preparePredicateRegistration || memcmp(sharedSite,image+0x2E60CF0,sizeof(sharedSite)))return 7;
    image[0x5B251A9]=originalCondition[0];
    image[0x2E60CE0]^=1;
    if(EliteStartRegistration(image) || memcmp(originalCondition,image+0x5B251A9,7))return 26;
    image[0x2E60CE0]^=1;image[0x6D2A1BB]^=1;
    if(EliteStartRegistration(image) || memcmp(originalCondition,image+0x5B251A9,7))return 27;
    image[0x6D2A1BB]^=1;
    if(!EliteStartRegistration(image)) {printf("registration: %s\n",g_registrationFailure);return 8;}
    if(!g_registrationReady || g_preparePredicateRegistration!=EliteRegistrationChannel || image[0x5B251A9]!=0xE8 || memcmp(sharedSite,image+0x2E60CF0,sizeof(sharedSite)))return 9;
    image[0x5DD01EC]^=1;
    if(EliteStartCombatTrace(image,L"unused-combat") || g_combatReady)return 18;
    image[0x5DD01EC]^=1;
    if(!EliteStartCombatTrace(image,L"unused-combat")) {printf("combat: %s\n",g_combatFailure);return 19;}
    for(auto& guard:kElitePathGuards) {
        image[guard.rva]^=1;
        if(EliteStartCatchup(image,L"unused-catchup",true) || g_catchupReady)return 34;
        image[guard.rva]^=1;
    }
    for(auto& guard:kEliteCatchupGuards) {
        image[guard.rva]^=1;
        if(EliteStartCatchup(image,L"unused-catchup") || g_catchupReady)return 31;
        image[guard.rva]^=1;
    }
    // Disabled path mode neither requires nor hooks the native allocator/search.
    unsigned char allocatorBefore[32]={};memcpy(allocatorBefore,image+0x6D642C0,sizeof(allocatorBefore));
    image[0x6D64820]^=1;
    if(!EliteStartCatchup(image,L"unused-catchup") || g_catchupPathEnabled || g_nativePathAllocate ||
        memcmp(allocatorBefore,image+0x6D642C0,sizeof(allocatorBefore)))return 36;
    image[0x6D64820]^=1;
    if(MH_DisableHook(image+0x5E08940)!=MH_OK || MH_RemoveHook(image+0x5E08940)!=MH_OK)return 37;
    g_catchupReady=false;g_catchupOriginal=nullptr;
    if(!EliteStartCatchup(image,L"unused-catchup",true)) {printf("catchup: %s\n",g_catchupFailure);return 32;}
    int path=NativePathFixture(image);if(path) {printf("native path fixture: %d\n",path);return 35;}
    if(memcmp(sharedSite,image+0x2E60CF0,sizeof(sharedSite)))return 20;
    auto assignTrampoline=g_combatAssignOriginal;auto deathTrampoline=g_combatDeathOriginal;
    auto nativeAssign=reinterpret_cast<EliteAttackAssign>(image+0x5DD0070);
    auto nativeDeath=reinterpret_cast<EliteDeathSend>(image+0x5DC86D0);
    auto nativeSet=reinterpret_cast<EliteKillerSet>(image+0x5C3E8A0);
    g_combatAssignOriginal=CombatAssignStub;nativeAssign(11,22);
    if(combatCalls!=1 || GetLastError()!=ERROR_ACCESS_DENIED)return 21;
    g_combatDeathOriginal=CombatDeathStub;
    if(nativeDeath(11,-3,241)!=UINT64_C(0xfedcba9876543210) || combatCalls!=2 || GetLastError()!=ERROR_ACCESS_DENIED)return 22;
    // Execute only the copied seven-byte native DWORD setter in this process.
    SetLastError(ERROR_ACCESS_DENIED);nativeSet(reinterpret_cast<uintptr_t>(combatVictim),65535);
    int stored=0;memcpy(&stored,combatVictim+26784,4);
    if(stored!=65535 || GetLastError()!=ERROR_ACCESS_DENIED)return 23;
    if(!OwnedSetterFixture(image))return 25;
    int reentry=ReentryRegistrationFixture(image);if(reentry) {printf("reentry fixture: %d\n",reentry);return 30;}
    int catchup=CatchupFixture(image);if(catchup) {printf("catchup fixture: %d\n",catchup);return 33;}
    int birth=BirthFixture(image);if(birth) {printf("birth fixture: %d\n",birth);return 38;}
    int position=PositionFixture(image);if(position) {printf("position fixture: %d\n",position);return 39;}
    int permission=PermissionFixture(image);if(permission) {printf("permission fixture: %d\n",permission);return 40;}
    int collision=CollisionFixture(image);if(collision) {printf("collision fixture: %d\n",collision);return 41;}
    int exception=FirstChanceFixture(image);if(exception) {printf("first-chance fixture: %d\n",exception);return 42;}
    g_combatContext={33,44,55,true};auto context=g_combatContext;
    g_combatAssignOriginal=CombatThrowStub;
    if(!CombatExceptionForwarded(nativeAssign) || memcmp(&context,&g_combatContext,sizeof(context)))return 24;
    g_combatAssignOriginal=assignTrampoline;g_combatDeathOriginal=deathTrampoline;
    EliteNativePredicate extra=nullptr;
    if(MH_CreateHook(image+0x2E60CF0,reinterpret_cast<void*>(NativeStub),reinterpret_cast<void**>(&extra))!=MH_ERROR_ALREADY_CREATED)return 10;
    auto shared=reinterpret_cast<EliteNativePredicate>(image+0x2E60CF0);
    auto trampoline=g_originalPredicate;g_originalPredicate=NativeStub;
    if(shared()!=37 || originalCalls!=1 || GetLastError()!=ERROR_ACCESS_DENIED)return 11;
    g_originalPredicate=ThrowStub;if(!ExceptionForwarded(shared))return 12;
    g_originalPredicate=trampoline;
    unsigned char patchedCondition[7]={};memcpy(patchedCondition,image+0x5B251A9,7);
    if(!EliteConditionPatch(image+0x5B251A9,patchedCondition,originalCondition))return 13;
    g_preparePredicateRegistration=nullptr;g_registrationReady=false;
    if(MH_DisableHook(MH_ALL_HOOKS)!=MH_OK || MH_Uninitialize()!=MH_OK)return 14;
    if(memcmp(originalCondition,image+0x5B251A9,7))return 15;
    VirtualFree(image,0,MEM_RELEASE);
    puts("registration startup: production preparation -> entry -> registration -> combat sequence, shared hook unchanged, dependency/mismatch refusal, original result/error/SEH forwarding, exact-callsite owned setter after exhausted budget, unrelated FFFF preservation and restoration; scoped fresh reentry getter, in-room/foreign/identity/duplicate/native-reference rejection, physical flag and CRef preservation passed; no client launched");
    puts("catchup startup: actual EXE byte guards, component getter X/Y/Z ABI/error/SEH, sustained native no-route teleport, reachable/unknown refusal, APC getter interleaving, exhausted log budget, town/dead/missing/duplicate scene/foreign owner/invalid binding/setter refusal passed");
    puts("catchup scene: actual EXE world getter town=1/dungeon=3, distinct loader/global-world anchor, absent/replaced world refusal, missing for 3s then exact native eight-argument registration, one-time restore, foreign scene refusal passed");
    puts("catchup retention: actual EXE virtual320 thunk/leaf and virtual328 getter, only actor720 changes, inactive APC restored then enabled, enabled APC preserved, unsupported retention slots refused; live render/AI pending");
    puts("catchup path isolation: default startup leaves allocator unhooked and ignores optional search guards; registered APC bytes preserved with zero path calls; missing-registration recovery/activation retained without path calls; live AI pending");
    puts("catchup derived virtual632: actual EXE special state allowance/denial, common fallback and arguments, unknown-slot refusal, missing registration recovery and post-activation native denial passed; complex dependencies use ABI stubs");
    return 0;
}
