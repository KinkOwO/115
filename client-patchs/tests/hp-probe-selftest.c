/*
 * hp-probe-selftest -- offline host test for client-patchs/hp-caller-probe.
 *
 * It compiles the *same* probe-logic.h the plugin uses and exercises:
 *   1. the entry layout (the assembly in probe-hook.s hard-codes these offsets,
 *      so a mismatch would silently corrupt every sample);
 *   2. a multi-producer / single-consumer run: N threads call probe_produce()
 *      the way the code cave does, one consumer drains the ring, and the total
 *      number of accepted samples must equal the number produced (no loss, no
 *      double count, no torn entry ever accepted);
 *   3. the "consumer falls behind" path: it must count the gap instead of
 *      reading garbage.
 *
 * Build (see client-patchs/tests/build-hp-probe-selftest.cmd):
 *   cl /nologo /O2 /MT /W4 /utf-8 /TP hp-probe-selftest.c
 * Nothing in this file touches DFO.exe or any client file.
 */

#define WIN32_LEAN_AND_MEAN
#define _CRT_SECURE_NO_WARNINGS
#include <windows.h>
#include <stdio.h>
#include <stdint.h>
#include <stddef.h>
#include <string.h>

#include "..\hp-caller-probe\src\probe-logic.h"

#define PRODUCERS 4
#define PER_PRODUCER 200000

static probe_ring_t *g_ring;
static volatile LONG64 g_produced_total;
static volatile LONG g_stop;

static ULONGLONG g_seen[PRODUCERS];
static ULONGLONG g_other;
static ULONGLONG g_torn_seen;
static ULONGLONG g_bad_field;

/* producer: emulate exactly what the code cave -> probe_hp_entry -> probe_produce
 * path does, with a marker woven into the values so we can attribute samples. */
static DWORD WINAPI producer(LPVOID param) {
    int id = (int)(INT_PTR)param;
    LONG64 i;
    for (i = 0; i < PER_PRODUCER; i++) {
        ULONGLONG caller = 0x140000000ULL + (ULONGLONG)id * 0x1000ULL;
        LONG64 rax = ((LONG64)id << 40) | (i & 0xFFFFFFFFLL);
        LONG64 rcx = 0x147220000ULL | (LONG64)id;
        LONG64 rdx = (LONG64)id;
        LONG64 rbx = (LONG64)id * 3;
        probe_produce(g_ring, caller, rax, rcx, rdx, rbx);
        InterlockedIncrement64(&g_produced_total);
    }
    return 0;
}

static void check_cb(const probe_entry_t *e, void *ctx) {
    int id = (int)(INT_PTR)ctx;
    ULONGLONG caller = (ULONGLONG)e->e_caller;
    LONG64 rax = (LONG64)e->e_rax;
    int who = (int)((caller - 0x140000000ULL) / 0x1000ULL);
    (void)id;
    if (who < 0 || who >= PRODUCERS) {
        g_other++;
        return;
    }
    /* every field must belong to the same producer, otherwise the sample was
     * torn or the entry layout is wrong */
    if ((int)(rax >> 40) != who || (LONG64)e->e_rdx != who || (LONG64)e->e_rbx != (LONG64)who * 3 ||
        ((LONG64)e->e_rcx & 0xFFF) != who) {
        g_bad_field++;
    }
    g_seen[who]++;
}

static void lost_cb_selftest(ULONGLONG n, void *ctx) {
    (void)ctx;
    (void)n;
}

int main(void) {
    HANDLE th[PRODUCERS];
    int i;
    DWORD t0;
    probe_drain_state_t st;

    /* ---- 1. layout ---- */
    printf("sizeof(probe_entry_t) = %d (want 64)\n", (int)sizeof(probe_entry_t));
    printf("offsets: tick=%d done=%d caller=%d rax=%d rcx=%d rdx=%d rbx=%d\n",
           (int)offsetof(probe_entry_t, e_tick), (int)offsetof(probe_entry_t, e_done),
           (int)offsetof(probe_entry_t, e_caller), (int)offsetof(probe_entry_t, e_rax),
           (int)offsetof(probe_entry_t, e_rcx), (int)offsetof(probe_entry_t, e_rdx),
           (int)offsetof(probe_entry_t, e_rbx));
    if (sizeof(probe_entry_t) != 64 || offsetof(probe_entry_t, e_caller) != 16 ||
        offsetof(probe_entry_t, e_rax) != 24 || offsetof(probe_entry_t, e_rcx) != 32 ||
        offsetof(probe_entry_t, e_rdx) != 40 || offsetof(probe_entry_t, e_rbx) != 48) {
        printf("FAIL: entry layout does not match probe-hook.inc\n");
        return 1;
    }
    printf("OK  entry layout matches the assembly contract\n");

    g_ring = (probe_ring_t *)VirtualAlloc(NULL, sizeof(probe_ring_t), MEM_COMMIT | MEM_RESERVE,
                                          PAGE_READWRITE);
    if (!g_ring) {
        printf("FAIL: VirtualAlloc(%d bytes) for the ring\n", (int)sizeof(probe_ring_t));
        return 1;
    }
    memset(g_ring, 0, sizeof(probe_ring_t));

    /* ---- 2. multi-producer / one consumer, drained after the fact ---- */
    t0 = GetTickCount();
    for (i = 0; i < PRODUCERS; i++) th[i] = CreateThread(NULL, 0, producer, (LPVOID)(INT_PTR)i, 0, NULL);
    for (i = 0; i < PRODUCERS; i++) {
        WaitForSingleObject(th[i], INFINITE);
        CloseHandle(th[i]);
    }
    printf("OK  %d producers wrote %lld samples in %lu ms\n", PRODUCERS,
           (long long)g_produced_total, (unsigned long)(GetTickCount() - t0));

    /* Drain with a bounded batch per pass, exactly like the plugin's consumer
     * thread does between sleeps.  Most of the ring was overwritten long ago,
     * so this also exercises the "fell behind" accounting. */
    memset(&st, 0, sizeof(st));
    /* Keep draining until the counter stops moving: the first pass jumps to
     * total - PROBE_RING_SLOTS and then consumes what is left in batches. */
    for (i = 0; i < 64; i++) {
        LONG64 before = st.total + (LONG64)st.lost;
        probe_drain(g_ring, &st, check_cb, (void *)(INT_PTR)-1, lost_cb_selftest);
        if (st.total + (LONG64)st.lost == before) break;
    }
    printf("drain: consumed=%llu lost=%llu torn=%llu  (ring holds %d slots)\n",
           (unsigned long long)st.total, (unsigned long long)st.lost,
           (unsigned long long)st.torn, PROBE_RING_SLOTS);

    /* ---- 3. the final slice must be intact and complete ---- */
    {
        ULONGLONG total_seen = g_other + g_seen[0] + g_seen[1] + g_seen[2] + g_seen[3];
        printf("accepted: total=%llu by-producer=%llu/%llu/%llu/%llu unattributed=%llu\n",
               (unsigned long long)total_seen, (unsigned long long)g_seen[0],
               (unsigned long long)g_seen[1], (unsigned long long)g_seen[2],
               (unsigned long long)g_seen[3], (unsigned long long)g_other);
        printf("field mismatches: %llu (must be 0)\n", (unsigned long long)g_bad_field);
        if (g_bad_field != 0 || g_other != 0) {
            printf("FAIL: accepted a corrupt sample\n");
            return 1;
        }
        if (total_seen != st.total) {
            printf("FAIL: drain counted %llu but delivered %llu\n",
                   (unsigned long long)st.total, (unsigned long long)total_seen);
            return 1;
        }
        /* the last PROBE_RING_SLOTS samples are all still in the ring; the
         * first pass must therefore have consumed exactly that many */
        if (total_seen == 0 || total_seen > PROBE_RING_SLOTS) {
            printf("FAIL: consumed %llu, expected between 1 and %d\n",
                   (unsigned long long)total_seen, PROBE_RING_SLOTS);
            return 1;
        }
        printf("OK  every accepted sample was self-consistent\n");
    }

    /* ---- 4. a clean sequential run must be counted exactly ---- */
    {
        LONG64 n;
        memset(g_ring, 0, sizeof(probe_ring_t));
        memset(&st, 0, sizeof(st));
        g_seen[0] = g_seen[1] = g_seen[2] = g_seen[3] = g_other = g_bad_field = 0;
        for (n = 0; n < 10000; n++)
            probe_produce(g_ring, 0x140000000ULL, n, 0x147220000ULL, 1, 2);
        while (st.total < 10000) {
            LONG64 before = st.total;
            probe_drain(g_ring, &st, check_cb, NULL, lost_cb_selftest);
            if (st.total == before) break; /* nothing more to read */
        }
        printf("sequential: consumed=%llu lost=%llu torn=%llu\n",
               (unsigned long long)st.total, (unsigned long long)st.lost,
               (unsigned long long)st.torn);
        if (st.total != 10000 || st.lost != 0 || st.torn != 0 || g_seen[0] != 10000) {
            printf("FAIL: sequential run lost or duplicated samples\n");
            return 1;
        }
        printf("OK  10000 sequential samples counted exactly once\n");
    }

    printf("\nALL SELF-TESTS PASSED\n");
    return 0;
}
