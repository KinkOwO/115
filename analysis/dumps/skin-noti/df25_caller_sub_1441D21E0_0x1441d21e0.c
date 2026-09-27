// caller_sub_1441D21E0_0x1441d21e0

__int64 __fastcall sub_1441D21E0(__int64 a1)
{
  __int64 v2; // rax
  void (__fastcall ***v3)(_QWORD); // rcx
  __int64 v4; // rcx
  __int64 v5; // rax
  void (__fastcall ***v6)(_QWORD); // rcx
  unsigned int *v7; // rdx
  __int64 v8; // rcx
  __int64 v9; // rax
  void (__fastcall ***v10)(_QWORD); // rcx
  __int64 result; // rax
  __int64 v12; // rcx
  unsigned __int64 v13; // rdx
  __int128 v14; // [rsp+38h] [rbp-20h] BYREF
  __int64 v15; // [rsp+48h] [rbp-10h]

  sub_14417BD90();
  sub_146694510(qword_14E683C78, 2475, -1, 0, 1);
  sub_141BB64F0(a1);
  if ( !qword_14E638F28 )
  {
    v2 = sub_146E8BA20(1472);
    if ( v2 )
      v3 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v2);
    else
      v3 = 0;
    qword_14E638F28 = (__int64)v3;
    (**v3)(v3);
  }
  sub_1444F1410();
  v4 = qword_14E638F28;
  if ( !qword_14E638F28 )
  {
    v5 = sub_146E8BA20(1472);
    if ( v5 )
      v6 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v5);
    else
      v6 = 0;
    qword_14E638F28 = (__int64)v6;
    (**v6)(v6);
    v4 = qword_14E638F28;
  }
  sub_1444EBC10(v4, &v14, 6);
  v7 = (unsigned int *)v14;
  if ( (__int64)(*((_QWORD *)&v14 + 1) - v14) >> 2 )
  {
    v8 = qword_14E63AE60;
    if ( !qword_14E63AE60 )
    {
      v9 = sub_146E8BA20(336);
      if ( v9 )
        v10 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v9);
      else
        v10 = 0;
      qword_14E63AE60 = (__int64)v10;
      (**v10)(v10);
      v7 = (unsigned int *)v14;
      v8 = qword_14E63AE60;
    }
    sub_142581F20(v8, *v7);
  }
  result = sub_146694510(qword_14E683C78, 2373, -1, 0, 1);
  v12 = v14;
  if ( (_QWORD)v14 )
  {
    v13 = (v15 - v14) & 0xFFFFFFFFFFFFFFFCuLL;
    if ( v13 >= 0x1000 )
    {
      v13 += 39LL;
      v12 = *(_QWORD *)(v14 - 8);
      if ( (unsigned __int64)(v14 - v12 - 8) > 0x1F )
        sub_148AAF304(v12, v13);
    }
    result = sub_146E9F3A0(v12, v13);
    v14 = 0;
    v15 = 0;
  }
  return result;
}

