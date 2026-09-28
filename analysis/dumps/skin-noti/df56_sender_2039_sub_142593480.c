// sender_2039_sub_142593480

void __fastcall sub_142593480(__int64 a1)
{
  __int64 v2; // rax
  unsigned int v3; // eax
  __int64 v4; // rdx
  __int64 v5; // r8
  __int64 v6; // r9
  __int64 v7; // rax
  _QWORD *v8; // rcx
  __int64 v9; // rsi
  void (__fastcall *v10)(__int64, __int64, __int64); // rbx
  __int64 v11; // rax
  __int64 v12; // r8
  unsigned __int64 v13; // rdx
  __int64 v14; // rcx
  _QWORD *v15; // rax
  unsigned __int64 v16; // rdx
  __int64 v17; // rcx
  _QWORD *v18; // rdx
  volatile signed __int32 *v19; // rbx
  __int64 v20; // rax
  __int64 v21; // rcx
  __int64 v22; // rax
  __int64 v23; // rdx
  __int64 v24; // rdx
  __int64 v25; // rcx
  unsigned __int64 v26; // rdx
  __int64 v27; // rcx
  __int64 v28; // [rsp+38h] [rbp-39h] BYREF
  volatile signed __int32 *v29; // [rsp+40h] [rbp-31h]
  __int64 v30; // [rsp+48h] [rbp-29h]
  _QWORD v31[2]; // [rsp+50h] [rbp-21h] BYREF
  __int64 v32; // [rsp+60h] [rbp-11h]
  unsigned __int64 v33; // [rsp+68h] [rbp-9h]
  _QWORD v34[3]; // [rsp+70h] [rbp-1h] BYREF
  unsigned __int64 v35; // [rsp+88h] [rbp+17h]
  _QWORD v36[3]; // [rsp+90h] [rbp+1Fh] BYREF
  unsigned __int64 v37; // [rsp+A8h] [rbp+37h]

  v30 = -2;
  if ( sub_1429BDDE0(qword_14E683C78) )
  {
    v2 = sub_1429BDDE0(qword_14E683C78);
    sub_1455262F0(v2);
  }
  if ( qword_14E682918 )
  {
    v3 = sub_146E9F840(a1 + 176);
    sub_145E295B0(qword_14E682918, v3);
    sub_146E9FF70(a1 + 176, v4, v5, v6);
  }
  v31[0] = 0;
  v32 = 0;
  v33 = 7;
  v7 = sub_148AA307C(*(_QWORD *)(a1 + 96), 0, (unsigned int)&off_14DD273E0, (unsigned int)&off_14DD9A3D0, 0);
  v9 = v7;
  if ( v7 && *(_DWORD *)(a1 + 76) == 1 )
  {
    v10 = *(void (__fastcall **)(__int64, __int64, __int64))(*(_QWORD *)a1 + 896LL);
    v11 = sub_14775DB90(v7, v34);
    LOBYTE(v12) = 1;
    v10(a1, v11, v12);
    if ( v35 >= 8 )
    {
      v13 = 2 * v35 + 2;
      v14 = v34[0];
      if ( v13 >= 0x1000 )
      {
        v13 = 2 * v35 + 41;
        v14 = *(_QWORD *)(v34[0] - 8LL);
        if ( (unsigned __int64)(v34[0] - v14 - 8) > 0x1F )
          sub_148AAF304(v14, v13);
      }
      sub_146E9F3A0(v14, v13);
    }
    v34[2] = 0;
    v35 = 7;
    LOWORD(v34[0]) = 0;
    v15 = (_QWORD *)sub_14775DB60(v9, v36);
    v8 = v31;
    if ( v31 != v15 )
    {
      if ( v15[3] >= 8u )
        v15 = (_QWORD *)*v15;
      sub_14014C8D0(v31, v15);
    }
    if ( v37 >= 8 )
    {
      v16 = 2 * v37 + 2;
      v17 = v36[0];
      if ( v16 >= 0x1000 )
      {
        v16 = 2 * v37 + 41;
        v17 = *(_QWORD *)(v36[0] - 8LL);
        if ( (unsigned __int64)(v36[0] - v17 - 8) > 0x1F )
          sub_148AAF304(v17, v16);
      }
      sub_146E9F3A0(v17, v16);
    }
    v36[2] = 0;
    v37 = 7;
    LOWORD(v36[0]) = 0;
  }
  if ( v32 )
  {
    v18 = v31;
    if ( v33 >= 8 )
      v18 = (_QWORD *)v31[0];
    sub_1466775B0(qword_14E683C78, v18, 1);
    sub_14667F660(qword_14E683C78, &v28);
    if ( v28 && !(unsigned __int8)sub_144AD3560() )
      sub_144AD3CD0(v28);
    v19 = v29;
    if ( v29 )
    {
      if ( _InterlockedExchangeAdd(v29 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v19)(v19);
        if ( _InterlockedExchangeAdd(v19 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v19 + 8LL))(v19);
      }
    }
  }
  else
  {
    v20 = sub_146D74000(v8);
    sub_146D746E0(v20, 1646);
    v22 = sub_146D74000(v21);
    LOBYTE(v23) = 1;
    sub_146D75CC0(v22, v23);
    sub_146D75AF0(v25, v24);
  }
  if ( sub_14667BB90(qword_14E683C78, 265, 0) )
    sub_146694510(qword_14E683C78, 265, -1, 0, 1);
  if ( v33 >= 8 )
  {
    v26 = 2 * v33 + 2;
    v27 = v31[0];
    if ( v26 >= 0x1000 )
    {
      v26 = 2 * v33 + 41;
      v27 = *(_QWORD *)(v31[0] - 8LL);
      if ( (unsigned __int64)(v31[0] - v27 - 8) > 0x1F )
        sub_148AAF304(v27, v26);
    }
    sub_146E9F3A0(v27, v26);
  }
  v32 = 0;
  v33 = 7;
  LOWORD(v31[0]) = 0;
}

