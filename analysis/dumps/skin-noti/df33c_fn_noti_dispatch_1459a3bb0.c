// fn_noti_dispatch_1459a3bb0

// sub_1459A3BB0
char __fastcall sub_1459A3BB0(_QWORD *a1, unsigned int a2)
{
  __int64 *v4; // rcx
  __int64 v5; // rax
  __int64 v6; // rdx
  __int64 v7; // rcx
  __int64 *v9; // rdx
  __int64 v10; // rax
  __int64 v11; // rcx
  __int64 v12; // rdx
  __int64 v13; // rcx
  _OWORD *v14; // r9
  __int64 v15; // rax
  _BYTE v16[24]; // [rsp+30h] [rbp-58h] BYREF
  __int64 v17; // [rsp+48h] [rbp-40h]
  _OWORD v18[2]; // [rsp+50h] [rbp-38h] BYREF

  v4 = (__int64 *)(a1[4]
                 + 16
                 * ((0x100000001B3LL
                   * (HIBYTE(a2)
                    ^ (0x100000001B3LL
                     * (BYTE2(a2)
                      ^ (0x100000001B3LL
                       * (BYTE1(a2) ^ (0x100000001B3LL * ((unsigned __int8)a2 ^ 0xCBF29CE484222325uLL))))))))
                  & a1[7]));
  v5 = v4[1];
  v6 = a1[2];
  if ( v5 != v6 )
  {
    v7 = *v4;
    if ( a2 == *(_DWORD *)(v5 + 16) )
    {
LABEL_5:
      if ( !v5 )
        v5 = a1[2];
      if ( v5 != v6 )
      {
        (**(void (__fastcall ***)(_QWORD, _QWORD))(v5 + 24))(a2, *(_QWORD *)(*(_QWORD *)(v5 + 24) + 8LL));
        return 1;
      }
    }
    else
    {
      while ( v5 != v7 )
      {
        v5 = *(_QWORD *)(v5 + 8);
        if ( a2 == *(_DWORD *)(v5 + 16) )
          goto LABEL_5;
      }
    }
  }
  v9 = (__int64 *)(a1[12]
                 + 16
                 * (a1[15]
                  & (0x100000001B3LL
                   * (HIBYTE(a2)
                    ^ (0x100000001B3LL
                     * (BYTE2(a2)
                      ^ (0x100000001B3LL
                       * (BYTE1(a2) ^ (0x100000001B3LL * ((unsigned __int8)a2 ^ 0xCBF29CE484222325uLL))))))))));
  v10 = v9[1];
  v11 = a1[10];
  if ( v10 == v11 )
    return 0;
  v12 = *v9;
  if ( a2 != *(_DWORD *)(v10 + 16) )
  {
    while ( v10 != v12 )
    {
      v10 = *(_QWORD *)(v10 + 8);
      if ( a2 == *(_DWORD *)(v10 + 16) )
        goto LABEL_13;
    }
    return 0;
  }
LABEL_13:
  if ( !v10 )
    v10 = a1[10];
  if ( v10 == v11 )
    return 0;
  v17 = 0;
  v13 = *(_QWORD *)(v10 + 24);
  if ( v13 )
  {
    v17 = *(_QWORD *)(v10 + 24);
    v14 = (_OWORD *)(v10 + 32);
    if ( (v13 & 1) != 0 )
    {
      v18[0] = *v14;
      v18[1] = *(_OWORD *)(v10 + 48);
    }
    else
    {
      (*(void (__fastcall **)(_OWORD *, _OWORD *, _QWORD))(v13 & 0xFFFFFFFFFFFFFFFEuLL))(v14, v18, 0);
    }
  }
  if ( !v17 )
  {
    v15 = sub_1401C30B0(v16);
    sub_1401C1200(v15);
  }
  (*(void (__fastcall **)(_OWORD *, __int64))((v17 & 0xFFFFFFFFFFFFFFFEuLL) + 8))(v18, v12);
  if ( (v17 & 1) == 0 && *(_QWORD *)(v17 & 0xFFFFFFFFFFFFFFFEuLL) )
    (*(void (__fastcall **)(_OWORD *, _OWORD *, __int64))(v17 & 0xFFFFFFFFFFFFFFFEuLL))(v18, v18, 2);
  v17 = 0;
  return 1;
}

