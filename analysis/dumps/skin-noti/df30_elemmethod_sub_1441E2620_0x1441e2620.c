// elemmethod_sub_1441E2620_0x1441e2620

char __fastcall sub_1441E2620(__int64 a1, __int64 a2, __int64 a3)
{
  int v3; // r12d
  unsigned int v4; // r13d
  __int64 v6; // r14
  void (__fastcall *v7)(__int64, int **, __int64); // rbx
  __int64 v8; // rax
  __int64 v9; // rdi
  void (__fastcall *v10)(__int64, __int64); // rbx
  __int64 v11; // rax
  __int64 v12; // rdx
  int *v13; // rcx
  unsigned int v14; // ebx
  __int64 v15; // rdx
  __int64 v16; // r8
  __int64 v17; // rcx
  __int64 v18; // rcx
  __int64 v20; // r13
  unsigned int v21; // ebx
  _QWORD *v22; // rdx
  _DWORD *v23; // r14
  __int64 v24; // r8
  __int64 v25; // rcx
  __int64 v26; // rcx
  _QWORD *v27; // rax
  _DWORD *v28; // r15
  int *v29; // rcx
  __int64 v30; // rdi
  void (__fastcall *v31)(__int64, __int64); // rbx
  __int64 v32; // rax
  __int64 v33; // rdx
  __int64 v34; // rdx
  __int64 v35; // r8
  __int64 v36; // r15
  __int64 v37; // r14
  _QWORD *v38; // rdx
  __int64 v39; // r8
  __int64 v40; // rcx
  __int64 v41; // rcx
  __int64 v42; // r9
  __int128 *v43; // rdx
  __int128 *v44; // rcx
  unsigned int v45; // ebx
  __int64 v46; // rcx
  __int64 v47; // rcx
  __int64 v48; // rdi
  void (__fastcall *v49)(__int64, __int64); // rbx
  __int64 v50; // rax
  __int64 v51; // rdx
  int *v52; // rcx
  _QWORD *v53; // rdx
  __int64 v54; // rdx
  __int64 v55; // rdx
  _QWORD *v56; // [rsp+28h] [rbp-E0h] BYREF
  _DWORD *v57; // [rsp+30h] [rbp-D8h] BYREF
  __int128 *v58; // [rsp+38h] [rbp-D0h]
  __int128 *v59; // [rsp+40h] [rbp-C8h]
  int *v60; // [rsp+48h] [rbp-C0h] BYREF
  __int128 v61; // [rsp+50h] [rbp-B8h] BYREF
  __int128 v62; // [rsp+60h] [rbp-A8h] BYREF
  __int128 v63; // [rsp+70h] [rbp-98h] BYREF
  __int128 v64; // [rsp+80h] [rbp-88h] BYREF
  __int128 v65; // [rsp+90h] [rbp-78h] BYREF
  __int128 v66; // [rsp+A0h] [rbp-68h] BYREF
  __int128 v67; // [rsp+B0h] [rbp-58h] BYREF
  __int128 v68; // [rsp+C0h] [rbp-48h] BYREF
  _QWORD v69[4]; // [rsp+D0h] [rbp-38h] BYREF
  _QWORD v70[5]; // [rsp+F0h] [rbp-18h] BYREF
  _BYTE v71[32]; // [rsp+118h] [rbp+10h] BYREF
  _BYTE v72[32]; // [rsp+138h] [rbp+30h] BYREF
  int *v73; // [rsp+1A0h] [rbp+98h] BYREF

  v70[4] = -2;
  v3 = a3;
  v4 = a2;
  if ( (_DWORD)a3 == 5 )
  {
    if ( (qword_14E6A7C70 - qword_14E6A7C68) / 96 > (unsigned __int64)(int)a2 )
    {
      v6 = qword_14E6A7C68 + 96LL * (int)a2;
      if ( qword_14F1C39C8 )
      {
        v7 = *(void (__fastcall **)(__int64, int **, __int64))(*(_QWORD *)qword_14F1C39C8 + 16LL);
        v8 = sub_146E8C7D0(&unk_14A2320E0);
        v7(qword_14F1C39C8, &v73, v8);
        if ( v73 )
        {
          v9 = *(_QWORD *)(a1 + 96);
          v10 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v9 + 664LL);
          v56 = &v57;
          v11 = (*(__int64 (__fastcall **)(int *, _DWORD **, _QWORD))(*(_QWORD *)v73 + 120LL))(
                  v73,
                  &v57,
                  *(int *)(v6 + 4));
          v10(v9, v11);
          LOBYTE(v12) = 1;
          (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 96) + 16LL))(*(_QWORD *)(a1 + 96), v12);
        }
        v13 = v73;
        if ( v73 )
        {
          --v73[2];
          if ( v13[2] <= 0 )
            (*(void (__fastcall **)(int *))(*(_QWORD *)v13 + 8LL))(v13);
        }
      }
      v14 = *(_DWORD *)(a1 + 160);
      v56 = v69;
      v15 = *(_QWORD *)(v6 + 56);
      v69[0] = 0;
      v69[2] = 0;
      v69[3] = 7;
      v16 = -1;
      do
        ++v16;
      while ( *(_WORD *)(v15 + 2 * v16) );
      sub_14014C8D0(v69, v15);
      v60 = (int *)&v61;
      v61 = 0;
      v17 = *(_QWORD *)(a1 + 176);
      if ( v17 )
      {
        _InterlockedIncrement((volatile signed __int32 *)(v17 + 8));
        v17 = *(_QWORD *)(a1 + 176);
      }
      *(_QWORD *)&v61 = *(_QWORD *)(a1 + 168);
      *((_QWORD *)&v61 + 1) = v17;
      v58 = &v62;
      v62 = 0;
      v18 = *(_QWORD *)(a1 + 192);
      if ( v18 )
      {
        _InterlockedIncrement((volatile signed __int32 *)(v18 + 8));
        v18 = *(_QWORD *)(a1 + 192);
      }
      *(_QWORD *)&v62 = *(_QWORD *)(a1 + 184);
      *((_QWORD *)&v62 + 1) = v18;
      sub_1441EC970(&v62, &v61, v69, v14);
      *(_DWORD *)(a1 + 40) = v4;
      *(_DWORD *)(a1 + 44) = 5;
      return 1;
    }
    return 0;
  }
  if ( (_DWORD)a3 == 9 )
  {
    *(_DWORD *)(a1 + 40) = a2;
    *(_DWORD *)(a1 + 44) = 9;
    LOBYTE(a3) = 1;
    v20 = sub_140283D60(qword_14E683B38, (unsigned int)a2, a3);
    if ( !v20 )
      return 0;
    v21 = *(_DWORD *)(a1 + 160);
    v73 = (int *)v70;
    v22 = (_QWORD *)(v20 + 496);
    if ( *(_QWORD *)(v20 + 520) >= 8u )
      v22 = (_QWORD *)*v22;
    v23 = 0;
    v70[0] = 0;
    v70[2] = 0;
    v70[3] = 7;
    v24 = -1;
    do
      ++v24;
    while ( *((_WORD *)v22 + v24) );
    sub_14014C8D0(v70, v22);
    v58 = &v63;
    v63 = 0;
    v25 = *(_QWORD *)(a1 + 176);
    if ( v25 )
    {
      _InterlockedIncrement((volatile signed __int32 *)(v25 + 8));
      v25 = *(_QWORD *)(a1 + 176);
    }
    *(_QWORD *)&v63 = *(_QWORD *)(a1 + 168);
    *((_QWORD *)&v63 + 1) = v25;
    v59 = &v64;
    v64 = 0;
    v26 = *(_QWORD *)(a1 + 192);
    if ( v26 )
    {
      _InterlockedIncrement((volatile signed __int32 *)(v26 + 8));
      v26 = *(_QWORD *)(a1 + 192);
    }
    *(_QWORD *)&v64 = *(_QWORD *)(a1 + 184);
    *((_QWORD *)&v64 + 1) = v26;
    sub_1441EC970(&v64, &v63, v70, v21);
    v57 = 0;
    if ( qword_14F1C39C8 )
    {
      v27 = (_QWORD *)(*(__int64 (__fastcall **)(__int64, int **, _QWORD))(*(_QWORD *)qword_14F1C39C8 + 16LL))(
                        qword_14F1C39C8,
                        &v60,
                        *(_QWORD *)(v20 + 352));
      v28 = (_DWORD *)*v27;
      *v27 = 0;
      v73 = 0;
      v23 = v28;
      v57 = v28;
      v29 = v60;
      if ( v60 )
      {
        --v60[2];
        if ( v29[2] <= 0 )
          (*(void (__fastcall **)(int *))(*(_QWORD *)v29 + 8LL))(v29);
      }
      if ( v28 )
      {
        v30 = *(_QWORD *)(a1 + 96);
        v31 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v30 + 664LL);
        v73 = (int *)&v56;
        v32 = (*(__int64 (__fastcall **)(_DWORD *, _QWORD **, _QWORD))(*(_QWORD *)v28 + 120LL))(
                v28,
                &v56,
                *(int *)(v20 + 368));
        v31(v30, v32);
        LOBYTE(v33) = 1;
        (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 96) + 16LL))(*(_QWORD *)(a1 + 96), v33);
      }
    }
    if ( v23 )
    {
      if ( (int)--v23[2] <= 0 )
        (*(void (__fastcall **)(_DWORD *))(*(_QWORD *)v23 + 8LL))(v23);
    }
    return 1;
  }
  v36 = sub_1444EBAB0(a2, a2, a3);
  if ( v3 == 4 )
    v36 = sub_1444EBAB0(0x9C40u, v34, v35);
  if ( !v36 || v3 != 10 && *(_DWORD *)(v36 + 8) != v3 )
    return 0;
  *(_DWORD *)(a1 + 40) = v4;
  *(_DWORD *)(a1 + 44) = v3;
  if ( !qword_14F1C39C8 )
    return 1;
  v37 = 0;
  if ( v3 == 4 )
  {
    LOBYTE(v35) = 1;
    v37 = sub_14021BE90(qword_14E683B30, v4, v35);
    if ( !v37 )
      return 0;
    v73 = (int *)v71;
    v38 = (_QWORD *)(v37 + 496);
    if ( *(_QWORD *)(v37 + 520) >= 8u )
      v38 = (_QWORD *)*v38;
    v39 = sub_14014C7B0(v71, v38);
    v59 = &v65;
    v65 = 0;
    v40 = *(_QWORD *)(a1 + 176);
    if ( v40 )
    {
      _InterlockedIncrement((volatile signed __int32 *)(v40 + 8));
      v40 = *(_QWORD *)(a1 + 176);
    }
    *(_QWORD *)&v65 = *(_QWORD *)(a1 + 168);
    *((_QWORD *)&v65 + 1) = v40;
    v58 = &v66;
    v66 = 0;
    v41 = *(_QWORD *)(a1 + 192);
    if ( v41 )
    {
      _InterlockedIncrement((volatile signed __int32 *)(v41 + 8));
      v41 = *(_QWORD *)(a1 + 192);
    }
    *(_QWORD *)&v66 = *(_QWORD *)(a1 + 184);
    *((_QWORD *)&v66 + 1) = v41;
    v42 = 0;
    v43 = &v65;
    v44 = &v66;
  }
  else
  {
    v45 = *(_DWORD *)(a1 + 160);
    v73 = (int *)v72;
    v39 = sub_14014C810(v72);
    v59 = &v67;
    v67 = 0;
    v46 = *(_QWORD *)(a1 + 176);
    if ( v46 )
    {
      _InterlockedIncrement((volatile signed __int32 *)(v46 + 8));
      v46 = *(_QWORD *)(a1 + 176);
    }
    *(_QWORD *)&v67 = *(_QWORD *)(a1 + 168);
    *((_QWORD *)&v67 + 1) = v46;
    v58 = &v68;
    v68 = 0;
    v47 = *(_QWORD *)(a1 + 192);
    if ( v47 )
    {
      _InterlockedIncrement((volatile signed __int32 *)(v47 + 8));
      v47 = *(_QWORD *)(a1 + 192);
    }
    *(_QWORD *)&v68 = *(_QWORD *)(a1 + 184);
    *((_QWORD *)&v68 + 1) = v47;
    v42 = v45;
    v43 = &v67;
    v44 = &v68;
  }
  sub_1441EC970(v44, v43, v39, v42);
  (*(void (__fastcall **)(_QWORD))(**(_QWORD **)(a1 + 200) + 688LL))(*(_QWORD *)(a1 + 200));
  if ( v3 == 4 )
  {
    (*(void (__fastcall **)(__int64, int **, _QWORD))(*(_QWORD *)qword_14F1C39C8 + 16LL))(
      qword_14F1C39C8,
      &v73,
      *(_QWORD *)(v37 + 352));
    if ( v73 )
    {
      v48 = *(_QWORD *)(a1 + 96);
      v49 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v48 + 664LL);
      v59 = (__int128 *)&v56;
      v50 = (*(__int64 (__fastcall **)(int *, _QWORD **, _QWORD))(*(_QWORD *)v73 + 120LL))(
              v73,
              &v56,
              *(int *)(v37 + 368));
      v49(v48, v50);
      LOBYTE(v51) = 1;
      (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 96) + 16LL))(*(_QWORD *)(a1 + 96), v51);
    }
    v52 = v73;
    if ( v73 )
    {
      --v73[2];
      if ( v52[2] <= 0 )
        (*(void (__fastcall **)(int *))(*(_QWORD *)v52 + 8LL))(v52);
    }
  }
  if ( !*(_QWORD *)(v36 + 368) || *(int *)(v36 + 384) < 0 || v3 == 4 )
    return 1;
  (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 96) + 16LL))(*(_QWORD *)(a1 + 96), 0);
  (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 128) + 16LL))(*(_QWORD *)(a1 + 128), 0);
  v53 = (_QWORD *)(v36 + 352);
  if ( *(_DWORD *)(v36 + 12) <= 1u )
  {
    if ( *(_QWORD *)(v36 + 376) >= 8u )
      v53 = (_QWORD *)*v53;
    sub_146EECE20(*(_QWORD *)(a1 + 128), v53);
    sub_146EECBB0(*(_QWORD *)(a1 + 128), *(unsigned int *)(v36 + 384));
    LOBYTE(v55) = 1;
    (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 128) + 16LL))(*(_QWORD *)(a1 + 128), v55);
    return 1;
  }
  else
  {
    if ( *(_QWORD *)(v36 + 376) >= 8u )
      v53 = (_QWORD *)*v53;
    sub_146EECE20(*(_QWORD *)(a1 + 96), v53);
    sub_146EECBB0(*(_QWORD *)(a1 + 96), *(unsigned int *)(v36 + 384));
    LOBYTE(v54) = 1;
    (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 96) + 16LL))(*(_QWORD *)(a1 + 96), v54);
    sub_146EED6D0(
      *(_QWORD *)(a1 + 96),
      *(unsigned __int8 *)(v36 + 388)
    | (*(unsigned __int8 *)(v36 + 392) << 8)
    | ((*(unsigned __int8 *)(v36 + 396) | 0xFFFFFF00) << 16));
    return 1;
  }
}

