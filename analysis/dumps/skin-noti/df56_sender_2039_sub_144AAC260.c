// sender_2039_sub_144AAC260

void __fastcall sub_144AAC260(__int64 a1)
{
  __int64 v2; // rax
  __int64 v3; // rax
  _QWORD *v4; // rcx
  __int64 v5; // rsi
  void (__fastcall *v6)(__int64, __int64, __int64); // rbx
  __int64 v7; // rax
  __int64 v8; // r8
  unsigned __int64 v9; // rdx
  __int64 v10; // rcx
  int v11; // eax
  _QWORD *v12; // rax
  unsigned __int64 v13; // rdx
  __int64 v14; // rcx
  _QWORD *v15; // rdx
  __int64 v16; // r8
  volatile signed __int32 *v17; // rbx
  __int64 v18; // rax
  __int64 v19; // rcx
  __int64 v20; // rax
  __int64 v21; // rdx
  __int64 v22; // rdx
  __int64 v23; // rcx
  unsigned __int64 v24; // rdx
  __int64 v25; // rcx
  __int64 v26; // [rsp+48h] [rbp-39h] BYREF
  volatile signed __int32 *v27; // [rsp+50h] [rbp-31h]
  __int64 v28; // [rsp+58h] [rbp-29h]
  _QWORD v29[2]; // [rsp+60h] [rbp-21h] BYREF
  __int64 v30; // [rsp+70h] [rbp-11h]
  unsigned __int64 v31; // [rsp+78h] [rbp-9h]
  _QWORD v32[3]; // [rsp+80h] [rbp-1h] BYREF
  unsigned __int64 v33; // [rsp+98h] [rbp+17h]
  _QWORD v34[3]; // [rsp+A0h] [rbp+1Fh] BYREF
  unsigned __int64 v35; // [rsp+B8h] [rbp+37h]

  v28 = -2;
  if ( sub_1429BDDE0(qword_14E683C78) )
  {
    v2 = sub_1429BDDE0(qword_14E683C78);
    sub_1455262F0(v2);
  }
  v29[0] = 0;
  v30 = 0;
  v31 = 7;
  v3 = sub_148AA307C(*(_QWORD *)(a1 + 96), 0, (unsigned int)&off_14DD273E0, (unsigned int)&off_14DDB34A0, 0);
  v5 = v3;
  if ( v3 && *(_DWORD *)(a1 + 76) == 1 )
  {
    v6 = *(void (__fastcall **)(__int64, __int64, __int64))(*(_QWORD *)a1 + 896LL);
    v7 = sub_14775DB90(v3, v32);
    LOBYTE(v8) = 1;
    v6(a1, v7, v8);
    if ( v33 >= 8 )
    {
      v9 = 2 * v33 + 2;
      v10 = v32[0];
      if ( v9 >= 0x1000 )
      {
        v9 = 2 * v33 + 41;
        v10 = *(_QWORD *)(v32[0] - 8LL);
        if ( (unsigned __int64)(v32[0] - v10 - 8) > 0x1F )
          sub_148AAF304(v10, v9);
      }
      sub_146E9F3A0(v10, v9);
    }
    v32[2] = 0;
    v33 = 7;
    LOWORD(v32[0]) = 0;
    v11 = sub_146E8C7D0(&unk_149669200);
    sub_145A31380(v11, -1, 0, 0, -1, -1, 0);
    v12 = (_QWORD *)sub_14775DB60(v5, v34);
    v4 = v29;
    if ( v29 != v12 )
    {
      if ( v12[3] >= 8u )
        v12 = (_QWORD *)*v12;
      sub_14014C8D0(v29, v12);
    }
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
  }
  if ( v30 )
  {
    v15 = v29;
    if ( v31 >= 8 )
      v15 = (_QWORD *)v29[0];
    sub_1466775B0(qword_14E683C78, v15, 1);
    sub_14667F660(qword_14E683C78, &v26);
    if ( v26 && !(unsigned __int8)sub_144AD3560() )
      sub_144AD3CD0(v26);
    v17 = v27;
    if ( v27 )
    {
      if ( _InterlockedExchangeAdd(v27 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v17)(v17);
        if ( _InterlockedExchangeAdd(v17 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v17 + 8LL))(v17);
      }
    }
  }
  else
  {
    v18 = sub_146D74000(v4);
    sub_146D746E0(v18, 1646);
    v20 = sub_146D74000(v19);
    LOBYTE(v21) = 1;
    sub_146D75CC0(v20, v21);
    sub_146D75AF0(v23, v22);
  }
  LOBYTE(v16) = 1;
  if ( (unsigned __int8)sub_146682140(qword_14E683C78, 3767, v16) )
    sub_146694510(qword_14E683C78, 3767, -1, 0, 1);
  if ( v31 >= 8 )
  {
    v24 = 2 * v31 + 2;
    v25 = v29[0];
    if ( v24 >= 0x1000 )
    {
      v24 = 2 * v31 + 41;
      v25 = *(_QWORD *)(v29[0] - 8LL);
      if ( (unsigned __int64)(v29[0] - v25 - 8) > 0x1F )
        sub_148AAF304(v25, v24);
    }
    sub_146E9F3A0(v25, v24);
  }
  v30 = 0;
  v31 = 7;
  LOWORD(v29[0]) = 0;
}

