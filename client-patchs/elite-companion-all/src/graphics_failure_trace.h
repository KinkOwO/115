#pragma once
#include "crash_trace.h"
#include "graphics_evidence.h"
#include <psapi.h>

// 1487F64A1 is reached only after FlushBuffer's native vtable+70 call returns
// a negative HRESULT. EAX is still the error; RBX=context, RSI=resource,
// R14=owner+98, RDI=offset, and [RSP+88]=bytes, as proved in current PE.
// Save volatile GPRs, flags and XMM0..5; observe; then execute the relocated
// original LEA and its unmodified logic_error constructor/throw continuation.
namespace GraphicsFailureTrace {
using Observer=void(__fastcall*)(DWORD,uintptr_t,uintptr_t,uintptr_t,uint64_t,uint64_t);
struct Code {
    unsigned char bytes[320]={};size_t size=0;
    void Raw(const unsigned char* data,size_t n){memcpy(bytes+size,data,n);size+=n;}
    void Byte(unsigned char v){bytes[size++]=v;}
    void Dword(uint32_t v){Raw((unsigned char*)&v,4);}
    void Pointer(const void* v){Raw((unsigned char*)&v,8);}
    void Xmm(unsigned char op,int reg,uint32_t offset){
        Byte(0xf3);Byte(0x0f);Byte(op);Byte((unsigned char)(0x84+reg*8));Byte(0x24);Dword(offset);
    }
};
static bool Install(InlineHook& hook,void* site,const unsigned char* head,size_t n,Observer observer) {
    if(!PrepareHook(hook,site,head,n,(void*)observer,3,7))return false;
    Code c;
    const unsigned char save[]={0x9c,0x50,0x51,0x52,0x41,0x50,0x41,0x51,0x41,0x52,0x41,0x53,0x48,0x81,0xec,0x90,0,0,0};c.Raw(save,sizeof(save));
    for(int i=0;i<6;i++)c.Xmm(0x7f,i,0x30+i*16);
    const unsigned char args[]={
        0x8b,0x8c,0x24,0xc0,0,0,0, // ECX=saved EAX
        0x48,0x8b,0xd3,             // RDX=RBX
        0x4c,0x8b,0xc6,             // R8=RSI
        0x4d,0x8b,0xce,             // R9=R14
        0x4c,0x8b,0x94,0x24,0x58,1,0,0, // original [RSP+88]
        0x4c,0x89,0x54,0x24,0x20,
        0x48,0x89,0x7c,0x24,0x28,
        0xff,0x15,2,0,0,0,0xeb,8};c.Raw(args,sizeof(args));c.Pointer((void*)observer);
    for(int i=0;i<6;i++)c.Xmm(0x6f,i,0x30+i*16);
    const unsigned char restore[]={0x48,0x81,0xc4,0x90,0,0,0,0x41,0x5b,0x41,0x5a,0x41,0x59,0x41,0x58,0x5a,0x59,0x58,0x9d,0xff,0x25,0,0,0,0};c.Raw(restore,sizeof(restore));c.Pointer(hook.page);
    unsigned char jump[14]={};AbsoluteJump(jump,hook.page+128);
    if(c.size>sizeof(c.bytes)||!WriteCode(hook.page+128,c.bytes,c.size)||!WriteCode(hook.page+64,jump,sizeof(jump)))return false;
    return PublishHook(hook);
}
static InlineHook hook;
static void __fastcall Observe(DWORD hr,uintptr_t context,uintptr_t resource,uintptr_t owner,uint64_t bytes,uint64_t offset) {
    PROCESS_MEMORY_COUNTERS_EX memory={};memory.cb=sizeof(memory);
    bool measured=GetProcessMemoryInfo(GetCurrentProcess(),(PROCESS_MEMORY_COUNTERS*)&memory,sizeof(memory))!=FALSE;
    uintptr_t vt=0,map=0;MEMORY_BASIC_INFORMATION image={};wchar_t module[MAX_PATH]={};
    if(Readable((void*)context,8))memcpy(&vt,(void*)context,8);
    if(vt&&Readable((void*)(vt+0x70),8))memcpy(&map,(void*)(vt+0x70),8);
    if(map&&VirtualQuery((void*)map,&image,sizeof(image))==sizeof(image)&&image.Type==MEM_IMAGE)
        GetModuleFileNameW((HMODULE)image.AllocationBase,module,MAX_PATH);
    const wchar_t* name=wcsrchr(module,L'\\');name=name?name+1:module;
    char line[1024]={};
    _snprintf_s(line,sizeof(line),_TRUNCATE,
        "tick=%lu tid=%lu GRAPHICS_FLUSH_FAILURE dungeon=%ld hresult=0x%08lx context=%p resource=%p owner_plus98=%p bytes=%llu offset=%llu map_module=%ls map_rva=0x%llx memory_valid=%d private_mib=%llu peak_commit_mib=%llu; native logic_error throw unchanged\r\n",
        GetTickCount(),GetCurrentThreadId(),CrashTrace::dungeon?*CrashTrace::dungeon:0,hr,(void*)context,(void*)resource,(void*)owner,
        (unsigned long long)bytes,(unsigned long long)offset,name,(unsigned long long)(image.AllocationBase?map-(uintptr_t)image.AllocationBase:0),
        measured,(unsigned long long)(memory.PrivateUsage/1048576),(unsigned long long)(memory.PeakPagefileUsage/1048576));
    CrashTrace::Line(line);
    if(CrashTrace::file!=INVALID_HANDLE_VALUE)FlushFileBuffers(CrashTrace::file);
}
static bool Start(uintptr_t base){return Install(hook,(void*)(base+kGraphicsFailureRVA),kGraphicsFailureHead,sizeof(kGraphicsFailureHead),&Observe);}
}
