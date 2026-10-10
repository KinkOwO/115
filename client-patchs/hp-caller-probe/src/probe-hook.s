; probe-hook.s -- v2 trampoline for the hp-caller-probe code cave.
;
; ASCII only, MSVC ml64 (Intel syntax, x64).  No macros and no includes: the
; generator (tools/cavegen.py) parses this file as TEXT and cross-checks the
; equates below against the really-assembled machine code, so every number here
; is a contract, not a convenience.
;
; ===========================================================================
; WHY v1 CRASHED (do not "simplify" this back)
; ===========================================================================
; v1 saved xmm6..xmm15 with MOVAPS to [rsp-128 .. rsp+16].  Two independent
; mistakes, both fatal:
;
;   (a) ALIGNMENT.  When the cave runs, rsp is 8 (mod 16): the getter's `ret`
;       is about to pop a return address, so rsp points at that slot and
;       "before the pop" means rsp % 16 == 8.  After the 8 GPR pushes rsp is
;       *still* 8 (mod 16).  MOVAPS demands a 16-byte-aligned address, so the
;       four stores at rsp-112 / rsp-80 / rsp-48 / rsp-16 are 8 (mod 16) and
;       fault.  The first instruction of the trampoline proper --
;       `movaps [rsp-0x80],xmm6` at cave+0x22 -- is exactly where both the
;       offline self-test and the live client died (0xc0000005).
;
;   (b) A HAND-COUNTED OFFSET THAT DID NOT SURVIVE ITS OWN `sub rsp`.  v1 read
;       "saved rax" from [rsp+0x18] *after* `sub rsp,32`; the real saved rax
;       was at [rsp+0x40] by then, so it recorded the saved rbx instead.  The
;       old cavegen.py check matched the literal operand text
;       "MOV RAX, [RSP+Reg(0)+0x40]" anywhere in the dump, so a wrong *reader*
;       still passed verification.
;
; v2 therefore:
;   * uses MOVUPS (unaligned 16-byte move) for every xmm save/restore --
;     same cost as MOVAPS on every CPU that runs this client, alignment-proof;
;   * keeps ONE frame base in rbp, captured once, used for the save *and* the
;     matching restore of every field -- no offset is ever re-derived;
;   * calls a pure leaf C function on the return path (probe_hp_entry does
;     nothing but a couple of stores and one `lock inc`), so there is no
;     shadow-space or callee-alignment trap either.
;
; ===========================================================================
; THE PATCH SITE
; ===========================================================================
; The HP getter at VA 0x147220E40 ends at VA 0x147221011 with
;     0x14722100E  5F            pop rdi
;     0x14722100F  5E            pop rsi
;     0x147221010  5B            pop rbx
;     0x147221011  C3            ret        <-- overwritten with E9 <rel32>
;     0x147221012  CC * 14       inter-function alignment filler
;     0x147221020  next function
; The plugin rewrites the five verified bytes `C3 CC CC CC CC` with a 5-byte
; rel32 jmp; nothing else in the image is touched.
;
; ===========================================================================
; WHAT THE CAVE DOES
; ===========================================================================
;   1. saves every general-purpose register, the flags, and xmm0..xmm15;
;   2. records {[rsp] = caller return address, rax = computed value} into the
;      probe's forward-only ring (two 8-byte stores + one `lock inc`);
;   3. restores every register and the flags bit-for-bit, then executes the
;      `ret` the patch replaced, with the caller's return address back at [rsp]
;      and rsp back at its entry value.
;
; ===========================================================================
; STACK / FRAME CONTRACT
; ===========================================================================
; At cave entry (the instant the getter's `ret` would have popped):
;     rsp % 16 == 8        (rsp points at the caller's return address)
;     [rsp]    == caller's return address
; After `push rbp` the helper rsp is 0 (mod 16), the state a Windows x64 caller
; must present at a `call`.  Every field is addressed off the frame base in rbp,
; captured after that push, so no field offset can drift when anything else in
; the file changes length.
;
; SAVE/RESTORE MATRIX (exactly what the offline self-test asserts):
;   rbp                   pushed, restored by the final `pop rbp`
;   rsp                   restored with `mov rsp, rbp` before the `ret`
;   rax rcx rdx rbx rsi rdi r8..r15    saved to the frame, restored
;   rflags                pushfq/popfq around the call
;   xmm0..xmm15           MOVUPS to/from the frame
;   anything else         never written by this code

; the C side of the probe: a leaf function in the plugin DLL (see
; src/hp-caller-probe.c).  It only reads the frame it is handed.
extern probe_hp_entry:proc

; ---------------------------------------------------------------------------
; layout equates -- ALL definitions must precede the segment block
; ---------------------------------------------------------------------------
OA_SITEJMP      equ 0        ; +0x00  E9 <rel32>   (patched by the plugin)
OA_STUB         equ 8        ; +0x08  FF 25 <disp32> <imm64>
OA_STUB_TARGET  equ 14       ; +0x0E  the imm64 the stub jumps through
OA_CODE         equ 22       ; +0x16  the trampoline proper (site-jump target)
OA_POOL         equ 0x200    ; +0x200 RIP-relative data pool (relocated)
OA_TOTAL        equ 0x400    ; whole image, matched by PROBE_TOTAL_SIZE

; frame size / field map.  cavegen.py asserts that the assembled code really
; uses these displacements, so a typo fails the build instead of the client.
FRM_SIZE        equ 640      ; 0x280; rsp is 0 (mod 16) after the `sub`
FRM_FLAGS       equ 8
FRM_RAX         equ 16
FRM_RCX         equ 24
FRM_RDX         equ 32
FRM_RBX         equ 40
FRM_RSI         equ 48
FRM_RDI         equ 56
FRM_R8          equ 64
FRM_R9          equ 72
FRM_R10         equ 80
FRM_R11         equ 88
FRM_R12         equ 96
FRM_R13         equ 104
FRM_R14         equ 112
FRM_R15         equ 120
FRM_XMM0        equ 128
FRM_XMM_STEP    equ 16
FRM_XMM15       equ 368      ; 128 + 15*16
FRM_LIMIT       equ 384      ; first byte past the last saved field
OA_ASM_FRAME    equ 0        ; qword: frame base, published for the C probe

    .code

; ---------------------------------------------------------------------------
; region 1: bytes +0x00 .. +0x15 (site jump, dead jump stub, 3-byte filler).
;
; ml64 emits all `db` data of a fragment BEFORE the instructions of the same
; fragment and only then applies `org` inside it (verified with a control
; experiment, see tools/cavegen.py): the first `db` block therefore really
; starts at +0x00, and `org OA_CODE` inside it pushes the location counter to
; +0x16, which is where the code fragment below begins.  Keep the head a pure
; data block -- mixing instructions into it moves everything.
;
; The stub at +0x08 is dead code kept on purpose: PROBE_OFF_STUB /
; PROBE_OFF_STUB_TARGET are part of the documented cave head in
; src/probe-hook-meta.h, and a future version may want a call-back-through-slot
; again.  Nothing jumps to +0x08 (the site jump at +0x00 targets +0x16).
; ---------------------------------------------------------------------------
a_head  db 0E9h                                          ; +0x00  E9 <rel32>
        db 000h, 000h, 000h, 000h                        ; +0x01  patched target
        db 0FFh, 025h                                    ; +0x05  FF 25 <disp32>
        db 000h, 000h, 000h, 000h                        ; +0x07
        db 000h, 000h, 000h, 000h, 000h, 000h, 000h, 000h ; +0x0B imm64 (zero)
        db 000h, 000h, 000h                              ; +0x13  3-byte filler
        org OA_CODE                                      ; +0x16  trampoline

; ---------------------------------------------------------------------------
; region 2: the trampoline proper (+0x16 .. )
; ---------------------------------------------------------------------------
    push    rbp
    mov     rbp, rsp
    sub     rsp, FRM_SIZE
    mov     qword ptr [rbp - OA_ASM_FRAME], rbp

    ; --- flags -------------------------------------------------------------
    ; r11 is still scratch here: it is saved a few lines further down.
    pushfq
    pop     r11
    mov     qword ptr [rbp - FRM_FLAGS], r11

    ; --- general purpose registers (rax FIRST: it is the value we came for)
    mov     qword ptr [rbp - FRM_RAX], rax
    mov     qword ptr [rbp - FRM_RCX], rcx
    mov     qword ptr [rbp - FRM_RDX], rdx
    mov     qword ptr [rbp - FRM_RBX], rbx
    mov     qword ptr [rbp - FRM_RSI], rsi
    mov     qword ptr [rbp - FRM_RDI], rdi
    mov     qword ptr [rbp - FRM_R8],  r8
    mov     qword ptr [rbp - FRM_R9],  r9
    mov     qword ptr [rbp - FRM_R10], r10
    mov     qword ptr [rbp - FRM_R11], r11
    mov     qword ptr [rbp - FRM_R12], r12
    mov     qword ptr [rbp - FRM_R13], r13
    mov     qword ptr [rbp - FRM_R14], r14
    mov     qword ptr [rbp - FRM_R15], r15

    ; --- xmm0..xmm15: MOVUPS, never MOVAPS (see the header comment) -------
    movups  xmmword ptr [rbp - FRM_XMM0], xmm0
    movups  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*1], xmm1
    movups  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*2], xmm2
    movups  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*3], xmm3
    movups  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*4], xmm4
    movups  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*5], xmm5
    movups  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*6], xmm6
    movups  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*7], xmm7
    movups  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*8], xmm8
    movups  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*9], xmm9
    movups  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*10], xmm10
    movups  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*11], xmm11
    movups  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*12], xmm12
    movups  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*13], xmm13
    movups  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*14], xmm14
    movups  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*15], xmm15

    ; --- evidence ---------------------------------------------------------
    ; rcx = frame base.  Its field map is emitted by cavegen.py from this same
    ; equate list, so the C side cannot disagree about where [rsp] (the
    ; caller's return address) and rax live.
    lea     rcx, [rbp - OA_ASM_FRAME]
    call    probe_hp_entry

    ; --- restore (mirror of the save, off the same frame base) ------------
    mov     rsp, rbp
    mov     qword ptr [rsp + 16], 0          ; clear the dead asm_frame slot
    popfq
    mov     rax, qword ptr [rbp - FRM_RAX]
    mov     rcx, qword ptr [rbp - FRM_RCX]
    mov     rdx, qword ptr [rbp - FRM_RDX]
    mov     rbx, qword ptr [rbp - FRM_RBX]
    mov     rsi, qword ptr [rbp - FRM_RSI]
    mov     rdi, qword ptr [rbp - FRM_RDI]
    mov     r8,  qword ptr [rbp - FRM_R8]
    mov     r9,  qword ptr [rbp - FRM_R9]
    mov     r10, qword ptr [rbp - FRM_R10]
    mov     r11, qword ptr [rbp - FRM_R11]
    mov     r12, qword ptr [rbp - FRM_R12]
    mov     r13, qword ptr [rbp - FRM_R13]
    mov     r14, qword ptr [rbp - FRM_R14]
    mov     r15, qword ptr [rbp - FRM_R15]
    movups  xmm0,  xmmword ptr [rbp - FRM_XMM0]
    movups  xmm1,  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*1]
    movups  xmm2,  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*2]
    movups  xmm3,  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*3]
    movups  xmm4,  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*4]
    movups  xmm5,  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*5]
    movups  xmm6,  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*6]
    movups  xmm7,  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*7]
    movups  xmm8,  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*8]
    movups  xmm9,  xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*9]
    movups  xmm10, xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*10]
    movups  xmm11, xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*11]
    movups  xmm12, xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*12]
    movups  xmm13, xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*13]
    movups  xmm14, xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*14]
    movups  xmm15, xmmword ptr [rbp - FRM_XMM0 - FRM_XMM_STEP*15]
    pop     rbp

    ; --- behave exactly like the patched-out `ret` ------------------------
    ; rsp is back to its entry value; push [rsp] (the caller's return address)
    ; and ret.  The caller sees a stack bit-identical to the unpatched getter's.
    push    qword ptr [rbp - 1]
    ret

; ---------------------------------------------------------------------------
; region 3: pool (+0x200 .. +0x21F), RIP-relative data relocated at install
; ---------------------------------------------------------------------------
    .code
b_pool  db 000h, 000h, 000h, 000h, 000h, 000h, 000h, 000h   ; +0x200 ring slot
        db 000h, 000h, 000h, 000h, 000h, 000h, 000h, 000h   ; +0x208 table slot
        db 000h, 000h, 000h, 000h, 000h, 000h, 000h, 000h   ; +0x210 spare
        db 000h, 000h, 000h, 000h, 000h, 000h, 000h, 000h   ; +0x218 spare
                                                            ; +0x220 end of pool
    end
