// caller_reg_cmd_0x14599d450

// sub_14599D450
__int64 __fastcall sub_14599D450(__int64 a1, unsigned int a2, __int64 a3)
{
  __int64 v5; // rcx
  __int64 v6; // rax
  void (__fastcall ***v7)(_QWORD); // rcx

  v5 = qword_14E6836F8;
  if ( !qword_14E6836F8 )
  {
    v6 = sub_146E8BA20(136);
    if ( v6 )
      v7 = (void (__fastcall ***)(_QWORD))sub_1459A2980(v6);
    else
      v7 = 0;
    qword_14E6836F8 = (__int64)v7;
    (**v7)(v7);
    v5 = qword_14E6836F8;
  }
  return sub_1459A2FB0(v5, a2, a3);
}

