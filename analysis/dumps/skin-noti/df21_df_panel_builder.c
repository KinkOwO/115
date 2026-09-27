// df_panel_builder = sub_1441CBDD0 0x1441CBDD0

void __fastcall sub_1441CBDD0(__int64 a1, __int64 a2, _QWORD *a3)
{
  volatile signed __int32 *v6; // rbx
  __int64 v7; // rcx
  __int64 v8; // rbx
  __int64 v9; // rax
  __int64 v10; // rax
  _QWORD *v11; // r13
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
  volatile signed __int32 *v28; // rbx
  __int64 v29; // rdi
  void (__fastcall *v30)(__int64, __int64); // rbx
  __int64 v31; // rax
  __int64 v32; // rdx
  int v33; // edi
  _QWORD *v34; // r12
  __int64 v35; // rax
  __int64 v36; // rax
  __int64 v37; // rax
  __int64 v38; // r8
  void (__fastcall *v39)(__int64, __int64); // rbx
  __int64 *v40; // r8
  __int64 v41; // rax
  __int64 v42; // rbx
  __int64 v43; // rax
  __int64 v44; // rax
  __int64 *v45; // rax
  __int64 v46; // rdi
  volatile signed __int32 *v47; // rbx
  volatile signed __int32 *v48; // rbx
  __int64 v49; // rbx
  __int64 v50; // rax
  __int64 v51; // rax
  __int64 *v52; // rax
  __int64 v53; // rdi
  volatile signed __int32 *v54; // rbx
  volatile signed __int32 *v55; // rbx
  __int64 v56; // rbx
  __int64 v57; // rax
  __int64 v58; // rax
  __int64 v59; // rax
  volatile signed __int32 *v60; // rbx
  volatile signed __int32 *v61; // rbx
  volatile signed __int32 *v62; // rbx
  __int64 v63; // [rsp+20h] [rbp-E0h]
  volatile signed __int32 *v64; // [rsp+28h] [rbp-D8h]
  __int64 v65; // [rsp+30h] [rbp-D0h]
  volatile signed __int32 *v66; // [rsp+38h] [rbp-C8h]
  __int128 v67; // [rsp+40h] [rbp-C0h] BYREF
  __int64 v68; // [rsp+58h] [rbp-A8h]
  _BYTE *v69; // [rsp+60h] [rbp-A0h]
  _BYTE v70[8]; // [rsp+68h] [rbp-98h] BYREF
  volatile signed __int32 *v71; // [rsp+70h] [rbp-90h]
  _BYTE v72[8]; // [rsp+78h] [rbp-88h] BYREF
  volatile signed __int32 *v73; // [rsp+80h] [rbp-80h]
  _BYTE v74[8]; // [rsp+88h] [rbp-78h] BYREF
  volatile signed __int32 *v75; // [rsp+90h] [rbp-70h]
  _BYTE v76[8]; // [rsp+98h] [rbp-68h] BYREF
  volatile signed __int32 *v77; // [rsp+A0h] [rbp-60h]
  _BYTE v78[8]; // [rsp+A8h] [rbp-58h] BYREF
  volatile signed __int32 *v79; // [rsp+B0h] [rbp-50h]
  _BYTE v80[8]; // [rsp+B8h] [rbp-48h] BYREF
  volatile signed __int32 *v81; // [rsp+C0h] [rbp-40h]
  _BYTE v82[16]; // [rsp+C8h] [rbp-38h] BYREF
  _BYTE v83[16]; // [rsp+D8h] [rbp-28h] BYREF
  _BYTE v84[8]; // [rsp+E8h] [rbp-18h] BYREF
  volatile signed __int32 *v85; // [rsp+F0h] [rbp-10h]
  _BYTE v86[8]; // [rsp+F8h] [rbp-8h] BYREF
  volatile signed __int32 *v87; // [rsp+100h] [rbp+0h]
  _BYTE v88[8]; // [rsp+108h] [rbp+8h] BYREF
  volatile signed __int32 *v89; // [rsp+110h] [rbp+10h]
  _BYTE v90[8]; // [rsp+118h] [rbp+18h] BYREF
  volatile signed __int32 *v91; // [rsp+120h] [rbp+20h]
  _BYTE v92[8]; // [rsp+128h] [rbp+28h] BYREF
  volatile signed __int32 *v93; // [rsp+130h] [rbp+30h]
  _BYTE v94[8]; // [rsp+138h] [rbp+38h] BYREF
  volatile signed __int32 *v95; // [rsp+140h] [rbp+40h]
  _BYTE v96[8]; // [rsp+148h] [rbp+48h] BYREF
  volatile signed __int32 *v97; // [rsp+150h] [rbp+50h]
  __int64 v98; // [rsp+1B8h] [rbp+B8h]

  v68 = -2;
  if ( !*a3 )
  {
    v6 = (volatile signed __int32 *)a3[1];
    if ( !v6 )
      return;
    goto LABEL_80;
  }
  v67 = 0;
  v7 = a3[1];
  if ( v7 )
  {
    _InterlockedIncrement((volatile signed __int32 *)(v7 + 8));
    v7 = a3[1];
  }
  *(_QWORD *)&v67 = *a3;
  *((_QWORD *)&v67 + 1) = v7;
  sub_1441CBA30(a1, a2, &v67);
  *(_QWORD *)(a1 + 8) = a2;
  if ( dword_14E662B58 > *(_DWORD *)(*((_QWORD *)NtCurrentTeb()->ThreadLocalStoragePointer
                                     + (unsigned int)dword_14F3BEE58)
                                   + 420620LL) )
  {
    sub_148860450(&dword_14E662B58);
    if ( dword_14E662B58 == -1 )
    {
      qword_14E662B38 = 0;
      qword_14E662B48 = 0;
      qword_14E662B50 = 7;
      sub_14014C8D0(&qword_14E662B38, &byte_14BAF7F08);
      sub_14885FFE8(sub_149024090);
      sub_1488603F0(&dword_14E662B58);
    }
  }
  v8 = *a3;
  v9 = sub_146E8C7D0(&unk_14928FEA8);
  v10 = sub_146EC8E30(v8, v70, v9);
  v11 = (_QWORD *)(a1 + 136);
  sub_1401E5080(a1 + 136, v10);
  v12 = v71;
  if ( v71 )
  {
    if ( _InterlockedExchangeAdd(v71 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v12)(v12);
      if ( _InterlockedExchangeAdd(v12 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v12 + 8LL))(v12);
    }
  }
  v13 = *v11;
  v14 = sub_146E8C7D0(&unk_1496A6D08);
  v15 = sub_146EC8E30(v13, v74, v14);
  v16 = sub_1404D51A0(v72, v15);
  sub_1401E5080(a1 + 112, v16);
  v17 = v73;
  if ( v73 )
  {
    if ( _InterlockedExchangeAdd(v73 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v17)(v17);
      if ( _InterlockedExchangeAdd(v17 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v17 + 8LL))(v17);
    }
  }
  v18 = v75;
  if ( v75 )
  {
    if ( _InterlockedExchangeAdd(v75 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v18)(v18);
      if ( _InterlockedExchangeAdd(v18 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v18 + 8LL))(v18);
    }
  }
  v19 = *v11;
  v20 = sub_146E8C7D0(&unk_14A232350);
  v21 = sub_146EC8E30(v19, v78, v20);
  v22 = sub_1401E9D00(v76, v21);
  sub_1401E5080(a1 + 2152, v22);
  v23 = v77;
  if ( v77 )
  {
    if ( _InterlockedExchangeAdd(v77 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v23)(v23);
      if ( _InterlockedExchangeAdd(v23 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v23 + 8LL))(v23);
    }
  }
  v24 = v79;
  if ( v79 )
  {
    if ( _InterlockedExchangeAdd(v79 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v24)(v24);
      if ( _InterlockedExchangeAdd(v24 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v24 + 8LL))(v24);
    }
  }
  v25 = *v11;
  v26 = sub_146E8C7D0(&unk_14A232370);
  v27 = sub_146EC8E30(v25, v80, v26);
  sub_1401E5080(a1 + 2168, v27);
  v28 = v81;
  if ( v81 )
  {
    if ( _InterlockedExchangeAdd(v81 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v28)(v28);
      if ( _InterlockedExchangeAdd(v28 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v28 + 8LL))(v28);
    }
  }
  v29 = *(_QWORD *)(a1 + 2152);
  v30 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v29 + 688LL);
  v31 = sub_14723C170(100002264);
  v30(v29, v31);
  v32 = *(_QWORD *)(a1 + 8);
  if ( v32 )
    (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 2152) + 664LL))(*(_QWORD *)(a1 + 2152), v32 + 17696);
  v33 = 0;
  v34 = (_QWORD *)(a1 + 248);
  v98 = a1 + 152;
  do
  {
    v35 = sub_146E8C7D0(&unk_14A2322B8);
    v36 = sub_146E8CF20(v82, v35, (unsigned int)v33);
    v37 = sub_14014F430(v36);
    v38 = -1;
    do
      ++v38;
    while ( *(_WORD *)(v37 + 2 * v38) );
    sub_14014C8D0(&qword_14E662B38, v37);
    sub_146E8C910(v82);
    v39 = *(void (__fastcall **)(__int64, __int64))*(v34 - 12);
    v69 = v83;
    v40 = &qword_14E662B38;
    if ( (unsigned __int64)qword_14E662B50 >= 8 )
      v40 = (__int64 *)qword_14E662B38;
    v41 = sub_146EC8E30(*v11, v83, v40);
    v39(v98, v41);
    (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)*v34 + 16LL))(*v34, 0);
    ++v33;
    v98 += 400;
    v34 += 50;
  }
  while ( v33 < 5 );
  v42 = *v11;
  v43 = sub_146E8C7D0(&unk_14A232398);
  v44 = sub_146EC8E30(v42, v86, v43);
  v45 = (__int64 *)sub_1403ACAE0(v84, v44);
  v46 = *v45;
  v65 = *v45;
  v66 = (volatile signed __int32 *)v45[1];
  *v45 = 0;
  v45[1] = 0;
  v47 = v85;
  if ( v85 )
  {
    if ( _InterlockedExchangeAdd(v85 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v47)(v47);
      if ( _InterlockedExchangeAdd(v47 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v47 + 8LL))(v47);
    }
  }
  v48 = v87;
  if ( v87 )
  {
    if ( _InterlockedExchangeAdd(v87 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v48)(v48);
      if ( _InterlockedExchangeAdd(v48 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v48 + 8LL))(v48);
    }
  }
  if ( v65 )
    (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v46 + 16LL))(v46, 0);
  v49 = *v11;
  v50 = sub_146E8C7D0(&unk_14A2323C0);
  v51 = sub_146EC8E30(v49, v90, v50);
  v52 = (__int64 *)sub_1401E9D00(v88, v51);
  v53 = *v52;
  v63 = *v52;
  v64 = (volatile signed __int32 *)v52[1];
  *v52 = 0;
  v52[1] = 0;
  v54 = v89;
  if ( v89 )
  {
    if ( _InterlockedExchangeAdd(v89 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v54)(v54);
      if ( _InterlockedExchangeAdd(v54 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v54 + 8LL))(v54);
    }
  }
  v55 = v91;
  if ( v91 )
  {
    if ( _InterlockedExchangeAdd(v91 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v55)(v55);
      if ( _InterlockedExchangeAdd(v55 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v55 + 8LL))(v55);
    }
  }
  if ( v63 )
    (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v53 + 16LL))(v53, 0);
  *(_DWORD *)(a1 + 2184) = 0;
  v56 = *v11;
  v57 = sub_146E8C7D0(&unk_14A232320);
  v58 = sub_146EC8E30(v56, v94, v57);
  v59 = sub_140462E10(v92, v58);
  sub_1401E5080(a1 + 2192, v59);
  v60 = v93;
  if ( v93 )
  {
    if ( _InterlockedExchangeAdd(v93 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v60)(v60);
      if ( _InterlockedExchangeAdd(v60 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v60 + 8LL))(v60);
    }
  }
  v61 = v95;
  if ( v95 )
  {
    if ( _InterlockedExchangeAdd(v95 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v61)(v61);
      if ( _InterlockedExchangeAdd(v61 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v61 + 8LL))(v61);
    }
  }
  sub_146F50610(*(_QWORD *)(a1 + 2192), v96, *(unsigned int *)(a1 + 2184));
  v62 = v97;
  if ( v97 )
  {
    if ( _InterlockedExchangeAdd(v97 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v62)(v62);
      if ( _InterlockedExchangeAdd(v62 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v62 + 8LL))(v62);
    }
  }
  if ( v64 )
  {
    if ( _InterlockedExchangeAdd(v64 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v64)(v64);
      if ( _InterlockedExchangeAdd(v64 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v64 + 8LL))(v64);
    }
  }
  if ( v66 )
  {
    if ( _InterlockedExchangeAdd(v66 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v66)(v66);
      if ( _InterlockedExchangeAdd(v66 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v66 + 8LL))(v66);
    }
  }
  v6 = (volatile signed __int32 *)a3[1];
  if ( v6 )
  {
LABEL_80:
    if ( _InterlockedExchangeAdd(v6 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v6)(v6);
      if ( _InterlockedExchangeAdd(v6 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v6 + 8LL))(v6);
    }
  }
}

