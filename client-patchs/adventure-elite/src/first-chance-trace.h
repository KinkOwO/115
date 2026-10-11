#pragma once
#include "position-collision-trace.h"

// First-chance evidence only. Never handle an exception or change its context.
static wchar_t g_firstChancePath[32768];
static uintptr_t g_firstChanceImage,g_firstChanceModule;
static void* g_firstChanceHandler;
static volatile LONG g_firstChanceBusy,g_firstChanceRecords;
static LONG CALLBACK EliteFirstChanceObserve(EXCEPTION_POINTERS* exception) {
    if(!exception || !exception->ExceptionRecord || !exception->ContextRecord ||
       (exception->ExceptionRecord->ExceptionCode!=EXCEPTION_ACCESS_VIOLATION && exception->ExceptionRecord->ExceptionCode!=EXCEPTION_IN_PAGE_ERROR) ||
       InterlockedCompareExchange(&g_firstChanceBusy,1,0))return EXCEPTION_CONTINUE_SEARCH;
    const DWORD error=GetLastError();
    __try {
        if(g_firstChanceRecords<64) {
            InterlockedIncrement(&g_firstChanceRecords);
            const auto* e=exception->ExceptionRecord;const auto* c=exception->ContextRecord;
            uint64_t stack[16]={};const bool stackKnown=EliteCopy(reinterpret_cast<void*>(c->Rsp),stack,sizeof(stack));
            const uintptr_t actor=g_permissionContext?g_permissionContext->position.actor:0;
            char line[3200]={};
            const int n=sprintf_s(line,"{\"traceVersion\":1,\"processId\":%lu,\"runTick\":%llu,\"tick\":%llu,\"thread\":%lu,\"exceptionCode\":%lu,\"operation\":%llu,\"faultAddress\":%llu,\"imageBase\":%llu,\"dllBase\":%llu,\"registers\":[%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu],\"eflags\":%lu,\"stackKnown\":%s,\"stackWords\":[%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu],\"permissionArmed\":%s,\"collisionArmed\":%s,\"actor\":%llu,\"firstChanceOnly\":true,\"continueSearch\":true,\"readOnly\":true}\r\n",
                GetCurrentProcessId(),g_eliteTraceRunTick,GetTickCount64(),GetCurrentThreadId(),e->ExceptionCode,
                static_cast<unsigned long long>(e->NumberParameters?e->ExceptionInformation[0]:0),static_cast<unsigned long long>(e->NumberParameters>1?e->ExceptionInformation[1]:0),
                static_cast<unsigned long long>(g_firstChanceImage),static_cast<unsigned long long>(g_firstChanceModule),
                c->Rip,c->Rsp,c->Rbp,c->Rax,c->Rbx,c->Rcx,c->Rdx,c->Rsi,c->Rdi,c->R8,c->R9,c->R10,c->R11,c->R12,c->R13,c->R14,c->R15,c->EFlags,
                stackKnown?"true":"false",stack[0],stack[1],stack[2],stack[3],stack[4],stack[5],stack[6],stack[7],stack[8],stack[9],stack[10],stack[11],stack[12],stack[13],stack[14],stack[15],
                g_permissionContext?"true":"false",g_collisionContext?"true":"false",static_cast<unsigned long long>(actor));
            HANDLE file=n>0?CreateFileW(g_firstChancePath,FILE_APPEND_DATA,FILE_SHARE_READ|FILE_SHARE_WRITE,nullptr,OPEN_ALWAYS,FILE_ATTRIBUTE_NORMAL,nullptr):INVALID_HANDLE_VALUE;
            if(file!=INVALID_HANDLE_VALUE) {DWORD written=0;WriteFile(file,line,static_cast<DWORD>(n),&written,nullptr);CloseHandle(file);}
        }
    }
    __except(GetExceptionCode()==EXCEPTION_ACCESS_VIOLATION || GetExceptionCode()==EXCEPTION_IN_PAGE_ERROR?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {}
    InterlockedExchange(&g_firstChanceBusy,0);SetLastError(error);return EXCEPTION_CONTINUE_SEARCH;
}
static bool EliteStartFirstChanceTrace(unsigned char* image,uintptr_t module,const wchar_t* path) {
    if(g_firstChanceHandler || !image || !module || !path || !path[0] || wcslen(path)>=ARRAYSIZE(g_firstChancePath))return false;
    wcscpy_s(g_firstChancePath,path);g_firstChanceImage=reinterpret_cast<uintptr_t>(image);g_firstChanceModule=module;
    g_firstChanceHandler=AddVectoredExceptionHandler(1,EliteFirstChanceObserve);
    return g_firstChanceHandler!=nullptr;
}
