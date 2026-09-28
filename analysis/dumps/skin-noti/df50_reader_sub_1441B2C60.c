// reader_sub_1441B2C60

__int64 __fastcall sub_1441B2C60(__int64 a1)
{
  __int64 v2; // r13
  int v3; // r14d
  __int64 v4; // rdx
  __int64 v5; // rbx
  __int64 *v6; // rax
  __int64 v7; // rbx
  __int64 v8; // rdi
  __int64 v9; // rdx
  __int64 v10; // rbx
  int v11; // ecx
  __int64 v12; // rbx
  int v13; // esi
  int v14; // eax
  __int64 v15; // rdx
  __int64 v16; // rdi
  void (__fastcall *v17)(__int64); // rbx
  __int64 v18; // rsi
  unsigned int v19; // eax
  __int64 v20; // rax
  __int64 v21; // rbx
  __int64 v22; // rax
  __int64 v23; // rcx
  int v24; // esi
  int v25; // ebx
  unsigned int v26; // esi
  char v27; // bl
  _QWORD *v28; // rax
  _QWORD *v29; // rax
  _QWORD *v30; // rax
  bool v31; // r14
  volatile signed __int32 *v32; // rdi
  volatile signed __int32 *v33; // rdi
  volatile signed __int32 *v34; // rdi
  volatile signed __int32 *v35; // rdi
  volatile signed __int32 *v36; // rbx
  __int64 v37; // rcx
  unsigned int v38; // eax
  __int64 v39; // rbx
  _QWORD *v40; // rdi
  int i; // ebx
  unsigned int v42; // eax
  __int64 v43; // r8
  __int64 v44; // rbx
  __int64 v45; // rax
  __int64 v46; // r8
  unsigned int v47; // eax
  __int64 *v48; // rbx
  unsigned int *v49; // r14
  unsigned int v50; // esi
  __int64 v51; // rax
  __int64 v52; // r12
  __int64 v53; // r8
  unsigned __int64 v54; // rdx
  __int64 v55; // rdi
  __int64 v56; // rsi
  __int64 v57; // rcx
  unsigned __int64 v58; // rdx
  __int64 v59; // rax
  __int64 v60; // rcx
  unsigned __int64 v61; // rdx
  unsigned int *v62; // rax
  __int64 v63; // r9
  __int64 v64; // rdx
  unsigned __int64 v65; // rdi
  __int64 *v66; // rcx
  __int64 v67; // rax
  __int64 **v68; // rax
  __int64 *j; // rax
  __int64 *k; // rcx
  int v71; // r14d
  __int64 v72; // rbx
  int v73; // edi
  int v74; // esi
  int v75; // edx
  int v76; // r8d
  __int64 result; // rax
  int v78; // r14d
  __int64 v79; // rbx
  int v80; // edi
  int v81; // esi
  int v82; // edx
  int v83; // r8d
  __int64 v84; // rcx
  __int64 v85; // rbx
  unsigned __int64 v86; // rdx
  __int128 v87; // [rsp+70h] [rbp-98h]
  __int128 v88; // [rsp+80h] [rbp-88h] BYREF
  __int64 v89; // [rsp+90h] [rbp-78h]
  unsigned int v90; // [rsp+98h] [rbp-70h]
  unsigned __int64 v91; // [rsp+A0h] [rbp-68h]
  __int128 *v92; // [rsp+A8h] [rbp-60h]
  __int128 v93; // [rsp+B0h] [rbp-58h]
  __int128 v94; // [rsp+C0h] [rbp-48h]
  __int64 v95; // [rsp+D0h] [rbp-38h]
  __int128 v96; // [rsp+D8h] [rbp-30h] BYREF
  _QWORD v97[5]; // [rsp+E8h] [rbp-20h] BYREF
  _BYTE v98[8]; // [rsp+110h] [rbp+8h] BYREF
  volatile signed __int32 *v99; // [rsp+118h] [rbp+10h]
  _BYTE v100[8]; // [rsp+120h] [rbp+18h] BYREF
  volatile signed __int32 *v101; // [rsp+128h] [rbp+20h]
  _BYTE v102[8]; // [rsp+130h] [rbp+28h] BYREF
  volatile signed __int32 *v103; // [rsp+138h] [rbp+30h]
  _BYTE v104[8]; // [rsp+140h] [rbp+38h] BYREF
  volatile signed __int32 *v105; // [rsp+148h] [rbp+40h]
  _BYTE v106[8]; // [rsp+150h] [rbp+48h] BYREF
  volatile signed __int32 *v107; // [rsp+158h] [rbp+50h]
  _BYTE v108[56]; // [rsp+168h] [rbp+60h] BYREF
  __int64 (__fastcall **v109)(); // [rsp+1A0h] [rbp+98h]
  __int64 v110; // [rsp+1D8h] [rbp+D0h]
  __int64 v111; // [rsp+1E0h] [rbp+D8h]
  __int64 v112; // [rsp+1E8h] [rbp+E0h]
  __int64 v113; // [rsp+1F0h] [rbp+E8h]
  __int128 v114; // [rsp+1F8h] [rbp+F0h]
  __int64 v115; // [rsp+288h] [rbp+180h]
  __int64 v116; // [rsp+290h] [rbp+188h]
  __int64 v117; // [rsp+298h] [rbp+190h]

  v97[4] = -2;
  v2 = 0;
  sub_145F70B70(a1);
  if ( *(_QWORD *)(a1 + 1560) )
    sub_146A0AD60();
  v3 = 134;
  if ( (*(unsigned __int8 (__fastcall **)(__int64, _QWORD, _QWORD))(*(_QWORD *)qword_14F1C0F28 + 48LL))(
         qword_14F1C0F28,
         0,
         0) )
  {
    v3 = 0;
    --*(_DWORD *)(a1 + 1576);
    goto LABEL_18;
  }
  if ( (*(unsigned __int8 (__fastcall **)(__int64, __int64))(*(_QWORD *)qword_14F1C0F28 + 48LL))(qword_14F1C0F28, 1) )
  {
    v3 = 1;
    ++*(_DWORD *)(a1 + 1576);
    goto LABEL_18;
  }
  if ( (*(unsigned __int8 (__fastcall **)(__int64, __int64))(*(_QWORD *)qword_14F1C0F28 + 48LL))(qword_14F1C0F28, 2) )
  {
    v3 = 2;
    *(_DWORD *)(a1 + 1576) -= 2;
    goto LABEL_18;
  }
  if ( (*(unsigned __int8 (__fastcall **)(__int64, __int64))(*(_QWORD *)qword_14F1C0F28 + 48LL))(qword_14F1C0F28, 3) )
  {
    v3 = 3;
    *(_DWORD *)(a1 + 1576) += 2;
    goto LABEL_18;
  }
  if ( (*(unsigned __int8 (__fastcall **)(__int64, __int64))(*(_QWORD *)qword_14F1C0F28 + 48LL))(qword_14F1C0F28, 19)
    || (*(unsigned __int8 (__fastcall **)(__int64, __int64))(*(_QWORD *)qword_14F1C0F28 + 48LL))(qword_14F1C0F28, 81) )
  {
    v4 = 0;
    goto LABEL_17;
  }
  if ( (*(unsigned __int8 (__fastcall **)(__int64, __int64))(*(_QWORD *)qword_14F1C0F28 + 48LL))(qword_14F1C0F28, 20)
    || (*(unsigned __int8 (__fastcall **)(__int64, __int64))(*(_QWORD *)qword_14F1C0F28 + 48LL))(qword_14F1C0F28, 82) )
  {
    LOBYTE(v4) = 1;
LABEL_17:
    sub_1441B44E0(a1, v4);
  }
LABEL_18:
  v88 = 0;
  v89 = 0;
  v5 = 0;
  if ( (unsigned __int8)sub_146ECFF20(*(_QWORD *)(a1 + 3048)) == 1 )
  {
    v6 = (__int64 *)(a1 + 1536);
  }
  else
  {
    if ( (unsigned __int8)sub_146ECFF20(*(_QWORD *)(a1 + 3064)) != 1 )
      goto LABEL_26;
    v6 = (__int64 *)(a1 + 1512);
    v5 = 0;
  }
  if ( &v88 != (__int128 *)v6 )
  {
    v7 = *v6;
    v8 = v6[1] - *v6;
    if ( v8 >> 2 )
      sub_1401C4C50(&v88);
    v9 = v7;
    v10 = v88;
    sub_148AA1E60(v88, v9, v8);
    v5 = v8 + v10;
    *((_QWORD *)&v88 + 1) = v5;
  }
LABEL_26:
  v11 = 10 * *(_DWORD *)(a1 + 1568) - 10;
  v12 = (v5 - (__int64)v88) >> 2;
  v13 = 2 * v11 + 9;
  if ( (int)v12 - v11 <= 10 )
    v13 = v12 - 1;
  v14 = *(_DWORD *)(a1 + 1576);
  if ( v13 >= v14 )
  {
    v13 = *(_DWORD *)(a1 + 1576);
    if ( v14 < v11 )
      v13 = 10 * *(_DWORD *)(a1 + 1568) - 10;
  }
  *(_DWORD *)(a1 + 1576) = v13;
  if ( v3 != 134 || v13 != *(_DWORD *)(a1 + 1572) )
  {
    *(_DWORD *)(a1 + 1572) = v13;
    v15 = (unsigned int)(v13 / 10);
    LOBYTE(v15) = 1;
    (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 2680) + 16LL))(*(_QWORD *)(a1 + 2680), v15);
    v16 = *(_QWORD *)(a1 + 2680);
    v17 = *(void (__fastcall **)(__int64))(*(_QWORD *)v16 + 112LL);
    v18 = 2LL * (v13 % 10);
    sub_142757420(*(_QWORD *)(a1 + 8 * v18 + 1592));
    sub_146ECA0C0(*(_QWORD *)(a1 + 8 * v18 + 1592));
    v17(v16);
    *(_BYTE *)(a1 + 1584) = 1;
    v19 = sub_1421B2820(*(_QWORD *)(a1 + 8 * v18 + 1752));
    sub_1441B4540(a1, v19);
  }
  v87 = 0;
  if ( (unsigned __int8)sub_146ED0010(*(_QWORD *)(a1 + 3096)) == 1 )
  {
    v93 = 0;
    v20 = *(_QWORD *)(a1 + 3104);
    if ( v20 )
    {
      _InterlockedIncrement((volatile signed __int32 *)(v20 + 8));
      v20 = *(_QWORD *)(a1 + 3104);
    }
    v21 = *(_QWORD *)(a1 + 3096);
    v93 = 0u;
    *(_QWORD *)&v87 = v21;
    *((_QWORD *)&v87 + 1) = v20;
  }
  else
  {
    v21 = 0;
    if ( (unsigned __int8)sub_146ED0010(*(_QWORD *)(a1 + 3112)) == 1 )
    {
      v94 = 0;
      v22 = *(_QWORD *)(a1 + 3120);
      if ( v22 )
      {
        _InterlockedIncrement((volatile signed __int32 *)(v22 + 8));
        v22 = *(_QWORD *)(a1 + 3120);
      }
      v21 = *(_QWORD *)(a1 + 3112);
      v94 = 0u;
      *(_QWORD *)&v87 = v21;
      *((_QWORD *)&v87 + 1) = v22;
    }
  }
  v23 = *(_QWORD *)(a1 + 848);
  if ( v21 )
  {
    sub_1466963D0(v23, 120);
    v24 = sub_146F1E6E0(v87);
    v25 = sub_14206BB60(v87);
    if ( (unsigned __int8)sub_146EC49A0() == 1 )
    {
      --v25;
    }
    else if ( (unsigned __int8)sub_146EC4990() == 1 )
    {
      ++v25;
    }
    v26 = v24 - 1;
    if ( v25 >= 0 )
    {
      if ( v25 > (int)v26 )
        v25 = 0;
      v26 = v25;
    }
    v27 = 1;
    v31 = 1;
    if ( *(_QWORD *)sub_146F1E500(v87, v106) )
    {
      v28 = (_QWORD *)sub_146F1E500(v87, v104);
      v27 = 7;
      if ( *(_QWORD *)sub_146F644B0(*v28, v102) )
      {
        v29 = (_QWORD *)sub_146F1E500(v87, v100);
        v30 = (_QWORD *)sub_146F644B0(*v29, v98);
        v27 = 31;
        if ( (unsigned __int8)sub_141FB6530(*v30) )
          v31 = 0;
      }
    }
    if ( (v27 & 0x10) != 0 )
    {
      v27 &= ~0x10u;
      v32 = v99;
      if ( v99 )
      {
        if ( _InterlockedExchangeAdd(v99 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v32)(v32);
          if ( _InterlockedExchangeAdd(v32 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v32 + 8LL))(v32);
        }
      }
    }
    if ( (v27 & 8) != 0 )
    {
      v27 &= ~8u;
      v33 = v101;
      if ( v101 )
      {
        if ( _InterlockedExchangeAdd(v101 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v33)(v33);
          if ( _InterlockedExchangeAdd(v33 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v33 + 8LL))(v33);
        }
      }
    }
    if ( (v27 & 4) != 0 )
    {
      v27 &= ~4u;
      v34 = v103;
      if ( v103 )
      {
        if ( _InterlockedExchangeAdd(v103 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v34)(v34);
          if ( _InterlockedExchangeAdd(v34 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v34 + 8LL))(v34);
        }
      }
    }
    if ( (v27 & 2) != 0 )
    {
      v27 &= ~2u;
      v35 = v105;
      if ( v105 )
      {
        if ( _InterlockedExchangeAdd(v105 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v35)(v35);
          if ( _InterlockedExchangeAdd(v35 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v35 + 8LL))(v35);
        }
      }
    }
    if ( (v27 & 1) != 0 )
    {
      v36 = v107;
      if ( v107 )
      {
        if ( _InterlockedExchangeAdd(v107 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v36)(v36);
          if ( _InterlockedExchangeAdd(v36 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v36 + 8LL))(v36);
        }
      }
    }
    if ( v31 )
    {
      sub_142268950(v87, v26);
      v92 = &v96;
      v96 = 0;
      sub_146F1F2F0(v87, v26, &v96);
    }
  }
  else
  {
    sub_1466963D0(v23, 0);
  }
  if ( (unsigned __int8)sub_146F1E8C0(*(_QWORD *)(a1 + 3096)) )
  {
    v37 = *(_QWORD *)(a1 + 16LL * (*(_DWORD *)(a1 + 1576) % 10) + 1752);
    if ( v37 )
    {
      v38 = sub_1421B2820(v37);
      sub_1441B3C30(a1, v38);
    }
  }
  else if ( !(unsigned __int8)sub_146F1E8C0(*(_QWORD *)(a1 + 3112)) && *(_BYTE *)(a1 + 1584) != 1 )
  {
    goto LABEL_135;
  }
  v39 = 0;
  v40 = (_QWORD *)(a1 + 1912);
  do
  {
    if ( (unsigned __int64)v39 <= 0xE )
      (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)*v40 + 16LL))(*v40, 0);
    ++v39;
    v40 += 2;
  }
  while ( v39 < 15 );
  for ( i = 0; i <= 10; ++i )
    sub_146A0BDF0(*(_QWORD *)(a1 + 1560), (unsigned int)i);
  sub_146A0BBF0(*(_QWORD *)(a1 + 1560));
  sub_146A0E970(*(_QWORD *)(a1 + 1560), 0);
  *(_BYTE *)(a1 + 1584) = 0;
  v42 = sub_1421B2820(*(_QWORD *)(a1 + 2984));
  LOBYTE(v43) = 1;
  v44 = sub_140283D60(qword_14E683B38, v42, v43);
  if ( v44 )
  {
    v92 = (__int128 *)v97;
    v45 = sub_146F1E840(*(_QWORD *)(a1 + 3096));
    v97[0] = 0;
    v97[2] = 0;
    v97[3] = 7;
    v46 = -1;
    do
      ++v46;
    while ( *(_WORD *)(v45 + 2 * v46) );
    sub_14014C8D0(v97, v45);
    v47 = sub_1459D7C80(v97);
    sub_146A0C7A0(*(_QWORD *)(a1 + 1560), v47, 0);
    v48 = **(__int64 ***)(v44 + 2672);
    while ( !*((_BYTE *)v48 + 25) )
    {
      if ( *((char *)v48 + 32) == (unsigned int)sub_14206BB60(*(_QWORD *)(a1 + 3096))
        && *((char *)v48 + 33) == (unsigned int)sub_14206BB60(*(_QWORD *)(a1 + 3112)) )
      {
        v49 = (unsigned int *)v48[5];
        v92 = (__int128 *)v48[6];
        if ( v49 != (unsigned int *)v92 )
        {
          v91 = 0;
          do
          {
            v90 = *v49;
            v50 = v90;
            v51 = sub_14576E6A0(v108);
            v52 = sub_14564D7B0(v50, v51);
            v53 = v115;
            if ( v115 )
            {
              v54 = 24 * ((v117 - v115) / 24);
              if ( v54 >= 0x1000 )
              {
                v54 += 39LL;
                v53 = *(_QWORD *)(v115 - 8);
                if ( (unsigned __int64)(v115 - v53 - 8) > 0x1F )
                  sub_148AAF304(v117 - v115, v54);
              }
              sub_146E9F3A0(v53, v54);
              v115 = 0;
              v116 = 0;
              v117 = 0;
            }
            v55 = v113;
            if ( v113 )
            {
              v56 = v114;
              v95 = v113;
              if ( v113 != (_QWORD)v114 )
              {
                do
                {
                  sub_14014C710(v55 + 32);
                  v55 += 72;
                  v95 = v55;
                }
                while ( v55 != v56 );
                v55 = v113;
              }
              v57 = *((_QWORD *)&v114 + 1) - v55;
              v58 = 72 * ((*((_QWORD *)&v114 + 1) - v55) / 72);
              v59 = v55;
              if ( v58 >= 0x1000 )
              {
                v58 += 39LL;
                v55 = *(_QWORD *)(v55 - 8);
                if ( (unsigned __int64)(v59 - v55 - 8) > 0x1F )
                  sub_148AAF304(v57, v58);
              }
              sub_146E9F3A0(v55, v58);
              v55 = 0;
              v113 = 0;
              v114 = 0;
              v50 = v90;
            }
            v60 = v110;
            if ( v110 )
            {
              v61 = 8 * ((v112 - v110) >> 3);
              if ( v61 >= 0x1000 )
              {
                v61 += 39LL;
                v60 = *(_QWORD *)(v110 - 8);
                if ( (unsigned __int64)(v110 - v60 - 8) > 0x1F )
                  sub_148AAF304(v60, v61);
              }
              sub_146E9F3A0(v60, v61);
              v110 = v55;
              v111 = v55;
              v112 = v55;
            }
            v109 = &off_149271DB0;
            if ( v52 )
            {
              v62 = (unsigned int *)(*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v52 + 1040LL))(v52);
              LOBYTE(v63) = 1;
              sub_146A0DBD0(*(_QWORD *)(a1 + 1560), *v62, v52, v63);
              v65 = v91;
              if ( v91 <= 0xE )
              {
                v66 = *(__int64 **)(a1 + v2 + 1912);
                v67 = *v66;
                if ( v50 == -1 )
                {
                  (*(void (__fastcall **)(__int64 *, _QWORD))(v67 + 16))(v66, 0);
                }
                else
                {
                  LOBYTE(v64) = 1;
                  (*(void (__fastcall **)(__int64 *, __int64))(v67 + 16))(v66, v64);
                  sub_14501C1F0(*(_QWORD *)(a1 + v2 + 1912), v50);
                }
              }
            }
            else
            {
              v65 = v91;
            }
            v91 = v65 + 1;
            v2 += 16;
            v49 += 36;
          }
          while ( v49 != (unsigned int *)v92 );
          v2 = 0;
        }
      }
      v68 = (__int64 **)v48[2];
      if ( *((_BYTE *)v68 + 25) )
      {
        for ( j = (__int64 *)v48[1]; !*((_BYTE *)j + 25); j = (__int64 *)j[1] )
        {
          if ( v48 != (__int64 *)j[2] )
            break;
          v48 = j;
        }
        v48 = j;
      }
      else
      {
        v48 = (__int64 *)v48[2];
        for ( k = *v68; !*((_BYTE *)k + 25); k = (__int64 *)*k )
          v48 = k;
      }
    }
  }
  sub_146A0BDF0(*(_QWORD *)(a1 + 1560), 10);
  sub_146A0BDF0(*(_QWORD *)(a1 + 1560), 12);
LABEL_135:
  if ( (unsigned __int8)sub_146ED0010(*(_QWORD *)(a1 + 3000)) == 1 )
  {
    v71 = sub_1429BDDE0(qword_14E683C78);
    v72 = sub_14723C170(46080);
    v73 = dword_14F1C0788;
    v74 = dword_14F1C0880;
    sub_142757420(*(_QWORD *)(a1 + 3000));
    sub_146ECA0C0(*(_QWORD *)(a1 + 3000));
    sub_145561F90(v71, v75, v76, v74, v73, 1, v72, 0, 0, 1153957888, 0, 0);
  }
  result = sub_146ED0010(*(_QWORD *)(a1 + 3016));
  if ( (_BYTE)result == 1 )
  {
    v78 = sub_1429BDDE0(qword_14E683C78);
    v79 = sub_14723C170(46081);
    v80 = dword_14F1C0788;
    v81 = dword_14F1C0880;
    sub_142757420(*(_QWORD *)(a1 + 3016));
    sub_146ECA0C0(*(_QWORD *)(a1 + 3016));
    result = sub_145561F90(v78, v82, v83, v81, v80, 1, v79, 0, 0, 1153957888, 0, 0);
  }
  v84 = *((_QWORD *)&v87 + 1);
  if ( *((_QWORD *)&v87 + 1) )
  {
    result = (unsigned int)_InterlockedExchangeAdd((volatile signed __int32 *)(*((_QWORD *)&v87 + 1) + 8LL), 0xFFFFFFFF);
    if ( (_DWORD)result == 1 )
    {
      result = (***((__int64 (__fastcall ****)(_QWORD))&v87 + 1))(*((_QWORD *)&v87 + 1));
      if ( _InterlockedExchangeAdd((volatile signed __int32 *)(*((_QWORD *)&v87 + 1) + 12LL), 0xFFFFFFFF) == 1 )
        result = (*(__int64 (__fastcall **)(_QWORD))(**((_QWORD **)&v87 + 1) + 8LL))(*((_QWORD *)&v87 + 1));
    }
  }
  v85 = v88;
  if ( (_QWORD)v88 )
  {
    v86 = (v89 - v88) & 0xFFFFFFFFFFFFFFFCuLL;
    if ( v86 >= 0x1000 )
    {
      v86 += 39LL;
      v85 = *(_QWORD *)(v88 - 8);
      if ( (unsigned __int64)(v88 - v85 - 8) > 0x1F )
        sub_148AAF304(v84, v86);
    }
    result = sub_146E9F3A0(v85, v86);
    v88 = 0;
    v89 = 0;
  }
  return result;
}

