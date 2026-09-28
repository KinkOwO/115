// reader_sub_1441E86C0

__int64 __fastcall sub_1441E86C0(__int64 a1, char a2)
{
  __int64 v3; // r15
  __int64 v4; // r8
  int v5; // esi
  _QWORD *v6; // rdi
  __int64 v7; // r14
  __int64 v8; // rax
  __int64 v9; // rax
  __int64 v10; // rax
  __int64 v11; // r8
  void (__fastcall *v12)(__int64, __int64); // rbx
  __int64 *v13; // r8
  __int64 v14; // rax
  __int64 v15; // rdx
  __int64 v16; // rcx
  volatile signed __int32 *v17; // rbx
  int *v18; // r12
  int *v19; // rbx
  int *v20; // r13
  __int64 v21; // rcx
  __int64 v22; // rdi
  __int64 v23; // rax
  void (__fastcall ***v24)(_QWORD); // rcx
  __int64 *v25; // rax
  __int64 v26; // rcx
  __int64 v27; // r8
  _QWORD *v28; // rax
  _WORD *v29; // r14
  unsigned __int16 *v30; // r11
  unsigned __int64 v31; // rcx
  __int64 v32; // rdi
  _WORD *v33; // r15
  _WORD *v34; // rdi
  unsigned __int16 v35; // r10
  __int64 v36; // rax
  _WORD *v37; // rcx
  _WORD *v38; // rdi
  unsigned __int64 v39; // r8
  unsigned __int16 *v40; // rax
  unsigned __int16 v41; // cx
  bool v42; // cc
  unsigned __int16 v43; // cx
  unsigned __int64 v44; // rdx
  __int64 v45; // rcx
  unsigned __int64 v46; // rdx
  __int64 v47; // rcx
  int v48; // ebx
  int v49; // esi
  __int64 v50; // r12
  __int64 v51; // rdi
  unsigned __int64 v52; // r13
  __int64 v53; // r14
  __int64 v54; // rdx
  __int64 v55; // r8
  __int64 v56; // rcx
  __int64 result; // rax
  unsigned int *v58; // rbx
  __int64 v59; // r13
  __int64 v60; // rcx
  void (__fastcall ***v61)(_QWORD); // rax
  __int64 v62; // r12
  __int64 v63; // rdi
  __int64 *v64; // rcx
  __int64 v65; // rdx
  int v66; // ebx
  char v67; // al
  __int64 v68; // rcx
  void (__fastcall ***v69)(_QWORD); // rax
  unsigned __int8 v70; // al
  void (__fastcall ***v71)(_QWORD); // rax
  unsigned __int8 v72; // r15
  _QWORD *v73; // rdx
  __int64 v74; // rdx
  __int64 v75; // rdx
  int v76; // eax
  __int64 v77; // rcx
  _QWORD *v78; // rdx
  __int64 v79; // rcx
  _QWORD *v80; // rdx
  __int64 v81; // rdx
  __int64 v82; // rcx
  __int64 v83; // rcx
  _DWORD *v84; // rax
  _DWORD *v85; // rcx
  __int64 v86; // rdx
  __int64 v87; // rcx
  __int64 v88; // rdi
  void (__fastcall *v89)(__int64, _QWORD); // rsi
  __int64 v90; // rbx
  void (__fastcall ***v91)(_QWORD); // rax
  __int64 *v92; // rax
  unsigned __int8 v93; // al
  __int64 v94; // rdx
  __int64 v95; // rcx
  __int64 v96; // rdx
  __int64 *v97; // rcx
  __int64 v98; // rax
  __int64 v99; // rdx
  __int64 v100; // rdx
  __int64 v101; // rcx
  volatile signed __int32 *v102; // rbx
  __int64 v103; // rdx
  __int64 v104; // rcx
  volatile signed __int32 *v105; // rbx
  __int64 v106; // rcx
  __int64 v107; // rdx
  volatile signed __int32 *v108; // rbx
  volatile signed __int32 *v109; // rbx
  _QWORD *v110; // rdi
  unsigned __int64 v111; // rsi
  __int64 v112; // rbx
  __int64 v113; // rcx
  __int64 v114; // rcx
  __int64 v115; // rdx
  _BYTE *v116; // rsi
  __int64 v117; // rdi
  __int64 v118; // rcx
  void (__fastcall *v119)(__int64, _QWORD); // rbx
  unsigned __int8 v120; // al
  unsigned __int64 v121; // rdx
  __int64 v122; // rcx
  __int64 v123; // rcx
  unsigned __int64 v124; // rdx
  int v126; // [rsp+2Ch] [rbp-DCh]
  int v128; // [rsp+38h] [rbp-D0h]
  unsigned __int64 v129; // [rsp+40h] [rbp-C8h]
  __int128 v130; // [rsp+48h] [rbp-C0h] BYREF
  int *v131; // [rsp+58h] [rbp-B0h]
  __int128 v132; // [rsp+60h] [rbp-A8h] BYREF
  __int128 v133; // [rsp+70h] [rbp-98h]
  int v134; // [rsp+80h] [rbp-88h]
  int v135; // [rsp+84h] [rbp-84h]
  bool v136; // [rsp+88h] [rbp-80h]
  _BYTE *v137; // [rsp+90h] [rbp-78h]
  __int64 v138; // [rsp+98h] [rbp-70h]
  __int64 v139; // [rsp+A0h] [rbp-68h]
  __int128 v140; // [rsp+A8h] [rbp-60h] BYREF
  unsigned __int64 v141; // [rsp+B8h] [rbp-50h]
  __int64 v142; // [rsp+C0h] [rbp-48h]
  __int128 v143; // [rsp+C8h] [rbp-40h]
  __int128 v144; // [rsp+D8h] [rbp-30h]
  __int128 v145; // [rsp+E8h] [rbp-20h] BYREF
  __int128 v146; // [rsp+F8h] [rbp-10h] BYREF
  __int64 v147; // [rsp+108h] [rbp+0h] BYREF
  volatile signed __int32 *v148; // [rsp+110h] [rbp+8h]
  __int64 v149; // [rsp+118h] [rbp+10h]
  char v150[8]; // [rsp+120h] [rbp+18h] BYREF
  void (__fastcall ***v151)(_QWORD); // [rsp+128h] [rbp+20h]
  void (__fastcall ***v152)(_QWORD); // [rsp+130h] [rbp+28h]
  void (__fastcall ***v153)(_QWORD); // [rsp+138h] [rbp+30h]
  void (__fastcall ***v154)(_QWORD); // [rsp+140h] [rbp+38h]
  char v155[8]; // [rsp+148h] [rbp+40h] BYREF
  __int64 v156; // [rsp+150h] [rbp+48h]
  __int128 *v157; // [rsp+158h] [rbp+50h]
  __int128 *v158; // [rsp+160h] [rbp+58h]
  __int128 *v159; // [rsp+168h] [rbp+60h]
  _BYTE v160[16]; // [rsp+170h] [rbp+68h] BYREF
  _BYTE v161[16]; // [rsp+180h] [rbp+78h] BYREF
  _QWORD v162[2]; // [rsp+190h] [rbp+88h] BYREF
  unsigned __int64 v163; // [rsp+1A0h] [rbp+98h]
  unsigned __int64 v164; // [rsp+1A8h] [rbp+A0h]
  _QWORD v165[2]; // [rsp+1B0h] [rbp+A8h] BYREF
  __int64 v166; // [rsp+1C0h] [rbp+B8h]
  unsigned __int64 v167; // [rsp+1C8h] [rbp+C0h]
  _QWORD v168[3]; // [rsp+1D0h] [rbp+C8h] BYREF
  unsigned __int64 v169; // [rsp+1E8h] [rbp+E0h]

  v149 = -2;
  v3 = a1;
  sub_1441E57B0();
  if ( a2 )
  {
    sub_146B26020(*(_QWORD *)(v3 + 112), 0, v4);
    if ( dword_14E662B30 > *(_DWORD *)(*((_QWORD *)NtCurrentTeb()->ThreadLocalStoragePointer
                                       + (unsigned int)dword_14F3BEE58)
                                     + 420620LL) )
    {
      sub_148860450(&dword_14E662B30);
      if ( dword_14E662B30 == -1 )
      {
        qword_14E662B10 = 0;
        qword_14E662B20 = 0;
        qword_14E662B28 = 7;
        sub_14885FFE8(sub_149024560);
        sub_1488603F0(&dword_14E662B30);
      }
    }
    sub_146F50610(*(_QWORD *)(v3 + 2352), &v147, *(unsigned int *)(v3 + 2344));
    v5 = 0;
    v6 = (_QWORD *)(v3 + 248);
    v7 = v3 + 152;
    do
    {
      v8 = sub_146E8C7D0(&unk_14A2322B8);
      v9 = sub_146E8CF20(v160, v8, (unsigned int)v5);
      v10 = sub_14014F430(v9);
      v11 = -1;
      do
        ++v11;
      while ( *(_WORD *)(v10 + 2 * v11) );
      sub_14014C8D0(&qword_14E662B10, v10);
      sub_146E8C910(v160);
      v12 = *(void (__fastcall **)(__int64, __int64))*(v6 - 12);
      v137 = v161;
      v13 = &qword_14E662B10;
      if ( (unsigned __int64)qword_14E662B28 >= 8 )
        v13 = (__int64 *)qword_14E662B10;
      v14 = sub_146EC8E30(v147, v161, v13);
      v12(v7, v14);
      (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)*v6 + 16LL))(*v6, 0);
      v16 = v6[4];
      if ( v16 )
        (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v16 + 16LL))(v16, 0);
      if ( (unsigned int)(*(_DWORD *)(v3 + 2344) - 1) <= 2 )
        LOBYTE(v15) = 1;
      else
        v15 = 0;
      (*(void (__fastcall **)(_QWORD, __int64))(*(_QWORD *)v6[35] + 24LL))(v6[35], v15);
      ++v5;
      v7 += 400;
      v6 += 50;
    }
    while ( v5 < 5 );
    v17 = v148;
    if ( v148 )
    {
      if ( _InterlockedExchangeAdd(v148 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v17)(v17);
        if ( _InterlockedExchangeAdd(v17 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v17 + 8LL))(v17);
      }
    }
  }
  v130 = 0;
  v18 = 0;
  v131 = 0;
  v19 = *(int **)(v3 + 16);
  v20 = 0;
  while ( v19 != *(int **)(v3 + 24) )
  {
    if ( (unsigned int)sub_14206BB60(*(_QWORD *)(v3 + 40)) == 2 && sub_1444EC2C0((__int64)v19)
      || (unsigned int)sub_14206BB60(*(_QWORD *)(v3 + 40)) == 3 && !sub_1444EC2C0((__int64)v19) )
    {
      goto LABEL_75;
    }
    v21 = *(_QWORD *)(v3 + 40);
    if ( !v21 || (unsigned int)sub_14206BB60(v21) != 1 )
      goto LABEL_32;
    v22 = qword_14E638F28;
    if ( !qword_14E638F28 )
    {
      v23 = sub_146E8BA20(1472);
      v137 = (_BYTE *)v23;
      if ( v23 )
        v24 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v23);
      else
        v24 = 0;
      qword_14E638F28 = (__int64)v24;
      (**v24)(v24);
      v22 = qword_14E638F28;
    }
    v25 = (__int64 *)sub_1441C3DB0(*(_QWORD *)(v3 + 8), v150);
    if ( !sub_1444EC320(v22, *v19, *v25) )
    {
LABEL_75:
      v19 += 6;
    }
    else
    {
LABEL_32:
      v26 = *(_QWORD *)(v3 + 56);
      if ( !v26 || !*(_BYTE *)(v3 + 104) )
        goto LABEL_72;
      (*(void (__fastcall **)(__int64, _QWORD *))(*(_QWORD *)v26 + 688LL))(v26, v162);
      v28 = sub_1444EB940(v19, v168, v27);
      v29 = v28;
      v30 = (unsigned __int16 *)v162;
      if ( v164 >= 8 )
        v30 = (unsigned __int16 *)v162[0];
      v31 = v28[2];
      if ( v28[3] >= 8u )
        v29 = (_WORD *)*v28;
      if ( v163 > v31 )
        goto LABEL_56;
      if ( v163 )
      {
        v33 = &v29[v31 - v163 + 1];
        v34 = v29;
        v35 = *v30;
        while ( 1 )
        {
          v36 = v33 - v34;
          v37 = 0;
          if ( v36 )
          {
            if ( *v34 == v35 )
            {
LABEL_46:
              v37 = v34;
            }
            else
            {
              while ( v36 != 1 )
              {
                --v36;
                if ( *++v34 == v35 )
                  goto LABEL_46;
              }
            }
          }
          v38 = v37;
          if ( !v37 )
            break;
          v39 = v163;
          v40 = v30;
          v41 = *v37;
          if ( v41 >= v35 )
          {
            v42 = v41 <= v35;
            while ( v42 )
            {
              if ( v39 == 1 )
              {
                v32 = v38 - v29;
                v3 = a1;
                goto LABEL_57;
              }
              --v39;
              v43 = *(unsigned __int16 *)((char *)++v40 + (char *)v38 - (char *)v30);
              v42 = v43 <= *v40;
              if ( v43 < *v40 )
                break;
            }
          }
          v34 = v38 + 1;
        }
        v3 = a1;
LABEL_56:
        v32 = -1;
        goto LABEL_57;
      }
      v32 = 0;
LABEL_57:
      if ( v169 >= 8 )
      {
        v44 = 2 * v169 + 2;
        v45 = v168[0];
        if ( v44 >= 0x1000 )
        {
          v44 = 2 * v169 + 41;
          v45 = *(_QWORD *)(v168[0] - 8LL);
          if ( (unsigned __int64)(v168[0] - v45 - 8) > 0x1F )
            sub_148AAF304(v45, v44);
        }
        sub_146E9F3A0(v45, v44);
      }
      v168[2] = 0;
      v169 = 7;
      LOWORD(v168[0]) = 0;
      if ( v32 == -1 )
      {
        if ( v164 >= 8 )
        {
          v46 = 2 * v164 + 2;
          v47 = v162[0];
          if ( v46 >= 0x1000 )
          {
            v46 = 2 * v164 + 41;
            v47 = *(_QWORD *)(v162[0] - 8LL);
            if ( (unsigned __int64)(v162[0] - v47 - 8) > 0x1F )
              goto LABEL_194;
          }
          sub_146E9F3A0(v47, v46);
        }
        v163 = 0;
        v164 = 7;
        LOWORD(v162[0]) = 0;
        v19 += 6;
      }
      else
      {
        if ( v164 >= 8 )
        {
          v46 = 2 * v164 + 2;
          v47 = v162[0];
          if ( v46 >= 0x1000 )
          {
            v46 = 2 * v164 + 41;
            v47 = *(_QWORD *)(v162[0] - 8LL);
            if ( (unsigned __int64)(v162[0] - v47 - 8) > 0x1F )
LABEL_194:
              sub_148AAF304(v47, v46);
          }
          sub_146E9F3A0(v47, v46);
        }
        v163 = 0;
        v164 = 7;
        LOWORD(v162[0]) = 0;
LABEL_72:
        if ( v20 == v18 )
        {
          sub_140154010(&v130, v20, v19);
          v18 = v131;
          v20 = (int *)*((_QWORD *)&v130 + 1);
          goto LABEL_75;
        }
        *v20++ = *v19;
        *((_QWORD *)&v130 + 1) = v20;
        v19 += 6;
      }
    }
  }
  v48 = 5 * sub_1421B2800(*(_QWORD *)(v3 + 112));
  v126 = v48;
  v49 = 0;
  v128 = 0;
  v50 = v48 - 5LL;
  v138 = v50;
  v51 = 0;
  v139 = 0;
  v52 = (__int64)((__int64)v20 - v130) >> 2;
  v129 = v52;
  do
  {
    v53 = 400 * v51;
    (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(400 * v51 + v3 + 200) + 16LL))(
      *(_QWORD *)(400 * v51 + v3 + 200),
      0);
    v137 = (_BYTE *)(v3 + 16 * v51);
    v56 = *((_QWORD *)v137 + 269);
    if ( v56 )
      (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v56 + 16LL))(v56, 0);
    result = (unsigned int)(v48 + v49 - 5);
    if ( v52 > (int)result )
    {
      v58 = (unsigned int *)(v130 + 4 * (v50 + v51));
      result = sub_1444EBAB0(*v58, v54, v55);
      v59 = result;
      if ( result && !*(_DWORD *)(result + 8) )
      {
        v60 = qword_14E638F28;
        if ( !qword_14E638F28 )
        {
          v61 = (void (__fastcall ***)(_QWORD))sub_146E8BA20(1472);
          v151 = v61;
          if ( v61 )
            v61 = (void (__fastcall ***)(_QWORD))sub_1444E81C0((__int64)v61);
          qword_14E638F28 = (__int64)v61;
          (**v61)(v61);
          v60 = qword_14E638F28;
        }
        result = (__int64)sub_1444EBD90(v60, 0, *v58);
        v62 = result;
        if ( result )
        {
          v63 = a1;
          v64 = *(__int64 **)(v53 + a1 + 200);
          v65 = *v64;
          LOBYTE(v65) = 1;
          (*(void (__fastcall **)(__int64 *, __int64))(*v64 + 16))(v64, v65);
          v66 = *v58;
          *(_DWORD *)(v53 + a1 + 192) = v66;
          v67 = sub_1441D10E0(a1, 2);
          v68 = qword_14E638F28;
          if ( v67 == 1 )
          {
            if ( !qword_14E638F28 )
            {
              v69 = (void (__fastcall ***)(_QWORD))sub_146E8BA20(1472);
              v152 = v69;
              if ( v69 )
                v69 = (void (__fastcall ***)(_QWORD))sub_1444E81C0((__int64)v69);
              qword_14E638F28 = (__int64)v69;
              (**v69)(v69);
              v66 = *(_DWORD *)(v53 + a1 + 192);
              v68 = qword_14E638F28;
            }
            v70 = sub_1444EC570(v68, v66);
          }
          else
          {
            if ( !qword_14E638F28 )
            {
              v71 = (void (__fastcall ***)(_QWORD))sub_146E8BA20(1472);
              v153 = v71;
              if ( v71 )
                v71 = (void (__fastcall ***)(_QWORD))sub_1444E81C0((__int64)v71);
              qword_14E638F28 = (__int64)v71;
              (**v71)(v71);
              v66 = *(_DWORD *)(v53 + a1 + 192);
              v68 = qword_14E638F28;
            }
            v70 = sub_1444EC5B0(v68, 0, v66);
          }
          v72 = v70;
          sub_1444EB5F0(v62, v165);
          v73 = v165;
          if ( v167 >= 8 )
            v73 = (_QWORD *)v165[0];
          (*(void (__fastcall **)(_QWORD, _QWORD *))(**(_QWORD **)(v53 + a1 + 368) + 688LL))(
            *(_QWORD *)(v53 + a1 + 368),
            v73);
          LOBYTE(v74) = v166 != 0;
          (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(v53 + a1 + 368) + 16LL))(
            *(_QWORD *)(v53 + a1 + 368),
            v74);
          v76 = *(_DWORD *)(v59 + 12);
          if ( v76 == 1 )
          {
            v77 = *(_QWORD *)(v53 + a1 + 280);
            if ( v77 )
            {
              LOBYTE(v75) = 1;
              (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v77 + 16LL))(v77, v75);
              v78 = (_QWORD *)(v59 + 352);
              if ( *(_QWORD *)(v59 + 376) >= 8u )
                v78 = (_QWORD *)*v78;
              sub_146EECE20(*(_QWORD *)(v53 + a1 + 280), v78);
              sub_146EECBB0(*(_QWORD *)(v53 + a1 + 280), *(unsigned int *)(v59 + 384));
            }
          }
          else if ( v76 == 2 )
          {
            v79 = *(_QWORD *)(v53 + a1 + 248);
            if ( v79 )
            {
              LOBYTE(v75) = 1;
              (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v79 + 16LL))(v79, v75);
              v80 = (_QWORD *)(v59 + 352);
              if ( *(_QWORD *)(v59 + 376) >= 8u )
                v80 = (_QWORD *)*v80;
              sub_146EECE20(*(_QWORD *)(v53 + a1 + 248), v80);
              sub_146EECBB0(*(_QWORD *)(v53 + a1 + 248), *(unsigned int *)(v59 + 384));
              sub_146EED6B0(
                *(_QWORD *)(v53 + a1 + 248),
                *(unsigned __int8 *)(v59 + 388),
                *(unsigned __int8 *)(v59 + 392),
                *(unsigned __int8 *)(v59 + 396));
            }
          }
          (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(v53 + a1 + 496) + 16LL))(
            *(_QWORD *)(v53 + a1 + 496),
            0);
          (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(v53 + a1 + 512) + 16LL))(
            *(_QWORD *)(v53 + a1 + 512),
            0);
          LOBYTE(v81) = 1;
          if ( *(_BYTE *)(v62 + 20) )
            v82 = *(_QWORD *)(a1 + v53 + 496);
          else
            v82 = *(_QWORD *)(a1 + v53 + 512);
          (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v82 + 16LL))(v82, v81);
          v83 = *(_QWORD *)(a1 + 8);
          if ( !v83 || (v84 = *(_DWORD **)(v83 + 3328), v85 = *(_DWORD **)(v83 + 3336), v84 == v85) )
          {
LABEL_118:
            v86 = 0;
          }
          else
          {
            v86 = *(unsigned int *)(a1 + v53 + 192);
            while ( *v84 != (_DWORD)v86 )
            {
              if ( ++v84 == v85 )
                goto LABEL_118;
            }
            LOBYTE(v86) = 1;
          }
          (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(v53 + a1 + 384) + 16LL))(
            *(_QWORD *)(v53 + a1 + 384),
            v86);
          (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(v53 + a1 + 432) + 16LL))(
            *(_QWORD *)(v53 + a1 + 432),
            v72);
          v87 = *(_QWORD *)(a1 + 8);
          if ( v87 )
          {
            v88 = *(_QWORD *)(v53 + a1 + 176);
            v89 = *(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v88 + 16LL);
            v90 = qword_14E638F28;
            if ( !qword_14E638F28 )
            {
              v91 = (void (__fastcall ***)(_QWORD))sub_146E8BA20(1472);
              v154 = v91;
              if ( v91 )
                v91 = (void (__fastcall ***)(_QWORD))sub_1444E81C0((__int64)v91);
              qword_14E638F28 = (__int64)v91;
              (**v91)(v91);
              v87 = *(_QWORD *)(a1 + 8);
              v90 = qword_14E638F28;
            }
            v92 = (__int64 *)sub_1441C3DB0(v87, v155);
            v93 = sub_1444EC320(v90, *(_DWORD *)(v53 + a1 + 192), *v92);
            v89(v88, v93);
            v63 = a1;
          }
          if ( sub_1444EC2C0(v62) )
          {
            (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(v53 + v63 + 400) + 16LL))(
              *(_QWORD *)(v53 + v63 + 400),
              0);
            (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(v53 + v63 + 416) + 16LL))(
              *(_QWORD *)(v53 + v63 + 416),
              0);
            v95 = *(_QWORD *)(v53 + v63 + 448);
            LOBYTE(v96) = 1;
          }
          else
          {
            v97 = *(__int64 **)(v63 + v53 + 400);
            v98 = *v97;
            if ( v72 )
            {
              (*(void (__fastcall **)(__int64 *, _QWORD))(v98 + 16))(v97, 0);
              LOBYTE(v99) = 1;
            }
            else
            {
              LOBYTE(v94) = 1;
              (*(void (__fastcall **)(__int64 *, __int64))(v98 + 16))(v97, v94);
              v99 = 0;
            }
            (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(v63 + v53 + 416) + 16LL))(
              *(_QWORD *)(v63 + v53 + 416),
              v99);
            v96 = 0;
            v95 = *(_QWORD *)(v63 + v53 + 448);
          }
          (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v95 + 16LL))(v95, v96);
          if ( *(_QWORD *)(v63 + 8) )
          {
            v132 = 0;
            v133 = 0;
            v134 = 0;
            v135 = -1;
            v136 = 0;
            v136 = sub_1444EC2C0(v62);
            v143 = 0;
            v100 = *(_QWORD *)(v53 + v63 + 168);
            if ( v100 )
            {
              _InterlockedIncrement((volatile signed __int32 *)(v100 + 8));
              v100 = *(_QWORD *)(v53 + v63 + 168);
            }
            v101 = *(_QWORD *)(v53 + v63 + 160);
            v143 = v132;
            *(_QWORD *)&v132 = v101;
            v102 = (volatile signed __int32 *)*((_QWORD *)&v132 + 1);
            *((_QWORD *)&v132 + 1) = v100;
            if ( *((_QWORD *)&v143 + 1) )
            {
              if ( _InterlockedExchangeAdd(v102 + 2, 0xFFFFFFFF) == 1 )
              {
                (**(void (__fastcall ***)(volatile signed __int32 *))v102)(v102);
                if ( _InterlockedExchangeAdd(v102 + 3, 0xFFFFFFFF) == 1 )
                  (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v102 + 8LL))(v102);
              }
            }
            v144 = 0;
            v103 = *(_QWORD *)(v53 + v63 + 184);
            if ( v103 )
            {
              _InterlockedIncrement((volatile signed __int32 *)(v103 + 8));
              v103 = *(_QWORD *)(v53 + v63 + 184);
            }
            v104 = *(_QWORD *)(v53 + v63 + 176);
            v144 = v133;
            *(_QWORD *)&v133 = v104;
            v105 = (volatile signed __int32 *)*((_QWORD *)&v133 + 1);
            *((_QWORD *)&v133 + 1) = v103;
            if ( *((_QWORD *)&v144 + 1) )
            {
              if ( _InterlockedExchangeAdd(v105 + 2, 0xFFFFFFFF) == 1 )
              {
                (**(void (__fastcall ***)(volatile signed __int32 *))v105)(v105);
                if ( _InterlockedExchangeAdd(v105 + 3, 0xFFFFFFFF) == 1 )
                  (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v105 + 8LL))(v105);
              }
            }
            v134 = *(_DWORD *)(v53 + v63 + 192);
            v135 = 0;
            v106 = *(_QWORD *)(v63 + 8) + 3280LL;
            v107 = *(_QWORD *)(*(_QWORD *)(v63 + 8) + 3288LL);
            if ( v107 == *(_QWORD *)(*(_QWORD *)(v63 + 8) + 3296LL) )
            {
              sub_1441B7240(v106, v107, &v132);
            }
            else
            {
              v156 = *(_QWORD *)(*(_QWORD *)(v63 + 8) + 3288LL);
              *(_QWORD *)v107 = 0;
              *(_QWORD *)(v107 + 8) = 0;
              if ( *((_QWORD *)&v132 + 1) )
                _InterlockedIncrement((volatile signed __int32 *)(*((_QWORD *)&v132 + 1) + 8LL));
              *(_OWORD *)v107 = v132;
              *(_QWORD *)(v107 + 16) = 0;
              *(_QWORD *)(v107 + 24) = 0;
              if ( *((_QWORD *)&v133 + 1) )
                _InterlockedIncrement((volatile signed __int32 *)(*((_QWORD *)&v133 + 1) + 8LL));
              *(_OWORD *)(v107 + 16) = v133;
              *(_DWORD *)(v107 + 32) = v134;
              *(_DWORD *)(v107 + 36) = v135;
              *(_BYTE *)(v107 + 40) = v136;
              *(_QWORD *)(v106 + 8) += 48LL;
            }
            v108 = (volatile signed __int32 *)*((_QWORD *)&v133 + 1);
            if ( *((_QWORD *)&v133 + 1) )
            {
              if ( _InterlockedExchangeAdd((volatile signed __int32 *)(*((_QWORD *)&v133 + 1) + 8LL), 0xFFFFFFFF) == 1 )
              {
                (**(void (__fastcall ***)(volatile signed __int32 *))v108)(v108);
                if ( _InterlockedExchangeAdd(v108 + 3, 0xFFFFFFFF) == 1 )
                  (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v108 + 8LL))(v108);
              }
            }
            v109 = (volatile signed __int32 *)*((_QWORD *)&v132 + 1);
            if ( *((_QWORD *)&v132 + 1) )
            {
              if ( _InterlockedExchangeAdd((volatile signed __int32 *)(*((_QWORD *)&v132 + 1) + 8LL), 0xFFFFFFFF) == 1 )
              {
                (**(void (__fastcall ***)(volatile signed __int32 *))v109)(v109);
                if ( _InterlockedExchangeAdd(v109 + 3, 0xFFFFFFFF) == 1 )
                  (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v109 + 8LL))(v109);
              }
            }
          }
          v157 = &v140;
          *(_QWORD *)&v140 = 0;
          v141 = 0;
          v142 = 0;
          v110 = (_QWORD *)(v59 + 192);
          v111 = *(_QWORD *)(v59 + 208);
          if ( *(_QWORD *)(v59 + 216) >= 8u )
            v110 = (_QWORD *)*v110;
          if ( v111 >= 8 )
          {
            v112 = v111 | 7;
            if ( (v111 | 7) > 0x7FFFFFFFFFFFFFFELL )
              v112 = 0x7FFFFFFFFFFFFFFELL;
            *(_QWORD *)&v140 = sub_14014CB50(&v140, v112 + 1);
            sub_148AA1E60(v140, v110, 2 * v111 + 2);
            v142 = v112;
          }
          else
          {
            v140 = *(_OWORD *)v110;
            v142 = 7;
          }
          v141 = v111;
          v158 = &v145;
          v145 = 0;
          v3 = a1;
          v113 = *(_QWORD *)(v53 + a1 + 328);
          if ( v113 )
          {
            _InterlockedIncrement((volatile signed __int32 *)(v113 + 8));
            v113 = *(_QWORD *)(v53 + a1 + 328);
          }
          *(_QWORD *)&v145 = *(_QWORD *)(v53 + a1 + 320);
          *((_QWORD *)&v145 + 1) = v113;
          v159 = &v146;
          v146 = 0;
          v114 = *(_QWORD *)(v53 + a1 + 344);
          if ( v114 )
          {
            _InterlockedIncrement((volatile signed __int32 *)(v114 + 8));
            v114 = *(_QWORD *)(v53 + a1 + 344);
          }
          *(_QWORD *)&v146 = *(_QWORD *)(v53 + a1 + 336);
          *((_QWORD *)&v146 + 1) = v114;
          sub_1441EC970(&v146, &v145, &v140, 0);
          (*(void (__fastcall **)(_QWORD))(**(_QWORD **)(v53 + a1 + 352) + 688LL))(*(_QWORD *)(v53 + a1 + 352));
          v115 = **(unsigned int **)(*(_QWORD *)(a1 + 8) + 3352LL);
          LOBYTE(v115) = (_DWORD)v115 == *(_DWORD *)(v53 + a1 + 192);
          result = (*(__int64 (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(v53 + a1 + 480) + 16LL))(
                     *(_QWORD *)(v53 + a1 + 480),
                     v115);
          v116 = v137;
          v117 = *((_QWORD *)v137 + 269);
          if ( v117 )
          {
            if ( *((_QWORD *)v137 + 279) )
            {
              v118 = *(_QWORD *)(v53 + a1 + 200);
              if ( v118 )
              {
                v119 = *(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v117 + 16LL);
                v120 = sub_141FB6530(v118);
                v119(v117, v120);
                result = sub_146F55940(*((_QWORD *)v116 + 279));
              }
            }
          }
          if ( v167 >= 8 )
          {
            v121 = 2 * v167 + 2;
            v122 = v165[0];
            if ( v121 >= 0x1000 )
            {
              v121 = 2 * v167 + 41;
              v122 = *(_QWORD *)(v165[0] - 8LL);
              if ( (unsigned __int64)(v165[0] - v122 - 8) > 0x1F )
                sub_148AAF304(v122, v121);
            }
            result = sub_146E9F3A0(v122, v121);
          }
          v166 = 0;
          v167 = 7;
          LOWORD(v165[0]) = 0;
          v51 = v139;
          v49 = v128;
        }
        v50 = v138;
      }
      v52 = v129;
      v48 = v126;
    }
    v128 = ++v49;
    v139 = ++v51;
  }
  while ( v51 < 5 );
  if ( a2 )
    result = sub_146B26100(*(_QWORD *)(v3 + 112), (unsigned int)v52 + (v52 != 5 * (v52 / 5)));
  v123 = v130;
  if ( (_QWORD)v130 )
  {
    v124 = ((unsigned __int64)v131 - v130) & 0xFFFFFFFFFFFFFFFCuLL;
    if ( v124 >= 0x1000 )
    {
      v124 += 39LL;
      v123 = *(_QWORD *)(v130 - 8);
      if ( (unsigned __int64)(v130 - v123 - 8) > 0x1F )
        sub_148AAF304(v123, v124);
    }
    result = sub_146E9F3A0(v123, v124);
    v130 = 0;
    v131 = 0;
  }
  return result;
}

