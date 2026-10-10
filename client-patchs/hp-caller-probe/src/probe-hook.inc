; probe-hook.inc -- layout contract for the hp-caller-probe code cave.
; ASCII only, MSVC ml64 (Intel syntax).  Included by src/probe-hook.s.
;
; The DLL places one freshly VirtualAlloc'ed executable block (the "cave"),
; patches the HP getter's exit `ret` (VA 0x147221011) with a 5-byte rel32 jmp
; into it, and fills the two slots marked PATCHED below.  The cave then hands
; the evidence to probe_hp_entry() and behaves like the `ret` it replaced.
;
; The authoritative layout is pinned by the `org` directives in probe-hook.s
; and re-derived from the assembled machine code by tools/cavegen.py, which
; refuses to emit probe-hook-image.h if anything moved.  These equates must
; match that; probe-hook-image.h carries the two offsets that follow the code.

ifndef PROBE_HOOK_INC
PROBE_HOOK_INC equ 1

; ---- cave layout ----------------------------------------------------------
PROBE_OFF_SITEJMP   equ 0     ; +0: E9 <disp32>   PATCHED (points at the caller)
PROBE_OFF_STUB      equ 8     ; +8: FF 25 00000000 <imm64> PATCHED -> log fn
PROBE_STUB_SIZE     equ 14
PROBE_OFF_CODE      equ 22    ; +22: the register discipline starts here
PROBE_TOTAL_SIZE    equ 0x400 ; bytes we reserve for the cave

; ---- ring entry layout (byte offsets inside probe_entry_t) ----------------
; These mirror probe-logic.h's probe_entry_t, which the offline self-test
; checks with offsetof(); the assembly hard-codes the same order for the
; producer-side stores it does not do (the C producer writes them).
PROBE_E_TICK        equ 0     ; qword  producer ticket (published last)
PROBE_E_SEQ         equ 8     ; qword  completeness flag
PROBE_E_CALLER      equ 16    ; qword  caller return address
PROBE_E_RAX         equ 24    ; qword  the value the getter computed
PROBE_E_RCX         equ 32    ; qword  descriptor pointer (rcx at getter entry)
PROBE_E_RDX         equ 40    ; qword  rdx evidence (rbx at the exit point)
PROBE_E_RBX         equ 48    ; qword  raw rbx at the exit point
PROBE_E_SIZE        equ 56

endif
