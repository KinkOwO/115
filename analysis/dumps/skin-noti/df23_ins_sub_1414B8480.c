// ins_sub_1414B8480

char __fastcall sub_1414B8480(__int64 a1)
{
  __int64 v1; // r14
  __int64 v3; // rax
  __int64 v4; // rax
  __int64 v5; // rax
  __int64 v6; // r8
  __int64 v7; // rbx
  __int64 v8; // rax
  __int64 v9; // rsi
  __int64 v10; // r8
  __int64 v11; // rbx
  unsigned __int64 v12; // rdx
  __int64 v13; // rcx
  __int64 v14; // rdi
  __int64 v15; // rax
  __int64 v16; // r8
  __int64 v17; // rdi
  unsigned __int64 v18; // rdx
  __int64 v19; // rcx
  __int64 v20; // rax
  __int64 v21; // rbx
  __int64 v22; // rax
  __int64 v23; // r8
  __int64 v24; // rbx
  unsigned __int64 v25; // rdx
  __int64 v26; // rcx
  __int64 v27; // rax
  __int64 v28; // rbx
  __int64 v29; // rax
  __int64 v30; // r8
  __int64 v31; // rbx
  unsigned __int64 v32; // rdx
  __int64 v33; // rcx
  __int64 v34; // rdi
  __int64 v35; // rax
  __int64 v36; // r8
  __int64 v37; // rdi
  unsigned __int64 v38; // rdx
  __int64 v39; // rcx
  __int64 v40; // rax
  __int64 v41; // rbx
  __int64 v42; // rax
  __int64 v43; // r8
  __int64 v44; // rbx
  unsigned __int64 v45; // rdx
  __int64 v46; // rcx
  __int64 v47; // rax
  __int64 v48; // rbx
  __int64 v49; // rax
  __int64 v50; // r8
  __int64 v51; // rbx
  unsigned __int64 v52; // rdx
  __int64 v53; // rcx
  __int64 v54; // rax
  __int64 v55; // rbx
  __int64 v56; // rax
  __int64 v57; // r8
  __int64 v58; // rbx
  unsigned __int64 v59; // rdx
  __int64 v60; // rcx
  __int64 v61; // rbx
  __int64 v62; // rax
  __int64 v63; // r8
  __int64 *v64; // rbx
  unsigned __int64 v65; // rdx
  __int64 v66; // rcx
  __int64 v67; // r12
  __int64 v68; // r13
  __int64 v69; // rbx
  __int64 v70; // rax
  __int64 v71; // r8
  __int64 v72; // rbx
  unsigned __int64 v73; // rdx
  __int64 v74; // rcx
  int v75; // eax
  unsigned __int64 v76; // rdx
  __int64 v77; // rcx
  _QWORD *v78; // rcx
  unsigned int v79; // eax
  __int64 v80; // rdx
  __int64 v81; // rdi
  __int64 v82; // rcx
  __int64 v83; // rax
  __int64 v84; // rbx
  __int64 v85; // r15
  _QWORD *v86; // rbx
  __int64 v87; // rbx
  __int64 v88; // rax
  __int64 v89; // r8
  __int64 *v90; // rbx
  unsigned __int64 v91; // rdx
  __int64 v92; // rcx
  __int64 v93; // rdi
  __int64 v94; // r14
  __int64 v95; // rbx
  __int64 v96; // rax
  __int64 v97; // r8
  __int64 v98; // rbx
  unsigned __int64 v99; // rdx
  __int64 v100; // rcx
  __int64 v101; // rax
  __int64 v102; // rax
  int v103; // esi
  __int64 v104; // rbx
  __int64 v105; // rax
  __int64 v106; // r8
  __int64 v107; // rbx
  unsigned __int64 v108; // rdx
  __int64 v109; // rcx
  __int64 v110; // rax
  __int64 v111; // rax
  int v112; // eax
  __int64 v113; // rbx
  __int64 v114; // rax
  __int64 v115; // r8
  __int64 v116; // rbx
  unsigned __int64 v117; // rdx
  __int64 v118; // rcx
  __int64 v119; // rbx
  __int64 v120; // rax
  __int64 v121; // r8
  __int64 *v122; // rbx
  unsigned __int64 v123; // rdx
  __int64 v124; // rcx
  __int64 v125; // r12
  __int64 v126; // r13
  __int64 v127; // rbx
  __int64 v128; // rax
  __int64 v129; // r8
  __int64 v130; // rbx
  unsigned __int64 v131; // rdx
  __int64 v132; // rcx
  int v133; // eax
  _QWORD *v134; // rcx
  __int64 v135; // rdx
  int v136; // ebx
  __int64 v137; // rax
  __int64 v138; // r9
  _QWORD *v139; // rcx
  __int64 v140; // rdi
  __int64 v141; // rax
  __int64 v142; // rcx
  __int64 v143; // rbx
  _QWORD *v144; // r14
  _QWORD *v145; // rbx
  __int64 v146; // rbx
  __int64 v147; // rax
  __int64 v148; // r8
  __int64 v149; // rbx
  __int64 v150; // rax
  __int64 *v151; // rax
  __int64 v152; // rsi
  __int64 v153; // r15
  int v154; // ebx
  __int64 *v155; // rdi
  __int64 *v156; // rcx
  __int64 *v157; // rdx
  __int64 v158; // rax
  __int64 v159; // rbx
  __int64 v160; // rax
  __int64 v161; // rbx
  __int64 v162; // rax
  __int64 *v163; // rax
  __int64 v164; // r15
  __int64 v165; // r13
  unsigned int v166; // eax
  __int64 v167; // r8
  int v168; // edi
  __int64 v169; // r14
  __int64 v170; // rax
  __int64 v171; // rbx
  __int64 v172; // rsi
  __int64 v173; // r12
  __int64 v174; // r14
  __int64 v175; // rax
  __int64 v176; // rcx
  __int64 v177; // rax
  unsigned int v178; // edi
  __int64 v179; // rax
  char v180; // [rsp+38h] [rbp-D0h] BYREF
  _BYTE v181[3]; // [rsp+39h] [rbp-CFh] BYREF
  unsigned int v182; // [rsp+3Ch] [rbp-CCh]
  __int64 v183; // [rsp+40h] [rbp-C8h]
  __int64 v184; // [rsp+48h] [rbp-C0h] BYREF
  __int128 v185; // [rsp+58h] [rbp-B0h] BYREF
  __int64 v186; // [rsp+78h] [rbp-90h]
  unsigned int v187; // [rsp+80h] [rbp-88h] BYREF
  int v188; // [rsp+84h] [rbp-84h] BYREF
  _DWORD v189[2]; // [rsp+88h] [rbp-80h] BYREF
  _QWORD v190[2]; // [rsp+90h] [rbp-78h] BYREF
  __int64 v191; // [rsp+A0h] [rbp-68h]
  __int64 v192; // [rsp+A8h] [rbp-60h]
  __int64 v193; // [rsp+B0h] [rbp-58h]
  __int128 v194; // [rsp+B8h] [rbp-50h]
  __int64 v195; // [rsp+C8h] [rbp-40h]
  __int64 v196; // [rsp+D0h] [rbp-38h]
  __int64 v197; // [rsp+D8h] [rbp-30h]
  __int64 v198; // [rsp+E0h] [rbp-28h]
  int *v199; // [rsp+E8h] [rbp-20h] BYREF
  __int64 v200; // [rsp+F0h] [rbp-18h]
  __int64 v201; // [rsp+F8h] [rbp-10h]
  __int128 v202; // [rsp+100h] [rbp-8h]
  __int64 v203; // [rsp+118h] [rbp+10h]
  __int64 v204; // [rsp+120h] [rbp+18h]
  __int64 v205; // [rsp+128h] [rbp+20h]
  _BYTE v206[16]; // [rsp+130h] [rbp+28h] BYREF
  _BYTE v207[24]; // [rsp+140h] [rbp+38h] BYREF
  __int128 v208; // [rsp+158h] [rbp+50h] BYREF
  __int128 v209; // [rsp+168h] [rbp+60h] BYREF
  __int128 v210; // [rsp+178h] [rbp+70h] BYREF
  char v211[16]; // [rsp+188h] [rbp+80h] BYREF
  _QWORD v212[2]; // [rsp+198h] [rbp+90h] BYREF
  __int64 v213; // [rsp+1A8h] [rbp+A0h]
  unsigned __int64 v214; // [rsp+1B0h] [rbp+A8h]
  _QWORD v215[2]; // [rsp+1B8h] [rbp+B0h] BYREF
  __int64 v216; // [rsp+1C8h] [rbp+C0h]
  unsigned __int64 v217; // [rsp+1D0h] [rbp+C8h]
  _QWORD v218[2]; // [rsp+1D8h] [rbp+D0h] BYREF
  __int64 v219; // [rsp+1E8h] [rbp+E0h]
  unsigned __int64 v220; // [rsp+1F0h] [rbp+E8h]
  _QWORD v221[2]; // [rsp+1F8h] [rbp+F0h] BYREF
  __int64 v222; // [rsp+208h] [rbp+100h]
  unsigned __int64 v223; // [rsp+210h] [rbp+108h]
  _QWORD v224[2]; // [rsp+218h] [rbp+110h] BYREF
  __int64 v225; // [rsp+228h] [rbp+120h]
  unsigned __int64 v226; // [rsp+230h] [rbp+128h]
  _QWORD v227[2]; // [rsp+238h] [rbp+130h] BYREF
  __int64 v228; // [rsp+248h] [rbp+140h]
  unsigned __int64 v229; // [rsp+250h] [rbp+148h]
  _QWORD v230[2]; // [rsp+258h] [rbp+150h] BYREF
  __int64 v231; // [rsp+268h] [rbp+160h]
  unsigned __int64 v232; // [rsp+270h] [rbp+168h]
  _QWORD v233[2]; // [rsp+278h] [rbp+170h] BYREF
  __int64 v234; // [rsp+288h] [rbp+180h]
  unsigned __int64 v235; // [rsp+290h] [rbp+188h]
  _QWORD v236[2]; // [rsp+298h] [rbp+190h] BYREF
  __int64 v237; // [rsp+2A8h] [rbp+1A0h]
  unsigned __int64 v238; // [rsp+2B0h] [rbp+1A8h]
  _QWORD v239[2]; // [rsp+2B8h] [rbp+1B0h] BYREF
  __int64 v240; // [rsp+2C8h] [rbp+1C0h]
  unsigned __int64 v241; // [rsp+2D0h] [rbp+1C8h]
  _QWORD v242[2]; // [rsp+2D8h] [rbp+1D0h] BYREF
  __int64 v243; // [rsp+2E8h] [rbp+1E0h]
  unsigned __int64 v244; // [rsp+2F0h] [rbp+1E8h]
  _QWORD v245[2]; // [rsp+2F8h] [rbp+1F0h] BYREF
  __int64 v246; // [rsp+308h] [rbp+200h]
  unsigned __int64 v247; // [rsp+310h] [rbp+208h]
  _QWORD v248[2]; // [rsp+318h] [rbp+210h] BYREF
  __int64 v249; // [rsp+328h] [rbp+220h]
  unsigned __int64 v250; // [rsp+330h] [rbp+228h]
  _QWORD v251[2]; // [rsp+338h] [rbp+230h] BYREF
  __int64 v252; // [rsp+348h] [rbp+240h]
  unsigned __int64 v253; // [rsp+350h] [rbp+248h]
  _QWORD v254[2]; // [rsp+358h] [rbp+250h] BYREF
  __int64 v255; // [rsp+368h] [rbp+260h]
  unsigned __int64 v256; // [rsp+370h] [rbp+268h]
  _QWORD v257[2]; // [rsp+378h] [rbp+270h] BYREF
  __int64 v258; // [rsp+388h] [rbp+280h]
  unsigned __int64 v259; // [rsp+390h] [rbp+288h]
  _QWORD v260[2]; // [rsp+398h] [rbp+290h] BYREF
  __int64 v261; // [rsp+3A8h] [rbp+2A0h]
  unsigned __int64 v262; // [rsp+3B0h] [rbp+2A8h]
  _QWORD v263[2]; // [rsp+3B8h] [rbp+2B0h] BYREF
  __int64 v264; // [rsp+3C8h] [rbp+2C0h]
  unsigned __int64 v265; // [rsp+3D0h] [rbp+2C8h]
  _QWORD v266[2]; // [rsp+3D8h] [rbp+2D0h] BYREF
  __int64 v267; // [rsp+3E8h] [rbp+2E0h]
  unsigned __int64 v268; // [rsp+3F0h] [rbp+2E8h]
  _QWORD v269[2]; // [rsp+3F8h] [rbp+2F0h] BYREF
  __int64 v270; // [rsp+408h] [rbp+300h]
  unsigned __int64 v271; // [rsp+410h] [rbp+308h]
  __int64 v272; // [rsp+418h] [rbp+310h] BYREF
  __int64 v273; // [rsp+428h] [rbp+320h]
  unsigned __int64 v274; // [rsp+430h] [rbp+328h]
  _QWORD v275[4]; // [rsp+438h] [rbp+330h] BYREF

  v203 = -2;
  v1 = a1;
  v186 = a1;
  v182 = 0;
  LODWORD(v183) = 0;
  if ( *(_BYTE *)(a1 + 68) )
    return 1;
  v190[0] = v206;
  v3 = sub_146E8C7D0(&unk_1496B13F0);
  v4 = sub_146E8D440(v206, v3);
  v5 = sub_146E94BD0(v207, v4);
  LOBYTE(v6) = 1;
  sub_1474CDEA0(&v184, v5, v6);
  sub_146E8D630(v207);
  v7 = v184;
  if ( v184 )
  {
    v180 = 0;
    v8 = sub_146E8C7D0(&unk_1496B1470);
    v221[0] = 0;
    v222 = 0;
    v223 = 7;
    v9 = -1;
    v10 = -1;
    do
      ++v10;
    while ( *(_WORD *)(v8 + 2 * v10) );
    sub_14014C8D0(v221, v8);
    v11 = sub_1474CCCD0(v7, v221, 0);
    if ( v223 >= 8 )
    {
      v12 = 2 * v223 + 2;
      v13 = v221[0];
      if ( v12 >= 0x1000 )
      {
        v12 = 2 * v223 + 41;
        v13 = *(_QWORD *)(v221[0] - 8LL);
        if ( (unsigned __int64)(v221[0] - v13 - 8) > 0x1F )
          sub_148AAF304(v13, v12);
      }
      sub_146E9F3A0(v13, v12);
    }
    v222 = 0;
    v223 = 7;
    LOWORD(v221[0]) = 0;
    if ( (unsigned __int8)sub_1474D19F0(v11) )
    {
      v14 = sub_1474CA7F0(v11);
      v15 = sub_146E8C7D0(&unk_1496B14A8);
      v224[0] = 0;
      v225 = 0;
      v226 = 7;
      v16 = -1;
      do
        ++v16;
      while ( *(_WORD *)(v15 + 2 * v16) );
      sub_14014C8D0(v224, v15);
      v17 = sub_1474CCBD0(v14, v224, 0);
      if ( v226 >= 8 )
      {
        v18 = 2 * v226 + 2;
        v19 = v224[0];
        if ( v18 >= 0x1000 )
        {
          v18 = 2 * v226 + 41;
          v19 = *(_QWORD *)(v224[0] - 8LL);
          if ( (unsigned __int64)(v224[0] - v19 - 8) > 0x1F )
            sub_148AAF304(v19, v18);
        }
        sub_146E9F3A0(v19, v18);
      }
      v225 = 0;
      v226 = 7;
      LOWORD(v224[0]) = 0;
      if ( (unsigned __int8)sub_1474D19F0(v17) )
      {
        v20 = sub_1474CA7F0(v17);
        *(_DWORD *)(v1 + 48) = sub_1474CD5D0(v20, 0, &v180, 0);
      }
      v21 = sub_1474CA7F0(v11);
      v22 = sub_146E8C7D0(&unk_1496B14C0);
      v227[0] = 0;
      v228 = 0;
      v229 = 7;
      v23 = -1;
      do
        ++v23;
      while ( *(_WORD *)(v22 + 2 * v23) );
      sub_14014C8D0(v227, v22);
      v24 = sub_1474CCBD0(v21, v227, 0);
      if ( v229 >= 8 )
      {
        v25 = 2 * v229 + 2;
        v26 = v227[0];
        if ( v25 >= 0x1000 )
        {
          v25 = 2 * v229 + 41;
          v26 = *(_QWORD *)(v227[0] - 8LL);
          if ( (unsigned __int64)(v227[0] - v26 - 8) > 0x1F )
            sub_148AAF304(v26, v25);
        }
        sub_146E9F3A0(v26, v25);
      }
      v228 = 0;
      v229 = 7;
      LOWORD(v227[0]) = 0;
      if ( (unsigned __int8)sub_1474D19F0(v24) )
      {
        v27 = sub_1474CA7F0(v24);
        *(_DWORD *)(v1 + 52) = sub_1474CD5D0(v27, 0, &v180, 0);
      }
    }
    v28 = v184;
    v29 = sub_146E8C7D0(&unk_1496B14E0);
    v230[0] = 0;
    v231 = 0;
    v232 = 7;
    v30 = -1;
    do
      ++v30;
    while ( *(_WORD *)(v29 + 2 * v30) );
    sub_14014C8D0(v230, v29);
    v31 = sub_1474CCCD0(v28, v230, 0);
    if ( v232 >= 8 )
    {
      v32 = 2 * v232 + 2;
      v33 = v230[0];
      if ( v32 >= 0x1000 )
      {
        v32 = 2 * v232 + 41;
        v33 = *(_QWORD *)(v230[0] - 8LL);
        if ( (unsigned __int64)(v230[0] - v33 - 8) > 0x1F )
          sub_148AAF304(v33, v32);
      }
      sub_146E9F3A0(v33, v32);
    }
    v231 = 0;
    v232 = 7;
    LOWORD(v230[0]) = 0;
    if ( (unsigned __int8)sub_1474D19F0(v31) )
    {
      v34 = sub_1474CA7F0(v31);
      v35 = sub_146E8C7D0(&unk_1496B1520);
      v233[0] = 0;
      v234 = 0;
      v235 = 7;
      v36 = -1;
      do
        ++v36;
      while ( *(_WORD *)(v35 + 2 * v36) );
      sub_14014C8D0(v233, v35);
      v37 = sub_1474CCBD0(v34, v233, 0);
      if ( v235 >= 8 )
      {
        v38 = 2 * v235 + 2;
        v39 = v233[0];
        if ( v38 >= 0x1000 )
        {
          v38 = 2 * v235 + 41;
          v39 = *(_QWORD *)(v233[0] - 8LL);
          if ( (unsigned __int64)(v233[0] - v39 - 8) > 0x1F )
            sub_148AAF304(v39, v38);
        }
        sub_146E9F3A0(v39, v38);
      }
      v234 = 0;
      v235 = 7;
      LOWORD(v233[0]) = 0;
      if ( (unsigned __int8)sub_1474D19F0(v37) )
      {
        v40 = sub_1474CA7F0(v37);
        *(_DWORD *)(v1 + 56) = sub_1474CD5D0(v40, 0, &v180, 0);
      }
      v41 = sub_1474CA7F0(v31);
      v42 = sub_146E8C7D0(&unk_1496B1538);
      v236[0] = 0;
      v237 = 0;
      v238 = 7;
      v43 = -1;
      do
        ++v43;
      while ( *(_WORD *)(v42 + 2 * v43) );
      sub_14014C8D0(v236, v42);
      v44 = sub_1474CCBD0(v41, v236, 0);
      if ( v238 >= 8 )
      {
        v45 = 2 * v238 + 2;
        v46 = v236[0];
        if ( v45 >= 0x1000 )
        {
          v45 = 2 * v238 + 41;
          v46 = *(_QWORD *)(v236[0] - 8LL);
          if ( (unsigned __int64)(v236[0] - v46 - 8) > 0x1F )
            sub_148AAF304(v46, v45);
        }
        sub_146E9F3A0(v46, v45);
      }
      v237 = 0;
      v238 = 7;
      LOWORD(v236[0]) = 0;
      if ( (unsigned __int8)sub_1474D19F0(v44) )
      {
        v47 = sub_1474CA7F0(v44);
        *(_DWORD *)(v1 + 60) = sub_1474CD5D0(v47, 0, &v180, 0);
      }
    }
    v48 = v184;
    v49 = sub_146E8C7D0(&unk_1496B1550);
    v239[0] = 0;
    v240 = 0;
    v241 = 7;
    v50 = -1;
    do
      ++v50;
    while ( *(_WORD *)(v49 + 2 * v50) );
    sub_14014C8D0(v239, v49);
    v51 = sub_1474CCCD0(v48, v239, 0);
    if ( v241 >= 8 )
    {
      v52 = 2 * v241 + 2;
      v53 = v239[0];
      if ( v52 >= 0x1000 )
      {
        v52 = 2 * v241 + 41;
        v53 = *(_QWORD *)(v239[0] - 8LL);
        if ( (unsigned __int64)(v239[0] - v53 - 8) > 0x1F )
          sub_148AAF304(v53, v52);
      }
      sub_146E9F3A0(v53, v52);
    }
    v240 = 0;
    v241 = 7;
    LOWORD(v239[0]) = 0;
    if ( (unsigned __int8)sub_1474D19F0(v51) )
    {
      v54 = sub_1474CA7F0(v51);
      *(_DWORD *)(v1 + 64) = sub_1474CD5D0(v54, 0, &v180, 0);
    }
    v55 = v184;
    v56 = sub_146E8C7D0(&unk_1496B1580);
    v242[0] = 0;
    v243 = 0;
    v244 = 7;
    v57 = -1;
    do
      ++v57;
    while ( *(_WORD *)(v56 + 2 * v57) );
    sub_14014C8D0(v242, v56);
    v58 = sub_1474CCCD0(v55, v242, 0);
    if ( v244 >= 8 )
    {
      v59 = 2 * v244 + 2;
      v60 = v242[0];
      if ( v59 >= 0x1000 )
      {
        v59 = 2 * v244 + 41;
        v60 = *(_QWORD *)(v242[0] - 8LL);
        if ( (unsigned __int64)(v242[0] - v60 - 8) > 0x1F )
          sub_148AAF304(v60, v59);
      }
      sub_146E9F3A0(v60, v59);
    }
    v243 = 0;
    v244 = 7;
    LOWORD(v242[0]) = 0;
    if ( (unsigned __int8)sub_1474D19F0(v58) )
    {
      v61 = sub_1474CA7F0(v58);
      v62 = sub_146E8C7D0(&unk_1496B15B0);
      v245[0] = 0;
      v246 = 0;
      v247 = 7;
      v63 = -1;
      do
        ++v63;
      while ( *(_WORD *)(v62 + 2 * v63) );
      sub_14014C8D0(v245, v62);
      v64 = (__int64 *)sub_1474CCB60(v61, v245);
      if ( v247 >= 8 )
      {
        v65 = 2 * v247 + 2;
        v66 = v245[0];
        if ( v65 >= 0x1000 )
        {
          v65 = 2 * v247 + 41;
          v66 = *(_QWORD *)(v245[0] - 8LL);
          if ( (unsigned __int64)(v245[0] - v66 - 8) > 0x1F )
            sub_148AAF304(v66, v65);
        }
        sub_146E9F3A0(v66, v65);
      }
      v246 = 0;
      v247 = 7;
      LOWORD(v245[0]) = 0;
      v67 = *v64;
      v68 = v64[1];
      while ( v67 != v68 )
      {
        v69 = sub_1474CA7F0(v67);
        v70 = sub_146E8C7D0(&unk_1496B13C0);
        v248[0] = 0;
        v249 = 0;
        v250 = 7;
        v71 = -1;
        do
          ++v71;
        while ( *(_WORD *)(v70 + 2 * v71) );
        sub_14014C8D0(v248, v70);
        v72 = sub_1474CCBD0(v69, v248, 0);
        if ( v250 >= 8 )
        {
          v73 = 2 * v250 + 2;
          v74 = v248[0];
          if ( v73 >= 0x1000 )
          {
            v73 = 2 * v250 + 41;
            v74 = *(_QWORD *)(v248[0] - 8LL);
            if ( (unsigned __int64)(v248[0] - v74 - 8) > 0x1F )
              sub_148AAF304(v74, v73);
          }
          sub_146E9F3A0(v74, v73);
        }
        v249 = 0;
        v250 = 7;
        LOWORD(v248[0]) = 0;
        if ( (unsigned __int8)sub_1474D19F0(v72) )
        {
          v75 = sub_1474CA7F0(v72);
          v272 = 0;
          v273 = 0;
          v274 = 7;
          sub_1474CDB90(v75, (unsigned int)v275, 0, (unsigned int)&v180, (__int64)&v272);
          if ( v274 >= 8 )
          {
            v76 = 2 * v274 + 2;
            v77 = v272;
            if ( v76 >= 0x1000 )
            {
              v76 = 2 * v274 + 41;
              v77 = *(_QWORD *)(v272 - 8);
              if ( (unsigned __int64)(v272 - v77 - 8) > 0x1F )
                sub_148AAF304(v77, v76);
            }
            sub_146E9F3A0(v77, v76);
          }
          v273 = 0;
          v274 = 7;
          LOWORD(v272) = 0;
          v78 = v275;
          if ( v275[3] >= 8u )
            v78 = (_QWORD *)v275[0];
          v79 = sub_1470A7700(v78);
          v80 = v79;
          if ( v79 != 17 )
          {
            v187 = v79;
            v81 = *(_QWORD *)(v1 + 16);
            v82 = *(_QWORD *)(v81 + 8);
            *(_QWORD *)&v185 = v82;
            DWORD2(v185) = 0;
            v83 = v81;
            while ( !*(_BYTE *)(v82 + 25) )
            {
              *(_QWORD *)&v185 = v82;
              if ( *(_DWORD *)(v82 + 32) >= (int)v80 )
              {
                DWORD2(v185) = 1;
                v83 = v82;
                v82 = *(_QWORD *)v82;
              }
              else
              {
                DWORD2(v185) = 0;
                v82 = *(_QWORD *)(v82 + 16);
              }
            }
            if ( *(_BYTE *)(v83 + 25) || (int)v80 < *(_DWORD *)(v83 + 32) )
            {
              if ( *(_QWORD *)(v1 + 24) == 0x492492492492492LL )
                sub_14014F360(v82, v80);
              v195 = v1 + 16;
              v196 = 0;
              v84 = sub_146E8BA20(56);
              v196 = v84;
              v199 = (int *)&v187;
              sub_1414B8170(v84 + 32, 0, &v199, v181);
              *(_QWORD *)v84 = v81;
              *(_QWORD *)(v84 + 8) = v81;
              *(_QWORD *)(v84 + 16) = v81;
              *(_WORD *)(v84 + 24) = 0;
              v196 = 0;
              v208 = v185;
              v83 = sub_14014F0E0(v1 + 16, &v208, v84);
            }
            v85 = v83 + 40;
            v86 = *(_QWORD **)(v83 + 40);
            sub_140150890(v83 + 40, v83 + 40, v86[1]);
            v86[1] = v86;
            *v86 = v86;
            v86[2] = v86;
            *(_QWORD *)(v85 + 8) = 0;
            v87 = sub_1474CA7F0(v67);
            v88 = sub_146E8C7D0(&unk_1496B15C8);
            v251[0] = 0;
            v252 = 0;
            v253 = 7;
            v9 = -1;
            v89 = -1;
            do
              ++v89;
            while ( *(_WORD *)(v88 + 2 * v89) );
            sub_14014C8D0(v251, v88);
            v90 = (__int64 *)sub_1474CCB60(v87, v251);
            if ( v253 >= 8 )
            {
              v91 = 2 * v253 + 2;
              v92 = v251[0];
              if ( v91 >= 0x1000 )
              {
                v91 = 2 * v253 + 41;
                v92 = *(_QWORD *)(v251[0] - 8LL);
                if ( (unsigned __int64)(v251[0] - v92 - 8) > 0x1F )
                  sub_148AAF304(v92, v91);
              }
              sub_146E9F3A0(v92, v91);
            }
            v252 = 0;
            v253 = 7;
            LOWORD(v251[0]) = 0;
            v93 = *v90;
            v94 = v90[1];
            if ( *v90 != v94 )
            {
              do
              {
                v95 = sub_1474CA7F0(v93);
                v96 = sub_146E8C7D0(&unk_1496B1600);
                v254[0] = 0;
                v255 = 0;
                v256 = 7;
                v97 = -1;
                do
                  ++v97;
                while ( *(_WORD *)(v96 + 2 * v97) );
                sub_14014C8D0(v254, v96);
                v98 = sub_1474CCBD0(v95, v254, 0);
                if ( v256 >= 8 )
                {
                  v99 = 2 * v256 + 2;
                  v100 = v254[0];
                  if ( v99 >= 0x1000 )
                  {
                    v99 = 2 * v256 + 41;
                    v100 = *(_QWORD *)(v254[0] - 8LL);
                    if ( (unsigned __int64)(v254[0] - v100 - 8) > 0x1F )
                      sub_148AAF304(v100, v99);
                  }
                  sub_146E9F3A0(v100, v99);
                }
                v255 = 0;
                v256 = 7;
                LOWORD(v254[0]) = 0;
                v101 = sub_1474CA7F0(v98);
                if ( (unsigned __int8)sub_14173D590(v101) )
                {
                  v102 = sub_1474CA7F0(v98);
                  v103 = sub_1474CD5D0(v102, 0, &v180, 0);
                  v104 = sub_1474CA7F0(v93);
                  v105 = sub_146E8C7D0(&unk_1496B1640);
                  v257[0] = 0;
                  v258 = 0;
                  v259 = 7;
                  v106 = -1;
                  do
                    ++v106;
                  while ( *(_WORD *)(v105 + 2 * v106) );
                  sub_14014C8D0(v257, v105);
                  v107 = sub_1474CCBD0(v104, v257, 0);
                  if ( v259 >= 8 )
                  {
                    v108 = 2 * v259 + 2;
                    v109 = v257[0];
                    if ( v108 >= 0x1000 )
                    {
                      v108 = 2 * v259 + 41;
                      v109 = *(_QWORD *)(v257[0] - 8LL);
                      if ( (unsigned __int64)(v257[0] - v109 - 8) > 0x1F )
                        sub_148AAF304(v109, v108);
                    }
                    sub_146E9F3A0(v109, v108);
                  }
                  v258 = 0;
                  v259 = 7;
                  LOWORD(v257[0]) = 0;
                  v110 = sub_1474CA7F0(v107);
                  if ( (unsigned __int8)sub_14173D590(v110) )
                  {
                    v111 = sub_1474CA7F0(v107);
                    v112 = sub_1474CD5D0(v111, 0, &v180, 0);
                    v189[0] = v103;
                    v189[1] = v112;
                    if ( v180 )
                      sub_140430D80(v85, v211, v189);
                  }
                  v9 = -1;
                }
                v93 += 16;
              }
              while ( v93 != v94 );
            }
            v1 = v186;
          }
          sub_14014C710(v275);
        }
        v67 += 16;
      }
    }
    v113 = v184;
    v114 = sub_146E8C7D0(&unk_1496B1678);
    v260[0] = 0;
    v261 = 0;
    v262 = 7;
    v115 = -1;
    do
      ++v115;
    while ( *(_WORD *)(v114 + 2 * v115) );
    sub_14014C8D0(v260, v114);
    v116 = sub_1474CCCD0(v113, v260, 0);
    if ( v262 >= 8 )
    {
      v117 = 2 * v262 + 2;
      v118 = v260[0];
      if ( v117 >= 0x1000 )
      {
        v117 = 2 * v262 + 41;
        v118 = *(_QWORD *)(v260[0] - 8LL);
        if ( (unsigned __int64)(v260[0] - v118 - 8) > 0x1F )
          sub_148AAF304(v118, v117);
      }
      sub_146E9F3A0(v118, v117);
    }
    v261 = 0;
    v262 = 7;
    LOWORD(v260[0]) = 0;
    if ( (unsigned __int8)sub_1474D19F0(v116) )
    {
      v119 = sub_1474CA7F0(v116);
      v120 = sub_146E8C7D0(&unk_1496B15B0);
      v263[0] = 0;
      v264 = 0;
      v265 = 7;
      v121 = -1;
      do
        ++v121;
      while ( *(_WORD *)(v120 + 2 * v121) );
      sub_14014C8D0(v263, v120);
      v122 = (__int64 *)sub_1474CCB60(v119, v263);
      if ( v265 >= 8 )
      {
        v123 = 2 * v265 + 2;
        v124 = v263[0];
        if ( v123 >= 0x1000 )
        {
          v123 = 2 * v265 + 41;
          v124 = *(_QWORD *)(v263[0] - 8LL);
          if ( (unsigned __int64)(v263[0] - v124 - 8) > 0x1F )
            sub_148AAF304(v124, v123);
        }
        sub_146E9F3A0(v124, v123);
      }
      v264 = 0;
      v265 = 7;
      LOWORD(v263[0]) = 0;
      v125 = *v122;
      v126 = v122[1];
      if ( *v122 != v126 )
      {
        while ( 1 )
        {
          v127 = sub_1474CA7F0(v125);
          v128 = sub_146E8C7D0(&unk_1496B13C0);
          v266[0] = 0;
          v267 = 0;
          v268 = 7;
          v129 = -1;
          do
            ++v129;
          while ( *(_WORD *)(v128 + 2 * v129) );
          sub_14014C8D0(v266, v128);
          v130 = sub_1474CCBD0(v127, v266, 0);
          if ( v268 >= 8 )
          {
            v131 = 2 * v268 + 2;
            v132 = v266[0];
            if ( v131 >= 0x1000 )
            {
              v131 = 2 * v268 + 41;
              v132 = *(_QWORD *)(v266[0] - 8LL);
              if ( (unsigned __int64)(v266[0] - v132 - 8) > 0x1F )
                sub_148AAF304(v132, v131);
            }
            sub_146E9F3A0(v132, v131);
          }
          v267 = 0;
          v268 = 7;
          LOWORD(v266[0]) = 0;
          if ( (unsigned __int8)sub_1474D19F0(v130) )
          {
            v133 = sub_1474CA7F0(v130);
            v269[0] = 0;
            v270 = 0;
            v271 = 7;
            sub_1474CDB90(v133, (unsigned int)v212, 0, (unsigned int)&v180, (__int64)v269);
            if ( v271 >= 8 )
              sub_14014CBC0(v269, v269[0], v271 + 1);
            v270 = 0;
            v271 = 7;
            LOWORD(v269[0]) = 0;
            v134 = v212;
            if ( v214 >= 8 )
              v134 = (_QWORD *)v212[0];
            v136 = sub_1470A7700(v134);
            if ( v136 != 17 )
              goto LABEL_156;
            v137 = sub_146E8C7D0(&unk_1496B13D8);
            v138 = -1;
            do
              ++v138;
            while ( *(_WORD *)(v137 + 2 * v138) );
            v139 = v212;
            if ( v214 >= 8 )
              v139 = (_QWORD *)v212[0];
            if ( (unsigned __int8)sub_14014DE60(v139, v213, v137, v138) )
            {
LABEL_156:
              v188 = v136;
              v140 = *(_QWORD *)v1;
              v141 = *(_QWORD *)(*(_QWORD *)v1 + 8LL);
              *(_QWORD *)&v185 = v141;
              DWORD2(v185) = 0;
              v142 = v140;
              while ( !*(_BYTE *)(v141 + 25) )
              {
                *(_QWORD *)&v185 = v141;
                if ( *(_DWORD *)(v141 + 32) >= v136 )
                {
                  DWORD2(v185) = 1;
                  v142 = v141;
                  v141 = *(_QWORD *)v141;
                }
                else
                {
                  DWORD2(v185) = 0;
                  v141 = *(_QWORD *)(v141 + 16);
                }
              }
              if ( *(_BYTE *)(v142 + 25) || v136 < *(_DWORD *)(v142 + 32) )
              {
                if ( *(_QWORD *)(v1 + 8) == 0x492492492492492LL )
                  sub_14014F360(v142, v135);
                v197 = v1;
                v198 = 0;
                v143 = sub_146E8BA20(56);
                v198 = v143;
                v190[0] = &v188;
                sub_1414B8110(v143 + 32, 0, v190, v181);
                *(_QWORD *)v143 = v140;
                *(_QWORD *)(v143 + 8) = v140;
                *(_QWORD *)(v143 + 16) = v140;
                *(_WORD *)(v143 + 24) = 0;
                v198 = 0;
                v209 = v185;
                v142 = sub_14014F0E0(v1, &v209, v143);
              }
              v144 = (_QWORD *)(v142 + 40);
              v145 = *(_QWORD **)(v142 + 40);
              sub_140178C60(v142 + 40, v142 + 40, v145[1]);
              v145[1] = v145;
              *v145 = v145;
              v145[2] = v145;
              v144[1] = 0;
              v146 = sub_1474CA7F0(v125);
              v147 = sub_146E8C7D0(&unk_1496B16A8);
              v215[0] = 0;
              v216 = 0;
              v217 = 7;
              v148 = -1;
              do
                ++v148;
              while ( *(_WORD *)(v147 + 2 * v148) );
              sub_14014C8D0(v215, v147);
              v149 = sub_1474CCBD0(v146, v215, 0);
              if ( v217 >= 8 )
                sub_14014CBC0(v215, v215[0], v217 + 1);
              v216 = 0;
              v217 = 7;
              LOWORD(v215[0]) = 0;
              v150 = sub_1474CA7F0(v149);
              v151 = (__int64 *)sub_14087CD80(v150);
              v152 = *v151;
              v153 = v151[1];
              if ( *v151 != v153 )
              {
                do
                {
                  v154 = sub_1474CD5B0(v152, &v180, 0);
                  if ( v180 )
                  {
                    v155 = (__int64 *)*v144;
                    v156 = *(__int64 **)(*v144 + 8LL);
                    *(_QWORD *)&v185 = v156;
                    DWORD2(v185) = 0;
                    v157 = v155;
                    while ( !*((_BYTE *)v156 + 25) )
                    {
                      *(_QWORD *)&v185 = v156;
                      if ( *((_DWORD *)v156 + 7) >= v154 )
                      {
                        DWORD2(v185) = 1;
                        v157 = v156;
                        v156 = (__int64 *)*v156;
                      }
                      else
                      {
                        DWORD2(v185) = 0;
                        v156 = (__int64 *)v156[2];
                      }
                    }
                    if ( *((_BYTE *)v157 + 25) || v154 < *((_DWORD *)v157 + 7) )
                    {
                      if ( v144[1] == 0x7FFFFFFFFFFFFFFLL )
                        sub_14014F360(0x7FFFFFFFFFFFFFFLL, v157);
                      v190[1] = v144;
                      v191 = 0;
                      v158 = sub_146E8BA20(32);
                      *(_DWORD *)(v158 + 28) = v154;
                      *(_QWORD *)v158 = v155;
                      *(_QWORD *)(v158 + 8) = v155;
                      *(_QWORD *)(v158 + 16) = v155;
                      *(_WORD *)(v158 + 24) = 0;
                      v191 = 0;
                      v210 = v185;
                      sub_14014F0E0(v144, &v210, v158);
                    }
                  }
                  v152 += 40;
                }
                while ( v152 != v153 );
              }
              if ( v214 >= 8 )
                sub_14014CBC0(v212, v212[0], v214 + 1);
              v213 = 0;
              v214 = 7;
              LOWORD(v212[0]) = 0;
              v9 = -1;
            }
            else
            {
              sub_14014C710(v212);
            }
          }
          v125 += 16;
          if ( v125 == v126 )
            break;
          v1 = v186;
        }
      }
    }
    v159 = v184;
    v160 = sub_146E8C7D0(&unk_1496B16C0);
    v218[0] = 0;
    v219 = 0;
    v220 = 7;
    do
      ++v9;
    while ( *(_WORD *)(v160 + 2 * v9) );
    sub_14014C8D0(v218, v160);
    v161 = sub_1474CCCD0(v159, v218, 0);
    if ( v220 >= 8 )
      sub_14014CBC0(v218, v218[0], v220 + 1);
    v219 = 0;
    v220 = 7;
    LOWORD(v218[0]) = 0;
    if ( (unsigned __int8)sub_1474D19F0(v161) )
    {
      v162 = sub_1474CA7F0(v161);
      v163 = (__int64 *)sub_14087CD80(v162);
      v164 = *v163;
      v165 = v163[1];
      if ( *v163 != v165 )
      {
        do
        {
          v166 = sub_1474CD5B0(v164, &v180, 0);
          v168 = v166;
          if ( v180 )
          {
            LOBYTE(v167) = 1;
            v169 = sub_140283D60(qword_14E683B38, v166, v167);
            if ( v169 )
            {
              v170 = sub_146E8BA20(264);
              v171 = v170;
              v204 = v170;
              if ( v170 )
              {
                *(_OWORD *)v170 = 0;
                *(_DWORD *)(v170 + 8) = 1;
                *(_DWORD *)(v170 + 12) = 1;
                *(_QWORD *)v170 = off_1496B1738;
                sub_14586BD00(v170 + 16);
              }
              else
              {
                v171 = 0;
              }
              v182 |= 1u;
              LODWORD(v183) = v182;
              v172 = v171 + 16;
              v200 = v171 + 16;
              v201 = v171;
              sub_146E8D740(v171 + 32, v169 + 496);
              *(_DWORD *)(v171 + 16) = v168;
              *(_DWORD *)(v171 + 20) = sub_14586CEA0(*(unsigned int *)(v169 + 20));
              *(_BYTE *)(v171 + 176) = *(_BYTE *)(v169 + 5922);
              v173 = v186 + 32;
              v174 = *(_QWORD *)(v186 + 32);
              v175 = *(_QWORD *)(v174 + 8);
              *(_QWORD *)&v202 = v175;
              DWORD2(v202) = 0;
              v176 = v174;
              while ( !*(_BYTE *)(v175 + 25) )
              {
                *(_QWORD *)&v202 = v175;
                if ( *(_DWORD *)(v175 + 32) >= v168 )
                {
                  DWORD2(v202) = 1;
                  v176 = v175;
                  v175 = *(_QWORD *)v175;
                }
                else
                {
                  DWORD2(v202) = 0;
                  v175 = *(_QWORD *)(v175 + 16);
                }
              }
              if ( *(_BYTE *)(v176 + 25) || v168 < *(_DWORD *)(v176 + 32) )
              {
                if ( *(_QWORD *)(v186 + 40) == 0x492492492492492LL )
                  sub_14014F360(v176, 0);
                v192 = v186 + 32;
                v193 = 0;
                v177 = sub_146E8BA20(56);
                v205 = v177 + 32;
                *(_DWORD *)(v177 + 32) = v168;
                *(_QWORD *)(v177 + 40) = 0;
                *(_QWORD *)(v177 + 48) = 0;
                *(_QWORD *)v177 = v174;
                *(_QWORD *)(v177 + 8) = v174;
                *(_QWORD *)(v177 + 16) = v174;
                *(_WORD *)(v177 + 24) = 0;
                v193 = 0;
                v185 = v202;
                v176 = sub_14014F0E0(v173, &v185, v177);
              }
              v194 = 0;
              if ( v171 )
              {
                _InterlockedIncrement((volatile signed __int32 *)(v171 + 8));
                v172 = v200;
                v178 = v183;
              }
              else
              {
                v178 = v182;
              }
              *(_QWORD *)&v194 = v172;
              *((_QWORD *)&v194 + 1) = v171;
              *(_QWORD *)&v194 = *(_QWORD *)(v176 + 40);
              *(_QWORD *)(v176 + 40) = v172;
              *((_QWORD *)&v194 + 1) = *(_QWORD *)(v176 + 48);
              v179 = *((_QWORD *)&v194 + 1);
              *(_QWORD *)(v176 + 48) = v171;
              if ( v179 )
                sub_1401DC510(v179);
              v182 = v178 & 0xFFFFFFFE;
              LODWORD(v183) = v178 & 0xFFFFFFFE;
              if ( v171 )
                sub_1401DC510(v171);
            }
          }
          v164 += 40;
        }
        while ( v164 != v165 );
      }
    }
    LOBYTE(v7) = 1;
  }
  if ( v184 )
    (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v184 + 72LL))(v184, 1);
  return v7;
}

