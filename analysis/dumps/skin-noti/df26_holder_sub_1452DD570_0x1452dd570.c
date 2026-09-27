// holder_sub_1452DD570_0x1452dd570

__int64 sub_1452DD570()
{
  __int64 v0; // rdx
  __int64 v1; // rdx
  __int64 v2; // rcx
  __int64 v3; // rax
  void (__fastcall ***v4)(_QWORD); // rcx
  char v6; // [rsp+50h] [rbp+18h] BYREF
  unsigned __int8 v7; // [rsp+58h] [rbp+20h] BYREF

  v7 = 0;
  v6 = 0;
  sub_146EA09F0(&v7, 1);
  sub_146EA09F0(&v6, 1);
  LOBYTE(v0) = v6 != 0;
  sub_14601C320(v7, v0);
  v2 = qword_14E63AE60;
  if ( !qword_14E63AE60 )
  {
    v3 = sub_146E8BA20(336);
    if ( v3 )
      v4 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v3);
    else
      v4 = 0;
    qword_14E63AE60 = (__int64)v4;
    (**v4)(v4);
    v2 = qword_14E63AE60;
  }
  return sub_1447EF520(v2, v1);
}

