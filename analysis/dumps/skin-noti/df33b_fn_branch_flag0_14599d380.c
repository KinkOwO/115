// fn_branch_flag0_14599d380

// sub_14599D380
__int64 __fastcall sub_14599D380(__int64 a1, __int64 a2, __int64 a3)
{
  unsigned __int16 v4; // di
  unsigned int v5; // ebx
  __int64 v6; // rcx
  __int64 v7; // rax
  void (__fastcall ***v8)(_QWORD); // rcx
  __int64 v9; // rcx
  __int64 v10; // rax
  __int64 v11; // rcx
  __int64 v12; // rax

  v4 = *(_WORD *)(a3 + 1);
  v5 = 199;
  if ( v4 == 199 )
  {
    v6 = qword_14E683700;
    if ( !qword_14E683700 )
    {
      v7 = sub_146E8BA20(136);
      if ( v7 )
        v8 = (void (__fastcall ***)(_QWORD))sub_1459A3A20(v7);
      else
        v8 = 0;
      qword_14E683700 = (__int64)v8;
      (**v8)(v8);
      v6 = qword_14E683700;
    }
    if ( (unsigned __int8)sub_1459A3BB0(v6, 199) )
    {
      v9 = 199;
LABEL_12:
      sub_1456B8AB0(v9, a3);
      return 1;
    }
  }
  else
  {
    v5 = *(unsigned __int16 *)(a3 + 1);
  }
  v10 = sub_146D74000(a1);
  if ( !(unsigned __int8)sub_146D76620(v10) )
  {
    nullsub_1();
    v12 = sub_1459A0280(v11);
    sub_1459A3BB0(v12, v5);
    v9 = v4;
    goto LABEL_12;
  }
  return 1;
}

