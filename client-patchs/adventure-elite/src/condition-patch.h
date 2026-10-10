#pragma once
#include <tlhelp32.h>
extern "C" {
#include "../vendor/minhook/src/buffer.h"
}
// Code changes occur only after all enumerated peer threads are suspended.
// A peer inside the seven-byte site, or any inspection failure, rejects the patch.
static bool EliteConditionPatch(unsigned char* site,const unsigned char* expected,const unsigned char* replacement,bool* installed=nullptr) {
    if(installed)*installed=false;
    HANDLE peers[512]={};size_t count=0;bool ok=true;
    HANDLE snapshot=CreateToolhelp32Snapshot(TH32CS_SNAPTHREAD,0);
    if(snapshot==INVALID_HANDLE_VALUE)return false;
    THREADENTRY32 entry={};entry.dwSize=sizeof(entry);
    if(!Thread32First(snapshot,&entry))ok=false;
    else do {
        if(entry.th32OwnerProcessID!=GetCurrentProcessId() || entry.th32ThreadID==GetCurrentThreadId())continue;
        HANDLE thread=OpenThread(THREAD_SUSPEND_RESUME|THREAD_GET_CONTEXT|THREAD_QUERY_INFORMATION,FALSE,entry.th32ThreadID);
        if(!thread) {if(GetLastError()==ERROR_INVALID_PARAMETER)continue;ok=false;break;}
        if(count==512 || SuspendThread(thread)==DWORD(-1)) {CloseHandle(thread);ok=false;break;}
        peers[count++]=thread;CONTEXT context={};context.ContextFlags=CONTEXT_CONTROL;
        if(!GetThreadContext(thread,&context) || (context.Rip>=reinterpret_cast<uintptr_t>(site) && context.Rip<reinterpret_cast<uintptr_t>(site)+7)) {ok=false;break;}
        entry.dwSize=sizeof(entry);
    } while(Thread32Next(snapshot,&entry));
    if(ok && GetLastError()!=ERROR_NO_MORE_FILES)ok=false;
    CloseHandle(snapshot);
    DWORD protection=0,ignored=0;
    if(ok) {
        ok=EliteReadable(site,7) && !memcmp(site,expected,7) && VirtualProtect(site,7,PAGE_EXECUTE_READWRITE,&protection);
        if(ok) {
            memcpy(site,replacement,7);
            ok=FlushInstructionCache(GetCurrentProcess(),site,7)!=0;
            if(!ok || !VirtualProtect(site,7,protection,&ignored)) {
                memcpy(site,expected,7);FlushInstructionCache(GetCurrentProcess(),site,7);
                VirtualProtect(site,7,protection,&ignored);ok=false;
            }
            if(ok && installed)*installed=true;
        }
    }
    while(count) {HANDLE thread=peers[--count];if(ResumeThread(thread)==DWORD(-1))ok=false;CloseHandle(thread);}
    return ok;
}
