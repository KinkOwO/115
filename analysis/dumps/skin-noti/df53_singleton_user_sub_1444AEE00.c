// singleton_user_sub_1444AEE00

char __fastcall sub_1444AEE00(__int64 a1)
{
  _QWORD *v2; // rax
  __int64 v3; // rsi
  volatile signed __int32 *v4; // rbx
  void (__fastcall ***v5)(_QWORD); // r14
  int v6; // eax
  __int64 v7; // rax
  __int64 v8; // rdi
  void (__fastcall *v9)(__int64, __int128 *); // rbx
  __int64 v10; // rax
  volatile signed __int32 *v11; // rbx
  __int64 v12; // rdi
  void (__fastcall *v13)(__int64, __int64); // rbx
  __int64 v14; // rax
  __int64 v15; // rcx
  __int64 v16; // rax
  void (__fastcall ***v17)(_QWORD); // rcx
  __int64 v18; // rax
  __int64 v19; // rdx
  __int64 v20; // rbx
  unsigned int v21; // edi
  unsigned int v22; // r12d
  unsigned int v23; // r13d
  __int64 v24; // rax
  __int64 v25; // rcx
  __int64 v26; // rax
  __int64 v27; // rax
  __int64 v28; // rdx
  __int64 v29; // rax
  __int64 v30; // rax
  __int128 v32; // [rsp+38h] [rbp-49h] BYREF
  _QWORD v33[4]; // [rsp+48h] [rbp-39h] BYREF
  _DWORD v34[6]; // [rsp+68h] [rbp-19h] BYREF
  __int64 v35; // [rsp+80h] [rbp-1h]
  _BYTE v36[8]; // [rsp+88h] [rbp+7h] BYREF
  volatile signed __int32 *v37; // [rsp+90h] [rbp+Fh]
  _BYTE v38[8]; // [rsp+98h] [rbp+17h] BYREF
  volatile signed __int32 *v39; // [rsp+A0h] [rbp+1Fh]
  _BYTE v40[16]; // [rsp+A8h] [rbp+27h] BYREF

  v35 = -2;
  v2 = (_QWORD *)(*(__int64 (__fastcall **)(__int64, _BYTE *))(*(_QWORD *)a1 + 272LL))(a1, v36);
  (*(void (__fastcall **)(_QWORD))(*(_QWORD *)*v2 + 320LL))(*v2);
  v3 = -1;
  v4 = v37;
  if ( v37 )
  {
    if ( _InterlockedExchangeAdd(v37 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v4)(v4);
      if ( _InterlockedExchangeAdd(v4 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v4 + 8LL))(v4);
    }
  }
  *(_BYTE *)(a1 + 992) = 1;
  sub_1457789D0(7);
  sub_146ECC550(*(_QWORD *)(a1 + 208), 0);
  v5 = 0;
  if ( !(unsigned int)sub_1454ED3D0() )
  {
    v6 = sub_14667EB40(qword_14E683C78, 2);
    v7 = sub_148AA307C(v6, 0, (unsigned int)&off_14DCB4760, (unsigned int)&off_14DD9C1A0, 0);
    v8 = *(_QWORD *)(a1 + 208);
    v9 = *(void (__fastcall **)(__int64, __int128 *))(*(_QWORD *)v8 + 200LL);
    v10 = (*(__int64 (__fastcall **)(__int64, _BYTE *))(*(_QWORD *)v7 + 272LL))(v7, v38);
    v32 = 0;
    v32 = *(_OWORD *)v10;
    *(_QWORD *)v10 = 0;
    *(_QWORD *)(v10 + 8) = 0;
    v9(v8, &v32);
    v11 = v39;
    if ( v39 )
    {
      if ( _InterlockedExchangeAdd(v39 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v11)(v11);
        if ( _InterlockedExchangeAdd(v11 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v11 + 8LL))(v11);
      }
    }
    v12 = *(_QWORD *)(a1 + 208);
    v13 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v12 + 200LL);
    v14 = sub_14667E6C0(qword_14E683C78, v40, 1);
    v13(v12, v14);
    *(_BYTE *)(a1 + 2352) = 1;
  }
  (*(void (__fastcall **)(_QWORD))(**(_QWORD **)(a1 + 208) + 112LL))(*(_QWORD *)(a1 + 208));
  sub_1467A9F20(a1, 0);
  sub_143FD16D0(*(_QWORD *)(a1 + 1584), 0);
  sub_146E9FBD0(a1 + 1616, 0, 0);
  sub_144F26F70(v34);
  v34[0] = 1;
  v34[1] = 11;
  v15 = qword_14E639AD8;
  if ( !qword_14E639AD8 )
  {
    v16 = sub_146E8BA20(464);
    if ( v16 )
      v17 = (void (__fastcall ***)(_QWORD))sub_144F26E40(v16);
    else
      v17 = 0;
    qword_14E639AD8 = (__int64)v17;
    (**v17)(v17);
    v15 = qword_14E639AD8;
  }
  sub_144F29800(v15, v34);
  v18 = sub_145F0BA60(qword_14E683C08);
  v20 = v18;
  if ( v18 )
  {
    v21 = sub_14106BFD0(v18);
    v22 = sub_140BA9880(v20);
    v23 = sub_1406FE620(v20);
    v24 = sub_141D37640(v20);
    v33[0] = 0;
    v33[2] = 0;
    v33[3] = 7;
    do
      ++v3;
    while ( *(_WORD *)(v24 + 2 * v3) );
    sub_14014C8D0(v33, v24);
    sub_1444ADF40(a1, 99, v33, v23, v22, v21);
  }
  v25 = qword_14E659EA8;
  if ( !qword_14E659EA8 )
  {
    v26 = sub_146E8BA20(496);
    if ( v26 )
      v5 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v26);
    qword_14E659EA8 = (__int64)v5;
    (**v5)(v5);
    v25 = qword_14E659EA8;
  }
  LOBYTE(v19) = 1;
  sub_14449ECE0(v25, v19);
  v27 = sub_1429BDDE0(qword_14E683C78);
  LOBYTE(v28) = 1;
  sub_145581D40(v27, v28);
  v29 = sub_146E8C7D0(&unk_14A304450);
  v30 = sub_145A0F110(v29, 185);
  (*(void (__fastcall **)(__int64, _QWORD, _QWORD, __int64))(*(_QWORD *)v30 + 136LL))(v30, 0, 0, 10);
  return 1;
}

