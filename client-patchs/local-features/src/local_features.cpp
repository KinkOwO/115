#define WIN32_LEAN_AND_MEAN
#define _CRT_SECURE_NO_WARNINGS
#include <winsock2.h>
#include <windows.h>
#include <iphlpapi.h>
#include <commctrl.h>
#include <bcrypt.h>
#include <cstdio>
#include <memory>
#include <algorithm>
#include "protocol.h"
#include "native_inventory.h"
#include "native_symbol_runtime.h"

namespace {
HMODULE selfModule;HWND window,gameWindow,list,detail,rowDetail,claimButton,refreshButton;HFONT uiFont;
features::Controller controller;size_t selectedCampaign=0;HANDLE worker=nullptr;FILE*logFile=nullptr;
constexpr UINT Done=WM_APP+70;
std::wstring wide(const std::string&s){if(s.empty())return {};int n=MultiByteToWideChar(CP_UTF8,MB_ERR_INVALID_CHARS,s.data(),int(s.size()),nullptr,0);if(!n)throw std::runtime_error("invalid UTF8");std::wstring w(n,0);MultiByteToWideChar(CP_UTF8,MB_ERR_INVALID_CHARS,s.data(),int(s.size()),&w[0],n);return w;}
bool expectedClient(){
 wchar_t path[32768];if(!GetModuleFileNameW(nullptr,path,32768))return false;HANDLE f=CreateFileW(path,GENERIC_READ,FILE_SHARE_READ,nullptr,OPEN_EXISTING,FILE_FLAG_SEQUENTIAL_SCAN,nullptr);if(f==INVALID_HANDLE_VALUE)return false;
 BCRYPT_ALG_HANDLE a=nullptr;BCRYPT_HASH_HANDLE h=nullptr;unsigned char digest[32];bool ok=false;
 if(BCryptOpenAlgorithmProvider(&a,BCRYPT_SHA256_ALGORITHM,nullptr,0)>=0&&BCryptCreateHash(a,&h,nullptr,0,nullptr,0,0)>=0){unsigned char buf[65536];DWORD n;bool good=true;for(;;){if(!ReadFile(f,buf,sizeof buf,&n,nullptr)){good=false;break;}if(!n)break;if(BCryptHashData(h,buf,n,0)<0){good=false;break;}}if(good&&BCryptFinishHash(h,digest,32,0)>=0){const unsigned char expected[32]={0x51,0x5f,0xef,0xa1,0xc7,0x15,0x77,0xa1,0x5f,0x90,0x51,0x3f,0x86,0x02,0x70,0x32,0x0f,0x35,0x02,0xbc,0x49,0x2a,0xd5,0x96,0xf1,0x0c,0xf1,0x76,0xb9,0x1c,0x21,0xeb};ok=memcmp(digest,expected,32)==0;}}
 if(h)BCryptDestroyHash(h);if(a)BCryptCloseAlgorithmProvider(a,0);CloseHandle(f);return ok;
}
std::vector<MIB_TCPROW_OWNER_PID> connections(){ULONG n=0;GetExtendedTcpTable(nullptr,&n,FALSE,AF_INET,TCP_TABLE_OWNER_PID_ALL,0);for(int retry=0;retry<3;retry++){if(n<4||n>16*1024*1024)break;std::vector<uint8_t>b(n);DWORD e=GetExtendedTcpTable(b.data(),&n,FALSE,AF_INET,TCP_TABLE_OWNER_PID_ALL,0);if(e==ERROR_INSUFFICIENT_BUFFER)continue;if(e)break;auto*t=reinterpret_cast<MIB_TCPTABLE_OWNER_PID*>(b.data());if(uint64_t(t->dwNumEntries)*sizeof(MIB_TCPROW_OWNER_PID)+4>b.size())break;return {t->table,t->table+t->dwNumEntries};}return {};}
bool loopback(DWORD address){return reinterpret_cast<unsigned char*>(&address)[0]==127;}
HANDLE pipe(){
 auto rows=connections();auto pid=GetCurrentProcessId();
 for(const auto&r:rows){if(r.dwOwningPid!=pid||r.dwState!=MIB_TCP_STATE_ESTAB||!loopback(r.dwLocalAddr)||!loopback(r.dwRemoteAddr))continue;
  DWORD serverPid=0;for(const auto&s:rows)if(s.dwState==MIB_TCP_STATE_ESTAB&&s.dwLocalAddr==r.dwRemoteAddr&&s.dwLocalPort==r.dwRemotePort&&s.dwRemoteAddr==r.dwLocalAddr&&s.dwRemotePort==r.dwLocalPort){serverPid=s.dwOwningPid;break;}if(!serverPid||serverPid==pid)continue;
  wchar_t name[160];swprintf_s(name,L"\\\\.\\pipe\\115us-features-v1-%lu-%u-%u",pid,unsigned(ntohs(USHORT(r.dwLocalPort))),unsigned(ntohs(USHORT(r.dwRemotePort))));
  HANDLE h=INVALID_HANDLE_VALUE;
  // A confirmed snapshot is immediately followed by its refresh request.
  // Give the server time to observe the previous handle closing instead of
  // reporting a lost session during the normal reconnect interval.
  for(int retry=0;retry<5;++retry){
   h=CreateFileW(name,GENERIC_READ|GENERIC_WRITE,0,nullptr,OPEN_EXISTING,FILE_FLAG_OVERLAPPED,nullptr);
   if(h!=INVALID_HANDLE_VALUE||GetLastError()!=ERROR_PIPE_BUSY)break;
   WaitNamedPipeW(name,100);
  }
  if(h==INVALID_HANDLE_VALUE)continue;ULONG actual=0;if(GetNamedPipeServerProcessId(h,&actual)&&actual==serverPid)return h;CloseHandle(h);
 }throw std::runtime_error("no bound local game session");
}
void transfer(HANDLE h,uint8_t*p,size_t size,bool write){
 while(size){OVERLAPPED o{};o.hEvent=CreateEventW(nullptr,TRUE,FALSE,nullptr);if(!o.hEvent)throw std::runtime_error("IPC event failed");DWORD n=0;BOOL ok=write?WriteFile(h,p,DWORD(size),&n,&o):ReadFile(h,p,DWORD(size),&n,&o);DWORD error=ok?0:GetLastError();
  if(!ok&&error==ERROR_IO_PENDING){DWORD wait=WaitForSingleObject(o.hEvent,4500);if(wait==WAIT_OBJECT_0)ok=GetOverlappedResult(h,&o,&n,FALSE);else{CancelIoEx(h,&o);GetOverlappedResult(h,&o,&n,TRUE);ok=FALSE;}}
  CloseHandle(o.hEvent);if(!ok||!n||n>size)throw std::runtime_error("IPC disconnected or timed out");p+=n;size-=n;
 }
}
struct Job {uint16_t type;uint64_t id;std::vector<uint8_t>body;};
std::vector<uint8_t> symbolTransport(uint16_t kind,uint64_t id,const std::vector<uint8_t>&body){
 static std::mutex transportMutex;std::lock_guard<std::mutex> lock(transportMutex);
 HANDLE h=pipe();
 try{
  auto p=features::frame(kind,id,body);transfer(h,p.data(),p.size(),true);
  std::vector<uint8_t> header(24);transfer(h,header.data(),header.size(),false);uint32_t status=0;
  auto n=features::replyHeader(header,kind,id,status);
  if(status)throw std::runtime_error(("server rejected symbol operation: "+std::to_string(status)).c_str());
  std::vector<uint8_t> reply(n);if(n)transfer(h,reply.data(),reply.size(),false);
  CloseHandle(h);return reply;
 }catch(...){CloseHandle(h);throw;}
}
struct Result {uint16_t type;uint64_t id;uint32_t status=0;features::View view;bool failed=false;};
DWORD WINAPI request(void*arg){
 std::unique_ptr<Job>j(static_cast<Job*>(arg));auto r=new Result;r->type=j->type;r->id=j->id;HANDLE h=INVALID_HANDLE_VALUE;
 try{h=pipe();auto p=features::frame(j->type,j->id,j->body);transfer(h,p.data(),p.size(),true);std::vector<uint8_t>header(24);transfer(h,header.data(),24,false);auto n=features::replyHeader(header,j->type,j->id,r->status);if(r->status==0){std::vector<uint8_t>body(n);transfer(h,body.data(),n,false);r->view=features::view(body);}}
 catch(const std::exception&){r->failed=true;}
 if(h!=INVALID_HANDLE_VALUE)CloseHandle(h);if(!PostMessageW(window,Done,0,reinterpret_cast<LPARAM>(r)))delete r;return 0;
}
void selectionDetail(){
 std::wstring text=L"选择一行查看完整奖励内容";
 if(controller.ready&&selectedCampaign<controller.current.campaigns.size()){
  const auto&c=controller.current.campaigns[selectedCampaign];int i=ListView_GetNextItem(list,-1,LVNI_SELECTED);
  if(i>=0&&size_t(i)<c.rows.size())text=wide(c.rows[i].label);
 }
 if(rowDetail)SetWindowTextW(rowDetail,text.c_str());
}
void buttons(){bool can=false;if(controller.ready&&selectedCampaign<controller.current.campaigns.size()){const auto&c=controller.current.campaigns[selectedCampaign];int i=ListView_GetNextItem(list,-1,LVNI_SELECTED);can=c.state==0&&i>=0&&size_t(i)<c.rows.size()&&c.rows[i].state==1;}EnableWindow(claimButton,can);EnableWindow(refreshButton,!controller.pending);}
std::wstring duration(uint64_t seconds){
 wchar_t value[64];swprintf_s(value,L"%llu:%02llu:%02llu",seconds/3600,(seconds/60)%60,seconds%60);return value;
}
void render(bool preserveSelection=false){
 int selected=preserveSelection?ListView_GetNextItem(list,-1,LVNI_SELECTED):-1;
 ListView_DeleteAllItems(list);selectionDetail();if(!controller.ready||selectedCampaign>=controller.current.campaigns.size()){buttons();return;}const auto&c=controller.current.campaigns[selectedCampaign];SetWindowTextW(detail,wide(c.detail).c_str());
 for(size_t i=0;i<c.rows.size();i++){const auto&r=c.rows[i];std::wstring label=wide(r.label);LVITEMW item{};item.mask=LVIF_TEXT;item.iItem=int(i);item.pszText=&label[0];ListView_InsertItem(list,&item);auto separator=label.find(L'：');std::wstring material;
 if(separator!=std::wstring::npos){material=label.substr(separator+1);label.resize(separator);ListView_SetItemText(list,int(i),0,&label[0]);}
 else material=L"数量 "+std::to_wstring(r.count);ListView_SetItemText(list,int(i),1,&material[0]);std::wstring progress=std::to_wstring(r.progress)+L" / "+std::to_wstring(r.goal);if(c.family==3||(c.family==1&&r.group==1))progress=duration(r.progress)+L" / "+duration(r.goal);
 ListView_SetItemText(list,int(i),2,&progress[0]);std::wstring state=r.state==2?L"已领取":r.state==1?L"可领取":L"未达成";ListView_SetItemText(list,int(i),3,&state[0]);}
 if(selected>=0&&size_t(selected)<c.rows.size())ListView_SetItemState(list,selected,LVIS_SELECTED|LVIS_FOCUSED,LVIS_SELECTED|LVIS_FOCUSED);
 selectionDetail();buttons();
}
void begin(uint16_t type,std::vector<uint8_t>body={}){
 if(controller.pending)return;try{auto id=controller.begin(type);auto*j=new Job{type,id,std::move(body)};worker=CreateThread(nullptr,0,request,j,0,nullptr);if(!worker){delete j;controller.fail();}SetWindowTextW(detail,L"正在同步…");buttons();}catch(const std::exception&){controller.fail();buttons();}
}
HWND control(const wchar_t*cls,const wchar_t*text,DWORD style,int x,int y,int w,int h,int id){HWND c=CreateWindowExW(0,cls,text,WS_CHILD|WS_VISIBLE|style,x,y,w,h,window,reinterpret_cast<HMENU>(INT_PTR(id)),selfModule,nullptr);SendMessageW(c,WM_SETFONT,reinterpret_cast<WPARAM>(uiFont),TRUE);return c;}
LRESULT CALLBACK wndproc(HWND h,UINT msg,WPARAM wp,LPARAM lp){
 switch(msg){case WM_CREATE:{window=h;uiFont=CreateFontW(-16,0,0,0,FW_NORMAL,FALSE,FALSE,FALSE,DEFAULT_CHARSET,OUT_DEFAULT_PRECIS,CLIP_DEFAULT_PRECIS,CLEARTYPE_QUALITY,DEFAULT_PITCH,L"Microsoft YaHei UI");const wchar_t*titles[]={L"周常",L"十日签到",L"在线奖励",L"拾光之证",L"徽章"};for(int i=0;i<5;i++)control(L"BUTTON",titles[i],BS_PUSHBUTTON,12+152*i,12,146,32,100+i);detail=control(L"STATIC",L"请进入角色后刷新活动",0,14,56,760,74,0);list=control(WC_LISTVIEWW,L"",LVS_REPORT|LVS_SINGLESEL|LVS_SHOWSELALWAYS,12,138,760,300,10);ListView_SetExtendedListViewStyle(list,LVS_EX_FULLROWSELECT|LVS_EX_DOUBLEBUFFER);const wchar_t*cols[]={L"目标",L"奖励内容",L"进度",L"状态"};int widths[]={230,280,150,92};for(int i=0;i<4;i++){LVCOLUMNW col{};col.mask=LVCF_TEXT|LVCF_WIDTH;col.cx=widths[i];col.pszText=const_cast<wchar_t*>(cols[i]);ListView_InsertColumn(list,i,&col);}rowDetail=control(L"STATIC",L"选择一行查看完整奖励内容",0,14,450,760,84,0);refreshButton=control(L"BUTTON",L"刷新",BS_PUSHBUTTON,560,548,96,34,20);claimButton=control(L"BUTTON",L"领取",BS_PUSHBUTTON,666,548,106,34,21);SetTimer(h,1,15000,nullptr);buttons();return 0;}
 case WM_COMMAND:{int id=LOWORD(wp);if(id>=100&&id<105){selectedCampaign=size_t(id-100);render();}else if(id==20)begin(1);else if(id==21&&controller.ready){int i=ListView_GetNextItem(list,-1,LVNI_SELECTED);if(selectedCampaign<controller.current.campaigns.size()){const auto&c=controller.current.campaigns[selectedCampaign];if(i>=0&&size_t(i)<c.rows.size()){try{begin(2,features::claim(c,c.rows[i]));}catch(const std::exception&){buttons();}}}}return 0;}
 case WM_NOTIFY:selectionDetail();buttons();return 0;
 case Done:{std::unique_ptr<Result>r(reinterpret_cast<Result*>(lp));if(worker){CloseHandle(worker);worker=nullptr;}if(r->failed||r->status){controller.fail();ListView_DeleteAllItems(list);selectionDetail();const wchar_t*message=r->failed?L"连接已断开或超时，请进入角色后刷新":r->status==1?L"请先进入角色":r->status==2?L"此服务端尚未支持该功能":r->status==3?L"未满足领取条件，或活动资源已变化，请刷新":L"同步失败，请刷新";SetWindowTextW(detail,message);buttons();}else{try{controller.accept(r->id,r->type,std::move(r->view));render(true);}catch(const std::exception&){controller.fail();buttons();}}return 0;}
 case WM_TIMER:if(wp==1&&IsWindowVisible(h)&&!controller.pending)begin(1);return 0;
 case WM_HOTKEY:{HWND fg=GetForegroundWindow();DWORD pid=0;GetWindowThreadProcessId(fg,&pid);if(pid!=GetCurrentProcessId())return 0;if(IsWindowVisible(h))ShowWindow(h,SW_HIDE);else{RECT rc;GetWindowRect(gameWindow,&rc);SetWindowPos(h,HWND_TOP,rc.left+80,rc.top+70,0,0,SWP_NOSIZE|SWP_NOACTIVATE);ShowWindow(h,SW_SHOW);SetForegroundWindow(h);if(!controller.pending)begin(1);}return 0;}
 case WM_CLOSE:ShowWindow(h,SW_HIDE);return 0;
 case WM_DESTROY:KillTimer(h,1);UnregisterHotKey(h,1);if(uiFont)DeleteObject(uiFont);PostQuitMessage(0);return 0;
 }return DefWindowProcW(h,msg,wp,lp);
}
BOOL CALLBACK findWindow(HWND h,LPARAM){DWORD pid=0;GetWindowThreadProcessId(h,&pid);if(pid==GetCurrentProcessId()&&IsWindowVisible(h)&&GetWindow(h,GW_OWNER)==nullptr){gameWindow=h;return FALSE;}return TRUE;}
}
extern "C" __declspec(dllexport) const char* WINAPI ModName(){return "115US local feature controllers v1";}
extern "C" __declspec(dllexport) DWORD WINAPI ModStart(){
 static volatile LONG started=0;if(InterlockedExchange(&started,1))return 0;
 wchar_t path[32768];if(GetModuleFileNameW(selfModule,path,32768)){wchar_t*slash=wcsrchr(path,L'\\');if(slash){wcscpy_s(slash+1,32768-size_t(slash+1-path),L"local-features.log");logFile=_wfopen(path,L"ab");}}
 if(!expectedClient()){if(logFile){fputs("client fingerprint mismatch; controller disabled\n",logFile);fclose(logFile);}return 1;}
 for(int i=0;i<120&&!gameWindow;i++){EnumWindows(findWindow,0);if(!gameWindow)Sleep(500);}if(!gameWindow){if(logFile)fclose(logFile);return 2;}
 INITCOMMONCONTROLSEX init{sizeof init,ICC_LISTVIEW_CLASSES};InitCommonControlsEx(&init);WNDCLASSW wc{};wc.lpfnWndProc=wndproc;wc.hInstance=selfModule;wc.hCursor=LoadCursorW(nullptr,IDC_ARROW);wc.hbrBackground=reinterpret_cast<HBRUSH>(COLOR_WINDOW+1);wc.lpszClassName=L"115USLocalFeaturesV1";if(!RegisterClassW(&wc)){if(logFile)fclose(logFile);return 3;}
 window=CreateWindowExW(WS_EX_TOOLWINDOW,wc.lpszClassName,L"115US 活动与徽章",WS_CAPTION|WS_SYSMENU,0,0,800,620,gameWindow,nullptr,selfModule,nullptr);if(!window){if(logFile)fclose(logFile);return 4;}
 if(!RegisterHotKey(window,1,MOD_CONTROL|MOD_NOREPEAT,VK_F10)){if(logFile){fputs("activity Ctrl+F10 hotkey registration failed; native emblem remains available\n",logFile);fflush(logFile);}}
 native_inventory::Callbacks symbolCallbacks{};
 // Both native controllers are admitted against the same paired archive.
 if(native_inventory::resourcesMatch()&&native_symbol_runtime::install(reinterpret_cast<uintptr_t>(GetModuleHandleW(nullptr)),logFile,symbolTransport)){
  if(!symbol_effect_sync::start(reinterpret_cast<uintptr_t>(GetModuleHandleW(nullptr)),logFile,symbolTransport)){
   if(logFile){fputs("symbol effects guard rejected; interactions disabled\n",logFile);fflush(logFile);}
  }else
  symbolCallbacks={native_symbol_runtime::enter,native_symbol_runtime::leave,native_symbol_runtime::update,native_symbol_runtime::star,native_symbol_runtime::bag,native_symbol_runtime::header,native_symbol_runtime::incompatible,native_symbol_runtime::draggedSlot,native_symbol_window::popupRoot};
 }else if(logFile){fputs("symbol window guard rejected; interactions disabled\n",logFile);fflush(logFile);}
 if(!native_inventory::install(logFile,symbolCallbacks)){if(logFile){fputs("native inventory guard rejected; native route disabled\n",logFile);fflush(logFile);}}
 if(logFile){fputs("cumulative activity and emblem transport loaded; Ctrl+F10 opens activities\n",logFile);fflush(logFile);}MSG msg;while(GetMessageW(&msg,nullptr,0,0)>0){TranslateMessage(&msg);DispatchMessageW(&msg);}symbol_effect_sync::shutdown();native_inventory::log=nullptr;if(logFile)fclose(logFile);return 0;
}
BOOL WINAPI DllMain(HMODULE m,DWORD reason,LPVOID){if(reason==DLL_PROCESS_ATTACH){selfModule=m;DisableThreadLibraryCalls(m);}return TRUE;}
