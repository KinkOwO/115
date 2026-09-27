// disasm_cmd_dispatch_1459a2d70

0x1459a2d70: push    rbp
0x1459a2d72: push    rsi
0x1459a2d73: push    rdi
0x1459a2d74: sub     rsp, 80h
0x1459a2d7b: mov     [rsp+98h+var_70], 0FFFFFFFFFFFFFFFEh
0x1459a2d84: mov     [rsp+98h+arg_0], rbx
0x1459a2d8c: mov     rax, cs:qword_14DCA9408
0x1459a2d93: xor     rax, rsp
0x1459a2d96: mov     [rsp+98h+var_28], rax
0x1459a2d9b: movzx   edi, r9w
0x1459a2d9f: movzx   ebx, r8b
0x1459a2da3: mov     r11d, edx
0x1459a2da6: mov     r10, rcx
0x1459a2da9: mov     [rsp+98h+var_78], r11d
0x1459a2dae: mov     r9d, edx
0x1459a2db1: shr     r9d, 18h
0x1459a2db5: shr     edx, 10h
0x1459a2db8: mov     eax, r11d
0x1459a2dbb: shr     eax, 8
0x1459a2dbe: movzx   r8d, r11b
0x1459a2dc2: mov     rbp, 0CBF29CE484222325h
0x1459a2dcc: xor     r8, rbp
0x1459a2dcf: mov     rsi, 100000001B3h
0x1459a2dd9: imul    r8, rsi
0x1459a2ddd: movzx   eax, al
0x1459a2de0: xor     r8, rax
0x1459a2de3: imul    r8, rsi
0x1459a2de7: movzx   eax, dl
0x1459a2dea: xor     r8, rax
0x1459a2ded: imul    r8, rsi
0x1459a2df1: xor     r8, r9
0x1459a2df4: imul    r8, rsi
0x1459a2df8: mov     rcx, [rcx+38h]
0x1459a2dfc: and     rcx, r8
0x1459a2dff: shl     rcx, 4
0x1459a2e03: add     rcx, [r10+20h]
0x1459a2e07: mov     rax, [rcx+8]
0x1459a2e0b: mov     r9, [r10+10h]
0x1459a2e0f: cmp     rax, r9
0x1459a2e12: jz      short loc_1459A2E57
0x1459a2e14: mov     rcx, [rcx]
0x1459a2e17: cmp     r11d, [rax+10h]
0x1459a2e1b: jz      short loc_1459A2E2F
0x1459a2e1d: nop     dword ptr [rax]
0x1459a2e20: cmp     rax, rcx
0x1459a2e23: jz      short loc_1459A2E57
0x1459a2e25: mov     rax, [rax+8]
0x1459a2e29: cmp     r11d, [rax+10h]
0x1459a2e2d: jnz     short loc_1459A2E20
0x1459a2e2f: test    rax, rax
0x1459a2e32: cmovz   rax, r9
0x1459a2e36: cmp     rax, r9
0x1459a2e39: jz      short loc_1459A2E57
0x1459a2e3b: mov     r10, [rax+18h]
0x1459a2e3f: mov     r9, [r10+8]
0x1459a2e43: movzx   r8d, di
0x1459a2e47: movzx   edx, bl
0x1459a2e4a: mov     ecx, r11d
0x1459a2e4d: call    qword ptr [r10]
0x1459a2e50: mov     al, 1
0x1459a2e52: jmp     loc_1459A2F74
0x1459a2e57: movzx   edx, r11b
0x1459a2e5b: xor     rdx, rbp
0x1459a2e5e: imul    rdx, rsi
0x1459a2e62: movzx   eax, byte ptr [rsp+98h+var_78+1]
0x1459a2e67: xor     rdx, rax
0x1459a2e6a: imul    rdx, rsi
0x1459a2e6e: movzx   eax, byte ptr [rsp+98h+var_78+2]
0x1459a2e73: xor     rdx, rax
0x1459a2e76: imul    rdx, rsi
