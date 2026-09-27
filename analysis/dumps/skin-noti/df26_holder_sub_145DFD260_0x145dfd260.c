// holder_sub_145DFD260_0x145dfd260

__int64 __fastcall sub_145DFD260(__int64 a1)
{
  __int64 v1; // r14
  __int64 v2; // rbx
  __int64 v3; // rax
  int v4; // r13d
  __int64 v5; // r13
  __int64 v6; // rcx
  int v7; // r15d
  __int64 v8; // rax
  void (__fastcall ***v9)(_QWORD); // rcx
  __int64 v10; // rdi
  int v11; // esi
  int v12; // eax
  __int64 v13; // r15
  __int64 v14; // rdi
  volatile signed __int32 *v15; // rcx
  __int64 result; // rax
  __int64 v17; // rax
  __int64 v18; // rsi
  char v19; // r15
  __int64 v20; // rax
  _QWORD *v21; // rbx
  int i; // edi
  __int64 v23; // rax
  __int64 v24; // rbx
  unsigned int v25; // edi
  __int64 v26; // r8
  unsigned __int64 v27; // rdx
  __int64 v28; // r12
  __int64 v29; // rax
  __int64 v30; // rax
  __int64 v31; // rcx
  __int64 v32; // r8
  __int64 v33; // rcx
  __int64 v34; // rdx
  __int64 v35; // rax
  __int64 v36; // rax
  __int64 v37; // rcx
  __int64 v38; // rbx
  __int64 v39; // rcx
  __int64 v40; // rax
  __int64 v41; // rax
  __int64 v42; // rcx
  __int64 v43; // rax
  unsigned int v44; // eax
  __int64 v45; // rcx
  __int64 v46; // rax
  __int64 v47; // rax
  __int64 v48; // rcx
  __int64 v49; // rcx
  __int64 v50; // rax
  __int64 v51; // rax
  __int64 v52; // rcx
  __int64 v53; // rcx
  __int64 v54; // rax
  __int64 v55; // r13
  __int64 *v56; // r15
  int v57; // eax
  __int64 *v58; // rcx
  __int64 *v59; // rdx
  int v60; // edi
  __int64 *v61; // rsi
  __int64 *v62; // rcx
  __int64 *v63; // rdx
  __int64 v64; // rbx
  unsigned int v65; // eax
  _QWORD *v66; // rax
  _QWORD *v67; // rdi
  __int64 v68; // rsi
  _QWORD *v69; // rax
  _QWORD *v70; // rbx
  __int64 v71; // rax
  __int64 v72; // rax
  __int64 v73; // rdx
  _QWORD *v74; // rax
  _QWORD *v75; // rbx
  __int64 v76; // rax
  __int64 v77; // rax
  __int64 v78; // rdx
  __int64 **v79; // rax
  __int64 j; // rax
  __int64 *k; // rcx
  int v82; // edi
  __int64 v83; // r8
  __int64 v84; // rcx
  __int64 v85; // rdx
  __int64 v86; // rbx
  __int64 v87; // rax
  __int64 v88; // rax
  __int64 v89; // rcx
  _QWORD *v90; // rdi
  _QWORD *v91; // rbx
  __int64 v92; // rax
  __int64 v93; // rax
  _DWORD *v94; // rsi
  __int64 v95; // r15
  int v96; // esi
  int v97; // r12d
  unsigned __int16 v98; // ax
  unsigned __int16 v99; // ax
  __int64 v100; // rcx
  __int64 v101; // rax
  void (__fastcall ***v102)(_QWORD); // rcx
  __int64 v103; // rdx
  unsigned __int64 v104; // r8
  __int64 v105; // r9
  _QWORD *v106; // rcx
  __int64 v107; // rax
  void (__fastcall ***v108)(_QWORD); // rcx
  __int64 v109; // rcx
  __int64 v110; // rax
  void (__fastcall ***v111)(_QWORD); // rcx
  __int64 v112; // rax
  void (__fastcall ***v113)(_QWORD); // rcx
  __int64 v114; // rcx
  __int64 v115; // rax
  void (__fastcall ***v116)(_QWORD); // rcx
  int v117; // eax
  __int64 v118; // rdi
  unsigned int v119; // ebx
  int v120; // [rsp+40h] [rbp-49h] BYREF
  __int64 v121; // [rsp+48h] [rbp-41h]
  __int64 v122; // [rsp+50h] [rbp-39h] BYREF
  __int64 *v123; // [rsp+60h] [rbp-29h] BYREF
  int v124; // [rsp+68h] [rbp-21h]
  __int64 v125; // [rsp+80h] [rbp-9h]
  __int64 v126; // [rsp+88h] [rbp-1h]
  __int64 v127; // [rsp+90h] [rbp+7h]
  __int64 v128; // [rsp+98h] [rbp+Fh]
  char v130; // [rsp+F8h] [rbp+6Fh]
  unsigned int v131; // [rsp+100h] [rbp+77h] BYREF
  int v132; // [rsp+108h] [rbp+7Fh] BYREF

  v127 = -2;
  v1 = a1;
  v2 = *(_QWORD *)(a1 + 88);
  v3 = *(_QWORD *)(a1 + 96);
  v4 = 0;
  if ( v2 != v3 )
  {
    v5 = v2 + 24;
    do
    {
      v6 = *(_QWORD *)(v5 - 16);
      if ( v6 && *(_DWORD *)(v6 + 8) && *(_QWORD *)(v5 - 8) )
      {
        v2 += 24;
        v5 += 24;
      }
      else
      {
        v7 = qword_14E6343D0;
        if ( !qword_14E6343D0 )
        {
          v8 = sub_146E8BA20(72);
          if ( v8 )
            v9 = (void (__fastcall ***)(_QWORD))sub_146E93360(v8);
          else
            v9 = 0;
          qword_14E6343D0 = (__int64)v9;
          (**v9)(v9);
          v7 = qword_14E6343D0;
        }
        v10 = sub_146E8C7D0(&unk_14A9719F0);
        v11 = sub_146E8C7D0(&unk_14A971A68);
        v12 = sub_146E8C7D0(&unk_14A971950);
        sub_146E938E0(v7, 0, v12, v11, 5039, (__int64)&qword_14E6CC9A0, v10);
        v13 = *(_QWORD *)(v1 + 96);
        v14 = v5;
        if ( v5 != v13 )
        {
          do
          {
            sub_1401F10B0(v2 - v5 + v14, v14);
            v14 += 24;
          }
          while ( v14 != v13 );
          v13 = *(_QWORD *)(v1 + 96);
        }
        v121 = v13 - 24;
        v15 = *(volatile signed __int32 **)(v13 - 24 + 8);
        if ( v15 && _InterlockedExchangeAdd(v15 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v15 + 8LL))(v15);
        *(_QWORD *)(v1 + 96) -= 24LL;
        v3 = *(_QWORD *)(v1 + 96);
      }
    }
    while ( v2 != v3 );
    v4 = 0;
  }
  result = sub_145F14EE0(qword_14E683C20, 0xFFFFFFFFLL);
  if ( (_BYTE)result )
  {
    *(_BYTE *)(v1 + 1136) = 0;
    v17 = sub_1459A9080(qword_14E66C090);
    v18 = v17;
    v121 = v17;
    v19 = 0;
    v130 = 0;
    if ( v17 && sub_140157C80(v17) )
    {
      v20 = sub_140157C80(v18);
      v19 = sub_145B34CF0(v20);
      v130 = v19;
    }
    if ( *(_QWORD *)(v1 + 1128) )
    {
      v21 = *(_QWORD **)(v1 + 1120);
      sub_1401E87E0(v1 + 1120, v1 + 1120, v21[1]);
      v21[1] = v21;
      *v21 = v21;
      v21[2] = v21;
      *(_QWORD *)(v1 + 1128) = 0;
    }
    if ( v19 )
    {
      for ( i = 0; i < 8; ++i )
      {
        v23 = sub_145F13060(qword_14E683C20, (unsigned int)i);
        v24 = v23;
        if ( v23 )
        {
          if ( !(*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v23 + 304LL))(v23) )
          {
            sub_145CD8EE0(v24, &v122, &v131);
            if ( v18 )
            {
              if ( sub_145B313A0(v18, (unsigned int)v122, v131, 0xFFFF) == v1 )
                sub_145DE47A0(v1, v24);
            }
          }
        }
      }
    }
    v25 = 0;
    v131 = 0;
    v26 = *(_QWORD *)(v1 + 88);
    v27 = (*(_QWORD *)(v1 + 96) - v26) / 24;
    if ( v27 )
    {
      v28 = 0;
      do
      {
        v29 = *(_QWORD *)(v28 + v26 + 8);
        if ( !v29 || !*(_DWORD *)(v29 + 8) )
          goto LABEL_102;
        v30 = *(_QWORD *)(v28 + v26 + 16);
        v31 = v30 - 48;
        if ( !v30 )
          v31 = 0;
        if ( !v31 || !(*(unsigned __int8 (__fastcall **)(__int64, unsigned __int64))(*(_QWORD *)v31 + 328LL))(v31, v27) )
          goto LABEL_102;
        v32 = *(_QWORD *)(v1 + 88);
        v33 = *(_QWORD *)(v32 + v28 + 8);
        if ( v33 && *(_DWORD *)(v33 + 8) )
          v34 = *(_QWORD *)(v32 + v28 + 16);
        else
          v34 = 0;
        v35 = v34 + 300;
        if ( !v34 )
          v35 = 348;
        if ( (*(_DWORD *)v35 & 0x211) == 0x211 && v33 && *(_DWORD *)(v33 + 8) )
        {
          v36 = *(_QWORD *)(v32 + v28 + 16);
          v37 = v36 - 48;
          if ( !v36 )
            v37 = 0;
          if ( v37 && (unsigned __int8)sub_145D72850() == 1 )
            goto LABEL_102;
        }
        v38 = qword_14F1C0CA0;
        v39 = *(_QWORD *)(v1 + 88);
        v40 = *(_QWORD *)(v39 + v28 + 8);
        if ( v40 && *(_DWORD *)(v40 + 8) )
          v41 = *(_QWORD *)(v39 + v28 + 16);
        else
          v41 = 0;
        v42 = v41 - 48;
        if ( !v41 )
          v42 = 0;
        v43 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v42 + 136LL))(v42);
        v44 = sub_145A01FA0(*(unsigned int *)(v43 + 12));
        if ( (unsigned __int8)sub_146EB6320(v38, v44) )
          goto LABEL_102;
        v45 = *(_QWORD *)(v1 + 88);
        v46 = *(_QWORD *)(v45 + v28 + 8);
        if ( v46 && *(_DWORD *)(v46 + 8) )
          v47 = *(_QWORD *)(v45 + v28 + 16);
        else
          v47 = 0;
        v48 = v47 - 48;
        if ( !v47 )
          v48 = 0;
        (*(void (__fastcall **)(__int64))(*(_QWORD *)v48 + 112LL))(v48);
        if ( !v18 || !v19 )
          goto LABEL_102;
        v49 = *(_QWORD *)(v1 + 88);
        v50 = *(_QWORD *)(v49 + v28 + 8);
        if ( v50 && *(_DWORD *)(v50 + 8) )
          v51 = *(_QWORD *)(v49 + v28 + 16);
        else
          v51 = 0;
        v52 = v51 - 48;
        if ( !v51 )
          v52 = 0;
        if ( !(*(unsigned __int8 (__fastcall **)(__int64))(*(_QWORD *)v52 + 2048LL))(v52) )
          goto LABEL_102;
        v53 = *(_QWORD *)(v1 + 88);
        v54 = *(_QWORD *)(v53 + v28 + 8);
        if ( !v54 || !*(_DWORD *)(v54 + 8) )
          goto LABEL_102;
        v55 = *(_QWORD *)(v53 + v28 + 16);
        if ( v55 && v55 != 48 )
        {
          v56 = (__int64 *)(v1 + 1120);
          v57 = sub_145B8B0A0(v55 - 48);
          v58 = *(__int64 **)(*(_QWORD *)(v1 + 1120) + 8LL);
          v59 = *(__int64 **)(v1 + 1120);
          while ( !*((_BYTE *)v58 + 25) )
          {
            if ( *((_DWORD *)v58 + 8) >= v57 )
            {
              v59 = v58;
              v58 = (__int64 *)*v58;
            }
            else
            {
              v58 = (__int64 *)v58[2];
            }
          }
          if ( *((_BYTE *)v59 + 25) || v57 < *((_DWORD *)v59 + 8) || v59 == *(__int64 **)(v1 + 1120) )
          {
            v60 = sub_145B8B0A0(v55 - 48);
            v61 = (__int64 *)*v56;
            v62 = *(__int64 **)(*v56 + 8);
            v123 = v62;
            v124 = 0;
            v63 = v61;
            while ( !*((_BYTE *)v62 + 25) )
            {
              v123 = v62;
              if ( *((_DWORD *)v62 + 8) >= v60 )
              {
                v124 = 1;
                v63 = v62;
                v62 = (__int64 *)*v62;
              }
              else
              {
                v124 = 0;
                v62 = (__int64 *)v62[2];
              }
            }
            if ( *((_BYTE *)v63 + 25) || v60 < *((_DWORD *)v63 + 8) )
            {
              if ( *(_QWORD *)(v1 + 1128) == 0x3FFFFFFFFFFFFFFLL )
                sub_14014F360(0x3FFFFFFFFFFFFFFLL, v63);
              v125 = v1 + 1120;
              v126 = 0;
              v64 = sub_146E8BA20(64);
              v126 = v64;
              v122 = v64 + 32;
              *(_DWORD *)(v64 + 32) = v60;
              v128 = v64 + 40;
              sub_146EA47F0(v64 + 40, v55);
              *(_QWORD *)v64 = v61;
              *(_QWORD *)(v64 + 8) = v61;
              *(_QWORD *)(v64 + 16) = v61;
              *(_WORD *)(v64 + 24) = 0;
              v4 = 0;
              v126 = 0;
              sub_14014F0E0(v1 + 1120, &v123, v64);
              v18 = v121;
              v25 = v131;
              v19 = v130;
              goto LABEL_102;
            }
            v18 = v121;
            v25 = v131;
          }
          v19 = v130;
        }
        v4 = 0;
LABEL_102:
        v131 = ++v25;
        v28 += 24;
        v26 = *(_QWORD *)(v1 + 88);
        v27 = (*(_QWORD *)(v1 + 96) - v26) / 24;
      }
      while ( (int)v25 < v27 );
    }
    sub_145DFF4D0(v1, v27);
    v65 = MEMORY[0xDC5CDC8]();
    if ( (unsigned __int8)sub_146D66260(v1 + 584, v65) )
    {
      v66 = *(_QWORD **)(v1 + 568);
      v67 = (_QWORD *)*v66;
      if ( (_QWORD *)*v66 != v66 )
      {
        do
        {
          v68 = v67[5];
          if ( v68 )
          {
            v69 = *(_QWORD **)(v68 + 8);
            v70 = (_QWORD *)*v69;
            if ( (_QWORD *)*v69 != v69 )
            {
              do
              {
                v71 = v70[3];
                if ( v71 && *(_DWORD *)(v71 + 8) )
                  v72 = v70[4];
                else
                  v72 = 0;
                v73 = v72 - 48;
                if ( !v72 )
                  v73 = 0;
                *((_DWORD *)v70 + 10) = sub_145DE83D0(v1, v73, v68);
                v70 = (_QWORD *)*v70;
              }
              while ( v70 != *(_QWORD **)(v68 + 8) );
            }
            v74 = *(_QWORD **)(v68 + 72);
            v75 = (_QWORD *)*v74;
            if ( (_QWORD *)*v74 != v74 )
            {
              do
              {
                v76 = v75[3];
                if ( v76 && *(_DWORD *)(v76 + 8) )
                  v77 = v75[4];
                else
                  v77 = 0;
                v78 = v77 - 48;
                if ( !v77 )
                  v78 = 0;
                *((_DWORD *)v75 + 10) = sub_145DE83D0(v1, v78, v68);
                v75 = (_QWORD *)*v75;
              }
              while ( v75 != *(_QWORD **)(v68 + 72) );
            }
          }
          v79 = (__int64 **)v67[2];
          if ( *((_BYTE *)v79 + 25) )
          {
            for ( j = v67[1]; !*(_BYTE *)(j + 25); j = *(_QWORD *)(j + 8) )
            {
              if ( v67 != *(_QWORD **)(j + 16) )
                break;
              v67 = (_QWORD *)j;
            }
            v67 = (_QWORD *)j;
          }
          else
          {
            v67 = (_QWORD *)v67[2];
            for ( k = *v79; !*((_BYTE *)k + 25); k = (__int64 *)*k )
              v67 = k;
          }
        }
        while ( v67 != *(_QWORD **)(v1 + 568) );
      }
    }
    sub_145DE71F0(v1);
    v82 = 0;
    v83 = *(_QWORD *)(v1 + 392);
    v84 = *(_QWORD *)(v1 + 400) - v83;
    v85 = v84 / 24;
    if ( v84 / 24 )
    {
      v86 = 0;
      do
      {
        v87 = *(_QWORD *)(v86 + v83 + 8);
        if ( v87 && *(_DWORD *)(v87 + 8) )
          v88 = *(_QWORD *)(v86 + v83 + 16);
        else
          v88 = 0;
        v89 = v88 - 48;
        if ( !v88 )
          v89 = 0;
        LOBYTE(v85) = 1;
        (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v89 + 3576LL))(v89, v85);
        ++v82;
        v86 += 24;
        v83 = *(_QWORD *)(v1 + 392);
        v84 = *(_QWORD *)(v1 + 400) - v83;
        v85 = v84 / 24;
      }
      while ( v82 < (unsigned __int64)(v84 / 24) );
    }
    if ( *(_DWORD *)(v1 + 656) + 1000 <= (unsigned int)MEMORY[0xDC5D148](v84, v85) )
    {
      *(_DWORD *)(v1 + 656) = MEMORY[0xDC5D148]();
      (*(void (__fastcall **)(__int64, __int64, __int64))(*(_QWORD *)qword_14E683C68 + 16LL))(qword_14E683C68, 34, 2);
      v90 = *(_QWORD **)(v1 + 424);
      v91 = (_QWORD *)*v90;
      if ( (_QWORD *)*v90 != v90 )
      {
        do
        {
          v92 = v91[4];
          if ( v92 && *(_DWORD *)(v92 + 8) )
          {
            v93 = v91[5];
            v94 = (_DWORD *)(v93 - 48);
            if ( !v93 )
              v94 = 0;
            if ( v94 )
            {
              if ( (v94[87] & 0x211) == 0x211 )
              {
                if ( (*(unsigned __int8 (__fastcall **)(_DWORD *))(*(_QWORD *)v94 + 1392LL))(v94) )
                {
                  v95 = sub_1450BE600(v94);
                  if ( !(*(unsigned int (__fastcall **)(__int64))(*(_QWORD *)v95 + 536LL))(v95) )
                  {
                    v96 = sub_145DB34F0(v95);
                    v97 = sub_145DB34B0(v95);
                    (*(void (__fastcall **)(__int64, int *, int *))(*(_QWORD *)v95 + 7816LL))(v95, &v132, &v120);
                    if ( v96 != v132 || v97 != v120 )
                    {
                      v98 = sub_145B8A420(v95);
                      sub_1466644F0(qword_14E683C68, v98);
                      v99 = sub_145B8C670(v95);
                      sub_1466644F0(qword_14E683C68, v99);
                      sub_1466642E0(qword_14E683C68, (unsigned __int8)v132);
                      sub_1466642E0(qword_14E683C68, (unsigned __int8)v120);
                      ++v4;
                    }
                  }
                }
              }
            }
          }
          v91 = (_QWORD *)*v91;
        }
        while ( v91 != v90 );
        v1 = a1;
        if ( v4 )
        {
          sub_1466644F0(qword_14E683C68, 0xFFFF);
          LOBYTE(v100) = 1;
          sub_14665AA60(v100, 0);
        }
      }
      if ( qword_14E682918 )
      {
        sub_145E28710();
        if ( qword_14E682918 )
          sub_145E287C0();
      }
    }
    if ( !qword_14E639CB8 )
    {
      v101 = sub_146E8BA20(112);
      if ( v101 )
        v102 = (void (__fastcall ***)(_QWORD))sub_144F2A020(v101);
      else
        v102 = 0;
      qword_14E639CB8 = (__int64)v102;
      (**v102)(v102);
    }
    sub_144F2B270();
    v106 = (_QWORD *)qword_14E63AE60;
    if ( qword_14E63AE60
      || ((v107 = sub_146E8BA20(336)) == 0 ? (v108 = 0) : (v108 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v107)),
          qword_14E63AE60 = (__int64)v108,
          (**v108)(v108),
          (v106 = (_QWORD *)qword_14E63AE60) != 0) )
    {
      sub_1447EE8F0(v106, v103, v104, v105);
    }
    v109 = qword_14E639098;
    if ( qword_14E639098
      || ((v110 = sub_146E8BA20(56)) == 0 ? (v111 = 0) : (v111 = (void (__fastcall ***)(_QWORD))sub_146613BF0(v110)),
          qword_14E639098 = (__int64)v111,
          (**v111)(v111),
          (v109 = qword_14E639098) != 0) )
    {
      sub_146615B30(v109, v1);
    }
    if ( qword_14E63AE68
      || ((v112 = sub_146E8BA20(96)) == 0 ? (v113 = 0) : (v113 = (void (__fastcall ***)(_QWORD))sub_142AC5B80(v112)),
          qword_14E63AE68 = (__int64)v113,
          (**v113)(v113),
          qword_14E63AE68) )
    {
      sub_142AC95D0();
    }
    result = sub_145E045D0(v1);
    if ( *(_QWORD *)(v1 + 1296) )
    {
      v114 = qword_14E634248;
      if ( !qword_14E634248 )
      {
        v115 = sub_146E8BA20(2496);
        if ( v115 )
          v116 = (void (__fastcall ***)(_QWORD))sub_1456918F0(v115);
        else
          v116 = 0;
        qword_14E634248 = (__int64)v116;
        (**v116)(v116);
        v114 = qword_14E634248;
      }
      v117 = sub_145693930(v114, 728);
      result = sub_148AA307C(v117, 0, (unsigned int)&off_14DFF5FB0, (unsigned int)&off_14E00B7C0, 0);
      v118 = result;
      if ( result )
      {
        v119 = MEMORY[0xDC5CDC8]() - *(_DWORD *)(v1 + 1276);
        result = sub_140BB0090(v118);
        if ( v119 > (unsigned int)result )
          return sub_145DEEE40(v1);
      }
    }
  }
  return result;
}

