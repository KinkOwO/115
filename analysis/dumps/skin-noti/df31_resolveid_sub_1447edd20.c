// resolveid_sub_1447edd20

__int64 __fastcall sub_1447EDD20(__int64 a1, signed int a2, __int64 a3)
{
  __int64 **v4; // rdi
  __int64 *v5; // rdx
  __int64 *v6; // rcx
  __int64 *v7; // rax
  __int64 v8; // rax
  __int64 v9; // rdx
  char v10; // bp
  __int64 *v11; // rsi
  __int64 *v12; // rax
  __int64 *v13; // rcx
  __int64 v14; // rax
  __int64 *v15; // rcx
  __int128 v17; // [rsp+28h] [rbp-30h]
  __int128 v18; // [rsp+40h] [rbp-18h] BYREF

  v4 = (__int64 **)(a1 + 320);
  v5 = *(__int64 **)(a1 + 320);
  v6 = (__int64 *)v5[1];
  v7 = v5;
  while ( !*((_BYTE *)v6 + 25) )
  {
    if ( *((_DWORD *)v6 + 7) >= a2 )
    {
      v7 = v6;
      v6 = (__int64 *)*v6;
    }
    else
    {
      v6 = (__int64 *)v6[2];
    }
  }
  if ( *((_BYTE *)v7 + 25) || a2 < *((_DWORD *)v7 + 7) || v7 == v5 )
  {
    LOBYTE(a3) = 1;
    v8 = sub_140283D60(qword_14E683BF8, (unsigned int)a2, a3);
    v10 = 0;
    if ( v8 )
      v10 = *(_BYTE *)(v8 + 625);
    v11 = *v4;
    v12 = (__int64 *)(*v4)[1];
    *(_QWORD *)&v17 = v12;
    DWORD2(v17) = 0;
    v13 = *v4;
    while ( !*((_BYTE *)v12 + 25) )
    {
      *(_QWORD *)&v17 = v12;
      if ( *((_DWORD *)v12 + 7) >= a2 )
      {
        DWORD2(v17) = 1;
        v13 = v12;
        v12 = (__int64 *)*v12;
      }
      else
      {
        DWORD2(v17) = 0;
        v12 = (__int64 *)v12[2];
      }
    }
    v18 = v17;
    if ( *((_BYTE *)v13 + 25) || a2 < *((_DWORD *)v13 + 7) )
    {
      if ( v4[1] == (__int64 *)0x666666666666666LL )
        sub_14014F360(v13, v9);
      v14 = sub_146E8BA20(40);
      *(_DWORD *)(v14 + 28) = a2;
      *(_BYTE *)(v14 + 32) = v10;
      *(_QWORD *)v14 = v11;
      *(_QWORD *)(v14 + 8) = v11;
      *(_QWORD *)(v14 + 16) = v11;
      *(_WORD *)(v14 + 24) = 0;
      sub_14014F0E0(v4, &v18, v14);
    }
    v15 = (__int64 *)(*v4)[1];
    v7 = *v4;
    while ( !*((_BYTE *)v15 + 25) )
    {
      if ( *((_DWORD *)v15 + 7) >= a2 )
      {
        v7 = v15;
        v15 = (__int64 *)*v15;
      }
      else
      {
        v15 = (__int64 *)v15[2];
      }
    }
    if ( *((_BYTE *)v7 + 25) || a2 < *((_DWORD *)v7 + 7) )
      v7 = *v4;
  }
  return *((unsigned __int8 *)v7 + 32);
}

