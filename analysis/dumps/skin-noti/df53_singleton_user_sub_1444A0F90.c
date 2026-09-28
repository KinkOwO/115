// singleton_user_sub_1444A0F90

void __fastcall sub_1444A0F90(__int64 a1)
{
  void (__fastcall ***v2)(_QWORD); // rbx
  __int64 v3; // rcx
  __int64 v4; // rax
  void (__fastcall ***v5)(_QWORD); // rcx
  unsigned int v6; // r15d
  __int64 v7; // rcx
  __int64 v8; // rax
  void (__fastcall ***v9)(_QWORD); // rcx
  unsigned int v10; // r12d
  __int64 v11; // rcx
  __int64 v12; // rax
  unsigned int v13; // r13d
  __int64 v14; // rdi
  void (__fastcall *v15)(__int64, __int64); // rbx
  __int64 v16; // rax
  __int64 v17; // rax
  volatile signed __int32 *v18; // rbx
  __int64 v19; // rsi
  void (__fastcall *v20)(__int64, __int64); // rbx
  __int64 v21; // rax
  __int64 v22; // rax
  __int64 v23; // rax
  volatile signed __int32 *v24; // rbx
  __int64 v25; // rax
  volatile signed __int32 *v26; // rbx
  __int64 v27; // rsi
  void (__fastcall *v28)(__int64, __int64); // rbx
  __int64 v29; // rax
  __int64 v30; // rax
  __int64 v31; // rax
  volatile signed __int32 *v32; // rbx
  __int64 v33; // rax
  volatile signed __int32 *v34; // rbx
  __int64 v35; // rsi
  void (__fastcall *v36)(__int64, __int64); // rbx
  __int64 v37; // rax
  __int64 v38; // rax
  __int64 v39; // rax
  volatile signed __int32 *v40; // rbx
  __int64 v41; // [rsp+30h] [rbp-A1h] BYREF
  volatile signed __int32 *v42; // [rsp+38h] [rbp-99h]
  __int64 v43; // [rsp+40h] [rbp-91h] BYREF
  volatile signed __int32 *v44; // [rsp+48h] [rbp-89h]
  __int64 v45; // [rsp+50h] [rbp-81h] BYREF
  volatile signed __int32 *v46; // [rsp+58h] [rbp-79h]
  __int64 v47; // [rsp+60h] [rbp-71h]
  _BYTE v48[8]; // [rsp+68h] [rbp-69h] BYREF
  volatile signed __int32 *v49; // [rsp+70h] [rbp-61h]
  _BYTE v50[16]; // [rsp+78h] [rbp-59h] BYREF
  _BYTE v51[8]; // [rsp+88h] [rbp-49h] BYREF
  volatile signed __int32 *v52; // [rsp+90h] [rbp-41h]
  _BYTE v53[16]; // [rsp+98h] [rbp-39h] BYREF
  _BYTE v54[8]; // [rsp+A8h] [rbp-29h] BYREF
  volatile signed __int32 *v55; // [rsp+B0h] [rbp-21h]
  _BYTE v56[16]; // [rsp+B8h] [rbp-19h] BYREF
  _BYTE v57[20]; // [rsp+C8h] [rbp-9h] BYREF
  _BYTE v58[20]; // [rsp+DCh] [rbp+Bh] BYREF
  _BYTE v59[24]; // [rsp+F0h] [rbp+1Fh] BYREF

  v47 = -2;
  v2 = 0;
  v3 = qword_14E659EA8;
  if ( !qword_14E659EA8 )
  {
    v4 = sub_146E8BA20(496);
    if ( v4 )
      v5 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v4);
    else
      v5 = 0;
    qword_14E659EA8 = (__int64)v5;
    (**v5)(v5);
    v3 = qword_14E659EA8;
  }
  v6 = *(_DWORD *)(sub_14449E0D0(v3, v57) + 8);
  v7 = qword_14E659EA8;
  if ( !qword_14E659EA8 )
  {
    v8 = sub_146E8BA20(496);
    if ( v8 )
      v9 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v8);
    else
      v9 = 0;
    qword_14E659EA8 = (__int64)v9;
    (**v9)(v9);
    v7 = qword_14E659EA8;
  }
  v10 = *(_DWORD *)(sub_14449E0D0(v7, v58) + 12);
  v11 = qword_14E659EA8;
  if ( !qword_14E659EA8 )
  {
    v12 = sub_146E8BA20(496);
    if ( v12 )
      v2 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v12);
    qword_14E659EA8 = (__int64)v2;
    (**v2)(v2);
    v11 = qword_14E659EA8;
  }
  v13 = *(_DWORD *)(sub_14449E0D0(v11, v59) + 16);
  v14 = *(_QWORD *)(a1 + 1672);
  if ( v14 )
  {
    v15 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v14 + 688LL);
    v16 = sub_14723C170(100001120);
    v15(v14, v16);
  }
  v17 = sub_145F6E370(a1, v48, 1);
  sub_1401E9D00(&v41, v17);
  v18 = v49;
  if ( v49 )
  {
    if ( _InterlockedExchangeAdd(v49 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v18)(v18);
      if ( _InterlockedExchangeAdd(v18 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v18 + 8LL))(v18);
    }
  }
  v19 = v41;
  if ( v41 )
  {
    v20 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v41 + 688LL);
    v21 = sub_14723C170(100001123);
    v22 = sub_146E8CF20(v50, v21, v6);
    v23 = sub_14014F430(v22);
    v20(v19, v23);
    sub_146E8C910(v50);
  }
  v24 = v42;
  if ( v42 )
  {
    if ( _InterlockedExchangeAdd(v42 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v24)(v24);
      if ( _InterlockedExchangeAdd(v24 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v24 + 8LL))(v24);
    }
  }
  v25 = sub_145F6E370(a1, v51, 2);
  sub_1401E9D00(&v43, v25);
  v26 = v52;
  if ( v52 )
  {
    if ( _InterlockedExchangeAdd(v52 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v26)(v26);
      if ( _InterlockedExchangeAdd(v26 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v26 + 8LL))(v26);
    }
  }
  v27 = v43;
  if ( v43 )
  {
    v28 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v43 + 688LL);
    v29 = sub_14723C170(100001124);
    v30 = sub_146E8CF20(v53, v29, v10);
    v31 = sub_14014F430(v30);
    v28(v27, v31);
    sub_146E8C910(v53);
  }
  v32 = v44;
  if ( v44 )
  {
    if ( _InterlockedExchangeAdd(v44 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v32)(v32);
      if ( _InterlockedExchangeAdd(v32 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v32 + 8LL))(v32);
    }
  }
  v33 = sub_145F6E370(a1, v54, 3);
  sub_1401E9D00(&v45, v33);
  v34 = v55;
  if ( v55 )
  {
    if ( _InterlockedExchangeAdd(v55 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v34)(v34);
      if ( _InterlockedExchangeAdd(v34 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v34 + 8LL))(v34);
    }
  }
  v35 = v45;
  if ( v45 )
  {
    v36 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v45 + 688LL);
    v37 = sub_14723C170(100001125);
    v38 = sub_146E8CF20(v56, v37, v13);
    v39 = sub_14014F430(v38);
    v36(v35, v39);
    sub_146E8C910(v56);
  }
  v40 = v46;
  if ( v46 && _InterlockedExchangeAdd(v46 + 2, 0xFFFFFFFF) == 1 )
  {
    (**(void (__fastcall ***)(volatile signed __int32 *))v40)(v40);
    if ( _InterlockedExchangeAdd(v40 + 3, 0xFFFFFFFF) == 1 )
      (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v40 + 8LL))(v40);
  }
}

