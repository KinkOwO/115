#pragma once
#include "native_room_scope.h"

// The manager latch outlives a direct-move map. Only the audited room CALL
// may ignore it once after a fresh successful N1879 clone generation.
struct NativeRegistrationFrame {
    uintptr_t manager=0;
    bool pending=false;
    bool queried=false;
    bool bypassed=false;
};
inline bool NativeRegistrationActive(bool native,NativeRegistrationFrame* frame,uintptr_t manager) {
    if(!frame||!frame->pending||frame->queried||!manager||manager!=frame->manager)return native;
    frame->queried=true;
    frame->bypassed=native;
    return false;
}
inline void RunRegistrationRoom(NativeRegistrationFrame*& current,NativeRegistrationFrame* frame,
                                 RoomFn original,void* self,int x,int y,bool a4,bool a5,
                                 void* actor,bool a7,bool a8,unsigned char* flag) {
    NativeRegistrationFrame* before=current;
    __try {
        current=frame;
        RunPilotRoom(original,self,x,y,a4,a5,actor,a7,a8,flag);
    } __finally {
        current=before;
    }
}
