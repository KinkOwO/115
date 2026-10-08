#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdio.h>
#include "entry-trace.h"
static int calls;
static void __fastcall Original(uintptr_t a,unsigned int b,unsigned int c,char d,char e,uintptr_t f,unsigned char g,char h) {
    if(a!=11 || b!=22 || c!=33 || d!=4 || e!=5 || f!=66 || g!=7 || h!=8) RaiseException(0xE1234001,0,0,nullptr);
    ++calls;SetLastError(ERROR_ACCESS_DENIED);
}
static void __fastcall Throwing(uintptr_t,unsigned int,unsigned int,char,char,uintptr_t,unsigned char,char) {RaiseException(0xE1234567,0,0,nullptr);}
static char __fastcall Gate() {++calls;SetLastError(ERROR_ACCESS_DENIED);return 37;}
static char __fastcall Register(uintptr_t a,uintptr_t b,unsigned int c,unsigned int d,uintptr_t e,uintptr_t f,char g,char h) {
    if(a!=11 || b!=22 || c!=33 || d!=44 || e!=55 || f!=66 || g!=7 || h!=8) RaiseException(0xE1234001,0,0,nullptr);
    ++calls;SetLastError(ERROR_ACCESS_DENIED);return 13;
}
int main() {
    auto image=reinterpret_cast<unsigned char*>(GetModuleHandleW(nullptr));
    if(EliteStartEntryTrace(image,L"unused") || EliteStartNativeTrace(image,L"unused") || EliteStartLifecycleTrace(image,L"unused",0) || EliteStartPreparationAdapter(image))return 1;
    g_entryOriginal=Original;g_entryWorld=123;g_entryLoaderCaller=456;g_entryFreshDungeon=true;
    EliteEntryObserve(11,22,33,4,5,66,7,8);
    if(calls!=1 || g_entryWorld!=123 || g_entryLoaderCaller!=456 || !g_entryFreshDungeon || GetLastError()!=ERROR_ACCESS_DENIED)return 2;
    g_entryGateOriginal=Gate;
    if(EliteEntryGateObserve()!=37 || calls!=2 || GetLastError()!=ERROR_ACCESS_DENIED)return 3;
    g_entryRegisterOriginal=Register;
    if(EliteEntryRegisterObserve(11,22,33,44,55,66,7,8)!=13 || calls!=3 || GetLastError()!=ERROR_ACCESS_DENIED)return 4;
    g_entryOriginal=Throwing;bool escaped=false;
    __try {EliteEntryObserve(1,2,3,4,5,6,7,8);} __except(GetExceptionCode()==0xE1234567?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH){escaped=true;}
    if(!escaped || g_entryWorld!=123 || g_entryLoaderCaller!=456 || !g_entryFreshDungeon || g_entryCount!=0)return 5;
    wchar_t path[32768]={};
    if(!GetModuleFileNameW(nullptr,path,32768))return 6;
    wchar_t* slash=wcsrchr(path,L'\\');if(!slash)return 7;*(slash+1)=0;
    if(wcscat_s(path,L"entry-trace-test.jsonl") || wcscpy_s(g_entryPath,path))return 8;
    g_entryReady=true;g_lifecycleArmed=true;g_lifecycleThread=GetCurrentThreadId();g_lifecycleChannel=0;g_lifecycleOwner=0;
    for(int i=0;i<600;++i)EliteEntryRecord("mechanism-budget",0);
    if(g_entryCount!=600 || g_entryDropped!=0)return 9;
    puts("entry observer: native 8-argument ABI, exact results, last-error, exception and TLS restoration; wrong image refusal passed");
    return 0;
}
