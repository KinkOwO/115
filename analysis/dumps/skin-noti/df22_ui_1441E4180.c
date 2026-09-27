// ui_1441E4180

__int64 __fastcall sub_1441E4180(__int64 a1)
{
  __int128 *v2; // rbp
  __int64 v3; // rcx
  __int64 v4; // rax
  void (__fastcall ***v5)(_QWORD); // rcx
  __int64 v6; // rax
  __int64 v7; // rdx
  _OWORD *v8; // r8
  __int64 *v9; // rbx
  __int64 v10; // rax
  int v11; // ecx
  int v13; // eax
  __int64 **v14; // rax
  __int64 *i; // rax
  __int64 *j; // rcx
  __int64 v17; // rdx
  __int64 v18; // rbx
  __int64 v19; // rcx
  __int64 v20; // rcx
  __int64 *v21; // rbx
  __int64 *v22; // rcx
  _QWORD v24[2]; // [rsp+28h] [rbp-30h] BYREF
  __int128 v25; // [rsp+38h] [rbp-20h]
  __int64 v26; // [rsp+60h] [rbp+8h]

  *(_QWORD *)(a1 + 24) = *(_QWORD *)(a1 + 16);
  v2 = 0;
  v3 = qword_14E638F28;
  if ( !qword_14E638F28 )
  {
    v4 = sub_146E8BA20(1472);
    if ( v4 )
      v5 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v4);
    else
      v5 = 0;
    qword_14E638F28 = (__int64)v5;
    (**v5)(v5);
    v3 = qword_14E638F28;
  }
  v6 = sub_1444EBDF0(v3, 1);
  sub_1441B8B20(v24, v6);
  v9 = *(__int64 **)v24[0];
  while ( v9 != (__int64 *)v24[0] )
  {
    v10 = sub_1444EBAB0(*((_DWORD *)v9 + 7), v7, (__int64)v8);
    if ( v10 && *(_DWORD *)(v10 + 8) == 1 )
    {
      v11 = *(_DWORD *)(a1 + 1448);
      if ( *(_DWORD *)(v10 + 12) == 3 ? v11 == 1 : v11 == 0 )
      {
        v13 = *((_DWORD *)v9 + 7);
        if ( v13 == 30000 || v13 == 100000 )
        {
          v2 = (__int128 *)(v9 + 4);
        }
        else
        {
          v8 = v9 + 4;
          v7 = *(_QWORD *)(a1 + 24);
          if ( v7 == *(_QWORD *)(a1 + 32) )
          {
            sub_1405EA310(a1 + 16, v7, v8);
          }
          else
          {
            *(_OWORD *)v7 = *v8;
            *(_QWORD *)(v7 + 16) = v9[6];
            *(_QWORD *)(a1 + 24) += 24LL;
          }
        }
      }
    }
    v14 = (__int64 **)v9[2];
    if ( *((_BYTE *)v14 + 25) )
    {
      for ( i = (__int64 *)v9[1]; !*((_BYTE *)i + 25); i = (__int64 *)i[1] )
      {
        if ( v9 != (__int64 *)i[2] )
          break;
        v9 = i;
      }
      v9 = i;
    }
    else
    {
      v9 = (__int64 *)v9[2];
      for ( j = *v14; !*((_BYTE *)j + 25); j = (__int64 *)*j )
        v9 = j;
    }
  }
  sub_141FD8CD0(*(_QWORD *)(a1 + 16), *(_QWORD *)(a1 + 24), (*(_QWORD *)(a1 + 24) - *(_QWORD *)(a1 + 16)) / 24LL, 1);
  if ( v2 )
  {
    v18 = *(_QWORD *)(a1 + 16);
    v19 = *(_QWORD *)(a1 + 24);
    if ( v19 == *(_QWORD *)(a1 + 32) )
    {
      sub_1405EA310(a1 + 16, *(_QWORD *)(a1 + 16), v2);
    }
    else if ( v18 == v19 )
    {
      *(_OWORD *)v19 = *v2;
      *(_QWORD *)(v19 + 16) = *((_QWORD *)v2 + 2);
      *(_QWORD *)(a1 + 24) += 24LL;
    }
    else
    {
      v25 = *v2;
      v26 = *((_QWORD *)v2 + 2);
      *(_OWORD *)v19 = *(_OWORD *)(v19 - 24);
      *(_QWORD *)(v19 + 16) = *(_QWORD *)(v19 - 24 + 16);
      *(_QWORD *)(a1 + 24) += 24LL;
      sub_148AA1E60(v18 + 24, v18, v19 - 24 - v18);
      *(_OWORD *)v18 = v25;
      *(_QWORD *)(v18 + 16) = v26;
    }
  }
  LOBYTE(v17) = *(_QWORD *)(a1 + 16) == *(_QWORD *)(a1 + 24);
  (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 1432) + 16LL))(*(_QWORD *)(a1 + 1432), v17);
  v20 = v24[0];
  v21 = *(__int64 **)(v24[0] + 8LL);
  if ( !*((_BYTE *)v21 + 25) )
  {
    do
    {
      sub_1401DBB80(v24, v24, v21[2]);
      v22 = v21;
      v21 = (__int64 *)*v21;
      sub_146E9F3A0(v22, 56);
    }
    while ( !*((_BYTE *)v21 + 25) );
    v20 = v24[0];
  }
  return sub_146E9F3A0(v20, 56);
}

