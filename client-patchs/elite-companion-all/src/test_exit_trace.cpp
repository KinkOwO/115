#include "exit_trace.h"
#include <assert.h>
static volatile LONG id=100004995;
int wmain(int argc,wchar_t** argv) {
    if(argc==3&&wcscmp(argv[1],L"child")==0) {
        if(!CrashTrace::Start(argv[2],&id)||!ExitTrace::Start())return 8;
        ExitProcess(3);
    }
    auto target=GetProcAddress(GetModuleHandleW(L"ntdll.dll"),"RtlExitUserProcess");
    assert(target&&memcmp((void*)target,kExitHead,sizeof(kExitHead))==0);
    assert(ExitTrace::Start());
    assert(!ExitTrace::Start());
    assert(ExitTrace::Stop());
    assert(memcmp((void*)target,kExitHead,sizeof(kExitHead))==0);
    // Byte mismatch must reject before creating or publishing an observer.
    unsigned char wrong[sizeof(kExitHead)]={};InlineHook rejected={};
    assert(!PrepareHook(rejected,(void*)target,wrong,sizeof(wrong),(void*)&ExitTrace::Observe));
    assert(!rejected.installed&&!rejected.page);
    wchar_t dir[MAX_PATH]={},path[MAX_PATH]={},exe[MAX_PATH]={},command[MAX_PATH*3]={};
    assert(GetTempPathW(MAX_PATH,dir)&&GetTempFileNameW(dir,L"ext",0,path));
    assert(GetModuleFileNameW(nullptr,exe,MAX_PATH));
    _snwprintf_s(command,sizeof(command)/sizeof(command[0]),_TRUNCATE,L"\"%s\" child \"%s\"",exe,path);
    STARTUPINFOW startup={sizeof(startup)};PROCESS_INFORMATION process={};
    assert(CreateProcessW(exe,command,nullptr,nullptr,FALSE,CREATE_NO_WINDOW,nullptr,nullptr,&startup,&process));
    assert(WaitForSingleObject(process.hProcess,10000)==WAIT_OBJECT_0);
    DWORD status=0;assert(GetExitCodeProcess(process.hProcess,&status)&&status==3);
    CloseHandle(process.hThread);CloseHandle(process.hProcess);
    FILE* file=nullptr;assert(_wfopen_s(&file,path,L"rb")==0);char data[16384]={};
    assert(fread(data,1,sizeof(data)-1,file)>0);fclose(file);
    assert(strstr(data,"native_process_exit dungeon=100004995 status=0x00000003"));
    assert(strstr(data,"frame=0")&&strstr(data,"end_native_process_exit"));
    assert(DeleteFileW(path));
    puts("PASS native exit observer: exact export guard, restoration, child exit status 3 preserved, stack recorded");
}
