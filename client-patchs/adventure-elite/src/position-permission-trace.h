#pragma once
#include "position-write-trace.h"

// Observe calls made by native virtual5680 and the one-shot birth virtual632.
// The birth scope is tagged separately and forwards exactly one existing call.
// No extra permission, path,
// collision, AI, position setter or cache function is invoked by this observer.
// All native boolean functions actually return AL. Keep full RAX as well.
using ElitePermission3=uintptr_t (__fastcall *)(uintptr_t,int,int);
using ElitePermission4=uintptr_t (__fastcall *)(uintptr_t,int,int,uintptr_t);
static ElitePermission3 g_permissionPoint,g_permissionGate,g_permissionGrid;
static ElitePermission4 g_permissionEffects,g_permissionMotion,g_permissionCollision,g_permissionLanding,g_permissionDerived;
static bool g_permissionReady,g_permissionFault;
static const char* g_permissionFailure="not-started";
static wchar_t g_permissionPath[32768];
static SRWLOCK g_permissionLock=SRWLOCK_INIT;
static unsigned g_permissionRecords,g_permissionSamples[3];
static uint64_t g_permissionNextSample[3],g_permissionOrdinal;
static uintptr_t g_permissionActors[3],g_permissionScenes[3];
struct ElitePermissionContext {
    ElitePositionObservation position={};uint64_t ordinal=0;
    int x=0,y=0;uintptr_t caller=0;
};
static __declspec(thread) ElitePermissionContext* g_permissionContext;
struct ElitePermissionGuard {uintptr_t rva;unsigned char bytes[32];};
static constexpr ElitePermissionGuard kElitePermissionGuards[]={
    {0x5c70e90, {0x48,0x89,0x5c,0x24,0x08,0x48,0x89,0x74,0x24,0x10,0x57,0x48,0x83,0xec,0x20,0x41,0x8b,0xd8,0x8b,0xfa,0x48,0x8b,0xf1,0xe8,0x54,0xff,0xff,0xff,0x84,0xc0,0x75,0x10}},
    {0x5c70e00, {0x48,0x89,0x5c,0x24,0x08,0x57,0x48,0x83,0xec,0x20,0x48,0x8b,0x01,0x41,0x8b,0xd8,0x8b,0xfa,0xff,0x90,0x90,0x00,0x00,0x00,0x3c,0x01,0x75,0x4f,0x48,0x8b,0x0d,0x6d}},
    {0x6d64a50, {0x4c,0x8b,0x89,0xf8,0xc0,0x01,0x00,0x4d,0x85,0xc9,0x74,0x2a,0x85,0xd2,0x78,0x26,0x45,0x85,0xc0,0x78,0x21,0x8b,0x81,0xec,0xc0,0x01,0x00,0x3b,0xd0,0x7d,0x17,0x44}},
    {0x5cf6910, {0x48,0x89,0x5c,0x24,0x08,0x48,0x89,0x6c,0x24,0x10,0x48,0x89,0x74,0x24,0x18,0x48,0x89,0x7c,0x24,0x20,0x41,0x56,0x48,0x83,0xec,0x20,0x8b,0x81,0x8c,0x22,0x01,0x00}},
    {0x5c232c0, {0x40,0x57,0x41,0x54,0x41,0x56,0x41,0x57,0x48,0x83,0xec,0x28,0x80,0xb9,0xd3,0x6c,0x00,0x00,0x00,0x4d,0x8b,0xf1,0x45,0x8b,0xf8,0x44,0x8b,0xe2,0x48,0x8b,0xf9,0x74}},
    {0x5d749a0, {0x48,0x89,0x5c,0x24,0x08,0x48,0x89,0x6c,0x24,0x10,0x48,0x89,0x74,0x24,0x18,0x48,0x89,0x7c,0x24,0x20,0x41,0x56,0x48,0x83,0xec,0x50,0x33,0xf6,0x49,0x8b,0xf9,0x41}},
    {0x5c70ee0, {0x48,0x89,0x5c,0x24,0x08,0x48,0x89,0x6c,0x24,0x10,0x48,0x89,0x74,0x24,0x18,0x57,0x48,0x83,0xec,0x20,0x49,0x8b,0xe9,0x41,0x8b,0xd8,0x8b,0xfa,0x48,0x8b,0xf1,0xe8}},
    {0x63de5d0, {0x48,0x89,0x6c,0x24,0x18,0x48,0x89,0x74,0x24,0x20,0x57,0x41,0x56,0x41,0x57,0x48,0x83,0xec,0x20,0x48,0x8b,0x01,0x49,0x8b,0xe9,0x45,0x8b,0xf0,0x44,0x8b,0xfa,0x48}}
};
static void ElitePermissionFault() {
    g_permissionFault=true;g_permissionFailure="runtime-memory-fault";
    EliteCatchupRecord("permission-observer-memory-fault",-1,0,0,0,0,0);
}
static ElitePermissionContext ElitePermissionBefore(uintptr_t actor,int x,int y,uintptr_t caller,bool birth=false) {
    ElitePermissionContext out={};
    if(!g_permissionReady || g_permissionFault || g_permissionContext || GetCurrentThreadId()!=g_lifecycleThread)return out;
    __try {
        uintptr_t component=0,vtable=0,point=0;
        if(!EliteCopy(reinterpret_cast<void*>(actor+368),&component,8) ||
           !EliteCopy(reinterpret_cast<void*>(actor),&vtable,8) ||
           !EliteCopy(reinterpret_cast<void*>(vtable+5680),&point,8) ||
           (!birth && point!=reinterpret_cast<uintptr_t>(g_prepareImage+0x5C70E90)) ||
           !ElitePositionIdentity(component,&out.position) || out.position.actor!=actor ||
           EliteCatchupMembership(out.position.scene,out.position.weak)!=1) {out.position.slot=-1;return out;}
        const int slot=out.position.slot;const auto now=GetTickCount64();
        if(g_permissionActors[slot]!=actor || g_permissionScenes[slot]!=out.position.scene) {
            g_permissionActors[slot]=actor;g_permissionScenes[slot]=out.position.scene;
            g_permissionSamples[slot]=0;g_permissionNextSample[slot]=0;
        }
        if(!birth && g_permissionSamples[slot]>=16 && now<g_permissionNextSample[slot]) {out.position.slot=-1;return out;}
        ++g_permissionSamples[slot];g_permissionNextSample[slot]=now+2000;
        out.ordinal=++g_permissionOrdinal;out.x=x;out.y=y;out.caller=caller;
    }
    __except(GetExceptionCode()==EXCEPTION_ACCESS_VIOLATION || GetExceptionCode()==EXCEPTION_IN_PAGE_ERROR?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {out.position.slot=-1;ElitePermissionFault();}
    return out;
}
static void ElitePermissionRecord(const char* phase,uintptr_t argument,int x,int y,uintptr_t auxiliary,uintptr_t result) {
    const auto* ctx=g_permissionContext;
    if(!ctx || ctx->position.slot<0 || g_permissionFault || (strcmp(phase,"grid") && argument!=ctx->position.actor))return;
    if(!TryAcquireSRWLockExclusive(&g_permissionLock))return;
    __try {
        if(g_permissionRecords<2048) {
            unsigned char blocked=0,enabled=0,mapOverride=0;
            uintptr_t attached[3]={},sceneRef[2]={};
            const auto actor=ctx->position.actor;
            const bool flagsKnown=EliteCopy(reinterpret_cast<void*>(actor+27859),&blocked,1) &&
                EliteCopy(reinterpret_cast<void*>(actor+720),&enabled,1) &&
                EliteCopy(g_prepareImage+0xE6839A4,&mapOverride,1) &&
                EliteCopy(reinterpret_cast<void*>(actor+28488),&attached[0],8) &&
                EliteCopy(reinterpret_cast<void*>(actor+28504),&attached[1],8) &&
                EliteCopy(reinterpret_cast<void*>(actor+28520),&attached[2],8) &&
                EliteCopy(reinterpret_cast<void*>(actor+400),sceneRef,16);
            char line[1800]={};const auto& p=ctx->position;
            const int n=sprintf_s(line,"{\"traceVersion\":1,\"processId\":%lu,\"runTick\":%llu,\"tick\":%llu,\"thread\":%lu,\"ordinal\":%llu,\"phase\":\"%s\",\"slot\":%d,\"actor\":%llu,\"scene\":%llu,\"argumentObject\":%llu,\"requested\":[%d,%d],\"pointRequested\":[%d,%d],\"auxiliary\":%llu,\"result\":%llu,\"resultLow32\":%u,\"resultByte\":%u,\"callerRva\":%llu,\"before\":[%d,%d,%d],\"cacheContext\":%llu,\"cacheKnown\":%s,\"cache\":[%d,%d],\"bounds\":[%d,%d,%d,%d],\"flagsKnown\":%s,\"blocked27859\":%u,\"enabled720\":%u,\"mapOverride\":%u,\"attachments\":[%llu,%llu,%llu],\"actorSceneRef\":[%llu,%llu],\"readOnly\":true}\r\n",
                GetCurrentProcessId(),g_eliteTraceRunTick,GetTickCount64(),GetCurrentThreadId(),
                ctx->ordinal,phase,p.slot,static_cast<unsigned long long>(actor),static_cast<unsigned long long>(p.scene),
                static_cast<unsigned long long>(argument),x,y,ctx->x,ctx->y,static_cast<unsigned long long>(auxiliary),
                static_cast<unsigned long long>(result),static_cast<unsigned>(result),static_cast<unsigned>(result&255),
                static_cast<unsigned long long>(ctx->caller),p.xyz[0],p.xyz[1],p.xyz[2],
                static_cast<unsigned long long>(p.cacheContext),p.cacheKnown?"true":"false",p.cache[0],p.cache[1],
                p.bounds[0],p.bounds[1],p.bounds[2],p.bounds[3],flagsKnown?"true":"false",
                static_cast<unsigned>(blocked),static_cast<unsigned>(enabled),static_cast<unsigned>(mapOverride),
                static_cast<unsigned long long>(attached[0]),static_cast<unsigned long long>(attached[1]),static_cast<unsigned long long>(attached[2]),
                static_cast<unsigned long long>(sceneRef[0]),static_cast<unsigned long long>(sceneRef[1]));
            ++g_permissionRecords;
            HANDLE file=n>0?CreateFileW(g_permissionPath,FILE_APPEND_DATA,FILE_SHARE_READ|FILE_SHARE_WRITE,nullptr,OPEN_ALWAYS,FILE_ATTRIBUTE_NORMAL,nullptr):INVALID_HANDLE_VALUE;
            DWORD written=0;if(file!=INVALID_HANDLE_VALUE) {WriteFile(file,line,static_cast<DWORD>(n),&written,nullptr);CloseHandle(file);}
        }
    }
    __except(GetExceptionCode()==EXCEPTION_ACCESS_VIOLATION || GetExceptionCode()==EXCEPTION_IN_PAGE_ERROR?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {ElitePermissionFault();}
    ReleaseSRWLockExclusive(&g_permissionLock);
}
static __declspec(noinline) uintptr_t __fastcall ElitePermissionPointObserve(uintptr_t actor,int x,int y) {
    const DWORD entryError=GetLastError();
    const auto caller=reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage);
    auto context=ElitePermissionBefore(actor,x,y,caller);auto previous=g_permissionContext;
    if(context.position.slot>=0)g_permissionContext=&context;
    SetLastError(entryError);uintptr_t result=0;
    // Restore TLS even when native code raises an exception; never catch it.
    __try {result=g_permissionPoint(actor,x,y);}
    __finally {if(AbnormalTermination())g_permissionContext=previous;}
    const DWORD error=GetLastError();
    if(context.position.slot>=0)ElitePermissionRecord("point",actor,x,y,0,result);
    g_permissionContext=previous;
    SetLastError(error);return result;
}
static __declspec(noinline) unsigned char __fastcall ElitePermissionBirthObserve(uintptr_t actor,int x,int y,uintptr_t auxiliary) {
    const DWORD entryError=GetLastError();
    const auto caller=reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage);
    auto context=ElitePermissionBefore(actor,x,y,caller,true);auto previous=g_permissionContext;
    if(context.position.slot>=0)g_permissionContext=&context;
    SetLastError(entryError);unsigned char result=0;
    __try {result=g_catchupLand(actor,x,y,auxiliary);}
    __finally {if(AbnormalTermination())g_permissionContext=previous;}
    const DWORD error=GetLastError();
    if(context.position.slot>=0)ElitePermissionRecord("birth-point",actor,x,y,auxiliary,result);
    g_permissionContext=previous;SetLastError(error);return result;
}
static uintptr_t ElitePermissionForward3(ElitePermission3 native,const char* phase,uintptr_t actor,int x,int y) {
    const uintptr_t result=native(actor,x,y);const DWORD error=GetLastError();
    ElitePermissionRecord(phase,actor,x,y,0,result);SetLastError(error);return result;
}
static uintptr_t ElitePermissionForward4(ElitePermission4 native,const char* phase,uintptr_t actor,int x,int y,uintptr_t aux) {
    const uintptr_t result=native(actor,x,y,aux);const DWORD error=GetLastError();
    ElitePermissionRecord(phase,actor,x,y,aux,result);SetLastError(error);return result;
}
static uintptr_t __fastcall ElitePermissionGateObserve(uintptr_t a,int x,int y) {return ElitePermissionForward3(g_permissionGate,"map-gate",a,x,y);}
static uintptr_t __fastcall ElitePermissionGridObserve(uintptr_t a,int x,int y) {return ElitePermissionForward3(g_permissionGrid,"grid",a,x,y);}
static uintptr_t __fastcall ElitePermissionEffectsObserve(uintptr_t a,int x,int y,uintptr_t c) {return ElitePermissionForward4(g_permissionEffects,"effect-gate",a,x,y,c);}
static uintptr_t __fastcall ElitePermissionMotionObserve(uintptr_t a,int x,int y,uintptr_t c) {return ElitePermissionForward4(g_permissionMotion,"motion-gate",a,x,y,c);}
static uintptr_t __fastcall ElitePermissionCollisionObserve(uintptr_t a,int x,int y,uintptr_t c);
static uintptr_t __fastcall ElitePermissionLandingObserve(uintptr_t a,int x,int y,uintptr_t c) {return ElitePermissionForward4(g_permissionLanding,"landing",a,x,y,c);}
static uintptr_t __fastcall ElitePermissionDerivedObserve(uintptr_t a,int x,int y,uintptr_t c) {return ElitePermissionForward4(g_permissionDerived,"derived-landing",a,x,y,c);}
static bool EliteStartPermissionTrace(unsigned char* image,const wchar_t* path) {
    if(!g_positionReady || image!=g_prepareImage || !path || !path[0] || wcslen(path)>=ARRAYSIZE(g_permissionPath)) {g_permissionFailure="dependency-or-path";return false;}
    for(const auto& g:kElitePermissionGuards)if(!EliteReadable(image+g.rva,32) || memcmp(image+g.rva,g.bytes,32)) {g_permissionFailure="permission-bytes-mismatch";return false;}
    wcscpy_s(g_permissionPath,path);
    void* hooks[]={reinterpret_cast<void*>(ElitePermissionPointObserve),reinterpret_cast<void*>(ElitePermissionGateObserve),reinterpret_cast<void*>(ElitePermissionGridObserve),
        reinterpret_cast<void*>(ElitePermissionEffectsObserve),reinterpret_cast<void*>(ElitePermissionMotionObserve),reinterpret_cast<void*>(ElitePermissionCollisionObserve),
        reinterpret_cast<void*>(ElitePermissionLandingObserve),reinterpret_cast<void*>(ElitePermissionDerivedObserve)};
    void** originals[]={reinterpret_cast<void**>(&g_permissionPoint),reinterpret_cast<void**>(&g_permissionGate),reinterpret_cast<void**>(&g_permissionGrid),
        reinterpret_cast<void**>(&g_permissionEffects),reinterpret_cast<void**>(&g_permissionMotion),reinterpret_cast<void**>(&g_permissionCollision),
        reinterpret_cast<void**>(&g_permissionLanding),reinterpret_cast<void**>(&g_permissionDerived)};
    size_t created=0,enabled=0;
    for(;created<ARRAYSIZE(hooks);++created)if(MH_CreateHook(image+kElitePermissionGuards[created].rva,hooks[created],originals[created])!=MH_OK)break;
    if(created==ARRAYSIZE(hooks))for(;enabled<ARRAYSIZE(hooks);++enabled)if(MH_EnableHook(image+kElitePermissionGuards[enabled].rva)!=MH_OK)break;
    if(created!=ARRAYSIZE(hooks) || enabled!=ARRAYSIZE(hooks)) {
        for(size_t i=0;i<created;++i) {MH_DisableHook(image+kElitePermissionGuards[i].rva);MH_RemoveHook(image+kElitePermissionGuards[i].rva);}
        g_permissionFailure=created!=ARRAYSIZE(hooks)?"hook-create":"hook-enable";return false;
    }
    g_permissionReady=true;g_permissionFailure="ready";g_birthLandingObserver=ElitePermissionBirthObserve;return true;
}
