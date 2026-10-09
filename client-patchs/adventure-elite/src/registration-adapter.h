#pragma once
#include "registration-policy.h"
#include "condition-patch.h"
extern "C" void EliteConditionRelay();
static bool g_registrationReady;
static const char* g_registrationFailure="not-started";
static __declspec(thread) bool g_registrationPermitted;
static __declspec(thread) uintptr_t g_registrationDungeon,g_registrationManager;
static __declspec(thread) bool g_registrationReentry;
using EliteManagerActive=uintptr_t (__fastcall *)(uintptr_t);
static EliteManagerActive g_registrationActiveOriginal;
static void EliteRegistrationReset() {g_registrationPermitted=false;g_registrationDungeon=0;g_registrationManager=0;g_registrationReentry=false;}
// Matches 145EFAFB0's primary/fallback weak references; does not call the getter.
static bool EliteNativePlayerReference(uintptr_t* out) {
    uintptr_t pair[2]={};int strong=0;
    const unsigned int offsets[]={0xEF2CA88u,0xEF2CA70u};
    for(unsigned int offset:offsets) {
        if(!EliteCopy(g_prepareImage+offset,pair,sizeof(pair)))return false;
        if(pair[0] && pair[1] && !EliteCopy(reinterpret_cast<void*>(pair[0]+8),&strong,sizeof(strong)))return false;
        if(pair[0] && pair[1] && strong>0) {
            if(pair[1]<48)return false;
            out[0]=pair[0];out[1]=pair[1];return true;
        }
    }
    return false;
}
static bool EliteRegistrationScope(uintptr_t dungeon) {
    uintptr_t manager=0,currentDungeon=0,player[2]={},native[2]={},after[2]={},nativeAfter[2]={},vector[3]={};
    unsigned char active=1;EliteRegistrationFacts f={};
    f.ready=g_registrationReady;f.identity=EliteEntryIdentity();f.world=g_entryWorld!=0;
    f.dungeon=f.world && EliteCopy(reinterpret_cast<void*>(g_entryWorld+6792),&currentDungeon,8) && dungeon==currentDungeon;
    f.scene=g_entryScene && EliteCopy(reinterpret_cast<void*>(g_entryScene+88),vector,sizeof(vector)) && vector[0]<=vector[1] && vector[1]<=vector[2] && (vector[1]-vector[0])%24==0 && (vector[1]-vector[0])/24<=4096;
    bool playerRead=EliteCurrentPlayerReference(player);
    bool nativeRead=EliteNativePlayerReference(native);
    memcpy(g_registrationPlayer,player,sizeof(player));memcpy(g_registrationNativePlayer,native,sizeof(native));
    f.nativePlayer=playerRead && nativeRead && !memcmp(player,native,sizeof(player));
    EliteCopy(g_prepareImage+0xE638EF8,&manager,8);
    auto s=EliteReadCompanions(manager,EliteCopy,playerRead?player:nullptr);
    f.alive=static_cast<unsigned int>(s.alive);f.stable=s.available && s.consistent && playerRead && EliteCurrentPlayerReference(after) && !memcmp(player,after,sizeof(player)) && EliteNativePlayerReference(nativeAfter) && !memcmp(native,nativeAfter,sizeof(native));
    f.references=true;f.notRegistered=f.scene;
    for(size_t i=0;i<3;++i) if(s.strong[i]>0) {
        if(s.identityValid[i]!=1 || s.ownerMatchesCurrentPlayer[i]!=1 || s.kind[i]!=5 || s.controllerBound[i]!=1)f.references=false;
        for(size_t j=0;j<i;++j)if(s.strong[j]>0 && s.objectId[j]==s.objectId[i])f.references=false;
        uintptr_t weak[2]={};if(!manager || !EliteCopy(reinterpret_cast<void*>(manager+72+24*i),weak,sizeof(weak)))f.references=false;
        if(f.scene)for(uintptr_t p=vector[0];p<vector[1];p+=24) {uintptr_t ref[3]={};if(!EliteCopy(reinterpret_cast<void*>(p),ref,sizeof(ref)) || (ref[1]==weak[0] && ref[2]==weak[1]))f.notRegistered=false;}
    }
    uintptr_t afterVector[3]={};if(f.scene && (!EliteCopy(reinterpret_cast<void*>(g_entryScene+88),afterVector,sizeof(afterVector)) || memcmp(vector,afterVector,sizeof(vector))))f.stable=false;
    f.managerIdle=manager && EliteCopy(reinterpret_cast<void*>(manager+576),&active,1) && (active==0 || (active==1 && g_entryFreshDungeon && g_entryLoaderCaller==0x6D2A1E7));
    g_registrationChecks=f;
    return EliteRegistrationAllowed(f);
}
extern "C" unsigned char EliteConditionDecision(uintptr_t dungeon,uintptr_t caller) {
    DWORD incoming=GetLastError();unsigned char native=0,effective=0;
    bool read=EliteCopy(reinterpret_cast<void*>(dungeon+6264),&native,1);effective=native;
    EliteRegistrationReset();
    if(read && native==0 && caller==reinterpret_cast<uintptr_t>(g_prepareImage)+0x5B251AE && EliteRegistrationScope(dungeon)) {
        effective=1;g_registrationPermitted=true;g_registrationDungeon=dungeon;
        unsigned char active=0;
        EliteCopy(g_prepareImage+0xE638EF8,&g_registrationManager,8);
        g_registrationReentry=g_registrationManager && EliteCopy(reinterpret_cast<void*>(g_registrationManager+576),&active,1) && active==1 && g_entryFreshDungeon;
    }
    if(caller==reinterpret_cast<uintptr_t>(g_prepareImage)+0x5B251AE)EliteEntryRecord(g_registrationPermitted?"registration-scope-allowed":(native?"registration-scope-native":"registration-scope-rejected"),0x5B251AE,g_entryScene,effective);
    SetLastError(incoming);return effective;
}
static char EliteRegistrationChannel(uintptr_t caller,char result) {
    DWORD error=GetLastError();
    if(caller==0x5B251C3 && g_registrationPermitted && EliteEntryIdentity()) {
        uintptr_t dungeon=0;
        if(EliteCopy(reinterpret_cast<void*>(g_entryWorld+6792),&dungeon,8) && dungeon==g_registrationDungeon) {
            EliteEntryRecord("registration-channel-native",caller,g_entryScene,result);result=1;
        }
    }
    SetLastError(error);return result;
}
// Only the checked 145B22F50 consumer sees idle during a fresh dungeon entry.
// The physical flag, clone references and every other getter caller stay native.
static __declspec(noinline) uintptr_t __fastcall EliteRegistrationActiveObserve(uintptr_t manager) {
    const auto caller=reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage);
    const uintptr_t native=g_registrationActiveOriginal(manager);
    const DWORD nativeError=GetLastError();uintptr_t effective=native,currentManager=0;
    if(native==1 && caller==0x5B251D8 && g_registrationPermitted && g_registrationReentry && manager==g_registrationManager && g_entryFreshDungeon && g_entryLoaderCaller==0x6D2A1E7 && EliteEntryIdentity() && EliteCopy(g_prepareImage+0xE638EF8,&currentManager,8) && manager==currentManager && EliteRegistrationScope(g_registrationDungeon)) {
        effective=0;EliteEntryRecord("registration-active-reentry",caller,g_entryScene,static_cast<int>(native));
    }
    SetLastError(nativeError);return effective;
}
static bool EliteStartRegistration(unsigned char* image) {
    // Preparation already checked and owns 142E60CF0's original bytes/trampoline.
    // Checking its now-patched entry against the original caused v0.3.5 to fail.
    if(!g_entryReady || !g_prepareReady || !g_originalPredicate || g_preparePredicateRegistration) {g_registrationFailure="shared-predicate-unavailable";return false;}
    const unsigned char original[]={0x80,0xb8,0x78,0x18,0,0,0};
    if(!EliteReadable(image+0x5B251A9,7) || memcmp(image+0x5B251A9,original,7)) {g_registrationFailure="condition-site-mismatch";return false;}
    const unsigned char activeGuard[]={0x0f,0xb6,0x81,0x40,0x02,0,0,0xc3};
    const unsigned char freshGuard[]={0xc6,0x44,0x24,0x38,0x01,0xc6,0x44,0x24,0x30,0x00,0x4c,0x89,0x74,0x24,0x28,0xc6,0x44,0x24,0x20,0x01,0x45,0x33,0xc9,0x44,0x8b,0x86,0x28,0x05,0,0,0x8b,0x96,0x24,0x05,0,0,0x49,0x8b,0xcf,0xe8,0x69,0x8d,0xdf,0xfe};
    if(!EliteReadable(image+0x2E60CE0,sizeof(activeGuard)) || memcmp(image+0x2E60CE0,activeGuard,sizeof(activeGuard))) {g_registrationFailure="active-getter-mismatch";return false;}
    if(!EliteReadable(image+0x6D2A1BB,sizeof(freshGuard)) || memcmp(image+0x6D2A1BB,freshGuard,sizeof(freshGuard))) {g_registrationFailure="fresh-entry-caller-mismatch";return false;}
    auto slot=static_cast<unsigned char*>(AllocateBuffer(image+0x5B251A9));
    if(!slot) {g_registrationFailure="near-allocation";return false;}
    const unsigned char jump[]={0xff,0x25,0,0,0,0};memcpy(slot,jump,6);
    uintptr_t target=reinterpret_cast<uintptr_t>(EliteConditionRelay);memcpy(slot+6,&target,8);
    auto distance=reinterpret_cast<intptr_t>(slot)-reinterpret_cast<intptr_t>(image+0x5B251AE);
    if(distance<INT32_MIN || distance>INT32_MAX || !FlushInstructionCache(GetCurrentProcess(),slot,14)) {FreeBuffer(slot);g_registrationFailure="near-distance";return false;}
    unsigned char replacement[]={0xe8,0,0,0,0,0x90,0x90};int32_t displacement=static_cast<int32_t>(distance);memcpy(replacement+1,&displacement,4);
    if(MH_CreateHook(image+0x2E60CE0,reinterpret_cast<void*>(EliteRegistrationActiveObserve),reinterpret_cast<void**>(&g_registrationActiveOriginal))!=MH_OK) {FreeBuffer(slot);g_registrationFailure="active-hook-create";return false;}
    if(MH_EnableHook(image+0x2E60CE0)!=MH_OK) {MH_RemoveHook(image+0x2E60CE0);FreeBuffer(slot);g_registrationFailure="active-hook-enable";return false;}
    bool installed=false;
    if(!EliteConditionPatch(image+0x5B251A9,original,replacement,&installed)) {
        MH_DisableHook(image+0x2E60CE0);MH_RemoveHook(image+0x2E60CE0);
        if(!installed)FreeBuffer(slot);
        g_registrationFailure="condition-install-rejected";return false;
    }
    g_preparePredicateRegistration=EliteRegistrationChannel;
    g_entryBranchReset=EliteRegistrationReset;g_registrationReady=true;g_registrationFailure="ready";return true;
}
