// method_sub_1441CEA40_0x1441cea40

__int64 __fastcall sub_1441CEA40(__int64 a1, __int64 a2, __int64 *a3)
{
  __int64 result; // rax
  unsigned int v6; // r14d
  volatile signed __int32 *v7; // rbx
  __int64 v8; // rbx
  __int64 v9; // rax
  __int64 v10; // rax
  _QWORD *v11; // r12
  volatile signed __int32 *v12; // rbx
  __int64 v13; // rbx
  __int64 v14; // rax
  __int64 v15; // rax
  __int64 v16; // rax
  volatile signed __int32 *v17; // rbx
  volatile signed __int32 *v18; // rbx
  __int64 v19; // rbx
  __int64 v20; // rax
  __int64 v21; // rax
  __int64 v22; // rax
  volatile signed __int32 *v23; // rbx
  volatile signed __int32 *v24; // rbx
  __int64 v25; // rbx
  __int64 v26; // rax
  __int64 v27; // rax
  __int64 v28; // rax
  volatile signed __int32 *v29; // rbx
  volatile signed __int32 *v30; // rbx
  __int64 v31; // rbx
  __int64 v32; // rax
  __int64 v33; // rax
  volatile signed __int32 *v34; // rbx
  __int64 v35; // rdi
  void (__fastcall *v36)(__int64, __int64); // rbx
  __int64 v37; // rax
  __int64 v38; // rdx
  __int64 v39; // rbx
  __int64 v40; // rax
  __int64 v41; // rax
  __int64 v42; // rax
  volatile signed __int32 *v43; // rbx
  volatile signed __int32 *v44; // rbx
  int v45; // edi
  void (__fastcall ***v46)(__int64, __int64); // r15
  __int64 v47; // rax
  __int64 v48; // rax
  __int64 v49; // rax
  __int64 v50; // r8
  void (__fastcall *v51)(__int64, __int64); // rbx
  __int64 *v52; // r8
  __int64 v53; // rax
  __int64 v54; // rcx
  __int64 v55; // rbx
  __int64 v56; // rax
  __int64 v57; // rax
  __int64 v58; // rax
  volatile signed __int32 *v59; // rbx
  volatile signed __int32 *v60; // rbx
  __int64 v61; // rax
  __int64 v62; // rax
  __int64 v63; // rax
  __int64 v64; // r8
  __int64 *v65; // r8
  volatile signed __int32 *v66; // rbx
  volatile signed __int32 *v67; // rbx
  _QWORD v68[3]; // [rsp+20h] [rbp-E8h] BYREF
  __int64 v69; // [rsp+40h] [rbp-C8h] BYREF
  volatile signed __int32 *v70; // [rsp+48h] [rbp-C0h]
  __int64 v71; // [rsp+50h] [rbp-B8h]
  __int64 v72; // [rsp+58h] [rbp-B0h] BYREF
  volatile signed __int32 *v73; // [rsp+60h] [rbp-A8h]
  __int64 v74; // [rsp+68h] [rbp-A0h] BYREF
  volatile signed __int32 *v75; // [rsp+70h] [rbp-98h]
  __int64 v76; // [rsp+78h] [rbp-90h] BYREF
  volatile signed __int32 *v77; // [rsp+80h] [rbp-88h]
  _BYTE v78[8]; // [rsp+88h] [rbp-80h] BYREF
  volatile signed __int32 *v79; // [rsp+90h] [rbp-78h]
  _BYTE v80[8]; // [rsp+98h] [rbp-70h] BYREF
  volatile signed __int32 *v81; // [rsp+A0h] [rbp-68h]
  _BYTE v82[8]; // [rsp+A8h] [rbp-60h] BYREF
  volatile signed __int32 *v83; // [rsp+B0h] [rbp-58h]
  _BYTE v84[8]; // [rsp+B8h] [rbp-50h] BYREF
  volatile signed __int32 *v85; // [rsp+C0h] [rbp-48h]
  _BYTE v86[8]; // [rsp+C8h] [rbp-40h] BYREF
  volatile signed __int32 *v87; // [rsp+D0h] [rbp-38h]
  _BYTE v88[8]; // [rsp+D8h] [rbp-30h] BYREF
  volatile signed __int32 *v89; // [rsp+E0h] [rbp-28h]
  _BYTE v90[8]; // [rsp+E8h] [rbp-20h] BYREF
  volatile signed __int32 *v91; // [rsp+F0h] [rbp-18h]
  _BYTE v92[16]; // [rsp+F8h] [rbp-10h] BYREF
  _BYTE v93[16]; // [rsp+108h] [rbp+0h] BYREF
  _BYTE v94[8]; // [rsp+118h] [rbp+10h] BYREF
  volatile signed __int32 *v95; // [rsp+120h] [rbp+18h]
  _BYTE v96[8]; // [rsp+128h] [rbp+20h] BYREF
  volatile signed __int32 *v97; // [rsp+130h] [rbp+28h]
  _BYTE v98[64]; // [rsp+138h] [rbp+30h] BYREF
  _UNKNOWN *retaddr; // [rsp+180h] [rbp+78h] BYREF
  __int64 v102; // [rsp+1A0h] [rbp+98h]

  result = (__int64)&retaddr;
  v71 = -2;
  v6 = 0;
  if ( !*a3 )
  {
    v7 = (volatile signed __int32 *)a3[1];
    if ( !v7 )
      return result;
    goto LABEL_82;
  }
  *(_QWORD *)(a1 + 8) = a2;
  if ( dword_14E662BA8 > *(_DWORD *)(*((_QWORD *)NtCurrentTeb()->ThreadLocalStoragePointer
                                     + (unsigned int)dword_14F3BEE58)
                                   + 420620LL) )
  {
    sub_148860450(&dword_14E662BA8);
    if ( dword_14E662BA8 == -1 )
    {
      qword_14E662B88 = 0;
      qword_14E662B98 = 0;
      qword_14E662BA0 = 7;
      sub_14014C8D0(&qword_14E662B88, &byte_14BAF7F08);
      sub_14885FFE8(sub_1490242C0);
      sub_1488603F0(&dword_14E662BA8);
    }
  }
  v8 = *a3;
  v9 = sub_146E8C7D0(&unk_14928FEA8);
  v10 = sub_146EC8E30(v8, &v72, v9);
  v11 = (_QWORD *)(a1 + 136);
  sub_1401E5080(a1 + 136, v10);
  v12 = v73;
  if ( v73 )
  {
    if ( _InterlockedExchangeAdd(v73 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v12)(v12);
      if ( _InterlockedExchangeAdd(v12 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v12 + 8LL))(v12);
    }
  }
  v13 = *v11;
  v14 = sub_146E8C7D0(&unk_1496A6D08);
  v15 = sub_146EC8E30(v13, &v76, v14);
  v16 = sub_1404D51A0(&v74, v15);
  sub_1401E5080(a1 + 112, v16);
  v17 = v75;
  if ( v75 )
  {
    if ( _InterlockedExchangeAdd(v75 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v17)(v17);
      if ( _InterlockedExchangeAdd(v17 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v17 + 8LL))(v17);
    }
  }
  v18 = v77;
  if ( v77 )
  {
    if ( _InterlockedExchangeAdd(v77 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v18)(v18);
      if ( _InterlockedExchangeAdd(v18 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v18 + 8LL))(v18);
    }
  }
  v19 = *v11;
  v20 = sub_146E8C7D0(&unk_1496B66D8);
  v21 = sub_146EC8E30(v19, v80, v20);
  v22 = sub_1401E9B50(v78, v21);
  sub_1401E5080(a1 + 152, v22);
  v23 = v79;
  if ( v79 )
  {
    if ( _InterlockedExchangeAdd(v79 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v23)(v23);
      if ( _InterlockedExchangeAdd(v23 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v23 + 8LL))(v23);
    }
  }
  v24 = v81;
  if ( v81 )
  {
    if ( _InterlockedExchangeAdd(v81 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v24)(v24);
      if ( _InterlockedExchangeAdd(v24 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v24 + 8LL))(v24);
    }
  }
  v25 = *v11;
  v26 = sub_146E8C7D0(&unk_14A232468);
  v27 = sub_146EC8E30(v25, v84, v26);
  v28 = sub_1401E9D00(v82, v27);
  sub_1401E5080(a1 + 1416, v28);
  v29 = v83;
  if ( v83 )
  {
    if ( _InterlockedExchangeAdd(v83 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v29)(v29);
      if ( _InterlockedExchangeAdd(v29 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v29 + 8LL))(v29);
    }
  }
  v30 = v85;
  if ( v85 )
  {
    if ( _InterlockedExchangeAdd(v85 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v30)(v30);
      if ( _InterlockedExchangeAdd(v30 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v30 + 8LL))(v30);
    }
  }
  v31 = *v11;
  v32 = sub_146E8C7D0(&unk_14A232488);
  v33 = sub_146EC8E30(v31, v86, v32);
  sub_1401E5080(a1 + 1432, v33);
  v34 = v87;
  if ( v87 )
  {
    if ( _InterlockedExchangeAdd(v87 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v34)(v34);
      if ( _InterlockedExchangeAdd(v34 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v34 + 8LL))(v34);
    }
  }
  v35 = *(_QWORD *)(a1 + 1416);
  v36 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v35 + 688LL);
  v37 = sub_14723C170(100002263);
  v36(v35, v37);
  v38 = *(_QWORD *)(a1 + 8);
  if ( v38 )
    (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 1416) + 664LL))(*(_QWORD *)(a1 + 1416), v38 + 17696);
  v39 = *v11;
  v40 = sub_146E8C7D0(&unk_14A2324B0);
  v41 = sub_146EC8E30(v39, v90, v40);
  v42 = sub_1401E9B50(v88, v41);
  sub_1401E5080(a1 + 1472, v42);
  v43 = v89;
  if ( v89 )
  {
    if ( _InterlockedExchangeAdd(v89 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v43)(v43);
      if ( _InterlockedExchangeAdd(v43 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v43 + 8LL))(v43);
    }
  }
  v44 = v91;
  if ( v91 )
  {
    if ( _InterlockedExchangeAdd(v91 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v44)(v44);
      if ( _InterlockedExchangeAdd(v44 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v44 + 8LL))(v44);
    }
  }
  v45 = 0;
  v46 = (void (__fastcall ***)(__int64, __int64))(a1 + 168);
  v102 = a1 + 168;
  do
  {
    v47 = sub_146E8C7D0(&unk_14A2322B8);
    v48 = sub_146E8CF20(v92, v47, (unsigned int)v45);
    v49 = sub_14014F430(v48);
    v50 = -1;
    do
      ++v50;
    while ( *(_WORD *)(v49 + 2 * v50) );
    sub_14014C8D0(&qword_14E662B88, v49);
    sub_146E8C910(v92);
    v51 = **v46;
    v68[1] = v93;
    v52 = &qword_14E662B88;
    if ( (unsigned __int64)qword_14E662BA0 >= 8 )
      v52 = (__int64 *)qword_14E662B88;
    v53 = sub_146EC8E30(*v11, v93, v52);
    v51(v102, v53);
    ++v45;
    v102 += 416;
    v46 += 52;
  }
  while ( v45 < 3 );
  *(_OWORD *)&v68[1] = 0;
  v54 = a3[1];
  if ( v54 )
  {
    _InterlockedIncrement((volatile signed __int32 *)(v54 + 8));
    v54 = a3[1];
  }
  v68[1] = *a3;
  v68[2] = v54;
  sub_1441CBA30((_QWORD *)a1, a2, &v68[1]);
  *(_DWORD *)(a1 + 1448) = 2;
  v55 = *v11;
  v56 = sub_146E8C7D0(&unk_14A232320);
  v57 = sub_146EC8E30(v55, v96, v56);
  v58 = sub_140462E10(v94, v57);
  result = sub_1401E5080(a1 + 1456, v58);
  v59 = v95;
  if ( v95 )
  {
    result = (unsigned int)_InterlockedExchangeAdd(v95 + 2, 0xFFFFFFFF);
    if ( (_DWORD)result == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v59)(v59);
      result = (unsigned int)_InterlockedExchangeAdd(v59 + 3, 0xFFFFFFFF);
      if ( (_DWORD)result == 1 )
        result = (*(__int64 (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v59 + 8LL))(v59);
    }
  }
  v60 = v97;
  if ( v97 )
  {
    result = (unsigned int)_InterlockedExchangeAdd(v97 + 2, 0xFFFFFFFF);
    if ( (_DWORD)result == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v60)(v60);
      result = (unsigned int)_InterlockedExchangeAdd(v60 + 3, 0xFFFFFFFF);
      if ( (_DWORD)result == 1 )
        result = (*(__int64 (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v60 + 8LL))(v60);
    }
  }
  if ( *(_QWORD *)(a1 + 1456) )
  {
    while ( 1 )
    {
      v61 = sub_146E8C7D0(&unk_14A232338);
      v62 = sub_146E8CF20(v98, v61, v6);
      v63 = sub_14014F430(v62);
      v64 = -1;
      do
        ++v64;
      while ( *(_WORD *)(v63 + 2 * v64) );
      sub_14014C8D0(&qword_14E662B88, v63);
      sub_146E8C910(v98);
      v65 = &qword_14E662B88;
      if ( (unsigned __int64)qword_14E662BA0 >= 8 )
        v65 = (__int64 *)qword_14E662B88;
      sub_146EC8E30(*(_QWORD *)(a1 + 1456), &v69, v65);
      if ( v69 )
      {
        if ( (unsigned __int8)sub_141FB6530(v69) )
          break;
      }
      v66 = v70;
      if ( v70 )
      {
        if ( _InterlockedExchangeAdd(v70 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v66)(v66);
          if ( _InterlockedExchangeAdd(v66 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v66 + 8LL))(v66);
        }
      }
      if ( (int)++v6 >= 2 )
        goto LABEL_80;
    }
    *(_DWORD *)(a1 + 1448) = v6;
    v67 = v70;
    if ( v70 )
    {
      if ( _InterlockedExchangeAdd(v70 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v67)(v67);
        if ( _InterlockedExchangeAdd(v67 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v67 + 8LL))(v67);
      }
    }
LABEL_80:
    result = sub_146F53250(*(_QWORD *)(a1 + 1456), *(unsigned int *)(a1 + 1448), 0);
  }
  v7 = (volatile signed __int32 *)a3[1];
  if ( v7 )
  {
LABEL_82:
    result = (unsigned int)_InterlockedExchangeAdd(v7 + 2, 0xFFFFFFFF);
    if ( (_DWORD)result == 1 )
    {
      result = (**(__int64 (__fastcall ***)(volatile signed __int32 *))v7)(v7);
      if ( _InterlockedExchangeAdd(v7 + 3, 0xFFFFFFFF) == 1 )
        return (*(__int64 (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v7 + 8LL))(v7);
    }
  }
  return result;
}

