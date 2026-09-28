// singleton_user_sub_1444AC7C0

void __fastcall sub_1444AC7C0(__int64 a1, _QWORD *a2)
{
  _QWORD *v2; // rdi
  int v4; // r13d
  _QWORD *v5; // r15
  int v6; // edx
  int v7; // r8d
  __int64 v8; // rcx
  __int64 v9; // rax
  void (__fastcall ***v10)(_QWORD); // rcx
  __int64 v11; // r12
  int v12; // ebx
  int v13; // eax
  int v14; // edx
  int v15; // r8d
  int *v16; // rcx
  __int64 v17; // rcx
  __int64 v18; // rax
  void (__fastcall ***v19)(_QWORD); // rcx
  int v20; // ebx
  __int64 v21; // rcx
  __int64 v22; // rax
  void (__fastcall ***v23)(_QWORD); // rcx
  __int64 v24; // rcx
  __int64 v25; // rax
  void (__fastcall ***v26)(_QWORD); // rcx
  int v27; // edi
  __int64 v28; // rcx
  __int64 v29; // rax
  void (__fastcall ***v30)(_QWORD); // rcx
  int v31; // esi
  int v32; // ebx
  int v33; // eax
  int v34; // edx
  int v35; // r8d
  __int64 v36; // r8
  int *v37; // rcx
  unsigned int v38; // eax
  int v39; // r8d
  int v40; // ebx
  int v41; // eax
  int v42; // edx
  int v43; // r8d
  __int64 v44; // r8
  int *v45; // rcx
  unsigned int v46; // eax
  int v47; // r8d
  int v48; // ebx
  int v49; // eax
  int v50; // edx
  int v51; // r8d
  int *v52; // rcx
  char v53; // si
  int v54; // eax
  int v55; // edi
  int v56; // ebx
  __int64 v57; // rax
  int v58; // edx
  int v59; // r8d
  __int64 v60; // rcx
  int *v61; // [rsp+88h] [rbp-80h] BYREF
  int *v62; // [rsp+90h] [rbp-78h] BYREF
  int *v63; // [rsp+98h] [rbp-70h] BYREF
  int *v64; // [rsp+A0h] [rbp-68h] BYREF
  int *v65; // [rsp+A8h] [rbp-60h] BYREF
  int *v66; // [rsp+B0h] [rbp-58h] BYREF
  __int64 v67; // [rsp+B8h] [rbp-50h]
  _BYTE v68[24]; // [rsp+C0h] [rbp-48h] BYREF
  __int64 v69; // [rsp+D8h] [rbp-30h]
  __int64 v70; // [rsp+E0h] [rbp-28h]
  __int64 v71; // [rsp+E8h] [rbp-20h]
  __int64 v72; // [rsp+F0h] [rbp-18h]
  __int64 v73; // [rsp+F8h] [rbp-10h]
  __int64 v74; // [rsp+100h] [rbp-8h]
  int **v75; // [rsp+108h] [rbp+0h]
  int **v76; // [rsp+110h] [rbp+8h]
  unsigned int v78; // [rsp+208h] [rbp+100h]
  int v79; // [rsp+210h] [rbp+108h]

  v69 = -2;
  v2 = a2;
  if ( *a2 )
  {
    sub_146ECA0C0(*a2);
    sub_142757420(*v2);
    if ( *v2 == *(_QWORD *)(a1 + 1496) )
      sub_1444AC300(a1 - 24, 220);
    sub_1467A6790(a1, v2);
    if ( *v2 == *(_QWORD *)(a1 + 1512) )
      sub_1444AC590(a1 - 24);
    v4 = 0;
    v5 = (_QWORD *)(a1 + 1888);
    do
    {
      if ( *v2 == *(v5 - 20) && !(unsigned __int8)sub_146B34290(*v5) )
      {
        sub_146B456F0(*v5, v6, v7, 0, 1, -1082130432, 1);
        sub_146B34310(*v5);
        if ( (unsigned __int8)sub_146B34270(*v5) )
        {
          v8 = qword_14E659EA8;
          if ( !qword_14E659EA8 )
          {
            v9 = sub_146E8BA20(496);
            v70 = v9;
            if ( v9 )
              v10 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v9);
            else
              v10 = 0;
            qword_14E659EA8 = (__int64)v10;
            (**v10)(v10);
            v8 = qword_14E659EA8;
          }
          v11 = (unsigned int)sub_14449E110(v8, (unsigned int)v4);
          v67 = sub_145F13700(qword_14E683C20, v11);
          if ( v67 )
          {
            v17 = qword_14E659EA8;
            if ( !qword_14E659EA8 )
            {
              v18 = sub_146E8BA20(496);
              v71 = v18;
              if ( v18 )
                v19 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v18);
              else
                v19 = 0;
              qword_14E659EA8 = (__int64)v19;
              (**v19)(v19);
              v17 = qword_14E659EA8;
            }
            v20 = *(_DWORD *)(sub_14449E120(v17, v68, (unsigned int)v11) + 8);
            v78 = v20;
            v21 = qword_14E659EA8;
            if ( !qword_14E659EA8 )
            {
              v22 = sub_146E8BA20(496);
              v72 = v22;
              if ( v22 )
                v23 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v22);
              else
                v23 = 0;
              qword_14E659EA8 = (__int64)v23;
              (**v23)(v23);
              v21 = qword_14E659EA8;
            }
            v79 = *(_DWORD *)(sub_14449E120(v21, v68, (unsigned int)v11) + 12);
            v24 = qword_14E659EA8;
            if ( !qword_14E659EA8 )
            {
              v25 = sub_146E8BA20(496);
              v73 = v25;
              if ( v25 )
                v26 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v25);
              else
                v26 = 0;
              qword_14E659EA8 = (__int64)v26;
              (**v26)(v26);
              v24 = qword_14E659EA8;
            }
            v27 = *(_DWORD *)(sub_14449E120(v24, v68, (unsigned int)v11) + 16);
            v28 = qword_14E659EA8;
            if ( !qword_14E659EA8 )
            {
              v29 = sub_146E8BA20(496);
              v74 = v29;
              if ( v29 )
                v30 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v29);
              else
                v30 = 0;
              qword_14E659EA8 = (__int64)v30;
              (**v30)(v30);
              v28 = qword_14E659EA8;
            }
            v31 = *(_DWORD *)(sub_14449E120(v28, v68, (unsigned int)v11) + 20);
            if ( v20 <= 0 || v27 <= 0 )
            {
              v48 = *(_DWORD *)(*(_QWORD *)(a1 + 8) + 4LL);
              v49 = (*(__int64 (__fastcall **)(_QWORD, int **, __int64))(**(_QWORD **)(a1 + 1536) + 120LL))(
                      *(_QWORD *)(a1 + 1536),
                      &v66,
                      9);
              sub_146EBBA60(v48 + a1 + 8, v50, v51, v49, 1065353216, 1065353216, 0, -1, 2123789977, 2123789977);
              v52 = v66;
              if ( v66 )
              {
                --v66[2];
                if ( v52[2] <= 0 )
                  (*(void (__fastcall **)(int *))(*(_QWORD *)v52 + 8LL))(v52);
              }
            }
            else
            {
              v32 = *(_DWORD *)(*(_QWORD *)(a1 + 8) + 4LL);
              v33 = (*(__int64 (__fastcall **)(_QWORD, int **, __int64))(**(_QWORD **)(a1 + 1536) + 120LL))(
                      *(_QWORD *)(a1 + 1536),
                      &v62,
                      12);
              sub_146EBBA60(v32 + a1 + 8, v34, v35, v33, 1065353216, 1065353216, 0, -1, 2123789977, 2123789977);
              v37 = v62;
              if ( v62 )
              {
                --v62[2];
                if ( v37[2] <= 0 )
                  (*(void (__fastcall **)(int *))(*(_QWORD *)v37 + 8LL))(v37);
              }
              v75 = &v63;
              v38 = (unsigned int)sub_14500A5B0(&v63, v78, v36);
              sub_1450064E0(
                *(_DWORD *)(*(_QWORD *)(a1 + 8) + 4LL) + a1 + 8,
                v79,
                v39,
                v38,
                v79,
                0,
                0,
                0,
                255,
                0,
                0,
                0,
                0,
                0,
                0);
              v40 = *(_DWORD *)(*(_QWORD *)(a1 + 8) + 4LL);
              v41 = (*(__int64 (__fastcall **)(_QWORD, int **, __int64))(**(_QWORD **)(a1 + 1536) + 120LL))(
                      *(_QWORD *)(a1 + 1536),
                      &v64,
                      12);
              sub_146EBBA60(v40 + a1 + 8, v42, v43, v41, 1065353216, 1065353216, 0, -1, 2123789977, 2123789977);
              v45 = v64;
              if ( v64 )
              {
                --v64[2];
                if ( v45[2] <= 0 )
                  (*(void (__fastcall **)(int *))(*(_QWORD *)v45 + 8LL))(v45);
              }
              v76 = &v65;
              v46 = (unsigned int)sub_14500A5B0(&v65, (unsigned int)v27, v44);
              sub_1450064E0(
                *(_DWORD *)(*(_QWORD *)(a1 + 8) + 4LL) + a1 + 8,
                0,
                v47,
                v46,
                v31,
                0,
                0,
                0,
                255,
                0,
                0,
                0,
                0,
                0,
                0);
            }
            v53 = BYTE10(xmmword_14DC6B350);
            BYTE10(xmmword_14DC6B350) = 1;
            sub_146EC0510(&xmmword_14DC6B350);
            v54 = sub_145F12D90(qword_14E683C20);
            v55 = dword_14F1C0880;
            if ( v54 == (_DWORD)v11 )
              v55 = dword_14F1C0A0C;
            v56 = *(_DWORD *)(*(_QWORD *)(a1 + 8) + 4LL);
            v57 = sub_141D37640(v67);
            sub_146EBDED0(v56 + a1 + 8, v58, v59, v55, v57, 0);
            sub_146EC0580(v60);
            BYTE10(xmmword_14DC6B350) = v53;
            v2 = a2;
          }
          else
          {
            v12 = *(_DWORD *)(*(_QWORD *)(a1 + 8) + 4LL);
            v13 = (*(__int64 (__fastcall **)(_QWORD, int **, __int64))(**(_QWORD **)(a1 + 1536) + 120LL))(
                    *(_QWORD *)(a1 + 1536),
                    &v61,
                    9);
            sub_146EBBA60(v12 + a1 + 8, v14, v15, v13, 1065353216, 1065353216, 0, -1, 2123789977, 2123789977);
            v16 = v61;
            if ( v61 )
            {
              --v61[2];
              if ( v16[2] <= 0 )
                (*(void (__fastcall **)(int *))(*(_QWORD *)v16 + 8LL))(v16);
            }
          }
        }
      }
      ++v4;
      v5 += 2;
    }
    while ( v4 < 3 );
  }
}

