// event2345_sub_1413C58F0

char __fastcall sub_1413C58F0(__int64 a1, __int64 a2)
{
  _QWORD *v4; // rdi
  __int64 v5; // rdx
  char v6; // bl
  char v7; // cl
  __int64 v8; // r9
  __int64 v9; // rcx
  __int64 v10; // rax
  __int64 v11; // rcx
  __int64 v12; // rax
  __int64 v13; // rcx
  __int64 v14; // rax
  __int128 v16; // [rsp+30h] [rbp-11h] BYREF
  __int128 v17; // [rsp+40h] [rbp-1h] BYREF
  __int128 v18; // [rsp+50h] [rbp+Fh] BYREF
  _QWORD v19[3]; // [rsp+60h] [rbp+1Fh] BYREF
  int v20; // [rsp+78h] [rbp+37h]
  int v21; // [rsp+7Ch] [rbp+3Bh]
  __int64 v22; // [rsp+80h] [rbp+3Fh]
  char v23; // [rsp+88h] [rbp+47h]
  char v24; // [rsp+89h] [rbp+48h]
  char v25; // [rsp+8Ah] [rbp+49h]
  unsigned __int8 v26; // [rsp+C0h] [rbp+7Fh]

  if ( !qword_14E683C78 || !a2 || !*(_BYTE *)(a1 + 2268) )
    return 0;
  v4 = (_QWORD *)(a1 + 2304);
  *(_QWORD *)(a1 + 2328) = -1;
  *(_QWORD *)(a1 + 2336) = 0;
  *(_WORD *)(a1 + 2344) = 0;
  *(_BYTE *)(a1 + 2346) = 1;
  *(_QWORD *)(a1 + 2312) = *(_QWORD *)(a1 + 2304);
  sub_1402506B0(v19, a2);
  v20 = *(_DWORD *)(a2 + 24);
  v21 = *(_DWORD *)(a2 + 28);
  v22 = *(_QWORD *)(a2 + 32);
  v23 = *(_BYTE *)(a2 + 40);
  v24 = *(_BYTE *)(a2 + 41);
  v25 = *(_BYTE *)(a2 + 42);
  v5 = v22;
  if ( v22 )
  {
    if ( v4 != v19 )
    {
      sub_1402BB950(v4, v19[0], v19[1], v26);
      v5 = v22;
    }
    *(_DWORD *)(a1 + 2328) = v20;
    *(_DWORD *)(a1 + 2332) = v21;
    *(_BYTE *)(a1 + 2344) = v23;
    v7 = v24;
    *(_BYTE *)(a1 + 2345) = v24;
    *(_QWORD *)(a1 + 2336) = v5;
    *(_BYTE *)(a1 + 2346) = v25;
    if ( v7 )
      sub_1413C5BE0(a1);
    else
      sub_1413C6B80(a1);
    v9 = *(_QWORD *)(a1 + 1888);
    if ( v9 )
    {
      v16 = 0;
      v10 = *(_QWORD *)(a1 + 1896);
      if ( v10 )
      {
        _InterlockedIncrement((volatile signed __int32 *)(v10 + 8));
        v9 = *(_QWORD *)(a1 + 1888);
        v10 = *(_QWORD *)(a1 + 1896);
      }
      *(_QWORD *)&v16 = v9;
      *((_QWORD *)&v16 + 1) = v10;
      LOBYTE(v8) = 1;
      sub_14674FF70(a1, 0, &v16, v8);
    }
    v11 = *(_QWORD *)(a1 + 1856);
    if ( v11 )
    {
      v17 = 0;
      v12 = *(_QWORD *)(a1 + 1864);
      if ( v12 )
      {
        _InterlockedIncrement((volatile signed __int32 *)(v12 + 8));
        v11 = *(_QWORD *)(a1 + 1856);
        v12 = *(_QWORD *)(a1 + 1864);
      }
      *(_QWORD *)&v17 = v11;
      *((_QWORD *)&v17 + 1) = v12;
      sub_14674FF70(a1, 1, &v17, 0);
    }
    v13 = *(_QWORD *)(a1 + 1872);
    if ( v13 )
    {
      v18 = 0;
      v14 = *(_QWORD *)(a1 + 1880);
      if ( v14 )
      {
        _InterlockedIncrement((volatile signed __int32 *)(v14 + 8));
        v13 = *(_QWORD *)(a1 + 1872);
        v14 = *(_QWORD *)(a1 + 1880);
      }
      *(_QWORD *)&v18 = v13;
      *((_QWORD *)&v18 + 1) = v14;
      sub_14674FF70(a1, 2, &v18, 0);
    }
    v6 = 1;
  }
  else
  {
    v6 = 0;
  }
  sub_14017AC10(v19);
  return v6;
}

