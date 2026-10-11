#include "graphics_failure_trace.h"
#include <assert.h>
static DWORD gotHR=0;static uintptr_t gotContext=0,gotResource=0,gotOwner=0;static uint64_t gotBytes=0,gotOffset=0;static int calls=0;
static void __fastcall Capture(DWORD hr,uintptr_t context,uintptr_t resource,uintptr_t owner,uint64_t bytes,uint64_t offset){
    gotHR=hr;gotContext=context;gotResource=resource;gotOwner=owner;gotBytes=bytes;gotOffset=offset;++calls;
}
int main(){
    auto page=(unsigned char*)VirtualAlloc(nullptr,4096,MEM_RESERVE|MEM_COMMIT,PAGE_EXECUTE_READWRITE);assert(page);
    // Private native branch fixture: six ABI arguments, live nonvolatile
    // context/resource/owner/offset registers, and the real +88 byte-count slot.
    const unsigned char setup[]={0x53,0x56,0x57,0x41,0x56,0x48,0x81,0xec,0xa8,0,0,0,
        0x48,0x8b,0xda,0x49,0x8b,0xf0,0x4d,0x8b,0xf1,0x8b,0xc1,
        0x4c,0x8b,0x94,0x24,0xf0,0,0,0,0x4c,0x89,0x94,0x24,0x88,0,0,0,
        0x48,0x8b,0xbc,0x24,0xf8,0,0,0};
    memcpy(page,setup,sizeof(setup));auto branch=page+sizeof(setup);
    unsigned char head[]={0x48,0x8d,0x15,0,0,0,0};assert(Rel32(head+3,(uintptr_t)branch+7,(uintptr_t)page+512));
    memcpy(branch,head,sizeof(head));
    const unsigned char finish[]={0x48,0x81,0xc4,0xa8,0,0,0,0x41,0x5e,0x5f,0x5e,0x5b,0xc3};memcpy(branch+7,finish,sizeof(finish));
    using Fixture=DWORD(__fastcall*)(DWORD,uintptr_t,uintptr_t,uintptr_t,uint64_t,uint64_t);auto fixture=(Fixture)page;
    assert(fixture(0x887a0005,11,22,33,44,55)==0x887a0005&&calls==0);
    InlineHook hook={};assert(GraphicsFailureTrace::Install(hook,branch,head,sizeof(head),&Capture));
    const DWORD hrs[]={0x887a0005,0x80070057,0xffffffff,0x887a0006};
    for(int i=0;i<4;i++){
        uint64_t bytes=0x100000000ull+(uint64_t)i,offset=0x200000000ull+(uint64_t)i;
        assert(fixture(hrs[i],101+i,202+i,303+i,bytes,offset)==hrs[i]);
        assert(gotHR==hrs[i]&&gotContext==101+i&&gotResource==202+i&&gotOwner==303+i&&gotBytes==bytes&&gotOffset==offset);
    }
    assert(calls==4&&RestoreHook(hook)&&memcmp(branch,head,sizeof(head))==0);
    assert(fixture(3,1,2,3,4,5)==3&&calls==4);
    head[0]^=1;InlineHook wrong={};assert(!GraphicsFailureTrace::Install(wrong,branch,head,sizeof(head),&Capture));assert(!wrong.installed);
    assert(VirtualFree(page,0,MEM_RELEASE));
    puts("PASS graphics failure branch: exact six ABI values, 64-bit count/offset, negative HRESULT and native continuation preserved, relocation/restore/mismatch");
}
