#pragma once
#include "crash_trace.h"
#include "exit_evidence.h"

// Termination may bypass first-chance exceptions (for example CRT abort ->
// ExitProcess(3)). Observe the audited native export and forward its original
// status. This diagnostic neither catches errors nor keeps the client alive.
namespace ExitTrace {
using ExitFn=void(NTAPI*)(LONG);
static InlineHook hook;
static ExitFn original=nullptr;
static volatile LONG recording=0;
static __declspec(noinline) void NTAPI Observe(LONG status) {
    if(CrashTrace::dungeon&&*CrashTrace::dungeon&&
       InterlockedCompareExchange(&recording,1,0)==0) {
        char line[256]={};
        _snprintf_s(line,sizeof(line),_TRUNCATE,
            "tick=%lu tid=%lu native_process_exit dungeon=%ld status=0x%08lx; original exit forwarded\r\n",
            GetTickCount(),GetCurrentThreadId(),*CrashTrace::dungeon,(DWORD)status);
        CrashTrace::Line(line);
        CONTEXT context={};RtlCaptureContext(&context);
        CrashTrace::Unwind(context);
        CrashTrace::Line("end_native_process_exit\r\n");
        if(CrashTrace::file!=INVALID_HANDLE_VALUE)FlushFileBuffers(CrashTrace::file);
    }
    original(status);
}
static bool Start() {
    if(hook.installed)return false;
    auto module=GetModuleHandleW(L"ntdll.dll");
    if(!module)return false;
    auto dos=(IMAGE_DOS_HEADER*)module;
    if(!Readable(dos,sizeof(*dos))||dos->e_magic!=IMAGE_DOS_SIGNATURE||
       dos->e_lfanew<0||dos->e_lfanew>0x100000)return false;
    auto nt=(IMAGE_NT_HEADERS64*)((uintptr_t)module+dos->e_lfanew);
    if(!Readable(nt,sizeof(*nt))||nt->Signature!=IMAGE_NT_SIGNATURE||
       nt->FileHeader.TimeDateStamp!=kExitImageTimestamp||
       nt->OptionalHeader.SizeOfImage!=kExitImageSize)return false;
    auto target=GetProcAddress(module,"RtlExitUserProcess");
    if(!target||!PrepareHook(hook,(void*)target,kExitHead,sizeof(kExitHead),(void*)&Observe))return false;
    original=(ExitFn)hook.original();
    return PublishHook(hook);
}
static bool Stop(){return RestoreHook(hook);}
}
