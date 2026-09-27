// ui_1441E47B0

__int64 __fastcall sub_1441E47B0(__int64 a1)
{
  __int64 v2; // rcx
  __int64 v3; // rax
  void (__fastcall ***v4)(_QWORD); // rcx
  __int64 v5; // rax
  __int64 v6; // rdx
  _OWORD *v7; // r8
  _QWORD *v8; // rbx
  __int64 v9; // rax
  __int64 **v10; // rax
  __int64 i; // rax
  __int64 *j; // rcx
  __int64 v13; // rdx
  __int64 v14; // rcx
  __int64 *v15; // rbx
  __int64 *v16; // rcx
  _QWORD v18[4]; // [rsp+28h] [rbp-20h] BYREF

  *(_QWORD *)(a1 + 24) = *(_QWORD *)(a1 + 16);
  v2 = qword_14E638F28;
  if ( !qword_14E638F28 )
  {
    v3 = sub_146E8BA20(1472);
    if ( v3 )
      v4 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v3);
    else
      v4 = 0;
    qword_14E638F28 = (__int64)v4;
    (**v4)(v4);
    v2 = qword_14E638F28;
  }
  v5 = sub_1444EBDF0(v2, 4);
  sub_1441B8B20(v18, v5);
  v8 = *(_QWORD **)v18[0];
  while ( v8 != (_QWORD *)v18[0] )
  {
    v9 = sub_1444EBAB0(0x9C40u, v6, (__int64)v7);
    if ( v9 && *(_DWORD *)(v9 + 8) == 4 )
    {
      v7 = v8 + 4;
      v6 = *(_QWORD *)(a1 + 24);
      if ( v6 == *(_QWORD *)(a1 + 32) )
      {
        sub_1405EA310(a1 + 16, v6, v7);
      }
      else
      {
        *(_OWORD *)v6 = *v7;
        *(_QWORD *)(v6 + 16) = v8[6];
        *(_QWORD *)(a1 + 24) += 24LL;
      }
    }
    v10 = (__int64 **)v8[2];
    if ( *((_BYTE *)v10 + 25) )
    {
      for ( i = v8[1]; !*(_BYTE *)(i + 25); i = *(_QWORD *)(i + 8) )
      {
        if ( v8 != *(_QWORD **)(i + 16) )
          break;
        v8 = (_QWORD *)i;
      }
      v8 = (_QWORD *)i;
    }
    else
    {
      v8 = (_QWORD *)v8[2];
      for ( j = *v10; !*((_BYTE *)j + 25); j = (__int64 *)*j )
        v8 = j;
    }
  }
  sub_141FD8CD0(*(_QWORD *)(a1 + 16), *(_QWORD *)(a1 + 24), (*(_QWORD *)(a1 + 24) - *(_QWORD *)(a1 + 16)) / 24LL, 4);
  LOBYTE(v13) = *(_QWORD *)(a1 + 16) == *(_QWORD *)(a1 + 24);
  (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 2424) + 16LL))(*(_QWORD *)(a1 + 2424), v13);
  v14 = v18[0];
  v15 = *(__int64 **)(v18[0] + 8LL);
  if ( !*((_BYTE *)v15 + 25) )
  {
    do
    {
      sub_1401DBB80(v18, v18, v15[2]);
      v16 = v15;
      v15 = (__int64 *)*v15;
      sub_146E9F3A0(v16, 56);
    }
    while ( !*((_BYTE *)v15 + 25) );
    v14 = v18[0];
  }
  return sub_146E9F3A0(v14, 56);
}

