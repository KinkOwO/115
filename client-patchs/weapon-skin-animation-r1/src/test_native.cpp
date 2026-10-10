#include "plugin.cpp"
#include "native_loop.h"
#include <assert.h>
#include <vector>
#include <utility>
using InvokeFn=void(__fastcall*)(uintptr_t,const int*);
static InvokeFn invoke;
static std::vector<std::pair<int,uintptr_t>> seen;
static uintptr_t __fastcall Equipment(uintptr_t actor,int slot,int,unsigned char){return ((uintptr_t*)(actor+0x2000))[slot];}
static uint32_t __fastcall Skin(uintptr_t actor){return *(uint32_t*)(actor+0x1858);}
static uintptr_t __fastcall Shape(uintptr_t actor){return *(uintptr_t*)(actor+0x6048);}
static void __fastcall Capture(uintptr_t,uintptr_t item,int slot){seen.emplace_back(slot,item);}
static void Write(uintptr_t at,const unsigned char* bytes,size_t n){memcpy((void*)at,bytes,n);}
static void Jump(uintptr_t at,void* target){AbsoluteJump((unsigned char*)at,target);}
static void __fastcall NativeCycle(uintptr_t actor){for(int slot:{10,12})invoke(actor,&slot);}
int main(){
    SetErrorMode(0x8003);
    const uintptr_t pages[]={0x145d10000ull,0x145cd0000ull,0x145c10000ull,0x145c20000ull,0x145d70000ull};
    for(auto page:pages)assert((uintptr_t)VirtualAlloc((void*)page,0x10000,MEM_RESERVE|MEM_COMMIT,PAGE_EXECUTE_READWRITE)==page);
    // Exact original selector, with explicit substitutes for non-local calls.
    unsigned char prolog[]={0x48,0x83,0xec,0x28,0xe9,7,0,0,0};Write(0x145d1f4f0,prolog,sizeof(prolog));
    Write(0x145d1f500,kNativeLoop,sizeof(kNativeLoop));
    unsigned char epilog[]={0x48,0x83,0xc4,0x28,0xc3};Write(0x145d1f573,epilog,sizeof(epilog));
    Jump(0x145cdc8e0,(void*)Resolve);Jump(0x145d71ad0,(void*)Skin);
    Write(0x145c20190,kShapeHead,sizeof(kShapeHead));Jump(0x145d1f940,(void*)Capture);
    unsigned char nil[]={0x31,0xc0,0xc3};Write(0x145c1f580,nil,sizeof(nil));Write(0x145c1f590,nil,sizeof(nil));
    unsigned char wrapper[]={0x53,0x56,0x57,0x41,0x56,0x41,0x57,0x48,0x83,0xec,0x20,0x48,0x89,0xce,0x48,0x89,0xd7,0x48,0xb8,0,0,0,0,0,0,0,0,0xff,0xd0,0x48,0x83,0xc4,0x20,0x41,0x5f,0x41,0x5e,0x5f,0x5e,0x5b,0xc3};
    uintptr_t entry=0x145d1f4f0;memcpy(wrapper+19,&entry,8);
    auto code=(uintptr_t)VirtualAlloc(nullptr,4096,MEM_RESERVE|MEM_COMMIT,PAGE_EXECUTE_READWRITE);assert(code);Write(code,wrapper,sizeof(wrapper));
    invoke=(InvokeFn)code;
    auto actor=(uintptr_t)VirtualAlloc(nullptr,0x7000,MEM_RESERVE|MEM_COMMIT,PAGE_READWRITE);assert(actor);
    ((uintptr_t*)(actor+0x2000))[10]=101;((uintptr_t*)(actor+0x2000))[12]=202;
    gEnabled=1;gSkin=Skin;gShape=Shape;gResolve=Equipment;gCycle=NativeCycle;gExpansion=NativeCycle;
    for(uint32_t skin:{0u,117020002u,117020007u})for(uintptr_t shape:{0ull,303ull}){
        *(uint32_t*)(actor+0x1858)=skin;*(uintptr_t*)(actor+0x6048)=shape;
        unsigned char before[0x7000];memcpy(before,(void*)actor,sizeof(before));
        seen.clear();NativeCycle(actor);
        const uintptr_t weapon=skin&&shape?shape:202;
        std::vector<std::pair<int,uintptr_t>> baseline={{10,101},{12,weapon}};assert(seen==baseline);
        for(auto fn:{Cycle,Expansion}){
            seen.clear();fn(actor);
            auto expected=skin&&shape?std::vector<std::pair<int,uintptr_t>>{{12,weapon}}:baseline;
            assert(seen==expected && !gVisualActor);
            assert(!memcmp(before,(void*)actor,sizeof(before)));
        }
        // Scope exit restores native equipment selection immediately.
        seen.clear();NativeCycle(actor);assert(seen==baseline);
    }
    for(auto page:pages)VirtualFree((void*)page,0,MEM_RELEASE);
    VirtualFree((void*)actor,0,MEM_RELEASE);VirtualFree((void*)code,0,MEM_RELEASE);
    puts("PASS exact native selector: original overlap reproduced; compiled adapter excludes slot10 only with eligible shape, clear/missing restores; actor bytes preserved. Eligibility globals and render services substituted.");
}
