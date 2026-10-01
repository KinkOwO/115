// sub_1438B4A20  va=0x1438B4A20  size=4086

__int64 __fastcall sub_1438B4A20(_QWORD *a1, __int64 a2)
{
  _QWORD *v3; // rsi
  __int64 v4; // r8
  char *v5; // r12
  __int64 v6; // rax
  __int64 v7; // rcx
  unsigned __int16 *v8; // r9
  signed __int64 v9; // r8
  unsigned __int16 v10; // cx
  bool v11; // cc
  unsigned __int16 v12; // cx
  char v13; // al
  __int64 v14; // rax
  __int64 v15; // rcx
  unsigned __int16 *v16; // r9
  signed __int64 v17; // r8
  unsigned __int16 v18; // cx
  bool v19; // cc
  unsigned __int16 v20; // cx
  __int64 v21; // rax
  __int64 v22; // rcx
  unsigned __int16 *v23; // r9
  signed __int64 v24; // r8
  unsigned __int16 v25; // cx
  bool v26; // cc
  unsigned __int16 v27; // cx
  __int64 v28; // rax
  __int64 v29; // rcx
  unsigned __int16 *v30; // r9
  signed __int64 v31; // r8
  unsigned __int16 v32; // cx
  bool v33; // cc
  unsigned __int16 v34; // cx
  __int64 v35; // rax
  __int64 v36; // rcx
  unsigned __int16 *v37; // r9
  signed __int64 v38; // r8
  unsigned __int16 v39; // cx
  bool v40; // cc
  unsigned __int16 v41; // cx
  char *v42; // rax
  __int64 v43; // rcx
  char *v44; // r9
  signed __int64 v45; // r8
  unsigned __int16 v46; // cx
  signed __int64 v47; // r9
  bool v48; // cc
  unsigned __int16 v49; // cx
  char v50; // al
  __int64 v51; // r8
  __int64 v52; // r9
  char v53; // si
  __int64 v54; // rdx
  __int64 v55; // r15
  __int64 v56; // rax
  char *v57; // r10
  __int64 v58; // r9
  __int64 v59; // r11
  char *v60; // rax
  int v61; // ecx
  int v62; // edx
  unsigned __int8 v63; // bl
  __int64 v64; // r8
  __int64 v65; // r9
  __int64 v66; // rdx
  int v67; // eax
  int v68; // ecx
  int v69; // ecx
  int v70; // ecx
  int v71; // ecx
  int v72; // ecx
  int v73; // ecx
  int v74; // ecx
  __int64 v75; // rax
  __int64 v76; // rcx
  char *v77; // rbx
  __int64 v78; // rdi
  __int64 v79; // r8
  __int64 v80; // r10
  char *v81; // rax
  int v82; // edx
  unsigned __int16 *v83; // rax
  int v84; // edx
  __int64 v85; // rax
  __int64 v86; // rax
  char *v87; // rbx
  __int64 v88; // rdi
  __int64 v89; // r8
  __int64 v90; // r10
  char *v91; // rax
  int v92; // edx
  unsigned __int16 *v93; // rax
  int v94; // edx
  __int64 v95; // rax
  int *v96; // rdx
  int *v97; // rdx
  int i; // ebx
  unsigned __int64 v99; // rdx
  __int64 v100; // rcx
  __int64 v101; // rax
  __int64 v102; // r8
  __int64 v103; // r9
  int *v104; // rbx
  __int64 *v105; // r8
  __int64 *v106; // rax
  __int64 *v107; // rcx
  unsigned __int64 v108; // rdx
  __int64 v109; // rcx
  unsigned __int64 v110; // rdx
  __int64 v111; // rcx
  _DWORD v113[4]; // [rsp+20h] [rbp-E0h] BYREF
  __int128 v114; // [rsp+30h] [rbp-D0h] BYREF
  __int128 v115; // [rsp+40h] [rbp-C0h]
  __int64 v116; // [rsp+50h] [rbp-B0h]
  int v117; // [rsp+58h] [rbp-A8h] BYREF
  char v118; // [rsp+5Ch] [rbp-A4h]
  int v119; // [rsp+60h] [rbp-A0h] BYREF
  int *v120; // [rsp+68h] [rbp-98h] BYREF
  int *v121; // [rsp+70h] [rbp-90h]
  int *v122; // [rsp+78h] [rbp-88h]
  _BYTE v123[24]; // [rsp+80h] [rbp-80h] BYREF
  char v124[24]; // [rsp+98h] [rbp-68h] BYREF
  int v125; // [rsp+B0h] [rbp-50h]
  int v126; // [rsp+B4h] [rbp-4Ch]
  _QWORD *v127; // [rsp+E0h] [rbp-20h]
  __int64 *v128; // [rsp+E8h] [rbp-18h] BYREF
  __int64 v129; // [rsp+F0h] [rbp-10h]
  __int64 *v130; // [rsp+F8h] [rbp-8h] BYREF
  __int64 v131; // [rsp+100h] [rbp+0h]
  __int64 v132; // [rsp+108h] [rbp+8h]
  _BYTE v133[16]; // [rsp+110h] [rbp+10h] BYREF
  char v134[16]; // [rsp+120h] [rbp+20h] BYREF
  char v135[16]; // [rsp+130h] [rbp+30h] BYREF
  char v136[16]; // [rsp+140h] [rbp+40h] BYREF
  char v137[16]; // [rsp+150h] [rbp+50h] BYREF
  char v138[16]; // [rsp+160h] [rbp+60h] BYREF
  char v139[120]; // [rsp+170h] [rbp+70h] BYREF
  _QWORD v140[2]; // [rsp+1E8h] [rbp+E8h] BYREF
  signed __int64 v141; // [rsp+1F8h] [rbp+F8h]
  unsigned __int64 v142; // [rsp+200h] [rbp+100h]
  _QWORD v143[2]; // [rsp+208h] [rbp+108h] BYREF
  __int64 v144; // [rsp+218h] [rbp+118h]
  unsigned __int64 v145; // [rsp+220h] [rbp+120h]
  _QWORD v146[2]; // [rsp+228h] [rbp+128h] BYREF
  __int64 v147; // [rsp+238h] [rbp+138h]
  unsigned __int64 v148; // [rsp+240h] [rbp+140h]

  v132 = -2;
  v3 = a1;
  v127 = a1;
  v140[0] = 0;
  v141 = 0;
  v142 = 7;
  sub_14014C8D0(v140, (void *)&Source);
  __wind
  {
    v146[0] = 0;
    v147 = 0;
    v148 = 7;
    sub_14014C8D0(v146, (void *)&Source);
    __wind
    {
      sub_14389E6E0(&v119);
      __wind
      {
        LOBYTE(v4) = 1;
        if ( (unsigned __int8)sub_1470A1A80(a2, v140, v4) == 0 )
          goto LABEL_229;
        v5 = (char *)NtCurrentTeb()->ThreadLocalStoragePointer + 8 * (unsigned int)TlsIndex;
        while ( 1 )
        {
          v6 = sub_146E8C7D0(&unk_149E4B778);
          v7 = -1;
          do
            ++v7;
          while ( *(_WORD *)(v6 + 2 * v7) != 0 );
          v8 = (unsigned __int16 *)v140;
          if ( v142 >= 8 )
            v8 = (unsigned __int16 *)v140[0];
          v9 = v141;
          if ( v141 != v7 )
            goto LABEL_15;
          if ( v141 != 0 )
            break;
LABEL_94:
          v13 = 1;
LABEL_16:
          if ( v13 != 0 )
          {
            v101 = sub_14389E4A0(v139, &v119, v9, v8);
            v104 = (int *)v101;
            *(_QWORD *)&v114 = v101;
            __wind
            {
              if ( *(_DWORD *)v101 == 52 )
                *(_BYTE *)(v101 + 112) = 1;
              if ( v3[9] == v3[10] )
              {
                sub_14389C0E0(v3 + 8, v3[9], v101);
              }
              else
              {
                sub_14389E4A0(v3[9], v101, v102, v103);
                v3[9] += 120LL;
              }
              v105 = (__int64 *)v3[11];
              v106 = (__int64 *)v105[1];
              v107 = v105;
              while ( *((_BYTE *)v106 + 25) == 0 )
              {
                if ( *((_DWORD *)v106 + 7) >= *v104 )
                {
                  v107 = v106;
                  v106 = (__int64 *)*v106;
                }
                else
                {
                  v106 = (__int64 *)v106[2];
                }
              }
              if ( *((_BYTE *)v107 + 25) != 0 || *v104 < *((_DWORD *)v107 + 7) || v107 == v105 )
              {
                v117 = *v104;
                v118 = 1;
                sub_1413249D0(v3 + 11, v133, &v117);
              }
            }
            __unwind
            {
              sub_14389FB90(v114);
            }
            __wind
            {
              __wind
              {
                sub_1438A0B40(v104 + 14);
              }
              __unwind
              {
                sub_1435A1920(v114 + 32);
              }
              sub_1435A2FE0(v104 + 8);
            }
            __unwind
            {
              sub_140156720((void *)(v114 + 8));
            }
            sub_1401574A0(v104 + 2);
LABEL_229:
            v63 = 1;
            goto LABEL_303;
          }
          v14 = sub_146E8C7D0(&unk_149F61060);
          v15 = -1;
          do
            ++v15;
          while ( *(_WORD *)(v14 + 2 * v15) != 0 );
          v16 = (unsigned __int16 *)v140;
          if ( v142 >= 8 )
            v16 = (unsigned __int16 *)v140[0];
          v17 = v141;
          if ( v141 == v15 )
          {
            if ( v141 == 0 )
            {
LABEL_95:
              v119 = 63;
              LOBYTE(v17) = 1;
              if ( (unsigned __int8)sub_1470A09C0(a2, v146, v17, v16) == 0
                || (unsigned int)(v119 = sub_1438B5A20(v3, v146)) > 0x3E )
              {
LABEL_97:
                v63 = 0;
                goto LABEL_303;
              }
              goto LABEL_210;
            }
            v18 = *v16;
            if ( *v16 >= *(_WORD *)v14 )
            {
              v16 = (unsigned __int16 *)((char *)v16 - v14);
              v19 = v18 <= *(_WORD *)v14;
              do
              {
                if ( !v19 )
                  break;
                if ( v17 == 1 )
                  goto LABEL_95;
                --v17;
                v14 += 2;
                v20 = *(unsigned __int16 *)((char *)v16 + v14);
                v19 = v20 <= *(_WORD *)v14;
              }
              while ( v20 >= *(_WORD *)v14 );
            }
          }
          v21 = sub_146E8C7D0(&unk_149684168);
          v22 = -1;
          do
            ++v22;
          while ( *(_WORD *)(v21 + 2 * v22) != 0 );
          v23 = (unsigned __int16 *)v140;
          if ( v142 >= 8 )
            v23 = (unsigned __int16 *)v140[0];
          v24 = v141;
          if ( v141 == v22 )
          {
            if ( v141 == 0 )
            {
LABEL_98:
              if ( (unsigned __int8)sub_1438AFBF0(v3, a2, &v119, v23) == 0 )
                goto LABEL_97;
              goto LABEL_210;
            }
            v25 = *v23;
            if ( *v23 >= *(_WORD *)v21 )
            {
              v23 = (unsigned __int16 *)((char *)v23 - v21);
              v26 = v25 <= *(_WORD *)v21;
              do
              {
                if ( !v26 )
                  break;
                if ( v24 == 1 )
                  goto LABEL_98;
                --v24;
                v21 += 2;
                v27 = *(unsigned __int16 *)((char *)v23 + v21);
                v26 = v27 <= *(_WORD *)v21;
              }
              while ( v27 >= *(_WORD *)v21 );
            }
          }
          v28 = sub_146E8C7D0(&unk_149F61088);
          v29 = -1;
          do
            ++v29;
          while ( *(_WORD *)(v28 + 2 * v29) != 0 );
          v30 = (unsigned __int16 *)v140;
          if ( v142 >= 8 )
            v30 = (unsigned __int16 *)v140[0];
          v31 = v141;
          if ( v141 == v29 )
          {
            if ( v141 == 0 )
            {
LABEL_100:
              v113[0] = 0;
              LOBYTE(v31) = 1;
              if ( (unsigned __int8)sub_14709DC20(a2, &v117, v31, v30) == 0 )
                goto LABEL_97;
              LOBYTE(v64) = 1;
              if ( (unsigned __int8)sub_14709DC20(a2, v113, v64, v65) == 0 )
                goto LABEL_97;
              v125 = v117;
              v126 = v113[0];
              goto LABEL_210;
            }
            v32 = *v30;
            if ( *v30 >= *(_WORD *)v28 )
            {
              v30 = (unsigned __int16 *)((char *)v30 - v28);
              v33 = v32 <= *(_WORD *)v28;
              do
              {
                if ( !v33 )
                  break;
                if ( v31 == 1 )
                  goto LABEL_100;
                --v31;
                v28 += 2;
                v34 = *(unsigned __int16 *)((char *)v30 + v28);
                v33 = v34 <= *(_WORD *)v28;
              }
              while ( v34 >= *(_WORD *)v28 );
            }
          }
          v35 = sub_146E8C7D0(&unk_149F610B0);
          v36 = -1;
          do
            ++v36;
          while ( *(_WORD *)(v35 + 2 * v36) != 0 );
          v37 = (unsigned __int16 *)v140;
          if ( v142 >= 8 )
            v37 = (unsigned __int16 *)v140[0];
          v38 = v141;
          if ( v141 == v36 )
          {
            if ( v141 == 0 )
            {
LABEL_103:
              if ( (unsigned __int8)sub_1438AD3A0(v3, a2, &v119, v37) == 0 )
                goto LABEL_97;
              goto LABEL_210;
            }
            v39 = *v37;
            if ( *v37 >= *(_WORD *)v35 )
            {
              v37 = (unsigned __int16 *)((char *)v37 - v35);
              v40 = v39 <= *(_WORD *)v35;
              do
              {
                if ( !v40 )
                  break;
                if ( v38 == 1 )
                  goto LABEL_103;
                --v38;
                v35 += 2;
                v41 = *(unsigned __int16 *)((char *)v37 + v35);
                v40 = v41 <= *(_WORD *)v35;
              }
              while ( v41 >= *(_WORD *)v35 );
            }
          }
          v42 = (char *)sub_146E8C7D0(&unk_149931448);
          v43 = -1;
          do
            ++v43;
          while ( *(_WORD *)&v42[2 * v43] != 0 );
          v44 = (char *)v140;
          if ( v142 >= 8 )
            v44 = (char *)v140[0];
          v45 = v141;
          if ( v141 != v43 )
            goto LABEL_72;
          if ( v141 != 0 )
          {
            v46 = *(_WORD *)v44;
            if ( *(_WORD *)v44 >= *(_WORD *)v42 )
            {
              v47 = v44 - v42;
              v48 = v46 <= *(_WORD *)v42;
              do
              {
                if ( !v48 )
                  break;
                if ( v45 == 1 )
                  goto LABEL_105;
                --v45;
                v42 += 2;
                v49 = *(_WORD *)&v42[v47];
                v48 = v49 <= *(_WORD *)v42;
              }
              while ( v49 >= *(_WORD *)v42 );
            }
LABEL_72:
            v50 = 0;
            goto LABEL_73;
          }
LABEL_105:
          v50 = 1;
LABEL_73:
          if ( v50 != 0 )
          {
            v143[0] = 0;
            v144 = 0;
            v145 = 7;
            sub_14014C8D0(v143, (void *)&Source);
            __wind
            {
              v121 = v120;
              v53 = 0;
              LOBYTE(v51) = 1;
              if ( (unsigned __int8)sub_1470A09C0(a2, v143, v51, v52) != 0 )
              {
                v55 = *(_QWORD *)v5;
                while ( 1 )
                {
LABEL_77:
                  if ( dword_14E65D168 > *(_DWORD *)(v55 + 420620) )
                  {
                    Init_thread_header(&dword_14E65D168, v54, v45);
                    if ( dword_14E65D168 == -1 )
                    {
                      __wind
                      {
                        qword_14E65D158 = 0;
                        qword_14E65D160 = 0;
                        v56 = sub_146E8BA20(48);
                        *(_QWORD *)v56 = v56;
                        *(_QWORD *)(v56 + 8) = v56;
                        *(_QWORD *)(v56 + 16) = v56;
                        *(_WORD *)(v56 + 24) = 257;
                        qword_14E65D158 = v56;
                        atexit(sub_149021260);
                      }
                      __unwind
                      {
                        Init_thread_abort(&dword_14E65D168);
                      }
                      Init_thread_footer(&dword_14E65D168);
                    }
                  }
                  if ( dword_14E65D174 > *(_DWORD *)(*(_QWORD *)v5 + 420620LL) )
                  {
                    Init_thread_header(&dword_14E65D174, v54, v45);
                    if ( dword_14E65D174 == -1 )
                    {
                      __wind
                      {
                        __crt_strtox::big_integer::big_integer((__crt_strtox::big_integer *)&unk_14E65D170);
                        atexit(sub_149021090);
                      }
                      __unwind
                      {
                        Init_thread_abort(&dword_14E65D174);
                      }
                      Init_thread_footer(&dword_14E65D174);
                    }
                  }
                  if ( byte_14E65D16C == 0 )
                    break;
                  v57 = (char *)v143;
                  if ( v145 >= 8 )
                    v57 = (char *)v143[0];
                  v58 = *(_QWORD *)(qword_14E65D158 + 8);
                  v59 = qword_14E65D158;
                  while ( *(_BYTE *)(v58 + 25) == 0 )
                  {
                    v60 = *(char **)(v58 + 32);
                    v45 = v57 - v60;
                    do
                    {
                      v61 = *(unsigned __int16 *)&v60[v45];
                      v62 = *(unsigned __int16 *)v60 - v61;
                      if ( v62 != 0 )
                        break;
                      v60 += 2;
                    }
                    while ( v61 != 0 );
                    if ( v62 >= 0 )
                    {
                      v59 = v58;
                      v58 = *(_QWORD *)v58;
                    }
                    else
                    {
                      v58 = *(_QWORD *)(v58 + 16);
                    }
                  }
                  if ( *(_BYTE *)(v59 + 25) != 0 )
                    goto LABEL_201;
                  v66 = *(_QWORD *)(v59 + 32) - (_QWORD)v57;
                  do
                  {
                    v67 = *(unsigned __int16 *)&v57[v66];
                    v68 = *(unsigned __int16 *)v57 - v67;
                    if ( v68 != 0 )
                      break;
                    v57 += 2;
                  }
                  while ( v67 != 0 );
                  if ( v68 < 0 || v59 == qword_14E65D158 )
                    goto LABEL_201;
                  v69 = *(_DWORD *)(v59 + 40);
                  if ( v69 <= 3137 )
                  {
                    if ( v69 == 3137 )
                      goto LABEL_166;
                    if ( v69 != 0 )
                    {
                      v70 = v69 - 3117;
                      if ( v70 != 0 )
                      {
                        v71 = v70 - 8;
                        if ( v71 != 0 )
                        {
                          if ( v71 != 6 )
                            goto LABEL_201;
                          goto LABEL_120;
                        }
LABEL_148:
                        if ( byte_14E65D16C != 0 )
                        {
                          if ( v53 == 0 )
                          {
                            v113[0] = 0;
                            v96 = v121;
                            if ( v121 == v122 )
                            {
LABEL_195:
                              sub_140B5E2E0(&v120, v96, v113);
                              goto LABEL_201;
                            }
                            *v121++ = 0;
                          }
                          goto LABEL_201;
                        }
                        v86 = sub_146E8C7D0(&unk_149F610D0);
                        v87 = (char *)v86;
                        v88 = qword_14E65D158;
                        v89 = *(_QWORD *)(qword_14E65D158 + 8);
                        *(_QWORD *)&v115 = v89;
                        DWORD2(v115) = 0;
                        v90 = qword_14E65D158;
                        v116 = qword_14E65D158;
                        if ( *(_BYTE *)(v89 + 25) == 0 )
                        {
                          do
                          {
                            *(_QWORD *)&v115 = v89;
                            v91 = *(char **)(v89 + 32);
                            v58 = v87 - v91;
                            do
                            {
                              v76 = *(unsigned __int16 *)&v91[v58];
                              v92 = *(unsigned __int16 *)v91 - (_DWORD)v76;
                              if ( v92 != 0 )
                                break;
                              v91 += 2;
                            }
                            while ( (_DWORD)v76 != 0 );
                            if ( v92 >= 0 )
                            {
                              DWORD2(v115) = 1;
                              v90 = v89;
                              v89 = *(_QWORD *)v89;
                            }
                            else
                            {
                              DWORD2(v115) = 0;
                              v89 = *(_QWORD *)(v89 + 16);
                            }
                          }
                          while ( *(_BYTE *)(v89 + 25) == 0 );
                          v116 = v90;
                        }
                        v114 = v115;
                        if ( *(_BYTE *)(v90 + 25) != 0 )
                          goto LABEL_163;
                        v93 = (unsigned __int16 *)v87;
                        v45 = *(_QWORD *)(v90 + 32) - (_QWORD)v87;
                        do
                        {
                          v76 = *(unsigned __int16 *)((char *)v93 + v45);
                          v94 = *v93 - (_DWORD)v76;
                          if ( v94 != 0 )
                            break;
                          ++v93;
                        }
                        while ( (_DWORD)v76 != 0 );
                        if ( v94 < 0 )
                        {
LABEL_163:
                          if ( qword_14E65D160 == 0x555555555555555LL )
LABEL_243:
                            unknown_libname_7(v76);
                          v130 = &qword_14E65D158;
                          v131 = 0;
                          __wind
                          {
                            v131 = 0;
                            v95 = sub_14014CAE0(&qword_14E65D158, 1);
                            v131 = v95;
                          }
                          __unwind
                          {
                            sub_14014EE00(&v130);
                          }
                          __wind
                          {
                            *(_QWORD *)(v95 + 32) = v87;
                            *(_DWORD *)(v95 + 40) = 3125;
                            *(_QWORD *)v95 = v88;
                            *(_QWORD *)(v95 + 8) = v88;
                            *(_QWORD *)(v95 + 16) = v88;
                            *(_WORD *)(v95 + 24) = 0;
                          }
                          __unwind
                          {
                            sub_14014EEA0(&v130);
                          }
                          __wind
                          {
                            v131 = 0;
                          }
                          __unwind
                          {
                            sub_14014EE70(&v130);
                          }
                          sub_14014F0E0(&qword_14E65D158, &v114, v95);
                        }
LABEL_120:
                        if ( byte_14E65D16C != 0 )
                        {
                          if ( v53 == 0 )
                          {
                            v113[0] = 1;
                            v96 = v121;
                            if ( v121 == v122 )
                              goto LABEL_195;
                            *v121++ = 1;
                          }
                          goto LABEL_201;
                        }
                        v113[0] = 3131;
                        *(_QWORD *)&v114 = sub_146E8C7D0(&unk_149F610E8);
                        sub_140166070(&qword_14E65D158, v134, &v114, v113);
LABEL_166:
                        if ( byte_14E65D16C != 0 )
                        {
                          if ( v53 == 0 )
                          {
                            v113[0] = 2;
                            v96 = v121;
                            if ( v121 == v122 )
                              goto LABEL_195;
                            *v121++ = 2;
                          }
                          goto LABEL_201;
                        }
                        v113[0] = 3137;
                        *(_QWORD *)&v114 = sub_146E8C7D0(&unk_149F61118);
                        sub_140166070(&qword_14E65D158, v135, &v114, v113);
LABEL_168:
                        if ( byte_14E65D16C != 0 )
                        {
                          if ( v53 == 0 )
                          {
                            v113[0] = 3;
                            v96 = v121;
                            if ( v121 == v122 )
                              goto LABEL_195;
                            *v121++ = 3;
                          }
                          goto LABEL_201;
                        }
                        v113[0] = 3143;
                        *(_QWORD *)&v114 = sub_146E8C7D0(&unk_149F61148);
                        sub_140166070(&qword_14E65D158, v136, &v114, v113);
LABEL_170:
                        if ( byte_14E65D16C != 0 )
                        {
                          if ( v53 == 0 )
                          {
                            v113[0] = 4;
                            v96 = v121;
                            if ( v121 == v122 )
                              goto LABEL_195;
                            *v121++ = 4;
                          }
                          goto LABEL_201;
                        }
                        v113[0] = 3149;
                        *(_QWORD *)&v114 = sub_146E8C7D0(&unk_149F61170);
                        sub_140166070(&qword_14E65D158, v137, &v114, v113);
                        goto LABEL_172;
                      }
                    }
                    if ( byte_14E65D16C != 0 )
                    {
                      v53 = 1;
                      v97 = v120;
                      v121 = v120;
                      for ( i = 0; i < 7; ++i )
                      {
                        v113[0] = i;
                        if ( v97 == v122 )
                        {
                          sub_140B5E2E0(&v120, v97, v113);
                          v97 = v121;
                        }
                        else
                        {
                          *v97 = i;
                          v97 = ++v121;
                        }
                      }
                      goto LABEL_201;
                    }
LABEL_131:
                    v75 = sub_146E8C7D0(&unk_1491BE780);
                    v77 = (char *)v75;
                    v78 = qword_14E65D158;
                    v79 = *(_QWORD *)(qword_14E65D158 + 8);
                    *(_QWORD *)&v115 = v79;
                    DWORD2(v115) = 0;
                    v80 = qword_14E65D158;
                    v116 = qword_14E65D158;
                    if ( *(_BYTE *)(v79 + 25) == 0 )
                    {
                      do
                      {
                        *(_QWORD *)&v115 = v79;
                        v81 = *(char **)(v79 + 32);
                        v58 = v77 - v81;
                        do
                        {
                          v76 = *(unsigned __int16 *)&v81[v58];
                          v82 = *(unsigned __int16 *)v81 - (_DWORD)v76;
                          if ( v82 != 0 )
                            break;
                          v81 += 2;
                        }
                        while ( (_DWORD)v76 != 0 );
                        if ( v82 >= 0 )
                        {
                          DWORD2(v115) = 1;
                          v80 = v79;
                          v79 = *(_QWORD *)v79;
                        }
                        else
                        {
                          DWORD2(v115) = 0;
                          v79 = *(_QWORD *)(v79 + 16);
                        }
                      }
                      while ( *(_BYTE *)(v79 + 25) == 0 );
                      v116 = v80;
                    }
                    v114 = v115;
                    if ( *(_BYTE *)(v80 + 25) != 0 )
                      goto LABEL_145;
                    v83 = (unsigned __int16 *)v77;
                    v45 = *(_QWORD *)(v80 + 32) - (_QWORD)v77;
                    do
                    {
                      v76 = *(unsigned __int16 *)((char *)v83 + v45);
                      v84 = *v83 - (_DWORD)v76;
                      if ( v84 != 0 )
                        break;
                      ++v83;
                    }
                    while ( (_DWORD)v76 != 0 );
                    if ( v84 < 0 )
                    {
LABEL_145:
                      if ( qword_14E65D160 == 0x555555555555555LL )
                        goto LABEL_243;
                      v128 = &qword_14E65D158;
                      v129 = 0;
                      __wind
                      {
                        v129 = 0;
                        v85 = sub_146E8BA20(48);
                        v129 = v85;
                      }
                      __unwind
                      {
                        sub_14014EE00(&v128);
                      }
                      __wind
                      {
                        *(_QWORD *)(v85 + 32) = v77;
                        *(_DWORD *)(v85 + 40) = 3117;
                        *(_QWORD *)v85 = v78;
                        *(_QWORD *)(v85 + 8) = v78;
                        *(_QWORD *)(v85 + 16) = v78;
                        *(_WORD *)(v85 + 24) = 0;
                      }
                      __unwind
                      {
                        sub_14014EEA0(&v128);
                      }
                      __wind
                      {
                        v129 = 0;
                      }
                      __unwind
                      {
                        sub_14014EE70(&v128);
                      }
                      sub_14014F0E0(&qword_14E65D158, &v114, v85);
                    }
                    goto LABEL_148;
                  }
                  v72 = v69 - 3143;
                  if ( v72 == 0 )
                    goto LABEL_168;
                  v73 = v72 - 6;
                  if ( v73 == 0 )
                    goto LABEL_170;
                  v74 = v73 - 6;
                  if ( v74 != 0 )
                  {
                    if ( v74 != 6 )
                      goto LABEL_201;
                    goto LABEL_126;
                  }
LABEL_172:
                  if ( byte_14E65D16C == 0 )
                  {
                    v113[0] = 3155;
                    *(_QWORD *)&v114 = sub_146E8C7D0(&unk_149F61198);
                    sub_140166070(&qword_14E65D158, v138, &v114, v113);
LABEL_126:
                    if ( byte_14E65D16C != 0 )
                    {
                      if ( v53 == 0 )
                      {
                        v113[0] = 6;
                        v96 = v121;
                        if ( v121 == v122 )
                          goto LABEL_195;
                        *v121++ = 6;
                      }
                    }
                    else
                    {
                      v113[0] = 3161;
                      *(_QWORD *)&v114 = sub_146E8C7D0(&unk_149F611C0);
                      sub_140166070(&qword_14E65D158, v133, &v114, v113);
                    }
                    goto LABEL_201;
                  }
                  if ( v53 == 0 )
                  {
                    v113[0] = 5;
                    v96 = v121;
                    if ( v121 == v122 )
                      goto LABEL_195;
                    *v121++ = 5;
                  }
LABEL_201:
                  if ( byte_14E65D16C != 0 )
                  {
                    LOBYTE(v45) = 1;
                    if ( (unsigned __int8)sub_1470A09C0(a2, v143, v45, v58) == 0 )
                      goto LABEL_265;
                  }
                  else
                  {
                    Atomic_lock_release(&unk_14E65D170);
                    byte_14E65D16C = 1;
                  }
                }
                sub_146E8BE40(&unk_14E65D170);
                if ( byte_14E65D16C != 0 )
                {
                  Atomic_lock_release(&unk_14E65D170);
                  goto LABEL_77;
                }
                goto LABEL_131;
              }
            }
            __unwind
            {
              unknown_libname_4(v143);
            }
LABEL_265:
            if ( v145 >= 8 )
            {
              v99 = 2 * v145 + 2;
              v100 = v143[0];
              if ( v99 >= 0x1000 )
              {
                v99 = 2 * v145 + 41;
                v100 = *(_QWORD *)(v143[0] - 8LL);
                if ( (unsigned __int64)(v143[0] - v100 - 8) > 0x1F )
                  invalid_parameter_noinfo_noreturn();
              }
              j_j_scalable_free(v100, v99);
            }
            v144 = 0;
            v145 = 7;
            LOWORD(v143[0]) = 0;
            v3 = v127;
          }
LABEL_210:
          LOBYTE(v45) = 1;
          if ( (unsigned __int8)sub_1470A1A80(a2, v140, v45) == 0 )
            goto LABEL_229;
        }
        v10 = *v8;
        if ( *v8 >= *(_WORD *)v6 )
        {
          v8 = (unsigned __int16 *)((char *)v8 - v6);
          v11 = v10 <= *(_WORD *)v6;
          do
          {
            if ( !v11 )
              break;
            if ( v9 == 1 )
              goto LABEL_94;
            --v9;
            v6 += 2;
            v12 = *(unsigned __int16 *)((char *)v8 + v6);
            v11 = v12 <= *(_WORD *)v6;
          }
          while ( v12 >= *(_WORD *)v6 );
        }
LABEL_15:
        v13 = 0;
        goto LABEL_16;
      }
      __unwind
      {
        sub_14389FB90(&v119);
      }
LABEL_303:
      __wind
      {
        __wind
        {
          sub_1438A0B40(v124);
        }
        __unwind
        {
          sub_1435A1920(v123);
        }
        sub_1435A2FE0(v123);
      }
      __unwind
      {
        sub_140156720(&v120);
      }
      sub_1401574A0(&v120);
    }
    __unwind
    {
      unknown_libname_4(v146);
    }
    if ( v148 >= 8 )
    {
      v108 = 2 * v148 + 2;
      v109 = v146[0];
      if ( v108 >= 0x1000 )
      {
        v108 = 2 * v148 + 41;
        v109 = *(_QWORD *)(v146[0] - 8LL);
        if ( (unsigned __int64)(v146[0] - v109 - 8) > 0x1F )
          invalid_parameter_noinfo_noreturn();
      }
      j_j_scalable_free(v109, v108);
    }
    v147 = 0;
    v148 = 7;
    LOWORD(v146[0]) = 0;
  }
  __unwind
  {
    unknown_libname_4(v140);
  }
  if ( v142 >= 8 )
  {
    v110 = 2 * v142 + 2;
    v111 = v140[0];
    if ( v110 >= 0x1000 )
    {
      v110 = 2 * v142 + 41;
      v111 = *(_QWORD *)(v140[0] - 8LL);
      if ( (unsigned __int64)(v140[0] - v111 - 8) > 0x1F )
        invalid_parameter_noinfo_noreturn();
    }
    j_j_scalable_free(v111, v110);
  }
  v141 = 0;
  v142 = 7;
  LOWORD(v140[0]) = 0;
  return v63;
}
