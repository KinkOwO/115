_QWORD *__fastcall sub_1422A4CC0(_QWORD *a1)
{
  int v2; // r9d
  void (__fastcall ***v3)(_QWORD); // rbx
  int v4; // ecx
  __int64 v5; // rax
  void (__fastcall ***v6)(_QWORD); // rcx
  __int64 v7; // rcx
  __int64 v8; // rax

  sub_14566CFF0(a1);
  *a1 = off_14995D668;
  sub_147A86AB0(a1 + 9);
  a1[20] = 0;
  sub_146E9F7D0(a1 + 21);
  sub_14599D5D0(qword_14E66C090, 2155, sub_1422A4F70, a1);
  v3 = 0;
  v4 = qword_14E634260;
  if ( !qword_14E634260 )
  {
    v5 = sub_146E8BA20(184);
    if ( v5 )
      v6 = (void (__fastcall ***)(_QWORD))sub_144920BB0(v5);
    else
      v6 = 0;
    qword_14E634260 = (__int64)v6;
    (**v6)(v6);
    v4 = qword_14E634260;
  }
  LOBYTE(v2) = 1;
  sub_144922980(v4, 255, 620, v2, 2);
  v7 = qword_14E634260;
  if ( !qword_14E634260 )
  {
    v8 = sub_146E8BA20(184);
    if ( v8 )
      v3 = (void (__fastcall ***)(_QWORD))sub_144920BB0(v8);
    qword_14E634260 = (__int64)v3;
    (**v3)(v3);
    v7 = qword_14E634260;
  }
  sub_144922330(v7, 255, sub_1422A5190, a1);
  (*(void (__fastcall **)(_QWORD *, _QWORD))(a1[9] + 8LL))(a1 + 9, 0);
  return a1;
}
