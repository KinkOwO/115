char __fastcall sub_1441D8460(__int64 a1, unsigned int *a2)
{
  __int64 *v3; // rcx
  __int64 v4; // rax
  _QWORD *v5; // rbx
  __int64 v6; // rdi
  __int64 v7; // rcx
  __int64 v8; // rax
  void (__fastcall ***v9)(_QWORD); // rcx
  __int64 *v10; // rax
  __int64 v11; // r15
  __int64 v12; // r14
  __int64 v13; // r14
  __int64 v14; // rdi
  __int64 v15; // rbx
  unsigned int *v16; // rdi
  unsigned int *i; // rbx
  char **v18; // r14
  char v19; // bl
  __int64 v20; // rcx
  __int64 v21; // rax
  void (__fastcall ***v22)(_QWORD); // rcx
  __int64 v23; // r15
  __int64 v24; // rax
  void (__fastcall ***v25)(_QWORD); // rcx
  __int128 v26; // kr00_16
  __int64 v27; // rbx
  __int64 v28; // rdi
  __int64 v29; // rbx
  __int64 v30; // rax
  void (__fastcall ***v31)(_QWORD); // rcx
  __int64 v32; // rbx
  __int64 v33; // rcx
  __int64 v34; // rax
  void (__fastcall ***v35)(_QWORD); // rcx
  __int64 v36; // rcx
  __int64 v37; // rcx
  __int64 v38; // r12
  _DWORD *v39; // r15
  _DWORD *j; // rbx
  _DWORD *v41; // rdx
  unsigned int *v42; // rax
  __int64 v43; // rax
  unsigned int *v44; // r15
  unsigned int *k; // rbx
  __int64 v46; // rax
  __int64 v47; // rax
  __int64 v48; // r8
  __int64 v49; // rcx
  __int64 v50; // rax
  __int64 v51; // rcx
  __int64 v52; // rax
  __int64 v53; // rax
  __int64 v54; // rdi
  __int64 v55; // rcx
  __int64 v56; // rax
  void (__fastcall ***v57)(_QWORD); // rcx
  __int64 v58; // rax
  unsigned int *v59; // rbx
  unsigned int *v60; // r15
  char *v61; // rcx
  unsigned __int64 v62; // rdx
  char *v63; // rax
  char *v64; // r8
  unsigned int v65; // r9d
  char *v66; // rdx
  unsigned __int64 v67; // rdx
  unsigned int *v68; // rax
  __int64 v69; // rdi
  __int64 v70; // rcx
  __int64 v71; // rax
  void (__fastcall ***v72)(_QWORD); // rcx
  __int64 v73; // rax
  unsigned int *v74; // rbx
  unsigned int *v75; // r15
  unsigned __int64 v76; // rdx
  char *v77; // rax
  char *v78; // r8
  unsigned int v79; // r9d
  char *v80; // rdx
  unsigned int *v81; // rax
  __int64 v82; // rdi
  __int64 v83; // rcx
  __int64 v84; // rax
  void (__fastcall ***v85)(_QWORD); // rcx
  __int64 v86; // rax
  unsigned int *v87; // rbx
  unsigned int *v88; // r15
  unsigned __int64 v89; // rdx
  char *v90; // rax
  char *v91; // r8
  unsigned int v92; // r9d
  char *v93; // rdx
  unsigned int *v94; // rax
  __int64 v95; // rcx
  unsigned __int64 v96; // rdx
  __int64 v97; // rcx
  unsigned __int64 v98; // rdx
  __int128 v100; // [rsp+30h] [rbp-49h] BYREF
  __int64 v101; // [rsp+40h] [rbp-39h]
  char *v102; // [rsp+48h] [rbp-31h] BYREF
  __int128 v103; // [rsp+50h] [rbp-29h]
  __int128 v104; // [rsp+60h] [rbp-19h] BYREF
  __int64 v105; // [rsp+70h] [rbp-9h]
  __int128 *v106; // [rsp+78h] [rbp-1h]
  __int128 v107; // [rsp+80h] [rbp+7h] BYREF
  __int64 v108; // [rsp+90h] [rbp+17h]
  __int64 v109; // [rsp+98h] [rbp+1Fh]
  __int128 *v111; // [rsp+F8h] [rbp+7Fh] BYREF

  v109 = -2;
  if ( (unsigned __int8)sub_145ADB2C0() || !(unsigned __int8)sub_145F6F4B0(a1) )
    return 0;
  (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 4008) + 16LL))(*(_QWORD *)(a1 + 4008), 0);
  v3 = *(__int64 **)(a1 + 4008);
  v4 = *v3;
  v106 = (__int128 *)&v111;
  v111 = 0;
  (*(void (__fastcall **)(__int64 *, __int128 **))(v4 + 664))(v3, &v111);
  v5 = (_QWORD *)(a1 + 4184);
  v6 = 4;
  do
  {
    (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)*(v5 - 5) + 16LL))(*(v5 - 5), 0);
    *((_DWORD *)v5 - 2) = 0;
    (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)*v5 + 24LL))(*v5, 0);
    (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)*v5 + 16LL))(*v5, 0);
    v5 += 15;
    --v6;
  }
  while ( v6 );
  sub_1441EC780(a1);
  v7 = *(_QWORD *)(a1 + 1576);
  if ( v7 )
    (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v7 + 16LL))(v7, 0);
  *(_DWORD *)(a1 + 4072) = 0;
  if ( !qword_14E638F28 )
  {
    v8 = sub_146E8BA20(1472);
    v111 = (__int128 *)v8;
    if ( v8 )
      v9 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v8);
    else
      v9 = 0;
    qword_14E638F28 = (__int64)v9;
    (**v9)(v9);
  }
  v10 = (__int64 *)sub_140157CF0();
  v104 = 0;
  v105 = 0;
  v11 = *v10;
  v12 = v10[1];
  if ( *v10 != v12 )
  {
    v13 = v12 - v11;
    *(_QWORD *)&v104 = sub_140157580(&v104, v13 >> 2);
    *((_QWORD *)&v104 + 1) = v104;
    v14 = 4 * (v13 >> 2);
    v105 = v14 + v104;
    v111 = &v104;
    v15 = v104;
    sub_148AA1E60(v104, v11, v13);
    *((_QWORD *)&v104 + 1) = v15 + v14;
    v111 = 0;
  }
  v16 = (unsigned int *)*((_QWORD *)&v104 + 1);
  for ( i = (unsigned int *)v104; i != v16; ++i )
  {
    sub_1441E0630(a1, *i, *(unsigned int *)(a1 + 4072));
    *(_DWORD *)(a1 + 4072) = (*(_DWORD *)(a1 + 4072) + 1) % 4;
  }
  v18 = (char **)(a1 + 3328);
  *(_QWORD *)(a1 + 3336) = *(_QWORD *)(a1 + 3328);
  if ( !qword_14E66C090 || (unsigned int)sub_1459A90F0(qword_14E66C090) == 1 )
  {
    v20 = qword_14E638F28;
    if ( !qword_14E638F28 )
    {
      v21 = sub_146E8BA20(1472);
      v111 = (__int128 *)v21;
      if ( v21 )
        v22 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v21);
      else
        v22 = 0;
      qword_14E638F28 = (__int64)v22;
      (**v22)(v22);
      v20 = qword_14E638F28;
    }
    sub_1444EBC10(v20, &v107, 1);
    v23 = qword_14E638F28;
    if ( !qword_14E638F28 )
    {
      v24 = sub_146E8BA20(1472);
      v111 = (__int128 *)v24;
      if ( v24 )
        v25 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v24);
      else
        v25 = 0;
      qword_14E638F28 = (__int64)v25;
      (**v25)(v25);
      v23 = qword_14E638F28;
    }
    v106 = &v100;
    v100 = 0;
    v101 = 0;
    v26 = v107;
    if ( (_QWORD)v107 != *((_QWORD *)&v107 + 1) )
    {
      v27 = (__int64)(*((_QWORD *)&v107 + 1) - v107) >> 2;
      *(_QWORD *)&v100 = sub_140157580(&v100, v27);
      *((_QWORD *)&v100 + 1) = v100;
      v28 = 4 * v27;
      v101 = 4 * v27 + v100;
      v111 = &v100;
      v29 = v100;
      sub_148AA1E60(v100, v26, *((_QWORD *)&v26 + 1) - v26);
      *((_QWORD *)&v100 + 1) = v29 + v28;
      v111 = 0;
    }
    sub_1444F1B60(v23, &v100);
    if ( !qword_14E638F28 )
    {
      v30 = sub_146E8BA20(1472);
      v111 = (__int128 *)v30;
      if ( v30 )
        v31 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v30);
      else
        v31 = 0;
      qword_14E638F28 = (__int64)v31;
      (**v31)(v31);
    }
    v32 = sub_1444EBC00();
    v33 = qword_14E638F28;
    if ( !qword_14E638F28 )
    {
      v34 = sub_146E8BA20(1472);
      v111 = (__int128 *)v34;
      if ( v34 )
        v35 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v34);
      else
        v35 = 0;
      qword_14E638F28 = (__int64)v35;
      (**v35)(v35);
      v33 = qword_14E638F28;
    }
    sub_1444F1BE0(v33, v32);
    v36 = *(_QWORD *)(a1 + 13608);
    if ( v36 )
      (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v36 + 24LL))(v36, 0);
    v37 = *(_QWORD *)(a1 + 1744);
    if ( v37 )
      (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v37 + 24LL))(v37, 0);
    if ( a2 )
    {
      if ( *((_QWORD *)a2 + 1) != *((_QWORD *)a2 + 2) )
      {
        v38 = sub_1401550D0(&v102, a2 + 2);
        v106 = (__int128 *)v38;
        v39 = *(_DWORD **)(v38 + 8);
        for ( j = *(_DWORD **)v38; j != v39; ++j )
        {
          v41 = *(_DWORD **)(a1 + 3336);
          if ( v41 == *(_DWORD **)(a1 + 3344) )
          {
            sub_140154010(a1 + 3328, v41, j);
          }
          else
          {
            *v41 = *j;
            *(_QWORD *)(a1 + 3336) += 4LL;
          }
        }
        sub_1401574A0(v38);
      }
      sub_1441E1C70(a1, *a2, 0xFFFFFFFFLL);
      v42 = (unsigned int *)*((_QWORD *)a2 + 1);
      if ( v42 != *((unsigned int **)a2 + 2) && (v43 = sub_1444EBAB0(*v42)) != 0 && *(_DWORD *)(v43 + 8) == 1 )
        sub_1441DB060(a1 + 11968, **((unsigned int **)a2 + 1));
      else
        *(_DWORD *)(a1 + 13416) = 0;
      v44 = (unsigned int *)*((_QWORD *)a2 + 2);
      for ( k = (unsigned int *)*((_QWORD *)a2 + 1); k != v44; ++k )
      {
        v46 = sub_1441C4080(a1, *a2);
        (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v46 + 88LL))(v46, *k);
      }
    }
    else
    {
      sub_1441E1C70(a1, 0, 0xFFFFFFFFLL);
    }
    if ( *(_QWORD *)(a1 + 15544) )
      sub_14045F4D0(a1 + 15544);
    v47 = sub_146E9F2A0(9824);
    v111 = (__int128 *)v47;
    if ( v47 )
      v48 = sub_145BB3500(v47, 1);
    else
      v48 = 0;
    *(_QWORD *)(a1 + 15544) = v48;
    if ( v48 && qword_14EF2CAA0 && *(_DWORD *)(qword_14EF2CAA0 + 8) )
    {
      v49 = qword_14EF2CAA8 - 48;
      if ( !qword_14EF2CAA8 )
        v49 = 0;
      if ( v49 )
      {
        sub_145BF2890(v48, 0);
        if ( !qword_14EF2CAA0 || (v50 = qword_14EF2CAA8, !*(_DWORD *)(qword_14EF2CAA0 + 8)) )
          v50 = 0;
        v51 = v50 - 48;
        if ( !v50 )
          v51 = 0;
        v52 = sub_145BDB770(v51);
        v53 = sub_140176170(v52);
        sub_1401F2380(a1 + 15552, v53);
        (*(void (__fastcall **)(_QWORD, __int64, _QWORD, _QWORD, _BYTE))(**(_QWORD **)(a1 + 15544) + 2432LL))(
          *(_QWORD *)(a1 + 15544),
          a1 + 15552,
          0,
          0,
          0);
      }
    }
    sub_1441ECDB0(a1, 0);
    sub_1441ECDB0(a1, 1);
    sub_1441ECDB0(a1, 2);
    v100 = 0;
    v54 = 0;
    v101 = 0;
    v55 = qword_14E638F28;
    if ( !qword_14E638F28 )
    {
      v56 = sub_146E8BA20(1472);
      v111 = (__int128 *)v56;
      if ( v56 )
        v57 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v56);
      else
        v57 = 0;
      qword_14E638F28 = (__int64)v57;
      (**v57)(v57);
      v55 = qword_14E638F28;
    }
    v58 = sub_1444EBC10(v55, &v102, 6);
    if ( &v100 == (__int128 *)v58 )
    {
      v60 = (unsigned int *)*((_QWORD *)&v100 + 1);
      v59 = (unsigned int *)v100;
    }
    else
    {
      v59 = *(unsigned int **)v58;
      *(_QWORD *)&v100 = *(_QWORD *)v58;
      v60 = *(unsigned int **)(v58 + 8);
      *((_QWORD *)&v100 + 1) = v60;
      v54 = *(_QWORD *)(v58 + 16);
      v101 = v54;
      *(_QWORD *)v58 = 0;
      *(_QWORD *)(v58 + 8) = 0;
      *(_QWORD *)(v58 + 16) = 0;
    }
    v61 = v102;
    if ( v102 )
    {
      v62 = 4 * ((__int64)(*((_QWORD *)&v103 + 1) - (_QWORD)v102) >> 2);
      if ( v62 >= 0x1000 )
      {
        v62 += 39LL;
        v61 = (char *)*((_QWORD *)v102 - 1);
        if ( (unsigned __int64)(v102 - v61 - 8) > 0x1F )
          sub_148AAF304(v61, v62);
      }
      sub_146E9F3A0(v61, v62);
      v102 = 0;
      v103 = 0;
    }
    if ( v59 != v60 )
    {
      sub_1441E0820(a1, 6, *v59);
      v61 = *v18;
      v63 = *v18;
      v64 = *(char **)(a1 + 3336);
      if ( *v18 != v64 )
      {
        v65 = *v59;
        while ( *(_DWORD *)v63 != v65 )
        {
          v63 += 4;
          if ( v63 == v64 )
            goto LABEL_102;
        }
        while ( 1 )
        {
          v66 = v61 + 4;
          if ( *(_DWORD *)v61 == v65 )
            break;
          v61 += 4;
          if ( v66 == v64 )
            goto LABEL_101;
        }
        sub_148AA1E60(v61, v66, v64 - v66);
        *(_QWORD *)(a1 + 3336) -= 4LL;
LABEL_101:
        sub_1441EC510(a1);
      }
    }
LABEL_102:
    if ( v59 )
    {
      v67 = 4 * ((v54 - (__int64)v59) >> 2);
      v68 = v59;
      if ( v67 >= 0x1000 )
      {
        v67 += 39LL;
        v59 = (unsigned int *)*((_QWORD *)v59 - 1);
        if ( (unsigned __int64)((char *)v68 - (char *)v59 - 8) > 0x1F )
          goto LABEL_174;
      }
      sub_146E9F3A0(v59, v67);
      v100 = 0;
      v101 = 0;
    }
    sub_1441ECDB0(a1, 3);
    sub_1441ECDB0(a1, 4);
    sub_1441ECDB0(a1, 5);
    v100 = 0;
    v69 = 0;
    v101 = 0;
    v70 = qword_14E638F28;
    if ( !qword_14E638F28 )
    {
      v71 = sub_146E8BA20(1472);
      v111 = (__int128 *)v71;
      if ( v71 )
        v72 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v71);
      else
        v72 = 0;
      qword_14E638F28 = (__int64)v72;
      (**v72)(v72);
      v70 = qword_14E638F28;
    }
    v73 = sub_1444EBC10(v70, &v102, 7);
    if ( &v100 == (__int128 *)v73 )
    {
      v75 = (unsigned int *)*((_QWORD *)&v100 + 1);
      v74 = (unsigned int *)v100;
    }
    else
    {
      v74 = *(unsigned int **)v73;
      *(_QWORD *)&v100 = *(_QWORD *)v73;
      v75 = *(unsigned int **)(v73 + 8);
      *((_QWORD *)&v100 + 1) = v75;
      v69 = *(_QWORD *)(v73 + 16);
      v101 = v69;
      *(_QWORD *)v73 = 0;
      *(_QWORD *)(v73 + 8) = 0;
      *(_QWORD *)(v73 + 16) = 0;
    }
    v61 = v102;
    if ( v102 )
    {
      v76 = 4 * ((__int64)(*((_QWORD *)&v103 + 1) - (_QWORD)v102) >> 2);
      if ( v76 >= 0x1000 )
      {
        v76 += 39LL;
        v61 = (char *)*((_QWORD *)v102 - 1);
        if ( (unsigned __int64)(v102 - v61 - 8) > 0x1F )
          sub_148AAF304(v61, v76);
      }
      sub_146E9F3A0(v61, v76);
      v102 = 0;
      v103 = 0;
    }
    if ( v74 != v75 )
    {
      sub_1441E0820(a1, 7, *v74);
      v61 = *v18;
      v77 = *v18;
      v78 = *(char **)(a1 + 3336);
      if ( *v18 != v78 )
      {
        v79 = *v74;
        while ( *(_DWORD *)v77 != v79 )
        {
          v77 += 4;
          if ( v77 == v78 )
            goto LABEL_129;
        }
        while ( 1 )
        {
          v80 = v61 + 4;
          if ( *(_DWORD *)v61 == v79 )
            break;
          v61 += 4;
          if ( v80 == v78 )
            goto LABEL_128;
        }
        sub_148AA1E60(v61, v80, v78 - v80);
        *(_QWORD *)(a1 + 3336) -= 4LL;
LABEL_128:
        sub_1441EC510(a1);
      }
    }
LABEL_129:
    if ( v74 )
    {
      v67 = 4 * ((v69 - (__int64)v74) >> 2);
      v81 = v74;
      if ( v67 >= 0x1000 )
      {
        v67 += 39LL;
        v74 = (unsigned int *)*((_QWORD *)v74 - 1);
        if ( (unsigned __int64)((char *)v81 - (char *)v74 - 8) > 0x1F )
          goto LABEL_174;
      }
      sub_146E9F3A0(v74, v67);
      v100 = 0;
      v101 = 0;
    }
    v100 = 0;
    v82 = 0;
    v101 = 0;
    v83 = qword_14E638F28;
    if ( !qword_14E638F28 )
    {
      v84 = sub_146E8BA20(1472);
      v111 = (__int128 *)v84;
      if ( v84 )
        v85 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v84);
      else
        v85 = 0;
      qword_14E638F28 = (__int64)v85;
      (**v85)(v85);
      v83 = qword_14E638F28;
    }
    v86 = sub_1444EBC10(v83, &v102, 8);
    if ( &v100 == (__int128 *)v86 )
    {
      v88 = (unsigned int *)*((_QWORD *)&v100 + 1);
      v87 = (unsigned int *)v100;
    }
    else
    {
      v87 = *(unsigned int **)v86;
      *(_QWORD *)&v100 = *(_QWORD *)v86;
      v88 = *(unsigned int **)(v86 + 8);
      *((_QWORD *)&v100 + 1) = v88;
      v82 = *(_QWORD *)(v86 + 16);
      v101 = v82;
      *(_QWORD *)v86 = 0;
      *(_QWORD *)(v86 + 8) = 0;
      *(_QWORD *)(v86 + 16) = 0;
    }
    v61 = v102;
    if ( v102 )
    {
      v89 = 4 * ((__int64)(*((_QWORD *)&v103 + 1) - (_QWORD)v102) >> 2);
      if ( v89 >= 0x1000 )
      {
        v89 += 39LL;
        v61 = (char *)*((_QWORD *)v102 - 1);
        if ( (unsigned __int64)(v102 - v61 - 8) > 0x1F )
          sub_148AAF304(v61, v89);
      }
      sub_146E9F3A0(v61, v89);
      v102 = 0;
      v103 = 0;
    }
    if ( v87 != v88 )
    {
      sub_1441E0820(a1, 8, *v87);
      v61 = *v18;
      v90 = *v18;
      v91 = *(char **)(a1 + 3336);
      if ( *v18 != v91 )
      {
        v92 = *v87;
        while ( *(_DWORD *)v90 != v92 )
        {
          v90 += 4;
          if ( v90 == v91 )
            goto LABEL_156;
        }
        while ( 1 )
        {
          v93 = v61 + 4;
          if ( *(_DWORD *)v61 == v92 )
            break;
          v61 += 4;
          if ( v93 == v91 )
            goto LABEL_155;
        }
        sub_148AA1E60(v61, v93, v91 - v93);
        *(_QWORD *)(a1 + 3336) -= 4LL;
LABEL_155:
        sub_1441EC510(a1);
      }
    }
LABEL_156:
    if ( !v87 )
      goto LABEL_160;
    v67 = 4 * ((v82 - (__int64)v87) >> 2);
    v94 = v87;
    if ( v67 < 0x1000
      || (v67 += 39LL,
          v87 = (unsigned int *)*((_QWORD *)v87 - 1),
          (unsigned __int64)((char *)v94 - (char *)v87 - 8) <= 0x1F) )
    {
      sub_146E9F3A0(v87, v67);
      v100 = 0;
      v101 = 0;
LABEL_160:
      sub_1441EC510(a1);
      (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)a1 + 400LL))(a1, 0);
      v19 = 1;
      v95 = v107;
      if ( (_QWORD)v107 )
      {
        v96 = 4 * ((v108 - (__int64)v107) >> 2);
        if ( v96 >= 0x1000 )
        {
          v96 += 39LL;
          v95 = *(_QWORD *)(v107 - 8);
          if ( (unsigned __int64)(v107 - v95 - 8) > 0x1F )
            sub_148AAF304(v95, v96);
        }
        sub_146E9F3A0(v95, v96);
        v107 = 0;
        v108 = 0;
      }
      goto LABEL_164;
    }
LABEL_174:
    sub_148AAF304(v61, v67);
  }
  v19 = 0;
LABEL_164:
  v97 = v104;
  if ( (_QWORD)v104 )
  {
    v98 = (v105 - v104) & 0xFFFFFFFFFFFFFFFCuLL;
    if ( v98 >= 0x1000 )
    {
      v98 += 39LL;
      v97 = *(_QWORD *)(v104 - 8);
      if ( (unsigned __int64)(v104 - v97 - 8) > 0x1F )
        sub_148AAF304(v97, v98);
    }
    sub_146E9F3A0(v97, v98);
    v104 = 0;
    v105 = 0;
  }
  return v19;
}
