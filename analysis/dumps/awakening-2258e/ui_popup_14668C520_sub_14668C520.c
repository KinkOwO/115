// sub_14668C520  va=0x14668C520  size=5024

__int64 __fastcall sub_14668C520(__int64 a1, int a2, __int64 a3, __int64 a4)
{
  __int64 v7; // r12
  __int64 v8; // rax
  std::ios_base *v9; // rax
  __int64 *v10; // r8
  unsigned int v11; // ebx
  __int64 v12; // rax
  std::ios_base *v13; // rax
  __int64 v14; // rax
  __int64 *v15; // rax
  __int64 *v16; // rcx
  __int64 v17; // r8
  __int64 v18; // r8
  bool v19; // zf
  bool v20; // zf
  __int64 v21; // rax
  __int64 v22; // r14
  __int64 v23; // rax
  __int64 v24; // rdx
  bool v25; // zf
  int v27; // eax
  __int64 v28; // r9
  __int64 v29; // rdx
  __int64 *v30; // r8
  unsigned __int64 v31; // rcx
  __int64 *v32; // r10
  __int64 v33; // r11
  unsigned __int64 v34; // rcx
  __int64 v35; // rdi
  __int64 v36; // rsi
  __int64 *v37; // r14
  __int64 v38; // rcx
  _QWORD *v39; // rax
  _QWORD *v40; // rdi
  _QWORD *v41; // rsi
  __int64 v42; // rax
  __int64 v43; // rbx
  __int64 v44; // rax
  unsigned int v45; // eax
  __int64 v46; // rbx
  __int64 v47; // rax
  char v48; // di
  volatile signed __int32 *v49; // rbx
  _QWORD *v50; // rax
  volatile signed __int32 *v51; // rbx
  __int64 *v52; // rdx
  _QWORD *v53; // rax
  _QWORD *v54; // rdx
  __int64 v55; // rcx
  __int64 *v56; // rdx
  int v57; // esi
  unsigned int v58; // ebx
  __int64 v59; // rax
  __int64 v60; // rax
  wchar_t *v61; // rbx
  int v62; // edi
  int v63; // eax
  __int64 v64; // rax
  __int64 v65; // rax
  int v66; // ebx
  int v67; // esi
  unsigned int v68; // ebx
  __int64 v69; // rax
  __int64 v70; // rax
  wchar_t *v71; // rbx
  int v72; // edi
  int v73; // eax
  _QWORD *v74; // rax
  __int64 v75; // r9
  _QWORD *v76; // rcx
  __int64 *v77; // rdx
  volatile __int64 *v78; // rsi
  __int64 v79; // rax
  __int64 v80; // rbx
  __int64 v81; // rdi
  unsigned __int64 v82; // rbx
  __int64 v83; // rdi
  void (__fastcall *v84)(__int64, __int128 *, __int64); // rbx
  __int64 v85; // rax
  volatile signed __int32 *v86; // rbx
  __int64 v87; // rax
  __int64 v88; // rdi
  void (__fastcall *v89)(__int64, __int128 *); // rbx
  __int64 v90; // rax
  __int64 v91; // rdi
  void (__fastcall *v92)(__int64, _QWORD *); // rbx
  __int64 v93; // rax
  __int64 v94; // rax
  _QWORD *v95; // rax
  __int64 v96; // rdx
  volatile signed __int32 *v97; // rbx
  _QWORD *v98; // rax
  bool v99; // bl
  __int64 v100; // rcx
  __int64 v101; // rax
  __int64 v102; // rax
  __int64 v103; // rax
  __int64 v104; // rax
  __int64 v105; // rcx
  unsigned __int64 v106; // rdx
  __int64 v107; // rax
  __int64 v108; // r8
  __int64 v109; // rax
  _LocaleUpdate *v110; // rcx
  _QWORD *v111; // rbx
  __int64 *v112; // rax
  _QWORD *v113; // rcx
  __int64 v114; // r8
  __int64 *v115; // rbx
  __int64 v116; // rdx
  __int64 **v117; // rax
  __int64 *i; // rax
  __int64 *j; // rcx
  __int64 v120; // rax
  __int64 v121; // rax
  __int64 v122; // rdi
  unsigned int v123; // ebx
  __int64 v124; // rax
  __int64 v125; // rax
  __int64 v126; // rsi
  void (__fastcall *v127)(__int64, __int64, __int64, _QWORD *); // rbx
  __int64 v128; // rax
  __int64 v129; // rax
  volatile signed __int32 *v130; // rbx
  _BYTE v131[24]; // [rsp+8h] [rbp-100h] BYREF
  __int64 v132; // [rsp+40h] [rbp-C8h]
  __int64 v133; // [rsp+48h] [rbp-C0h] BYREF
  __int128 *v134; // [rsp+50h] [rbp-B8h]
  int v135; // [rsp+58h] [rbp-B0h] BYREF
  int v136; // [rsp+5Ch] [rbp-ACh] BYREF
  __int128 *v137; // [rsp+60h] [rbp-A8h] BYREF
  _QWORD v138[3]; // [rsp+70h] [rbp-98h] BYREF
  __int64 v139; // [rsp+88h] [rbp-80h]
  __int128 v140; // [rsp+90h] [rbp-78h] BYREF
  __int128 v141; // [rsp+A0h] [rbp-68h] BYREF
  __int128 v142; // [rsp+B0h] [rbp-58h] BYREF
  _QWORD v143[2]; // [rsp+C0h] [rbp-48h] BYREF
  __int64 v144; // [rsp+D8h] [rbp-30h] BYREF
  volatile signed __int32 *v145; // [rsp+E0h] [rbp-28h]
  _QWORD v146[3]; // [rsp+E8h] [rbp-20h] BYREF
  _QWORD v147[4]; // [rsp+100h] [rbp-8h] BYREF
  char v148[8]; // [rsp+120h] [rbp+18h] BYREF
  volatile signed __int32 *v149; // [rsp+128h] [rbp+20h]
  char v150[8]; // [rsp+130h] [rbp+28h] BYREF
  volatile signed __int32 *v151; // [rsp+138h] [rbp+30h]
  _BYTE v152[16]; // [rsp+140h] [rbp+38h] BYREF
  char v153[8]; // [rsp+150h] [rbp+48h] BYREF
  volatile signed __int32 *v154; // [rsp+158h] [rbp+50h]
  char v155[8]; // [rsp+160h] [rbp+58h] BYREF
  volatile signed __int32 *v156; // [rsp+168h] [rbp+60h]
  char v157[8]; // [rsp+170h] [rbp+68h] BYREF
  volatile signed __int32 *v158; // [rsp+178h] [rbp+70h]
  char v159[8]; // [rsp+180h] [rbp+78h] BYREF
  volatile signed __int32 *v160; // [rsp+188h] [rbp+80h]
  char v161[8]; // [rsp+190h] [rbp+88h] BYREF
  volatile signed __int32 *v162; // [rsp+198h] [rbp+90h]
  _QWORD v163[7]; // [rsp+1A0h] [rbp+98h] BYREF
  _QWORD *v164; // [rsp+1D8h] [rbp+D0h]
  _BYTE v165[32]; // [rsp+1E0h] [rbp+D8h] BYREF
  _BYTE v166[32]; // [rsp+200h] [rbp+F8h] BYREF
  char v167[72]; // [rsp+220h] [rbp+118h] BYREF
  _UNKNOWN *retaddr; // [rsp+270h] [rbp+168h]
  int v169; // [rsp+280h] [rbp+178h] BYREF
  __int64 v170; // [rsp+288h] [rbp+180h]
  __int64 v171; // [rsp+290h] [rbp+188h]

  v171 = a4;
  v170 = a3;
  v169 = a2;
  v147[3] = -2;
  v7 = 0;
  if ( a2 >= 4176 )
    return 0;
  if ( qword_14E6343D8 == nullptr )
  {
    v8 = sub_146E9F2A0(184);
    v137 = (__int128 *)v8;
    __eh34_enter_wind_state(-1, 0);
    if ( v8 != 0 )
      v9 = (std::ios_base *)sub_146EC2F90(v8);
    else
      v9 = nullptr;
    if ( __eh34_unwind(0) )
    {
unwind_state_0:
      j_j_scalable_free(v137, v131);
      __eh34_propagate_exception_into_caller(0, -1);
    }
    __eh34_exit_wind_state(0, -1);
    qword_14E6343D8 = v9;
    (**((void (__fastcall ***)(__int64))v9 + 2))((__int64)v9 + 16);
  }
  if ( (unsigned __int8)sub_146EC3CF0() == 0 )
  {
    v11 = v169;
    if ( qword_14E6343D8 == nullptr )
    {
      v12 = sub_146E9F2A0(184);
      v137 = (__int128 *)v12;
      __wind
      {
        if ( v12 != 0 )
          v13 = (std::ios_base *)sub_146EC2F90(v12);
        else
          v13 = nullptr;
      }
      __unwind
      {
        j_j_scalable_free(v137, v131);
      }
      qword_14E6343D8 = v13;
      (**((void (__fastcall ***)(__int64))v13 + 2))((__int64)v13 + 16);
    }
    if ( (unsigned __int8)sub_146EC3CF0() != 0 )
    {
      sub_14668C520(a1, v11, a3, a4);
    }
    else
    {
      v14 = sub_146E8BA20(56);
      v137 = (__int128 *)v14;
      __wind
      {
        if ( v14 != 0 )
          v7 = sub_14674C540(v14);
      }
      __unwind
      {
        j_j_scalable_free(v137, 56);
      }
      v137 = (__int128 *)v7;
      sub_146751400(v7);
      *(_DWORD *)v7 = v11;
      *(_QWORD *)(v7 + 8) = a3;
      *(_QWORD *)(v7 + 16) = a4;
      sub_146E8BE40(*(_QWORD *)(a1 + 201064));
      sub_140379170(a1 + 201024, &v137);
      Atomic_lock_release(*(_QWORD *)(a1 + 201064));
    }
    return 0;
  }
  if ( *(_DWORD *)(a1 + 202256) == 0 )
  {
    v10 = *(__int64 **)(a1 + 203040);
    v15 = (__int64 *)v10[1];
    v16 = v10;
    while ( *((_BYTE *)v15 + 25) == 0 )
    {
      if ( *((_DWORD *)v15 + 7) >= v169 )
      {
        v16 = v15;
        v15 = (__int64 *)*v15;
      }
      else
      {
        v15 = (__int64 *)v15[2];
      }
    }
    if ( *((_BYTE *)v16 + 25) != 0 || v169 < *((_DWORD *)v16 + 7) || v16 == v10 )
      return 0;
  }
  LOBYTE(v10) = 1;
  if ( (unsigned __int8)sub_146682140(a1, 579, v10) != 0 )
  {
    LOBYTE(v17) = 1;
    if ( ((unsigned __int8)sub_146682140(a1, 951, v17) == 0 || v169 != 2475)
      && (unsigned __int8)sub_1441489C0((unsigned int)v169) == 0 )
    {
      sub_14414C160((unsigned int)v169);
      return 0;
    }
  }
  LOBYTE(v17) = 1;
  if ( (unsigned __int8)sub_146682140(a1, 583, v17) != 0 )
  {
    if ( v169 > 881 )
    {
      if ( v169 > 1430 )
      {
        if ( v169 != 2869 && v169 != 2101 && v169 != 2335 && v169 != 2475 && v169 != 3102 && v169 != 3856 )
          goto LABEL_44;
        goto LABEL_74;
      }
      if ( v169 == 1430 )
        goto LABEL_74;
      if ( v169 > 1288 )
      {
        if ( v169 == 1289 || v169 == 1376 )
          goto LABEL_74;
        v19 = v169 == 1384;
      }
      else
      {
        if ( v169 == 1288 || v169 == 979 || v169 == 980 )
          goto LABEL_74;
        v19 = v169 == 1284;
      }
    }
    else
    {
      if ( v169 == 881 )
        goto LABEL_74;
      if ( v169 > 518 )
      {
        __eh34_enter_wind_state(-1, 0);
        switch ( v169 )
        {
          case 537:
          case 550:
          case 569:
          case 720:
          case 730:
          case 740:
            if ( __eh34_unwind(0) )
              goto unwind_state_0;
            __eh34_exit_wind_state(0, -1);
            goto LABEL_74;
          default:
            if ( __eh34_unwind(0) )
              goto unwind_state_0;
            __eh34_exit_wind_state(0, -1);
            goto LABEL_44;
        }
      }
      if ( v169 == 518 )
        goto LABEL_74;
      if ( v169 <= 333 )
      {
        if ( v169 != 333 && v169 != 132 && v169 != 159 )
        {
          v19 = v169 == 325;
          goto LABEL_43;
        }
LABEL_74:
        LOBYTE(v18) = 1;
        if ( (unsigned __int8)sub_146682140(a1, 951, v18) == 0 || v169 != 2475 )
        {
          v21 = sub_14667BB90(a1, 583, 0);
          (*(void (__fastcall **)(__int64))(*(_QWORD *)v21 + 208LL))(v21);
          return 0;
        }
        goto LABEL_44;
      }
      if ( v169 == 417 || v169 == 454 )
        goto LABEL_74;
      v19 = v169 == 489;
    }
LABEL_43:
    if ( !v19 )
      goto LABEL_44;
    goto LABEL_74;
  }
LABEL_44:
  LOBYTE(v18) = 1;
  if ( (unsigned __int8)sub_146682140(a1, 3480, v18) != 0 )
  {
    if ( v169 > 1284 )
    {
      if ( v169 > 2335 )
      {
        if ( v169 == 2869 || v169 == 3083 || v169 == 3084 || v169 == 3085 )
          return 0;
        v20 = v169 == 3102;
      }
      else
      {
        if ( v169 == 2335 )
          return 0;
        if ( v169 > 2017 )
        {
          if ( v169 == 2018 )
            return 0;
          v20 = v169 == 2101;
        }
        else
        {
          if ( v169 == 2017 || v169 == 1376 || v169 == 1384 )
            return 0;
          v20 = v169 == 1430;
        }
      }
    }
    else
    {
      if ( v169 == 1284 )
        return 0;
      if ( v169 > 489 )
      {
        if ( v169 > 881 )
        {
          if ( v169 == 979 )
            return 0;
          v20 = v169 == 980;
        }
        else
        {
          if ( v169 == 881 || v169 == 537 || v169 == 569 )
            return 0;
          v20 = v169 == 720;
        }
      }
      else
      {
        if ( v169 == 489 )
          return 0;
        if ( v169 > 333 )
        {
          if ( v169 == 417 )
            return 0;
          v20 = v169 == 454;
        }
        else
        {
          if ( v169 == 333 || v169 == 132 || v169 == 159 )
            return 0;
          v20 = v169 == 325;
        }
      }
    }
    if ( v20 )
      return 0;
  }
  if ( (unsigned __int8)sub_146684750(a1, 1, (unsigned int)v169, 12) == 0 )
    return 0;
  v22 = 0;
  v133 = 0;
  v23 = sub_14170F720();
  if ( (unsigned __int8)sub_143CA0E80(v23) != 0 )
  {
    if ( v169 > 2613 )
    {
      if ( v169 > 3726 )
      {
        v24 = (unsigned int)(v169 - 3778);
        if ( v169 != 3778 )
        {
          v24 = (unsigned int)(v169 - 3781);
          if ( v169 != 3781 && v169 != 4052 )
            return 0;
        }
        goto LABEL_125;
      }
      if ( v169 == 3726 )
        goto LABEL_125;
      v24 = (unsigned int)(v169 - 2875);
      if ( v169 == 2875 )
        goto LABEL_125;
      v24 = (unsigned int)(v169 - 3513);
      if ( v169 == 3513 )
        goto LABEL_125;
      v24 = (unsigned int)(v169 - 3518);
      if ( v169 == 3518 )
        goto LABEL_125;
      v25 = (_DWORD)v24 == 2;
    }
    else
    {
      if ( v169 == 2613 )
        goto LABEL_125;
      if ( v169 > 2323 )
      {
        v24 = (unsigned int)(v169 - 2333);
        if ( v169 == 2333 )
          goto LABEL_125;
        v24 = (unsigned int)(v169 - 2427);
        if ( v169 == 2427 )
          goto LABEL_125;
        v25 = (_DWORD)v24 == 6;
      }
      else
      {
        if ( v169 == 2323 )
          goto LABEL_125;
        v24 = (unsigned int)(v169 - 417);
        if ( v169 == 417 )
          goto LABEL_125;
        v24 = (unsigned int)(v169 - 418);
        if ( v169 == 418 )
          goto LABEL_125;
        v24 = (unsigned int)(v169 - 476);
        if ( v169 == 476 )
          goto LABEL_125;
        v25 = (_DWORD)v24 == 908;
      }
    }
    if ( !v25 )
      return 0;
  }
LABEL_125:
  v27 = sub_1467A2A80((unsigned int)v169, v24);
  v29 = 3LL * v169;
  v30 = *(__int64 **)(a1 + 24LL * v169 + 472);
  v31 = (__int64)(*(_QWORD *)(a1 + 24LL * v169 + 480) - (_QWORD)v30) >> 3;
  if ( v27 > v31 )
  {
    v37 = *(__int64 **)(a1 + 24LL * v169 + 100696);
    if ( v37 == *(__int64 **)(a1 + 24LL * v169 + 100704) )
      goto LABEL_147;
    v22 = *v37;
    v133 = v22;
  }
  else
  {
    if ( v31 != 0 )
    {
      v22 = *v30;
      v133 = *v30;
    }
    sub_146671A70(a1, v22, 0);
    v29 = *(_QWORD *)(a1 + 200992);
    v32 = *(__int64 **)(a1 + 200968);
    if ( a1 == -200968 )
      v32 = nullptr;
    v33 = v29 + *(_QWORD *)(a1 + 201000);
    if ( v32 != nullptr )
      v28 = *v32;
    else
      v28 = 0;
    if ( v29 != v33 )
    {
      v34 = *(_QWORD *)(a1 + 200992);
      LOBYTE(v30) = v34;
      v35 = *(_QWORD *)(v28 + 16) - 1LL;
      v36 = *(_QWORD *)(v28 + 8);
      while ( 1 )
      {
        v28 = v29++;
        if ( *(_QWORD *)(*(_QWORD *)(v36 + 8 * (v35 & (v34 >> 1))) + 8LL * ((unsigned __int8)v30 & 1)) == v22 )
          break;
        v34 = v29;
        v30 = (__int64 *)v29;
        if ( v29 == v33 )
          goto LABEL_141;
      }
      v146[1] = 0;
      v146[0] = v32;
      v146[2] = v29;
      v147[1] = 0;
      v147[0] = v32;
      v147[2] = v28;
      sub_1438A6460(a1 + 200968, v167, v147, v146);
    }
  }
LABEL_141:
  if ( v22 != 0 )
  {
    v38 = 3LL * v169;
    v39 = *(_QWORD **)(a1 + 24LL * v169 + 100704);
    v40 = *(_QWORD **)(a1 + 24LL * v169 + 100696);
    if ( v40 != v39 )
    {
      while ( 1 )
      {
        v41 = v40 + 1;
        if ( *v40 == v22 )
          break;
        ++v40;
        if ( v41 == v39 )
          goto LABEL_149;
      }
      (*(void (__fastcall **)(_QWORD, __int64, __int64 *, __int64))(*(_QWORD *)*v40 + 504LL))(*v40, v29, v30, v28);
      v42 = v169 + 4196LL;
      v43 = 3 * v42;
      memmove(v40, v40 + 1, *(_QWORD *)(a1 + 24 * v42) - (_QWORD)v41);
      *(_QWORD *)(a1 + 8 * v43) -= 8LL;
    }
    goto LABEL_149;
  }
LABEL_147:
  v44 = sub_146753320((unsigned int)v169, a1);
  v22 = v44;
  v133 = v44;
  if ( v44 == 0 )
    return 0;
  (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v44 + 680LL))(v44, v44);
  v45 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v22 + 304LL))(v22);
  sub_1467AB910(v22, v45);
LABEL_149:
  v46 = sub_14021A140(v38, v29, v30, v28);
  v137 = &v140;
  v47 = (*(__int64 (__fastcall **)(__int64, char *))(*(_QWORD *)v22 + 272LL))(v22, v148);
  __wind
  {
    v140 = 0;
    v140 = *(_OWORD *)v47;
    *(_QWORD *)v47 = 0;
    *(_QWORD *)(v47 + 8) = 0;
    v48 = sub_146EC61B0(v46, &v140);
  }
  __unwind
  {
    sub_1401566D0(v148);
  }
  v49 = v149;
  if ( v149 != nullptr )
  {
    if ( _InterlockedExchangeAdd(v149 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v49)(v49);
      if ( _InterlockedExchangeAdd(v49 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v49 + 8LL))(v49);
    }
    v22 = v133;
  }
  if ( v48 != 0 && (unsigned int)sub_1467A2AF0((unsigned int)v169) - 2 <= 1 && (unsigned __int8)sub_1467A3730(v22) != 0 )
  {
    v50 = (_QWORD *)(*(__int64 (__fastcall **)(__int64, char *))(*(_QWORD *)v22 + 272LL))(v22, v150);
    __wind
    {
      sub_146EC8870(*v50);
    }
    __unwind
    {
      sub_1401566D0(v150);
    }
    v51 = v151;
    if ( v151 != nullptr )
    {
      if ( _InterlockedExchangeAdd(v151 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v51)(v51);
        if ( _InterlockedExchangeAdd(v51 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v51 + 8LL))(v51);
      }
      v22 = v133;
    }
  }
  sub_146693120(a1, (unsigned int)v169);
  v52 = *(__int64 **)(a1 + 200928);
  if ( v52 == *(__int64 **)(a1 + 200936) )
  {
    sub_1401C06F0(a1 + 200920, v52, &v133);
    v22 = v133;
  }
  else
  {
    *v52 = v22;
    *(_QWORD *)(a1 + 200928) += 8LL;
  }
  if ( (*(unsigned __int8 (__fastcall **)(__int64, __int64, __int64))(*(_QWORD *)v22 + 40LL))(v22, v170, v171) == 0 )
  {
    v53 = *(_QWORD **)(a1 + 24LL * v169 + 100704);
    v54 = *(_QWORD **)(a1 + 24LL * v169 + 100696);
    if ( v53 == v54 )
      goto LABEL_172;
    do
    {
      if ( *v54 == v22 )
        break;
      ++v54;
    }
    while ( v54 != v53 );
    if ( v53 == v54 )
    {
LABEL_172:
      if ( (unsigned __int8)sub_146682350((unsigned int)v169) == 0 )
      {
        v55 = a1 + 8 * (v169 + 2LL * v169 + 12587);
        v56 = *(__int64 **)(v55 + 8);
        if ( v56 == *(__int64 **)(v55 + 16) )
        {
          sub_1401C06F0(v55, v56, &v133);
        }
        else
        {
          *v56 = v22;
          *(_QWORD *)(v55 + 8) += 8LL;
        }
      }
    }
    *(_QWORD *)(a1 + 200928) -= 8LL;
    if ( (unsigned __int8)sub_146683B80(a1, (unsigned int)v169) != 0 )
    {
      v57 = sub_14021A860();
      v58 = v169;
      v59 = sub_146E8C7D0(&unk_14B0AA2C0);
      v60 = sub_146E8CF20(&v137, v59, v58);
      __wind
      {
        v61 = (wchar_t *)sub_14014F430(v60);
        v62 = sub_146E8C7D0(&unk_14B0AA320);
        v63 = sub_146E8C7D0(&unk_14B0AA090);
        sub_146E938E0(v57, 0, v63, v62, 11057, (__int64)&qword_14EF545F0, v61, v132);
      }
      __unwind
      {
        sub_140156DE0(&v137);
      }
      sub_146E8C910(&v137);
    }
    return 0;
  }
  *(_QWORD *)(a1 + 200928) -= 8LL;
  v64 = sub_1421807C0();
  if ( (unsigned __int8)sub_144922030(v64, &v169) != 0 )
  {
    v65 = sub_1421807C0();
    sub_144921520(v65, &v169);
  }
  v66 = v169;
  if ( (unsigned __int8)sub_146683B80(a1, (unsigned int)v169) != 0 )
  {
    v67 = sub_14021A860();
    v68 = v169;
    v69 = sub_146E8C7D0(&unk_14B0AA380);
    v70 = sub_146E8CF20(v152, v69, v68);
    __wind
    {
      v71 = (wchar_t *)sub_14014F430(v70);
      v72 = sub_146E8C7D0(&unk_14B0AA320);
      v73 = sub_146E8C7D0(&unk_14B0AA090);
      sub_146E938E0(v67, 0, v73, v72, 11093, (__int64)&qword_14EF545F0, v71, v132);
    }
    __unwind
    {
      sub_140156DE0(v152);
    }
    sub_146E8C910(v152);
    v66 = v169;
  }
  if ( v66 != 2 )
  {
    v74 = *(_QWORD **)(a1 + 24LL * v66 + 480);
    v75 = a1 + 8 * (3LL * v66 + 59);
    v76 = *(_QWORD **)v75;
    if ( v74 == *(_QWORD **)v75 )
      goto LABEL_189;
    do
    {
      if ( *v76 == v22 )
        break;
      ++v76;
    }
    while ( v76 != v74 );
    if ( v74 == v76 )
    {
LABEL_189:
      v77 = *(__int64 **)(v75 + 8);
      if ( v77 == *(__int64 **)(v75 + 16) )
      {
        sub_1401C06F0(a1 + 8 * (3LL * v66 + 59), v77, &v133);
        v22 = v133;
      }
      else
      {
        *v77 = v22;
        *(_QWORD *)(v75 + 8) += 8LL;
      }
      v66 = v169;
    }
  }
  if ( v66 == 499 || v66 == 489 || v66 == 1404 )
  {
    v78 = (volatile __int64 *)sub_1480A6E70();
    if ( retaddr != nullptr && (unsigned int)++dword_14EF54638 >= 0x64 )
    {
      dword_14EF54638 = 0;
      v79 = sub_146E8BA20(40);
      v80 = v79;
      v134 = (__int128 *)v79;
      __wind
      {
        if ( v79 != 0 )
        {
          *(_OWORD *)v79 = 0;
          *(_OWORD *)(v79 + 16) = 0;
          *(_QWORD *)(v79 + 32) = 0;
          *(_QWORD *)(v79 + 32) = 0;
        }
        else
        {
          v80 = 0;
        }
      }
      __unwind
      {
        j_j_scalable_free(v134, 40);
      }
      *(_DWORD *)v80 = 11;
      *(_QWORD *)(v80 + 16) = retaddr;
      *(_QWORD *)(v80 + 8) = 0;
      v81 = *((_QWORD *)NtCurrentTeb()->ThreadLocalStoragePointer + (unsigned int)TlsIndex);
      if ( *(_BYTE *)(v81 + 420624) == 0 )
        _dyn_tls_on_demand_init();
      *(_BYTE *)(v80 + 24) = *(_DWORD *)(v81 + 7748) != 3857697;
      *(_DWORD *)(v80 + 28) = 0;
      *(_QWORD *)_InterlockedExchange64(v78, v80 + 32) = v80 + 32;
      v22 = v133;
    }
    v66 = v169;
  }
  sub_146693660(a1, (unsigned int)v66);
  v82 = (int)sub_1467A2AF0((unsigned int)v169);
  if ( (unsigned __int8)sub_145B8E370(v22) != 0 )
  {
    v83 = *(_QWORD *)(a1 + 48);
    v84 = *(void (__fastcall **)(__int64, __int128 *, __int64))(*(_QWORD *)v83 + 208LL);
    v134 = &v141;
    v85 = (*(__int64 (__fastcall **)(__int64, char *))(*(_QWORD *)v22 + 272LL))(v22, v153);
    __wind
    {
      v141 = 0;
      v141 = *(_OWORD *)v85;
      *(_QWORD *)v85 = 0;
      *(_QWORD *)(v85 + 8) = 0;
      v84(v83, &v141, 1);
    }
    __unwind
    {
      sub_1401566D0(v153);
    }
    v86 = v154;
LABEL_216:
    if ( v86 != nullptr )
    {
      if ( _InterlockedExchangeAdd(v86 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v86)(v86);
        if ( _InterlockedExchangeAdd(v86 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v86 + 8LL))(v86);
      }
      v22 = v133;
    }
    goto LABEL_221;
  }
  if ( (unsigned __int8)sub_1432C9BE0(v22) != 0 )
  {
    v135 = 3;
    v87 = sub_140457BE0(a1 + 96, &v135);
    v88 = *(_QWORD *)v87;
    v89 = *(void (__fastcall **)(__int64, __int128 *))(**(_QWORD **)v87 + 200LL);
    v134 = &v142;
    v90 = (*(__int64 (__fastcall **)(__int64, char *))(*(_QWORD *)v22 + 272LL))(v22, v155);
    __wind
    {
      v142 = 0;
      v142 = *(_OWORD *)v90;
      *(_QWORD *)v90 = 0;
      *(_QWORD *)(v90 + 8) = 0;
      v89(v88, &v142);
    }
    __unwind
    {
      sub_1401566D0(v155);
    }
    v86 = v156;
    goto LABEL_216;
  }
  if ( (v82 & 0x80000000) == 0LL && v82 < 5 )
  {
    v91 = *(_QWORD *)(a1 + 16 * v82 + 392);
    v92 = *(void (__fastcall **)(__int64, _QWORD *))(*(_QWORD *)v91 + 200LL);
    v134 = (__int128 *)v143;
    v93 = (*(__int64 (__fastcall **)(__int64, char *))(*(_QWORD *)v22 + 272LL))(v22, v157);
    __wind
    {
      *(_OWORD *)v143 = 0;
      *(_OWORD *)v143 = *(_OWORD *)v93;
      *(_QWORD *)v93 = 0;
      *(_QWORD *)(v93 + 8) = 0;
      v92(v91, v143);
    }
    __unwind
    {
      sub_1401566D0(v157);
    }
    v86 = v158;
    goto LABEL_216;
  }
LABEL_221:
  v19 = (unsigned int)sub_145778260() == 5;
  v94 = *(_QWORD *)v22;
  if ( v19 )
  {
    v95 = (_QWORD *)(*(__int64 (__fastcall **)(__int64, char *))(v94 + 280))(v22, v159);
    __wind
    {
      LOBYTE(v96) = 1;
      sub_146ED3A80(*v95, v96);
    }
    __unwind
    {
      sub_1401566D0(v159);
    }
    v97 = v160;
  }
  else
  {
    v98 = (_QWORD *)(*(__int64 (__fastcall **)(__int64, char *))(v94 + 280))(v22, v161);
    __wind
    {
      sub_146ED3A80(*v98, 0);
    }
    __unwind
    {
      sub_1401566D0(v161);
    }
    v97 = v162;
  }
  if ( v97 != nullptr )
  {
    if ( _InterlockedExchangeAdd(v97 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v97)(v97);
      if ( _InterlockedExchangeAdd(v97 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v97 + 8LL))(v97);
    }
    v22 = v133;
  }
  (*(void (__fastcall **)(__int64))(*(_QWORD *)v22 + 56LL))(v22);
  v99 = (*(unsigned __int8 (__fastcall **)(__int64))(*(_QWORD *)v22 + 536LL))(v22) == 0;
  if ( (*(unsigned __int8 (__fastcall **)(__int64))(*(_QWORD *)v22 + 864LL))(v22) != 0
    && v99
    && (*(unsigned __int8 (__fastcall **)(__int64))(*(_QWORD *)v22 + 872LL))(v22) != 0 )
  {
    *(_DWORD *)(a1 + 202012) = 0;
    *(_QWORD *)(a1 + 201992) = 0;
    v100 = *(_QWORD *)(a1 + 202000);
    if ( v100 != v22 )
    {
      if ( v100 != 0 )
        (*(void (__fastcall **)(__int64))(*(_QWORD *)v100 + 192LL))(v100);
      *(_QWORD *)(a1 + 202000) = v22;
      (*(void (__fastcall **)(__int64))(*(_QWORD *)v22 + 184LL))(v22);
    }
    *(_DWORD *)(a1 + 202008) = 2;
  }
  sub_1467AA620(v22, *(unsigned int *)(a1 + 202636));
  v101 = sub_145EFAFB0();
  if ( v101 != 0 )
    (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v101 + 8720LL))(v101, v22);
  (*(void (__fastcall **)(__int64))(*(_QWORD *)v22 + 528LL))(v22);
  v102 = sub_144A7FD70();
  if ( (unsigned __int8)sub_145667DA0(v102, (unsigned int)v169) != 0 )
  {
    *(_OWORD *)&v138[1] = 0;
    v139 = 0;
    __wind
    {
      v136 = v169;
      sub_140154010(&v138[1], v138[2], &v136);
      v103 = sub_144A7FD70();
      sub_145669720(v103, &v138[1]);
      v104 = sub_144A7FD70();
      sub_145668DB0(v104, 4);
    }
    __unwind
    {
      sub_140156720(&v138[1]);
    }
    v105 = v138[1];
    if ( v138[1] != 0 )
    {
      v106 = 4 * ((v139 - v138[1]) >> 2);
      if ( v106 >= 0x1000 )
      {
        v106 += 39LL;
        v105 = *(_QWORD *)(v138[1] - 8LL);
        if ( (unsigned __int64)(v138[1] - v105 - 8) > 0x1F )
          invalid_parameter_noinfo_noreturn();
      }
      j_j_scalable_free(v105, v106);
      *(_OWORD *)&v138[1] = 0;
      v139 = 0;
    }
  }
  if ( *(_BYTE *)(a1 + 204893) != 0 )
  {
    v107 = sub_1401DCCB0();
    if ( (unsigned __int16)sub_1403F5B80(v107, 279) == 0 )
    {
      LOBYTE(v108) = 1;
      if ( (unsigned __int8)sub_146682140(a1, 4033, v108) == 0 )
      {
        if ( qword_14E634410 == nullptr )
        {
          v109 = sub_146E8BA20(2760);
          v134 = (__int128 *)v109;
          __wind
          {
            if ( v109 != 0 )
              v110 = (_LocaleUpdate *)sub_1402CC150(v109);
            else
              v110 = nullptr;
          }
          __unwind
          {
            j_j_scalable_free(v134, 2760);
          }
          qword_14E634410 = v110;
          (**(void (__fastcall ***)(_LocaleUpdate *))v110)(v110);
        }
        if ( (unsigned __int8)sub_1402CD740() == 0 )
        {
          v111 = *(_QWORD **)(a1 + 204896);
          v112 = (__int64 *)v111[1];
          v113 = v111;
          v114 = (unsigned int)v169;
          while ( *((_BYTE *)v112 + 25) == 0 )
          {
            if ( *((_DWORD *)v112 + 7) >= v169 )
            {
              v113 = v112;
              v112 = (__int64 *)*v112;
            }
            else
            {
              v112 = (__int64 *)v112[2];
            }
          }
          if ( *((_BYTE *)v113 + 25) == 0 && v169 >= *((_DWORD *)v113 + 7) && v113 != v111 )
          {
            v115 = (__int64 *)*v111;
            while ( *((_BYTE *)v115 + 25) == 0 )
            {
              v116 = *((unsigned int *)v115 + 7);
              if ( (_DWORD)v116 != (_DWORD)v114 )
              {
                LOBYTE(v114) = 1;
                if ( (unsigned __int8)sub_146682140(a1, v116, v114) != 0 )
                {
                  sub_14668C520(a1, 4033, 0, 0);
                  if ( *(_QWORD *)&qword_14E683C78 != 0 )
                  {
                    v120 = sub_14723C170(101036153);
                    v121 = sub_14668C520(*(_QWORD *)&qword_14E683C78, 2875, v120, 0);
                  }
                  else
                  {
                    v121 = 0;
                  }
                  v122 = _RTDynamicCast(v121, 0, &off_14DCB4760, &off_14DFF6540, 0);
                  if ( v122 != 0 )
                  {
                    v123 = dword_14F1C092C;
                    v134 = (__int128 *)v165;
                    v124 = sub_14723C170(101036155);
                    v125 = sub_14014C7B0(v165, v124);
                    sub_1416D6D50(v122, v125, v123);
                    sub_1416D2F30(v122, &v144);
                    __wind
                    {
                      v126 = v144;
                      if ( v144 != 0 )
                      {
                        v127 = *(void (__fastcall **)(__int64, __int64, __int64, _QWORD *))(*(_QWORD *)v144 + 688LL);
                        v138[0] = qword_14F0FE8D8;
                        v134 = (__int128 *)v166;
                        v128 = sub_14723C170(101036154);
                        v129 = sub_14014C7B0(v166, v128);
                        v127(v126, v129, 1, v138);
                      }
                      v134 = (__int128 *)v163;
                      v164 = nullptr;
                      __wind
                      {
                        v163[0] = control_event::XW4TYPE::Z::_Func_impl_no_alloc<`CNRDInterfaceManager::openPopupWindow'::`145'::_lambda_1_,APEAVIRDPopupWindow * const,enum ENUM_POPUP_WINDOW_TYPE,void *,__int64>::`vftable';
                        v164 = v163;
                      }
                      __unwind
                      {
                        sub_1401EDFD0(v134);
                      }
                      sub_1416D1E50(v122, 13, v163);
                    }
                    __unwind
                    {
                      sub_1401566D0(&v144);
                    }
                    v130 = v145;
                    if ( v145 != nullptr )
                    {
                      if ( _InterlockedExchangeAdd(v145 + 2, 0xFFFFFFFF) == 1 )
                      {
                        (**(void (__fastcall ***)(volatile signed __int32 *))v130)(v130);
                        if ( _InterlockedExchangeAdd(v130 + 3, 0xFFFFFFFF) == 1 )
                          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v130 + 8LL))(v130);
                      }
                      return v133;
                    }
                  }
                  return v22;
                }
                v114 = (unsigned int)v169;
              }
              v117 = (__int64 **)v115[2];
              if ( *((_BYTE *)v117 + 25) != 0 )
              {
                for ( i = (__int64 *)v115[1]; *((_BYTE *)i + 25) == 0; i = (__int64 *)i[1] )
                {
                  if ( v115 != (__int64 *)i[2] )
                    break;
                  v115 = i;
                }
                v115 = i;
              }
              else
              {
                v115 = (__int64 *)v115[2];
                for ( j = *v117; *((_BYTE *)j + 25) == 0; j = (__int64 *)*j )
                  v115 = j;
              }
            }
          }
        }
      }
    }
  }
  return v22;
}
