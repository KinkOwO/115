// reader_sub_1444E5210

unsigned __int64 __fastcall sub_1444E5210(
        __int64 a1,
        int a2,
        unsigned int a3,
        int a4,
        unsigned __int16 *a5,
        __int64 a6)
{
  __int64 v6; // rsi
  unsigned __int64 v8; // r8
  __int64 v9; // r13
  __int64 v10; // rdx
  unsigned __int64 result; // rax
  __int64 v12; // rdi
  __int64 v13; // rax
  __int64 v14; // rcx
  unsigned __int64 v15; // rbx
  __int64 v16; // rax
  char v17; // r14
  char v18; // r15
  char v19; // r12
  __int64 *v20; // rax
  __int64 *v21; // rbx
  __int64 v22; // r8
  _QWORD *v23; // rdi
  __int64 v24; // rsi
  __int64 v25; // rcx
  __int64 v26; // rax
  void (__fastcall ***v27)(_QWORD); // rcx
  __int64 v28; // rcx
  int v29; // ecx
  int v30; // eax
  int v31; // ecx
  int v32; // eax
  int v33; // ecx
  int v34; // eax
  int v35; // edx
  int v36; // ecx
  __int64 v37; // rsi
  _DWORD *v38; // r14
  __int64 v39; // rdi
  __int64 v40; // rcx
  __int64 v41; // r8
  __int64 v42; // rdx
  __int64 v43; // rcx
  __int64 v44; // rdx
  __int64 v45; // rcx
  __int64 v46; // rdx
  unsigned int v47; // esi
  __int64 v48; // rcx
  __int64 v49; // rax
  void (__fastcall ***v50)(_QWORD); // rcx
  __int64 v51; // rdx
  __int64 v52; // rdx
  __int16 *v53; // r12
  unsigned int *v54; // r15
  _QWORD *v55; // rdi
  _WORD *v56; // r14
  __int64 v57; // r13
  __int64 v58; // rcx
  __int64 v59; // r8
  __int64 v60; // rdx
  __int64 v61; // rcx
  __int64 v62; // rdx
  __int64 v63; // rcx
  __int64 v64; // rdx
  unsigned int v65; // esi
  __int64 v66; // rcx
  __int64 v67; // rax
  void (__fastcall ***v68)(_QWORD); // rcx
  __int64 v69; // rdx
  __int64 v70; // rdx
  __int64 v71; // rcx
  __int64 v72; // rdx
  __int64 v73; // rdx
  __int64 v74; // rdi
  __int64 v75; // rcx
  __int64 v76; // rdx
  __int64 v77; // rcx
  __int64 v78; // rdx
  __int64 *v79; // rdi
  _WORD *v80; // rsi
  __int64 v81; // r14
  __int64 v82; // rcx
  __int64 v83; // rdx
  __int64 v84; // rdx
  unsigned __int64 v85; // rax
  __int64 v86; // rcx
  _QWORD *v87; // r8
  __int64 v88; // rdx
  __int64 v89; // rbx
  unsigned __int16 *v90; // r15
  __int64 v91; // rcx
  __int64 v92; // r9
  _BYTE *v93; // rcx
  __int64 v94; // rdx
  __int64 v95; // r8
  _QWORD *v96; // rbx
  _WORD *v97; // rdi
  __int64 v98; // rcx
  __int64 v99; // rax
  void (__fastcall ***v100)(_QWORD); // rcx
  __int64 v101; // rax
  unsigned __int64 v102; // r14
  __int64 v103; // rcx
  __int64 v104; // rax
  void (__fastcall ***v105)(_QWORD); // rcx
  __int64 v106; // rdx
  __int64 v107; // rcx
  int v108; // eax
  __int16 v109; // [rsp+40h] [rbp-C0h] BYREF
  char v110; // [rsp+42h] [rbp-BEh]
  unsigned int v111; // [rsp+44h] [rbp-BCh]
  unsigned __int16 *v112; // [rsp+48h] [rbp-B8h]
  unsigned int v113; // [rsp+50h] [rbp-B0h]
  unsigned __int64 v114; // [rsp+58h] [rbp-A8h]
  unsigned __int16 *v115; // [rsp+60h] [rbp-A0h]
  _DWORD *v116; // [rsp+68h] [rbp-98h]
  __int128 v117; // [rsp+70h] [rbp-90h] BYREF
  __int64 v118; // [rsp+80h] [rbp-80h]
  __int128 v119; // [rsp+88h] [rbp-78h] BYREF
  __int128 v120; // [rsp+98h] [rbp-68h] BYREF
  __int64 v121; // [rsp+A8h] [rbp-58h]
  __int128 *v122; // [rsp+B0h] [rbp-50h]
  __int64 v123; // [rsp+B8h] [rbp-48h]
  __int128 *v124; // [rsp+C0h] [rbp-40h]
  __int64 v125; // [rsp+C8h] [rbp-38h]
  __int64 v126; // [rsp+D0h] [rbp-30h]
  _BYTE v127[288]; // [rsp+E0h] [rbp-20h] BYREF
  _BYTE v128[1968]; // [rsp+200h] [rbp+100h] BYREF
  _BYTE v129[1968]; // [rsp+9B0h] [rbp+8B0h] BYREF
  __int64 v130; // [rsp+1160h] [rbp+1060h] BYREF
  int v131; // [rsp+1168h] [rbp+1068h]

  v125 = -2;
  v6 = a4;
  v8 = a2;
  v113 = a2;
  v9 = a1;
  v121 = a1;
  v115 = a5;
  *(_QWORD *)&v117 = a6;
  v10 = *(_QWORD *)(a1 + 4352);
  result = (*(_QWORD *)(a1 + 4360) - v10) >> 2;
  if ( v8 < result )
  {
    v12 = 4 * v8;
    if ( *(_DWORD *)(4 * v8 + v10) == a3 )
    {
      v13 = sub_1444D2BB0(a1);
      result = sub_1444D2A50(v13, a3);
      v15 = result;
      v114 = result;
      if ( result )
      {
        v16 = sub_1444D2BB0(v14);
        result = sub_1444D2A20(v16);
        v112 = (unsigned __int16 *)result;
        if ( result )
        {
          if ( *(_WORD *)result >= *(_WORD *)(v15 + 8) && a5 )
          {
            v17 = 0;
            v109 = 0;
            v18 = 0;
            v19 = 0;
            v110 = 0;
            v130 = 0;
            v131 = 0;
            v111 = 0;
            sub_148AA1E60(
              v12 + *(_QWORD *)(v9 + 4352),
              v12 + *(_QWORD *)(v9 + 4352) + 4,
              *(_QWORD *)(v9 + 4360) - (v12 + *(_QWORD *)(v9 + 4352) + 4));
            *(_QWORD *)(v9 + 4360) -= 4LL;
            v20 = *(__int64 **)(v15 + 48);
            v21 = (__int64 *)*v20;
            v22 = v6;
            v118 = v6;
            if ( v21 != v20 )
            {
              do
              {
                switch ( *((_DWORD *)v21 + 4) )
                {
                  case 1:
                    v35 = v112[179 * v22 + 447];
                    v36 = v115[179 * v22 + 447];
                    if ( v112[179 * v22 + 447] )
                    {
                      if ( !v115[179 * v22 + 447] )
                      {
                        *((_BYTE *)&v109 + v22) = 1;
                        v19 = v110;
                        v18 = HIBYTE(v109);
                        v17 = v109;
                      }
                      *((_DWORD *)&v130 + v22) = v36 - v35;
                    }
                    break;
                  case 2:
                    v29 = v112[447];
                    v30 = v115[447];
                    if ( v112[447] )
                    {
                      if ( !v115[447] )
                        v17 = 1;
                      LOBYTE(v109) = v17;
                      LODWORD(v130) = v30 - v29;
                    }
                    v31 = v112[626];
                    v32 = v115[626];
                    if ( v112[626] )
                    {
                      if ( !v115[626] )
                        v18 = 1;
                      HIBYTE(v109) = v18;
                      HIDWORD(v130) = v32 - v31;
                    }
                    v33 = v112[805];
                    v34 = v115[805];
                    if ( v112[805] )
                    {
                      if ( !v115[805] )
                        v19 = 1;
                      v110 = v19;
                      v131 = v34 - v33;
                    }
                    break;
                  case 0x12:
                    v28 = (__int64)(*(_QWORD *)(v9 + 4360) - *(_QWORD *)(v9 + 4352)) >> 2;
                    if ( v28 + (unsigned __int64)*((unsigned int *)v21 + 5) > 8 )
                      v111 = 8 - v28;
                    break;
                  case 0x13:
                    *(_QWORD *)(v9 + 4360) = *(_QWORD *)(v9 + 4352);
                    v23 = (_QWORD *)(v9 + 3688);
                    v24 = 8;
                    do
                    {
                      if ( *v23 )
                        (*(void (__fastcall **)(_QWORD, _QWORD, __int64))(*(_QWORD *)*v23 + 16LL))(*v23, 0, v22);
                      v23 += 2;
                      --v24;
                    }
                    while ( v24 );
                    v25 = qword_14E664BF8;
                    if ( !qword_14E664BF8 )
                    {
                      v26 = sub_146E8BA20(2792);
                      v116 = (_DWORD *)v26;
                      if ( v26 )
                        v27 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v26);
                      else
                        v27 = 0;
                      qword_14E664BF8 = (__int64)v27;
                      (**v27)(v27);
                      v25 = qword_14E664BF8;
                    }
                    v111 = sub_1401B63C0(v25);
                    v22 = v118;
                    break;
                }
                v21 = (__int64 *)*v21;
                v20 = *(__int64 **)(v114 + 48);
              }
              while ( v21 != v20 );
              v21 = (__int64 *)*v20;
            }
            v37 = 2;
            if ( v21 == v20 )
            {
              v85 = v114;
            }
            else
            {
              v38 = (_DWORD *)&v130 + v22;
              v116 = v38;
              do
              {
                switch ( *((_DWORD *)v21 + 4) )
                {
                  case 1:
                    if ( v22 < 3 && v112[179 * v22 + 447] )
                    {
                      v39 = 2 * v22;
                      v40 = *(_QWORD *)(v9 + 16 * v22 + 2824);
                      if ( v40 )
                      {
                        if ( *((_BYTE *)&v109 + v22) )
                        {
                          v41 = *(_QWORD *)(v9 + 16 * v22 + 3064);
                          if ( v41 )
                          {
                            (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)v41 + 16LL))(
                              *(_QWORD *)(v9 + 8 * v39 + 3064),
                              0);
                            v40 = *(_QWORD *)(v9 + 8 * v39 + 2824);
                          }
                          v42 = 1;
                        }
                        else
                        {
                          v42 = 2;
                        }
                        sub_146AF0950(v40, v42);
                        (*(void (__fastcall **)(_QWORD))(**(_QWORD **)(v9 + 8 * v39 + 2824) + 432LL))(*(_QWORD *)(v9 + 8 * v39 + 2824));
                      }
                      v43 = *(_QWORD *)(v9 + 8 * v39 + 2776);
                      if ( v43 && !(unsigned __int8)sub_141FB6530(v43) )
                      {
                        sub_146AF0950(*(_QWORD *)(v9 + 8 * v39 + 2776), 0);
                        (*(void (__fastcall **)(_QWORD))(**(_QWORD **)(v9 + 8 * v39 + 2776) + 432LL))(*(_QWORD *)(v9 + 8 * v39 + 2776));
                        LOBYTE(v44) = 1;
                        (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(v9 + 8 * v39 + 2776) + 16LL))(
                          *(_QWORD *)(v9 + 8 * v39 + 2776),
                          v44);
                      }
                      v45 = *(_QWORD *)(v9 + 8 * v39 + 2872);
                      if ( v45 && !(unsigned __int8)sub_141FB6530(v45) )
                      {
                        sub_146AF0950(*(_QWORD *)(v9 + 8 * v39 + 2872), 0);
                        (*(void (__fastcall **)(_QWORD))(**(_QWORD **)(v9 + 8 * v39 + 2872) + 432LL))(*(_QWORD *)(v9 + 8 * v39 + 2872));
                        LOBYTE(v46) = 1;
                        (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(v9 + 8 * v39 + 2872) + 16LL))(
                          *(_QWORD *)(v9 + 8 * v39 + 2872),
                          v46);
                      }
                      if ( *(_QWORD *)(v9 + 8 * v39 + 3496) )
                      {
                        v47 = *v38;
                        if ( *v38 )
                        {
                          v48 = qword_14E664BF8;
                          if ( !qword_14E664BF8 )
                          {
                            v49 = sub_146E8BA20(2792);
                            v126 = v49;
                            if ( v49 )
                              v50 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v49);
                            else
                              v50 = 0;
                            qword_14E664BF8 = (__int64)v50;
                            (**v50)(v50);
                            v48 = qword_14E664BF8;
                          }
                          v122 = &v119;
                          v119 = 0;
                          v51 = *(_QWORD *)(v9 + 8 * v39 + 3504);
                          if ( v51 )
                          {
                            _InterlockedIncrement((volatile signed __int32 *)(v51 + 8));
                            v51 = *(_QWORD *)(v9 + 8 * v39 + 3504);
                          }
                          *(_QWORD *)&v119 = *(_QWORD *)(v9 + 8 * v39 + 3496);
                          *((_QWORD *)&v119 + 1) = v51;
                          sub_1444D3910(v48, &v119, v47);
                          (*(void (__fastcall **)(_QWORD))(**(_QWORD **)(v9 + 8 * v39 + 3496) + 432LL))(*(_QWORD *)(v9 + 8 * v39 + 3496));
                          LOBYTE(v52) = 1;
                          (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(v9 + 8 * v39 + 3496) + 16LL))(
                            *(_QWORD *)(v9 + 8 * v39 + 3496),
                            v52);
                        }
                      }
                    }
                    goto LABEL_122;
                  case 2:
                    v53 = &v109;
                    v54 = (unsigned int *)&v130;
                    v55 = (_QWORD *)(v9 + 2824);
                    v56 = v112 + 447;
                    v57 = 3;
                    do
                    {
                      if ( *v56 )
                      {
                        v58 = *v55;
                        if ( *v55 )
                        {
                          if ( *(_BYTE *)v53 )
                          {
                            v59 = v55[30];
                            if ( v59 )
                            {
                              (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)v59 + 16LL))(v55[30], 0);
                              v58 = *v55;
                            }
                            v60 = 1;
                          }
                          else
                          {
                            v60 = 2;
                          }
                          sub_146AF0950(v58, v60);
                          (*(void (__fastcall **)(_QWORD))(*(_QWORD *)*v55 + 432LL))(*v55);
                        }
                        v61 = *(v55 - 6);
                        if ( v61 && !(unsigned __int8)sub_141FB6530(v61) )
                        {
                          sub_146AF0950(*(v55 - 6), 0);
                          (*(void (__fastcall **)(_QWORD))(*(_QWORD *)*(v55 - 6) + 432LL))(*(v55 - 6));
                          LOBYTE(v62) = 1;
                          (*(void (__fastcall **)(_QWORD, __int64))(*(_QWORD *)*(v55 - 6) + 16LL))(*(v55 - 6), v62);
                        }
                        v63 = v55[6];
                        if ( v63 && !(unsigned __int8)sub_141FB6530(v63) )
                        {
                          sub_146AF0950(v55[6], 0);
                          (*(void (__fastcall **)(_QWORD))(*(_QWORD *)v55[6] + 432LL))(v55[6]);
                          LOBYTE(v64) = 1;
                          (*(void (__fastcall **)(_QWORD, __int64))(*(_QWORD *)v55[6] + 16LL))(v55[6], v64);
                        }
                        if ( v55[84] )
                        {
                          v65 = *v54;
                          if ( *v54 )
                          {
                            v66 = qword_14E664BF8;
                            if ( !qword_14E664BF8 )
                            {
                              v67 = sub_146E8BA20(2792);
                              v123 = v67;
                              if ( v67 )
                                v68 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v67);
                              else
                                v68 = 0;
                              qword_14E664BF8 = (__int64)v68;
                              (**v68)(v68);
                              v66 = qword_14E664BF8;
                            }
                            v124 = &v120;
                            v120 = 0;
                            v69 = v55[85];
                            if ( v69 )
                            {
                              _InterlockedIncrement((volatile signed __int32 *)(v69 + 8));
                              v69 = v55[85];
                            }
                            *(_QWORD *)&v120 = v55[84];
                            *((_QWORD *)&v120 + 1) = v69;
                            sub_1444D3910(v66, &v120, v65);
                            (*(void (__fastcall **)(_QWORD))(*(_QWORD *)v55[84] + 432LL))(v55[84]);
                            LOBYTE(v70) = 1;
                            (*(void (__fastcall **)(_QWORD, __int64))(*(_QWORD *)v55[84] + 16LL))(v55[84], v70);
                          }
                        }
                      }
                      v56 += 179;
                      v53 = (__int16 *)((char *)v53 + 1);
                      ++v54;
                      v55 += 2;
                      --v57;
                    }
                    while ( v57 );
                    v9 = v121;
                    goto LABEL_121;
                  case 3:
                    v71 = *(_QWORD *)(v9 + 3896);
                    if ( !v71 )
                      goto LABEL_122;
                    v72 = 1;
                    break;
                  case 4:
                  case 5:
                  case 7:
                  case 0x15:
                    v71 = *(_QWORD *)(v9 + 3896);
                    if ( !v71 )
                      goto LABEL_122;
                    v72 = 2;
                    break;
                  case 6:
                    v71 = *(_QWORD *)(v9 + 3896);
                    if ( !v71 )
                      goto LABEL_122;
                    v72 = 3;
                    break;
                  case 8:
                  case 0xA:
                    if ( v22 < 3 && v112[179 * v22 + 447] )
                    {
                      v74 = v9 + 16 * v22;
                      v75 = *(_QWORD *)(v74 + 2776);
                      if ( v75 && !(unsigned __int8)sub_141FB6530(v75) )
                      {
                        sub_146AF0950(*(_QWORD *)(v74 + 2776), 3);
                        (*(void (__fastcall **)(_QWORD))(**(_QWORD **)(v74 + 2776) + 432LL))(*(_QWORD *)(v74 + 2776));
                        LOBYTE(v76) = 1;
                        (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(v74 + 2776) + 16LL))(
                          *(_QWORD *)(v74 + 2776),
                          v76);
                      }
                      v77 = *(_QWORD *)(v74 + 2872);
                      if ( v77 && !(unsigned __int8)sub_141FB6530(v77) )
                      {
                        sub_146AF0950(*(_QWORD *)(v74 + 2872), 3);
                        (*(void (__fastcall **)(_QWORD))(**(_QWORD **)(v74 + 2872) + 432LL))(*(_QWORD *)(v74 + 2872));
                        LOBYTE(v78) = 1;
                        (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(v74 + 2872) + 16LL))(
                          *(_QWORD *)(v74 + 2872),
                          v78);
                      }
                    }
                    goto LABEL_122;
                  case 9:
                  case 0xB:
                    v79 = (__int64 *)(v9 + 2872);
                    v80 = v112 + 447;
                    v81 = 3;
                    do
                    {
                      if ( *v80 )
                      {
                        v82 = *(v79 - 12);
                        if ( v82 && !(unsigned __int8)sub_141FB6530(v82) )
                        {
                          sub_146AF0950(*(v79 - 12), 3);
                          (*(void (__fastcall **)(_QWORD))(*(_QWORD *)*(v79 - 12) + 432LL))(*(v79 - 12));
                          LOBYTE(v83) = 1;
                          (*(void (__fastcall **)(_QWORD, __int64))(*(_QWORD *)*(v79 - 12) + 16LL))(*(v79 - 12), v83);
                        }
                        if ( *v79 && !(unsigned __int8)sub_141FB6530(*v79) )
                        {
                          sub_146AF0950(*v79, 3);
                          (*(void (__fastcall **)(__int64))(*(_QWORD *)*v79 + 432LL))(*v79);
                          LOBYTE(v84) = 1;
                          (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)*v79 + 16LL))(*v79, v84);
                        }
                      }
                      v80 += 179;
                      v79 += 2;
                      --v81;
                    }
                    while ( v81 );
LABEL_121:
                    v38 = v116;
                    goto LABEL_122;
                  default:
                    goto LABEL_122;
                }
                sub_146AF0950(v71, v72);
                (*(void (__fastcall **)(_QWORD))(**(_QWORD **)(v9 + 3896) + 432LL))(*(_QWORD *)(v9 + 3896));
                LOBYTE(v73) = 1;
                (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(v9 + 3896) + 16LL))(*(_QWORD *)(v9 + 3896), v73);
LABEL_122:
                v21 = (__int64 *)*v21;
                v85 = v114;
                v37 = 2;
                v22 = v118;
              }
              while ( v21 != *(__int64 **)(v114 + 48) );
            }
            v86 = *(_QWORD *)(v9 + 3832);
            if ( v86 )
            {
              v87 = (_QWORD *)(v85 + 16);
              if ( *(_QWORD *)(v85 + 40) >= 8u )
                v87 = (_QWORD *)*v87;
              sub_146AF0F40(v86, 0, v87);
              sub_146AF0950(*(_QWORD *)(v9 + 3832), 0);
              (*(void (__fastcall **)(_QWORD))(**(_QWORD **)(v9 + 3832) + 432LL))(*(_QWORD *)(v9 + 3832));
              LOBYTE(v88) = 1;
              (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(v9 + 3832) + 16LL))(*(_QWORD *)(v9 + 3832), v88);
            }
            v89 = sub_1444D2BB0(v86);
            v90 = v115;
            sub_148AA1E60(v128, v115, 1968);
            sub_1444D38A0(v89, v128);
            v92 = sub_1444D2BB0(v91);
            v93 = v127;
            v94 = v117;
            do
            {
              *(_OWORD *)v93 = *(_OWORD *)v94;
              *((_OWORD *)v93 + 1) = *(_OWORD *)(v94 + 16);
              *((_OWORD *)v93 + 2) = *(_OWORD *)(v94 + 32);
              *((_OWORD *)v93 + 3) = *(_OWORD *)(v94 + 48);
              *((_OWORD *)v93 + 4) = *(_OWORD *)(v94 + 64);
              *((_OWORD *)v93 + 5) = *(_OWORD *)(v94 + 80);
              *((_OWORD *)v93 + 6) = *(_OWORD *)(v94 + 96);
              v93 += 128;
              *((_OWORD *)v93 - 1) = *(_OWORD *)(v94 + 112);
              v94 += 128;
              --v37;
            }
            while ( v37 );
            *(_OWORD *)v93 = *(_OWORD *)v94;
            *((_QWORD *)v93 + 2) = *(_QWORD *)(v94 + 16);
            *((_DWORD *)v93 + 6) = *(_DWORD *)(v94 + 24);
            sub_1444D40D0(v92, v127);
            sub_148AA1E60(v129, v90, 1968);
            sub_1444E0750(v9, v129);
            sub_1444E4E40(v9);
            sub_1444DD420(v9, v111, v113);
            v96 = (_QWORD *)(v9 + 3064);
            v97 = v90 + 447;
            do
            {
              v98 = qword_14E664BF8;
              if ( !qword_14E664BF8 )
              {
                v99 = sub_146E8BA20(2792);
                v124 = (__int128 *)v99;
                if ( v99 )
                  v100 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v99);
                else
                  v100 = 0;
                qword_14E664BF8 = (__int64)v100;
                (**v100)(v100);
                v98 = qword_14E664BF8;
              }
              v101 = sub_1444D2CC0(v98, *((unsigned int *)v97 - 1), v95);
              if ( v101 && *v96 && *v97 )
              {
                v102 = (*(_DWORD *)(v9 + 4404) - 1)
                     % (unsigned __int64)((*(_QWORD *)(v101 + 56) - *(_QWORD *)(v101 + 48)) / 48LL);
                v103 = qword_14E664BF8;
                if ( !qword_14E664BF8 )
                {
                  v104 = sub_146E8BA20(2792);
                  v123 = v104;
                  if ( v104 )
                    v105 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v104);
                  else
                    v105 = 0;
                  qword_14E664BF8 = (__int64)v105;
                  (**v105)(v105);
                  v103 = qword_14E664BF8;
                }
                v122 = &v117;
                v117 = 0;
                v106 = v96[1];
                if ( v106 )
                {
                  _InterlockedIncrement((volatile signed __int32 *)(v106 + 8));
                  v106 = v96[1];
                }
                *(_QWORD *)&v117 = *v96;
                *((_QWORD *)&v117 + 1) = v106;
                sub_1444D4530(v103, &v117, (unsigned int)v37, (unsigned int)v102);
              }
              LODWORD(v37) = v37 + 1;
              v97 += 179;
              v96 += 2;
            }
            while ( (int)v37 < 3 );
            *(_QWORD *)(v9 + 4280) = -1;
            v107 = *(_QWORD *)(v9 + 2584);
            if ( v107 )
              sub_143FD16D0(v107, *v90);
            v108 = sub_146E8C7D0(&unk_149CFD1A0);
            return sub_145A31380(v108, -1, 0, 0, -1, -1, 0);
          }
        }
      }
    }
  }
  return result;
}

