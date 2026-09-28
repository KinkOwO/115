// sender_1551_sub_141B17730

char __fastcall sub_141B17730(__int64 a1, _QWORD *a2, unsigned int a3, __int64 a4, __int64 a5)
{
  __int64 (__fastcall *v9)(__int64, __int128 *); // r8
  __int64 v10; // rcx
  _DWORD *v11; // rax
  unsigned __int8 (__fastcall *v12)(__int64, __int128 *); // r8
  __int64 v13; // rcx
  unsigned __int8 (__fastcall *v14)(__int64, __int128 *); // r8
  __int64 v15; // rdx
  int v16; // esi
  int v17; // r15d
  __int64 v18; // r12
  __int64 v19; // r13
  unsigned __int8 (__fastcall ***v20)(_QWORD, __int128 *); // rcx
  unsigned __int8 (__fastcall *v21)(_QWORD, __int128 *); // r8
  __int64 v22; // rdx
  __int64 v23; // rcx
  __int64 v24; // rax
  __int64 v25; // rcx
  __int64 v26; // rbx
  __int64 v27; // rax
  __int64 v28; // rcx
  int v29; // ebx
  __int64 v30; // rax
  __int64 v31; // rcx
  __int64 v32; // rax
  __int64 v33; // rax
  __int64 v34; // rcx
  __int64 v35; // rax
  __int64 v36; // rdx
  __int64 v37; // rcx
  __int64 v38; // rbx
  __int64 (__fastcall ***v39)(_QWORD, __int128 *); // rcx
  __int64 (__fastcall *v40)(_QWORD, __int128 *); // r8
  __int64 v41; // rdx
  __int64 v42; // rax
  __int64 v43; // rbx
  __int64 v44; // rcx
  __int64 v45; // rcx
  __int64 v46; // rax
  __int64 v47; // rcx
  __int64 v48; // rax
  __int64 v49; // rdx
  __int64 v50; // rcx
  __int64 (__fastcall *v51)(__int64, __int128 *); // r8
  __int64 v52; // rdx
  int v53; // r15d
  __int64 v54; // r12
  __int64 v55; // r13
  __int64 (__fastcall ***v56)(_QWORD, __int128 *); // rcx
  __int64 (__fastcall *v57)(_QWORD, __int128 *); // r8
  __int64 v58; // rdx
  __int64 v59; // rcx
  __int64 v60; // rax
  __int64 v61; // rbx
  __int64 (__fastcall *v62)(__int64, __int128 *); // r8
  __int64 v63; // rdx
  int v64; // esi
  __int64 v65; // r15
  __int64 v66; // r12
  __int64 (__fastcall ***v67)(_QWORD, __int128 *); // rcx
  __int64 (__fastcall *v68)(_QWORD, __int128 *); // r8
  __int64 v69; // rdx
  __int64 v70; // rcx
  __int64 v71; // rax
  __int64 v72; // rcx
  __int64 v73; // rax
  __int64 v74; // r13
  __int64 v75; // rax
  __int64 v76; // rdx
  __int64 v77; // rax
  __int64 v78; // rax
  _DWORD *v79; // rbx
  int v80; // r9d
  int v81; // r8d
  _BYTE *v82; // rdx
  _QWORD *v83; // rdx
  _BYTE *v84; // rcx
  _BYTE *v85; // rdx
  int v86; // r15d
  __int64 v87; // rbx
  unsigned __int8 (__fastcall ***v88)(_QWORD, __int128 *); // rcx
  unsigned __int8 (__fastcall *v89)(_QWORD, __int128 *); // r8
  __int64 v90; // rdx
  __int64 v91; // rcx
  __int64 v92; // rax
  __int64 v93; // rcx
  __int64 v94; // rax
  __int64 v95; // rdx
  __int64 v96; // rcx
  __int64 (__fastcall *v97)(__int64, __int128 *); // r8
  __int64 v98; // rdx
  int v99; // esi
  __int64 i; // rbx
  __int64 (__fastcall ***v101)(_QWORD, __int128 *); // rcx
  __int64 (__fastcall *v102)(_QWORD, __int128 *); // r8
  __int64 v103; // rdx
  __int64 v104; // rdx
  __int64 (__fastcall *v105)(__int64, __int128 *); // r8
  __int64 v106; // rdx
  __int64 v107; // rbx
  __int64 (__fastcall ***v108)(_QWORD, __int128 *); // rcx
  __int64 (__fastcall *v109)(_QWORD, __int128 *); // r8
  __int64 v110; // rdx
  __int64 v111; // rcx
  _DWORD *v113; // [rsp+40h] [rbp-C0h]
  _DWORD *v114; // [rsp+40h] [rbp-C0h]
  __int64 v115; // [rsp+58h] [rbp-A8h] BYREF
  __int128 v116; // [rsp+60h] [rbp-A0h] BYREF
  __int128 v117; // [rsp+70h] [rbp-90h] BYREF
  __int128 v118; // [rsp+80h] [rbp-80h] BYREF
  __int128 v119; // [rsp+90h] [rbp-70h] BYREF
  __int128 v120; // [rsp+A0h] [rbp-60h] BYREF
  __int128 v121; // [rsp+B0h] [rbp-50h] BYREF
  __int128 v122; // [rsp+C0h] [rbp-40h] BYREF
  __int128 v123; // [rsp+D0h] [rbp-30h] BYREF
  __int128 v124; // [rsp+E0h] [rbp-20h] BYREF
  __int128 v125; // [rsp+F0h] [rbp-10h] BYREF
  __int128 v126; // [rsp+100h] [rbp+0h] BYREF
  __int128 v127; // [rsp+110h] [rbp+10h] BYREF
  __int128 v128; // [rsp+120h] [rbp+20h] BYREF
  __int128 v129; // [rsp+130h] [rbp+30h] BYREF
  _QWORD v130[7]; // [rsp+140h] [rbp+40h] BYREF
  _QWORD *v131; // [rsp+178h] [rbp+78h]
  __int64 v132; // [rsp+180h] [rbp+80h]
  _BYTE v133[56]; // [rsp+190h] [rbp+90h] BYREF
  _BYTE *v134; // [rsp+1C8h] [rbp+C8h]
  _BYTE v135[13]; // [rsp+1D0h] [rbp+D0h] BYREF
  int v136; // [rsp+1DDh] [rbp+DDh]
  int v137; // [rsp+1E1h] [rbp+E1h]
  _BYTE v138[13]; // [rsp+1E8h] [rbp+E8h] BYREF
  int v139; // [rsp+1F5h] [rbp+F5h]
  int v140; // [rsp+1F9h] [rbp+F9h]
  _BYTE v141[13]; // [rsp+200h] [rbp+100h] BYREF
  int v142; // [rsp+20Dh] [rbp+10Dh]
  int v143; // [rsp+211h] [rbp+111h]
  _BYTE v144[56]; // [rsp+220h] [rbp+120h] BYREF
  _BYTE *v145; // [rsp+258h] [rbp+158h]

  v132 = -2;
  v9 = **(__int64 (__fastcall ***)(__int64, __int128 *))a1;
  v116 = 0;
  v10 = a2[1];
  if ( v10 )
  {
    _InterlockedIncrement((volatile signed __int32 *)(v10 + 8));
    v10 = a2[1];
  }
  *(_QWORD *)&v116 = *a2;
  *((_QWORD *)&v116 + 1) = v10;
  LOBYTE(v11) = v9(a1, &v116);
  if ( (_BYTE)v11 )
  {
    v12 = **(unsigned __int8 (__fastcall ***)(__int64, __int128 *))(a1 + 32);
    v117 = 0;
    v13 = a2[1];
    if ( v13 )
    {
      _InterlockedIncrement((volatile signed __int32 *)(v13 + 8));
      v13 = a2[1];
    }
    *(_QWORD *)&v117 = *a2;
    *((_QWORD *)&v117 + 1) = v13;
    if ( v12(a1 + 32, &v117) )
      sub_141B171C0(a1 + 32, a2, a3, a4, a5);
    LOBYTE(v11) = a3 - 8;
    switch ( a3 )
    {
      case 8u:
        if ( *(_DWORD *)(a1 + 5368) == -1 )
        {
          v62 = **(__int64 (__fastcall ***)(__int64, __int128 *))(a1 + 1232);
          v123 = 0;
          v63 = a2[1];
          if ( v63 )
          {
            _InterlockedIncrement((volatile signed __int32 *)(v63 + 8));
            v63 = a2[1];
          }
          *(_QWORD *)&v123 = *a2;
          *((_QWORD *)&v123 + 1) = v63;
          LOBYTE(v11) = v62(a1 + 1232, &v123);
          if ( (_BYTE)v11 )
          {
            v64 = 0;
            v65 = 0;
            v66 = 0;
            do
            {
              v67 = (__int64 (__fastcall ***)(_QWORD, __int128 *))(v65 + a1 + 1272);
              v68 = **v67;
              v124 = 0;
              v69 = a2[1];
              if ( v69 )
              {
                _InterlockedIncrement((volatile signed __int32 *)(v69 + 8));
                v69 = a2[1];
              }
              *(_QWORD *)&v124 = *a2;
              *((_QWORD *)&v124 + 1) = v69;
              LOBYTE(v11) = v68(v67, &v124);
              if ( (_BYTE)v11 )
              {
                LOBYTE(v11) = sub_146ECFE30(*(_QWORD *)(a1 + v65 + 1304));
                if ( (_BYTE)v11 )
                {
                  v71 = sub_141B013C0(v70);
                  v11 = (_DWORD *)sub_141B01690(v71);
                  if ( v11[v66 + 1] == 2 )
                  {
                    v73 = sub_141B013C0(v72);
                    LOBYTE(v11) = sub_141B03CC0(v73, (unsigned int)v64);
                    if ( !(_BYTE)v11 )
                    {
                      v74 = sub_14501B3E0(*(_QWORD *)(a1 + v65 + 1304));
                      v75 = sub_14501B3E0(*(_QWORD *)(a1 + v65 + 1304));
                      v76 = *(_QWORD *)((*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v75 + 152LL))(v75) + 584);
                      v115 = v76;
                      if ( v76 )
                        ++*(_DWORD *)(v76 + 8);
                      v77 = sub_14501B3E0(*(_QWORD *)(a1 + v65 + 1304));
                      v78 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v77 + 152LL))(v77);
                      v79 = (_DWORD *)(v78 + 24);
                      LOBYTE(v80) = 1;
                      LOBYTE(v81) = 50;
                      sub_1480A6620(v78 + 24, 4, v81, v80, v78 + 28);
                      LOBYTE(v11) = sub_146695090(qword_14E683C78, 2, 37, *v79, v64, (__int64)&v115, v74);
                      *(_DWORD *)(a1 + 5368) = v64;
                    }
                  }
                }
              }
              ++v64;
              ++v66;
              v65 += 128;
            }
            while ( v64 < 32 );
          }
        }
        return (char)v11;
      case 0xAu:
        v130[0] = off_1497B0D68;
        v130[1] = a1;
        v131 = v130;
        v134 = 0;
        v145 = 0;
        if ( v130 )
          v145 = (_BYTE *)(*(__int64 (__fastcall **)(_QWORD *, _BYTE *))v130[0])(v130, v144);
        sub_140438CE0(v144, v133);
        if ( v145 )
        {
          v82 = v144;
          LOBYTE(v82) = v145 != v144;
          (*(void (__fastcall **)(_BYTE *, _BYTE *))(*(_QWORD *)v145 + 32LL))(v145, v82);
          v145 = 0;
        }
        if ( v131 )
        {
          v83 = v130;
          LOBYTE(v83) = v131 != v130;
          (*(void (__fastcall **)(_QWORD *, _QWORD *))(*v131 + 32LL))(v131, v83);
          v131 = 0;
        }
        if ( (unsigned int)sub_14667E740(qword_14E683C78) == 2 )
        {
          v86 = 0;
          v87 = 0;
          while ( 1 )
          {
            v88 = (unsigned __int8 (__fastcall ***)(_QWORD, __int128 *))(v87 + a1 + 56);
            v89 = **v88;
            v125 = 0;
            v90 = a2[1];
            if ( v90 )
            {
              _InterlockedIncrement((volatile signed __int32 *)(v90 + 8));
              v90 = a2[1];
            }
            *(_QWORD *)&v125 = *a2;
            *((_QWORD *)&v125 + 1) = v90;
            if ( v89(v88, &v125) )
              break;
            ++v86;
            v87 += 168;
            if ( v87 >= 1176 )
              goto LABEL_84;
          }
          v142 = v86;
          v143 = *(_DWORD *)(a1 + 5368);
          v92 = sub_146D74000(v91);
          sub_146D746E0(v92, 2207);
          v94 = sub_146D74000(v93);
          sub_146D75B10(v94, v141, 21);
          sub_146D75AF0(v96, v95);
LABEL_84:
          v84 = v134;
          if ( !v134 )
            sub_14883BB10();
        }
        else
        {
          v84 = v134;
          if ( !v134 )
            sub_14883BB10();
        }
        (*(void (__fastcall **)(_BYTE *))(*(_QWORD *)v134 + 16LL))(v84);
        v11 = v133;
        if ( v134 )
        {
          v85 = v133;
          LOBYTE(v85) = v134 != v133;
          LOBYTE(v11) = (*(__int64 (__fastcall **)(_BYTE *, _BYTE *))(*(_QWORD *)v134 + 32LL))(v134, v85);
          v134 = 0;
        }
        return (char)v11;
      case 0xDu:
        v51 = **(__int64 (__fastcall ***)(__int64, __int128 *))(a1 + 1232);
        v121 = 0;
        v52 = a2[1];
        if ( v52 )
        {
          _InterlockedIncrement((volatile signed __int32 *)(v52 + 8));
          v52 = a2[1];
        }
        *(_QWORD *)&v121 = *a2;
        *((_QWORD *)&v121 + 1) = v52;
        LOBYTE(v11) = v51(a1 + 1232, &v121);
        if ( (_BYTE)v11 )
        {
          v53 = 0;
          v54 = 0;
          v55 = 0;
          do
          {
            v56 = (__int64 (__fastcall ***)(_QWORD, __int128 *))(v55 + a1 + 1272);
            v57 = **v56;
            v122 = 0;
            v58 = a2[1];
            if ( v58 )
            {
              _InterlockedIncrement((volatile signed __int32 *)(v58 + 8));
              v58 = a2[1];
            }
            *(_QWORD *)&v122 = *a2;
            *((_QWORD *)&v122 + 1) = v58;
            LOBYTE(v11) = v57(v56, &v122);
            if ( (_BYTE)v11 )
            {
              v60 = sub_141B013C0(v59);
              v11 = (_DWORD *)(v54 + sub_141B01690(v60) + 4);
              v114 = v11;
              if ( *v11 != 2 )
              {
                v61 = *(_QWORD *)(a1 + 24);
                sub_146F53250(*(_QWORD *)(v61 + 1512), 1, 0);
                (**(void (__fastcall ***)(__int64, _QWORD, __int64))(v61 + 1336))(
                  v61 + 1336,
                  *(unsigned int *)(*(_QWORD *)(v61 + 1512) + 336LL),
                  24);
                LOBYTE(v11) = (_BYTE)v114;
                if ( *v114 == 1 )
                  LOBYTE(v11) = sub_141B1AEF0(*(_QWORD *)(a1 + 24) + 6968LL, (unsigned int)v53);
              }
            }
            ++v53;
            v55 += 128;
            v54 += 4;
          }
          while ( v53 < 32 );
        }
        return (char)v11;
      case 0xEu:
        v14 = **(unsigned __int8 (__fastcall ***)(__int64, __int128 *))(a1 + 1232);
        v118 = 0;
        v15 = a2[1];
        if ( v15 )
        {
          _InterlockedIncrement((volatile signed __int32 *)(v15 + 8));
          v15 = a2[1];
        }
        *(_QWORD *)&v118 = *a2;
        *((_QWORD *)&v118 + 1) = v15;
        v16 = 0;
        if ( !v14(a1 + 1232, &v118) )
          goto LABEL_27;
        v17 = 0;
        v18 = 0;
        v19 = 0;
        while ( 2 )
        {
          v20 = (unsigned __int8 (__fastcall ***)(_QWORD, __int128 *))(v19 + a1 + 1272);
          v21 = **v20;
          v119 = 0;
          v22 = a2[1];
          if ( v22 )
          {
            _InterlockedIncrement((volatile signed __int32 *)(v22 + 8));
            v22 = a2[1];
          }
          *(_QWORD *)&v119 = *a2;
          *((_QWORD *)&v119 + 1) = v22;
          if ( !v21(v20, &v119) )
            goto LABEL_19;
          v24 = sub_141B013C0(v23);
          v113 = (_DWORD *)(v18 + sub_141B01690(v24) + 4);
          if ( *v113 != 2 )
          {
            v26 = *(_QWORD *)(a1 + 24);
            sub_146F53250(*(_QWORD *)(v26 + 1512), 1, 0);
            (**(void (__fastcall ***)(__int64, _QWORD, __int64))(v26 + 1336))(
              v26 + 1336,
              *(unsigned int *)(*(_QWORD *)(v26 + 1512) + 336LL),
              24);
            if ( *v113 == 1 )
              sub_141B1AEF0(*(_QWORD *)(a1 + 24) + 6968LL, (unsigned int)v17);
LABEL_19:
            ++v17;
            v19 += 128;
            v18 += 4;
            if ( v17 >= 32 )
              goto LABEL_27;
            continue;
          }
          break;
        }
        v27 = sub_141B013C0(v25);
        v29 = sub_141B01320(v27);
        if ( v29 != -1 )
        {
          v30 = sub_141B013C0(v28);
          if ( (unsigned __int8)sub_141AFEBA0(v30, (unsigned int)v17) != 1 )
          {
            if ( (unsigned int)sub_1459A90F0(qword_14E66C090) == 1 )
            {
              v136 = v29;
              v137 = v17;
              v33 = sub_146D74000(v31);
              sub_146D746E0(v33, 2207);
              v35 = sub_146D74000(v34);
              sub_146D75B10(v35, v135, 21);
              sub_146D75AF0(v37, v36);
            }
            else if ( qword_14E683C78 )
            {
              v32 = sub_14723C170(36526);
              sub_14668C520(qword_14E683C78, 2875, v32, 0);
            }
          }
        }
LABEL_27:
        v38 = 0;
        while ( 1 )
        {
          v39 = (__int64 (__fastcall ***)(_QWORD, __int128 *))(v38 + a1 + 56);
          v40 = **v39;
          v120 = 0;
          v41 = a2[1];
          if ( v41 )
          {
            _InterlockedIncrement((volatile signed __int32 *)(v41 + 8));
            v41 = a2[1];
          }
          *(_QWORD *)&v120 = *a2;
          *((_QWORD *)&v120 + 1) = v41;
          LOBYTE(v11) = v40(v39, &v120);
          if ( (_BYTE)v11 )
            break;
          ++v16;
          v38 += 168;
          if ( v38 >= 1176 )
            return (char)v11;
        }
        LODWORD(v11) = sub_1459A90F0(qword_14E66C090);
        if ( (_DWORD)v11 == 1 )
        {
          v43 = 168LL * v16;
          v44 = *(_QWORD *)(v43 + a1 + 96);
          if ( !v44 || (LOBYTE(v11) = sub_141FB6530(v44), (_BYTE)v11) )
          {
            (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(v43 + a1 + 96) + 16LL))(
              *(_QWORD *)(v43 + a1 + 96),
              0);
            v139 = v16;
            v140 = -1;
            v46 = sub_146D74000(v45);
            sub_146D746E0(v46, 2207);
            v48 = sub_146D74000(v47);
            sub_146D75B10(v48, v138, 21);
            LOBYTE(v11) = sub_146D75AF0(v50, v49);
          }
        }
        else if ( qword_14E683C78 )
        {
          v42 = sub_14723C170(36526);
          LOBYTE(v11) = sub_14668C520(qword_14E683C78, 2875, v42, 0);
        }
        return (char)v11;
      case 0x12u:
        v97 = **(__int64 (__fastcall ***)(__int64, __int128 *))(a1 + 1232);
        v126 = 0;
        v98 = a2[1];
        if ( v98 )
        {
          _InterlockedIncrement((volatile signed __int32 *)(v98 + 8));
          v98 = a2[1];
        }
        *(_QWORD *)&v126 = *a2;
        *((_QWORD *)&v126 + 1) = v98;
        LOBYTE(v11) = v97(a1 + 1232, &v126);
        if ( (_BYTE)v11 )
        {
          v99 = 0;
          for ( i = 0; i < 4096; i += 128 )
          {
            v101 = (__int64 (__fastcall ***)(_QWORD, __int128 *))(i + a1 + 1272);
            v102 = **v101;
            v127 = 0;
            v103 = a2[1];
            if ( v103 )
            {
              _InterlockedIncrement((volatile signed __int32 *)(v103 + 8));
              v103 = a2[1];
            }
            *(_QWORD *)&v127 = *a2;
            *((_QWORD *)&v127 + 1) = v103;
            LOBYTE(v11) = v102(v101, &v127);
            if ( (_BYTE)v11 )
            {
              LOBYTE(v104) = 1;
              goto LABEL_106;
            }
            ++v99;
          }
        }
        return (char)v11;
      case 0x13u:
        v105 = **(__int64 (__fastcall ***)(__int64, __int128 *))(a1 + 1232);
        v128 = 0;
        v106 = a2[1];
        if ( v106 )
        {
          _InterlockedIncrement((volatile signed __int32 *)(v106 + 8));
          v106 = a2[1];
        }
        *(_QWORD *)&v128 = *a2;
        *((_QWORD *)&v128 + 1) = v106;
        LOBYTE(v11) = v105(a1 + 1232, &v128);
        if ( !(_BYTE)v11 )
          return (char)v11;
        v99 = 0;
        v107 = 0;
        break;
      default:
        LOBYTE(v11) = a3 - 8;
        return (char)v11;
    }
    while ( 1 )
    {
      v108 = (__int64 (__fastcall ***)(_QWORD, __int128 *))(v107 + a1 + 1272);
      v109 = **v108;
      v129 = 0;
      v110 = a2[1];
      if ( v110 )
      {
        _InterlockedIncrement((volatile signed __int32 *)(v110 + 8));
        v110 = a2[1];
      }
      *(_QWORD *)&v129 = *a2;
      *((_QWORD *)&v129 + 1) = v110;
      LOBYTE(v11) = v109(v108, &v129);
      if ( (_BYTE)v11 )
        break;
      ++v99;
      v107 += 128;
      if ( v107 >= 4096 )
        return (char)v11;
    }
    v104 = 0;
LABEL_106:
    v111 = *(_QWORD *)(((__int64)v99 << 7) + a1 + 1384);
    LOBYTE(v11) = (*(__int64 (__fastcall **)(__int64, __int64))(*(_QWORD *)v111 + 16LL))(v111, v104);
  }
  return (char)v11;
}

