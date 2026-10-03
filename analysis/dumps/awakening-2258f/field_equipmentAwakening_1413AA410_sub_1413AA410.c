// sub_1413AA410  va=0x1413AA410  size=3675

__int64 __fastcall sub_1413AA410(_QWORD *a1, __int64 a2, __int64 a3)
{
  char *v4; // r15
  _DWORD *v5; // r12
  __int64 v6; // rax
  char *v7; // r10
  __int64 v8; // r9
  __int64 v9; // r11
  char *v10; // rax
  signed __int64 v11; // r8
  int v12; // ecx
  int v13; // edx
  __int64 v14; // rdx
  int v15; // eax
  int v16; // ecx
  int v17; // eax
  unsigned int v18; // esi
  __int64 v19; // rax
  __int64 v20; // rcx
  char *v21; // rbx
  __int64 v22; // rdi
  __int64 v23; // r8
  __int64 v24; // r10
  char *v25; // rax
  signed __int64 v26; // r9
  int v27; // edx
  unsigned __int16 *v28; // rax
  int v29; // edx
  __int64 v30; // rax
  __int64 v31; // rax
  char *v32; // rbx
  __int64 v33; // rdi
  __int64 v34; // r8
  __int64 v35; // r10
  char *v36; // rax
  signed __int64 v37; // r9
  int v38; // edx
  unsigned __int16 *v39; // rax
  int v40; // edx
  __int64 v41; // rax
  __int64 v42; // rax
  char *v43; // rbx
  __int64 v44; // rdi
  __int64 v45; // r8
  __int64 v46; // r10
  char *v47; // rax
  signed __int64 v48; // r9
  int v49; // edx
  unsigned __int16 *v50; // rax
  int v51; // edx
  __int64 v52; // rax
  __int64 v53; // rax
  char *v54; // rbx
  __int64 v55; // rdi
  __int64 v56; // r8
  __int64 v57; // r10
  char *v58; // rax
  signed __int64 v59; // r9
  int v60; // edx
  unsigned __int16 *v61; // rax
  int v62; // edx
  __int64 v63; // rax
  __int64 v64; // rax
  char *v65; // rbx
  __int64 v66; // rdi
  __int64 v67; // r8
  __int64 v68; // r10
  char *v69; // rax
  signed __int64 v70; // r9
  int v71; // edx
  unsigned __int16 *v72; // rax
  int v73; // edx
  __int64 v74; // rax
  __int64 v75; // rax
  char *v76; // rbx
  __int64 v77; // rdi
  __int64 v78; // r8
  __int64 v79; // r10
  char *v80; // rax
  signed __int64 v81; // r9
  int v82; // edx
  unsigned __int16 *v83; // rax
  int v84; // edx
  __int64 v85; // rax
  __int64 v86; // rax
  char *v87; // rbx
  __int64 v88; // rdi
  __int64 v89; // r8
  __int64 v90; // r10
  char *v91; // rax
  signed __int64 v92; // r9
  int v93; // edx
  unsigned __int16 *v94; // rax
  int v95; // edx
  __int64 v96; // rax
  int v98; // [rsp+28h] [rbp-E0h] BYREF
  __int128 v99; // [rsp+30h] [rbp-D8h]
  _QWORD v100[3]; // [rsp+40h] [rbp-C8h] BYREF
  __int64 *v101; // [rsp+58h] [rbp-B0h] BYREF
  __int64 v102; // [rsp+60h] [rbp-A8h]
  __int64 *v103; // [rsp+68h] [rbp-A0h] BYREF
  __int64 v104; // [rsp+70h] [rbp-98h]
  __int64 *v105; // [rsp+78h] [rbp-90h] BYREF
  __int64 v106; // [rsp+80h] [rbp-88h]
  __int64 *v107; // [rsp+88h] [rbp-80h] BYREF
  __int64 v108; // [rsp+90h] [rbp-78h]
  __int64 *v109; // [rsp+98h] [rbp-70h] BYREF
  __int64 v110; // [rsp+A0h] [rbp-68h]
  __int64 *v111; // [rsp+A8h] [rbp-60h] BYREF
  __int64 v112; // [rsp+B0h] [rbp-58h]
  __int64 *v113; // [rsp+B8h] [rbp-50h] BYREF
  __int64 v114; // [rsp+C0h] [rbp-48h]
  __int64 v115; // [rsp+C8h] [rbp-40h]
  char v116[16]; // [rsp+D0h] [rbp-38h] BYREF
  char v117[16]; // [rsp+E0h] [rbp-28h] BYREF
  char v118[16]; // [rsp+F0h] [rbp-18h] BYREF
  char v119[16]; // [rsp+100h] [rbp-8h] BYREF
  char v120[16]; // [rsp+110h] [rbp+8h] BYREF
  char v121[16]; // [rsp+120h] [rbp+18h] BYREF
  char v122[16]; // [rsp+130h] [rbp+28h] BYREF
  void *v123; // [rsp+140h] [rbp+38h]

  v115 = -2;
  v123 = a1;
  __wind
  {
    v18 = 0;
    v4 = (char *)NtCurrentTeb()->ThreadLocalStoragePointer + 8 * (unsigned int)TlsIndex;
    v5 = (_DWORD *)(*(_QWORD *)v4 + 420620LL);
    while ( 1 )
    {
      if ( dword_14E64C8F0 > *v5 )
      {
        Init_thread_header(&dword_14E64C8F0, a2, a3);
        if ( dword_14E64C8F0 == -1 )
        {
          __wind
          {
            qword_14E64C8E0 = 0;
            qword_14E64C8E8 = 0;
            v6 = sub_146E8BA20(48);
            *(_QWORD *)v6 = v6;
            *(_QWORD *)(v6 + 8) = v6;
            *(_QWORD *)(v6 + 16) = v6;
            *(_WORD *)(v6 + 24) = 257;
            qword_14E64C8E0 = v6;
            atexit(sub_149011FE0);
          }
          __unwind
          {
            Init_thread_abort(&dword_14E64C8F0);
          }
          Init_thread_footer(&dword_14E64C8F0);
        }
      }
      if ( dword_14E64C8FC > *(_DWORD *)(*(_QWORD *)v4 + 420620LL) )
      {
        Init_thread_header(&dword_14E64C8FC, a2, a3);
        if ( dword_14E64C8FC == -1 )
        {
          __wind
          {
            __crt_strtox::big_integer::big_integer((__crt_strtox::big_integer *)&unk_14E64C8F8);
            atexit(sub_149011E70);
          }
          __unwind
          {
            Init_thread_abort(&dword_14E64C8FC);
          }
          Init_thread_footer(&dword_14E64C8FC);
        }
      }
      if ( byte_14E64C8F4 != 0 )
        break;
      sub_146E8BE40(&unk_14E64C8F8);
      if ( byte_14E64C8F4 != 0 )
      {
        Atomic_lock_release(&unk_14E64C8F8);
      }
      else
      {
        v19 = sub_146E8C7D0(&unk_149684D18);
        v21 = (char *)v19;
        v22 = qword_14E64C8E0;
        v23 = *(_QWORD *)(qword_14E64C8E0 + 8);
        *(_QWORD *)&v99 = v23;
        DWORD2(v99) = 0;
        v24 = qword_14E64C8E0;
        v100[0] = qword_14E64C8E0;
        if ( *(_BYTE *)(v23 + 25) == 0 )
        {
          do
          {
            *(_QWORD *)&v99 = v23;
            v25 = *(char **)(v23 + 32);
            v26 = v21 - v25;
            do
            {
              v20 = *(unsigned __int16 *)&v25[v26];
              v27 = *(unsigned __int16 *)v25 - (_DWORD)v20;
              if ( v27 != 0 )
                break;
              v25 += 2;
            }
            while ( (_DWORD)v20 != 0 );
            if ( v27 >= 0 )
            {
              DWORD2(v99) = 1;
              v24 = v23;
              v23 = *(_QWORD *)v23;
            }
            else
            {
              DWORD2(v99) = 0;
              v23 = *(_QWORD *)(v23 + 16);
            }
          }
          while ( *(_BYTE *)(v23 + 25) == 0 );
          v100[0] = v24;
        }
        *(_OWORD *)&v100[1] = v99;
        if ( *(_BYTE *)(v24 + 25) != 0 )
          goto LABEL_48;
        v28 = (unsigned __int16 *)v21;
        do
        {
          v20 = *(unsigned __int16 *)((char *)v28 + *(_QWORD *)(v24 + 32) - (_QWORD)v21);
          v29 = *v28 - (_DWORD)v20;
          if ( v29 != 0 )
            break;
          ++v28;
        }
        while ( (_DWORD)v20 != 0 );
        if ( v29 < 0 )
        {
LABEL_48:
          if ( qword_14E64C8E8 == 0x555555555555555LL )
            goto LABEL_189;
          v101 = &qword_14E64C8E0;
          v102 = 0;
          __wind
          {
            v102 = 0;
            v30 = sub_146E8BA20(48);
            v102 = v30;
          }
          __unwind
          {
            sub_14014EE00(&v101);
          }
          __wind
          {
            *(_QWORD *)(v30 + 32) = v21;
            *(_DWORD *)(v30 + 40) = 4586;
            *(_QWORD *)v30 = v22;
            *(_QWORD *)(v30 + 8) = v22;
            *(_QWORD *)(v30 + 16) = v22;
            *(_WORD *)(v30 + 24) = 0;
          }
          __unwind
          {
            sub_14014EEA0(&v101);
          }
          __wind
          {
            v102 = 0;
          }
          __unwind
          {
            sub_14014EE70(&v101);
          }
          sub_14014F0E0(&qword_14E64C8E0, &v100[1], v30);
        }
LABEL_51:
        if ( byte_14E64C8F4 != 0 )
        {
          v18 = 2;
        }
        else
        {
          v31 = sub_146E8C7D0(&unk_149684D48);
          v32 = (char *)v31;
          v33 = qword_14E64C8E0;
          v34 = *(_QWORD *)(qword_14E64C8E0 + 8);
          *(_QWORD *)&v99 = v34;
          DWORD2(v99) = 0;
          v35 = qword_14E64C8E0;
          v100[0] = qword_14E64C8E0;
          if ( *(_BYTE *)(v34 + 25) == 0 )
          {
            do
            {
              *(_QWORD *)&v99 = v34;
              v36 = *(char **)(v34 + 32);
              v37 = v32 - v36;
              do
              {
                v20 = *(unsigned __int16 *)&v36[v37];
                v38 = *(unsigned __int16 *)v36 - (_DWORD)v20;
                if ( v38 != 0 )
                  break;
                v36 += 2;
              }
              while ( (_DWORD)v20 != 0 );
              if ( v38 >= 0 )
              {
                DWORD2(v99) = 1;
                v35 = v34;
                v34 = *(_QWORD *)v34;
              }
              else
              {
                DWORD2(v99) = 0;
                v34 = *(_QWORD *)(v34 + 16);
              }
            }
            while ( *(_BYTE *)(v34 + 25) == 0 );
            v100[0] = v35;
          }
          *(_OWORD *)&v100[1] = v99;
          if ( *(_BYTE *)(v35 + 25) != 0 )
            goto LABEL_66;
          v39 = (unsigned __int16 *)v32;
          do
          {
            v20 = *(unsigned __int16 *)((char *)v39 + *(_QWORD *)(v35 + 32) - (_QWORD)v32);
            v40 = *v39 - (_DWORD)v20;
            if ( v40 != 0 )
              break;
            ++v39;
          }
          while ( (_DWORD)v20 != 0 );
          if ( v40 < 0 )
          {
LABEL_66:
            if ( qword_14E64C8E8 == 0x555555555555555LL )
              goto LABEL_189;
            v103 = &qword_14E64C8E0;
            v104 = 0;
            __wind
            {
              v104 = 0;
              v41 = sub_146E8BA20(48);
              v104 = v41;
            }
            __unwind
            {
              sub_14014EE00(&v103);
            }
            __wind
            {
              *(_QWORD *)(v41 + 32) = v32;
              *(_DWORD *)(v41 + 40) = 4590;
              *(_QWORD *)v41 = v33;
              *(_QWORD *)(v41 + 8) = v33;
              *(_QWORD *)(v41 + 16) = v33;
              *(_WORD *)(v41 + 24) = 0;
            }
            __unwind
            {
              sub_14014EEA0(&v103);
            }
            __wind
            {
              v104 = 0;
            }
            __unwind
            {
              sub_14014EE70(&v103);
            }
            sub_14014F0E0(&qword_14E64C8E0, &v100[1], v41);
          }
LABEL_69:
          if ( byte_14E64C8F4 != 0 )
          {
            v18 = 3;
          }
          else
          {
            v42 = sub_146E8C7D0(&unk_1495CEF90);
            v43 = (char *)v42;
            v44 = qword_14E64C8E0;
            v45 = *(_QWORD *)(qword_14E64C8E0 + 8);
            *(_QWORD *)&v99 = v45;
            DWORD2(v99) = 0;
            v46 = qword_14E64C8E0;
            v100[0] = qword_14E64C8E0;
            if ( *(_BYTE *)(v45 + 25) == 0 )
            {
              do
              {
                *(_QWORD *)&v99 = v45;
                v47 = *(char **)(v45 + 32);
                v48 = v43 - v47;
                do
                {
                  v20 = *(unsigned __int16 *)&v47[v48];
                  v49 = *(unsigned __int16 *)v47 - (_DWORD)v20;
                  if ( v49 != 0 )
                    break;
                  v47 += 2;
                }
                while ( (_DWORD)v20 != 0 );
                if ( v49 >= 0 )
                {
                  DWORD2(v99) = 1;
                  v46 = v45;
                  v45 = *(_QWORD *)v45;
                }
                else
                {
                  DWORD2(v99) = 0;
                  v45 = *(_QWORD *)(v45 + 16);
                }
              }
              while ( *(_BYTE *)(v45 + 25) == 0 );
              v100[0] = v46;
            }
            *(_OWORD *)&v100[1] = v99;
            if ( *(_BYTE *)(v46 + 25) != 0 )
              goto LABEL_84;
            v50 = (unsigned __int16 *)v43;
            do
            {
              v20 = *(unsigned __int16 *)((char *)v50 + *(_QWORD *)(v46 + 32) - (_QWORD)v43);
              v51 = *v50 - (_DWORD)v20;
              if ( v51 != 0 )
                break;
              ++v50;
            }
            while ( (_DWORD)v20 != 0 );
            if ( v51 < 0 )
            {
LABEL_84:
              if ( qword_14E64C8E8 == 0x555555555555555LL )
                goto LABEL_189;
              v105 = &qword_14E64C8E0;
              v106 = 0;
              __wind
              {
                v106 = 0;
                v52 = sub_146E8BA20(48);
                v106 = v52;
              }
              __unwind
              {
                sub_14014EE00(&v105);
              }
              __wind
              {
                *(_QWORD *)(v52 + 32) = v43;
                *(_DWORD *)(v52 + 40) = 4594;
                *(_QWORD *)v52 = v44;
                *(_QWORD *)(v52 + 8) = v44;
                *(_QWORD *)(v52 + 16) = v44;
                *(_WORD *)(v52 + 24) = 0;
              }
              __unwind
              {
                sub_14014EEA0(&v105);
              }
              __wind
              {
                v106 = 0;
              }
              __unwind
              {
                sub_14014EE70(&v105);
              }
              sub_14014F0E0(&qword_14E64C8E0, &v100[1], v52);
            }
LABEL_87:
            if ( byte_14E64C8F4 != 0 )
            {
              v18 = 4;
            }
            else
            {
              v53 = sub_146E8C7D0(&unk_149684D78);
              v54 = (char *)v53;
              v55 = qword_14E64C8E0;
              v56 = *(_QWORD *)(qword_14E64C8E0 + 8);
              *(_QWORD *)&v99 = v56;
              DWORD2(v99) = 0;
              v57 = qword_14E64C8E0;
              v100[0] = qword_14E64C8E0;
              if ( *(_BYTE *)(v56 + 25) == 0 )
              {
                do
                {
                  *(_QWORD *)&v99 = v56;
                  v58 = *(char **)(v56 + 32);
                  v59 = v54 - v58;
                  do
                  {
                    v20 = *(unsigned __int16 *)&v58[v59];
                    v60 = *(unsigned __int16 *)v58 - (_DWORD)v20;
                    if ( v60 != 0 )
                      break;
                    v58 += 2;
                  }
                  while ( (_DWORD)v20 != 0 );
                  if ( v60 >= 0 )
                  {
                    DWORD2(v99) = 1;
                    v57 = v56;
                    v56 = *(_QWORD *)v56;
                  }
                  else
                  {
                    DWORD2(v99) = 0;
                    v56 = *(_QWORD *)(v56 + 16);
                  }
                }
                while ( *(_BYTE *)(v56 + 25) == 0 );
                v100[0] = v57;
              }
              *(_OWORD *)&v100[1] = v99;
              if ( *(_BYTE *)(v57 + 25) != 0 )
                goto LABEL_102;
              v61 = (unsigned __int16 *)v54;
              do
              {
                v20 = *(unsigned __int16 *)((char *)v61 + *(_QWORD *)(v57 + 32) - (_QWORD)v54);
                v62 = *v61 - (_DWORD)v20;
                if ( v62 != 0 )
                  break;
                ++v61;
              }
              while ( (_DWORD)v20 != 0 );
              if ( v62 < 0 )
              {
LABEL_102:
                if ( qword_14E64C8E8 == 0x555555555555555LL )
                  goto LABEL_189;
                v107 = &qword_14E64C8E0;
                v108 = 0;
                __wind
                {
                  v108 = 0;
                  v63 = sub_146E8BA20(48);
                  v108 = v63;
                }
                __unwind
                {
                  sub_14014EE00(&v107);
                }
                __wind
                {
                  *(_QWORD *)(v63 + 32) = v54;
                  *(_DWORD *)(v63 + 40) = 4598;
                  *(_QWORD *)v63 = v55;
                  *(_QWORD *)(v63 + 8) = v55;
                  *(_QWORD *)(v63 + 16) = v55;
                  *(_WORD *)(v63 + 24) = 0;
                }
                __unwind
                {
                  sub_14014EEA0(&v107);
                }
                __wind
                {
                  v108 = 0;
                }
                __unwind
                {
                  sub_14014EE70(&v107);
                }
                sub_14014F0E0(&qword_14E64C8E0, &v100[1], v63);
              }
LABEL_105:
              if ( byte_14E64C8F4 != 0 )
              {
                v18 = 5;
              }
              else
              {
                v64 = sub_146E8C7D0(&unk_149684D98);
                v65 = (char *)v64;
                v66 = qword_14E64C8E0;
                v67 = *(_QWORD *)(qword_14E64C8E0 + 8);
                *(_QWORD *)&v99 = v67;
                DWORD2(v99) = 0;
                v68 = qword_14E64C8E0;
                v100[0] = qword_14E64C8E0;
                if ( *(_BYTE *)(v67 + 25) == 0 )
                {
                  do
                  {
                    *(_QWORD *)&v99 = v67;
                    v69 = *(char **)(v67 + 32);
                    v70 = v65 - v69;
                    do
                    {
                      v20 = *(unsigned __int16 *)&v69[v70];
                      v71 = *(unsigned __int16 *)v69 - (_DWORD)v20;
                      if ( v71 != 0 )
                        break;
                      v69 += 2;
                    }
                    while ( (_DWORD)v20 != 0 );
                    if ( v71 >= 0 )
                    {
                      DWORD2(v99) = 1;
                      v68 = v67;
                      v67 = *(_QWORD *)v67;
                    }
                    else
                    {
                      DWORD2(v99) = 0;
                      v67 = *(_QWORD *)(v67 + 16);
                    }
                  }
                  while ( *(_BYTE *)(v67 + 25) == 0 );
                  v100[0] = v68;
                }
                *(_OWORD *)&v100[1] = v99;
                if ( *(_BYTE *)(v68 + 25) != 0 )
                  goto LABEL_120;
                v72 = (unsigned __int16 *)v65;
                do
                {
                  v20 = *(unsigned __int16 *)((char *)v72 + *(_QWORD *)(v68 + 32) - (_QWORD)v65);
                  v73 = *v72 - (_DWORD)v20;
                  if ( v73 != 0 )
                    break;
                  ++v72;
                }
                while ( (_DWORD)v20 != 0 );
                if ( v73 < 0 )
                {
LABEL_120:
                  if ( qword_14E64C8E8 == 0x555555555555555LL )
                    goto LABEL_189;
                  v109 = &qword_14E64C8E0;
                  v110 = 0;
                  __wind
                  {
                    v110 = 0;
                    v74 = sub_146E8BA20(48);
                    v110 = v74;
                  }
                  __unwind
                  {
                    sub_14014EE00(&v109);
                  }
                  __wind
                  {
                    *(_QWORD *)(v74 + 32) = v65;
                    *(_DWORD *)(v74 + 40) = 4602;
                    *(_QWORD *)v74 = v66;
                    *(_QWORD *)(v74 + 8) = v66;
                    *(_QWORD *)(v74 + 16) = v66;
                    *(_WORD *)(v74 + 24) = 0;
                  }
                  __unwind
                  {
                    sub_14014EEA0(&v109);
                  }
                  __wind
                  {
                    v110 = 0;
                  }
                  __unwind
                  {
                    sub_14014EE70(&v109);
                  }
                  sub_14014F0E0(&qword_14E64C8E0, &v100[1], v74);
                }
LABEL_123:
                if ( byte_14E64C8F4 != 0 )
                {
                  v18 = 6;
                }
                else
                {
                  v75 = sub_146E8C7D0(&unk_149684DC0);
                  v76 = (char *)v75;
                  v77 = qword_14E64C8E0;
                  v78 = *(_QWORD *)(qword_14E64C8E0 + 8);
                  *(_QWORD *)&v99 = v78;
                  DWORD2(v99) = 0;
                  v79 = qword_14E64C8E0;
                  v100[0] = qword_14E64C8E0;
                  if ( *(_BYTE *)(v78 + 25) == 0 )
                  {
                    do
                    {
                      *(_QWORD *)&v99 = v78;
                      v80 = *(char **)(v78 + 32);
                      v81 = v76 - v80;
                      do
                      {
                        v20 = *(unsigned __int16 *)&v80[v81];
                        v82 = *(unsigned __int16 *)v80 - (_DWORD)v20;
                        if ( v82 != 0 )
                          break;
                        v80 += 2;
                      }
                      while ( (_DWORD)v20 != 0 );
                      if ( v82 >= 0 )
                      {
                        DWORD2(v99) = 1;
                        v79 = v78;
                        v78 = *(_QWORD *)v78;
                      }
                      else
                      {
                        DWORD2(v99) = 0;
                        v78 = *(_QWORD *)(v78 + 16);
                      }
                    }
                    while ( *(_BYTE *)(v78 + 25) == 0 );
                    v100[0] = v79;
                  }
                  *(_OWORD *)&v100[1] = v99;
                  if ( *(_BYTE *)(v79 + 25) != 0 )
                    goto LABEL_138;
                  v83 = (unsigned __int16 *)v76;
                  do
                  {
                    v20 = *(unsigned __int16 *)((char *)v83 + *(_QWORD *)(v79 + 32) - (_QWORD)v76);
                    v84 = *v83 - (_DWORD)v20;
                    if ( v84 != 0 )
                      break;
                    ++v83;
                  }
                  while ( (_DWORD)v20 != 0 );
                  if ( v84 < 0 )
                  {
LABEL_138:
                    if ( qword_14E64C8E8 == 0x555555555555555LL )
                      goto LABEL_189;
                    v111 = &qword_14E64C8E0;
                    v112 = 0;
                    __wind
                    {
                      v112 = 0;
                      v85 = sub_146E8BA20(48);
                      v112 = v85;
                    }
                    __unwind
                    {
                      sub_14014EE00(&v111);
                    }
                    __wind
                    {
                      *(_QWORD *)(v85 + 32) = v76;
                      *(_DWORD *)(v85 + 40) = 4606;
                      *(_QWORD *)v85 = v77;
                      *(_QWORD *)(v85 + 8) = v77;
                      *(_QWORD *)(v85 + 16) = v77;
                      *(_WORD *)(v85 + 24) = 0;
                    }
                    __unwind
                    {
                      sub_14014EEA0(&v111);
                    }
                    __wind
                    {
                      v112 = 0;
                    }
                    __unwind
                    {
                      sub_14014EE70(&v111);
                    }
                    sub_14014F0E0(&qword_14E64C8E0, &v100[1], v85);
                  }
LABEL_141:
                  if ( byte_14E64C8F4 != 0 )
                  {
                    v18 = 7;
                  }
                  else
                  {
                    v86 = sub_146E8C7D0(&unk_149684DE8);
                    v87 = (char *)v86;
                    v88 = qword_14E64C8E0;
                    v89 = *(_QWORD *)(qword_14E64C8E0 + 8);
                    *(_QWORD *)&v99 = v89;
                    DWORD2(v99) = 0;
                    v90 = qword_14E64C8E0;
                    v100[0] = qword_14E64C8E0;
                    if ( *(_BYTE *)(v89 + 25) == 0 )
                    {
                      do
                      {
                        *(_QWORD *)&v99 = v89;
                        v91 = *(char **)(v89 + 32);
                        v92 = v87 - v91;
                        do
                        {
                          v20 = *(unsigned __int16 *)&v91[v92];
                          v93 = *(unsigned __int16 *)v91 - (_DWORD)v20;
                          if ( v93 != 0 )
                            break;
                          v91 += 2;
                        }
                        while ( (_DWORD)v20 != 0 );
                        if ( v93 >= 0 )
                        {
                          DWORD2(v99) = 1;
                          v90 = v89;
                          v89 = *(_QWORD *)v89;
                        }
                        else
                        {
                          DWORD2(v99) = 0;
                          v89 = *(_QWORD *)(v89 + 16);
                        }
                      }
                      while ( *(_BYTE *)(v89 + 25) == 0 );
                      v100[0] = v90;
                    }
                    *(_OWORD *)&v100[1] = v99;
                    if ( *(_BYTE *)(v90 + 25) != 0 )
                      goto LABEL_156;
                    v94 = (unsigned __int16 *)v87;
                    do
                    {
                      v20 = *(unsigned __int16 *)((char *)v94 + *(_QWORD *)(v90 + 32) - (_QWORD)v87);
                      v95 = *v94 - (_DWORD)v20;
                      if ( v95 != 0 )
                        break;
                      ++v94;
                    }
                    while ( (_DWORD)v20 != 0 );
                    if ( v95 < 0 )
                    {
LABEL_156:
                      if ( qword_14E64C8E8 == 0x555555555555555LL )
LABEL_189:
                        unknown_libname_7(v20);
                      v113 = &qword_14E64C8E0;
                      v114 = 0;
                      __wind
                      {
                        v114 = 0;
                        v96 = sub_14014CAE0(&qword_14E64C8E0, 1);
                        v114 = v96;
                      }
                      __unwind
                      {
                        sub_14014EE00(&v113);
                      }
                      __wind
                      {
                        *(_QWORD *)(v96 + 32) = v87;
                        *(_DWORD *)(v96 + 40) = 4610;
                        *(_QWORD *)v96 = v88;
                        *(_QWORD *)(v96 + 8) = v88;
                        *(_QWORD *)(v96 + 16) = v88;
                        *(_WORD *)(v96 + 24) = 0;
                      }
                      __unwind
                      {
                        sub_14014EEA0(&v113);
                      }
                      __wind
                      {
                        v114 = 0;
                      }
                      __unwind
                      {
                        sub_14014EE70(&v113);
                      }
                      sub_14014F0E0(&qword_14E64C8E0, &v100[1], v96);
                    }
LABEL_159:
                    if ( byte_14E64C8F4 != 0 )
                    {
                      v18 = 8;
                    }
                    else
                    {
                      v98 = 4614;
                      v100[1] = sub_146E8C7D0(&unk_149684E08);
                      sub_140166070(&qword_14E64C8E0, v116, &v100[1], &v98);
LABEL_161:
                      if ( byte_14E64C8F4 != 0 )
                      {
                        v18 = 9;
                      }
                      else
                      {
                        v98 = 4618;
                        v100[1] = sub_146E8C7D0(&unk_149684E30);
                        sub_140166070(&qword_14E64C8E0, v117, &v100[1], &v98);
LABEL_163:
                        if ( byte_14E64C8F4 != 0 )
                        {
                          v18 = 10;
                        }
                        else
                        {
                          v98 = 4622;
                          v100[1] = sub_146E8C7D0(&unk_149684E58);
                          sub_140166070(&qword_14E64C8E0, v118, &v100[1], &v98);
LABEL_165:
                          if ( byte_14E64C8F4 != 0 )
                          {
                            v18 = 11;
                          }
                          else
                          {
                            v98 = 4626;
                            v100[1] = sub_146E8C7D0(&unk_149684E80);
                            sub_140166070(&qword_14E64C8E0, v119, &v100[1], &v98);
LABEL_167:
                            if ( byte_14E64C8F4 != 0 )
                            {
                              v18 = 13;
                            }
                            else
                            {
                              v98 = 4631;
                              v100[1] = sub_146E8C7D0(&unk_149684EB0);
                              sub_140166070(&qword_14E64C8E0, v120, &v100[1], &v98);
LABEL_169:
                              if ( byte_14E64C8F4 != 0 )
                              {
                                v18 = 14;
                              }
                              else
                              {
                                v98 = 4645;
                                v100[1] = sub_146E8C7D0(&unk_149684EF0);
                                sub_140166070(&qword_14E64C8E0, v121, &v100[1], &v98);
LABEL_171:
                                if ( byte_14E64C8F4 != 0 )
                                {
                                  v18 = 15;
                                }
                                else
                                {
                                  v98 = 4648;
                                  v100[1] = sub_146E8C7D0(&unk_149684F20);
                                  sub_140166070(&qword_14E64C8E0, v122, &v100[1], &v98);
                                }
                              }
                            }
                          }
                        }
                      }
                    }
                  }
                }
              }
            }
          }
        }
LABEL_186:
        if ( byte_14E64C8F4 != 0 )
          goto LABEL_190;
        Atomic_lock_release(&unk_14E64C8F8);
        byte_14E64C8F4 = 1;
      }
    }
    v7 = (char *)a1;
    if ( a1[3] >= 8u )
      v7 = (char *)*a1;
    v8 = *(_QWORD *)(qword_14E64C8E0 + 8);
    v9 = qword_14E64C8E0;
    while ( *(_BYTE *)(v8 + 25) == 0 )
    {
      v10 = *(char **)(v8 + 32);
      v11 = v7 - v10;
      do
      {
        v12 = *(unsigned __int16 *)&v10[v11];
        v13 = *(unsigned __int16 *)v10 - v12;
        if ( v13 != 0 )
          break;
        v10 += 2;
      }
      while ( v12 != 0 );
      if ( v13 >= 0 )
      {
        v9 = v8;
        v8 = *(_QWORD *)v8;
      }
      else
      {
        v8 = *(_QWORD *)(v8 + 16);
      }
    }
    if ( *(_BYTE *)(v9 + 25) == 0 )
    {
      v14 = *(_QWORD *)(v9 + 32) - (_QWORD)v7;
      do
      {
        v15 = *(unsigned __int16 *)&v7[v14];
        v16 = *(unsigned __int16 *)v7 - v15;
        if ( v16 != 0 )
          break;
        v7 += 2;
      }
      while ( v15 != 0 );
      if ( v16 >= 0 && v9 != qword_14E64C8E0 )
      {
        v17 = *(_DWORD *)(v9 + 40);
        if ( v17 > 4586 )
        {
          switch ( v17 )
          {
            case 4590:
              goto LABEL_51;
            case 4594:
              goto LABEL_69;
            case 4598:
              goto LABEL_87;
            case 4602:
              goto LABEL_105;
            case 4606:
              goto LABEL_123;
            case 4610:
              goto LABEL_141;
            case 4614:
              goto LABEL_159;
            case 4618:
              goto LABEL_161;
            case 4622:
              goto LABEL_163;
            case 4626:
              goto LABEL_165;
            case 4631:
              goto LABEL_167;
            case 4645:
              goto LABEL_169;
            case 4648:
              goto LABEL_171;
            default:
              goto LABEL_186;
          }
        }
        if ( v17 == 4586 || v17 == 0 )
          v18 = 1;
      }
    }
    goto LABEL_186;
  }
  __unwind
  {
    unknown_libname_4(v123);
  }
LABEL_190:
  unknown_libname_4(a1);
  return v18;
}
