// sender_2039_sub_1417E0AB0

char __fastcall sub_1417E0AB0(__int64 a1)
{
  __int64 v2; // rcx
  __int64 v3; // rax
  void (__fastcall ***v4)(_QWORD); // rcx
  __int64 v5; // rdx
  __int64 v6; // rcx
  __int64 v7; // r8
  __int64 v8; // rax
  __int64 v9; // rcx
  __int64 v10; // rdx
  __int64 v11; // rcx
  __int64 v12; // rax
  _QWORD *v13; // rax
  unsigned __int64 v14; // rdx
  __int64 v15; // rcx
  __int64 v16; // rax
  __int64 v17; // rcx
  __int64 v18; // rax
  __int16 *v19; // rdx
  __int64 v20; // rsi
  int v21; // ebx
  __int64 v22; // rcx
  __int64 v23; // rax
  __int64 v24; // rax
  unsigned __int8 v25; // si
  __int64 v26; // rcx
  __int64 v27; // rax
  __int64 v28; // rdx
  __int64 v29; // rax
  unsigned __int16 v30; // bx
  __int64 v31; // rcx
  __int64 v32; // rax
  __int64 v33; // rcx
  __int64 v34; // rax
  __int64 v35; // rdx
  __int64 v36; // rcx
  __int64 v37; // rax
  __int64 v38; // rdx
  __int64 v39; // rax
  __int64 v40; // rcx
  __int64 v41; // rax
  __int64 v42; // rdx
  __int64 v43; // rcx
  __int64 v44; // rcx
  __int64 v45; // rax
  __int64 v46; // rcx
  __int64 v47; // rax
  __int64 v48; // rcx
  __int64 v49; // rax
  _QWORD *v50; // rdx
  __int64 v51; // rcx
  __int64 v52; // rax
  __int64 v53; // rdx
  unsigned __int64 v54; // rdx
  __int64 v55; // rcx
  __int64 v57; // rax
  __int64 v58; // rdx
  __int128 v59; // [rsp+28h] [rbp-E0h] BYREF
  __int128 *v60; // [rsp+38h] [rbp-D0h]
  __int64 v61; // [rsp+40h] [rbp-C8h]
  _QWORD v62[4]; // [rsp+48h] [rbp-C0h] BYREF
  int v63; // [rsp+68h] [rbp-A0h]
  _QWORD v64[2]; // [rsp+E8h] [rbp-20h] BYREF
  __int64 v65; // [rsp+F8h] [rbp-10h]
  unsigned __int64 v66; // [rsp+100h] [rbp-8h]
  _QWORD v67[3]; // [rsp+108h] [rbp+0h] BYREF
  unsigned __int64 v68; // [rsp+120h] [rbp+18h]
  __int16 v69; // [rsp+128h] [rbp+20h] BYREF
  char v70; // [rsp+12Ah] [rbp+22h] BYREF

  v61 = -2;
  if ( !sub_145EFAFB0() )
    return 0;
  v2 = qword_14E634230;
  if ( !qword_14E634230 )
  {
    v3 = sub_146E8BA20(112);
    v60 = (__int128 *)v3;
    if ( v3 )
      v4 = (void (__fastcall ***)(_QWORD))sub_1403DE110(v3);
    else
      v4 = 0;
    qword_14E634230 = (__int64)v4;
    (**v4)(v4);
    v2 = qword_14E634230;
  }
  if ( (unsigned __int16)sub_1403F5B80(v2, 27) == 1 )
  {
    if ( qword_14E683C78 )
    {
      v9 = 38027;
      goto LABEL_49;
    }
LABEL_50:
    (*(void (__fastcall **)(_QWORD))(**(_QWORD **)(a1 + 56) + 688LL))(*(_QWORD *)(a1 + 56));
    sub_14558EBB0(*(_QWORD *)(a1 + 72), v58);
    return 0;
  }
  v8 = sub_1401DCCB0(v6, v5, v7);
  if ( (unsigned __int16)sub_1403F5B80(v8, 28) == 1 )
  {
    if ( qword_14E683C78 )
    {
      v9 = 38058;
LABEL_49:
      v57 = sub_14723C170(v9);
      sub_14668C520(qword_14E683C78, 2875, v57, 0);
      goto LABEL_50;
    }
    goto LABEL_50;
  }
  sub_148AA2510(&v69, 0, 2048);
  v12 = sub_1403F4B10(v11, v10);
  sub_145592DD0(v12, *(_QWORD *)(a1 + 88), 0);
  v13 = (_QWORD *)sub_1454E22F0(*(_QWORD *)(a1 + 88), v67);
  if ( v13[3] >= 8u )
    v13 = (_QWORD *)*v13;
  sub_148AB9D74(&v69, 1024, v13);
  if ( v68 >= 8 )
  {
    v14 = 2 * v68 + 2;
    v15 = v67[0];
    if ( v14 >= 0x1000 )
    {
      v14 = 2 * v68 + 41;
      v15 = *(_QWORD *)(v67[0] - 8LL);
      if ( (unsigned __int64)(v67[0] - v15 - 8) > 0x1F )
        sub_148AAF304(v15, v14);
    }
    sub_146E9F3A0(v15, v14);
  }
  v67[2] = 0;
  v68 = 7;
  LOWORD(v67[0]) = 0;
  v16 = -1;
  do
    ++v16;
  while ( *(&v69 + v16) );
  if ( !(_DWORD)v16 )
    return 0;
  v17 = (int)v16;
  if ( (int)v16 > 0 )
  {
    v18 = 0;
    do
    {
      if ( *(&v69 + v18) == 37 )
        *(&v69 + v18) = 32;
      ++v18;
    }
    while ( v18 < v17 );
  }
  v19 = &v69;
  if ( v69 == 127 )
    v19 = (__int16 *)&v70;
  if ( (unsigned __int8)sub_146D70A20(qword_14E682A28, v19) )
  {
    v20 = sub_14723C170(281);
    if ( v20 )
    {
      v21 = dword_14F1C0880;
      sub_1454E45F0(v62);
      v62[1] = v20;
      v62[2] = 0;
      v63 = v21;
      v60 = &v59;
      v59 = 0;
      v22 = *(_QWORD *)(a1 + 48);
      if ( v22 )
      {
        _InterlockedIncrement((volatile signed __int32 *)(v22 + 8));
        v22 = *(_QWORD *)(a1 + 48);
      }
      *(_QWORD *)&v59 = *(_QWORD *)(a1 + 40);
      *((_QWORD *)&v59 + 1) = v22;
      sub_1454E5660(&v59, v62);
    }
    goto LABEL_50;
  }
  v64[0] = 0;
  v65 = 0;
  v66 = 7;
  v23 = sub_145EFAFB0();
  v24 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v23 + 2704LL))(v23);
  sub_14014C8B0(v64, v24);
  v25 = 0;
  v26 = qword_14E683C08;
  if ( qword_14E683C08 && sub_145F0BA60(qword_14E683C08) )
  {
    v27 = sub_145F0BA60(qword_14E683C08);
    v25 = sub_144F02AF0(v27);
  }
  v28 = *(unsigned int *)(a1 + 232);
  if ( !(_DWORD)v28 )
  {
    v39 = sub_146D74000(v26);
    sub_146D746E0(v39, 17);
    v41 = sub_146D74000(v40);
    LOBYTE(v42) = 2;
    sub_146D75CC0(v41, v42);
    v37 = sub_146D74000(v43);
    v38 = 0;
    goto LABEL_39;
  }
  if ( (_DWORD)v28 == 1 )
  {
    v29 = sub_140D66E10();
    v30 = sub_140E3D9D0(v29, 0);
    v32 = sub_146D74000(v31);
    sub_146D746E0(v32, 17);
    v34 = sub_146D74000(v33);
    LOBYTE(v35) = 1;
    sub_146D75CC0(v34, v35);
    v37 = sub_146D74000(v36);
    v38 = v30;
LABEL_39:
    sub_146D76180(v37, v38);
    v45 = sub_146D74000(v44);
    sub_146D75CE0(v45, 0);
    v47 = sub_146D74000(v46);
    sub_146D76080(v47, &v69);
    v49 = sub_146D74000(v48);
    v50 = v64;
    if ( v66 >= 8 )
      v50 = (_QWORD *)v64[0];
    sub_146D76080(v49, v50);
    v52 = sub_146D74000(v51);
    sub_146D75CC0(v52, v25);
  }
  sub_146D75AF0(v26, v28);
  (*(void (__fastcall **)(_QWORD))(**(_QWORD **)(a1 + 56) + 688LL))(*(_QWORD *)(a1 + 56));
  sub_14558EBB0(*(_QWORD *)(a1 + 72), v53);
  if ( v66 >= 8 )
  {
    v54 = 2 * v66 + 2;
    v55 = v64[0];
    if ( v54 >= 0x1000 )
    {
      v54 = 2 * v66 + 41;
      v55 = *(_QWORD *)(v64[0] - 8LL);
      if ( (unsigned __int64)(v64[0] - v55 - 8) > 0x1F )
        sub_148AAF304(v55, v54);
    }
    sub_146E9F3A0(v55, v54);
  }
  v65 = 0;
  v66 = 7;
  LOWORD(v64[0]) = 0;
  return 1;
}

