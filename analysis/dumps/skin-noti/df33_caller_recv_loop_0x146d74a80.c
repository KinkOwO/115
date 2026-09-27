// caller_recv_loop_0x146d74a80

// sub_146D74A80
char __fastcall sub_146D74A80(__int64 a1)
{
  __int64 v1; // r13
  _QWORD *v2; // rax
  __int64 v3; // rcx
  __int64 v4; // rcx
  unsigned __int64 v5; // rbx
  int v6; // esi
  __int64 v7; // rax
  void (__fastcall ***v8)(_QWORD); // rcx
  __int64 v9; // rbx
  int v10; // edi
  int v11; // eax
  __int64 v12; // rcx
  int v13; // esi
  __int64 v14; // rax
  void (__fastcall ***v15)(_QWORD); // rcx
  __int64 v16; // rbx
  int v17; // edi
  int v18; // eax
  int v19; // r14d
  __int64 v20; // rax
  void (__fastcall ***v21)(_QWORD); // rcx
  __int64 v22; // rbx
  int v23; // edi
  int v24; // eax
  unsigned __int64 v25; // r15
  __int64 *v26; // r12
  int v27; // r14d
  __int64 v28; // rax
  void (__fastcall ***v29)(_QWORD); // rcx
  __int64 v30; // rcx
  __int64 v31; // rcx
  unsigned __int16 v32; // ax
  __int64 v33; // rbx
  int v34; // edi
  int v35; // eax
  int v36; // r14d
  __int64 v37; // rax
  void (__fastcall ***v38)(_QWORD); // rcx
  __int64 v39; // rbx
  int v40; // edi
  int v41; // eax
  __int64 v42; // rbx
  __int64 v43; // rax
  _QWORD *v44; // rbx
  unsigned __int64 v45; // rcx
  _QWORD *v46; // r14
  __int64 v47; // rdx
  __int64 v48; // rcx
  __int64 v49; // rdi
  unsigned __int64 v50; // rdi
  __int64 v51; // rsi
  __int64 v52; // rax
  bool v53; // zf
  __int64 v54; // r8
  __int64 v55; // r9
  __int64 v56; // rdi
  __int64 v57; // rax
  __int64 v58; // rax
  __int64 v59; // rax
  _QWORD *v60; // rbx
  char v61; // di
  __int64 v62; // r15
  __int64 v63; // rdx
  __int64 v64; // rcx
  __int64 v65; // r8
  __int64 v66; // r9
  int v67; // r14d
  __int64 v68; // rbx
  int v69; // edi
  int v70; // eax
  __int64 v71; // rcx
  __int64 v72; // rax
  __int64 v73; // rcx
  unsigned __int64 v74; // rax
  __int64 v75; // rbx
  __int64 v76; // rcx
  __int64 v77; // rdx
  __int64 v78; // rax
  __int64 v79; // rcx
  int v81; // [rsp+20h] [rbp-59h]
  __int128 v82; // [rsp+68h] [rbp-11h] BYREF
  __int64 v83; // [rsp+78h] [rbp-1h]
  unsigned __int64 v84; // [rsp+80h] [rbp+7h]
  __int64 v85; // [rsp+88h] [rbp+Fh]
  __int64 v86; // [rsp+E0h] [rbp+67h] BYREF
  char v87; // [rsp+E8h] [rbp+6Fh] BYREF
  __int64 v88; // [rsp+F0h] [rbp+77h]
  __int64 v89; // [rsp+F8h] [rbp+7Fh]

  v86 = a1;
  v1 = a1;
  if ( !*(_BYTE *)(a1 + 1097) || byte_14F2EDE50 )
    return 1;
  v82 = 0;
  v83 = 0;
  v84 = 0;
  v85 = 0;
  v2 = (_QWORD *)sub_146E8BA20(16);
  v2[1] = 0;
  *(_QWORD *)&v82 = v2;
  *v2 = &v82;
  v3 = *(_QWORD *)(v1 + 1280);
  if ( v3 )
    sub_146D74090(v3, &v82);
  v4 = *(_QWORD *)(v1 + 1288);
  if ( v4 )
    sub_146D74090(v4, &v82);
  v5 = v84;
  while ( 1 )
  {
    if ( (unsigned __int8)sub_146D765F0(v1) )
    {
      if ( !*(_BYTE *)(v1 + 1098) )
        goto LABEL_69;
      v36 = qword_14E6343D0;
      if ( !qword_14E6343D0 )
      {
        v37 = sub_146E8BA20(72);
        if ( v37 )
          v38 = (void (__fastcall ***)(_QWORD))sub_146E93360(v37);
        else
          v38 = 0;
        qword_14E6343D0 = (__int64)v38;
        (**v38)(v38);
        v36 = qword_14E6343D0;
      }
      v39 = sub_146E8C7D0(&unk_14B16F720);
      v40 = sub_146E8C7D0(&unk_14B16F600);
      v41 = sub_146E8C7D0(&unk_14B16EBA0);
      sub_146E938E0(v36, 4, v41, v40, 892, (__int64)&qword_14EF5B6C8, v39);
      if ( v85 )
      {
        v42 = *(_QWORD *)(v1 + 1280);
        if ( v42 )
        {
          sub_1459CB240(&v87, *(_QWORD *)(v42 + 48));
          v43 = v85;
          if ( v85 )
          {
            v44 = (_QWORD *)(v42 + 144);
            v45 = v84;
            do
            {
              v46 = (_QWORD *)(*(_QWORD *)(*((_QWORD *)&v82 + 1) + 8 * ((v83 - 1) & ((v45 + v43 - 1) >> 1)))
                             + 8LL * (((_BYTE)v45 + (_BYTE)v43 - 1) & 1));
              v47 = v44[3];
              if ( (v47 & 1) == 0 && v44[2] <= (unsigned __int64)(v44[4] + 2LL) >> 1 )
              {
                sub_1401F5940(v44, 1);
                v47 = v44[3];
              }
              v48 = v44[2];
              v49 = v47 & (2 * v48 - 1);
              v44[3] = v49;
              if ( !v49 )
                v49 = 2 * v48;
              v50 = v49 - 1;
              v51 = 8 * ((v50 >> 1) & (v48 - 1));
              v52 = v44[1];
              if ( !*(_QWORD *)(v52 + v51) )
              {
                *(_QWORD *)(v51 + v44[1]) = sub_146E8BA20(16);
                v52 = v44[1];
              }
              *(_QWORD *)(*(_QWORD *)(v51 + v52) + 8 * (v50 & 1)) = *v46;
              v44[3] = v50;
              ++v44[4];
              v43 = v85 - 1;
              v53 = v85-- == 1;
              v45 = v84;
              if ( v53 )
                v45 = 0;
              v84 = v45;
            }
            while ( v43 );
          }
          sub_1470082F0(&v87);
        }
      }
      *(_BYTE *)(v1 + 1098) = 0;
LABEL_68:
      v5 = v84;
      goto LABEL_69;
    }
    v6 = qword_14E6343D0;
    if ( !qword_14E6343D0 )
    {
      v7 = sub_146E8BA20(72);
      v88 = v7;
      if ( v7 )
        v8 = (void (__fastcall ***)(_QWORD))sub_146E93360(v7);
      else
        v8 = 0;
      qword_14E6343D0 = (__int64)v8;
      (**v8)(v8);
      v6 = qword_14E6343D0;
    }
    v9 = sub_146E8C7D0(&unk_14B16F5B8);
    v10 = sub_146E8C7D0(&unk_14B16F600);
    v11 = sub_146E8C7D0(&unk_14B16EBA0);
    sub_146E938E0(v6, 4, v11, v10, 848, (__int64)&qword_14EF5B6C8, v9);
    v12 = *(_QWORD *)(v1 + 1280);
    if ( v12 )
      sub_146D74090(v12, &v82);
    if ( !v85 )
    {
      sub_146D73940(v1);
      v61 = 0;
      goto LABEL_85;
    }
    v13 = qword_14E6343D0;
    if ( !qword_14E6343D0 )
    {
      v14 = sub_146E8BA20(72);
      v89 = v14;
      if ( v14 )
        v15 = (void (__fastcall ***)(_QWORD))sub_146E93360(v14);
      else
        v15 = 0;
      qword_14E6343D0 = (__int64)v15;
      (**v15)(v15);
      v13 = qword_14E6343D0;
    }
    v16 = sub_146E8C7D0(&unk_14B16F650);
    v17 = sub_146E8C7D0(&unk_14B16F600);
    v18 = sub_146E8C7D0(&unk_14B16EBA0);
    sub_146E938E0(v13, 4, v18, v17, 862, (__int64)&qword_14EF5B6C8, v16);
    if ( *(_BYTE *)(v1 + 1103) )
      goto LABEL_68;
    v19 = qword_14E6343D0;
    if ( !qword_14E6343D0 )
    {
      v20 = sub_146E8BA20(72);
      if ( v20 )
        v21 = (void (__fastcall ***)(_QWORD))sub_146E93360(v20);
      else
        v21 = 0;
      qword_14E6343D0 = (__int64)v21;
      (**v21)(v21);
      v19 = qword_14E6343D0;
    }
    v22 = sub_146E8C7D0(&unk_14B16F6B0);
    v23 = sub_146E8C7D0(&unk_14B16F600);
    v24 = sub_146E8C7D0(&unk_14B16EBA0);
    sub_146E938E0(v19, 4, v24, v23, 865, (__int64)&qword_14EF5B6C8, v22);
    v5 = v84;
    v25 = v84;
    v26 = (__int64 *)v82;
    if ( v84 != v84 + v85 )
    {
      while ( 1 )
      {
        v27 = qword_14E6343D0;
        if ( !qword_14E6343D0 )
        {
          v28 = sub_146E8BA20(72);
          if ( v28 )
            v29 = (void (__fastcall ***)(_QWORD))sub_146E93360(v28);
          else
            v29 = 0;
          qword_14E6343D0 = (__int64)v29;
          (**v29)(v29);
          v27 = qword_14E6343D0;
        }
        if ( v26 )
          v30 = *v26;
        else
          v30 = 0;
        v31 = *(_QWORD *)(*(_QWORD *)(*(_QWORD *)(v30 + 8) + 8 * ((*(_QWORD *)(v30 + 16) - 1LL) & (v25 >> 1)))
                        + 8 * (v25 & 1));
        v32 = *(_WORD *)(v31 + 1);
        if ( *(_BYTE *)v31 )
        {
          if ( *(_BYTE *)v31 == 1 && v32 < 0x97Cu )
            goto LABEL_43;
        }
        else if ( v32 < 0xB54u )
        {
          goto LABEL_43;
        }
        sub_146E8C7D0(&unk_14B16FBD8);
LABEL_43:
        v33 = sub_146E8C7D0(&unk_14B16F6F0);
        v34 = sub_146E8C7D0(&unk_14B16F600);
        v35 = sub_146E8C7D0(&unk_14B16EBA0);
        sub_146E938E0(v27, 4, v35, v34, 868, (__int64)&qword_14EF5B6C8, v33);
        ++v25;
        v5 = v84;
        if ( v25 == v84 + v85 )
        {
          v1 = v86;
          break;
        }
      }
    }
    *(_BYTE *)(v1 + 1103) = 1;
LABEL_69:
    if ( !v85 )
      break;
    v54 = v83;
    v55 = *((_QWORD *)&v82 + 1);
    v56 = *(_QWORD *)(*(_QWORD *)(*((_QWORD *)&v82 + 1) + 8 * ((v5 >> 1) & (v83 - 1))) + 8 * (v5 & 1));
    if ( v56 && v56 != -16 )
    {
      sub_146EA2160(v56 + 16);
      sub_146EA2170((unsigned int)(*(_DWORD *)(v56 + 3) - 16));
      if ( !qword_14E6343D8 )
      {
        v57 = sub_146E9F2A0(184);
        if ( v57 )
          v58 = sub_146EC2F90(v57);
        else
          v58 = 0;
        qword_14E6343D8 = v58;
        (**(void (__fastcall ***)(__int64))(v58 + 16))(v58 + 16);
      }
      v59 = sub_1417698B0();
      sub_1459A1BB0(qword_14E66C090, v59, (unsigned __int8 *)v56);
      if ( (unsigned __int8)sub_146EA19D0() )
      {
        v62 = sub_146EA14F0();
        v67 = sub_14021A860(v64, v63, v65, v66, v81);
        sub_146D76590(v62);
        v68 = sub_146E8C7D0(&unk_14B16F790);
        v69 = sub_146E8C7D0(&unk_14B16F600);
        v70 = sub_146E8C7D0(&unk_14B16EBA0);
        sub_146E938E0(v67, 0, v70, v69, 928, (__int64)&qword_14EF5B6C8, v68);
        sub_146D746E0(v1, 217);
        LOBYTE(v86) = *(_BYTE *)v62;
        sub_146D75B10(v1, &v86, 1);
        LOWORD(v86) = *(_WORD *)(v62 + 1);
        sub_146D75B10(v1, &v86, 2);
        v72 = sub_146D74000(v71);
        sub_146D73A70(v72);
        sub_146EA0310();
        *(_BYTE *)(v1 + 1101) = 1;
        v61 = 0;
        goto LABEL_85;
      }
      v5 = v84;
      v54 = v83;
      v55 = *((_QWORD *)&v82 + 1);
    }
    v60 = (_QWORD *)(*(_QWORD *)(v55 + 8 * ((v54 - 1) & (v5 >> 1))) + 8 * (v5 & 1));
    sub_146E991C0(qword_14EF5B6C0, *v60);
    *v60 = 0;
    if ( --v85 )
    {
      v5 = ++v84;
    }
    else
    {
      v5 = 0;
      v84 = 0;
    }
  }
  v61 = 1;
LABEL_85:
  v73 = v85;
  if ( v85 )
  {
    v74 = v84;
    do
    {
      v85 = --v73;
      if ( !v73 )
        v74 = 0;
      v84 = v74;
    }
    while ( v73 );
  }
  v75 = v83;
  v76 = *((_QWORD *)&v82 + 1);
  if ( v83 )
  {
    do
    {
      --v75;
      if ( *(_QWORD *)(v76 + 8 * v75) )
      {
        sub_146E9F3A0(*(_QWORD *)(v76 + 8 * v75), 16);
        v76 = *((_QWORD *)&v82 + 1);
      }
    }
    while ( v75 );
    v75 = v83;
  }
  if ( v76 )
  {
    v77 = 8 * v75;
    v78 = v76;
    if ( (unsigned __int64)(8 * v75) >= 0x1000 )
    {
      v77 += 39;
      v76 = *(_QWORD *)(v76 - 8);
      if ( (unsigned __int64)(v78 - v76 - 8) > 0x1F )
        sub_148AAF304(v76, v77);
    }
    sub_146E9F3A0(v76, v77);
  }
  v83 = 0;
  v79 = v82;
  v82 = 0u;
  sub_146E9F3A0(v79, 16);
  return v61;
}

