#pragma once
#include "entry-trace.h"
#include "combat-ownership.h"
using EliteAttackAssign=void (__fastcall *)(uintptr_t,uintptr_t);
using EliteKillerSet=void (__fastcall *)(uintptr_t,int);
using EliteDeathSend=uintptr_t (__fastcall *)(uintptr_t,char,unsigned char);
static EliteAttackAssign g_combatAssignOriginal;
static EliteKillerSet g_combatSetOriginal;
static EliteDeathSend g_combatDeathOriginal;
static bool g_combatReady;
static const char* g_combatFailure="not-started";
static wchar_t g_combatPath[32768];
static SRWLOCK g_combatLock=SRWLOCK_INIT;
static LONG g_combatCount,g_combatDropped,g_combatTransaction;
static const LONG kEliteCombatLimit=4096;
struct EliteCombatContext {uintptr_t victim,input;LONG transaction;bool assigning;};
static __declspec(thread) EliteCombatContext g_combatContext;
static EliteCombatSnapshot EliteCombatCapture(uintptr_t victim) {
    uintptr_t manager=0,player[2]={},after[2]={};
    EliteCopy(g_prepareImage+0xE638EF8,&manager,8);
    bool available=EliteCurrentPlayerReference(player);
    auto s=EliteReadCombat(victim,manager,available?player:nullptr,EliteCopy);
    if(!available || !EliteCurrentPlayerReference(after) || memcmp(player,after,16))s.stable=false;
    return s;
}
static void EliteCombatRecord(const char* phase,LONG transaction,LONG parent,uintptr_t caller,uintptr_t input,int argument,
        bool completed,uintptr_t result,const EliteCombatSnapshot& before,const EliteCombatSnapshot& after,int effective=-1) {
    if(!TryAcquireSRWLockExclusive(&g_combatLock)) {InterlockedIncrement(&g_combatDropped);return;}
    LONG ordinal=++g_combatCount;
    if(ordinal>kEliteCombatLimit+1) {ReleaseSRWLockExclusive(&g_combatLock);return;}
    const auto& sample=!strcmp(phase,"death-send")?before:after;
    char line[2300]={};
    int length=sprintf_s(line,
        "{\"traceVersion\":2,\"processId\":%lu,\"runTick\":%llu,\"ordinal\":%ld,\"tick\":%llu,\"thread\":%lu,\"phase\":\"%s\",\"transaction\":%ld,\"parent\":%ld,\"callerRva\":%llu,\"input\":%llu,\"argument\":%d,\"completed\":%d,\"result\":%llu,\"traceDropped\":%ld,\"traceLimit\":%d,\"victim\":%llu,\"victimId\":%lld,\"victimKind\":%d,\"readable\":[%d,%d],\"stable\":[%d,%d],\"killer\":[%u,%u],\"sourceField\":[%u,%u],\"sourceRef\":[%llu,%llu,%llu],\"sourceId\":%lld,\"sourceKind\":%d,\"sourceStrong\":%d,\"sourceController\":%d,\"combatGetter\":%llu,\"eliteSlot\":%d,\"ownerId\":%lld,\"ownerController\":%d,\"ownerMatches\":%d,\"playerRef\":[%llu,%llu],\"battleEnabled\":false,\"nativeWritesChanged\":%s,\"effectiveArgument\":%d,\"uniqueSource\":%d,\"controllerBound\":%d,\"controllerWire\":%d,\"combatGetterRva\":%llu,\"roomSerial\":%ld}\r\n",
        GetCurrentProcessId(),g_eliteTraceRunTick,ordinal,GetTickCount64(),GetCurrentThreadId(),phase,transaction,parent,
        static_cast<unsigned long long>(caller),static_cast<unsigned long long>(input),argument,completed,static_cast<unsigned long long>(result),
        InterlockedCompareExchange(&g_combatDropped,0,0),ordinal==kEliteCombatLimit+1,
        static_cast<unsigned long long>(sample.victim),sample.victimId,sample.victimKind,before.readable,after.readable,before.stable,after.stable,
        before.killer,after.killer,before.sourceField,after.sourceField,
        static_cast<unsigned long long>(sample.sourceRef[0]),static_cast<unsigned long long>(sample.sourceRef[1]),static_cast<unsigned long long>(sample.sourceRef[2]),
        sample.sourceId,sample.sourceKind,sample.sourceStrong,sample.sourceController,static_cast<unsigned long long>(sample.combatGetter),sample.slot,
        sample.ownerId,sample.ownerController,sample.ownerMatches,static_cast<unsigned long long>(sample.playerRef[0]),static_cast<unsigned long long>(sample.playerRef[1]),effective>=0 && effective!=argument ? "true":"false",effective>=0?effective:argument,sample.unique,sample.bound,sample.controllerWire,static_cast<unsigned long long>(sample.combatGetter>=reinterpret_cast<uintptr_t>(g_prepareImage)?sample.combatGetter-reinterpret_cast<uintptr_t>(g_prepareImage):0),InterlockedCompareExchange(&g_eliteRoomSerial,0,0));
    HANDLE file=length>0?CreateFileW(g_combatPath,FILE_APPEND_DATA,FILE_SHARE_READ|FILE_SHARE_WRITE,nullptr,OPEN_ALWAYS,FILE_ATTRIBUTE_NORMAL,nullptr):INVALID_HANDLE_VALUE;
    DWORD written=0;
    if(file==INVALID_HANDLE_VALUE || !WriteFile(file,line,static_cast<DWORD>(length),&written,nullptr) || written!=static_cast<DWORD>(length))InterlockedIncrement(&g_combatDropped);
    if(file!=INVALID_HANDLE_VALUE)CloseHandle(file);
    ReleaseSRWLockExclusive(&g_combatLock);
}
static bool EliteCombatArmed() {return g_combatReady && InterlockedCompareExchange(&g_combatCount,0,0)<=kEliteCombatLimit && EliteEntryIdentity();}
static __declspec(noinline) void __fastcall EliteCombatAssignObserve(uintptr_t victim,uintptr_t input) {
    DWORD incoming=GetLastError();bool armed=EliteCombatArmed();
    EliteCombatContext previous=g_combatContext;
    LONG transaction=armed?InterlockedIncrement(&g_combatTransaction):0;
    uintptr_t caller=reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage);
    auto before=armed?EliteCombatCapture(victim):EliteCombatSnapshot{};
    g_combatContext={victim,input,transaction,true};bool completed=false;
    SetLastError(incoming);
    __try {g_combatAssignOriginal(victim,input);completed=true;}
    __finally {
        DWORD nativeError=GetLastError();
        if(armed)EliteCombatRecord("source-assignment",transaction,previous.transaction,caller,input,0,completed,0,before,EliteCombatCapture(victim));
        g_combatContext=previous;SetLastError(nativeError);
    }
}
static __declspec(noinline) void __fastcall EliteCombatSetObserve(uintptr_t victim,int value) {
    DWORD incoming=GetLastError();bool armed=EliteCombatArmed();
    LONG transaction=armed?InterlockedIncrement(&g_combatTransaction):0;
    uintptr_t caller=reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage);
    bool identity=g_combatReady && EliteEntryIdentity();
    auto before=(armed || identity)?EliteCombatCapture(victim):EliteCombatSnapshot{};bool completed=false;
    int effective=EliteOwnedKiller(before,value,caller,identity,g_combatContext.assigning,g_combatContext.victim==victim,g_lifecycleOwner,reinterpret_cast<uintptr_t>(g_prepareImage));
    SetLastError(incoming);
    __try {g_combatSetOriginal(victim,effective);completed=true;}
    __finally {
        DWORD nativeError=GetLastError();
        if(armed)EliteCombatRecord("killer-write",transaction,g_combatContext.victim==victim?g_combatContext.transaction:0,caller,
            g_combatContext.victim==victim?g_combatContext.input:0,value,completed,0,before,EliteCombatCapture(victim),effective);
        SetLastError(nativeError);
    }
}
static __declspec(noinline) uintptr_t __fastcall EliteCombatDeathObserve(uintptr_t victim,char a2,unsigned char a3) {
    DWORD incoming=GetLastError();bool armed=EliteCombatArmed();
    LONG transaction=armed?InterlockedIncrement(&g_combatTransaction):0;
    uintptr_t caller=reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage);
    auto before=armed?EliteCombatCapture(victim):EliteCombatSnapshot{};bool completed=false;uintptr_t result=0;
    SetLastError(incoming);
    __try {result=g_combatDeathOriginal(victim,a2,a3);completed=true;}
    __finally {
        DWORD nativeError=GetLastError();
        if(armed)EliteCombatRecord("death-send",transaction,g_combatContext.transaction,caller,0,
            static_cast<int>(static_cast<unsigned char>(a2))|(static_cast<int>(a3)<<8),completed,result,before,EliteCombatCapture(victim));
        SetLastError(nativeError);
    }
    return result;
}
static bool EliteStartCombatTrace(unsigned char* image,const wchar_t* path) {
    if(!g_registrationReady || image!=g_prepareImage) {g_combatFailure="registration-dependency";return false;}
    const unsigned char selfGetter[]={0x48,0x8b,0xc1,0xc3};
    const unsigned char assign[]={0x40,0x57,0x48,0x83,0xec,0x50,0x48,0xc7,0x44,0x24,0x20,0xfe,0xff,0xff,0xff,0x48,0x89,0x5c,0x24,0x68,0x48,0x89,0x6c,0x24,0x70,0x48,0x89,0x74,0x24,0x78,0x48,0x8b};
    const unsigned char setter[]={0x89,0x91,0xa0,0x68,0x00,0x00,0xc3};
    const unsigned char death[]={0x40,0x55,0x56,0x57,0x41,0x54,0x41,0x55,0x41,0x56,0x41,0x57,0x48,0x8d,0xac,0x24,0xe0,0xfe,0xff,0xff,0x48,0x81,0xec,0x20,0x02,0x00,0x00,0x48,0xc7,0x85,0xd0,0x00};
    const unsigned char chain[]={0x74,0x3d,0x48,0x8b,0xcb,0xe8,0x7a,0xc4,0xdb,0xff,0x89,0x87,0x04,0x89,0x00,0x00,0x48,0x8b,0x03,0x48,0x8b,0xcb,0xff,0x90,0xc0,0x07,0x00,0x00,0x48,0x85,0xc0,0x74,0x1e,0x48,0x8b,0x03,0x48,0x8b,0xcb,0xff,0x90,0xc0,0x07,0x00,0x00,0x48,0x8b,0xc8,0xe8,0x3f,0x26,0xf1,0xff,0x8b,0xd0,0x48,0x8b,0xcf,0xe8,0x75,0xe6,0xe6,0xff,0x48,0x8b,0x5c,0x24};
    const unsigned char packet[]={0xe8,0x2e,0xa4,0xfa,0x00,0xba,0x27,0x00,0x00,0x00,0x48,0x8b,0xc8,0xe8,0x01,0xab,0xfa,0x00,0xe8,0x1c,0xa4,0xfa,0x00,0x48,0x8b,0xf8,0x48,0x8b,0xcb,0xe8,0x81,0x2a,0xdc,0xff,0x8b,0xd0,0x48,0x8b,0xcf,0xe8,0xe7,0xc0,0xfa,0x00,0xe8,0x02,0xa4,0xfa,0x00,0x41,0x0f,0xb7,0xd7,0x48,0x8b,0xc8,0xe8,0x76,0xc5,0xfa,0x00};
    struct Guard {uintptr_t rva;const unsigned char* bytes;size_t size;};
    const Guard guards[]={{0x14CC20,selfGetter,sizeof(selfGetter)},{0x5DD0070,assign,sizeof(assign)},{0x5C3E8A0,setter,sizeof(setter)},{0x5DC86D0,death,sizeof(death)},{0x5DD01EC,chain,sizeof(chain)},{0x5DC9BCD,packet,sizeof(packet)}};
    for(auto& guard:guards)if(!EliteReadable(image+guard.rva,guard.size) || memcmp(image+guard.rva,guard.bytes,guard.size)) {g_combatFailure="combat-bytes-mismatch";return false;}
    if(wcscpy_s(g_combatPath,path)) {g_combatFailure="path";return false;}
    void* targets[]={image+0x5DD0070,image+0x5C3E8A0,image+0x5DC86D0};
    void* replacements[]={reinterpret_cast<void*>(EliteCombatAssignObserve),reinterpret_cast<void*>(EliteCombatSetObserve),reinterpret_cast<void*>(EliteCombatDeathObserve)};
    void** originals[]={reinterpret_cast<void**>(&g_combatAssignOriginal),reinterpret_cast<void**>(&g_combatSetOriginal),reinterpret_cast<void**>(&g_combatDeathOriginal)};
    size_t created=0;bool ok=true;
    for(;created<3;++created) {
        if(MH_CreateHook(targets[created],replacements[created],originals[created])!=MH_OK) {g_combatFailure="hook-create";ok=false;break;}
        if(MH_QueueEnableHook(targets[created])!=MH_OK) {++created;g_combatFailure="hook-queue";ok=false;break;}
    }
    if(ok && MH_ApplyQueued()!=MH_OK) {g_combatFailure="hook-apply";ok=false;}
    if(!ok) {while(created) {--created;MH_DisableHook(targets[created]);MH_RemoveHook(targets[created]);}return false;}
    g_combatReady=true;g_combatFailure="ready";return true;
}
