__int64 __fastcall sub_14603E2D0(__int64 a1, unsigned int a2, __int64 a3)
{
  __int64 v3; // rdi
  __int64 v6; // rcx
  __int64 v7; // rax
  void (__fastcall ***v8)(_QWORD); // rcx
  __int64 v9; // rsi
  __int64 v10; // rax
  __int64 v11; // rax
  __int64 v12; // r14
  unsigned __int8 v13; // si
  __int64 v14; // rax
  unsigned int *v15; // rbx
  int v16; // r9d
  int v17; // r8d
  unsigned int v18; // ebx
  int v19; // eax
  __int64 v20; // rax
  int v21; // edx
  int v22; // ecx
  int v23; // eax
  __int64 v24; // rax
  unsigned int *v25; // rbx
  int v26; // r9d
  int v27; // r8d
  int v28; // ecx
  int v29; // esi
  __int64 v30; // rax
  void (__fastcall ***v31)(_QWORD); // rcx
  __int64 v32; // rax
  __int64 v33; // rax
  __int64 v34; // rbx
  int v35; // edi
  int v36; // eax
  __int64 v37; // rsi
  _QWORD *v38; // rbx
  _QWORD *v39; // rcx
  volatile signed __int32 *v40; // rbx
  volatile signed __int32 *v41; // rbx
  __int64 (__fastcall **v43)(); // [rsp+48h] [rbp-69h] BYREF
  __int128 v44; // [rsp+50h] [rbp-61h]
  __int64 v45; // [rsp+60h] [rbp-51h]
  _QWORD *v46; // [rsp+68h] [rbp-49h]
  __int128 v47; // [rsp+78h] [rbp-39h]
  __int128 v48; // [rsp+88h] [rbp-29h]
  _BYTE v49[24]; // [rsp+98h] [rbp-19h] BYREF
  __int64 v50; // [rsp+B0h] [rbp-1h]
  __int128 v51; // [rsp+B8h] [rbp+7h]
  _BYTE v52[16]; // [rsp+C8h] [rbp+17h] BYREF
  __int128 v53; // [rsp+D8h] [rbp+27h]
  void *retaddr; // [rsp+110h] [rbp+5Fh]
  __int64 v55; // [rsp+128h] [rbp+77h] BYREF

  v50 = -2;
  v3 = a3;
  if ( !a3 )
    v3 = a1 + 32;
  v43 = &off_149254B70;
  v51 = 0;
  v47 = 0u;
  v44 = 0u;
  v6 = qword_14E634248;
  if ( !qword_14E634248 )
  {
    v7 = sub_146E8BA20(2496);
    v55 = v7;
    if ( v7 )
      v8 = (void (__fastcall ***)(_QWORD))sub_1456918F0(v7);
    else
      v8 = 0;
    qword_14E634248 = (__int64)v8;
    (**v8)(v8);
    v6 = qword_14E634248;
  }
  if ( !(unsigned __int8)sub_145695000(v6, 484)
    && (sub_1470909B0(&unk_14E6B4590, a2) || sub_1470909B0(&unk_14E6B4600, a2) || sub_1470909B0(&unk_14E6B4670, a2)) )
  {
    v9 = 0;
  }
  else
  {
    if ( sub_1470909B0(&unk_14F357D10, a2) )
    {
      v10 = sub_14564D850(a1 + 8, v49, a2, v3);
      sub_1402DCDC0(&v43, *(_QWORD *)(v10 + 8));
      sub_14033E820(v49);
    }
    else if ( sub_1470909B0(&unk_14F35DC90, a2) )
    {
      v11 = sub_14592A8F0(a1 + 9, v49, a2, v3);
      sub_1402DCDC0(&v43, *(_QWORD *)(v11 + 8));
      sub_142442620(v49);
    }
    v12 = v44;
    if ( (_QWORD)v44 )
    {
      v13 = *(_BYTE *)(v3 + 283);
      v14 = (*(__int64 (__fastcall **)(_QWORD))(*(_QWORD *)v44 + 152LL))(v44);
      v15 = (unsigned int *)(v14 + 24);
      LOBYTE(v16) = 1;
      LOBYTE(v17) = 50;
      sub_1480A6620(v14 + 24, 4, v17, v16, v14 + 28);
      v18 = *v15;
      v19 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v12 + 16LL))(v12);
      if ( v13 )
      {
        if ( v19 == 2 )
        {
          v20 = sub_145A70830(v18);
          if ( v20 )
          {
            if ( *(_BYTE *)(v20 + 7376) )
            {
              sub_146E920A0(v3 + 24, &v55);
              v21 = v55;
              v22 = *(_DWORD *)(v3 + 24) + v55 + 196;
              v23 = *(_DWORD *)(v3 + 28);
              if ( v23 && v22 && v23 != v22 && retaddr )
              {
                sub_146D89B40(retaddr, v3 + 24);
                v21 = v55;
              }
              if ( !v21 )
                goto LABEL_30;
            }
          }
        }
        v24 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v12 + 152LL))(v12);
        v25 = (unsigned int *)(v24 + 160);
        LOBYTE(v26) = 1;
        LOBYTE(v27) = 34;
        sub_1480A6620(v24 + 160, 4, v27, v26, v24 + 164);
        if ( *v25 > 8 || (v28 = 296, !_bittest(&v28, *v25)) )
LABEL_30:
          v13 = 0;
      }
      (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)v44 + 968LL))(v44, v13);
      sub_1450133D0(v44, v3, v13);
      sub_144FFE4B0(v44, v3, v13);
    }
    else if ( a2 != -1 )
    {
      v29 = qword_14E6343D0;
      if ( !qword_14E6343D0 )
      {
        v30 = sub_146E8BA20(72);
        v55 = v30;
        if ( v30 )
          v31 = (void (__fastcall ***)(_QWORD))sub_146E93360(v30);
        else
          v31 = 0;
        qword_14E6343D0 = (__int64)v31;
        (**v31)(v31);
        v29 = qword_14E6343D0;
      }
      v32 = sub_146E8C7D0(&unk_14A9B4020);
      v33 = sub_146E8CF20(v52, v32, a2);
      v34 = sub_14014F430(v33);
      v35 = sub_146E8C7D0(&unk_14A9B4070);
      v36 = sub_146E8C7D0(&unk_14A9B40B0);
      sub_146E938E0(v29, 0, v36, v35, 80, (__int64)&qword_14EF2DDC8, v34);
      sub_146E8C910(v52);
    }
    v37 = *(_QWORD *)(a1 + 16);
    if ( *(_QWORD *)(a1 + 24) == 0x666666666666666LL )
      sub_14883BB54("list too long");
    v45 = a1 + 16;
    v46 = 0;
    v38 = (_QWORD *)sub_146E8BA20(40);
    v46 = v38;
    sub_141211840(v38 + 2, &v43);
    ++*(_QWORD *)(a1 + 24);
    v39 = *(_QWORD **)(v37 + 8);
    *v38 = v37;
    v38[1] = v39;
    v46 = 0;
    *(_QWORD *)(v37 + 8) = v38;
    *v39 = v38;
    v9 = v44;
  }
  v43 = &off_149254B70;
  v53 = 0;
  v48 = v44;
  v40 = (volatile signed __int32 *)*((_QWORD *)&v44 + 1);
  v44 = 0u;
  if ( *((_QWORD *)&v48 + 1) )
  {
    if ( _InterlockedExchangeAdd(v40 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v40)(v40);
      if ( _InterlockedExchangeAdd(v40 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v40 + 8LL))(v40);
    }
  }
  v41 = (volatile signed __int32 *)*((_QWORD *)&v44 + 1);
  if ( *((_QWORD *)&v44 + 1) )
  {
    if ( _InterlockedExchangeAdd((volatile signed __int32 *)(*((_QWORD *)&v44 + 1) + 8LL), 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v41)(v41);
      if ( _InterlockedExchangeAdd(v41 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v41 + 8LL))(v41);
    }
  }
  return v9;
}
