#include "symbol_page_state.h"
#include "native_symbol_widgets.h"
#include "native_symbol_window.h"
#include "symbol_protocol.h"
#include <cassert>
#include <cstdio>
using namespace symbol_page;
int main() {
    // Same layout as Go TestSymbolViewBoundsAndFrame, including the sparse slot.
    std::vector<uint8_t> bytes;
    features::put(bytes,2,8);features::put(bytes,0,8);bytes.insert(bytes.end(),64,'a');
    features::put(bytes,1,2);features::put(bytes,1,2);features::put(bytes,10,2);
    const std::string red="red_symbol";bytes.insert(bytes.end(),red.begin(),red.end());
    features::put(bytes,1,2);features::put(bytes,2,2);features::put(bytes,590004573,4);features::put(bytes,0,4);
    features::put(bytes,1,4);features::put(bytes,0,8);features::put(bytes,0,4);features::put(bytes,289,2);features::put(bytes,1,2);features::put(bytes,590004579,4);features::put(bytes,9,4);features::put(bytes,0,4);
    assert(bytes.size()==140);
    auto snapshot=symbol_wire::decode(bytes);
    assert(snapshot.role==2&&snapshot.groups[0].items[0].id==590004573&&snapshot.bag[0].slot==289);
    auto withAttributes=bytes;features::put(withAttributes,0x31544153,4);
    for(const std::string s:{std::string(u8"力量 +60"),std::string(u8"无属性加成"),std::string(u8"力量 +60")}){features::put(withAttributes,s.size(),2);withAttributes.insert(withAttributes.end(),s.begin(),s.end());}
    auto detailed=symbol_wire::decode(withAttributes);assert(detailed.attributes==u8"力量 +60"&&detailed.groups[0].attributes==detailed.attributes);
    for(size_t n=bytes.size()+1;n<withAttributes.size();++n){bool rejected=false;try{symbol_wire::decode({withAttributes.begin(),withAttributes.begin()+n});}catch(...){rejected=true;}assert(rejected);}
    auto withStar=withAttributes;features::put(withStar,0x31525353,4);features::put(withStar,1,2);features::put(withStar,1986000001,4);features::put(withStar,1986000002,4);
    features::put(withStar,1,1);features::put(withStar,0,4);features::put(withStar,1,4);features::put(withStar,12,4);
    for(int i=0;i<12;++i){std::string s=u8"增益量 +10";features::put(withStar,s.size(),2);withStar.insert(withStar.end(),s.begin(),s.end());}
    auto stars=symbol_wire::decode(withStar);assert(stars.powers.size()==1&&stars.groups[0].star.present&&stars.groups[0].star.level==1&&stars.groups[0].star.fame==12);
    for(size_t n=withAttributes.size()+1;n<withStar.size();++n){bool rejected=false;try{symbol_wire::decode({withStar.begin(),withStar.begin()+n});}catch(...){rejected=true;}assert(rejected);}
    auto exchange=symbol_wire::exchange(snapshot,red,0,&snapshot.bag[0]);
    assert(exchange.size()==104&&exchange[0]==2&&exchange[80]==10&&exchange[98]==0x21&&exchange[99]==1);
    for(size_t n=0;n<bytes.size();++n){bool rejected=false;try{symbol_wire::decode({bytes.begin(),bytes.begin()+n});}catch(...){rejected=true;}assert(rejected);}
    bytes.push_back(0);bool trailing=false;try{symbol_wire::decode(bytes);}catch(...){trailing=true;}assert(trailing);
    const Identity role{2,10,std::string(64,'a')};
    State page;
    page.enter(role);auto generation=page.generation();
    Ticket first;
    assert(!page.openGroup("red_symbol",4) && !page.begin(first));
    assert(page.snapshot(role,generation,0));
    assert(page.openGroup("red_symbol",4));
    assert(!page.selectSocket(4) && page.selectSocket(3));
    assert(page.begin(first));
    Ticket duplicate;
    assert(!page.begin(duplicate));
    assert(!page.snapshot(role,generation,1));
    assert(!page.accept(first,2));
    assert(page.accept(first,1) && !page.accept(first,1));
    assert(!page.snapshot(role,generation,0));
    assert(page.begin(first));
    assert(page.fail(first) && !page.ready() && !page.begin(duplicate));
    // Lost reply with a server commit: refresh restores the newer sequence.
    assert(page.snapshot(role,generation,2) && page.ready());
    assert(page.begin(first));
    page.closePanel();
    assert(!page.panelOpen() && page.accept(first,3));
    assert(page.ready() && !page.begin(duplicate));
    assert(page.openGroup("multi_symbol",6) && page.selectSocket(5));
    assert(page.begin(first));
    page.close();
    assert(!page.accept(first,4) && !page.snapshot(role,generation,4));
    // Same role after reconnect must not accept the previous connection's view.
    Identity reconnected=role;reconnected.connection++;
    page.enter(reconnected);auto next=page.generation();
    assert(!page.snapshot(role,next,4));
    assert(!page.snapshot(reconnected,generation,4));
    assert(page.snapshot(reconnected,next,3));
    assert(!page.openGroup("red_symbol",0));
    assert(page.openGroup("red_symbol",1) && !page.selectSocket(1));
    assert(!page.accept(first,4));
    // No DFO image is loaded in this mechanism test. Native API stays disabled.
    native_symbol_widgets::API widgets;
    assert(!widgets.bind(0) && !widgets.setTemplate(nullptr,590004573));
    // Rebased synthetic PE pages validate ASLR guards without calling game code.
    auto memory=static_cast<unsigned char*>(VirtualAlloc(nullptr,0xb000000,MEM_RESERVE,PAGE_NOACCESS));
    assert(memory);
    auto base=reinterpret_cast<uintptr_t>(memory);
    auto at=[&](uintptr_t va){return memory+(va-0x140000000ull);};
    auto commit=[&](uintptr_t va){assert(VirtualAlloc(at(va&~uintptr_t(0xfff)),0x1000,MEM_COMMIT,PAGE_READWRITE));};
    for(const auto& guard:native_symbol_widget_binding::guards) {
        commit(guard.va);memcpy(at(guard.va),guard.bytes,guard.size);
    }
    commit(native_symbol_widget_binding::itemVTable);
    uintptr_t methods[4];
    for(unsigned i=0;i<4;++i)methods[i]=base+native_symbol_widget_binding::itemMethods[i]-0x140000000ull;
    memcpy(at(native_symbol_widget_binding::itemVTable),methods,sizeof methods);
    assert(widgets.bind(base));
    auto site=at(native_symbol_widget_binding::guards[0].va);
    *site^=1;assert(!widgets.bind(base));*site^=1;
    assert(!widgets.setTemplate(nullptr,590004573));
    assert(widgets.bind(base));
    methods[0]^=1;memcpy(at(native_symbol_widget_binding::itemVTable),methods,sizeof methods);
    assert(!widgets.bind(base));
    methods[0]^=1;memcpy(at(native_symbol_widget_binding::itemVTable),methods,sizeof methods);
    for(const auto& guard:native_symbol_window_binding::guards){
        commit(guard.va);commit(guard.va+guard.size-1);memcpy(at(guard.va),guard.bytes,guard.size);
    }
    auto windowTable=native_symbol_window_binding::tableVA;
    commit(windowTable-8);
    uintptr_t windowMethods[121]{};
    for(unsigned i=0;i<120;++i)windowMethods[i+1]=base+native_symbol_window_binding::methods[i]-0x140000000ull;
    memcpy(at(windowTable-8),windowMethods,sizeof windowMethods);
    auto branch=at(native_symbol_window_binding::guards[1].va);
    *branch^=1;assert(!native_symbol_window::install(base,nullptr,{}));*branch^=1;
    assert(!native_symbol_window::open("red_symbol"));
    assert(native_symbol_window::install(base,nullptr,{}));
    assert(!native_symbol_window::open("unknown"));
    assert(RestoreHook(native_symbol_window::factoryHook));
    auto& entry=native_symbol_window_binding::guards[0];
    assert(!memcmp(at(entry.va),entry.bytes,entry.size));
    assert(!native_symbol_window::render("red_symbol",{}));
    assert(VirtualFree(native_symbol_window::factoryHook.page,0,MEM_RELEASE));
    assert(VirtualFree(memory,0,MEM_RELEASE));
    puts("symbol page lifecycle and native adapter fail-closed checks passed");
}
