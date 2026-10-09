#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdio.h>
#include "entry-trace.h"
static bool g_registrationReady;
#include "combat-trace.h"
static int calls;
static DWORD expectedError=123;
static void __fastcall Assign(uintptr_t victim,uintptr_t input) {
    if(victim!=11 || input!=22 || GetLastError()!=expectedError)RaiseException(0xE1234001,0,0,nullptr);
    ++calls;SetLastError(ERROR_ACCESS_DENIED);
}
static void __fastcall Set(uintptr_t victim,int value) {
    if(victim!=11 || value!=65535 || GetLastError()!=expectedError)RaiseException(0xE1234002,0,0,nullptr);
    ++calls;SetLastError(ERROR_ACCESS_DENIED);
}
static uintptr_t __fastcall Send(uintptr_t victim,char a2,unsigned char a3) {
    if(victim!=11 || a2!=-3 || a3!=241 || GetLastError()!=expectedError)RaiseException(0xE1234003,0,0,nullptr);
    ++calls;SetLastError(ERROR_ACCESS_DENIED);return UINT64_C(0xfedcba9876543210);
}
static void __fastcall Throw(uintptr_t,uintptr_t) {RaiseException(0xE1234567,0,0,nullptr);}
static void __fastcall Nested(uintptr_t victim,uintptr_t input) {
    if(g_combatContext.victim!=victim || g_combatContext.input!=input)RaiseException(0xE1234004,0,0,nullptr);
    auto context=g_combatContext;auto original=g_combatAssignOriginal;g_combatAssignOriginal=Assign;
    SetLastError(expectedError);EliteCombatAssignObserve(11,22);g_combatAssignOriginal=original;
    if(memcmp(&context,&g_combatContext,sizeof(context)))RaiseException(0xE1234005,0,0,nullptr);
}
static unsigned char victimData[36000],sourceData[101000],playerData[101000],managerData[200],contextData[5500],controllerData[2200],controlData[32],ownerControl[32],vtableData[2000];
static bool mutate=false,block=false;static int identityReads;
static bool Read(const void* address,void* dest,size_t size) {
    if(block && address==sourceData+356)return false;
    if(mutate && address==sourceData+356 && ++identityReads==2) {mutate=false;sourceData[356]^=1;}
    return EliteCopy(address,dest,size);
}
static void Identity(unsigned char* actor,uint32_t id) {
    uint32_t words[2]={(id+4)^UINT32_C(0x1f2a025c),0};words[1]=words[0]+id+196;memcpy(actor+356,words,8);
}
static void Ptr(unsigned char* data,size_t offset,uintptr_t value) {memcpy(data+offset,&value,8);}
static void Int(unsigned char* data,size_t offset,int value) {memcpy(data+offset,&value,4);}
static bool SnapshotTests() {
    Identity(victimData,4096);Identity(sourceData,0);Identity(playerData,2);
    Int(victimData,352,4);Int(victimData,26784,65535);Int(victimData,35076,0);
    Int(sourceData,352,5);Int(sourceData,74020,65535);Int(playerData,352,3);Int(playerData,74020,2);
    Int(controlData,8,1);Int(ownerControl,8,1);
    Ptr(victimData,29496,reinterpret_cast<uintptr_t>(controlData));Ptr(victimData,29504,reinterpret_cast<uintptr_t>(sourceData+48));
    Ptr(sourceData,25272,reinterpret_cast<uintptr_t>(ownerControl));Ptr(sourceData,25280,reinterpret_cast<uintptr_t>(playerData+48));
    Ptr(sourceData,100888,reinterpret_cast<uintptr_t>(contextData));Ptr(contextData,5424,reinterpret_cast<uintptr_t>(controllerData));
    unsigned short wire=65535;memcpy(controllerData+112,&wire,2);
    Ptr(controllerData,2080,reinterpret_cast<uintptr_t>(controlData));Ptr(controllerData,2088,reinterpret_cast<uintptr_t>(sourceData+48));
    Ptr(managerData,72,reinterpret_cast<uintptr_t>(controlData));Ptr(managerData,80,reinterpret_cast<uintptr_t>(sourceData+48));
    Ptr(sourceData,0,reinterpret_cast<uintptr_t>(vtableData));Ptr(vtableData,1984,0x14014cc20);
    uintptr_t player[]={reinterpret_cast<uintptr_t>(ownerControl),reinterpret_cast<uintptr_t>(playerData+48)};
    auto s=EliteReadCombat(reinterpret_cast<uintptr_t>(victimData),reinterpret_cast<uintptr_t>(managerData),player,Read);
    if(!s.readable || !s.stable || s.victimId!=4096 || s.sourceId!=0 || s.slot!=0 || s.ownerId!=2 || s.ownerController!=2 || s.ownerMatches!=1 || s.killer!=65535 || s.sourceController!=65535)return false;
    player[0]=1;s=EliteReadCombat(reinterpret_cast<uintptr_t>(victimData),reinterpret_cast<uintptr_t>(managerData),player,Read);
    if(s.ownerMatches!=0)return false;
    Int(controlData,8,0);s=EliteReadCombat(reinterpret_cast<uintptr_t>(victimData),reinterpret_cast<uintptr_t>(managerData),player,Read);
    if(s.slot!=-1 || s.sourceId!=-1)return false;
    Int(controlData,8,1);block=true;s=EliteReadCombat(reinterpret_cast<uintptr_t>(victimData),reinterpret_cast<uintptr_t>(managerData),player,Read);block=false;
    if(s.stable || s.slot!=-1)return false;
    sourceData[360]^=1;s=EliteReadCombat(reinterpret_cast<uintptr_t>(victimData),reinterpret_cast<uintptr_t>(managerData),player,Read);sourceData[360]^=1;
    if(s.sourceId!=-1 || s.slot!=-1)return false;
    auto invalid=EliteReadCombat(1,0,nullptr,Read);if(invalid.readable || invalid.stable)return false;
    // Mutation after the first identity read must be detected at the final reread.
    mutate=true;identityReads=0;s=EliteReadCombat(reinterpret_cast<uintptr_t>(victimData),reinterpret_cast<uintptr_t>(managerData),player,Read);
    sourceData[356]^=1;if(s.stable || s.slot!=-1)return false;
    return Int(victimData,26784,65535),true;
}
int main() {
    auto image=reinterpret_cast<unsigned char*>(GetModuleHandleW(nullptr));g_prepareImage=image;
    if(EliteStartCombatTrace(image,L"unused") || g_combatReady || EliteStartEntryTrace(image,L"unused") || EliteStartNativeTrace(image,L"unused") || EliteStartLifecycleTrace(image,L"unused",0) || EliteStartPreparationAdapter(image))return 1;
    if(!SnapshotTests())return 2;
    g_combatAssignOriginal=Assign;g_combatSetOriginal=Set;g_combatDeathOriginal=Send;
    g_combatContext={44,55,66,true};auto saved=g_combatContext;
    SetLastError(expectedError);EliteCombatAssignObserve(11,22);
    if(calls!=1 || GetLastError()!=ERROR_ACCESS_DENIED || memcmp(&saved,&g_combatContext,sizeof(saved)))return 3;
    SetLastError(expectedError);EliteCombatSetObserve(11,65535);if(calls!=2 || GetLastError()!=ERROR_ACCESS_DENIED)return 4;
    SetLastError(expectedError);auto result=EliteCombatDeathObserve(11,-3,241);
    if(calls!=3 || result!=UINT64_C(0xfedcba9876543210) || GetLastError()!=ERROR_ACCESS_DENIED)return 5;
    g_combatAssignOriginal=Nested;EliteCombatAssignObserve(111,222);
    if(calls!=4 || memcmp(&saved,&g_combatContext,sizeof(saved)))return 6;
    bool escaped=false;g_combatAssignOriginal=Throw;
    __try {EliteCombatAssignObserve(11,22);} __except(GetExceptionCode()==0xE1234567?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {escaped=true;}
    if(!escaped || memcmp(&saved,&g_combatContext,sizeof(saved)) || g_combatCount)return 7;
    wchar_t path[32768]={};if(!GetModuleFileNameW(nullptr,path,32768))return 8;
    wchar_t* slash=wcsrchr(path,L'\\');if(!slash)return 9;*(slash+1)=0;
    if(wcscat_s(path,L"combat-trace-test.jsonl") || wcscpy_s(g_combatPath,path))return 10;
    g_combatReady=true;g_entryReady=true;g_lifecycleArmed=true;
    g_lifecycleThread=GetCurrentThreadId();g_lifecycleChannel=0;g_lifecycleOwner=0;
    g_combatAssignOriginal=Assign;SetLastError(expectedError);EliteCombatAssignObserve(11,22);
    if(GetLastError()!=ERROR_ACCESS_DENIED || g_combatCount!=1 || memcmp(&saved,&g_combatContext,sizeof(saved)))return 12;
    AcquireSRWLockExclusive(&g_combatLock);
    EliteCombatSnapshot empty;
    EliteCombatRecord("busy-test",99,0,0,0,65535,true,0,empty,empty);
    ReleaseSRWLockExclusive(&g_combatLock);
    if(g_combatDropped!=1)return 13;
    g_combatCount=0;g_combatDropped=0;
    for(int i=0;i<4200;++i)EliteCombatRecord("mechanism-budget",i,0,0,0,65535,true,0,empty,empty);
    if(g_combatCount!=4200 || g_combatDropped)return 11;
    puts("combat observation: exact ABI/arguments/64-bit result/last-error/SEH/nested TLS; readable/stable identity, owner mismatch, expired refs, invalid checksum, missing memory; bounded log passed; no client launched");
    return 0;
}
