// sender_1551_sub_14242BAC0

double __fastcall sub_14242BAC0(__int64 a1, int a2)
{
  int v4; // r12d
  int v5; // r15d
  __int64 v6; // r14
  unsigned int v7; // ebx
  __int64 v8; // rsi
  __int64 v9; // rdx
  __int64 v10; // rdx
  __int64 v11; // r15
  __int64 v12; // rcx
  __int64 v13; // rax
  __int64 v14; // rax
  __int64 v15; // rsi
  __int64 v16; // r14
  __int64 v17; // r15
  __int64 v18; // r15
  __int64 v19; // rcx
  __int64 v20; // rax
  __int64 v21; // rax
  __int64 v22; // r15
  __int64 v23; // rcx
  __int64 v24; // rax
  __int64 v25; // rdx
  __int64 v26; // rdx
  __int64 v27; // rbx
  __int64 v28; // r12
  __int64 v29; // rax
  __int64 v30; // rdx
  __int64 v31; // r15
  double result; // xmm0_8
  int v33; // eax
  __int64 v34; // rsi
  __int64 v35; // rax
  unsigned __int64 v36; // rcx
  __int64 v37; // rbx
  __int64 v38; // rdx
  __int64 v39; // rax
  __int64 v40; // rsi
  __int64 v41; // rbx
  __int64 v42; // rdx
  __int64 v43; // rcx
  __int64 v44; // rdx
  unsigned int v45; // ebx
  unsigned int v46; // edi
  __int64 v47; // rcx
  __int64 v48; // rax
  __int64 v49; // rcx
  __int64 v50; // rax
  __int64 v51; // rcx
  __int64 v52; // rax
  __int64 v53; // rcx
  __int64 v54; // rax
  __int64 v55; // rcx
  __int64 v56; // rax
  __int64 v57; // rdx
  __int64 v58; // rcx
  unsigned __int64 v59; // rdx
  __int64 v60; // rcx
  __int128 v61; // [rsp+50h] [rbp-B0h]
  int v62; // [rsp+60h] [rbp-A0h]
  __int128 v63; // [rsp+70h] [rbp-90h]
  __int128 v64; // [rsp+80h] [rbp-80h] BYREF
  __int64 v65; // [rsp+90h] [rbp-70h]
  __int128 v66; // [rsp+98h] [rbp-68h]
  __int128 v67; // [rsp+A8h] [rbp-58h]
  __int128 v68; // [rsp+B8h] [rbp-48h]
  __int128 v69; // [rsp+C8h] [rbp-38h]
  __int128 v70; // [rsp+D8h] [rbp-28h]
  __int128 v71; // [rsp+E8h] [rbp-18h]
  __int128 v72; // [rsp+F8h] [rbp-8h]
  __int128 v73; // [rsp+108h] [rbp+8h]
  __int128 v74; // [rsp+118h] [rbp+18h]
  _QWORD v75[4]; // [rsp+128h] [rbp+28h] BYREF
  __int128 v76; // [rsp+148h] [rbp+48h] BYREF
  __int64 v77; // [rsp+158h] [rbp+58h]
  __int64 v78; // [rsp+160h] [rbp+60h] BYREF
  __int128 v79; // [rsp+168h] [rbp+68h]
  __int128 v80; // [rsp+178h] [rbp+78h]
  int v81; // [rsp+188h] [rbp+88h]
  int v82; // [rsp+18Ch] [rbp+8Ch]
  int v83; // [rsp+190h] [rbp+90h]
  __int128 v84; // [rsp+198h] [rbp+98h]
  __int128 v85; // [rsp+1A8h] [rbp+A8h]
  int v86; // [rsp+1B8h] [rbp+B8h]
  int v87; // [rsp+1BCh] [rbp+BCh]
  __int128 v88; // [rsp+1C0h] [rbp+C0h]
  __int64 v89; // [rsp+1D0h] [rbp+D0h]
  __int64 v90; // [rsp+1D8h] [rbp+D8h]
  _QWORD v91[8]; // [rsp+1E0h] [rbp+E0h] BYREF
  _QWORD v92[8]; // [rsp+220h] [rbp+120h] BYREF
  _QWORD v93[8]; // [rsp+260h] [rbp+160h] BYREF
  _BYTE v94[56]; // [rsp+2A0h] [rbp+1A0h] BYREF
  __int64 v95; // [rsp+2D8h] [rbp+1D8h]
  __int64 v96; // [rsp+2E0h] [rbp+1E0h]
  __int128 v97; // [rsp+2E8h] [rbp+1E8h] BYREF
  __int128 v98; // [rsp+2F8h] [rbp+1F8h]
  int v99; // [rsp+308h] [rbp+208h]
  int v100; // [rsp+30Ch] [rbp+20Ch]
  int v101; // [rsp+310h] [rbp+210h]
  __int128 v102; // [rsp+318h] [rbp+218h] BYREF
  __int128 v103; // [rsp+328h] [rbp+228h]
  int v104; // [rsp+338h] [rbp+238h]
  int v105; // [rsp+33Ch] [rbp+23Ch]
  __int128 v106; // [rsp+340h] [rbp+240h]
  __int64 v107; // [rsp+350h] [rbp+250h]

  v90 = -2;
  if ( !*(_QWORD *)(a1 + 1536) || !*(_QWORD *)(a1 + 1512) )
    return result;
  v4 = sub_146EE4DA0(*(_QWORD *)(a1 + 3896));
  v63 = 0;
  v61 = 0;
  v69 = 0;
  v5 = 0;
  v6 = 0;
  v7 = v4 + 1;
  v8 = a1 + 2216;
  while ( *(_DWORD *)(*(_QWORD *)(v8 - 112) + 336LL) == a2 )
  {
    if ( sub_14748CB30(*(_QWORD *)(a1 + 1512), 0, v7) )
    {
      v10 = 0;
      v18 = 320LL * v5;
      v68 = 0;
      v19 = *(_QWORD *)(v18 + a1 + 2160);
      if ( v19 )
      {
        _InterlockedIncrement((volatile signed __int32 *)(v19 + 8));
        v19 = *(_QWORD *)(v18 + a1 + 2160);
      }
      v20 = *(_QWORD *)(v18 + a1 + 2152);
      v68 = 0u;
      *(_QWORD *)&v63 = v20;
      *((_QWORD *)&v63 + 1) = v19;
      v67 = 0;
      v21 = *(_QWORD *)(v18 + a1 + 2176);
      if ( v21 )
      {
        _InterlockedIncrement((volatile signed __int32 *)(v21 + 8));
        v21 = *(_QWORD *)(v18 + a1 + 2176);
      }
      v15 = *(_QWORD *)(v18 + a1 + 2168);
      v67 = 0u;
      *((_QWORD *)&v61 + 1) = v21;
      v66 = 0;
      v16 = *(_QWORD *)(v18 + a1 + 2192);
      if ( v16 )
      {
        _InterlockedIncrement((volatile signed __int32 *)(v16 + 8));
        v16 = *(_QWORD *)(v18 + a1 + 2192);
      }
      v17 = *(_QWORD *)(v18 + a1 + 2184);
      v66 = 0u;
      *(_QWORD *)&v69 = v17;
      *((_QWORD *)&v69 + 1) = v16;
      goto LABEL_31;
    }
    LOBYTE(v9) = 2;
    if ( sub_14748CB30(*(_QWORD *)(a1 + 1512), v9, v7) )
    {
      v10 = 2;
      v11 = 320LL * v5;
      v72 = 0;
      v12 = *(_QWORD *)(v11 + a1 + 2160);
      if ( v12 )
      {
        _InterlockedIncrement((volatile signed __int32 *)(v12 + 8));
        v12 = *(_QWORD *)(v11 + a1 + 2160);
      }
      v13 = *(_QWORD *)(v11 + a1 + 2152);
      v72 = 0u;
      *(_QWORD *)&v63 = v13;
      *((_QWORD *)&v63 + 1) = v12;
      v71 = 0;
      v14 = *(_QWORD *)(v11 + a1 + 2176);
      if ( v14 )
      {
        _InterlockedIncrement((volatile signed __int32 *)(v14 + 8));
        v14 = *(_QWORD *)(v11 + a1 + 2176);
      }
      v15 = *(_QWORD *)(v11 + a1 + 2168);
      v71 = 0u;
      *((_QWORD *)&v61 + 1) = v14;
      v70 = 0;
      v16 = *(_QWORD *)(v11 + a1 + 2192);
      if ( v16 )
      {
        _InterlockedIncrement((volatile signed __int32 *)(v16 + 8));
        v16 = *(_QWORD *)(v11 + a1 + 2192);
      }
      v17 = *(_QWORD *)(v11 + a1 + 2184);
      v70 = 0u;
      *(_QWORD *)&v69 = v17;
      *((_QWORD *)&v69 + 1) = v16;
      goto LABEL_31;
    }
LABEL_15:
    ++v5;
    ++v7;
    ++v6;
    v8 += 320;
    if ( v6 >= 5 )
    {
      v16 = *((_QWORD *)&v69 + 1);
      goto LABEL_35;
    }
  }
  if ( *(_DWORD *)(*(_QWORD *)v8 + 336LL) != a2 )
    goto LABEL_15;
  v10 = 1;
  v7 = v5 + v4 + 1;
  v22 = 320LL * v5;
  v23 = *(_QWORD *)(v22 + a1 + 2272);
  if ( v23 )
  {
    _InterlockedIncrement((volatile signed __int32 *)(v23 + 8));
    v23 = *(_QWORD *)(v22 + a1 + 2272);
  }
  *(_QWORD *)&v63 = *(_QWORD *)(v22 + a1 + 2264);
  *((_QWORD *)&v63 + 1) = v23;
  v73 = 0;
  v24 = *(_QWORD *)(v22 + a1 + 2288);
  if ( v24 )
  {
    _InterlockedIncrement((volatile signed __int32 *)(v24 + 8));
    v24 = *(_QWORD *)(v22 + a1 + 2288);
  }
  v15 = *(_QWORD *)(v22 + a1 + 2280);
  v73 = 0u;
  *((_QWORD *)&v61 + 1) = v24;
  v74 = 0;
  v16 = *(_QWORD *)(v22 + a1 + 2304);
  if ( v16 )
  {
    _InterlockedIncrement((volatile signed __int32 *)(v16 + 8));
    v16 = *(_QWORD *)(v22 + a1 + 2304);
  }
  v17 = *(_QWORD *)(v22 + a1 + 2296);
  v74 = 0u;
  *(_QWORD *)&v69 = v17;
  *((_QWORD *)&v69 + 1) = v16;
LABEL_31:
  sub_14242DCC0(a1, v10, v7);
  if ( (_QWORD)v63 && v15 && v17 )
  {
    LOBYTE(v25) = 1;
    (*(void (__fastcall **)(_QWORD, __int64))(*(_QWORD *)v63 + 16LL))(v63, v25);
    (*(void (__fastcall **)(_QWORD))(*(_QWORD *)v63 + 432LL))(v63);
    (*(void (__fastcall **)(__int64))(*(_QWORD *)v15 + 432LL))(v15);
    LOBYTE(v26) = 1;
    (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v17 + 16LL))(v17, v26);
    (*(void (__fastcall **)(__int64))(*(_QWORD *)v17 + 432LL))(v17);
  }
LABEL_35:
  if ( *(_DWORD *)(*(_QWORD *)(a1 + 1688) + 336LL) == a2 )
  {
    sub_146670390(qword_14E683C78);
    *(_QWORD *)&v68 = &v78;
    v27 = -1;
    v96 = -1;
    v28 = 0;
    *(_QWORD *)&v97 = 0;
    *(_QWORD *)&v98 = 0;
    *((_QWORD *)&v98 + 1) = 7;
    sub_14014C8D0(&v97, &byte_14BAF7F08);
    v99 = 2;
    v100 = dword_14E64D628;
    v101 = dword_14E64D62C;
    *(_QWORD *)&v102 = 0;
    *(_QWORD *)&v103 = 0;
    *((_QWORD *)&v103 + 1) = 7;
    sub_14014C8D0(&v102, &byte_14BAF7F08);
    v104 = 134;
    v105 = 1;
    v106 = 0;
    v107 = 0;
    v78 = v96;
    v79 = v97;
    v80 = v98;
    *(_QWORD *)&v98 = 0;
    *((_QWORD *)&v98 + 1) = 7;
    LOWORD(v97) = 0;
    v81 = v99;
    v82 = v100;
    v83 = v101;
    v84 = v102;
    v85 = v103;
    *(_QWORD *)&v103 = 0;
    *((_QWORD *)&v103 + 1) = 7;
    LOWORD(v102) = 0;
    v86 = 134;
    v87 = 1;
    v88 = 0;
    v89 = 0;
    *(_QWORD *)&v67 = v91;
    v91[0] = std::_Func_impl_no_alloc<`NewCharacterPassVer2Window::onClickRewardButton'::`32'::_lambda_2_,control_event::XW4TYPE::Z::AXH &,__int64,__int64>::`vftable';
    v91[7] = v91;
    *(_QWORD *)&v66 = v94;
    v95 = 0;
    v92[0] = std::_Func_impl_no_alloc<`NewCharacterPassVer2Window::onClickRewardButton'::`32'::_lambda_2_,control_event::XW4TYPE::Z::AXH &,__int64,__int64>::`vftable';
    v92[7] = v92;
    *(_QWORD *)&v70 = &v76;
    v76 = 0;
    v77 = 0;
    *(_QWORD *)&v71 = v93;
    v93[0] = off_1499B07E0;
    v93[7] = v93;
    *(_QWORD *)&v72 = v75;
    v29 = sub_14723C170(39501);
    v75[0] = 0;
    v75[2] = 0;
    v75[3] = 7;
    do
      ++v27;
    while ( *(_WORD *)(v29 + 2 * v27) );
    sub_14014C8D0(v75, v29);
    sub_14668DCB0(
      qword_14E683C78,
      12,
      (unsigned int)v75,
      (unsigned int)v93,
      (__int64)&v76,
      (__int64)v92,
      (__int64)v94,
      (__int64)v91,
      (__int64)&v78);
    *(_QWORD *)&v103 = 0;
    *((_QWORD *)&v103 + 1) = 7;
    LOWORD(v102) = 0;
    *(_QWORD *)&v98 = 0;
    *((_QWORD *)&v98 + 1) = 7;
    LOWORD(v97) = 0;
    *(_BYTE *)(a1 + 5240) = 1;
    sub_1467A9F20(a1, 0);
    LOBYTE(v30) = 1;
    sub_1466961D0(qword_14E683C78, v30);
    sub_146E9FBD0(a1 + 5248, 0, 0);
  }
  else
  {
    v28 = 0;
  }
  v31 = (int)sub_146EE4DA0(*(_QWORD *)(a1 + 4024));
  result = 0.0;
  v64 = 0;
  v65 = 0;
  v33 = *(_DWORD *)(a1 + 1532);
  if ( v33 )
  {
    if ( v33 != 1 )
    {
      v34 = 0;
      goto LABEL_59;
    }
    v35 = sub_140E7FC30(*(_QWORD *)(a1 + 1536));
    sub_142426620(&v64, v35);
LABEL_44:
    v34 = v64;
  }
  else
  {
    v39 = sub_14017AEC0(*(_QWORD *)(a1 + 1536));
    if ( &v64 == (__int128 *)v39 )
      goto LABEL_44;
    v40 = *(_QWORD *)v39;
    v41 = *(_QWORD *)(v39 + 8) - *(_QWORD *)v39;
    if ( v41 >> 4 )
      sub_14032D190(&v64);
    v42 = v40;
    v34 = v64;
    result = sub_148AA1E60(v64, v42, v41);
    *((_QWORD *)&v64 + 1) = v34 + v41;
  }
  v36 = (*((_QWORD *)&v64 + 1) - v34) >> 4;
  *(_QWORD *)&v66 = v36;
  v62 = 0;
  v37 = a1 + 4712;
  v38 = v34 + 16 * v31 - a1 - 4712;
  *(_QWORD *)&v67 = v38;
  while ( (int)v31 < v36 )
  {
    *(_QWORD *)&v68 = sub_14748A8B0(
                        *(_QWORD *)(a1 + 1512),
                        *(unsigned __int8 *)(v38 + v37),
                        *(unsigned __int8 *)(a1 + 1532));
    if ( (_QWORD)v68 && *(_DWORD *)(*(_QWORD *)v37 + 336LL) == a2 && *(int *)(a1 + 1532) <= 1 )
    {
      v43 = *(_QWORD *)(a1 + 16LL * v62 + 4136);
      (*(void (__fastcall **)(__int64))(*(_QWORD *)v43 + 432LL))(v43);
      LOBYTE(v44) = 1;
      (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 16LL * v62 + 4136) + 16LL))(
        *(_QWORD *)(a1 + 16LL * v62 + 4136),
        v44);
      v45 = *(unsigned __int8 *)v68;
      v46 = *(_DWORD *)(a1 + 1532);
      if ( (dword_14E6527CC & 2) != 0 && (unsigned __int8)sub_146E9FA80(&unk_14E6527C8) == 1 )
      {
        v48 = sub_146D74000(v47);
        sub_146D746E0(v48, 681);
        v50 = sub_146D74000(v49);
        sub_146D75CE0(v50, 802);
        v52 = sub_146D74000(v51);
        sub_146D75CE0(v52, 4);
        v54 = sub_146D74000(v53);
        sub_146D75CE0(v54, v46);
        v56 = sub_146D74000(v55);
        sub_146D75CE0(v56, v45);
        sub_146D75AF0(v58, v57);
        sub_146E9FF10(&unk_14E6527C8, 1000, 0);
      }
      break;
    }
    ++v62;
    LODWORD(v31) = v31 + 1;
    ++v28;
    v37 += 16;
    if ( v28 >= 6 )
      break;
    v36 = v66;
    v38 = v67;
  }
LABEL_59:
  if ( v34 )
  {
    v59 = (v65 - v34) & 0xFFFFFFFFFFFFFFF0uLL;
    if ( v59 >= 0x1000 )
    {
      v59 += 39LL;
      v60 = *(_QWORD *)(v34 - 8);
      if ( (unsigned __int64)(v34 - v60 - 8) > 0x1F )
        sub_148AAF304(v60, v59);
      v34 = *(_QWORD *)(v34 - 8);
    }
    sub_146E9F3A0(v34, v59);
    result = 0.0;
    v64 = 0;
    v65 = 0;
  }
  if ( v16 )
    result = sub_1401DC510(v16);
  if ( *((_QWORD *)&v61 + 1) )
    result = sub_1401DC510(*((_QWORD *)&v61 + 1));
  if ( *((_QWORD *)&v63 + 1) )
    return sub_1401DC510(*((_QWORD *)&v63 + 1));
  return result;
}

