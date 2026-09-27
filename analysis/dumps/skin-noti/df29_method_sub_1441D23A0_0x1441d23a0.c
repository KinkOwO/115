// method_sub_1441D23A0_0x1441d23a0

__int64 __fastcall sub_1441D23A0(__int64 a1)
{
  __int64 v2; // rax
  void (__fastcall ***v3)(_QWORD); // rcx
  __int64 v4; // rax
  void (__fastcall ***v5)(_QWORD, __int64); // rcx
  __int64 v6; // rax
  __int64 v7; // rcx
  __int64 v8; // rcx
  __int64 v9; // rax
  void (__fastcall ***v10)(_QWORD); // rcx
  unsigned int *v11; // rdx
  __int64 result; // rax
  __int64 v13; // rcx
  __int64 v14; // rax
  void (__fastcall ***v15)(_QWORD); // rcx
  __int64 v16; // rcx
  unsigned __int64 v17; // rdx
  __int128 v18; // [rsp+48h] [rbp-20h] BYREF
  __int64 v19; // [rsp+58h] [rbp-10h]

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
  sub_1444E9630();
  v4 = *(_QWORD *)(a1 + 8);
  if ( v4 )
  {
    v5 = *(void (__fastcall ****)(_QWORD, __int64))(v4 + 3688);
    if ( v5 )
    {
      (**v5)(v5, 1);
      v4 = *(_QWORD *)(a1 + 8);
    }
    *(_QWORD *)(v4 + 3688) = 0;
    v6 = sub_146E8BA20(400);
    if ( v6 )
      v7 = sub_1447E3ED0(v6);
    else
      v7 = 0;
    *(_QWORD *)(*(_QWORD *)(a1 + 8) + 3688LL) = v7;
    sub_1447EB510(*(_QWORD *)(*(_QWORD *)(a1 + 8) + 3688LL), 0, 0, 0, 1234000, 0, 0, -1);
  }
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
  sub_1444EBC10(v8, &v18, 6);
  v11 = (unsigned int *)v18;
  result = (__int64)(*((_QWORD *)&v18 + 1) - v18) >> 2;
  if ( result )
  {
    v13 = qword_14E63AE60;
    if ( !qword_14E63AE60 )
    {
      v14 = sub_146E8BA20(336);
      if ( v14 )
        v15 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v14);
      else
        v15 = 0;
      qword_14E63AE60 = (__int64)v15;
      (**v15)(v15);
      v11 = (unsigned int *)v18;
      v13 = qword_14E63AE60;
    }
    result = sub_142581F20(v13, *v11);
  }
  v16 = v18;
  if ( (_QWORD)v18 )
  {
    v17 = (v19 - v18) & 0xFFFFFFFFFFFFFFFCuLL;
    if ( v17 >= 0x1000 )
    {
      v17 += 39LL;
      v16 = *(_QWORD *)(v18 - 8);
      if ( (unsigned __int64)(v18 - v16 - 8) > 0x1F )
        sub_148AAF304(v16, v17);
    }
    result = sub_146E9F3A0(v16, v17);
    v18 = 0;
    v19 = 0;
  }
  return result;
}

