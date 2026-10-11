#pragma once
#include "entry-trace.h"
#include "registration-adapter.h"
#include "catchup-policy.h"
#include "native-path.h"

using EliteCatchupGet=int (__fastcall *)(uintptr_t);
using EliteCatchupScene=uintptr_t (__fastcall *)(uintptr_t);
using EliteCatchupSet=uintptr_t (__fastcall *)(uintptr_t,int,int,int);
using EliteCatchupLand=unsigned char (__fastcall *)(uintptr_t,int,int,uintptr_t);
using EliteCatchupPositionGet=int64_t (__fastcall *)(uintptr_t,int);
using EliteCatchupEnable=void (__fastcall *)(uintptr_t,unsigned char);
static EliteCatchupPositionGet g_catchupOriginal;
static EliteCatchupGet g_catchupX,g_catchupY,g_catchupZ,g_catchupMode;
static ElitePathResult (*g_catchupQuery)(uintptr_t,int,int,int,int)=EliteNativePathToPlayer;
static EliteCatchupScene g_catchupScene;
static EliteCatchupSet g_catchupSet;
static EliteCatchupLand g_catchupLand;
static bool g_catchupReady,g_catchupFault;
static const char* g_catchupFailure="not-started";
static wchar_t g_catchupPath[32768];
static LONG g_catchupRecords;
static SRWLOCK g_catchupRecordLock=SRWLOCK_INIT;
static volatile LONG g_catchupHits;
static uint64_t g_catchupPoll,g_catchupLog;
static uint64_t g_catchupRejectedLog;
static EliteCatchupState g_catchupStates[3];
static EliteCatchupState g_catchupMissing[3];
static EliteCatchupKey g_catchupAnchors[3];
static int g_catchupWorldMode=-1;
// 1459A90F0 / 146E920A0 and PID7996: town=1, dungeon=3.
static constexpr int kEliteCatchupDungeonMode=3;
static __declspec(thread) bool g_catchupBusy;
struct EliteCatchupGuard {uintptr_t rva;const unsigned char* bytes;size_t size;};
static constexpr unsigned char kEliteCatchupBytes0[]={0x48,0x8b,0x89,0x70,0x1,0x0,0x0,0x33,0xd2,0xe9,0x62,0xc1,0x27,0x0};
static constexpr unsigned char kEliteCatchupBytes1[]={0x48,0x8b,0x89,0x70,0x1,0x0,0x0,0xba,0x1,0x0,0x0,0x0,0xe9,0x9f,0xc0,0x27,0x0};
static constexpr unsigned char kEliteCatchupBytes2[]={0x48,0x8b,0x89,0x70,0x1,0x0,0x0,0xba,0x2,0x0,0x0,0x0,0xe9,0x6f,0xc0,0x27,0x0};
static constexpr unsigned char kEliteCatchupBytes3[]={0x40,0x57,0x41,0x56,0x41,0x57,0x48,0x83,0xec,0x30,0x48,0xc7,0x44,0x24,0x20,0xfe,0xff,0xff,0xff,0x48,0x89,0x5c,0x24,0x58,0x48,0x89,0x6c,0x24,0x60,0x48,0x89,0x74};
static constexpr unsigned char kEliteCatchupBytes4[]={0x48,0x83,0xec,0x28,0xe8,0x17,0xff,0xff,0xff,0x48,0x85,0xc0,0x74,0xc,0x48,0x8b,0xc8,0x48,0x83,0xc4,0x28,0xe9,0x36,0x9e,0x4e,0x0,0x48,0x83,0xc4,0x28,0xc3,0xcc};
static constexpr unsigned char kEliteCatchupBytes5[]={0x40,0x53,0x48,0x83,0xec,0x20,0x48,0x8b,0xd9,0x48,0x8b,0xd,0xf0,0xa8,0xcd,0x8,0x48,0x85,0xc9,0x74,0x1e,0x48,0x8b,0x1,0xff,0x50,0x60,0x83,0xf8,0xff,0x74,0x13};
static constexpr unsigned char kEliteCatchupBytes6[]={0xf,0xb6,0x81,0x2,0x15,0x1,0x0,0xc3};
static constexpr unsigned char kEliteCatchupBytes7[]={0x48,0x89,0x5c,0x24,0x8,0x48,0x89,0x6c,0x24,0x10,0x48,0x89,0x74,0x24,0x18,0x57,0x48,0x83,0xec,0x20,0x49,0x8b,0xe9,0x41,0x8b,0xd8,0x8b,0xfa,0x48,0x8b,0xf1,0xe8};
static constexpr unsigned char kEliteCatchupDerivedLanding[]={
    0x48,0x89,0x6c,0x24,0x18,0x48,0x89,0x74,0x24,0x20,0x57,0x41,
    0x56,0x41,0x57,0x48,0x83,0xec,0x20,0x48,0x8b,0x01,0x49,0x8b,
    0xe9,0x45,0x8b,0xf0,0x44,0x8b,0xfa,0x48,0x8b,0xf9,0x40,0xb6,
    0x01,0xff,0x90,0x48,0x01,0x00,0x00,0x84,0xc0,0x0f,0x84,0xe3,
    0x00,0x00,0x00,0x48,0x83,0xbf,0x48,0x6f,0x00,0x00,0x00,0x4c,
    0x89,0x64,0x24,0x48,0x0f,0x96,0xc0,0x45,0x33,0xe4,0x41,0x8b,
    0xcc,0x0f,0xb6,0xc0,0x48,0x39,0x8f,0x58,0x6f,0x00,0x00,0x0f,
    0x46,0xc8,0x0f,0xb6,0xf1,0x4c,0x39,0xa7,0x68,0x6f,0x00,0x00,
    0x0f,0x86,0xab,0x00,0x00,0x00,0x48,0x8b,0x87,0x60,0x6f,0x00,
    0x00,0x48,0x89,0x5c,0x24,0x40,0x48,0x8b,0x18,0x48,0x3b,0xd8,
    0x0f,0x84,0x8e,0x00,0x00,0x00,0x66,0x90,0x48,0x8b,0x4b,0x20,
    0xe8,0x07,0x90,0xb5,0xff,0x48,0x3b,0xc7,0x75,0x20,0x48,0x8b,
    0x4b,0x20,0x4c,0x8b,0xcd,0x45,0x8b,0xc6,0x41,0x8b,0xd7,0x48,
    0x8b,0x01,0xff,0x90,0xd8,0x01,0x00,0x00,0x84,0xc0,0x40,0x0f,
    0xb6,0xf6,0x41,0x0f,0x44,0xf4,0x48,0x8b,0x43,0x10,0x44,0x38,
    0x60,0x19,0x74,0x22,0x48,0x8b,0x43,0x08,0x44,0x38,0x60,0x19,
    0x75,0x13,0x48,0x3b,0x58,0x10,0x75,0x0d,0x48,0x8b,0xd8,0x48,
    0x8b,0x40,0x08,0x44,0x38,0x60,0x19,0x74,0xed,0x48,0x8b,0xd8,
    0xeb,0x25,0x48,0x8b,0x08,0x48,0x8b,0xd8,0x44,0x38,0x61,0x19,
    0x75,0x19,0x66,0x66,0x0f,0x1f,0x84,0x00,0x00,0x00,0x00,0x00,
    0x48,0x8b,0x01,0x48,0x8b,0xd9,0x48,0x8b,0xc8,0x44,0x38,0x60,
    0x19,0x74,0xf1,0x48,0x3b,0x9f,0x60,0x6f,0x00,0x00,0x0f,0x85,
    0x74,0xff,0xff,0xff,0x48,0x8b,0x5c,0x24,0x40,0x4c,0x8b,0x64,
    0x24,0x48,0x48,0x8b,0x07,0xba,0x10,0x00,0x00,0x00,0x48,0x8b,
    0xcf,0xff,0x90,0x38,0x15,0x00,0x00,0x84,0xc0,0x74,0x19,0x40,
    0x84,0xf6,0x74,0x14,0x48,0x8b,0x07,0x48,0x8b,0xcf,0xff,0x90,
    0x18,0x02,0x00,0x00,0x83,0xf8,0x05,0x0f,0x94,0xc0,0xeb,0x11,
    0x4c,0x8b,0xcd,0x45,0x8b,0xc6,0x41,0x8b,0xd7,0x48,0x8b,0xcf,
    0xe8,0xbb,0x27,0x89,0xff,0x48,0x8b,0x6c,0x24,0x50,0x48,0x8b,
    0x74,0x24,0x58,0x48,0x83,0xc4,0x20,0x41,0x5f,0x41,0x5e,0x5f,
    0xc3
};
static constexpr unsigned char kEliteCatchupEnableThunk[]={0xe9,0xbb,0x77,0xc4,0x02};
static constexpr unsigned char kEliteCatchupEnableLeaf[]={0x88,0x91,0xd0,0x02,0x00,0x00,0xc3};
static constexpr unsigned char kEliteCatchupEnabledLeaf[]={0x0f,0xb6,0x81,0xd0,0x02,0x00,0x00,0xc3};
static constexpr unsigned char kEliteCatchupEnableCall[]={0xb2,0x01,0x48,0x8b,0xce,0xff,0x90,0x40,0x01,0x00,0x00};
static constexpr EliteCatchupGuard kEliteCatchupGuards[]={
    {0x5b8c7d0,kEliteCatchupBytes0,sizeof(kEliteCatchupBytes0)},
    {0x5b8c890,kEliteCatchupBytes1,sizeof(kEliteCatchupBytes1)},
    {0x5b8c8c0,kEliteCatchupBytes2,sizeof(kEliteCatchupBytes2)},
    {0x5b960d0,kEliteCatchupBytes3,sizeof(kEliteCatchupBytes3)},
    {0x59a9260,kEliteCatchupBytes4,sizeof(kEliteCatchupBytes4)},
    {0x59a90f0,kEliteCatchupBytes5,sizeof(kEliteCatchupBytes5)},
    {0x5cf6c20,kEliteCatchupBytes6,sizeof(kEliteCatchupBytes6)},
    {0x5c70ee0,kEliteCatchupBytes7,sizeof(kEliteCatchupBytes7)},
    {0x63de5d0,kEliteCatchupDerivedLanding,sizeof(kEliteCatchupDerivedLanding)},
    {0x1854970,kEliteCatchupEnableThunk,sizeof(kEliteCatchupEnableThunk)},
    {0x449c130,kEliteCatchupEnableLeaf,sizeof(kEliteCatchupEnableLeaf)},
    {0x449bf20,kEliteCatchupEnabledLeaf,sizeof(kEliteCatchupEnabledLeaf)},
    {0x5b253b7,kEliteCatchupEnableCall,sizeof(kEliteCatchupEnableCall)}
};

static void EliteCatchupReset() {for(auto& state:g_catchupStates)state={};for(auto& state:g_catchupMissing)state={};}
static void EliteCatchupRecord(const char* reason,int slot,uintptr_t scene,int px,int py,int ax,int ay,int tx=0,int ty=0,int afterX=0,int afterY=0,uintptr_t world=0,uintptr_t loaderContext=0,int retentionBefore=-1,int retentionAfter=-1) {
    if(!TryAcquireSRWLockExclusive(&g_catchupRecordLock))return;
    if(g_catchupRecords>=1024) {ReleaseSRWLockExclusive(&g_catchupRecordLock);return;}
    ++g_catchupRecords;
    char line[1024]={};
    const bool pathRecord=!strncmp(reason,"path-",5) || !strcmp(reason,"teleport");
    int length=sprintf_s(line,"{\"traceVersion\":2,\"processId\":%lu,\"runTick\":%llu,\"tick\":%llu,\"thread\":%lu,\"reason\":\"%s\",\"slot\":%d,\"scene\":%llu,\"player\":[%d,%d],\"before\":[%d,%d],\"target\":[%d,%d],\"after\":[%d,%d],\"criterion\":\"native-astar-or-registration-loss\",\"delayMs\":3000,\"worldMode\":%d,\"pathDetail\":\"%s\",\"grid\":[%d,%d],\"world\":%llu,\"anchorWorld\":%llu,\"loaderContext\":%llu,\"retentionBefore\":%d,\"retentionAfter\":%d}\r\n",
        GetCurrentProcessId(),g_eliteTraceRunTick,GetTickCount64(),GetCurrentThreadId(),reason,slot,static_cast<unsigned long long>(scene),px,py,ax,ay,tx,ty,afterX,afterY,GetCurrentThreadId()==g_lifecycleThread?g_catchupWorldMode:-1,pathRecord?g_nativePathReason:"not-queried",pathRecord?g_nativePathWidth:0,pathRecord?g_nativePathHeight:0,
        static_cast<unsigned long long>(world),static_cast<unsigned long long>(GetCurrentThreadId()==g_lifecycleThread && slot>=0 && slot<3?g_catchupAnchors[slot].world:0),static_cast<unsigned long long>(loaderContext),retentionBefore,retentionAfter);
    HANDLE file=length>0?CreateFileW(g_catchupPath,FILE_APPEND_DATA,FILE_SHARE_READ|FILE_SHARE_WRITE,nullptr,OPEN_ALWAYS,FILE_ATTRIBUTE_NORMAL,nullptr):INVALID_HANDLE_VALUE;
    DWORD written=0;if(file!=INVALID_HANDLE_VALUE) {WriteFile(file,line,static_cast<DWORD>(length),&written,nullptr);CloseHandle(file);}
    ReleaseSRWLockExclusive(&g_catchupRecordLock);
}
static void EliteCatchupRejected(const char* reason,uint64_t now) {
    EliteCatchupReset();
    if(now>=g_catchupRejectedLog && now-g_catchupRejectedLog<2000)return;
    g_catchupRejectedLog=now;EliteCatchupRecord(reason,-1,0,0,0,0,0);
}
// Exact CRef membership, including duplicate rejection. actor+400 may be empty
// even after successful native registration; it is not a registration oracle.
static int EliteCatchupMembership(uintptr_t scene,const uintptr_t* weak) {
    uintptr_t vector[3]={},after[3]={};
    if(!scene || !EliteCopy(reinterpret_cast<void*>(scene+88),vector,sizeof(vector)) ||
        vector[0]>vector[1] || vector[1]>vector[2] || (vector[1]-vector[0])%24 || (vector[1]-vector[0])/24>4096)return -1;
    int count=0;
    for(uintptr_t item=vector[0];item<vector[1];item+=24) {
        uintptr_t ref[3]={};if(!EliteCopy(reinterpret_cast<void*>(item),ref,sizeof(ref)))return -1;
        if(ref[1]==weak[0] && ref[2]==weak[1])++count;
    }
    if(!EliteCopy(reinterpret_cast<void*>(scene+88),after,sizeof(after)) || memcmp(vector,after,sizeof(vector)))return -1;
    return count;
}
static EliteCatchupLand EliteCatchupLandingFunction(uintptr_t actor) {
    uintptr_t vtable=0,landing=0;
    if(!actor || !EliteCopy(reinterpret_cast<void*>(actor),&vtable,8) || !vtable ||
        !EliteCopy(reinterpret_cast<void*>(vtable+632),&landing,8) ||
        (landing!=reinterpret_cast<uintptr_t>(g_prepareImage+0x5C70EE0) &&
         landing!=reinterpret_cast<uintptr_t>(g_prepareImage+0x63DE5D0)))return nullptr;
    return reinterpret_cast<EliteCatchupLand>(landing);
}
static unsigned char __fastcall EliteCatchupLanding(uintptr_t actor,int x,int y,uintptr_t context) {
    // PID23648: slot0 overrides virtual632. Preserve its native state/attachment
    // checks instead of substituting the common implementation or accepting any slot.
    auto native=EliteCatchupLandingFunction(actor);
    return native?native(actor,x,y,context):0;
}
static bool EliteCatchupActor(uintptr_t actor,bool companion) {
    uintptr_t vtable=0,setter=0,deadGetter=0,position=0,enable=0,enabled=0;unsigned char dead=1;
    if(!actor || !EliteCopy(reinterpret_cast<void*>(actor),&vtable,8) || !vtable ||
        !EliteCopy(reinterpret_cast<void*>(vtable+8464),&deadGetter,8) || deadGetter!=reinterpret_cast<uintptr_t>(g_prepareImage+0x5CF6C20) ||
        !EliteCopy(reinterpret_cast<void*>(actor+70914),&dead,1) || dead ||
        !EliteCopy(reinterpret_cast<void*>(actor+368),&position,8) || !position || !EliteReadable(reinterpret_cast<const unsigned char*>(position),640))return false;
    if(companion && (!EliteCopy(reinterpret_cast<void*>(vtable+432),&setter,8) || setter!=reinterpret_cast<uintptr_t>(g_prepareImage+0x5B960D0) ||
        !EliteCopy(reinterpret_cast<void*>(vtable+320),&enable,8) || enable!=reinterpret_cast<uintptr_t>(g_prepareImage+0x1854970) ||
        !EliteCopy(reinterpret_cast<void*>(vtable+328),&enabled,8) || enabled!=reinterpret_cast<uintptr_t>(g_prepareImage+0x449BF20) ||
        !EliteCatchupLandingFunction(actor)))return false;
    return true;
}
static bool EliteCatchupRegistrationFree(uintptr_t scene,uint32_t id) {
    uintptr_t vector[3]={},after[3]={};
    if(!EliteCopy(reinterpret_cast<void*>(scene+88),vector,sizeof(vector)) || vector[0]>vector[1] || vector[1]>vector[2] ||
        (vector[1]-vector[0])%24 || (vector[1]-vector[0])/24>4096)return false;
    for(uintptr_t p=vector[0];p<vector[1];p+=24) {
        uintptr_t ref[3]={};int strong=0,kind=0;uint32_t words[2]={},decoded=0;
        if(!EliteCopy(reinterpret_cast<void*>(p),ref,sizeof(ref)))return false;
        if(!ref[1])continue;
        if(!EliteCopy(reinterpret_cast<void*>(ref[1]+8),&strong,4))return false;
        if(strong<=0)continue;
        if(ref[2]<48 || !EliteCopy(reinterpret_cast<void*>(ref[2]-48+352),&kind,4))return false;
        if(kind!=5)continue;
        if(!EliteCopy(reinterpret_cast<void*>(ref[2]-48+356),words,8) || !EliteSnapshotIdentity(words[0],words[1],&decoded) || decoded==id)return false;
    }
    return EliteCopy(reinterpret_cast<void*>(scene+88),after,sizeof(after)) && !memcmp(vector,after,sizeof(vector));
}
static bool EliteCatchupContext(uintptr_t world,uintptr_t scene,const uintptr_t* player) {
    uintptr_t currentWorld=0,current[2]={},native[2]={};
    return EliteEntryIdentity() && !g_entryWorld && EliteCopy(g_prepareImage+0xE66C090,&currentWorld,8) && world==currentWorld &&
        g_catchupMode(world)==kEliteCatchupDungeonMode && g_catchupScene(world)==scene && EliteCurrentPlayerReference(current) &&
        !memcmp(current,player,16) && EliteNativePlayerReference(native) && !memcmp(native,player,16);
}
// Remember only an actually successful native elite registration. Comparing
// complete weak identities limits recovery to the same live scene generation.
static void EliteCatchupRegistered(uintptr_t loaderContext,uintptr_t scene,uintptr_t actor,unsigned int id) {
    uintptr_t player[2]={},native[2]={},manager=0,world=0,worldAfter=0;
    if(!g_catchupReady || !EliteEntryIdentity() || !EliteCurrentPlayerReference(player) ||
        !EliteNativePlayerReference(native) || memcmp(player,native,16) ||
        !EliteCopy(g_prepareImage+0xE638EF8,&manager,8) || !manager)return;
    // 145B22F50's first argument is a dungeon loading context, not the global
    // world consumed by 1459A90F0/1459A9260. PID8232 proved these differ.
    // Keep the exact registered scene; bind its anchor to the same global
    // world source used by sampling, without dereferencing the loader here.
    if(!loaderContext || !EliteCopy(g_prepareImage+0xE66C090,&world,8) || !world) {
        EliteCatchupRecord("anchor-world-unavailable",-1,scene,0,0,0,0);return;
    }
    auto s=EliteReadCompanions(manager,EliteCopy,player);
    if(!s.available || !s.consistent || !EliteCopy(g_prepareImage+0xE66C090,&worldAfter,8) || worldAfter!=world)return;
    for(size_t i=0;i<3;++i) {
        uintptr_t weak[2]={};
        if(s.strong[i]<=0 || s.kind[i]!=5 || s.identityValid[i]!=1 || s.ownerMatchesCurrentPlayer[i]!=1 ||
            s.controllerBound[i]!=1 || s.objectId[i]!=id || !EliteCopy(reinterpret_cast<void*>(manager+72+24*i),weak,16) ||
            weak[1]<48 || weak[1]-48!=actor || EliteCatchupMembership(scene,weak)!=1)continue;
        g_catchupAnchors[i]={world,scene,player[0],player[1],weak[0],weak[1],id,0};
        g_catchupMissing[i]={};
        EliteCatchupRecord("registration-anchored",static_cast<int>(i),scene,0,0,0,0,0,0,0,0,world,loaderContext);
    }
}
static void EliteCatchupSample(uintptr_t callerPosition,uint64_t now) {
    if(now>=g_catchupPoll && now-g_catchupPoll<500)return;
    uintptr_t player[2]={},native[2]={},world=0,manager=0;
    if(!EliteCurrentPlayerReference(player) || player[1]<48) {EliteCatchupRejected("player-reference-unavailable",now);return;}
    if(!EliteNativePlayerReference(native) || memcmp(player,native,16)) {EliteCatchupRejected("native-player-mismatch",now);return;}
    uintptr_t callerActor=player[1]-48,position=0;
    if(!EliteCopy(reinterpret_cast<void*>(callerActor+368),&position,8) || callerPosition!=position)return;
    g_catchupPoll=now;
    bool log=now<g_catchupLog || now-g_catchupLog>=2000;
    if(log)g_catchupLog=now;
    if(!EliteEntryIdentity() || g_entryWorld) {EliteCatchupReset();if(log)EliteCatchupRecord(g_entryWorld?"room-loading":"identity-unavailable",-1,0,0,0,0,0);return;}
    if(!EliteCopy(g_prepareImage+0xE66C090,&world,8) || !world) {EliteCatchupRejected("world-unavailable",now);return;}
    g_catchupWorldMode=g_catchupMode(world);
    if(g_catchupWorldMode!=kEliteCatchupDungeonMode) {EliteCatchupRejected("world-mode-ineligible",now);return;}
    if(!EliteCopy(g_prepareImage+0xE638EF8,&manager,8) || !manager) {EliteCatchupRejected("manager-unavailable",now);return;}
    if(!EliteCatchupActor(callerActor,false)) {EliteCatchupRejected("player-actor-ineligible",now);return;}
    uintptr_t scene=g_catchupScene(world);if(!scene) {EliteCatchupRejected("scene-unavailable",now);return;}
    auto snapshot=EliteReadCompanions(manager,EliteCopy,player);
    if(!snapshot.available || !snapshot.consistent) {EliteCatchupRejected("companion-snapshot-unavailable",now);return;}
    int px=g_catchupX(callerActor),py=g_catchupY(callerActor),pz=g_catchupZ(callerActor);
    for(size_t i=0;i<3;++i) {
        uintptr_t weak[2]={};bool owned=snapshot.strong[i]>0 && snapshot.kind[i]==5 && snapshot.identityValid[i]==1 &&
            snapshot.ownerMatchesCurrentPlayer[i]==1 && snapshot.controllerBound[i]==1 && snapshot.ownerControllerId[i]==static_cast<int>(g_lifecycleOwner) &&
            snapshot.controllerId[i]==65535 && snapshot.controllerWire[i]==65535;
        for(size_t j=0;j<3;++j)if(i!=j && snapshot.strong[j]>0 && snapshot.objectId[i]==snapshot.objectId[j])owned=false;
        if(!owned || !EliteCopy(reinterpret_cast<void*>(manager+72+24*i),weak,16) || weak[1]<48) {
            g_catchupStates[i]={};g_catchupMissing[i]={};if(log)EliteCatchupRecord("owned-slot-ineligible",static_cast<int>(i),scene,px,py,0,0);continue;
        }
        uintptr_t actor=weak[1]-48;
        if(!EliteCatchupActor(actor,true)) {g_catchupStates[i]={};g_catchupMissing[i]={};if(log)EliteCatchupRecord("dead-or-unsupported-actor",static_cast<int>(i),scene,px,py,0,0);continue;}
        int membership=EliteCatchupMembership(scene,weak);
        int ax=g_catchupX(actor),ay=g_catchupY(actor),az=g_catchupZ(actor);
        EliteCatchupKey key={world,scene,player[0],player[1],weak[0],weak[1],static_cast<uint32_t>(snapshot.objectId[i]),static_cast<uint32_t>(g_eliteRoomSerial)};
        auto anchorKey=key;anchorKey.room=0;
        const bool missing=membership==0 && EliteCatchupSame(g_catchupAnchors[i],anchorKey);
        if(membership!=1 && !missing) {
            g_catchupStates[i]={};g_catchupMissing[i]={};
            if(log)EliteCatchupRecord(membership==0?"unanchored-scene":"ambiguous-scene",static_cast<int>(i),scene,px,py,ax,ay,0,0,0,0,world);
            continue;
        }
        if(pz!=0 || az!=0) {g_nativePathReason="airborne";g_nativePathWidth=0;g_nativePathHeight=0;}
        bool due=false;
        if(missing) {
            // Loss of registration is a separate observation, not an A* result.
            g_nativePathReason="registration-loss";g_nativePathWidth=0;g_nativePathHeight=0;
            g_catchupStates[i]={};
            if(log)EliteCatchupRecord("registration-missing",static_cast<int>(i),scene,px,py,ax,ay);
            due=EliteCatchupDue(g_catchupMissing[i],key,now,pz==0 && az==0,ElitePathResult::Unreachable,px,py,ax,ay);
        } else {
            g_catchupMissing[i]={};
            auto path=pz==0 && az==0?g_catchupQuery(actor,px,py,ax,ay):ElitePathResult::Unknown;
            if(log)EliteCatchupRecord(path==ElitePathResult::Reachable?"path-reachable":path==ElitePathResult::Unreachable?"path-unreachable":"path-unknown",static_cast<int>(i),scene,px,py,ax,ay);
            due=EliteCatchupDue(g_catchupStates[i],key,now,pz==0 && az==0,path,px,py,ax,ay);
        }
        if(!due)continue;
        uintptr_t after[2]={},currentManager=0;
        auto check=EliteReadCompanions(manager,EliteCopy,player);
        if(!check.consistent || check.identityValid[i]!=1 || check.objectId[i]!=snapshot.objectId[i] || check.ownerMatchesCurrentPlayer[i]!=1 ||
            check.controllerBound[i]!=1 || !EliteCopy(g_prepareImage+0xE638EF8,&currentManager,8) || currentManager!=manager ||
            !EliteCopy(reinterpret_cast<void*>(manager+72+24*i),after,16) || memcmp(weak,after,16) ||
            !EliteCatchupContext(world,scene,player) || EliteCatchupMembership(scene,weak)!=(missing?0:1) || !EliteCatchupActor(actor,true))continue;
        // Player's current walkable point is the fallback if the nearby offset is blocked.
        int tx=px+64*(static_cast<int>(i)-1),ty=py;
        if(!g_catchupLand(actor,tx,ty,0)) {tx=px;if(!g_catchupLand(actor,tx,ty,0)) {EliteCatchupRecord("landing-rejected",static_cast<int>(i),scene,px,py,ax,ay);continue;}}
        if(missing) {
            if(!EliteCatchupRegistrationFree(scene,key.objectId)) {EliteCatchupRecord("registration-id-conflict",static_cast<int>(i),scene,px,py,ax,ay);continue;}
            // Exact eight arguments of 145B25278..145B25298. Native code owns
            // CRefs, kind vectors, indexes and initialization; no vector writes.
            char registered=g_entryRegisterOriginal(scene,0,5,key.objectId,actor,0,0,1);
            if(registered!=1 || EliteCatchupMembership(scene,weak)!=1) {
                EliteCatchupRecord("registration-restore-rejected",static_cast<int>(i),scene,px,py,ax,ay);continue;
            }
            check=EliteReadCompanions(manager,EliteCopy,player);
            if(!check.consistent || check.strong[i]<=0 || check.kind[i]!=5 || check.identityValid[i]!=1 || check.objectId[i]!=key.objectId ||
                check.ownerMatchesCurrentPlayer[i]!=1 || check.controllerBound[i]!=1 || check.controllerId[i]!=65535 || check.controllerWire[i]!=65535 ||
                !EliteCopy(reinterpret_cast<void*>(manager+72+24*i),after,16) || memcmp(weak,after,16) ||
                !EliteCatchupContext(world,scene,player) || !EliteCatchupActor(actor,true))continue;
            EliteCatchupRecord("registration-restored",static_cast<int>(i),scene,px,py,ax,ay);
            // PID15920: 145DFFCD5 removes re-registered APCs with virtual328=0.
            // Complete the original loader's post-register virtual320(actor,1)
            // at 145B253BC; never bypass native cleanup or write actor fields.
            int retentionBefore=reinterpret_cast<EliteCatchupGet>(g_prepareImage+0x449BF20)(actor);
            if(!retentionBefore)reinterpret_cast<EliteCatchupEnable>(g_prepareImage+0x1854970)(actor,1);
            int retentionAfter=reinterpret_cast<EliteCatchupGet>(g_prepareImage+0x449BF20)(actor);
            EliteCatchupRecord(retentionAfter==1?"registration-reactivated":"reactivation-rejected",static_cast<int>(i),scene,px,py,ax,ay,0,0,0,0,world,0,retentionBefore,retentionAfter);
            if(retentionAfter!=1 || !EliteCatchupContext(world,scene,player) || EliteCatchupMembership(scene,weak)!=1)continue;
        }
        // Activation can change the derived permission result. Re-check after
        // native registration/enable, while preserving the owner's point fallback.
        if(missing && !g_catchupLand(actor,tx,ty,0)) {
            tx=px;if(!g_catchupLand(actor,tx,ty,0)) {EliteCatchupRecord("landing-rejected",static_cast<int>(i),scene,px,py,ax,ay);continue;}
        }
        g_catchupSet(actor,tx,ty,0);
        EliteCatchupRecord("teleport",static_cast<int>(i),scene,px,py,ax,ay,tx,ty,g_catchupX(actor),g_catchupY(actor));
    }
}
static __declspec(noinline) int64_t __fastcall EliteCatchupObserve(uintptr_t position,int axis) {
    // Native result/error/SEH are preserved. All game calls stay on the native
    // owner thread; there is no worker thread or retained actor/scene pointer.
    int64_t result=g_catchupOriginal(position,axis);DWORD nativeError=GetLastError();
    if(g_catchupReady && !g_catchupBusy && axis==0 && g_catchupHits<16 && InterlockedIncrement(&g_catchupHits)<=16)
        EliteCatchupRecord(!g_lifecycleArmed?"owner-not-armed":GetCurrentThreadId()!=g_lifecycleThread?"wrong-thread":"native-getter-hit",-1,0,0,0,0,0);
    if(g_catchupReady && !g_catchupFault && !g_catchupBusy && g_lifecycleArmed && GetCurrentThreadId()==g_lifecycleThread) {
        g_catchupBusy=true;
        __try {
            __try {if(axis==0)EliteCatchupSample(position,GetTickCount64());}
            __except(GetExceptionCode()==EXCEPTION_ACCESS_VIOLATION || GetExceptionCode()==EXCEPTION_IN_PAGE_ERROR?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {
                g_catchupFault=true;g_catchupFailure="runtime-memory-fault";EliteCatchupReset();EliteCatchupRecord(g_catchupFailure,-1,0,0,0,0,0);
            }
        } __finally {g_catchupBusy=false;SetLastError(nativeError);}
    }
    SetLastError(nativeError);return result;
}
static bool EliteStartCatchup(unsigned char* image,const wchar_t* path) {
    if(!g_registrationReady || image!=g_prepareImage) {g_catchupFailure="registration-dependency";return false;}
    for(auto& guard:kEliteCatchupGuards)if(!EliteReadable(image+guard.rva,guard.size) || memcmp(image+guard.rva,guard.bytes,guard.size)) {
        g_catchupFailure="catchup-bytes-mismatch";return false;
    }
    if(wcscpy_s(g_catchupPath,path)) {g_catchupFailure="path";return false;}
    if(!EliteStartNativePath(image)) {g_catchupFailure="native-path-guard-or-hook";return false;}
    g_catchupX=reinterpret_cast<EliteCatchupGet>(image+0x5B8C7D0);
    g_catchupY=reinterpret_cast<EliteCatchupGet>(image+0x5B8C890);g_catchupZ=reinterpret_cast<EliteCatchupGet>(image+0x5B8C8C0);
    g_catchupMode=reinterpret_cast<EliteCatchupGet>(image+0x59A90F0);g_catchupScene=reinterpret_cast<EliteCatchupScene>(image+0x59A9260);
    g_catchupSet=reinterpret_cast<EliteCatchupSet>(image+0x5B960D0);g_catchupLand=EliteCatchupLanding;
    if(MH_CreateHook(image+0x5E08940,reinterpret_cast<void*>(EliteCatchupObserve),reinterpret_cast<void**>(&g_catchupOriginal))!=MH_OK) {
        MH_DisableHook(image+0x6D642C0);MH_RemoveHook(image+0x6D642C0);g_catchupFailure="hook-create";return false;
    }
    if(MH_EnableHook(image+0x5E08940)!=MH_OK) {
        MH_RemoveHook(image+0x5E08940);MH_DisableHook(image+0x6D642C0);MH_RemoveHook(image+0x6D642C0);g_catchupFailure="hook-enable";return false;
    }
    g_catchupReady=true;g_entryRegistrationDone=EliteCatchupRegistered;g_catchupFailure="ready";EliteCatchupRecord("started",-1,0,0,0,0,0);return true;
}
