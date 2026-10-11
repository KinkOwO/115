#pragma once
#include <array>
#include <string>
#include <vector>
#include "../../elite-companion-pilot/src/inline_hook.h"
#include "native_symbol_widgets.h"
#include "native_symbol_window_binding.h"

// The native manager owns allocation, focus, parent attachment, close and
// destruction. No foreign HWND and no writes to manager container internals.
// Install only after the caller verifies the PE and the paired resource hash.
namespace native_symbol_window {
using Factory = void*(__fastcall*)(int,void*);
using Destroy = void*(__fastcall*)(void*,unsigned);
using Tick = void(__fastcall*)(void*);
struct Callbacks {
    void (*opened)(const std::string&)=nullptr;
    void (*selected)(const std::string&,uint16_t)=nullptr;
    void (*closed)()=nullptr;
    void (*updated)()=nullptr;
    void (*removed)(const std::string&,uint16_t)=nullptr;
};
static uintptr_t base;
static InlineHook factoryHook;
static native_symbol_widgets::API widgets;
static void* popup;
static void* infoPopup;
static uint64_t infoGeneration=0;
static void* infoManager;
static bool infoRequested=false;
static void* manager;
static FILE* log;
static DWORD uiThread;
static bool closing;
static bool rightWasDown;
static uint16_t selection=0;
static int hoverSocket=-1;
static ULONGLONG flashUntil=0;
static uint16_t flashSocket=0;
static std::string group;
static std::string requestedGroup;
static Callbacks callbacks;
// Include the RTTI pointer preceding the table for native dynamic_cast.
static std::array<uintptr_t,121> table;
static uintptr_t address(uintptr_t va){return base+va-0x140000000ull;}
template<class T> static T read(void* p,size_t off=0){
    T v{};SIZE_T n=0;
    if(p)ReadProcessMemory(GetCurrentProcess(),static_cast<char*>(p)+off,&v,sizeof v,&n);
    return n==sizeof v?v:T{};
}
static void release(void* owner){
    if(!owner)return;
    if(InterlockedDecrement(reinterpret_cast<volatile LONG*>(static_cast<char*>(owner)+8))==0){
        auto vt=read<uintptr_t*>(owner);
        reinterpret_cast<void(__fastcall*)(void*)>(vt[0])(owner);
        if(InterlockedDecrement(reinterpret_cast<volatile LONG*>(static_cast<char*>(owner)+12))==0)
            reinterpret_cast<void(__fastcall*)(void*)>(vt[1])(owner);
    }
}
struct Control {
    void* value=nullptr;void* owner=nullptr;
    Control(void* root,const wchar_t* name,bool child=false){
        // Window and widget lookup have different native object layouts.
        // Never send a popup object into IControl's child-tree traversal.
        if(child&&(root==popup||root==infoPopup))return;
        if(root)reinterpret_cast<void*(__fastcall*)(void*,void*,const wchar_t*)>
            (address(child?0x146ec8e30:0x145f6e4e0))(root,this,name);
    }
    ~Control(){release(owner);}
    Control(const Control&)=delete;
};
static void visible(void* control,bool show){
    if(control)reinterpret_cast<void(__fastcall*)(void*,bool)>(read<uintptr_t*>(control)[2])(control,show);
}
static bool clicked(void* control){
    // Same click predicate used by the native popup's close button.
    return control&&reinterpret_cast<bool(__fastcall*)(void*)>(address(0x146ecfd90))(control);
}
static bool hovered(void* control){
    // Continuous hover including descendants. 146ecfff0 is a different
    // transient input identity and does not stay set while the pointer rests.
    return control&&reinterpret_cast<bool(__fastcall*)(void*)>(address(0x146ed0030))(control);
}
static void text(void* control,const std::wstring& value){
    // CNUIControlText::SetText(wchar_t*), virtual +2b0. Native code copies
    // the string into its own allocator; no plugin-owned string is retained.
    if(control&&read<uintptr_t>(control)==address(0x14b1c9490))
        reinterpret_cast<void(__fastcall*)(void*,const wchar_t*)>(address(0x146ef7860))(control,value.c_str());
}
static void imageIndex(void* control,int index){
    if(control)reinterpret_cast<void(__fastcall*)(void*,int)>(address(0x146eecbb0))(control,index);
}
static void highlight(uint16_t index){
    selection=index;
    if(!popup||group.empty())return;
    Control root(popup,std::wstring(group.begin(),group.end()).c_str());
    for(uint16_t i=0;i<256;++i){
        Control slot(root.value,(L"slot_"+std::to_wstring(i)).c_str(),true);if(!slot.value)break;
        Control selected(slot.value,L"dodge",true);visible(selected.value,i==selection);
    }
}
static const wchar_t* groups[]={L"red_symbol",L"yellow_symbol",L"green_symbol",L"blue_symbol",L"multi_symbol",L"platinum",L"buff_platinum"};
static std::wstring wideGroup(){return std::wstring(group.begin(),group.end());}
static bool knownGroup(const std::string& s){
    std::wstring w(s.begin(),s.end());
    for(auto n:groups)if(w==n)return true;
    return false;
}
static void selectGroup(){
    if(!popup)return;
    auto selected=wideGroup();
    for(auto name:groups){Control c(popup,name);visible(c.value,selected==name);}
}
static void notifyClosed(){
    group.clear();hoverSocket=-1;
    if(callbacks.closed)callbacks.closed();
}
static void* __fastcall destroy(void* self,unsigned flags){
    if(popup==self){popup=nullptr;manager=nullptr;notifyClosed();}
    if(infoPopup==self){infoPopup=nullptr;infoManager=nullptr;}
    // Native destructor installs its own table before freeing any children.
    return reinterpret_cast<Destroy>(address(0x145f6dca0))(self,flags);
}
static void __fastcall onClose(void* self){
    // Original generic popup onClose is the empty native method.
    if(self==popup)notifyClosed();
    reinterpret_cast<Tick>(address(native_symbol_window_binding::methods[0x30/8]))(self);
}
static bool __fastcall onOpen(void* self,void* arg,intptr_t data){
    const bool ok=reinterpret_cast<bool(__fastcall*)(void*,void*,intptr_t)>(address(0x145f6f4b0))(self,arg,data);
    if(ok&&self==popup&&!requestedGroup.empty()){
        group=requestedGroup;requestedGroup.clear();selectGroup();highlight(0);rightWasDown=(GetAsyncKeyState(VK_RBUTTON)&0x8000)!=0;
        if(callbacks.opened)callbacks.opened(group);
    }
    if(ok&&self==infoPopup){infoRequested=false;++infoGeneration;}
    return ok;
}
static void __fastcall tick(void* self){
    reinterpret_cast<Tick>(address(0x145f70b70))(self);
    if(self==infoPopup&&GetCurrentThreadId()==uiThread){
        if(callbacks.updated)callbacks.updated();
        Control button(self,L"close_button");
        if(clicked(button.value)&&infoManager)
            reinterpret_cast<void(__fastcall*)(void*,void*,bool)>(address(0x146671a70))(infoManager,self,false);
        return;
    }
    if(self!=popup||group.empty()||GetCurrentThreadId()!=uiThread)return;
    hoverSocket=-1;
    DWORD foregroundPID=0;GetWindowThreadProcessId(GetForegroundWindow(),&foregroundPID);
    const bool rightDown=foregroundPID==GetCurrentProcessId()&&(GetAsyncKeyState(VK_RBUTTON)&0x8000)!=0;
    const bool rightPressed=rightDown&&!rightWasDown;rightWasDown=rightDown;
    // Resolve source slots by presence; no duplicated gameplay slot counts.
    Control root(self,wideGroup().c_str());
    for(uint16_t i=0;i<256;++i){
        auto name=L"slot_"+std::to_wstring(i);
        Control slot(root.value,name.c_str(),true);
        if(!slot.value)break;
        Control icon(slot.value,L"emblemcontrol",true);
        Control border(slot.value,L"slot_image",true);
        Control glow(slot.value,L"dodge",true);
        Control animation(slot.value,L"top_ani",true);
        visible(animation.value,i==flashSocket&&GetTickCount64()<flashUntil);
        const bool over=hovered(slot.value)||hovered(icon.value)||hovered(border.value)||hovered(glow.value);
        if(over)hoverSocket=i;
        if(rightPressed&&over&&callbacks.removed){
            highlight(i);callbacks.removed(group,i);break;
        }
        if((clicked(slot.value)||clicked(icon.value)||clicked(border.value)||clicked(glow.value))&&callbacks.selected){
            highlight(i);callbacks.selected(group,i);break;
        }
    }
    if(callbacks.updated)callbacks.updated();
}
static void flash(uint16_t index){flashSocket=index;flashUntil=GetTickCount64()+900;}
static void* popupRoot(){return popup&&!group.empty()?read<void*>(popup,0xd0):nullptr;}
static int dropTarget(){
    if(!popup||group.empty())return -1;
    Control root(popup,wideGroup().c_str());
    for(uint16_t i=0;i<256;++i){
        Control slot(root.value,(L"slot_"+std::to_wstring(i)).c_str(),true);if(!slot.value)break;
        // Native CNUIControl::CheckMousePoint (146ed27d0 -> 146eca5a0).
        // During a drag the hover identity remains owned by the bag cell.
        // This predicate reads the actual cursor and the control's scaled
        // native rectangle, independent of that capture identity.
        // Source slot containers all start at (0,0); their children carry
        // the distinct socket positions. Hit the image, not the container.
        for(auto name:{L"slot_image",L"emblemcontrol"}){
            Control target(slot.value,name,true);
            if(target.value&&reinterpret_cast<bool(__fastcall*)(void*)>(address(0x146eca5a0))(target.value))return i;
        }
    }
    return -1;
}
static void* __fastcall factory(int id,void* context){
    auto original=reinterpret_cast<Factory>(factoryHook.original());
    if(id!=4111&&id!=4159)return original(id,context);
    if(!context||(id==4111?requestedGroup.empty():!infoRequested)||GetCurrentThreadId()!=uiThread)return nullptr;
    auto memory=reinterpret_cast<void*(__fastcall*)(size_t)>(address(0x146e9f2a0))(0x620);
    if(!memory)return nullptr;
    // Native std::wstring is consumed by the constructor. Its allocation must
    // use the game's CRT, so build it with the game's string constructor.
    struct NativeString {wchar_t data[8];size_t length=0,capacity=7;};
    static_assert(sizeof(NativeString)==32,"native wstring ABI");
    NativeString path{};
        const wchar_t* name=id==4111?L"local/20261008/symbol_r30/ui/equipemblemwindow.xui":L"local/20261008/symbol_r30/ui/symbolemblemdetailinfowindow.xui";
    reinterpret_cast<void*(__fastcall*)(void*,const wchar_t*,size_t)>(address(0x14014c8d0))(&path,name,wcslen(name));
    reinterpret_cast<void*(__fastcall*)(void*,void*,int,void*,int)>(address(0x145f6da20))(memory,context,id,&path,1);
    if(read<uintptr_t>(memory)!=address(native_symbol_window_binding::tableVA)){
        if(log){fputs("symbol popup constructor table mismatch\n",log);fflush(log);}
        reinterpret_cast<Destroy>(address(0x145f6dca0))(memory,1);return nullptr;
    }
    // Only this object receives the private table. Other generic popups keep
    // their original methods and native behavior.
    *static_cast<uintptr_t**>(memory)=table.data()+1;
    if(id==4111){popup=memory;manager=context;}else{infoPopup=memory;infoManager=context;}
    return memory;
}
static bool render(const std::string& name,const std::vector<uint32_t>& items){
    if(!popup||name!=group||GetCurrentThreadId()!=uiThread)return false;
    Control root(popup,wideGroup().c_str());
    if(!root.value)return false;
    // Require exact source/control agreement before updating any item.
    for(size_t i=0;i<=items.size();++i){
        Control slot(root.value,(L"slot_"+std::to_wstring(i)).c_str(),true);
        if((i<items.size())!=bool(slot.value))return false;
    }
    for(size_t i=0;i<items.size();++i){
        Control slot(root.value,(L"slot_"+std::to_wstring(i)).c_str(),true);
        Control icon(slot.value,L"emblemcontrol",true);
        if(!widgets.setTemplate(icon.value,items[i]))return false;
        Control idle(slot.value,L"idle_ani",true);visible(idle.value,items[i]!=0);
    }
    highlight(selection);
    return true;
}
static bool open(const std::string& name){
    if(!factoryHook.installed||!knownGroup(name))return false;
    if(uiThread&&uiThread!=GetCurrentThreadId())return false;
    uiThread=GetCurrentThreadId();requestedGroup=name;
    auto context=read<void*>(reinterpret_cast<void*>(address(0x14e683c78)));
    if(!context){requestedGroup.clear();return false;}
    // Native manager may defer opening until its safe UI phase. The factory
    // uses the pending group when that request reaches the main UI thread.
    reinterpret_cast<void*(__fastcall*)(void*,int,uintptr_t,uintptr_t)>(address(0x14668c520))(context,4111,0,0);
    return true;
}
static bool openInfo(){
    if(!factoryHook.installed||(uiThread&&uiThread!=GetCurrentThreadId()))return false;
    uiThread=GetCurrentThreadId();
    auto context=read<void*>(reinterpret_cast<void*>(address(0x14e683c78)));
    if(!context)return false;
    infoRequested=true;
    reinterpret_cast<void*(__fastcall*)(void*,int,uintptr_t,uintptr_t)>(address(0x14668c520))(context,4159,0,0);
    return true;
}
static void close(){
    if(closing||GetCurrentThreadId()!=uiThread)return;
    closing=true;
    requestedGroup.clear();
    infoRequested=false;
    if(infoPopup&&infoManager)reinterpret_cast<void(__fastcall*)(void*,void*,bool)>(address(0x146671a70))(infoManager,infoPopup,false);
    if(popup&&manager)reinterpret_cast<void(__fastcall*)(void*,void*,bool)>(address(0x146671a70))(manager,popup,false);
    if(!group.empty())notifyClosed();
    closing=false;
}
static bool install(uintptr_t imageBase,FILE* sink,Callbacks handlers){
    if(factoryHook.installed)return false;
    base=imageBase;log=sink;
    if(!widgets.bind(base))return false;
    for(auto& g:native_symbol_window_binding::guards){
        auto p=reinterpret_cast<void*>(address(g.va));
        if(!Readable(p,g.size)||memcmp(p,g.bytes,g.size))return false;
    }
    auto vt=reinterpret_cast<uintptr_t*>(address(native_symbol_window_binding::tableVA));
    if(!Readable(vt-1,sizeof table))return false;
    for(size_t i=0;i<120;++i)if(vt[i]!=address(native_symbol_window_binding::methods[i]))return false;
    memcpy(table.data(),vt-1,sizeof table);
    table[1]=reinterpret_cast<uintptr_t>(destroy);
    table[1+0x18/8]=reinterpret_cast<uintptr_t>(tick);
    table[1+0x28/8]=reinterpret_cast<uintptr_t>(onOpen);
    table[1+0x30/8]=reinterpret_cast<uintptr_t>(onClose);
    auto& prologue=native_symbol_window_binding::guards[0];
    if(!PrepareHook(factoryHook,reinterpret_cast<void*>(address(prologue.va)),prologue.bytes,prologue.size,reinterpret_cast<void*>(factory)))return false;
    callbacks=handlers;
    return PublishHook(factoryHook);
}
}
