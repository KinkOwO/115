// sender_1551_sub_140D95B80

__int64 __fastcall sub_140D95B80(__int64 a1, __int64 a2, __int64 a3)
{
  __int64 v4; // r12
  unsigned int v5; // r14d
  unsigned int v6; // r15d
  __int64 v7; // rax
  __int64 v8; // rbx
  int v9; // esi
  __int64 v10; // rax
  __int64 v11; // rax
  __int64 v12; // rax
  void (__fastcall ***v13)(_QWORD); // rcx
  unsigned int v14; // ebx
  __int64 v15; // rax
  __int64 v16; // rax
  __int64 v17; // rax
  __int64 v18; // r8
  int v19; // ebp
  __int64 v20; // rax
  void (__fastcall ***v21)(_QWORD); // rcx
  _QWORD *v22; // rdi
  int v23; // ebx
  int v24; // eax
  __int64 result; // rax
  __int64 v26; // rcx
  int v27; // esi
  __int64 v28; // rax
  __int64 v29; // rax
  __int64 v30; // rcx
  __int64 v31; // rax
  __int64 v32; // rcx
  __int64 v33; // rax
  unsigned __int64 v34; // rdx
  __int64 v35; // rcx
  _BYTE v36[16]; // [rsp+50h] [rbp-78h] BYREF
  _QWORD v37[2]; // [rsp+60h] [rbp-68h] BYREF
  __int64 v38; // [rsp+70h] [rbp-58h]
  unsigned __int64 v39; // [rsp+78h] [rbp-50h]

  v4 = sub_146C82A50();
  v5 = 0;
  v6 = sub_147681E30(a3 + 56);
  v7 = sub_146E8C7D0(&unk_1494D8BF0);
  v8 = a3 + 24;
  if ( (unsigned __int8)sub_1401500C0(v8, v7) )
  {
    v9 = 0;
  }
  else
  {
    v10 = sub_146E8C7D0(&unk_1494D8C18);
    if ( (unsigned __int8)sub_1401500C0(v8, v10) )
    {
      v9 = 1;
    }
    else
    {
      v11 = sub_146E8C7D0(&unk_1494D8C40);
      v9 = 0;
      if ( (unsigned __int8)sub_1401500C0(v8, v11) )
        v9 = 2;
    }
  }
  v37[0] = 0;
  v38 = 0;
  v39 = 7;
  if ( v9 == 1 )
  {
    if ( !qword_14E634498 )
    {
      v12 = sub_146E8BA20(928);
      if ( v12 )
        v13 = (void (__fastcall ***)(_QWORD))sub_1438BCE40(v12);
      else
        v13 = 0;
      qword_14E634498 = (__int64)v13;
      (**v13)(v13);
    }
    v14 = ((__int64 (*)(void))sub_140C15FA0)();
    v15 = sub_146E8C7D0(&unk_1494D8C60);
    v16 = sub_146E8CF20(v36, v15, v14);
    v17 = sub_14014F430(v16);
    v18 = -1;
    do
      ++v18;
    while ( *(_WORD *)(v17 + 2 * v18) );
    sub_14014C8D0(v37, v17);
    sub_146E8C910(v36);
  }
  v19 = qword_14E6343D0;
  if ( !qword_14E6343D0 )
  {
    v20 = sub_146E8BA20(72);
    if ( v20 )
      v21 = (void (__fastcall ***)(_QWORD))sub_146E93360(v20);
    else
      v21 = 0;
    qword_14E6343D0 = (__int64)v21;
    (**v21)(v21);
    v19 = qword_14E6343D0;
  }
  v22 = v37;
  if ( v39 >= 8 )
    v22 = (_QWORD *)v37[0];
  v23 = sub_146E8C7D0(&unk_1494D8CF0);
  v24 = sub_146E8C7D0(&unk_1494D8DB0);
  sub_146E938E0(v19, 0, v24, v23, 7339, (__int64)&qword_14E63A5C8, (__int64)v22);
  result = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v4 + 1392LL))(v4);
  if ( (_BYTE)result )
  {
    v27 = v9 - 1;
    if ( v27 )
    {
      if ( v27 == 1 )
        v5 = 2;
    }
    else
    {
      v5 = 1;
      v28 = sub_140D66E70();
      v6 = sub_140C15FA0(v28);
    }
    v29 = sub_146D74000(v26);
    sub_146D746E0(v29, 1546);
    v31 = sub_146D74000(v30);
    sub_146D75CE0(v31, v5);
    v33 = sub_146D74000(v32);
    sub_146D75CE0(v33, v6);
    result = sub_146D75AF0();
  }
  if ( v39 >= 8 )
  {
    v34 = 2 * v39 + 2;
    v35 = v37[0];
    if ( v34 >= 0x1000 )
    {
      v34 = 2 * v39 + 41;
      v35 = *(_QWORD *)(v37[0] - 8LL);
      if ( (unsigned __int64)(v37[0] - v35 - 8) > 0x1F )
        sub_148AAF304(v35, v34);
    }
    result = sub_146E9F3A0(v35, v34);
  }
  v38 = 0;
  v39 = 7;
  LOWORD(v37[0]) = 0;
  return result;
}

