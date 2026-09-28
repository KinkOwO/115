// sender_2039_sub_144AAC5B0

void __fastcall sub_144AAC5B0(__int64 a1)
{
  __int64 v2; // rax
  __int64 v3; // rax
  _QWORD *v4; // rcx
  __int64 v5; // r14
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
  volatile signed __int32 *v16; // rbx
  __int64 v17; // rax
  __int64 v18; // rcx
  __int64 v19; // rax
  __int64 v20; // rdx
  __int64 v21; // rdx
  __int64 v22; // rcx
  unsigned __int64 v23; // rdx
  __int64 v24; // rcx
  __int64 v25; // [rsp+48h] [rbp-39h] BYREF
  volatile signed __int32 *v26; // [rsp+50h] [rbp-31h]
  __int64 v27; // [rsp+58h] [rbp-29h]
  _QWORD v28[2]; // [rsp+60h] [rbp-21h] BYREF
  __int64 v29; // [rsp+70h] [rbp-11h]
  unsigned __int64 v30; // [rsp+78h] [rbp-9h]
  _QWORD v31[3]; // [rsp+80h] [rbp-1h] BYREF
  unsigned __int64 v32; // [rsp+98h] [rbp+17h]
  _QWORD v33[3]; // [rsp+A0h] [rbp+1Fh] BYREF
  unsigned __int64 v34; // [rsp+B8h] [rbp+37h]

  v27 = -2;
  if ( sub_1429BDDE0(qword_14E683C78) )
  {
    v2 = sub_1429BDDE0(qword_14E683C78);
    sub_1455262F0(v2);
  }
  v28[0] = 0;
  v29 = 0;
  v30 = 7;
  v3 = sub_148AA307C(*(_QWORD *)(a1 + 96), 0, (unsigned int)&off_14DD273E0, (unsigned int)&off_14DDB3470, 0);
  v5 = v3;
  if ( v3 && *(_DWORD *)(a1 + 76) == 1 )
  {
    v6 = *(void (__fastcall **)(__int64, __int64, __int64))(*(_QWORD *)a1 + 896LL);
    v7 = sub_14775DB90(v3, v31);
    LOBYTE(v8) = 1;
    v6(a1, v7, v8);
    if ( v32 >= 8 )
    {
      v9 = 2 * v32 + 2;
      v10 = v31[0];
      if ( v9 >= 0x1000 )
      {
        v9 = 2 * v32 + 41;
        v10 = *(_QWORD *)(v31[0] - 8LL);
        if ( (unsigned __int64)(v31[0] - v10 - 8) > 0x1F )
          sub_148AAF304(v10, v9);
      }
      sub_146E9F3A0(v10, v9);
    }
    v31[2] = 0;
    v32 = 7;
    LOWORD(v31[0]) = 0;
    v11 = sub_146E8C7D0(&unk_149669200);
    sub_145A31380(v11, -1, 0, 0, -1, -1, 0);
    v12 = (_QWORD *)sub_14775DB60(v5, v33);
    v4 = v28;
    if ( v28 != v12 )
    {
      if ( v12[3] >= 8u )
        v12 = (_QWORD *)*v12;
      sub_14014C8D0(v28, v12);
    }
    if ( v34 >= 8 )
    {
      v13 = 2 * v34 + 2;
      v14 = v33[0];
      if ( v13 >= 0x1000 )
      {
        v13 = 2 * v34 + 41;
        v14 = *(_QWORD *)(v33[0] - 8LL);
        if ( (unsigned __int64)(v33[0] - v14 - 8) > 0x1F )
          sub_148AAF304(v14, v13);
      }
      sub_146E9F3A0(v14, v13);
    }
    v33[2] = 0;
    v34 = 7;
    LOWORD(v33[0]) = 0;
  }
  if ( v29 )
  {
    v15 = v28;
    if ( v30 >= 8 )
      v15 = (_QWORD *)v28[0];
    sub_1466775B0(qword_14E683C78, v15, 1);
    sub_14667F660(qword_14E683C78, &v25);
    if ( v25 && !(unsigned __int8)sub_144AD3560() )
      sub_144AD3CD0(v25);
    v16 = v26;
    if ( v26 )
    {
      if ( _InterlockedExchangeAdd(v26 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v16)(v16);
        if ( _InterlockedExchangeAdd(v16 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v16 + 8LL))(v16);
      }
    }
  }
  else
  {
    v17 = sub_146D74000(v4);
    sub_146D746E0(v17, 1646);
    v19 = sub_146D74000(v18);
    LOBYTE(v20) = 1;
    sub_146D75CC0(v19, v20);
    sub_146D75AF0(v22, v21);
  }
  if ( v30 >= 8 )
  {
    v23 = 2 * v30 + 2;
    v24 = v28[0];
    if ( v23 >= 0x1000 )
    {
      v23 = 2 * v30 + 41;
      v24 = *(_QWORD *)(v28[0] - 8LL);
      if ( (unsigned __int64)(v28[0] - v24 - 8) > 0x1F )
        sub_148AAF304(v24, v23);
    }
    sub_146E9F3A0(v24, v23);
  }
  v29 = 0;
  v30 = 7;
  LOWORD(v28[0]) = 0;
}

