; cave_test_entry.asm -- harness side of the trampoline test (ml64, x64, ASCII).
;
; NOTE (ml64 quirk, found the hard way): this assembler rejects a hexadecimal
; displacement inside a memory operand (`[r11+0x68]` -> error A2206).  All
; displacements below are therefore written in decimal.
;
; Drives the copied code cave through a full "virtual call + simulated ret"
; cycle and hands everything the cave did to a C callback:
;
;   1. loads the control block into rax / rdi / rbx and every non-volatile
;      register, plus xmm6 / xmm15;
;   2. jumps into the cave exactly the way the game's patched `ret` does --
;      an absolute jump with a return address on the stack;
;   3. the cave ends with `jmp [tail_slot]`, which the C harness points at
;      cave_test_landing.  The landing pad records the complete register state
;      and calls the C callback (cave_capture_state), which never returns to
;      the cave (it longjmps out).
;
;   void cave_test_enter(unsigned long long *ctrl)
;
;   ctrl[0]      rax before entry (the value the getter "computed")
;   ctrl[1]      rdi before entry  (in the getter this is rcx at entry)
;   ctrl[2]      rbx before entry
;   ctrl[3]      address of the copied cave
;   ctrl[4..10]  rbx / rbp / rsi / r12 / r13 / r14 / r15 to load
;   ctrl[11]     xmm6  (low lane, as a double)
;   ctrl[12]     xmm15 (low lane, as a double)
;   ctrl[13]     destination block for the landing pad (also placed on the
;                stack where the game would find its own return address)
;
; Capture block (14 qwords are written):
;   0 rax, 1 rbx, 2 rbp, 3 rsi, 4 rdi, 5 r12, 6 r13, 7 r14, 8 r15,
;   9 rsp, 10 rcx, 11 rdx, 12 xmm6, 13 xmm15
;
; Only the low 64 lanes of xmm6 / xmm15 are compared; the cave moves whole
; 16-byte registers but the harness only seeds the low lane.

extern cave_capture_state:proc

.code

public cave_test_enter
cave_test_enter proc
    push    rbx
    push    rbp
    push    rsi
    push    rdi
    push    r12
    push    r13
    push    r14
    push    r15
    sub     rsp, 48

    mov     r11, rcx                                  ; ctrl
    mov     rax, [r11 + 104]                          ; ctrl[13] = capture block
    mov     [rsp + 0], rax                            ; where the game has its return address
    mov     rbx, [r11 + 32]                           ; ctrl[4]
    mov     rbp, [r11 + 40]                           ; ctrl[5]
    mov     rsi, [r11 + 48]                           ; ctrl[6]
    mov     r12, [r11 + 56]                           ; ctrl[7]
    mov     r13, [r11 + 64]                           ; ctrl[8]
    mov     r14, [r11 + 72]                           ; ctrl[9]
    mov     r15, [r11 + 80]                           ; ctrl[10]
    movsd   xmm6, qword ptr [r11 + 88]                          ; ctrl[11]
    movsd   xmm15, qword ptr [r11 + 96]                         ; ctrl[12]
    mov     rdx, [r11 + 16]                           ; ctrl[2]
    mov     rdi, [r11 + 8]                            ; ctrl[1]
    mov     rax, [r11 + 0]                            ; ctrl[0]
    mov     r10, [r11 + 24]                           ; ctrl[3] = cave address
    mov     rcx, [r11 + 104]                          ; ctrl[13] -> rcx for the pad
    jmp     r10
cave_test_enter endp

; ---------------------------------------------------------------------------
; Landing pad: the cave's simulated `ret` arrives here with the game's stack
; depth restored (rsp points at the value pushed above) and rax holding the
; getter's return value.
; ---------------------------------------------------------------------------
public cave_test_landing
cave_test_landing proc
    mov     r11, rcx
    test    r11, r11
    jnz     have_out
    mov     r11, [rsp + 8]
have_out:
    mov     [r11 + 0], rax
    mov     [r11 + 8], rbx
    mov     [r11 + 16], rbp
    mov     [r11 + 24], rsi
    mov     [r11 + 32], rdi
    mov     [r11 + 40], r12
    mov     [r11 + 48], r13
    mov     [r11 + 56], r14
    mov     [r11 + 64], r15
    mov     [r11 + 72], rsp
    mov     [r11 + 80], rcx
    mov     [r11 + 88], rdx
    movsd   qword ptr [r11 + 96], xmm6
    movsd   qword ptr [r11 + 104], xmm15
    ; hand control to C; this never returns
    sub     rsp, 40
    call    cave_capture_state
    int     3
cave_test_landing endp

end
