// singleton_user_sub_1444A02B0

char __fastcall sub_1444A02B0(_QWORD *a1)
{
  __int64 v2; // rsi
  __int64 v3; // rax
  volatile signed __int32 *v4; // rbx
  _DWORD *v5; // rcx
  _DWORD *v6; // rcx
  __int64 v7; // rax
  __int64 v8; // rbx
  __int64 v9; // rax
  void (__fastcall ***v10)(_QWORD); // rcx
  unsigned int v11; // edi
  unsigned int v12; // ebx
  __int64 v13; // rax
  unsigned int v14; // eax
  __int64 v15; // r8
  _QWORD *v16; // rdi
  __int64 v17; // rdx
  __int64 v18; // rdx
  __int64 v19; // r8
  __int64 v20; // rcx
  _QWORD *v21; // rdx
  _QWORD *v22; // r8
  unsigned __int64 v23; // rdx
  __int64 v24; // rcx
  __int64 v25; // rdx
  __int64 v26; // rcx
  __int64 v27; // rax
  _DWORD *v28; // rcx
  int v29; // r8d
  __int64 v30; // rax
  __int64 v31; // rdx
  __int64 v32; // rcx
  __int64 v33; // rax
  _DWORD *v34; // rcx
  int v35; // r8d
  __int64 v36; // rax
  int *v37; // rcx
  unsigned __int64 v38; // rdx
  __int64 v39; // rcx
  __int64 v40; // rbx
  __int64 v41; // rax
  _QWORD *v42; // rax
  __int64 v43; // rdx
  __int64 v44; // rcx
  volatile signed __int32 *v45; // rbx
  int *v47; // [rsp+40h] [rbp-91h] BYREF
  void (__fastcall *v48)(__int64, int **, _QWORD *); // [rsp+48h] [rbp-89h]
  __int128 v49; // [rsp+50h] [rbp-81h]
  _DWORD *v50; // [rsp+60h] [rbp-71h]
  _DWORD *v51; // [rsp+68h] [rbp-69h]
  _DWORD *v52; // [rsp+70h] [rbp-61h] BYREF
  _DWORD *v53; // [rsp+78h] [rbp-59h] BYREF
  _DWORD *v54; // [rsp+80h] [rbp-51h] BYREF
  _DWORD *v55; // [rsp+88h] [rbp-49h] BYREF
  __int128 v56; // [rsp+90h] [rbp-41h]
  __int64 v57; // [rsp+A0h] [rbp-31h]
  __int128 v58; // [rsp+A8h] [rbp-29h]
  _QWORD v59[2]; // [rsp+B8h] [rbp-19h] BYREF
  unsigned __int64 v60; // [rsp+C8h] [rbp-9h]
  unsigned __int64 v61; // [rsp+D0h] [rbp-1h]
  _QWORD v62[2]; // [rsp+D8h] [rbp+7h] BYREF
  __int64 v63; // [rsp+E8h] [rbp+17h]
  unsigned __int64 v64; // [rsp+F0h] [rbp+1Fh]

  v57 = -2;
  v2 = 0;
  v3 = a1[205];
  if ( v3 )
  {
    *(_QWORD *)(v3 + 960) = 0;
    sub_146B62340(a1[205], 0);
    v56 = 0;
    v58 = 0;
    *(_QWORD *)&v56 = a1[205];
    a1[205] = 0;
    *((_QWORD *)&v56 + 1) = a1[206];
    v4 = (volatile signed __int32 *)*((_QWORD *)&v56 + 1);
    a1[206] = 0;
    if ( v4 )
    {
      if ( _InterlockedExchangeAdd(v4 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v4)(v4);
        if ( _InterlockedExchangeAdd(v4 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v4 + 8LL))(v4);
      }
    }
  }
  v50 = 0;
  v50 = (_DWORD *)a1[207];
  v5 = v50;
  a1[207] = 0;
  if ( v5 )
  {
    if ( (int)--v5[2] <= 0 )
      (*(void (__fastcall **)(_DWORD *))(*(_QWORD *)v5 + 8LL))(v5);
  }
  v51 = 0;
  v51 = (_DWORD *)a1[208];
  v6 = v51;
  a1[208] = 0;
  if ( v6 )
  {
    if ( (int)--v6[2] <= 0 )
      (*(void (__fastcall **)(_DWORD *))(*(_QWORD *)v6 + 8LL))(v6);
  }
  v7 = sub_145F0BA60(qword_14E683C08);
  v8 = v7;
  if ( v7 )
  {
    if ( !qword_14E659EA8 )
    {
      v9 = sub_146E8BA20(496);
      v48 = (void (__fastcall *)(__int64, int **, _QWORD *))v9;
      if ( v9 )
        v10 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v9);
      else
        v10 = 0;
      qword_14E659EA8 = (__int64)v10;
      (**v10)(v10);
    }
    sub_14449DFB0();
    v11 = sub_1406FE620(v8);
    v12 = sub_140BA9880(v8);
    v13 = sub_14355BA70();
    v14 = sub_14449DFC0(v13, v11, v12);
    LOBYTE(v15) = 1;
    v16 = (_QWORD *)sub_140283D60(qword_14E683B20, v14, v15);
    v17 = v16[955];
    if ( v17 == v16[956] )
      goto LABEL_54;
    v18 = *(_QWORD *)(v17 + 8);
    v59[0] = 0;
    v60 = 0;
    v61 = 7;
    v19 = -1;
    do
      ++v19;
    while ( *(_WORD *)(v18 + 2 * v19) );
    sub_14014C8D0(v59, v18);
    v20 = *(_QWORD *)qword_14F1C39C8;
    v48 = *(void (__fastcall **)(__int64, int **, _QWORD *))(*(_QWORD *)qword_14F1C39C8 + 16LL);
    v62[0] = 0;
    v63 = 0;
    v64 = 7;
    if ( v60 < 7 )
      sub_1401F6210(v20);
    v21 = v59;
    if ( v61 >= 8 )
      v21 = (_QWORD *)v59[0];
    sub_14014C8D0(v62, (char *)v21 + 14);
    v22 = v62;
    if ( v64 >= 8 )
      v22 = (_QWORD *)v62[0];
    v48(qword_14F1C39C8, &v47, v22);
    if ( v64 >= 8 )
    {
      v23 = 2 * v64 + 2;
      v24 = v62[0];
      if ( v23 >= 0x1000 )
      {
        v23 = 2 * v64 + 41;
        v24 = *(_QWORD *)(v62[0] - 8LL);
        if ( (unsigned __int64)(v62[0] - v24 - 8) > 0x1F )
          sub_148AAF304(v24, v23);
      }
      sub_146E9F3A0(v24, v23);
    }
    v63 = 0;
    v64 = 7;
    LOWORD(v62[0]) = 0;
    v25 = v16[955];
    v26 = *(_QWORD *)(v25 + 536);
    if ( (unsigned __int64)((*(_QWORD *)(v25 + 544) - v26) >> 3) <= 2 || *(_DWORD *)(v26 + 16) == -1 )
    {
      v29 = *(_DWORD *)(v25 + 32);
      if ( v29 == -1 )
        goto LABEL_38;
      v30 = (*(__int64 (__fastcall **)(int *, _DWORD **, _QWORD))(*(_QWORD *)v47 + 120LL))(v47, &v53, 2 * v29);
      sub_1401F1060(a1 + 207, v30);
      v28 = v53;
    }
    else
    {
      v27 = (*(__int64 (__fastcall **)(int *, _DWORD **))(*(_QWORD *)v47 + 120LL))(v47, &v52);
      sub_1401F1060(a1 + 207, v27);
      v28 = v52;
    }
    if ( v28 )
    {
      if ( (int)--v28[2] <= 0 )
        (*(void (__fastcall **)(_DWORD *))(*(_QWORD *)v28 + 8LL))(v28);
    }
LABEL_38:
    v31 = v16[955];
    v32 = *(_QWORD *)(v31 + 536);
    if ( (unsigned __int64)((*(_QWORD *)(v31 + 544) - v32) >> 3) <= 3 || *(_DWORD *)(v32 + 24) == -1 )
    {
      v35 = *(_DWORD *)(v31 + 36);
      if ( v35 == -1 )
      {
LABEL_46:
        v37 = v47;
        if ( v47 )
        {
          --v47[2];
          if ( v37[2] <= 0 )
            (*(void (__fastcall **)(int *))(*(_QWORD *)v37 + 8LL))(v37);
        }
        if ( v61 >= 8 )
        {
          v38 = 2 * v61 + 2;
          v39 = v59[0];
          if ( v38 >= 0x1000 )
          {
            v38 = 2 * v61 + 41;
            v39 = *(_QWORD *)(v59[0] - 8LL);
            if ( (unsigned __int64)(v59[0] - v39 - 8) > 0x1F )
              sub_148AAF304(v39, v38);
          }
          sub_146E9F3A0(v39, v38);
        }
        v60 = 0;
        v61 = 7;
        LOWORD(v59[0]) = 0;
LABEL_54:
        v40 = v16[594];
        v41 = sub_146E8BA20(2800);
        v48 = (void (__fastcall *)(__int64, int **, _QWORD *))v41;
        if ( v41 )
          v2 = sub_146B6BC70(v41, v40, v16[477], 0, 0);
        v42 = (_QWORD *)sub_140251BC0(v2);
        v49 = 0;
        v43 = v42[1];
        if ( v43 )
        {
          _InterlockedIncrement((volatile signed __int32 *)(v43 + 8));
          v43 = v42[1];
        }
        *(_QWORD *)&v49 = *v42;
        v44 = v49;
        *((_QWORD *)&v49 + 1) = v43;
        *(_QWORD *)&v49 = a1[205];
        a1[205] = v44;
        *((_QWORD *)&v49 + 1) = a1[206];
        v45 = (volatile signed __int32 *)*((_QWORD *)&v49 + 1);
        a1[206] = v43;
        if ( v45 )
        {
          if ( _InterlockedExchangeAdd(v45 + 2, 0xFFFFFFFF) == 1 )
          {
            (**(void (__fastcall ***)(volatile signed __int32 *))v45)(v45);
            if ( _InterlockedExchangeAdd(v45 + 3, 0xFFFFFFFF) == 1 )
              (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v45 + 8LL))(v45);
          }
        }
        LOBYTE(v7) = 1;
        return v7;
      }
      v36 = (*(__int64 (__fastcall **)(int *, _DWORD **, _QWORD))(*(_QWORD *)v47 + 120LL))(v47, &v55, 2 * v35);
      sub_1401F1060(a1 + 208, v36);
      v34 = v55;
    }
    else
    {
      v33 = (*(__int64 (__fastcall **)(int *, _DWORD **))(*(_QWORD *)v47 + 120LL))(v47, &v54);
      sub_1401F1060(a1 + 208, v33);
      v34 = v54;
    }
    if ( v34 )
    {
      if ( (int)--v34[2] <= 0 )
        (*(void (__fastcall **)(_DWORD *))(*(_QWORD *)v34 + 8LL))(v34);
    }
    goto LABEL_46;
  }
  return v7;
}

