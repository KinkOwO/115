option casemap:none
EXTERN EliteConditionRelay:PROC
PUBLIC EliteRelayFixture
.code
EliteRelayFixture PROC FRAME
    push rbx
    .pushreg rbx
    sub rsp,20h
    .allocstack 20h
    .endprolog
    mov rbx,rdx
    mov rax,rcx
    mov rcx,11h
    mov rdx,22h
    mov r8,33h
    mov r9,44h
    mov r10,55h
    mov r11,66h
    pcmpeqd xmm0,xmm0
    pcmpeqd xmm1,xmm1
    pcmpeqd xmm2,xmm2
    pcmpeqd xmm3,xmm3
    pcmpeqd xmm4,xmm4
    pcmpeqd xmm5,xmm5
    call EliteConditionRelay
    mov [rbx],rax
    mov [rbx+8],rcx
    mov [rbx+10h],rdx
    mov [rbx+18h],r8
    mov [rbx+20h],r9
    mov [rbx+28h],r10
    mov [rbx+30h],r11
    pushfq
    pop QWORD PTR [rbx+38h]
    movdqu [rbx+40h],xmm0
    movdqu [rbx+50h],xmm1
    movdqu [rbx+60h],xmm2
    movdqu [rbx+70h],xmm3
    movdqu [rbx+80h],xmm4
    movdqu [rbx+90h],xmm5
    add rsp,20h
    pop rbx
    ret
EliteRelayFixture ENDP
END
