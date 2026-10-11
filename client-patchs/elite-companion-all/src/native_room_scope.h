#pragma once
#include <windows.h>
#include <stdint.h>

// Native room initializer has four register and four stack arguments. Preserve
// them all, including the actor pointer, when entering the audited spawn path.
using RoomFn=void(__fastcall*)(void*,int,int,bool,bool,void*,bool,bool);
inline void RunPilotRoom(RoomFn original,void* self,int x,int y,bool a4,
                         bool a5,void* actor,bool a7,bool a8,unsigned char* flag) {
    if(!flag){original(self,x,y,a4,a5,actor,a7,a8);return;}
    unsigned char before=*flag;
    __try {
        *flag=1;
        original(self,x,y,a4,a5,actor,a7,a8);
    } __finally {
        *flag=before;
    }
}
