// fn_cmd_dispatch_1459a2d70

// sub_1459A2D70
char __fastcall sub_1459A2D70(_QWORD *a1, unsigned int a2, unsigned __int8 a3, unsigned __int16 a4)
{
  __int64 *v8; // rcx
  __int64 v9; // rax
  __int64 v10; // r9
  __int64 v11; // rcx
  __int64 *v13; // rdx
  __int64 v14; // rax
  __int64 v15; // rcx
  __int64 v16; // rdx
  __int64 v17; // rcx
  _OWORD *v18; // r9
  __int64 v19; // rax
  _BYTE v20[24]; // [rsp+30h] [rbp-68h] BYREF
  __int64 v21; // [rsp+48h] [rbp-50h]
  _OWORD v22[2]; // [rsp+50h] [rbp-48h] BYREF

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
      {
        (**(void (__fastcall ***)(_QWORD, _QWORD, _QWORD, _QWORD, unsigned int, __int64))(v9 + 24))(
          a2,
          a3,
          a4,
          *(_QWORD *)(*(_QWORD *)(v9 + 24) + 8LL),
          a2,
          -2);
        return 1;
      }
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
  v13 = (__int64 *)(a1[12]
                  + 16
                  * (a1[15]
                   & (0x100000001B3LL
                    * (HIBYTE(a2)
                     ^ (0x100000001B3LL
                      * (BYTE2(a2)
                       ^ (0x100000001B3LL
                        * (BYTE1(a2) ^ (0x100000001B3LL * ((unsigned __int8)a2 ^ 0xCBF29CE484222325uLL))))))))));
  v14 = v13[1];
  v15 = a1[10];
  if ( v14 == v15 )
    return 0;
  v16 = *v13;
  if ( a2 != *(_DWORD *)(v14 + 16) )
  {
    while ( v14 != v16 )
    {
      v14 = *(_QWORD *)(v14 + 8);
      if ( a2 == *(_DWORD *)(v14 + 16) )
        goto LABEL_13;
    }
    return 0;
  }
LABEL_13:
  if ( !v14 )
    v14 = a1[10];
  if ( v14 == v15 )
    return 0;
  v21 = 0;
  v17 = *(_QWORD *)(v14 + 24);
  if ( v17 )
  {
    v21 = *(_QWORD *)(v14 + 24);
    v18 = (_OWORD *)(v14 + 32);
    if ( (v17 & 1) != 0 )
    {
      v22[0] = *v18;
      v22[1] = *(_OWORD *)(v14 + 48);
    }
    else
    {
      (*(void (__fastcall **)(_OWORD *, _OWORD *, _QWORD))(v17 & 0xFFFFFFFFFFFFFFFEuLL))(v18, v22, 0);
    }
  }
  if ( !v21 )
  {
    v19 = sub_1401C30B0(v20);
    sub_1401C1200(v19);
  }
  (*(void (__fastcall **)(_OWORD *, _QWORD, _QWORD))((v21 & 0xFFFFFFFFFFFFFFFEuLL) + 8))(v22, a3, a4);
  if ( (v21 & 1) == 0 && *(_QWORD *)(v21 & 0xFFFFFFFFFFFFFFFEuLL) )
    (*(void (__fastcall **)(_OWORD *, _OWORD *, __int64))(v21 & 0xFFFFFFFFFFFFFFFEuLL))(v22, v22, 2);
  v21 = 0;
  return 1;
}

