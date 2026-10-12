#define WIN32_LEAN_AND_MEAN
#define _CRT_SECURE_NO_WARNINGS
#include <windows.h>
#include <tlhelp32.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <wchar.h>

/* The native panel has two pointers at +3a0 and a confirmation window at
 * +3b0. Expanding the loop alone corrupts that window. Keep the original
 * object layout intact and append a separate three-pointer array at +428.
 * Every reviewed array consumer moves together with allocation/destruction.
 * No packet writer, source operation, or confirmation dialog is replaced. */
typedef struct { DWORD rva; unsigned len; unsigned char before[16], after[16]; unsigned hook; } Patch;
#include "patches.h"
#define COUNT(a) (sizeof(a)/sizeof((a)[0]))
#define CARD_ARRAY 0x428
#define PANEL_SIZE 0x440
#define WINDOW_MANAGER_RVA 0xe683c78
static HMODULE self;
static unsigned char *image;
static wchar_t directory[MAX_PATH];
static LONG started, initialized;
static BOOL enabled;
typedef uintptr_t (__fastcall *Native4)(void*,uintptr_t,uintptr_t,uintptr_t);
static Native4 originalCtor, originalInit;

static void status(const char *reason) {
    wchar_t path[MAX_PATH];
    FILE *f;
    _snwprintf_s(path,COUNT(path),_TRUNCATE,L"%sapocalypse-third-card.status.json",directory);
    f=_wfopen(path,L"wb");
    if(f){fprintf(f,"{\"ready\":%s,\"enabled\":%s,\"initializedPanels\":%ld,\"rejectReason\":\"%s\"}\n",enabled?"true":"false",enabled?"true":"false",initialized,reason);fclose(f);}
    _snwprintf_s(path,COUNT(path),_TRUNCATE,L"%sapocalypse-third-card.log",directory);
    f=_wfopen(path,L"ab");
    if(f){fprintf(f,"enabled=%u panels=%ld %s\n",enabled,initialized,reason);fclose(f);}
}

static uintptr_t __fastcall panelCtor(void *p,uintptr_t a,uintptr_t b,uintptr_t c) {
    uintptr_t result=originalCtor(p,a,b,c);
    if(result) memset((void*)(result+CARD_ARRAY),0,3*sizeof(void*));
    return result;
}

static uintptr_t __fastcall panelInit(void *p,uintptr_t a,uintptr_t b,uintptr_t c) {
    uintptr_t result=originalInit(p,a,b,c);
    void **cards=(void**)((unsigned char*)p+CARD_ARRAY);
    unsigned i;
    for(i=0;i<3;i++) {
        if(!cards[i] || *(uintptr_t*)cards[i]!=(uintptr_t)(image+0x92e4e60)) {
            status("native-operation-card-resource-unavailable");
            return result;
        }
    }
    /* The current source already contains operation_2 and its own Card_2
     * animation/XUI resources; only its initial enable/visible flags differ.
     * Parent visibility still controls the whole operation-selection screen. */
    {
        uintptr_t *vt=*(uintptr_t**)cards[2];
        typedef void (__fastcall *SetFlag)(void*,BOOL);
        ((SetFlag)vt[0x10/8])(cards[2],TRUE);
        ((SetFlag)vt[0x18/8])(cards[2],TRUE);
    }
    InterlockedIncrement(&initialized);
    status("");
    return result;
}

static BOOL readable(const void *p,SIZE_T size) {
    MEMORY_BASIC_INFORMATION m;
    if(VirtualQuery(p,&m,sizeof(m))!=sizeof(m) || m.State!=MEM_COMMIT || (m.Protect&(PAGE_NOACCESS|PAGE_GUARD)))return FALSE;
    return (uintptr_t)p+size>=(uintptr_t)p && (uintptr_t)p+size<=(uintptr_t)m.BaseAddress+m.RegionSize;
}

static unsigned char *nearPage(void) {
    uintptr_t at=(uintptr_t)image & ~(uintptr_t)0xffff;
    uintptr_t distance;
    for(distance=0x10000;distance<0x70000000;distance+=0x10000) {
        MEMORY_BASIC_INFORMATION m;
        unsigned char *p=(unsigned char*)(at+distance);
        if(VirtualQuery(p,&m,sizeof(m))==sizeof(m) && m.State==MEM_FREE) {
            p=VirtualAlloc(p,4096,MEM_COMMIT|MEM_RESERVE,PAGE_READWRITE);
            if(p)return p;
        }
    }
    return NULL;
}

static BOOL prepareCalls(unsigned char *page) {
    unsigned i;
    for(i=0;i<COUNT(patches);i++) {
        Patch *p=&patches[i];
        if(p->hook) {
            unsigned char *stub=page+(p->hook-1)*16;
            uintptr_t target=(uintptr_t)(p->hook==1?panelCtor:panelInit);
            intptr_t delta=(intptr_t)stub-(intptr_t)(image+p->rva+5);
            int32_t rel;
            if(delta<INT32_MIN || delta>INT32_MAX)return FALSE;
            stub[0]=0x48;stub[1]=0xb8;memcpy(stub+2,&target,8);stub[10]=0xff;stub[11]=0xe0;
            rel=(int32_t)delta;p->after[0]=0xe8;memcpy(p->after+1,&rel,4);
        }
    }
    return TRUE;
}

static BOOL expectedBytes(void) {
    unsigned i;
    for(i=0;i<COUNT(patches);i++) {
        Patch *p=&patches[i];
        if(!readable(image+p->rva,p->len) || memcmp(image+p->rva,p->before,p->len))return FALSE;
    }
    return TRUE;
}

static void resumeThreads(HANDLE *threads,unsigned count) {
    while(count){HANDLE h=threads[--count];ResumeThread(h);CloseHandle(h);}
}

static BOOL suspendThreads(HANDLE *threads,unsigned *count) {
    THREADENTRY32 entry;
    HANDLE snapshot=CreateToolhelp32Snapshot(TH32CS_SNAPTHREAD,0);
    BOOL ok;
    *count=0;
    if(snapshot==INVALID_HANDLE_VALUE)return FALSE;
    entry.dwSize=sizeof(entry);
    ok=Thread32First(snapshot,&entry);
    while(ok) {
        if(entry.th32OwnerProcessID==GetCurrentProcessId() && entry.th32ThreadID!=GetCurrentThreadId()) {
            HANDLE h;
            if(*count==256){CloseHandle(snapshot);resumeThreads(threads,*count);*count=0;return FALSE;}
            h=OpenThread(THREAD_SUSPEND_RESUME,FALSE,entry.th32ThreadID);
            if(!h || SuspendThread(h)==(DWORD)-1) {
                if(h)CloseHandle(h);
                CloseHandle(snapshot);resumeThreads(threads,*count);*count=0;return FALSE;
            }
            threads[(*count)++]=h;
        }
        ok=Thread32Next(snapshot,&entry);
    }
    CloseHandle(snapshot);
    return TRUE;
}

/* Protect every location before changing any instruction. A failed preflight
 * or protection request leaves all original bytes in place. */
static BOOL installBytes(void) {
    DWORD protections[COUNT(patches)];
    unsigned i,j;
    if(!expectedBytes())return FALSE;
    for(i=0;i<COUNT(patches);i++) {
        Patch *p=&patches[i];
        if(!VirtualProtect(image+p->rva,p->len,PAGE_EXECUTE_READWRITE,&protections[i])) {
            for(j=i;j>0;j--){DWORD ignored;VirtualProtect(image+patches[j-1].rva,patches[j-1].len,protections[j-1],&ignored);}
            return FALSE;
        }
    }
    for(i=0;i<COUNT(patches);i++)memcpy(image+patches[i].rva,patches[i].after,patches[i].len);
    FlushInstructionCache(GetCurrentProcess(),NULL,0);
    /* Reverse order matters when multiple locations share the same page. */
    for(i=(unsigned)COUNT(patches);i>0;i--){DWORD ignored;Patch *p=&patches[i-1];VirtualProtect(image+p->rva,p->len,protections[i-1],&ignored);}
    return TRUE;
}

__declspec(dllexport) const char *WINAPI ModName(void){return "Apocalypse third operation card";}
__declspec(dllexport) DWORD WINAPI ModStart(void) {
    HANDLE threads[256];unsigned count=0;DWORD old;unsigned char *page;wchar_t *slash;
    if(InterlockedCompareExchange(&started,1,0))return enabled?0:1;
    if(!GetModuleFileNameW(self,directory,COUNT(directory)))return 1;
    slash=wcsrchr(directory,L'\\');if(!slash)return 1;slash[1]=0;
    image=(unsigned char*)GetModuleHandleW(NULL);
    originalCtor=(Native4)(image+0x6a0720);originalInit=(Native4)(image+0x6a7430);
    if(!expectedBytes()){status("client-instruction-mismatch");return 1;}
    page=nearPage();
    if(!page || !prepareCalls(page) || !VirtualProtect(page,4096,PAGE_EXECUTE_READ,&old)) {
        if(page)VirtualFree(page,0,MEM_RELEASE);status("call-relay-unavailable");return 1;
    }
    FlushInstructionCache(GetCurrentProcess(),page,4096);
    if(!suspendThreads(threads,&count)){VirtualFree(page,0,MEM_RELEASE);status("thread-freeze-unavailable");return 1;}
    /* Fail closed on a late load: existing 0x428-byte panels cannot safely
     * consume the extended array. The window manager must not exist yet. */
    if(readable(image+WINDOW_MANAGER_RVA,sizeof(void*)) && *(void**)(image+WINDOW_MANAGER_RVA)==NULL)enabled=installBytes();
    resumeThreads(threads,count);
    if(!enabled){VirtualFree(page,0,MEM_RELEASE);status("late-load-or-patch-preflight-failed");return 1;}
    status("");return 0;
}

BOOL WINAPI DllMain(HINSTANCE h,DWORD reason,LPVOID reserved) {
    (void)reserved;
    if(reason==DLL_PROCESS_ATTACH){self=h;DisableThreadLibraryCalls(h);}
    return TRUE;
}
