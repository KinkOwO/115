// reader_sub_1444E2DD0

void __fastcall sub_1444E2DD0(__int64 a1, __int64 a2)
{
  __int64 v3; // rbx
  void (__fastcall *v4)(__int64, _QWORD, _QWORD); // rbp
  __int64 v5; // rax
  void (__fastcall ***v6)(_QWORD); // rcx
  unsigned int v7; // esi
  __int64 v8; // rcx
  __int64 v9; // rax
  void (__fastcall ***v10)(_QWORD); // rcx
  unsigned int v11; // eax
  __int64 v12; // rsi
  void (__fastcall *v13)(__int64, __int64); // r14
  __int64 v14; // rax
  void (__fastcall ***v15)(_QWORD); // rcx
  __int64 v16; // rcx
  __int64 v17; // rax
  void (__fastcall ***v18)(_QWORD); // rcx
  unsigned int v19; // ebx
  __int64 v20; // rax
  __int64 v21; // rax
  __int64 v22; // rax
  __int64 v23; // rsi
  void (__fastcall *v24)(__int64, __int64); // rbp
  __int64 v25; // rax
  void (__fastcall ***v26)(_QWORD); // rcx
  unsigned int v27; // ebx
  __int64 v28; // rax
  __int64 v29; // rax
  __int64 v30; // rax
  __int64 v31; // rcx
  __int64 v32; // rdx
  int v33; // eax
  __int64 v34; // rcx
  __int64 v35; // r8
  int v36; // eax
  __int64 v37; // rcx
  __int64 v38; // rdx
  int v39; // eax
  __int64 v40; // rcx
  int v41; // eax
  __int64 v42; // rax
  int v43; // eax
  __int64 v44; // rcx
  __int64 v45; // rax
  void (__fastcall ***v46)(_QWORD); // rcx
  int v47; // esi
  unsigned __int64 v48; // rbp
  _QWORD *v49; // rbx
  unsigned __int64 v50; // rdx
  __int64 v51; // rcx
  __int64 v52; // rdx
  __int64 v53; // rcx
  void (__fastcall *v54)(__int64, __int64); // r8
  __int64 v55; // rax
  void (__fastcall ***v56)(_QWORD); // rcx
  unsigned int v57; // eax
  __int64 v58; // rcx
  unsigned __int64 v59; // rdx
  _BYTE v60[16]; // [rsp+48h] [rbp-40h] BYREF
  __int128 v61; // [rsp+58h] [rbp-30h] BYREF
  __int64 v62; // [rsp+68h] [rbp-20h]
  __int64 v63; // [rsp+90h] [rbp+8h]

  v3 = *(_QWORD *)(a1 + 5192);
  if ( v3 )
  {
    v4 = *(void (__fastcall **)(__int64, _QWORD, _QWORD))(*(_QWORD *)v3 + 672LL);
    if ( !qword_14E664BF8 )
    {
      v5 = sub_146E8BA20(2792);
      if ( v5 )
        v6 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v5);
      else
        v6 = 0;
      qword_14E664BF8 = (__int64)v6;
      (**v6)(v6);
    }
    v7 = sub_1444D2C60();
    v8 = qword_14E664BF8;
    if ( !qword_14E664BF8 )
    {
      v9 = sub_146E8BA20(2792);
      if ( v9 )
        v10 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v9);
      else
        v10 = 0;
      qword_14E664BF8 = (__int64)v10;
      (**v10)(v10);
      v8 = qword_14E664BF8;
    }
    v11 = sub_1444D2B20(v8);
    v4(v3, v11, v7);
  }
  v12 = *(_QWORD *)(a1 + 5208);
  if ( v12 )
  {
    v13 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v12 + 688LL);
    if ( !qword_14E664BF8 )
    {
      v14 = sub_146E8BA20(2792);
      if ( v14 )
        v15 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v14);
      else
        v15 = 0;
      qword_14E664BF8 = (__int64)v15;
      (**v15)(v15);
    }
    sub_1444D2C60();
    v16 = qword_14E664BF8;
    if ( !qword_14E664BF8 )
    {
      v17 = sub_146E8BA20(2792);
      if ( v17 )
        v18 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v17);
      else
        v18 = 0;
      qword_14E664BF8 = (__int64)v18;
      (**v18)(v18);
      v16 = qword_14E664BF8;
    }
    v19 = sub_1444D2B20(v16);
    v20 = sub_14723C170(27075);
    v21 = sub_146E8CF20(v60, v20, v19);
    v22 = sub_14014F430(v21);
    v13(v12, v22);
    sub_146E8C910(v60);
  }
  v23 = *(_QWORD *)(a1 + 5224);
  if ( v23 )
  {
    v24 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v23 + 688LL);
    if ( !qword_14E664BF8 )
    {
      v25 = sub_146E8BA20(2792);
      if ( v25 )
        v26 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v25);
      else
        v26 = 0;
      qword_14E664BF8 = (__int64)v26;
      (**v26)(v26);
    }
    v27 = sub_1444D2B80();
    v28 = sub_146E8C7D0(&unk_1491CE3A0);
    v29 = sub_146E8CF20(v60, v28, v27);
    v30 = sub_14014F430(v29);
    v24(v23, v30);
    sub_146E8C910(v60);
  }
  v31 = *(_QWORD *)(a1 + 5240);
  if ( v31 && sub_146ECFD90(v31) && !(unsigned __int8)sub_141FB6530(*(_QWORD *)(a1 + 5544)) )
  {
    if ( (unsigned __int8)sub_1444DDCF0(a1, *(_DWORD *)(a1 + 1516) == 3) )
    {
      LOBYTE(v32) = 1;
      (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 5544) + 16LL))(*(_QWORD *)(a1 + 5544), v32);
    }
    v33 = sub_146E8C7D0(&unk_14A315E38);
    sub_145A31380(v33, -1, 0, 0, -1, -1, 0);
  }
  v34 = *(_QWORD *)(a1 + 5256);
  if ( v34 && sub_146ECFD90(v34) )
  {
    LOBYTE(v35) = 1;
    if ( !(unsigned __int8)sub_146682140(qword_14E683C78, 3335, v35) )
      sub_14668C520(qword_14E683C78, 3335, 0, 0);
    v36 = sub_146E8C7D0(&unk_14A315E38);
    sub_145A31380(v36, -1, 0, 0, -1, -1, 0);
  }
  v37 = *(_QWORD *)(a1 + 5272);
  if ( v37 )
  {
    if ( *(_DWORD *)(a1 + 1516) == 2 )
      LOBYTE(a2) = 1;
    else
      a2 = 0;
    (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v37 + 24LL))(v37, a2);
    if ( *(_QWORD *)(a1 + 5512) )
    {
      if ( sub_146ECFD90(*(_QWORD *)(a1 + 5272)) )
      {
        if ( *(_DWORD *)(a1 + 1516) == 2 )
        {
          LOBYTE(v38) = 1;
          (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 5512) + 16LL))(*(_QWORD *)(a1 + 5512), v38);
        }
        v39 = sub_146E8C7D0(&unk_14A315E38);
        sub_145A31380(v39, -1, 0, 0, -1, -1, 0);
      }
    }
  }
  v40 = *(_QWORD *)(a1 + 5528);
  if ( v40 && *(_QWORD *)(a1 + 5512) && sub_146ECFD90(v40) )
  {
    (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 5512) + 16LL))(*(_QWORD *)(a1 + 5512), 0);
    v41 = sub_146E8C7D0(&unk_14A315E38);
    sub_145A31380(v41, -1, 0, 0, -1, -1, 0);
  }
  if ( *(_QWORD *)(a1 + 5288) && sub_145EFAFB0() )
  {
    v42 = sub_145EFAFB0();
    v43 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v42 + 4832LL))(v42);
    sub_146EECBB0(*(_QWORD *)(a1 + 5288), (unsigned int)(v43 + 28));
  }
  v44 = qword_14E664BF8;
  if ( !qword_14E664BF8 )
  {
    v45 = sub_146E8BA20(2792);
    if ( v45 )
      v46 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v45);
    else
      v46 = 0;
    qword_14E664BF8 = (__int64)v46;
    (**v46)(v46);
    v44 = qword_14E664BF8;
  }
  sub_1444D2D60(v44, &v61);
  v47 = 0;
  v48 = 0;
  v49 = (_QWORD *)(a1 + 5304);
  do
  {
    v50 = (__int64)(*((_QWORD *)&v61 + 1) - v61) >> 3;
    if ( v47 >= v50 )
      break;
    v51 = *v49;
    if ( *v49 && v49[14] )
    {
      if ( v50 <= v48 )
        sub_1401790B0(v51, v50);
      v63 = *(_QWORD *)(v61 + 8 * v48);
      sub_14501C1F0(v51, (unsigned int)v63);
      sub_14501C490(*v49, HIDWORD(v63));
      v53 = v49[14];
      v54 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v53 + 16LL);
      if ( *(_DWORD *)(a1 + 1516) == 3 )
      {
        v54(v53, 0);
        if ( (unsigned __int8)sub_146ED01B0(*v49) )
          sub_1444E5E80(a1, (unsigned int)v47);
      }
      else
      {
        LOBYTE(v52) = 1;
        v54(v53, v52);
      }
    }
    ++v47;
    ++v48;
    v49 += 2;
  }
  while ( v47 < 6 );
  if ( *(_QWORD *)(a1 + 5400) )
  {
    if ( !qword_14E664BF8 )
    {
      v55 = sub_146E8BA20(2792);
      if ( v55 )
        v56 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v55);
      else
        v56 = 0;
      qword_14E664BF8 = (__int64)v56;
      (**v56)(v56);
    }
    v57 = sub_1444D2E10();
    sub_146FB9AE0(*(_QWORD *)(a1 + 5400), v57);
  }
  v58 = v61;
  if ( (_QWORD)v61 )
  {
    v59 = (v62 - v61) & 0xFFFFFFFFFFFFFFF8uLL;
    if ( v59 >= 0x1000 )
    {
      v59 += 39LL;
      v58 = *(_QWORD *)(v61 - 8);
      if ( (unsigned __int64)(v61 - v58 - 8) > 0x1F )
        sub_148AAF304(v58, v59);
    }
    sub_146E9F3A0(v58, v59);
    v61 = 0;
    v62 = 0;
  }
}

