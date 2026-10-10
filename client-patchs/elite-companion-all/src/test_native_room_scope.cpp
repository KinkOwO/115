#include "native_room_scope.h"
#include <assert.h>
#include <stdio.h>
static unsigned char flag=0;
static int calls=0;
static bool fail=false,nested=false;
static void __fastcall Native(void* self,int x,int y,bool a4,bool a5,void* actor,bool a7,bool a8) {
    assert(self==(void*)0x1234&&x==19&&y==-27&&a4&&!a5&&actor==(void*)0x5678&&a7&&!a8);
    ++calls;
    if(nested) {
        nested=false;
        assert(flag==1);
        RunPilotRoom(Native,self,x,y,a4,a5,actor,a7,a8,&flag);
        assert(flag==1);
    }
    if(fail)RaiseException(0xe0a18790,0,0,nullptr);
}
static void Invoke(unsigned char* scoped) {
    RunPilotRoom(Native,(void*)0x1234,19,-27,true,false,(void*)0x5678,true,false,scoped);
}
int main() {
    Invoke(nullptr);assert(flag==0&&calls==1);
    nested=true;Invoke(&flag);assert(flag==0&&calls==3);
    flag=7;Invoke(&flag);assert(flag==7&&calls==4);
    flag=0;fail=true;bool caught=false;
    __try {Invoke(&flag);}
    __except(GetExceptionCode()==0xe0a18790?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH) {caught=true;}
    assert(caught&&flag==0&&calls==5);
    fail=false;Invoke(nullptr);assert(flag==0&&calls==6);
    puts("PASS native room eight arguments, nesting, exact flag restoration, propagated SEH and later non-pilot call");
}
