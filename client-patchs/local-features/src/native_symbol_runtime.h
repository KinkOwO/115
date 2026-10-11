#pragma once
#include <mutex>
#include <memory>
#include "symbol_protocol.h"
#include "native_symbol_window.h"
#include "native_symbol_info.h"
#include "symbol_effect_sync.h"

namespace native_symbol_runtime {
using Exchange=std::vector<uint8_t>(*)(uint16_t,uint64_t,const std::vector<uint8_t>&);
struct Job {uint64_t generation,id;uint16_t kind;std::vector<uint8_t> body;};
struct Result {uint64_t generation,id;uint16_t kind;symbol_wire::View view;std::string error;};
static Exchange transport;
static FILE* log;
static std::mutex mutex;
static std::unique_ptr<Result> result;
static HANDLE worker;
static uint64_t generation=0,nextID=0,pendingID=0;
static bool active=false,ready=false;
static symbol_wire::View view;
static std::string desiredGroup;
static uint16_t selectedSocket=0;
static ULONGLONG retryAfter=0;
static bool gesture=false;
static uint16_t gestureSlot=0;
static POINT gestureOrigin{};
static ULONGLONG lastRelease=0;

static void report(const char* text){if(log){fprintf(log,"symbol: %s\n",text);fflush(log);}}
static DWORD WINAPI run(void*arg){
    std::unique_ptr<Job> j(static_cast<Job*>(arg));
    auto r=std::make_unique<Result>();r->generation=j->generation;r->id=j->id;r->kind=j->kind;
    try {r->view=symbol_wire::decode(transport(j->kind,j->id,j->body));if(j->kind==4)symbol_effect_sync::changed();}
    catch(const std::exception&e){r->error=e.what();}
    catch(...){r->error="unexpected transport failure";}
    std::lock_guard<std::mutex> lock(mutex);result=std::move(r);return 0;
}
static void begin(uint16_t kind,std::vector<uint8_t> body={}){
    if(worker||!transport||!active)return;
    auto j=std::make_unique<Job>();j->generation=generation;j->id=++nextID;j->kind=kind;j->body=std::move(body);
    pendingID=j->id;ready=false;
    worker=CreateThread(nullptr,0,run,j.get(),0,nullptr);
    if(worker)j.release();else {pendingID=0;retryAfter=GetTickCount64()+2000;report("worker creation failed");}
}
static void render(){
    auto g=view.find(desiredGroup);
    if(!ready||!g)return;
    std::vector<uint32_t> items;for(auto&i:g->items)items.push_back(i.id);
    if(!native_symbol_window::render(desiredGroup,items))report("popup/source slot agreement rejected");
}
static void closed(){if(native_symbol_window::requestedGroup.empty()){desiredGroup.clear();selectedSocket=0;}}
static void opened(const std::string&name){
    if(!active||!ready||name!=desiredGroup){native_symbol_window::close();return;}
    render();
}
static void star(const std::string&name){
    if(!active||!ready||!view.find(name))return;
    desiredGroup=name;selectedSocket=0;
    native_symbol_info::flash(name);
    if(!native_symbol_window::open(name)){desiredGroup.clear();report("native manager refused popup");}
}
static void socket(const std::string&name,uint16_t index){
    auto g=view.find(name);
    if(!ready||name!=desiredGroup||!g||index>=g->items.size())return;
    selectedSocket=index;
    native_symbol_window::flash(index);
}
static void remove(const std::string&name,uint16_t index){
    auto g=view.find(name);
    if(!ready||name!=desiredGroup||!g||index>=g->items.size()||!g->items[index].id)return;
    selectedSocket=index;
    try{begin(4,symbol_wire::exchange(view,name,index,nullptr));}catch(const std::exception&e){report(e.what());}
}
static void equip(uint16_t slot){
    if(!ready||!active)return;
    const symbol_wire::BagItem* incoming=nullptr;
    for(auto&i:view.bag)if(i.slot==slot){incoming=&i;break;}
    if(!incoming)return;
    auto g=view.find(desiredGroup);
    if(!g||!(g->mask&incoming->mask))return;
    if(selectedSocket>=g->items.size())return;
    if(g->items[selectedSocket].id==incoming->id)return;
    try{begin(4,symbol_wire::exchange(view,desiredGroup,selectedSocket,incoming));}catch(const std::exception&e){report(e.what());}
}
static void bag(uint16_t slot){
    if(!ready||!active||GetTickCount64()-lastRelease<120)return;
    if(GetAsyncKeyState(VK_LBUTTON)&0x8000){
        if(!gesture){gesture=true;gestureSlot=slot;GetCursorPos(&gestureOrigin);if(log){fprintf(log,"symbol drag begin bag=%u\n",slot);fflush(log);}}
    }else if(!gesture){equip(slot);lastRelease=GetTickCount64();}
}
static bool incompatible(uint16_t slot){
    if(!active||desiredGroup.empty()||!native_symbol_window::popup)return false;
    auto g=view.find(desiredGroup);if(!g)return true;
    for(auto&i:view.bag)if(i.slot==slot)return !(i.mask&g->mask);
    return true;
}
static int draggedSlot(){
    if(!active||!ready||!gesture||!(GetAsyncKeyState(VK_LBUTTON)&0x8000))return -1;
    DWORD pid=0;GetWindowThreadProcessId(GetForegroundWindow(),&pid);
    if(pid!=GetCurrentProcessId())return -1;
    POINT at{};GetCursorPos(&at);
    if(abs(at.x-gestureOrigin.x)<GetSystemMetrics(SM_CXDRAG)&&abs(at.y-gestureOrigin.y)<GetSystemMetrics(SM_CYDRAG))return -1;
    return gestureSlot;
}
static void header(void* tab){native_symbol_info::header(tab,view,ready);}
static void update(){
    std::unique_ptr<Result> r;
    {std::lock_guard<std::mutex> lock(mutex);r=std::move(result);}
    if(r){
        if(worker){CloseHandle(worker);worker=nullptr;}
        if(active&&r->generation==generation&&r->id==pendingID){
            pendingID=0;
            if(!r->error.empty()){
                report(r->error.c_str());ready=false;retryAfter=GetTickCount64()+1500;
            }else if(r->kind==4&&(r->view.role!=view.role||r->view.source!=view.source||r->view.sequence!=view.sequence+1)){
                report("exchange identity/sequence mismatch");ready=false;retryAfter=GetTickCount64()+1500;
            }else{
                view=std::move(r->view);ready=true;retryAfter=0;
                if(r->kind==4){native_symbol_window::flash(selectedSocket);native_symbol_info::flash(desiredGroup);}
                if(!desiredGroup.empty())render();
            }
        }
    }
    if(gesture&&!(GetAsyncKeyState(VK_LBUTTON)&0x8000)){
        gesture=false;lastRelease=GetTickCount64();
        DWORD pid=0;GetWindowThreadProcessId(GetForegroundWindow(),&pid);
        POINT end{};GetCursorPos(&end);
        const bool moved=abs(end.x-gestureOrigin.x)>=GetSystemMetrics(SM_CXDRAG)||abs(end.y-gestureOrigin.y)>=GetSystemMetrics(SM_CYDRAG);
        const int target=native_symbol_window::dropTarget();
        if(log){fprintf(log,"symbol drag release moved=%d target=%d ready=%d foreground=%d\n",int(moved),target,int(ready),int(pid==GetCurrentProcessId()));fflush(log);}
        if(active&&ready&&pid==GetCurrentProcessId()&&(!moved||target>=0)){
            if(target>=0){selectedSocket=uint16_t(target);native_symbol_window::highlight(selectedSocket);}
            equip(gestureSlot);
        }
    }
    native_symbol_info::summary(view,ready);
    if(active&&!ready&&!worker&&GetTickCount64()>=retryAfter)begin(3);
}
static void enter(){++generation;active=true;ready=false;gesture=false;native_symbol_info::reset();view={};desiredGroup.clear();selectedSocket=0;retryAfter=0;update();}
static void leave(){++generation;active=false;ready=false;gesture=false;pendingID=0;native_symbol_window::close();native_symbol_info::reset();desiredGroup.clear();view={};}
static bool install(uintptr_t base,FILE* sink,Exchange exchange){
    transport=exchange;log=sink;
    return native_symbol_window::install(base,sink,{opened,socket,closed,update,remove});
}
}
