#include "plugin.cpp"
#include <assert.h>
#include <vector>
static std::vector<int> received;
static uintptr_t peer;
static uint32_t eligible=0;
static uintptr_t shape=0;
static int nativeCalls=0;
static uint32_t __fastcall TestSkin(uintptr_t){return eligible;}
static uintptr_t __fastcall TestShape(uintptr_t){return shape;}
static uintptr_t __fastcall TestResolve(uintptr_t,int slot,int flags,unsigned char original){++nativeCalls;received.push_back(slot);return 1000+slot+flags+original;}
static uintptr_t __fastcall TestLookup(uintptr_t,int slot){++nativeCalls;received.push_back(slot);return 2000+slot;}
static void __fastcall TestCycle(uintptr_t actor){
    const bool active=eligible&&shape;
    assert(Resolve(actor,10,7,1)==(active?0:1018));
    assert(Lookup(actor,10)==(active?0:2010));
    assert(Resolve(actor,12,7,1)==1020);
    assert(Lookup(actor,12)==2012);
    assert(Resolve(peer,10,0,0)==1010);
    for(int slot=0;slot<50;++slot)if(slot!=10)assert(Lookup(actor,slot)==2000+(uintptr_t)slot);
}
int main(){
    auto actor=(uintptr_t)VirtualAlloc(nullptr,0x20000,MEM_RESERVE|MEM_COMMIT,PAGE_READWRITE);assert(actor);
    peer=actor+0x10000;
    memset((void*)actor,0xa5,0x20000);
    unsigned char before[0x7000];memcpy(before,(void*)actor,sizeof(before));
    gEnabled=1;gSkin=TestSkin;gShape=TestShape;gResolve=TestResolve;gLookup=TestLookup;gCycle=TestCycle;gExpansion=TestCycle;
    for(unsigned job=0;job<17;++job)for(uint32_t skin:{0u,117020002u,117020007u})for(uintptr_t object:{0ull,303ull}) {
        eligible=skin;shape=object;
        Cycle(actor);assert(!gVisualActor);
        Expansion(actor);assert(!gVisualActor);
        assert(Resolve(actor,10,7,1)==1018 && Lookup(actor,10)==2010);
        {VisualScope outer(actor);auto previous=gVisualActor;{VisualScope nested(peer);assert(gVisualActor==(skin&&object?peer:0));}assert(gVisualActor==previous);}
        assert(!gVisualActor && !memcmp(before,(void*)actor,sizeof(before)));
    }
    eligible=117020002;shape=303;
    {VisualScope scope(actor);assert(!Resolve(actor,10,0,0));eligible=0;{VisualScope cleared(actor);assert(Resolve(actor,10,0,0)==1010);}assert(gVisualActor==actor);}
    assert(!HasShape(0));gEnabled=0;assert(!HasShape(actor));
    assert(!memcmp(before,(void*)actor,sizeof(before)));
    VirtualFree((void*)actor,0,MEM_RELEASE);
    puts("PASS 102 job-independent policy vectors, active/clear/missing instance, native pass-through, actor isolation, nested scopes and state byte preservation; rendered professions not tested");
}
