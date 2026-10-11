#pragma once
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdint.h>
#include <string.h>
#include <limits.h>

// Only complete, audited instructions may be supplied. No generic decoder.
struct InlineHook {
    unsigned char* site = nullptr;
    unsigned char* page = nullptr;
    unsigned char expected[16] = {};
    size_t length = 0;
    bool installed = false;
    void* original() const { return page; }
};
inline bool Readable(const void* p, size_t n) {
    MEMORY_BASIC_INFORMATION m = {};
    if (!p || !n || VirtualQuery(p, &m, sizeof(m)) != sizeof(m) || m.State != MEM_COMMIT ||
        (m.Protect & (PAGE_GUARD | PAGE_NOACCESS))) return false;
    return (uintptr_t)p >= (uintptr_t)m.BaseAddress && n <= m.RegionSize &&
        (uintptr_t)p - (uintptr_t)m.BaseAddress <= m.RegionSize - n;
}
inline bool Rel32(unsigned char* out, uintptr_t next, uintptr_t target) {
    int64_t d = (int64_t)target - (int64_t)next;
    if (d < INT32_MIN || d > INT32_MAX) return false;
    int32_t v = (int32_t)d; memcpy(out, &v, 4); return true;
}
inline void AbsoluteJump(unsigned char* at, const void* to) {
    at[0]=0xff; at[1]=0x25; memset(at+2,0,4); memcpy(at+6,&to,8);
}
inline unsigned char* NearPage(uintptr_t site) {
    uintptr_t aligned = site & ~(uintptr_t)0xffff;
    for (uintptr_t d=0x10000; d<0x70000000; d+=0x10000) {
        uintptr_t candidates[2]={aligned+d, aligned>d ? aligned-d : 0};
        for (uintptr_t p : candidates) {
            if (p<0x10000) continue;
            MEMORY_BASIC_INFORMATION m={};
            if (VirtualQuery((void*)p,&m,sizeof(m))!=sizeof(m) || m.State!=MEM_FREE) continue;
            auto page=(unsigned char*)VirtualAlloc((void*)p,4096,MEM_RESERVE|MEM_COMMIT,PAGE_READWRITE);
            if(page) return page;
        }
    }
    return nullptr;
}
// ripDisp is the displacement offset; ripEnd is the end of its instruction.
inline bool PrepareHook(InlineHook& h, void* site, const unsigned char* expected,
                        size_t n, void* hook, size_t ripDisp=0, size_t ripEnd=0) {
    if(n<5 || n>sizeof(h.expected) || !Readable(site,n) || memcmp(site,expected,n)) return false;
    h.site=(unsigned char*)site; h.length=n; memcpy(h.expected,expected,n);
    h.page=NearPage((uintptr_t)site); if(!h.page) return false;
    memcpy(h.page,expected,n);
    if(ripDisp) {
        if(ripDisp+4>n || ripEnd>n) goto fail;
        int32_t old; memcpy(&old,expected+ripDisp,4);
        if(!Rel32(h.page+ripDisp,(uintptr_t)h.page+ripEnd,(uintptr_t)site+ripEnd+old)) goto fail;
    }
    h.page[n]=0xe9;
    if(!Rel32(h.page+n+1,(uintptr_t)h.page+n+5,(uintptr_t)site+n)) goto fail;
    AbsoluteJump(h.page+64,hook);
    { DWORD old;
      if(!VirtualProtect(h.page,4096,PAGE_EXECUTE_READ,&old)) goto fail; }
    FlushInstructionCache(GetCurrentProcess(),h.page,4096); return true;
fail:
    VirtualFree(h.page,0,MEM_RELEASE); h.page=nullptr; return false;
}
inline bool WriteCode(void* site, const void* data, size_t n) {
    DWORD old,ignored;
    if(!Readable(site,n) || !VirtualProtect(site,n,PAGE_EXECUTE_READWRITE,&old)) return false;
    memcpy(site,data,n); FlushInstructionCache(GetCurrentProcess(),site,n);
    // The write succeeded. Keep all relays alive even if restoration fails.
    VirtualProtect(site,n,old,&ignored); return true;
}
inline bool PublishHook(InlineHook& h, bool call=false) {
    if(!h.page || !Readable(h.site,h.length) || memcmp(h.site,h.expected,h.length)) return false;
    unsigned char patch[16]; memset(patch,0x90,sizeof(patch)); patch[0]=call?0xe8:0xe9;
    if(!Rel32(patch+1,(uintptr_t)h.site+5,(uintptr_t)h.page+64)) return false;
    if(!WriteCode(h.site,patch,h.length)) return false;
    h.installed=true; return true;
}
inline bool RestoreHook(InlineHook& h) {
    if(!h.installed) return true;
    unsigned char patch[16]; memset(patch,0x90,sizeof(patch));
    unsigned char current[16];
    if(!Readable(h.site,h.length)) return false;
    memcpy(current,h.site,h.length); patch[0]=current[0];
    if((patch[0]!=0xe9 && patch[0]!=0xe8) || !Rel32(patch+1,(uintptr_t)h.site+5,(uintptr_t)h.page+64) ||
        memcmp(current,patch,h.length)) return false;
    if(!WriteCode(h.site,h.expected,h.length)) return false;
    h.installed=false; return true;
}
