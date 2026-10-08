#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdio.h>
#include "preparation-adapter.h"
#include "lifecycle-trace.h"

static int flushCalls, observationCalls;
static __int64 __fastcall NativeFlush() { ++flushCalls; SetLastError(ERROR_ACCESS_DENIED); return 0x123456789ABCDEF; }
static __int64 __fastcall ThrowingFlush() { RaiseException(0xE1234567,0,0,nullptr); return 0; }
static void ObserveFlush(uintptr_t) { ++observationCalls; SetLastError(ERROR_INVALID_DATA); }
static DWORD WINAPI DifferentThread(void*) { EliteLifecycleSample("different-thread",0); return 0; }
int main() {
    EliteEnterPreparation(1754);
    EliteLeavePreparation(false,ElitePreparationTLS{});
    auto image=reinterpret_cast<unsigned char*>(GetModuleHandleW(nullptr));
    if(EliteStartPreparationAdapter(image) || EliteStartLifecycleTrace(image,L"unused",0)) return 9;
    g_originalFlush = NativeFlush; g_prepareFlushObservation = ObserveFlush;
    if (ElitePreparedFlush()!=0x123456789ABCDEF || flushCalls!=1 || observationCalls!=1 || GetLastError()!=ERROR_ACCESS_DENIED) return 1;
    g_originalFlush = ThrowingFlush;
    bool escaped=false;
    __try { ElitePreparedFlush(); }
    __except (GetExceptionCode()==0xE1234567 ? EXCEPTION_EXECUTE_HANDLER : EXCEPTION_CONTINUE_SEARCH) { escaped=true; }
    if(!escaped || observationCalls!=1) return 2;
    wchar_t path[32768] = {};
    if(!GetModuleFileNameW(nullptr,path,32768)) return 3;
    wchar_t* slash=wcsrchr(path,L'\\'); if(!slash) return 4;
    *(slash+1)=0;
    if(wcscat_s(path,L"lifecycle-trace-test.jsonl") || wcscpy_s(g_lifecyclePath,path)) return 5;
    // All game pointers stay null, so the observer must report unavailable.
    // Only this process's dist/ test file is written.
    g_lifecycleReady=true; g_lifecycleRunTick=123;
    EliteArmLifecycle(22,2); // Current identity is unavailable; record then disarm.
    if(g_lifecycleSamples!=1 || g_lifecycleArmed) return 6;
    g_lifecycleArmed=true;
    HANDLE other=CreateThread(nullptr,0,DifferentThread,nullptr,0,nullptr);
    if(!other || WaitForSingleObject(other,5000)!=WAIT_OBJECT_0 || g_lifecycleSamples!=1) return 7;
    CloseHandle(other);
    // Matching null cached identity allows exercising the budget in isolation.
    g_lifecycleChannel=0; g_lifecycleOwner=0;
    for(int i=0;i<1100;++i) EliteLifecycleSample("native-flush",0x3C768AF);
    if(g_lifecycleSamples!=1101 || g_lifecycleDropped!=0) return 8;
    puts("lifecycle trace: native flush result/last-error/exception forwarding, thread boundary, identity disarm and bounded file output passed");
    return 0;
}
