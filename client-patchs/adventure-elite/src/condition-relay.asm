option casemap:none
EXTERN EliteConditionDecision:PROC
PUBLIC EliteConditionRelay
.code
EliteConditionRelay PROC FRAME
    push rbp
    .pushreg rbp
    sub rsp,0D0h
    .allocstack 0D0h
    lea rbp,[rsp+0D0h]
    .setframe rbp,0D0h
    .endprolog
    mov [rsp+20h],rax
    mov [rsp+28h],rcx
    mov [rsp+30h],rdx
    mov [rsp+38h],r8
    mov [rsp+40h],r9
    mov [rsp+48h],r10
    mov [rsp+50h],r11
    pushfq
    pop QWORD PTR [rsp+58h]
    movdqu [rsp+60h],xmm0
    movdqu [rsp+70h],xmm1
    movdqu [rsp+80h],xmm2
    movdqu [rsp+90h],xmm3
    movdqu [rsp+0A0h],xmm4
    movdqu [rsp+0B0h],xmm5
    mov rcx,rax
    mov rdx,[rbp+8]
    call EliteConditionDecision
    mov [rsp+0C0h],al
    movdqu xmm0,[rsp+60h]
    movdqu xmm1,[rsp+70h]
    movdqu xmm2,[rsp+80h]
    movdqu xmm3,[rsp+90h]
    movdqu xmm4,[rsp+0A0h]
    movdqu xmm5,[rsp+0B0h]
    mov rax,[rsp+20h]
    mov rcx,[rsp+28h]
    mov rdx,[rsp+30h]
    mov r8,[rsp+38h]
    mov r9,[rsp+40h]
    mov r10,[rsp+48h]
    mov r11,[rsp+50h]
    push QWORD PTR [rsp+58h]
    popfq
    cmp BYTE PTR [rsp+0C0h],0
    lea rsp,[rbp]
    pop rbp
    ret
EliteConditionRelay ENDP
END
