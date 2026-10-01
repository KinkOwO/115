// sub_144452020  va=0x144452020  size=4128

__int64 __fastcall sub_144452020(__int64 a1, __int64 a2)
{
  __int64 v4; // r8
  _QWORD *v5; // r15
  __int64 v6; // rax
  __int64 v7; // rcx
  unsigned __int16 *v8; // r9
  unsigned __int64 v9; // r8
  unsigned __int16 v10; // cx
  bool v11; // cc
  unsigned __int16 v12; // cx
  __int64 v13; // rax
  __int64 v14; // rcx
  unsigned __int16 *v15; // r9
  unsigned __int64 v16; // r8
  unsigned __int16 v17; // cx
  bool v18; // cc
  unsigned __int16 v19; // cx
  char *v20; // rax
  __int64 v21; // rcx
  char *v22; // r9
  unsigned __int64 v23; // r8
  unsigned __int16 v24; // cx
  signed __int64 v25; // r9
  bool v26; // cc
  unsigned __int16 v27; // cx
  char v28; // al
  __int64 v29; // r8
  char *v30; // rax
  __int64 v31; // rcx
  char *v32; // r9
  __int64 v33; // r8
  unsigned __int16 v34; // cx
  signed __int64 v35; // r9
  bool v36; // cc
  unsigned __int16 v37; // cx
  __int64 v38; // rax
  __int64 v39; // rcx
  unsigned __int16 *v40; // r9
  __int64 v41; // r8
  unsigned __int16 v42; // cx
  bool v43; // cc
  unsigned __int16 v44; // cx
  __int64 v45; // rax
  __int64 v46; // rcx
  unsigned __int16 *v47; // r9
  __int64 v48; // r8
  unsigned __int16 v49; // cx
  bool v50; // cc
  unsigned __int16 v51; // cx
  __int64 v52; // rax
  __int64 v53; // rcx
  unsigned __int16 *v54; // r9
  __int64 v55; // r8
  unsigned __int16 v56; // cx
  bool v57; // cc
  unsigned __int16 v58; // cx
  unsigned __int64 v59; // rsi
  void **v60; // rdi
  __int64 v61; // rbx
  __int64 v62; // r8
  __int64 v63; // r9
  unsigned __int64 v64; // rdx
  __int64 v65; // rcx
  unsigned __int8 v66; // bl
  int v67; // eax
  __int64 v68; // r8
  __int64 v69; // r9
  int *v70; // rdx
  int i; // ebx
  __int64 v72; // rdx
  unsigned __int16 *v73; // r9
  __int64 v74; // rax
  __int64 v75; // rcx
  unsigned __int16 v76; // cx
  bool v77; // cc
  unsigned __int16 v78; // cx
  _DWORD *v79; // rdi
  __int64 v80; // rax
  char *v81; // r10
  __int64 v82; // rax
  __int64 v83; // r11
  char *v84; // rcx
  int v85; // edx
  int v86; // r8d
  __int64 v87; // rdx
  int v88; // eax
  int v89; // ecx
  int v90; // eax
  int v91; // eax
  int v92; // eax
  int v93; // eax
  int v94; // eax
  int v95; // eax
  void *v96; // rdx
  int *v97; // rdx
  int j; // ebx
  __int64 v100; // r15
  __int64 v101; // rsi
  const void *v102; // r12
  _BYTE *v103; // r14
  signed __int64 v104; // rbx
  __int64 v105; // rax
  __int64 v106; // rdi
  char *v107; // rbx
  void **v108; // r14
  void *v109; // r13
  char *v110; // r12
  __int64 v111; // rbx
  __int64 v112; // rax
  __int64 v113; // rdi
  char *v114; // rbx
  int v115; // [rsp+20h] [rbp-E0h] BYREF
  _BYTE v116[4]; // [rsp+24h] [rbp-DCh] BYREF
  __int64 v117; // [rsp+28h] [rbp-D8h] BYREF
  void *v118[2]; // [rsp+30h] [rbp-D0h] BYREF
  int *v119; // [rsp+40h] [rbp-C0h]
  void *v120; // [rsp+48h] [rbp-B8h] BYREF
  int *v121; // [rsp+50h] [rbp-B0h]
  int *v122; // [rsp+58h] [rbp-A8h]
  __int64 v123; // [rsp+60h] [rbp-A0h]
  int v124; // [rsp+68h] [rbp-98h]
  __int64 v125; // [rsp+70h] [rbp-90h] BYREF
  __int128 v126; // [rsp+78h] [rbp-88h] BYREF
  unsigned __int64 v127; // [rsp+88h] [rbp-78h]
  __int64 v128; // [rsp+90h] [rbp-70h]
  char *v129; // [rsp+98h] [rbp-68h]
  __int64 v130; // [rsp+A0h] [rbp-60h]
  char v131[16]; // [rsp+A8h] [rbp-58h] BYREF
  char v132[16]; // [rsp+B8h] [rbp-48h] BYREF
  char v133[16]; // [rsp+C8h] [rbp-38h] BYREF
  char v134[16]; // [rsp+D8h] [rbp-28h] BYREF
  char v135[16]; // [rsp+E8h] [rbp-18h] BYREF
  char v136[16]; // [rsp+F8h] [rbp-8h] BYREF
  char v137[16]; // [rsp+108h] [rbp+8h] BYREF
  char v138[16]; // [rsp+118h] [rbp+18h] BYREF
  _QWORD v139[2]; // [rsp+128h] [rbp+28h] BYREF
  __int64 v140; // [rsp+138h] [rbp+38h]
  unsigned __int64 v141; // [rsp+140h] [rbp+40h]
  _QWORD v142[2]; // [rsp+148h] [rbp+48h] BYREF
  unsigned __int64 v143; // [rsp+158h] [rbp+58h]
  unsigned __int64 v144; // [rsp+160h] [rbp+60h]
  _QWORD v145[2]; // [rsp+168h] [rbp+68h] BYREF
  __int64 v146; // [rsp+178h] [rbp+78h]
  unsigned __int64 v147; // [rsp+180h] [rbp+80h]
  void *Src[2]; // [rsp+188h] [rbp+88h] BYREF
  unsigned __int64 v149; // [rsp+198h] [rbp+98h]
  unsigned __int64 v150; // [rsp+1A0h] [rbp+A0h]

  v130 = -2;
  v142[0] = 0;
  v143 = 0;
  v144 = 7;
  sub_14014C8D0(v142, (void *)&Source);
  __wind
  {
    Src[0] = nullptr;
    v149 = 0;
    v150 = 7;
    sub_14014C8D0(Src, (void *)&Source);
    __wind
    {
      v145[0] = 0;
      v146 = 0;
      v147 = 7;
      sub_14014C8D0(v145, (void *)&Source);
      __wind
      {
        v116[0] = 0;
        *(_OWORD *)v118 = 0;
        v119 = nullptr;
        __wind
        {
          v120 = nullptr;
          v121 = nullptr;
          v122 = nullptr;
          __wind
          {
            v118[1] = v118[0];
            v121 = (int *)v120;
            v123 = 0;
            v124 = 59;
          }
          __unwind
          {
            sub_1401574A0(&v120);
          }
        }
        __unwind
        {
          sub_140156720(v118);
        }
        __wind
        {
          v118[1] = v118[0];
          v121 = (int *)v120;
          v123 = 0;
          v124 = 59;
          LOBYTE(v4) = 1;
          if ( (unsigned __int8)sub_1470A1A80(a2, v142, v4) == 0 )
            goto LABEL_218;
          v5 = (char *)NtCurrentTeb()->ThreadLocalStoragePointer + 8 * (unsigned int)TlsIndex;
          while ( 1 )
          {
            v6 = sub_146E8C7D0(&unk_14A2EF598);
            v7 = -1;
            do
              ++v7;
            while ( *(_WORD *)(v6 + 2 * v7) != 0 );
            v8 = (unsigned __int16 *)v142;
            if ( v144 >= 8 )
              v8 = (unsigned __int16 *)v142[0];
            v9 = v143;
            if ( v143 == v7 )
            {
              if ( v143 == 0 )
                break;
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
                    goto LABEL_232;
                  --v9;
                  v6 += 2;
                  v12 = *(unsigned __int16 *)((char *)v8 + v6);
                  v11 = v12 <= *(_WORD *)v6;
                }
                while ( v12 >= *(_WORD *)v6 );
              }
            }
            v13 = sub_146E8C7D0(&unk_14A2EF5D0);
            v14 = -1;
            do
              ++v14;
            while ( *(_WORD *)(v13 + 2 * v14) != 0 );
            v15 = (unsigned __int16 *)v142;
            if ( v144 >= 8 )
              v15 = (unsigned __int16 *)v142[0];
            v16 = v143;
            if ( v143 != v14 )
              goto LABEL_26;
            if ( v143 != 0 )
            {
              v17 = *v15;
              if ( *v15 >= *(_WORD *)v13 )
              {
                v15 = (unsigned __int16 *)((char *)v15 - v13);
                v18 = v17 <= *(_WORD *)v13;
                do
                {
                  if ( !v18 )
                    break;
                  if ( v16 == 1 )
                    goto LABEL_85;
                  --v16;
                  v13 += 2;
                  v19 = *(unsigned __int16 *)((char *)v15 + v13);
                  v18 = v19 <= *(_WORD *)v13;
                }
                while ( v19 >= *(_WORD *)v13 );
              }
LABEL_26:
              v20 = (char *)sub_146E8C7D0(&unk_149684168);
              v21 = -1;
              do
                ++v21;
              while ( *(_WORD *)&v20[2 * v21] != 0 );
              v22 = (char *)v142;
              if ( v144 >= 8 )
                v22 = (char *)v142[0];
              v23 = v143;
              if ( v143 == v21 )
              {
                if ( v143 != 0 )
                {
                  v24 = *(_WORD *)v22;
                  if ( *(_WORD *)v22 >= *(_WORD *)v20 )
                  {
                    v25 = v22 - v20;
                    v26 = v24 <= *(_WORD *)v20;
                    do
                    {
                      if ( !v26 )
                        break;
                      if ( v23 == 1 )
                        goto LABEL_95;
                      --v23;
                      v20 += 2;
                      v27 = *(_WORD *)&v20[v25];
                      v26 = v27 <= *(_WORD *)v20;
                    }
                    while ( v27 >= *(_WORD *)v20 );
                  }
                  goto LABEL_37;
                }
LABEL_95:
                v28 = 1;
              }
              else
              {
LABEL_37:
                v28 = 0;
              }
              if ( v28 != 0 )
              {
                v139[0] = 0;
                v140 = 0;
                v141 = 7;
                sub_14014C8D0(v139, (void *)&Source);
                __eh34_enter_wind_state(5, 19);
                LOBYTE(v29) = 1;
                if ( (unsigned __int8)sub_1470A1A80(a2, v139, v29) != 0 )
                {
                  do
                  {
                    v30 = (char *)sub_146E8C7D0(&unk_149931650);
                    v31 = -1;
                    do
                      ++v31;
                    while ( *(_WORD *)&v30[2 * v31] != 0 );
                    v32 = (char *)v139;
                    if ( v141 >= 8 )
                      v32 = (char *)v139[0];
                    v33 = v140;
                    if ( v140 == v31 )
                    {
                      if ( v140 == 0 )
                        goto LABEL_264;
                      v34 = *(_WORD *)v32;
                      if ( *(_WORD *)v32 >= *(_WORD *)v30 )
                      {
                        v35 = v32 - v30;
                        v36 = v34 <= *(_WORD *)v30;
                        do
                        {
                          if ( !v36 )
                            break;
                          if ( v33 == 1 )
                            goto LABEL_264;
                          --v33;
                          v30 += 2;
                          v37 = *(_WORD *)&v30[v35];
                          v36 = v37 <= *(_WORD *)v30;
                        }
                        while ( v37 >= *(_WORD *)v30 );
                      }
                    }
                    v38 = sub_146E8C7D0(&unk_1492433B8);
                    v39 = -1;
                    do
                      ++v39;
                    while ( *(_WORD *)(v38 + 2 * v39) != 0 );
                    v40 = (unsigned __int16 *)v139;
                    if ( v141 >= 8 )
                      v40 = (unsigned __int16 *)v139[0];
                    v41 = v140;
                    if ( v140 == v39 )
                    {
                      if ( v140 == 0 )
                      {
LABEL_96:
                        LOBYTE(v41) = 1;
                        if ( (unsigned __int8)sub_14709DC20(a2, &v125, v41, v40) == 0
                          || (LOBYTE(v62) = 1, (unsigned __int8)sub_14709DC20(a2, (char *)&v125 + 4, v62, v63) == 0)
                          || (v123 = v125, (int)v125 > SHIDWORD(v125)) )
                        {
                          if ( __eh34_unwind(19) )
                          {
unwind_state_19:
                            unknown_libname_4(v139);
                            __eh34_continue_unwinding(19, 5);
                          }
                          __eh34_exit_wind_state(19, 5);
                          if ( v141 >= 8 )
                          {
                            v64 = 2 * v141 + 2;
                            v65 = v139[0];
                            if ( v64 >= 0x1000 )
                            {
                              v64 = 2 * v141 + 41;
                              v65 = *(_QWORD *)(v139[0] - 8LL);
                              if ( (unsigned __int64)(v139[0] - v65 - 8) > 0x1F )
                                invalid_parameter_noinfo_noreturn();
                            }
                            j_j_scalable_free(v65, v64);
                          }
                          v140 = 0;
                          v141 = 7;
                          LOWORD(v139[0]) = 0;
                          goto LABEL_105;
                        }
                        goto LABEL_212;
                      }
                      v42 = *v40;
                      if ( *v40 >= *(_WORD *)v38 )
                      {
                        v40 = (unsigned __int16 *)((char *)v40 - v38);
                        v43 = v42 <= *(_WORD *)v38;
                        do
                        {
                          if ( !v43 )
                            break;
                          if ( v41 == 1 )
                            goto LABEL_96;
                          --v41;
                          v38 += 2;
                          v44 = *(unsigned __int16 *)((char *)v40 + v38);
                          v43 = v44 <= *(_WORD *)v38;
                        }
                        while ( v44 >= *(_WORD *)v38 );
                      }
                    }
                    v45 = sub_146E8C7D0(&unk_14A2EF5F0);
                    v46 = -1;
                    do
                      ++v46;
                    while ( *(_WORD *)(v45 + 2 * v46) != 0 );
                    v47 = (unsigned __int16 *)v139;
                    if ( v141 >= 8 )
                      v47 = (unsigned __int16 *)v139[0];
                    v48 = v140;
                    if ( v140 == v46 )
                    {
                      if ( v140 == 0 )
                      {
LABEL_106:
                        v121 = (int *)v120;
                        LOBYTE(v48) = 1;
                        if ( (unsigned __int8)sub_1470A09C0(a2, Src, v48, v47) != 0 )
                        {
                          do
                          {
                            v67 = sub_14716C530(Src);
                            v70 = v121;
                            if ( v67 == 17 )
                            {
                              for ( i = 0; i < 17; ++i )
                              {
                                v115 = i;
                                if ( v70 == v122 )
                                {
                                  sub_1401C0850(&v120, v70, &v115);
                                  v70 = v121;
                                }
                                else
                                {
                                  *v70 = i;
                                  v70 = ++v121;
                                }
                              }
                            }
                            else
                            {
                              v115 = v67;
                              if ( v121 == v122 )
                                sub_1401C0850(&v120, v121, &v115);
                              else
                                *v121++ = v67;
                            }
                            LOBYTE(v68) = 1;
                          }
                          while ( (unsigned __int8)sub_1470A09C0(a2, Src, v68, v69) != 0 );
                        }
                        goto LABEL_212;
                      }
                      v49 = *v47;
                      if ( *v47 >= *(_WORD *)v45 )
                      {
                        v47 = (unsigned __int16 *)((char *)v47 - v45);
                        v50 = v49 <= *(_WORD *)v45;
                        do
                        {
                          if ( !v50 )
                            break;
                          if ( v48 == 1 )
                            goto LABEL_106;
                          --v48;
                          v45 += 2;
                          v51 = *(unsigned __int16 *)((char *)v47 + v45);
                          v50 = v51 <= *(_WORD *)v45;
                        }
                        while ( v51 >= *(_WORD *)v45 );
                      }
                    }
                    v52 = sub_146E8C7D0(&unk_14A2EF618);
                    v53 = -1;
                    do
                      ++v53;
                    while ( *(_WORD *)(v52 + 2 * v53) != 0 );
                    v54 = (unsigned __int16 *)v139;
                    if ( v141 >= 8 )
                      v54 = (unsigned __int16 *)v139[0];
                    v55 = v140;
                    if ( v140 == v53 )
                    {
                      if ( v140 != 0 )
                      {
                        v56 = *v54;
                        if ( *v54 >= *(_WORD *)v52 )
                        {
                          v54 = (unsigned __int16 *)((char *)v54 - v52);
                          v57 = v56 <= *(_WORD *)v52;
                          do
                          {
                            if ( !v57 )
                              break;
                            if ( v55 == 1 )
                              goto LABEL_119;
                            --v55;
                            v52 += 2;
                            v58 = *(unsigned __int16 *)((char *)v54 + v52);
                            v57 = v58 <= *(_WORD *)v52;
                          }
                          while ( v58 >= *(_WORD *)v52 );
                        }
                      }
                      else
                      {
LABEL_119:
                        v118[1] = v118[0];
                        LOBYTE(v54) = 1;
                        if ( (unsigned __int8)sub_1470A32B0(a2, v116, v145, v54) != 0 )
                        {
                          while ( v116[0] != 0 )
                          {
                            v74 = sub_146E8C7D0(&unk_14A2EF630);
                            v75 = -1;
                            do
                              ++v75;
                            while ( *(_WORD *)(v74 + 2 * v75) != 0 );
                            v73 = (unsigned __int16 *)v145;
                            if ( v147 >= 8 )
                              v73 = (unsigned __int16 *)v145[0];
                            v55 = v146;
                            if ( v146 == v75 )
                            {
                              if ( v146 == 0 )
                                goto LABEL_212;
                              v76 = *v73;
                              v72 = *(unsigned __int16 *)v74;
                              if ( *v73 >= (unsigned __int16)v72 )
                              {
                                v73 = (unsigned __int16 *)((char *)v73 - v74);
                                v77 = v76 <= (unsigned __int16)v72;
                                do
                                {
                                  if ( !v77 )
                                    break;
                                  if ( v55 == 1 )
                                    goto LABEL_212;
                                  --v55;
                                  v74 += 2;
                                  v78 = *(unsigned __int16 *)((char *)v73 + v74);
                                  v72 = *(unsigned __int16 *)v74;
                                  v77 = v78 <= (unsigned __int16)v72;
                                }
                                while ( v78 >= (unsigned __int16)v72 );
                              }
                            }
                            if ( v116[0] == 0 )
                              break;
LABEL_211:
                            LOBYTE(v73) = 1;
                            if ( (unsigned __int8)sub_1470A32B0(a2, v116, v145, v73) == 0 )
                              goto LABEL_212;
                          }
                          v79 = (_DWORD *)(*v5 + 420620LL);
                          while ( 2 )
                          {
                            while ( 2 )
                            {
                              if ( dword_14E6645E8 > *v79 )
                              {
                                Init_thread_header(&dword_14E6645E8, v72, v55);
                                if ( dword_14E6645E8 == -1 )
                                {
                                  __wind
                                  {
                                    qword_14E6645D8 = 0;
                                    qword_14E6645E0 = 0;
                                    v80 = sub_146E8BA20(48);
                                    *(_QWORD *)v80 = v80;
                                    *(_QWORD *)(v80 + 8) = v80;
                                    *(_QWORD *)(v80 + 16) = v80;
                                    *(_WORD *)(v80 + 24) = 257;
                                    qword_14E6645D8 = v80;
                                    atexit(sub_1490255F0);
                                  }
                                  __unwind
                                  {
                                    Init_thread_abort(&dword_14E6645E8);
                                  }
                                  Init_thread_footer(&dword_14E6645E8);
                                }
                              }
                              if ( dword_14E6645F4 > *v79 )
                              {
                                Init_thread_header(&dword_14E6645F4, v72, v55);
                                if ( dword_14E6645F4 == -1 )
                                {
                                  __wind
                                  {
                                    __crt_strtox::big_integer::big_integer((__crt_strtox::big_integer *)&unk_14E6645F0);
                                    atexit(sub_1490255E0);
                                  }
                                  __unwind
                                  {
                                    Init_thread_abort(&dword_14E6645F4);
                                  }
                                  Init_thread_footer(&dword_14E6645F4);
                                }
                              }
                              if ( byte_14E6645EC == 0 )
                              {
                                sub_146E8BE40(&unk_14E6645F0);
                                if ( byte_14E6645EC != 0 )
                                {
                                  Atomic_lock_release(&unk_14E6645F0);
                                  continue;
                                }
                                goto LABEL_174;
                              }
                              break;
                            }
                            v81 = (char *)v145;
                            if ( v147 >= 8 )
                              v81 = (char *)v145[0];
                            v82 = *(_QWORD *)(qword_14E6645D8 + 8);
                            v83 = qword_14E6645D8;
                            while ( *(_BYTE *)(v82 + 25) == 0 )
                            {
                              v84 = *(char **)(v82 + 32);
                              v73 = (unsigned __int16 *)(v81 - v84);
                              do
                              {
                                v85 = *(unsigned __int16 *)((char *)v73 + (_QWORD)v84);
                                v86 = *(unsigned __int16 *)v84 - v85;
                                if ( v86 != 0 )
                                  break;
                                v84 += 2;
                              }
                              while ( v85 != 0 );
                              if ( v86 >= 0 )
                              {
                                v83 = v82;
                                v82 = *(_QWORD *)v82;
                              }
                              else
                              {
                                v82 = *(_QWORD *)(v82 + 16);
                              }
                            }
                            if ( *(_BYTE *)(v83 + 25) != 0 )
                              goto LABEL_209;
                            v87 = *(_QWORD *)(v83 + 32) - (_QWORD)v81;
                            do
                            {
                              v88 = *(unsigned __int16 *)&v81[v87];
                              v89 = *(unsigned __int16 *)v81 - v88;
                              if ( v89 != 0 )
                                break;
                              v81 += 2;
                            }
                            while ( v88 != 0 );
                            if ( v89 < 0 || v83 == qword_14E6645D8 )
                              goto LABEL_209;
                            v90 = *(_DWORD *)(v83 + 40);
                            if ( v90 <= 1669 )
                            {
                              if ( v90 == 1669 )
                                goto LABEL_179;
                              if ( v90 != 0 )
                              {
                                v91 = v90 - 1652;
                                if ( v91 != 0 )
                                {
                                  v92 = v91 - 7;
                                  if ( v92 != 0 )
                                  {
                                    if ( v92 != 5 )
                                      goto LABEL_209;
LABEL_177:
                                    if ( byte_14E6645EC != 0 )
                                    {
                                      v115 = 1;
                                      v96 = v118[1];
                                      if ( v118[1] != v119 )
                                      {
                                        *(_DWORD *)v118[1] = 1;
                                        v118[1] = (char *)v118[1] + 4;
                                        goto LABEL_209;
                                      }
                                    }
                                    else
                                    {
                                      v115 = 1664;
                                      v117 = sub_146E8C7D0(&unk_149F610E8);
                                      sub_140166070(&qword_14E6645D8, v133, &v117, &v115);
LABEL_179:
                                      if ( byte_14E6645EC != 0 )
                                      {
                                        v115 = 2;
                                        v96 = v118[1];
                                        if ( v118[1] != v119 )
                                        {
                                          *(_DWORD *)v118[1] = 2;
                                          v118[1] = (char *)v118[1] + 4;
                                          goto LABEL_209;
                                        }
                                      }
                                      else
                                      {
                                        v115 = 1669;
                                        v117 = sub_146E8C7D0(&unk_149F61118);
                                        sub_140166070(&qword_14E6645D8, v134, &v117, &v115);
LABEL_181:
                                        if ( byte_14E6645EC == 0 )
                                        {
                                          v115 = 1674;
                                          v117 = sub_146E8C7D0(&unk_149F61148);
                                          sub_140166070(&qword_14E6645D8, v135, &v117, &v115);
                                          goto LABEL_183;
                                        }
                                        v115 = 3;
                                        v96 = v118[1];
                                        if ( v118[1] != v119 )
                                        {
                                          *(_DWORD *)v118[1] = 3;
                                          v118[1] = (char *)v118[1] + 4;
                                          goto LABEL_209;
                                        }
                                      }
                                    }
LABEL_203:
                                    sub_140B5E2E0(v118, v96, &v115);
                                    goto LABEL_209;
                                  }
LABEL_175:
                                  if ( byte_14E6645EC == 0 )
                                  {
                                    v115 = 1659;
                                    v117 = sub_146E8C7D0(&unk_149F610D0);
                                    sub_140166070(&qword_14E6645D8, v132, &v117, &v115);
                                    goto LABEL_177;
                                  }
                                  v115 = 0;
                                  v96 = v118[1];
                                  if ( v118[1] != v119 )
                                  {
                                    *(_DWORD *)v118[1] = 0;
                                    v118[1] = (char *)v118[1] + 4;
                                    goto LABEL_209;
                                  }
                                  goto LABEL_203;
                                }
                              }
                              if ( byte_14E6645EC != 0 )
                              {
                                v97 = (int *)v118[0];
                                v118[1] = v118[0];
                                for ( j = 0; j < 7; ++j )
                                {
                                  v115 = j;
                                  if ( v97 == v119 )
                                  {
                                    sub_140B5E2E0(v118, v97, &v115);
                                    v97 = (int *)v118[1];
                                  }
                                  else
                                  {
                                    *v97 = j;
                                    v97 = (int *)((char *)v118[1] + 4);
                                    v118[1] = (char *)v118[1] + 4;
                                  }
                                }
                                goto LABEL_209;
                              }
LABEL_174:
                              v115 = 1652;
                              v117 = sub_146E8C7D0(&unk_1491BE780);
                              sub_140166070(&qword_14E6645D8, v131, &v117, &v115);
                              goto LABEL_175;
                            }
                            v93 = v90 - 1674;
                            if ( v93 == 0 )
                              goto LABEL_181;
                            v94 = v93 - 5;
                            if ( v94 != 0 )
                            {
                              v95 = v94 - 5;
                              if ( v95 != 0 )
                              {
                                if ( v95 != 5 )
                                  goto LABEL_209;
                                goto LABEL_187;
                              }
LABEL_185:
                              if ( byte_14E6645EC != 0 )
                              {
                                v115 = 5;
                                v96 = v118[1];
                                if ( v118[1] == v119 )
                                  goto LABEL_203;
                                *(_DWORD *)v118[1] = 5;
                                v118[1] = (char *)v118[1] + 4;
                              }
                              else
                              {
                                v115 = 1684;
                                v117 = sub_146E8C7D0(&unk_149F61198);
                                sub_140166070(&qword_14E6645D8, v137, &v117, &v115);
LABEL_187:
                                if ( byte_14E6645EC != 0 )
                                {
                                  v115 = 6;
                                  v96 = v118[1];
                                  if ( v118[1] == v119 )
                                    goto LABEL_203;
                                  *(_DWORD *)v118[1] = 6;
                                  v118[1] = (char *)v118[1] + 4;
                                }
                                else
                                {
                                  v115 = 1689;
                                  v117 = sub_146E8C7D0(&unk_149F611C0);
                                  sub_140166070(&qword_14E6645D8, v138, &v117, &v115);
                                }
                              }
                            }
                            else
                            {
LABEL_183:
                              if ( byte_14E6645EC == 0 )
                              {
                                v115 = 1679;
                                v117 = sub_146E8C7D0(&unk_149F61170);
                                sub_140166070(&qword_14E6645D8, v136, &v117, &v115);
                                goto LABEL_185;
                              }
                              v115 = 4;
                              v96 = v118[1];
                              if ( v118[1] == v119 )
                                goto LABEL_203;
                              *(_DWORD *)v118[1] = 4;
                              v118[1] = (char *)v118[1] + 4;
                            }
LABEL_209:
                            if ( byte_14E6645EC != 0 )
                              goto LABEL_211;
                            Atomic_lock_release(&unk_14E6645F0);
                            byte_14E6645EC = 1;
                            continue;
                          }
                        }
                      }
                    }
LABEL_212:
                    LOBYTE(v55) = 1;
                  }
                  while ( (unsigned __int8)sub_1470A1A80(a2, v139, v55) != 0 );
                }
                if ( __eh34_unwind(19) )
                  goto unwind_state_19;
LABEL_264:
                __eh34_exit_wind_state(19, 5);
                v23 = v141;
                if ( v141 >= 8 )
                  std::allocator<wchar_t>::deallocate(v139, v139[0], v141 + 1);
                v140 = 0;
                v141 = 7;
                LOWORD(v139[0]) = 0;
              }
              goto LABEL_217;
            }
LABEL_85:
            LOBYTE(v16) = 1;
            if ( (unsigned __int8)sub_1470A09C0(a2, Src, v16, v15) == 0 )
              goto LABEL_105;
            v129 = (char *)&v126;
            *(_QWORD *)&v126 = 0;
            v127 = 0;
            v128 = 0;
            v59 = v149;
            v60 = Src;
            if ( v150 >= 8 )
              v60 = (void **)Src[0];
            if ( v149 >= 8 )
            {
              v61 = v149 | 7;
              if ( (v149 | 7) > 0x7FFFFFFFFFFFFFFELL )
                v61 = 0x7FFFFFFFFFFFFFFELL;
              *(_QWORD *)&v126 = sub_14014CB50(&v126, v61 + 1);
              memmove((void *)v126, v60, 2 * v59 + 2);
              v128 = v61;
            }
            else
            {
              v126 = *(_OWORD *)v60;
              v128 = 7;
            }
            v127 = v59;
            v124 = sub_144DA1480(&v126);
            if ( v124 == 0 )
            {
LABEL_105:
              v66 = 0;
              goto LABEL_285;
            }
LABEL_217:
            LOBYTE(v23) = 1;
            if ( (unsigned __int8)sub_1470A1A80(a2, v142, v23) == 0 )
              goto LABEL_218;
          }
LABEL_232:
          v100 = a1 + 64;
          v101 = *(_QWORD *)(a1 + 72);
          if ( v101 == *(_QWORD *)(a1 + 80) )
          {
            sub_14444BBB0(a1 + 64, *(_QWORD *)(a1 + 72), v118, v8);
          }
          else
          {
            v129 = *(char **)(a1 + 72);
            *(_QWORD *)v101 = 0;
            *(_QWORD *)(v101 + 8) = 0;
            *(_QWORD *)(v101 + 16) = 0;
            v102 = v118[0];
            v103 = v118[1];
            if ( v118[0] != v118[1] )
            {
              v104 = ((char *)v118[1] - (char *)v118[0]) >> 2;
              v105 = sub_140157580(v101, v104, v9, v8);
              *(_QWORD *)v101 = v105;
              *(_QWORD *)(v101 + 8) = v105;
              v106 = 4 * v104;
              *(_QWORD *)(v101 + 16) = 4 * v104 + v105;
              v117 = v101;
              __wind
              {
                v107 = *(char **)v101;
                memmove(*(void **)v101, v102, v103 - (_BYTE *)v102);
                *(_QWORD *)(v101 + 8) = &v107[v106];
                v117 = 0;
              }
              __unwind
              {
                sub_140155CC0(&v117);
              }
            }
            __wind
            {
              v108 = (void **)(v101 + 24);
              *(_QWORD *)(v101 + 24) = 0;
              *(_QWORD *)(v101 + 32) = 0;
              *(_QWORD *)(v101 + 40) = 0;
              v109 = v120;
              v110 = (char *)v121;
              if ( v120 != v121 )
              {
                v111 = ((char *)v121 - (_BYTE *)v120) >> 2;
                v112 = sub_140157580(v101 + 24, v111, v9, v8);
                *v108 = (void *)v112;
                *(_QWORD *)(v101 + 32) = v112;
                v113 = 4 * v111;
                *(_QWORD *)(v101 + 40) = 4 * v111 + v112;
                v117 = v101 + 24;
                __wind
                {
                  v114 = (char *)*v108;
                  memmove(*v108, v109, v110 - (_BYTE *)v109);
                  *(_QWORD *)(v101 + 32) = &v114[v113];
                  v117 = 0;
                }
                __unwind
                {
                  sub_140155CC0(&v117);
                }
              }
              __wind
              {
                *(_QWORD *)(v101 + 48) = v123;
                *(_DWORD *)(v101 + 56) = v124;
              }
              __unwind
              {
                sub_1401574A0(v129 + 24);
              }
            }
            __unwind
            {
              sub_140156720(v129);
            }
            *(_QWORD *)(v100 + 8) += 64LL;
          }
LABEL_218:
          v66 = 1;
        }
        __unwind
        {
          sub_14444C3C0(v118);
        }
LABEL_285:
        __wind
        {
          if ( v120 != nullptr )
          {
            sub_1401576D0(&v120, v120, ((char *)v122 - (_BYTE *)v120) >> 2);
            v120 = nullptr;
            v121 = nullptr;
            v122 = nullptr;
          }
        }
        __unwind
        {
          sub_140156720(v118);
        }
        if ( v118[0] != nullptr )
        {
          sub_1401576D0(v118, v118[0], ((char *)v119 - (char *)v118[0]) >> 2);
          *(_OWORD *)v118 = 0;
          v119 = nullptr;
        }
      }
      __unwind
      {
        unknown_libname_4(v145);
      }
      if ( v147 >= 8 )
        std::allocator<wchar_t>::deallocate(v145, v145[0], v147 + 1);
      v146 = 0;
      v147 = 7;
      LOWORD(v145[0]) = 0;
    }
    __unwind
    {
      unknown_libname_4(Src);
    }
    if ( v150 >= 8 )
      std::allocator<wchar_t>::deallocate(Src, Src[0], v150 + 1);
    v149 = 0;
    v150 = 7;
    LOWORD(Src[0]) = 0;
  }
  __unwind
  {
    unknown_libname_4(v142);
  }
  if ( v144 >= 8 )
    std::allocator<wchar_t>::deallocate(v142, v142[0], v144 + 1);
  v143 = 0;
  v144 = 7;
  LOWORD(v142[0]) = 0;
  return v66;
}
