#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdio.h>
#include "entry-trace.h"
#include "registration-adapter.h"
#include "combat-trace.h"
#include "catchup-adapter.h"
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
static ElitePathResult CatchupQuery(uintptr_t,int,int,int,int) {return catchupPath;}
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
    if(retained(reinterpret_cast<uintptr_t>(ownedActor))!=1)return 26;
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
    g_entryRegisterOriginal=registerOriginal;
    g_lifecycleArmed=false;SetLastError(ERROR_ACCESS_DENIED);int calls=catchupCalls;
    if(native(player,0)!=INT64_C(0x1234567800000064) || GetLastError()!=ERROR_ACCESS_DENIED || catchupCalls!=calls+1)return 12;
    if(native(player,1)!=INT64_C(0x1234567800000065) || native(player,2)!=INT64_C(0x1234567800000066) || GetLastError()!=ERROR_ACCESS_DENIED || catchupCalls!=calls+3)return 13;
    g_catchupOriginal=original;EliteCatchupReset();return 0;
}
static char __fastcall ThrowStub() {RaiseException(0xE1234567,0,0,nullptr);return 0;}
static bool ExceptionForwarded(EliteNativePredicate call) {
    bool caught=false;
    __try {call();} __except(GetExceptionCode()==0xE1234567?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {caught=true;}
    return caught;
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
    const DWORD sites[]={0x2E5EDA0,0x2E60CF0,0x2E5A59D,0x2E5AAE5,0x44FD014,0x44FD1CA,0x44FDB42,0x44FAC66,0x6D75AF0,0x2E5AEEB,0x5F0BA60,0x5B22F50,0x5F0C980,0x5B2518C,0x5DE4360,0x5B25298,0x5DD0070,0x5C3E8A0,0x5DC86D0,0x5DD01EC,0x5DC9BCD,0x14CC20,0x2E60CE0,0x6D2A1BB,0x5B8C7D0,0x5B8C890,0x5B8C8C0,0x5B960D0,0x59A9260,0x59A90F0,0x6E920A0,0x5CF6C20,0x5C70EE0,0x5E08940,0x6D64820,0x6D64330,0x6D64B70,0x6D642C0,0x6D64E00,0x6D6491B,0x5D714E0,0x5D70740,0x1854970,0x449C130,0x449BF20,0x5B253B7};
    bool ok=true;
    for(DWORD rva:sites) {
        DWORD byteCount=(rva==0x6D64820 || rva==0x6D64330 || rva==0x6D64B70 || rva==0x6D64E00)?4096:rva==0x59A90F0?256:128;
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
    DWORD old=0;
    if(ok)ok=VirtualProtect(image,nt->OptionalHeader.SizeOfImage,PAGE_EXECUTE_READWRITE,&old)!=0;
    CloseHandle(file);
    if(!ok) {VirtualFree(image,0,MEM_RELEASE);return nullptr;}
    return image;
}
int wmain(int argc,wchar_t** argv) {
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
        if(EliteStartCatchup(image,L"unused-catchup") || g_catchupReady)return 34;
        image[guard.rva]^=1;
    }
    for(auto& guard:kEliteCatchupGuards) {
        image[guard.rva]^=1;
        if(EliteStartCatchup(image,L"unused-catchup") || g_catchupReady)return 31;
        image[guard.rva]^=1;
    }
    if(!EliteStartCatchup(image,L"unused-catchup")) {printf("catchup: %s\n",g_catchupFailure);return 32;}
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
    return 0;
}
