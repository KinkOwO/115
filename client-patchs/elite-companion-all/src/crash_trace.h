#pragma once
#include "inline_hook.h"
#include <psapi.h>
#include <stdio.h>

// Passive first-chance exception evidence. Never changes CONTEXT, handles an
// exception, attaches a debugger or reads packet/save payloads.
namespace CrashTrace {
static HANDLE file=INVALID_HANDLE_VALUE;
static PVOID handler=nullptr;
static volatile LONG busy=0, count=0;
static const volatile LONG* dungeon=nullptr;
static void Line(const char* text) {
    DWORD written=0;
    if(file!=INVALID_HANDLE_VALUE) WriteFile(file,text,(DWORD)strlen(text),&written,nullptr);
}
static void Frame(int index,uintptr_t pc) {
    MEMORY_BASIC_INFORMATION m={}; wchar_t module[MAX_PATH]={};
    uintptr_t base=0;
    if(VirtualQuery((void*)pc,&m,sizeof(m))==sizeof(m)&&m.Type==MEM_IMAGE) {
        base=(uintptr_t)m.AllocationBase;
        GetModuleFileNameW((HMODULE)base,module,MAX_PATH);
    }
    const wchar_t* name=wcsrchr(module,L'\\'); name=name?name+1:module;
    char line[768]={};
    _snprintf_s(line,sizeof(line),_TRUNCATE,"frame=%d pc=%p module=%ls rva=0x%llx\r\n",
        index,(void*)pc,name,(unsigned long long)(base?pc-base:0)); Line(line);
}
static void Unwind(CONTEXT context) {
    __try {
        for(int i=0;i<64&&context.Rip;i++) {
            Frame(i,(uintptr_t)context.Rip);
            DWORD64 base=0; auto entry=RtlLookupFunctionEntry(context.Rip,&base,nullptr);
            auto previous=context.Rsp;
            if(entry) {
                PVOID data=nullptr; DWORD64 frame=0;
                RtlVirtualUnwind(UNW_FLAG_NHANDLER,base,context.Rip,entry,&context,&data,&frame,nullptr);
            } else {
                if(!Readable((void*)context.Rsp,sizeof(DWORD64))) break;
                context.Rip=*(DWORD64*)context.Rsp;context.Rsp+=sizeof(DWORD64);
            }
            if(context.Rsp<=previous||context.Rsp-previous>1024*1024)break;
        }
    } __except(EXCEPTION_EXECUTE_HANDLER){ Line("unwind_incomplete\r\n"); }
}
static LONG CALLBACK Observe(EXCEPTION_POINTERS* info) {
    if(!info||!info->ExceptionRecord||!info->ContextRecord||!dungeon||
       (info->ExceptionRecord->ExceptionCode!=EXCEPTION_ACCESS_VIOLATION && info->ExceptionRecord->ExceptionCode!=0xe06d7363)||
       (info->ExceptionRecord->ExceptionCode==EXCEPTION_ACCESS_VIOLATION && !*dungeon)||
       InterlockedCompareExchange(&busy,1,0)!=0) return EXCEPTION_CONTINUE_SEARCH;
    if(InterlockedIncrement(&count)<=4096) {
        const auto e=info->ExceptionRecord;const auto c=info->ContextRecord;
        char line[1024]={};
        _snprintf_s(line,sizeof(line),_TRUNCATE,
            "tick=%lu tid=%lu first_chance_exception dungeon=%ld code=0x%08lx exception=%p access=%llu address=%p rcx=%p rdx=%p r8=%p r9=%p rsp=%p rax=%p rbx=%p rsi=%p rdi=%p rbp=%p\r\n",
            GetTickCount(),GetCurrentThreadId(),*dungeon,e->ExceptionCode,e->ExceptionAddress,
            (unsigned long long)(e->NumberParameters>0?e->ExceptionInformation[0]:0),
            (void*)(e->NumberParameters>1?e->ExceptionInformation[1]:0),
            (void*)c->Rcx,(void*)c->Rdx,(void*)c->R8,(void*)c->R9,(void*)c->Rsp,(void*)c->Rax,(void*)c->Rbx,(void*)c->Rsi,(void*)c->Rdi,(void*)c->Rbp);
        Line(line);
        _snprintf_s(line,sizeof(line),_TRUNCATE,"parameters=%lu info0=%p info1=%p info2=%p info3=%p\r\n",e->NumberParameters,
            (void*)(e->NumberParameters>0?e->ExceptionInformation[0]:0),
            (void*)(e->NumberParameters>1?e->ExceptionInformation[1]:0),
            (void*)(e->NumberParameters>2?e->ExceptionInformation[2]:0),
            (void*)(e->NumberParameters>3?e->ExceptionInformation[3]:0));
        Line(line);
        PROCESS_MEMORY_COUNTERS_EX mem={};mem.cb=sizeof(mem);
        MEMORYSTATUSEX system={};system.dwLength=sizeof(system);
        const BOOL memoryOK=GetProcessMemoryInfo(GetCurrentProcess(),(PROCESS_MEMORY_COUNTERS*)&mem,sizeof(mem));
        const BOOL systemOK=GlobalMemoryStatusEx(&system);
        _snprintf_s(line,sizeof(line),_TRUNCATE,
            "memory_ok=%d private_bytes=%llu peak_commit_bytes=%llu working_set_bytes=%llu system_ok=%d available_virtual_bytes=%llu available_pagefile_bytes=%llu\r\n",
            memoryOK,(unsigned long long)mem.PrivateUsage,(unsigned long long)mem.PeakPagefileUsage,
            (unsigned long long)mem.WorkingSetSize,systemOK,(unsigned long long)system.ullAvailVirtual,
            (unsigned long long)system.ullAvailPageFile);
        Line(line); Unwind(*c); Line("end_first_chance_exception (may be handled by native client)\r\n");
        FlushFileBuffers(file);
    }
    InterlockedExchange(&busy,0);
    return EXCEPTION_CONTINUE_SEARCH;
}
static bool Start(const wchar_t* path,const volatile LONG* id) {
    if(handler)return false;
    file=CreateFileW(path,FILE_APPEND_DATA,FILE_SHARE_READ|FILE_SHARE_WRITE,nullptr,OPEN_ALWAYS,
        FILE_ATTRIBUTE_NORMAL,nullptr);
    if(file==INVALID_HANDLE_VALUE)return false;
    dungeon=id;count=0;busy=0;
    handler=AddVectoredExceptionHandler(1,Observe);
    if(!handler){CloseHandle(file);file=INVALID_HANDLE_VALUE;return false;}
    Line("r6 passive diagnostics; C++ exceptions throughout elite test session, AV in admitted dungeon; parameters/memory/64 frames; CONTINUE_SEARCH\r\n");
    return true;
}
static void Stop() {
    if(handler){RemoveVectoredExceptionHandler(handler);handler=nullptr;}
    if(file!=INVALID_HANDLE_VALUE){CloseHandle(file);file=INVALID_HANDLE_VALUE;}
}
}
