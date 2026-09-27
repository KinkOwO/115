// holder_sub_145E9FD50_0x145e9fd50

__int64 __fastcall sub_145E9FD50(__int64 a1)
{
  __int64 v2; // rcx
  __int64 v3; // rax
  __int64 v4; // rax
  __int64 v5; // rcx
  __int64 v6; // rcx
  __int64 v7; // rax
  __int64 v8; // rcx
  __int64 v9; // rax
  __int64 v10; // rcx
  __int64 v11; // rax
  void (__fastcall ***v12)(_QWORD); // rcx
  __int64 v13; // rax
  __int64 v14; // rcx
  __int64 v15; // rax
  __int64 v16; // rax
  __int64 v17; // rcx
  int v18; // r15d
  unsigned int i; // r14d
  __int64 v20; // rcx
  __int64 v21; // rcx
  __int64 v22; // rax
  __int64 v23; // rbx
  __int64 v24; // rax
  __int64 v25; // rbp
  __int64 v26; // rcx
  __int64 v27; // rax
  void (__fastcall ***v28)(_QWORD); // rcx
  unsigned int v29; // ebx
  unsigned int v30; // esi
  __int64 v31; // rcx
  __int64 v32; // rax
  void (__fastcall ***v33)(_QWORD); // rcx
  __int64 v34; // rax
  __int64 v35; // rax
  __int64 v36; // rsi
  unsigned int j; // ebx
  __int64 v38; // rax
  __int64 v39; // rcx
  __int64 v40; // rax
  __int64 v41; // rcx
  __int64 v42; // rax
  __int64 v43; // rcx
  __int64 v44; // rdx
  __int64 v45; // rcx
  __int64 v46; // rax
  __int64 v47; // rax
  void (__fastcall ***v48)(_QWORD); // rcx
  __int64 v49; // rcx
  __int64 v50; // rbx
  __int64 v51; // rax
  __int64 v52; // rax
  __int64 v53; // rax
  __int64 v54; // rax
  __int64 v55; // rbx
  __int64 v56; // rax
  void (__fastcall ***v57)(_QWORD); // rcx
  __int64 v58; // rax
  __int64 v59; // rax
  __int64 v60; // rcx
  __int64 v61; // rcx
  _QWORD *v62; // rax
  _QWORD *v63; // rcx
  __int64 v64; // rcx
  __int64 v65; // rax
  void (__fastcall ***v66)(_QWORD); // rcx
  __int64 v67; // rax
  __int64 v68; // rbx
  volatile signed __int32 *v69; // rcx
  unsigned int *v70; // rsi
  unsigned int *v71; // rbp
  unsigned int v72; // ebx
  __int64 v73; // rax
  __int64 v74; // rax
  void (__fastcall ***v75)(_QWORD); // rcx
  __int64 v76; // rax
  __int64 result; // rax
  __int64 v78; // rcx
  __int64 v79; // [rsp+20h] [rbp-38h]
  _QWORD *v80; // [rsp+60h] [rbp+8h] BYREF
  __int64 v81; // [rsp+68h] [rbp+10h]

  v79 = -2;
  if ( sub_145EFAFB0() )
  {
    v3 = sub_145EFAFB0();
    if ( !(unsigned __int8)sub_145CF0200(v3) )
    {
      v4 = sub_145EFAFB0();
      (*(void (__fastcall **)(__int64))(*(_QWORD *)v4 + 7616LL))(v4);
    }
  }
  if ( (unsigned __int8)sub_145F0C980(v2) )
  {
    v5 = *(_QWORD *)(a1 + 280);
    if ( v5 )
    {
      if ( (unsigned __int8)sub_145B35690(v5) )
      {
        v7 = sub_140AD4570(v6);
        if ( (unsigned __int8)sub_142E60CE0(v7) )
        {
          v9 = sub_140AD4570(v8);
          sub_142E63E20(v9);
        }
      }
    }
  }
  *(_QWORD *)(a1 + 2960) = 100;
  sub_146E9FBC0(a1 + 2976);
  v10 = qword_14E63AE60;
  if ( qword_14E63AE60
    || ((v11 = sub_146E8BA20(336), (v80 = (_QWORD *)v11) == 0)
      ? (v12 = 0)
      : (v12 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v11)),
        qword_14E63AE60 = (__int64)v12,
        (**v12)(v12),
        (v10 = qword_14E63AE60) != 0) )
  {
    sub_1447E6B20(v10);
  }
  if ( *(_QWORD *)(a1 + 280) && *(_QWORD *)(a1 + 176) )
  {
    v13 = sub_140732EE0(v10);
    if ( (unsigned __int8)sub_1412EC0E0(v13, *(_QWORD *)(a1 + 280), 0) )
    {
      v15 = sub_140732EE0(v14);
      if ( (unsigned __int8)sub_1412EBF10(v15, *(_QWORD *)(a1 + 280)) )
      {
        v16 = *(_QWORD *)(a1 + 264);
        if ( v16 )
        {
          if ( *(_DWORD *)(v16 + 8) )
          {
            v17 = *(_QWORD *)(a1 + 272);
            if ( v17 )
            {
              v18 = sub_145DF0410(v17);
              for ( i = 0; (int)i < v18; ++i )
              {
                v20 = *(_QWORD *)(a1 + 264);
                if ( v20 && *(_DWORD *)(v20 + 8) )
                  v21 = *(_QWORD *)(a1 + 272);
                else
                  v21 = 0;
                v22 = sub_145DF0390(v21, i);
                v23 = v22;
                if ( v22 )
                {
                  v24 = sub_1450BE0B0(v22);
                  if ( !v24 || !(unsigned __int8)sub_145C70DF0(v24) )
                  {
                    v25 = sub_1450BE600(v23);
                    if ( v25 )
                    {
                      v26 = qword_14E638CD0;
                      if ( !qword_14E638CD0 )
                      {
                        v27 = sub_146E8BA20(328);
                        v80 = (_QWORD *)v27;
                        if ( v27 )
                          v28 = (void (__fastcall ***)(_QWORD))sub_1412E99B0(v27);
                        else
                          v28 = 0;
                        qword_14E638CD0 = (__int64)v28;
                        (**v28)(v28);
                        v26 = qword_14E638CD0;
                      }
                      if ( (unsigned __int8)sub_1412EC0E0(v26, *(_QWORD *)(a1 + 280), v25) )
                      {
                        v29 = *(_DWORD *)sub_145B2D640(*(_QWORD *)(a1 + 280));
                        v30 = sub_144D24C00(*(_QWORD *)(a1 + 176));
                        v31 = qword_14E638CD0;
                        if ( !qword_14E638CD0 )
                        {
                          v32 = sub_146E8BA20(328);
                          v81 = v32;
                          if ( v32 )
                            v33 = (void (__fastcall ***)(_QWORD))sub_1412E99B0(v32);
                          else
                            v33 = 0;
                          qword_14E638CD0 = (__int64)v33;
                          (**v33)(v33);
                          v31 = qword_14E638CD0;
                        }
                        sub_1412EDA40(v31, v29, v30);
                        if ( *(_BYTE *)(sub_140157C80(*(_QWORD *)(a1 + 280)) + 2824) )
                        {
                          v34 = sub_145F0BA60(qword_14E683C08);
                          if ( v34 )
                          {
                            v35 = sub_145EFFF10(v34);
                            v36 = v35;
                            if ( v35 )
                            {
                              if ( *(_QWORD *)(sub_1401D2D30(v35) + 8) == 2 )
                              {
                                for ( j = 0; (int)j < 2; ++j )
                                {
                                  if ( sub_144507AA0(v36, j) )
                                  {
                                    v38 = sub_144507AA0(v36, j);
                                    v39 = *(_QWORD *)(v38 + 1800);
                                    if ( v39 )
                                    {
                                      if ( *(_DWORD *)(v39 + 8) )
                                      {
                                        v40 = *(_QWORD *)(v38 + 1808);
                                        v41 = v40 - 48;
                                        if ( !v40 )
                                          v41 = 0;
                                        if ( v41 )
                                        {
                                          v42 = sub_144507AA0(v36, j);
                                          v43 = *(_QWORD *)(v42 + 1800);
                                          if ( v43 && *(_DWORD *)(v43 + 8) )
                                            v44 = *(_QWORD *)(v42 + 1808);
                                          else
                                            v44 = 0;
                                          v45 = v44 - 48;
                                          if ( !v44 )
                                            v45 = 0;
                                          sub_145156750(v45, v25, 0, 0, v79);
                                        }
                                      }
                                    }
                                  }
                                }
                              }
                            }
                          }
                        }
                        else
                        {
                          v46 = sub_145EFAFB0();
                          sub_145156750(v46, v25, 0, 0, v79);
                        }
                      }
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
    if ( sub_140157C80(*(_QWORD *)(a1 + 280))
      && (unsigned int)(*(_DWORD *)(sub_140157C80(*(_QWORD *)(a1 + 280)) + 2100) - 109) <= 1 )
    {
      if ( !qword_14E6535A0 )
      {
        v47 = sub_146E8BA20(472);
        v80 = (_QWORD *)v47;
        if ( v47 )
          v48 = (void (__fastcall ***)(_QWORD))sub_142C9EAC0(v47);
        else
          v48 = 0;
        qword_14E6535A0 = (__int64)v48;
        (**v48)(v48);
      }
      sub_142CA2860();
    }
  }
  v49 = *(_QWORD *)(a1 + 280);
  if ( v49 && *(int *)(sub_140157C80(v49) + 4176) >= 0 )
  {
    v50 = sub_146ED84E0();
    v51 = sub_140157C80(*(_QWORD *)(a1 + 280));
    sub_146ED78E0(v50, *(unsigned int *)(v51 + 4176));
  }
  sub_145EA3210(a1, 0);
  sub_145EA3EA0(a1, 0);
  v52 = qword_14E6380D0;
  if ( !qword_14E6380D0 )
  {
    v53 = sub_146E8BA20(1208);
    v80 = (_QWORD *)v53;
    if ( v53 )
      v54 = sub_1446417B0(v53);
    else
      v54 = 0;
    qword_14E6380D0 = v54;
    (**(void (__fastcall ***)(__int64))(v54 + 1056))(v54 + 1056);
    v52 = qword_14E6380D0;
  }
  if ( *(_DWORD *)(v52 + 64) != -1 )
  {
    v55 = qword_14E634408;
    if ( !qword_14E634408 )
    {
      v56 = sub_146E8BA20(1520);
      v80 = (_QWORD *)v56;
      if ( v56 )
        v57 = (void (__fastcall ***)(_QWORD))sub_145194C50(v56);
      else
        v57 = 0;
      qword_14E634408 = (__int64)v57;
      (**v57)(v57);
      v52 = qword_14E6380D0;
      v55 = qword_14E634408;
    }
    if ( !v52 )
    {
      v58 = sub_146E8BA20(1208);
      v80 = (_QWORD *)v58;
      if ( v58 )
        v59 = sub_1446417B0(v58);
      else
        v59 = 0;
      qword_14E6380D0 = v59;
      (**(void (__fastcall ***)(__int64))(v59 + 1056))(v59 + 1056);
      v52 = qword_14E6380D0;
    }
    sub_14519FFF0(v55, *(unsigned int *)(v52 + 64));
  }
  v60 = *(_QWORD *)(a1 + 280);
  if ( v60 && *(_DWORD *)(sub_140157C80(v60) + 212) == 3524 )
    sub_145E920E0(a1);
  if ( *(_QWORD *)(a1 + 280) )
  {
    sub_145B1BD60();
    sub_145B1B0B0(*(_QWORD *)(a1 + 280));
    nullsub_1();
    if ( *(_QWORD *)(a1 + 280) )
    {
      sub_145B1AA70();
      v61 = *(_QWORD *)(a1 + 280);
      if ( v61 )
        (*(void (__fastcall **)(__int64))(*(_QWORD *)v61 + 120LL))(v61);
    }
  }
  *(_QWORD *)(a1 + 280) = 0;
  sub_145210B70();
  if ( !qword_14E639CA8 )
  {
    v62 = (_QWORD *)sub_146E8BA20(48);
    v80 = v62;
    if ( v62 )
      v63 = sub_144F2C110(v62);
    else
      v63 = 0;
    qword_14E639CA8 = (__int64)v63;
    (*(void (__fastcall **)(_QWORD *))*v63)(v63);
  }
  sub_144F2C260();
  v64 = qword_14E636800;
  if ( !qword_14E636800 )
  {
    v65 = sub_146E8BA20(360);
    v80 = (_QWORD *)v65;
    if ( v65 )
      v66 = (void (__fastcall ***)(_QWORD))sub_145362D40(v65);
    else
      v66 = 0;
    qword_14E636800 = (__int64)v66;
    (**v66)(v66);
    v64 = qword_14E636800;
  }
  sub_14536BAE0(v64, 3);
  if ( sub_145EFAFB0() )
  {
    v67 = sub_145EFAFB0();
    sub_145D40F90(v67, 0);
  }
  sub_146E9FBC0(a1 + 1696);
  *(_BYTE *)(a1 + 2856) = 0;
  sub_144BBBF80(qword_14E683C20, 0xFFFFFFFFLL);
  LODWORD(v80) = 1065353216;
  sub_146E922E0(&v80, &dword_14F360088);
  dword_14F36008C = (_DWORD)v80 + dword_14F360088 + 196;
  *(_QWORD *)(a1 + 288) = -1;
  byte_14E6849A0 = 0;
  if ( *(_QWORD *)(a1 + 2312) )
  {
    sub_146E45EB0();
    sub_146E45B60(*(_QWORD *)(a1 + 2312));
    *(_BYTE *)(a1 + 2320) = 0;
    v68 = *(_QWORD *)(a1 + 2312);
    v81 = v68;
    if ( v68 )
    {
      sub_146E9F800(v68 + 120);
      sub_14033EC80(v68 + 96);
      v81 = v68 + 64;
      v69 = *(volatile signed __int32 **)(v68 + 72);
      if ( v69 && _InterlockedExchangeAdd(v69 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v69 + 8LL))(v69);
      sub_146E9F3A0(v68, 152);
    }
    *(_QWORD *)(a1 + 2312) = 0;
  }
  sub_145E91A40(a1);
  sub_1402B0ED0(a1 + 3088);
  v70 = *(unsigned int **)(a1 + 3192);
  v71 = *(unsigned int **)(a1 + 3200);
  if ( v70 != v71 )
  {
    do
    {
      v72 = *v70;
      v73 = sub_146ED84E0();
      sub_146ED8720(v73, v72);
      ++v70;
    }
    while ( v70 != v71 );
    v70 = *(unsigned int **)(a1 + 3192);
  }
  *(_QWORD *)(a1 + 3200) = v70;
  if ( !qword_14E6BC700 )
  {
    v74 = sub_146E8BA20(168);
    v80 = (_QWORD *)v74;
    if ( v74 )
      v75 = (void (__fastcall ***)(_QWORD))sub_141F79C60(v74);
    else
      v75 = 0;
    qword_14E6BC700 = (__int64)v75;
    (**v75)(v75);
  }
  sub_141F7A450();
  *(_DWORD *)(a1 + 2896) = 0;
  *(_BYTE *)(a1 + 2900) = 0;
  *(_DWORD *)(a1 + 2904) = 5;
  *(_DWORD *)(a1 + 2908) = 5;
  *(_QWORD *)(a1 + 2912) = -1;
  *(_DWORD *)(a1 + 2920) = -1;
  *(_QWORD *)(a1 + 2924) = 0;
  *(_DWORD *)(a1 + 2932) = 0;
  *(_BYTE *)(a1 + 2936) = 1;
  sub_145E82ED0(a1);
  if ( *(_QWORD *)(a1 + 3296) )
    sub_140E15510();
  v76 = sub_146E74CB0();
  result = sub_146E79BE0(v76);
  v78 = *(_QWORD *)(a1 + 3408);
  if ( v78 )
  {
    while ( 1 )
    {
      result = v78 - 1;
      v78 = result;
      *(_QWORD *)(a1 + 3408) = result;
      if ( !result )
        break;
      ++*(_QWORD *)(a1 + 3400);
    }
    *(_QWORD *)(a1 + 3400) = 0;
  }
  return result;
}

