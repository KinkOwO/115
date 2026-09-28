// reader_sub_1444E1080

void __fastcall sub_1444E1080(__int64 a1)
{
  __int64 v2; // rcx
  int v3; // eax
  __int64 v4; // rcx
  unsigned int v5; // edi
  unsigned int v6; // esi
  unsigned int v7; // eax
  __int64 v8; // rcx
  __int64 v9; // rcx
  __int64 v10; // rcx
  __int64 v11; // rax
  __int64 v12; // rcx
  __int64 v13; // rax
  __int64 v14; // rdx
  __int64 v15; // rcx
  __int64 v16; // rcx
  __int64 v17; // r8
  __int64 v18; // rax
  __int64 v19; // r12
  __int64 v20; // rcx
  __int64 v21; // rax
  __int64 v22; // rdx
  __int64 v23; // rcx
  __int64 v24; // rcx
  __int64 v25; // rcx
  bool v26; // r13
  int v27; // ebx
  int v28; // esi
  int v29; // r14d
  unsigned int v30; // r15d
  __int64 *v31; // r8
  __int64 v32; // r9
  __int64 v33; // rcx
  __int64 v34; // rdx
  _QWORD *v35; // rdi
  __int64 v36; // rcx
  __int64 v37; // rdx
  __int64 v38; // rcx
  __int64 v39; // rcx
  __int64 i; // rbx
  unsigned __int16 *v41; // rdi
  __int64 *v42; // r15
  __int64 v43; // rcx
  __int64 v44; // rax
  void (__fastcall ***v45)(_QWORD); // rcx
  __int64 v46; // r12
  __int64 v47; // rcx
  __int64 v48; // rcx
  int v49; // r14d
  __int64 v50; // rax
  void (__fastcall ***v51)(_QWORD); // rcx
  __int64 v52; // rbx
  int v53; // esi
  int v54; // eax
  __int64 v55; // rcx
  __int64 v56; // rcx
  __int64 v57; // r14
  void (__fastcall *v58)(__int64, __int64); // rsi
  unsigned int v59; // edi
  __int64 v60; // rax
  __int64 v61; // rax
  __int64 v62; // rax
  int v63; // ebx
  int v64; // edi
  int v65; // esi
  unsigned int v66; // r12d
  __int64 v67; // r14
  __int64 v68; // r9
  __int64 v69; // rcx
  __int64 v70; // rdx
  __int64 v71; // rcx
  _QWORD *v72; // r14
  __int64 v73; // rdx
  __int64 v74; // rax
  __int64 v75; // rcx
  __int64 v76; // rbx
  __int64 v77; // rcx
  __int64 v78; // rbx
  _QWORD *v79; // rdi
  __int64 v80; // rcx
  __int64 v81; // rcx
  __int64 v82; // rcx
  __int64 v83; // rax
  __int64 v84; // rdi
  _QWORD *v85; // rbx
  __int64 v86; // rdi
  __int64 v87; // rcx
  __int64 v88; // rcx
  __int64 v89; // rcx
  void (__fastcall *v90)(__int64); // rbx
  __int64 v91; // rdx
  __int64 v92; // rcx
  __int64 v93; // rax
  void (__fastcall ***v94)(_QWORD); // rcx
  unsigned __int64 v95; // rdx
  __int64 v96; // r8
  __int64 v97; // rax
  char v98; // r9
  _QWORD *v99; // r8
  _QWORD *v100; // rax
  _QWORD *v101; // rbx
  __int64 v102; // rdi
  __int64 v103; // rcx
  __int64 v104; // rdx
  __int64 v105; // rcx
  bool v106; // dl
  _BOOL8 v107; // rcx
  __int64 v108; // rax
  __int64 v109; // rcx
  __int64 v110; // rax
  __int64 v111; // rdx
  __int64 v112; // rcx
  char v113; // [rsp+50h] [rbp-98h]
  int v114; // [rsp+54h] [rbp-94h]
  unsigned __int16 *v115; // [rsp+58h] [rbp-90h]
  _QWORD *v116; // [rsp+60h] [rbp-88h]
  __int64 v117; // [rsp+68h] [rbp-80h]
  _WORD *v118; // [rsp+70h] [rbp-78h]
  _BYTE v119[16]; // [rsp+88h] [rbp-60h] BYREF
  _QWORD v120[2]; // [rsp+98h] [rbp-50h] BYREF

  v2 = (unsigned int)(*(_DWORD *)(a1 + 4400) - 1);
  if ( (_DWORD)v2 )
  {
    if ( (_DWORD)v2 == 1 )
      sub_1444E2AE0(a1);
  }
  else
  {
    sub_1444E3430(a1);
  }
  v3 = *(_DWORD *)(a1 + 4400);
  if ( !v3 )
  {
    v4 = *(_QWORD *)(a1 + 3848);
    if ( v4 && (unsigned __int8)sub_146AEF960(v4) )
    {
      if ( (unsigned __int8)sub_141FB6530(*(_QWORD *)(a1 + 3848)) )
      {
        (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 3848) + 16LL))(*(_QWORD *)(a1 + 3848), 0);
        sub_1444E4540(a1);
      }
    }
    return;
  }
  if ( v3 == 3 )
  {
    if ( (*(_DWORD *)(a1 + 4300) & 2) != 0 )
    {
      v5 = sub_146E9F840(a1 + 4296);
      v6 = sub_140193D40(a1 + 4296);
      if ( (unsigned __int8)sub_146E9FA80(a1 + 4296) )
      {
        v5 = v6;
        sub_146E9FBC0(a1 + 4296);
      }
      v7 = sub_146EA1750(0, 255, v5, v6);
      v8 = *(_QWORD *)(a1 + 3928);
      if ( v8 )
        (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v8 + 376LL))(v8, v7);
    }
    v9 = *(_QWORD *)(a1 + 3976);
    if ( v9 && sub_146ECFD90(v9) )
    {
      v11 = sub_146D74000(v10);
      sub_146D746E0(v11, 1623);
      v13 = sub_146D74000(v12);
      sub_146D75B10(v13, v120, 13);
      sub_146D75AF0(v15, v14);
    }
    v16 = *(_QWORD *)(a1 + 4008);
    if ( v16 && sub_146ECFD90(v16) )
    {
      LOBYTE(v17) = 1;
      sub_1444E4990(a1, 1, v17);
    }
    return;
  }
  v18 = sub_1444D2BB0(v2);
  v19 = sub_1444D2A20(v18);
  v118 = (_WORD *)v19;
  if ( !v19 )
    return;
  v20 = *(_QWORD *)(a1 + 3896);
  if ( v20 && (unsigned __int8)sub_141FB6530(v20) && (unsigned __int8)sub_146AEF960(*(_QWORD *)(a1 + 3896)) )
    (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 3896) + 16LL))(*(_QWORD *)(a1 + 3896), 0);
  v21 = sub_1444D2BB0(v20);
  if ( !(unsigned int)sub_1444D2B20(v21) )
  {
    *(_DWORD *)(a1 + 4400) = 3;
    v23 = *(_QWORD *)(a1 + 3928);
    if ( v23 )
    {
      LOBYTE(v22) = 1;
      (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v23 + 16LL))(v23, v22);
    }
    v24 = *(_QWORD *)(a1 + 3960);
    if ( v24 )
    {
      LOBYTE(v22) = 1;
      (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v24 + 16LL))(v24, v22);
    }
    v25 = *(_QWORD *)(a1 + 3992);
    if ( v25 )
      (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v25 + 16LL))(v25, 0);
    sub_145F588B0(46);
    sub_145F58310(47);
    sub_146E9FBD0(a1 + 4296, 1500, 0);
  }
  v26 = 0;
  v27 = 0;
  v28 = 0;
  v29 = 0;
  v30 = 0;
  v31 = (__int64 *)(v19 + 534);
  v32 = 22;
  do
  {
    v33 = *v31;
    v34 = (unsigned int)*v31 - 3;
    if ( (unsigned int)*v31 == 3 )
    {
      v28 += HIDWORD(v33);
    }
    else
    {
      v34 = (unsigned int)*v31 - 4;
      if ( (unsigned int)*v31 == 4 )
      {
        v27 += HIDWORD(v33);
      }
      else
      {
        v34 = (unsigned int)*v31 - 7;
        if ( (unsigned int)*v31 == 7 )
        {
          v34 = HIDWORD(v33) / 0x3E8;
          v29 += v34;
        }
        else if ( (unsigned int)*v31 == 8 )
        {
          v27 -= HIDWORD(v33);
        }
      }
    }
    v31 += 2;
    --v32;
  }
  while ( v32 );
  v35 = (_QWORD *)(a1 + 3544);
  v36 = *(_QWORD *)(a1 + 3544);
  if ( v36 && *(_QWORD *)(a1 + 3608) && v27 )
  {
    if ( v27 <= 0 )
    {
      sub_146EECBB0(v36, 3);
      sub_146F073E0(*(_QWORD *)(a1 + 3608), 9);
      v37 = abs32(v27);
    }
    else
    {
      sub_146EECBB0(v36, 1);
      sub_146F073E0(*(_QWORD *)(a1 + 3608), 0xFFFFFFFFLL);
      v37 = (unsigned int)v27;
    }
    sub_143FD16D0(*(_QWORD *)(a1 + 3608), v37);
    v30 = 1;
  }
  v38 = *(_QWORD *)(a1 + 16LL * v30 + 3544);
  if ( v38 && *(_QWORD *)(a1 + 16LL * v30 + 3608) && v28 > 0 )
  {
    sub_146EECBB0(v38, 0);
    sub_146F073E0(*(_QWORD *)(a1 + 16LL * v30 + 3608), 0xFFFFFFFFLL);
    sub_143FD16D0(*(_QWORD *)(a1 + 16LL * v30++ + 3608), (unsigned int)v28);
  }
  v39 = *(_QWORD *)(a1 + 16LL * v30 + 3544);
  if ( v39 && *(_QWORD *)(a1 + 16LL * v30 + 3608) && v29 > 0 )
  {
    sub_146EECBB0(v39, 2);
    sub_146F073E0(*(_QWORD *)(a1 + 16LL * v30 + 3608), 0xFFFFFFFFLL);
    sub_143FD16D0(*(_QWORD *)(a1 + 16LL * v30++ + 3608), (unsigned int)v29);
  }
  for ( i = 0; i < 4; ++i )
  {
    if ( *v35 )
    {
      if ( i >= (unsigned __int64)v30 )
        v34 = 0;
      else
        LOBYTE(v34) = 1;
      (*(void (__fastcall **)(_QWORD, __int64))(*(_QWORD *)*v35 + 16LL))(*v35, v34);
    }
    v35 += 2;
  }
  v113 = 0;
  v114 = 0;
  v117 = 0;
  v116 = (_QWORD *)(a1 + 3112);
  v41 = (unsigned __int16 *)(v19 + 894);
  v115 = (unsigned __int16 *)(v19 + 894);
  v42 = (__int64 *)(a1 + 2824);
  do
  {
    v43 = qword_14E664BF8;
    if ( !qword_14E664BF8 )
    {
      v44 = sub_146E8BA20(2792);
      if ( v44 )
        v45 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v44);
      else
        v45 = 0;
      qword_14E664BF8 = (__int64)v45;
      (**v45)(v45);
      v43 = qword_14E664BF8;
    }
    v46 = sub_1444D2CC0(v43, *((unsigned int *)v41 - 1), v31);
    if ( v46 )
    {
      v47 = *(v42 - 26);
      if ( v47 && (unsigned __int8)sub_141FB6530(v47) )
      {
        v48 = *(v42 - 6);
        if ( v48 && (unsigned __int8)sub_141FB6530(v48) && (unsigned __int8)sub_146AEF960(*(v42 - 6)) )
          (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)*(v42 - 6) + 16LL))(*(v42 - 6), 0);
        if ( *v42 && (unsigned __int8)sub_141FB6530(*v42) )
        {
          if ( (unsigned int)sub_140335730(*v42) == 2 && (unsigned __int8)sub_146AEF960(*v42) )
          {
            sub_146AF0950(*v42, 0);
            (*(void (__fastcall **)(__int64))(*(_QWORD *)*v42 + 432LL))(*v42);
          }
          else if ( (unsigned int)sub_140335730(*v42) == 1 && (unsigned __int8)sub_146AEF960(*v42) )
          {
            (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)*(v42 - 26) + 16LL))(*(v42 - 26), 0);
            v113 = 1;
            v49 = qword_14E6343D0;
            if ( !qword_14E6343D0 )
            {
              v50 = sub_146E8BA20(72);
              v120[0] = v50;
              if ( v50 )
                v51 = (void (__fastcall ***)(_QWORD))sub_146E93360(v50);
              else
                v51 = 0;
              qword_14E6343D0 = (__int64)v51;
              (**v51)(v51);
              v49 = qword_14E6343D0;
            }
            v52 = sub_146E8C7D0(&unk_14A315E60);
            v53 = sub_146E8C7D0(&unk_14A315EC0);
            v54 = sub_146E8C7D0(&unk_14A315F10);
            sub_146E938E0(v49, 0, v54, v53, 1856, (__int64)&qword_14E664C18, v52);
            v41 = v115;
          }
        }
        v55 = v42[6];
        if ( v55 && (unsigned __int8)sub_141FB6530(v55) && (unsigned __int8)sub_146AEF960(v42[6]) )
          (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v42[6] + 16LL))(v42[6], 0);
        v56 = v42[12];
        if ( v56 )
          (*(void (__fastcall **)(__int64, _QWORD, _QWORD))(*(_QWORD *)v56 + 672LL))(
            v56,
            *v41,
            *(unsigned __int16 *)(v46 + 44));
        v57 = v42[18];
        if ( v57 )
        {
          v58 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v57 + 688LL);
          v59 = *v41;
          v60 = sub_14723C170(27075);
          v61 = sub_146E8CF20(v119, v60, v59);
          v62 = sub_14014F430(v61);
          v58(v57, v62);
          sub_146E8C910(v119);
        }
      }
      v63 = 0;
      v64 = 0;
      v65 = 0;
      v66 = 0;
      v67 = 0;
      v31 = (__int64 *)(v115 + 1);
      v68 = 22;
      do
      {
        v69 = *v31;
        v70 = (unsigned int)*v31 - 3;
        if ( (unsigned int)*v31 == 3 )
        {
          v64 += HIDWORD(v69);
        }
        else
        {
          v70 = (unsigned int)*v31 - 4;
          if ( (unsigned int)*v31 == 4 )
          {
            v63 += HIDWORD(v69);
          }
          else
          {
            v70 = (unsigned int)*v31 - 7;
            if ( (unsigned int)*v31 == 7 )
            {
              v70 = HIDWORD(v69) / 0x3E8;
              v65 += v70;
            }
            else if ( (unsigned int)*v31 == 8 )
            {
              v63 -= HIDWORD(v69);
            }
          }
        }
        v31 += 2;
        --v68;
      }
      while ( v68 );
      v71 = *v116;
      if ( *v116 && v116[24] && v63 )
      {
        if ( v63 <= 0 )
        {
          sub_146EECBB0(v71, 3);
          v72 = v116;
          sub_146F073E0(v116[24], 9);
          v73 = abs32(v63);
        }
        else
        {
          sub_146EECBB0(v71, 1);
          v72 = v116;
          sub_146F073E0(v116[24], 0xFFFFFFFFLL);
          v73 = (unsigned int)v63;
        }
        sub_143FD16D0(v72[24], v73);
        v66 = 1;
        v67 = 1;
      }
      v74 = v117;
      v75 = *(_QWORD *)(a1 + 16 * (v117 + v67) + 3112);
      if ( v75 && *(_QWORD *)(a1 + 16 * (v117 + v67) + 3304) && v64 > 0 )
      {
        sub_146EECBB0(v75, 0);
        sub_146F073E0(*(_QWORD *)(a1 + 16 * (v117 + v67) + 3304), 0xFFFFFFFFLL);
        sub_143FD16D0(*(_QWORD *)(a1 + 16 * (v117 + v67) + 3304), (unsigned int)v64);
        ++v66;
        ++v67;
        v74 = v117;
      }
      v76 = 2 * (v74 + v67);
      v77 = *(_QWORD *)(a1 + 16 * (v74 + v67) + 3112);
      if ( v77 && *(_QWORD *)(a1 + 16 * (v74 + v67) + 3304) && v65 > 0 )
      {
        sub_146EECBB0(v77, 2);
        sub_146F073E0(*(_QWORD *)(a1 + 8 * v76 + 3304), 0xFFFFFFFFLL);
        sub_143FD16D0(*(_QWORD *)(a1 + 8 * v76 + 3304), (unsigned int)v65);
        ++v66;
      }
      v78 = 0;
      v79 = v116;
      do
      {
        if ( *v79 )
        {
          if ( v78 >= (unsigned __int64)v66 )
            v70 = 0;
          else
            LOBYTE(v70) = 1;
          (*(void (__fastcall **)(_QWORD, __int64))(*(_QWORD *)*v79 + 16LL))(*v79, v70);
        }
        ++v78;
        v79 += 2;
      }
      while ( v78 < 4 );
      v80 = v42[84];
      if ( v80 && (unsigned __int8)sub_146AEF960(v80) && (unsigned __int8)sub_141FB6530(v42[84]) )
        (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v42[84] + 16LL))(v42[84], 0);
      v41 = v115;
    }
    ++v114;
    v41 += 179;
    v115 = v41;
    v117 += 4;
    v116 += 8;
    v42 += 2;
  }
  while ( v114 < 3 );
  v81 = *(_QWORD *)(a1 + 3832);
  if ( v81 && (unsigned __int8)sub_146AEF960(v81) && (unsigned __int8)sub_141FB6530(*(_QWORD *)(a1 + 3832)) )
    (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 3832) + 16LL))(*(_QWORD *)(a1 + 3832), 0);
  v82 = *(_QWORD *)(a1 + 3912);
  if ( v82 && (unsigned __int8)sub_146AEF960(v82) && (unsigned __int8)sub_141FB6530(*(_QWORD *)(a1 + 3912)) )
    (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 3912) + 16LL))(*(_QWORD *)(a1 + 3912), 0);
  v83 = *(int *)(a1 + 4284);
  v84 = *(_QWORD *)(a1 + 3816);
  if ( (_DWORD)v83 != -1 )
  {
    if ( v84 )
    {
      v89 = *(_QWORD *)(a1 + 16 * v83 + 3688);
      if ( v89 )
      {
        v90 = *(void (__fastcall **)(__int64))(*(_QWORD *)v84 + 112LL);
        sub_142757420(v89);
        sub_146ECA0C0(*(_QWORD *)(a1 + 16LL * *(int *)(a1 + 4284) + 3688));
        v90(v84);
        LOBYTE(v91) = 1;
        (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 3816) + 16LL))(*(_QWORD *)(a1 + 3816), v91);
      }
    }
    v92 = qword_14E664BF8;
    if ( !qword_14E664BF8 )
    {
      v93 = sub_146E8BA20(2792);
      v120[0] = v93;
      if ( v93 )
        v94 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v93);
      else
        v94 = 0;
      qword_14E664BF8 = (__int64)v94;
      (**v94)(v94);
      v92 = qword_14E664BF8;
    }
    v95 = *(int *)(a1 + 4284);
    v96 = *(_QWORD *)(a1 + 4352);
    if ( (*(_QWORD *)(a1 + 4360) - v96) >> 2 <= v95 )
      sub_1401790B0(v92, v95);
    v97 = sub_1444D2A50(v92, *(unsigned int *)(v96 + 4 * v95));
    if ( !v97 )
      goto LABEL_190;
    v98 = 0;
    v99 = *(_QWORD **)(v97 + 48);
    v100 = (_QWORD *)*v99;
    if ( (_QWORD *)*v99 == v99 )
      goto LABEL_190;
    do
    {
      if ( *((_DWORD *)v100 + 4) == 1 || *((_DWORD *)v100 + 4) == 8 || *((_DWORD *)v100 + 4) == 10 )
        v98 = 1;
      v100 = (_QWORD *)*v100;
    }
    while ( v100 != v99 );
    if ( !v98 )
      goto LABEL_190;
    v101 = (_QWORD *)(a1 + 2664);
    v102 = 3;
    while ( 1 )
    {
      v103 = *(v101 - 6);
      if ( v103 && (unsigned __int8)sub_141FB6530(v103) && (unsigned __int8)sub_146ED0010(*(v101 - 6)) )
      {
        if ( *v101 )
          sub_146EECBB0(*v101, 1);
        v105 = v101[6];
        if ( !v105 )
          goto LABEL_189;
        LOBYTE(v104) = 1;
      }
      else
      {
        if ( *v101 )
          sub_146EECBB0(*v101, 0);
        v105 = v101[6];
        if ( !v105 )
          goto LABEL_189;
        v104 = 0;
      }
      (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v105 + 16LL))(v105, v104);
LABEL_189:
      v101 += 2;
      if ( !--v102 )
        goto LABEL_190;
    }
  }
  if ( v84 )
    (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)v84 + 16LL))(*(_QWORD *)(a1 + 3816), 0);
  v85 = (_QWORD *)(a1 + 2712);
  v86 = 3;
  do
  {
    v87 = *(v85 - 6);
    if ( v87 )
      (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v87 + 16LL))(v87, 0);
    if ( *v85 )
      (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)*v85 + 16LL))(*v85, 0);
    v85 += 2;
    --v86;
  }
  while ( v86 );
  v88 = *(_QWORD *)(a1 + 3944);
  if ( v88 )
    (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v88 + 16LL))(v88, 0);
  sub_1466963D0(qword_14E683C78, 0);
LABEL_190:
  if ( v113 )
  {
    v106 = 0;
    v107 = v118[447] == 0;
    if ( !v118[626] )
      v106 = v118[447] == 0;
    if ( !v118[805] )
      v26 = v106;
    if ( v26 )
    {
      *(_DWORD *)(a1 + 4400) = 3;
      v108 = sub_146D74000(v107);
      sub_146D746E0(v108, 1618);
      v110 = sub_146D74000(v109);
      sub_146D75B10(v110, v120, 13);
      sub_146D75AF0(v112, v111);
    }
  }
}

