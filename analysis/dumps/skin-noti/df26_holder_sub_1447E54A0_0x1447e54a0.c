// holder_sub_1447E54A0_0x1447e54a0

_QWORD **__fastcall sub_1447E54A0(__int64 a1, float a2)
{
  int v4; // r12d
  __int64 v5; // r8
  __int64 v6; // rax
  _QWORD *v7; // r8
  _QWORD **result; // rax
  _QWORD *v9; // r14
  __int64 (__fastcall *v10)(__int64, int **, __int64); // rbx
  __int64 v11; // rax
  int *v12; // rcx
  __int128 v13; // rcx
  unsigned int v14; // r13d
  float v15; // xmm0_4
  float v16; // xmm1_4
  float *v17; // rax
  int v18; // eax
  int v19; // ecx
  unsigned int v20; // ebx
  __int64 v21; // rsi
  __int64 *v22; // rbx
  int v23; // edi
  _QWORD *v24; // rax
  __int64 v25; // rcx
  bool v26; // zf
  __int64 v27; // rcx
  float v28; // xmm6_4
  __int64 v29; // rcx
  float v30; // xmm6_4
  __int64 v31; // rcx
  float v32; // xmm6_4
  __int64 v33; // rcx
  int v34; // ecx
  __int64 v35; // rax
  __int64 v36; // rcx
  __int64 v37; // rax
  void (__fastcall ***v38)(_QWORD); // rcx
  int v39; // r13d
  __int64 v40; // rdi
  _QWORD *v41; // r8
  _QWORD *v42; // rax
  int *v43; // rcx
  int v44; // edi
  __int64 v45; // rax
  __int64 v46; // rcx
  unsigned __int64 v47; // rdx
  int *v48; // rcx
  _DWORD *v49; // rsi
  int v50; // ecx
  int v51; // ecx
  int v52; // ecx
  int v53; // ecx
  _DWORD **v54; // rax
  int *v55; // rcx
  __int128 v56; // xmm0
  _DWORD **v57; // rax
  int *v58; // rcx
  _DWORD **v59; // rax
  int *v60; // rcx
  _DWORD **v61; // rax
  int *v62; // rcx
  __int64 v63; // rdi
  int v64; // r10d
  float v65; // xmm0_4
  int v66; // eax
  int v67; // r8d
  float v68; // xmm0_4
  int v69; // edx
  float v70; // xmm0_4
  int v71; // ecx
  float v72; // xmm0_4
  unsigned int v73; // eax
  int v74; // edx
  int v75; // ecx
  _QWORD *v76; // rax
  _DWORD *v77; // rdx
  _DWORD *v78; // rcx
  int *v79; // rcx
  __int64 v80; // rcx
  _DWORD **v81; // rax
  int *v82; // rcx
  __int64 v83; // rcx
  __int64 v84; // rax
  void (__fastcall ***v85)(_QWORD); // rcx
  __int64 **v86; // rbx
  int *v87; // rcx
  int v88; // eax
  __int64 v89; // rcx
  __int64 v90; // rax
  void (__fastcall ***v91)(_QWORD); // rcx
  __int64 *v92; // rbx
  char *v93; // rdi
  _QWORD *v94; // rax
  __int64 v95; // rdx
  float v96; // xmm6_4
  __int64 v97; // rcx
  float v98; // xmm6_4
  __int64 v99; // rcx
  __int64 v100; // rax
  __int64 v101; // rcx
  __int64 v102; // rax
  void (__fastcall ***v103)(_QWORD); // rcx
  int v104; // eax
  int v105; // r13d
  __int64 v106; // rdi
  _QWORD *v107; // r8
  _QWORD *v108; // rax
  int *v109; // rcx
  int v110; // edi
  __int64 v111; // rax
  unsigned __int64 v112; // rdx
  __int64 v113; // rcx
  int *v114; // rcx
  _DWORD *v115; // rbx
  _DWORD *v116; // rdi
  int v117; // ecx
  int v118; // edx
  _DWORD **v119; // rax
  int *v120; // rcx
  __int64 v121; // rcx
  _DWORD **v122; // rax
  int *v123; // rcx
  int *v124; // rcx
  int v125; // eax
  __int64 *v126; // rbx
  __int64 *i; // rdi
  int *v128; // rdx
  int *v129; // r8
  int v130; // ecx
  int v131; // ecx
  __int64 v132; // rcx
  __int64 v133; // rax
  int *v134; // [rsp+30h] [rbp-D8h] BYREF
  int *v135; // [rsp+38h] [rbp-D0h] BYREF
  int *v136; // [rsp+40h] [rbp-C8h] BYREF
  __int64 v137; // [rsp+48h] [rbp-C0h]
  _QWORD *v138; // [rsp+50h] [rbp-B8h]
  int *v139; // [rsp+58h] [rbp-B0h] BYREF
  __int64 v140; // [rsp+60h] [rbp-A8h]
  int *v141; // [rsp+68h] [rbp-A0h] BYREF
  __int64 v142; // [rsp+70h] [rbp-98h]
  int *v143; // [rsp+78h] [rbp-90h] BYREF
  __int64 v144; // [rsp+80h] [rbp-88h]
  __int64 v145; // [rsp+88h] [rbp-80h]
  _DWORD *v146; // [rsp+90h] [rbp-78h]
  unsigned __int64 v147; // [rsp+98h] [rbp-70h] BYREF
  int *v148; // [rsp+A0h] [rbp-68h] BYREF
  unsigned __int64 v149; // [rsp+A8h] [rbp-60h] BYREF
  int *v150; // [rsp+B0h] [rbp-58h] BYREF
  __int128 v151; // [rsp+B8h] [rbp-50h]
  __int64 v152; // [rsp+C8h] [rbp-40h]
  _QWORD *v153; // [rsp+D0h] [rbp-38h]
  __int64 v154; // [rsp+D8h] [rbp-30h]
  _QWORD *v155; // [rsp+E0h] [rbp-28h]
  __int64 v156; // [rsp+E8h] [rbp-20h]
  __int64 v157; // [rsp+F0h] [rbp-18h]
  _DWORD *v158; // [rsp+F8h] [rbp-10h]
  _QWORD v159[2]; // [rsp+100h] [rbp-8h] BYREF
  _QWORD v160[2]; // [rsp+110h] [rbp+8h] BYREF
  _QWORD v161[2]; // [rsp+120h] [rbp+18h] BYREF
  int *v162; // [rsp+130h] [rbp+28h] BYREF
  int **v163; // [rsp+138h] [rbp+30h]
  _QWORD *v164; // [rsp+140h] [rbp+38h]
  _QWORD *v165; // [rsp+148h] [rbp+40h]
  _QWORD *v166; // [rsp+150h] [rbp+48h]
  __int64 v167; // [rsp+158h] [rbp+50h]
  _QWORD *v168; // [rsp+1D8h] [rbp+D0h] BYREF
  int *v169; // [rsp+1E8h] [rbp+E0h] BYREF
  _DWORD *v170; // [rsp+1F0h] [rbp+E8h]

  v167 = -2;
  v4 = 0;
  sub_1447E3940(a1 + 120, *(_QWORD *)(a1 + 120));
  **(_QWORD **)(a1 + 120) = *(_QWORD *)(a1 + 120);
  *(_QWORD *)(*(_QWORD *)(a1 + 120) + 8LL) = *(_QWORD *)(a1 + 120);
  *(_QWORD *)(a1 + 128) = 0;
  v138 = 0;
  LOBYTE(v5) = 1;
  v6 = sub_140283D60(qword_14E683BF8, *(unsigned int *)(a1 + 104), v5);
  v137 = v6;
  if ( v6 && (v7 = *(_QWORD **)(v6 + 656), (__int64)(*(_QWORD *)(v6 + 664) - (_QWORD)v7) >> 5) )
  {
    if ( v7[3] >= 8u )
      v7 = (_QWORD *)*v7;
    result = (_QWORD **)(*(__int64 (__fastcall **)(__int64, int **, _QWORD *))(*(_QWORD *)qword_14F1C39C8 + 16LL))(
                          qword_14F1C39C8,
                          &v169,
                          v7);
    v9 = *result;
    *result = 0;
    v168 = 0;
    v138 = v9;
  }
  else
  {
    v10 = *(__int64 (__fastcall **)(__int64, int **, __int64))(*(_QWORD *)qword_14F1C39C8 + 16LL);
    v11 = sub_146E8C7D0(&unk_14A3F4690);
    result = (_QWORD **)v10(qword_14F1C39C8, &v169, v11);
    v9 = *result;
    *result = 0;
    v168 = 0;
    v138 = v9;
  }
  v12 = v169;
  if ( v169 )
  {
    --v169[2];
    if ( v12[2] <= 0 )
      result = (_QWORD **)(*(__int64 (__fastcall **)(int *))(*(_QWORD *)v12 + 8LL))(v12);
  }
  if ( v9 )
  {
    *(_QWORD *)&v13 = v9[4];
    v151 = 0;
    *((_QWORD *)&v13 + 1) = v9[5];
    if ( *((_QWORD *)&v13 + 1) )
    {
      _InterlockedIncrement((volatile signed __int32 *)(*((_QWORD *)&v13 + 1) + 8LL));
      *((_QWORD *)&v13 + 1) = v9[5];
      v9 = v138;
    }
    v151 = v13;
    sub_1401B7010(v13, 2);
    v14 = 0;
    if ( *((_QWORD *)&v13 + 1) )
    {
      if ( _InterlockedExchangeAdd((volatile signed __int32 *)(*((_QWORD *)&v13 + 1) + 8LL), 0xFFFFFFFF) == 1 )
      {
        (***((void (__fastcall ****)(_QWORD))&v13 + 1))(*((_QWORD *)&v13 + 1));
        if ( _InterlockedExchangeAdd((volatile signed __int32 *)(*((_QWORD *)&v13 + 1) + 12LL), 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(_QWORD))(**((_QWORD **)&v13 + 1) + 8LL))(*((_QWORD *)&v13 + 1));
      }
      v14 = 0;
      v9 = v138;
    }
    v15 = *(float *)(a1 + 252) - a2;
    *(float *)(a1 + 252) = v15;
    if ( v15 < 0.0 )
    {
      v16 = *(float *)(a1 + 248);
      if ( v16 != 1.0 )
      {
        LODWORD(v168) = 1008981770;
        *(float *)&v169 = v16 - 1.0;
        v17 = (float *)&v168;
        if ( (float)(v16 - 1.0) >= 0.0099999998 )
          v17 = (float *)&v169;
        *(float *)(a1 + 248) = v16 - (float)((float)(a2 * *v17) * 15.0);
      }
    }
    if ( *(float *)(a1 + 248) < 1.0 )
      *(_DWORD *)(a1 + 248) = 1065353216;
    v18 = *(_DWORD *)(a1 + 296);
    v19 = v18 - *(_DWORD *)(a1 + 260);
    *(_DWORD *)(a1 + 92) = 0;
    v20 = 0;
    LODWORD(v168) = 0;
    if ( v18 )
    {
      v156 = v19;
      v21 = 0;
      v163 = &v141;
      v22 = (__int64 *)(a1 + 304);
      v157 = -(__int64)v19;
      do
      {
        v23 = 0;
        if ( (*(_DWORD *)(a1 + 180) & 2) != 0 && !(unsigned __int8)sub_146E9FA80(a1 + 176) )
        {
          if ( v156 > v21 )
          {
            v32 = 1.0 - sub_146E9FA10(a1 + 176);
            v23 = (int)(float)((float)((int)sub_146E9BA90(v33) % 8) * v32);
          }
          else
          {
            v24 = (_QWORD *)(a1 + 264);
            if ( *(_QWORD *)(a1 + 288) >= 8u )
              v24 = (_QWORD *)*v24;
            v25 = a1 + 304;
            if ( *(_QWORD *)(a1 + 328) >= 8u )
              v25 = *v22;
            v26 = *(_WORD *)(v25 + 2 * v21) == *((_WORD *)v24 + v21 + v157);
            v27 = a1 + 176;
            if ( v26 )
            {
              v30 = 1.0 - sub_146E9FA10(v27);
              v23 = (int)(float)((float)((int)sub_146E9BA90(v31) % 5 - 2) * v30);
            }
            else
            {
              v28 = 1.0 - sub_146E9FA10(v27);
              v23 = (int)(float)((float)((int)sub_146E9BA90(v29) % 8) * v28);
            }
          }
        }
        if ( v137 )
          v34 = *(_DWORD *)(*(_QWORD *)(v137 + 632) + 4LL);
        else
          v34 = 0;
        v35 = a1 + 304;
        if ( *(_QWORD *)(a1 + 328) >= 8u )
          v35 = *v22;
        (*(void (__fastcall **)(_QWORD *, int **, _QWORD))(*v9 + 120LL))(
          v9,
          &v134,
          *(unsigned __int16 *)(v35 + 2 * v21) - 48 + v34);
        v36 = qword_14E63AE60;
        if ( !qword_14E63AE60 )
        {
          v37 = sub_146E8BA20(336);
          v164 = (_QWORD *)v37;
          if ( v37 )
            v38 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v37);
          else
            v38 = 0;
          qword_14E63AE60 = (__int64)v38;
          (**v38)(v38);
          v36 = qword_14E63AE60;
        }
        LODWORD(v169) = -(v23 + sub_1447EAC20(v36, *(unsigned int *)(a1 + 104)));
        LODWORD(v144) = *(_DWORD *)(a1 + 92);
        HIDWORD(v144) = (_DWORD)v169;
        v140 = v144;
        v141 = v134;
        if ( v134 )
          ++v134[2];
        v39 = v14 | 4;
        v40 = *(_QWORD *)(a1 + 120);
        if ( *(_QWORD *)(a1 + 128) == 0x7FFFFFFFFFFFFFFLL )
          sub_14883BB54("list too long");
        v152 = a1 + 120;
        v153 = 0;
        v41 = (_QWORD *)sub_146E8BA20(32);
        v153 = v41;
        v165 = v41 + 2;
        v41[2] = v140;
        v166 = v41 + 3;
        v41[3] = v141;
        v141 = 0;
        ++*(_QWORD *)(a1 + 128);
        v42 = *(_QWORD **)(v40 + 8);
        *v41 = v40;
        v41[1] = v42;
        v153 = 0;
        *(_QWORD *)(v40 + 8) = v41;
        *v42 = v41;
        v14 = v39 & 0xFFFFFFFB;
        *(_QWORD *)&v151 = &v141;
        v43 = v141;
        if ( v141 )
        {
          --v141[2];
          if ( v43[2] <= 0 )
            (*(void (__fastcall **)(int *))(*(_QWORD *)v43 + 8LL))(v43);
        }
        v44 = 0;
        v45 = a1 + 304;
        if ( *(_QWORD *)(a1 + 328) >= 8u )
          v45 = *v22;
        if ( v137 )
        {
          v46 = *(_QWORD *)(v137 + 632);
          v47 = *(unsigned __int16 *)(v45 + 2 * v21) - 40LL;
          if ( (*(_QWORD *)(v137 + 640) - v46) >> 2 > v47 )
            v44 = *(_DWORD *)(v46 + 4 * v47) + 1;
        }
        *(_DWORD *)(a1 + 92) += v44 + (*(__int64 (__fastcall **)(int *))(*(_QWORD *)v134 + 48LL))(v134) - 1;
        v48 = v134;
        if ( v134 )
        {
          --v134[2];
          if ( v48[2] <= 0 )
            (*(void (__fastcall **)(int *))(*(_QWORD *)v48 + 8LL))(v48);
        }
        LODWORD(v168) = (_DWORD)v168 + 1;
        ++v21;
      }
      while ( (unsigned int)v168 < *(_DWORD *)(a1 + 296) );
      v20 = (unsigned int)v169;
    }
    v49 = 0;
    v170 = 0;
    v50 = *(_DWORD *)(a1 + 216);
    if ( v50 )
    {
      v51 = v50 - 1;
      if ( v51 )
      {
        v52 = v51 - 1;
        if ( v52 )
        {
          v53 = v52 - 1;
          if ( v53 )
          {
            if ( v53 != 1 )
            {
LABEL_87:
              v63 = v137;
              if ( v137 )
              {
                v64 = *(_DWORD *)(v137 + 616);
                if ( v64 >= 0 )
                {
                  v65 = *(float *)(a1 + 232);
                  v66 = 255;
                  if ( v65 < 1.0 )
                  {
                    if ( v65 > 0.0 )
                      v67 = (int)(float)((float)(v65 * 255.0) + 0.5);
                    else
                      v67 = 0;
                  }
                  else
                  {
                    v67 = 255;
                  }
                  v68 = *(float *)(a1 + 236);
                  if ( v68 < 1.0 )
                  {
                    if ( v68 > 0.0 )
                      v69 = (int)(float)((float)(v68 * 255.0) + 0.5);
                    else
                      v69 = 0;
                  }
                  else
                  {
                    v69 = 255;
                  }
                  v70 = *(float *)(a1 + 240);
                  if ( v70 < 1.0 )
                  {
                    if ( v70 > 0.0 )
                      v71 = (int)(float)((float)(v70 * 255.0) + 0.5);
                    else
                      v71 = 0;
                  }
                  else
                  {
                    v71 = 255;
                  }
                  v72 = *(float *)(a1 + 244);
                  if ( v72 < 1.0 )
                  {
                    if ( v72 > 0.0 )
                      v66 = (int)(float)((float)(v72 * 255.0) + 0.5);
                    else
                      v66 = 0;
                  }
                  v73 = sub_146EA0A30(
                          0,
                          v71 | ((((v67 | (v66 << 8)) << 8) | (unsigned int)v69) << 8),
                          (unsigned int)v64,
                          100);
                  *(float *)(a1 + 232) = (float)BYTE2(v73) * 0.0039215689;
                  *(float *)(a1 + 236) = (float)BYTE1(v73) * 0.0039215689;
                  *(float *)(a1 + 240) = (float)(unsigned __int8)v73 * 0.0039215689;
                  *(float *)(a1 + 244) = (float)HIBYTE(v73) * 0.0039215689;
                }
                v74 = *(_DWORD *)(v63 + 620);
                if ( v74 >= 0 )
                {
                  v75 = *(_DWORD *)(a1 + 216);
                  if ( v75 >= 1 )
                  {
                    v76 = (_QWORD *)(*(__int64 (__fastcall **)(_QWORD *, _QWORD *, __int64))(*v9 + 120LL))(
                                      v9,
                                      v159,
                                      v74 + v75 - 1LL);
                    v77 = (_DWORD *)*v76;
                    *v76 = 0;
                    v78 = v49;
                    v158 = v49;
                    v49 = v77;
                    v170 = v77;
                    if ( v158 )
                    {
                      if ( (int)--v78[2] <= 0 )
                        (*(void (__fastcall **)(_DWORD *))(*(_QWORD *)v78 + 8LL))(v78);
                    }
                    v79 = (int *)v159[0];
                    if ( v159[0] )
                    {
                      --*(_DWORD *)(v159[0] + 8LL);
                      if ( v79[2] <= 0 )
                        (*(void (__fastcall **)(int *))(*(_QWORD *)v79 + 8LL))(v79);
                    }
                  }
                }
              }
              if ( v49 )
                goto LABEL_123;
              v80 = *(int *)(a1 + 216);
              if ( (int)v80 < 1 )
                goto LABEL_134;
              v81 = (_DWORD **)(*(__int64 (__fastcall **)(_QWORD *, _QWORD *, __int64))(*v9 + 120LL))(v9, v160, v80 + 9);
              v49 = *v81;
              *v81 = 0;
              v159[1] = 0;
              v170 = v49;
              v82 = (int *)v160[0];
              if ( v160[0] )
              {
                --*(_DWORD *)(v160[0] + 8LL);
                if ( v82[2] <= 0 )
                  (*(void (__fastcall **)(int *))(*(_QWORD *)v82 + 8LL))(v82);
              }
              if ( v49 )
              {
LABEL_123:
                v83 = qword_14E63AE60;
                if ( !qword_14E63AE60 )
                {
                  v84 = sub_146E8BA20(336);
                  v168 = (_QWORD *)v84;
                  if ( v84 )
                    v85 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v84);
                  else
                    v85 = 0;
                  qword_14E63AE60 = (__int64)v85;
                  (**v85)(v85);
                  v83 = qword_14E63AE60;
                }
                if ( !(unsigned __int8)sub_1447EDD20(v83, *(unsigned int *)(a1 + 104)) )
                  *(_DWORD *)(a1 + 92) += 4;
                v147 = *(unsigned int *)(a1 + 92) | ((unsigned __int64)v20 << 32);
                v168 = &v148;
                v148 = v49;
                ++v49[2];
                v86 = (__int64 **)(a1 + 120);
                sub_1447EEF80(a1 + 120, &v147);
                v14 &= ~8u;
                v168 = &v148;
                v87 = v148;
                if ( v148 )
                {
                  --v148[2];
                  if ( v87[2] <= 0 )
                    (*(void (__fastcall **)(int *))(*(_QWORD *)v87 + 8LL))(v87);
                }
                v88 = (*(__int64 (__fastcall **)(_DWORD *))(*(_QWORD *)v49 + 32LL))(v49);
                *(_DWORD *)(a1 + 256) = v88;
                *(_DWORD *)(a1 + 92) += v88 - 2;
              }
              else
              {
LABEL_134:
                v86 = (__int64 **)(a1 + 120);
              }
              v89 = qword_14E63AE60;
              if ( !qword_14E63AE60 )
              {
                v90 = sub_146E8BA20(336);
                v168 = (_QWORD *)v90;
                if ( v90 )
                  v91 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v90);
                else
                  v91 = 0;
                qword_14E63AE60 = (__int64)v91;
                (**v91)(v91);
                v89 = qword_14E63AE60;
              }
              if ( (unsigned __int8)sub_1447EDD20(v89, *(unsigned int *)(a1 + 104))
                && *(_DWORD *)(a1 + 296) <= 2u
                && *(int *)(a1 + 216) >= 2 )
              {
                *(_DWORD *)(a1 + 92) += 6;
                LODWORD(v169) = 0;
                *(_QWORD *)&v151 = &v143;
                v92 = (__int64 *)(a1 + 368);
                v93 = 0;
                v134 = 0;
                do
                {
                  LODWORD(v168) = 0;
                  if ( (*(_DWORD *)(a1 + 180) & 2) != 0 && !(unsigned __int8)sub_146E9FA80(a1 + 176) )
                  {
                    if ( (unsigned __int64)(int)v169 >= *(_QWORD *)(a1 + 384)
                      || (unsigned __int64)(int)v169 >= *(_QWORD *)(a1 + 352) )
                    {
                      goto LABEL_154;
                    }
                    v94 = (_QWORD *)(a1 + 336);
                    if ( *(_QWORD *)(a1 + 360) >= 8u )
                      v94 = (_QWORD *)*v94;
                    v95 = a1 + 368;
                    if ( *(_QWORD *)(a1 + 392) >= 8u )
                      v95 = *v92;
                    if ( *(_WORD *)&v93[v95] == *(_WORD *)&v93[(_QWORD)v94] )
                    {
LABEL_154:
                      v98 = 1.0 - sub_146E9FA10(a1 + 176);
                      LODWORD(v168) = (int)(float)((float)((int)sub_146E9BA90(v99) % 5 - 2) * v98);
                    }
                    else
                    {
                      v96 = 1.0 - sub_146E9FA10(a1 + 176);
                      LODWORD(v168) = (int)(float)((float)((int)sub_146E9BA90(v97) % 8) * v96);
                    }
                  }
                  v100 = a1 + 368;
                  if ( *(_QWORD *)(a1 + 392) >= 8u )
                    v100 = *v92;
                  (*(void (__fastcall **)(_QWORD *, int **, __int64))(*v9 + 120LL))(
                    v9,
                    &v139,
                    *(unsigned __int16 *)&v93[v100] - 48LL);
                  v101 = qword_14E63AE60;
                  if ( !qword_14E63AE60 )
                  {
                    v102 = sub_146E8BA20(336);
                    v166 = (_QWORD *)v102;
                    if ( v102 )
                      v103 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v102);
                    else
                      v103 = 0;
                    qword_14E63AE60 = (__int64)v103;
                    (**v103)(v103);
                    v101 = qword_14E63AE60;
                  }
                  v104 = sub_1447EAC20(v101, *(unsigned int *)(a1 + 104));
                  LODWORD(v168) = -((_DWORD)v168 + v104);
                  LODWORD(v145) = *(_DWORD *)(a1 + 92);
                  HIDWORD(v145) = (_DWORD)v168;
                  v142 = v145;
                  v143 = v139;
                  if ( v139 )
                    ++v139[2];
                  v105 = v14 | 0x10;
                  v106 = *(_QWORD *)(a1 + 120);
                  if ( *(_QWORD *)(a1 + 128) == 0x7FFFFFFFFFFFFFFLL )
                    sub_14883BB54("list too long");
                  v154 = a1 + 120;
                  v155 = 0;
                  v107 = (_QWORD *)sub_146E8BA20(32);
                  v155 = v107;
                  v165 = v107 + 2;
                  v107[2] = v142;
                  v164 = v107 + 3;
                  v107[3] = v143;
                  v143 = 0;
                  ++*(_QWORD *)(a1 + 128);
                  v108 = *(_QWORD **)(v106 + 8);
                  *v107 = v106;
                  v107[1] = v108;
                  v155 = 0;
                  *(_QWORD *)(v106 + 8) = v107;
                  *v108 = v107;
                  v14 = v105 & 0xFFFFFFEF;
                  v163 = &v143;
                  v109 = v143;
                  if ( v143 )
                  {
                    --v143[2];
                    if ( v109[2] <= 0 )
                      (*(void (__fastcall **)(int *))(*(_QWORD *)v109 + 8LL))(v109);
                  }
                  v110 = 0;
                  v111 = a1 + 368;
                  if ( *(_QWORD *)(a1 + 392) >= 8u )
                    v111 = *v92;
                  v112 = *(unsigned __int16 *)((char *)v134 + v111);
                  if ( v137 )
                  {
                    v113 = *(_QWORD *)(v137 + 632);
                    v112 -= 40LL;
                    if ( (*(_QWORD *)(v137 + 640) - v113) >> 2 > v112 )
                      v110 = *(_DWORD *)(v113 + 4 * v112) + 1;
                  }
                  *(_DWORD *)(a1 + 92) += v110
                                        + (*(__int64 (__fastcall **)(int *, unsigned __int64))(*(_QWORD *)v139 + 48LL))(
                                            v139,
                                            v112)
                                        - 1;
                  v114 = v139;
                  if ( v139 )
                  {
                    --v139[2];
                    if ( v114[2] <= 0 )
                      (*(void (__fastcall **)(int *))(*(_QWORD *)v114 + 8LL))(v114);
                  }
                  LODWORD(v169) = (_DWORD)v169 + 1;
                  v93 = (char *)v134 + 2;
                  v134 = (int *)((char *)v134 + 2);
                }
                while ( (int)v169 < 4 );
                v115 = 0;
                v146 = 0;
                v116 = 0;
                if ( v137 )
                {
                  v117 = *(_DWORD *)(v137 + 620);
                  if ( v117 >= 0 )
                  {
                    v118 = *(_DWORD *)(a1 + 216);
                    if ( v118 > 1 )
                    {
                      v119 = (_DWORD **)(*(__int64 (__fastcall **)(_QWORD *, _QWORD *, __int64))(*v9 + 120LL))(
                                          v9,
                                          v161,
                                          v117 + v118 - 2LL);
                      v115 = *v119;
                      *v119 = 0;
                      v160[1] = 0;
                      v146 = v115;
                      v116 = v115;
                      v120 = (int *)v161[0];
                      if ( v161[0] )
                      {
                        --*(_DWORD *)(v161[0] + 8LL);
                        if ( v120[2] <= 0 )
                          (*(void (__fastcall **)(int *))(*(_QWORD *)v120 + 8LL))(v120);
                      }
                    }
                  }
                }
                if ( v116 )
                  goto LABEL_192;
                v121 = *(int *)(a1 + 216);
                v116 = 0;
                if ( (int)v121 < 1 )
                  goto LABEL_196;
                v122 = (_DWORD **)(*(__int64 (__fastcall **)(_QWORD *, int **, __int64))(*v9 + 120LL))(
                                    v9,
                                    &v162,
                                    v121 + 9);
                v116 = *v122;
                *v122 = 0;
                v161[1] = v115;
                v146 = v116;
                if ( v115 )
                {
                  if ( (int)--v115[2] <= 0 )
                    (*(void (__fastcall **)(_DWORD *))(*(_QWORD *)v115 + 8LL))(v115);
                }
                v123 = v162;
                if ( v162 )
                {
                  --v162[2];
                  if ( v123[2] <= 0 )
                    (*(void (__fastcall **)(int *))(*(_QWORD *)v123 + 8LL))(v123);
                }
                if ( v116 )
                {
LABEL_192:
                  v149 = *(unsigned int *)(a1 + 92) | ((unsigned __int64)(unsigned int)v168 << 32);
                  v168 = &v150;
                  v150 = v116;
                  ++v116[2];
                  v86 = (__int64 **)(a1 + 120);
                  sub_1447EEF80(a1 + 120, &v149);
                  v169 = (int *)&v150;
                  v124 = v150;
                  if ( v150 )
                  {
                    --v150[2];
                    if ( v124[2] <= 0 )
                      (*(void (__fastcall **)(int *))(*(_QWORD *)v124 + 8LL))(v124);
                  }
                  v125 = (*(__int64 (__fastcall **)(_DWORD *))(*(_QWORD *)v116 + 32LL))(v116);
                  *(_DWORD *)(a1 + 256) = v125;
                  *(_DWORD *)(a1 + 92) += v125 - 2;
                }
                else
                {
LABEL_196:
                  v86 = (__int64 **)(a1 + 120);
                }
                if ( v116 )
                {
                  if ( (int)--v116[2] <= 0 )
                    (*(void (__fastcall **)(_DWORD *))(*(_QWORD *)v116 + 8LL))(v116);
                }
              }
              result = (_QWORD **)v137;
              if ( v137 )
              {
                if ( *(_BYTE *)(v137 + 624) )
                {
                  v126 = *v86;
                  for ( i = (__int64 *)*v126; i != v126; i = (__int64 *)*i )
                  {
                    v126 = (__int64 *)v126[1];
                    if ( i == v126 )
                      break;
                    v128 = (int *)(v126 + 2);
                    v129 = (int *)(i + 2);
                    if ( i + 2 != v126 + 2 )
                    {
                      v130 = *v129;
                      *v129 = *v128;
                      *v128 = v130;
                      v131 = *((_DWORD *)i + 5);
                      *((_DWORD *)i + 5) = *((_DWORD *)v126 + 5);
                      *((_DWORD *)v126 + 5) = v131;
                      result = (_QWORD **)sub_1447E39C0(i + 3, v126 + 3);
                    }
                  }
                }
              }
              v132 = *(_QWORD *)(a1 + 152);
              if ( v132 )
              {
                result = (_QWORD **)sub_146B33E50(v132);
                if ( (int)result > 0 )
                {
                  do
                  {
                    v133 = sub_146B33E00(*(_QWORD *)(a1 + 152), (unsigned int)v4);
                    sub_1472F7850(v133, (unsigned int)-(*(_DWORD *)(a1 + 92) >> 1));
                    ++v4;
                    result = (_QWORD **)sub_146B33E50(*(_QWORD *)(a1 + 152));
                  }
                  while ( v4 < (int)result );
                }
              }
              if ( v49 )
              {
                if ( (int)--v49[2] <= 0 )
                  result = (_QWORD **)(*(__int64 (__fastcall **)(_DWORD *))(*(_QWORD *)v49 + 8LL))(v49);
              }
              goto LABEL_213;
            }
            v54 = (_DWORD **)(*(__int64 (__fastcall **)(_QWORD *, int **, __int64))(*v9 + 120LL))(v9, &v136, 13);
            v49 = *v54;
            *v54 = 0;
            v135 = 0;
            v170 = v49;
            v55 = v136;
            if ( v136 )
            {
              --v136[2];
              if ( v55[2] <= 0 )
                (*(void (__fastcall **)(int *))(*(_QWORD *)v55 + 8LL))(v55);
            }
            v56 = xmmword_14DC66940;
          }
          else
          {
            v57 = (_DWORD **)(*(__int64 (__fastcall **)(_QWORD *, int **, __int64))(*v9 + 120LL))(v9, &v135, 12);
            v49 = *v57;
            *v57 = 0;
            v136 = 0;
            v170 = v49;
            v58 = v135;
            if ( v135 )
            {
              --v135[2];
              if ( v58[2] <= 0 )
                (*(void (__fastcall **)(int *))(*(_QWORD *)v58 + 8LL))(v58);
            }
            v56 = xmmword_14DC66930;
          }
        }
        else
        {
          v59 = (_DWORD **)(*(__int64 (__fastcall **)(_QWORD *, int **, __int64))(*v9 + 120LL))(v9, &v135, 11);
          v49 = *v59;
          *v59 = 0;
          v136 = 0;
          v170 = v49;
          v60 = v135;
          if ( v135 )
          {
            --v135[2];
            if ( v60[2] <= 0 )
              (*(void (__fastcall **)(int *))(*(_QWORD *)v60 + 8LL))(v60);
          }
          v56 = xmmword_14DC66920;
        }
      }
      else
      {
        v61 = (_DWORD **)(*(__int64 (__fastcall **)(_QWORD *, int **, __int64))(*v9 + 120LL))(v9, &v135, 10);
        v49 = *v61;
        *v61 = 0;
        v136 = 0;
        v170 = v49;
        v62 = v135;
        if ( v135 )
        {
          --v135[2];
          if ( v62[2] <= 0 )
            (*(void (__fastcall **)(int *))(*(_QWORD *)v62 + 8LL))(v62);
        }
        v56 = xmmword_14DC66910;
      }
    }
    else
    {
      v56 = xmmword_14DC66900;
    }
    *(_OWORD *)(a1 + 232) = v56;
    goto LABEL_87;
  }
LABEL_213:
  if ( v9 )
  {
    if ( (int)--*((_DWORD *)v9 + 2) <= 0 )
      return (_QWORD **)(*(__int64 (__fastcall **)(_QWORD *))(*v9 + 8LL))(v9);
  }
  return result;
}

