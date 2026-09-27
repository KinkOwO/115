// sub_1444ECBD0

char __fastcall sub_1444ECBD0(__int64 a1)
{
  __int64 v2; // rsi
  __int64 v3; // rax
  __int64 *v4; // rdi
  __int64 v5; // r8
  __int64 v6; // rcx
  __int64 v7; // r8
  __int64 v8; // rax
  _QWORD *v9; // rdx
  unsigned int v10; // r14d
  unsigned int v11; // r15d
  int v12; // r12d
  int v13; // r13d
  __int64 v14; // rbx
  int v15; // eax
  __int64 v16; // rax
  __int64 v17; // rbx
  _QWORD *v18; // rax
  _QWORD *v19; // rax
  int v20; // eax
  __int64 v21; // rcx
  __int64 v22; // rax
  __int64 v23; // rcx
  __int64 v24; // rax
  __int64 v25; // rdx
  __int64 v26; // rcx
  volatile signed __int32 *v27; // rbx
  __int64 v28; // rcx
  unsigned __int64 v29; // rdx
  unsigned __int64 v31; // [rsp+28h] [rbp-39h] BYREF
  _DWORD v32[2]; // [rsp+30h] [rbp-31h] BYREF
  _DWORD v33[2]; // [rsp+38h] [rbp-29h] BYREF
  __int128 v34; // [rsp+40h] [rbp-21h] BYREF
  __int64 v35; // [rsp+50h] [rbp-11h]
  __int64 v36; // [rsp+58h] [rbp-9h] BYREF
  volatile signed __int32 *v37; // [rsp+60h] [rbp-1h]
  __int64 v38; // [rsp+68h] [rbp+7h]
  _BYTE v39[13]; // [rsp+70h] [rbp+Fh] BYREF
  unsigned __int64 v40; // [rsp+7Dh] [rbp+1Ch]
  unsigned __int8 v41; // [rsp+85h] [rbp+24h]

  v38 = -2;
  v2 = a1 + 1408;
  LOBYTE(v3) = sub_146E9FA80(a1 + 1408);
  if ( !(_BYTE)v3 )
    return v3;
  LODWORD(v3) = sub_1459A90F0(qword_14E66C090);
  if ( !(_DWORD)v3 )
    return v3;
  LODWORD(v3) = v3 - 1;
  if ( (_DWORD)v3 )
  {
    if ( (_DWORD)v3 == 1 )
      return v3;
    v3 = sub_145EFAFB0();
    v4 = (__int64 *)v3;
  }
  else
  {
    LOBYTE(v3) = qword_14EF2CAA0;
    if ( !qword_14EF2CAA0 || !*(_DWORD *)(qword_14EF2CAA0 + 8) || !qword_14EF2CAA8 )
      return v3;
    v4 = (__int64 *)(qword_14EF2CAA8 - 48);
  }
  if ( !v4 )
    return v3;
  sub_1444EBC10(a1, &v34, 7);
  LOBYTE(v3) = v34;
  if ( (_QWORD)v34 == *((_QWORD *)&v34 + 1) )
  {
    if ( !qword_14E683C78 )
      goto LABEL_31;
    v6 = 101033730;
    goto LABEL_17;
  }
  LOBYTE(v5) = 1;
  v3 = sub_140283D60(qword_14E683BF8, *(unsigned int *)v34, v5);
  if ( v3 )
  {
    v9 = (_QWORD *)(v3 + 88);
    if ( *(_QWORD *)(v3 + 112) >= 8u )
      v9 = (_QWORD *)*v9;
    LOBYTE(v7) = 1;
    sub_144724390(&v36, v9, v7, 0);
    v10 = sub_145B8C890(v4);
    v11 = sub_145B8C7D0(v4);
    v12 = 0;
    v13 = 0;
    v14 = v36;
    v15 = sub_146B33E50(v36);
    v16 = sub_146B33E00(v14, (unsigned int)(v15 - 1));
    v17 = v16;
    if ( v16 && *(_QWORD *)sub_1472E7960(v16) )
    {
      v18 = (_QWORD *)sub_1472E7960(v17);
      v12 = (*(__int64 (__fastcall **)(_QWORD))(*(_QWORD *)*v18 + 48LL))(*v18);
      v19 = (_QWORD *)sub_1472E7960(v17);
      v13 = (*(__int64 (__fastcall **)(_QWORD))(*(_QWORD *)*v19 + 56LL))(*v19);
    }
    v31 = __PAIR64__(v10, v11);
    v32[0] = v12;
    v32[1] = v13;
    v33[0] = v11;
    v33[1] = v10;
    LOBYTE(v3) = sub_1444EA630(v4, v33, v32, &v31);
    if ( (_BYTE)v3 )
    {
      sub_146E9FBD0(v2, 1000, 0);
      v40 = v31;
      v41 = 1;
      v20 = (*(__int64 (__fastcall **)(__int64 *))(*v4 + 736))(v4);
      v21 = v41;
      if ( !v20 )
        v21 = 0;
      v41 = v21;
      v22 = sub_146D74000(v21);
      sub_146D746E0(v22, 2140);
      v24 = sub_146D74000(v23);
      sub_146D75B10(v24, v39, 22);
      LOBYTE(v3) = sub_146D75AF0(v26, v25);
    }
    v27 = v37;
    if ( v37 )
    {
      LODWORD(v3) = _InterlockedExchangeAdd(v37 + 2, 0xFFFFFFFF);
      if ( (_DWORD)v3 == 1 )
      {
        LOBYTE(v3) = (**(__int64 (__fastcall ***)(volatile signed __int32 *))v27)(v27);
        if ( _InterlockedExchangeAdd(v27 + 3, 0xFFFFFFFF) == 1 )
          LOBYTE(v3) = (*(__int64 (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v27 + 8LL))(v27);
      }
    }
    goto LABEL_31;
  }
  if ( qword_14E683C78 )
  {
    v6 = 100002246;
LABEL_17:
    v8 = sub_14723C170(v6);
    LOBYTE(v3) = sub_14668C520(qword_14E683C78, 2875, v8, 0);
  }
LABEL_31:
  v28 = v34;
  if ( (_QWORD)v34 )
  {
    v29 = (v35 - v34) & 0xFFFFFFFFFFFFFFFCuLL;
    if ( v29 >= 0x1000 )
    {
      v29 += 39LL;
      v28 = *(_QWORD *)(v34 - 8);
      if ( (unsigned __int64)(v34 - v28 - 8) > 0x1F )
        sub_148AAF304(v28, v29);
    }
    LOBYTE(v3) = sub_146E9F3A0(v28, v29);
    v34 = 0;
    v35 = 0;
  }
  return v3;
}

