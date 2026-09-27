// ui_1441E36F0

__int64 __fastcall sub_1441E36F0(_QWORD *a1)
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
  __int64 v14; // rcx
  __int64 *v15; // rbx
  __int64 *v16; // rcx
  __int64 v18[4]; // [rsp+28h] [rbp-20h] BYREF

  a1[3] = a1[2];
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
  v5 = (_QWORD *)sub_1444EBDF0(v2, 9);
  sub_1441B8B20(v18, v5);
  v6 = *(__int64 **)v18[0];
  if ( !*(_BYTE *)(*(_QWORD *)v18[0] + 25LL) )
  {
    do
    {
      v7 = v6 + 4;
      v8 = a1[3];
      if ( v8 == a1[4] )
      {
        sub_1405EA310(a1 + 2, v8, v7);
      }
      else
      {
        *(_OWORD *)v8 = *v7;
        *(_QWORD *)(v8 + 16) = v6[6];
        a1[3] += 24LL;
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
  sub_141FD8CD0(a1[2], a1[3], (a1[3] - a1[2]) / 24LL, 9);
  v13 = a1[148];
  if ( v13 )
  {
    LOBYTE(v12) = a1[2] == a1[3];
    (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v13 + 16LL))(v13, v12);
  }
  v14 = v18[0];
  v15 = *(__int64 **)(v18[0] + 8);
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

