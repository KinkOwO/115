// holder_sub_1429E7600_0x1429e7600

void __fastcall sub_1429E7600(__int64 a1, unsigned __int8 a2)
{
  __int64 v3; // rax
  __int64 v4; // r13
  void (__fastcall ***v5)(_QWORD); // r15
  int v6; // r12d
  unsigned __int64 v7; // rbx
  _QWORD *v8; // r8
  _QWORD *v9; // rax
  __int64 v10; // r14
  unsigned __int64 v11; // rcx
  _QWORD **v12; // rcx
  _QWORD *v13; // rcx
  _QWORD *v14; // rdi
  __int64 *v15; // rcx
  __int64 v16; // rax
  __int64 v17; // rdx
  __int64 v18; // rcx
  __int64 v19; // rdi
  __int64 v20; // rcx
  __int64 v21; // rax
  __int64 v22; // rdi
  __int64 v23; // rcx
  __int64 v24; // rax
  __int64 v25; // r8
  __int64 v26; // r8
  __int64 v27; // rax
  unsigned __int64 v28; // rcx
  __int64 v29; // rdi
  __int64 v30; // rcx
  __int64 v31; // rax
  __int64 v32; // rax
  __int64 v33; // rax
  __int64 v34; // rbx
  __int64 v35; // [rsp+70h] [rbp+8h] BYREF
  unsigned __int64 v36; // [rsp+80h] [rbp+18h]
  _QWORD *v37; // [rsp+88h] [rbp+20h]

  if ( !*(_BYTE *)(a1 + 140) )
  {
    if ( a2 )
      *(_BYTE *)(a1 + 140) = 1;
    v3 = sub_1459A9260(qword_14E66C090);
    v4 = v3;
    v5 = 0;
    if ( v3 )
    {
      v6 = 0;
      if ( (int)sub_145DF44B0(v3) > 0 )
      {
        v36 = 0xCBF29CE484222325uLL;
        do
        {
          v7 = sub_145DF2D80(v4, (unsigned int)v6);
          v8 = (_QWORD *)qword_14E638018;
          if ( !qword_14E638018 )
          {
            v9 = (_QWORD *)sub_146E8BA20(136);
            v10 = (__int64)v9;
            v37 = v9;
            if ( v9 )
            {
              *v9 = off_14925EC00;
              sub_14043DB80(v9 + 1);
              sub_14043DB80(v10 + 72);
              v11 = *(_QWORD *)(v10 + 24);
              if ( v11 )
              {
                if ( *(_QWORD *)(v10 + 64) >> 3 <= v11 )
                {
                  v12 = *(_QWORD ***)(v10 + 16);
                  *v12[1] = 0;
                  v13 = *v12;
                  if ( v13 )
                  {
                    do
                    {
                      v14 = (_QWORD *)*v13;
                      sub_146E9F3A0(v13, 24);
                      v13 = v14;
                    }
                    while ( v14 );
                  }
                  **(_QWORD **)(v10 + 16) = *(_QWORD *)(v10 + 16);
                  *(_QWORD *)(*(_QWORD *)(v10 + 16) + 8LL) = *(_QWORD *)(v10 + 16);
                  *(_QWORD *)(v10 + 24) = 0;
                  v35 = *(_QWORD *)(v10 + 16);
                  sub_14015FD80(*(_QWORD *)(v10 + 32), *(_QWORD *)(v10 + 40), &v35);
                }
                else
                {
                  sub_14043F1E0(v10 + 8, **(_QWORD **)(v10 + 16), *(_QWORD *)(v10 + 16));
                }
              }
            }
            else
            {
              v10 = 0;
            }
            qword_14E638018 = v10;
            (**(void (__fastcall ***)(__int64))v10)(v10);
            v8 = (_QWORD *)qword_14E638018;
          }
          if ( v7 )
          {
            if ( v8[3] )
            {
              v15 = (__int64 *)(v8[4]
                              + 16
                              * ((0x100000001B3LL
                                * (HIBYTE(v7)
                                 ^ (0x100000001B3LL
                                  * (BYTE6(v7)
                                   ^ (0x100000001B3LL
                                    * (BYTE5(v7)
                                     ^ (0x100000001B3LL
                                      * (BYTE4(v7)
                                       ^ (0x100000001B3LL
                                        * (BYTE3(v7)
                                         ^ (0x100000001B3LL
                                          * (BYTE2(v7)
                                           ^ (0x100000001B3LL
                                            * (BYTE1(v7) ^ (0x100000001B3LL * (v36 ^ (unsigned __int8)v7))))))))))))))))
                               & v8[7]));
              v16 = v15[1];
              v17 = v8[2];
              if ( v16 != v17 )
              {
                v18 = *v15;
                if ( v7 != *(_QWORD *)(v16 + 16) )
                {
                  while ( v16 != v18 )
                  {
                    v16 = *(_QWORD *)(v16 + 8);
                    if ( v7 == *(_QWORD *)(v16 + 16) )
                      goto LABEL_24;
                  }
                  goto LABEL_41;
                }
LABEL_24:
                if ( !v16 )
                  v16 = v8[2];
                if ( v16 != v17 )
                {
                  v19 = sub_1450BE2E0(v7);
                  v21 = sub_140D66C20(v20);
                  if ( (unsigned __int8)sub_140440AB0(v21, v19) )
                  {
                    v22 = (*(__int64 (__fastcall **)(unsigned __int64))(*(_QWORD *)v7 + 288LL))(v7);
                    v24 = sub_140D66C20(v23);
                    if ( (unsigned __int8)sub_140440AB0(v24, v22)
                      && (v22 == sub_145EFAFB0()
                       || (*(unsigned __int8 (__fastcall **)(__int64))(*(_QWORD *)v22 + 152LL))(v22)
                       || (*(unsigned __int8 (__fastcall **)(__int64))(*(_QWORD *)v22 + 160LL))(v22)) )
                    {
                      LOBYTE(v25) = 1;
                      (*(void (__fastcall **)(unsigned __int64, _QWORD, __int64))(*(_QWORD *)v7 + 1968LL))(v7, a2, v25);
                    }
                    if ( (*(unsigned __int8 (__fastcall **)(unsigned __int64))(*(_QWORD *)v7 + 152LL))(v7) )
                      goto LABEL_40;
                    v27 = *(_QWORD *)v7;
                    v28 = v7;
                  }
                  else
                  {
                    if ( !(unsigned __int8)sub_145B8D6B0(v7) )
                      goto LABEL_41;
                    v29 = (*(__int64 (__fastcall **)(unsigned __int64))(*(_QWORD *)v7 + 1992LL))(v7);
                    v31 = sub_140D66C20(v30);
                    if ( !(unsigned __int8)sub_140440AB0(v31, v29) )
                      goto LABEL_41;
                    if ( (*(unsigned __int8 (__fastcall **)(__int64))(*(_QWORD *)v29 + 152LL))(v29) )
                    {
LABEL_40:
                      LOBYTE(v26) = 1;
                      (*(void (__fastcall **)(unsigned __int64, _QWORD, __int64))(*(_QWORD *)v7 + 1968LL))(v7, a2, v26);
                      goto LABEL_41;
                    }
                    v27 = *(_QWORD *)v29;
                    v28 = v29;
                  }
                  if ( (*(unsigned __int8 (__fastcall **)(unsigned __int64))(v27 + 160))(v28) )
                    goto LABEL_40;
                }
              }
            }
          }
LABEL_41:
          ++v6;
        }
        while ( v6 < (int)sub_145DF44B0(v4) );
      }
    }
    if ( !qword_14E63AE60 )
    {
      v32 = sub_146E8BA20(336);
      v35 = v32;
      if ( v32 )
        v5 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v32);
      qword_14E63AE60 = (__int64)v5;
      (**v5)(v5);
    }
    sub_1447EF2C0(a2);
    v33 = sub_14667C540(qword_14E683C78);
    v34 = v33;
    if ( v33 )
    {
      sub_146A18D10(v33, a2 ^ 1u);
      if ( a2 )
        sub_146A18E50(v34, 0);
    }
  }
}

