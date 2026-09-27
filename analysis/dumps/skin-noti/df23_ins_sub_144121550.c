// ins_sub_144121550

__int64 __fastcall sub_144121550(__int64 a1, unsigned int a2, int a3)
{
  __int64 v4; // r14
  __int64 v5; // rax
  __int64 v6; // r15
  volatile signed __int32 *v7; // rbx
  __int64 v8; // r13
  _QWORD *ThreadLocalStoragePointer; // rbx
  __int64 v10; // rax
  __int64 v11; // rax
  __int64 v12; // rax
  __int64 v13; // r8
  char *v14; // r10
  __int64 v15; // r9
  __int64 v16; // r11
  char *v17; // rax
  int v18; // ecx
  int v19; // edx
  __int64 v20; // rdx
  int v21; // eax
  int v22; // ecx
  int v23; // ecx
  int v24; // ecx
  int v25; // ecx
  __int64 v26; // rdx
  __int64 v27; // rcx
  __int64 v28; // rbx
  __int64 v29; // rdi
  __int64 v30; // r8
  __int64 v31; // r10
  unsigned __int16 *v32; // rax
  __int64 v33; // r9
  int v34; // ecx
  unsigned __int16 *v35; // rax
  __int64 v36; // rax
  __int64 v37; // rbx
  __int64 v38; // rdi
  __int64 v39; // r8
  __int64 v40; // r10
  unsigned __int16 *v41; // rax
  __int64 v42; // r9
  unsigned __int16 *v43; // rax
  __int64 v44; // rax
  __int64 v45; // rbx
  __int64 v46; // rdi
  __int64 v47; // r8
  __int64 v48; // r10
  unsigned __int16 *v49; // rax
  __int64 v50; // r9
  unsigned __int16 *v51; // rax
  __int64 v52; // rax
  __int64 v53; // rbx
  __int64 v54; // rdi
  __int64 v55; // r8
  __int64 v56; // r10
  unsigned __int16 *v57; // rax
  __int64 v58; // r9
  unsigned __int16 *v59; // rax
  __int64 v60; // rax
  __int64 v61; // rbx
  __int64 v62; // rdi
  __int64 v63; // r8
  __int64 v64; // r10
  unsigned __int16 *v65; // rax
  __int64 v66; // r9
  unsigned __int16 *v67; // rax
  __int64 v68; // rax
  __int64 v69; // rbx
  __int64 v70; // rdi
  __int64 v71; // r8
  __int64 v72; // r10
  unsigned __int16 *v73; // rax
  __int64 v74; // r9
  unsigned __int16 *v75; // rax
  __int64 v76; // rax
  __int64 v77; // rax
  int v78; // eax
  __int64 v79; // r8
  __int64 v80; // rbx
  __int64 v81; // rdi
  __int64 *v82; // rdx
  int *v83; // rax
  __int64 *v84; // rcx
  __int64 v85; // rax
  __int64 v86; // rbx
  __int64 v87; // rax
  __int64 v88; // rax
  void (__fastcall ***v89)(_QWORD); // rcx
  __int64 v90; // rax
  volatile signed __int32 *v91; // rbx
  __int64 v92; // rcx
  __int64 v93; // rax
  void (__fastcall ***v94)(_QWORD); // rcx
  __int64 v95; // rax
  int v96; // ecx
  int v97; // r9d
  _QWORD *v98; // rax
  _QWORD *v99; // rax
  volatile signed __int32 *v100; // rbx
  volatile signed __int32 *v101; // rbx
  __int128 v103; // [rsp+38h] [rbp-D0h]
  __int64 i; // [rsp+50h] [rbp-B8h]
  __int64 v105; // [rsp+B8h] [rbp-50h] BYREF
  volatile signed __int32 *v106; // [rsp+C0h] [rbp-48h]
  __int64 v107; // [rsp+C8h] [rbp-40h] BYREF
  __int64 v108; // [rsp+D0h] [rbp-38h]
  __int64 v109; // [rsp+D8h] [rbp-30h] BYREF
  volatile signed __int32 *v110; // [rsp+E0h] [rbp-28h]
  __int128 v111; // [rsp+E8h] [rbp-20h]
  __int64 v112; // [rsp+F8h] [rbp-10h]
  __int64 v113; // [rsp+100h] [rbp-8h]
  __int64 v114; // [rsp+108h] [rbp+0h]
  __int64 v115; // [rsp+110h] [rbp+8h]
  __int128 v116; // [rsp+118h] [rbp+10h] BYREF
  __int128 v117; // [rsp+128h] [rbp+20h] BYREF
  __int128 v118; // [rsp+138h] [rbp+30h] BYREF
  __int128 v119; // [rsp+148h] [rbp+40h] BYREF
  __int128 v120; // [rsp+158h] [rbp+50h] BYREF
  __int128 v121; // [rsp+168h] [rbp+60h] BYREF
  char v122[8]; // [rsp+178h] [rbp+70h] BYREF
  volatile signed __int32 *v123; // [rsp+180h] [rbp+78h]
  char v124[8]; // [rsp+188h] [rbp+80h] BYREF
  volatile signed __int32 *v125; // [rsp+190h] [rbp+88h]
  char v126[32]; // [rsp+198h] [rbp+90h] BYREF
  int v127; // [rsp+1F8h] [rbp+F0h] BYREF

  v112 = -2;
  if ( a3 != 13 )
    return 0;
  v4 = a1 - 1336;
  v5 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)(a1 - 1336) + 712LL))(a1 - 1336);
  v6 = v5;
  if ( !v5 )
    return 0;
  sub_145F6B590(v5, &v105, a2);
  if ( !v105 )
  {
    v7 = v106;
    if ( v106 )
    {
      if ( _InterlockedExchangeAdd(v106 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v7)(v7);
        if ( _InterlockedExchangeAdd(v7 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v7 + 8LL))(v7);
      }
    }
    return 0;
  }
  v8 = (unsigned int)dword_14F3BEE58;
  ThreadLocalStoragePointer = NtCurrentTeb()->ThreadLocalStoragePointer;
  v10 = ThreadLocalStoragePointer[dword_14F3BEE58];
  for ( i = v10; ; v10 = i )
  {
    while ( 1 )
    {
      if ( dword_14E662758 > *(_DWORD *)(v10 + 420620) )
      {
        sub_148860450(&dword_14E662758);
        if ( dword_14E662758 == -1 )
        {
          qword_14E662748 = 0;
          qword_14E662750 = 0;
          v11 = sub_146E8BA20(48);
          *(_QWORD *)v11 = v11;
          *(_QWORD *)(v11 + 8) = v11;
          *(_QWORD *)(v11 + 16) = v11;
          *(_WORD *)(v11 + 24) = 257;
          qword_14E662748 = v11;
          sub_14885FFE8(sub_149023880);
          sub_1488603F0(&dword_14E662758);
        }
      }
      if ( dword_14E662764 > *(_DWORD *)(ThreadLocalStoragePointer[v8] + 420620LL) )
      {
        sub_148860450(&dword_14E662764);
        if ( dword_14E662764 == -1 )
        {
          sub_1439A4AA0(&unk_14E662760);
          sub_14885FFE8(sub_149023850);
          sub_1488603F0(&dword_14E662764);
        }
      }
      if ( byte_14E66275C )
        break;
      sub_146E8BE40(&unk_14E662760);
      if ( !byte_14E66275C )
        goto LABEL_40;
      sub_146E8BED0(&unk_14E662760);
      v10 = i;
    }
    v12 = sub_140E7FA20(v105);
    v14 = (char *)v12;
    if ( *(_QWORD *)(v12 + 24) >= 8u )
      v14 = *(char **)v12;
    v15 = *(_QWORD *)(qword_14E662748 + 8);
    v16 = qword_14E662748;
    while ( !*(_BYTE *)(v15 + 25) )
    {
      v17 = *(char **)(v15 + 32);
      v13 = v14 - v17;
      do
      {
        v18 = *(unsigned __int16 *)&v17[v13];
        v19 = *(unsigned __int16 *)v17 - v18;
        if ( v19 )
          break;
        v17 += 2;
      }
      while ( v18 );
      if ( v19 >= 0 )
      {
        v16 = v15;
        v15 = *(_QWORD *)v15;
      }
      else
      {
        v15 = *(_QWORD *)(v15 + 16);
      }
    }
    if ( *(_BYTE *)(v16 + 25) )
      goto LABEL_207;
    v20 = *(_QWORD *)(v16 + 32) - (_QWORD)v14;
    do
    {
      v21 = *(unsigned __int16 *)&v14[v20];
      v22 = *(unsigned __int16 *)v14 - v21;
      if ( v22 )
        break;
      v14 += 2;
    }
    while ( v21 );
    if ( v22 < 0 || v16 == qword_14E662748 )
      goto LABEL_207;
    v23 = *(_DWORD *)(v16 + 40);
    if ( v23 <= 203 )
    {
      if ( v23 == 203 )
      {
LABEL_93:
        if ( byte_14E66275C )
        {
          if ( qword_14E683C08 && (unsigned __int8)sub_145F0B700(qword_14E683C08) )
          {
            v92 = qword_14E634248;
            if ( !qword_14E634248 )
            {
              v93 = sub_146E8BA20(2496);
              v115 = v93;
              if ( v93 )
                v94 = (void (__fastcall ***)(_QWORD))sub_1456918F0(v93);
              else
                v94 = 0;
              qword_14E634248 = (__int64)v94;
              (**v94)(v94);
              v92 = qword_14E634248;
            }
            v95 = sub_145693930(v92, 675);
            if ( v95 )
              sub_141FB4B90(v95);
          }
          v96 = qword_14E682A38;
          if ( qword_14E682A38 )
          {
            v97 = 5;
LABEL_203:
            sub_144CFE360(v96, 1, 9, v97, 0);
            goto LABEL_207;
          }
          goto LABEL_207;
        }
        v53 = sub_146E8C7D0(&unk_14A212830);
        v54 = qword_14E662748;
        v55 = *(_QWORD *)(qword_14E662748 + 8);
        *(_QWORD *)&v103 = v55;
        DWORD2(v103) = 0;
        v56 = qword_14E662748;
        while ( !*(_BYTE *)(v55 + 25) )
        {
          *(_QWORD *)&v103 = v55;
          v57 = *(unsigned __int16 **)(v55 + 32);
          v58 = v53 - (_QWORD)v57;
          do
          {
            v27 = *(unsigned __int16 *)((char *)v57 + v58);
            v26 = *v57 - (unsigned int)v27;
            if ( (_DWORD)v26 )
              break;
            ++v57;
          }
          while ( (_DWORD)v27 );
          if ( (int)v26 >= 0 )
          {
            DWORD2(v103) = 1;
            v56 = v55;
            v55 = *(_QWORD *)v55;
          }
          else
          {
            DWORD2(v103) = 0;
            v55 = *(_QWORD *)(v55 + 16);
          }
        }
        if ( *(_BYTE *)(v56 + 25) )
          goto LABEL_107;
        v59 = (unsigned __int16 *)v53;
        do
        {
          v27 = *(unsigned __int16 *)((char *)v59 + *(_QWORD *)(v56 + 32) - v53);
          v26 = *v59 - (unsigned int)v27;
          if ( (_DWORD)v26 )
            break;
          ++v59;
        }
        while ( (_DWORD)v27 );
        if ( (int)v26 < 0 )
        {
LABEL_107:
          if ( qword_14E662750 == 0x555555555555555LL )
            goto LABEL_212;
          v60 = sub_146E8BA20(48);
          *(_QWORD *)(v60 + 32) = v53;
          *(_DWORD *)(v60 + 40) = 203;
          *(_QWORD *)v60 = v54;
          *(_QWORD *)(v60 + 8) = v54;
          *(_QWORD *)(v60 + 16) = v54;
          *(_WORD *)(v60 + 24) = 0;
          v119 = v103;
          sub_14014F0E0(&qword_14E662748, &v119, v60);
        }
LABEL_109:
        if ( byte_14E66275C )
        {
          if ( !qword_14E661068 )
          {
            v88 = sub_146E8BA20(8);
            v114 = v88;
            if ( v88 )
              v89 = (void (__fastcall ***)(_QWORD))sub_143F9F700(v88);
            else
              v89 = 0;
            qword_14E661068 = (__int64)v89;
            (**v89)(v89);
          }
          sub_143FA0D70();
          v90 = sub_146E8C7D0(&unk_14A212890);
          sub_145F6B5E0(v6, &v109, v90);
          if ( v109 )
          {
            (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v109 + 24LL))(v109, 0);
            (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v109 + 16LL))(v109, 0);
          }
          v91 = v110;
          if ( v110 )
          {
            if ( _InterlockedExchangeAdd(v110 + 2, 0xFFFFFFFF) == 1 )
            {
              (**(void (__fastcall ***)(volatile signed __int32 *))v91)(v91);
              if ( _InterlockedExchangeAdd(v91 + 3, 0xFFFFFFFF) == 1 )
                (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v91 + 8LL))(v91);
            }
          }
          goto LABEL_207;
        }
        v61 = sub_146E8C7D0(&unk_14A212878);
        v62 = qword_14E662748;
        v63 = *(_QWORD *)(qword_14E662748 + 8);
        *(_QWORD *)&v103 = v63;
        DWORD2(v103) = 0;
        v64 = qword_14E662748;
        while ( !*(_BYTE *)(v63 + 25) )
        {
          *(_QWORD *)&v103 = v63;
          v65 = *(unsigned __int16 **)(v63 + 32);
          v66 = v61 - (_QWORD)v65;
          do
          {
            v27 = *(unsigned __int16 *)((char *)v65 + v66);
            v26 = *v65 - (unsigned int)v27;
            if ( (_DWORD)v26 )
              break;
            ++v65;
          }
          while ( (_DWORD)v27 );
          if ( (int)v26 >= 0 )
          {
            DWORD2(v103) = 1;
            v64 = v63;
            v63 = *(_QWORD *)v63;
          }
          else
          {
            DWORD2(v103) = 0;
            v63 = *(_QWORD *)(v63 + 16);
          }
        }
        if ( *(_BYTE *)(v64 + 25) )
          goto LABEL_123;
        v67 = (unsigned __int16 *)v61;
        do
        {
          v27 = *(unsigned __int16 *)((char *)v67 + *(_QWORD *)(v64 + 32) - v61);
          v26 = *v67 - (unsigned int)v27;
          if ( (_DWORD)v26 )
            break;
          ++v67;
        }
        while ( (_DWORD)v27 );
        if ( (int)v26 < 0 )
        {
LABEL_123:
          if ( qword_14E662750 == 0x555555555555555LL )
            goto LABEL_212;
          v68 = sub_146E8BA20(48);
          *(_QWORD *)(v68 + 32) = v61;
          *(_DWORD *)(v68 + 40) = 219;
          *(_QWORD *)v68 = v62;
          *(_QWORD *)(v68 + 8) = v62;
          *(_QWORD *)(v68 + 16) = v62;
          *(_WORD *)(v68 + 24) = 0;
          v120 = v103;
          sub_14014F0E0(&qword_14E662748, &v120, v68);
        }
        goto LABEL_125;
      }
      if ( v23 && (v24 = v23 - 184) != 0 )
      {
        v25 = v24 - 6;
        if ( v25 )
        {
          if ( v25 != 6 )
            goto LABEL_207;
LABEL_77:
          if ( byte_14E66275C )
          {
            LOBYTE(v13) = 1;
            if ( !(unsigned __int8)sub_146682140(qword_14E683C78, 903, v13) )
              sub_14668C520(qword_14E683C78, 903, 0, 0);
            v96 = qword_14E682A38;
            if ( qword_14E682A38 )
            {
              v97 = 2;
              goto LABEL_203;
            }
            goto LABEL_207;
          }
          v45 = sub_146E8C7D0(&unk_149D05C48);
          v46 = qword_14E662748;
          v47 = *(_QWORD *)(qword_14E662748 + 8);
          *(_QWORD *)&v103 = v47;
          DWORD2(v103) = 0;
          v48 = qword_14E662748;
          while ( !*(_BYTE *)(v47 + 25) )
          {
            *(_QWORD *)&v103 = v47;
            v49 = *(unsigned __int16 **)(v47 + 32);
            v50 = v45 - (_QWORD)v49;
            do
            {
              v27 = *(unsigned __int16 *)((char *)v49 + v50);
              v26 = *v49 - (unsigned int)v27;
              if ( (_DWORD)v26 )
                break;
              ++v49;
            }
            while ( (_DWORD)v27 );
            if ( (int)v26 >= 0 )
            {
              DWORD2(v103) = 1;
              v48 = v47;
              v47 = *(_QWORD *)v47;
            }
            else
            {
              DWORD2(v103) = 0;
              v47 = *(_QWORD *)(v47 + 16);
            }
          }
          if ( *(_BYTE *)(v48 + 25) )
            goto LABEL_91;
          v51 = (unsigned __int16 *)v45;
          do
          {
            v27 = *(unsigned __int16 *)((char *)v51 + *(_QWORD *)(v48 + 32) - v45);
            v26 = *v51 - (unsigned int)v27;
            if ( (_DWORD)v26 )
              break;
            ++v51;
          }
          while ( (_DWORD)v27 );
          if ( (int)v26 < 0 )
          {
LABEL_91:
            if ( qword_14E662750 == 0x555555555555555LL )
              goto LABEL_212;
            v52 = sub_146E8BA20(48);
            *(_QWORD *)(v52 + 32) = v45;
            *(_DWORD *)(v52 + 40) = 196;
            *(_QWORD *)v52 = v46;
            *(_QWORD *)(v52 + 8) = v46;
            *(_QWORD *)(v52 + 16) = v46;
            *(_WORD *)(v52 + 24) = 0;
            v118 = v103;
            sub_14014F0E0(&qword_14E662748, &v118, v52);
          }
          goto LABEL_93;
        }
      }
      else
      {
        if ( byte_14E66275C )
        {
          if ( qword_14E682A38 )
            sub_144CFE360(qword_14E682A38, 1, 9, 4, 0);
          (*(void (__fastcall **)(__int64))(*(_QWORD *)v4 + 264LL))(v4);
          goto LABEL_207;
        }
LABEL_40:
        v28 = sub_146E8C7D0(&unk_1491E0F68);
        v29 = qword_14E662748;
        v30 = *(_QWORD *)(qword_14E662748 + 8);
        *(_QWORD *)&v103 = v30;
        DWORD2(v103) = 0;
        v31 = qword_14E662748;
        while ( !*(_BYTE *)(v30 + 25) )
        {
          *(_QWORD *)&v103 = v30;
          v32 = *(unsigned __int16 **)(v30 + 32);
          v33 = v28 - (_QWORD)v32;
          do
          {
            v27 = *(unsigned __int16 *)((char *)v32 + v33);
            v26 = *v32 - (unsigned int)v27;
            if ( (_DWORD)v26 )
              break;
            ++v32;
          }
          while ( (_DWORD)v27 );
          if ( (int)v26 >= 0 )
          {
            DWORD2(v103) = 1;
            v31 = v30;
            v30 = *(_QWORD *)v30;
          }
          else
          {
            DWORD2(v103) = 0;
            v30 = *(_QWORD *)(v30 + 16);
          }
        }
        if ( *(_BYTE *)(v31 + 25) )
          goto LABEL_59;
        v35 = (unsigned __int16 *)v28;
        do
        {
          v27 = *(unsigned __int16 *)((char *)v35 + *(_QWORD *)(v31 + 32) - v28);
          v26 = *v35 - (unsigned int)v27;
          if ( (_DWORD)v26 )
            break;
          ++v35;
        }
        while ( (_DWORD)v27 );
        if ( (int)v26 < 0 )
        {
LABEL_59:
          if ( qword_14E662750 == 0x555555555555555LL )
            goto LABEL_212;
          v36 = sub_146E8BA20(48);
          *(_QWORD *)(v36 + 32) = v28;
          *(_DWORD *)(v36 + 40) = 184;
          *(_QWORD *)v36 = v29;
          *(_QWORD *)(v36 + 8) = v29;
          *(_QWORD *)(v36 + 16) = v29;
          *(_WORD *)(v36 + 24) = 0;
          v116 = v103;
          sub_14014F0E0(&qword_14E662748, &v116, v36);
        }
      }
      if ( byte_14E66275C )
      {
        v98 = (_QWORD *)sub_145F6B590(v6, v124, a2);
        sub_142757420(*v98);
        v99 = (_QWORD *)sub_145F6B590(v6, v122, a2);
        sub_146ECA0C0(*v99);
        sub_144120F30(v4);
        v100 = v123;
        if ( v123 )
        {
          if ( _InterlockedExchangeAdd(v123 + 2, 0xFFFFFFFF) == 1 )
          {
            (**(void (__fastcall ***)(volatile signed __int32 *))v100)(v100);
            if ( _InterlockedExchangeAdd(v100 + 3, 0xFFFFFFFF) == 1 )
              (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v100 + 8LL))(v100);
          }
        }
        v101 = v125;
        if ( v125 )
        {
          if ( _InterlockedExchangeAdd(v125 + 2, 0xFFFFFFFF) == 1 )
          {
            (**(void (__fastcall ***)(volatile signed __int32 *))v101)(v101);
            if ( _InterlockedExchangeAdd(v101 + 3, 0xFFFFFFFF) == 1 )
              (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v101 + 8LL))(v101);
          }
        }
        v96 = qword_14E682A38;
        if ( qword_14E682A38 )
        {
          v97 = 3;
          goto LABEL_203;
        }
        goto LABEL_207;
      }
      v37 = sub_146E8C7D0(&unk_149D05C30);
      v38 = qword_14E662748;
      v39 = *(_QWORD *)(qword_14E662748 + 8);
      *(_QWORD *)&v103 = v39;
      DWORD2(v103) = 0;
      v40 = qword_14E662748;
      while ( !*(_BYTE *)(v39 + 25) )
      {
        *(_QWORD *)&v103 = v39;
        v41 = *(unsigned __int16 **)(v39 + 32);
        v42 = v37 - (_QWORD)v41;
        do
        {
          v27 = *(unsigned __int16 *)((char *)v41 + v42);
          v26 = *v41 - (unsigned int)v27;
          if ( (_DWORD)v26 )
            break;
          ++v41;
        }
        while ( (_DWORD)v27 );
        if ( (int)v26 >= 0 )
        {
          DWORD2(v103) = 1;
          v40 = v39;
          v39 = *(_QWORD *)v39;
        }
        else
        {
          DWORD2(v103) = 0;
          v39 = *(_QWORD *)(v39 + 16);
        }
      }
      if ( *(_BYTE *)(v40 + 25) )
        goto LABEL_75;
      v43 = (unsigned __int16 *)v37;
      v13 = *(_QWORD *)(v40 + 32) - v37;
      do
      {
        v27 = *(unsigned __int16 *)((char *)v43 + v13);
        v26 = *v43 - (unsigned int)v27;
        if ( (_DWORD)v26 )
          break;
        ++v43;
      }
      while ( (_DWORD)v27 );
      if ( (int)v26 < 0 )
      {
LABEL_75:
        if ( qword_14E662750 == 0x555555555555555LL )
          goto LABEL_212;
        v44 = sub_146E8BA20(48);
        *(_QWORD *)(v44 + 32) = v37;
        *(_DWORD *)(v44 + 40) = 190;
        *(_QWORD *)v44 = v38;
        *(_QWORD *)(v44 + 8) = v38;
        *(_QWORD *)(v44 + 16) = v38;
        *(_WORD *)(v44 + 24) = 0;
        v117 = v103;
        sub_14014F0E0(&qword_14E662748, &v117, v44);
      }
      goto LABEL_77;
    }
    v34 = v23 - 219;
    if ( !v34 )
      goto LABEL_109;
    v27 = (unsigned int)(v34 - 13);
    if ( (_DWORD)v27 )
    {
      if ( (_DWORD)v27 != 28 )
        goto LABEL_207;
      goto LABEL_141;
    }
LABEL_125:
    if ( !byte_14E66275C )
    {
      v69 = sub_146E8C7D0(&unk_14A2128A8);
      v70 = qword_14E662748;
      v71 = *(_QWORD *)(qword_14E662748 + 8);
      *(_QWORD *)&v103 = v71;
      DWORD2(v103) = 0;
      v72 = qword_14E662748;
      while ( !*(_BYTE *)(v71 + 25) )
      {
        *(_QWORD *)&v103 = v71;
        v73 = *(unsigned __int16 **)(v71 + 32);
        v74 = v69 - (_QWORD)v73;
        do
        {
          v27 = *(unsigned __int16 *)((char *)v73 + v74);
          v26 = *v73 - (unsigned int)v27;
          if ( (_DWORD)v26 )
            break;
          ++v73;
        }
        while ( (_DWORD)v27 );
        if ( (int)v26 >= 0 )
        {
          DWORD2(v103) = 1;
          v72 = v71;
          v71 = *(_QWORD *)v71;
        }
        else
        {
          DWORD2(v103) = 0;
          v71 = *(_QWORD *)(v71 + 16);
        }
      }
      if ( *(_BYTE *)(v72 + 25) )
        goto LABEL_139;
      v75 = (unsigned __int16 *)v69;
      do
      {
        v27 = *(unsigned __int16 *)((char *)v75 + *(_QWORD *)(v72 + 32) - v69);
        v26 = *v75 - (unsigned int)v27;
        if ( (_DWORD)v26 )
          break;
        ++v75;
      }
      while ( (_DWORD)v27 );
      if ( (int)v26 < 0 )
      {
LABEL_139:
        if ( qword_14E662750 == 0x555555555555555LL )
LABEL_212:
          sub_14014F360(v27, v26);
        v76 = sub_146E8BA20(48);
        *(_QWORD *)(v76 + 32) = v69;
        *(_DWORD *)(v76 + 40) = 232;
        *(_QWORD *)v76 = v70;
        *(_QWORD *)(v76 + 8) = v70;
        *(_QWORD *)(v76 + 16) = v70;
        *(_WORD *)(v76 + 24) = 0;
        v121 = v103;
        sub_14014F0E0(&qword_14E662748, &v121, v76);
      }
LABEL_141:
      if ( byte_14E66275C )
      {
        sub_14668C520(qword_14E683C78, 3390, 0, 0);
      }
      else
      {
        v127 = 260;
        v107 = sub_146E8C7D0(&unk_14A2128C0);
        sub_140166070(&qword_14E662748, v126, &v107, &v127);
      }
      goto LABEL_207;
    }
    v77 = sub_143FA06D0(v27);
    v78 = sub_143FA08F0(v77);
    if ( v78 < 0 )
    {
      v78 = 3084;
LABEL_147:
      LOBYTE(v79) = 1;
      v80 = sub_140283D60(qword_14E683B88, (unsigned int)v78, v79);
      if ( v80 )
      {
        v81 = sub_146E8BA20(8);
        if ( v81 )
          *(_QWORD *)v81 = 0;
        else
          v81 = 0;
        v82 = *(__int64 **)(v80 + 768);
        v83 = (int *)v82[1];
        v84 = v82;
        while ( !*((_BYTE *)v83 + 25) )
        {
          if ( v83[7] >= 0 )
          {
            v84 = (__int64 *)v83;
            v83 = *(int **)v83;
          }
          else
          {
            v83 = (int *)*((_QWORD *)v83 + 2);
          }
        }
        if ( !*((_BYTE *)v84 + 25) && *((int *)v84 + 7) <= 0 && v84 != v82 )
          *(_DWORD *)v81 = *((_DWORD *)v84 + 8);
        *(_DWORD *)(v81 + 4) = 2;
        v111 = 0;
        v108 = v81;
        v85 = sub_146E8BA20(24);
        v86 = v85;
        v113 = v85;
        if ( v85 )
        {
          *(_OWORD *)v85 = 0;
          *(_DWORD *)(v85 + 8) = 1;
          *(_DWORD *)(v85 + 12) = 1;
          *(_QWORD *)v85 = off_1491D3CE8;
          *(_QWORD *)(v85 + 16) = v81;
        }
        else
        {
          v86 = 0;
        }
        *(_QWORD *)&v111 = v81;
        *((_QWORD *)&v111 + 1) = v86;
        v108 = 0;
        sub_146E9F3A0(0, 8);
        sub_14668C520(qword_14E683C78, 489, v81, 0);
        if ( v86 )
          sub_1401DC510(v86);
      }
      goto LABEL_207;
    }
    if ( v78 > 0 )
      goto LABEL_147;
    if ( qword_14E683C78 )
    {
      v87 = sub_14723C170(60489);
      sub_14668C520(qword_14E683C78, 2875, v87, 0);
    }
LABEL_207:
    if ( byte_14E66275C )
      break;
    sub_146E8BED0(&unk_14E662760);
    byte_14E66275C = 1;
    ThreadLocalStoragePointer = NtCurrentTeb()->ThreadLocalStoragePointer;
  }
  if ( v106 )
    sub_1401DC510(v106);
  return 0;
}

