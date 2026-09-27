// ins_sub_144A797F0

void __fastcall sub_144A797F0(__int64 a1, int a2)
{
  __int64 v2; // r15
  unsigned int v4; // esi
  _QWORD *v5; // r14
  _QWORD *i; // rbx
  _QWORD *v7; // r14
  _QWORD *j; // rbx
  __int64 v9; // rax
  int v10; // ebx
  __int64 v11; // rax
  __int64 v12; // rax
  int v13; // eax
  __int64 *v14; // rcx
  __int64 *v15; // rdx
  __int64 v16; // rax
  __int64 v17; // rax
  __int64 v18; // rdx
  __int64 v19; // rax
  __int64 v20; // rax
  __int64 v21; // rax
  __int64 v22; // rax
  __int64 v23; // rdx
  __int64 v24; // rax
  __int64 v25; // rax
  __int64 v26; // rax
  __int64 v27; // rax
  unsigned int *v28; // r13
  unsigned int *m; // r14
  unsigned __int16 v30; // r15
  __int64 v31; // rax
  __int64 v32; // rbx
  int v33; // esi
  _QWORD *v34; // rdx
  volatile signed __int32 *v35; // rbx
  _QWORD *v36; // rbx
  _QWORD *n; // rsi
  __int64 v38; // rcx
  __int64 v39; // rcx
  unsigned __int64 v40; // rdx
  __int64 v41; // rcx
  __int64 v42; // rcx
  __int64 v43; // rcx
  unsigned int *v44; // r13
  unsigned int *ii; // r14
  unsigned __int16 v46; // r15
  __int64 v47; // rax
  __int64 v48; // rbx
  int v49; // esi
  _QWORD *v50; // rdx
  volatile signed __int32 *v51; // rbx
  _QWORD *v52; // rbx
  _QWORD *jj; // rsi
  __int64 v54; // rcx
  __int64 v55; // rcx
  unsigned __int64 v56; // rdx
  __int64 v57; // rcx
  __int64 v58; // rcx
  __int64 v59; // rcx
  __int64 v60; // rcx
  unsigned int *v61; // r13
  unsigned int *kk; // r14
  unsigned __int16 v63; // r15
  __int64 v64; // rax
  __int64 v65; // rbx
  int v66; // esi
  _QWORD *v67; // rdx
  volatile signed __int32 *v68; // rbx
  _QWORD *v69; // rbx
  _QWORD *mm; // rsi
  __int64 v71; // rcx
  __int64 v72; // rcx
  unsigned __int64 v73; // rdx
  __int64 v74; // rax
  __int64 v75; // rcx
  __int64 v76; // rax
  __int64 v77; // rdx
  __int64 v78; // rcx
  __int64 v79; // rax
  unsigned int v80; // r15d
  __int64 v81; // rax
  unsigned int v82; // eax
  __int64 v83; // r8
  __int64 v84; // rbx
  unsigned int v85; // edi
  __int64 v86; // rax
  char v87; // r14
  int k; // ebx
  __int64 v89; // rax
  __int64 v90; // rax
  __int64 v91; // r8
  __int64 v92; // rax
  __int64 v93; // rsi
  __int64 v94; // rax
  void (__fastcall ***v95)(_QWORD); // rcx
  __int64 v96; // rax
  __int64 v97; // rax
  __int128 v98; // [rsp+38h] [rbp-C8h] BYREF
  __int128 v99; // [rsp+48h] [rbp-B8h] BYREF
  __int128 v100; // [rsp+58h] [rbp-A8h] BYREF
  __int128 v101; // [rsp+68h] [rbp-98h] BYREF
  __int128 v102; // [rsp+78h] [rbp-88h] BYREF
  __int128 v103; // [rsp+88h] [rbp-78h] BYREF
  __int128 v104; // [rsp+98h] [rbp-68h] BYREF
  __int128 v105; // [rsp+A8h] [rbp-58h] BYREF
  __int128 v106; // [rsp+B8h] [rbp-48h] BYREF
  __int128 v107; // [rsp+C8h] [rbp-38h] BYREF
  __int128 v108; // [rsp+D8h] [rbp-28h] BYREF
  __int128 v109; // [rsp+E8h] [rbp-18h] BYREF
  __int128 v110; // [rsp+F8h] [rbp-8h] BYREF
  __int128 v111; // [rsp+108h] [rbp+8h] BYREF
  __int64 v112; // [rsp+118h] [rbp+18h]
  __int128 v113; // [rsp+120h] [rbp+20h] BYREF
  __int64 v114; // [rsp+130h] [rbp+30h]
  __int128 v115; // [rsp+138h] [rbp+38h] BYREF
  __int64 v116; // [rsp+148h] [rbp+48h]
  __int64 v117; // [rsp+150h] [rbp+50h]

  v117 = -2;
  v2 = a2;
  v4 = 0;
  if ( *(_DWORD *)(a1 + 368) != a2 )
  {
    sub_146E9FBC0(a1 + 392);
    sub_146E9FF10(a1 + 392, 0, 0);
    *(_QWORD *)(a1 + 380) = 0;
    *(_DWORD *)(a1 + 372) = 0;
    v5 = *(_QWORD **)(a1 + 328);
    for ( i = *(_QWORD **)(a1 + 320); i != v5; i += 2 )
      (*(void (__fastcall **)(_QWORD, __int64))(*(_QWORD *)*i + 32LL))(*i, 2);
    v7 = *(_QWORD **)(a1 + 352);
    for ( j = *(_QWORD **)(a1 + 344); j != v7; j += 2 )
      (*(void (__fastcall **)(_QWORD, __int64))(*(_QWORD *)*j + 32LL))(*j, 2);
    *(_DWORD *)(a1 + 368) = v2;
    if ( sub_145EFAFB0() )
    {
      v9 = sub_145EFAFB0();
      v10 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v9 + 4832LL))(v9);
      if ( v10 != 13 || (unsigned __int8)sub_144A761A0(a1) != 1 )
      {
        switch ( v2 )
        {
          case 0LL:
            sub_14668C520(qword_14E683C78, 3455, 0, 1);
            v11 = sub_1429DA6A0(qword_14E683C78);
            sub_1460112D0(v11, 0);
            if ( sub_145EFAFB0() && qword_14E684AD0 )
            {
              v12 = sub_145EFAFB0();
              v13 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v12 + 4832LL))(v12);
              v14 = *(__int64 **)(*(_QWORD *)qword_14E684AD0 + 8LL);
              v15 = *(__int64 **)qword_14E684AD0;
              while ( !*((_BYTE *)v14 + 25) )
              {
                if ( *((_DWORD *)v14 + 8) >= v13 )
                {
                  v15 = v14;
                  v14 = (__int64 *)*v14;
                }
                else
                {
                  v14 = (__int64 *)v14[2];
                }
              }
              if ( !*((_BYTE *)v15 + 25) && v13 >= *((_DWORD *)v15 + 8) && v15 != *(__int64 **)qword_14E684AD0 )
              {
                v16 = sub_145EFAFB0();
                v17 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v16 + 304LL))(v16);
                LOBYTE(v18) = 1;
                sub_145E041D0(v17, v18);
              }
            }
            sub_146694150(qword_14E683C78);
            sub_145F588B0(1);
            break;
          case 1LL:
            sub_144A7A5A0(a1);
            if ( sub_145EFAFB0() )
            {
              v19 = sub_145EFAFB0();
              if ( (*(unsigned int (__fastcall **)(__int64))(*(_QWORD *)v19 + 4832LL))(v19) == 14 )
              {
                v20 = sub_145EFAFB0();
                if ( (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v20 + 304LL))(v20) )
                {
                  v21 = sub_145EFAFB0();
                  v22 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v21 + 304LL))(v21);
                  LOBYTE(v23) = 1;
                  sub_145E041D0(v22, v23);
                }
              }
            }
            break;
          case 2LL:
            if ( sub_145EFAFB0() )
            {
              v79 = sub_145EFAFB0();
              v80 = sub_145CD9240(v79);
              v81 = sub_145EFAFB0();
              v82 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v81 + 4832LL))(v81);
              LOBYTE(v83) = 1;
              v84 = sub_140283D60(qword_14E683B18, v82, v83);
              v85 = 0;
              if ( v84 )
              {
                v86 = sub_145EFAFB0();
                v85 = *(_DWORD *)(v84
                                + 4LL
                                * *(int *)((*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v86 + 7656LL))(v86) + 12)
                                + 468);
              }
              v87 = 0;
              for ( k = 0; k < 2; ++k )
              {
                v89 = sub_145EFAFB0();
                sub_145D30800(v89, (unsigned int)k);
                v90 = sub_145EFAFB0();
                LOBYTE(v91) = 1;
                v92 = (*(__int64 (__fastcall **)(__int64, __int64, __int64))(*(_QWORD *)v90 + 8264LL))(v90, 197, v91);
                v93 = v92;
                if ( v92 )
                {
                  if ( (unsigned int)sub_145ECAC70(v92) != v85 )
                  {
                    if ( v85 )
                    {
                      sub_145EE9440(v93, v85);
                      if ( !v87 )
                      {
                        if ( !qword_14E634230 )
                        {
                          v94 = sub_146E8BA20(112);
                          if ( v94 )
                            v95 = (void (__fastcall ***)(_QWORD))sub_1403DE110(v94);
                          else
                            v95 = 0;
                          qword_14E634230 = (__int64)v95;
                          (**v95)(v95);
                        }
                        sub_1403EF2C0();
                        v87 = 1;
                      }
                    }
                  }
                }
              }
              v96 = sub_145EFAFB0();
              sub_145D30800(v96, v80);
              v97 = sub_145EFAFB0();
              (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v97 + 4816LL))(v97, 0);
            }
            break;
          case 4LL:
            v24 = sub_1429DA6A0(qword_14E683C78);
            sub_1460112D0(v24, 0);
            v25 = sub_1459A9B20(qword_14E66C090, 3);
            if ( v25 )
            {
              v26 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v25 + 216LL))(v25);
              if ( v26 )
                sub_145EA4240(v26, 0);
            }
            sub_14668C520(qword_14E683C78, 1189, 0, 0);
            if ( sub_145EFAFB0() )
            {
              v27 = sub_145EFAFB0();
              if ( (*(unsigned int (__fastcall **)(__int64))(*(_QWORD *)v27 + 4832LL))(v27) != 10 )
                sub_144A73B60(a1, 3);
            }
            if ( v10 == 10 )
              sub_14668C520(qword_14E683C78, 3903, 0, 0);
            break;
          case 5LL:
            if ( v10 != 9 )
            {
              sub_144A77850(a1, &v111);
              sub_144A74280(a1);
              v28 = (unsigned int *)*((_QWORD *)&v111 + 1);
              for ( m = (unsigned int *)v111; m != v28; ++m )
              {
                v30 = sub_14454D650(*m);
                v31 = sub_146E8BA20(408);
                v32 = v31;
                if ( v31 )
                {
                  *(_OWORD *)v31 = 0;
                  *(_DWORD *)(v31 + 8) = 1;
                  *(_DWORD *)(v31 + 12) = 1;
                  *(_QWORD *)v31 = off_14A53D1E8;
                  sub_144548F90(v31 + 16, v30, 0);
                }
                else
                {
                  v32 = 0;
                }
                v33 = v4 | 1;
                *(_QWORD *)&v98 = v32 + 16;
                *((_QWORD *)&v98 + 1) = v32;
                v34 = *(_QWORD **)(a1 + 280);
                if ( v34 == *(_QWORD **)(a1 + 288) )
                {
                  sub_1401E7EF0(a1 + 272, v34, &v98);
                }
                else
                {
                  *v34 = 0;
                  v34[1] = 0;
                  *(_OWORD *)v34 = v98;
                  v98 = 0;
                  *(_QWORD *)(a1 + 280) += 16LL;
                }
                v4 = v33 & 0xFFFFFFFE;
                v35 = (volatile signed __int32 *)*((_QWORD *)&v98 + 1);
                if ( *((_QWORD *)&v98 + 1) )
                {
                  if ( _InterlockedExchangeAdd((volatile signed __int32 *)(*((_QWORD *)&v98 + 1) + 8LL), 0xFFFFFFFF) == 1 )
                  {
                    (**(void (__fastcall ***)(volatile signed __int32 *))v35)(v35);
                    if ( _InterlockedExchangeAdd(v35 + 3, 0xFFFFFFFF) == 1 )
                      (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v35 + 8LL))(v35);
                  }
                }
              }
              v36 = *(_QWORD **)(a1 + 272);
              for ( n = *(_QWORD **)(a1 + 280); v36 != n; v36 += 2 )
              {
                v101 = 0;
                v38 = v36[1];
                if ( v38 )
                {
                  _InterlockedIncrement((volatile signed __int32 *)(v38 + 8));
                  v38 = v36[1];
                }
                *(_QWORD *)&v101 = *v36;
                *((_QWORD *)&v101 + 1) = v38;
                sub_144A73A40(a1, &v101);
              }
              sub_144A73DA0(a1);
              v39 = v111;
              if ( (_QWORD)v111 )
              {
                v40 = (v112 - v111) & 0xFFFFFFFFFFFFFFFCuLL;
                if ( v40 >= 0x1000 )
                {
                  v40 += 39LL;
                  v39 = *(_QWORD *)(v111 - 8);
                  if ( (unsigned __int64)(v111 - v39 - 8) > 0x1F )
                    sub_148AAF304(v39, v40);
                }
                sub_146E9F3A0(v39, v40);
                v111 = 0;
                v112 = 0;
              }
            }
            break;
          case 7LL:
          case 15LL:
          case 35LL:
            sub_144A73B60(a1, 3);
            sub_144A73B60(a1, 3);
            break;
          case 11LL:
            sub_144A738F0(a1, 2);
            break;
          case 12LL:
            sub_144A738F0(a1, 3);
            break;
          case 13LL:
            v102 = 0;
            v41 = *(_QWORD *)(a1 + 168);
            if ( v41 )
            {
              _InterlockedIncrement((volatile signed __int32 *)(v41 + 8));
              v41 = *(_QWORD *)(a1 + 168);
            }
            *(_QWORD *)&v102 = *(_QWORD *)(a1 + 160);
            *((_QWORD *)&v102 + 1) = v41;
            sub_144A737D0(a1, &v102);
            v103 = 0;
            v42 = *(_QWORD *)(a1 + 184);
            if ( v42 )
            {
              _InterlockedIncrement((volatile signed __int32 *)(v42 + 8));
              v42 = *(_QWORD *)(a1 + 184);
            }
            *(_QWORD *)&v103 = *(_QWORD *)(a1 + 176);
            *((_QWORD *)&v103 + 1) = v42;
            sub_144A737D0(a1, &v103);
            v104 = 0;
            v43 = *(_QWORD *)(a1 + 200);
            if ( v43 )
            {
              _InterlockedIncrement((volatile signed __int32 *)(v43 + 8));
              v43 = *(_QWORD *)(a1 + 200);
            }
            *(_QWORD *)&v104 = *(_QWORD *)(a1 + 192);
            *((_QWORD *)&v104 + 1) = v43;
            sub_144A737D0(a1, &v104);
            break;
          case 14LL:
            sub_144A73B60(a1, 0);
            break;
          case 16LL:
            sub_144A77850(a1, &v113);
            sub_144A74280(a1);
            v44 = (unsigned int *)*((_QWORD *)&v113 + 1);
            for ( ii = (unsigned int *)v113; ii != v44; ++ii )
            {
              v46 = sub_14454D650(*ii);
              v47 = sub_146E8BA20(408);
              v48 = v47;
              if ( v47 )
              {
                *(_OWORD *)v47 = 0;
                *(_DWORD *)(v47 + 8) = 1;
                *(_DWORD *)(v47 + 12) = 1;
                *(_QWORD *)v47 = off_14A53D1E8;
                sub_144548F90(v47 + 16, v46, 0);
              }
              else
              {
                v48 = 0;
              }
              v49 = v4 | 2;
              *(_QWORD *)&v99 = v48 + 16;
              *((_QWORD *)&v99 + 1) = v48;
              v50 = *(_QWORD **)(a1 + 280);
              if ( v50 == *(_QWORD **)(a1 + 288) )
              {
                sub_1401E7EF0(a1 + 272, v50, &v99);
              }
              else
              {
                *v50 = 0;
                v50[1] = 0;
                *(_OWORD *)v50 = v99;
                v99 = 0;
                *(_QWORD *)(a1 + 280) += 16LL;
              }
              v4 = v49 & 0xFFFFFFFD;
              v51 = (volatile signed __int32 *)*((_QWORD *)&v99 + 1);
              if ( *((_QWORD *)&v99 + 1) )
              {
                if ( _InterlockedExchangeAdd((volatile signed __int32 *)(*((_QWORD *)&v99 + 1) + 8LL), 0xFFFFFFFF) == 1 )
                {
                  (**(void (__fastcall ***)(volatile signed __int32 *))v51)(v51);
                  if ( _InterlockedExchangeAdd(v51 + 3, 0xFFFFFFFF) == 1 )
                    (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v51 + 8LL))(v51);
                }
              }
            }
            v52 = *(_QWORD **)(a1 + 272);
            for ( jj = *(_QWORD **)(a1 + 280); v52 != jj; v52 += 2 )
            {
              v105 = 0;
              v54 = v52[1];
              if ( v54 )
              {
                _InterlockedIncrement((volatile signed __int32 *)(v54 + 8));
                v54 = v52[1];
              }
              *(_QWORD *)&v105 = *v52;
              *((_QWORD *)&v105 + 1) = v54;
              sub_144A73A40(a1, &v105);
            }
            sub_144A73DA0(a1);
            v55 = v113;
            if ( (_QWORD)v113 )
            {
              v56 = (v114 - v113) & 0xFFFFFFFFFFFFFFFCuLL;
              if ( v56 >= 0x1000 )
              {
                v56 += 39LL;
                v55 = *(_QWORD *)(v113 - 8);
                if ( (unsigned __int64)(v113 - v55 - 8) > 0x1F )
                  sub_148AAF304(v55, v56);
              }
              sub_146E9F3A0(v55, v56);
              v113 = 0;
              v114 = 0;
            }
            break;
          case 34LL:
            v106 = 0;
            v57 = *(_QWORD *)(a1 + 216);
            if ( v57 )
            {
              _InterlockedIncrement((volatile signed __int32 *)(v57 + 8));
              v57 = *(_QWORD *)(a1 + 216);
            }
            *(_QWORD *)&v106 = *(_QWORD *)(a1 + 208);
            *((_QWORD *)&v106 + 1) = v57;
            sub_144A737D0(a1, &v106);
            v107 = 0;
            v58 = *(_QWORD *)(a1 + 232);
            if ( v58 )
            {
              _InterlockedIncrement((volatile signed __int32 *)(v58 + 8));
              v58 = *(_QWORD *)(a1 + 232);
            }
            *(_QWORD *)&v107 = *(_QWORD *)(a1 + 224);
            *((_QWORD *)&v107 + 1) = v58;
            sub_144A737D0(a1, &v107);
            v108 = 0;
            v59 = *(_QWORD *)(a1 + 248);
            if ( v59 )
            {
              _InterlockedIncrement((volatile signed __int32 *)(v59 + 8));
              v59 = *(_QWORD *)(a1 + 248);
            }
            *(_QWORD *)&v108 = *(_QWORD *)(a1 + 240);
            *((_QWORD *)&v108 + 1) = v59;
            sub_144A737D0(a1, &v108);
            v109 = 0;
            v60 = *(_QWORD *)(a1 + 264);
            if ( v60 )
            {
              _InterlockedIncrement((volatile signed __int32 *)(v60 + 8));
              v60 = *(_QWORD *)(a1 + 264);
            }
            *(_QWORD *)&v109 = *(_QWORD *)(a1 + 256);
            *((_QWORD *)&v109 + 1) = v60;
            sub_144A737D0(a1, &v109);
            break;
          case 36LL:
            sub_144A779B0(a1, &v115);
            sub_144A74280(a1);
            v61 = (unsigned int *)*((_QWORD *)&v115 + 1);
            for ( kk = (unsigned int *)v115; kk != v61; ++kk )
            {
              v63 = sub_14454D650(*kk);
              v64 = sub_146E8BA20(408);
              v65 = v64;
              if ( v64 )
              {
                *(_OWORD *)v64 = 0;
                *(_DWORD *)(v64 + 8) = 1;
                *(_DWORD *)(v64 + 12) = 1;
                *(_QWORD *)v64 = off_14A53D1E8;
                sub_144548F90(v64 + 16, v63, 0);
              }
              else
              {
                v65 = 0;
              }
              v66 = v4 | 4;
              *(_QWORD *)&v100 = v65 + 16;
              *((_QWORD *)&v100 + 1) = v65;
              v67 = *(_QWORD **)(a1 + 280);
              if ( v67 == *(_QWORD **)(a1 + 288) )
              {
                sub_1401E7EF0(a1 + 272, v67, &v100);
              }
              else
              {
                *v67 = 0;
                v67[1] = 0;
                *(_OWORD *)v67 = v100;
                v100 = 0;
                *(_QWORD *)(a1 + 280) += 16LL;
              }
              v4 = v66 & 0xFFFFFFFB;
              v68 = (volatile signed __int32 *)*((_QWORD *)&v100 + 1);
              if ( *((_QWORD *)&v100 + 1) )
              {
                if ( _InterlockedExchangeAdd((volatile signed __int32 *)(*((_QWORD *)&v100 + 1) + 8LL), 0xFFFFFFFF) == 1 )
                {
                  (**(void (__fastcall ***)(volatile signed __int32 *))v68)(v68);
                  if ( _InterlockedExchangeAdd(v68 + 3, 0xFFFFFFFF) == 1 )
                    (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v68 + 8LL))(v68);
                }
              }
            }
            v69 = *(_QWORD **)(a1 + 272);
            for ( mm = *(_QWORD **)(a1 + 280); v69 != mm; v69 += 2 )
            {
              v110 = 0;
              v71 = v69[1];
              if ( v71 )
              {
                _InterlockedIncrement((volatile signed __int32 *)(v71 + 8));
                v71 = v69[1];
              }
              *(_QWORD *)&v110 = *v69;
              *((_QWORD *)&v110 + 1) = v71;
              sub_144A73A40(a1, &v110);
            }
            sub_144A73DA0(a1);
            v72 = v115;
            if ( (_QWORD)v115 )
            {
              v73 = (v116 - v115) & 0xFFFFFFFFFFFFFFFCuLL;
              if ( v73 >= 0x1000 )
              {
                v73 += 39LL;
                v72 = *(_QWORD *)(v115 - 8);
                if ( (unsigned __int64)(v115 - v72 - 8) > 0x1F )
                  sub_148AAF304(v72, v73);
              }
              sub_146E9F3A0(v72, v73);
              v115 = 0;
              v116 = 0;
            }
            break;
          case 37LL:
            v74 = sub_144A7FCE0();
            sub_144A81A90(v74, 38);
            break;
          case 39LL:
            (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 144) + 32LL))(*(_QWORD *)(a1 + 144), 2);
            v76 = sub_146D74000(v75);
            sub_146D746E0(v76, 475);
            sub_146D75AF0(v78, v77);
            break;
          case 40LL:
            if ( v10 == 10 )
            {
              sub_14668C520(qword_14E683C78, 3904, 0, 0);
              sub_146694510(qword_14E683C78, 3903, -1, 0, 1);
            }
            break;
          default:
            return;
        }
      }
    }
  }
}

