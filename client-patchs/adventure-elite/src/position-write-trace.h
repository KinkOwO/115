#pragma once
#include "catchup-adapter.h"

// Native assembly uses RCX=component, EDX=axis, R8D=value, including the
// unconditional setter whose old IDB prototype omits R8D. Observe only; never
// replace the requested value, call AI/path APIs, or complete a rejected write.
using ElitePositionForce=uintptr_t (__fastcall *)(uintptr_t,int,int);
using ElitePositionConditional=void (__fastcall *)(uintptr_t,int,int);
static ElitePositionForce g_positionForceOriginal;
static ElitePositionConditional g_positionConditionalOriginal;
static bool g_positionReady,g_positionFault;
static const char* g_positionFailure="not-started";
static wchar_t g_positionPath[32768];
static SRWLOCK g_positionLock=SRWLOCK_INIT;
static unsigned g_positionRecords,g_positionSamples[3];
static uintptr_t g_positionScenes[3],g_positionActors[3];
static __declspec(thread) bool g_positionBusy;
static constexpr unsigned char kElitePositionForceBytes[]={0x40,0x56,0x57,0x41,0x54,0x41,0x56,0x41,0x57,0x48,0x83,0xec,0x30,0x48,0xc7,0x44,0x24,0x20,0xfe,0xff,0xff,0xff,0x48,0x89,0x5c,0x24,0x68,0x48,0x89,0x6c,0x24,0x70};
static constexpr unsigned char kElitePositionConditionalBytes[]={0x48,0x89,0x5c,0x24,0x18,0x55,0x57,0x41,0x56,0x48,0x83,0xec,0x30,0x80,0xb9,0x7c,0x02,0x00,0x00,0x00,0x45,0x8b,0xf0,0x8b,0xea,0xc6,0x81,0x38,0x01,0x00,0x00,0x01};
struct ElitePositionObservation {
    int slot=-1,xyz[3]={},membership=-1;
    uintptr_t actor=0,component=0,scene=0,weak[2]={};
    uintptr_t cacheContext=0;int cache[2]={},bounds[4]={};bool cacheKnown=false;
    void* stack[12]={};USHORT stackCount=0;
};
static bool ElitePositionIdentity(uintptr_t component,ElitePositionObservation* out) {
    if(!g_positionReady || g_positionFault || !component || !EliteEntryIdentity())return false;
    // Fast rejection before walking the manager/scene on unrelated actor writes.
    bool anchored=false;
    for(const auto& key:g_catchupAnchors) {
        uintptr_t p=0;
        if(key.alias>=48 && EliteCopy(reinterpret_cast<void*>(key.alias-48+368),&p,8) && p==component)anchored=true;
    }
    if(!anchored)return false;
    uintptr_t manager=0,player[2]={},native[2]={},world=0;
    if(!EliteCopy(g_prepareImage+0xE638EF8,&manager,8) || !manager ||
       !EliteCurrentPlayerReference(player) || !EliteNativePlayerReference(native) || memcmp(player,native,16) ||
       !EliteCopy(g_prepareImage+0xE66C090,&world,8) || !world)return false;
    auto s=EliteReadCompanions(manager,EliteCopy,player);
    if(!s.available || !s.consistent)return false;
    for(int i=0;i<3;++i) {
        const auto& key=g_catchupAnchors[i];uintptr_t weak[2]={},p=0;
        if(s.strong[i]<=0 || s.kind[i]!=5 || s.identityValid[i]!=1 || s.ownerMatchesCurrentPlayer[i]!=1 ||
           s.controllerBound[i]!=1 || s.controllerId[i]!=65535 || s.controllerWire[i]!=65535 ||
           s.ownerControllerId[i]!=static_cast<int>(g_lifecycleOwner) ||
           key.world!=world || key.playerControl!=player[0] || key.playerAlias!=player[1] || key.objectId!=s.objectId[i] ||
           !EliteCopy(reinterpret_cast<void*>(manager+72+24*i),weak,16) || weak[1]<48 ||
           weak[0]!=key.control || weak[1]!=key.alias || !key.scene ||
           !EliteCopy(reinterpret_cast<void*>(weak[1]-48+368),&p,8) || p!=component)continue;
        for(int j=0;j<3;++j)if(j!=i && s.strong[j]>0 && s.objectId[j]==s.objectId[i])return false;
        out->slot=i;out->actor=weak[1]-48;out->component=component;out->scene=key.scene;
        memcpy(out->weak,weak,16);
        // Use the existing original coordinate getter, bypassing its sampling
        // hook. These three reads cannot run registration recovery or A*.
        for(int axis=0;axis<3;++axis)out->xyz[axis]=static_cast<int>(g_catchupOriginal(component,axis));
        out->cacheKnown=EliteCopy(reinterpret_cast<void*>(out->actor+100888),&out->cacheContext,8) && out->cacheContext &&
            EliteCopy(reinterpret_cast<void*>(out->cacheContext+5408),out->cache,8);
        //145CF68D0 consumes these four actor-local bounds. Read only; do not
        // invoke its map/permission dependencies for diagnostic purposes.
        EliteCopy(reinterpret_cast<void*>(out->actor+74372),out->bounds,16);
        return true;
    }
    return false;
}
static void ElitePositionFault() {
    g_positionFault=true;g_positionFailure="runtime-memory-fault";
    EliteCatchupRecord("position-write-runtime-memory-fault",-1,0,0,0,0,0);
}
static ElitePositionObservation ElitePositionBefore(uintptr_t component,int axis,int value) {
    ElitePositionObservation out={};
    if(axis<0 || axis>2 || g_positionBusy || GetCurrentThreadId()!=g_lifecycleThread)return out;
    g_positionBusy=true;
    __try {
        if(ElitePositionIdentity(component,&out)) {
            // Avoid walking the whole scene twice on every normal AI step.
            // Unknown pre-membership remains -1 if a nonzero request unexpectedly
            // becomes zero after the initial window; its stack/XYZ still survive.
            if(g_positionScenes[out.slot]!=out.scene || g_positionActors[out.slot]!=out.actor ||
               g_positionSamples[out.slot]<96 || value==0)out.membership=EliteCatchupMembership(out.scene,out.weak);
            out.stackCount=CaptureStackBackTrace(1,12,out.stack,nullptr);
        }
    }
    __except(GetExceptionCode()==EXCEPTION_ACCESS_VIOLATION || GetExceptionCode()==EXCEPTION_IN_PAGE_ERROR?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {out.slot=-1;ElitePositionFault();}
    g_positionBusy=false;return out;
}
static void ElitePositionAfter(const ElitePositionObservation* before,const char* setter,int axis,int value,uintptr_t caller) {
    if(before->slot<0 || g_positionBusy || g_positionFault)return;
    g_positionBusy=true;
    __try {
        ElitePositionObservation after={};
        if(ElitePositionIdentity(before->component,&after) && after.slot==before->slot && after.actor==before->actor &&
           after.scene==before->scene && !memcmp(after.weak,before->weak,16)) {
            const int slot=before->slot;
            if(g_positionScenes[slot]!=before->scene || g_positionActors[slot]!=before->actor) {
                g_positionScenes[slot]=before->scene;g_positionActors[slot]=before->actor;g_positionSamples[slot]=0;
            }
            const bool zeroTransition=before->xyz[axis]!=0 && after.xyz[axis]==0;
            if((g_positionSamples[slot]++<96 || zeroTransition) && TryAcquireSRWLockExclusive(&g_positionLock)) {
                if(g_positionRecords<4096) {
                    after.membership=EliteCatchupMembership(after.scene,after.weak);
                    ++g_positionRecords;char stack[400]={};size_t used=0;
                    for(USHORT i=0;i<before->stackCount;++i) {
                        const uintptr_t a=reinterpret_cast<uintptr_t>(before->stack[i]),base=reinterpret_cast<uintptr_t>(g_prepareImage);
                        const uintptr_t rva=a>=base && a-base<0x30000000?a-base:0;
                        int n=sprintf_s(stack+used,sizeof(stack)-used,"%s%llu",i?",":"",static_cast<unsigned long long>(rva));if(n>0)used+=n;
                    }
                    char line[1900]={};int length=sprintf_s(line,"{\"traceVersion\":1,\"processId\":%lu,\"runTick\":%llu,\"tick\":%llu,\"thread\":%lu,\"setter\":\"%s\",\"slot\":%d,\"actor\":%llu,\"component\":%llu,\"scene\":%llu,\"axis\":%d,\"argument\":%d,\"before\":[%d,%d,%d],\"after\":[%d,%d,%d],\"membershipBefore\":%d,\"membershipAfter\":%d,\"callerRva\":%llu,\"stackRvas\":[%s],\"zeroTransition\":%s,\"readOnly\":true,\"cacheContext\":%llu,\"cacheKnown\":%s,\"cacheBefore\":[%d,%d],\"cacheAfter\":[%d,%d],\"bounds\":[%d,%d,%d,%d]}\r\n",
                        GetCurrentProcessId(),g_eliteTraceRunTick,GetTickCount64(),GetCurrentThreadId(),setter,slot,
                        static_cast<unsigned long long>(before->actor),static_cast<unsigned long long>(before->component),static_cast<unsigned long long>(before->scene),axis,value,
                        before->xyz[0],before->xyz[1],before->xyz[2],after.xyz[0],after.xyz[1],after.xyz[2],before->membership,after.membership,
                        static_cast<unsigned long long>(caller),stack,zeroTransition?"true":"false",static_cast<unsigned long long>(before->cacheContext),before->cacheKnown && after.cacheKnown && before->cacheContext==after.cacheContext?"true":"false",before->cache[0],before->cache[1],after.cache[0],after.cache[1],before->bounds[0],before->bounds[1],before->bounds[2],before->bounds[3]);
                    HANDLE f=length>0?CreateFileW(g_positionPath,FILE_APPEND_DATA,FILE_SHARE_READ|FILE_SHARE_WRITE,nullptr,OPEN_ALWAYS,FILE_ATTRIBUTE_NORMAL,nullptr):INVALID_HANDLE_VALUE;
                    DWORD written=0;if(f!=INVALID_HANDLE_VALUE) {WriteFile(f,line,static_cast<DWORD>(length),&written,nullptr);CloseHandle(f);}
                }
                ReleaseSRWLockExclusive(&g_positionLock);
            }
        }
    }
    __except(GetExceptionCode()==EXCEPTION_ACCESS_VIOLATION || GetExceptionCode()==EXCEPTION_IN_PAGE_ERROR?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {ElitePositionFault();}
    g_positionBusy=false;
}
static __declspec(noinline) uintptr_t __fastcall ElitePositionForceObserve(uintptr_t component,int axis,int value) {
    const DWORD entryError=GetLastError();const uintptr_t caller=reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage);
    const auto before=ElitePositionBefore(component,axis,value);SetLastError(entryError);
    const uintptr_t result=g_positionForceOriginal(component,axis,value);const DWORD error=GetLastError();
    ElitePositionAfter(&before,"unconditional",axis,value,caller);SetLastError(error);return result;
}
static __declspec(noinline) void __fastcall ElitePositionConditionalObserve(uintptr_t component,int axis,int value) {
    const DWORD entryError=GetLastError();const uintptr_t caller=reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage);
    const auto before=ElitePositionBefore(component,axis,value);SetLastError(entryError);
    g_positionConditionalOriginal(component,axis,value);const DWORD error=GetLastError();
    ElitePositionAfter(&before,"conditional",axis,value,caller);SetLastError(error);
}
static bool EliteStartPositionWrites(unsigned char* image,const wchar_t* path) {
    if(!g_catchupReady || image!=g_prepareImage || !path || !path[0] || wcslen(path)>=ARRAYSIZE(g_positionPath)) {g_positionFailure="dependency-or-path";return false;}
    if(!EliteReadable(image+0x5E0CD40,sizeof(kElitePositionForceBytes)) || memcmp(image+0x5E0CD40,kElitePositionForceBytes,sizeof(kElitePositionForceBytes)) ||
       !EliteReadable(image+0x5E0EC50,sizeof(kElitePositionConditionalBytes)) || memcmp(image+0x5E0EC50,kElitePositionConditionalBytes,sizeof(kElitePositionConditionalBytes))) {g_positionFailure="setter-bytes-mismatch";return false;}
    wcscpy_s(g_positionPath,path);
    if(MH_CreateHook(image+0x5E0CD40,reinterpret_cast<void*>(ElitePositionForceObserve),reinterpret_cast<void**>(&g_positionForceOriginal))!=MH_OK) {g_positionFailure="force-hook-create";return false;}
    if(MH_CreateHook(image+0x5E0EC50,reinterpret_cast<void*>(ElitePositionConditionalObserve),reinterpret_cast<void**>(&g_positionConditionalOriginal))!=MH_OK) {MH_RemoveHook(image+0x5E0CD40);g_positionFailure="conditional-hook-create";return false;}
    if(MH_EnableHook(image+0x5E0CD40)!=MH_OK || MH_EnableHook(image+0x5E0EC50)!=MH_OK) {
        MH_DisableHook(image+0x5E0CD40);MH_DisableHook(image+0x5E0EC50);MH_RemoveHook(image+0x5E0CD40);MH_RemoveHook(image+0x5E0EC50);g_positionFailure="hook-enable";return false;
    }
    g_positionReady=true;g_positionFailure="ready";return true;
}
