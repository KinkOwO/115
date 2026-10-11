#include "crash_trace.h"
#include <assert.h>
#include <stdexcept>
#include <new>
static volatile LONG id=0;
static void RaiseAndCatchAV() {
    bool caught=false;
    __try {RaiseException(EXCEPTION_ACCESS_VIOLATION,0,0,nullptr);}
    __except(EXCEPTION_EXECUTE_HANDLER){caught=true;}
    assert(caught);
}
static void LogicError() {
    bool caught=false;
    try {throw std::logic_error("native propagation marker");}
    catch(const std::logic_error& e) {caught=strcmp(e.what(),"native propagation marker")==0;}
    assert(caught);
}
static void BadAlloc() {
    bool caught=false;
    try {throw std::bad_alloc();}
    catch(const std::bad_alloc&) {caught=true;}
    assert(caught);
}
int main() {
    wchar_t dir[MAX_PATH]={},path[MAX_PATH]={};
    assert(GetTempPathW(MAX_PATH,dir));assert(GetTempFileNameW(dir,L"ect",0,path));
    assert(CrashTrace::Start(path,&id));
    RaiseAndCatchAV();assert(CrashTrace::count==0);
    LogicError();assert(CrashTrace::count==1); // Town/return C++ must be observed.
    id=17;RaiseAndCatchAV();assert(CrashTrace::count==2);
    BadAlloc();assert(CrashTrace::count==3);
    CrashTrace::Stop();
    FILE* f=nullptr;assert(_wfopen_s(&f,path,L"rb")==0);char data[65536]={};
    size_t n=fread(data,1,sizeof(data)-1,f);fclose(f);assert(n>0);
    assert(strstr(data,"dungeon=0")&&strstr(data,"dungeon=17"));
    assert(strstr(data,"code=0xe06d7363")&&strstr(data,"parameters=4"));
    assert(strstr(data,"info2=")&&strstr(data,"info3="));
    assert(strstr(data,"memory_ok=1")&&strstr(data,"private_bytes="));
    assert(strstr(data,"frame=0")&&strstr(data,"CONTINUE_SEARCH"));
    assert(DeleteFileW(path));
    puts("PASS passive AV/C++ observer: town and dungeon scope, native typed catches, throw metadata, memory, unwind");
}
