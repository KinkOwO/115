// reader_sub_1441D3B50

__int64 __fastcall sub_1441D3B50(_QWORD *a1, unsigned int a2, unsigned int a3, __int64 a4, __int64 a5)
{
  _QWORD *v6; // r12
  int v7; // r15d
  unsigned int v8; // r13d
  _QWORD *v9; // rsi
  __int64 v10; // r8
  __int64 v11; // rdx
  int v12; // ebx
  _QWORD *v13; // rdi
  _QWORD *v14; // rax
  _QWORD *v15; // rbx
  unsigned int v16; // ecx
  __int64 v17; // r8
  __int64 **v18; // rax
  __int64 i; // rax
  __int64 *j; // rcx
  _QWORD *v21; // rax
  _QWORD *v22; // rbx
  unsigned int v23; // ecx
  __int64 v24; // r8
  __int64 **v25; // rax
  __int64 k; // rax
  __int64 *m; // rcx
  int v28; // esi
  int v29; // edi
  _QWORD *v30; // rbx
  __int64 v31; // rcx
  __int64 v32; // rcx
  int v33; // eax
  __int64 v34; // rax
  __int64 v35; // rax
  __int64 v36; // rax
  int v37; // eax
  __int64 v38; // rax
  _QWORD *v39; // rax
  _QWORD *v40; // rbx
  volatile signed __int32 *v41; // rdi
  __int64 v42; // rsi
  __int64 **v43; // rax
  __int64 n; // rax
  __int64 *ii; // rcx
  __int64 v46; // rax
  __int64 v47; // r8
  __int64 v48; // rax
  __int64 v49; // r8
  __int64 v50; // rdx
  unsigned __int64 v51; // rdx
  __int64 v52; // rcx
  __int64 v53; // rdx
  __int64 v54; // rax
  int v55; // edx
  int v56; // r8d
  _QWORD *v57; // rax
  _QWORD *v58; // rbx
  __int64 v59; // r14
  __int64 v60; // rdi
  unsigned int *v61; // rdx
  int v62; // edi
  __int64 v63; // rdx
  __int64 v64; // rsi
  int v65; // r12d
  __int64 v66; // rcx
  int v67; // ecx
  int v68; // edx
  __int64 v69; // rax
  _QWORD *v70; // rdi
  _QWORD *v71; // rdx
  _QWORD *v72; // rdx
  __int64 v73; // rax
  __int64 v74; // r8
  __int64 v75; // rax
  __int64 v76; // r8
  int v77; // r13d
  __int64 v78; // rdx
  unsigned int v79; // r13d
  __int64 v80; // rax
  int v81; // r8d
  _QWORD *v82; // rdi
  __int64 v83; // rcx
  __int64 v84; // rsi
  void (__fastcall *v85)(__int64, __int64); // rdi
  __int64 v86; // rax
  __int64 v87; // rsi
  void (__fastcall *v88)(__int64, __int64); // rdi
  __int64 v89; // rax
  int v90; // eax
  __int64 **v91; // rax
  __int64 jj; // rax
  __int64 *kk; // rcx
  __int64 v94; // rax
  __int64 *v95; // rcx
  bool v96; // zf
  char v97; // bl
  __int64 v98; // rax
  __int64 v99; // rax
  __int64 v100; // rax
  __int64 v101; // rax
  __int64 v102; // rax
  int v103; // edx
  int v104; // r8d
  __int64 v105; // rdi
  unsigned int v106; // esi
  __int64 v107; // r14
  __int64 v108; // rax
  unsigned int v110; // [rsp+50h] [rbp-B0h]
  _QWORD v112[2]; // [rsp+60h] [rbp-A0h] BYREF
  __int64 v113; // [rsp+70h] [rbp-90h]
  __int128 v114; // [rsp+78h] [rbp-88h] BYREF
  __int64 v115; // [rsp+88h] [rbp-78h]
  __int128 v116; // [rsp+90h] [rbp-70h] BYREF
  __int64 v117; // [rsp+A0h] [rbp-60h]
  unsigned int v118; // [rsp+A8h] [rbp-58h]
  __int64 v119; // [rsp+B0h] [rbp-50h]
  _QWORD *v120; // [rsp+B8h] [rbp-48h]
  __int64 v121; // [rsp+C0h] [rbp-40h]
  _QWORD v122[2]; // [rsp+C8h] [rbp-38h] BYREF
  _QWORD v123[2]; // [rsp+D8h] [rbp-28h] BYREF
  _QWORD v124[2]; // [rsp+E8h] [rbp-18h] BYREF
  __int128 v125; // [rsp+F8h] [rbp-8h] BYREF
  __int128 v126; // [rsp+108h] [rbp+8h] BYREF
  __int64 v127; // [rsp+118h] [rbp+18h]
  __int128 v128; // [rsp+120h] [rbp+20h]
  __int64 v129; // [rsp+130h] [rbp+30h]
  char v130[8]; // [rsp+138h] [rbp+38h] BYREF
  char v131[8]; // [rsp+140h] [rbp+40h] BYREF
  __int64 v132; // [rsp+148h] [rbp+48h]
  _QWORD v133[2]; // [rsp+150h] [rbp+50h] BYREF
  __int64 v134; // [rsp+160h] [rbp+60h]
  unsigned __int64 v135; // [rsp+168h] [rbp+68h]
  _QWORD v136[2]; // [rsp+170h] [rbp+70h] BYREF
  __int64 v137; // [rsp+180h] [rbp+80h]
  unsigned __int64 v138; // [rsp+188h] [rbp+88h]
  _QWORD v139[2]; // [rsp+190h] [rbp+90h] BYREF
  __int64 v140; // [rsp+1A0h] [rbp+A0h]
  unsigned __int64 v141; // [rsp+1A8h] [rbp+A8h]
  _QWORD v142[2]; // [rsp+1B0h] [rbp+B0h] BYREF
  __int64 v143; // [rsp+1C0h] [rbp+C0h]
  unsigned __int64 v144; // [rsp+1C8h] [rbp+C8h]
  __int128 v145; // [rsp+1D0h] [rbp+D0h] BYREF
  __m128i v146; // [rsp+1E0h] [rbp+E0h]
  int v147; // [rsp+1F0h] [rbp+F0h]
  __int128 v148; // [rsp+1F8h] [rbp+F8h] BYREF
  __m128i si128; // [rsp+208h] [rbp+108h]
  int v150; // [rsp+218h] [rbp+118h]
  __int128 v151; // [rsp+220h] [rbp+120h] BYREF
  __int128 v152; // [rsp+230h] [rbp+130h]
  int v153; // [rsp+240h] [rbp+140h]
  _QWORD v154[2]; // [rsp+248h] [rbp+148h] BYREF
  __m128i v155; // [rsp+258h] [rbp+158h]
  _QWORD v156[2]; // [rsp+268h] [rbp+168h] BYREF
  __m128i v157; // [rsp+278h] [rbp+178h]
  _BYTE v158[32]; // [rsp+288h] [rbp+188h] BYREF
  int v159; // [rsp+2A8h] [rbp+1A8h]
  _BYTE v160[40]; // [rsp+2B0h] [rbp+1B0h] BYREF
  _BYTE v161[40]; // [rsp+2D8h] [rbp+1D8h] BYREF

  v129 = -2;
  v121 = a4;
  v118 = a3;
  v6 = a1;
  v7 = 0;
  v8 = 0;
  v110 = 0;
  v9 = a1 - 167;
  v120 = a1 - 167;
  sub_145F6E370(a1 - 167, v112, a2);
  v11 = v112[0];
  if ( !v112[0] )
    goto LABEL_211;
  if ( a3 == 13 )
  {
    v12 = 0;
    v13 = v6 + 419;
    do
    {
      if ( v11 == *v13 )
      {
        sub_1441E1C70(v9, (unsigned int)v12, 0xFFFFFFFFLL);
        v11 = v112[0];
      }
      ++v12;
      v13 += 10;
    }
    while ( v12 < 7 );
    v14 = (_QWORD *)v6[332];
    v15 = (_QWORD *)*v14;
    if ( (_QWORD *)*v14 != v14 )
    {
      do
      {
        if ( v11 == v15[5] )
        {
          switch ( *((_DWORD *)v15 + 8) )
          {
            case 0:
              v16 = 1;
              goto LABEL_25;
            case 1:
              v16 = 3;
              goto LABEL_25;
            case 2:
            case 6:
              v16 = 2;
              goto LABEL_25;
            case 3:
              v16 = 4;
              goto LABEL_25;
            case 4:
            case 5:
            case 7:
            case 8:
              v16 = 5;
              switch ( *((_DWORD *)v15 + 8) )
              {
                case 4:
                  v17 = 0;
                  break;
                case 5:
                  v17 = 1;
                  break;
                case 7:
                  v17 = 2;
                  break;
                case 8:
                  v17 = 3;
                  break;
                default:
                  v17 = 4;
                  break;
              }
              goto LABEL_26;
            case 9:
              v16 = 6;
              goto LABEL_25;
            default:
              v16 = 7;
LABEL_25:
              v17 = 0xFFFFFFFFLL;
LABEL_26:
              sub_1441E1C70(v9, v16, v17);
              v11 = v112[0];
              break;
          }
        }
        v18 = (__int64 **)v15[2];
        if ( *((_BYTE *)v18 + 25) )
        {
          for ( i = v15[1]; !*(_BYTE *)(i + 25); i = *(_QWORD *)(i + 8) )
          {
            if ( v15 != *(_QWORD **)(i + 16) )
              break;
            v15 = (_QWORD *)i;
          }
          v15 = (_QWORD *)i;
        }
        else
        {
          v15 = (_QWORD *)v15[2];
          for ( j = *v18; !*((_BYTE *)j + 25); j = (__int64 *)*j )
            v15 = j;
        }
      }
      while ( v15 != (_QWORD *)v6[332] );
    }
    v21 = (_QWORD *)v6[288];
    v22 = (_QWORD *)*v21;
    if ( (_QWORD *)*v21 != v21 )
    {
      do
      {
        if ( v11 == v22[5] )
        {
          switch ( *((_DWORD *)v22 + 8) )
          {
            case 0:
              v23 = 1;
              goto LABEL_53;
            case 1:
              v23 = 3;
              goto LABEL_53;
            case 2:
            case 6:
              v23 = 2;
              goto LABEL_53;
            case 3:
              v23 = 4;
              goto LABEL_53;
            case 4:
            case 5:
            case 7:
            case 8:
              v23 = 5;
              switch ( *((_DWORD *)v22 + 8) )
              {
                case 4:
                  v24 = 0;
                  break;
                case 5:
                  v24 = 1;
                  break;
                case 7:
                  v24 = 2;
                  break;
                case 8:
                  v24 = 3;
                  break;
                default:
                  v24 = 4;
                  break;
              }
              goto LABEL_54;
            case 9:
              v23 = 6;
              goto LABEL_53;
            default:
              v23 = 7;
LABEL_53:
              v24 = 0xFFFFFFFFLL;
LABEL_54:
              sub_1441E1C70(v9, v23, v24);
              v11 = v112[0];
              break;
          }
        }
        v25 = (__int64 **)v22[2];
        if ( *((_BYTE *)v25 + 25) )
        {
          for ( k = v22[1]; !*(_BYTE *)(k + 25); k = *(_QWORD *)(k + 8) )
          {
            if ( v22 != *(_QWORD **)(k + 16) )
              break;
            v22 = (_QWORD *)k;
          }
          v22 = (_QWORD *)k;
        }
        else
        {
          v22 = (_QWORD *)v22[2];
          for ( m = *v25; !*((_BYTE *)m + 25); m = (__int64 *)*m )
            v22 = m;
        }
      }
      while ( v22 != (_QWORD *)v6[288] );
    }
    v28 = 0;
    v29 = 1;
    v30 = v6 + 355;
    do
    {
      if ( v11 == *(v30 - 10) )
        *((_DWORD *)v6 + 684) = v28;
      if ( v11 == *(v30 - 2) )
      {
        if ( *(_DWORD *)v30 )
        {
          v31 = v6[1534];
          if ( v31 )
          {
            LOBYTE(v11) = 1;
            (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v31 + 24LL))(v31, v11);
          }
        }
        *(_DWORD *)v30 = 0;
        (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)*(v30 - 4) + 16LL))(*(v30 - 4), 0);
        (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)v30[1] + 24LL))(v30[1], 0);
        (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)v30[1] + 16LL))(v30[1], 0);
        sub_146EECBB0(*(v30 - 6), (unsigned int)v29);
        v11 = v112[0];
      }
      ++v28;
      v29 += 2;
      v30 += 15;
    }
    while ( v29 < 9 );
    v32 = qword_14E683C78;
    if ( v11 == v6[405] && qword_14E683C78 )
    {
      LOBYTE(v10) = 1;
      if ( !(unsigned __int8)sub_146682140(qword_14E683C78, 733, v10) )
      {
        v33 = sub_14668C520(qword_14E683C78, 733, 0, 0);
        v34 = sub_148AA307C(v33, 0, (unsigned int)&off_14DCB4760, (unsigned int)&off_14DCB6A70, 0);
        if ( v34 )
        {
          *(_DWORD *)(v34 + 1512) = 0;
          sub_1441D9D90(v34);
        }
      }
      v32 = qword_14E683C78;
      v11 = v112[0];
    }
    v35 = v6[409];
    if ( v35 && v11 == v35 || (v36 = v6[407]) != 0 && v11 == v36 )
    {
      if ( v32 )
      {
        LOBYTE(v10) = 1;
        if ( !(unsigned __int8)sub_146682140(v32, 733, v10) )
        {
          v37 = sub_14668C520(qword_14E683C78, 733, 0, 0);
          v38 = sub_148AA307C(v37, 0, (unsigned int)&off_14DCB4760, (unsigned int)&off_14DCB6A70, 0);
          if ( v38 )
          {
            *(_DWORD *)(v38 + 1512) = 1;
            sub_1441D9D90(v38);
          }
        }
      }
    }
    goto LABEL_209;
  }
  if ( a3 != 12 )
    goto LABEL_209;
  v39 = (_QWORD *)v6[288];
  v40 = (_QWORD *)*v39;
  if ( (_QWORD *)*v39 != v39 )
  {
    while ( 1 )
    {
      v128 = 0;
      v41 = (volatile signed __int32 *)v40[6];
      if ( v41 )
      {
        _InterlockedIncrement(v41 + 2);
        v41 = (volatile signed __int32 *)v40[6];
      }
      v42 = v40[5];
      *(_QWORD *)&v128 = v42;
      *((_QWORD *)&v128 + 1) = v41;
      if ( v42 == v112[0] )
      {
        if ( sub_1444EBAB0(*(_DWORD *)v6[3 * *((int *)v40 + 8) + 252], v11, v10) )
          break;
      }
      if ( v41 )
      {
        if ( _InterlockedExchangeAdd(v41 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v41)(v41);
          if ( _InterlockedExchangeAdd(v41 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v41 + 8LL))(v41);
        }
      }
      v43 = (__int64 **)v40[2];
      if ( *((_BYTE *)v43 + 25) )
      {
        for ( n = v40[1]; !*(_BYTE *)(n + 25); n = *(_QWORD *)(n + 8) )
        {
          if ( v40 != *(_QWORD **)(n + 16) )
            break;
          v40 = (_QWORD *)n;
        }
        v40 = (_QWORD *)n;
      }
      else
      {
        v40 = (_QWORD *)v40[2];
        for ( ii = *v43; !*((_BYTE *)ii + 25); ii = (__int64 *)*ii )
          v40 = ii;
      }
      if ( v40 == (_QWORD *)v6[288] )
      {
        v8 = 0;
        goto LABEL_127;
      }
    }
    (*(void (__fastcall **)(_QWORD))(*(_QWORD *)v6[413] + 688LL))(v6[413]);
    (*(void (__fastcall **)(_QWORD))(*(_QWORD *)v6[415] + 688LL))(v6[415]);
    v114 = 0;
    v115 = 0;
    v142[0] = 0;
    v143 = 0;
    v144 = 7;
    v139[0] = 0;
    v140 = 0;
    v141 = 7;
    v46 = sub_146EF69A0(v6[413]);
    v47 = -1;
    do
      ++v47;
    while ( *(_WORD *)(v46 + 2 * v47) );
    sub_14014C8D0(v142, v46);
    v48 = sub_146EF69A0(v6[415]);
    v49 = -1;
    do
      ++v49;
    while ( *(_WORD *)(v48 + 2 * v49) );
    sub_14014C8D0(v139, v48);
    sub_14014C810(&v148);
    v150 = dword_14F1C089C;
    v50 = *((_QWORD *)&v114 + 1);
    if ( *((_QWORD *)&v114 + 1) == v115 )
    {
      sub_1402AEE00(&v114, *((_QWORD *)&v114 + 1), &v148);
    }
    else
    {
      v113 = *((_QWORD *)&v114 + 1);
      **((_QWORD **)&v114 + 1) = 0;
      *(_QWORD *)(v50 + 16) = 0;
      *(_QWORD *)(v50 + 24) = 0;
      *(_OWORD *)v50 = v148;
      *(__m128i *)(v50 + 16) = si128;
      si128 = _mm_load_si128(xmmword_1491AB7C0);
      LOWORD(v148) = 0;
      *(_DWORD *)(v50 + 32) = v150;
      *((_QWORD *)&v114 + 1) += 40LL;
    }
    if ( si128.m128i_i64[1] >= 8uLL )
    {
      v51 = 2 * si128.m128i_i64[1] + 2;
      v52 = v148;
      if ( v51 >= 0x1000 )
      {
        v51 = 2 * si128.m128i_i64[1] + 41;
        v52 = *(_QWORD *)(v148 - 8);
        if ( (unsigned __int64)(v148 - v52 - 8) > 0x1F )
          sub_148AAF304(v52, v51);
      }
      sub_146E9F3A0(v52, v51);
    }
    si128 = _mm_load_si128(xmmword_1491AB7C0);
    LOWORD(v148) = 0;
    sub_14014C810(&v145);
    v147 = dword_14F1C08A8;
    v53 = *((_QWORD *)&v114 + 1);
    if ( *((_QWORD *)&v114 + 1) == v115 )
    {
      sub_1402AEE00(&v114, *((_QWORD *)&v114 + 1), &v145);
    }
    else
    {
      v113 = *((_QWORD *)&v114 + 1);
      **((_QWORD **)&v114 + 1) = 0;
      *(_QWORD *)(v53 + 16) = 0;
      *(_QWORD *)(v53 + 24) = 0;
      *(_OWORD *)v53 = v145;
      *(__m128i *)(v53 + 16) = v146;
      v146 = _mm_load_si128(xmmword_1491AB7C0);
      LOWORD(v145) = 0;
      *(_DWORD *)(v53 + 32) = v147;
      *((_QWORD *)&v114 + 1) += 40LL;
    }
    v8 = 0;
    v110 = 0;
    if ( v146.m128i_i64[1] >= 8uLL )
      sub_14014CBC0(&v145, v145, v146.m128i_i64[1] + 1);
    v146 = _mm_load_si128(xmmword_1491AB7C0);
    LOWORD(v145) = 0;
    v54 = sub_146E8C7D0(&unk_14A232098);
    sub_146EC8E30(v42, v122, v54);
    sub_142757420(v122[0]);
    sub_146ECA0C0(v122[0]);
    sub_14555F4C0(
      (_DWORD)v6 + *(_DWORD *)(*(v6 - 163) + 4LL) - 1304,
      v55,
      v56,
      2,
      (__int64)&v114,
      0,
      0,
      1153957888,
      0,
      0);
    sub_1401566D0(v122);
    if ( v141 >= 8 )
      sub_14014CBC0(v139, v139[0], v141 + 1);
    v140 = 0;
    v141 = 7;
    LOWORD(v139[0]) = 0;
    if ( v144 >= 8 )
      sub_14014CBC0(v142, v142[0], v144 + 1);
    v143 = 0;
    v144 = 7;
    LOWORD(v142[0]) = 0;
    sub_1401516C0(&v114);
    if ( v41 )
      sub_1401DC510(v41);
LABEL_127:
    v9 = v6 - 167;
  }
  v57 = (_QWORD *)v6[332];
  v58 = (_QWORD *)*v57;
  if ( (_QWORD *)*v57 == v57 )
    goto LABEL_209;
  while ( 1 )
  {
    v125 = 0;
    v59 = v58[6];
    if ( v59 )
    {
      _InterlockedIncrement((volatile signed __int32 *)(v59 + 8));
      v59 = v58[6];
      v8 = v110;
    }
    v113 = v58[5];
    *(_QWORD *)&v125 = v113;
    *((_QWORD *)&v125 + 1) = v59;
    if ( v113 != v112[0] )
    {
LABEL_132:
      if ( v59 )
        sub_1401DC510(v59);
      goto LABEL_192;
    }
    v60 = *((int *)v58 + 8);
    if ( (_DWORD)v60 == 3 )
    {
      if ( *(_DWORD *)sub_1441C3DB0(v9, v130) == 3 )
        goto LABEL_132;
LABEL_136:
      v61 = (unsigned int *)v6[3 * v60 + 252];
      goto LABEL_137;
    }
    if ( (_DWORD)v60 == 9 )
    {
      if ( *(_DWORD *)sub_1441C3DB0(v9, v131) == 9 )
        goto LABEL_132;
      goto LABEL_136;
    }
    if ( (_DWORD)v60 != 1 )
      goto LABEL_136;
    v61 = (unsigned int *)(v6[255] + 4LL * *((int *)v6 + 3020));
LABEL_137:
    v62 = *v61;
    v64 = sub_1444EBAB0(*v61, (__int64)v61, v10);
    v119 = v64;
    if ( *((_DWORD *)v58 + 8) == 4 )
    {
      v64 = sub_1444EBAB0(0x9C40u, v63, v10);
      v119 = v64;
    }
    v65 = v62;
    if ( v64 )
      break;
    if ( v62 <= 0 )
    {
      v66 = a1[30];
      if ( v66 )
      {
        if ( (unsigned __int8)sub_141FB6530(v66) )
          v65 = sub_1421B2820(a1[30]);
      }
    }
    v67 = *((_DWORD *)v58 + 8);
    v68 = v67;
    if ( v67 == 9 )
    {
      LOBYTE(v10) = 1;
      v69 = sub_140283D60(qword_14E683B38, (unsigned int)v65, v10);
      v116 = 0;
      v117 = 0;
      v136[0] = 0;
      v137 = 0;
      v138 = 7;
      v133[0] = 0;
      v134 = 0;
      v135 = 7;
      if ( v69 )
      {
        v70 = (_QWORD *)(v69 + 496);
        if ( v136 != (_QWORD *)(v69 + 496) )
        {
          v71 = (_QWORD *)(v69 + 496);
          if ( *(_QWORD *)(v69 + 520) >= 8u )
            v71 = (_QWORD *)*v70;
          sub_14014C8D0(v136, v71);
        }
        if ( v133 == v70 )
          goto LABEL_165;
        if ( v70[3] >= 8u )
          v70 = (_QWORD *)*v70;
        v72 = v70;
      }
      else
      {
        v73 = sub_14723C170(101036695);
        v74 = -1;
        do
          ++v74;
        while ( *(_WORD *)(v73 + 2 * v74) );
        sub_14014C8D0(v136, v73);
        v75 = sub_14723C170(101036704);
        v76 = -1;
        do
          ++v76;
        while ( *(_WORD *)(v75 + 2 * v76) );
        v72 = (_QWORD *)v75;
      }
      sub_14014C8D0(v133, v72);
LABEL_165:
      sub_14014C810(&v151);
      v153 = dword_14F1C089C;
      v77 = v8 | 4;
      v78 = *((_QWORD *)&v116 + 1);
      if ( *((_QWORD *)&v116 + 1) == v117 )
      {
        sub_1402AEE00(&v116, *((_QWORD *)&v116 + 1), &v151);
      }
      else
      {
        v132 = *((_QWORD *)&v116 + 1);
        **((_QWORD **)&v116 + 1) = 0;
        *(_QWORD *)(v78 + 16) = 0;
        *(_QWORD *)(v78 + 24) = 0;
        *(_OWORD *)v78 = v151;
        *(_OWORD *)(v78 + 16) = v152;
        *(_QWORD *)&v152 = 0;
        *((_QWORD *)&v152 + 1) = 7;
        LOWORD(v151) = 0;
        *(_DWORD *)(v78 + 32) = v153;
        *((_QWORD *)&v116 + 1) += 40LL;
      }
      sub_14014C710(&v151);
      sub_14014C810(v158);
      v159 = dword_14F1C08A8;
      v79 = v77 & 0xFFFFFFF3 | 8;
      if ( *((_QWORD *)&v116 + 1) == v117 )
      {
        sub_1402AEE00(&v116, *((_QWORD *)&v116 + 1), v158);
      }
      else
      {
        sub_14024D5C0(*((_QWORD *)&v116 + 1), v158);
        *((_QWORD *)&v116 + 1) += 40LL;
      }
      v8 = v79 & 0xFFFFFFF7;
      v110 = v8;
      sub_14014C710(v158);
      v80 = sub_146E8C7D0(&unk_14A232098);
      sub_146EC8E30(v113, v123, v80);
      if ( v123[0] )
      {
        sub_142757420(v123[0]);
        sub_146ECA0C0(v123[0]);
        sub_14555F4C0(
          (_DWORD)a1 - 1304 + *(_DWORD *)(*(a1 - 163) + 4LL),
          (_DWORD)a1 - 1304,
          v81,
          1,
          (__int64)&v116,
          0,
          0,
          1153957888,
          0,
          0);
      }
      sub_1401566D0(v123);
      if ( v135 >= 8 )
        sub_14014CBC0(v133, v133[0], v135 + 1);
      v134 = 0;
      v135 = 7;
      LOWORD(v133[0]) = 0;
      if ( v138 >= 8 )
        sub_14014CBC0(v136, v136[0], v138 + 1);
      v137 = 0;
      v138 = 7;
      LOWORD(v136[0]) = 0;
      sub_1401516C0(&v116);
      v67 = *((_DWORD *)v58 + 8);
      v68 = v67;
    }
    if ( v67 == 3 )
      goto LABEL_181;
LABEL_189:
    if ( v59 )
      sub_1401DC510(v59);
    v6 = a1;
LABEL_192:
    v91 = (__int64 **)v58[2];
    if ( *((_BYTE *)v91 + 25) )
    {
      for ( jj = v58[1]; !*(_BYTE *)(jj + 25); jj = *(_QWORD *)(jj + 8) )
      {
        if ( v58 != *(_QWORD **)(jj + 16) )
          break;
        v58 = (_QWORD *)jj;
      }
      v58 = (_QWORD *)jj;
    }
    else
    {
      v58 = (_QWORD *)v58[2];
      for ( kk = *v91; !*((_BYTE *)kk + 25); kk = (__int64 *)*kk )
        v58 = kk;
    }
    if ( v58 == (_QWORD *)v6[332] )
      goto LABEL_209;
    v9 = v6 - 167;
  }
  v68 = *((_DWORD *)v58 + 8);
LABEL_181:
  v82 = a1;
  if ( v68 == 3 )
  {
    v83 = a1[334];
    if ( v83 )
    {
      if ( !(unsigned __int8)sub_141FB6530(v83) )
      {
        v84 = a1[413];
        v85 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v84 + 688LL);
        v86 = sub_14723C170(100002266);
        v85(v84, v86);
        v87 = a1[415];
        v88 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v87 + 688LL);
        v89 = sub_14723C170(100002267);
        v88(v87, v89);
        v64 = v119;
        v82 = a1;
      }
    }
  }
  if ( !v64 )
    goto LABEL_205;
  v90 = *(_DWORD *)(v64 + 8);
  if ( v90 == 4 )
  {
    if ( v65 > 0 )
    {
      LOBYTE(v10) = 1;
      if ( sub_14021BE90(qword_14E683B30, (unsigned int)v65, v10) )
      {
        v94 = *(_QWORD *)v82[413];
        v95 = (__int64 *)v82[413];
LABEL_203:
        (*(void (__fastcall **)(__int64 *))(v94 + 688))(v95);
        (*(void (__fastcall **)(_QWORD, char *))(*(_QWORD *)v82[415] + 688LL))(v82[415], &byte_14BAF7F08);
        v97 = 1;
        goto LABEL_206;
      }
    }
    goto LABEL_189;
  }
  v95 = (__int64 *)v82[413];
  v96 = v90 == 7;
  v94 = *v95;
  if ( v96 )
    goto LABEL_203;
  (*(void (**)(void))(v94 + 688))();
  (*(void (__fastcall **)(_QWORD))(*(_QWORD *)v82[415] + 688LL))(v82[415]);
LABEL_205:
  v97 = 0;
LABEL_206:
  v126 = 0;
  v127 = 0;
  v156[0] = 0;
  v157 = _mm_load_si128(xmmword_1491AB7C0);
  v154[0] = 0;
  v155 = v157;
  v98 = sub_146EF69A0(v82[413]);
  sub_14014C8B0(v156, v98);
  v99 = sub_146EF69A0(v82[415]);
  sub_14014C8B0(v154, v99);
  v100 = sub_14275BD60(v160, v156, &dword_14F1C089C);
  sub_140675EA0(&v126, v100);
  sub_1401512E0(v160);
  if ( !v97 )
  {
    v101 = sub_14275BD60(v161, v154, &dword_14F1C08A8);
    sub_140675EA0(&v126, v101);
    sub_1401512E0(v161);
  }
  v102 = sub_146E8C7D0(&unk_14A232098);
  sub_146EC8E30(v113, v124, v102);
  sub_142757420(v124[0]);
  sub_146ECA0C0(v124[0]);
  sub_14555F4C0(
    (_DWORD)v82 + *(_DWORD *)(*(v82 - 163) + 4LL) - 1304,
    v103,
    v104,
    2,
    (__int64)&v126,
    0,
    0,
    1153957888,
    0,
    0);
  sub_1401566D0(v124);
  sub_14014C710(v154);
  sub_14014C710(v156);
  sub_1401516C0(&v126);
  sub_1401566D0(&v125);
LABEL_209:
  v105 = (__int64)v120;
  v106 = v118;
  v107 = v121;
  do
  {
    v108 = sub_1441C4080(v105, v7);
    (*(void (__fastcall **)(__int64, _QWORD *, _QWORD, __int64, __int64))(*(_QWORD *)v108 + 40LL))(
      v108,
      v112,
      v106,
      v107,
      a5);
    ++v7;
  }
  while ( v7 < 7 );
LABEL_211:
  sub_1401566D0(v112);
  return 0;
}

