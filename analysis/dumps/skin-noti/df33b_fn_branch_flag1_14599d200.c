// fn_branch_flag1_14599d200

// sub_14599D200
__int64 __fastcall sub_14599D200(__int64 a1, __int64 a2, unsigned __int8 a3, unsigned __int16 a4)
{
  unsigned int v6; // edi
  __int64 v7; // rax
  __int64 v8; // rdx
  __int64 v9; // r8
  __int64 v10; // r9
  void (__fastcall ***v11)(_QWORD); // rbx
  __int64 v12; // rcx
  int v13; // eax
  __int64 v14; // rax
  char v15; // al
  int v16; // esi
  __int64 v17; // rbx
  int v18; // edi
  int v19; // eax
  __int64 v20; // rcx
  __int64 v21; // rax
  int v23; // [rsp+20h] [rbp-38h]

  v6 = *(unsigned __int16 *)(a2 + 1);
  v7 = sub_146D74000(a1);
  v11 = 0;
  if ( !(unsigned __int8)sub_146D76620(v7)
    || ((v12 = qword_14E683C78) == 0
     || (v13 = sub_14667BB90(qword_14E683C78, 2889, 0),
         (v14 = sub_148AA307C(v13, 0, (unsigned int)&off_14DCB4760, (unsigned int)&off_14E19A800, 0)) == 0)
     || (unsigned int)sub_14517C390(v14) != 1
     || (v12 = 1794, (unsigned __int16)(v6 - 1794) > 1u)
      ? (v15 = 0)
      : (v15 = 1),
        v6 == 3 || v15) )
  {
    nullsub_1();
    v20 = qword_14E6836F8;
    if ( !qword_14E6836F8 )
    {
      v21 = sub_146E8BA20(136);
      if ( v21 )
        v11 = (void (__fastcall ***)(_QWORD))sub_1459A2980(v21);
      qword_14E6836F8 = (__int64)v11;
      (**v11)(v11);
      v20 = qword_14E6836F8;
    }
    sub_1459A2D70(v20, v6, a3, a4);
  }
  else
  {
    v16 = sub_14021A860(v12, v8, v9, v10, v23);
    v17 = sub_146E8C7D0(&unk_14A8FEA40);
    v18 = sub_146E8C7D0(&unk_14A8FEAB0);
    v19 = sub_146E8C7D0(&unk_14A8FE250);
    sub_146E938E0(v16, 0, v19, v18, 2094, (__int64)&qword_14E6836A0, v17);
  }
  return 1;
}

