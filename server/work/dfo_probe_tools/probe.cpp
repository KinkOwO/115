#define UNICODE
#define _UNICODE
#define WIN32_LEAN_AND_MEAN
#include <winsock2.h>
#include <ws2tcpip.h>
#include <windows.h>
#include <winternl.h>
#include <fwpmu.h>
#include <tlhelp32.h>
#include <filesystem>
#include <fstream>
#include <iostream>
#include <vector>
#include <string>
#include <map>
#include <set>
#include <algorithm>
#include <codecvt>
#include <locale>
#pragma comment(lib,"fwpuclnt.lib")
#pragma comment(lib,"rpcrt4.lib")
#pragma comment(lib,"ws2_32.lib")
namespace fs = std::filesystem;
std::wofstream probe_log;
ULONGLONG started;
void log(const std::wstring& s) { probe_log << GetTickCount64()-started << L" " << s << std::endl; }
std::wstring hx(ULONGLONG v) { wchar_t b[40]; swprintf_s(b,L"0x%llX",v); return b; }
std::wstring remote_hex(HANDLE h,ULONGLONG address,SIZE_T count){
    count=std::min<SIZE_T>(count,1024);std::vector<BYTE> bytes(count);SIZE_T n=0;
    if(!ReadProcessMemory(h,(void*)address,bytes.data(),count,&n))return L"unreadable";
    std::wstring s;for(SIZE_T i=0;i<n;i++){wchar_t tmp[3];swprintf_s(tmp,L"%02X",bytes[i]);s+=tmp;}return s;
}
std::wstring path_of(HANDLE h) { wchar_t b[32768]; DWORD n=GetFinalPathNameByHandleW(h,b,32768,FILE_NAME_NORMALIZED); return n&&n<32768?std::wstring(b,n):L"?"; }
std::wstring process_path(HANDLE h) { wchar_t b[32768]; DWORD n=32768; return QueryFullProcessImageNameW(h,0,b,&n)?std::wstring(b,n):L"?"; }
std::wstring command_of(HANDLE h){
    using Q=NTSTATUS(NTAPI*)(HANDLE,PROCESSINFOCLASS,PVOID,ULONG,PULONG);auto q=(Q)GetProcAddress(GetModuleHandleW(L"ntdll.dll"),"NtQueryInformationProcess");PROCESS_BASIC_INFORMATION p{};PEB peb{};RTL_USER_PROCESS_PARAMETERS pp{};SIZE_T n;
    if(!q||q(h,ProcessBasicInformation,&p,sizeof(p),nullptr)<0||!ReadProcessMemory(h,p.PebBaseAddress,&peb,sizeof(peb),&n)||!ReadProcessMemory(h,peb.ProcessParameters,&pp,sizeof(pp),&n)||pp.CommandLine.Length>8192)return L"?";
    std::wstring s(pp.CommandLine.Length/2,L'\0');return ReadProcessMemory(h,pp.CommandLine.Buffer,s.data(),pp.CommandLine.Length,&n)?s:L"?";
}
bool under(const fs::path& p,const fs::path& root) {
    auto a=fs::absolute(p).lexically_normal().wstring(); auto b=fs::absolute(root).lexically_normal().wstring()+L"\\";
    std::transform(a.begin(),a.end(),a.begin(),towlower); std::transform(b.begin(),b.end(),b.begin(),towlower); return a.rfind(b,0)==0;
}
struct Guard {
    HANDLE h=nullptr; GUID sub{}; unsigned filters=0;
    ~Guard(){if(h){FwpmEngineClose0(h);log(L"WFP_DYNAMIC_SESSION_CLOSED");}}
    bool init() {
        FWPM_SESSION0 s{};s.flags=FWPM_SESSION_FLAG_DYNAMIC;s.displayData.name=(wchar_t*)L"DFO isolated startup probe";
        DWORD e=FwpmEngineOpen0(nullptr,RPC_C_AUTHN_WINNT,nullptr,&s,&h);if(e){log(L"WFP_OPEN_ERROR "+hx(e));return false;}
        UuidCreate(&sub);FWPM_SUBLAYER0 sl{};sl.subLayerKey=sub;sl.displayData.name=(wchar_t*)L"DFO probe paths only";sl.weight=0xFFFF;
        e=FwpmSubLayerAdd0(h,&sl,nullptr);if(e){log(L"WFP_SUBLAYER_ERROR "+hx(e));return false;}return true;
    }
    bool add(const fs::path& p){
        FWP_BYTE_BLOB* app=nullptr;DWORD e=FwpmGetAppIdFromFileName0(p.c_str(),&app);if(e){log(L"WFP_APP_ERROR "+p.wstring()+L" "+hx(e));return false;}
        FWPM_FILTER_CONDITION0 c[2]{};c[0].fieldKey=FWPM_CONDITION_ALE_APP_ID;c[0].matchType=FWP_MATCH_EQUAL;c[0].conditionValue.type=FWP_BYTE_BLOB_TYPE;c[0].conditionValue.byteBlob=app;
        c[1].fieldKey=FWPM_CONDITION_FLAGS;c[1].matchType=FWP_MATCH_FLAGS_NONE_SET;c[1].conditionValue.type=FWP_UINT32;c[1].conditionValue.uint32=FWP_CONDITION_FLAG_IS_LOOPBACK;
        for(const GUID* layer:{&FWPM_LAYER_ALE_AUTH_CONNECT_V4,&FWPM_LAYER_ALE_AUTH_CONNECT_V6}){
            FWPM_FILTER0 f{};f.displayData.name=(wchar_t*)L"DFO probe block non-loopback";f.layerKey=*layer;f.subLayerKey=sub;f.action.type=FWP_ACTION_BLOCK;f.weight.type=FWP_UINT8;f.weight.uint8=15;f.numFilterConditions=2;f.filterCondition=c;
            UINT64 id=0;e=FwpmFilterAdd0(h,&f,nullptr,&id);if(e){log(L"WFP_FILTER_ERROR "+hx(e));FwpmFreeMemory0((void**)&app);return false;}++filters;
        }
        FwpmFreeMemory0((void**)&app);return true;
    }
};
int net_check(){
    WSADATA wd{};if(WSAStartup(MAKEWORD(2,2),&wd))return 21;
    SOCKET l=socket(AF_INET,SOCK_STREAM,IPPROTO_TCP);sockaddr_in a{};a.sin_family=AF_INET;a.sin_addr.s_addr=htonl(INADDR_LOOPBACK);
    if(bind(l,(sockaddr*)&a,sizeof(a))||listen(l,1))return 22;int n=sizeof(a);getsockname(l,(sockaddr*)&a,&n);SOCKET c=socket(AF_INET,SOCK_STREAM,IPPROTO_TCP);
    if(connect(c,(sockaddr*)&a,sizeof(a)))return 23;closesocket(c);closesocket(l);
    c=socket(AF_INET,SOCK_STREAM,IPPROTO_TCP);u_long mode=1;ioctlsocket(c,FIONBIO,&mode);a.sin_port=htons(9);InetPtonW(AF_INET,L"192.0.2.1",&a.sin_addr);
    int r=connect(c,(sockaddr*)&a,sizeof(a));int err=r==SOCKET_ERROR?WSAGetLastError():0;
    if(err==WSAEWOULDBLOCK){fd_set w,e;FD_ZERO(&w);FD_ZERO(&e);FD_SET(c,&w);FD_SET(c,&e);timeval tv{2,0};if(select(0,nullptr,&w,&e,&tv)>0){int z=sizeof(err);getsockopt(c,SOL_SOCKET,SO_ERROR,(char*)&err,&z);}else err=WSAETIMEDOUT;}
    closesocket(c);WSACleanup();return err==WSAEACCES?0:(err?err:24);
}
DWORD get_entry(const fs::path& p){std::ifstream f(p,std::ios::binary);IMAGE_DOS_HEADER d{};f.read((char*)&d,sizeof(d));f.seekg(d.e_lfanew);IMAGE_NT_HEADERS64 n{};f.read((char*)&n,sizeof(n));return n.OptionalHeader.AddressOfEntryPoint;}
struct ApiDef{std::wstring module,label;ULONGLONG offset;bool persistent;};
struct ApiBreakpoint{BYTE original;std::wstring label;bool persistent;};
std::vector<ApiDef> api_defs;
std::map<std::pair<DWORD,ULONGLONG>,ApiBreakpoint> api_breaks;
std::map<DWORD,std::pair<DWORD,ULONGLONG>> pending_steps;
void init_apis(){
    for(auto spec:std::vector<std::pair<const wchar_t*,const char*>>{{L"kernel32.dll","CreateFileW"},{L"kernel32.dll","CreateFileA"},{L"kernel32.dll","ExitProcess"},{L"ntdll.dll","RtlExitUserProcess"},{L"ntdll.dll","NtTerminateProcess"},{L"user32.dll","MessageBoxW"},{L"user32.dll","MessageBoxA"}}){
        HMODULE h=LoadLibraryExW(spec.first,nullptr,LOAD_LIBRARY_SEARCH_SYSTEM32);FARPROC a=h?GetProcAddress(h,spec.second):nullptr;HMODULE owner=nullptr;
        if(!a||!GetModuleHandleExW(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS|GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT,(LPCWSTR)a,&owner))continue;
        wchar_t p[32768];GetModuleFileNameW(owner,p,32768);std::wstring name=fs::path(p).filename().wstring(),label;for(const char* c=spec.second;*c;c++)label+=*c;
        std::transform(name.begin(),name.end(),name.begin(),towlower);api_defs.push_back({name,label,(ULONGLONG)a-(ULONGLONG)owner,label.rfind(L"CreateFile",0)==0});
    }
}
bool patch_byte(HANDLE h,ULONGLONG a,BYTE value){DWORD old,tmp;SIZE_T n;if(!VirtualProtectEx(h,(void*)a,1,PAGE_EXECUTE_READWRITE,&old))return false;BOOL ok=WriteProcessMemory(h,(void*)a,&value,1,&n);VirtualProtectEx(h,(void*)a,1,old,&tmp);FlushInstructionCache(h,(void*)a,1);return ok;}
void install_apis(HANDLE h,DWORD pid,ULONGLONG base,const std::wstring& path,bool root){
    auto name=fs::path(path).filename().wstring();std::transform(name.begin(),name.end(),name.begin(),towlower);
    for(const auto& d:api_defs){if(d.module!=name||(!root&&d.label!=L"NtTerminateProcess"))continue;ULONGLONG a=base+d.offset;if(api_breaks.count({pid,a}))continue;BYTE v;SIZE_T n;if(ReadProcessMemory(h,(void*)a,&v,1,&n)&&patch_byte(h,a,0xCC)){api_breaks[{pid,a}]={v,d.label,d.persistent};log(L"TRACE_API pid="+std::to_wstring(pid)+L" "+d.label+L" "+hx(a));}}
}
std::wstring remote_text(HANDLE h,ULONGLONG address,bool wide){
    std::wstring out;for(unsigned i=0;i<1024;i++){SIZE_T n;if(wide){wchar_t c;if(!ReadProcessMemory(h,(void*)(address+i*2),&c,2,&n)||!c)break;out+=c;}else{char c;if(!ReadProcessMemory(h,(void*)(address+i),&c,1,&n)||!c)break;out+=(unsigned char)c;}}return out;
}
void dump_context(HANDLE proc,DWORD tid,void* address){
    HANDLE th=OpenThread(THREAD_GET_CONTEXT|THREAD_QUERY_INFORMATION,FALSE,tid);CONTEXT c{};c.ContextFlags=CONTEXT_FULL;
    if(th&&GetThreadContext(th,&c)){
        log(L"CONTEXT RIP="+hx(c.Rip)+L" RSP="+hx(c.Rsp)+L" RAX="+hx(c.Rax)+L" RBX="+hx(c.Rbx)+L" RCX="+hx(c.Rcx)+L" RDX="+hx(c.Rdx)+L" R8="+hx(c.R8)+L" R9="+hx(c.R9));
        ULONGLONG stack[24]{};SIZE_T n=0;if(ReadProcessMemory(proc,(void*)c.Rsp,stack,sizeof(stack),&n)){std::wstring s=L"STACK";for(unsigned i=0;i<n/8;i++)s+=L" "+hx(stack[i]);log(s);}
    }
    if(th)CloseHandle(th);BYTE b[64]{};SIZE_T n=0;if(ReadProcessMemory(proc,address,b,sizeof(b),&n)){std::wstring s=L"CODE";for(unsigned i=0;i<n;i++){wchar_t x[4];swprintf_s(x,L"%02X",b[i]);s+=L" ";s+=x;}log(s);}
}
int wmain(int argc,wchar_t** argv){
    if(argc>1&&std::wstring(argv[1])==L"--net-check")return net_check();
    if(argc<4)return 2;fs::path root=fs::absolute(argv[1]).lexically_normal();fs::path target=root/L"DFO.exe";
    if(!fs::exists(target))return 3;
    probe_log.imbue(std::locale(std::locale::classic(),new std::codecvt_utf8_utf16<wchar_t>));probe_log.open(fs::path(argv[2]));started=GetTickCount64();unsigned seconds=std::clamp(_wtoi(argv[3]),1,55);bool stop_entry=argc>4&&std::wstring(argv[4])==L"entry";init_apis();
    if(argc>5){std::wifstream spec{fs::path(argv[5])};ULONGLONG rva;std::wstring label;while(spec>>std::hex>>rva>>label)api_defs.push_back({L"dfo.exe",label,rva,label==L"SEND_PLAIN"||label==L"SEND_RAW"});}
    wchar_t selfbuf[32768];GetModuleFileNameW(nullptr,selfbuf,32768);
    Guard guard;
    bool wfp_ok=guard.init()&&guard.add(selfbuf);
    if(wfp_ok){
        for(auto& e:fs::recursive_directory_iterator(root)){
            if(!e.is_regular_file())continue;auto ext=e.path().extension().wstring();std::transform(ext.begin(),ext.end(),ext.begin(),towlower);
            if(ext==L".exe"||ext==L".aes")if(!guard.add(e.path())){wfp_ok=false;break;}
        }
    }
    if(wfp_ok)log(L"WFP_READY filters="+std::to_wstring(guard.filters)+L" NON_LOOPBACK_BLOCKED IPV4_IPV6");
    else log(L"WFP_NOT_AVAILABLE_RUNNING_WITHOUT_ISOLATION");
    HANDLE job=CreateJobObjectW(nullptr,nullptr);JOBOBJECT_EXTENDED_LIMIT_INFORMATION limits{};limits.BasicLimitInformation.LimitFlags=JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
    if(!job||!SetInformationJobObject(job,JobObjectExtendedLimitInformation,&limits,sizeof(limits))){log(L"JOB_ERROR "+hx(GetLastError()));return 7;}
    STARTUPINFOW si{};si.cb=sizeof(si);si.dwFlags=STARTF_USESHOWWINDOW;si.wShowWindow=SW_HIDE;
    if(wfp_ok){
        PROCESS_INFORMATION test{};std::wstring tc=L"\""+std::wstring(selfbuf)+L"\" --net-check";
        if(CreateProcessW(selfbuf,tc.data(),nullptr,nullptr,FALSE,CREATE_SUSPENDED|CREATE_NO_WINDOW,nullptr,nullptr,&si,&test)){
            if(AssignProcessToJobObject(job,test.hProcess)){
                ResumeThread(test.hThread);DWORD tw=WaitForSingleObject(test.hProcess,4000);DWORD te=99;GetExitCodeProcess(test.hProcess,&te);
                if(tw!=WAIT_OBJECT_0||te){log(L"NETWORK_SELFTEST_FAILED "+hx(te));}
                else log(L"NETWORK_SELFTEST_PASS loopback_ok remote_WSAEACCES");
            }else{TerminateProcess(test.hProcess,99);}
            CloseHandle(test.hThread);CloseHandle(test.hProcess);
        }
    }
    SetErrorMode(SEM_FAILCRITICALERRORS|SEM_NOGPFAULTERRORBOX|SEM_NOOPENFILEERRORBOX);PROCESS_INFORMATION pi{};std::wstring command=L"\""+target.wstring()+L"\"";
    if(argc>6){command+=L" ";command+=argv[6];log(L"SYNTHETIC_TEST_ARGUMENTS "+std::wstring(argv[6]));}
    bool interactive=argc>4&&std::wstring(argv[4])==L"interactive-ui";
    bool capture_only=argc>4&&std::wstring(argv[4])==L"trace-owned-ui";
    bool normal=interactive||(argc>4&&std::wstring(argv[4])==L"normal-ui");
    bool root_debug=capture_only||(argc>4&&std::wstring(argv[4])==L"trace-root-ui");
    if(argc>4&&(std::wstring(argv[4])==L"trace-ui"||normal||root_debug))si.wShowWindow=SW_SHOWNORMAL;
    if(!CreateProcessW(target.c_str(),command.data(),nullptr,nullptr,FALSE,(normal?0:(root_debug?DEBUG_ONLY_THIS_PROCESS:DEBUG_PROCESS))|CREATE_SUSPENDED,nullptr,root.c_str(),&si,&pi)){log(L"CREATE_ERROR "+hx(GetLastError()));CloseHandle(job);return 11;}
    if(!AssignProcessToJobObject(job,pi.hProcess)){log(L"ASSIGN_ERROR "+hx(GetLastError()));TerminateProcess(pi.hProcess,99);CloseHandle(pi.hThread);CloseHandle(pi.hProcess);CloseHandle(job);return 12;}
    log(L"ROOT_PID "+std::to_wstring(pi.dwProcessId));ResumeThread(pi.hThread);
    if(normal){
        log(L"NORMAL_RUN no_debugger no_breakpoints");if(interactive)log(L"INTERACTIVE_RUN until_client_closes");std::set<DWORD> seen;ULONGLONG deadline=GetTickCount64()+seconds*1000;bool exited=false;
        while(interactive||GetTickCount64()<deadline){
            BYTE buffer[sizeof(JOBOBJECT_BASIC_PROCESS_ID_LIST)+sizeof(ULONG_PTR)*128]{};auto list=(JOBOBJECT_BASIC_PROCESS_ID_LIST*)buffer;
            if(QueryInformationJobObject(job,JobObjectBasicProcessIdList,list,sizeof(buffer),nullptr))for(DWORD i=0;i<list->NumberOfProcessIdsInList;i++){
                DWORD pid=(DWORD)list->ProcessIdList[i];if(seen.insert(pid).second){HANDLE h=OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION,FALSE,pid);if(h){log(L"JOB_PROCESS pid="+std::to_wstring(pid)+L" path="+process_path(h));CloseHandle(h);}}
            }
            if(WaitForSingleObject(pi.hProcess,200)==WAIT_OBJECT_0){exited=true;break;}
        }
        if(!exited)log(L"TIMEOUT");DWORD code=STILL_ACTIVE;GetExitCodeProcess(pi.hProcess,&code);log(L"NORMAL_BEFORE_CLEANUP exit="+hx(code));TerminateJobObject(job,0xE0000001);WaitForSingleObject(pi.hProcess,2000);GetExitCodeProcess(pi.hProcess,&code);log(L"SUMMARY exit="+hx(code)+L" normal_run=1");CloseHandle(pi.hThread);CloseHandle(pi.hProcess);CloseHandle(job);log(L"JOB_CLOSED");return 0;
    }
    std::map<DWORD,HANDLE> processes;bool done=false,ep_hit=false;std::set<DWORD> initial_breaks;BYTE original=0;void* ep=nullptr;unsigned exceptions=0;ULONGLONG deadline=GetTickCount64()+seconds*1000;
    if(capture_only)log(L"OWNED_EXCEPTION_TRACE no_code_breakpoints until_client_closes");
    while(!done&&(capture_only||GetTickCount64()<deadline)){
        DEBUG_EVENT ev{};if(!WaitForDebugEvent(&ev,200))continue;DWORD cont=DBG_CONTINUE;
        switch(ev.dwDebugEventCode){
        case CREATE_PROCESS_DEBUG_EVENT:{
            auto& v=ev.u.CreateProcessInfo;processes[ev.dwProcessId]=v.hProcess;std::wstring pp=process_path(v.hProcess);log(L"CREATE_PROCESS pid="+std::to_wstring(ev.dwProcessId)+L" base="+hx((ULONGLONG)v.lpBaseOfImage)+L" path="+pp);
            if(!under(fs::path(pp),root)){
                auto cmd=command_of(v.hProcess);auto path=pp;std::transform(path.begin(),path.end(),path.begin(),towlower);auto cc=cmd;std::transform(cc.begin(),cc.end(),cc.begin(),towlower);
                bool allowed=path==L"c:\\windows\\system32\\systeminfo.exe"&&(cc==L"systeminfo"||cc==L"systeminfo.exe"||cc==path||cc==L"\""+path+L"\"");
                if(allowed)log(L"OUTSIDE_CHILD_ALLOWED_SYSTEMINFO no_arguments");else{log(L"OUTSIDE_CHILD_BLOCKED");TerminateProcess(v.hProcess,99);}
            }
            if(!capture_only)install_apis(v.hProcess,ev.dwProcessId,(ULONGLONG)v.lpBaseOfImage,pp,ev.dwProcessId==pi.dwProcessId);
            if(!capture_only&&ev.dwProcessId==pi.dwProcessId){ep=(BYTE*)v.lpBaseOfImage+get_entry(target);SIZE_T n=0;DWORD old=0;if(ReadProcessMemory(v.hProcess,ep,&original,1,&n)&&VirtualProtectEx(v.hProcess,ep,1,PAGE_EXECUTE_READWRITE,&old)){BYTE cc=0xCC;BOOL w=WriteProcessMemory(v.hProcess,ep,&cc,1,&n);DWORD tmp;VirtualProtectEx(v.hProcess,ep,1,old,&tmp);FlushInstructionCache(v.hProcess,ep,1);log(L"ENTRY_BREAKPOINT addr="+hx((ULONGLONG)ep)+L" installed="+std::to_wstring(w));}}
            if(v.hFile)CloseHandle(v.hFile);if(v.hThread)CloseHandle(v.hThread);break;}
        case CREATE_THREAD_DEBUG_EVENT:if(ev.u.CreateThread.hThread)CloseHandle(ev.u.CreateThread.hThread);break;
        case LOAD_DLL_DEBUG_EVENT:{auto& v=ev.u.LoadDll;auto path=path_of(v.hFile);log(L"LOAD_DLL pid="+std::to_wstring(ev.dwProcessId)+L" base="+hx((ULONGLONG)v.lpBaseOfDll)+L" path="+path);if(!capture_only)install_apis(processes[ev.dwProcessId],ev.dwProcessId,(ULONGLONG)v.lpBaseOfDll,path,ev.dwProcessId==pi.dwProcessId);if(v.hFile)CloseHandle(v.hFile);break;}
        case OUTPUT_DEBUG_STRING_EVENT:{auto v=ev.u.DebugString;SIZE_T n=0;auto h=processes[ev.dwProcessId];std::wstring t;
            if(v.fUnicode){std::vector<wchar_t> s(std::min<unsigned>(v.nDebugStringLength,4000)+1);ReadProcessMemory(h,v.lpDebugStringData,s.data(),(s.size()-1)*2,&n);t=s.data();}
            else {std::vector<char>s(std::min<unsigned>(v.nDebugStringLength,4000)+1);ReadProcessMemory(h,v.lpDebugStringData,s.data(),s.size()-1,&n);for(char ch:s){if(!ch)break;t+=(wchar_t)(unsigned char)ch;}}
            log(L"DEBUG_STRING "+t);break;}
        case EXCEPTION_DEBUG_EVENT:{auto& v=ev.u.Exception;auto& r=v.ExceptionRecord;auto h=processes[ev.dwProcessId];
            if(r.ExceptionCode==EXCEPTION_SINGLE_STEP&&pending_steps.count(ev.dwThreadId)){
                auto key=pending_steps[ev.dwThreadId];patch_byte(h,key.second,0xCC);pending_steps.erase(ev.dwThreadId);HANDLE th=OpenThread(THREAD_GET_CONTEXT|THREAD_SET_CONTEXT,FALSE,ev.dwThreadId);CONTEXT c{};c.ContextFlags=CONTEXT_CONTROL;if(th&&GetThreadContext(th,&c)){c.EFlags&=~0x100;SetThreadContext(th,&c);}if(th)CloseHandle(th);break;
            }
            auto key=std::make_pair(ev.dwProcessId,(ULONGLONG)r.ExceptionAddress);auto ap=api_breaks.find(key);
            if(r.ExceptionCode==EXCEPTION_BREAKPOINT&&ap!=api_breaks.end()){
                auto bp=ap->second;HANDLE th=OpenThread(THREAD_GET_CONTEXT|THREAD_SET_CONTEXT,FALSE,ev.dwThreadId);CONTEXT c{};c.ContextFlags=CONTEXT_FULL;
                if(th&&GetThreadContext(th,&c)){
                    std::wstring msg=L"API pid="+std::to_wstring(ev.dwProcessId)+L" "+bp.label+L" RAX="+hx(c.Rax)+L" RCX="+hx(c.Rcx)+L" RDX="+hx(c.Rdx)+L" R8="+hx(c.R8)+L" R9="+hx(c.R9);
                    ULONGLONG ret=0;SIZE_T n;ReadProcessMemory(h,(void*)c.Rsp,&ret,8,&n);msg+=L" CALLER="+hx(ret);
                    if(bp.label.rfind(L"CreateFile",0)==0)msg+=L" FILE="+remote_text(h,c.Rcx,bp.label.back()==L'W');
                    if(bp.label.rfind(L"MessageBox",0)==0)msg+=L" TEXT="+remote_text(h,c.Rdx,bp.label.back()==L'W')+L" TITLE="+remote_text(h,c.R8,bp.label.back()==L'W');
                    if(bp.label==L"PARSER_INPUT")msg+=L" INPUT="+remote_text(h,c.Rdx,true);
                    if(bp.label==L"PACKET_CRC")msg+=L" EXPECTED="+hx(c.Rax)+L" HEADER="+remote_hex(h,c.R15,16)+L" POLY_GLOBALS="+remote_hex(h,0x14F0E9620,16);
                    if(bp.label==L"SESSION_KEYS")msg+=L" KEY_INPUT="+remote_hex(h,c.Rdx,std::min<ULONGLONG>(c.R8,512));
                    auto relevantPacket=[](unsigned id){return id==1||id==4||id==5||id==8||id==684||id==1554;};
                    if(bp.label==L"SEND_RAW"){
                        unsigned short id=0;ReadProcessMemory(h,(void*)(c.Rdx+1),&id,2,&n);
                        if(relevantPacket(id))msg+=L" WIRE="+remote_hex(h,c.Rdx,c.R8);
                    }
                    if(bp.label==L"SEND_PLAIN"&&relevantPacket((unsigned)c.Rdx))msg+=L" PLAIN="+remote_hex(h,c.R8,c.R9);
                    if(bp.label==L"ARGUMENT_VECTOR"){
                        ULONGLONG bounds[2]{};if(ReadProcessMemory(h,(void*)(c.Rcx+0x100),bounds,sizeof(bounds),&n)&&bounds[1]>=bounds[0]&&(bounds[1]-bounds[0])/16<=32){
                            for(unsigned j=0;j<(bounds[1]-bounds[0])/16;j++){ULONGLONG ptr=0;if(ReadProcessMemory(h,(void*)(bounds[0]+j*16),&ptr,8,&n))msg+=L" ARG["+std::to_wstring(j)+L"]="+remote_text(h,ptr,true);}
                        }
                    }
                    log(msg);if(bp.label.find(L"Exit")!=std::wstring::npos||bp.label==L"NtTerminateProcess")dump_context(h,ev.dwThreadId,r.ExceptionAddress);
                    patch_byte(h,key.second,bp.original);c.Rip=key.second;if(bp.persistent){c.EFlags|=0x100;pending_steps[ev.dwThreadId]=key;}else api_breaks.erase(key);SetThreadContext(th,&c);
                }
                if(th)CloseHandle(th);break;
            }
            if(r.ExceptionCode==0x406D1388){log(L"THREAD_NAME_EVENT pid="+std::to_wstring(ev.dwProcessId));break;}
            log(L"EXCEPTION pid="+std::to_wstring(ev.dwProcessId)+L" code="+hx(r.ExceptionCode)+L" first="+std::to_wstring(v.dwFirstChance)+L" address="+hx((ULONGLONG)r.ExceptionAddress));
            if(r.ExceptionCode==EXCEPTION_BREAKPOINT&&r.ExceptionAddress==ep&&ev.dwProcessId==pi.dwProcessId){ep_hit=true;SIZE_T n;DWORD old,tmp;VirtualProtectEx(h,ep,1,PAGE_EXECUTE_READWRITE,&old);WriteProcessMemory(h,ep,&original,1,&n);VirtualProtectEx(h,ep,1,old,&tmp);FlushInstructionCache(h,ep,1);HANDLE th=OpenThread(THREAD_GET_CONTEXT|THREAD_SET_CONTEXT,FALSE,ev.dwThreadId);CONTEXT c{};c.ContextFlags=CONTEXT_CONTROL;if(th&&GetThreadContext(th,&c)){c.Rip=(DWORD64)ep;SetThreadContext(th,&c);}if(th)CloseHandle(th);log(L"ENTRY_REACHED");if(stop_entry){TerminateJobObject(job,0xE0000002);}}
            else if(r.ExceptionCode==EXCEPTION_BREAKPOINT&&!initial_breaks.count(ev.dwProcessId)){initial_breaks.insert(ev.dwProcessId);log(L"INITIAL_DEBUGGER_BREAK");}
            else{cont=DBG_EXCEPTION_NOT_HANDLED;dump_context(h,ev.dwThreadId,r.ExceptionAddress);for(DWORD i=0;i<r.NumberParameters;i++)log(L"EXCEPTION_PARAM "+std::to_wstring(i)+L" "+hx(r.ExceptionInformation[i]));if((!v.dwFirstChance||r.ExceptionCode==EXCEPTION_ACCESS_VIOLATION)&&++exceptions>30)TerminateJobObject(job,0xE0000003);}
            break;}
        case EXIT_PROCESS_DEBUG_EVENT:log(L"EXIT_PROCESS pid="+std::to_wstring(ev.dwProcessId)+L" code="+hx(ev.u.ExitProcess.dwExitCode));if(ev.dwProcessId==pi.dwProcessId)done=true;break;
        default:break;
        }
        ContinueDebugEvent(ev.dwProcessId,ev.dwThreadId,cont);
    }
    if(!done)log(L"TIMEOUT");TerminateJobObject(job,0xE0000001);
    ULONGLONG drain=GetTickCount64()+2000;while(GetTickCount64()<drain){DEBUG_EVENT ev{};if(!WaitForDebugEvent(&ev,100))break;ContinueDebugEvent(ev.dwProcessId,ev.dwThreadId,DBG_CONTINUE);}
    WaitForSingleObject(pi.hProcess,2000);DWORD code=0;GetExitCodeProcess(pi.hProcess,&code);log(L"SUMMARY exit="+hx(code)+L" entry_reached="+std::to_wstring(ep_hit));
    for(auto& [pid,h]:processes)if(h&&h!=pi.hProcess)CloseHandle(h);CloseHandle(pi.hThread);CloseHandle(pi.hProcess);CloseHandle(job);log(L"JOB_CLOSED");return 0;
}
