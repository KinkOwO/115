// composer_2039_sub_1444F0ED0

__int64 __fastcall sub_1444F0ED0(__int64 a1, _QWORD *a2, unsigned __int8 a3)
{
  __int64 v5; // rbx
  __int64 v6; // rcx
  __int64 v7; // rax
  void (__fastcall ***v8)(_QWORD); // rcx
  __int64 v9; // rcx
  __int64 v10; // rax
  __int64 v11; // rcx
  __int64 v12; // rdi
  __int64 v13; // rax
  __int64 v14; // rcx
  __int64 v15; // rax
  __int64 v16; // rax
  __int64 v17; // rdx
  __int64 v18; // rcx

  v5 = 0;
  v6 = qword_14E634230;
  if ( !qword_14E634230 )
  {
    v7 = sub_146E8BA20(112);
    if ( v7 )
      v8 = (void (__fastcall ***)(_QWORD))sub_1403DE110(v7);
    else
      v8 = 0;
    qword_14E634230 = (__int64)v8;
    (**v8)(v8);
    v6 = qword_14E634230;
  }
  sub_1403FCE50(v6, a3);
  v10 = sub_146D74000(v9);
  sub_146D746E0(v10, 2039);
  v11 = a2[1] - *a2;
  v12 = v11 / 25;
  v13 = sub_146D74000(v11);
  sub_146D75CC0(v13, (unsigned __int8)v12);
  if ( (_BYTE)v12 )
  {
    v12 = (unsigned __int8)v12;
    do
    {
      v15 = sub_146D74000(v14);
      sub_146D75B10(v15, v5 + *a2, 25);
      v5 += 25;
      --v12;
    }
    while ( v12 );
  }
  v16 = sub_146D74000(v14);
  sub_146D75CC0(v16, a3);
  return sub_146D75AF0(v18, v17);
}

