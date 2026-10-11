#pragma once
#include <mutex>
#include <vector>
#include "symbol_protocol.h"
#include "native_symbol_effects_binding.h"
#include "../../elite-companion-pilot/src/inline_hook.h"

// Append native PVF enchant options at the character's existing collection
// boundary. Never edit encrypted attributes or manufacture an equipment row.
namespace native_symbol_effects {
struct Identity {uintptr_t character=0,owner=0;uint64_t epoch=0;};
struct Effect {uint32_t id;bool buff;bool platinum;uint32_t bufferID=0;};
struct NativeVector {void** begin;void** end;void** capacity;};
using Collect=void(__fastcall*)(void*,NativeVector*);
using Skills=void(__fastcall*)(void*);
using Destroy=void(__fastcall*)(void*);
static uintptr_t base;
static InlineHook collectHook,skillsHook,destroyHook;
static std::mutex mutex;
static Identity observed,bound;
static uint64_t epoch=1,role=0,sequence=0;
static std::vector<Effect> effects;
static volatile LONG collectCalls=0,skillCalls=0;
static uintptr_t address(uintptr_t va){return base+va-0x140000000ull;}
template<class T> static T read(uintptr_t p){T v{};SIZE_T n=0;ReadProcessMemory(GetCurrentProcess(),reinterpret_cast<void*>(p),&v,sizeof v,&n);return n==sizeof v?v:T{};}
static bool same(const Identity&a,const Identity&b){return a.character==b.character&&a.owner==b.owner&&a.epoch==b.epoch;}
static Identity identity(){
    std::lock_guard<std::mutex> lock(mutex);
    auto owner=read<uintptr_t>(address(0x14ef2ca88));
    auto ptr=read<uintptr_t>(address(0x14ef2ca90));
    // 145efafb0 has a second return path after its first RET: ordinary town
    // play uses this fallback shared actor, while the first pair is empty.
    if(!owner||!ptr||read<LONG>(owner+8)<=0){
        owner=read<uintptr_t>(address(0x14ef2ca70));
        ptr=read<uintptr_t>(address(0x14ef2ca78));
    }
    Identity now{};
    if(owner&&ptr&&read<LONG>(owner+8)>0)now={ptr-0x30,owner,epoch};
    if(now.character!=observed.character||now.owner!=observed.owner){
        ++epoch;now.epoch=epoch;observed=now;bound={};effects.clear();role=sequence=0;
    }else now.epoch=epoch;
    return now;
}
static bool publish(const Identity& expected,const symbol_wire::View& v){
    if(!expected.character||v.source!=native_symbol_effects_binding::source||!same(expected,identity()))return false;
    std::vector<Effect> next;
    for(auto& g:v.groups)for(auto& i:g.items)if(i.id>1&&(i.flags&1))next.push_back({i.id,g.name=="buff_platinum",g.name=="platinum"});
    for(auto& p:v.powers)next.push_back({p.first,false,false,p.second});
    std::lock_guard<std::mutex> lock(mutex);
    if(expected.epoch!=epoch)return false;
    if(same(bound,expected)&&role==v.role&&v.sequence<sequence)return false;
    bool changed=!same(bound,expected)||role!=v.role||next.size()!=effects.size();
    if(!changed)for(size_t i=0;i<next.size();++i)if(next[i].id!=effects[i].id||next[i].buff!=effects[i].buff||next[i].platinum!=effects[i].platinum||next[i].bufferID!=effects[i].bufferID){changed=true;break;}
    bound=expected;role=v.role;sequence=v.sequence;effects=std::move(next);return changed;
}
static std::vector<Effect> forCharacter(void* self){
    auto current=identity();
    if(current.character!=reinterpret_cast<uintptr_t>(self))return {};
    std::lock_guard<std::mutex> lock(mutex);
    return same(current,bound)?effects:std::vector<Effect>{};
}
static void* option(uint32_t id){return reinterpret_cast<void*(__fastcall*)(uint32_t)>(address(0x146037e10))(id);}
static bool buffEnabled(void* self){
    auto table=*static_cast<uintptr_t**>(self);
    return reinterpret_cast<bool(__fastcall*)(void*)>(table[0x1c70/8])(self);
}
static void __fastcall collect(void* self,NativeVector* out){
    reinterpret_cast<Collect>(collectHook.original())(self,out);
    auto list=forCharacter(self);if(list.empty())return;
    InterlockedIncrement(&collectCalls);
    for(auto& e:list){
        // Buff-platinum belongs to the native buff skill context, never the
        // ordinary character accumulator. Normal platinum remains active.
        if(e.buff)continue;
        // 141e90fd0 ([IS BUFF CHARACTER]) uses this exact virtual predicate.
        auto p=option(e.bufferID&&buffEnabled(self)?e.bufferID:e.id);if(!p)continue;
        if(out->end==out->capacity)
            reinterpret_cast<void(__fastcall*)(NativeVector*,void**,void**)>(address(0x140295d90))(out,out->end,&p);
        else *out->end++=p;
    }
}
struct BuffScope {
    uintptr_t storage=0;
    explicit BuffScope(bool buff){reinterpret_cast<void(__fastcall*)(void*,bool)>(address(0x145a44cd0))(&storage,buff);}
    ~BuffScope(){reinterpret_cast<void(__fastcall*)(void*)>(address(0x145a47a60))(&storage);}
};
static void __fastcall skills(void* self){
    reinterpret_cast<Skills>(skillsHook.original())(self);
    auto list=forCharacter(self);if(list.empty())return;
    const bool buff=buffEnabled(self);
    InterlockedIncrement(&skillCalls);
    for(auto& e:list){
        if(e.bufferID)continue; // star options contain no skill-level rows
        if(e.buff&&!buff)continue;
        auto p=option(e.id);if(!p)continue;
        BuffScope scope(e.buff&&buff);
        reinterpret_cast<void(__fastcall*)(void*,void*)>(address(0x145c9d290))(self,p);
    }
}
static void __fastcall destroy(void* self){
    {std::lock_guard<std::mutex> lock(mutex);
     if(observed.character==reinterpret_cast<uintptr_t>(self)||bound.character==reinterpret_cast<uintptr_t>(self)){
        ++epoch;observed={};bound={};effects.clear();role=sequence=0;
     }}
    reinterpret_cast<Destroy>(destroyHook.original())(self);
}
static bool install(uintptr_t imageBase){
    base=imageBase;
    for(auto& g:native_symbol_effects_binding::guards){auto p=reinterpret_cast<void*>(address(g.va));if(!Readable(p,g.size)||memcmp(p,g.bytes,g.size))return false;}
    InlineHook* hooks[]={&collectHook,&skillsHook,&destroyHook};
    void* handlers[]={reinterpret_cast<void*>(collect),reinterpret_cast<void*>(skills),reinterpret_cast<void*>(destroy)};
    for(size_t i=0;i<3;++i){auto& g=native_symbol_effects_binding::guards[i];if(!PrepareHook(*hooks[i],reinterpret_cast<void*>(address(g.va)),g.bytes,g.size,handlers[i]))return false;}
    for(auto h:hooks)if(!PublishHook(*h)){for(auto restore:hooks)RestoreHook(*restore);return false;}
    return true;
}
}
