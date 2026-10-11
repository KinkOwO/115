#include "../../elite-companion-pilot/src/inline_hook.h"
#include "native_evidence.h"
#include <assert.h>
#include <stdio.h>
static void Dummy(){}
int main(){
    auto page=(unsigned char*)VirtualAlloc(nullptr,4096,MEM_RESERVE|MEM_COMMIT,PAGE_EXECUTE_READWRITE);assert(page);
    const unsigned char* heads[]={kResolveHead,kLookupHead,kCycleHead,kExpansionHead};
    const size_t sizes[]={sizeof(kResolveHead),sizeof(kLookupHead),sizeof(kCycleHead),sizeof(kExpansionHead)};
    InlineHook hooks[4];
    for(int i=0;i<4;++i){
        auto site=page+i*64;memcpy(site,heads[i],sizes[i]);
        assert(PrepareHook(hooks[i],site,heads[i],sizes[i],(void*)Dummy));
        assert(!memcmp(site,heads[i],sizes[i]));assert(!memcmp(hooks[i].page,heads[i],sizes[i]));
    }
    for(int i=0;i<4;++i)assert(PublishHook(hooks[i]));
    InlineHook conflict;assert(!PrepareHook(conflict,page,heads[0],sizes[0],(void*)Dummy));
    for(int i=3;i>=0;--i){assert(RestoreHook(hooks[i]));assert(!memcmp(page+i*64,heads[i],sizes[i]));}
    // Reject an installation changed by a different writer after preparation.
    InlineHook stale;assert(PrepareHook(stale,page,heads[0],sizes[0],(void*)Dummy));
    page[0]=0xcc;assert(!PublishHook(stale));assert(page[0]==0xcc);
    for(auto& h:hooks)VirtualFree(h.page,0,MEM_RELEASE);
    VirtualFree(stale.page,0,MEM_RELEASE);VirtualFree(page,0,MEM_RELEASE);
    puts("PASS four audited hook boundaries, conflict rejection, prepublication recheck and byte-exact rollback");
}
