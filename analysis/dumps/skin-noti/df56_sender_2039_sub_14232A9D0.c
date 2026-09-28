// sender_2039_sub_14232A9D0

_UNKNOWN **__fastcall sub_14232A9D0(__int64 a1, char a2)
{
  _UNKNOWN **result; // rax
  __int64 v3; // r15
  __int64 v4; // rcx
  __int64 v5; // rax
  __int64 v6; // rcx
  __int64 v7; // rax
  unsigned int v8; // eax
  __int64 v9; // rax
  unsigned int *v10; // rsi
  __int64 v11; // rax
  _QWORD *v12; // r13
  unsigned int v13; // eax
  __int64 v14; // rcx
  __int64 *v15; // rdx
  __int64 *v16; // rax
  __int64 *v17; // rbx
  _BYTE *v18; // rsi
  _BYTE *j; // rbx
  _QWORD *v20; // rdx
  signed int i; // ebx
  _QWORD *v22; // rdx
  unsigned __int64 v23; // rsi
  __int64 v24; // rbx
  __int64 v25; // r12
  _DWORD *v26; // r14
  unsigned __int8 v27; // dl
  unsigned __int64 v28; // rbx
  __int64 v29; // rcx
  unsigned __int64 v30; // rbx
  unsigned __int64 v31; // rdx
  __int64 v32; // rax
  unsigned __int64 v33; // rbx
  __int64 v34; // rax
  __int64 v35; // rax
  __int64 v36; // rax
  __int64 v37; // r8
  unsigned __int8 *v38; // r13
  unsigned __int8 v39; // bl
  unsigned __int8 *v40; // r15
  unsigned int v41; // edi
  __int64 v42; // rax
  __int64 v43; // rax
  __int64 v44; // rax
  unsigned __int64 v45; // rdx
  _QWORD *v46; // rcx
  char *v47; // rsi
  unsigned __int8 *v48; // rdi
  int v49; // esi
  __int64 v50; // rax
  void (__fastcall ***v51)(_QWORD); // rcx
  unsigned __int8 *v52; // rdi
  int v53; // ebx
  int v54; // eax
  unsigned __int64 v55; // rdx
  __int64 v56; // rcx
  int v57; // eax
  _UNKNOWN **v58; // rbx
  __int64 v59; // rax
  __int64 v60; // rax
  __int64 v61; // r12
  __int64 v62; // rsi
  __int64 v63; // rbx
  __int64 v64; // rcx
  __int64 v65; // rdi
  __int64 v66; // r9
  __int64 v67; // rdx
  int v68; // eax
  __int64 v69; // rsi
  __int64 v70; // rbx
  __int64 v71; // rcx
  __int64 v72; // rdi
  float v73; // xmm6_4
  int v74; // eax
  int v75; // eax
  __int64 v76; // rcx
  __int64 v77; // rax
  __int64 v78; // rcx
  __int64 v79; // rax
  __int64 v80; // rdx
  __int64 v81; // rcx
  __int64 v82; // rdx
  __int64 v83; // r8
  __int64 v84; // r9
  int v85; // esi
  __int64 v86; // rbx
  int v87; // edi
  int v88; // eax
  __int64 v89; // rax
  __int64 v90; // rdx
  __int64 v91; // rax
  _UNKNOWN **v92; // r13
  unsigned int v93; // esi
  _QWORD *v94; // rdx
  __int64 v95; // r8
  unsigned __int64 v96; // rcx
  unsigned __int64 v97; // rdi
  unsigned int v98; // ebx
  unsigned __int8 *v99; // rax
  __int64 v100; // rax
  __int64 v101; // rax
  __int64 v102; // rax
  __int64 v103; // rax
  __int64 v104; // r8
  unsigned __int64 v105; // rdx
  __int64 v106; // rcx
  __int64 v107; // rbx
  __int64 v108; // rax
  __int64 v109; // rcx
  __int64 v110; // r8
  __int64 v111; // r9
  unsigned __int64 v112; // rdx
  unsigned __int64 v113; // rdx
  __int64 v114; // rcx
  int v115; // esi
  __int64 v116; // rbx
  __int64 v117; // rax
  __int64 v118; // rax
  __int64 v119; // rbx
  int v120; // edi
  int v121; // eax
  int v122; // eax
  __int64 v123; // rax
  int v124; // esi
  __int64 v125; // rbx
  __int64 v126; // rax
  __int64 v127; // rax
  __int64 v128; // rbx
  int v129; // edi
  int v130; // eax
  int v131; // [rsp+20h] [rbp-A9h]
  __int64 v132; // [rsp+50h] [rbp-79h] BYREF
  unsigned __int8 v133; // [rsp+58h] [rbp-71h]
  __int64 v134; // [rsp+60h] [rbp-69h]
  unsigned __int8 v135[16]; // [rsp+68h] [rbp-61h] BYREF
  _QWORD *v136; // [rsp+78h] [rbp-51h]
  unsigned __int64 v137; // [rsp+80h] [rbp-49h]
  _QWORD v138[2]; // [rsp+88h] [rbp-41h] BYREF
  __int64 v139; // [rsp+98h] [rbp-31h]
  unsigned __int64 v140; // [rsp+A0h] [rbp-29h]
  _QWORD v141[2]; // [rsp+A8h] [rbp-21h] BYREF
  __int64 v142; // [rsp+B8h] [rbp-11h]
  unsigned __int64 v143; // [rsp+C0h] [rbp-9h]
  _BYTE v144[15]; // [rsp+C8h] [rbp-1h] BYREF
  int v145; // [rsp+D7h] [rbp+Eh]
  _UNKNOWN *retaddr; // [rsp+128h] [rbp+5Fh] BYREF

  result = &retaddr;
  v134 = -2;
  v3 = a1;
  v138[0] = a1;
  if ( *(_BYTE *)(a1 + 48) == a2 )
    return result;
  *(_BYTE *)(a1 + 48) = a2;
  v4 = (unsigned int)(a2 - 1);
  if ( a2 == 1 )
  {
    ++*(_DWORD *)(v3 + 424);
    v60 = sub_1423280B0(v4);
    v61 = -1;
    if ( (int)sub_14289F980(v60, 5) > 0 && sub_145EFAFB0() )
    {
      v62 = sub_14029CC70(95);
      v63 = sub_145EFAFB0();
      v65 = sub_1423280B0(v64);
      (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v63 + 3016LL))(v63);
      sub_14289F900(v65, 5);
      LOBYTE(v66) = 1;
      (*(void (__fastcall **)(__int64, __int64, _QWORD, __int64, int))(*(_QWORD *)v62 + 1360LL))(v62, v67, 0, v66, 1);
      LODWORD(v63) = sub_145EFAFB0();
      v138[0] = &v132;
      v132 = 0;
      v68 = sub_145EFAFB0();
      sub_145C01600(v63, v62, v68, 0, (__int64)&v132, 0, -1, 0, 0);
      v69 = sub_14029CC70(98);
      v70 = sub_145EFAFB0();
      v72 = sub_1423280B0(v71);
      v73 = (float)(*(int (__fastcall **)(__int64))(*(_QWORD *)v70 + 3232LL))(v70);
      sub_14289F900(v72, 5);
      (*(void (__fastcall **)(__int64, _QWORD, _QWORD, __int64))(*(_QWORD *)v69 + 1360LL))(
        v69,
        (unsigned int)(int)(float)((float)((float)v74 * v73) * 0.0099999998),
        0,
        1);
      LODWORD(v70) = sub_145EFAFB0();
      v141[0] = &v132;
      v132 = 0;
      v75 = sub_145EFAFB0();
      sub_145C01600(v70, v69, v75, 0, (__int64)&v132, 0, -1, 0, 0);
    }
    if ( *(_DWORD *)sub_14232BDA0(v3) == 5 )
      *(_BYTE *)(v3 + 568) = 1;
    v77 = sub_146D74000(v76);
    sub_146D746E0(v77, 1945);
    v144[13] = *(_BYTE *)(v3 + 56);
    v144[14] = *(_BYTE *)(v3 + 57);
    v145 = *(_DWORD *)(v3 + 92);
    v79 = sub_146D74000(v78);
    sub_146D75B10(v79, v144, 19);
    sub_146D75AF0(v81, v80);
    sub_14232C120(v3, v135, v3 + 56);
    if ( v136 )
    {
      v89 = v136[1];
      v90 = *v136;
      if ( *v136 != v89 )
      {
        result = (_UNKNOWN **)(v89 - v90);
        if ( (unsigned __int64)result <= 3 )
        {
          if ( qword_14E66C090 )
          {
            result = (_UNKNOWN **)sub_1459A9080(qword_14E66C090);
            if ( result )
            {
              v91 = sub_1459A9080(qword_14E66C090);
              result = (_UNKNOWN **)sub_145B2DEF0(v91);
              v92 = result;
              if ( result )
              {
                v93 = 0;
                v94 = v136;
                v95 = *v136;
                v96 = v136[1] - *v136;
                if ( v96 )
                {
                  v97 = 0;
                  do
                  {
                    v98 = 0;
                    if ( v96 <= v97 )
                      sub_1401790B0(v96, v94);
                    if ( v135[0] < (unsigned __int64)((__int64)(*(_QWORD *)(v3 + 8) - *(_QWORD *)v3) >> 1) )
                    {
                      v99 = (unsigned __int8 *)(*(_QWORD *)v3 + 2LL * v135[0]);
                      if ( v99 )
                      {
                        v100 = sub_1475C5C40(v3 + 168, *v99, v99[1], *(unsigned __int8 *)(v97 + v95));
                        if ( v100 )
                        {
                          v98 = 99;
                          if ( *(_BYTE *)(v100 + 4) != 1 )
                            v98 = *(_DWORD *)v100;
                        }
                      }
                    }
                    v101 = sub_146E8C7D0(&unk_14997D3E0);
                    v102 = sub_146E8CF20(v138, v101, v93);
                    v103 = sub_14014F430(v102);
                    v141[0] = 0;
                    v142 = 0;
                    v143 = 7;
                    v104 = -1;
                    do
                      ++v104;
                    while ( *(_WORD *)(v103 + 2 * v104) );
                    sub_14014C8D0(v141, v103);
                    sub_144CCDF70(v92, v141, v98);
                    if ( v143 >= 8 )
                    {
                      v105 = 2 * v143 + 2;
                      v106 = v141[0];
                      if ( v105 >= 0x1000 )
                      {
                        v105 = 2 * v143 + 41;
                        v106 = *(_QWORD *)(v141[0] - 8LL);
                        if ( (unsigned __int64)(v141[0] - v106 - 8) > 0x1F )
                          sub_148AAF304(v106, v105);
                      }
                      sub_146E9F3A0(v106, v105);
                    }
                    v142 = 0;
                    v143 = 7;
                    LOWORD(v141[0]) = 0;
                    sub_146E8C910(v138);
                    ++v93;
                    ++v97;
                    v94 = v136;
                    v95 = *v136;
                    v96 = v136[1] - *v136;
                  }
                  while ( (int)v93 < v96 );
                }
                v107 = v94[1] - *v94;
                *(_QWORD *)(v3 + 72) = v107;
                v108 = sub_146E8C7D0(&unk_14997CC48);
                v138[0] = 0;
                v139 = 0;
                v140 = 7;
                do
                  ++v61;
                while ( *(_WORD *)(v108 + 2 * v61) );
                sub_14014C8D0(v138, v108);
                sub_144CCDF70(v92, v138, (unsigned int)v107);
                v112 = v140;
                if ( v140 >= 8 )
                {
                  v113 = 2 * v140 + 2;
                  v114 = v138[0];
                  if ( v113 >= 0x1000 )
                  {
                    v113 = 2 * v140 + 41;
                    v114 = *(_QWORD *)(v138[0] - 8LL);
                    if ( (unsigned __int64)(v138[0] - v114 - 8) > 0x1F )
                      sub_148AAF304(v114, v113);
                  }
                  sub_146E9F3A0(v114, v113);
                }
                v139 = 0;
                v140 = 7;
                LOWORD(v138[0]) = 0;
                v115 = sub_14021A860(v109, v112, v110, v111, v131);
                v116 = v136[1] - *v136;
                v117 = sub_146E8C7D0(&unk_14997D410);
                v118 = sub_146E8CF20(v138, v117, v116);
                v119 = sub_14014F430(v118);
                v120 = sub_146E8C7D0(&unk_14997D300);
                v121 = sub_146E8C7D0(&unk_14997CD40);
                sub_146E938E0(v115, 0, v121, v120, 802, (__int64)&qword_14E652208, v119);
                sub_146E8C910(v138);
                v122 = sub_14667BB90(qword_14E683C78, 183, 0);
                result = (_UNKNOWN **)sub_148AA307C(
                                        v122,
                                        0,
                                        (unsigned int)&off_14DCB4760,
                                        (unsigned int)&off_14DD97018,
                                        0);
                if ( result )
                {
                  v123 = sub_14087B670(result);
                  return (_UNKNOWN **)sub_142328470(v123);
                }
              }
            }
          }
          return result;
        }
      }
      v124 = sub_14021A860(v136, v90, v83, v84, v131);
      v125 = v136[1] - *v136;
      v126 = sub_146E8C7D0(&unk_14997D380);
      v127 = sub_146E8CF20(v138, v126, v125);
      v128 = sub_14014F430(v127);
      v129 = sub_146E8C7D0(&unk_14997D300);
      v130 = sub_146E8C7D0(&unk_14997CD40);
      sub_146E938E0(v124, 0, v130, v129, 765, (__int64)&qword_14E652208, v128);
      sub_146E8C910(v138);
    }
    else
    {
      v85 = sub_14021A860(0, v82, v83, v84, v131);
      v86 = sub_146E8C7D0(&unk_14997D2A0);
      v87 = sub_146E8C7D0(&unk_14997D300);
      v88 = sub_146E8C7D0(&unk_14997CD40);
      sub_146E938E0(v85, 0, v88, v87, 759, (__int64)&qword_14E652208, v86);
    }
    return (_UNKNOWN **)sub_14289E800(*(_QWORD *)(v3 + 160), 5);
  }
  if ( a2 != 2 )
    return result;
  v5 = sub_1423280B0(v4);
  if ( (int)sub_1428A13B0(v5, 6) > 0 )
  {
    if ( *(_DWORD *)(v3 + 424) )
    {
      v7 = sub_1423280B0(v6);
      v8 = sub_1428A12F0(v7, 6);
      if ( !(*(_DWORD *)(v3 + 424) % (int)v8) )
      {
        v9 = sub_1423280B0(v8);
        sub_1428A4E50(v9);
      }
    }
  }
  v10 = (unsigned int *)sub_14232BDA0(v3);
  v141[0] = v10;
  v11 = sub_1475C5E40(v3 + 168, *(unsigned __int8 *)(v3 + 56));
  v12 = (_QWORD *)v11;
  if ( !v10 || !v11 )
    return (_UNKNOWN **)sub_14289E800(*(_QWORD *)(v3 + 160), 5);
  v13 = sub_1475BFA80(v3 + 168, *(unsigned __int8 *)(v3 + 56), *v10);
  sub_14232B5F0(v3, v13);
  *(_QWORD *)(v3 + 32) = *(_QWORD *)(v3 + 24);
  *(_BYTE *)(v3 + 80) = 0;
  *(_QWORD *)(v3 + 84) = 0;
  sub_146E9FBD0(v3 + 128, 0, 0);
  sub_146E9FBD0(v3 + 96, 0, 0);
  sub_146E9FBD0(v3 + 432, 0, 0);
  v14 = *v10;
  if ( (unsigned int)(v14 - 2) <= 2 || (_DWORD)v14 == 7 )
  {
    v15 = (__int64 *)*((_QWORD *)v10 + 2);
    v16 = (__int64 *)v15[1];
    v17 = v15;
    if ( !*((_BYTE *)v16 + 25) )
    {
      v14 = *(unsigned __int8 *)(v3 + 56);
      do
      {
        if ( *((_BYTE *)v16 + 32) >= (unsigned __int8)v14 )
        {
          v17 = v16;
          v16 = (__int64 *)*v16;
        }
        else
        {
          v16 = (__int64 *)v16[2];
        }
      }
      while ( !*((_BYTE *)v16 + 25) );
    }
    if ( *((_BYTE *)v17 + 25) || *(_BYTE *)(v3 + 56) < *((_BYTE *)v17 + 32) || v17 == v15 )
    {
      for ( i = 0; i < (int)v10[2]; ++i )
      {
        LOBYTE(v132) = 0;
        HIDWORD(v132) = 0;
        v22 = *(_QWORD **)(v3 + 32);
        if ( v22 == *(_QWORD **)(v3 + 40) )
        {
          sub_140179DD0(v3 + 24, v22, &v132);
        }
        else
        {
          *v22 = v132;
          *(_QWORD *)(v3 + 32) += 8LL;
        }
      }
    }
    else
    {
      v18 = (_BYTE *)v17[6];
      for ( j = (_BYTE *)v17[5]; j != v18; ++j )
      {
        HIDWORD(v132) = 0;
        LOBYTE(v132) = *j;
        v20 = *(_QWORD **)(v3 + 32);
        if ( v20 == *(_QWORD **)(v3 + 40) )
        {
          sub_140179DD0(v3 + 24, v20, &v132);
        }
        else
        {
          *v20 = v132;
          *(_QWORD *)(v3 + 32) += 8LL;
        }
      }
    }
    v23 = 0;
    v24 = *(_QWORD *)(v3 + 32);
    v25 = *(_QWORD *)(v3 + 24);
    if ( (v24 - v25) >> 3 )
    {
      v26 = (_DWORD *)v141[0];
      while ( 1 )
      {
        v27 = *(_BYTE *)(v25 + 8 * v23);
        if ( v27 )
          goto LABEL_41;
        v28 = v12[3] - v12[2];
        v27 = *(_BYTE *)((int)sub_146E9BA90(v14) % v28 + v12[2]);
        *(_BYTE *)(v25 + 8 * v23) = v27;
        v29 = (__int64)(*(_QWORD *)(v3 + 32) - *(_QWORD *)(v3 + 24)) >> 3;
        if ( v23 + 1 != v29 )
          goto LABEL_41;
        if ( *v26 == 3 )
          break;
        if ( *v26 == 4 )
        {
          v33 = v12[6] - v12[5];
          v31 = (int)sub_146E9BA90(v29) % v33;
          v32 = v12[5];
          goto LABEL_40;
        }
LABEL_41:
        *(_DWORD *)(v25 + 8 * v23++ + 4) = sub_14232C3B0(v3, v27);
        v24 = *(_QWORD *)(v3 + 32);
        v25 = *(_QWORD *)(v3 + 24);
        if ( v23 >= (v24 - v25) >> 3 )
          goto LABEL_42;
      }
      v30 = v12[9] - v12[8];
      v31 = (int)sub_146E9BA90(v29) % v30;
      v32 = v12[8];
LABEL_40:
      v27 = *(_BYTE *)(v31 + v32);
      *(_BYTE *)(v25 + 8 * v23) = v27;
      goto LABEL_41;
    }
LABEL_42:
    v34 = sub_146E8C7D0(&unk_14997D480);
    v35 = sub_146E8CF20(v141, v34, (v24 - v25) >> 3);
    v36 = sub_14014F430(v35);
    *(_QWORD *)v135 = 0;
    v136 = 0;
    v137 = 7;
    v37 = -1;
    do
      ++v37;
    while ( *(_WORD *)(v36 + 2 * v37) );
    sub_14014C8D0(v135, v36);
    sub_146E8C910(v141);
    v38 = *(unsigned __int8 **)(v3 + 24);
    if ( v38 != *(unsigned __int8 **)(v3 + 32) )
    {
      v39 = v133;
      v40 = *(unsigned __int8 **)(v3 + 32);
      do
      {
        v41 = *v38;
        v42 = sub_146E8C7D0(&unk_1491CE4D8);
        v43 = sub_146E8CF20(v141, v42, v41);
        v44 = sub_14014F430(v43);
        v45 = -1;
        do
          ++v45;
        while ( *(_WORD *)(v44 + 2 * v45) );
        v46 = v136;
        if ( v45 > v137 - (unsigned __int64)v136 )
        {
          sub_1401E8C00((unsigned int)v135, v45, v39, v44, v45);
        }
        else
        {
          v47 = (char *)v136 + v45;
          v136 = (_QWORD *)((char *)v136 + v45);
          v48 = v135;
          if ( v137 >= 8 )
            v48 = *(unsigned __int8 **)v135;
          sub_148AA1E60(&v48[2 * (_QWORD)v46], v44, 2 * v45);
          *(_WORD *)&v48[2 * (_QWORD)v47] = 0;
        }
        sub_146E8C910(v141);
        v38 += 8;
      }
      while ( v38 != v40 );
      v3 = v138[0];
    }
    v49 = qword_14E6343D0;
    if ( !qword_14E6343D0 )
    {
      v50 = sub_146E8BA20(72);
      v138[0] = v50;
      if ( v50 )
        v51 = (void (__fastcall ***)(_QWORD))sub_146E93360(v50);
      else
        v51 = 0;
      qword_14E6343D0 = (__int64)v51;
      (**v51)(v51);
      v49 = qword_14E6343D0;
    }
    v52 = v135;
    if ( v137 >= 8 )
      v52 = *(unsigned __int8 **)v135;
    v53 = sub_146E8C7D0(&unk_14997D300);
    v54 = sub_146E8C7D0(&unk_14997CD40);
    sub_146E938E0(v49, 0, v54, v53, 906, (__int64)&qword_14E652208, (__int64)v52);
    if ( v137 >= 8 )
    {
      v55 = 2 * v137 + 2;
      v56 = *(_QWORD *)v135;
      if ( v55 >= 0x1000 )
      {
        v55 = 2 * v137 + 41;
        v56 = *(_QWORD *)(*(_QWORD *)v135 - 8LL);
        if ( (unsigned __int64)(*(_QWORD *)v135 - v56 - 8) > 0x1F )
          sub_148AAF304(v56, v55);
      }
      sub_146E9F3A0(v56, v55);
    }
    v136 = 0;
    v137 = 7;
    *(_WORD *)v135 = 0;
  }
  v57 = sub_14667BB90(qword_14E683C78, 183, 0);
  result = (_UNKNOWN **)sub_148AA307C(v57, 0, (unsigned int)&off_14DCB4760, (unsigned int)&off_14DD97018, 0);
  v58 = result;
  if ( result )
  {
    sub_142C1C950(result);
    v59 = sub_14087B670(v58);
    return (_UNKNOWN **)sub_142329A40(v59, *(unsigned __int8 *)(v3 + 56), *(unsigned __int8 *)(v3 + 57));
  }
  return result;
}

