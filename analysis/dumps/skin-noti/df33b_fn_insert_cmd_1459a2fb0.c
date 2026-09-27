// fn_insert_cmd_1459a2fb0

// sub_1459A2FB0
void __fastcall sub_1459A2FB0(_QWORD *a1, unsigned int a2, __int64 a3, __int64 a4)
{
  __int64 *v8; // rcx
  __int64 v9; // rax
  __int64 v10; // rdx
  __int64 v11; // rcx
  __int64 *v12; // rcx
  __int64 v13; // rax
  __int64 v14; // rdx
  int v15; // eax
  __int64 v16; // rbx
  __int64 v17; // rax
  __int64 v18; // rax
  __int64 v19; // rax
  __int64 v20; // rdx
  __int64 v21; // rcx
  __int64 v22; // r8
  __int64 v23; // r9
  int v24; // esi
  _QWORD *v25; // rdi
  int v26; // ebx
  int v27; // eax
  unsigned __int64 v28; // rdx
  __int64 v29; // rcx
  _QWORD *v30; // rax
  _QWORD *v31; // rbx
  int v32; // [rsp+20h] [rbp-88h]
  int v33; // [rsp+40h] [rbp-68h] BYREF
  _BYTE v34[16]; // [rsp+48h] [rbp-60h] BYREF
  __int64 v35; // [rsp+58h] [rbp-50h]
  _QWORD v36[3]; // [rsp+60h] [rbp-48h] BYREF
  unsigned __int64 v37; // [rsp+78h] [rbp-30h]

  v35 = -2;
  v33 = a2;
  v8 = (__int64 *)(a1[4]
                 + 16
                 * ((0x100000001B3LL
                   * (HIBYTE(a2)
                    ^ (0x100000001B3LL
                     * (BYTE2(a2)
                      ^ (0x100000001B3LL
                       * (BYTE1(a2) ^ (0x100000001B3LL * ((unsigned __int8)a2 ^ 0xCBF29CE484222325uLL))))))))
                  & a1[7]));
  v9 = v8[1];
  v10 = a1[2];
  if ( v9 != v10 )
  {
    v11 = *v8;
    if ( a2 == *(_DWORD *)(v9 + 16) )
    {
LABEL_5:
      if ( !v9 )
        v9 = a1[2];
      if ( v9 != v10 )
        goto LABEL_15;
    }
    else
    {
      while ( v9 != v11 )
      {
        v9 = *(_QWORD *)(v9 + 8);
        if ( a2 == *(_DWORD *)(v9 + 16) )
          goto LABEL_5;
      }
    }
  }
  v12 = (__int64 *)(a1[12]
                  + 16
                  * ((0x100000001B3LL
                    * (HIBYTE(v33)
                     ^ (0x100000001B3LL
                      * (BYTE2(v33)
                       ^ (0x100000001B3LL
                        * (BYTE1(v33) ^ (0x100000001B3LL * ((unsigned __int8)a2 ^ 0xCBF29CE484222325uLL))))))))
                   & a1[15]));
  v13 = v12[1];
  v14 = a1[10];
  if ( v13 == v14 )
    goto LABEL_24;
  v11 = *v12;
  if ( a2 != *(_DWORD *)(v13 + 16) )
  {
    while ( v13 != v11 )
    {
      v13 = *(_QWORD *)(v13 + 8);
      if ( a2 == *(_DWORD *)(v13 + 16) )
        goto LABEL_12;
    }
    goto LABEL_24;
  }
LABEL_12:
  if ( !v13 )
    v13 = a1[10];
  if ( v13 == v14 )
  {
LABEL_24:
    v30 = (_QWORD *)sub_146E8BA20(16);
    v31 = v30;
    if ( v30 )
    {
      *v30 = a3;
      v30[1] = a4;
    }
    else
    {
      v31 = 0;
    }
    *(_QWORD *)(*(_QWORD *)sub_1459A2360(a1 + 1, v34, &v33) + 24LL) = v31;
    return;
  }
LABEL_15:
  v15 = sub_14661EB00(v11);
  if ( v33 >= 0 && v33 < v15 )
  {
    v16 = qword_14EF38F60[v33];
    v17 = sub_146E8C7D0(&unk_14A900470);
    v18 = sub_146E8CF20(v34, v17, v16);
    v19 = sub_14014F430(v18);
    sub_14014C7B0(v36, v19);
    sub_146E8C910(v34);
    v24 = sub_14021A860(v21, v20, v22, v23, v32);
    v25 = v36;
    if ( v37 >= 8 )
      v25 = (_QWORD *)v36[0];
    v26 = sub_146E8C7D0(&unk_14A9004D0);
    v27 = sub_146E8C7D0(&unk_14A900510);
    sub_146E938E0(v24, 0, v27, v26, 36, (__int64)&qword_14E683970, (__int64)v25);
    if ( v37 >= 8 )
    {
      v28 = 2 * v37 + 2;
      v29 = v36[0];
      if ( v28 >= 0x1000 )
      {
        v28 = 2 * v37 + 41;
        v29 = *(_QWORD *)(v36[0] - 8LL);
        if ( (unsigned __int64)(v36[0] - v29 - 8) > 0x1F )
          sub_148AAF304(v29, v28);
      }
      sub_146E9F3A0(v29, v28);
    }
    v36[2] = 0;
    v37 = 7;
    LOWORD(v36[0]) = 0;
  }
}

