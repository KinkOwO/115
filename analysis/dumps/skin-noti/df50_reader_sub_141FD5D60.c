// reader_sub_141FD5D60

double __fastcall sub_141FD5D60(__int64 a1)
{
  unsigned int v2; // r12d
  __int64 v3; // rcx
  __int64 v4; // rdi
  __int64 v5; // rbx
  __int64 v6; // rax
  __int64 v7; // rax
  __int64 **v8; // rdx
  __int64 *i; // rcx
  __int64 v10; // rcx
  __int64 j; // rax
  unsigned int v12; // esi
  unsigned int v13; // r14d
  __int64 v14; // r13
  __int64 v15; // rax
  volatile signed __int32 *v16; // rdi
  __int64 v17; // rax
  volatile signed __int32 *v18; // rdi
  __int64 v19; // rsi
  void (__fastcall *v20)(__int64, __int64); // rdi
  __int64 v21; // rax
  int v22; // r14d
  __int64 v23; // rax
  __int64 v24; // rax
  __int64 v25; // rax
  volatile signed __int32 *v26; // rdi
  __int64 v27; // rax
  volatile signed __int32 *v28; // rdi
  __int64 v29; // rax
  __int64 v30; // rax
  unsigned int *v31; // r15
  volatile signed __int32 *v32; // rdi
  volatile signed __int32 *v33; // rdi
  volatile signed __int32 *v34; // rdi
  __int64 v35; // rdx
  __int64 v36; // rdx
  __int64 v37; // rdx
  __int64 v38; // rax
  volatile signed __int32 *v39; // rdi
  __int64 v40; // r14
  void (__fastcall *v41)(__int64, __int64); // rsi
  unsigned int v42; // edi
  __int64 v43; // rax
  __int64 v44; // rax
  __int64 v45; // rax
  volatile signed __int32 *v46; // rdi
  volatile signed __int32 *v47; // rdi
  volatile signed __int32 *v48; // rdi
  volatile signed __int32 *v49; // rdi
  volatile signed __int32 *v50; // rdi
  volatile signed __int32 *v51; // rdi
  __int64 v52; // rcx
  __int64 *v53; // rbx
  __int64 *v54; // rcx
  __int128 v56; // [rsp+20h] [rbp-E0h] BYREF
  __int64 v57; // [rsp+30h] [rbp-D0h] BYREF
  volatile signed __int32 *v58; // [rsp+38h] [rbp-C8h]
  __int64 v59; // [rsp+40h] [rbp-C0h] BYREF
  volatile signed __int32 *v60; // [rsp+48h] [rbp-B8h]
  __int64 v61; // [rsp+58h] [rbp-A8h] BYREF
  volatile signed __int32 *v62; // [rsp+60h] [rbp-A0h]
  __int128 *v63; // [rsp+68h] [rbp-98h]
  __int128 *v64; // [rsp+70h] [rbp-90h]
  __int64 v65; // [rsp+78h] [rbp-88h] BYREF
  volatile signed __int32 *v66; // [rsp+80h] [rbp-80h]
  __int64 v67; // [rsp+88h] [rbp-78h] BYREF
  volatile signed __int32 *v68; // [rsp+90h] [rbp-70h]
  __int64 v69; // [rsp+98h] [rbp-68h] BYREF
  volatile signed __int32 *v70; // [rsp+A0h] [rbp-60h]
  __int64 v71; // [rsp+A8h] [rbp-58h]
  _BYTE v72[8]; // [rsp+B0h] [rbp-50h] BYREF
  volatile signed __int32 *v73; // [rsp+B8h] [rbp-48h]
  _BYTE v74[8]; // [rsp+C0h] [rbp-40h] BYREF
  volatile signed __int32 *v75; // [rsp+C8h] [rbp-38h]
  _BYTE v76[16]; // [rsp+D0h] [rbp-30h] BYREF
  _BYTE v77[8]; // [rsp+E0h] [rbp-20h] BYREF
  volatile signed __int32 *v78; // [rsp+E8h] [rbp-18h]
  _BYTE v79[8]; // [rsp+F0h] [rbp-10h] BYREF
  volatile signed __int32 *v80; // [rsp+F8h] [rbp-8h]
  _BYTE v81[8]; // [rsp+100h] [rbp+0h] BYREF
  volatile signed __int32 *v82; // [rsp+108h] [rbp+8h]
  _BYTE v83[64]; // [rsp+110h] [rbp+10h] BYREF
  unsigned __int8 v84; // [rsp+168h] [rbp+68h]
  unsigned int v85; // [rsp+168h] [rbp+68h]
  __int64 v86; // [rsp+170h] [rbp+70h]

  v71 = -2;
  v2 = 0;
  sub_141FC6980(a1, 0);
  v4 = sub_141F880C0(v3);
  v86 = v4;
  v5 = sub_1401E65D0(v4);
  v56 = 0;
  v63 = &v56;
  v64 = &v56;
  v6 = sub_146E8BA20(40);
  *(_QWORD *)v6 = v6;
  *(_QWORD *)(v6 + 8) = v6;
  *(_QWORD *)(v6 + 16) = v6;
  *(_WORD *)(v6 + 24) = 257;
  *(_QWORD *)&v56 = v6;
  v7 = sub_1401DBAA0(&v56, *(_QWORD *)(*(_QWORD *)(v5 + 176) + 8LL), v6, v84);
  *(_QWORD *)(v56 + 8) = v7;
  *((_QWORD *)&v56 + 1) = *(_QWORD *)(v5 + 184);
  v8 = *(__int64 ***)(v56 + 8);
  if ( *((_BYTE *)v8 + 25) )
  {
    *(_QWORD *)v56 = v56;
    *(_QWORD *)(v56 + 16) = v56;
  }
  else
  {
    for ( i = *v8; !*((_BYTE *)i + 25); i = (__int64 *)*i )
      v8 = (__int64 **)i;
    *(_QWORD *)v56 = v8;
    v10 = *(_QWORD *)(v56 + 8);
    for ( j = *(_QWORD *)(v10 + 16); !*(_BYTE *)(j + 25); j = *(_QWORD *)(j + 16) )
      v10 = j;
    *(_QWORD *)(v56 + 16) = v10;
  }
  v64 = 0;
  v12 = sub_141F88070(v4);
  v85 = v12;
  v13 = DWORD2(v56);
  v14 = a1 + 1512;
  v15 = sub_141FC7BD0(v14, v72, 71);
  sub_140242390(&v69, v15);
  v16 = v73;
  if ( v73 )
  {
    if ( _InterlockedExchangeAdd(v73 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v16)(v16);
      if ( _InterlockedExchangeAdd(v16 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v16 + 8LL))(v16);
    }
  }
  (*(void (__fastcall **)(__int64, _QWORD, _QWORD))(*(_QWORD *)v69 + 672LL))(v69, v12, v13);
  v17 = sub_141FC7BD0(v14, v74, 34);
  sub_1401E9D00(&v67, v17);
  v18 = v75;
  if ( v75 )
  {
    if ( _InterlockedExchangeAdd(v75 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v18)(v18);
      if ( _InterlockedExchangeAdd(v18 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v18 + 8LL))(v18);
    }
  }
  v19 = v67;
  v20 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v67 + 688LL);
  v21 = sub_146E8C7D0(&unk_1491CDFA0);
  v22 = v85;
  v23 = sub_146E8CF20(v76, v21, v85);
  v24 = sub_14014F430(v23);
  v20(v19, v24);
  sub_146E8C910(v76);
  do
  {
    sub_141FC7BD0(v14, &v61, v2 + 82);
    v25 = sub_141FC7BD0(v14, v77, v2 + 102);
    sub_1402424B0(&v59, v25);
    v26 = v78;
    if ( v78 )
    {
      if ( _InterlockedExchangeAdd(v78 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v26)(v26);
        if ( _InterlockedExchangeAdd(v26 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v26 + 8LL))(v26);
      }
    }
    v27 = sub_141FC7BD0(v14, v79, v2 + 72);
    sub_1402425E0(&v57, v27);
    v28 = v80;
    if ( v80 )
    {
      if ( _InterlockedExchangeAdd(v80 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v28)(v28);
        if ( _InterlockedExchangeAdd(v28 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v28 + 8LL))(v28);
      }
    }
    v29 = sub_1401E65D0(v86);
    v30 = sub_1475BC430(v29, v2);
    v31 = (unsigned int *)v30;
    if ( v30 )
    {
      sub_14501C1F0(v57, *(unsigned int *)(v30 + 4));
      sub_14501C490(v57, v31[2]);
      LOBYTE(v35) = (int)v31[2] > 1;
      sub_14501C4E0(v57, v35);
      LOBYTE(v36) = (int)*v31 <= v22 && (unsigned __int8)sub_141F88620(v86, v2);
      (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v61 + 24LL))(v61, v36);
      if ( (int)*v31 <= v22 && !(unsigned __int8)sub_141F88620(v86, v2) )
      {
        if ( !(unsigned __int8)sub_141FB6530(v59) )
          (*(void (__fastcall **)(__int64))(*(_QWORD *)v59 + 432LL))(v59);
        LOBYTE(v37) = 1;
        (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v59 + 16LL))(v59, v37);
      }
      v38 = sub_141FC7BD0(v14, v81, v2 + 92);
      sub_1401E9D00(&v65, v38);
      v39 = v82;
      if ( v82 )
      {
        if ( _InterlockedExchangeAdd(v82 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v39)(v39);
          if ( _InterlockedExchangeAdd(v39 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v39 + 8LL))(v39);
        }
      }
      v40 = v65;
      v41 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v65 + 688LL);
      v42 = *v31;
      v43 = sub_146E8C7D0(&unk_1491CE3A0);
      v44 = sub_146E8CF20(v83, v43, v42);
      v45 = sub_14014F430(v44);
      v41(v40, v45);
      sub_146E8C910(v83);
      v46 = v66;
      if ( v66 )
      {
        if ( _InterlockedExchangeAdd(v66 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v46)(v46);
          if ( _InterlockedExchangeAdd(v46 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v46 + 8LL))(v46);
        }
      }
      v47 = v58;
      if ( v58 )
      {
        if ( _InterlockedExchangeAdd(v58 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v47)(v47);
          if ( _InterlockedExchangeAdd(v47 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v47 + 8LL))(v47);
        }
      }
      v48 = v60;
      if ( v60 )
      {
        if ( _InterlockedExchangeAdd(v60 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v48)(v48);
          if ( _InterlockedExchangeAdd(v48 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v48 + 8LL))(v48);
        }
      }
      v49 = v62;
      if ( v62 )
      {
        if ( _InterlockedExchangeAdd(v62 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v49)(v49);
          if ( _InterlockedExchangeAdd(v49 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v49 + 8LL))(v49);
        }
      }
      v22 = v85;
    }
    else
    {
      v32 = v58;
      if ( v58 )
      {
        if ( _InterlockedExchangeAdd(v58 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v32)(v32);
          if ( _InterlockedExchangeAdd(v32 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v32 + 8LL))(v32);
        }
      }
      v33 = v60;
      if ( v60 )
      {
        if ( _InterlockedExchangeAdd(v60 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v33)(v33);
          if ( _InterlockedExchangeAdd(v33 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v33 + 8LL))(v33);
        }
      }
      v34 = v62;
      if ( v62 )
      {
        if ( _InterlockedExchangeAdd(v62 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v34)(v34);
          if ( _InterlockedExchangeAdd(v34 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v34 + 8LL))(v34);
        }
      }
    }
    ++v2;
  }
  while ( (int)(v2 + 82) <= 91 );
  v50 = v68;
  if ( v68 )
  {
    if ( _InterlockedExchangeAdd(v68 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v50)(v50);
      if ( _InterlockedExchangeAdd(v50 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v50 + 8LL))(v50);
    }
  }
  v51 = v70;
  if ( v70 )
  {
    if ( _InterlockedExchangeAdd(v70 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v51)(v51);
      if ( _InterlockedExchangeAdd(v51 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v51 + 8LL))(v51);
    }
  }
  v52 = v56;
  v53 = *(__int64 **)(v56 + 8);
  if ( !*((_BYTE *)v53 + 25) )
  {
    do
    {
      sub_140150890(&v56, &v56, v53[2]);
      v54 = v53;
      v53 = (__int64 *)*v53;
      sub_146E9F3A0(v54, 40);
    }
    while ( !*((_BYTE *)v53 + 25) );
    v52 = v56;
  }
  return sub_146E9F3A0(v52, 40);
}

