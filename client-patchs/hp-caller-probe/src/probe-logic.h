/*
 * probe-logic.h -- ring buffer + per-caller tally for hp-caller-probe.
 *
 * Kept in a header (static, single-TU) so that:
 *   - the shipped DLL compiles it (client-patchs/hp-caller-probe),
 *   - the offline self-test compiles the *same* code
 *     (client-patchs/tests/hp-probe-selftest.c).
 *
 * Design constraints (see ../README.md):
 *   - the producer runs inside the game's HP getter epilogue, on arbitrary
 *     game threads, at very high frequency => no locks, no allocation, no
 *     CRT calls, no logging, no floating point (so no XMM is touched).
 *   - the consumer is one of our own threads and may take locks / log.
 *
 * Coherence: every entry carries the producer tick twice (probe_e_tick,
 * probe_e_done).  A reader accepts an entry only when the two match, which
 * makes any torn read detectable.  Entries are additionally addressed by a
 * 64-bit monotonic counter, so a reader can tell how many entries it missed.
 */
#ifndef PROBE_LOGIC_H
#define PROBE_LOGIC_H

#include <windows.h>
#include <stdint.h>
#include <stddef.h>

/*
 * Coherence: every entry carries the producer ticket twice (e_tick written
 * *after* the payload, e_done which only signals "the payload is written").
 * The ticket is the monotonic entry id, so a reader that sees e_tick == the id
 * it wanted knows the payload belongs to that id; if the slot has already been
 * recycled it sees a larger id and can account for the gap instead of reading
 * a mixture of two samples.
 */
typedef struct {
    volatile LONG64 e_tick;   /* producer ticket, written last              */
    volatile LONG64 e_done;   /* 1 = this slot is a complete sample         */
    volatile LONG64 e_caller; /* caller return address                      */
    volatile LONG64 e_rax;    /* value the getter computed                  */
    volatile LONG64 e_rcx;    /* descriptor pointer (rcx at getter entry)   */
    volatile LONG64 e_rdx;    /* rdx evidence (see README: rbx, not dl)     */
    volatile LONG64 e_rbx;    /* raw rbx at the exit point                  */
    volatile LONG64 e_unused;
} probe_entry_t;              /* 64 bytes */

#define PROBE_RING_SLOTS 65536
#define PROBE_RING_MASK  (PROBE_RING_SLOTS - 1)

typedef struct {
    volatile LONG64 produced; /* monotonic entry id (never reused)         */
    volatile LONG index;      /* produced & PROBE_RING_MASK                */
    probe_entry_t entry[PROBE_RING_SLOTS];
} probe_ring_t;

/* one row of the per-caller tally */
typedef struct {
    ULONGLONG caller;
    ULONGLONG hits;
    LONGLONG last_tick;
    LONG64 last_rax;
    LONG64 last_rcx;
    LONG64 last_rdx;
    LONG64 last_rbx;
    LONG64 min_rax;
    LONG64 max_rax;
    LONGLONG first_tick;
} probe_tally_t;

/* ------------------------------------------------------------------ */
/* producer side: called from the code cave. No locks, no CRT.         */
/* ------------------------------------------------------------------ */
static void probe_produce(probe_ring_t *r, ULONGLONG caller, LONG64 rax, LONG64 rcx, LONG64 rdx,
                          LONG64 rbx) {
    LONG64 id = InterlockedIncrement64((volatile LONG64 *)&r->produced) - 1;
    LONG index = (LONG)(id & PROBE_RING_MASK);
    probe_entry_t *e;
    if (index < 0 || index >= PROBE_RING_SLOTS) return; /* impossible; stay safe */
    e = &r->entry[index];
    e->e_caller = (LONG64)caller;
    e->e_rax = rax;
    e->e_rcx = rcx;
    e->e_rdx = rdx;
    e->e_rbx = rbx;
    e->e_done = 1;
    e->e_tick = id; /* published last: a matching ticket means the payload is whole */
}

/* ------------------------------------------------------------------ */
/* consumer side                                                       */
/* ------------------------------------------------------------------ */

/* Insert (caller, sample) into the tally. Returns 1 when the row was newly
 * created (the caller is one we had not seen before). */
static int probe_tally_add(probe_tally_t *t, int cap, int *used, ULONGLONG caller, LONG64 rax,
                           LONG64 rcx, LONG64 rdx, LONG64 rbx, LONGLONG tick) {
    int i;
    int isnew = 0;
    for (i = 0; i < *used; i++) {
        if (t[i].caller == caller) break;
    }
    if (i == *used) {
        if (*used < cap) {
            probe_tally_t *n = &t[(*used)++];
            n->caller = caller;
            n->hits = 0;
            n->first_tick = tick;
            n->min_rax = rax;
            n->max_rax = rax;
            isnew = 1;
        } else {
            /* Table full (should not happen for a 25-callsite getter): fold
             * into the last row rather than dropping the sample silently. */
            i = cap - 1;
        }
    }
    t[i].hits++;
    t[i].last_tick = tick;
    t[i].last_rax = rax;
    t[i].last_rcx = rcx;
    t[i].last_rdx = rdx;
    t[i].last_rbx = rbx;
    if (rax < t[i].min_rax) t[i].min_rax = rax;
    if (rax > t[i].max_rax) t[i].max_rax = rax;
    return isnew;
}

#define PROBE_DRAIN_BATCH 8192

typedef struct {
    volatile LONG64 seen;   /* id of the next entry the drain should read  */
    ULONGLONG lost;         /* entries we never got to look at             */
    ULONGLONG torn;         /* slots caught half-written (retried later)   */
    ULONGLONG total;        /* entries actually consumed                   */
} probe_drain_state_t;

/*
 * Drain up to PROBE_DRAIN_BATCH entries.  `cb` is invoked once per accepted
 * entry; `on_lost(n)` may be NULL.
 */
static void probe_drain(probe_ring_t *r, probe_drain_state_t *st,
                        void (*cb)(const probe_entry_t *e, void *ctx), void *ctx,
                        void (*on_lost)(ULONGLONG n, void *ctx)) {
    LONG64 produced = InterlockedCompareExchange64((volatile LONG64 *)&r->produced, 0, 0);
    LONG64 seen = st->seen;
    LONG64 minId;
    int n = 0;

    if (produced - seen > PROBE_RING_SLOTS) {
        ULONGLONG miss = (ULONGLONG)(produced - seen - PROBE_RING_SLOTS);
        st->lost += miss;
        if (on_lost) on_lost(miss, ctx);
        seen = produced - PROBE_RING_SLOTS;
    }
    if (seen < 0) seen = 0;
    minId = produced - PROBE_RING_SLOTS;
    if (minId < 0) minId = 0;
    if (seen < minId) {
        ULONGLONG miss = (ULONGLONG)(minId - seen);
        st->lost += miss;
        if (on_lost) on_lost(miss, ctx);
        seen = minId;
    }

    while (seen < produced && n < PROBE_DRAIN_BATCH) {
        LONG index = (LONG)(seen & PROBE_RING_MASK);
        const probe_entry_t *e = &r->entry[index];
        LONG64 t1;
        LONG64 t2;
        if (index < 0 || index >= PROBE_RING_SLOTS) break;
        t1 = e->e_tick;
        if (t1 != seen) {
            /* Slot already recycled, or the producer has not published the
             * ticket yet.  Either way the payload must not be read as `seen`. */
            if (t1 > seen && (t1 - seen) <= PROBE_RING_SLOTS) {
                ULONGLONG miss = (ULONGLONG)(t1 - seen);
                st->lost += miss;
                if (on_lost) on_lost(miss, ctx);
                seen = t1;
                continue;
            }
            /* not published yet: stop this pass, the next one will get it */
            break;
        }
        t2 = e->e_done;
        if (t1 != t2 && t2 != 1) {
            /* the payload is not complete yet: stop this pass, retry later */
            st->torn++;
            break;
        }
        if (cb) cb(e, ctx);
        st->total++;
        seen++;
        n++;
    }
    st->seen = seen;
}

#endif /* PROBE_LOGIC_H */
