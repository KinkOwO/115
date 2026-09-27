// dfvtable_4_1441E5870

__int64 __fastcall sub_1441E5870(__int64 a1, char a2)
{
  __int64 v3; // r12
  __int64 v4; // r8
  __int64 v5; // rsi
  int v6; // edi
  _QWORD *v7; // rsi
  __int64 v8; // r14
  __int64 v9; // rax
  __int64 v10; // rax
  __int64 v11; // rax
  __int64 v12; // r8
  void (__fastcall *v13)(__int64, __int64); // rbx
  __int64 *v14; // r8
  __int64 v15; // rax
  volatile signed __int32 *v16; // rbx
  int *v17; // r13
  int *v18; // rbx
  int *v19; // r15
  __int64 v20; // rcx
  __int64 v21; // rdi
  __int64 v22; // rax
  void (__fastcall ***v23)(_QWORD); // rcx
  __int64 *v24; // rax
  __int64 v25; // r8
  _QWORD *v26; // rax
  _WORD *v27; // r14
  unsigned __int16 *v28; // r11
  unsigned __int64 v29; // rcx
  __int64 v30; // rdi
  _WORD *v31; // r12
  _WORD *v32; // rdi
  unsigned __int16 v33; // r10
  __int64 v34; // rax
  _WORD *v35; // rcx
  _WORD *v36; // rdi
  unsigned __int64 v37; // r8
  unsigned __int16 *v38; // rax
  unsigned __int16 v39; // cx
  bool v40; // cc
  unsigned __int16 v41; // cx
  unsigned __int64 v42; // rdx
  __int64 v43; // rcx
  unsigned __int64 v44; // rdx
  __int64 v45; // rcx
  int v46; // eax
  __int64 v47; // rdi
  int *v48; // rbx
  __int64 v49; // rcx
  __int64 v50; // rax
  void (__fastcall ***v51)(_QWORD); // rcx
  __int64 *v52; // rax
  __int64 v53; // rcx
  __int64 v54; // rax
  void (__fastcall ***v55)(_QWORD); // rcx
  int *v56; // rcx
  int v57; // eax
  unsigned __int64 v58; // r15
  int v59; // r14d
  unsigned int *v60; // rbx
  __int64 *v61; // rdi
  __int64 v62; // rdx
  __int64 v63; // r8
  __int64 result; // rax
  __int64 v65; // r15
  __int64 v66; // rcx
  void (__fastcall ***v67)(_QWORD); // rax
  __int64 v68; // r13
  __int64 *v69; // rcx
  __int64 v70; // rdx
  int v71; // r8d
  __int64 v72; // r12
  __int64 v73; // rcx
  void (__fastcall ***v74)(_QWORD); // rax
  __int64 v75; // rdx
  _QWORD *v76; // rsi
  unsigned __int64 v77; // r14
  __int64 v78; // rbx
  __int64 v79; // rcx
  __int64 v80; // rcx
  __int64 v81; // rbx
  void (__fastcall *v82)(__int64, _QWORD *); // rsi
  __int64 v83; // rdx
  _QWORD *v84; // rax
  __int64 v85; // rdx
  __int64 v86; // rdx
  __int64 v87; // rcx
  __int64 v88; // rax
  _DWORD *v89; // rcx
  _DWORD *v90; // rdx
  _QWORD *v91; // rdx
  __int64 v92; // rsi
  void (__fastcall *v93)(__int64, _QWORD); // r14
  __int64 v94; // rbx
  void (__fastcall ***v95)(_QWORD); // rax
  __int64 *v96; // rax
  unsigned __int8 v97; // al
  bool v98; // al
  __int64 v99; // rcx
  void (__fastcall *v100)(__int64, _QWORD); // r8
  __int64 v101; // rdx
  __int64 v102; // rdx
  __int64 v103; // rdx
  __int64 v104; // rcx
  volatile signed __int32 *v105; // rbx
  __int64 v106; // rdx
  __int64 v107; // rcx
  volatile signed __int32 *v108; // rbx
  __int64 v109; // rcx
  __int64 v110; // rdx
  volatile signed __int32 *v111; // rbx
  volatile signed __int32 *v112; // rbx
  unsigned __int64 v113; // rdx
  __int64 v114; // rcx
  __int64 v115; // rcx
  unsigned __int64 v116; // rdx
  unsigned __int8 v117; // [rsp+28h] [rbp-E0h]
  int v119; // [rsp+2Ch] [rbp-DCh]
  unsigned int *v121; // [rsp+38h] [rbp-D0h]
  unsigned __int64 v122; // [rsp+40h] [rbp-C8h]
  __int128 v123; // [rsp+48h] [rbp-C0h] BYREF
  int *v124; // [rsp+58h] [rbp-B0h]
  __int128 v125; // [rsp+60h] [rbp-A8h] BYREF
  __int128 v126; // [rsp+70h] [rbp-98h]
  int v127; // [rsp+80h] [rbp-88h]
  int v128; // [rsp+84h] [rbp-84h]
  bool v129; // [rsp+88h] [rbp-80h]
  __int64 v130; // [rsp+90h] [rbp-78h]
  __int128 v131; // [rsp+98h] [rbp-70h] BYREF
  unsigned __int64 v132; // [rsp+A8h] [rbp-60h]
  __int64 v133; // [rsp+B0h] [rbp-58h]
  __int128 v134; // [rsp+B8h] [rbp-50h] BYREF
  __int128 v135; // [rsp+C8h] [rbp-40h] BYREF
  __int128 v136; // [rsp+D8h] [rbp-30h]
  __int128 v137; // [rsp+E8h] [rbp-20h]
  __int64 v138; // [rsp+F8h] [rbp-10h] BYREF
  volatile signed __int32 *v139; // [rsp+100h] [rbp-8h]
  __int64 v140; // [rsp+108h] [rbp+0h]
  char v141[8]; // [rsp+110h] [rbp+8h] BYREF
  void (__fastcall ***v142)(_QWORD); // [rsp+118h] [rbp+10h]
  void (__fastcall ***v143)(_QWORD); // [rsp+120h] [rbp+18h]
  __int128 *v144; // [rsp+128h] [rbp+20h]
  __int128 *v145; // [rsp+130h] [rbp+28h]
  __int128 *v146; // [rsp+138h] [rbp+30h]
  void (__fastcall ***v147)(_QWORD); // [rsp+140h] [rbp+38h]
  char v148[8]; // [rsp+148h] [rbp+40h] BYREF
  __int64 v149; // [rsp+150h] [rbp+48h]
  _BYTE v150[16]; // [rsp+158h] [rbp+50h] BYREF
  char v151[16]; // [rsp+168h] [rbp+60h] BYREF
  _QWORD v152[2]; // [rsp+178h] [rbp+70h] BYREF
  unsigned __int64 v153; // [rsp+188h] [rbp+80h]
  unsigned __int64 v154; // [rsp+190h] [rbp+88h]
  _QWORD v155[3]; // [rsp+198h] [rbp+90h] BYREF
  unsigned __int64 v156; // [rsp+1B0h] [rbp+A8h]
  _QWORD v157[3]; // [rsp+1B8h] [rbp+B0h] BYREF
  unsigned __int64 v158; // [rsp+1D0h] [rbp+C8h]

  v140 = -2;
  v3 = a1;
  sub_1441E57B0();
  v5 = 0;
  if ( a2 )
  {
    sub_146B26020(*(_QWORD *)(v3 + 112), 0, v4);
    if ( dword_14E662B80 > *(_DWORD *)(*((_QWORD *)NtCurrentTeb()->ThreadLocalStoragePointer
                                       + (unsigned int)dword_14F3BEE58)
                                     + 420620LL) )
    {
      sub_148860450(&dword_14E662B80);
      if ( dword_14E662B80 == -1 )
      {
        qword_14E662B60 = 0;
        qword_14E662B70 = 0;
        qword_14E662B78 = 7;
        sub_14885FFE8(sub_1490244F0);
        sub_1488603F0(&dword_14E662B80);
      }
    }
    sub_146F50610(*(_QWORD *)(v3 + 2192), &v138, *(unsigned int *)(v3 + 2184));
    v6 = 0;
    v7 = (_QWORD *)(v3 + 248);
    v8 = v3 + 152;
    do
    {
      v9 = sub_146E8C7D0(&unk_14A2322B8);
      v10 = sub_146E8CF20(v150, v9, (unsigned int)v6);
      v11 = sub_14014F430(v10);
      v12 = -1;
      do
        ++v12;
      while ( *(_WORD *)(v11 + 2 * v12) );
      sub_14014C8D0(&qword_14E662B60, v11);
      sub_146E8C910(v150);
      v13 = *(void (__fastcall **)(__int64, __int64))*(v7 - 12);
      v14 = &qword_14E662B60;
      if ( (unsigned __int64)qword_14E662B78 >= 8 )
        v14 = (__int64 *)qword_14E662B60;
      v15 = sub_146EC8E30(v138, v151, v14);
      v13(v8, v15);
      (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)*v7 + 16LL))(*v7, 0);
      ++v6;
      v8 += 400;
      v7 += 50;
    }
    while ( v6 < 5 );
    v16 = v139;
    if ( v139 )
    {
      if ( _InterlockedExchangeAdd(v139 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v16)(v16);
        if ( _InterlockedExchangeAdd(v16 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v16 + 8LL))(v16);
      }
    }
    v5 = 0;
  }
  v123 = 0;
  v17 = 0;
  v124 = 0;
  v18 = *(int **)(v3 + 16);
  v19 = 0;
  while ( v18 != *(int **)(v3 + 24) )
  {
    if ( (unsigned int)sub_14206BB60(*(_QWORD *)(v3 + 40)) == 2 && sub_1444EC2C0((__int64)v18)
      || (unsigned int)sub_14206BB60(*(_QWORD *)(v3 + 40)) == 3 && !sub_1444EC2C0((__int64)v18) )
    {
      goto LABEL_71;
    }
    v20 = *(_QWORD *)(v3 + 40);
    if ( !v20 || (unsigned int)sub_14206BB60(v20) != 1 )
      goto LABEL_28;
    v21 = qword_14E638F28;
    if ( !qword_14E638F28 )
    {
      v22 = sub_146E8BA20(1472);
      if ( v22 )
        v23 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v22);
      else
        v23 = 0;
      qword_14E638F28 = (__int64)v23;
      (**v23)(v23);
      v21 = qword_14E638F28;
    }
    v24 = (__int64 *)sub_1441C3DB0(*(_QWORD *)(v3 + 8), v141);
    if ( !sub_1444EC320(v21, *v18, *v24) )
    {
LABEL_71:
      v18 += 6;
    }
    else
    {
LABEL_28:
      if ( !*(_BYTE *)(v3 + 104) )
        goto LABEL_68;
      (*(void (__fastcall **)(_QWORD, _QWORD *))(**(_QWORD **)(v3 + 56) + 688LL))(*(_QWORD *)(v3 + 56), v152);
      v26 = sub_1444EB940(v18, v157, v25);
      v27 = v26;
      v28 = (unsigned __int16 *)v152;
      if ( v154 >= 8 )
        v28 = (unsigned __int16 *)v152[0];
      v29 = v26[2];
      if ( v26[3] >= 8u )
        v27 = (_WORD *)*v26;
      v5 = v153;
      if ( v153 > v29 )
        goto LABEL_51;
      if ( v153 )
      {
        v31 = &v27[v29 - v153 + 1];
        v32 = v27;
        v33 = *v28;
        while ( 1 )
        {
          v34 = v31 - v32;
          v35 = 0;
          if ( v34 )
          {
            if ( *v32 == v33 )
            {
LABEL_41:
              v35 = v32;
            }
            else
            {
              while ( v34 != 1 )
              {
                --v34;
                if ( *++v32 == v33 )
                  goto LABEL_41;
              }
            }
          }
          v36 = v35;
          if ( !v35 )
            break;
          v37 = v153;
          v38 = v28;
          v39 = *v35;
          if ( v39 >= v33 )
          {
            v40 = v39 <= v33;
            while ( v40 )
            {
              if ( v37 == 1 )
              {
                v30 = v36 - v27;
                v3 = a1;
                goto LABEL_52;
              }
              --v37;
              v41 = *(unsigned __int16 *)((char *)++v38 + (char *)v36 - (char *)v28);
              v40 = v41 <= *v38;
              if ( v41 < *v38 )
                break;
            }
          }
          v32 = v36 + 1;
        }
        v3 = a1;
LABEL_51:
        v30 = -1;
LABEL_52:
        v5 = 0;
        goto LABEL_53;
      }
      v30 = 0;
LABEL_53:
      if ( v158 >= 8 )
      {
        v42 = 2 * v158 + 2;
        v43 = v157[0];
        if ( v42 >= 0x1000 )
        {
          v42 = 2 * v158 + 41;
          v43 = *(_QWORD *)(v157[0] - 8LL);
          if ( (unsigned __int64)(v157[0] - v43 - 8) > 0x1F )
            sub_148AAF304(v43, v42);
        }
        sub_146E9F3A0(v43, v42);
      }
      v157[2] = 0;
      v158 = 7;
      LOWORD(v157[0]) = 0;
      if ( v30 == -1 )
      {
        if ( v154 >= 8 )
        {
          v44 = 2 * v154 + 2;
          v45 = v152[0];
          if ( v44 >= 0x1000 )
          {
            v44 = 2 * v154 + 41;
            v45 = *(_QWORD *)(v152[0] - 8LL);
            if ( (unsigned __int64)(v152[0] - v45 - 8) > 0x1F )
              goto LABEL_188;
          }
          sub_146E9F3A0(v45, v44);
        }
        v153 = 0;
        v154 = 7;
        LOWORD(v152[0]) = 0;
        v18 += 6;
      }
      else
      {
        if ( v154 >= 8 )
        {
          v44 = 2 * v154 + 2;
          v45 = v152[0];
          if ( v44 >= 0x1000 )
          {
            v44 = 2 * v154 + 41;
            v45 = *(_QWORD *)(v152[0] - 8LL);
            if ( (unsigned __int64)(v152[0] - v45 - 8) > 0x1F )
LABEL_188:
              sub_148AAF304(v45, v44);
          }
          sub_146E9F3A0(v45, v44);
        }
        v153 = 0;
        v154 = 7;
        LOWORD(v152[0]) = 0;
LABEL_68:
        if ( v19 == v17 )
        {
          sub_140154010(&v123, v19, v18);
          v17 = v124;
          v19 = (int *)*((_QWORD *)&v123 + 1);
          goto LABEL_71;
        }
        *v19++ = *v18;
        *((_QWORD *)&v123 + 1) = v19;
        v18 += 6;
      }
    }
  }
  v46 = *(_DWORD *)(v3 + 2184);
  v47 = v123;
  if ( v46 == 1 )
  {
    v48 = (int *)v123;
    while ( v48 != v19 )
    {
      v49 = qword_14E638F28;
      if ( !qword_14E638F28 )
      {
        v50 = sub_146E8BA20(1472);
        if ( v50 )
          v51 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v50);
        else
          v51 = 0;
        qword_14E638F28 = (__int64)v51;
        (**v51)(v51);
        v49 = qword_14E638F28;
      }
      v52 = sub_1444EBD90(v49, 2, *v48);
      if ( v52 && sub_1444EC2C0((__int64)v52) )
        goto LABEL_87;
      v53 = qword_14E63AE60;
      if ( !qword_14E63AE60 )
      {
        v54 = sub_146E8BA20(336);
        v130 = v54;
        if ( v54 )
          v55 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v54);
        else
          v55 = 0;
        qword_14E63AE60 = (__int64)v55;
        (**v55)(v55);
        v53 = qword_14E63AE60;
      }
      if ( !(unsigned __int8)sub_1447EAF60(v53, (unsigned int)*v48) )
      {
LABEL_87:
        sub_148AA1E60(v48, v48 + 1, (char *)v19-- - (char *)(v48 + 1));
        *((_QWORD *)&v123 + 1) = v19;
      }
      else
      {
        ++v48;
      }
    }
  }
  else if ( !v46 )
  {
    v56 = (int *)v123;
    if ( (int *)v123 != v19 )
    {
      while ( *v56 != 99999999 )
      {
        if ( ++v56 == v19 )
          goto LABEL_98;
      }
      if ( v56 != v19 )
      {
        sub_148AA1E60(v56, v56 + 1, (char *)v19-- - (char *)(v56 + 1));
        *((_QWORD *)&v123 + 1) = v19;
      }
    }
  }
LABEL_98:
  v57 = sub_1421B2800(*(_QWORD *)(v3 + 112));
  v130 = 0;
  v58 = ((__int64)v19 - v47) >> 2;
  v122 = v58;
  v59 = 5 * v57 - 5;
  v119 = v59;
  v60 = (unsigned int *)(v47 - 20 + 20LL * v57);
  v121 = v60;
  v61 = (__int64 *)(v3 + 344);
  do
  {
    (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)*(v61 - 18) + 16LL))(*(v61 - 18), 0);
    result = v59;
    if ( v58 > v59 )
    {
      result = sub_1444EBAB0(*v60, v62, v63);
      v65 = result;
      if ( result && *(_DWORD *)(result + 8) == 2 )
      {
        v66 = qword_14E638F28;
        if ( !qword_14E638F28 )
        {
          v67 = (void (__fastcall ***)(_QWORD))sub_146E8BA20(1472);
          v142 = v67;
          if ( v67 )
            v67 = (void (__fastcall ***)(_QWORD))sub_1444E81C0((__int64)v67);
          qword_14E638F28 = (__int64)v67;
          (**v67)(v67);
          v66 = qword_14E638F28;
        }
        result = (__int64)sub_1444EBD90(v66, 2, *v60);
        v68 = result;
        if ( result )
        {
          v69 = (__int64 *)*(v61 - 18);
          v70 = *v69;
          LOBYTE(v70) = 1;
          (*(void (__fastcall **)(__int64 *, __int64))(*v69 + 16))(v69, v70);
          v71 = *v60;
          *((_DWORD *)v61 - 38) = *v60;
          v72 = 6;
          if ( !*(_DWORD *)(a1 + 2184) )
            v72 = 2;
          v73 = qword_14E638F28;
          if ( !qword_14E638F28 )
          {
            v74 = (void (__fastcall ***)(_QWORD))sub_146E8BA20(1472);
            v143 = v74;
            if ( v74 )
              v74 = (void (__fastcall ***)(_QWORD))sub_1444E81C0((__int64)v74);
            qword_14E638F28 = (__int64)v74;
            (**v74)(v74);
            v71 = *((_DWORD *)v61 - 38);
            v73 = qword_14E638F28;
          }
          v117 = sub_1444EC5B0(v73, v72, v71);
          *((_DWORD *)v61 - 37) = v72;
          v144 = &v131;
          LOBYTE(v75) = *(_DWORD *)(a1 + 2184) == 1;
          v76 = (_QWORD *)sub_147C1C570(v65, v75);
          *(_QWORD *)&v131 = 0;
          v132 = 0;
          v133 = 0;
          v77 = v76[2];
          if ( v76[3] >= 8u )
            v76 = (_QWORD *)*v76;
          if ( v77 >= 8 )
          {
            v78 = v77 | 7;
            if ( (v77 | 7) > 0x7FFFFFFFFFFFFFFELL )
              v78 = 0x7FFFFFFFFFFFFFFELL;
            *(_QWORD *)&v131 = sub_14014CB50(&v131, v78 + 1);
            sub_148AA1E60(v131, v76, 2 * v77 + 2);
            v133 = v78;
          }
          else
          {
            v131 = *(_OWORD *)v76;
            v133 = 7;
          }
          v132 = v77;
          v145 = &v134;
          v134 = 0;
          v79 = *(v61 - 2);
          if ( v79 )
          {
            _InterlockedIncrement((volatile signed __int32 *)(v79 + 8));
            v79 = *(v61 - 2);
          }
          *(_QWORD *)&v134 = *(v61 - 3);
          *((_QWORD *)&v134 + 1) = v79;
          v146 = &v135;
          v135 = 0;
          v80 = *v61;
          if ( *v61 )
          {
            _InterlockedIncrement((volatile signed __int32 *)(v80 + 8));
            v80 = *v61;
          }
          *(_QWORD *)&v135 = *(v61 - 1);
          *((_QWORD *)&v135 + 1) = v80;
          sub_1441EC970(&v135, &v134, &v131, 0);
          v81 = v61[1];
          v82 = *(void (__fastcall **)(__int64, _QWORD *))(*(_QWORD *)v81 + 688LL);
          LOBYTE(v83) = *(_DWORD *)(a1 + 2184) == 0;
          v84 = (_QWORD *)sub_147C1C530(v65, v83);
          if ( v84[3] >= 8u )
            v84 = (_QWORD *)*v84;
          v82(v81, v84);
          v85 = **(unsigned int **)(*(_QWORD *)(a1 + 8) + 24 * v72 + 3352);
          LOBYTE(v85) = *((_DWORD *)v61 - 38) == (_DWORD)v85;
          (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v61[17] + 16LL))(v61[17], v85);
          (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v61[19] + 16LL))(v61[19], 0);
          (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v61[21] + 16LL))(v61[21], 0);
          LOBYTE(v86) = 1;
          if ( *(_BYTE *)(v68 + 20) )
            v87 = v61[19];
          else
            v87 = v61[21];
          (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v87 + 16LL))(v87, v86);
          v3 = a1;
          v88 = *(_QWORD *)(a1 + 8);
          v89 = *(_DWORD **)(v88 + 3328);
          v90 = *(_DWORD **)(v88 + 3336);
          if ( v89 == v90 )
          {
LABEL_132:
            v90 = 0;
          }
          else
          {
            while ( *v89 != *((_DWORD *)v61 - 38) )
            {
              if ( ++v89 == v90 )
                goto LABEL_132;
            }
            LOBYTE(v90) = 1;
          }
          (*(void (__fastcall **)(__int64, _DWORD *))(*(_QWORD *)v61[5] + 16LL))(v61[5], v90);
          sub_1444EB5F0(v68, v155);
          v91 = v155;
          if ( v156 >= 8 )
            v91 = (_QWORD *)v155[0];
          (*(void (__fastcall **)(__int64, _QWORD *))(*(_QWORD *)v61[3] + 688LL))(v61[3], v91);
          v92 = *(v61 - 21);
          v93 = *(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v92 + 16LL);
          v94 = qword_14E638F28;
          if ( !qword_14E638F28 )
          {
            v95 = (void (__fastcall ***)(_QWORD))sub_146E8BA20(1472);
            v147 = v95;
            if ( v95 )
              v95 = (void (__fastcall ***)(_QWORD))sub_1444E81C0((__int64)v95);
            qword_14E638F28 = (__int64)v95;
            (**v95)(v95);
            v94 = qword_14E638F28;
          }
          v96 = (__int64 *)sub_1441C3DB0(*(_QWORD *)(a1 + 8), v148);
          v97 = sub_1444EC320(v94, *((_DWORD *)v61 - 38), *v96);
          v93(v92, v97);
          v98 = sub_1444EC2C0(v68);
          v99 = v61[11];
          v100 = *(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v99 + 16LL);
          if ( v98 )
          {
            v100(v99, 0);
            (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v61[7] + 16LL))(v61[7], 0);
            (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v61[9] + 16LL))(v61[9], 0);
            LOBYTE(v101) = 1;
          }
          else
          {
            v100(v99, v117);
            LOBYTE(v102) = v117 == 0;
            (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v61[7] + 16LL))(v61[7], v102);
            (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v61[9] + 16LL))(v61[9], v117);
            v101 = 0;
          }
          (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v61[13] + 16LL))(v61[13], v101);
          v125 = 0;
          v126 = 0;
          v127 = 0;
          v128 = -1;
          v129 = 0;
          v129 = sub_1444EC2C0(v68);
          v136 = 0;
          v103 = *(v61 - 22);
          if ( v103 )
          {
            _InterlockedIncrement((volatile signed __int32 *)(v103 + 8));
            v103 = *(v61 - 22);
          }
          v104 = *(v61 - 23);
          v136 = v125;
          *(_QWORD *)&v125 = v104;
          v105 = (volatile signed __int32 *)*((_QWORD *)&v125 + 1);
          *((_QWORD *)&v125 + 1) = v103;
          if ( *((_QWORD *)&v136 + 1) )
          {
            if ( _InterlockedExchangeAdd(v105 + 2, 0xFFFFFFFF) == 1 )
            {
              (**(void (__fastcall ***)(volatile signed __int32 *))v105)(v105);
              if ( _InterlockedExchangeAdd(v105 + 3, 0xFFFFFFFF) == 1 )
                (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v105 + 8LL))(v105);
            }
          }
          v137 = 0;
          v106 = *(v61 - 20);
          if ( v106 )
          {
            _InterlockedIncrement((volatile signed __int32 *)(v106 + 8));
            v106 = *(v61 - 20);
          }
          v107 = *(v61 - 21);
          v137 = v126;
          *(_QWORD *)&v126 = v107;
          v108 = (volatile signed __int32 *)*((_QWORD *)&v126 + 1);
          *((_QWORD *)&v126 + 1) = v106;
          if ( *((_QWORD *)&v137 + 1) )
          {
            if ( _InterlockedExchangeAdd(v108 + 2, 0xFFFFFFFF) == 1 )
            {
              (**(void (__fastcall ***)(volatile signed __int32 *))v108)(v108);
              if ( _InterlockedExchangeAdd(v108 + 3, 0xFFFFFFFF) == 1 )
                (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v108 + 8LL))(v108);
            }
          }
          v127 = *((_DWORD *)v61 - 38);
          v128 = *((_DWORD *)v61 - 37);
          v109 = *(_QWORD *)(a1 + 8) + 3280LL;
          v110 = *(_QWORD *)(*(_QWORD *)(a1 + 8) + 3288LL);
          if ( v110 == *(_QWORD *)(*(_QWORD *)(a1 + 8) + 3296LL) )
          {
            result = sub_1441B7240(v109, v110, &v125);
          }
          else
          {
            v149 = *(_QWORD *)(*(_QWORD *)(a1 + 8) + 3288LL);
            *(_QWORD *)v110 = 0;
            *(_QWORD *)(v110 + 8) = 0;
            if ( *((_QWORD *)&v125 + 1) )
              _InterlockedIncrement((volatile signed __int32 *)(*((_QWORD *)&v125 + 1) + 8LL));
            *(_OWORD *)v110 = v125;
            *(_QWORD *)(v110 + 16) = 0;
            *(_QWORD *)(v110 + 24) = 0;
            if ( *((_QWORD *)&v126 + 1) )
              _InterlockedIncrement((volatile signed __int32 *)(*((_QWORD *)&v126 + 1) + 8LL));
            *(_OWORD *)(v110 + 16) = v126;
            *(_DWORD *)(v110 + 32) = v127;
            *(_DWORD *)(v110 + 36) = v128;
            result = v129;
            *(_BYTE *)(v110 + 40) = v129;
            *(_QWORD *)(v109 + 8) += 48LL;
          }
          v111 = (volatile signed __int32 *)*((_QWORD *)&v126 + 1);
          if ( *((_QWORD *)&v126 + 1) )
          {
            result = (unsigned int)_InterlockedExchangeAdd(
                                     (volatile signed __int32 *)(*((_QWORD *)&v126 + 1) + 8LL),
                                     0xFFFFFFFF);
            if ( (_DWORD)result == 1 )
            {
              (**(void (__fastcall ***)(volatile signed __int32 *))v111)(v111);
              result = (unsigned int)_InterlockedExchangeAdd(v111 + 3, 0xFFFFFFFF);
              if ( (_DWORD)result == 1 )
                result = (*(__int64 (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v111 + 8LL))(v111);
            }
          }
          v112 = (volatile signed __int32 *)*((_QWORD *)&v125 + 1);
          if ( *((_QWORD *)&v125 + 1) )
          {
            result = (unsigned int)_InterlockedExchangeAdd(
                                     (volatile signed __int32 *)(*((_QWORD *)&v125 + 1) + 8LL),
                                     0xFFFFFFFF);
            if ( (_DWORD)result == 1 )
            {
              (**(void (__fastcall ***)(volatile signed __int32 *))v112)(v112);
              result = (unsigned int)_InterlockedExchangeAdd(v112 + 3, 0xFFFFFFFF);
              if ( (_DWORD)result == 1 )
                result = (*(__int64 (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v112 + 8LL))(v112);
            }
          }
          if ( v156 >= 8 )
          {
            v113 = 2 * v156 + 2;
            v114 = v155[0];
            if ( v113 >= 0x1000 )
            {
              v113 = 2 * v156 + 41;
              v114 = *(_QWORD *)(v155[0] - 8LL);
              if ( (unsigned __int64)(v155[0] - v114 - 8) > 0x1F )
                sub_148AAF304(v114, v113);
            }
            result = sub_146E9F3A0(v114, v113);
          }
          v155[2] = 0;
          v156 = 7;
          LOWORD(v155[0]) = 0;
          v60 = v121;
          v5 = v130;
          v59 = v119;
        }
      }
      v58 = v122;
    }
    v119 = ++v59;
    v130 = ++v5;
    v121 = ++v60;
    v61 += 50;
  }
  while ( v5 < 5 );
  if ( a2 )
    result = sub_146B26100(*(_QWORD *)(v3 + 112), (unsigned int)v58 + (v58 != 5 * (v58 / 5)));
  v115 = v123;
  if ( (_QWORD)v123 )
  {
    v116 = ((unsigned __int64)v124 - v123) & 0xFFFFFFFFFFFFFFFCuLL;
    if ( v116 >= 0x1000 )
    {
      v116 += 39LL;
      v115 = *(_QWORD *)(v123 - 8);
      if ( (unsigned __int64)(v123 - v115 - 8) > 0x1F )
        sub_148AAF304(v115, v116);
    }
    result = sub_146E9F3A0(v115, v116);
    v123 = 0;
    v124 = 0;
  }
  return result;
}

