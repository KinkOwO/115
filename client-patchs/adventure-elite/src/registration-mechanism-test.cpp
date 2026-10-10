#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdio.h>
#include <stdint.h>
#include <string.h>
static bool EliteReadable(const void* p,size_t n) {MEMORY_BASIC_INFORMATION m={};return p && VirtualQuery(p,&m,sizeof(m)) && m.State==MEM_COMMIT && !(m.Protect&(PAGE_GUARD|PAGE_NOACCESS)) && n<=m.RegionSize-(reinterpret_cast<uintptr_t>(p)-reinterpret_cast<uintptr_t>(m.BaseAddress));}
#include "registration-policy.h"
#include "condition-patch.h"
extern "C" void EliteRelayFixture(uintptr_t,uintptr_t*);
static bool throwing;
extern "C" unsigned char EliteConditionDecision(uintptr_t value,uintptr_t caller) {
    if(!caller)RaiseException(0xE1234000,0,0,nullptr);
    if(throwing)RaiseException(0xE1234567,0,0,nullptr);
    SetLastError(ERROR_ACCESS_DENIED);return static_cast<unsigned char>(value);
}
static volatile LONG running=1,iterations;
static DWORD WINAPI Peer(void*) {while(InterlockedCompareExchange(&running,0,0)) {InterlockedIncrement(&iterations);Sleep(1);}return 0;}
int main() {
    EliteRegistrationFacts f={true,true,true,true,true,true,true,true,true,true,2};
    if(!EliteRegistrationAllowed(f))return 1;
    bool EliteRegistrationFacts::*fields[]={&EliteRegistrationFacts::ready,&EliteRegistrationFacts::identity,&EliteRegistrationFacts::world,&EliteRegistrationFacts::dungeon,&EliteRegistrationFacts::scene,&EliteRegistrationFacts::stable,&EliteRegistrationFacts::nativePlayer,&EliteRegistrationFacts::references,&EliteRegistrationFacts::notRegistered,&EliteRegistrationFacts::managerIdle};
    for(auto field:fields) {auto bad=f;bad.*field=false;if(EliteRegistrationAllowed(bad))return 2;}
    f.alive=0;if(EliteRegistrationAllowed(f))return 3;f.alive=4;if(EliteRegistrationAllowed(f))return 4;
    const uintptr_t values[]={0,1,128,255};
    for(auto value:values) {
        uintptr_t out[20]={};EliteRelayFixture(value,out);
        const uintptr_t expected[]={value,0x11,0x22,0x33,0x44,0x55,0x66};
        for(size_t i=0;i<7;++i)if(out[i]!=expected[i])return 5;
        for(size_t i=8;i<20;++i)if(out[i]!=UINT64_MAX)return 6;
        uintptr_t flags=(value==0?0x40:0) | (value&128?0x80:0);
        unsigned int bits=0;for(unsigned int n=0;n<8;++n)bits+=static_cast<unsigned int>((value>>n)&1);
        if(!(bits&1))flags|=4;
        if((out[7]&0x8D5)!=flags || GetLastError()!=ERROR_ACCESS_DENIED)return 7;
    }
    throwing=true;bool caught=false;uintptr_t out[20]={};
    __try {EliteRelayFixture(1,out);} __except(GetExceptionCode()==0xE1234567?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {caught=true;}
    if(!caught)return 8;throwing=false;
    auto page=static_cast<unsigned char*>(VirtualAlloc(nullptr,4096,MEM_RESERVE|MEM_COMMIT,PAGE_READWRITE));if(!page)return 9;
    const unsigned char before[]={0x80,0xb8,0x78,0x18,0,0,0},after[]={0xe8,1,2,3,4,0x90,0x90};
    memcpy(page,before,7);DWORD old=0;if(!VirtualProtect(page,4096,PAGE_EXECUTE_READ,&old))return 10;
    HANDLE peer=CreateThread(nullptr,0,Peer,nullptr,0,nullptr);if(!peer)return 11;Sleep(10);
    bool installed=false;
    if(!EliteConditionPatch(page,before,after,&installed) || !installed || memcmp(page,after,7))return 12;
    MEMORY_BASIC_INFORMATION memory={};if(!VirtualQuery(page,&memory,sizeof(memory)) || memory.Protect!=PAGE_EXECUTE_READ)return 13;
    if(EliteConditionPatch(page,before,after) || memcmp(page,after,7))return 14;
    if(!EliteConditionPatch(page,after,before) || memcmp(page,before,7))return 15;
    LONG tick=iterations;Sleep(20);if(iterations<=tick)return 16;
    InterlockedExchange(&running,0);WaitForSingleObject(peer,1000);CloseHandle(peer);VirtualFree(page,0,MEM_RELEASE);
    puts("registration mechanism: all scope gates, CMP flags, volatile GPR/XMM preservation, SEH unwind, suspended-peer patch/mismatch/restore and RX protection passed");return 0;
}
