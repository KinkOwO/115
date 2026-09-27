// caller_reg_noti_0x14599d5d0

// sub_14599D5D0
__int64 __fastcall sub_14599D5D0(__int64 a1, unsigned int a2, __int64 a3, __int64 a4)
{
  __int64 v7; // rcx
  __int64 v8; // rax
  void (__fastcall ***v9)(_QWORD); // rcx

  v7 = qword_14E683700;
  if ( !qword_14E683700 )
  {
    v8 = sub_146E8BA20(136);
    if ( v8 )
      v9 = (void (__fastcall ***)(_QWORD))sub_1459A3A20(v8);
    else
      v9 = 0;
    qword_14E683700 = (__int64)v9;
    (**v9)(v9);
    v7 = qword_14E683700;
  }
  return sub_1459A3DD0(v7, a2, a3, a4);
}

