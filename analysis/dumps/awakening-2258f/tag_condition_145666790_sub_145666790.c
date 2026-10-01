// sub_145666790  va=0x145666790  size=3455

__int64 __fastcall sub_145666790(__int64 a1, __int64 a2, __int64 a3)
{
  __int64 v3; // r14
  __int64 v6; // rdx
  __int64 v7; // r8
  char *v8; // r12
  _DWORD *v9; // r13
  __int64 v10; // rax
  char *v11; // r10
  __int64 v12; // r8
  __int64 v13; // r11
  char *v14; // rax
  signed __int64 v15; // r9
  int v16; // ecx
  int v17; // edx
  __int64 v18; // rdx
  int v19; // eax
  int v20; // ecx
  int v21; // ecx
  int v22; // ecx
  int v23; // ecx
  int v24; // ecx
  int v25; // ecx
  __int64 v26; // rax
  __int64 v27; // rcx
  char *v28; // rbx
  __int64 v29; // rdi
  __int64 v30; // r8
  __int64 v31; // r10
  char *v32; // rax
  signed __int64 v33; // r9
  int v34; // edx
  unsigned __int16 *v35; // rax
  int v36; // edx
  __int64 v37; // rax
  __int64 v38; // rax
  char *v39; // rbx
  __int64 v40; // rdi
  __int64 v41; // r8
  __int64 v42; // r10
  char *v43; // rax
  signed __int64 v44; // r9
  int v45; // edx
  unsigned __int16 *v46; // rax
  int v47; // edx
  __int64 v48; // rax
  __int64 v49; // rax
  char *v50; // rbx
  __int64 v51; // rdi
  __int64 v52; // r8
  __int64 v53; // r10
  char *v54; // rax
  signed __int64 v55; // r9
  int v56; // edx
  unsigned __int16 *v57; // rax
  int v58; // edx
  __int64 v59; // rax
  __int64 v60; // rax
  char *v61; // rbx
  __int64 v62; // rdi
  __int64 v63; // r8
  __int64 v64; // r10
  char *v65; // rax
  signed __int64 v66; // r9
  int v67; // edx
  unsigned __int16 *v68; // rax
  int v69; // edx
  __int64 v70; // rax
  __int64 v71; // rax
  char *v72; // rbx
  __int64 v73; // rdi
  __int64 v74; // r8
  __int64 v75; // r10
  char *v76; // rax
  signed __int64 v77; // r9
  int v78; // edx
  unsigned __int16 *v79; // rax
  int v80; // edx
  __int64 v81; // rax
  __int64 v82; // rax
  char *v83; // rbx
  __int64 v84; // rdi
  __int64 v85; // r8
  __int64 v86; // r10
  char *v87; // rax
  signed __int64 v88; // r9
  int v89; // edx
  unsigned __int16 *v90; // rax
  int v91; // edx
  __int64 v92; // rax
  __int64 v93; // rax
  char *v94; // rbx
  __int64 v95; // rdi
  __int64 v96; // r8
  __int64 v97; // r10
  char *v98; // rax
  signed __int64 v99; // r9
  int v100; // edx
  unsigned __int16 *v101; // rax
  int v102; // edx
  __int64 v103; // rax
  __int64 v104; // r8
  char v105; // al
  __int64 v106; // r8
  __int64 v107; // r8
  char v108; // al
  _BYTE *v109; // rcx
  unsigned __int8 v110; // bl
  unsigned __int64 v111; // rdx
  __int64 v112; // rcx
  __int128 v114; // [rsp+20h] [rbp-E0h]
  __int128 v115; // [rsp+40h] [rbp-C0h] BYREF
  _BYTE v116[16]; // [rsp+50h] [rbp-B0h] BYREF
  _BYTE v117[16]; // [rsp+60h] [rbp-A0h] BYREF
  __int64 *v118; // [rsp+70h] [rbp-90h] BYREF
  __int64 v119; // [rsp+78h] [rbp-88h]
  __int64 *v120; // [rsp+80h] [rbp-80h] BYREF
  __int64 v121; // [rsp+88h] [rbp-78h]
  __int64 *v122; // [rsp+90h] [rbp-70h] BYREF
  __int64 v123; // [rsp+98h] [rbp-68h]
  __int64 *v124; // [rsp+A0h] [rbp-60h] BYREF
  __int64 v125; // [rsp+A8h] [rbp-58h]
  __int64 *v126; // [rsp+B0h] [rbp-50h] BYREF
  __int64 v127; // [rsp+B8h] [rbp-48h]
  __int64 *v128; // [rsp+C0h] [rbp-40h] BYREF
  __int64 v129; // [rsp+C8h] [rbp-38h]
  __int64 *v130; // [rsp+D0h] [rbp-30h] BYREF
  __int64 v131; // [rsp+D8h] [rbp-28h]
  __int64 v132; // [rsp+E0h] [rbp-20h]
  _QWORD v133[2]; // [rsp+E8h] [rbp-18h] BYREF
  __int64 v134; // [rsp+F8h] [rbp-8h]
  unsigned __int64 v135; // [rsp+100h] [rbp+0h]

  v132 = -2;
  v3 = a3;
  v133[0] = 0;
  v134 = 0;
  v135 = 7;
  __wind
  {
    LOBYTE(a3) = 1;
    if ( (unsigned __int8)sub_1470A1A80(a1, v133, a3) == 0 )
    {
LABEL_185:
      v110 = 0;
      goto LABEL_195;
    }
    v8 = (char *)NtCurrentTeb()->ThreadLocalStoragePointer + 8 * (unsigned int)TlsIndex;
    v9 = (_DWORD *)(*(_QWORD *)v8 + 420620LL);
    while ( 1 )
    {
      while ( 1 )
      {
        while ( 1 )
        {
          if ( dword_14E66F8F0 > *v9 )
          {
            Init_thread_header(&dword_14E66F8F0, v6, v7);
            if ( dword_14E66F8F0 == -1 )
            {
              __wind
              {
                qword_14E66F8E0 = 0;
                qword_14E66F8E8 = 0;
                v10 = sub_146E8BA20(48);
                *(_QWORD *)v10 = v10;
                *(_QWORD *)(v10 + 8) = v10;
                *(_QWORD *)(v10 + 16) = v10;
                *(_WORD *)(v10 + 24) = 257;
                qword_14E66F8E0 = v10;
                atexit(sub_1490339E0);
              }
              __unwind
              {
                Init_thread_abort(&dword_14E66F8F0);
              }
              Init_thread_footer(&dword_14E66F8F0);
            }
          }
          if ( dword_14E66F8FC > *(_DWORD *)(*(_QWORD *)v8 + 420620LL) )
          {
            Init_thread_header(&dword_14E66F8FC, v6, v7);
            if ( dword_14E66F8FC == -1 )
            {
              __wind
              {
                __crt_strtox::big_integer::big_integer((__crt_strtox::big_integer *)&unk_14E66F8F8);
                atexit(sub_149033870);
              }
              __unwind
              {
                Init_thread_abort(&dword_14E66F8FC);
              }
              Init_thread_footer(&dword_14E66F8FC);
            }
          }
          if ( byte_14E66F8F4 != 0 )
            break;
          sub_146E8BE40(&unk_14E66F8F8);
          if ( byte_14E66F8F4 == 0 )
            goto LABEL_43;
          Atomic_lock_release(&unk_14E66F8F8);
        }
        v11 = (char *)v133;
        if ( v135 >= 8 )
          v11 = (char *)v133[0];
        v12 = *(_QWORD *)(qword_14E66F8E0 + 8);
        v13 = qword_14E66F8E0;
        while ( *(_BYTE *)(v12 + 25) == 0 )
        {
          v14 = *(char **)(v12 + 32);
          v15 = v11 - v14;
          do
          {
            v16 = *(unsigned __int16 *)&v14[v15];
            v17 = *(unsigned __int16 *)v14 - v16;
            if ( v17 != 0 )
              break;
            v14 += 2;
          }
          while ( v16 != 0 );
          if ( v17 >= 0 )
          {
            v13 = v12;
            v12 = *(_QWORD *)v12;
          }
          else
          {
            v12 = *(_QWORD *)(v12 + 16);
          }
        }
        if ( *(_BYTE *)(v13 + 25) != 0 )
          goto LABEL_161;
        v18 = *(_QWORD *)(v13 + 32) - (_QWORD)v11;
        do
        {
          v19 = *(unsigned __int16 *)&v11[v18];
          v20 = *(unsigned __int16 *)v11 - v19;
          if ( v20 != 0 )
            break;
          v11 += 2;
        }
        while ( v19 != 0 );
        if ( v20 < 0 || v13 == qword_14E66F8E0 )
          goto LABEL_161;
        v21 = *(_DWORD *)(v13 + 40);
        if ( v21 <= 471 )
          break;
        v24 = v21 - 472;
        if ( v24 == 0 )
          goto LABEL_110;
        v25 = v24 - 6;
        if ( v25 == 0 )
          goto LABEL_127;
        if ( v25 != 1 )
          goto LABEL_161;
LABEL_144:
        if ( byte_14E66F8F4 != 0 )
        {
LABEL_163:
          sub_146E8D5D0(v116);
          __wind
          {
            LOBYTE(v104) = 1;
            v105 = sub_1470A0D70(a1, v116, v104);
          }
          __unwind
          {
            sub_146E8D630(v116);
          }
          if ( v105 == 0 )
          {
            v109 = v116;
LABEL_184:
            sub_146E8D630(v109);
            goto LABEL_185;
          }
          if ( *(_QWORD *)(v3 + 64) == *(_QWORD *)(v3 + 72) )
          {
            sub_14018E310(v3 + 56, *(_QWORD *)(v3 + 64), v116);
          }
          else
          {
            sub_146E8D340(*(void **)(v3 + 64), v116);
            *(_QWORD *)(v3 + 64) += 16LL;
          }
          sub_146E8D630(v116);
          goto LABEL_179;
        }
        v93 = sub_146E8C7D0(&unk_14A878CD8);
        v94 = (char *)v93;
        v95 = qword_14E66F8E0;
        v96 = *(_QWORD *)(qword_14E66F8E0 + 8);
        *(_QWORD *)&v114 = v96;
        DWORD2(v114) = 0;
        v97 = qword_14E66F8E0;
        while ( *(_BYTE *)(v96 + 25) == 0 )
        {
          *(_QWORD *)&v114 = v96;
          v98 = *(char **)(v96 + 32);
          v99 = v94 - v98;
          do
          {
            v27 = *(unsigned __int16 *)&v98[v99];
            v100 = *(unsigned __int16 *)v98 - (_DWORD)v27;
            if ( v100 != 0 )
              break;
            v98 += 2;
          }
          while ( (_DWORD)v27 != 0 );
          if ( v100 >= 0 )
          {
            DWORD2(v114) = 1;
            v97 = v96;
            v96 = *(_QWORD *)v96;
          }
          else
          {
            DWORD2(v114) = 0;
            v96 = *(_QWORD *)(v96 + 16);
          }
        }
        v115 = v114;
        if ( *(_BYTE *)(v97 + 25) != 0 )
          goto LABEL_158;
        v101 = (unsigned __int16 *)v94;
        do
        {
          v27 = *(unsigned __int16 *)((char *)v101 + *(_QWORD *)(v97 + 32) - (_QWORD)v94);
          v102 = *v101 - (_DWORD)v27;
          if ( v102 != 0 )
            break;
          ++v101;
        }
        while ( (_DWORD)v27 != 0 );
        if ( v102 < 0 )
        {
LABEL_158:
          if ( qword_14E66F8E8 == 0x555555555555555LL )
LABEL_193:
            unknown_libname_7(v27);
          v130 = &qword_14E66F8E0;
          v131 = 0;
          __wind
          {
            v131 = 0;
            v103 = sub_14014CAE0(&qword_14E66F8E0, 1);
            v131 = v103;
          }
          __unwind
          {
            sub_14014EE00(&v130);
          }
          __wind
          {
            *(_QWORD *)(v103 + 32) = v94;
            *(_DWORD *)(v103 + 40) = 479;
            *(_QWORD *)v103 = v95;
            *(_QWORD *)(v103 + 8) = v95;
            *(_QWORD *)(v103 + 16) = v95;
            *(_WORD *)(v103 + 24) = 0;
          }
          __unwind
          {
            sub_14014EEA0(&v130);
          }
          __wind
          {
            v131 = 0;
          }
          __unwind
          {
            sub_14014EE70(&v130);
          }
          sub_14014F0E0(&qword_14E66F8E0, &v115, v103);
        }
LABEL_161:
        if ( byte_14E66F8F4 != 0 )
          goto LABEL_185;
LABEL_180:
        Atomic_lock_release(&unk_14E66F8F8);
        byte_14E66F8F4 = 1;
      }
      if ( v21 != 471 )
        break;
LABEL_93:
      if ( byte_14E66F8F4 != 0 )
        goto LABEL_169;
      v60 = sub_146E8C7D0(&unk_14A878C58);
      v61 = (char *)v60;
      v62 = qword_14E66F8E0;
      v63 = *(_QWORD *)(qword_14E66F8E0 + 8);
      *(_QWORD *)&v114 = v63;
      DWORD2(v114) = 0;
      v64 = qword_14E66F8E0;
      while ( *(_BYTE *)(v63 + 25) == 0 )
      {
        *(_QWORD *)&v114 = v63;
        v65 = *(char **)(v63 + 32);
        v66 = v61 - v65;
        do
        {
          v27 = *(unsigned __int16 *)&v65[v66];
          v67 = *(unsigned __int16 *)v65 - (_DWORD)v27;
          if ( v67 != 0 )
            break;
          v65 += 2;
        }
        while ( (_DWORD)v27 != 0 );
        if ( v67 >= 0 )
        {
          DWORD2(v114) = 1;
          v64 = v63;
          v63 = *(_QWORD *)v63;
        }
        else
        {
          DWORD2(v114) = 0;
          v63 = *(_QWORD *)(v63 + 16);
        }
      }
      v115 = v114;
      if ( *(_BYTE *)(v64 + 25) != 0 )
        goto LABEL_107;
      v68 = (unsigned __int16 *)v61;
      do
      {
        v27 = *(unsigned __int16 *)((char *)v68 + *(_QWORD *)(v64 + 32) - (_QWORD)v61);
        v69 = *v68 - (_DWORD)v27;
        if ( v69 != 0 )
          break;
        ++v68;
      }
      while ( (_DWORD)v27 != 0 );
      if ( v69 < 0 )
      {
LABEL_107:
        if ( qword_14E66F8E8 == 0x555555555555555LL )
          goto LABEL_193;
        v124 = &qword_14E66F8E0;
        v125 = 0;
        __wind
        {
          v125 = 0;
          v70 = sub_146E8BA20(48);
          v125 = v70;
        }
        __unwind
        {
          sub_14014EE00(&v124);
        }
        __wind
        {
          *(_QWORD *)(v70 + 32) = v61;
          *(_DWORD *)(v70 + 40) = 471;
          *(_QWORD *)v70 = v62;
          *(_QWORD *)(v70 + 8) = v62;
          *(_QWORD *)(v70 + 16) = v62;
          *(_WORD *)(v70 + 24) = 0;
        }
        __unwind
        {
          sub_14014EEA0(&v124);
        }
        __wind
        {
          v125 = 0;
        }
        __unwind
        {
          sub_14014EE70(&v124);
        }
        sub_14014F0E0(&qword_14E66F8E0, &v115, v70);
      }
LABEL_110:
      if ( byte_14E66F8F4 == 0 )
      {
        v71 = sub_146E8C7D0(&unk_14A878C80);
        v72 = (char *)v71;
        v73 = qword_14E66F8E0;
        v74 = *(_QWORD *)(qword_14E66F8E0 + 8);
        *(_QWORD *)&v114 = v74;
        DWORD2(v114) = 0;
        v75 = qword_14E66F8E0;
        while ( *(_BYTE *)(v74 + 25) == 0 )
        {
          *(_QWORD *)&v114 = v74;
          v76 = *(char **)(v74 + 32);
          v77 = v72 - v76;
          do
          {
            v27 = *(unsigned __int16 *)&v76[v77];
            v78 = *(unsigned __int16 *)v76 - (_DWORD)v27;
            if ( v78 != 0 )
              break;
            v76 += 2;
          }
          while ( (_DWORD)v27 != 0 );
          if ( v78 >= 0 )
          {
            DWORD2(v114) = 1;
            v75 = v74;
            v74 = *(_QWORD *)v74;
          }
          else
          {
            DWORD2(v114) = 0;
            v74 = *(_QWORD *)(v74 + 16);
          }
        }
        v115 = v114;
        if ( *(_BYTE *)(v75 + 25) != 0 )
          goto LABEL_124;
        v79 = (unsigned __int16 *)v72;
        do
        {
          v27 = *(unsigned __int16 *)((char *)v79 + *(_QWORD *)(v75 + 32) - (_QWORD)v72);
          v80 = *v79 - (_DWORD)v27;
          if ( v80 != 0 )
            break;
          ++v79;
        }
        while ( (_DWORD)v27 != 0 );
        if ( v80 < 0 )
        {
LABEL_124:
          if ( qword_14E66F8E8 == 0x555555555555555LL )
            goto LABEL_193;
          v126 = &qword_14E66F8E0;
          v127 = 0;
          __wind
          {
            v127 = 0;
            v81 = sub_146E8BA20(48);
            v127 = v81;
          }
          __unwind
          {
            sub_14014EE00(&v126);
          }
          __wind
          {
            *(_QWORD *)(v81 + 32) = v72;
            *(_DWORD *)(v81 + 40) = 472;
            *(_QWORD *)v81 = v73;
            *(_QWORD *)(v81 + 8) = v73;
            *(_QWORD *)(v81 + 16) = v73;
            *(_WORD *)(v81 + 24) = 0;
          }
          __unwind
          {
            sub_14014EEA0(&v126);
          }
          __wind
          {
            v127 = 0;
          }
          __unwind
          {
            sub_14014EE70(&v126);
          }
          sub_14014F0E0(&qword_14E66F8E0, &v115, v81);
        }
LABEL_127:
        if ( byte_14E66F8F4 != 0 )
          goto LABEL_163;
        v82 = sub_146E8C7D0(&unk_14A878CA8);
        v83 = (char *)v82;
        v84 = qword_14E66F8E0;
        v85 = *(_QWORD *)(qword_14E66F8E0 + 8);
        *(_QWORD *)&v114 = v85;
        DWORD2(v114) = 0;
        v86 = qword_14E66F8E0;
        while ( *(_BYTE *)(v85 + 25) == 0 )
        {
          *(_QWORD *)&v114 = v85;
          v87 = *(char **)(v85 + 32);
          v88 = v83 - v87;
          do
          {
            v27 = *(unsigned __int16 *)&v87[v88];
            v89 = *(unsigned __int16 *)v87 - (_DWORD)v27;
            if ( v89 != 0 )
              break;
            v87 += 2;
          }
          while ( (_DWORD)v27 != 0 );
          if ( v89 >= 0 )
          {
            DWORD2(v114) = 1;
            v86 = v85;
            v85 = *(_QWORD *)v85;
          }
          else
          {
            DWORD2(v114) = 0;
            v85 = *(_QWORD *)(v85 + 16);
          }
        }
        v115 = v114;
        if ( *(_BYTE *)(v86 + 25) != 0 )
          goto LABEL_141;
        v90 = (unsigned __int16 *)v83;
        do
        {
          v27 = *(unsigned __int16 *)((char *)v90 + *(_QWORD *)(v86 + 32) - (_QWORD)v83);
          v91 = *v90 - (_DWORD)v27;
          if ( v91 != 0 )
            break;
          ++v90;
        }
        while ( (_DWORD)v27 != 0 );
        if ( v91 < 0 )
        {
LABEL_141:
          if ( qword_14E66F8E8 == 0x555555555555555LL )
            goto LABEL_193;
          v128 = &qword_14E66F8E0;
          v129 = 0;
          __wind
          {
            v129 = 0;
            v92 = sub_14014CAE0(&qword_14E66F8E0, 1);
            v129 = v92;
          }
          __unwind
          {
            sub_14014EE00(&v128);
          }
          __wind
          {
            *(_QWORD *)(v92 + 32) = v83;
            *(_DWORD *)(v92 + 40) = 478;
            *(_QWORD *)v92 = v84;
            *(_QWORD *)(v92 + 8) = v84;
            *(_QWORD *)(v92 + 16) = v84;
            *(_WORD *)(v92 + 24) = 0;
          }
          __unwind
          {
            sub_14014EEA0(&v128);
          }
          __wind
          {
            v129 = 0;
          }
          __unwind
          {
            sub_14014EE70(&v128);
          }
          sub_14014F0E0(&qword_14E66F8E0, &v115, v92);
        }
        goto LABEL_144;
      }
LABEL_169:
      sub_146E8D5D0(v117);
      __wind
      {
        LOBYTE(v107) = 1;
        v108 = sub_1470A0D70(a1, v117, v107);
      }
      __unwind
      {
        sub_146E8D630(v117);
      }
      if ( v108 == 0 )
      {
        v109 = v117;
        goto LABEL_184;
      }
      if ( *(_QWORD *)(v3 + 40) == *(_QWORD *)(v3 + 48) )
      {
        sub_14018E310(v3 + 32, *(_QWORD *)(v3 + 40), v117);
      }
      else
      {
        sub_146E8D340(*(void **)(v3 + 40), v117);
        *(_QWORD *)(v3 + 40) += 16LL;
      }
      sub_146E8D630(v117);
LABEL_179:
      if ( byte_14E66F8F4 == 0 )
        goto LABEL_180;
      LOBYTE(v106) = 1;
      if ( (unsigned __int8)sub_1470A1A80(a1, v133, v106) == 0 )
        goto LABEL_185;
    }
    if ( v21 != 0 && (v22 = v21 - 455) != 0 )
    {
      v23 = v22 - 2;
      if ( v23 != 0 )
      {
        if ( v23 != 10 )
          goto LABEL_161;
LABEL_76:
        if ( byte_14E66F8F4 != 0 )
        {
          if ( (unsigned __int8)sub_145665B50(a1, v3 + 8) == 0 )
            goto LABEL_185;
          goto LABEL_179;
        }
        v49 = sub_146E8C7D0(&unk_149684168);
        v50 = (char *)v49;
        v51 = qword_14E66F8E0;
        v52 = *(_QWORD *)(qword_14E66F8E0 + 8);
        *(_QWORD *)&v114 = v52;
        DWORD2(v114) = 0;
        v53 = qword_14E66F8E0;
        while ( *(_BYTE *)(v52 + 25) == 0 )
        {
          *(_QWORD *)&v114 = v52;
          v54 = *(char **)(v52 + 32);
          v55 = v50 - v54;
          do
          {
            v27 = *(unsigned __int16 *)&v54[v55];
            v56 = *(unsigned __int16 *)v54 - (_DWORD)v27;
            if ( v56 != 0 )
              break;
            v54 += 2;
          }
          while ( (_DWORD)v27 != 0 );
          if ( v56 >= 0 )
          {
            DWORD2(v114) = 1;
            v53 = v52;
            v52 = *(_QWORD *)v52;
          }
          else
          {
            DWORD2(v114) = 0;
            v52 = *(_QWORD *)(v52 + 16);
          }
        }
        v115 = v114;
        if ( *(_BYTE *)(v53 + 25) != 0 )
          goto LABEL_90;
        v57 = (unsigned __int16 *)v50;
        do
        {
          v27 = *(unsigned __int16 *)((char *)v57 + *(_QWORD *)(v53 + 32) - (_QWORD)v50);
          v58 = *v57 - (_DWORD)v27;
          if ( v58 != 0 )
            break;
          ++v57;
        }
        while ( (_DWORD)v27 != 0 );
        if ( v58 < 0 )
        {
LABEL_90:
          if ( qword_14E66F8E8 == 0x555555555555555LL )
            goto LABEL_193;
          v122 = &qword_14E66F8E0;
          v123 = 0;
          __wind
          {
            v123 = 0;
            v59 = sub_146E8BA20(48);
            v123 = v59;
          }
          __unwind
          {
            sub_14014EE00(&v122);
          }
          __wind
          {
            *(_QWORD *)(v59 + 32) = v50;
            *(_DWORD *)(v59 + 40) = 467;
            *(_QWORD *)v59 = v51;
            *(_QWORD *)(v59 + 8) = v51;
            *(_QWORD *)(v59 + 16) = v51;
            *(_WORD *)(v59 + 24) = 0;
          }
          __unwind
          {
            sub_14014EEA0(&v122);
          }
          __wind
          {
            v123 = 0;
          }
          __unwind
          {
            sub_14014EE70(&v122);
          }
          sub_14014F0E0(&qword_14E66F8E0, &v115, v59);
        }
        goto LABEL_93;
      }
    }
    else
    {
      if ( byte_14E66F8F4 != 0 )
      {
        v110 = 1;
        goto LABEL_195;
      }
LABEL_43:
      v26 = sub_146E8C7D0(&unk_149931610);
      v28 = (char *)v26;
      v29 = qword_14E66F8E0;
      v30 = *(_QWORD *)(qword_14E66F8E0 + 8);
      *(_QWORD *)&v114 = v30;
      DWORD2(v114) = 0;
      v31 = qword_14E66F8E0;
      while ( *(_BYTE *)(v30 + 25) == 0 )
      {
        *(_QWORD *)&v114 = v30;
        v32 = *(char **)(v30 + 32);
        v33 = v28 - v32;
        do
        {
          v27 = *(unsigned __int16 *)&v32[v33];
          v34 = *(unsigned __int16 *)v32 - (_DWORD)v27;
          if ( v34 != 0 )
            break;
          v32 += 2;
        }
        while ( (_DWORD)v27 != 0 );
        if ( v34 >= 0 )
        {
          DWORD2(v114) = 1;
          v31 = v30;
          v30 = *(_QWORD *)v30;
        }
        else
        {
          DWORD2(v114) = 0;
          v30 = *(_QWORD *)(v30 + 16);
        }
      }
      v115 = v114;
      if ( *(_BYTE *)(v31 + 25) != 0 )
        goto LABEL_56;
      v35 = (unsigned __int16 *)v28;
      v12 = *(_QWORD *)(v31 + 32) - (_QWORD)v28;
      do
      {
        v27 = *(unsigned __int16 *)((char *)v35 + v12);
        v36 = *v35 - (_DWORD)v27;
        if ( v36 != 0 )
          break;
        ++v35;
      }
      while ( (_DWORD)v27 != 0 );
      if ( v36 < 0 )
      {
LABEL_56:
        if ( qword_14E66F8E8 == 0x555555555555555LL )
          goto LABEL_193;
        v118 = &qword_14E66F8E0;
        v119 = 0;
        __wind
        {
          v119 = 0;
          v37 = sub_146E8BA20(48);
          v119 = v37;
        }
        __unwind
        {
          sub_14014EE00(&v118);
        }
        __wind
        {
          *(_QWORD *)(v37 + 32) = v28;
          *(_DWORD *)(v37 + 40) = 455;
          *(_QWORD *)v37 = v29;
          *(_QWORD *)(v37 + 8) = v29;
          *(_QWORD *)(v37 + 16) = v29;
          *(_WORD *)(v37 + 24) = 0;
        }
        __unwind
        {
          sub_14014EEA0(&v118);
        }
        __wind
        {
          v119 = 0;
        }
        __unwind
        {
          sub_14014EE70(&v118);
        }
        sub_14014F0E0(&qword_14E66F8E0, &v115, v37);
      }
    }
    if ( byte_14E66F8F4 != 0 )
    {
      LOBYTE(v12) = 1;
      if ( (unsigned __int8)sub_1470A0D70(a1, a2, v12) == 0 || sub_1456678A0(a2) != 0 )
        goto LABEL_185;
      goto LABEL_179;
    }
    v38 = sub_146E8C7D0(&unk_149243320);
    v39 = (char *)v38;
    v40 = qword_14E66F8E0;
    v41 = *(_QWORD *)(qword_14E66F8E0 + 8);
    *(_QWORD *)&v114 = v41;
    DWORD2(v114) = 0;
    v42 = qword_14E66F8E0;
    while ( *(_BYTE *)(v41 + 25) == 0 )
    {
      *(_QWORD *)&v114 = v41;
      v43 = *(char **)(v41 + 32);
      v44 = v39 - v43;
      do
      {
        v27 = *(unsigned __int16 *)&v43[v44];
        v45 = *(unsigned __int16 *)v43 - (_DWORD)v27;
        if ( v45 != 0 )
          break;
        v43 += 2;
      }
      while ( (_DWORD)v27 != 0 );
      if ( v45 >= 0 )
      {
        DWORD2(v114) = 1;
        v42 = v41;
        v41 = *(_QWORD *)v41;
      }
      else
      {
        DWORD2(v114) = 0;
        v41 = *(_QWORD *)(v41 + 16);
      }
    }
    v115 = v114;
    if ( *(_BYTE *)(v42 + 25) != 0 )
      goto LABEL_73;
    v46 = (unsigned __int16 *)v39;
    do
    {
      v27 = *(unsigned __int16 *)((char *)v46 + *(_QWORD *)(v42 + 32) - (_QWORD)v39);
      v47 = *v46 - (_DWORD)v27;
      if ( v47 != 0 )
        break;
      ++v46;
    }
    while ( (_DWORD)v27 != 0 );
    if ( v47 < 0 )
    {
LABEL_73:
      if ( qword_14E66F8E8 == 0x555555555555555LL )
        goto LABEL_193;
      v120 = &qword_14E66F8E0;
      v121 = 0;
      __wind
      {
        v121 = 0;
        v48 = sub_146E8BA20(48);
        v121 = v48;
      }
      __unwind
      {
        sub_14014EE00(&v120);
      }
      __wind
      {
        *(_QWORD *)(v48 + 32) = v39;
        *(_DWORD *)(v48 + 40) = 457;
        *(_QWORD *)v48 = v40;
        *(_QWORD *)(v48 + 8) = v40;
        *(_QWORD *)(v48 + 16) = v40;
        *(_WORD *)(v48 + 24) = 0;
      }
      __unwind
      {
        sub_14014EEA0(&v120);
      }
      __wind
      {
        v121 = 0;
      }
      __unwind
      {
        sub_14014EE70(&v120);
      }
      sub_14014F0E0(&qword_14E66F8E0, &v115, v48);
    }
    goto LABEL_76;
  }
  __unwind
  {
    unknown_libname_4(v133);
  }
LABEL_195:
  if ( v135 >= 8 )
  {
    v111 = 2 * v135 + 2;
    v112 = v133[0];
    if ( v111 >= 0x1000 )
    {
      v111 = 2 * v135 + 41;
      v112 = *(_QWORD *)(v133[0] - 8LL);
      if ( (unsigned __int64)(v133[0] - v112 - 8) > 0x1F )
        invalid_parameter_noinfo_noreturn();
    }
    j_j_scalable_free(v112, v111);
  }
  v134 = 0;
  v135 = 7;
  LOWORD(v133[0]) = 0;
  return v110;
}
