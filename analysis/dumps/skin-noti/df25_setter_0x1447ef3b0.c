// setter_0x1447ef3b0

__int64 __fastcall sub_1447EF3B0(__int64 a1, int a2, int a3)
{
  int v3; // ebp
  __int64 v4; // rax
  void (__fastcall ***v5)(_QWORD); // rcx
  __int64 v6; // rbx
  int v7; // edi
  int v8; // eax
  int v10; // [rsp+90h] [rbp+18h] BYREF

  v10 = a3;
  *(_DWORD *)sub_1401C4620(a1 + 120, &v10) = a2;
  v3 = qword_14E6343D0;
  if ( !qword_14E6343D0 )
  {
    v4 = sub_146E8BA20(72);
    if ( v4 )
      v5 = (void (__fastcall ***)(_QWORD))sub_146E93360(v4);
    else
      v5 = 0;
    qword_14E6343D0 = (__int64)v5;
    (**v5)(v5);
    v3 = qword_14E6343D0;
  }
  v6 = sub_146E8C7D0(&unk_14A3F4700);
  v7 = sub_146E8C7D0(&unk_14A3F4750);
  v8 = sub_146E8C7D0(&unk_14A3F47A0);
  return sub_146E938E0(v3, 0, v8, v7, 3744, (__int64)&qword_14E665EA0, v6);
}

