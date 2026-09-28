// sender_1551_sub_1429C6B20

char __fastcall sub_1429C6B20(_QWORD *a1)
{
  __int64 v2; // rcx
  __int64 v3; // rax
  void (__fastcall ***v4)(_QWORD); // rcx
  __int64 v5; // rdx
  __int64 v6; // rcx
  __int64 v7; // rax
  void (__fastcall ***v8)(_QWORD); // rcx
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
  __int64 v20; // rax
  __int64 v21; // rax
  __int64 v22; // rax
  unsigned __int8 v23; // di
  __int64 v24; // rcx
  __int64 v25; // rax
  __int64 v26; // rax
  __int64 v27; // rcx
  __int64 v28; // rax
  __int64 v29; // rdx
  __int64 v30; // rcx
  __int64 v31; // rax
  __int64 v32; // rcx
  __int64 v33; // rax
  __int64 v34; // rcx
  __int64 v35; // rax
  __int64 v36; // rcx
  __int64 v37; // rax
  _QWORD *v38; // rdx
  __int64 v39; // rcx
  __int64 v40; // rax
  __int64 v41; // rdx
  __int64 v42; // rcx
  __int64 v43; // rdx
  __int64 v44; // rcx
  __int64 v45; // rcx
  unsigned __int64 v46; // rdx
  __int64 v47; // rcx
  __int64 v49; // rax
  __int64 v50; // rcx
  __int64 v51; // rcx
  __int64 v52; // [rsp+28h] [rbp-E0h]
  _QWORD v53[2]; // [rsp+38h] [rbp-D0h] BYREF
  __int64 v54; // [rsp+48h] [rbp-C0h]
  unsigned __int64 v55; // [rsp+50h] [rbp-B8h]
  _QWORD v56[3]; // [rsp+58h] [rbp-B0h] BYREF
  unsigned __int64 v57; // [rsp+70h] [rbp-98h]
  __int16 v58; // [rsp+78h] [rbp-90h] BYREF
  char v59; // [rsp+7Ah] [rbp-8Eh] BYREF

  v2 = qword_14E634230;
  if ( !qword_14E634230 )
  {
    v3 = sub_146E8BA20(112);
    v52 = v3;
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
    v50 = a1[404];
    if ( v50 )
      (*(void (__fastcall **)(__int64))(*(_QWORD *)v50 + 688LL))(v50);
    v51 = a1[397];
    if ( v51 )
      sub_14558EBB0(v51, v5);
    return 0;
  }
  v6 = qword_14E634230;
  if ( !qword_14E634230 )
  {
    v7 = sub_146E8BA20(112);
    v52 = v7;
    if ( v7 )
      v8 = (void (__fastcall ***)(_QWORD))sub_1403DE110(v7);
    else
      v8 = 0;
    qword_14E634230 = (__int64)v8;
    (**v8)(v8);
    v6 = qword_14E634230;
  }
  if ( (unsigned __int16)sub_1403F5B80(v6, 28) == 1 )
  {
    if ( qword_14E683C78 )
    {
      v9 = 38058;
LABEL_49:
      v49 = sub_14723C170(v9);
      sub_14668C520(qword_14E683C78, 2875, v49, 0);
      goto LABEL_50;
    }
    goto LABEL_50;
  }
  sub_148AA2510(&v58, 0, 2048);
  v12 = sub_1403F4B10(v11, v10);
  sub_145592DD0(v12, a1[399], 0);
  v13 = (_QWORD *)sub_1454E22F0(a1[399], v56);
  if ( v13[3] >= 8u )
    v13 = (_QWORD *)*v13;
  sub_148AB9D74(&v58, 1024, v13);
  if ( v57 >= 8 )
  {
    v14 = 2 * v57 + 2;
    v15 = v56[0];
    if ( v14 >= 0x1000 )
    {
      v14 = 2 * v57 + 41;
      v15 = *(_QWORD *)(v56[0] - 8LL);
      if ( (unsigned __int64)(v56[0] - v15 - 8) > 0x1F )
        sub_148AAF304(v15, v14);
    }
    sub_146E9F3A0(v15, v14);
  }
  v56[2] = 0;
  v57 = 7;
  LOWORD(v56[0]) = 0;
  v16 = -1;
  do
    ++v16;
  while ( *(&v58 + v16) );
  if ( !(_DWORD)v16 )
    return 0;
  v17 = (int)v16;
  if ( (int)v16 > 0 )
  {
    v18 = 0;
    do
    {
      if ( *(&v58 + v18) == 37 )
        *(&v58 + v18) = 32;
      ++v18;
    }
    while ( v18 < v17 );
  }
  v19 = &v58;
  if ( v58 == 127 )
    v19 = (__int16 *)&v59;
  if ( (unsigned __int8)sub_146D70A20(qword_14E682A28, v19) )
  {
    v20 = sub_14723C170(281);
    sub_1429C2060(a1, v20, 0, 0, v52, -2);
    goto LABEL_50;
  }
  v53[0] = 0;
  v54 = 0;
  v55 = 7;
  v21 = sub_145EFAFB0();
  v22 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v21 + 2704LL))(v21);
  sub_14014C8B0(v53, v22);
  v23 = 0;
  v24 = qword_14E683C08;
  if ( qword_14E683C08 && sub_145F0BA60(qword_14E683C08) )
  {
    v25 = sub_145F0BA60(qword_14E683C08);
    v23 = sub_144F02AF0(v25);
  }
  v26 = sub_146D74000(v24);
  sub_146D746E0(v26, 17);
  v28 = sub_146D74000(v27);
  LOBYTE(v29) = 75;
  sub_146D75CC0(v28, v29);
  v31 = sub_146D74000(v30);
  sub_146D76180(v31, 0);
  v33 = sub_146D74000(v32);
  sub_146D75CE0(v33, 0);
  v35 = sub_146D74000(v34);
  sub_146D76080(v35, &v58);
  v37 = sub_146D74000(v36);
  v38 = v53;
  if ( v55 >= 8 )
    v38 = (_QWORD *)v53[0];
  sub_146D76080(v37, v38);
  v40 = sub_146D74000(v39);
  sub_146D75CC0(v40, v23);
  sub_146D75AF0(v42, v41);
  v44 = a1[404];
  if ( v44 )
    (*(void (__fastcall **)(__int64))(*(_QWORD *)v44 + 688LL))(v44);
  v45 = a1[397];
  if ( v45 )
    sub_14558EBB0(v45, v43);
  if ( v55 >= 8 )
  {
    v46 = 2 * v55 + 2;
    v47 = v53[0];
    if ( v46 >= 0x1000 )
    {
      v46 = 2 * v55 + 41;
      v47 = *(_QWORD *)(v53[0] - 8LL);
      if ( (unsigned __int64)(v53[0] - v47 - 8) > 0x1F )
        sub_148AAF304(v47, v46);
    }
    sub_146E9F3A0(v47, v46);
  }
  v54 = 0;
  v55 = 7;
  LOWORD(v53[0]) = 0;
  return 1;
}

