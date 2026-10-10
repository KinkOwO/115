#define _CRT_SECURE_NO_WARNINGS
#include "../../elite-companion-pilot/src/inline_hook.h"
#include "native_evidence.h"
#include <stdio.h>
#include <stdlib.h>

using CycleFn=void(__fastcall*)(uintptr_t);
using ResolveFn=uintptr_t(__fastcall*)(uintptr_t,int,int,unsigned char);
using LookupFn=uintptr_t(__fastcall*)(uintptr_t,int);
using SkinFn=uint32_t(__fastcall*)(uintptr_t);
using ShapeFn=uintptr_t(__fastcall*)(uintptr_t);
static CycleFn gCycle,gExpansion;
static ResolveFn gResolve;
static LookupFn gLookup;
static SkinFn gSkin;
static ShapeFn gShape;
static InlineHook gHooks[4];
static HMODULE gSelf;
static uintptr_t gBase;
static FILE* gLog;
static volatile LONG gStarted=0,gEnabled=0,gObserved=0;
static thread_local uintptr_t gVisualActor=0;

static bool HasShape(uintptr_t actor) {
    if(!gEnabled||!Readable((void*)actor,0x6050))return false;
    // Use the native eligibility getter (including its scene restrictions).
    // Do not hide anything while the native synthetic weapon is absent.
    __try {return gSkin(actor)!=0 && gShape(actor)!=0;}
    __except(EXCEPTION_EXECUTE_HANDLER){return false;}
}
struct VisualScope {
    uintptr_t previous;
    explicit VisualScope(uintptr_t actor):previous(gVisualActor) {
        gVisualActor=HasShape(actor)?actor:0;
    }
    ~VisualScope(){gVisualActor=previous;}
};
static bool Hidden(uintptr_t actor,int slot) {
    return slot==10 && actor!=0 && actor==gVisualActor;
}
static uintptr_t __fastcall Resolve(uintptr_t actor,int slot,int flags,unsigned char original) {
    if(Hidden(actor,slot))return 0;
    return gResolve(actor,slot,flags,original);
}
static uintptr_t __fastcall Lookup(uintptr_t actor,int slot) {
    if(Hidden(actor,slot))return 0;
    return gLookup(actor,slot);
}
static void __fastcall Cycle(uintptr_t actor) {
    VisualScope scope(actor);
    if(gVisualActor && !InterlockedCompareExchange(&gObserved,1,0) && gLog) {
        fputs("OBSERVED native transmog active; slot10 excluded only inside animation scopes\n",gLog);
        fflush(gLog);
    }
    gCycle(actor);
}
static void __fastcall Expansion(uintptr_t actor) {
    VisualScope scope(actor);
    gExpansion(actor);
}

template<size_t N> static bool Match(uintptr_t rva,const unsigned char(&bytes)[N]) {
    return Readable((void*)(gBase+rva),N) && !memcmp((void*)(gBase+rva),bytes,N);
}
static void Log(const char* text) {if(gLog){fputs(text,gLog);fflush(gLog);}}
extern "C" __declspec(dllexport) const char* WINAPI ModName(){return "Weapon transmog animation priority r1";}
extern "C" __declspec(dllexport) DWORD WINAPI ModStart(){
    if(InterlockedCompareExchange(&gStarted,1,0))return 0;
    wchar_t path[1024]={},opt[64]={};
    DWORD n=GetModuleFileNameW(gSelf,path,_countof(path));
    if(!n||n>=_countof(path))return 1;
    auto slash=wcsrchr(path,L'\\');if(!slash)return 1;
    wcscpy_s(slash+1,_countof(path)-(slash+1-path),L"weapon-skin-animation.log");
    gLog=_wfopen(path,L"ab");if(!gLog)return 1;
    if(GetEnvironmentVariableW(L"DFO_WEAPON_SKIN_ANIMATION",opt,_countof(opt))!=2||wcscmp(opt,L"r1")) {
        Log("INACTIVE candidate opt-in absent; no hooks\n");return 0;
    }
    gBase=(uintptr_t)GetModuleHandleW(nullptr);
    auto dos=(IMAGE_DOS_HEADER*)gBase;
    if(!Readable(dos,sizeof(*dos))||dos->e_magic!=IMAGE_DOS_SIGNATURE||dos->e_lfanew<0||dos->e_lfanew>0x100000){Log("DISABLED invalid DOS header\n");return 2;}
    auto nt=(IMAGE_NT_HEADERS64*)(gBase+dos->e_lfanew);
    if(!Readable(nt,sizeof(*nt))||nt->Signature!=IMAGE_NT_SIGNATURE||nt->FileHeader.Machine!=IMAGE_FILE_MACHINE_AMD64||
       nt->FileHeader.TimeDateStamp!=kImageTimestamp||nt->OptionalHeader.SizeOfImage!=kImageSize||
       !Match(kCycleRVA,kCycleHead)||!Match(kExpansionRVA,kExpansionHead)||
       !Match(kResolveRVA,kResolveHead)||!Match(kLookupRVA,kLookupHead)||
       !Match(kSkinRVA,kSkinHead)||!Match(kShapeRVA,kShapeHead)) {
        Log("DISABLED image/function byte mismatch; no hooks\n");return 2;
    }
    gSkin=(SkinFn)(gBase+kSkinRVA);gShape=(ShapeFn)(gBase+kShapeRVA);
    bool prepared=PrepareHook(gHooks[0],(void*)(gBase+kResolveRVA),kResolveHead,sizeof(kResolveHead),(void*)Resolve) &&
        PrepareHook(gHooks[1],(void*)(gBase+kLookupRVA),kLookupHead,sizeof(kLookupHead),(void*)Lookup) &&
        PrepareHook(gHooks[2],(void*)(gBase+kCycleRVA),kCycleHead,sizeof(kCycleHead),(void*)Cycle) &&
        PrepareHook(gHooks[3],(void*)(gBase+kExpansionRVA),kExpansionHead,sizeof(kExpansionHead),(void*)Expansion);
    if(!prepared){Log("DISABLED preparation failed; no hooks published\n");return 3;}
    gResolve=(ResolveFn)gHooks[0].original();gLookup=(LookupFn)gHooks[1].original();
    gCycle=(CycleFn)gHooks[2].original();gExpansion=(CycleFn)gHooks[3].original();
    for(auto& hook:gHooks)if(!PublishHook(hook)) {
        bool restored=true;
        for(int i=3;i>=0;--i)restored=RestoreHook(gHooks[i])&&restored;
        Log(restored?"DISABLED publication mismatch; originals restored\n":"DISABLED publication mismatch; rollback conflict; relays retained\n");return 4;
    }
    InterlockedExchange(&gEnabled,1);
    Log("READY r1 native animation/expansion scopes; equipment/stat calls outside scopes unchanged; manual visual validation pending\n");
    return 0;
}
BOOL WINAPI DllMain(HINSTANCE self,DWORD reason,LPVOID) {
    if(reason==DLL_PROCESS_ATTACH){gSelf=self;DisableThreadLibraryCalls(self);}return TRUE;
}
