// elemmethod_sub_1441CFEC0_0x1441cfec0

void __fastcall sub_1441CFEC0(__int64 a1, __int64 a2, __int64 *a3)
{
  int v5; // ebp
  volatile signed __int32 *v6; // rbx
  __int64 v7; // rbx
  __int64 v8; // rax
  __int64 v9; // rax
  volatile signed __int32 *v10; // rbx
  __int64 v11; // rbx
  __int64 v12; // rax
  __int64 v13; // rax
  __int64 v14; // rax
  volatile signed __int32 *v15; // rbx
  volatile signed __int32 *v16; // rbx
  _QWORD *v17; // r15
  __int64 v18; // r13
  __int64 v19; // rax
  __int64 v20; // rax
  __int64 v21; // rax
  __int64 v22; // r8
  void (__fastcall *v23)(__int64, __int64); // rbx
  __int64 *v24; // r8
  __int64 v25; // rax
  __int64 v26; // rbx
  __int64 v27; // rax
  __int64 v28; // rax
  volatile signed __int32 *v29; // rbx
  __int64 v30; // rcx
  __int64 v31[3]; // [rsp+20h] [rbp-B8h] BYREF
  __int64 v32; // [rsp+38h] [rbp-A0h]
  _BYTE v33[8]; // [rsp+40h] [rbp-98h] BYREF
  volatile signed __int32 *v34; // [rsp+48h] [rbp-90h]
  _BYTE v35[8]; // [rsp+50h] [rbp-88h] BYREF
  volatile signed __int32 *v36; // [rsp+58h] [rbp-80h]
  _BYTE v37[8]; // [rsp+60h] [rbp-78h] BYREF
  volatile signed __int32 *v38; // [rsp+68h] [rbp-70h]
  _BYTE v39[16]; // [rsp+70h] [rbp-68h] BYREF
  _BYTE v40[16]; // [rsp+80h] [rbp-58h] BYREF
  _BYTE v41[8]; // [rsp+90h] [rbp-48h] BYREF
  volatile signed __int32 *v42; // [rsp+98h] [rbp-40h]

  v32 = -2;
  v5 = 0;
  if ( !*a3 )
  {
    v6 = (volatile signed __int32 *)a3[1];
    if ( !v6 )
      return;
    goto LABEL_30;
  }
  *(_QWORD *)(a1 + 8) = a2;
  if ( dword_14E662BF8 > *(_DWORD *)(*((_QWORD *)NtCurrentTeb()->ThreadLocalStoragePointer
                                     + (unsigned int)dword_14F3BEE58)
                                   + 420620LL) )
  {
    sub_148860450(&dword_14E662BF8);
    if ( dword_14E662BF8 == -1 )
    {
      qword_14E662BD8 = 0;
      qword_14E662BE8 = 0;
      qword_14E662BF0 = 7;
      sub_14014C8D0(&qword_14E662BD8, &byte_14BAF7F08);
      sub_14885FFE8(sub_149024480);
      sub_1488603F0(&dword_14E662BF8);
    }
  }
  v7 = *a3;
  v8 = sub_146E8C7D0(&unk_14928FEA8);
  v9 = sub_146EC8E30(v7, v33, v8);
  sub_1401E5080(a1 + 136, v9);
  v10 = v34;
  if ( v34 )
  {
    if ( _InterlockedExchangeAdd(v34 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v10)(v10);
      if ( _InterlockedExchangeAdd(v10 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v10 + 8LL))(v10);
    }
  }
  v11 = *(_QWORD *)(a1 + 136);
  v12 = sub_146E8C7D0(&unk_1496A6D08);
  v13 = sub_146EC8E30(v11, v37, v12);
  v14 = sub_1404D51A0(v35, v13);
  sub_1401E5080(a1 + 112, v14);
  v15 = v36;
  if ( v36 )
  {
    if ( _InterlockedExchangeAdd(v36 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v15)(v15);
      if ( _InterlockedExchangeAdd(v15 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v15 + 8LL))(v15);
    }
  }
  v16 = v38;
  if ( v38 )
  {
    if ( _InterlockedExchangeAdd(v38 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v16)(v16);
      if ( _InterlockedExchangeAdd(v16 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v16 + 8LL))(v16);
    }
  }
  *(_DWORD *)(a1 + 2420) = 17;
  v17 = (_QWORD *)(a1 + 376);
  v18 = a1 + 152;
  do
  {
    v19 = sub_146E8C7D0(&unk_14A2322B8);
    v20 = sub_146E8CF20(v39, v19, (unsigned int)v5);
    v21 = sub_14014F430(v20);
    v22 = -1;
    do
      ++v22;
    while ( *(_WORD *)(v21 + 2 * v22) );
    sub_14014C8D0(&qword_14E662BD8, v21);
    sub_146E8C910(v39);
    v23 = *(void (__fastcall **)(__int64, __int64))*(v17 - 28);
    v24 = &qword_14E662BD8;
    if ( (unsigned __int64)qword_14E662BF0 >= 8 )
      v24 = (__int64 *)qword_14E662BD8;
    v25 = sub_146EC8E30(*(_QWORD *)(a1 + 136), v40, v24);
    v23(v18, v25);
    (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)*v17 + 16LL))(*v17, 0);
    (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)v17[2] + 16LL))(v17[2], 0);
    ++v5;
    v18 += 320;
    v17 += 40;
  }
  while ( v5 < 7 );
  v26 = *(_QWORD *)(a1 + 136);
  v27 = sub_146E8C7D0(&unk_14A232370);
  v28 = sub_146EC8E30(v26, v41, v27);
  sub_1401E5080(a1 + 2424, v28);
  v29 = v42;
  if ( v42 )
  {
    if ( _InterlockedExchangeAdd(v42 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v29)(v29);
      if ( _InterlockedExchangeAdd(v29 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v29 + 8LL))(v29);
    }
  }
  *(_OWORD *)v31 = 0;
  v30 = a3[1];
  if ( v30 )
  {
    _InterlockedIncrement((volatile signed __int32 *)(v30 + 8));
    v30 = a3[1];
  }
  v31[0] = *a3;
  v31[1] = v30;
  sub_1441CBA30((_QWORD *)a1, a2, v31);
  v6 = (volatile signed __int32 *)a3[1];
  if ( v6 )
  {
LABEL_30:
    if ( _InterlockedExchangeAdd(v6 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v6)(v6);
      if ( _InterlockedExchangeAdd(v6 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v6 + 8LL))(v6);
    }
  }
}

