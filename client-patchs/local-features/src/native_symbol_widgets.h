#pragma once
#include <windows.h>
#include <cstdint>
#include <cstring>
#include "native_symbol_widget_binding.h"

// Candidate widget ABI, not a popup controller. The owner must already have
// verified the executable SHA256 and paired resources, and call on the UI
// thread while holding the native SharedControl reference. Not wired into r20.
namespace native_symbol_widgets {
class API {
    uintptr_t base_=0;
    uintptr_t address(uintptr_t va)const{return base_+va-0x140000000ull;}
    static bool copy(uintptr_t address,void* data,size_t bytes) {
        SIZE_T got=0;
        return address && ReadProcessMemory(GetCurrentProcess(),reinterpret_cast<void*>(address),data,bytes,&got) && got==bytes;
    }
public:
    bool bind(uintptr_t imageBase) {
        base_=0;if(!imageBase)return false;
        for(const auto& guard:native_symbol_widget_binding::guards) {
            unsigned char bytes[32]{};
            if(!copy(imageBase+guard.va-0x140000000ull,bytes,guard.size) ||
               memcmp(bytes,guard.bytes,guard.size))return false;
        }
        uintptr_t methods[4]{};
        if(!copy(imageBase+native_symbol_widget_binding::itemVTable-0x140000000ull,methods,sizeof methods))return false;
        for(unsigned i=0;i<4;++i)
            if(methods[i]!=imageBase+native_symbol_widget_binding::itemMethods[i]-0x140000000ull)return false;
        base_=imageBase;return true;
    }
    bool setTemplate(void* control,uint32_t item)const {
        if(!base_ || !control || item>INT32_MAX)return false;
        unsigned char object[0x6b8];
        if(!copy(reinterpret_cast<uintptr_t>(control),object,sizeof object))return false;
        uintptr_t table=0;memcpy(&table,object,sizeof table);
        if(table!=address(native_symbol_widget_binding::itemVTable))return false;
        // Native callers clear with -1, then rebuild via createItem(this,null).
        reinterpret_cast<void(__fastcall*)(void*,int32_t)>(address(0x14501c1f0))
            (control,item?int32_t(item):-1);
        reinterpret_cast<void(__fastcall*)(void*,void*)>(address(0x14501a490))(control,nullptr);
        return true;
    }
};
}
