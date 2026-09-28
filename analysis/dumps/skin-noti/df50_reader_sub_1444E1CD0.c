// reader_sub_1444E1CD0

void __fastcall sub_1444E1CD0(__int64 a1)
{
  int v2; // r13d
  int v3; // r14d
  __int64 *v4; // rsi
  __int64 v5; // rcx
  int v6; // eax
  __int64 v7; // rdx
  unsigned __int64 v8; // rbx
  __int64 v9; // rcx
  __int64 v10; // rax
  void (__fastcall ***v11)(_QWORD); // rcx
  __int64 v12; // rax
  unsigned __int64 v13; // rdx
  int v14; // r8d
  __int64 v15; // rdx
  __int64 v16; // rdx
  int v17; // eax
  __int64 *v18; // rcx
  __int64 v19; // rax
  void (__fastcall *v20)(__int64 *, unsigned __int64); // r10
  unsigned __int64 v21; // rdx
  int v22; // eax
  __int64 v23; // rcx
  __int64 v24; // rax
  void (__fastcall ***v25)(_QWORD); // rcx
  __int64 v26; // rdx
  __int64 v27; // rax
  __int64 v28; // rdx
  __int64 v29; // r12
  __int64 v30; // r15
  void (__fastcall *v31)(__int64, __int64); // rdi
  unsigned int v32; // ebx
  __int64 v33; // rax
  __int64 v34; // rax
  __int64 v35; // rax
  __int64 v36; // rcx
  void (__fastcall *v37)(__int64, __int64); // r8
  __int64 v38; // rax
  void (__fastcall ***v39)(_QWORD); // rcx
  __int64 v40; // rax
  __int64 v41; // r15
  void (__fastcall *v42)(__int64, __int64); // rdi
  unsigned int v43; // ebx
  __int64 v44; // rax
  __int64 v45; // rax
  __int64 v46; // rax
  __int64 *v47; // rdi
  __int64 v48; // rsi
  __int64 v49; // rdx
  __int128 v50; // [rsp+40h] [rbp-78h] BYREF
  _BYTE v51[16]; // [rsp+50h] [rbp-68h] BYREF
  _BYTE v52[16]; // [rsp+60h] [rbp-58h] BYREF

  v2 = -1;
  v3 = 0;
  v4 = (__int64 *)(a1 + 5592);
  do
  {
    if ( !*v4 )
      goto LABEL_50;
    v5 = *(_QWORD *)(a1 + 6152);
    if ( !v5 )
      goto LABEL_50;
    v6 = sub_146EE4DA0(v5);
    v7 = *(_QWORD *)(a1 + 6176);
    v8 = v3 + 4 * v6;
    if ( v8 >= (v7 - *(_QWORD *)(a1 + 6168)) >> 2 )
    {
      v18 = (__int64 *)*v4;
      v19 = *(_QWORD *)*v4;
LABEL_46:
      (*(void (__fastcall **)(__int64 *, _QWORD))(v19 + 16))(v18, 0);
      goto LABEL_47;
    }
    v9 = qword_14E664BF8;
    if ( !qword_14E664BF8 )
    {
      v10 = sub_146E8BA20(2792);
      if ( v10 )
        v11 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v10);
      else
        v11 = 0;
      qword_14E664BF8 = (__int64)v11;
      (**v11)(v11);
      v7 = *(_QWORD *)(a1 + 6176);
      v9 = qword_14E664BF8;
    }
    v12 = *(_QWORD *)(a1 + 6168);
    v13 = (v7 - v12) >> 2;
    if ( v13 <= v8 )
      sub_1401790B0(v9, v13);
    v14 = *(_DWORD *)(v12 + 4 * v8);
    v50 = 0;
    v15 = v4[1];
    if ( v15 )
    {
      _InterlockedIncrement((volatile signed __int32 *)(v15 + 8));
      v15 = v4[1];
    }
    *(_QWORD *)&v50 = *v4;
    *((_QWORD *)&v50 + 1) = v15;
    sub_1444CECC0(v9, (unsigned int)&v50, v14, 0, 1);
    LOBYTE(v16) = 1;
    (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)*v4 + 16LL))(*v4, v16);
    v17 = *(_DWORD *)(a1 + 6224);
    if ( v17 != 1 )
    {
      if ( v17 == 2 )
      {
        v23 = qword_14E664BF8;
        if ( !qword_14E664BF8 )
        {
          v24 = sub_146E8BA20(2792);
          if ( v24 )
            v25 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v24);
          else
            v25 = 0;
          qword_14E664BF8 = (__int64)v25;
          (**v25)(v25);
          v23 = qword_14E664BF8;
        }
        v26 = *(_QWORD *)(a1 + 6168);
        if ( (*(_QWORD *)(a1 + 6176) - v26) >> 2 <= v8 )
          goto LABEL_57;
        v27 = sub_1444D2A50(v23, *(unsigned int *)(v26 + 4 * v8));
        v29 = v27;
        if ( v27 )
        {
          if ( v3 < 4 )
          {
            v30 = v4[24];
            if ( v30 )
            {
              v31 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v30 + 688LL);
              v32 = *(unsigned __int16 *)(v27 + 68);
              v33 = sub_146E8C7D0(&unk_1491CE3A0);
              v34 = sub_146E8CF20(v51, v33, v32);
              v35 = sub_14014F430(v34);
              v31(v30, v35);
              sub_146E8C910(v51);
            }
          }
          v36 = v4[48];
          if ( v36 )
          {
            v37 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v36 + 16LL);
            if ( *(_DWORD *)(v29 + 64) )
            {
              v37(v36, 0);
            }
            else
            {
              LOBYTE(v28) = 1;
              v37(v36, v28);
            }
          }
        }
      }
      else if ( v17 == 3 )
      {
        v23 = qword_14E664BF8;
        if ( !qword_14E664BF8 )
        {
          v38 = sub_146E8BA20(2792);
          if ( v38 )
            v39 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v38);
          else
            v39 = 0;
          qword_14E664BF8 = (__int64)v39;
          (**v39)(v39);
          v23 = qword_14E664BF8;
        }
        v26 = *(_QWORD *)(a1 + 6168);
        if ( (*(_QWORD *)(a1 + 6176) - v26) >> 2 <= v8 )
LABEL_57:
          sub_1401790B0(v23, v26);
        v40 = sub_1444D2A50(v23, *(unsigned int *)(v26 + 4 * v8));
        if ( v40 )
        {
          if ( v3 < 4 )
          {
            v41 = v4[24];
            if ( v41 )
            {
              v42 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v41 + 688LL);
              v43 = *(unsigned __int16 *)(v40 + 72);
              v44 = sub_146E8C7D0(&unk_1491CE3A0);
              v45 = sub_146E8CF20(v52, v44, v43);
              v46 = sub_14014F430(v45);
              v42(v41, v46);
              sub_146E8C910(v52);
            }
          }
        }
      }
      goto LABEL_47;
    }
    v18 = (__int64 *)v4[32];
    if ( v18 )
    {
      v19 = *v18;
      if ( v8 >= *(_QWORD *)(a1 + 6216) )
        goto LABEL_46;
      v20 = *(void (__fastcall **)(__int64 *, unsigned __int64))(v19 + 16);
      v21 = v8 & 0x1F;
      v22 = *(_DWORD *)(*(_QWORD *)(a1 + 6192) + 4 * (v8 >> 5));
      if ( _bittest(&v22, v21) )
      {
        LOBYTE(v21) = 1;
        v20(v18, v21);
      }
      else
      {
        v20(v18, 0);
      }
    }
LABEL_47:
    if ( (unsigned __int8)sub_141FB6530(*v4) && (unsigned __int8)sub_146ED0010(*v4) )
      v2 = v3;
LABEL_50:
    ++v3;
    v4 += 2;
  }
  while ( v3 < 8 );
  v47 = *(__int64 **)(a1 + 6104);
  if ( v47 )
  {
    v48 = *v47;
    if ( v2 == -1 )
    {
      (*(void (__fastcall **)(_QWORD, _QWORD))(v48 + 16))(*(_QWORD *)(a1 + 6104), 0);
    }
    else
    {
      sub_142757420(*(_QWORD *)(a1 + 16LL * v2 + 5592));
      sub_146ECA0C0(*(_QWORD *)(a1 + 16LL * v2 + 5592));
      (*(void (__fastcall **)(__int64 *))(v48 + 112))(v47);
      LOBYTE(v49) = 1;
      (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 6104) + 16LL))(*(_QWORD *)(a1 + 6104), v49);
    }
  }
}

