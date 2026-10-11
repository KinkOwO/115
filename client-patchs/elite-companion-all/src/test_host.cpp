#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <assert.h>
#include <stdio.h>
#include <string.h>
static bool Contains(const wchar_t* path,const char* needle){
    HANDLE f=CreateFileW(path,GENERIC_READ,FILE_SHARE_READ|FILE_SHARE_WRITE,nullptr,OPEN_EXISTING,0,nullptr);
    if(f==INVALID_HANDLE_VALUE)return false;
    char data[32768]={};DWORD got=0;bool read=ReadFile(f,data,sizeof(data)-1,&got,nullptr)!=0;
    CloseHandle(f);return read&&strstr(data,needle)!=nullptr;
}
int wmain(int argc,wchar_t** argv){
    assert(argc==2);wchar_t path[MAX_PATH],log[MAX_PATH];
    swprintf_s(path,L"%s\\dinput8.dll",argv[1]);
    HMODULE proxy=LoadLibraryW(path);assert(proxy);
    const char* exports[]={"DirectInput8Create","DllCanUnloadNow","DllGetClassObject","DllRegisterServer","DllUnregisterServer","GetdfDIJoystick"};
    for(auto name:exports)assert(GetProcAddress(proxy,name));
    ((HRESULT(WINAPI*)())GetProcAddress(proxy,"DllCanUnloadNow"))();
    swprintf_s(log,L"%s\\.115us-mods\\elite-companion-pilot.log",argv[1]);
    wchar_t bridgeLog[MAX_PATH];swprintf_s(bridgeLog,L"%s\\115-shop-tabs.log",argv[1]);
    bool disabled=false;for(int i=0;i<100;i++){
        if(Contains(log,"DISABLED client layout/helper fingerprint mismatch")&&Contains(bridgeLog,"CLIENT_HOST started result=0")){disabled=true;break;}Sleep(50);}
    assert(disabled);
    swprintf_s(log,L"%s\\115-shop-tabs.log",argv[1]);
    assert(Contains(log,"CLIENT_HOST started result=0"));
    puts("PASS accepted proxy exports -> client-host -> plugin ABI; incompatible test host disabled all game hooks");
    return 0;
}
