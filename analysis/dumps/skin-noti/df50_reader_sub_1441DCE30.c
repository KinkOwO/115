// reader_sub_1441DCE30

__int64 __fastcall sub_1441DCE30(_QWORD *a1)
{
  _QWORD *v2; // rbx
  __int64 v3; // rbp
  __int64 v4; // rsi
  void (__fastcall *v5)(__int64, __int64); // r14
  __int64 v6; // rdx
  __int64 v7; // rcx
  __int64 v8; // rax
  void (__fastcall ***v9)(_QWORD); // rcx
  int v10; // eax
  __int64 v11; // rbx
  _QWORD *v12; // rdx
  __int64 v13; // rax
  __int64 v14; // rax
  __int64 v15; // r8
  unsigned __int64 v16; // rdx
  __int64 v17; // rcx
  __int64 v18; // rdx
  __int64 v19; // rdx
  _QWORD v21[3]; // [rsp+20h] [rbp-58h] BYREF
  _QWORD v22[2]; // [rsp+38h] [rbp-40h] BYREF
  __int64 v23; // [rsp+48h] [rbp-30h]
  unsigned __int64 v24; // [rsp+50h] [rbp-28h]

  v21[2] = -2;
  if ( a1[7] && (unsigned __int8)sub_146F593F0() )
    (*(void (__fastcall **)(_QWORD *))(*a1 + 112LL))(a1);
  (*(void (__fastcall **)(_QWORD *))(*a1 + 80LL))(a1);
  (*(void (__fastcall **)(_QWORD *))(*a1 + 96LL))(a1);
  v2 = a1 + 39;
  v3 = 9;
  do
  {
    v4 = *(v2 - 2);
    v5 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v4 + 16LL);
    if ( (unsigned __int8)sub_141FB6530(*v2) || !(unsigned __int8)sub_146ED0030(*(v2 - 12)) )
      v6 = 0;
    else
      LOBYTE(v6) = 1;
    v5(v4, v6);
    v2 += 22;
    --v3;
  }
  while ( v3 );
  v7 = qword_14E634230;
  if ( !qword_14E634230 )
  {
    v8 = sub_146E8BA20(112);
    v21[0] = v8;
    if ( v8 )
      v9 = (void (__fastcall ***)(_QWORD))sub_1403DE110(v8);
    else
      v9 = 0;
    qword_14E634230 = (__int64)v9;
    (**v9)(v9);
    v7 = qword_14E634230;
  }
  v10 = sub_1403F49B0(v7, 137);
  if ( v10 > 133 )
  {
    (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)a1[221] + 16LL))(a1[221], 0);
    LOBYTE(v19) = 1;
  }
  else
  {
    v11 = a1[221];
    v12 = a1 + 223;
    if ( a1[226] >= 8u )
      v12 = (_QWORD *)*v12;
    v13 = sub_146E8CF20(v21, v12, qword_14F1C0F40[v10]);
    v14 = sub_14014F430(v13);
    v22[0] = 0;
    v23 = 0;
    v24 = 7;
    v15 = -1;
    do
      ++v15;
    while ( *(_WORD *)(v14 + 2 * v15) );
    sub_14014C8D0(v22, v14);
    sub_146F19EE0(v11, 1, v22);
    if ( v24 >= 8 )
    {
      v16 = 2 * v24 + 2;
      v17 = v22[0];
      if ( v16 >= 0x1000 )
      {
        v16 = 2 * v24 + 41;
        v17 = *(_QWORD *)(v22[0] - 8LL);
        if ( (unsigned __int64)(v22[0] - v17 - 8) > 0x1F )
          sub_148AAF304(v17, v16);
      }
      sub_146E9F3A0(v17, v16);
    }
    v23 = 0;
    v24 = 7;
    LOWORD(v22[0]) = 0;
    sub_146E8C910(v21);
    LOBYTE(v18) = 1;
    (*(void (__fastcall **)(_QWORD, __int64))(*(_QWORD *)a1[221] + 16LL))(a1[221], v18);
    v19 = 0;
  }
  return (*(__int64 (__fastcall **)(_QWORD, __int64))(*(_QWORD *)a1[227] + 16LL))(a1[227], v19);
}

