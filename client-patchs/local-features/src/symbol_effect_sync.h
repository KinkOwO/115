#pragma once
#include "native_symbol_effects.h"

namespace symbol_effect_sync {
using Transport=std::vector<uint8_t>(*)(uint16_t,uint64_t,const std::vector<uint8_t>&);
static Transport transport;
static HANDLE worker=nullptr,wake=nullptr,stop=nullptr;
static FILE* log;
static DWORD WINAPI run(void*){
    uint64_t id=0x4000000000000000ull;
    bool refreshNeeded=false;
    native_symbol_effects::Identity pending{};
    ULONGLONG lastError=0;
    while(WaitForSingleObject(stop,0)!=WAIT_OBJECT_0){
        const auto who=native_symbol_effects::identity();
        if(who.character){
            try{
                auto view=symbol_wire::decode(transport(3,++id,{}));
                if(view.source!=native_symbol_effects_binding::source)throw std::runtime_error("symbol source differs from the installed client");
                if(!native_symbol_effects::same(who,native_symbol_effects::identity()))continue;
                if(!native_symbol_effects::same(who,pending))refreshNeeded=false;
                if(native_symbol_effects::publish(who,view)){refreshNeeded=true;pending=who;}
                if(refreshNeeded){
                    auto confirmed=symbol_wire::decode(transport(5,++id,symbol_wire::refresh(view)));
                    if(confirmed.role!=view.role||confirmed.sequence!=view.sequence||confirmed.source!=view.source)throw std::runtime_error("stale symbol refresh");
                    refreshNeeded=false;
                    if(log){fprintf(log,"symbol effects refreshed role=%llu sequence=%llu\n",view.role,view.sequence);fflush(log);}
                }
            }catch(const std::exception& e){
                // Retry login transitions without calling native code from
                // the worker, but retain throttled diagnostic evidence.
                if(log&&GetTickCount64()-lastError>10000){lastError=GetTickCount64();fprintf(log,"symbol effects retry: %s\n",e.what());fflush(log);}
            }
        }else refreshNeeded=false;
        HANDLE events[]={stop,wake};WaitForMultipleObjects(2,events,FALSE,1500);
    }
    return 0;
}
static bool start(uintptr_t base,FILE* sink,Transport exchange){
    transport=exchange;log=sink;
    if(!native_symbol_effects::install(base))return false;
    wake=CreateEventW(nullptr,FALSE,FALSE,nullptr);stop=CreateEventW(nullptr,TRUE,FALSE,nullptr);
    if(!wake||!stop)return false;
    worker=CreateThread(nullptr,0,run,nullptr,0,nullptr);return worker!=nullptr;
}
static void changed(){if(wake)SetEvent(wake);}
static void shutdown(){
    if(stop)SetEvent(stop);
    if(worker){WaitForSingleObject(worker,INFINITE);CloseHandle(worker);worker=nullptr;}
    if(wake){CloseHandle(wake);wake=nullptr;}if(stop){CloseHandle(stop);stop=nullptr;}
    log=nullptr;
}
}
