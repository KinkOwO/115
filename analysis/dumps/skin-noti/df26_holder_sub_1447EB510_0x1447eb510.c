// holder_sub_1447EB510_0x1447eb510

void __fastcall sub_1447EB510(__int64 a1, int a2, int a3, int a4, __int64 a5, __int64 *a6, __int64 a7, int a8)
{
  __int64 v12; // r15
  __int64 v13; // rax
  void (__fastcall ***v14)(_QWORD); // rcx
  __int64 v15; // rax
  __int64 v16; // r8
  __int64 v17; // rax
  volatile signed __int32 *v18; // rbx
  __int64 v19; // r8
  __int64 v20; // rsi
  __int64 v21; // r9
  unsigned __int64 v22; // rdx
  __int64 v23; // rax
  __int64 v24; // rcx
  __int64 v25; // rcx
  __int64 v26; // rax
  void (__fastcall ***v27)(_QWORD); // rcx
  __int64 v28; // r8
  int v29; // r12d
  __int64 v30; // rcx
  __int64 v31; // rax
  void (__fastcall ***v32)(_QWORD); // rcx
  __int64 v33; // rbx
  int *v34; // rdx
  int *v35; // rcx
  int v36; // r13d
  __int64 v37; // rax
  __int64 v38; // rax
  __int64 v39; // rax
  __int64 v40; // r8
  __int64 v41; // rbx
  __int64 v42; // rax
  __int64 v43; // rax
  __int64 v44; // rax
  __int64 v45; // r8
  __int64 v46; // rbx
  float v47; // xmm0_4
  __int64 v48; // rtt
  int *v49; // rdx
  int *v50; // rcx
  int v51; // r13d
  __int64 v52; // r12
  __int64 v53; // rax
  __int64 v54; // rax
  __int64 v55; // rax
  __int64 v56; // r8
  __int64 v57; // rbx
  __int64 v58; // rax
  __int64 v59; // rax
  __int64 v60; // rax
  __int64 v61; // r8
  __int64 v62; // rdx
  _QWORD *v63; // rcx
  _QWORD *i; // rax
  float v65; // xmm6_4
  __int64 *v66; // r8
  __int64 v67; // rcx
  int v68; // eax
  int v69; // eax
  int v70; // eax
  _DWORD *v71; // r8
  _DWORD *v72; // rdx
  _DWORD *v73; // r8
  _DWORD *v74; // rdx
  _DWORD *v75; // r8
  _DWORD *v76; // rdx
  _DWORD *v77; // r8
  __int64 v78; // rax
  __int64 v79; // rax
  __int64 v80; // rdx
  __int64 v81; // rcx
  volatile signed __int32 *v82; // rbx
  volatile signed __int32 *v83; // rbx
  volatile signed __int32 *v84; // rbx
  __int64 v85; // rdx
  int v86; // edx
  int v87; // r8d
  __int64 v88; // rbx
  __int64 v89; // rax
  void (__fastcall ***v90)(_QWORD); // rcx
  int v91; // eax
  __int64 v92; // rcx
  unsigned __int64 v93; // rdx
  __int64 v94; // [rsp+30h] [rbp-D0h] BYREF
  int v95; // [rsp+38h] [rbp-C8h] BYREF
  int v96; // [rsp+3Ch] [rbp-C4h] BYREF
  int v97; // [rsp+40h] [rbp-C0h] BYREF
  int v98; // [rsp+44h] [rbp-BCh] BYREF
  int v99; // [rsp+48h] [rbp-B8h] BYREF
  int v100; // [rsp+4Ch] [rbp-B4h] BYREF
  __int128 v101; // [rsp+50h] [rbp-B0h]
  __int64 v102; // [rsp+68h] [rbp-98h]
  _BYTE v103[8]; // [rsp+70h] [rbp-90h] BYREF
  volatile signed __int32 *v104; // [rsp+78h] [rbp-88h]
  _BYTE v105[16]; // [rsp+80h] [rbp-80h] BYREF
  _BYTE v106[16]; // [rsp+90h] [rbp-70h] BYREF
  _BYTE v107[16]; // [rsp+A0h] [rbp-60h] BYREF
  _BYTE v108[16]; // [rsp+B0h] [rbp-50h] BYREF
  _BYTE v109[8]; // [rsp+C0h] [rbp-40h] BYREF
  volatile signed __int32 *v110; // [rsp+C8h] [rbp-38h]
  _BYTE v111[8]; // [rsp+D0h] [rbp-30h] BYREF
  volatile signed __int32 *v112; // [rsp+D8h] [rbp-28h]
  int v113; // [rsp+E0h] [rbp-20h] BYREF
  __int64 v114; // [rsp+E8h] [rbp-18h] BYREF
  __int128 v115; // [rsp+F0h] [rbp-10h]
  _BYTE v116[48]; // [rsp+100h] [rbp+0h] BYREF

  v102 = -2;
  v12 = qword_14E63AE60;
  if ( !qword_14E63AE60 )
  {
    v13 = sub_146E8BA20(336);
    v94 = v13;
    v14 = v13 ? (void (__fastcall ***)(_QWORD))sub_1447E41D0(v13) : 0LL;
    qword_14E63AE60 = (__int64)v14;
    (**v14)(v14);
    v12 = qword_14E63AE60;
    if ( !qword_14E63AE60 )
      return;
  }
  v15 = sub_146E8C7D0(&unk_14A3F45F0);
  LOBYTE(v16) = 1;
  v17 = sub_144724390(v103, v15, v16, 0);
  sub_1401E5080(a1 + 152, v17);
  v18 = v104;
  if ( v104 )
  {
    if ( _InterlockedExchangeAdd(v104 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v18)(v18);
      if ( _InterlockedExchangeAdd(v18 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v18 + 8LL))(v18);
    }
  }
  if ( a7 )
  {
    *(_QWORD *)(a1 + 208) = *(_QWORD *)(a7 + 208);
    *(_DWORD *)(a1 + 248) = *(_DWORD *)(a7 + 248);
    *(_DWORD *)(a1 + 252) = *(_DWORD *)(a7 + 252);
    *(_DWORD *)(a1 + 216) = *(_DWORD *)(a7 + 216);
  }
  else
  {
    *(_QWORD *)(a1 + 208) = 0;
    *(_DWORD *)(a1 + 248) = 1065353216;
  }
  sub_146E9FBD0(a1 + 176, 300, 0);
  *(_DWORD *)(a1 + 76) = a2;
  *(_DWORD *)(a1 + 80) = a3;
  *(_DWORD *)(a1 + 84) = a4;
  *(_DWORD *)(a1 + 88) = MEMORY[0xDC5CDC8]();
  v20 = abs64(a5);
  if ( a6 )
  {
    v19 = 0;
    v21 = *a6;
    v22 = (a6[1] - *a6) / 36;
    if ( v22 )
    {
      v23 = 0;
      do
      {
        v24 = (unsigned int)(int)*(float *)(v21 + 36 * v23 + 24);
        if ( !(int)*(float *)(v21 + 36 * v23 + 24) )
          v24 = 1;
        v20 += v24;
        v19 = (unsigned int)(v19 + 1);
        v23 = (unsigned int)v19;
      }
      while ( v22 > (unsigned int)v19 );
    }
  }
  v25 = qword_14E63AE60;
  if ( !qword_14E63AE60 )
  {
    v26 = sub_146E8BA20(336);
    v94 = v26;
    if ( v26 )
      v27 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v26);
    else
      v27 = 0;
    qword_14E63AE60 = (__int64)v27;
    (**v27)(v27);
    v25 = qword_14E63AE60;
  }
  v29 = ((unsigned __int8)sub_1447EDD20(v25, *(_DWORD *)(a1 + 104), v19) != 0) + 3;
  v30 = qword_14E63AE60;
  if ( !qword_14E63AE60 )
  {
    v31 = sub_146E8BA20(336);
    v94 = v31;
    if ( v31 )
      v32 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v31);
    else
      v32 = 0;
    qword_14E63AE60 = (__int64)v32;
    (**v32)(v32);
    v30 = qword_14E63AE60;
  }
  v95 = (unsigned __int8)sub_1447EDD20(v30, *(_DWORD *)(a1 + 104), v28) == 0;
  v33 = *(_QWORD *)(a1 + 208);
  v98 = 4;
  v96 = (int)sub_148AC8A40() / v29 - v95;
  v97 = 0;
  v34 = &v96;
  if ( v96 <= 0 )
    v34 = &v97;
  v35 = &v98;
  if ( *v34 <= 4 )
    v35 = v34;
  v36 = *v35;
  v94 = (unsigned int)(int)sub_148ABB2B0();
  v37 = sub_146E8C7D0(&unk_14A3F4658);
  v38 = sub_146E8CF20(v105, v37, v33 / v94);
  v39 = sub_14014F430(v38);
  v40 = -1;
  do
    ++v40;
  while ( *(_WORD *)(v39 + 2 * v40) );
  sub_14014C8D0(a1 + 264, v39);
  sub_146E8C910(v105);
  if ( v36 >= 1 )
  {
    v41 = *(_QWORD *)(a1 + 208) % v94 / (unsigned int)(int)sub_148ABB2B0();
    v42 = sub_146E8C7D0(&unk_14A3F4670);
    v43 = sub_146E8CF20(v106, v42, v41);
    v44 = sub_14014F430(v43);
    v45 = -1;
    do
      ++v45;
    while ( *(_WORD *)(v44 + 2 * v45) );
    sub_14014C8D0(a1 + 336, v44);
    sub_146E8C910(v106);
  }
  *(_DWORD *)(a1 + 260) = *(_DWORD *)(a1 + 280);
  *(_QWORD *)(a1 + 208) += v20;
  v46 = *(_QWORD *)(a1 + 208);
  if ( v46 > 0xDE0B6B3A763FFFFLL )
  {
    *(_QWORD *)(a1 + 208) = 0xDE0B6B3A763FFFFLL;
    v46 = 0xDE0B6B3A763FFFFLL;
  }
  v47 = sub_148AC8A40();
  v100 = 4;
  LODWORD(v48) = (int)v47;
  HIDWORD(v48) = (int)v47 >> 31;
  v95 = v48 / v29 - v95;
  v99 = 0;
  v49 = &v95;
  if ( v95 <= 0 )
    v49 = &v99;
  v50 = &v100;
  if ( *v49 <= 4 )
    v50 = v49;
  v51 = *v50;
  v52 = (unsigned int)(int)sub_148ABB2B0();
  v53 = sub_146E8C7D0(&unk_14A3F4658);
  v54 = sub_146E8CF20(v107, v53, v46 / v52);
  v55 = sub_14014F430(v54);
  v56 = -1;
  do
    ++v56;
  while ( *(_WORD *)(v55 + 2 * v56) );
  sub_14014C8D0(a1 + 304, v55);
  sub_146E8C910(v107);
  *(_DWORD *)(a1 + 296) = *(_DWORD *)(a1 + 320);
  if ( v51 >= 1 )
  {
    v57 = *(_QWORD *)(a1 + 208) % v52 / (unsigned int)(int)sub_148ABB2B0();
    v58 = sub_146E8C7D0(&unk_14A3F4670);
    v59 = sub_146E8CF20(v108, v58, v57);
    v60 = sub_14014F430(v59);
    v61 = -1;
    do
      ++v61;
    while ( *(_WORD *)(v60 + 2 * v61) );
    sub_14014C8D0(a1 + 368, v60);
    sub_146E8C910(v108);
  }
  v94 = v20;
  v62 = 0;
  v63 = *(_QWORD **)(v12 + 216);
  for ( i = (_QWORD *)*v63; i != v63; i = (_QWORD *)*i )
    v62 += i[2];
  if ( *(_DWORD *)(v12 + 224) )
  {
    v65 = (float)(int)(v62 / *(int *)(v12 + 224));
    if ( (unsigned int)(int)(float)(v65 * 0.1) <= v20 )
    {
      sub_140E59090(v12 + 216, &v94);
      if ( *(_QWORD *)(v12 + 224) > 0x64u )
      {
        v66 = **(__int64 ***)(v12 + 216);
        v67 = *v66;
        --*(_QWORD *)(v12 + 224);
        *(_QWORD *)v66[1] = v67;
        *(_QWORD *)(v67 + 8) = v66[1];
        sub_146E9F3A0(v66, 24);
      }
      if ( (unsigned int)(int)(float)(v65 * 1.5) < v94 )
        goto LABEL_61;
    }
  }
  else
  {
    sub_140E59090(v12 + 216, &v94);
  }
  v68 = *(_DWORD *)(a1 + 216);
  if ( v68 == 5 || v68 == v51 )
  {
    v69 = 6;
    goto LABEL_63;
  }
LABEL_61:
  *(_DWORD *)(a1 + 248) = 1067869798;
  *(_DWORD *)(a1 + 252) = 1050253722;
  v69 = 15;
LABEL_63:
  *(_DWORD *)(a1 + 168) = v69;
  *(_DWORD *)(a1 + 216) = v51;
  v70 = MEMORY[0xDC5CDC8]();
  *(_DWORD *)(a1 + 224) = v70;
  *(_DWORD *)(a1 + 228) = v70 + 500;
  sub_146EA91A0(&v113);
  v113 = 20;
  v71 = (_DWORD *)(a1 + 232);
  if ( (_QWORD)v115 == *((_QWORD *)&v115 + 1) )
  {
    sub_140154010(&v114, v115, v71);
    v72 = (_DWORD *)v115;
  }
  else
  {
    *(_DWORD *)v115 = *v71;
    v72 = (_DWORD *)(v115 + 4);
    *(_QWORD *)&v115 = v115 + 4;
  }
  v73 = (_DWORD *)(a1 + 236);
  if ( v72 == *((_DWORD **)&v115 + 1) )
  {
    sub_140154010(&v114, v72, v73);
    v74 = (_DWORD *)v115;
  }
  else
  {
    *v72 = *v73;
    v74 = (_DWORD *)(v115 + 4);
    *(_QWORD *)&v115 = v115 + 4;
  }
  v75 = (_DWORD *)(a1 + 240);
  if ( v74 == *((_DWORD **)&v115 + 1) )
  {
    sub_140154010(&v114, v74, v75);
    v76 = (_DWORD *)v115;
  }
  else
  {
    *v74 = *v75;
    v76 = (_DWORD *)(v115 + 4);
    *(_QWORD *)&v115 = v115 + 4;
  }
  v77 = (_DWORD *)(a1 + 244);
  if ( v76 == *((_DWORD **)&v115 + 1) )
  {
    sub_140154010(&v114, v76, v77);
  }
  else
  {
    *v76 = *v77;
    *(_QWORD *)&v115 = v115 + 4;
  }
  v78 = sub_146C7BC10(v111, &v113);
  v79 = sub_14184F710(v109, v78);
  v101 = 0;
  v101 = *(_OWORD *)v79;
  v80 = *((_QWORD *)&v101 + 1);
  v81 = v101;
  *(_QWORD *)v79 = 0;
  *(_QWORD *)(v79 + 8) = 0;
  *(_QWORD *)&v101 = *(_QWORD *)(a1 + 136);
  *(_QWORD *)(a1 + 136) = v81;
  *((_QWORD *)&v101 + 1) = *(_QWORD *)(a1 + 144);
  v82 = (volatile signed __int32 *)*((_QWORD *)&v101 + 1);
  *(_QWORD *)(a1 + 144) = v80;
  if ( v82 )
  {
    if ( _InterlockedExchangeAdd(v82 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v82)(v82);
      if ( _InterlockedExchangeAdd(v82 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v82 + 8LL))(v82);
    }
  }
  v83 = v110;
  if ( v110 )
  {
    if ( _InterlockedExchangeAdd(v110 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v83)(v83);
      if ( _InterlockedExchangeAdd(v83 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v83 + 8LL))(v83);
    }
  }
  v84 = v112;
  if ( v112 )
  {
    if ( _InterlockedExchangeAdd(v112 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v84)(v84);
      if ( _InterlockedExchangeAdd(v84 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v84 + 8LL))(v84);
    }
  }
  LOBYTE(v80) = 1;
  sub_146B62250(*(_QWORD *)(a1 + 152), v80);
  LOBYTE(v85) = 1;
  sub_146B5AA70(*(_QWORD *)(a1 + 152), v85);
  v88 = *(_QWORD *)(a1 + 152);
  if ( !qword_14E63AE60 )
  {
    v89 = sub_146E8BA20(336);
    v94 = v89;
    if ( v89 )
      v90 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v89);
    else
      v90 = 0;
    qword_14E63AE60 = (__int64)v90;
    (**v90)(v90);
  }
  LOBYTE(v87) = 1;
  sub_146B5BD80(v88, v86, v87, 0, 0, 0);
  v91 = a8;
  *(_DWORD *)(a1 + 108) = a8;
  if ( !*(_BYTE *)(a1 + 64) )
  {
    if ( a8 <= -1 )
      v91 = *(_DWORD *)(sub_143C61150() + 232);
    *(_DWORD *)(a1 + 104) = v91;
    *(_BYTE *)(a1 + 112) = 1;
  }
  sub_1447E54A0(a1, 0.0);
  sub_14014C710(v116);
  v92 = v114;
  if ( v114 )
  {
    v93 = (*((_QWORD *)&v115 + 1) - v114) & 0xFFFFFFFFFFFFFFFCuLL;
    if ( v93 >= 0x1000 )
    {
      v93 += 39LL;
      v92 = *(_QWORD *)(v114 - 8);
      if ( (unsigned __int64)(v114 - v92 - 8) > 0x1F )
        sub_148AAF304(v92, v93);
    }
    sub_146E9F3A0(v92, v93);
    v114 = 0;
    v115 = 0;
  }
}

