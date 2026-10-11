#pragma once
#include "native-trace.h"
#include "registration-policy.h"

using EliteRoomLoader = void (__fastcall *)(uintptr_t,unsigned int,unsigned int,char,char,uintptr_t,unsigned char,char);
using EliteSceneRegister = char (__fastcall *)(uintptr_t,uintptr_t,unsigned int,unsigned int,uintptr_t,uintptr_t,char,char);
static EliteRoomLoader g_entryOriginal;
static EliteNativePredicate g_entryGateOriginal;
static EliteSceneRegister g_entryRegisterOriginal;
static wchar_t g_entryPath[32768];
static SRWLOCK g_entryLock=SRWLOCK_INIT;
static LONG g_entryCount,g_entryDropped;
static bool g_entryReady;
static const char* g_entryFailure="not-started";
static __declspec(thread) uintptr_t g_entryWorld,g_entryScene,g_entryLoaderCaller;
static __declspec(thread) bool g_entryFreshDungeon;
static __declspec(thread) EliteRegistrationFacts g_registrationChecks;
static void (*g_entryBranchReset)();
static __declspec(thread) uintptr_t g_registrationPlayer[2],g_registrationNativePlayer[2];

static bool EliteEntryIdentity() {
    return g_entryReady && g_lifecycleArmed && GetCurrentThreadId()==g_lifecycleThread &&
        EliteNativeChannel()==g_lifecycleChannel && EliteCachedOwner()==g_lifecycleOwner;
}
static void EliteEntryRecord(const char* phase,uintptr_t caller,uintptr_t scene=0,int nativeResult=-1) {
    if (!EliteEntryIdentity()) return;
    if (!TryAcquireSRWLockExclusive(&g_entryLock)) {InterlockedIncrement(&g_entryDropped);return;}
    LONG ordinal=++g_entryCount;
    if (ordinal>513) {ReleaseSRWLockExclusive(&g_entryLock);return;}
    uintptr_t manager=0,player[2]={},dungeon=0;
    EliteCopy(g_prepareImage+0xE638EF8,&manager,sizeof(manager));
    bool playerRead=EliteCurrentPlayerReference(player);
    auto s=EliteReadCompanions(manager,EliteCopy,playerRead?player:nullptr);
    unsigned char active=0,condition=0,condition8056=0;
    bool worldRead=g_entryWorld && EliteCopy(reinterpret_cast<void*>(g_entryWorld+6792),&dungeon,sizeof(dungeon));
    bool conditionRead=dungeon && EliteCopy(reinterpret_cast<void*>(dungeon+6264),&condition,1) && EliteCopy(reinterpret_cast<void*>(dungeon+8056),&condition8056,1);
    bool managerRead=manager && EliteCopy(reinterpret_cast<void*>(manager+576),&active,1);
    uintptr_t playerScene[2]={},vtable[3]={},ownerGetter[3]={},combatGetter[3]={};
    int membership[3]={-1,-1,-1};
    if(playerRead) EliteCopy(reinterpret_cast<void*>(player[1]-48+400),playerScene,sizeof(playerScene));
    uintptr_t vector[3]={},afterVector[3]={};
    bool vectorRead=scene && EliteCopy(reinterpret_cast<void*>(scene+88),vector,sizeof(vector)) && vector[0]<=vector[1] && vector[1]<=vector[2] && (vector[1]-vector[0])%24==0 && (vector[1]-vector[0])/24<=4096;
    for(size_t i=0;i<3;++i) {
        uintptr_t weak[2]={};
        if(!manager || !EliteCopy(reinterpret_cast<void*>(manager+72+24*i),weak,sizeof(weak)) || s.strong[i]<=0 || weak[1]<48) continue;
        EliteCopy(reinterpret_cast<void*>(weak[1]-48),&vtable[i],sizeof(uintptr_t));
        if(vtable[i]) {EliteCopy(reinterpret_cast<void*>(vtable[i]+288),&ownerGetter[i],sizeof(uintptr_t));EliteCopy(reinterpret_cast<void*>(vtable[i]+1984),&combatGetter[i],sizeof(uintptr_t));}
        if(vectorRead) {
            membership[i]=0;
            for(uintptr_t item=vector[0];item<vector[1];item+=24) {
                uintptr_t ref[3]={};
                if(!EliteCopy(reinterpret_cast<void*>(item),ref,sizeof(ref))) {vectorRead=false;membership[i]=-1;break;}
                if(ref[1]==weak[0] && ref[2]==weak[1]) ++membership[i];
            }
        }
    }
    if(vectorRead && (!EliteCopy(reinterpret_cast<void*>(scene+88),afterVector,sizeof(afterVector)) || memcmp(vector,afterVector,sizeof(vector)))) vectorRead=false;
    uintptr_t playerAfter[2]={};
    bool playerStable=playerRead && EliteCurrentPlayerReference(playerAfter) && !memcmp(player,playerAfter,sizeof(player));
    char line[2800]={};
    int length=sprintf_s(line,sizeof(line),
        "{\"traceVersion\":1,\"processId\":%lu,\"runTick\":%llu,\"ordinal\":%ld,\"tick\":%llu,\"thread\":%lu,\"phase\":\"%s\",\"callerRva\":%llu,\"loaderCallerRva\":%llu,\"freshDungeonEntry\":%d,\"nativeResult\":%d,\"traceDropped\":%ld,\"traceLimit\":%d,\"worldRead\":%d,\"conditionRead\":%d,\"condition6264\":%u,\"condition8056\":%u,\"managerRead\":%d,\"manager576\":%u,\"scene\":%llu,\"playerScene\":%llu,\"playerStable\":%d,\"sceneVectorReadable\":%d,\"sceneVectorMatches\":[%d,%d,%d],\"available\":%d,\"consistent\":%d,\"weakAlive\":%d,\"objectId\":[%lld,%lld,%lld],\"ownerMatchesCurrentPlayer\":[%d,%d,%d],\"controllerId\":[%d,%d,%d],\"vtable\":[%llu,%llu,%llu],\"ownerGetter\":[%llu,%llu,%llu],\"combatGetter\":[%llu,%llu,%llu],\"registrationChecks\":[%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%u],\"registrationPlayerRef\":[%llu,%llu],\"registrationNativePlayerRef\":[%llu,%llu],\"battleEnabled\":false}\r\n",
        GetCurrentProcessId(),g_eliteTraceRunTick,ordinal,GetTickCount64(),GetCurrentThreadId(),phase,static_cast<unsigned long long>(caller),static_cast<unsigned long long>(g_entryLoaderCaller),g_entryFreshDungeon,nativeResult,
        InterlockedCompareExchange(&g_entryDropped,0,0),ordinal==513,worldRead,conditionRead,condition,condition8056,managerRead,active,
        static_cast<unsigned long long>(scene),static_cast<unsigned long long>(playerScene[1]),playerStable,vectorRead,
        membership[0],membership[1],membership[2],s.available,s.consistent,s.alive,s.objectId[0],s.objectId[1],s.objectId[2],
        s.ownerMatchesCurrentPlayer[0],s.ownerMatchesCurrentPlayer[1],s.ownerMatchesCurrentPlayer[2],s.controllerId[0],s.controllerId[1],s.controllerId[2],
        static_cast<unsigned long long>(vtable[0]),static_cast<unsigned long long>(vtable[1]),static_cast<unsigned long long>(vtable[2]),
        static_cast<unsigned long long>(ownerGetter[0]),static_cast<unsigned long long>(ownerGetter[1]),static_cast<unsigned long long>(ownerGetter[2]),
        static_cast<unsigned long long>(combatGetter[0]),static_cast<unsigned long long>(combatGetter[1]),static_cast<unsigned long long>(combatGetter[2]),g_registrationChecks.ready,g_registrationChecks.identity,g_registrationChecks.world,g_registrationChecks.dungeon,g_registrationChecks.scene,g_registrationChecks.stable,g_registrationChecks.nativePlayer,g_registrationChecks.references,g_registrationChecks.notRegistered,g_registrationChecks.managerIdle,g_registrationChecks.alive,static_cast<unsigned long long>(g_registrationPlayer[0]),static_cast<unsigned long long>(g_registrationPlayer[1]),static_cast<unsigned long long>(g_registrationNativePlayer[0]),static_cast<unsigned long long>(g_registrationNativePlayer[1]));
    HANDLE f=length>0?CreateFileW(g_entryPath,FILE_APPEND_DATA,FILE_SHARE_READ|FILE_SHARE_WRITE,nullptr,OPEN_ALWAYS,FILE_ATTRIBUTE_NORMAL,nullptr):INVALID_HANDLE_VALUE;
    DWORD written=0;
    if(f==INVALID_HANDLE_VALUE || !WriteFile(f,line,static_cast<DWORD>(length),&written,nullptr) || written!=static_cast<DWORD>(length)) InterlockedIncrement(&g_entryDropped);
    if(f!=INVALID_HANDLE_VALUE) CloseHandle(f);
    ReleaseSRWLockExclusive(&g_entryLock);
}
static __declspec(noinline) void __fastcall EliteEntryObserve(uintptr_t world,unsigned int x,unsigned int y,char a4,char a5,uintptr_t a6,unsigned char a7,char a8) {
    if(EliteEntryIdentity())InterlockedIncrement(&g_eliteRoomSerial);
    const auto previous=g_entryWorld;
    const auto previousCaller=g_entryLoaderCaller;const bool previousFresh=g_entryFreshDungeon;
    g_entryWorld=world;g_entryScene=0;g_registrationChecks={};
    memset(g_registrationPlayer,0,sizeof(g_registrationPlayer));memset(g_registrationNativePlayer,0,sizeof(g_registrationNativePlayer));
    if(g_entryBranchReset)g_entryBranchReset();
    const auto caller=reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage);
    g_entryLoaderCaller=caller;
    g_entryFreshDungeon=caller==0x6D2A1E7 && a4==0 && a5==1 && a6==0 && a7==0 && a8==1;
    DWORD incoming=GetLastError();
    EliteEntryRecord("loader-before",caller);
    SetLastError(incoming);
    __try {
        g_entryOriginal(world,x,y,a4,a5,a6,a7,a8);
        DWORD nativeError=GetLastError();
        EliteEntryRecord("loader-after",caller,g_entryScene);
        EliteLifecycleSample("native-room-loaded",caller);
        SetLastError(nativeError);
    } __finally {g_entryWorld=previous;g_entryLoaderCaller=previousCaller;g_entryFreshDungeon=previousFresh;g_entryScene=0;if(g_entryBranchReset)g_entryBranchReset();}
}
static __declspec(noinline) char __fastcall EliteEntryGateObserve() {
    const auto caller=reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage);
    // Native 145B22F50 RSP locals: scene at +0x90; return slot is RSP-8.
    // Observe, never redirect a return or modify the condition result.
    DWORD incoming=GetLastError();
    uintptr_t scene=0;
    if(caller==0x5B25191 && g_entryWorld) EliteCopy(static_cast<unsigned char*>(_AddressOfReturnAddress())+0x98,&scene,sizeof(scene));
    SetLastError(incoming);
    char result=g_entryGateOriginal();
    DWORD nativeError=GetLastError();
    if(caller==0x5B25191 && g_entryWorld) {g_entryScene=scene;EliteEntryRecord("registration-gate",caller,scene,result);}
    SetLastError(nativeError);
    return result;
}
static __declspec(noinline) char __fastcall EliteEntryRegisterObserve(uintptr_t scene,uintptr_t parent,unsigned int kind,unsigned int id,uintptr_t actor,uintptr_t a6,char a7,char a8) {
    const auto caller=reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage);
    char result=g_entryRegisterOriginal(scene,parent,kind,id,actor,a6,a7,a8);
    DWORD nativeError=GetLastError();
    if(caller==0x5B2529D && g_entryWorld) EliteEntryRecord("native-registration-result",caller,scene,result);
    SetLastError(nativeError);
    return result;
}
static bool EliteStartEntryTrace(unsigned char* image,const wchar_t* path) {
    const unsigned char guard0[]={0x40,0x55,0x56,0x57,0x41,0x54,0x41,0x55,0x41,0x56,0x41,0x57,0x48,0x8d,0xac,0x24,0x00,0xff,0xff,0xff,0x48,0x81,0xec,0x00,0x02,0x00,0x00};
    if(!EliteReadable(image+0x5b22f50,sizeof(guard0)) || memcmp(image+0x5b22f50,guard0,sizeof(guard0))) {g_entryFailure="bytes-0x5b22f50";return false;}
    const unsigned char guard1[]={0x40,0x53,0x48,0x83,0xec,0x20,0x48,0x8b,0x1d,0x7b,0x72,0x77,0x08,0x48,0x85,0xdb,0x74};
    if(!EliteReadable(image+0x5f0c980,sizeof(guard1)) || memcmp(image+0x5f0c980,guard1,sizeof(guard1))) {g_entryFailure="bytes-0x5f0c980";return false;}
    const unsigned char guard2[]={0xe8,0xef,0x77,0x3e,0x00,0x84,0xc0,0x0f,0x84,0x28,0x03,0x00,0x00,0x49,0x8b,0x87,0x88,0x1a,0x00,0x00,0x48,0x85,0xc0,0x0f,0x84,0x18,0x03,0x00,0x00,0x80,0xb8,0x78,0x18,0x00,0x00,0x00,0x0f,0x84,0x0b,0x03,0x00,0x00,0xe8,0xb5,0xf3,0xfa,0xfa,0x48,0x8b,0xc8,0xe8,0x2d,0xbb,0x33,0xfd,0x84,0xc0,0x0f,0x84,0xf6,0x02,0x00,0x00,0xe8,0xa0,0xf3,0xfa,0xfa,0x48,0x8b,0xc8,0xe8,0x08,0xbb,0x33,0xfd,0x45,0x33,0xe4,0x84,0xc0,0x0f,0x85,0xe1,0x02,0x00,0x00,0x45,0x8b,0xf4};
    if(!EliteReadable(image+0x5b2518c,sizeof(guard2)) || memcmp(image+0x5b2518c,guard2,sizeof(guard2))) {g_entryFailure="bytes-0x5b2518c";return false;}
    const unsigned char guard3[]={0x41,0x83,0xf8,0x02,0x7f,0x03,0x32,0xc0,0xc3,0x0f,0xb6,0x44,0x24,0x40,0x88,0x44,0x24,0x40,0x0f,0xb6,0x44,0x24,0x38,0x88};
    if(!EliteReadable(image+0x5de4360,sizeof(guard3)) || memcmp(image+0x5de4360,guard3,sizeof(guard3))) {g_entryFailure="bytes-0x5de4360";return false;}
    const unsigned char guard4[]={0xe8,0xc3,0xf0,0x2b,0x00,0x84,0xc0,0x0f,0x84,0x9f,0x01,0x00};
    if(!EliteReadable(image+0x5b25298,sizeof(guard4)) || memcmp(image+0x5b25298,guard4,sizeof(guard4))) {g_entryFailure="bytes-0x5b25298";return false;}
    if(wcscpy_s(g_entryPath,path)) {g_entryFailure="path";return false;}
    void* targets[]={image+0x5B22F50,image+0x5F0C980,image+0x5DE4360};
    void* replacements[]={reinterpret_cast<void*>(EliteEntryObserve),reinterpret_cast<void*>(EliteEntryGateObserve),reinterpret_cast<void*>(EliteEntryRegisterObserve)};
    void** originals[]={reinterpret_cast<void**>(&g_entryOriginal),reinterpret_cast<void**>(&g_entryGateOriginal),reinterpret_cast<void**>(&g_entryRegisterOriginal)};
    size_t created=0;
    bool ok=true;
    for(;created<3;++created) {
        if(MH_CreateHook(targets[created],replacements[created],originals[created])!=MH_OK) {g_entryFailure="hook-create";ok=false;break;}
        if(MH_QueueEnableHook(targets[created])!=MH_OK) {++created;g_entryFailure="hook-queue";ok=false;break;}
    }
    if(ok && MH_ApplyQueued()!=MH_OK) {g_entryFailure="hook-apply";ok=false;}
    if(!ok) {while(created) {--created;MH_DisableHook(targets[created]);MH_RemoveHook(targets[created]);}return false;}
    g_entryFailure="ready";g_entryReady=true;return true;
}
