// ebc10caller_sub_146D07A30_0x146d07a30

void __fastcall sub_146D07A30(__int64 a1, __int64 a2)
{
  unsigned int *v2; // rax
  __int64 v3; // r8
  __int64 v4; // rcx
  unsigned int v5; // edi
  __int64 v6; // rcx
  __int64 v7; // rax
  __int64 v8; // r8
  __int64 v9; // rax
  __int64 v10; // rcx
  __int64 v11; // rcx
  unsigned __int64 v12; // rdx
  __int64 v13; // rcx
  __int64 v14; // rax
  __int64 v15; // rdx
  __int64 v16; // rcx
  __int128 v17; // [rsp+28h] [rbp-40h] BYREF
  __int64 v18; // [rsp+38h] [rbp-30h]
  _BYTE v19[8]; // [rsp+40h] [rbp-28h] BYREF
  __int64 v20; // [rsp+48h] [rbp-20h]
  __int64 v21; // [rsp+50h] [rbp-18h]

  if ( *(int *)(a1 + 1440) <= 0 )
  {
    *(_OWORD *)(a1 + 1440) = *(_OWORD *)a2;
    *(_QWORD *)(a1 + 1456) = *(_QWORD *)(a2 + 16);
    *(_DWORD *)(a1 + 1464) = *(_DWORD *)(a2 + 24);
    v2 = *(unsigned int **)(a1 + 664);
    if ( v2 )
      v3 = *v2;
    else
      v3 = 0xFFFFFFFFLL;
    if ( sub_146D06E30(a1, *(unsigned int *)(a1 + 640), v3) )
    {
      sub_146EA4750(v19, &qword_14EF2CA98);
      v5 = 0;
      v7 = sub_140764510(v6);
      sub_1444EBC10(v7, &v17, 8);
      if ( (__int64)(*((_QWORD *)&v17 + 1) - v17) >> 2 )
        v5 = *(_DWORD *)v17;
      if ( !v20 || (v9 = v21, !*(_DWORD *)(v20 + 8)) )
        v9 = 0;
      v10 = v9 - 48;
      if ( !v9 )
        v10 = 0;
      sub_145BF28B0(v10, v5, v8);
      v11 = v17;
      if ( (_QWORD)v17 )
      {
        v12 = 4 * ((v18 - (__int64)v17) >> 2);
        if ( v12 >= 0x1000 )
        {
          v12 += 39LL;
          v11 = *(_QWORD *)(v17 - 8);
          if ( (unsigned __int64)(v17 - v11 - 8) > 0x1F )
            sub_148AAF304(v11, v12);
        }
        sub_146E9F3A0(v11, v12);
        v17 = 0;
        v18 = 0;
      }
      v13 = v20;
      if ( v20 )
      {
        if ( _InterlockedExchangeAdd((volatile signed __int32 *)(v20 + 12), 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(__int64))(*(_QWORD *)v13 + 8LL))(v13);
      }
    }
    else
    {
      v14 = sub_146D74000(v4);
      sub_146D746E0(v14, 2261);
      sub_146D75AF0(v16, v15);
    }
  }
}

