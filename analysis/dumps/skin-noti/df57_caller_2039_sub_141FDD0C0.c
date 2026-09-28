// caller_2039_sub_141FDD0C0

void __fastcall sub_141FDD0C0(__int64 a1, unsigned __int8 a2)
{
  __int64 *v3; // rbx
  __int64 v4; // rdx
  _OWORD *v5; // r8
  __int64 **v6; // rax
  __int64 *i; // rax
  __int64 *j; // rcx
  __int64 v9; // rcx
  __int64 v10; // rax
  void (__fastcall ***v11)(_QWORD); // rcx
  __int64 v12; // r8
  unsigned __int64 v13; // rdx
  __int128 v14; // [rsp+28h] [rbp-20h] BYREF
  __int64 v15; // [rsp+38h] [rbp-10h]

  v14 = 0;
  v15 = 0;
  v3 = **(__int64 ***)(a1 + 4264);
  if ( !*((_BYTE *)v3 + 25) )
  {
    v4 = *((_QWORD *)&v14 + 1);
    do
    {
      v5 = v3 + 4;
      if ( v4 == v15 )
      {
        sub_141FD7160((unsigned __int64 *)&v14, v4, (__int64)v5);
        v4 = *((_QWORD *)&v14 + 1);
      }
      else
      {
        *(_OWORD *)v4 = *v5;
        *(_QWORD *)(v4 + 16) = v3[6];
        *(_BYTE *)(v4 + 24) = *((_BYTE *)v3 + 56);
        v4 = *((_QWORD *)&v14 + 1) + 25LL;
        *((_QWORD *)&v14 + 1) += 25LL;
      }
      v6 = (__int64 **)v3[2];
      if ( *((_BYTE *)v6 + 25) )
      {
        for ( i = (__int64 *)v3[1]; !*((_BYTE *)i + 25); i = (__int64 *)i[1] )
        {
          if ( v3 != (__int64 *)i[2] )
            break;
          v3 = i;
        }
        v3 = i;
      }
      else
      {
        v3 = (__int64 *)v3[2];
        for ( j = *v6; !*((_BYTE *)j + 25); j = (__int64 *)*j )
          v3 = j;
      }
    }
    while ( !*((_BYTE *)v3 + 25) );
  }
  v9 = qword_14E638F28;
  if ( !qword_14E638F28 )
  {
    v10 = sub_146E8BA20(1472);
    if ( v10 )
      v11 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v10);
    else
      v11 = 0;
    qword_14E638F28 = (__int64)v11;
    (**v11)(v11);
    v9 = qword_14E638F28;
  }
  sub_1444F0ED0(v9, &v14, a2);
  v12 = v14;
  if ( (_QWORD)v14 )
  {
    v13 = 25 * ((v15 - (__int64)v14) / 25);
    if ( v13 >= 0x1000 )
    {
      v13 += 39LL;
      v12 = *(_QWORD *)(v14 - 8);
      if ( (unsigned __int64)(v14 - v12 - 8) > 0x1F )
        sub_148AAF304(v15 - v14, v13);
    }
    sub_146E9F3A0(v12, v13);
    v14 = 0;
    v15 = 0;
  }
}

