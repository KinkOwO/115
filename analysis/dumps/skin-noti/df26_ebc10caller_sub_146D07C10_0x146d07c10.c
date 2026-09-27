// ebc10caller_sub_146D07C10_0x146d07c10

void __fastcall sub_146D07C10(__int64 a1, __int64 a2, __int64 a3)
{
  _QWORD *v4; // rbx
  _WORD *v5; // rdx
  __int64 v6; // rax
  void (__fastcall ***v7)(_QWORD); // rcx
  unsigned int v8; // edi
  __int64 v9; // rax
  __int64 v10; // r8
  _WORD *v11; // rax
  __int64 v12; // r8
  __int64 v13; // r8
  unsigned int v14; // edx
  unsigned int *v15; // rax
  __int64 v16; // r8
  __int64 v17; // rax
  __int64 v18; // r13
  __int64 v19; // rax
  __int64 v20; // rcx
  __int64 v21; // rax
  __int64 v22; // rcx
  __int64 v23; // rax
  __int64 v24; // rax
  __int64 v25; // r8
  __int64 v26; // rax
  __int64 v27; // rax
  _QWORD *v28; // r8
  _QWORD *v29; // rax
  __int64 **v30; // rcx
  __int64 i; // rcx
  __int64 *j; // rdx
  __int64 v33; // rax
  __int64 v34; // rdx
  __int64 v35; // rcx
  __int64 v36; // rax
  int v37; // eax
  __int64 v38; // rax
  __int64 v39; // rax
  unsigned int v40; // edi
  __int64 v41; // rdx
  __int64 v42; // rcx
  __int64 v43; // r8
  __int64 v44; // rax
  __int64 v45; // r8
  __int64 v46; // r8
  __int64 v47; // rax
  _QWORD *v48; // rcx
  _QWORD *v49; // rax
  __int64 **v50; // rdx
  __int64 k; // rdx
  __int64 *m; // r8
  __int64 v53; // rax
  __int64 v54; // rdx
  __int64 v55; // rcx
  __int64 v56; // rax
  __int64 v57; // rcx
  __int64 v58; // rbx
  __int64 v59; // rdx
  __int64 v60; // rax
  __int64 v61; // r14
  int v62; // esi
  unsigned int v63; // edi
  __int64 v64; // rax
  __int64 v65; // rax
  __int64 v66; // rcx
  __int64 v67; // rax
  void (__fastcall ***v68)(_QWORD); // rcx
  __int64 v69; // rax
  __int64 v70; // rdx
  int v71; // eax
  __int64 v72; // rcx
  __int64 v73; // rax
  void (__fastcall ***v74)(_QWORD); // rcx
  __int64 v75; // rcx
  __int64 v76; // rax
  void (__fastcall ***v77)(_QWORD); // rcx
  __int64 v78; // rax
  __int64 v79; // rdi
  unsigned __int64 v80; // rbx
  __int64 v81; // rax
  __int64 v82; // rax
  __int64 v83; // rax
  __int64 v84; // rax
  void (__fastcall ***v85)(_QWORD); // rcx
  __int64 v86; // rcx
  __int64 v87; // rax
  void (__fastcall ***v88)(_QWORD); // rcx
  __int64 v89; // rax
  void (__fastcall ***v90)(_QWORD); // rcx
  __int64 v91; // rdx
  __int64 v92; // rax
  __int64 v93; // rcx
  __int64 v94; // rax
  void (__fastcall ***v95)(_QWORD); // rcx
  __int64 v96; // rax
  void (__fastcall ***v97)(_QWORD); // rcx
  __int64 v98; // rax
  void (__fastcall ***v99)(_QWORD); // rcx
  unsigned int v100; // ebx
  __int64 v101; // rcx
  __int64 v102; // rax
  void (__fastcall ***v103)(_QWORD); // rcx
  __int64 v104; // r8
  __int64 v105; // rdx
  _BYTE *v106; // rcx
  __int64 v107; // rcx
  unsigned __int64 v108; // rdx
  __int64 v109; // rcx
  __int64 v110; // rcx
  __int64 v111; // rax
  void (__fastcall ***v112)(_QWORD); // rcx
  unsigned int v113; // ebx
  __int64 v114; // rcx
  __int64 v115; // rax
  void (__fastcall ***v116)(_QWORD); // rcx
  __int64 v117; // rcx
  unsigned __int64 v118; // rdx
  unsigned int v119; // [rsp+40h] [rbp-C0h] BYREF
  unsigned int v120; // [rsp+44h] [rbp-BCh] BYREF
  _QWORD *v121; // [rsp+48h] [rbp-B8h]
  _BYTE v122[8]; // [rsp+50h] [rbp-B0h] BYREF
  __int128 v123; // [rsp+58h] [rbp-A8h] BYREF
  __int64 v124; // [rsp+68h] [rbp-98h]
  __int128 v125; // [rsp+70h] [rbp-90h] BYREF
  __int64 v126; // [rsp+80h] [rbp-80h]
  _QWORD v127[4]; // [rsp+88h] [rbp-78h] BYREF
  _BYTE v128[8]; // [rsp+A8h] [rbp-58h] BYREF
  __int64 v129; // [rsp+B0h] [rbp-50h]
  __int64 v130; // [rsp+B8h] [rbp-48h]
  __int64 v131; // [rsp+C0h] [rbp-40h]
  _BYTE v132[256]; // [rsp+D0h] [rbp-30h] BYREF

  v131 = -2;
  if ( *(_QWORD *)(a1 + 1400) )
  {
    sub_148AA2510(v132, 0, 256);
    v4 = (_QWORD *)(a1 + 1384);
    v5 = (_WORD *)(a1 + 1384);
    if ( *(_QWORD *)(a1 + 1408) >= 8u )
      v5 = (_WORD *)*v4;
    sub_146EA9450(v132, v5);
    if ( !qword_14EF5B380 )
    {
      v6 = sub_146E8BA20(32);
      v121 = (_QWORD *)v6;
      if ( v6 )
        v7 = (void (__fastcall ***)(_QWORD))sub_1410D0C60(v6);
      else
        v7 = 0;
      qword_14EF5B380 = (__int64)v7;
      (**v7)(v7);
    }
    v8 = sub_1410D1100();
    v121 = v127;
    v9 = sub_146E8C7D0(&unk_1495CD300);
    v127[0] = 0;
    v127[2] = 0;
    v127[3] = 7;
    v10 = -1;
    do
      ++v10;
    while ( *(_WORD *)(v9 + 2 * v10) );
    sub_14014C8D0(v127, v9);
    sub_146D10000(v122, v132, v127, v8);
    v11 = (_WORD *)(a1 + 1384);
    if ( *(_QWORD *)(a1 + 1408) >= 8u )
      v11 = (_WORD *)*v4;
    *(_QWORD *)(a1 + 1400) = 0;
    *v11 = 0;
  }
  LOBYTE(a3) = 1;
  if ( (unsigned __int8)sub_146682140(qword_14E683C78, 2475, a3) )
    sub_146694510(qword_14E683C78, 2475, -1, 0, 1);
  LOBYTE(v12) = 1;
  if ( (unsigned __int8)sub_146682140(qword_14E683C78, 586, v12) )
    sub_146694510(qword_14E683C78, 586, -1, 0, 1);
  LOBYTE(v13) = 1;
  if ( (unsigned __int8)sub_146682140(qword_14E683C78, 2554, v13) )
    sub_146694510(qword_14E683C78, 2554, -1, 0, 1);
  v14 = *(_DWORD *)(a1 + 640);
  v119 = v14;
  v15 = *(unsigned int **)(a1 + 664);
  if ( v15 )
    v16 = *v15;
  else
    v16 = 0xFFFFFFFFLL;
  v120 = v16;
  v17 = sub_146D02440(a1, v14, v16);
  v18 = v17;
  if ( v17 && sub_144D24C20(v17) && *(_BYTE *)(sub_144D24C20(v18) + 9392) )
  {
    sub_1465DDE80();
    sub_1465DDEA0();
  }
  if ( v119 == -1 || v120 == -1 )
    return;
  sub_1459AFA20(qword_14E66C090);
  v19 = sub_1443261C0();
  sub_1443318F0(v19);
  v21 = sub_14194A460(v20);
  if ( !(unsigned __int8)sub_1453CDB90(v21, v119, v120) )
  {
    v23 = sub_14194A460(v22);
    sub_1453CD830(v23);
  }
  v24 = sub_1429BDDE0(qword_14E683C78);
  if ( (unsigned __int8)sub_145569110(v24) )
  {
    v26 = sub_1429BDDE0(qword_14E683C78);
    sub_145585DF0(v26, 0);
    *(_DWORD *)(a1 + 900) = -1;
  }
  if ( v119 != 39 && v119 != 40 || v120 )
  {
    LOBYTE(v25) = 1;
    v27 = sub_140283D60(qword_14E683B70, v119, v25);
    if ( v27 )
    {
      v28 = *(_QWORD **)(v27 + 360);
      v29 = (_QWORD *)*v28;
      if ( (_QWORD *)*v28 != v28 )
      {
        while ( *((_DWORD *)v29 + 10) != v120 )
        {
          v30 = (__int64 **)v29[2];
          if ( *((_BYTE *)v30 + 25) )
          {
            for ( i = v29[1]; !*(_BYTE *)(i + 25); i = *(_QWORD *)(i + 8) )
            {
              if ( v29 != *(_QWORD **)(i + 16) )
                break;
              v29 = (_QWORD *)i;
            }
            v29 = (_QWORD *)i;
          }
          else
          {
            v29 = (_QWORD *)v29[2];
            for ( j = *v30; !*((_BYTE *)j + 25); j = (__int64 *)*j )
              v29 = j;
          }
          if ( v29 == v28 )
            goto LABEL_55;
        }
        if ( *((_DWORD *)v29 + 20) == 4 )
        {
          v33 = sub_14021A3E0();
          if ( (unsigned __int8)sub_144DA8780(v33) )
          {
            LOBYTE(v34) = 1;
            sub_1459AD0F0(qword_14E66C090, v34);
          }
          v36 = sub_14021A2C0(v35, v34);
          v37 = sub_145693930(v36, 2583);
          v38 = sub_148AA307C(v37, 0, (unsigned int)&off_14DFF5FB0, (unsigned int)&off_14DD9C560, 0);
          if ( *(_DWORD *)(*(_QWORD *)(a1 + 664) + 40LL) == 4 && (!v38 || *(_DWORD *)(v38 + 132) != v119) )
          {
            sub_146CFDD20(a1);
            v39 = sub_144964EA0();
            sub_14536BAE0(v39, 13);
          }
        }
      }
    }
  }
LABEL_55:
  v40 = sub_14667EA60(qword_14E683C78);
  v44 = sub_1401DCCB0(v42, v41, v43);
  if ( (unsigned int)sub_1403F4ED0(v44, L"CONFIG_PARTY_LIST_WINDOW_AUTO_OPEN") )
  {
    LOBYTE(v45) = 1;
    if ( !(unsigned __int8)sub_146682140(qword_14E683C78, v40, v45) )
    {
      LOBYTE(v46) = 1;
      v47 = sub_140283D60(qword_14E683B70, v119, v46);
      if ( v47 )
      {
        v48 = *(_QWORD **)(v47 + 360);
        v49 = (_QWORD *)*v48;
        if ( (_QWORD *)*v48 != v48 )
        {
          while ( *((_DWORD *)v49 + 10) != v120 )
          {
            v50 = (__int64 **)v49[2];
            if ( *((_BYTE *)v50 + 25) )
            {
              for ( k = v49[1]; !*(_BYTE *)(k + 25); k = *(_QWORD *)(k + 8) )
              {
                if ( v49 != *(_QWORD **)(k + 16) )
                  break;
                v49 = (_QWORD *)k;
              }
              v49 = (_QWORD *)k;
            }
            else
            {
              v49 = (_QWORD *)v49[2];
              for ( m = *v50; !*((_BYTE *)m + 25); m = (__int64 *)*m )
                v49 = m;
            }
            if ( v49 == v48 )
              goto LABEL_71;
          }
          if ( *((_BYTE *)v49 + 488) )
            sub_14668C520(qword_14E683C78, v40, 0, 0);
        }
      }
    }
  }
LABEL_71:
  v53 = sub_14021A6E0();
  sub_144451C80(v53);
  v56 = sub_14021A2C0(v55, v54);
  if ( (unsigned __int8)sub_145695000(v56, 4047) )
    sub_145F588B0(21);
  sub_144F46C40(qword_14E683D40, &v123);
  v57 = *((_QWORD *)&v123 + 1);
  v58 = v123;
  if ( (_QWORD)v123 == *((_QWORD *)&v123 + 1) )
  {
LABEL_85:
    v66 = qword_14E634408;
    if ( !qword_14E634408 )
    {
      v67 = sub_146E8BA20(1520);
      v121 = (_QWORD *)v67;
      if ( v67 )
        v68 = (void (__fastcall ***)(_QWORD))sub_145194C50(v67);
      else
        v68 = 0;
      qword_14E634408 = (__int64)v68;
      (**v68)(v68);
      v66 = qword_14E634408;
    }
    sub_145197B50(v66, 0, 0xFFFFFFFFLL, 0xFFFFFFFFLL);
    v72 = qword_14E634408;
    if ( !qword_14E634408 )
    {
      v73 = sub_146E8BA20(1520);
      v121 = (_QWORD *)v73;
      if ( v73 )
        v74 = (void (__fastcall ***)(_QWORD))sub_145194C50(v73);
      else
        v74 = 0;
      qword_14E634408 = (__int64)v74;
      (**v74)(v74);
      v72 = qword_14E634408;
    }
    sub_145197B50(v72, 2, 0xFFFFFFFFLL, 0xFFFFFFFFLL);
    v75 = qword_14E66EA50;
    if ( !qword_14E66EA50 )
    {
      v76 = sub_146E8BA20(56);
      v121 = (_QWORD *)v76;
      if ( v76 )
        v77 = (void (__fastcall ***)(_QWORD))sub_143866830(v76);
      else
        v77 = 0;
      qword_14E66EA50 = (__int64)v77;
      (**v77)(v77);
      v75 = qword_14E66EA50;
    }
    sub_143866A20(v75, 1, v119, v120);
    v78 = sub_146D03A30(a1, v119, v120);
    v79 = v78;
    if ( v78 )
    {
      v80 = 0;
      if ( (unsigned int)sub_145DF44B0(v78) )
      {
        do
        {
          v81 = sub_145DF2D80(v79, (unsigned int)v80);
          if ( v81 )
            sub_145B998A0(v81);
          ++v80;
        }
        while ( v80 < (int)sub_145DF44B0(v79) );
      }
      v82 = sub_146D02440(a1, v119, v120);
      if ( v82 )
        sub_145B998A0(v82);
    }
    if ( *(_DWORD *)(a1 + 720) == 4 )
    {
      v83 = qword_14E6344E0;
      if ( !qword_14E6344E0 )
      {
        v84 = sub_146E8BA20(752);
        v121 = (_QWORD *)v84;
        if ( v84 )
          v85 = (void (__fastcall ***)(_QWORD))sub_144309A90(v84);
        else
          v85 = 0;
        qword_14E6344E0 = (__int64)v85;
        (**v85)(v85);
        v83 = qword_14E6344E0;
      }
      *(_BYTE *)(v83 + 576) = 1;
    }
    if ( (unsigned __int8)sub_1459ABDF0(qword_14E66C090) )
    {
      v86 = qword_14E6344E0;
      if ( !qword_14E6344E0 )
      {
        v87 = sub_146E8BA20(752);
        v121 = (_QWORD *)v87;
        if ( v87 )
          v88 = (void (__fastcall ***)(_QWORD))sub_144309A90(v87);
        else
          v88 = 0;
        qword_14E6344E0 = (__int64)v88;
        (**v88)(v88);
        v86 = qword_14E6344E0;
      }
      sub_144310DB0(v86, v119, v120);
    }
    if ( !qword_14E64E088 )
    {
      v89 = sub_146E8BA20(120);
      v121 = (_QWORD *)v89;
      if ( v89 )
        v90 = (void (__fastcall ***)(_QWORD))sub_14184D830(v89);
      else
        v90 = 0;
      qword_14E64E088 = (__int64)v90;
      (**v90)(v90);
    }
    sub_14184EC10();
    if ( (unsigned __int8)sub_145242380(qword_14E66C090) )
    {
      v92 = *(_QWORD *)(a1 + 664);
      if ( v92 )
      {
        v93 = qword_14E638088;
        if ( !qword_14E638088 )
        {
          v94 = sub_146E8BA20(11920);
          v121 = (_QWORD *)v94;
          if ( v94 )
            v95 = (void (__fastcall ***)(_QWORD))sub_1422AF680(v94);
          else
            v95 = 0;
          qword_14E638088 = (__int64)v95;
          (**v95)(v95);
          v92 = *(_QWORD *)(a1 + 664);
          v93 = qword_14E638088;
        }
        sub_1422B3680(v93, v119, v120, *(unsigned int *)(v92 + 40));
      }
    }
    LOBYTE(v91) = 1;
    if ( (unsigned __int8)sub_1459AD0F0(qword_14E66C090, v91) )
      sub_142ABD9A0(qword_14E683C40, v119, v120);
    *(_BYTE *)(a1 + 865) = 0;
    if ( !qword_14E666FC8 )
    {
      v96 = sub_146E8BA20(24);
      v121 = (_QWORD *)v96;
      if ( v96 )
        v97 = (void (__fastcall ***)(_QWORD))sub_1449B5180(v96);
      else
        v97 = 0;
      qword_14E666FC8 = (__int64)v97;
      (**v97)(v97);
    }
    sub_1449C6E80();
    if ( !qword_14E6520E8 )
    {
      v98 = sub_146E8BA20(208);
      v121 = (_QWORD *)v98;
      if ( v98 )
        v99 = (void (__fastcall ***)(_QWORD))sub_142F9E6F0(v98);
      else
        v99 = 0;
      qword_14E6520E8 = (__int64)v99;
      (**v99)(v99);
    }
    if ( (unsigned __int8)sub_142FA1230() )
    {
      if ( *(_BYTE *)(a1 + 1468) )
      {
        sub_146EA4750(v128, &qword_14EF2CA98);
        v100 = 0;
        v101 = qword_14E638F28;
        if ( !qword_14E638F28 )
        {
          v102 = sub_146E8BA20(1472);
          v121 = (_QWORD *)v102;
          if ( v102 )
            v103 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v102);
          else
            v103 = 0;
          qword_14E638F28 = (__int64)v103;
          (**v103)(v103);
          v101 = qword_14E638F28;
        }
        sub_1444EBC10(v101, &v125, 8);
        if ( (__int64)(*((_QWORD *)&v125 + 1) - v125) >> 2 )
          v100 = *(_DWORD *)v125;
        if ( v129 )
        {
          if ( *(_DWORD *)(v129 + 8) )
          {
            v105 = v130;
            if ( v130 )
            {
              if ( v130 != 48 )
              {
                if ( !*(_DWORD *)(v129 + 8) )
                  v105 = 0;
                v106 = (_BYTE *)(v105 - 48);
                if ( !v105 )
                  v106 = 0;
                sub_145BF2AB0(v106, v100, v104);
              }
            }
          }
        }
        v107 = v125;
        if ( (_QWORD)v125 )
        {
          v108 = 4 * ((v126 - (__int64)v125) >> 2);
          if ( v108 >= 0x1000 )
          {
            v108 += 39LL;
            v107 = *(_QWORD *)(v125 - 8);
            if ( (unsigned __int64)(v125 - v107 - 8) > 0x1F )
              sub_148AAF304(v107, v108);
          }
          sub_146E9F3A0(v107, v108);
          v125 = 0;
          v126 = 0;
        }
        v109 = v129;
        if ( v129 && _InterlockedExchangeAdd((volatile signed __int32 *)(v129 + 12), 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(__int64))(*(_QWORD *)v109 + 8LL))(v109);
      }
      *(_BYTE *)(a1 + 1468) = 0;
    }
    *(_QWORD *)(a1 + 1440) = 0;
    *(_QWORD *)(a1 + 1448) = 0;
    *(_QWORD *)(a1 + 1456) = 0;
    *(_WORD *)(a1 + 1464) = 0;
    v110 = qword_14E634250;
    if ( !qword_14E634250 )
    {
      v111 = sub_146E8BA20(344);
      v121 = (_QWORD *)v111;
      if ( v111 )
        v112 = (void (__fastcall ***)(_QWORD))sub_1407498E0(v111);
      else
        v112 = 0;
      qword_14E634250 = (__int64)v112;
      (**v112)(v112);
      v110 = qword_14E634250;
    }
    sub_14074D860(v110, &v119, &v120, 0);
    if ( v18 )
    {
      v113 = sub_144D24C00(v18);
      v114 = qword_14E66C248;
      if ( !qword_14E66C248 )
      {
        v115 = sub_146E8BA20(32);
        v121 = (_QWORD *)v115;
        if ( v115 )
          v116 = (void (__fastcall ***)(_QWORD))sub_14360FAE0(v115);
        else
          v116 = 0;
        qword_14E66C248 = (__int64)v116;
        (**v116)(v116);
        v114 = qword_14E66C248;
      }
      sub_143610D40(v114, v113, v119, v120);
    }
    goto LABEL_185;
  }
  v59 = v119;
  while ( *(_DWORD *)(*(_QWORD *)v58 + 1504LL) != (_DWORD)v59 )
  {
LABEL_84:
    v58 += 8;
    if ( v58 == v57 )
      goto LABEL_85;
  }
  v60 = sub_146D03A30(a1, v59, v120);
  v61 = v60;
  if ( !v60 )
    goto LABEL_185;
  v62 = sub_145DF44B0(v60);
  v63 = 0;
  if ( v62 <= 0 )
  {
LABEL_83:
    v59 = v119;
    v57 = *((_QWORD *)&v123 + 1);
    goto LABEL_84;
  }
  while ( 1 )
  {
    v64 = sub_145DF2D80(v61, v63);
    if ( v64 )
    {
      if ( (*(_BYTE *)(v64 + 348) & 0x22) == 0x22 )
      {
        v65 = sub_1450BE620(v64);
        if ( v65 )
        {
          if ( (*(unsigned int (__fastcall **)(__int64))(*(_QWORD *)v65 + 2408LL))(v65) == *(_DWORD *)(*(_QWORD *)v58 + 632LL) )
            break;
        }
      }
    }
    if ( (int)++v63 >= v62 )
      goto LABEL_83;
  }
  v69 = sub_1429BDDE0(qword_14E683C78);
  LOBYTE(v70) = 1;
  sub_145585DF0(v69, v70);
  *(_DWORD *)(a1 + 900) = 0;
  v71 = sub_146E8C7D0(&unk_14A7A0448);
  sub_145A31380(v71, -1, 0, 0, -1, -1, 0);
LABEL_185:
  v117 = v123;
  if ( (_QWORD)v123 )
  {
    v118 = (v124 - v123) & 0xFFFFFFFFFFFFFFF8uLL;
    if ( v118 >= 0x1000 )
    {
      v118 += 39LL;
      v117 = *(_QWORD *)(v123 - 8);
      if ( (unsigned __int64)(v123 - v117 - 8) > 0x1F )
        sub_148AAF304(v117, v118);
    }
    sub_146E9F3A0(v117, v118);
    v123 = 0;
    v124 = 0;
  }
}

