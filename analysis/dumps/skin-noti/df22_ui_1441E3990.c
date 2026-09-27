// ui_1441E3990

__int64 __fastcall sub_1441E3990(__int64 a1)
{
  __int64 v2; // r12
  __int64 v3; // rcx
  __int64 v4; // rax
  void (__fastcall ***v5)(_QWORD); // rcx
  __int64 v6; // rax
  __int64 v7; // rdx
  _OWORD *v8; // r8
  __int64 *v9; // rbx
  __int64 v10; // rax
  __int64 **v11; // rax
  __int64 *i; // rax
  __int64 *j; // rcx
  __int64 v14; // rdx
  __int64 v15; // r14
  int v16; // ebx
  int v17; // edi
  int v18; // esi
  int v19; // eax
  __int64 v20; // r14
  int v21; // ebx
  int v22; // edi
  int v23; // esi
  int v24; // eax
  __int64 v25; // r14
  int v26; // ebx
  int v27; // edi
  int v28; // esi
  int v29; // eax
  int v30; // r14d
  __int64 v31; // r8
  __int64 v32; // rcx
  __int64 v33; // rdx
  __int64 v34; // rax
  __int64 v35; // rbx
  __int64 v36; // r10
  __int64 v37; // rsi
  unsigned __int64 v38; // rdi
  char *v39; // rax
  __int64 v40; // rcx
  char *v41; // r9
  unsigned __int16 v42; // dx
  signed __int64 v43; // r9
  bool v44; // cc
  unsigned __int16 v45; // dx
  __int64 v46; // rcx
  __int64 v47; // rbx
  __int64 v48; // rdi
  unsigned __int64 v49; // rdx
  __int64 v50; // rax
  __int64 v51; // rcx
  __int64 *v52; // rbx
  __int64 *v53; // rcx
  __int128 v55; // [rsp+60h] [rbp-49h] BYREF
  __int64 v56; // [rsp+70h] [rbp-39h]
  __int128 v57; // [rsp+78h] [rbp-31h] BYREF
  __int64 v58; // [rsp+88h] [rbp-21h]
  _QWORD v59[14]; // [rsp+90h] [rbp-19h] BYREF
  __int64 v60; // [rsp+110h] [rbp+67h] BYREF
  __int128 *v61; // [rsp+118h] [rbp+6Fh]
  __int64 *v62; // [rsp+120h] [rbp+77h]
  __int128 *v63; // [rsp+128h] [rbp+7Fh]

  v59[2] = -2;
  *(_QWORD *)(a1 + 24) = *(_QWORD *)(a1 + 16);
  v2 = 0;
  v3 = qword_14E638F28;
  if ( !qword_14E638F28 )
  {
    v4 = sub_146E8BA20(1472);
    v60 = v4;
    if ( v4 )
      v5 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v4);
    else
      v5 = 0;
    qword_14E638F28 = (__int64)v5;
    (**v5)(v5);
    v3 = qword_14E638F28;
  }
  v6 = sub_1444EBDF0(v3, 3);
  sub_1441B8B20(v59, v6);
  v9 = *(__int64 **)v59[0];
  while ( v9 != (__int64 *)v59[0] )
  {
    v10 = sub_1444EBAB0(*((_DWORD *)v9 + 7), v7, (__int64)v8);
    if ( v10 && *(_DWORD *)(v10 + 8) == 3 )
    {
      v8 = v9 + 4;
      v7 = *(_QWORD *)(a1 + 24);
      if ( v7 == *(_QWORD *)(a1 + 32) )
      {
        sub_1405EA310(a1 + 16, v7, v8);
      }
      else
      {
        *(_OWORD *)v7 = *v8;
        *(_QWORD *)(v7 + 16) = v9[6];
        *(_QWORD *)(a1 + 24) += 24LL;
      }
    }
    v11 = (__int64 **)v9[2];
    if ( *((_BYTE *)v11 + 25) )
    {
      for ( i = (__int64 *)v9[1]; !*((_BYTE *)i + 25); i = (__int64 *)i[1] )
      {
        if ( v9 != (__int64 *)i[2] )
          break;
        v9 = i;
      }
      v9 = i;
    }
    else
    {
      v9 = (__int64 *)v9[2];
      for ( j = *v11; !*((_BYTE *)j + 25); j = (__int64 *)*j )
        v9 = j;
    }
  }
  sub_141FD8CD0(*(_QWORD *)(a1 + 16), *(_QWORD *)(a1 + 24), (*(_QWORD *)(a1 + 24) - *(_QWORD *)(a1 + 16)) / 24LL, 3);
  LOBYTE(v14) = *(_QWORD *)(a1 + 16) == *(_QWORD *)(a1 + 24);
  (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 2072) + 16LL))(*(_QWORD *)(a1 + 2072), v14);
  sub_146F1E010(*(_QWORD *)(a1 + 40));
  v15 = *(_QWORD *)(a1 + 40);
  v61 = &v55;
  v55 = 0;
  v56 = 0;
  v62 = &v60;
  v60 = 0;
  v16 = dword_14F1C0920;
  v17 = dword_14F1C091C;
  v18 = dword_14F1C0918;
  v19 = sub_14723C170(100003171);
  sub_146F1AD60(v15, v19, 0, 0, v18, v17, v17, v16, (__int64)&v60, 1, 1, (__int64)&v55);
  v20 = *(_QWORD *)(a1 + 40);
  v63 = &v55;
  v55 = 0;
  v56 = 0;
  v59[3] = &v60;
  v60 = 0;
  v21 = dword_14F1C0920;
  v22 = dword_14F1C091C;
  v23 = dword_14F1C0918;
  v24 = sub_14723C170(101036737);
  sub_146F1AD60(v20, v24, 1, 0, v23, v22, v22, v21, (__int64)&v60, 1, 1, (__int64)&v55);
  v25 = *(_QWORD *)(a1 + 40);
  v59[4] = &v55;
  v55 = 0;
  v56 = 0;
  v59[5] = &v60;
  v60 = 0;
  v26 = dword_14F1C0920;
  v27 = dword_14F1C091C;
  v28 = dword_14F1C0918;
  v29 = sub_14723C170(100003172);
  sub_146F1AD60(v25, v29, 2, 0, v28, v27, v27, v26, (__int64)&v60, 1, 1, (__int64)&v55);
  v57 = 0;
  v58 = 0;
  v30 = 0;
  v31 = *(_QWORD *)(a1 + 16);
  v32 = *(_QWORD *)(a1 + 24) - v31;
  v33 = v32 / 24;
  if ( !(v32 / 24) )
    goto LABEL_46;
  do
  {
    v34 = sub_1444EBAB0(*(_DWORD *)(v31 + v2), v33, v31);
    if ( !v34 || *(_DWORD *)(v34 + 8) != 3 )
      goto LABEL_45;
    v35 = v34 + 256;
    v36 = v57;
    if ( (_QWORD)v57 == *((_QWORD *)&v57 + 1) )
      goto LABEL_39;
    v37 = *(_QWORD *)(v34 + 272);
    v38 = *(_QWORD *)(v34 + 280);
    while ( 1 )
    {
      v39 = (char *)v35;
      if ( v38 >= 8 )
        v39 = *(char **)v35;
      v40 = *(_QWORD *)(v36 + 16);
      v41 = (char *)v36;
      if ( *(_QWORD *)(v36 + 24) >= 8u )
        v41 = *(char **)v36;
      if ( v40 == v37 )
        break;
LABEL_36:
      v36 += 32;
      if ( v36 == *((_QWORD *)&v57 + 1) )
        goto LABEL_39;
    }
    if ( v40 )
    {
      v42 = *(_WORD *)v41;
      if ( *(_WORD *)v41 >= *(_WORD *)v39 )
      {
        v43 = v41 - v39;
        v44 = v42 <= *(_WORD *)v39;
        do
        {
          if ( !v44 )
            break;
          if ( v40 == 1 )
            goto LABEL_38;
          --v40;
          v39 += 2;
          v45 = *(_WORD *)&v39[v43];
          v44 = v45 <= *(_WORD *)v39;
        }
        while ( v45 >= *(_WORD *)v39 );
      }
      goto LABEL_36;
    }
LABEL_38:
    if ( v36 == *((_QWORD *)&v57 + 1) )
    {
LABEL_39:
      if ( *((_QWORD *)&v57 + 1) == v58 )
      {
        sub_140177EC0(&v57, *((_QWORD *)&v57 + 1), v35);
      }
      else
      {
        sub_14014C810(*((_QWORD *)&v57 + 1));
        *((_QWORD *)&v57 + 1) += 32LL;
      }
      v46 = *(_QWORD *)(a1 + 40);
      v61 = &v55;
      v55 = 0;
      v56 = 0;
      v62 = &v60;
      v60 = 0;
      if ( *(_QWORD *)(v35 + 24) >= 8u )
        v35 = *(_QWORD *)v35;
      sub_146F1AD60(
        v46,
        v35,
        v30 + 3,
        0,
        dword_14F1C0918,
        dword_14F1C091C,
        dword_14F1C091C,
        dword_14F1C0920,
        (__int64)&v60,
        1,
        1,
        (__int64)&v55);
    }
LABEL_45:
    ++v30;
    v2 += 24;
    v31 = *(_QWORD *)(a1 + 16);
    v32 = *(_QWORD *)(a1 + 24) - v31;
    v33 = v32 / 24;
  }
  while ( v30 < (unsigned __int64)(v32 / 24) );
LABEL_46:
  v47 = v57;
  if ( (_QWORD)v57 )
  {
    v48 = *((_QWORD *)&v57 + 1);
    if ( (_QWORD)v57 != *((_QWORD *)&v57 + 1) )
    {
      do
      {
        sub_14014C710(v47);
        v47 += 32;
      }
      while ( v47 != v48 );
      v47 = v57;
    }
    v49 = (v58 - v47) & 0xFFFFFFFFFFFFFFE0uLL;
    v50 = v47;
    if ( v49 >= 0x1000 )
    {
      v49 += 39LL;
      v47 = *(_QWORD *)(v47 - 8);
      if ( (unsigned __int64)(v50 - v47 - 8) > 0x1F )
        sub_148AAF304(v32, v49);
    }
    sub_146E9F3A0(v47, v49);
    v57 = 0;
    v58 = 0;
  }
  v51 = v59[0];
  v52 = *(__int64 **)(v59[0] + 8LL);
  if ( !*((_BYTE *)v52 + 25) )
  {
    do
    {
      sub_1401DBB80(v59, v59, v52[2]);
      v53 = v52;
      v52 = (__int64 *)*v52;
      sub_146E9F3A0(v53, 56);
    }
    while ( !*((_BYTE *)v52 + 25) );
    v51 = v59[0];
  }
  return sub_146E9F3A0(v51, 56);
}

