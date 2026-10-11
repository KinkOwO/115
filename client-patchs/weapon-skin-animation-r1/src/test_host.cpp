#include <windows.h>
#include <assert.h>
#include <stdio.h>
int wmain(int argc,wchar_t** argv){
    assert(argc==3);SetErrorMode(SEM_FAILCRITICALERRORS|SEM_NOGPFAULTERRORBOX);
    bool enabled=!wcscmp(argv[2],L"enabled");
    SetEnvironmentVariableW(L"DFO_WEAPON_SKIN_ANIMATION",enabled?L"r1":nullptr);
    auto dll=LoadLibraryW(argv[1]);assert(dll);
    auto start=(DWORD(WINAPI*)())GetProcAddress(dll,"ModStart");assert(start&&GetProcAddress(dll,"ModName"));
    assert(start()==(enabled?2:0));assert(start()==0);
    puts(enabled?"PASS incompatible image disabled without hooks":"PASS inactive outside candidate; no hooks");
}
