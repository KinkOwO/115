#define _CRT_SECURE_NO_WARNINGS
#include "inline_hook.h"
#include "pilot_policy.h"
#include "all_dungeon_evidence.h"
#include "native_evidence.h"
#include "crash_trace.h"
#include "exit_trace.h"
#include "graphics_failure_trace.h"
#include "companion_resource_guard.h"
#include "companion_load_scope.h"
#include "native_room_scope.h"
#include "native_panel_binding.h"
#include "native_registration_scope.h"
#include <stdio.h>
#include <stdarg.h>
#include <stdlib.h>

static HMODULE gSelf;
static uintptr_t gBase;
static FILE* gLog;
static SRWLOCK gLogLock=SRWLOCK_INIT;
static volatile LONG gStarted=0,gEnabled=0,gID=0,gPending=0;
static DWORD gEntered=0;
static InlineHook gHooks[14];
static __declspec(thread) LONG gCompanionReadDepth=0;
static __declspec(thread) LONG gTownPanelDepth=0;
static void* volatile gSourceObject=nullptr;
static bool gAll=false;
static volatile LONG gCompanionsReady=0;
static void* volatile gDungeonInstance=nullptr;
static PanelBindingSnapshot gPanelBinding={};
static volatile LONG gLoadSerial=0,gRegistrationPending=0;
static void* volatile gRegistrationManager=nullptr;
static __declspec(thread) NativeRegistrationFrame* gRegistrationFrame=nullptr;
static volatile LONG gPanelManaged=0;
static volatile LONG gRetreatLogged=0,gLastScene=-1;
using GateFn=bool(__fastcall*)(void*);
using ModeFn=int(__fastcall*)(void*);
using KeyFn=int(__fastcall*)(void*,uint32_t);
using PacketFn=void(__fastcall*)();
using LookupFn=void*(__fastcall*)(uint32_t);
using PositionFn=void(__fastcall*)(void*,float,float);
using TownFn=bool(__fastcall*)(uint32_t);
using MenuPermissionFn=bool(__fastcall*)(void*,int);
using PreloadFn=void(__fastcall*)(void*,uint32_t);
using ResourceLookupFn=void*(__fastcall*)(void*,uint32_t,bool);
static GateFn gGate;
static ModeFn gMode;
static KeyFn gKey;
static PacketFn gLoaded,gMap;
static MenuPermissionFn gMenuPermission;
static PacketFn gTagInfo;
static PreloadFn gPetPreload;
static RoomFn gRoom;

static void Log(const char* fmt,...) {
    if(!gLog)return;
    AcquireSRWLockExclusive(&gLogLock);
    fprintf(gLog,"[%lu] ",GetTickCount()); va_list a; va_start(a,fmt); vfprintf(gLog,fmt,a); va_end(a);
    fputc('\n',gLog); fflush(gLog); ReleaseSRWLockExclusive(&gLogLock);
}
template<class T> static bool Read(uintptr_t p,T& result) {
    if(!Readable((void*)p,sizeof(T)))return false;
    __try { memcpy(&result,(void*)p,sizeof(T)); return true; }
    __except(EXCEPTION_EXECUTE_HANDLER){return false;}
}
static bool Scene(uint32_t& scene) {
    uintptr_t world=0;
    if(!Read(gBase+0xe66c090,world)||!world||!Readable((void*)world,0x110))return false;
    // The native getter first consults the current override's virtual +0x60,
    // then its protected field fallback. Passive live evidence confirms the
    // override is populated; reproducing only the fallback would be wrong.
    __try {
        scene=((uint32_t(__fastcall*)(void*))(gBase+kSceneGetterRVA))((void*)world);
        return scene!=UINT32_MAX;
    } __except(EXCEPTION_EXECUTE_HANDLER){return false;}
}
static bool AllowedID(uint32_t id) {
    return gAll?PilotSortedID(id,kAllDungeonIDs,_countof(kAllDungeonIDs)):PilotID(id,kPilotIDs,_countof(kPilotIDs));
}
static bool Scope() {
    uint32_t scene=0; bool valid=Scene(scene);
    if(valid&&gEnabled&&gID&&InterlockedExchange(&gLastScene,(LONG)scene)!=(LONG)scene)
        Log("pilot native scene transition dungeon=%ld scene=%u",gID,scene);
    return PilotScope(InterlockedCompareExchange(&gEnabled,0,0)!=0,
        AllowedID((uint32_t)InterlockedCompareExchange(&gID,0,0))&&PilotGenerationReady(gAll,gPending!=0,gCompanionsReady!=0),
        InterlockedCompareExchange(&gPending,0,0)!=0,GetTickCount()-gEntered,valid,scene);
}
static bool __fastcall Gate(void* self){return gGate(self)||Scope();}
static int __fastcall Mode(void* self){
    int native=gMode(self);
    // The town override exists only around the native UI opener. Do not let
    // town reset/C1811 or player container readers inherit dungeon mode 2.
    return native==0&&(gTownPanelDepth>0||Scope())?2:native;
}
static int __fastcall Key(void* self,uint32_t slot){
    int native=gKey(self,slot);
    // This getter feeds elite roster filters and portraits, not map admission.
    // Preserve its native slot-to-preset result in the all-dungeon candidate.
    return PilotRosterKey(gAll,gEnabled!=0,AllowedID(slot),native);
}
static void PanelVisibility(void* root,bool show) {
    uintptr_t vt=0,setter=0;
    if(Read((uintptr_t)root,vt)&&Read(vt+0x10,setter)&&setter==gBase+kVisibilityRVA)
        ((void(__fastcall*)(void*,bool))setter)(root,show);
}
// This CALL belongs only to window 168's per-frame root positioning. Moving
// its root moves both the panel and native expand/collapse tabs, without
// accumulating offsets or moving any other sidebar.
static void __fastcall Position(void* root,float x,float y) {
    bool managed=gEnabled&&InterlockedCompareExchange(&gPanelManaged,0,0)!=0;
    ((PositionFn)(gBase+kPositionRVA))(root,x,managed?y+180.0f:y);
    uint32_t scene=0;
    if(managed&&Scene(scene)) {
        // Live Window168 root vtable+0x10 resolves to this fingerprinted native
        // visibility setter. Hide its full control tree in loading/combat;
        // keep the window alive so the native update can restore it in town.
        PanelVisibility(root,scene==1);
    }
}
static bool RetreatScope(uint32_t scene,int action) {
    bool allowed=PilotRetreat(Scope(),scene,action);
    if(allowed&&InterlockedCompareExchange(&gRetreatLogged,1,0)==0)
        Log("GO_TO_TOWN allowed in pilot combat; native confirmation/C42 unchanged dungeon=%ld",gID);
    return allowed;
}
// Only GO_TO_TOWN's audited switch-arm CALL is replaced, not the common
// 83-action predicate. The native action and its confirmation are untouched.
static bool __fastcall Town(uint32_t scene) {
    bool native=((TownFn)(gBase+kTownRVA))(scene);
    return native||RetreatScope(scene,22);
}
static bool __fastcall MenuPermission(void* self,int action) {
    bool native=gMenuPermission(self,action);
    if(native||action!=22)return native;
    uint32_t scene=0;
    return Scene(scene)&&RetreatScope(scene,action);
}
// The captured native crash is inside NOTI1382 -> pet ctor -> preload:
// 145E47F60 returns null, then 145E47F74 forms null+0xC0 and dereferences it.
// Keep the original reader/cursor and valid resource paths unchanged.
static void __fastcall TagInfo() {
    Log("N1382 native companion load begin dungeon=%ld",gID);
    RunCompanionLoad(gCompanionReadDepth,gTagInfo);
    Log("N1382 native companion load completed dungeon=%ld",gID);
}
static void __fastcall PetPreload(void* self,uint32_t id) {
    bool active=gCompanionReadDepth>0&&Scope();
    uintptr_t manager=0;
    if(active)Read(gBase+0xe683b28,manager);
    auto result=CompanionPreload(active,(void*)manager,id,
        (ResourceLookupFn)(gBase+kResourceLookupRVA),gPetPreload,self);
    if(result!=PreloadResult::Native)
        Log("COMPANION_PRELOAD_SKIPPED dungeon=%ld resource=%u cause=%s; resource absent, associated pet effect unavailable",gID,id,
            result==PreloadResult::MissingResource?"missing_resource":"missing_manager");
}
// This observer replaces ONLY N28's audited CALL after its native u32 reader.
// The packet cursor and all following native validation stay unchanged.
static void* __fastcall Lookup(uint32_t id) {
    void* result=((LookupFn)(gBase+kLookupRVA))(id);
    uintptr_t nativeManager=0;
    int nativeMode=0;
    if(gAll&&Read(gBase+0xe638ef8,nativeManager)&&nativeManager)nativeMode=gMode((void*)nativeManager);
    bool allowed=result&&AllowedID(id)&&PilotNativeModeAllowed(gAll,nativeMode);
    InterlockedExchange(&gCompanionsReady,0);
    uintptr_t source=0;
    // Lookup returns a runtime Dungeon instance, not its PVF definition.
    // The exact same +1A88 indirection is used by native Room and observed live.
    if(allowed)Read((uintptr_t)result+0x1a88,source);
    InterlockedExchangePointer(&gDungeonInstance,allowed?result:nullptr);
    InterlockedExchangePointer(&gSourceObject,allowed?(void*)source:nullptr);
    InterlockedExchange(&gRegistrationPending,0);
    InterlockedExchangePointer(&gRegistrationManager,nullptr);
    InterlockedExchange(&gID,allowed?(LONG)id:0);
    InterlockedExchange(&gRetreatLogged,0);
    InterlockedExchange(&gLastScene,-1);
    gEntered=GetTickCount(); InterlockedExchange(&gPending,allowed?1:0);
    Log("N28 dungeon=%u runtime_instance=%p source_definition=%p pilot=%d",id,result,(void*)source,allowed); return result;
}
static void RefreshTownPanel(uintptr_t window,uintptr_t manager) {
    uintptr_t world=0,roles=0,vt=0;
    if(!Read(window,vt)||vt!=gBase+kWindowVtableRVA||
       !Read(gBase+0xe66c090,world)||!world||!Read(world+0x110,roles)||!roles)return;
    auto config=((void*(__fastcall*)(void*,int))(gBase+kEliteConfigRVA))((void*)manager,2);
    if(!config)return; // Native saved mode 2 has not arrived yet.
    PanelBindingSnapshot next={};next.window=window;next.roles=roles;next.config=(uintptr_t)config;
    for(int i=0;i<3;i++) {
        if(!Read((uintptr_t)config+0x27+i*4,next.slots[i]))return;
        if((int32_t)next.slots[i]>=0)
            next.sources[i]=(uintptr_t)((void*(__fastcall*)(void*,uint32_t,int))(gBase+kRoleLookupRVA))((void*)roles,next.slots[i],-1);
        if(!Read(manager+0x50+i*24,next.clones[i]))return;
    }
    if(!PanelBindingChanged(gPanelBinding,next))return;
    // This is the same two-argument native binder used by Window168::Open and
    // N1879. It owns all weak-reference release/acquire operations. No mode
    // override leaks into player readers, and no network packets are invented.
    ((void(__fastcall*)(void*,int))(gBase+kPanelRefreshRVA))((void*)window,2);
    gPanelBinding=next;
    unsigned count=0;
    for(int i=0;i<3;i++) {
        uintptr_t control=0,actor=0;uint32_t alive=0;
        if(Read(window+0x640+i*24,control)&&control&&Read(control+8,alive)&&alive&&
           Read(window+0x648+i*24,actor)&&actor)++count;
    }
    Log("TOWN_PANEL native rebind mode=2 selected=%u,%u,%u source_ready=%d,%d,%d bound=%u",next.slots[0],next.slots[1],next.slots[2],next.sources[0]!=0,next.sources[1]!=0,next.sources[2]!=0,count);
}
// This is the final, unconditional native CALL of the current town update,
// after its ordinary gate/reset/open logic. Preserve the original operation
// and only request Window168 through its existing opener in native scene 1.
static void __fastcall TownFinish(void* self) {
    ((void(__fastcall*)(void*))(gBase+kTownFinishRVA))(self);
    uint32_t scene=0;uintptr_t manager=0;
    if(!gEnabled||!Scene(scene)||scene!=1||!Read(gBase+0xe638ef8,manager)||!manager)return;
    if(gMode((void*)manager)!=0) {InterlockedExchange(&gPanelManaged,0);return;}
    InterlockedExchange(&gPanelManaged,1);
    ++gTownPanelDepth;
    __try {((void(__fastcall*)(void*))(gBase+kOpenRVA))((void*)manager);}
    __finally {--gTownPanelDepth;}
    // Native UI manager's IsOpen(168,false) reads this vector. Restore its
    // root independently of the window update, which may skip hidden roots.
    uintptr_t ui=0,begin=0,end=0,window=0,root=0;
    if(Read(gBase+0xe683c78,ui)&&ui&&Read(ui+0x1d8+168*24,begin)&&
       Read(ui+0x1e0+168*24,end)&&begin&&end-begin==8&&Read(begin,window)&&window&&
       Read(window+0xd0,root)&&root) {
        PanelVisibility((void*)root,true);
        RefreshTownPanel(window,manager);
    }
}
static bool __fastcall RoomActive(void* manager) {
    bool native=((GateFn)(gBase+kActiveGetterRVA))(manager);
    return NativeRegistrationActive(native,gRegistrationFrame,(uintptr_t)manager);
}
static int ValidClones(uintptr_t manager) {
    int count=0;
    for(int i=0;i<3;i++) {
        uintptr_t control=0,actor=0;uint32_t refs=0;
        if(Read(manager+0x48+i*24,control)&&control&&Read(control+8,refs)&&refs&&
           Read(manager+0x50+i*24,actor)&&actor)++count;
    }
    return count;
}
static void __fastcall Room(void* self,int x,int y,bool a4,bool a5,void* actor,bool a7,bool a8) {
    uintptr_t source=0;unsigned char flag=0;unsigned char* scopedFlag=nullptr;
    bool scope=Scope()&&PilotRoomReady(gAll,InterlockedCompareExchange(&gCompanionsReady,0,0)!=0);
    bool identity=Read((uintptr_t)self+0x1a88,source)&&
       PilotRoomIdentity(scope,(uintptr_t)self,source,
           (uintptr_t)InterlockedCompareExchangePointer(&gDungeonInstance,nullptr,nullptr),
           (uintptr_t)InterlockedCompareExchangePointer(&gSourceObject,nullptr,nullptr));
    if(identity&&Read(source+0x1878,flag)&&flag==0) {
        MEMORY_BASIC_INFORMATION m={};
        if(VirtualQuery((void*)(source+0x1878),&m,sizeof(m))==sizeof(m)&&m.State==MEM_COMMIT&&
           !(m.Protect&(PAGE_GUARD|PAGE_NOACCESS))&&(m.Protect&(PAGE_READWRITE|PAGE_WRITECOPY)))
            scopedFlag=(unsigned char*)(source+0x1878);
    }
    uintptr_t manager=0;unsigned char beforeActive=0,active=0;
    bool managerValid=Read(gBase+0xe638ef8,manager)&&manager&&Read(manager+0x240,beforeActive);
    LONG epoch=InterlockedCompareExchange(&gRegistrationPending,0,0);
    NativeRegistrationFrame frame={manager,identity&&managerValid&&epoch!=0&&
        manager==(uintptr_t)InterlockedCompareExchangePointer(&gRegistrationManager,nullptr,nullptr)&&ValidClones(manager)>0};
    RunRegistrationRoom(gRegistrationFrame,&frame,gRoom,self,x,y,a4,a5,actor,a7,a8,scopedFlag);
    bool valid=managerValid&&Read(manager+0x240,active);
    if(frame.queried&&valid&&active)InterlockedCompareExchange(&gRegistrationPending,0,epoch);
    if(scope) {
        Log("native room spawn returned dungeon=%ld instance=%p bound_instance=%p source=%p bound_source=%p scoped_flag=%d original_flag=%u active_valid=%d active=%u; visible companions require manual acceptance",gID,self,InterlockedCompareExchangePointer(&gDungeonInstance,nullptr,nullptr),(void*)source,InterlockedCompareExchangePointer(&gSourceObject,nullptr,nullptr),scopedFlag!=nullptr,flag,valid,active);
        Log("REGISTRATION epoch=%ld manager=%p fresh=%d queried=%d stale_latch_bypassed=%d active_before=%u active_after=%u pending=%ld",epoch,(void*)manager,frame.pending,frame.queried,frame.bypassed,beforeActive,active,InterlockedCompareExchange(&gRegistrationPending,0,0));
    }
}
static void __fastcall Loaded(){
    Log("N1879 native AI load begin dungeon=%ld",gID);
    InterlockedExchange(&gRegistrationPending,0);
    // Manual r3 crash unwinds through N1879 -> AI clone -> pet constructor.
    // The same missing preload resource must be checked in this second stage.
    RunCompanionLoad(gCompanionReadDepth,gLoaded);
    if(Scope()) {
        uintptr_t manager=0;
        if(Read(gBase+0xe638ef8,manager)&&manager&&ValidClones(manager)>0) {
            InterlockedExchange(&gCompanionsReady,1);
            InterlockedExchangePointer(&gRegistrationManager,(void*)manager);
            LONG epoch=InterlockedIncrement(&gLoadSerial);
            InterlockedExchange(&gRegistrationPending,epoch);
            Log("N1879 fresh clone generation epoch=%ld manager=%p clones=%d",epoch,(void*)manager,ValidClones(manager));
        }
        Log("N1879 native clone handler returned dungeon=%ld; world registration is checked separately",gID);
    }
}
static void __fastcall Map(){
    Log("N29 native map begin dungeon=%ld",gID);
    // No fresh N1879 means ordinary player container/map initialization. End
    // the provisional N1382 gate before native N29 can read player caches.
    if(gAll)InterlockedExchange(&gPending,0);
    gMap(); InterlockedExchange(&gPending,0);
    uint32_t scene=0; bool valid=Scene(scene);
    Log("N29 native map start dungeon=%ld scene_valid=%d scene=%u",gID,valid,scene);
}
template<size_t N> static bool Match(uintptr_t rva,const unsigned char(&bytes)[N]){
    return Readable((void*)(gBase+rva),N)&&memcmp((void*)(gBase+rva),bytes,N)==0;
}
extern "C" __declspec(dllexport) const char* WINAPI ModName(){return "Account elite companions r8 all admitted dungeons candidate";}
extern "C" __declspec(dllexport) DWORD WINAPI ModStart(){
    if(InterlockedCompareExchange(&gStarted,1,0))return 0;
    wchar_t path[MAX_PATH]={}; DWORD n=GetModuleFileNameW(gSelf,path,MAX_PATH);
    if(!n||n>=MAX_PATH)return 1; auto slash=wcsrchr(path,L'\\'); if(!slash)return 1;
    wcscpy_s(slash+1,MAX_PATH-(slash+1-path),L"elite-companion-pilot.log");gLog=_wfopen(path,L"ab");
    wcscpy_s(slash+1,MAX_PATH-(slash+1-path),L"elite-companion-crash.log");
    if(!CrashTrace::Start(path,&gID)){Log("DISABLED passive crash observer failed; no hooks written");return 5;}
    wchar_t candidate[16]={};
    gAll=GetEnvironmentVariableW(L"DFO_ELITE_COMPANION_PILOT",candidate,_countof(candidate))==3&&wcscmp(candidate,L"all")==0;
    if(gAll)Log("ELITE_ALL r6-diag source=%s count=%zu; roster key=native; empty roster does not activate clones; native modes 3/4 preserved",kAllSourceSHA,_countof(kAllDungeonIDs));
    gBase=(uintptr_t)GetModuleHandleW(nullptr);
    auto dos=(IMAGE_DOS_HEADER*)gBase;
    if(!Readable(dos,sizeof(*dos))||dos->e_magic!=IMAGE_DOS_SIGNATURE||dos->e_lfanew<0||dos->e_lfanew>0x100000)return 2;
    auto nt=(IMAGE_NT_HEADERS64*)(gBase+dos->e_lfanew);
    if(!Readable(nt,sizeof(*nt))||nt->Signature!=IMAGE_NT_SIGNATURE||
       nt->FileHeader.TimeDateStamp!=kImageTimestamp||nt->OptionalHeader.SizeOfImage!=kImageSize ||
       !Match(kLookupRVA,kLookupHead)||!Match(kOpenRVA,kOpenHead)||
       !Match(kWorldSceneRVA,kWorldSceneHead)||!Match(kSceneGetterRVA,kSceneGetterHead)||!Match(kChannelRVA,kChannelHead)||
       !Match(kPositionRVA,kPositionHead)||!Match(kTownRVA,kTownHead)||!Match(kResourceLookupRVA,kResourceLookupHead)||
       !Match(kTownFinishRVA,kTownFinishHead)||!Match(kVisibilityRVA,kVisibilityHead)||!Match(kSpawnBranchRVA,kSpawnBranchHead)||
       !Match(kWindowVectorRVA,kWindowVectorHead)||!Match(kPanelRefreshRVA,kPanelRefreshHead)||
       !Match(kEliteConfigRVA,kEliteConfigHead)||!Match(kRoleLookupRVA,kRoleLookupHead)||
       !Match(kActiveGetterRVA,kActiveGetterHead)||!Match(kActiveGuardRVA,kActiveGuardHead)){
        Log("DISABLED client layout/helper fingerprint mismatch; no hooks written");return 2;}
    bool ready=PrepareHook(gHooks[0],(void*)(gBase+kGateRVA),kGateHead,sizeof(kGateHead),(void*)&Gate,7,11)&&
        PrepareHook(gHooks[1],(void*)(gBase+kModeRVA),kModeHead,sizeof(kModeHead),(void*)&Mode)&&
        PrepareHook(gHooks[2],(void*)(gBase+kKeyRVA),kKeyHead,sizeof(kKeyHead),(void*)&Key)&&
        PrepareHook(gHooks[3],(void*)(gBase+kLoadedRVA),kLoadedHead,sizeof(kLoadedHead),(void*)&Loaded)&&
        PrepareHook(gHooks[4],(void*)(gBase+kMapRVA),kMapHead,sizeof(kMapHead),(void*)&Map)&&
        PrepareHook(gHooks[5],(void*)(gBase+kLookupCallRVA),kLookupCallHead,sizeof(kLookupCallHead),(void*)&Lookup)&&
        PrepareHook(gHooks[6],(void*)(gBase+kPositionCallRVA),kPositionCallHead,sizeof(kPositionCallHead),(void*)&Position)&&
        PrepareHook(gHooks[7],(void*)(gBase+kTownCallRVA),kTownCallHead,sizeof(kTownCallHead),(void*)&Town)&&
        PrepareHook(gHooks[8],(void*)(gBase+kMenuPermissionRVA),kMenuPermissionHead,sizeof(kMenuPermissionHead),(void*)&MenuPermission)&&
        PrepareHook(gHooks[9],(void*)(gBase+kTagInfoRVA),kTagInfoHead,sizeof(kTagInfoHead),(void*)&TagInfo)&&
        PrepareHook(gHooks[10],(void*)(gBase+kPetPreloadRVA),kPetPreloadHead,sizeof(kPetPreloadHead),(void*)&PetPreload)&&
        PrepareHook(gHooks[11],(void*)(gBase+kRoomRVA),kRoomHead,sizeof(kRoomHead),(void*)&Room)&&
        PrepareHook(gHooks[12],(void*)(gBase+kTownFinishCallRVA),kTownFinishCallHead,sizeof(kTownFinishCallHead),(void*)&TownFinish)&&
        PrepareHook(gHooks[13],(void*)(gBase+kRoomActiveCallRVA),kRoomActiveCallHead,sizeof(kRoomActiveCallHead),(void*)&RoomActive);
    if(!ready){Log("DISABLED incomplete hook preparation; no hooks written");return 3;}
    // All fallback pointers exist BEFORE publishing the first branch.
    gGate=(GateFn)gHooks[0].original();gMode=(ModeFn)gHooks[1].original();gKey=(KeyFn)gHooks[2].original();
    gLoaded=(PacketFn)gHooks[3].original();gMap=(PacketFn)gHooks[4].original();
    gMenuPermission=(MenuPermissionFn)gHooks[8].original();
    gTagInfo=(PacketFn)gHooks[9].original();gPetPreload=(PreloadFn)gHooks[10].original();
    gRoom=(RoomFn)gHooks[11].original();
    for(int i=0;i<14;i++)if(!PublishHook(gHooks[i],i==5||i==6||i==7||i==12||i==13)){
        for(int j=i-1;j>=0;j--)if(!RestoreHook(gHooks[j]))Log("ROLLBACK FAILED hook=%d",j);
        Log("DISABLED hook publication failed index=%d",i);return 4;}
    if(!ExitTrace::Start())Log("DIAGNOSTIC exit observer unavailable; gameplay hooks unchanged");
    else Log("DIAGNOSTIC exit observer ready; original termination status is preserved");
    if(GraphicsFailureTrace::Start(gBase))Log("DIAGNOSTIC graphics failure observer ready; HRESULT, memory and native throw path preserved");
    else Log("DIAGNOSTIC graphics failure observer unavailable; native path unchanged");
    Log("DIAGNOSTIC C++ exception observer ready; native exception propagation preserved");
    InterlockedExchange(&gEnabled,1);
    Log("READY r7 scope=continuous-map-registration attempt=1/3 fresh N1879 generation bypasses stale latch only in audited room CALL; native activation write unchanged; town-only squad panel y_offset=180; combat_retreat=22 count=%zu source=%s; gameplay acceptance pending",_countof(kPilotIDs),kSourceSHA);
    return 0;
}
BOOL WINAPI DllMain(HINSTANCE self,DWORD reason,LPVOID){
    if(reason==DLL_PROCESS_ATTACH){gSelf=self;DisableThreadLibraryCalls(self);}return TRUE;
}
