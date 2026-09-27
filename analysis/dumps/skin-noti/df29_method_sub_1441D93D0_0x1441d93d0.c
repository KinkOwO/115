// method_sub_1441D93D0_0x1441d93d0

__int64 __fastcall sub_1441D93D0(_QWORD *a1)
{
  void (__fastcall ***v2)(_QWORD); // rbx
  __int64 v3; // rcx
  __int64 v4; // rax
  void (__fastcall ***v5)(_QWORD); // rcx
  __int64 v6; // rdx
  __int64 v7; // rsi
  __int64 v8; // rcx
  __int64 v9; // rax
  void (__fastcall ***v10)(_QWORD); // rcx
  __int64 v11; // rdx
  __int64 v12; // rdi
  __int64 v13; // rcx
  __int64 v14; // rax
  __int64 v15; // rdx

  *((_DWORD *)a1 + 32) = -1;
  (*(void (__fastcall **)(_QWORD *))(*a1 + 104LL))(a1);
  v2 = 0;
  v3 = qword_14E638F28;
  if ( !qword_14E638F28 )
  {
    v4 = sub_146E8BA20(1472);
    if ( v4 )
      v5 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v4);
    else
      v5 = 0;
    qword_14E638F28 = (__int64)v5;
    (**v5)(v5);
    v3 = qword_14E638F28;
  }
  sub_1444E9710(v3);
  (*(void (__fastcall **)(_QWORD *))(*a1 + 24LL))(a1);
  LOBYTE(v6) = 1;
  (*(void (__fastcall **)(_QWORD *, __int64))(*a1 + 32LL))(a1, v6);
  v7 = a1[19];
  v8 = qword_14E638F28;
  if ( !qword_14E638F28 )
  {
    v9 = sub_146E8BA20(1472);
    if ( v9 )
      v10 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v9);
    else
      v10 = 0;
    qword_14E638F28 = (__int64)v10;
    (**v10)(v10);
    v8 = qword_14E638F28;
  }
  LOBYTE(v11) = (int)sub_1444EBF80(v8) > 1;
  sub_146F01920(v7, v11);
  v12 = a1[184];
  v13 = qword_14E638F28;
  if ( !qword_14E638F28 )
  {
    v14 = sub_146E8BA20(1472);
    if ( v14 )
      v2 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v14);
    qword_14E638F28 = (__int64)v2;
    (**v2)(v2);
    v13 = qword_14E638F28;
  }
  LOBYTE(v15) = (int)sub_1444EBFC0(v13) > 1;
  return sub_146F01920(v12, v15);
}

