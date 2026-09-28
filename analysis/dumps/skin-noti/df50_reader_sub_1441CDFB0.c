// reader_sub_1441CDFB0

__int64 __fastcall sub_1441CDFB0(__int64 a1, __int64 a2, __int64 *a3)
{
  __int64 result; // rax
  volatile signed __int32 *v6; // rdi
  __int64 v7; // rbx
  __int64 v8; // rax
  __int64 v9; // rax
  volatile signed __int32 *v10; // rdi
  __int64 v11; // rdi
  __int64 v12; // rax
  __int64 v13; // rax
  __int64 v14; // rax
  volatile signed __int32 *v15; // rdi
  volatile signed __int32 *v16; // rdi
  __int64 v17; // rdi
  __int64 v18; // rax
  __int64 v19; // rax
  __int64 v20; // rax
  volatile signed __int32 *v21; // rdi
  volatile signed __int32 *v22; // rdi
  __int64 v23; // rdi
  __int64 v24; // rax
  __int64 v25; // rax
  volatile signed __int32 *v26; // rdi
  __int64 v27; // rsi
  void (__fastcall *v28)(__int64, __int64); // rdi
  __int64 v29; // rax
  __int64 v30; // rdx
  __int64 v31; // rdi
  __int64 v32; // rax
  __int64 v33; // rax
  __int64 v34; // rax
  _QWORD *v35; // r12
  volatile signed __int32 *v36; // rdi
  volatile signed __int32 *v37; // rdi
  __int64 v38; // rcx
  unsigned int v39; // esi
  __int64 v40; // rax
  __int64 v41; // rax
  __int64 v42; // rax
  __int64 v43; // r8
  __int64 *v44; // r8
  volatile signed __int32 *v45; // rdi
  volatile signed __int32 *v46; // rdi
  int v47; // esi
  _QWORD *v48; // r14
  __int64 v49; // r15
  __int64 v50; // rax
  __int64 v51; // rax
  __int64 v52; // rax
  __int64 v53; // r8
  void (__fastcall *v54)(__int64, __int64); // rdi
  __int64 *v55; // r8
  __int64 v56; // rax
  __int64 v57; // rdx
  int v58; // r14d
  _QWORD *v59; // rsi
  _QWORD *v60; // rax
  __int64 v61; // rcx
  __int64 v62; // rdx
  volatile signed __int32 *v63; // rdi
  volatile signed __int32 *v64; // rdi
  __int64 v65; // rdi
  __int64 v66; // rax
  __int64 v67; // rax
  __int64 v68; // rax
  __int64 v69; // rax
  __int64 v70; // rax
  __int128 v71; // kr00_16
  volatile signed __int32 *v72; // rdi
  volatile signed __int32 *v73; // rdi
  volatile signed __int32 *v74; // rdi
  __int64 v75; // rdi
  __int64 v76; // rax
  __int64 v77; // rax
  __int64 v78; // rax
  __int64 v79; // rax
  __int64 v80; // rax
  __int128 v81; // kr10_16
  volatile signed __int32 *v82; // rdi
  volatile signed __int32 *v83; // rdi
  volatile signed __int32 *v84; // rdi
  __int64 v85; // rcx
  __int64 v86; // rdx
  volatile signed __int32 *v87; // rdi
  __int64 v88; // rcx
  volatile signed __int32 *v89; // rdi
  __int128 v90; // [rsp+28h] [rbp-E0h] BYREF
  __int128 v91; // [rsp+38h] [rbp-D0h]
  __int128 v92; // [rsp+48h] [rbp-C0h]
  __int128 v93; // [rsp+58h] [rbp-B0h]
  __int128 v94; // [rsp+68h] [rbp-A0h] BYREF
  __int128 v95; // [rsp+78h] [rbp-90h] BYREF
  __int64 v96; // [rsp+90h] [rbp-78h] BYREF
  volatile signed __int32 *v97; // [rsp+98h] [rbp-70h]
  __int64 v98; // [rsp+A0h] [rbp-68h]
  _BYTE v99[8]; // [rsp+A8h] [rbp-60h] BYREF
  volatile signed __int32 *v100; // [rsp+B0h] [rbp-58h]
  _BYTE v101[8]; // [rsp+B8h] [rbp-50h] BYREF
  volatile signed __int32 *v102; // [rsp+C0h] [rbp-48h]
  _BYTE v103[8]; // [rsp+C8h] [rbp-40h] BYREF
  volatile signed __int32 *v104; // [rsp+D0h] [rbp-38h]
  _BYTE v105[8]; // [rsp+D8h] [rbp-30h] BYREF
  volatile signed __int32 *v106; // [rsp+E0h] [rbp-28h]
  _BYTE v107[8]; // [rsp+E8h] [rbp-20h] BYREF
  volatile signed __int32 *v108; // [rsp+F0h] [rbp-18h]
  _BYTE v109[8]; // [rsp+F8h] [rbp-10h] BYREF
  volatile signed __int32 *v110; // [rsp+100h] [rbp-8h]
  _BYTE v111[8]; // [rsp+108h] [rbp+0h] BYREF
  volatile signed __int32 *v112; // [rsp+110h] [rbp+8h]
  _BYTE v113[8]; // [rsp+118h] [rbp+10h] BYREF
  volatile signed __int32 *v114; // [rsp+120h] [rbp+18h]
  _BYTE v115[16]; // [rsp+128h] [rbp+20h] BYREF
  _BYTE v116[16]; // [rsp+138h] [rbp+30h] BYREF
  _BYTE v117[16]; // [rsp+148h] [rbp+40h] BYREF
  _BYTE v118[8]; // [rsp+158h] [rbp+50h] BYREF
  volatile signed __int32 *v119; // [rsp+160h] [rbp+58h]
  _BYTE v120[8]; // [rsp+168h] [rbp+60h] BYREF
  volatile signed __int32 *v121; // [rsp+170h] [rbp+68h]
  _BYTE v122[8]; // [rsp+178h] [rbp+70h] BYREF
  volatile signed __int32 *v123; // [rsp+180h] [rbp+78h]
  _BYTE v124[16]; // [rsp+188h] [rbp+80h] BYREF
  _BYTE v125[8]; // [rsp+198h] [rbp+90h] BYREF
  volatile signed __int32 *v126; // [rsp+1A0h] [rbp+98h]
  _BYTE v127[8]; // [rsp+1A8h] [rbp+A0h] BYREF
  volatile signed __int32 *v128; // [rsp+1B0h] [rbp+A8h]
  _BYTE v129[64]; // [rsp+1B8h] [rbp+B0h] BYREF
  _UNKNOWN *retaddr; // [rsp+200h] [rbp+F8h] BYREF

  result = (__int64)&retaddr;
  v98 = -2;
  if ( !*a3 )
  {
    v6 = (volatile signed __int32 *)a3[1];
    if ( !v6 )
      return result;
    goto LABEL_117;
  }
  *(_QWORD *)(a1 + 8) = a2;
  if ( dword_14E662B08 > *(_DWORD *)(*((_QWORD *)NtCurrentTeb()->ThreadLocalStoragePointer
                                     + (unsigned int)dword_14F3BEE58)
                                   + 420620LL) )
  {
    sub_148860450(&dword_14E662B08);
    if ( dword_14E662B08 == -1 )
    {
      qword_14E662AE8 = 0;
      qword_14E662AF8 = 0;
      qword_14E662B00 = 7;
      sub_14885FFE8(sub_149024250);
      sub_1488603F0(&dword_14E662B08);
    }
  }
  v7 = *a3;
  v8 = sub_146E8C7D0(&unk_14928FEA8);
  v9 = sub_146EC8E30(v7, v99, v8);
  sub_1401E5080(a1 + 136, v9);
  v10 = v100;
  if ( v100 )
  {
    if ( _InterlockedExchangeAdd(v100 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v10)(v10);
      if ( _InterlockedExchangeAdd(v10 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v10 + 8LL))(v10);
    }
  }
  v11 = *(_QWORD *)(a1 + 136);
  v12 = sub_146E8C7D0(&unk_1496A6D08);
  v13 = sub_146EC8E30(v11, v103, v12);
  v14 = sub_1404D51A0(v101, v13);
  sub_1401E5080(a1 + 112, v14);
  v15 = v102;
  if ( v102 )
  {
    if ( _InterlockedExchangeAdd(v102 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v15)(v15);
      if ( _InterlockedExchangeAdd(v15 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v15 + 8LL))(v15);
    }
  }
  v16 = v104;
  if ( v104 )
  {
    if ( _InterlockedExchangeAdd(v104 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v16)(v16);
      if ( _InterlockedExchangeAdd(v16 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v16 + 8LL))(v16);
    }
  }
  v17 = *(_QWORD *)(a1 + 136);
  v18 = sub_146E8C7D0(&unk_14A2322D8);
  v19 = sub_146EC8E30(v17, v107, v18);
  v20 = sub_1401E9D00(v105, v19);
  sub_1401E5080(a1 + 2312, v20);
  v21 = v106;
  if ( v106 )
  {
    if ( _InterlockedExchangeAdd(v106 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v21)(v21);
      if ( _InterlockedExchangeAdd(v21 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v21 + 8LL))(v21);
    }
  }
  v22 = v108;
  if ( v108 )
  {
    if ( _InterlockedExchangeAdd(v108 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v22)(v22);
      if ( _InterlockedExchangeAdd(v22 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v22 + 8LL))(v22);
    }
  }
  v23 = *(_QWORD *)(a1 + 136);
  v24 = sub_146E8C7D0(&unk_14A2322F8);
  v25 = sub_146EC8E30(v23, v109, v24);
  sub_1401E5080(a1 + 2328, v25);
  v26 = v110;
  if ( v110 )
  {
    if ( _InterlockedExchangeAdd(v110 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v26)(v26);
      if ( _InterlockedExchangeAdd(v26 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v26 + 8LL))(v26);
    }
  }
  v27 = *(_QWORD *)(a1 + 2312);
  v28 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v27 + 688LL);
  v29 = sub_14723C170(100020042);
  v28(v27, v29);
  v30 = *(_QWORD *)(a1 + 8);
  if ( v30 )
    (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 2312) + 664LL))(*(_QWORD *)(a1 + 2312), v30 + 17696);
  *(_DWORD *)(a1 + 2344) = 4;
  v31 = *(_QWORD *)(a1 + 136);
  v32 = sub_146E8C7D0(&unk_14A232320);
  v33 = sub_146EC8E30(v31, v113, v32);
  v34 = sub_140462E10(v111, v33);
  v35 = (_QWORD *)(a1 + 2352);
  sub_1401E5080(a1 + 2352, v34);
  v36 = v112;
  if ( v112 )
  {
    if ( _InterlockedExchangeAdd(v112 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v36)(v36);
      if ( _InterlockedExchangeAdd(v36 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v36 + 8LL))(v36);
    }
  }
  v37 = v114;
  if ( v114 )
  {
    if ( _InterlockedExchangeAdd(v114 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v37)(v37);
      if ( _InterlockedExchangeAdd(v37 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v37 + 8LL))(v37);
    }
  }
  v38 = *v35;
  if ( *v35 )
  {
    v39 = 0;
    while ( 1 )
    {
      v40 = sub_146E8C7D0(&unk_14A232338);
      v41 = sub_146E8CF20(v115, v40, v39);
      v42 = sub_14014F430(v41);
      v43 = -1;
      do
        ++v43;
      while ( *(_WORD *)(v42 + 2 * v43) );
      sub_14014C8D0(&qword_14E662AE8, v42);
      sub_146E8C910(v115);
      v44 = &qword_14E662AE8;
      if ( (unsigned __int64)qword_14E662B00 >= 8 )
        v44 = (__int64 *)qword_14E662AE8;
      sub_146EC8E30(*v35, &v96, v44);
      if ( v96 )
      {
        if ( (unsigned __int8)sub_141FB6530(v96) == 1 )
          break;
      }
      v45 = v97;
      if ( v97 )
      {
        if ( _InterlockedExchangeAdd(v97 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v45)(v45);
          if ( _InterlockedExchangeAdd(v45 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v45 + 8LL))(v45);
        }
      }
      if ( (int)++v39 >= 4 )
        goto LABEL_57;
    }
    *(_DWORD *)(a1 + 2344) = v39;
    v46 = v97;
    if ( v97 )
    {
      if ( _InterlockedExchangeAdd(v97 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v46)(v46);
        if ( _InterlockedExchangeAdd(v46 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v46 + 8LL))(v46);
      }
    }
LABEL_57:
    sub_146F53250(*v35, *(unsigned int *)(a1 + 2344), 0);
    v38 = *v35;
  }
  sub_146F50610(v38, &v90, *(unsigned int *)(a1 + 2344));
  v47 = 0;
  v48 = (_QWORD *)(a1 + 528);
  v49 = a1 + 152;
  do
  {
    v50 = sub_146E8C7D0(&unk_14A2322B8);
    v51 = sub_146E8CF20(v116, v50, (unsigned int)v47);
    v52 = sub_14014F430(v51);
    v53 = -1;
    do
      ++v53;
    while ( *(_WORD *)(v52 + 2 * v53) );
    sub_14014C8D0(&qword_14E662AE8, v52);
    sub_146E8C910(v116);
    v54 = *(void (__fastcall **)(__int64, __int64))*(v48 - 47);
    v55 = &qword_14E662AE8;
    if ( (unsigned __int64)qword_14E662B00 >= 8 )
      v55 = (__int64 *)qword_14E662AE8;
    v56 = sub_146EC8E30(v90, v117, v55);
    v54(v49, v56);
    (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)*(v48 - 35) + 16LL))(*(v48 - 35), 0);
    if ( *(_DWORD *)(a1 + 2344) == 1 )
      LOBYTE(v57) = 1;
    else
      v57 = 0;
    (*(void (__fastcall **)(_QWORD, __int64))(*(_QWORD *)*v48 + 24LL))(*v48, v57);
    ++v47;
    v49 += 400;
    v48 += 50;
  }
  while ( v47 < 5 );
  v58 = 0;
  v59 = (_QWORD *)(a1 + 2160);
  do
  {
    v60 = (_QWORD *)sub_146F50610(*v35, v118, 3);
    v91 = 0;
    *(_QWORD *)&v91 = *v60;
    v61 = v91;
    v62 = v60[1];
    *v60 = 0;
    v60[1] = 0;
    v91 = v90;
    *(_QWORD *)&v90 = v61;
    v63 = (volatile signed __int32 *)*((_QWORD *)&v90 + 1);
    *((_QWORD *)&v90 + 1) = v62;
    if ( *((_QWORD *)&v91 + 1) )
    {
      if ( _InterlockedExchangeAdd(v63 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v63)(v63);
        if ( _InterlockedExchangeAdd(v63 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v63 + 8LL))(v63);
      }
    }
    v64 = v119;
    if ( v119 )
    {
      if ( _InterlockedExchangeAdd(v119 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v64)(v64);
        if ( _InterlockedExchangeAdd(v64 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v64 + 8LL))(v64);
      }
    }
    v65 = v90;
    if ( !(_QWORD)v90 )
      break;
    v66 = sub_146E8C7D0(&unk_149D17328);
    v67 = sub_146E8CF20(v124, v66, (unsigned int)v58);
    v68 = sub_14014F430(v67);
    v69 = sub_146EC8E30(v65, v122, v68);
    v70 = sub_1401E9A20(v120, v69);
    v92 = 0;
    v92 = *(_OWORD *)v70;
    v71 = v92;
    *(_QWORD *)v70 = 0;
    *(_QWORD *)(v70 + 8) = 0;
    *(_QWORD *)&v92 = *(v59 - 1);
    *(v59 - 1) = v71;
    *((_QWORD *)&v92 + 1) = *v59;
    v72 = (volatile signed __int32 *)*((_QWORD *)&v92 + 1);
    *v59 = *((_QWORD *)&v71 + 1);
    if ( v72 )
    {
      if ( _InterlockedExchangeAdd(v72 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v72)(v72);
        if ( _InterlockedExchangeAdd(v72 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v72 + 8LL))(v72);
      }
    }
    v73 = v121;
    if ( v121 )
    {
      if ( _InterlockedExchangeAdd(v121 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v73)(v73);
        if ( _InterlockedExchangeAdd(v73 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v73 + 8LL))(v73);
      }
    }
    v74 = v123;
    if ( v123 )
    {
      if ( _InterlockedExchangeAdd(v123 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v74)(v74);
        if ( _InterlockedExchangeAdd(v74 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v74 + 8LL))(v74);
      }
    }
    sub_146E8C910(v124);
    v75 = *(v59 - 1);
    if ( v75 )
    {
      v76 = sub_146E8C7D0(&unk_1494E6790);
      v77 = sub_146E8CF20(v129, v76, (unsigned int)v58);
      v78 = sub_14014F430(v77);
      v79 = sub_146EC8E30(v75, v127, v78);
      v80 = sub_140498000(v125, v79);
      v93 = 0;
      v93 = *(_OWORD *)v80;
      v81 = v93;
      *(_QWORD *)v80 = 0;
      *(_QWORD *)(v80 + 8) = 0;
      *(_QWORD *)&v93 = v59[9];
      v59[9] = v81;
      *((_QWORD *)&v93 + 1) = v59[10];
      v82 = (volatile signed __int32 *)*((_QWORD *)&v93 + 1);
      v59[10] = *((_QWORD *)&v81 + 1);
      if ( v82 )
      {
        if ( _InterlockedExchangeAdd(v82 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v82)(v82);
          if ( _InterlockedExchangeAdd(v82 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v82 + 8LL))(v82);
        }
      }
      v83 = v126;
      if ( v126 )
      {
        if ( _InterlockedExchangeAdd(v126 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v83)(v83);
          if ( _InterlockedExchangeAdd(v83 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v83 + 8LL))(v83);
        }
      }
      v84 = v128;
      if ( v128 )
      {
        if ( _InterlockedExchangeAdd(v128 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v84)(v84);
          if ( _InterlockedExchangeAdd(v84 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v84 + 8LL))(v84);
        }
      }
      sub_146E8C910(v129);
      v85 = v59[9];
      if ( v85 )
      {
        v94 = 0;
        v86 = *v59;
        if ( *v59 )
        {
          _InterlockedIncrement((volatile signed __int32 *)(v86 + 8));
          v86 = *v59;
        }
        *(_QWORD *)&v94 = *(v59 - 1);
        *((_QWORD *)&v94 + 1) = v86;
        sub_146F557B0(v85, &v94);
        v87 = (volatile signed __int32 *)*((_QWORD *)&v94 + 1);
        if ( *((_QWORD *)&v94 + 1) )
        {
          if ( _InterlockedExchangeAdd((volatile signed __int32 *)(*((_QWORD *)&v94 + 1) + 8LL), 0xFFFFFFFF) == 1 )
          {
            (**(void (__fastcall ***)(volatile signed __int32 *))v87)(v87);
            if ( _InterlockedExchangeAdd(v87 + 3, 0xFFFFFFFF) == 1 )
              (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v87 + 8LL))(v87);
          }
        }
      }
    }
    ++v58;
    v59 += 2;
  }
  while ( v58 < 5 );
  v95 = 0;
  v88 = a3[1];
  if ( v88 )
  {
    _InterlockedIncrement((volatile signed __int32 *)(v88 + 8));
    v88 = a3[1];
  }
  *(_QWORD *)&v95 = *a3;
  *((_QWORD *)&v95 + 1) = v88;
  result = sub_1441CBA30((_QWORD *)a1, a2, (__int64 *)&v95);
  v89 = (volatile signed __int32 *)*((_QWORD *)&v90 + 1);
  if ( *((_QWORD *)&v90 + 1) )
  {
    result = (unsigned int)_InterlockedExchangeAdd((volatile signed __int32 *)(*((_QWORD *)&v90 + 1) + 8LL), 0xFFFFFFFF);
    if ( (_DWORD)result == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v89)(v89);
      result = (unsigned int)_InterlockedExchangeAdd(v89 + 3, 0xFFFFFFFF);
      if ( (_DWORD)result == 1 )
        result = (*(__int64 (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v89 + 8LL))(v89);
    }
  }
  v6 = (volatile signed __int32 *)a3[1];
  if ( v6 )
  {
LABEL_117:
    result = (unsigned int)_InterlockedExchangeAdd(v6 + 2, 0xFFFFFFFF);
    if ( (_DWORD)result == 1 )
    {
      result = (**(__int64 (__fastcall ***)(volatile signed __int32 *))v6)(v6);
      if ( _InterlockedExchangeAdd(v6 + 3, 0xFFFFFFFF) == 1 )
        return (*(__int64 (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v6 + 8LL))(v6);
    }
  }
  return result;
}

