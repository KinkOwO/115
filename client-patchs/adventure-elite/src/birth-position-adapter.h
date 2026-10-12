#pragma once
#include "catchup-adapter.h"

// 145B2531F calls virtual408, returning at145B25325. Its native conditional
// position setter145B98210 can leave a registered APC at its old coordinates.
// Live0.3.19: native birth succeeds, then145C76FB0 restores context5408/5412
// (both zero). Synchronize that stale cache through native virtual5688 only
// after the exact successful birth. Landing correction uses native gates and
// the same conditional setter only after loading has completed.
using EliteBirthSet=void (__fastcall *)(uintptr_t,int,int,int);
using EliteBirthCache=void (__fastcall *)(uintptr_t);
static EliteBirthSet g_birthOriginal;
static EliteBirthCache g_birthCache;
static bool g_birthReady,g_birthFault,g_birthLandingEnabled;
// Optional transparent diagnostic scope; it must forward exactly one native
// virtual632 call. It is installed only after permission observers are ready.
static EliteCatchupLand g_birthLandingObserver;
static unsigned char EliteBirthLand(uintptr_t actor,int x,int y) {
    return g_birthLandingObserver?g_birthLandingObserver(actor,x,y,0):g_catchupLand(actor,x,y,0);
}
static const char* g_birthFailure="not-started";
static constexpr unsigned char kEliteBirthCall[]={0x48,0x8b,0x06,0x45,0x33,0xc9,0x44,0x8b,0x45,0xa0,0x48,0x8b,0xce,0xff,0x90,0x98,0x01,0x00,0x00};
static constexpr unsigned char kEliteBirthCacheBytes[]={
    0x40,0x53,0x48,0x83,0xec,0x20,0x48,0x83,0xb9,0x18,0x8a,0x01,0x00,0x00,0x48,0x8b,0xd9,0x74,0x27,
    0xe8,0x38,0x19,0xf2,0xff,0x48,0x8b,0x93,0x18,0x8a,0x01,0x00,0x48,0x8b,0xcb,0x89,0x82,0x20,0x15,0x00,0x00,
    0xe8,0xe3,0x19,0xf2,0xff,0x48,0x8b,0x93,0x18,0x8a,0x01,0x00,0x89,0x82,0x24,0x15,0x00,0x00,
    0x48,0x8b,0xcb,0x48,0x83,0xc4,0x20,0x5b,0xe9,0xb9,0x4c,0xfa,0xff
};
static constexpr unsigned char kEliteBirthCacheTail[]={0xc7,0x81,0x5c,0x62,0x00,0x00,0xff,0xff,0xff,0xff,0xc3};
static constexpr unsigned char kEliteBirthSetter[]={
    0x48,0x89,0x5c,0x24,0x08,0x48,0x89,0x74,0x24,0x10,0x57,0x48,
    0x83,0xec,0x20,0x41,0x8b,0xd8,0x48,0x8b,0xf9,0x48,0x8b,0x89,
    0x70,0x01,0x00,0x00,0x44,0x8b,0xc2,0x33,0xd2,0x41,0x8b,0xf1,
    0xe8,0x17,0x6a,0x27,0x00,0x48,0x8b,0x8f,0x70,0x01,0x00,0x00,
    0x44,0x8b,0xc3,0xba,0x01,0x00,0x00,0x00,0xe8,0x03,0x6a,0x27,
    0x00,0x48,0x8b,0x8f,0x70,0x01,0x00,0x00,0x44,0x8b,0xc6,0xba,
    0x02,0x00,0x00,0x00,0x48,0x8b,0x5c,0x24,0x30,0x48,0x8b,0x74,
    0x24,0x38,0x48,0x83,0xc4,0x20,0x5f,0xe9,0xe0,0x69,0x27,0x00
};
static int EliteBirthSlot(uintptr_t actor,uintptr_t caller,int z) {
    if(!g_birthReady || !g_catchupReady || caller!=0x5B25325 || z!=0 ||
       !EliteEntryIdentity() || !g_entryWorld || !g_entryScene || !g_registrationPermitted ||
       !g_entryFreshDungeon || g_entryLoaderCaller!=0x6D2A1E7 || !EliteCatchupActor(actor,true))return -1;
    uintptr_t dungeon=0,manager=0,player[2]={},native[2]={},vtable=0,setter=0;
    if(!EliteCopy(reinterpret_cast<void*>(g_entryWorld+6792),&dungeon,8) || dungeon!=g_registrationDungeon ||
       !EliteCopy(g_prepareImage+0xE638EF8,&manager,8) || manager!=g_registrationManager || !manager ||
       !EliteCurrentPlayerReference(player) || !EliteNativePlayerReference(native) || memcmp(player,native,16) ||
       !EliteCopy(reinterpret_cast<void*>(actor),&vtable,8) ||
       !EliteCopy(reinterpret_cast<void*>(vtable+408),&setter,8) || setter!=reinterpret_cast<uintptr_t>(g_prepareImage+0x5B98210))return -1;
    auto s=EliteReadCompanions(manager,EliteCopy,player);
    if(!s.available || !s.consistent)return -1;
    for(size_t i=0;i<3;++i) {
        uintptr_t weak[2]={};
        if(s.strong[i]<=0 || s.kind[i]!=5 || s.identityValid[i]!=1 || s.ownerMatchesCurrentPlayer[i]!=1 ||
           s.controllerBound[i]!=1 || s.controllerId[i]!=65535 || s.controllerWire[i]!=65535 ||
           s.ownerControllerId[i]!=static_cast<int>(g_lifecycleOwner) ||
           !EliteCopy(reinterpret_cast<void*>(manager+72+24*i),weak,16) || weak[1]<48 || weak[1]-48!=actor ||
           EliteCatchupMembership(g_entryScene,weak)!=1)continue;
        for(size_t j=0;j<3;++j)if(i!=j && s.strong[j]>0 && s.objectId[j]==s.objectId[i])return -1;
        return static_cast<int>(i);
    }
    return -1;
}
struct EliteBirthPending {
    EliteCatchupKey key;
    uintptr_t manager=0,context=0;
    int x=0,y=0;
    bool active=false,waiting=false;
    const char* waitReason=nullptr;
};
static EliteBirthPending g_birthPending[3];
static bool EliteBirthCacheContext(uintptr_t actor,uintptr_t& context,int* cached) {
    uintptr_t vtable=0,setter=0,cacheSetter=0;
    return EliteCopy(reinterpret_cast<void*>(actor+100888),&context,8) && context &&
        EliteCopy(reinterpret_cast<void*>(context+5408),cached,8) &&
        EliteCopy(reinterpret_cast<void*>(actor),&vtable,8) &&
        EliteCopy(reinterpret_cast<void*>(vtable+408),&setter,8) && setter==reinterpret_cast<uintptr_t>(g_prepareImage+0x5B98210) &&
        EliteCopy(reinterpret_cast<void*>(vtable+5688),&cacheSetter,8) && cacheSetter==reinterpret_cast<uintptr_t>(g_birthCache);
}
static void EliteBirthComplete(uintptr_t actor,uintptr_t caller,int x,int y,int z) {
    const int slot=EliteBirthSlot(actor,caller,z);if(slot<0)return;
    g_birthPending[slot]={};
    const int ax=g_catchupX(actor),ay=g_catchupY(actor),az=g_catchupZ(actor);
    if(ax!=x || ay!=y || az!=z) {
        EliteCatchupRecord("birth-native-position-mismatch",slot,g_entryScene,0,0,ax,ay,x,y,ax,ay,0,g_entryWorld,-1,-1,INT_MIN,az,az);return;
    }
    EliteCatchupRecord("birth-native-position-ok",slot,g_entryScene,0,0,ax,ay,x,y,ax,ay,0,g_entryWorld,-1,-1,INT_MIN,az,az);
    uintptr_t context=0;int cached[2]={},after[2]={};
    if(!EliteBirthCacheContext(actor,context,cached)) {
        EliteCatchupRecord("birth-cache-unavailable",slot,g_entryScene,0,0,ax,ay);return;
    }
    // Cache synchronization must not depend on owner eligibility or landing.
    // PID20672: an early owner refusal skipped this, restoring each APC to0/0.
    const bool stale=cached[0]==0 && cached[1]==0 && (x!=0 || y!=0);
    if(stale)g_birthCache(actor);
    if(!EliteCopy(reinterpret_cast<void*>(context+5408),after,8))return;
    const bool stable=g_catchupX(actor)==ax && g_catchupY(actor)==ay && g_catchupZ(actor)==az;
    const bool synced=stable && after[0]==x && after[1]==y;
    EliteCatchupRecord(stale?(synced?"birth-cache-synchronized":"birth-cache-sync-failed"):"birth-cache-preserved",
        slot,g_entryScene,0,0,ax,ay,x,y,ax,ay,0,g_entryWorld,-1,-1,INT_MIN,az,az,context,cached[0],cached[1],after[0],after[1]);
    if(!g_birthLandingEnabled || !stale || !synced)return;
    // No native collision calls during loader mode2. Queue only the exact
    // anchored successful zero-cache birth; finish after dungeon mode3.
    const auto anchor=g_catchupAnchors[slot];uintptr_t player[2]={};
    if(anchor.scene!=g_entryScene || anchor.alias<48 || anchor.alias-48!=actor ||
       !EliteCurrentPlayerReference(player) || anchor.playerControl!=player[0] || anchor.playerAlias!=player[1] ||
       !anchor.world || !anchor.control) {
        EliteCatchupRecord("birth-landing-anchor-unavailable",slot,g_entryScene,0,0,ax,ay,x,y,ax,ay,0,g_entryWorld,-1,-1,INT_MIN,az,az);return;
    }
    const auto snapshot=EliteReadCompanions(g_registrationManager,EliteCopy,player);
    if(!snapshot.available || !snapshot.consistent || snapshot.objectId[slot]!=anchor.objectId)return;
    auto key=anchor;key.room=static_cast<uint32_t>(g_eliteRoomSerial);
    g_birthPending[slot]={key,g_registrationManager,context,x,y,true,false};
    EliteCatchupRecord("birth-landing-deferred",slot,key.scene,0,0,ax,ay,x,y,ax,ay,key.world,g_entryWorld,-1,-1,INT_MIN,az,az,context,cached[0],cached[1],after[0],after[1]);
}
static bool EliteBirthWait(int slot,const EliteCatchupKey& key,int px,int py,int pz,const char* reason) {
    auto& pending=g_birthPending[slot];
    if(!pending.waitReason || strcmp(pending.waitReason,reason))
        EliteCatchupRecord(reason,slot,key.scene,px,py,0,0,pending.x,pending.y,0,0,key.world,0,-1,-1,pz);
    pending.waitReason=reason;return true;
}
static bool EliteBirthSample(int slot,const EliteCatchupKey& key,int px,int py,int pz) {
    if(slot<0 || slot>=3 || !g_birthReady || g_birthFault || !g_birthLandingEnabled || !g_birthPending[slot].active)return false;
    const auto pending=g_birthPending[slot];
    if(!EliteCatchupSame(pending.key,key)) {
        g_birthPending[slot]={};
        EliteCatchupRecord("birth-landing-generation-discarded",slot,key.scene,px,py,pending.x,pending.y,0,0,0,0,key.world);return false;
    }
    uintptr_t player[2]={key.playerControl,key.playerAlias},weak[2]={key.control,key.alias};
    auto anchor=key;anchor.room=0;
    // Validated by the existing sample, then rechecked here before consuming
    // retained identity values. No dereference of a stale captured actor.
    uintptr_t manager=0;
    if(!g_catchupBusy || GetCurrentThreadId()!=g_lifecycleThread || !g_lifecycleArmed)return false;
    if(!EliteCatchupSame(anchor,g_catchupAnchors[slot]) || !EliteCatchupContext(key.world,key.scene,player) ||
       key.alias<48 || key.playerAlias<48)return EliteBirthWait(slot,key,px,py,pz,"birth-landing-context-wait");
    if(!EliteCopy(g_prepareImage+0xE638EF8,&manager,8) || manager!=pending.manager)
        return EliteBirthWait(slot,key,px,py,pz,"birth-landing-manager-wait");
    if(EliteCatchupMembership(key.scene,weak)!=1)return EliteBirthWait(slot,key,px,py,pz,"birth-landing-apc-scene-wait");
    if(EliteCatchupMembership(key.scene,player)!=1)return EliteBirthWait(slot,key,px,py,pz,"birth-landing-owner-scene-wait");
    const uintptr_t actor=key.alias-48;
    if(!EliteCatchupActor(actor,true) || !EliteCatchupActor(key.playerAlias-48,false))return EliteBirthWait(slot,key,px,py,pz,"birth-landing-actor-wait");
    const int ax=g_catchupX(actor),ay=g_catchupY(actor),az=g_catchupZ(actor);
    if(pz!=0 || az!=0) {
        if(!pending.waiting)EliteCatchupRecord("birth-landing-ground-wait",slot,key.scene,px,py,ax,ay,pending.x,pending.y,ax,ay,key.world,0,-1,-1,pz,az,az);
        g_birthPending[slot].waiting=true;return true;
    }
    // One native evaluation per birth generation, even if a gate refuses.
    g_birthPending[slot]={};
    uintptr_t sceneRef[2]={},context=0;int strong=0,cached[2]={},after[2]={};
    if(!EliteBirthCacheContext(actor,context,cached) || context!=pending.context ||
       !EliteCopy(reinterpret_cast<void*>(actor+400),sceneRef,16) || !sceneRef[0] || sceneRef[1]!=key.scene ||
       !EliteCopy(reinterpret_cast<void*>(sceneRef[0]+8),&strong,4) || strong<=0) {
        EliteCatchupRecord("birth-landing-scene-unavailable",slot,key.scene,px,py,ax,ay,pending.x,pending.y,ax,ay,key.world,0,-1,-1,pz,az,az);return true;
    }
    if(ax!=pending.x || ay!=pending.y || cached[0]!=pending.x || cached[1]!=pending.y) {
        EliteCatchupRecord("birth-landing-native-superseded",slot,key.scene,px,py,ax,ay,pending.x,pending.y,ax,ay,key.world,0,-1,-1,pz,az,az,context,cached[0],cached[1],cached[0],cached[1]);return true;
    }
    if(EliteBirthLand(actor,ax,ay)) {
        EliteCatchupRecord("birth-landing-native-accepted",slot,key.scene,px,py,ax,ay,ax,ay,ax,ay,key.world,0,-1,-1,pz,az,az,context,cached[0],cached[1],cached[0],cached[1],1,-1,1);return true;
    }
    if(!EliteBirthLand(actor,px,py)) {
        EliteCatchupRecord("birth-landing-owner-rejected",slot,key.scene,px,py,ax,ay,px,py,ax,ay,key.world,0,-1,-1,pz,az,az,context,cached[0],cached[1],cached[0],cached[1],0,0,-1);return true;
    }
    g_birthOriginal(actor,px,py,0);
    const int fx=g_catchupX(actor),fy=g_catchupY(actor),fz=g_catchupZ(actor);
    const bool accepted=fx==px && fy==py && fz==0 && EliteBirthLand(actor,px,py);
    if(!accepted) {
        g_birthOriginal(actor,ax,ay,az);
        EliteCatchupRecord("birth-landing-correction-failed",slot,key.scene,px,py,ax,ay,px,py,fx,fy,key.world,0,-1,-1,pz,az,fz,context,cached[0],cached[1],cached[0],cached[1],0,1,0);
        EliteCatchupRecord("birth-landing-correction-restored",slot,key.scene,px,py,fx,fy,ax,ay,g_catchupX(actor),g_catchupY(actor),key.world,0,-1,-1,pz,fz,g_catchupZ(actor));return true;
    }
    g_birthCache(actor);
    const bool copied=EliteCopy(reinterpret_cast<void*>(context+5408),after,8);
    const bool synced=copied && after[0]==px && after[1]==py && g_catchupX(actor)==px && g_catchupY(actor)==py && g_catchupZ(actor)==0;
    EliteCatchupRecord(synced?"birth-landing-owner-corrected":"birth-landing-cache-failed",slot,key.scene,px,py,ax,ay,px,py,g_catchupX(actor),g_catchupY(actor),key.world,0,-1,-1,pz,az,g_catchupZ(actor),context,cached[0],cached[1],after[0],after[1],0,1,1);return true;
}
static __declspec(noinline) void __fastcall EliteBirthObserve(uintptr_t actor,int x,int y,int z) {
    const auto caller=reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage);
    // Forward every native call, argument, exception and last-error unchanged.
    g_birthOriginal(actor,x,y,z);const DWORD nativeError=GetLastError();
    if(g_birthReady && !g_birthFault) {
        const bool busy=g_catchupBusy;g_catchupBusy=true;
        __try {
        __try {EliteBirthComplete(actor,caller,x,y,z);}
        __except(GetExceptionCode()==EXCEPTION_ACCESS_VIOLATION || GetExceptionCode()==EXCEPTION_IN_PAGE_ERROR?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {
            g_birthFault=true;g_birthFailure="runtime-memory-fault";
            EliteCatchupRecord("birth-runtime-memory-fault",-1,g_entryScene,0,0,0,0);
        }
        } __finally {g_catchupBusy=busy;}
    }
    SetLastError(nativeError);
}
static bool EliteStartBirthPosition(unsigned char* image) {
    if(!g_catchupReady || image!=g_prepareImage) {g_birthFailure="catchup-dependency";return false;}
    if(!EliteReadable(image+0x5B25312,sizeof(kEliteBirthCall)) || memcmp(image+0x5B25312,kEliteBirthCall,sizeof(kEliteBirthCall)) ||
       !EliteReadable(image+0x5B98210,sizeof(kEliteBirthSetter)) || memcmp(image+0x5B98210,kEliteBirthSetter,sizeof(kEliteBirthSetter)) ||
       !EliteReadable(image+0x5C6AE80,sizeof(kEliteBirthCacheBytes)) || memcmp(image+0x5C6AE80,kEliteBirthCacheBytes,sizeof(kEliteBirthCacheBytes)) ||
       !EliteReadable(image+0x5C0FB80,sizeof(kEliteBirthCacheTail)) || memcmp(image+0x5C0FB80,kEliteBirthCacheTail,sizeof(kEliteBirthCacheTail))) {
        g_birthFailure="birth-bytes-mismatch";return false;
    }
    if(MH_CreateHook(image+0x5B98210,reinterpret_cast<void*>(EliteBirthObserve),reinterpret_cast<void**>(&g_birthOriginal))!=MH_OK) {g_birthFailure="hook-create";return false;}
    if(MH_EnableHook(image+0x5B98210)!=MH_OK) {MH_RemoveHook(image+0x5B98210);g_birthFailure="hook-enable";return false;}
    g_birthCache=reinterpret_cast<EliteBirthCache>(image+0x5C6AE80);
    g_birthReady=true;g_birthLandingEnabled=true;g_catchupBirthSample=EliteBirthSample;g_birthFailure="ready";return true;
}
