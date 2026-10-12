#define WIN32_LEAN_AND_MEAN
#define _CRT_SECURE_NO_WARNINGS
#include <windows.h>
#include <stdio.h>
#include "eligibility-patch.h"
#include "native-trace.h"
#include "entry-trace.h"
#include "registration-adapter.h"
#include "combat-trace.h"
#include "catchup-adapter.h"
#include "birth-position-adapter.h"
#include "position-write-trace.h"
#include "first-chance-trace.h"

static HMODULE g_self;
static volatile LONG g_started;
static volatile LONG g_exit = ERROR_BUSY;

static DWORD Finish(DWORD code) {
    InterlockedExchange(&g_exit, static_cast<LONG>(code));
    return code;
}

static bool EnvironmentEnabled() {
    wchar_t value[4] = {};
    return GetEnvironmentVariableW(L"DFO_ADVENTURE_ELITE", value, ARRAYSIZE(value)) == 1 && value[0] == L'1';
}

static bool PathCatchupEnabled() {
    wchar_t value[4] = {};
    return GetEnvironmentVariableW(L"DFO_ADVENTURE_ELITE_PATH_CATCHUP", value, ARRAYSIZE(value)) == 1 && value[0] == L'1';
}

static bool LocalPath(const wchar_t* dir, const wchar_t* name, wchar_t* out, size_t capacity) {
    return swprintf_s(out, capacity, L"%s\\%s", dir, name) > 0;
}

extern "C" __declspec(dllexport) const char* WINAPI ModName() { return "AdventureElite eligibility candidate"; }

extern "C" __declspec(dllexport) DWORD WINAPI ModStart() {
    if (InterlockedCompareExchange(&g_started, 1, 0) != 0) return static_cast<DWORD>(InterlockedCompareExchange(&g_exit, 0, 0));
    wchar_t dir[32768], logPath[32768], status[32768], trace[32768], lifecycle[32768], entry[32768], combat[32768], catchup[32768];
    DWORD length = GetModuleFileNameW(g_self, dir, ARRAYSIZE(dir));
    if (!length || length >= ARRAYSIZE(dir)) return Finish(ERROR_BAD_PATHNAME);
    wchar_t* slash = wcsrchr(dir, L'\\');
    if (!slash) return Finish(ERROR_BAD_PATHNAME);
    *slash = 0;
    if (!LocalPath(dir, L"adventure-elite.log", logPath, ARRAYSIZE(logPath)) ||
        !LocalPath(dir, L"adventure-elite.status.json", status, ARRAYSIZE(status)) ||
        !LocalPath(dir, L"adventure-elite-native.jsonl", trace, ARRAYSIZE(trace))) return Finish(ERROR_BAD_PATHNAME);
    g_eliteTraceRunTick = GetTickCount64();
    char traceName[96] = {};
    wchar_t wideTraceName[96] = {};
    sprintf_s(traceName, "adventure-elite-native-%lu-%llu.jsonl", GetCurrentProcessId(), g_eliteTraceRunTick);
    swprintf_s(wideTraceName, L"adventure-elite-native-%lu-%llu.jsonl", GetCurrentProcessId(), g_eliteTraceRunTick);
    if (!LocalPath(dir, wideTraceName, trace, ARRAYSIZE(trace))) return Finish(ERROR_BAD_PATHNAME);
    char lifecycleName[96] = {};
    wchar_t wideLifecycleName[96] = {};
    sprintf_s(lifecycleName,"adventure-elite-lifecycle-%lu-%llu.jsonl",GetCurrentProcessId(),g_eliteTraceRunTick);
    swprintf_s(wideLifecycleName,L"adventure-elite-lifecycle-%lu-%llu.jsonl",GetCurrentProcessId(),g_eliteTraceRunTick);
    if (!LocalPath(dir,wideLifecycleName,lifecycle,ARRAYSIZE(lifecycle))) return Finish(ERROR_BAD_PATHNAME);
    char entryName[96]={}; wchar_t wideEntryName[96]={};
    sprintf_s(entryName,"adventure-elite-entry-%lu-%llu.jsonl",GetCurrentProcessId(),g_eliteTraceRunTick);
    swprintf_s(wideEntryName,L"adventure-elite-entry-%lu-%llu.jsonl",GetCurrentProcessId(),g_eliteTraceRunTick);
    if(!LocalPath(dir,wideEntryName,entry,ARRAYSIZE(entry))) return Finish(ERROR_BAD_PATHNAME);
    char combatName[96]={}; wchar_t wideCombatName[96]={};
    sprintf_s(combatName,"adventure-elite-combat-%lu-%llu.jsonl",GetCurrentProcessId(),g_eliteTraceRunTick);
    swprintf_s(wideCombatName,L"adventure-elite-combat-%lu-%llu.jsonl",GetCurrentProcessId(),g_eliteTraceRunTick);
    if(!LocalPath(dir,wideCombatName,combat,ARRAYSIZE(combat)))return Finish(ERROR_BAD_PATHNAME);
    char catchupName[96]={};wchar_t wideCatchupName[96]={};
    sprintf_s(catchupName,"adventure-elite-catchup-%lu-%llu.jsonl",GetCurrentProcessId(),g_eliteTraceRunTick);
    swprintf_s(wideCatchupName,L"adventure-elite-catchup-%lu-%llu.jsonl",GetCurrentProcessId(),g_eliteTraceRunTick);
    if(!LocalPath(dir,wideCatchupName,catchup,ARRAYSIZE(catchup)))return Finish(ERROR_BAD_PATHNAME);
    char positionName[96]={};wchar_t widePositionName[96]={},position[32768]={};
    sprintf_s(positionName,"adventure-elite-position-%lu-%llu.jsonl",GetCurrentProcessId(),g_eliteTraceRunTick);
    swprintf_s(widePositionName,L"adventure-elite-position-%lu-%llu.jsonl",GetCurrentProcessId(),g_eliteTraceRunTick);
    if(!LocalPath(dir,widePositionName,position,ARRAYSIZE(position)))return Finish(ERROR_BAD_PATHNAME);
    char permissionName[96]={};wchar_t widePermissionName[96]={},permission[32768]={};
    sprintf_s(permissionName,"adventure-elite-permission-%lu-%llu.jsonl",GetCurrentProcessId(),g_eliteTraceRunTick);
    swprintf_s(widePermissionName,L"adventure-elite-permission-%lu-%llu.jsonl",GetCurrentProcessId(),g_eliteTraceRunTick);
    if(!LocalPath(dir,widePermissionName,permission,ARRAYSIZE(permission)))return Finish(ERROR_BAD_PATHNAME);
    char collisionName[96]={};wchar_t wideCollisionName[96]={},collision[32768]={};
    sprintf_s(collisionName,"adventure-elite-collision-%lu-%llu.jsonl",GetCurrentProcessId(),g_eliteTraceRunTick);
    swprintf_s(wideCollisionName,L"adventure-elite-collision-%lu-%llu.jsonl",GetCurrentProcessId(),g_eliteTraceRunTick);
    if(!LocalPath(dir,wideCollisionName,collision,ARRAYSIZE(collision)))return Finish(ERROR_BAD_PATHNAME);
    char firstChanceName[96]={};wchar_t wideFirstChanceName[96]={},firstChance[32768]={};
    sprintf_s(firstChanceName,"adventure-elite-exception-%lu-%llu.jsonl",GetCurrentProcessId(),g_eliteTraceRunTick);
    swprintf_s(wideFirstChanceName,L"adventure-elite-exception-%lu-%llu.jsonl",GetCurrentProcessId(),g_eliteTraceRunTick);
    if(!LocalPath(dir,wideFirstChanceName,firstChance,ARRAYSIZE(firstChance)))return Finish(ERROR_BAD_PATHNAME);
    // One process-start switch shared with the server. Old INI files are ignored.
    const bool enabled = EnvironmentEnabled();
    unsigned char* base = (unsigned char*)GetModuleHandleW(nullptr);
    const bool firstChanceReady=enabled && EliteStartFirstChanceTrace(base,reinterpret_cast<uintptr_t>(g_self),firstChance);
    unsigned char* targets[5] = {};
    for (size_t i = 0; i < kEliteEligibilitySiteCount; ++i) targets[i] = base + kEliteEligibilitySites[i].rva;
    ElitePatchResult result = enabled ? EliteEligibilityPatch(targets, true) : ElitePatchResult{true, false, 0, "disabled"};
    const bool nativeTrace = enabled && result.ready && EliteStartNativeTrace(base, trace);
    const bool lifecycleConfigured = nativeTrace && EliteStartLifecycleTrace(base,lifecycle,g_eliteTraceRunTick);
    const bool preparation = lifecycleConfigured && EliteStartPreparationAdapter(base);
    const bool lifecycleReady = preparation && lifecycleConfigured;
    const bool entryReady = lifecycleReady && EliteStartEntryTrace(base,entry);
    const bool registrationReady = entryReady && EliteStartRegistration(base);
    const bool combatReady = registrationReady && EliteStartCombatTrace(base,combat);
    const bool catchupStarted = combatReady && EliteStartCatchup(base,catchup,PathCatchupEnabled());
    const bool birthReady = catchupStarted && EliteStartBirthPosition(base);
    const bool positionReady = birthReady && EliteStartPositionWrites(base,position);
    const bool permissionReady = positionReady && EliteStartPermissionTrace(base,permission);
    const bool collisionReady = permissionReady && EliteStartCollisionTrace(base,collision);
    const bool catchupReady = catchupStarted && birthReady && positionReady && permissionReady && collisionReady && firstChanceReady;
    FILE* f = nullptr;
    if (_wfopen_s(&f, logPath, L"a") == 0) {
        fprintf(f, "DFO_ADVENTURE_ELITE=%d eligibility ready=%d enabled=%d changed=%u reason=%s ordinaryReady=0 nativeTraceReady=%d ordinaryPrepareReady=%d pid=%lu runTick=%llu prepareFailure=%s hookStatus=%d traceFile=%s entryReady=%d registrationReady=%d registrationFailure=%s\n", enabled,
                result.ready, result.enabled, result.changed, result.reason, nativeTrace, preparation, GetCurrentProcessId(), g_eliteTraceRunTick, g_prepareFailure, g_prepareHookStatus, traceName,entryReady,registrationReady,g_registrationFailure);
        fclose(f);
    }
    if(_wfopen_s(&f,logPath,L"a")==0) {fprintf(f,"APC catchup ready=%d reason=%s criterion=%s pathQueryEnabled=%d delayMs=3000 file=%s\n",catchupReady,g_catchupFailure,EliteCatchupCriterion(),g_catchupPathEnabled,catchupName);fclose(f);}
    if (_wfopen_s(&f, status, L"w") != 0) return Finish(ERROR_WRITE_FAULT);
    fprintf(f, "{\"version\":\"0.3.25\",\"stage\":\"deferred-native-birth-landing\",\"ready\":%s,\"enabled\":%s,"
               "\"ordinaryReady\":false,\"ordinaryPrepareReady\":%s,\"nativeTraceReady\":%s,\"traceVersion\":2,\"traceLimitSelection\":128,\"traceLimitInfoDone\":64,\"processId\":%lu,\"runTick\":%llu,\"nativeTraceFile\":\"%s\",\"nativeTraceFailureSite\":\"%s\",\"preparationFailureSite\":\"%s\",\"hookStatus\":%d,\"changedBytes\":%u,\"rejectReason\":\"%s\",\"ordinaryLifecycleReady\":%s,\"lifecycleTraceVersion\":2,\"lifecycleTraceLimit\":1024,\"lifecycleTraceFile\":\"%s\",\"ordinaryEntryTraceReady\":%s,\"entryTraceFile\":\"%s\",\"entryFailureSite\":\"%s\",\"ordinaryRegistrationReady\":%s,\"registrationFailureSite\":\"%s\",\"ordinaryCombatTraceReady\":%s,\"combatTraceVersion\":2,\"combatTraceLimit\":4096,\"combatTraceFile\":\"%s\",\"combatFailureSite\":\"%s\",\"apcCatchupReady\":%s,\"apcCatchupFailureSite\":\"%s\",\"apcCatchupTraceFile\":\"%s\",\"apcCatchupCriterion\":\"%s\",\"apcPathQueryEnabled\":%s,\"nativePathAllocatorHooked\":%s,\"apcCatchupTraceVersion\":2,\"apcCatchupDelayMs\":3000,\"apcBirthPositionReady\":%s,\"birthPositionFailureSite\":\"%s\",\"apcBirthCorrectionEnabled\":false,\"apcBirthLandingFallbackEnabled\":true,\"apcBirthLandingDeferred\":true,\"apcBirthCacheSyncEnabled\":true,\"positionWriteTraceReady\":%s,\"positionWriteFailureSite\":\"%s\",\"positionWriteTraceFile\":\"%s\",\"positionWriteReadOnly\":true,\"permissionTraceReady\":%s,\"permissionFailureSite\":\"%s\",\"permissionTraceFile\":\"%s\",\"permissionReadOnly\":true,\"collisionTraceReady\":%s,\"collisionFailureSite\":\"%s\",\"collisionTraceFile\":\"%s\",\"collisionReadOnly\":true,\"firstChanceTraceReady\":%s,\"firstChanceTraceFile\":\"%s\",\"firstChanceOnly\":true}\n",
               result.ready && (!enabled || catchupReady) ? "true" : "false", result.enabled ? "true" : "false",
               preparation ? "true" : "false", nativeTrace ? "true" : "false", GetCurrentProcessId(), g_eliteTraceRunTick, traceName, g_eliteTraceFailure, g_prepareFailure, g_prepareHookStatus, result.changed,
               enabled && result.ready && !catchupReady ? "catchup-unavailable" : result.reason,lifecycleReady ? "true" : "false",lifecycleName,entryReady ? "true" : "false",entryName,g_entryFailure,registrationReady ? "true" : "false",g_registrationFailure,combatReady ? "true" : "false",combatName,g_combatFailure,catchupReady ? "true" : "false",g_catchupFailure,catchupName,EliteCatchupCriterion(),g_catchupPathEnabled?"true":"false",g_nativePathAllocate?"true":"false",birthReady?"true":"false",g_birthFailure,positionReady?"true":"false",g_positionFailure,positionName,permissionReady?"true":"false",g_permissionFailure,permissionName,collisionReady?"true":"false",g_collisionFailure,collisionName,firstChanceReady?"true":"false",firstChanceName);
    bool saved = fclose(f) == 0;
    return Finish(!saved ? ERROR_WRITE_FAULT : (result.ready && (!enabled || catchupReady) ? ERROR_SUCCESS : ERROR_INVALID_DATA));
}

// Startup injection additionally proves that the new client inherited the switch.
// The plugin-host ABI remains ModStart; both routes share one idempotent result.
extern "C" __declspec(dllexport) DWORD WINAPI ModStartInjected(LPVOID) {
    if (!EnvironmentEnabled()) return ERROR_ENVVAR_NOT_FOUND;
    return ModStart();
}

BOOL WINAPI DllMain(HINSTANCE module, DWORD reason, LPVOID) {
    if (reason == DLL_PROCESS_ATTACH) {
        g_self = module;
        DisableThreadLibraryCalls(module);
    }
    if(reason==DLL_PROCESS_DETACH && g_firstChanceHandler) {
        RemoveVectoredExceptionHandler(g_firstChanceHandler);g_firstChanceHandler=nullptr;
    }
    return TRUE;
}
