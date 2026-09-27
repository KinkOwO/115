// ui_1441E4430

__int64 __fastcall sub_1441E4430(__int64 a1)
{
  __int64 v2; // rcx
  __int64 v3; // rax
  void (__fastcall ***v4)(_QWORD); // rcx
  _QWORD *v5; // rax
  __int64 *v6; // rbx
  _OWORD *v7; // r8
  __int64 v8; // rdx
  __int64 **v9; // rax
  __int64 *i; // rax
  __int64 *j; // rcx
  __int64 v12; // rdx
  __int64 v13; // rcx
  __int64 *v14; // rbx
  __int64 *v15; // rcx
  __int64 v17[4]; // [rsp+28h] [rbp-20h] BYREF

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
  v5 = (_QWORD *)sub_1444EBDF0(v2, 7);
  sub_1441B8B20(v17, v5);
  v6 = *(__int64 **)v17[0];
  if ( !*(_BYTE *)(*(_QWORD *)v17[0] + 25LL) )
  {
    do
    {
      v7 = v6 + 4;
      v8 = *(_QWORD *)(a1 + 24);
      if ( v8 == *(_QWORD *)(a1 + 32) )
      {
        sub_1405EA310(a1 + 16, v8, v7);
      }
      else
      {
        *(_OWORD *)v8 = *v7;
        *(_QWORD *)(v8 + 16) = v6[6];
        *(_QWORD *)(a1 + 24) += 24LL;
      }
      v9 = (__int64 **)v6[2];
      if ( *((_BYTE *)v9 + 25) )
      {
        for ( i = (__int64 *)v6[1]; !*((_BYTE *)i + 25); i = (__int64 *)i[1] )
        {
          if ( v6 != (__int64 *)i[2] )
            break;
          v6 = i;
        }
        v6 = i;
      }
      else
      {
        v6 = (__int64 *)v6[2];
        for ( j = *v9; !*((_BYTE *)j + 25); j = (__int64 *)*j )
          v6 = j;
      }
    }
    while ( !*((_BYTE *)v6 + 25) );
  }
  sub_141FD8CD0(*(_QWORD *)(a1 + 16), *(_QWORD *)(a1 + 24), (*(_QWORD *)(a1 + 24) - *(_QWORD *)(a1 + 16)) / 24LL, 7);
  LOBYTE(v12) = *(_QWORD *)(a1 + 16) == *(_QWORD *)(a1 + 24);
  (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 1752) + 16LL))(*(_QWORD *)(a1 + 1752), v12);
  v13 = v17[0];
  v14 = *(__int64 **)(v17[0] + 8);
  if ( !*((_BYTE *)v14 + 25) )
  {
    do
    {
      sub_1401DBB80(v17, v17, v14[2]);
      v15 = v14;
      v14 = (__int64 *)*v14;
      sub_146E9F3A0(v15, 56);
    }
    while ( !*((_BYTE *)v14 + 25) );
    v13 = v17[0];
  }
  return sub_146E9F3A0(v13, 56);
}

