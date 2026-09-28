// cand_0x1447eaf90

char __fastcall sub_1447EAF90(__int64 a1, __int64 a2, __int64 a3)
{
  int v3; // r12d
  __int64 v5; // rax
  __int64 v6; // r15
  __int64 v7; // r8
  int **v8; // rax
  char v9; // bl
  int *v10; // rdx
  int *v11; // rcx
  int *v12; // rcx
  int *v13; // rcx
  __int64 v14; // rsi
  _QWORD *v15; // r14
  _QWORD *v16; // rdx
  __int64 v17; // rax
  __int64 v18; // rdx
  __int64 v19; // rcx
  volatile signed __int32 *v20; // rbx
  volatile signed __int32 *v21; // rbx
  __int64 v22; // rdx
  _QWORD *v23; // rdx
  __int64 v24; // rdx
  _QWORD *v25; // rdx
  volatile signed __int32 *v26; // rbx
  __int64 v27; // rdx
  _QWORD *v28; // rdx
  __int64 v29; // rdx
  _QWORD *v30; // rdx
  volatile signed __int32 *v31; // rbx
  __int64 *v32; // rsi
  __int64 *v33; // rax
  __int64 *v34; // rcx
  __int64 v35; // rbx
  int *v36; // rcx
  int *v37; // rcx
  int *v39; // [rsp+28h] [rbp-D8h]
  __int64 v40; // [rsp+38h] [rbp-C8h] BYREF
  volatile signed __int32 *v41; // [rsp+40h] [rbp-C0h]
  __int64 v42; // [rsp+48h] [rbp-B8h] BYREF
  volatile signed __int32 *v43; // [rsp+50h] [rbp-B0h]
  int *v44; // [rsp+58h] [rbp-A8h] BYREF
  int *v45; // [rsp+60h] [rbp-A0h] BYREF
  __int128 v46; // [rsp+70h] [rbp-90h] BYREF
  __int128 v47; // [rsp+80h] [rbp-80h] BYREF
  __int128 v48; // [rsp+98h] [rbp-68h]
  __int64 v49; // [rsp+A8h] [rbp-58h]
  int *v50; // [rsp+B0h] [rbp-50h] BYREF
  _BYTE v51[128]; // [rsp+B8h] [rbp-48h] BYREF
  char v52[8]; // [rsp+138h] [rbp+38h] BYREF
  _QWORD *v53; // [rsp+140h] [rbp+40h]
  _QWORD *v54; // [rsp+148h] [rbp+48h]
  char v55[8]; // [rsp+150h] [rbp+50h] BYREF
  _QWORD *v56; // [rsp+158h] [rbp+58h]
  _QWORD *v57; // [rsp+160h] [rbp+60h]
  signed int v58; // [rsp+170h] [rbp+70h]
  _BYTE v59[184]; // [rsp+178h] [rbp+78h] BYREF

  v49 = -2;
  v3 = a2;
  LOBYTE(a3) = 1;
  v5 = sub_140283D60(qword_14E683BF8, a2, a3);
  v6 = v5;
  if ( !v5 || *(_DWORD *)(v5 + 8) != 2 )
    return 0;
  sub_1447E46E0(&v50);
  (*(void (__fastcall **)(__int64, int **))(*(_QWORD *)qword_14F1C39C8 + 16LL))(qword_14F1C39C8, &v45);
  if ( v45 )
  {
    v8 = (int **)(*(__int64 (__fastcall **)(int *, __int128 *, _QWORD))(*(_QWORD *)v45 + 120LL))(
                   v45,
                   &v46,
                   *(int *)(v6 + 384));
    v9 = 1;
    v10 = *v8;
  }
  else
  {
    v44 = 0;
    v8 = &v44;
    v9 = 2;
    v10 = 0;
  }
  *v8 = 0;
  v11 = v50;
  v39 = v50;
  v50 = v10;
  if ( v39 )
  {
    if ( --v11[2] <= 0 )
      (*(void (__fastcall **)(int *))(*(_QWORD *)v11 + 8LL))(v11);
  }
  if ( (v9 & 2) != 0 )
  {
    v9 &= ~2u;
    v12 = v44;
    if ( v44 )
    {
      --v44[2];
      if ( v12[2] <= 0 )
        (*(void (__fastcall **)(int *))(*(_QWORD *)v12 + 8LL))(v12);
    }
  }
  if ( (v9 & 1) != 0 )
  {
    v13 = (int *)v46;
    if ( (_QWORD)v46 )
    {
      --*(_DWORD *)(v46 + 8);
      if ( v13[2] <= 0 )
        (*(void (__fastcall **)(int *))(*(_QWORD *)v13 + 8LL))(v13);
    }
  }
  v14 = 0;
  v15 = v51;
  do
  {
    v16 = (_QWORD *)(v14 + *(_QWORD *)(v6 + 680));
    if ( v16[3] >= 8u )
      v16 = (_QWORD *)*v16;
    LOBYTE(v7) = 1;
    v17 = sub_144724390(&v47, v16, v7, 0);
    v48 = 0;
    v48 = *(_OWORD *)v17;
    v18 = *((_QWORD *)&v48 + 1);
    v19 = v48;
    *(_QWORD *)v17 = 0;
    *(_QWORD *)(v17 + 8) = 0;
    *(_QWORD *)&v48 = *v15;
    *v15 = v19;
    *((_QWORD *)&v48 + 1) = v15[1];
    v20 = (volatile signed __int32 *)*((_QWORD *)&v48 + 1);
    v15[1] = v18;
    if ( v20 )
    {
      if ( _InterlockedExchangeAdd(v20 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v20)(v20);
        if ( _InterlockedExchangeAdd(v20 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v20 + 8LL))(v20);
      }
    }
    v21 = (volatile signed __int32 *)*((_QWORD *)&v47 + 1);
    if ( *((_QWORD *)&v47 + 1) )
    {
      if ( _InterlockedExchangeAdd((volatile signed __int32 *)(*((_QWORD *)&v47 + 1) + 8LL), 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v21)(v21);
        if ( _InterlockedExchangeAdd(v21 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v21 + 8LL))(v21);
      }
    }
    LOBYTE(v18) = 1;
    sub_146B5AA70(*v15, v18);
    v14 += 32;
    v15 += 2;
  }
  while ( v14 < 256 );
  v22 = *(_QWORD *)(v6 + 680);
  if ( (unsigned __int64)((*(_QWORD *)(v6 + 688) - v22) >> 5) > 8 )
  {
    v23 = (_QWORD *)(v22 + 256);
    if ( v23[3] >= 8u )
      v23 = (_QWORD *)*v23;
    LOBYTE(v7) = 1;
    sub_144724390(&v40, v23, v7, 0);
    LOBYTE(v24) = 1;
    sub_146B5AA70(v40, v24);
    v25 = v53;
    if ( v53 == v54 )
    {
      sub_1401E7AA0(v52, v53, &v40);
    }
    else
    {
      *v53 = 0;
      v25[1] = 0;
      if ( v41 )
        _InterlockedIncrement(v41 + 2);
      *v25 = v40;
      v25[1] = v41;
      v53 += 2;
    }
    v26 = v41;
    if ( v41 )
    {
      if ( _InterlockedExchangeAdd(v41 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v26)(v26);
        if ( _InterlockedExchangeAdd(v26 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v26 + 8LL))(v26);
      }
    }
  }
  v27 = *(_QWORD *)(v6 + 680);
  if ( (unsigned __int64)((*(_QWORD *)(v6 + 688) - v27) >> 5) > 9 )
  {
    v28 = (_QWORD *)(v27 + 288);
    if ( v28[3] >= 8u )
      v28 = (_QWORD *)*v28;
    LOBYTE(v7) = 1;
    sub_144724390(&v42, v28, v7, 0);
    LOBYTE(v29) = 1;
    sub_146B5AA70(v42, v29);
    v30 = v56;
    if ( v56 == v57 )
    {
      sub_1401E7AA0(v55, v56, &v42);
    }
    else
    {
      *v56 = 0;
      v30[1] = 0;
      if ( v43 )
        _InterlockedIncrement(v43 + 2);
      *v30 = v42;
      v30[1] = v43;
      v56 += 2;
    }
    v31 = v43;
    if ( v43 )
    {
      if ( _InterlockedExchangeAdd(v43 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v31)(v31);
        if ( _InterlockedExchangeAdd(v31 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v31 + 8LL))(v31);
      }
    }
  }
  v58 = v3;
  sub_1447E4640(v59, &v50);
  v32 = *(__int64 **)(a1 + 96);
  v33 = (__int64 *)v32[1];
  *(_QWORD *)&v47 = v33;
  DWORD2(v47) = 0;
  v34 = v32;
  while ( !*((_BYTE *)v33 + 25) )
  {
    *(_QWORD *)&v47 = v33;
    if ( *((_DWORD *)v33 + 8) >= v58 )
    {
      DWORD2(v47) = 1;
      v34 = v33;
      v33 = (__int64 *)*v33;
    }
    else
    {
      DWORD2(v47) = 0;
      v33 = (__int64 *)v33[2];
    }
  }
  if ( *((_BYTE *)v34 + 25) || v58 < *((_DWORD *)v34 + 8) )
  {
    if ( *(_QWORD *)(a1 + 104) == 0x124924924924924LL )
      sub_14014F360(v34, (unsigned int)v58);
    v35 = sub_146E8BA20(224);
    *(_QWORD *)&v46 = v35 + 32;
    *(_DWORD *)(v35 + 32) = v58;
    sub_1447E4640(v35 + 40, v59);
    *(_QWORD *)v35 = v32;
    *(_QWORD *)(v35 + 8) = v32;
    *(_QWORD *)(v35 + 16) = v32;
    *(_WORD *)(v35 + 24) = 0;
    v46 = v47;
    sub_14014F0E0(a1 + 96, &v46, v35);
  }
  sub_1447E50C0(v59);
  v36 = v45;
  if ( v45 )
  {
    --v45[2];
    if ( v36[2] <= 0 )
      (*(void (__fastcall **)(int *))(*(_QWORD *)v36 + 8LL))(v36);
  }
  sub_1401EF0B0(v55);
  sub_1401EF0B0(v52);
  sub_1488606A0(v51, 16, 8, sub_1401566D0);
  *(_QWORD *)&v46 = &v50;
  v37 = v50;
  if ( v50 )
  {
    --v50[2];
    if ( v37[2] <= 0 )
      (*(void (__fastcall **)(int *))(*(_QWORD *)v37 + 8LL))(v37);
  }
  return 1;
}

