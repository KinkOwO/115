// reader_sub_141FC2D90

char __fastcall sub_141FC2D90(__int64 a1, _QWORD *a2, int a3)
{
  __int64 v3; // rdi
  __int64 v5; // r14
  __int64 v6; // rcx
  volatile signed __int32 *v7; // rbx
  __int64 v8; // rax
  int v9; // eax
  __int64 v10; // rbx
  __int64 v11; // rax
  __int64 v12; // rax
  volatile signed __int32 *v13; // rbx
  __int64 v14; // rbx
  __int64 v15; // rax
  __int64 v16; // rax
  volatile signed __int32 *v17; // rbx
  __int64 v18; // rbx
  __int64 v19; // rax
  __int64 v20; // rax
  volatile signed __int32 *v21; // rbx
  __int64 v22; // rbx
  __int64 v23; // rax
  __int64 v24; // rax
  volatile signed __int32 *v25; // rbx
  __int64 v26; // rbx
  __int64 v27; // rax
  __int64 v28; // rax
  volatile signed __int32 *v29; // rbx
  __int64 v30; // rbx
  __int64 v31; // rax
  __int64 v32; // rax
  volatile signed __int32 *v33; // rbx
  __int64 v34; // rbx
  __int64 v35; // rax
  __int64 v36; // rax
  volatile signed __int32 *v37; // rbx
  __int64 v38; // rbx
  __int64 v39; // rax
  __int64 v40; // rax
  unsigned __int64 v41; // rdx
  __int64 v42; // rdi
  volatile signed __int32 *v43; // rbx
  volatile signed __int32 *v44; // rbx
  volatile signed __int32 *v45; // rbx
  volatile signed __int32 *v46; // rbx
  volatile signed __int32 *v47; // rbx
  volatile signed __int32 *v48; // rbx
  volatile signed __int32 *v49; // rbx
  __int64 v50; // r12
  int v51; // esi
  __int64 v52; // rcx
  __int64 v53; // r8
  __int64 v54; // rcx
  unsigned int *v55; // rdx
  __int64 v56; // rdx
  __int64 v57; // rsi
  void (__fastcall *v58)(_QWORD, _QWORD); // rax
  _QWORD *v59; // rdi
  unsigned __int64 v60; // r14
  __int64 v61; // rbx
  __int64 v62; // rdi
  __int64 (__fastcall *v63)(__int64, __int64, __int64); // rbx
  __int64 v64; // rax
  int v65; // ebx
  __int64 v66; // r14
  void (__fastcall *v67)(__int64, __int64); // rsi
  _QWORD *v68; // rbx
  __int64 v69; // rax
  __int64 v70; // rax
  __int64 v71; // rax
  unsigned __int64 v72; // rdx
  __int64 v73; // rcx
  unsigned __int64 v74; // rdx
  __int64 v75; // rcx
  unsigned __int8 v76; // di
  __int64 v77; // rdx
  bool v78; // bl
  __int64 v79; // rdx
  _BOOL8 v80; // rdx
  int v82; // [rsp+20h] [rbp-E0h]
  __int64 v83; // [rsp+28h] [rbp-D8h] BYREF
  volatile signed __int32 *v84; // [rsp+30h] [rbp-D0h]
  __int64 v85; // [rsp+38h] [rbp-C8h] BYREF
  volatile signed __int32 *v86; // [rsp+40h] [rbp-C0h]
  __int64 v87; // [rsp+48h] [rbp-B8h] BYREF
  volatile signed __int32 *v88; // [rsp+50h] [rbp-B0h]
  __int64 v89; // [rsp+58h] [rbp-A8h] BYREF
  volatile signed __int32 *v90; // [rsp+60h] [rbp-A0h]
  __int64 v91; // [rsp+68h] [rbp-98h] BYREF
  volatile signed __int32 *v92; // [rsp+70h] [rbp-90h]
  __int128 v93; // [rsp+78h] [rbp-88h] BYREF
  unsigned __int64 v94; // [rsp+88h] [rbp-78h]
  __int64 v95; // [rsp+90h] [rbp-70h]
  void (__fastcall *v96)(_QWORD, _QWORD); // [rsp+A0h] [rbp-60h]
  _QWORD v97[2]; // [rsp+A8h] [rbp-58h] BYREF
  __int64 v98; // [rsp+B8h] [rbp-48h]
  __int64 v99; // [rsp+C0h] [rbp-40h] BYREF
  volatile signed __int32 *v100; // [rsp+C8h] [rbp-38h]
  __int64 v101; // [rsp+D0h] [rbp-30h] BYREF
  volatile signed __int32 *v102; // [rsp+D8h] [rbp-28h]
  __int64 v103; // [rsp+E0h] [rbp-20h]
  _QWORD *v104; // [rsp+E8h] [rbp-18h]
  __int128 *v105; // [rsp+F0h] [rbp-10h]
  _BYTE v106[8]; // [rsp+F8h] [rbp-8h] BYREF
  volatile signed __int32 *v107; // [rsp+100h] [rbp+0h]
  _BYTE v108[8]; // [rsp+108h] [rbp+8h] BYREF
  volatile signed __int32 *v109; // [rsp+110h] [rbp+10h]
  _BYTE v110[8]; // [rsp+118h] [rbp+18h] BYREF
  volatile signed __int32 *v111; // [rsp+120h] [rbp+20h]
  _BYTE v112[8]; // [rsp+128h] [rbp+28h] BYREF
  volatile signed __int32 *v113; // [rsp+130h] [rbp+30h]
  _BYTE v114[8]; // [rsp+138h] [rbp+38h] BYREF
  volatile signed __int32 *v115; // [rsp+140h] [rbp+40h]
  _BYTE v116[8]; // [rsp+148h] [rbp+48h] BYREF
  volatile signed __int32 *v117; // [rsp+150h] [rbp+50h]
  _BYTE v118[8]; // [rsp+158h] [rbp+58h] BYREF
  volatile signed __int32 *v119; // [rsp+160h] [rbp+60h]
  __int64 v120; // [rsp+168h] [rbp+68h] BYREF
  __int64 v121; // [rsp+178h] [rbp+78h]
  unsigned __int64 v122; // [rsp+180h] [rbp+80h]
  _QWORD v123[2]; // [rsp+188h] [rbp+88h] BYREF
  __int64 v124; // [rsp+198h] [rbp+98h]
  unsigned __int64 v125; // [rsp+1A0h] [rbp+A0h]

  v103 = -2;
  v3 = a3;
  v5 = a1;
  v97[0] = a1;
  v104 = a2;
  v6 = *(_QWORD *)(a1 + 168);
  if ( !v6 )
  {
    v7 = (volatile signed __int32 *)a2[1];
LABEL_115:
    if ( v7 )
    {
      if ( _InterlockedExchangeAdd(v7 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v7)(v7);
        if ( _InterlockedExchangeAdd(v7 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v7 + 8LL))(v7);
      }
    }
    return 0;
  }
  v8 = sub_1401E65D0(v6);
  v9 = sub_140BB0090(v8);
  if ( (int)v3 < 0 || (int)v3 >= v9 )
  {
    v7 = (volatile signed __int32 *)a2[1];
    goto LABEL_115;
  }
  v10 = *a2;
  v11 = sub_146E8C7D0(&unk_1491D2200);
  v12 = sub_146EC8E30(v10, v106, v11);
  sub_1402425E0(&v85, v12);
  v13 = v107;
  if ( v107 )
  {
    if ( _InterlockedExchangeAdd(v107 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v13)(v13);
      if ( _InterlockedExchangeAdd(v13 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v13 + 8LL))(v13);
    }
  }
  v14 = *a2;
  v15 = sub_146E8C7D0(&unk_1491AB478);
  v16 = sub_146EC8E30(v14, v108, v15);
  sub_1401E9D00(&v101, v16);
  v17 = v109;
  if ( v109 )
  {
    if ( _InterlockedExchangeAdd(v109 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v17)(v17);
      if ( _InterlockedExchangeAdd(v17 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v17 + 8LL))(v17);
    }
  }
  v18 = *a2;
  v19 = sub_146E8C7D0(&unk_1492530D0);
  v20 = sub_146EC8E30(v18, v110, v19);
  sub_1401E9D00(&v91, v20);
  v21 = v111;
  if ( v111 )
  {
    if ( _InterlockedExchangeAdd(v111 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v21)(v21);
      if ( _InterlockedExchangeAdd(v21 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v21 + 8LL))(v21);
    }
  }
  v22 = *a2;
  v23 = sub_146E8C7D0(&unk_1492BDD28);
  v24 = sub_146EC8E30(v22, v112, v23);
  sub_1401E9A20(&v89, v24);
  v25 = v113;
  if ( v113 )
  {
    if ( _InterlockedExchangeAdd(v113 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v25)(v25);
      if ( _InterlockedExchangeAdd(v25 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v25 + 8LL))(v25);
    }
  }
  v26 = *a2;
  v27 = sub_146E8C7D0(&unk_1492BE468);
  v28 = sub_146EC8E30(v26, v114, v27);
  sub_1402424B0(&v83, v28);
  v29 = v115;
  if ( v115 )
  {
    if ( _InterlockedExchangeAdd(v115 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v29)(v29);
      if ( _InterlockedExchangeAdd(v29 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v29 + 8LL))(v29);
    }
  }
  v30 = *a2;
  v31 = sub_146E8C7D0(&unk_1496E57F8);
  v32 = sub_146EC8E30(v30, v116, v31);
  sub_1401E9C70(&v87, v32);
  v33 = v117;
  if ( v117 )
  {
    if ( _InterlockedExchangeAdd(v117 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v33)(v33);
      if ( _InterlockedExchangeAdd(v33 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v33 + 8LL))(v33);
    }
  }
  v34 = *a2;
  v35 = sub_146E8C7D0(&unk_1494927F8);
  v36 = sub_146EC8E30(v34, v118, v35);
  sub_1401E9C70(&v99, v36);
  v37 = v119;
  if ( v119 )
  {
    if ( _InterlockedExchangeAdd(v119 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v37)(v37);
      if ( _InterlockedExchangeAdd(v37 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v37 + 8LL))(v37);
    }
  }
  v38 = *(_QWORD *)(sub_141FB3A30(*(_QWORD *)(v5 + 168)) + 8 * v3 + 4);
  v98 = v38;
  v39 = sub_1401E65D0(*(_QWORD *)(v5 + 168));
  v40 = sub_147317840(v39, (unsigned int)v3);
  v42 = v40;
  if ( v40 )
  {
    v50 = (int)v38;
    v51 = *(_DWORD *)(v40 + 8);
    v82 = v51;
    if ( !*(_DWORD *)(v40 + 4) )
    {
      v52 = *(int *)sub_141FB3A30(*(_QWORD *)(v5 + 168));
      v50 = (int)v38 - v52 + (int)sub_145A11A50(v52);
      v53 = *(int *)(v42 + 48);
      if ( (_DWORD)v53 )
      {
        v50 /= v53;
        v41 = (unsigned int)(v51 >> 31);
        LODWORD(v41) = v51 % (int)v53;
        v82 = v51 / (int)v53;
      }
    }
    v54 = v85;
    if ( v85 )
    {
      v55 = *(unsigned int **)(v42 + 56);
      if ( !((__int64)(*(_QWORD *)(v42 + 64) - (_QWORD)v55) >> 3)
        || (sub_14501C1F0(v85, *v55), v54 = *(_QWORD *)(v42 + 56), !((*(_QWORD *)(v42 + 64) - v54) >> 3)) )
      {
        sub_1401790B0(v54, v55);
      }
      sub_14501C490(v85, *(unsigned int *)(v54 + 4));
      LOBYTE(v56) = 1;
      sub_14501C4E0(v85, v56);
    }
    v57 = v101;
    if ( v101 )
    {
      v58 = *(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)v101 + 680LL);
      v96 = v58;
      v105 = &v93;
      *(_QWORD *)&v93 = 0;
      v94 = 0;
      v95 = 0;
      v59 = (_QWORD *)(v42 + 80);
      v60 = v59[2];
      if ( v59[3] >= 8u )
        v59 = (_QWORD *)*v59;
      if ( v60 >= 8 )
      {
        v61 = v60 | 7;
        if ( (v60 | 7) > 0x7FFFFFFFFFFFFFFELL )
          v61 = 0x7FFFFFFFFFFFFFFELL;
        *(_QWORD *)&v93 = sub_14014CB50(&v93, v61 + 1);
        sub_148AA1E60(v93, v59, 2 * v60 + 2);
        v58 = v96;
      }
      else
      {
        v93 = *(_OWORD *)v59;
        v61 = 7;
      }
      v94 = v60;
      v95 = v61;
      v58(v57, &v93);
      v5 = v97[0];
    }
    if ( v91 )
    {
      if ( v82 < v50 )
        v50 = v82;
      v123[0] = 0;
      v124 = 0;
      v125 = 7;
      v120 = 0;
      v121 = 0;
      v122 = 7;
      v62 = *(_QWORD *)(v5 + 32);
      if ( v62 )
      {
        v63 = *(__int64 (__fastcall **)(__int64, __int64, __int64))(*(_QWORD *)(v62 + 16) + 80LL);
        v64 = sub_146E8C7D0(&unk_1498D22A0);
        v65 = v63(v62 + 16, v64, 1);
        if ( v65 < 1 )
          v65 = 1;
        sub_146EA0080((unsigned int)v50, v123, (unsigned int)v65);
        sub_146EA0080((unsigned int)v82, &v120, (unsigned int)v65);
      }
      v66 = v91;
      v67 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v91 + 688LL);
      v68 = v123;
      if ( v125 >= 8 )
        v68 = (_QWORD *)v123[0];
      v69 = sub_146E8C7D0(&unk_1498D22C8);
      v70 = sub_146E8CF20(v97, v69, v68);
      v71 = sub_14014F430(v70);
      v67(v66, v71);
      sub_146E8C910(v97);
      if ( v122 >= 8 )
      {
        v72 = 2 * v122 + 2;
        v73 = v120;
        if ( v72 >= 0x1000 )
        {
          v72 = 2 * v122 + 41;
          v73 = *(_QWORD *)(v120 - 8);
          if ( (unsigned __int64)(v120 - v73 - 8) > 0x1F )
            sub_148AAF304(v73, v72);
        }
        sub_146E9F3A0(v73, v72);
      }
      v121 = 0;
      v122 = 7;
      LOWORD(v120) = 0;
      v41 = v125;
      if ( v125 >= 8 )
      {
        v74 = 2 * v125 + 2;
        v75 = v123[0];
        if ( v74 >= 0x1000 )
        {
          v74 = 2 * v125 + 41;
          v75 = *(_QWORD *)(v123[0] - 8LL);
          if ( (unsigned __int64)(v123[0] - v75 - 8) > 0x1F )
            sub_148AAF304(v75, v74);
        }
        sub_146E9F3A0(v75, v74);
      }
      v124 = 0;
      v125 = 7;
      LOWORD(v123[0]) = 0;
    }
    v76 = BYTE4(v98);
    if ( v89 )
    {
      LOBYTE(v41) = BYTE4(v98) == 0;
      (*(void (__fastcall **)(__int64, unsigned __int64))(*(_QWORD *)v89 + 16LL))(v89, v41);
      LOBYTE(v77) = v50 >= v82;
      (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v89 + 24LL))(v89, v77);
    }
    if ( v87 && v83 )
    {
      v78 = 0;
      if ( (unsigned __int8)sub_141FB6530(v83) )
        v78 = (unsigned __int8)sub_146AEF960(v83) == 0;
      if ( !v76 || v78 )
        v79 = 0;
      else
        LOBYTE(v79) = 1;
      (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v87 + 16LL))(v87, v79);
      v80 = v76 && v78;
      (*(void (__fastcall **)(__int64, _BOOL8))(*(_QWORD *)v83 + 16LL))(v83, v80);
    }
    if ( v99 )
      (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v99 + 16LL))(v99, v76);
  }
  v43 = v100;
  if ( v100 )
  {
    if ( _InterlockedExchangeAdd(v100 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v43)(v43);
      if ( _InterlockedExchangeAdd(v43 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v43 + 8LL))(v43);
    }
  }
  v44 = v88;
  if ( v88 )
  {
    if ( _InterlockedExchangeAdd(v88 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v44)(v44);
      if ( _InterlockedExchangeAdd(v44 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v44 + 8LL))(v44);
    }
  }
  v45 = v84;
  if ( v84 )
  {
    if ( _InterlockedExchangeAdd(v84 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v45)(v45);
      if ( _InterlockedExchangeAdd(v45 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v45 + 8LL))(v45);
    }
  }
  v46 = v90;
  if ( v90 )
  {
    if ( _InterlockedExchangeAdd(v90 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v46)(v46);
      if ( _InterlockedExchangeAdd(v46 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v46 + 8LL))(v46);
    }
  }
  v47 = v92;
  if ( v92 )
  {
    if ( _InterlockedExchangeAdd(v92 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v47)(v47);
      if ( _InterlockedExchangeAdd(v47 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v47 + 8LL))(v47);
    }
  }
  v48 = v102;
  if ( v102 )
  {
    if ( _InterlockedExchangeAdd(v102 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v48)(v48);
      if ( _InterlockedExchangeAdd(v48 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v48 + 8LL))(v48);
    }
  }
  v49 = v86;
  if ( v86 )
  {
    if ( _InterlockedExchangeAdd(v86 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v49)(v49);
      if ( _InterlockedExchangeAdd(v49 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v49 + 8LL))(v49);
    }
  }
  sub_1401566D0(a2);
  return 0;
}

