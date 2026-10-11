#include "inline_hook.h"
#include <stdio.h>
#include <assert.h>
static uintptr_t Replacement(){return 99;}
int main(){
    auto p=(unsigned char*)VirtualAlloc(nullptr,4096,MEM_RESERVE|MEM_COMMIT,PAGE_READWRITE);
    assert(p); uintptr_t value=32; memcpy(p+128,&value,8);
    unsigned char code[]={0x48,0x83,0xec,0x28,0x48,0x8b,0x0d,0,0,0,0,
        0x48,0x8b,0xc1,0x48,0x83,0xc4,0x28,0xc3};
    assert(Rel32(code+7,(uintptr_t)p+11,(uintptr_t)p+128)); memcpy(p,code,sizeof(code));
    DWORD old; assert(VirtualProtect(p,4096,PAGE_EXECUTE_READ,&old));
    auto fn=(uintptr_t(*)())p; assert(fn()==32);
    InlineHook h; assert(PrepareHook(h,p,code,11,(void*)&Replacement,7,11));
    assert(((uintptr_t(*)())h.original())()==32); // RIP-relative relocation.
    assert(PublishHook(h)); assert(fn()==99); assert(RestoreHook(h)); assert(fn()==32);
    auto bad=code; bad[0]^=1; InlineHook rejected;
    assert(!PrepareHook(rejected,p,bad,11,(void*)&Replacement,7,11));
    assert(!Readable((void*)1,8)); unsigned char rel[4]; assert(!Rel32(rel,0,0x100000000ULL));
    // A native CALL site must return to its original following instruction.
    auto q=(unsigned char*)VirtualAlloc(nullptr,4096,MEM_RESERVE|MEM_COMMIT,PAGE_READWRITE); assert(q);
    unsigned char caller[]={0x48,0x83,0xec,0x28,0xe8,0,0,0,0,0x48,0x83,0xc4,0x28,0xc3};
    unsigned char callee[]={0xb8,0x20,0,0,0,0xc3}; memcpy(q+128,callee,sizeof(callee));
    assert(Rel32(caller+5,(uintptr_t)q+9,(uintptr_t)q+128)); memcpy(q,caller,sizeof(caller));
    assert(VirtualProtect(q,4096,PAGE_EXECUTE_READ,&old)); InlineHook call;
    assert(PrepareHook(call,q+4,caller+4,5,(void*)&Replacement)); assert(PublishHook(call,true));
    assert(((uintptr_t(*)())q)()==99); assert(RestoreHook(call)); assert(((uintptr_t(*)())q)()==32);
    puts("PASS inline relocation, original trampoline, CALL return, rollback, mismatch, unreadable, rel32 range");
    return 0;
}
