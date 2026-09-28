// singleton_user_sub_14355BEF0

__int64 __fastcall sub_14355BEF0(__int64 a1)
{
  __int64 v2; // rax
  __int64 v3; // rdx
  void (__fastcall ***v4)(_QWORD); // rsi
  __int64 v5; // rcx
  __int64 v6; // rax
  void (__fastcall ***v7)(_QWORD); // rcx
  __int64 v8; // rcx
  __int64 v9; // rax
  void (__fastcall ***v10)(_QWORD); // rcx
  __int64 v11; // rcx
  __int64 v12; // rax
  void (__fastcall ***v13)(_QWORD); // rcx
  __int64 v14; // rax
  __int64 v15; // rdx
  __int64 v16; // rax
  __int64 v17; // rdx
  __int64 v18; // rax
  __int64 v19; // rax
  __int64 v20; // rax
  int i; // edi
  __int64 v22; // rax
  __int64 v23; // rdx
  __int64 v24; // rbx
  __int64 v25; // rax
  __int64 v26; // rdx
  __int64 v27; // rcx
  __int64 v28; // rax

  v2 = sub_145EFAFB0();
  v4 = 0;
  if ( (*(unsigned int (__fastcall **)(__int64))(*(_QWORD *)v2 + 4832LL))(v2) == 10 )
  {
    v5 = qword_14E634230;
    if ( !qword_14E634230 )
    {
      v6 = sub_146E8BA20(112);
      if ( v6 )
        v7 = (void (__fastcall ***)(_QWORD))sub_1403DE110(v6);
      else
        v7 = 0;
      qword_14E634230 = (__int64)v7;
      (**v7)(v7);
      v5 = qword_14E634230;
    }
    LOBYTE(v3) = 4;
    sub_1403E4A10(v5, v3, a1 + 23651);
    v8 = qword_14E634230;
    if ( !qword_14E634230 )
    {
      v9 = sub_146E8BA20(112);
      if ( v9 )
        v10 = (void (__fastcall ***)(_QWORD))sub_1403DE110(v9);
      else
        v10 = 0;
      qword_14E634230 = (__int64)v10;
      (**v10)(v10);
      v8 = qword_14E634230;
    }
    sub_1403FD080(v8, 0);
    v11 = qword_14E634230;
    if ( !qword_14E634230 )
    {
      v12 = sub_146E8BA20(112);
      if ( v12 )
        v13 = (void (__fastcall ***)(_QWORD))sub_1403DE110(v12);
      else
        v13 = 0;
      qword_14E634230 = (__int64)v13;
      (**v13)(v13);
      v11 = qword_14E634230;
    }
    sub_1403E2270(v11);
  }
  v14 = sub_1429BDDE0(qword_14E683C78);
  LOBYTE(v15) = 1;
  sub_145581D40(v14, v15);
  if ( sub_145EFAFB0() )
  {
    v16 = sub_145EFAFB0();
    LOBYTE(v17) = 1;
    sub_145D36A70(v16, v17);
  }
  v18 = sub_146E8C7D0(&unk_149E389A8);
  if ( sub_145A0F110(v18, 80) )
  {
    v19 = sub_146E8C7D0(&unk_149E389A8);
    v20 = sub_145A0F110(v19, 81);
    (*(void (__fastcall **)(__int64, _QWORD, _QWORD, __int64, __int64))(*(_QWORD *)v20 + 136LL))(v20, 0, 0, 99, -2);
  }
  for ( i = 0; i < 8; ++i )
  {
    v22 = sub_145F13060(qword_14E683C20, (unsigned int)i);
    v24 = v22;
    if ( v22 && (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v22 + 6048LL))(v22) )
    {
      v25 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v24 + 6048LL))(v24);
      LOBYTE(v26) = 1;
      sub_145E4B250(v25, v26);
    }
  }
  LOBYTE(v23) = 1;
  sub_1466967D0(qword_14E683C78, v23);
  *(_WORD *)(a1 + 23649) = 0;
  *(_BYTE *)(a1 + 23648) = 0;
  v27 = qword_14E659EA8;
  if ( !qword_14E659EA8 )
  {
    v28 = sub_146E8BA20(496);
    if ( v28 )
      v4 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v28);
    qword_14E659EA8 = (__int64)v4;
    (**v4)(v4);
    v27 = qword_14E659EA8;
  }
  sub_14449EBA0(v27, 0);
  sub_146E9FBC0(a1 + 24128);
  return sub_144D2B6F0(a1);
}

