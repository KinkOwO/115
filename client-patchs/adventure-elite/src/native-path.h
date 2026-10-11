#pragma once
#include "catchup-policy.h"
// 145DCE4D0 -> 146D64950 -> 146D64820. Use the same search, with a
// private work arena: a getter may be called while the global A* arena is busy.
using ElitePathSearch=uintptr_t (__fastcall *)(uintptr_t,int,int,int,int,int,char,uintptr_t);
using ElitePathCell=bool (__fastcall *)(uintptr_t,int,int,int,char,uintptr_t);
using ElitePathAllocate=uintptr_t (__fastcall *)(uintptr_t);
static ElitePathSearch g_nativePathSearch;
static ElitePathCell g_nativePathCell;
static ElitePathAllocate g_nativePathAllocate;
alignas(16) static unsigned char g_nativePathArena[114944];
static __declspec(thread) bool g_nativePathActive;
static __declspec(thread) unsigned int g_nativePathAllocations;
static const char* g_nativePathReason="not-queried";
static int g_nativePathWidth,g_nativePathHeight;
static constexpr DWORD kElitePathPoolLimit=0xE1150001;
static constexpr unsigned char kElitePathBytes0[]={0x48,0x83,0xec,0x28,0x4c,0x8b,0x81,0x8,0x1,0x0,0x0,0x4d,0x85,0xc0,0x74,0x17,0x80,0xb9,0x10,0x1,0x0,0x0,0x0,0x74};
static constexpr unsigned char kElitePathBytes1[]={0x48,0x89,0x5c,0x24,0x8,0x48,0x89,0x6c,0x24,0x10,0x48,0x89,0x74,0x24,0x18,0x57,0x41,0x54,0x41,0x55,0x41,0x56,0x41,0x57};
static constexpr unsigned char kElitePathBytes2[]={0x48,0x89,0x5c,0x24,0x8,0x48,0x89,0x6c,0x24,0x10,0x48,0x89,0x74,0x24,0x18,0x57,0x41,0x56,0x41,0x57,0x48,0x83,0xec,0x30};
static constexpr unsigned char kElitePathBytes3[]={0x48,0x89,0x5c,0x24,0x8,0x48,0x89,0x6c,0x24,0x18,0x48,0x89,0x74,0x24,0x20,0x57,0x41,0x54,0x41,0x55,0x41,0x56,0x41,0x57};
static constexpr unsigned char kElitePathBytes4[]={0x8b,0x81,0xe8,0xc0,0x1,0x0,0x48,0x8b,0xd1,0x3d,0x0,0x4,0x0,0x0,0x7c,0x8,0x33,0xc0,0x89,0x81,0xe8,0xc0,0x1,0x0};
static constexpr unsigned char kElitePathBytes5[]={0x40,0x56,0x57,0x41,0x55,0x41,0x56,0x41,0x57,0x48,0x83,0xec,0x30,0x48,0xc7,0x44,0x24,0x20,0xfe,0xff,0xff,0xff,0x48,0x89};
static constexpr unsigned char kElitePathBytes6[]={0x8b,0x81,0x10,0xf,0x0,0x0,0xc3,0xcc,0xcc,0xcc,0xcc,0xcc,0xcc,0xcc,0xcc,0xcc,0x48,0x8b,0x81,0x8,0x20,0x0,0x0,0xc3};
static constexpr unsigned char kElitePathBytes7[]={0x8b,0x81,0x0,0x9,0x0,0x0,0xc3,0xcc,0xcc,0xcc,0xcc,0xcc,0xcc,0xcc,0xcc,0xcc,0x48,0x89,0x5c,0x24,0x8,0x57,0x48,0x83};
static constexpr unsigned char kElitePathBytes8[]={0x41,0x81,0xff,0xf4,0x1,0x0,0x0};
struct ElitePathGuard {uintptr_t rva;const unsigned char* bytes;size_t size;};
static constexpr ElitePathGuard kElitePathGuards[]={
    {0x5e08940,kElitePathBytes0,sizeof(kElitePathBytes0)},
    {0x6d64820,kElitePathBytes1,sizeof(kElitePathBytes1)},
    {0x6d64330,kElitePathBytes2,sizeof(kElitePathBytes2)},
    {0x6d64b70,kElitePathBytes3,sizeof(kElitePathBytes3)},
    {0x6d642c0,kElitePathBytes4,sizeof(kElitePathBytes4)},
    {0x6d64e00,kElitePathBytes5,sizeof(kElitePathBytes5)},
    {0x5d714e0,kElitePathBytes6,sizeof(kElitePathBytes6)},
    {0x5d70740,kElitePathBytes7,sizeof(kElitePathBytes7)},
    {0x6d6491b,kElitePathBytes8,sizeof(kElitePathBytes8)}
};
static uintptr_t __fastcall ElitePathAllocateObserve(uintptr_t arena) {
    // Native node pool wraps at 1024. Stop only our isolated query before it
    // overwrites a live node; all original AI calls are forwarded unchanged.
    if(g_nativePathActive && arena==reinterpret_cast<uintptr_t>(g_nativePathArena) &&
        ++g_nativePathAllocations>1024)RaiseException(kElitePathPoolLimit,0,0,nullptr);
    return g_nativePathAllocate(arena);
}
static ElitePathResult EliteNativePathSearch(int sx,int sy,int tx,int ty,int kind,char pass,uintptr_t actor) {
    if(sx==tx && sy==ty) {g_nativePathReason="same-cell";return ElitePathResult::Reachable;}
    uintptr_t node=0,open=0;int endpoint[2]={};bool limited=false;
    g_nativePathAllocations=0;g_nativePathActive=true;
    __try {
        __try {
            node=g_nativePathSearch(reinterpret_cast<uintptr_t>(g_nativePathArena),sx,sy,tx,ty,kind,pass,actor);
            if(node && !EliteCopy(reinterpret_cast<void*>(node+12),endpoint,sizeof(endpoint)))limited=true;
            memcpy(&open,g_nativePathArena+112,8);
        } __except(GetExceptionCode()==kElitePathPoolLimit?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {limited=true;}
    } __finally {g_nativePathActive=false;}
    auto result=ElitePathClassify(node!=0,endpoint[0]==tx && endpoint[1]==ty,open==0,limited);
    g_nativePathReason=limited?"node-pool-limit":node?(result==ElitePathResult::Reachable?"goal-reached":"cost-bound"):
        open?"search-limit":"open-list-exhausted";
    return result;
}
static ElitePathResult EliteNativePathToPlayer(uintptr_t actor,int px,int py,int ax,int ay) {
    g_nativePathReason="map-or-navigation-unavailable";g_nativePathWidth=0;g_nativePathHeight=0;
    if(!EliteCatchupCoordinate(px) || !EliteCatchupCoordinate(py) || !EliteCatchupCoordinate(ax) || !EliteCatchupCoordinate(ay))return ElitePathResult::Unknown;
    int width=0,height=0;uintptr_t tiles=0,vtable=0,kindGetter=0,passGetter=0;
    auto grid=g_prepareImage+0xE684F00;
    if(!EliteCopy(grid+114924,&width,4) || !EliteCopy(grid+114928,&height,4) || !EliteCopy(grid+114936,&tiles,8) ||
        width<=0 || height<=0 || width>4096 || height>4096 || static_cast<int64_t>(width)*height>1048576 || !tiles ||
        !EliteReadable(reinterpret_cast<const unsigned char*>(tiles),static_cast<size_t>(width)*height) ||
        !EliteCopy(reinterpret_cast<void*>(actor),&vtable,8) || !vtable ||
        !EliteCopy(reinterpret_cast<void*>(vtable+2632),&kindGetter,8) || kindGetter!=reinterpret_cast<uintptr_t>(g_prepareImage+0x5D714E0) ||
        !EliteCopy(reinterpret_cast<void*>(vtable+2896),&passGetter,8) || passGetter!=reinterpret_cast<uintptr_t>(g_prepareImage+0x5D70740))return ElitePathResult::Unknown;
    const int sx=ax/16,sy=ay/12,tx=px/16,ty=py/12;
    g_nativePathWidth=width;g_nativePathHeight=height;
    if(sx<0 || sy<0 || tx<0 || ty<0 || sx>=width || tx>=width || sy>=height || ty>=height) {g_nativePathReason="outside-current-map";return ElitePathResult::Unknown;}
    const int kind=reinterpret_cast<int (__fastcall *)(uintptr_t)>(kindGetter)(actor);
    const char pass=reinterpret_cast<int (__fastcall *)(uintptr_t)>(passGetter)(actor)>0?1:0;
    memset(g_nativePathArena,0,sizeof(g_nativePathArena));
    memcpy(g_nativePathArena+114924,&width,4);memcpy(g_nativePathArena+114928,&height,4);memcpy(g_nativePathArena+114936,&tiles,8);
    // A blocked/moving destination is not proof of no route to the owner.
    if(!g_nativePathCell(reinterpret_cast<uintptr_t>(g_nativePathArena),tx,ty,kind,pass,actor)) {g_nativePathReason="goal-cell-blocked";return ElitePathResult::Unknown;}
    return EliteNativePathSearch(sx,sy,tx,ty,kind,pass,actor);
}
static bool EliteStartNativePath(unsigned char* image) {
    for(auto& guard:kElitePathGuards)if(!EliteReadable(image+guard.rva,guard.size) || memcmp(image+guard.rva,guard.bytes,guard.size))return false;
    g_nativePathSearch=reinterpret_cast<ElitePathSearch>(image+0x6D64820);
    g_nativePathCell=reinterpret_cast<ElitePathCell>(image+0x6D64330);
    if(MH_CreateHook(image+0x6D642C0,reinterpret_cast<void*>(ElitePathAllocateObserve),reinterpret_cast<void**>(&g_nativePathAllocate))!=MH_OK)return false;
    if(MH_EnableHook(image+0x6D642C0)!=MH_OK) {MH_RemoveHook(image+0x6D642C0);return false;}
    return true;
}
