#pragma once
#include "../../elite-companion-pilot/src/inline_hook.h"
#include "native_inventory_binding.h"
#include "native_emblem_description_binding.h"

// Current 115US only. Native CNTabGroup owns the fifth page's visual tree.
// The legacy inventory controller must NEVER receive index 4: its +5e8
// array has four entries and +608 belongs to a different controller.
namespace native_inventory {
using WindowFn = void(__fastcall*)(void*);
using InputFn = bool(__fastcall*)(void*);
using SelectFn = void(__fastcall*)(void*, int, int);
using DestroyFn = void*(__fastcall*)(void*,unsigned int);
using LookupFn = void*(__fastcall*)(void*, void*, const wchar_t*);
static uintptr_t base;
static FILE* log;
static InlineHook selectHook;
static InlineHook pathHook;
static void* active;
static WindowFn originalUpdate;
static InputFn originalInputs[4];
static DestroyFn originalDestroy;
struct SharedControl {void* value=nullptr;void* owner=nullptr;};
static SharedControl savedAvatarGrid;
static void* borrowedAvatar=nullptr;
static int savedAvatarTab=-1;
struct Callbacks {
    void (*entered)()=nullptr;
    void (*left)()=nullptr;
    void (*updated)()=nullptr;
    void (*star)(const std::string&)=nullptr;
    void (*bagItem)(uint16_t)=nullptr;
    void (*header)(void*)=nullptr;
    bool (*incompatible)(uint16_t)=nullptr;
    int (*draggedSlot)()=nullptr;
    void* (*popupRoot)()=nullptr;
};
static Callbacks callbacks;
static InlineHook bagHook;
static InlineHook renderHook;
static InlineHook tooltipHook;
static bool leftWasDown=false;
using DrawFn=void(__fastcall*)(void*,int,SharedControl*);
static uintptr_t rendererTable[4]{}; // RTTI followed by its three methods
static uintptr_t* savedRendererTable=nullptr;
static bool bagPressPending=false;
static int hoveredSlot=-1;
static ULONGLONG hoveredAt=0;
static POINT hoveredPoint{};
static POINT pressPoint{};
static int draggedRow=-1;
static ULONGLONG draggedRowAt=0;
static void bagPress(void* avatar);
static void pollBagPress(){
    DWORD pid=0;GetWindowThreadProcessId(GetForegroundWindow(),&pid);
    const bool down=pid==GetCurrentProcessId()&&(GetAsyncKeyState(VK_LBUTTON)&0x8000)!=0;
    if(down&&!leftWasDown){bagPressPending=true;GetCursorPos(&pressPoint);bagPress(borrowedAvatar);}
    if(!down)bagPressPending=false;
    leftWasDown=down;
}

template<class T> T read(void* object, size_t offset) {
    T result{}; SIZE_T got=0;
    ReadProcessMemory(GetCurrentProcess(), static_cast<unsigned char*>(object)+offset,
                      &result, sizeof(result), &got);
    return got==sizeof(result)?result:T{};
}
static uintptr_t address(uintptr_t va) { return base + va - 0x140000000; }
static void __fastcall drawCell(void* renderer,int row,SharedControl* cell){
    reinterpret_cast<DrawFn>(savedRendererTable[1])(renderer,row,cell);
    if(!borrowedAvatar||renderer!=static_cast<char*>(borrowedAvatar)+0x80||!cell||!cell->value)return;
    // A focused child popup can suspend the parent tick while still drawing
    // its bag. Sample here as well; the shared edge state prevents duplicates.
    pollBagPress();
    const int slot=reinterpret_cast<int(__fastcall*)(void*,int)>(address(0x1458bf660))(renderer,row);
    if(slot<0||slot>UINT16_MAX)return;
    if(callbacks.draggedSlot&&callbacks.draggedSlot()==slot){draggedRow=row;draggedRowAt=GetTickCount64();}
    if(reinterpret_cast<InputFn>(address(0x146ed0030))(cell->value)){
        hoveredSlot=slot;hoveredAt=GetTickCount64();
        GetCursorPos(&hoveredPoint);
        // Capture while the native cell is known, including the first frame
        // of a drag. Update alone can run before the grid changes selection.
        DWORD pid=0;GetWindowThreadProcessId(GetForegroundWindow(),&pid);
        const bool atOrigin=abs(hoveredPoint.x-pressPoint.x)<GetSystemMetrics(SM_CXDRAG)&&abs(hoveredPoint.y-pressPoint.y)<GetSystemMetrics(SM_CYDRAG);
        if(bagPressPending&&atOrigin&&pid==GetCurrentProcessId()&&(GetAsyncKeyState(VK_LBUTTON)&0x8000)&&callbacks.bagItem){
            bagPressPending=false;callbacks.bagItem(uint16_t(slot));
        }
    }
    if(!callbacks.incompatible||!callbacks.incompatible(uint16_t(slot)))return;
    auto item=reinterpret_cast<void*(__fastcall*)(void*,int)>(address(0x1458c2130))(renderer,row);
    if(!item)return;
    // Exact native draw options from 1458c5c00. Byte 12 selects the disabled
    // item rendering; no item flags or inventory state are modified.
    const unsigned char options[17]={0,1,1,1,1,0,0,0,1,1,1,1,1,1,0,1,0};
    const float x=reinterpret_cast<float(__fastcall*)(void*)>(address(0x146eca0c0))(cell->value)+1.f;
    const float y=reinterpret_cast<float(__fastcall*)(void*)>(address(0x142757420))(cell->value)+1.f;
    auto table=read<uintptr_t*>(item,0);
    reinterpret_cast<void(__fastcall*)(void*,float,float,const void*)>(table[7])(item,x,y,options);
}
static void bagPress(void* avatar){
    if(!avatar||!callbacks.bagItem)return;
    // The native drag owner may replace hover identity on the press frame.
    // Use the last rendered cell only while the pointer is still there.
    POINT at{};GetCursorPos(&at);
    const int slot=GetTickCount64()-hoveredAt<150&&abs(at.x-hoveredPoint.x)<GetSystemMetrics(SM_CXDRAG)&&abs(at.y-hoveredPoint.y)<GetSystemMetrics(SM_CYDRAG)?hoveredSlot:-1;
    if(slot>=0&&slot<=UINT16_MAX){bagPressPending=false;callbacks.bagItem(uint16_t(slot));}
}
static bool resourcesMatch() {
    wchar_t path[32768]{};
    if(!GetModuleFileNameW(nullptr,path,32768))return false;
    auto slash=wcsrchr(path,L'\\');if(!slash)return false;
    wcscpy_s(slash+1,32768-size_t(slash+1-path),L"Script.pvf");
    HANDLE f=CreateFileW(path,GENERIC_READ,FILE_SHARE_READ,nullptr,OPEN_EXISTING,FILE_FLAG_SEQUENTIAL_SCAN,nullptr);
    if(f==INVALID_HANDLE_VALUE)return false;
    BCRYPT_ALG_HANDLE a=nullptr;BCRYPT_HASH_HANDLE h=nullptr;
    unsigned char digest[32]{};bool ok=false;
    if(BCryptOpenAlgorithmProvider(&a,BCRYPT_SHA256_ALGORITHM,nullptr,0)>=0 &&
       BCryptCreateHash(a,&h,nullptr,0,nullptr,0,0)>=0) {
        unsigned char block[65536];DWORD n=0;bool good=true;
        for(;;) {
            if(!ReadFile(f,block,sizeof block,&n,nullptr)){good=false;break;}
            if(!n)break;
            if(BCryptHashData(h,block,n,0)<0){good=false;break;}
        }
        ok=good && BCryptFinishHash(h,digest,32,0)>=0 &&
           memcmp(digest,native_inventory_binding::pvfSHA256,32)==0;
    }
    if(h)BCryptDestroyHash(h);if(a)BCryptCloseAlgorithmProvider(a,0);CloseHandle(f);return ok;
}
static const wchar_t* __fastcall inventoryPath(void*) {
    return L"Local/20261008/Emblem/ItemInventoryWindow.xui";
}
static void release(void* owner) {
    if(!owner) return;
    auto strong=reinterpret_cast<volatile LONG*>(static_cast<unsigned char*>(owner)+8);
    auto weak=reinterpret_cast<volatile LONG*>(static_cast<unsigned char*>(owner)+12);
    if(InterlockedDecrement(strong)==0) {
        auto table=read<uintptr_t*>(owner,0);
        reinterpret_cast<void(__fastcall*)(void*)>(table[0])(owner);
        if(InterlockedDecrement(weak)==0)
            reinterpret_cast<void(__fastcall*)(void*)>(table[1])(owner);
    }
}
struct Control {
    void* value=nullptr;
    void* owner=nullptr;
    Control(void* root,const wchar_t* id,bool widget=false) {
        if(root) reinterpret_cast<LookupFn>(address(widget?0x146ec8e30:0x145f6e4e0))(root,this,id);
    }
    ~Control(){release(owner);}
    Control(const Control&)=delete;
};
static void visible(void* control,bool show) {
    if(!control)return;
    auto table=read<uintptr_t*>(control,0);
    reinterpret_cast<void(__fastcall*)(void*,bool)>(table[0x10/8])(control,show);
}
static void guide(void* window,bool allowHover) {
    // Same hover predicate -> SetVisible chain as native 14311a8a8..8e8.
    // Scope lookup to tab 4: the other inventory pages also have guide IDs.
    struct Shared {void* value=nullptr;void* owner=nullptr;} tab;
    auto group=read<void*>(window,0x618);
    reinterpret_cast<void*(__fastcall*)(void*,void*,int)>(address(0x146f50610))(group,&tab,4);
    {
        Control button(tab.value,L"guide_button",true);
        Control tooltip(tab.value,L"guide_tooltip",true);
        const bool over=allowHover && button.value &&
            reinterpret_cast<InputFn>(address(0x146ed0010))(button.value);
        visible(tooltip.value,over);
    }
    release(tab.owner);
}
static void title(void* window) {
    // Native title setter consumes an MSVC wide string by value. Keep this
    // label in its inline buffer; never give the engine a plugin-heap pointer.
    struct NativeString {wchar_t data[8];size_t size;size_t capacity;};
    static_assert(sizeof(NativeString)==32,"native wide string ABI");
    NativeString text{{0x5fbd,0x7ae0,0},2,7}; // CN tab caption: 徽章
    const auto color=read<unsigned int>(reinterpret_cast<void*>(address(0x14f1c092c)),0);
    reinterpret_cast<void(__fastcall*)(void*,void*,unsigned int)>(address(0x1467aa630))(window,&text,color);
}
static void unbindGrid() {
    if(!borrowedAvatar)return;
    if(savedRendererTable){*reinterpret_cast<uintptr_t**>(static_cast<char*>(borrowedAvatar)+0x80)=savedRendererTable;savedRendererTable=nullptr;}
    hoveredSlot=-1;hoveredAt=0;leftWasDown=false;
    // Transfer the original strong reference back before its controller dies.
    auto slot=reinterpret_cast<SharedControl*>(static_cast<unsigned char*>(borrowedAvatar)+0x40);
    auto current=*slot;
    if(current.value)
        reinterpret_cast<void(__fastcall*)(void*,void*)>(address(0x146f41790))(current.value,nullptr);
    *slot=savedAvatarGrid;savedAvatarGrid={};
    auto avatar=borrowedAvatar;borrowedAvatar=nullptr;
    if(savedAvatarTab>=0 && savedAvatarTab<2)
        reinterpret_cast<void(__fastcall*)(void*,int)>(address(0x1458bedd0))(avatar,savedAvatarTab);
    savedAvatarTab=-1;
    release(current.owner);
}
static bool bindGrid(void* window,int previousTab) {
    auto avatar=read<void*>(window,0x5f8);
    if(!avatar || read<uintptr_t>(avatar,0)!=address(0x14a8e0770) ||
       read<uintptr_t>(reinterpret_cast<void*>(address(0x14a8e0770)),0x78)!=address(0x1458a0800) ||
       read<int>(avatar,0x210)!=1 || read<int>(avatar,0xd0)!=0x121 ||
       read<uintptr_t>(avatar,0x80)!=address(0x14a8e3860))return false;
    SharedControl tab;
    auto group=read<void*>(window,0x618);
    reinterpret_cast<void*(__fastcall*)(void*,void*,int)>(address(0x146f50610))(group,&tab,4);
    Control grid(tab.value,L"pocketControl",true);
    release(tab.owner);
    if(!grid.value || !grid.owner)return false;
    auto slot=reinterpret_cast<SharedControl*>(static_cast<unsigned char*>(avatar)+0x40);
    if(!slot->value || !slot->owner)return false;
    savedAvatarGrid=*slot;
    *slot={grid.value,grid.owner};grid.value=nullptr;grid.owner=nullptr;
    borrowedAvatar=avatar;savedAvatarTab=previousTab;
    savedRendererTable=read<uintptr_t*>(avatar,0x80);
    memcpy(rendererTable,savedRendererTable-1,sizeof rendererTable);
    rendererTable[2]=reinterpret_cast<uintptr_t>(drawCell);
    *reinterpret_cast<uintptr_t**>(static_cast<char*>(avatar)+0x80)=rendererTable+1;
    // Native avatar initializer 1458a194a attaches exactly this renderer.
    // Its source is the live emblem bag (subpage 1 / native item type 0x121).
    reinterpret_cast<void(__fastcall*)(void*,void*)>(address(0x146f41790))
        (slot->value,static_cast<unsigned char*>(avatar)+0x80);
    return true;
}
static void* __fastcall destroy(void* window,unsigned int flags) {
    if(active==window){if(callbacks.left)callbacks.left();unbindGrid();active=nullptr;}
    return originalDestroy(window,flags);
}
static int selected(void* window) {
    auto group=read<void*>(window,0x618);
    if(!group) return -1;
    return reinterpret_cast<int(__fastcall*)(void*)>(address(0x146f50350))(group);
}
static bool ownsPage(void* window) {
    if(read<uintptr_t>(window,0)!=address(0x14a8e56d8)) return false;
    auto group=read<void*>(window,0x618);
    if(!group || reinterpret_cast<int(__fastcall*)(void*)>(address(0x146f50740))(group)!=5) return false;
    Control marker(window,L"local_emblem_route_r15");
    return marker.value!=nullptr;
}
static bool emblem(void* window) {
    return window==active && selected(window)==4;
}
static void drawDraggedItem(){
    if(!active||!borrowedAvatar||!emblem(active)||!callbacks.draggedSlot)return;
    const int slot=callbacks.draggedSlot();
    if(slot<0||draggedRow<0||GetTickCount64()-draggedRowAt>100)return;
    auto renderer=static_cast<char*>(borrowedAvatar)+0x80;
    if(reinterpret_cast<int(__fastcall*)(void*,int)>(address(0x1458bf660))(renderer,draggedRow)!=slot)return;
    // Resolve afresh on the UI thread; never retain an inventory item across
    // exchanges, page changes or native controller destruction.
    auto item=reinterpret_cast<void*(__fastcall*)(void*,int)>(address(0x1458c2130))(renderer,draggedRow);
    auto grid=read<void*>(borrowedAvatar,0x40);
    auto layout=read<void*>(reinterpret_cast<void*>(address(0x14e6343e0)),0);
    if(!item||!grid||!layout)return;
    // Same native HWND cursor and coordinate conversion as CheckMousePoint.
    // This handles the UI anchor and client scaling without screen constants.
    const uint64_t packed=reinterpret_cast<uint64_t(__fastcall*)(void*)>(address(0x146ec45d0))(read<void*>(grid,0x190));
    const int x=int(uint32_t(packed)),y=int(uint32_t(packed>>32));
    if(x<0||y<0)return;
    const int anchor=reinterpret_cast<int(__fastcall*)(void*)>(address(0x140738710))(layout);
    // Inventory icons occupy 28 logical UI pixels. Offset in the converted
    // UI coordinate space so the hotspot stays centered at every UI scale.
    const float fx=reinterpret_cast<float(__fastcall*)(void*,int,float)>(address(0x146ec6b00))(layout,anchor,float(x))-14.f;
    const float fy=reinterpret_cast<float(__fastcall*)(void*,int,float)>(address(0x146ec6c90))(layout,anchor,float(y))-14.f;
    const unsigned char options[17]={0,1,1,1,1,0,0,0,1,1,1,1,0,1,0,1,0};
    auto table=read<uintptr_t*>(item,0);
    if(!table||!Readable(table,8*8))return;
    reinterpret_cast<void(__fastcall*)(void*,float,float,const void*)>(table[7])(item,fx,fy,options);
    static unsigned reported=0;
    if(log&&reported<8){++reported;fprintf(log,"symbol ghost render slot=%d row=%d cursor=%.1f,%.1f item=%p\n",slot,draggedRow,fx,fy,item);fflush(log);}
}
// IControl::Render (+40): 146ed1fb0 invokes +48, draws children through
// 146ecb590, then restores the saved clipping rectangle via 146ebafd0.
// +20 on a window is instead the input dispatcher and must stay native.
static uint64_t __fastcall renderControl(void* control){
    const auto result=reinterpret_cast<uint64_t(__fastcall*)(void*)>(renderHook.original())(control);
    if(active&&(control==read<void*>(active,0xd0)||(callbacks.popupRoot&&control==callbacks.popupRoot()))&&
       reinterpret_cast<InputFn>(address(0x141fb6530))(control))
        drawDraggedItem();
    return result;
}
// Stackable tooltips take 1468963c0, not the equipment formatter chain
// 1430cc4d0 / 146881d80. Extend its rows before 1468953b0 performs layout.
static std::wstring compact(const std::wstring& value){
    std::wstring out;for(auto c:value)if(c!=L' '&&c!=L'\t'&&c!=L'\r'&&c!=L'\n')out+=c;return out;
}
static bool containsLine(void* tooltip,const wchar_t* value){
    auto begin=read<uintptr_t>(tooltip,0x660),end=read<uintptr_t>(tooltip,0x668);
    if(!begin||end<begin||end-begin>248*300||(end-begin)%248)return true; // fail closed
    const auto wanted=compact(value);
    for(auto at=begin;at<end;at+=248){
        auto string=reinterpret_cast<void*>(at+0x18);
        auto length=read<size_t>(string,16),capacity=read<size_t>(string,24);
        if(!length||length>4096||capacity<length)continue;
        auto data=capacity>7?read<const wchar_t*>(string,0):reinterpret_cast<const wchar_t*>(string);
        if(data&&Readable(data,length*2)&&compact(std::wstring(data,length)).find(wanted)!=std::wstring::npos)return true;
    }
    return false;
}
// The native emblem template reserves a property row immediately before the
// <inlay emblem> paragraph. Locate that unused row, without hardcoded row IDs.
static int propertyPosition(void* tooltip,size_t& index){
    auto begin=read<uintptr_t>(tooltip,0x660),end=read<uintptr_t>(tooltip,0x668);
    if(!begin||end<begin||end-begin>248*300||(end-begin)%248)return -1;
    for(auto at=begin;at<end;at+=248){
        auto s=reinterpret_cast<void*>(at+0x18);auto n=read<size_t>(s,16),cap=read<size_t>(s,24);
        auto p=cap>7?read<const wchar_t*>(s,0):reinterpret_cast<const wchar_t*>(s);
        if(!n||n>4096||cap<n||!p||!Readable(p,n*2))continue;
        if(std::wstring(p,n).find(L"<\u9576\u5d4c\u5fbd\u7ae0>")==std::wstring::npos)continue;
        const int row=read<int>(reinterpret_cast<void*>(at),0)-1;
        if(row<0)return -1;
        for(auto other=begin;other<end;other+=248)
            if(read<int>(reinterpret_cast<void*>(other),0)==row)return -1;
        index=(at-begin)/248;return row;
    }
    return -1;
}
static unsigned propertyColor(void* tooltip){
    // Match the active native text palette, including comparison/tint mode.
    // The current emblem category line uses the same gold as its option text.
    auto begin=read<uintptr_t>(tooltip,0x660),end=read<uintptr_t>(tooltip,0x668);
    if(begin&&end>=begin&&end-begin<=248*300&&(end-begin)%248==0){
        for(auto at=begin;at<end;at+=248){
            auto s=reinterpret_cast<void*>(at+0x18);auto n=read<size_t>(s,16),cap=read<size_t>(s,24);
            auto p=cap>7?read<const wchar_t*>(s,0):reinterpret_cast<const wchar_t*>(s);
            if(n==2&&cap>=n&&p&&Readable(p,n*2)&&std::wstring(p,n)==L"\u5fbd\u7ae0")
                return read<unsigned>(reinterpret_cast<void*>(at),0x10);
        }
    }
    return read<unsigned>(reinterpret_cast<void*>(address(0x14f1c0880)),0);
}
static void moveLastProperty(void* tooltip,size_t index){
    auto begin=read<unsigned char*>(tooltip,0x660),end=read<unsigned char*>(tooltip,0x668);
    if(!begin||end<=begin||size_t(end-begin)%248||index>=size_t(end-begin)/248)return;
    auto target=begin+index*248,last=end-248;
    if(target==last)return;
    // Rows contain native strings, vectors and shared references. Use native
    // copy construction/assignment/destruction; never byte-move their owners.
    alignas(8) unsigned char temporary[248]{};
    reinterpret_cast<void(__fastcall*)(void*,const void*)>(address(0x1405729c0))(temporary,last);
    auto assign=reinterpret_cast<void*(__fastcall*)(const void*,const void*,void*)>(address(0x140b6c5e0));
    for(auto dest=last;dest>target;dest-=248)assign(dest-248,dest,dest);
    assign(temporary,temporary+248,target);
    reinterpret_cast<void(__fastcall*)(void*,void*)>(address(0x140585720))(temporary,temporary+248);
}
static uint64_t __fastcall buildStackableTooltip(void* tooltip,int* row){
    const auto result=reinterpret_cast<uint64_t(__fastcall*)(void*,int*)>(tooltipHook.original())(tooltip,row);
    if(active&&borrowedAvatar&&emblem(active)&&row&&*row>=0&&*row<300&&
       read<uintptr_t>(tooltip,0)==address(0x14b0e92f8)){
        auto item=read<void*>(reinterpret_cast<void*>(address(0x14dc68c40)),0);
        if(item&&item==read<void*>(borrowedAvatar,0x238)&&read<uintptr_t>(item,0)==address(0x14a927bb0)){
            // Live CStackableItem +98 -> common data at +58; source item ID
            // is read by the native getter, not interpreted as a bag position.
            const auto id=reinterpret_cast<unsigned(__fastcall*)(void*)>(address(0x140256dc0))(static_cast<char*>(item)+0x68);
            const auto line=native_emblem_description_binding::find(id);
            if(line&&!containsLine(tooltip,line)){
                size_t position=0;const int propertyRow=propertyPosition(tooltip,position);
                const auto color=propertyColor(tooltip);
                const auto oldEnd=read<uintptr_t>(tooltip,0x668);
                reinterpret_cast<void(__fastcall*)(void*,int,const wchar_t*,void*,unsigned,bool,unsigned)>(address(0x14687f8a0))
                    (tooltip,propertyRow>=0?propertyRow:*row,line,reinterpret_cast<void*>(address(0x14ef55690)),color,false,0);
                if(read<uintptr_t>(tooltip,0x668)==oldEnd)return result;
                if(propertyRow>=0)moveLastProperty(tooltip,position);
                else ++*row;
                // Same pagination expression at the end of 1468963c0.
                *reinterpret_cast<int*>(static_cast<char*>(tooltip)+0x838)=(*row-1)/0x26;
                static unsigned reported=0;
                if(log&&reported<8){++reported;fprintf(log,"symbol intelligence property source item=%u row=%d placed=%d color=%08x\n",id,propertyRow>=0?propertyRow:*row-1,propertyRow>=0,color);fflush(log);}
            }
        }
    }
    return result;
}
static void hideLegacyEmblemButton(void* window) {
    if(!ownsPage(window))return;
    auto avatar=read<void*>(window,0x5f8);
    if(!avatar || read<uintptr_t>(avatar,0)!=address(0x14a8e0770))return;
    auto group=read<void*>(avatar,0x1d0);
    if(!group || reinterpret_cast<int(__fastcall*)(void*)>(address(0x146f50740))(group)!=2)return;
    SharedControl tab;
    reinterpret_cast<void*(__fastcall*)(void*,void*,int)>(address(0x146f50610))(group,&tab,1);
    // CNTab constructor 146f8bd30 retains a separate CNTabButton at +358.
    // XUI reader 146f8b200 applies ButtonSize/Pos to that child, not the page.
    // Keep both logical subpages: index 1 still owns the real emblem bag.
    if(tab.value && read<uintptr_t>(tab.value,0)==address(0x14b1dd5a0)) {
        auto button=read<void*>(tab.value,0x358);
        if(button && read<uintptr_t>(button,0)==address(0x14b1dd1b8) &&
           read<uintptr_t>(reinterpret_cast<void*>(address(0x14b1dd1b8)),0x10)==address(0x146ed3a90))
            visible(button,false);
    }
    release(tab.owner);
}
static void __fastcall select(void* window,int page,int subpage) {
    auto original=reinterpret_cast<SelectFn>(selectHook.original());
    // Existing native shortcuts may still request avatar / emblem (2,1).
    // They now enter the same fifth tab as the visible top-level button.
    if(page==2 && ownsPage(window)) {
        if(subpage==1)page=4;
        else subpage=0;
    }
    if(page!=4) {
        if(active==window) {if(callbacks.left)callbacks.left();guide(window,false);unbindGrid();active=nullptr;}
        if(page>=0 && page<4) original(window,page,subpage);
        hideLegacyEmblemButton(window);
        return;
    }
    if(!ownsPage(window)) {
        // Even an unpaired resource revision must not index the four-page array.
        original(window,0,-1);
        return;
    }
    if(!emblem(window)) {
        // Run the native transition's cleanup with a valid legacy page.
        auto avatar=read<void*>(window,0x5f8);
        const int previousTab=0; // the remaining visible avatar subpage
        original(window,2,1);
        if(read<int>(window,0x628)!=2) return; // native game-mode gate refused
        if(!bindGrid(window,previousTab)) {
            if(previousTab>=0 && previousTab<2)
                reinterpret_cast<void(__fastcall*)(void*,int)>(address(0x1458bedd0))(avatar,previousTab);
            original(window,0,-1);
            if(log){fputs("native emblem inventory binding rejected\n",log);fflush(log);}
            return;
        }
        auto group=read<void*>(window,0x618);
        reinterpret_cast<void(__fastcall*)(void*,int,bool)>(address(0x146f53250))(group,4,false);
        active=window;
        if(callbacks.entered)callbacks.entered();
        guide(window,false);
        title(window);
        hideLegacyEmblemButton(window);
        if(log){fputs("native emblem page selected; legacy page index remains bounded\n",log);fflush(log);}
    }
}
static void __fastcall update(void* window) {
    // A legacy subpage callback can bypass the main selector. Redirect it
    // while the owned five-page resource is active, before updating that bag.
    if(selected(window)==2 && active!=window && ownsPage(window)) {
        auto avatar=read<void*>(window,0x5f8);
        if(read<uintptr_t>(avatar,0)==address(0x14a8e0770) && read<int>(avatar,0x210)==1)
            select(window,4,-1);
    }
    if(selected(window)==4) {
        if(!emblem(window)) select(window,4,-1);
        if(emblem(window)) {
            pollBagPress();
            // Native inventory controller owns contents, tooltip and scrolling.
            reinterpret_cast<WindowFn>(address(0x14589f7e0))(borrowedAvatar);
            reinterpret_cast<WindowFn>(address(0x1467a72e0))(window);
            guide(window,true);
            hideLegacyEmblemButton(window);
            if(callbacks.updated)callbacks.updated();
            SharedControl tab;
            reinterpret_cast<void*(__fastcall*)(void*,void*,int)>(address(0x146f50610))(read<void*>(window,0x618),&tab,4);
            const wchar_t* names[]={L"red_symbol",L"yellow_symbol",L"green_symbol",L"blue_symbol",L"multi_symbol",L"platinum",L"buff_platinum"};
            for(auto name:names){
                Control c(tab.value,name,true);
                if(c.value&&reinterpret_cast<InputFn>(address(0x146ecfd90))(c.value)&&callbacks.star){
                    std::wstring w(name);callbacks.star(std::string(w.begin(),w.end()));break;
                }
            }
            if(callbacks.header)callbacks.header(tab.value);
            release(tab.owner);
            return;
        }
    }
    if(active==window) {
        const int page=selected(window);
        if(callbacks.left)callbacks.left();
        guide(window,false);unbindGrid();active=nullptr;
        // The native controller stores safe legacy index 2. Returning to the
        // avatar page must still run the native transition and restore title.
        if(page>=0 && page<4)
            reinterpret_cast<SelectFn>(selectHook.original())(window,page,-1);
    }
    originalUpdate(window);
    hideLegacyEmblemButton(window);
}
static bool rootInput(void* window,uintptr_t target) {
    // These native inventory input methods combine a root-widget query with
    // item operations. Only the former applies to the owned emblem page.
    struct Shared {void* value;void* owner;} root{};
    auto table=read<uintptr_t*>(window,0);
    reinterpret_cast<void*(__fastcall*)(void*,void*)>(table[0x110/8])(window,&root);
    bool result=root.value && reinterpret_cast<InputFn>(address(target))(root.value);
    release(root.owner);
    return result;
}
template<int N,uintptr_t Target> static bool __fastcall input(void* window) {
    if(selected(window)==4 && ownsPage(window)) {
        select(window,4,-1);
        if(emblem(window) && borrowedAvatar)return originalInputs[N](window);
        return rootInput(window,Target);
    }
    return originalInputs[N](window);
}
struct Slot {size_t offset;uintptr_t expected;void* replacement;};
static void __fastcall bagInput(void* avatar) {
    if(avatar!=borrowedAvatar||!active||!emblem(active)||!callbacks.bagItem) {
        reinterpret_cast<WindowFn>(bagHook.original())(avatar);return;
    }
    // The original avatar input queries grid mode 3 at 1458c98d6, then maps
    // renderer indices through 1458bf660. Never use the visible row as bag ID.
    auto grid=read<void*>(avatar,0x40);
    const int index=grid?reinterpret_cast<int(__fastcall*)(void*,int)>(address(0x146f41360))(grid,3):-1;
    if(index<0)return;
    const int selectedRow=read<int>(avatar,0x374);
    if(selectedRow<0)return;
    const int slot=reinterpret_cast<int(__fastcall*)(void*,int)>(address(0x1458bf660))(static_cast<char*>(avatar)+0x80,selectedRow);
    if(slot>=0&&slot<=UINT16_MAX&&callbacks.bagItem)callbacks.bagItem(uint16_t(slot));
    // Native input consumes successful emblem actions via this same flag.
    *reinterpret_cast<unsigned char*>(static_cast<char*>(avatar)+0x38d)=1;
}
static bool install(FILE* sink,Callbacks handlers={}) {
    base=reinterpret_cast<uintptr_t>(GetModuleHandleW(nullptr));log=sink;
    if(!resourcesMatch())return false;
    for(const auto& guard:native_inventory_binding::guards) {
        auto site=reinterpret_cast<void*>(address(guard.va));
        if(!Readable(site,sizeof guard.bytes) || memcmp(site,guard.bytes,sizeof guard.bytes))return false;
    }
    const unsigned char prologue[]={0x40,0x57,0x48,0x83,0xec,0x50};
    Slot slots[]={
        {0x0,0x1458d7430,reinterpret_cast<void*>(destroy)},
        {0x18,0x1458ddaa0,reinterpret_cast<void*>(update)},
        {0x58,0x1458dc9e0,reinterpret_cast<void*>(input<0,0x146ecff40>)},
        {0x60,0x1458dcc10,reinterpret_cast<void*>(input<1,0x146ecfe30>)},
        {0x68,0x1458dcac0,reinterpret_cast<void*>(input<2,0x146ecfff0>)},
        {0x80,0x1458dd930,reinterpret_cast<void*>(input<3,0x146ed0170>)}
    };
    auto table=reinterpret_cast<unsigned char*>(address(0x14a8e56d8));
    for(auto& slot:slots)
        if(!Readable(table+slot.offset,8) || *reinterpret_cast<uintptr_t*>(table+slot.offset)!=address(slot.expected)) return false;
    if(!PrepareHook(selectHook,reinterpret_cast<void*>(address(0x1458d9a40)),prologue,sizeof(prologue),reinterpret_cast<void*>(select))) return false;
    if(!PrepareHook(pathHook,reinterpret_cast<void*>(address(0x1458d5e35)),native_inventory_binding::pathCall,5,reinterpret_cast<void*>(inventoryPath)))return false;
    const unsigned char bagPrologue[]={0x40,0x55,0x56,0x57,0x48,0x8b,0xec};
    if(!PrepareHook(bagHook,reinterpret_cast<void*>(address(0x1458c9510)),bagPrologue,sizeof bagPrologue,reinterpret_cast<void*>(bagInput)))return false;
    const unsigned char renderPrologue[]={0x48,0x8b,0xc4,0x56,0x57};
    if(!PrepareHook(renderHook,reinterpret_cast<void*>(address(0x146ed1fb0)),renderPrologue,sizeof renderPrologue,reinterpret_cast<void*>(renderControl)))return false;
    const unsigned char tooltipPrologue[]={0x40,0x55,0x56,0x57,0x41,0x54};
    if(!PrepareHook(tooltipHook,reinterpret_cast<void*>(address(0x1468963c0)),tooltipPrologue,sizeof tooltipPrologue,reinterpret_cast<void*>(buildStackableTooltip)))return false;
    callbacks=handlers;
    originalDestroy=reinterpret_cast<DestroyFn>(address(slots[0].expected));
    originalUpdate=reinterpret_cast<WindowFn>(address(slots[1].expected));
    for(int i=0;i<4;++i) originalInputs[i]=reinterpret_cast<InputFn>(address(slots[i+2].expected));
    // Selector guard precedes publication of every fifth-page consumer.
    if(!PublishHook(selectHook)) return false;
    if(!PublishHook(bagHook)){RestoreHook(selectHook);return false;}
    if(!PublishHook(renderHook)){RestoreHook(bagHook);RestoreHook(selectHook);return false;}
    if(!PublishHook(tooltipHook)){RestoreHook(renderHook);RestoreHook(bagHook);RestoreHook(selectHook);return false;}
    DWORD old=0;
    if(!VirtualProtect(table,0x88,PAGE_READWRITE,&old)) {RestoreHook(tooltipHook);RestoreHook(renderHook);RestoreHook(bagHook);RestoreHook(selectHook);return false;}
    for(auto& slot:slots)
        InterlockedExchangePointer(reinterpret_cast<void*volatile*>(table+slot.offset),slot.replacement);
    DWORD ignored=0;VirtualProtect(table,0x88,old,&ignored);
    // Fail-closed resource pairing: the original XUI still has only four tabs.
    // Expose the private fifth tab only after all its consumers are protected.
    if(!PublishHook(pathHook,true)) {
        if(VirtualProtect(table,0x88,PAGE_READWRITE,&old)) {
            for(auto& slot:slots)InterlockedExchangePointer(reinterpret_cast<void*volatile*>(table+slot.offset),reinterpret_cast<void*>(address(slot.expected)));
            VirtualProtect(table,0x88,old,&ignored);
            RestoreHook(selectHook);
            RestoreHook(bagHook);
            RestoreHook(renderHook);
            RestoreHook(tooltipHook);
        }
        return false;
    }
    if(log){fputs("native inventory route installed; paired XUI marker required\n",log);fflush(log);}
    return true;
}
}
