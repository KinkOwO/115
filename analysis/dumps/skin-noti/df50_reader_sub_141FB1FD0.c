// reader_sub_141FB1FD0

void __fastcall sub_141FB1FD0(__int64 a1, int a2)
{
  __int64 v2; // rsi
  __int64 v4; // rax
  __int64 v5; // r14
  __int64 v6; // rdx
  __int64 v7; // rcx
  __int64 v8; // rdi
  __int64 v9; // rax
  __int64 v10; // rcx
  __int64 v11; // rcx
  __int64 v12; // rax
  __int64 v13; // rax
  unsigned int *v14; // rbx
  int v15; // r9d
  int v16; // r8d
  unsigned int v17; // r15d
  __int64 v18; // rbx
  unsigned int v19; // eax
  __int64 v20; // rdx
  __int64 v21; // r8
  _QWORD *v22; // rax
  __int64 v23; // rdx
  __int64 v24; // r8
  _QWORD *v25; // rax
  __int64 v26; // rax
  _QWORD *v27; // rdi
  __int64 v28; // rax
  __int64 v29; // rax
  __int64 v30; // rax
  __int64 v31; // rax
  __int64 v32; // rax
  __int64 v33; // r8
  __int64 v34; // r9
  __int128 v35; // rdi
  __int64 v36; // rbx
  __int64 *v37; // rbx
  __int64 v38; // rax
  unsigned __int64 v39; // rsi
  __int128 *v40; // rdi
  __int64 v41; // rbx
  void (__fastcall ***v42)(_QWORD, __int64); // rcx
  char *v43; // rbx
  char *v44; // rdi
  unsigned __int64 v45; // rdx
  char *v46; // rax
  unsigned __int64 v47; // rdx
  __int64 v48; // rcx
  unsigned __int64 v49; // rdx
  __int64 v50; // rcx
  __int128 *v51; // [rsp+50h] [rbp-B0h] BYREF
  __int128 v52; // [rsp+58h] [rbp-A8h] BYREF
  __int64 v53; // [rsp+68h] [rbp-98h]
  __int128 *v54; // [rsp+70h] [rbp-90h]
  __int128 v55; // [rsp+78h] [rbp-88h] BYREF
  __int64 v56; // [rsp+88h] [rbp-78h]
  __int128 v57; // [rsp+90h] [rbp-70h] BYREF
  unsigned __int64 v58; // [rsp+A0h] [rbp-60h]
  __int64 v59; // [rsp+A8h] [rbp-58h]
  __int64 *v60; // [rsp+B0h] [rbp-50h]
  __int64 *v61; // [rsp+B8h] [rbp-48h]
  __int128 *v62; // [rsp+C0h] [rbp-40h]
  _QWORD v63[2]; // [rsp+C8h] [rbp-38h] BYREF
  __int64 v64; // [rsp+D8h] [rbp-28h] BYREF
  __int128 v65; // [rsp+E0h] [rbp-20h]
  __int128 v66; // [rsp+F0h] [rbp-10h]
  int v67; // [rsp+100h] [rbp+0h]
  int v68; // [rsp+104h] [rbp+4h]
  int v69; // [rsp+108h] [rbp+8h]
  __int128 v70; // [rsp+110h] [rbp+10h]
  __int128 v71; // [rsp+120h] [rbp+20h]
  int v72; // [rsp+130h] [rbp+30h]
  int v73; // [rsp+134h] [rbp+34h]
  __int128 v74; // [rsp+138h] [rbp+38h]
  __int64 v75; // [rsp+148h] [rbp+48h]
  __int64 v76; // [rsp+150h] [rbp+50h]
  __int64 *v77; // [rsp+158h] [rbp+58h]
  _BYTE *v78; // [rsp+160h] [rbp+60h]
  _BYTE *v79; // [rsp+168h] [rbp+68h]
  _BYTE *v80; // [rsp+170h] [rbp+70h]
  _QWORD v81[8]; // [rsp+178h] [rbp+78h] BYREF
  _BYTE v82[56]; // [rsp+1B8h] [rbp+B8h] BYREF
  __int64 v83; // [rsp+1F0h] [rbp+F0h]
  _BYTE v84[56]; // [rsp+1F8h] [rbp+F8h] BYREF
  __int64 v85; // [rsp+230h] [rbp+130h]
  _BYTE v86[56]; // [rsp+238h] [rbp+138h] BYREF
  __int64 v87; // [rsp+270h] [rbp+170h]
  _QWORD v88[2]; // [rsp+278h] [rbp+178h] BYREF
  unsigned __int64 v89; // [rsp+288h] [rbp+188h]
  unsigned __int64 v90; // [rsp+290h] [rbp+190h]
  _QWORD v91[2]; // [rsp+298h] [rbp+198h] BYREF
  __int64 v92; // [rsp+2A8h] [rbp+1A8h]
  unsigned __int64 v93; // [rsp+2B0h] [rbp+1B0h]
  __int64 v94; // [rsp+2C0h] [rbp+1C0h]
  __int128 v95; // [rsp+2C8h] [rbp+1C8h] BYREF
  __int128 v96; // [rsp+2D8h] [rbp+1D8h]
  int v97; // [rsp+2E8h] [rbp+1E8h]
  int v98; // [rsp+2ECh] [rbp+1ECh]
  int v99; // [rsp+2F0h] [rbp+1F0h]
  __int128 v100; // [rsp+2F8h] [rbp+1F8h] BYREF
  __int128 v101; // [rsp+308h] [rbp+208h]
  int v102; // [rsp+318h] [rbp+218h]
  int v103; // [rsp+31Ch] [rbp+21Ch]
  __int128 v104; // [rsp+320h] [rbp+220h]
  __int64 v105; // [rsp+330h] [rbp+230h]

  v76 = -2;
  v2 = a2;
  v4 = sub_141FAB150();
  v5 = v4;
  if ( v4 )
  {
    v8 = sub_140E511C0(v4);
    if ( v8 )
    {
      v9 = sub_14021A2C0(v7, v6);
      if ( (unsigned __int8)sub_145695000(v9, 2662) )
      {
        if ( (int)v2 >= 0 && !*(_BYTE *)(a1 + 2160) && !sub_14667BB90(qword_14E683C78, 2475, 0) && (int)v2 < 5 )
        {
          v10 = *(_QWORD *)(120 * v2 + a1 + 1592);
          if ( v10 )
          {
            if ( (unsigned __int8)sub_141FB6530(v10) )
            {
              v11 = *(_QWORD *)(120 * v2 + a1 + 1512);
              if ( v11 )
              {
                v12 = sub_14501B3E0(v11);
                if ( v12 )
                {
                  v13 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v12 + 152LL))(v12);
                  v14 = (unsigned int *)(v13 + 24);
                  LOBYTE(v15) = 1;
                  LOBYTE(v16) = 50;
                  sub_1480A6620(v13 + 24, 4, v16, v15, v13 + 28);
                  v17 = *v14;
                  v18 = sub_141FAB7D0(v5, *v14);
                  if ( v18 )
                  {
                    v19 = sub_140157900(v8);
                    if ( *(_DWORD *)(v18 + 4) == 1 )
                      v19 = sub_140157810(v8);
                    v22 = sub_14500A970(v19, v20, v21);
                    v91[0] = 0;
                    v92 = 0;
                    v93 = 7;
                    sub_14014C8B0(v91, v22);
                    v25 = sub_14500A970(v17, v23, v24);
                    v88[0] = 0;
                    v89 = 0;
                    v90 = 7;
                    sub_14014C8B0(v88, v25);
                    v26 = sub_14723C170(950);
                    sub_140243A10(v88, v26);
                    v27 = v91;
                    if ( v93 >= 8 )
                      v27 = (_QWORD *)v91[0];
                    v28 = sub_14723C170(949);
                    v29 = sub_146E8CF20(v63, v28, v27);
                    v30 = sub_14014F430(v29);
                    sub_140243A10(v88, v30);
                    sub_146E8C910(v63);
                    v31 = sub_14723C170(951);
                    sub_140243A10(v88, v31);
                    v52 = 0;
                    v53 = 0;
                    v32 = sub_146E8BA20(16);
                    v54 = (__int128 *)v32;
                    if ( v32 )
                    {
                      *(_QWORD *)v32 = &off_1491CC5C0;
                      *(_QWORD *)v32 = &off_1491CC5D8;
                      *(_QWORD *)v32 = off_1491CEDC8;
                      *(_DWORD *)(v32 + 8) = v2;
                    }
                    else
                    {
                      v32 = 0;
                    }
                    v51 = (__int128 *)v32;
                    if ( *((_QWORD *)&v52 + 1) == v53 )
                    {
                      sub_1401E82A0(&v52, *((_QWORD *)&v52 + 1), &v51);
                    }
                    else
                    {
                      **((_QWORD **)&v52 + 1) = v51;
                      v51 = 0;
                      *((_QWORD *)&v52 + 1) += 8LL;
                    }
                    if ( v51 )
                      (**(void (__fastcall ***)(__int128 *, __int64))v51)(v51, 1);
                    v77 = &v64;
                    v94 = -1;
                    *(_QWORD *)&v95 = 0;
                    *(_QWORD *)&v96 = 0;
                    *((_QWORD *)&v96 + 1) = 7;
                    sub_14014C8B0(&v95, &byte_14BAF7F08);
                    v97 = 2;
                    v98 = dword_14E64D628;
                    v99 = dword_14E64D62C;
                    *(_QWORD *)&v100 = 0;
                    *(_QWORD *)&v101 = 0;
                    *((_QWORD *)&v101 + 1) = 7;
                    sub_14014C8B0(&v100, &byte_14BAF7F08);
                    v102 = 134;
                    v103 = 1;
                    v104 = 0;
                    v105 = 0;
                    v64 = v94;
                    v65 = v95;
                    v66 = v96;
                    *(_QWORD *)&v96 = 0;
                    *((_QWORD *)&v96 + 1) = 7;
                    LOWORD(v95) = 0;
                    v67 = v97;
                    v68 = v98;
                    v69 = v99;
                    v70 = v100;
                    v71 = v101;
                    *(_QWORD *)&v101 = 0;
                    *((_QWORD *)&v101 + 1) = 7;
                    LOWORD(v100) = 0;
                    v72 = 134;
                    v73 = 1;
                    v74 = 0;
                    v75 = 0;
                    v78 = v82;
                    v83 = 0;
                    v79 = v84;
                    v85 = 0;
                    v80 = v86;
                    v87 = 0;
                    v63[0] = &v55;
                    v55 = 0;
                    v56 = 0;
                    v35 = v52;
                    if ( (_QWORD)v52 != *((_QWORD *)&v52 + 1) )
                    {
                      v36 = (__int64)(*((_QWORD *)&v52 + 1) - v52) >> 3;
                      *(_QWORD *)&v55 = sub_14017E100(&v55, v36, v33, v34);
                      *((_QWORD *)&v55 + 1) = v55;
                      v56 = v55 + 8 * v36;
                      v54 = &v55;
                      v37 = (__int64 *)v55;
                      v60 = (__int64 *)v55;
                      v61 = (__int64 *)v55;
                      v62 = &v55;
                      do
                      {
                        if ( *(_QWORD *)v35 )
                          v38 = (*(__int64 (__fastcall **)(_QWORD))(**(_QWORD **)v35 + 16LL))(*(_QWORD *)v35);
                        else
                          v38 = 0;
                        *v37++ = v38;
                        v61 = v37;
                        *(_QWORD *)&v35 = v35 + 8;
                      }
                      while ( (_QWORD)v35 != *((_QWORD *)&v35 + 1) );
                      v60 = v37;
                      *((_QWORD *)&v55 + 1) = v37;
                      v54 = 0;
                    }
                    v54 = (__int128 *)v81;
                    v81[0] = off_1498CD488;
                    v81[7] = v81;
                    v51 = &v57;
                    *(_QWORD *)&v57 = 0;
                    v58 = 0;
                    v59 = 0;
                    v39 = v89;
                    v40 = (__int128 *)v88;
                    if ( v90 >= 8 )
                      v40 = (__int128 *)v88[0];
                    if ( v89 >= 8 )
                    {
                      v41 = v89 | 7;
                      if ( (v89 | 7) > 0x7FFFFFFFFFFFFFFELL )
                        v41 = 0x7FFFFFFFFFFFFFFELL;
                      *(_QWORD *)&v57 = sub_14014CB50(&v57, v41 + 1);
                      sub_148AA1E60(v57, v40, 2 * v39 + 2);
                    }
                    else
                    {
                      v57 = *v40;
                      v41 = 7;
                    }
                    v58 = v39;
                    v59 = v41;
                    sub_14668DCB0(
                      qword_14E683C78,
                      12,
                      (unsigned int)&v57,
                      (unsigned int)v81,
                      (__int64)&v55,
                      (__int64)v86,
                      (__int64)v84,
                      (__int64)v82,
                      (__int64)&v64);
                    *(_QWORD *)&v101 = 0;
                    *((_QWORD *)&v101 + 1) = 7;
                    LOWORD(v100) = 0;
                    *(_QWORD *)&v96 = 0;
                    *((_QWORD *)&v96 + 1) = 7;
                    LOWORD(v95) = 0;
                    v43 = (char *)v52;
                    if ( (_QWORD)v52 )
                    {
                      v44 = (char *)*((_QWORD *)&v52 + 1);
                      if ( (_QWORD)v52 != *((_QWORD *)&v52 + 1) )
                      {
                        do
                        {
                          v42 = *(void (__fastcall ****)(_QWORD, __int64))v43;
                          if ( *(_QWORD *)v43 )
                            (**v42)(v42, 1);
                          v43 += 8;
                        }
                        while ( v43 != v44 );
                        v43 = (char *)v52;
                      }
                      v45 = 8 * ((v53 - (__int64)v43) >> 3);
                      v46 = v43;
                      if ( v45 >= 0x1000 )
                      {
                        v45 += 39LL;
                        v43 = (char *)*((_QWORD *)v43 - 1);
                        if ( (unsigned __int64)(v46 - v43 - 8) > 0x1F )
                          sub_148AAF304(v42, v45);
                      }
                      sub_146E9F3A0(v43, v45);
                      v52 = 0;
                      v53 = 0;
                    }
                    if ( v90 >= 8 )
                    {
                      v47 = 2 * v90 + 2;
                      v48 = v88[0];
                      if ( v47 >= 0x1000 )
                      {
                        v47 = 2 * v90 + 41;
                        v48 = *(_QWORD *)(v88[0] - 8LL);
                        if ( (unsigned __int64)(v88[0] - v48 - 8) > 0x1F )
                          sub_148AAF304(v48, v47);
                      }
                      sub_146E9F3A0(v48, v47);
                    }
                    v89 = 0;
                    v90 = 7;
                    LOWORD(v88[0]) = 0;
                    if ( v93 >= 8 )
                    {
                      v49 = 2 * v93 + 2;
                      v50 = v91[0];
                      if ( v49 >= 0x1000 )
                      {
                        v49 = 2 * v93 + 41;
                        v50 = *(_QWORD *)(v91[0] - 8LL);
                        if ( (unsigned __int64)(v91[0] - v50 - 8) > 0x1F )
                          sub_148AAF304(v50, v49);
                      }
                      sub_146E9F3A0(v50, v49);
                    }
                    v92 = 0;
                    v93 = 7;
                    LOWORD(v91[0]) = 0;
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

