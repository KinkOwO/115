#include "native_registration_scope.h"
#include <assert.h>
#include <stdio.h>
static NativeRegistrationFrame* current=nullptr;
static bool active=false,fail=false,nest=false;
static unsigned char flag=0;
static int registrations=0;
static void __fastcall Native(void* self,int x,int y,bool a4,bool a5,void* actor,bool a7,bool a8) {
    assert(self==(void*)0x1234&&x==19&&y==-27&&a4&&!a5&&actor==(void*)0x5678&&a7&&!a8);
    assert(flag==1);
    if(nest) {
        nest=false;NativeRegistrationFrame other={0x9999,true};
        NativeRegistrationFrame* before=current;
        RunRegistrationRoom(current,&other,Native,self,x,y,a4,a5,actor,a7,a8,&flag);
        assert(current==before);
    }
    if(fail)RaiseException(0xe0a18791,0,0,nullptr);
    if(!NativeRegistrationActive(active,current,0x1111)){registrations+=3;active=true;}
}
static void Invoke(NativeRegistrationFrame* frame) {
    RunRegistrationRoom(current,frame,Native,(void*)0x1234,19,-27,true,false,(void*)0x5678,true,false,&flag);
}
int main() {
    NativeRegistrationFrame first={0x1111,true};
    Invoke(&first);assert(active&&registrations==3&&first.queried&&!first.bypassed&&flag==0&&!current);
    NativeRegistrationFrame consumed={0x1111,false};
    Invoke(&consumed);assert(registrations==3&&!consumed.queried);
    // New map, fresh clones, stale native latch: the old path skips all three.
    assert(NativeRegistrationActive(active,nullptr,0x1111));
    NativeRegistrationFrame second={0x1111,true};
    Invoke(&second);assert(registrations==6&&second.bypassed&&active);
    assert(NativeRegistrationActive(true,&second,0x1111)); // same frame cannot bypass twice
    NativeRegistrationFrame wrong={0x2222,true};
    Invoke(&wrong);assert(registrations==6&&!wrong.queried);
    assert(NativeRegistrationActive(true,&second,0));
    nest=true;NativeRegistrationFrame third={0x1111,true};
    Invoke(&third);assert(registrations==9&&!current&&flag==0);
    fail=true;bool caught=false;
    __try{Invoke(&third);}
    __except(GetExceptionCode()==0xe0a18791?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH){caught=true;}
    assert(caught&&!current&&flag==0&&active&&registrations==9);
    fail=false;Invoke(nullptr);assert(registrations==9&&active&&!current);
    puts("PASS fresh/repeated generations, stale-latch next map, consumed/no duplicate, wrong manager, nested context, native latch unchanged and propagated SEH");
}
