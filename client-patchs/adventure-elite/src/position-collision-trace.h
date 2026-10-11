#pragma once
#include "position-permission-trace.h"

// Observe the calls already made by native collision. Read snapshots only;
// never query another position, execute collision again, or change its result.
using EliteCollision0=uintptr_t (__fastcall *)();
using EliteCollision1=uintptr_t (__fastcall *)(uintptr_t);
using EliteCollision2=uintptr_t (__fastcall *)(uintptr_t,uintptr_t);
using EliteCollision3=uintptr_t (__fastcall *)(uintptr_t,uintptr_t,uintptr_t);
struct EliteCollisionGuard {uintptr_t rva;unsigned char bytes[32];};
static constexpr EliteCollisionGuard kEliteCollisionGuards[]={
{0x4d27990,{0x40,0x55,0x56,0x57,0x41,0x54,0x41,0x55,0x41,0x56,0x41,0x57,0x48,0x83,0xec,0x40,0x48,0xc7,0x44,0x24,0x30,0xfe,0xff,0xff,0xff,0x48,0x89,0x9c,0x24,0x80,0x0,0x0}},
{0x5df2660,{0x48,0x89,0x5c,0x24,0x8,0x48,0x89,0x6c,0x24,0x10,0x48,0x89,0x74,0x24,0x18,0x48,0x89,0x7c,0x24,0x20,0x41,0x56,0x48,0x83,0xec,0x20,0x48,0x8b,0xf2,0x48,0x8b,0xe9}},
{0x5b32610,{0x4c,0x8b,0x91,0xf8,0x1,0x0,0x0,0x4c,0x8b,0xca,0x48,0x8b,0x89,0x0,0x2,0x0,0x0,0x48,0xb8,0xab,0xaa,0xaa,0xaa,0xaa,0xaa,0xaa,0x2a,0x49,0x2b,0xca,0x45,0x33}},
{0x5b2f130,{0x85,0xd2,0x78,0x22,0x48,0x8b,0x81,0xc0,0x1,0x0,0x0,0x48,0x8b,0x89,0xc8,0x1,0x0,0x0,0x48,0x2b,0xc8,0x4c,0x63,0xc2,0x48,0xc1,0xf9,0x3,0x4c,0x3b,0xc1,0x73}},
{0x4d24c40,{0x8b,0x81,0x58,0x40,0x0,0x0,0x48,0x8b,0xd1,0x85,0xc0,0xf,0x85,0xb4,0x0,0x0,0x0,0x48,0x8b,0x81,0xf0,0x24,0x0,0x0,0x48,0x85,0xc0,0x74,0x10,0x80,0x38,0x0}},
{0x4d23f20,{0x48,0x89,0x5c,0x24,0x8,0x57,0x48,0x83,0xec,0x20,0x48,0x8b,0x81,0x28,0x5a,0x0,0x0,0x48,0x8b,0xd9,0x48,0x85,0xc0,0x74,0x78,0x80,0x78,0x60,0x0,0x74,0x72,0x8b}},
{0x4d22b70,{0x48,0x8b,0x81,0x28,0x5a,0x0,0x0,0x48,0x85,0xc0,0x74,0x4f,0x80,0x78,0x60,0x0,0x74,0x49,0x8b,0x50,0x18,0x83,0xfa,0x2,0x75,0x6,0x48,0x63,0x50,0x40,0xeb,0x9}},
{0x4d26b50,{0x48,0x8b,0x81,0x28,0x5a,0x0,0x0,0x4c,0x8b,0xc1,0x48,0x85,0xc0,0x74,0x55,0x80,0x78,0x60,0x0,0x74,0x4f,0x8b,0x50,0x18,0x83,0xfa,0x2,0x75,0x6,0x48,0x63,0x48}},
{0x4d25bf0,{0x40,0x53,0x48,0x83,0xec,0x20,0x48,0x8b,0x81,0x28,0x5a,0x0,0x0,0x48,0x85,0xc0,0x74,0x59,0x80,0x78,0x60,0x0,0x74,0x53,0x8b,0x50,0x18,0x83,0xfa,0x2,0x75,0x6}},
{0x4d25a80,{0x48,0x89,0x5c,0x24,0x8,0x48,0x8b,0x81,0x28,0x5a,0x0,0x0,0x4c,0x8b,0xd9,0x4c,0x63,0xca,0x48,0x85,0xc0,0xf,0x84,0xde,0x0,0x0,0x0,0x80,0x78,0x60,0x0,0xf}},
{0x7211920,{0x4d,0x85,0xc0,0x74,0x24,0x45,0x8b,0x8,0x41,0x3b,0xc9,0x7c,0x1c,0x45,0x3,0x48,0x8,0x41,0x3b,0xc9,0x7d,0x13,0x41,0x8b,0x48,0x4,0x3b,0xd1,0x7c,0xb,0x41,0x3}},
{0x4d28250,{0x48,0x89,0x5c,0x24,0x8,0x48,0x89,0x6c,0x24,0x10,0x48,0x89,0x74,0x24,0x18,0x48,0x89,0x7c,0x24,0x20,0x41,0x56,0x48,0x83,0xec,0x20,0x80,0x79,0x18,0x0,0x48,0x8b}},
{0xeeb380,{0x40,0x57,0x41,0x56,0x41,0x57,0x48,0x83,0xec,0x40,0x48,0xc7,0x44,0x24,0x28,0xfe,0xff,0xff,0xff,0x48,0x89,0x5c,0x24,0x60,0x48,0x89,0x6c,0x24,0x68,0x48,0x89,0x74}},
{0x5494680,{0x40,0x53,0x48,0x83,0xec,0x20,0x48,0x8b,0xd,0x3,0x7a,0x1d,0x9,0xe8,0xee,0x49,0x51,0x0,0x48,0x8b,0xd8,0x48,0x85,0xc0,0x74,0x36,0x48,0x8b,0xc8,0xe8,0x9e,0x8f}},
{0x4d26d70,{0x48,0x83,0xec,0x38,0x48,0xc7,0x44,0x24,0x20,0xfe,0xff,0xff,0xff,0x48,0x8b,0xd,0x34,0x3e,0x91,0x9,0x48,0x85,0xc9,0x75,0x3b,0xb9,0xc8,0x0,0x0,0x0,0xe8,0x8d}},
{0x4d26de0,{0x40,0x53,0x48,0x83,0xec,0x30,0x48,0xc7,0x44,0x24,0x20,0xfe,0xff,0xff,0xff,0x48,0x8b,0x1d,0xc2,0x3d,0x91,0x9,0x48,0x85,0xdb,0x75,0x3b,0xb9,0xc8,0x0,0x0,0x0}},
{0xe0cec0,{0x48,0x83,0xec,0x38,0x48,0xc7,0x44,0x24,0x20,0xfe,0xff,0xff,0xff,0x48,0x8b,0xd,0xe4,0xdc,0x82,0xd,0x48,0x85,0xc9,0x75,0x3b,0xb9,0xc8,0x0,0x0,0x0,0xe8,0x3d}},
{0x5d5efa0,{0x48,0x89,0x5c,0x24,0x8,0x48,0x89,0x6c,0x24,0x10,0x48,0x89,0x74,0x24,0x18,0x57,0x41,0x56,0x41,0x57,0x48,0x83,0xec,0x20,0x41,0xf,0xb6,0xd8,0x48,0x8b,0xf2,0x48}},
{0x5b8c7d0,{0x48,0x8b,0x89,0x70,0x1,0x0,0x0,0x33,0xd2,0xe9,0x62,0xc1,0x27,0x0,0xcc,0xcc,0x48,0x8b,0x1,0x33,0xd2,0x48,0xff,0xa0,0xc8,0x1,0x0,0x0,0xcc,0xcc,0xcc,0xcc}},
{0x5b8c890,{0x48,0x8b,0x89,0x70,0x1,0x0,0x0,0xba,0x1,0x0,0x0,0x0,0xe9,0x9f,0xc0,0x27,0x0,0xcc,0xcc,0xcc,0xcc,0xcc,0xcc,0xcc,0xcc,0xcc,0xcc,0xcc,0xcc,0xcc,0xcc,0xcc}},
{0x5b8c7e0,{0x48,0x8b,0x1,0x33,0xd2,0x48,0xff,0xa0,0xc8,0x1,0x0,0x0,0xcc,0xcc,0xcc,0xcc,0x48,0x89,0x5c,0x24,0x8,0x57,0x48,0x83,0xec,0x20,0x48,0x8b,0x89,0x70,0x1,0x0}},
{0x5b8c8b0,{0x48,0x8b,0x1,0xba,0x1,0x0,0x0,0x0,0x48,0xff,0xa0,0xc8,0x1,0x0,0x0,0xcc,0x48,0x8b,0x89,0x70,0x1,0x0,0x0,0xba,0x2,0x0,0x0,0x0,0xe9,0x6f,0xc0,0x27}}
};
static void* g_collisionOriginals[22]={};
static bool g_collisionReady,g_collisionFault;
static const char* g_collisionFailure="not-started";
static wchar_t g_collisionPath[32768];
static SRWLOCK g_collisionLock=SRWLOCK_INIT;
static unsigned g_collisionLeaves,g_collisionRoots;
struct EliteCollisionContext {ElitePermissionContext* permission=nullptr;unsigned leaves=0,dropped=0;};
static __declspec(thread) EliteCollisionContext* g_collisionContext;
static const char* kEliteCollisionPhases[]={"map-check","object-check","scene-index","scene-map","map-width","map-height","map-y-offset","map-columns","map-rows","map-tile","walk-rectangle","restricted-region","viewport-left","world-check-mode","viewport-mode-a","viewport-mode-b","viewport-mode-c","object-pair","coordinate-int-x","coordinate-int-y","coordinate-box-x","coordinate-box-y"};
static void EliteCollisionRecord(const char* phase,uintptr_t a,uintptr_t b,uintptr_t c,uintptr_t result,uintptr_t caller,bool root=false) {
    auto* ctx=g_collisionContext;if(!ctx || !ctx->permission || g_collisionFault)return;
    if(!root && (ctx->leaves>=512 || g_collisionLeaves>=15360)) {++ctx->dropped;return;}
    if(root && g_collisionRoots>=1024)return;
    if(!TryAcquireSRWLockExclusive(&g_collisionLock)) {++ctx->dropped;return;}
    __try {
        const auto& p=ctx->permission->position;
        int xyz[3]={},body[4]={},targetBody[4]={},rect[4]={};unsigned char pass=0,targetPass=0;
        const bool actorKnown=EliteCopy(reinterpret_cast<void*>(p.actor+348),&body[0],4) &&
            EliteCopy(reinterpret_cast<void*>(p.actor+2288),body+1,12) && EliteCopy(reinterpret_cast<void*>(p.actor+4460),&pass,1);
        for(int axis=0;axis<3;++axis)xyz[axis]=static_cast<int>(g_catchupOriginal(p.component,axis));
        const bool mapPhase=!strcmp(phase,"map-check") && b==p.actor;
        uint64_t map[18]={},fence[4]={},variant[6]={};bool mapKnown=false,fenceKnown=false,variantKnown=false;
        if(mapPhase) {
            static constexpr size_t offsets[]={16472,16476,8676,16296,9456,9248,9256,14208,14216,14248,14256,15152,15160,16072,23080,22696,22704,22720};
            mapKnown=true;
            for(size_t i=0;i<ARRAYSIZE(offsets);++i) {
                const size_t bytes=i==1 || i==17?1:i<4?4:8;
                if(!EliteCopy(reinterpret_cast<void*>(a+offsets[i]),&map[i],bytes))mapKnown=false;
            }
            const uintptr_t f=static_cast<uintptr_t>(map[4]),v=static_cast<uintptr_t>(map[14]);
            if(f)fenceKnown=EliteCopy(reinterpret_cast<void*>(f),&fence[0],1) && EliteCopy(reinterpret_cast<void*>(f+8),&fence[1],4) && EliteCopy(reinterpret_cast<void*>(f+12),&fence[2],1) && EliteCopy(reinterpret_cast<void*>(f+16),&fence[3],4);
            if(v)variantKnown=EliteCopy(reinterpret_cast<void*>(v),&variant[0],8) && EliteCopy(reinterpret_cast<void*>(v+8),&variant[1],8) && EliteCopy(reinterpret_cast<void*>(v+24),&variant[2],4) && EliteCopy(reinterpret_cast<void*>(v+64),&variant[3],4) && EliteCopy(reinterpret_cast<void*>(v+68),&variant[4],4) && EliteCopy(reinterpret_cast<void*>(v+96),&variant[5],1);
        }
        const uintptr_t target=!strcmp(phase,"object-check")?result:!strcmp(phase,"object-pair")?b:0;
        const bool targetKnown=target && EliteCopy(reinterpret_cast<void*>(target+348),&targetBody[0],4) && EliteCopy(reinterpret_cast<void*>(target+2288),targetBody+1,12) && EliteCopy(reinterpret_cast<void*>(target+4460),&targetPass,1);
        const bool rectKnown=!strcmp(phase,"walk-rectangle") && c && EliteCopy(reinterpret_cast<void*>(c),rect,16);
        char line[4000]={};
        const int n=sprintf_s(line,"{\"traceVersion\":1,\"processId\":%lu,\"runTick\":%llu,\"tick\":%llu,\"thread\":%lu,\"ordinal\":%llu,\"slot\":%d,\"actor\":%llu,\"scene\":%llu,\"phase\":\"%s\",\"arguments\":[%llu,%llu,%llu],\"result\":%llu,\"resultLow32\":%u,\"resultByte\":%u,\"callerRva\":%llu,\"xyz\":[%d,%d,%d],\"actorKnown\":%s,\"actorBody\":[%d,%d,%d,%d],\"pass4460\":%u,\"mapKnown\":%s,\"mapFields\":[%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu,%llu],\"fenceKnown\":%s,\"fenceFields\":[%llu,%llu,%llu,%llu],\"variantKnown\":%s,\"variantFields\":[%llu,%llu,%llu,%llu,%llu,%llu],\"target\":%llu,\"targetKnown\":%s,\"targetBody\":[%d,%d,%d,%d],\"targetPass4460\":%u,\"rectangleKnown\":%s,\"rectangle\":[%d,%d,%d,%d],\"leafRecords\":%u,\"dropped\":%u,\"readOnly\":true}\r\n",
            GetCurrentProcessId(),g_eliteTraceRunTick,GetTickCount64(),GetCurrentThreadId(),ctx->permission->ordinal,p.slot,
            static_cast<unsigned long long>(p.actor),static_cast<unsigned long long>(p.scene),phase,
            static_cast<unsigned long long>(a),static_cast<unsigned long long>(b),static_cast<unsigned long long>(c),static_cast<unsigned long long>(result),static_cast<unsigned>(result),static_cast<unsigned>(result&255),static_cast<unsigned long long>(caller),
            xyz[0],xyz[1],xyz[2],actorKnown?"true":"false",body[0],body[1],body[2],body[3],static_cast<unsigned>(pass),mapKnown?"true":"false",
            map[0],map[1],map[2],map[3],map[4],map[5],map[6],map[7],map[8],map[9],map[10],map[11],map[12],map[13],map[14],map[15],map[16],map[17],
            fenceKnown?"true":"false",fence[0],fence[1],fence[2],fence[3],variantKnown?"true":"false",variant[0],variant[1],variant[2],variant[3],variant[4],variant[5],
            static_cast<unsigned long long>(target),targetKnown?"true":"false",targetBody[0],targetBody[1],targetBody[2],targetBody[3],static_cast<unsigned>(targetPass),rectKnown?"true":"false",rect[0],rect[1],rect[2],rect[3],ctx->leaves,ctx->dropped);
        HANDLE file=n>0?CreateFileW(g_collisionPath,FILE_APPEND_DATA,FILE_SHARE_READ|FILE_SHARE_WRITE,nullptr,OPEN_ALWAYS,FILE_ATTRIBUTE_NORMAL,nullptr):INVALID_HANDLE_VALUE;
        DWORD written=0;
        if(file!=INVALID_HANDLE_VALUE) {WriteFile(file,line,static_cast<DWORD>(n),&written,nullptr);CloseHandle(file);}
        if(file==INVALID_HANDLE_VALUE || written!=static_cast<DWORD>(n)) {++ctx->dropped;g_collisionFailure="log-write-failed";}
        if(root)++g_collisionRoots;else {++ctx->leaves;++g_collisionLeaves;}
    }
    __except(GetExceptionCode()==EXCEPTION_ACCESS_VIOLATION || GetExceptionCode()==EXCEPTION_IN_PAGE_ERROR?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {g_collisionFault=true;g_collisionFailure="observer-memory-fault";}
    ReleaseSRWLockExclusive(&g_collisionLock);
}
static uintptr_t EliteCollisionForward0(size_t i,uintptr_t caller) {
    auto native=reinterpret_cast<EliteCollision0>(g_collisionOriginals[i]);
    if(!g_collisionContext)return native();
    const auto result=native();const DWORD error=GetLastError();EliteCollisionRecord(kEliteCollisionPhases[i],0,0,0,result,caller);SetLastError(error);return result;
}
static uintptr_t EliteCollisionForward1(size_t i,uintptr_t a,uintptr_t caller) {
    auto native=reinterpret_cast<EliteCollision1>(g_collisionOriginals[i]);
    if(!g_collisionContext)return native(a);
    const auto result=native(a);const DWORD error=GetLastError();EliteCollisionRecord(kEliteCollisionPhases[i],a,0,0,result,caller);SetLastError(error);return result;
}
static uintptr_t EliteCollisionForward2(size_t i,uintptr_t a,uintptr_t b,uintptr_t caller) {
    auto native=reinterpret_cast<EliteCollision2>(g_collisionOriginals[i]);
    if(!g_collisionContext)return native(a,b);
    const auto result=native(a,b);const DWORD error=GetLastError();EliteCollisionRecord(kEliteCollisionPhases[i],a,b,0,result,caller);SetLastError(error);return result;
}
static uintptr_t EliteCollisionForward3(size_t i,uintptr_t a,uintptr_t b,uintptr_t c,uintptr_t caller) {
    auto native=reinterpret_cast<EliteCollision3>(g_collisionOriginals[i]);
    if(!g_collisionContext)return native(a,b,c);
    const auto result=native(a,b,c);const DWORD error=GetLastError();EliteCollisionRecord(kEliteCollisionPhases[i],a,b,c,result,caller);SetLastError(error);return result;
}
static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve0(uintptr_t a,uintptr_t b,uintptr_t c) {return EliteCollisionForward3(0,a,b,c,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}
static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve1(uintptr_t a,uintptr_t b) {return EliteCollisionForward2(1,a,b,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}
static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve2(uintptr_t a,uintptr_t b) {return EliteCollisionForward2(2,a,b,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}
static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve3(uintptr_t a,uintptr_t b) {return EliteCollisionForward2(3,a,b,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}
static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve4(uintptr_t a) {return EliteCollisionForward1(4,a,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}
static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve5(uintptr_t a) {return EliteCollisionForward1(5,a,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}
static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve6(uintptr_t a) {return EliteCollisionForward1(6,a,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}
static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve7(uintptr_t a) {return EliteCollisionForward1(7,a,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}
static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve8(uintptr_t a) {return EliteCollisionForward1(8,a,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}
static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve9(uintptr_t a,uintptr_t b,uintptr_t c) {return EliteCollisionForward3(9,a,b,c,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}
static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve10(uintptr_t a,uintptr_t b,uintptr_t c) {return EliteCollisionForward3(10,a,b,c,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}
static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve11(uintptr_t a,uintptr_t b) {return EliteCollisionForward2(11,a,b,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}
// 140EEB380 retains R8 as an optional object pointer and forwards it to
// 140EEB800. It must survive unrelated UI calls as well as collision calls.
static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve12(uintptr_t a,uintptr_t b,uintptr_t c) {return EliteCollisionForward3(12,a,b,c,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}
static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve13() {return EliteCollisionForward0(13,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}
static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve14() {return EliteCollisionForward0(14,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}
static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve15() {return EliteCollisionForward0(15,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}
static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve16() {return EliteCollisionForward0(16,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}
static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve17(uintptr_t a,uintptr_t b,uintptr_t c) {return EliteCollisionForward3(17,a,b,c,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}

static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve18(uintptr_t a) {return EliteCollisionForward1(18,a,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}
static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve19(uintptr_t a) {return EliteCollisionForward1(19,a,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}
static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve20(uintptr_t a) {return EliteCollisionForward1(20,a,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}
static __declspec(noinline) uintptr_t __fastcall EliteCollisionObserve21(uintptr_t a) {return EliteCollisionForward1(21,a,reinterpret_cast<uintptr_t>(_ReturnAddress())-reinterpret_cast<uintptr_t>(g_prepareImage));}

static uintptr_t __fastcall ElitePermissionCollisionObserve(uintptr_t a,int x,int y,uintptr_t aux) {
    const DWORD entryError=GetLastError();EliteCollisionContext context={};auto previous=g_collisionContext;
    if(g_collisionReady && !g_collisionFault && g_permissionContext && !previous && g_permissionContext->position.actor==a) {
        context.permission=g_permissionContext;g_collisionContext=&context;
    }
    SetLastError(entryError);uintptr_t result=0;
    __try {result=g_permissionCollision(a,x,y,aux);}
    __finally {if(AbnormalTermination())g_collisionContext=previous;}
    const DWORD error=GetLastError();
    if(context.permission)EliteCollisionRecord("collision-root",a,static_cast<uint32_t>(x),static_cast<uint32_t>(y),result,context.permission->caller,true);
    g_collisionContext=previous;ElitePermissionRecord("collision",a,x,y,aux,result);SetLastError(error);return result;
}
static bool EliteStartCollisionTrace(unsigned char* image,const wchar_t* path) {
    if(!g_permissionReady || image!=g_prepareImage || !path || !path[0] || wcslen(path)>=ARRAYSIZE(g_collisionPath)) {g_collisionFailure="dependency-or-path";return false;}
    for(const auto& g:kEliteCollisionGuards)if(!EliteReadable(image+g.rva,32) || memcmp(image+g.rva,g.bytes,32)) {g_collisionFailure="collision-bytes-mismatch";return false;}
    wcscpy_s(g_collisionPath,path);
    void* hooks[]={reinterpret_cast<void*>(EliteCollisionObserve0),reinterpret_cast<void*>(EliteCollisionObserve1),reinterpret_cast<void*>(EliteCollisionObserve2),reinterpret_cast<void*>(EliteCollisionObserve3),reinterpret_cast<void*>(EliteCollisionObserve4),reinterpret_cast<void*>(EliteCollisionObserve5),reinterpret_cast<void*>(EliteCollisionObserve6),reinterpret_cast<void*>(EliteCollisionObserve7),reinterpret_cast<void*>(EliteCollisionObserve8),reinterpret_cast<void*>(EliteCollisionObserve9),reinterpret_cast<void*>(EliteCollisionObserve10),reinterpret_cast<void*>(EliteCollisionObserve11),reinterpret_cast<void*>(EliteCollisionObserve12),reinterpret_cast<void*>(EliteCollisionObserve13),reinterpret_cast<void*>(EliteCollisionObserve14),reinterpret_cast<void*>(EliteCollisionObserve15),reinterpret_cast<void*>(EliteCollisionObserve16),reinterpret_cast<void*>(EliteCollisionObserve17),reinterpret_cast<void*>(EliteCollisionObserve18),reinterpret_cast<void*>(EliteCollisionObserve19),reinterpret_cast<void*>(EliteCollisionObserve20),reinterpret_cast<void*>(EliteCollisionObserve21)};size_t created=0,enabled=0;
    for(;created<ARRAYSIZE(hooks);++created)if(MH_CreateHook(image+kEliteCollisionGuards[created].rva,hooks[created],&g_collisionOriginals[created])!=MH_OK)break;
    if(created==ARRAYSIZE(hooks))for(;enabled<ARRAYSIZE(hooks);++enabled)if(MH_EnableHook(image+kEliteCollisionGuards[enabled].rva)!=MH_OK)break;
    if(created!=ARRAYSIZE(hooks) || enabled!=ARRAYSIZE(hooks)) {
        for(size_t i=0;i<created;++i) {MH_DisableHook(image+kEliteCollisionGuards[i].rva);MH_RemoveHook(image+kEliteCollisionGuards[i].rva);}
        g_collisionFailure=created!=ARRAYSIZE(hooks)?"hook-create":"hook-enable";return false;
    }
    g_collisionReady=true;g_collisionFailure="ready";return true;
}
