/* Mechanism test in this process only; never loads or launches DFO. */
#include "../apocalypse-third-card/src/apocalypse-third-card.c"
#include <assert.h>

static unsigned char panel[PANEL_SIZE];
typedef struct { uintptr_t *vt; BOOL enabled, visible; } TestCard;
static TestCard testCards[3];
static uintptr_t __fastcall testCtor(void *p,uintptr_t a,uintptr_t b,uintptr_t c) {
    (void)a;(void)b;(void)c;memset(p,0xcc,PANEL_SIZE);return (uintptr_t)p;
}
static uintptr_t __fastcall testInit(void *p,uintptr_t a,uintptr_t b,uintptr_t c) {
    unsigned i;(void)a;(void)b;(void)c;
    for(i=0;i<3;i++)((void**)((unsigned char*)p+CARD_ARRAY))[i]=&testCards[i];
    return 0;
}
static void __fastcall setEnabled(void *p,BOOL value){((TestCard*)p)->enabled=value;}
static void __fastcall setVisible(void *p,BOOL value){((TestCard*)p)->visible=value;}
static void commitAt(DWORD rva) {
    uintptr_t at=(uintptr_t)(image+rva)&~(uintptr_t)0xfff;
    assert(VirtualAlloc((void*)at,4096,MEM_COMMIT,PAGE_READWRITE));
}
int main(void) {
    unsigned i,j;DWORD previous;MEMORY_BASIC_INFORMATION m;unsigned char *relay;
    image=VirtualAlloc(NULL,0x9400000,MEM_RESERVE,PAGE_NOACCESS);assert(image);
    for(i=0;i<COUNT(patches);i++){
        commitAt(patches[i].rva);
        memcpy(image+patches[i].rva,patches[i].before,patches[i].len);
    }
    assert(expectedBytes());
    image[patches[COUNT(patches)-1].rva]^=1;
    relay=nearPage();assert(relay && prepareCalls(relay));
    assert(VirtualProtect(relay,4096,PAGE_EXECUTE_READ,&previous));
    FlushInstructionCache(GetCurrentProcess(),relay,4096);
    for(i=0;i<COUNT(patches);i++)if(patches[i].hook){
        int32_t displacement;memcpy(&displacement,patches[i].after+1,4);
        assert(image+patches[i].rva+5+displacement==relay+(patches[i].hook-1)*16);
    }
    assert(!installBytes());
    for(i=0;i<COUNT(patches)-1;i++)assert(!memcmp(image+patches[i].rva,patches[i].before,patches[i].len));
    image[patches[COUNT(patches)-1].rva]^=1;
    for(i=0;i<COUNT(patches);i++)assert(VirtualProtect(image+patches[i].rva,patches[i].len,PAGE_EXECUTE_READ,&previous));
    assert(installBytes());
    for(i=0;i<COUNT(patches);i++){
        assert(!memcmp(image+patches[i].rva,patches[i].after,patches[i].len));
        assert(VirtualQuery(image+patches[i].rva,&m,sizeof(m))==sizeof(m));
        assert(m.Protect==PAGE_EXECUTE_READ);
    }
    assert(!installBytes());
    originalCtor=testCtor;originalInit=testInit;
    assert(((Native4)relay)(panel,0,0,0)==(uintptr_t)panel);
    for(i=0;i<CARD_ARRAY;i++)assert(panel[i]==0xcc);
    for(i=CARD_ARRAY;i<PANEL_SIZE;i++)assert(panel[i]==0);
    commitAt(0x92e4e60);
    ((uintptr_t*)(image+0x92e4e60))[2]=(uintptr_t)setEnabled;
    ((uintptr_t*)(image+0x92e4e60))[3]=(uintptr_t)setVisible;
    for(i=0;i<3;i++)testCards[i].vt=(uintptr_t*)(image+0x92e4e60);
    /* Status files go into the test executable's own dist directory. */
    assert(GetModuleFileNameW(NULL,directory,COUNT(directory)));
    {wchar_t *slash=wcsrchr(directory,L'\\');assert(slash);slash[1]=0;}
    ((Native4)(relay+16))(panel,0,0,0);
    assert(initialized==1 && testCards[2].enabled && testCards[2].visible);
    for(j=0;j<2;j++)assert(!testCards[j].enabled && !testCards[j].visible);
    assert(panel[0x3b0]==0xcc); /* confirmation window never overlaps card 3 */
    testCards[2].vt=NULL;panelInit(panel,0,0,0);assert(initialized==1);
    VirtualFree(relay,0,MEM_RELEASE);
    VirtualFree(image,0,MEM_RELEASE);
    puts("PASS: byte preflight, all-or-none writes, page restoration, extended layout, third-card flags");
    return 0;
}
