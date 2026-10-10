#include "companion_resource_guard.h"
#include "companion_load_scope.h"
#include <assert.h>
#include <stdio.h>
static LONG depth=0;
static int cloneChecks=0,validCalls=0;
static void* __fastcall Absent(void*,uint32_t,bool){return nullptr;}
static void __fastcall Valid(void*,uint32_t){++validCalls;}
static void __fastcall Clone(){
    assert(depth>0);
    assert(CompanionPreload(depth>0,(void*)1,490100178,Absent,Valid,nullptr)==PreloadResult::MissingResource);
    ++cloneChecks;
}
static void __fastcall NestedClone(){
    assert(depth==1);
    RunCompanionLoad(depth,Clone);
    assert(depth==1);
}
static void __fastcall Fault(){RaiseException(0xE0421879,0,0,nullptr);}
static void VerifyLoadStages(){
    RunCompanionLoad(depth,Clone); // Original source-reader stage.
    assert(depth==0);
    RunCompanionLoad(depth,Clone); // Later AI clone stage must still guard.
    assert(depth==0&&cloneChecks==2&&validCalls==0);
    RunCompanionLoad(depth,NestedClone);
    assert(depth==0&&cloneChecks==3);
    bool propagated=false;
    __try {RunCompanionLoad(depth,Fault);}
    __except(GetExceptionCode()==0xE0421879?EXCEPTION_EXECUTE_HANDLER:EXCEPTION_CONTINUE_SEARCH){propagated=true;}
    assert(propagated&&depth==0);
    assert(CompanionPreload(depth>0,(void*)1,490100178,Absent,Valid,nullptr)==PreloadResult::Native);
    assert(validCalls==1); // Guard never leaks into subsequent player loading.
}
int main(){
    VerifyLoadStages();
    int lookups=0,calls=0;uint32_t received=0;
    void* manager=(void*)0x1234;void* self=(void*)0x5678;bool present=true;
    auto lookup=[&](void* m,uint32_t id,bool load)->void*{
        assert(m==manager&&load);lookups++;received=id;return present?self:nullptr;
    };
    auto original=[&](void* p,uint32_t id){assert(p==self);calls++;received=id;};
    assert(CompanionPreload(true,manager,17,lookup,original,self)==PreloadResult::Native);
    assert(lookups==1&&calls==1&&received==17);
    present=false;
    assert(CompanionPreload(true,manager,23,lookup,original,self)==PreloadResult::MissingResource);
    assert(lookups==2&&calls==1&&received==23); // Native null dereference is not invoked.
    assert(CompanionPreload(true,nullptr,31,lookup,original,self)==PreloadResult::MissingManager);
    assert(lookups==2&&calls==1);
    assert(CompanionPreload(false,manager,23,lookup,original,self)==PreloadResult::Native);
    assert(lookups==2&&calls==2&&received==23); // Ordinary player/native content untouched.
    present=true;
    assert(CompanionPreload(true,manager,37,lookup,original,self)==PreloadResult::Native);
    assert(lookups==3&&calls==3&&received==37); // Never disables valid later resources.
    puts("PASS companion preload: valid original, absent resource/manager, ordinary bypass, recovery");
    puts("PASS N1382/N1879 stages: clone guarded, nesting restored, SEH propagates, player path restored");
}
