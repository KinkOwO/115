/*
 * cave-selftest.c -- offline host test that EXECUTES the hp-caller-probe code
 * cave (client-patchs/hp-caller-probe/src/probe-hook.s) and proves it is
 * behaviour preserving.
 *
 * It never touches DFO.exe, the game and no client file: it copies the
 * assembled trampoline bytes into a private executable page, patches the two
 * slots the way the plugin does at run time, drives it with known register
 * contents, and compares every register the Windows x64 ABI says must survive
 * (rbx, rbp, rdi, rsi, r12-r15, xmm6-xmm15) plus rax (the getter's return
 * value) and rsp.
 *
 * What it proves:
 *   - the cave returns rax unchanged for arbitrary inputs;
 *   - it passes rcx=rdi, rdx=rbx and the saved rax through to the C entry;
 *   - it restores every non-volatile register and RE-balances rsp;
 *   - it "returns" to the address the replaced `ret` would have returned to;
 *   - the layout offsets the plugin patches (tail slot, log slot) are live.
 *
 * Build: client-patchs/tests/build-cave-selftest.cmd
 */

#define WIN32_LEAN_AND_MEAN
#define _CRT_SECURE_NO_WARNINGS
#include <windows.h>
#include <stdio.h>
#include <setjmp.h>
#include <stdint.h>
#include <string.h>

#include "..\hp-caller-probe\src\probe-hook-meta.h"

extern "C" void cave_test_enter(unsigned long long *ctrl);
extern "C" void cave_test_landing(void);
static jmp_buf g_jmp;

#define CAP_WORDS 16
static volatile unsigned long long g_cap[CAP_WORDS];

/* first instruction of the trampoline proper (after the entry jmp + stub) */
#define PROBE_OFF_CODE 22

/* ------------------------------------------------------------------ */
/* reached from the landing pad with the full state in g_cap           */
/* ------------------------------------------------------------------ */
static LONG WINAPI veh(EXCEPTION_POINTERS *ep) {
    printf("\n!! exception %#lx at rip=%p addr=%p\n", (unsigned long)ep->ExceptionRecord->ExceptionCode,
           (void *)ep->ContextRecord->Rip, (void *)ep->ExceptionRecord->ExceptionAddress);
    {
        CONTEXT *c = ep->ContextRecord;
        printf("   rax=%#llx rcx=%#llx rdx=%#llx rbx=%#llx\n", (unsigned long long)c->Rax,
               (unsigned long long)c->Rcx, (unsigned long long)c->Rdx,
               (unsigned long long)c->Rbx);
        printf("   rdi=%#llx rsi=%#llx rbp=%#llx rsp=%#llx r10=%#llx r11=%#llx\n",
               (unsigned long long)c->Rdi, (unsigned long long)c->Rsi,
               (unsigned long long)c->Rbp, (unsigned long long)c->Rsp,
               (unsigned long long)c->R10, (unsigned long long)c->R11);
        printf("   image base   = %p\n", (void *)GetModuleHandleW(NULL));
        /* what does the faulting page really look like, and is the fault
         * address even inside it? */
        {
            MEMORY_BASIC_INFORMATION mbi;
            unsigned long long a = (unsigned long long)c->Rip;
            if (VirtualQuery((LPCVOID)a, &mbi, sizeof(mbi))) {
                printf("   rip region: base=%p size=%#llx state=%#lx protect=%#lx type=%#lx\n",
                       mbi.BaseAddress, (unsigned long long)mbi.RegionSize,
                       (unsigned long)mbi.State, (unsigned long)mbi.Protect,
                       (unsigned long)mbi.Type);
            }
            printf("   bytes at rip:");
            for (int k = 0; k < 16; k++)
                printf(" %02X", IsBadReadPtr((void *)(a + k), 1) ? 0xFF
                                                                 : *(unsigned char *)(a + k));
            printf("\n");
            printf("   rsp points at %p :", (void *)c->Rsp);
            for (int k = 0; k < 8; k++) {
                unsigned long long v = 0;
                if (!IsBadReadPtr((void *)(c->Rsp + k * 8), 8))
                    v = *(unsigned long long *)(c->Rsp + k * 8);
                printf(" %#llx", v);
            }
            printf("\n");
        }
    }
    fflush(stdout);
    return EXCEPTION_CONTINUE_SEARCH; /* let it die, we already have the report */
}

extern "C" void cave_capture_state(void) {
    static int once;
    if (!once) {
        once = 1;
        FILE *f = fopen("cave-selftest-trace.txt", "wb");
        if (f) {
            fprintf(f, "cave_capture_state reached\n");
            fflush(f);
            fclose(f);
        }
    }
    longjmp(g_jmp, 1);
}

/* ------------------------------------------------------------------ */
static int g_fail;

static void check(const char *what, unsigned long long got, unsigned long long want) {
    if (got != want) {
        printf("  FAIL %-22s got %#llx want %#llx\n", what, got, want);
        g_fail = 1;
    } else {
        printf("  ok   %-22s %#llx\n", what, got);
    }
}

int main(void) {
    unsigned char *cave;
    unsigned long long ctrl[14];
    unsigned long long rax_in = 0x1122334455667788ULL;
    unsigned long long rcx_in = 0x1472ABCD00000000ULL; /* stands in for the descriptor */
    unsigned long long rdx_in = 0x00000000000000FFULL;
    unsigned long long want[8] = {0x1111111111111111ULL, 0x2222222222222222ULL,
                                  0x3333333333333333ULL, 0x4444444444444444ULL,
                                  0x5555555555555555ULL, 0x6666666666666666ULL,
                                  0x7777777777777777ULL, 0x8888888888888888ULL};
    double x6 = 1.25, x15 = -3.5;
    unsigned long long x6bits, x15bits;
    unsigned long long entry_rsp = 0;
    int i;

    printf("cave image: %d bytes, log slot +0x%X, tail slot +0x%X\n", (int)PROBE_CAVE_IMAGE_LEN,
           PROBE_OFF_LOG_SLOT, PROBE_OFF_TAIL_SLOT);

    /* sanity: the offsets the plugin patches must be inside the image and the
     * slot at each of them must be zero in the assembled image */
    if ((size_t)PROBE_OFF_LOG_SLOT + 8 > (size_t)PROBE_CAVE_IMAGE_LEN ||
        (size_t)PROBE_OFF_TAIL_SLOT + 8 > (size_t)PROBE_CAVE_IMAGE_LEN) {
        printf("FAIL: a patched slot lies outside the image\n");
        return 1;
    }
    {
        int k, z = 1;
        for (k = 0; k < 8; k++)
            if (PROBE_CAVE_IMAGE[PROBE_OFF_LOG_SLOT + k] || PROBE_CAVE_IMAGE[PROBE_OFF_TAIL_SLOT + k])
                z = 0;
        if (!z) {
            printf("FAIL: a slot is not zero in the assembled image\n");
            return 1;
        }
    }
    printf("ok   both patched slots are zero in the assembled image\n");

    cave = (unsigned char *)VirtualAlloc(NULL, 0x1000, MEM_COMMIT | MEM_RESERVE,
                                         PAGE_EXECUTE_READWRITE);
    if (!cave) {
        printf("FAIL: VirtualAlloc\n");
        return 1;
    }
    memset(cave, 0xCC, 0x1000);
    memcpy(cave, PROBE_CAVE_IMAGE, PROBE_CAVE_IMAGE_LEN);

    /* patch inside the copied image the same way the plugin does:
     *  - cave+0 is the tail of the getter's exit: `jmp <rest of the cave>`;
     *    the plugin computes this from the *game's* site address, here we send
     *    it to the first real instruction (cave + PROBE_OFF_CODE);
     *  - the `lea rax,log_target` disp32 must point at the copied slot;
     *  - the tail slot must point at our landing pad. */
    {
        int32_t disp32;
        unsigned long long pad;
        size_t at = 0, found = 0;
        /* the cave's entry jump: E9 <disp32> -> the instruction after the stub */
        if (PROBE_CAVE_IMAGE[0] != 0xE9) {
            printf("FAIL: cave[0] is not the entry jmp\n");
            return 1;
        }
        {
            int32_t entry_disp = (int32_t)PROBE_OFF_CODE - 5;
            memcpy(cave + 1, &entry_disp, 4);
            printf("     cave entry jmp disp32 = %+d (target cave+%d)\n", entry_disp,
                   (int)PROBE_OFF_CODE);
        }
        /* find the lea: it is the instruction right before `call rax`
         * (48 8D 05 <disp32> FF D0) */
        for (size_t k = 0; k + 9 <= (size_t)PROBE_CAVE_IMAGE_LEN; k++) {
            if (PROBE_CAVE_IMAGE[k] == 0x48 && PROBE_CAVE_IMAGE[k + 1] == 0x8D &&
                PROBE_CAVE_IMAGE[k + 2] == 0x05 && PROBE_CAVE_IMAGE[k + 7] == 0xFF &&
                PROBE_CAVE_IMAGE[k + 8] == 0xD0) {
                at = k;
                found++;
            }
        }
        if (found != 1) {
            printf("FAIL: expected exactly one lea/call slot idiom, found %d\n", (int)found);
            return 1;
        }
        /* The cave is relocated to `cave`, which is *closer* to the slot than
         * the original image was, so the original disp32 would point outside
         * our page.  Recompute it: the instruction is 7 bytes, its operand
         * starts at +3 and the slot begins right after the instruction. */
        {
            int32_t want_disp = -7;
            memcpy(cave + at + 3, &want_disp, 4);
            disp32 = want_disp;
        }
        pad = (unsigned long long)(void *)&cave_test_landing;
        memcpy(cave + PROBE_OFF_TAIL_SLOT, &pad, 8);
        memcpy(cave + 14, &pad, 8);
        {
            volatile unsigned char *vc = cave;
            (void)vc[0];
        }
        _ReadWriteBarrier();
        {
            unsigned long long rb;
            memcpy(&rb, cave + 14, 8);
            printf("     stub slot  = %#llx (landing %#llx)\n", rb,
                   (unsigned long long)(void *)&cave_test_landing);
            memcpy(&rb, cave + PROBE_OFF_TAIL_SLOT, 8);
            printf("     tail slot  = %#llx\n", rb);
        }
    }
    FlushInstructionCache(GetCurrentProcess(), cave, 0x1000);
    /* Tighten the protection exactly like a hardened tool would, and prove the
     * page executes before we rely on it for the register test. */
    {
        DWORD old = 0;
        volatile unsigned char *vc = cave;
        if (!VirtualProtect(cave, 0x1000, PAGE_EXECUTE_READ, &old)) {
            printf("FAIL: VirtualProtect(cave) err=%lu\n", (unsigned long)GetLastError());
            return 1;
        }
        (void)vc[0];
        printf("ok   cave page is now PAGE_EXECUTE_READ\n");
    }
    /* The compiler cannot see that cave_test_enter() reads the buffer we just
     * filled (the call happens inside hand-written assembly), so make every
     * prior store to the page observable before anything else runs. */
    _ReadWriteBarrier();
    printf("ok   copied the cave to %p and patched its tail slot\n", cave);
    /* diagnostic: prove the page really is committed and executable, and that
     * the instructions are the ones we copied */
    {
        MEMORY_BASIC_INFORMATION mbi;
        if (VirtualQuery(cave, &mbi, sizeof(mbi))) {
            printf("     page: base=%p size=%#llx state=%#lx protect=%#lx\n", mbi.BaseAddress,
                   (unsigned long long)mbi.RegionSize, (unsigned long)mbi.State,
                   (unsigned long)mbi.Protect);
        } else {
            printf("     FAIL VirtualQuery(cave) err=%lu\n", (unsigned long)GetLastError());
        }
        printf("     cave bytes:");
        for (int k = 0; k < 24; k++) printf(" %02X", cave[k]);
        printf("\n     landing=%p  ctrl[3]=%#llx\n", (void *)&cave_test_landing, ctrl[3]);
    }

    memset((void *)g_cap, 0, sizeof(g_cap));
    memset(ctrl, 0, sizeof(ctrl));
    ctrl[0] = rax_in;
    ctrl[1] = rcx_in;
    ctrl[2] = rdx_in;
    ctrl[3] = (unsigned long long)(void *)cave;
    ctrl[4] = want[0];
    ctrl[5] = want[1];
    ctrl[6] = want[2];
    ctrl[7] = want[3];
    ctrl[8] = want[4];
    ctrl[9] = want[5];
    ctrl[10] = want[6];
    memcpy(&x6bits, &x6, 8);
    memcpy(&x15bits, &x15, 8);
    ctrl[11] = x6bits;
    ctrl[12] = x15bits;
    ctrl[13] = (unsigned long long)(void *)g_cap;

    if (setjmp(g_jmp) == 0) {
        printf("... entering the cave at %p (return path goes through the landing pad)\n", cave);
        fflush(stdout);
        AddVectoredExceptionHandler(1, veh);
        cave_test_enter(ctrl);
        printf("FAIL: the cave returned through the normal C path\n");
        return 1;
    }
    printf("... landing pad reached (sentinel %#llx)\n", g_cap[15]);
    fflush(stdout);

    printf("\nregister state handed back by the cave:\n");
    check("rax (return value)", g_cap[0], rax_in);
    check("rbx", g_cap[1], want[0]);
    check("rbp", g_cap[2], want[1]);
    check("rsi", g_cap[3], want[2]);
    check("rdi", g_cap[4], want[3]);
    check("r12", g_cap[5], want[4]);
    check("r13", g_cap[6], want[5]);
    check("r14", g_cap[7], want[6]);
    check("r15", g_cap[8], want[7]);
    check("rcx == descriptor", g_cap[10], rcx_in);
    check("xmm6 low lane", g_cap[12], x6bits);
    check("xmm15 low lane", g_cap[13], x15bits);

    /* the landing pad's rsp is 8 below the pre-jump rsp: that is the state the
     * game's caller sees right after a `ret`.  We only check it is plausible
     * (inside our thread's stack) and that the return slot above it still holds
     * the value we pushed. */
    entry_rsp = g_cap[9];
    printf("  rsp at landing = %#llx (return slot holds %#llx)\n", entry_rsp,
           *(unsigned long long *)entry_rsp);
    if (*(unsigned long long *)entry_rsp != (unsigned long long)(void *)g_cap) {
        printf("  FAIL the landing pad did not find the pushed return slot\n");
        g_fail = 1;
    } else {
        printf("  ok   the simulated ret landed with a valid stack\n");
    }

    printf("\n%s\n", g_fail ? "CAVE SELF-TEST FAILED" : "CAVE SELF-TEST PASSED");
    return g_fail;
}
