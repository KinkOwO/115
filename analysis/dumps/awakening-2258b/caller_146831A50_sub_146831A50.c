// sub_146831A50  va=0x146831A50  size=1828

__int64 __fastcall sub_146831A50(__int64 a1, int a2)
{
  __int64 result; // rax
  int v4; // r12d
  unsigned int *v5; // rbx
  unsigned int i; // edi
  unsigned int *v7; // rbx
  unsigned int v8; // r13d
  int v9; // r14d
  int j; // edi
  __int64 v11; // rax
  __int64 v12; // rax
  __int64 v13; // rax
  _DWORD *v14; // rsi
  int v15; // r9d
  int v16; // r8d
  int v17; // eax
  __int64 v18; // rcx
  __int64 v19; // rbx
  __int64 v20; // rax
  __int64 v21; // rax
  __int64 v22; // rdx
  _WORD *v23; // rax
  __int64 v24; // r8
  _WORD *v25; // rax
  char v26; // bl
  unsigned __int64 v27; // rdx
  __int64 v28; // rcx
  unsigned __int64 v29; // rdx
  __int64 v30; // rcx
  unsigned __int64 v31; // rdx
  __int64 v32; // rcx
  unsigned __int64 v33; // rdx
  __int64 v34; // rcx
  int v35; // r14d
  __int64 v36; // rax
  __int64 v37; // rdx
  unsigned __int16 v38; // di
  unsigned int v39; // r12d
  __int64 v40; // rcx
  __int64 v41; // rcx
  __int64 v42; // rax
  unsigned int v43; // r13d
  __int64 v44; // rcx
  __int64 v45; // rcx
  _DWORD *v46; // rbx
  unsigned __int8 v47; // si
  int v48; // eax
  int v49; // ecx
  unsigned int v50; // eax
  _DWORD *v51; // rcx
  _LocaleUpdate *v52; // rcx
  __int64 v53; // rax
  _LocaleUpdate *v54; // rcx
  __int64 v55; // rax
  __int64 v56; // rcx
  __int64 v57; // rax
  __int64 v58; // rcx
  __int64 v59; // rax
  __int64 v60; // rcx
  __int64 v61; // rax
  __int64 v62; // rcx
  __int64 v63; // rax
  __int64 v64; // rcx
  __int64 v65; // rax
  __int64 v66; // rcx
  __int64 v67; // rax
  __int64 v68; // rcx
  __int64 v69; // rax
  __int64 v70; // rcx
  __int64 v71; // rax
  __int64 v72; // rcx
  char v73; // [rsp+38h] [rbp-D0h]
  int v74; // [rsp+3Ch] [rbp-CCh] BYREF
  __int64 v75; // [rsp+40h] [rbp-C8h] BYREF
  __int64 v76; // [rsp+48h] [rbp-C0h] BYREF
  _QWORD *v77; // [rsp+50h] [rbp-B8h]
  _QWORD v78[4]; // [rsp+58h] [rbp-B0h] BYREF
  __int64 v79; // [rsp+78h] [rbp-90h]
  __int64 v80; // [rsp+80h] [rbp-88h]
  __int64 v81; // [rsp+88h] [rbp-80h] BYREF
  char v82; // [rsp+95h] [rbp-73h]
  _BYTE v83[18]; // [rsp+96h] [rbp-72h]
  _QWORD v84[2]; // [rsp+A8h] [rbp-60h] BYREF
  __int64 v85; // [rsp+B8h] [rbp-50h]
  unsigned __int64 v86; // [rsp+C0h] [rbp-48h]
  _QWORD v87[2]; // [rsp+C8h] [rbp-40h] BYREF
  __m128i si128; // [rsp+D8h] [rbp-30h]
  _QWORD v89[2]; // [rsp+F0h] [rbp-18h] BYREF
  __m128i v90; // [rsp+100h] [rbp-8h]
  void *retaddr; // [rsp+150h] [rbp+48h]

  v80 = -2;
  LODWORD(v75) = a2;
  result = sub_145ADA180(qword_14E683C80);
  if ( (int)result <= 0 )
  {
    v4 = 255;
    v5 = *(unsigned int **)(a1 + 1392);
    for ( i = 1; v5 != *(unsigned int **)(a1 + 1400); v5 += 2 )
    {
      if ( (unsigned int)sub_14500CD30(*v5) == 2 )
        v4 = 1;
    }
    v74 = 1;
    v7 = *(unsigned int **)(a1 + 1368);
    if ( v7 != *(unsigned int **)(a1 + 1376) )
    {
      while ( 1 )
      {
        v8 = *v7;
        LODWORD(v77) = sub_146830600(a1, i, v7[1]);
        v9 = sub_145AD9120(qword_14E683C80, v8);
        if ( sub_145EFAFB0() != 0 )
        {
          for ( j = 0; j < 48; ++j )
          {
            v11 = sub_145EFAFB0();
            v12 = (*(__int64 (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v11 + 7944LL))(v11, (unsigned int)j);
            if ( v12 != 0 )
            {
              v13 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v12 + 152LL))(v12);
              v14 = (_DWORD *)(v13 + 24);
              LOBYTE(v15) = 1;
              LOBYTE(v16) = 50;
              sub_1480A6620(v13 + 24, 4, v16, v15, v13 + 28);
              if ( *v14 == v8 )
                ++v9;
            }
          }
          i = v74;
        }
        v17 = v9 / (int)v77;
        if ( v9 / (int)v77 == 0 )
          break;
        if ( v17 >= v4 )
          v17 = v4;
        v4 = v17;
        v7 += 2;
        v74 = ++i;
        if ( v7 == *(unsigned int **)(a1 + 1376) )
          goto LABEL_20;
      }
      v4 = 0;
    }
LABEL_20:
    if ( v4 < (int)v75 )
    {
      v18 = *(unsigned int *)(a1 + 1764);
LABEL_22:
      v19 = *(_QWORD *)(a1 + 848);
      v20 = sub_14723C170(v18);
      return sub_14668C520(v19, 2875, v20, 0);
    }
    v21 = *(_QWORD *)(a1 + 1352);
    if ( v21 != 0 && *(_DWORD *)(v21 + 8) != 0 )
      v22 = *(_QWORD *)(a1 + 1360);
    else
      v22 = 0;
    if ( (unsigned int)sub_145AD5C20(qword_14E683C80, v22) == -1 )
    {
      v18 = *(unsigned int *)(a1 + 1760);
      goto LABEL_22;
    }
    v77 = v78;
    if ( byte_14DC67898 != 0 )
    {
      LODWORD(v75) = 590;
      v23 = (_WORD *)sub_146E8C7D0(&unk_14B0E39F0);
      v84[0] = 0;
      v85 = 0;
      v86 = 7;
      v24 = -1;
      do
        ++v24;
      while ( v23[v24] != 0 );
      sub_14014C8D0(v84, v23);
      __eh34_enter_wind_state(-1, 0);
      v25 = (_WORD *)sub_140438530(v89, v84, &v75);
      __eh34_enter_wind_state(0, 1);
      v26 = 3;
      __eh34_enter_wind_state(1, 2);
      __eh34_enter_wind_state(2, 3);
    }
    else
    {
      LODWORD(v75) = 0;
      v81 = 0;
      *(_QWORD *)&v83[2] = 0;
      *(_QWORD *)&v83[10] = 7;
      sub_14014C8D0(&v81, (void *)&Source);
      __eh34_enter_wind_state(-1, 0);
      __eh34_enter_wind_state(0, 1);
      __eh34_enter_wind_state(1, 2);
      v25 = (_WORD *)sub_140438530(v87, &v81, &v75);
      __eh34_enter_wind_state(2, 3);
      v26 = 12;
    }
    v73 = v26;
    v78[0] = 0;
    v78[2] = 0;
    v78[3] = 0;
    qmemcpy(v78, v25, sizeof(v78));
    *((_QWORD *)v25 + 2) = 0;
    *((_QWORD *)v25 + 3) = 7;
    *v25 = 0;
    __wind
    {
      LODWORD(v79) = *((_DWORD *)v25 + 8);
    }
    __unwind
    {
      unknown_libname_4(v77);
    }
    sub_145ACB980(qword_14E683C80, 12, v78);
    if ( __eh34_unwind(3) )
    {
      if ( (v26 & 8) != 0 )
        sub_1401512E0(v87);
      __eh34_continue_unwinding(3, 2);
    }
    __eh34_exit_wind_state(3, 2);
    if ( (v26 & 8) != 0 )
    {
      v26 &= ~8u;
      v73 = v26;
      if ( si128.m128i_i64[1] >= 8uLL )
      {
        v27 = 2 * si128.m128i_i64[1] + 2;
        v28 = v87[0];
        if ( v27 >= 0x1000 )
        {
          v27 = 2 * si128.m128i_i64[1] + 41;
          v28 = *(_QWORD *)(v87[0] - 8LL);
          if ( (unsigned __int64)(v87[0] - v28 - 8) > 0x1F )
            invalid_parameter_noinfo_noreturn();
        }
        j_j_scalable_free(v28, v27);
      }
      si128 = _mm_load_si128((const __m128i *)&xmmword_1491AB7C0);
      LOWORD(v87[0]) = 0;
    }
    if ( __eh34_unwind(2) )
    {
      if ( (v73 & 4) != 0 )
        unknown_libname_4(&v81);
      __eh34_continue_unwinding(2, 1);
    }
    __eh34_exit_wind_state(2, 1);
    if ( (v26 & 4) != 0 )
    {
      v26 &= ~4u;
      v73 = v26;
      if ( *(_QWORD *)&v83[10] >= 8u )
      {
        v29 = 2LL * *(_QWORD *)&v83[10] + 2;
        v30 = v81;
        if ( v29 >= 0x1000 )
        {
          v29 = 2LL * *(_QWORD *)&v83[10] + 41;
          v30 = *(_QWORD *)(v81 - 8);
          if ( (unsigned __int64)(v81 - v30 - 8) > 0x1F )
            invalid_parameter_noinfo_noreturn();
        }
        j_j_scalable_free(v30, v29);
      }
      *(_QWORD *)&v83[2] = 0;
      *(_QWORD *)&v83[10] = 7;
      LOWORD(v81) = 0;
    }
    if ( __eh34_unwind(1) )
    {
      if ( (v73 & 2) != 0 )
        sub_1401512E0(v89);
      __eh34_continue_unwinding(1, 0);
    }
    __eh34_exit_wind_state(1, 0);
    if ( (v26 & 2) != 0 )
    {
      v26 &= ~2u;
      v73 = v26;
      if ( v90.m128i_i64[1] >= 8uLL )
      {
        v31 = 2 * v90.m128i_i64[1] + 2;
        v32 = v89[0];
        if ( v31 >= 0x1000 )
        {
          v31 = 2 * v90.m128i_i64[1] + 41;
          v32 = *(_QWORD *)(v89[0] - 8LL);
          if ( (unsigned __int64)(v89[0] - v32 - 8) > 0x1F )
            invalid_parameter_noinfo_noreturn();
        }
        j_j_scalable_free(v32, v31);
      }
      v90 = _mm_load_si128((const __m128i *)&xmmword_1491AB7C0);
      LOWORD(v89[0]) = 0;
    }
    if ( __eh34_unwind(0) )
    {
      if ( (v73 & 1) != 0 )
        unknown_libname_4(v84);
      __eh34_propagate_exception_into_caller(0, -1);
    }
    __eh34_exit_wind_state(0, -1);
    if ( (v26 & 1) != 0 )
    {
      if ( v86 >= 8 )
      {
        v33 = 2 * v86 + 2;
        v34 = v84[0];
        if ( v33 >= 0x1000 )
        {
          v33 = 2 * v86 + 41;
          v34 = *(_QWORD *)(v84[0] - 8LL);
          if ( (unsigned __int64)(v84[0] - v34 - 8) > 0x1F )
            invalid_parameter_noinfo_noreturn();
        }
        j_j_scalable_free(v34, v33);
      }
      v85 = 0;
      v86 = 7;
      LOWORD(v84[0]) = 0;
    }
    v35 = *(_DWORD *)sub_14503CE30(a1);
    v36 = *(_QWORD *)(a1 + 1352);
    if ( v36 != 0 && *(_DWORD *)(v36 + 8) != 0 )
      v37 = *(_QWORD *)(a1 + 1360);
    else
      v37 = 0;
    result = sub_145AD5C20(qword_14E683C80, v37);
    v38 = result;
    if ( v35 >= 0 && (int)result >= 0 )
    {
      v39 = sub_14503CDD0(a1);
      v40 = *(_QWORD *)(a1 + 1352);
      if ( v40 != 0 && *(_DWORD *)(v40 + 8) != 0 )
        v41 = *(_QWORD *)(a1 + 1360);
      else
        v41 = 0;
      v42 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v41 + 152LL))(v41);
      v43 = sub_140256DC0(v42 + 16);
      v44 = *(_QWORD *)(a1 + 1352);
      if ( v44 != 0 && *(_DWORD *)(v44 + 8) != 0 )
        v45 = *(_QWORD *)(a1 + 1360);
      else
        v45 = 0;
      v46 = (_DWORD *)(*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v45 + 1048LL))(v45);
      sub_146E920A0(v46, &v74);
      v47 = v74;
      v48 = v74 + *v46 + 196;
      v49 = v46[1];
      if ( v49 != 0 && v48 != 0 && v49 != v48 && retaddr != nullptr )
      {
        sub_146D89B40(retaddr, v46);
        v47 = v74;
      }
      v50 = sub_14503CDD0(a1);
      result = sub_145ABB480(v50);
      if ( result != 0 )
      {
        v51 = *(_DWORD **)(a1 + 1352);
        if ( v51 != nullptr && v51[2] != 0 )
        {
          v51 = *(_DWORD **)(a1 + 1360);
          if ( v51 != nullptr
            && *(_BYTE *)(result + 6440) == 1
            && *(_BYTE *)((*(__int64 (__fastcall **)(_DWORD *))(*(_QWORD *)v51 + 1080LL))(v51) + 285) != 0 )
          {
            *(_DWORD *)v83 = -1;
            *(_DWORD *)&v83[7] = -1;
            v82 = 1;
            v83[4] = 0;
            *(_WORD *)&v83[5] = v38;
            v52 = qword_14E6387A8;
            if ( qword_14E6387A8 == nullptr )
            {
              v53 = sub_146E8BA20(176);
              v75 = v53;
              __wind
              {
                if ( v53 != 0 )
                  v54 = (_LocaleUpdate *)sub_140B896A0(v53);
                else
                  v54 = nullptr;
              }
              __unwind
              {
                j_j_scalable_free(v75, 176);
              }
              qword_14E6387A8 = v54;
              (**(void (__fastcall ***)(_LocaleUpdate *))v54)(v54);
              v52 = qword_14E6387A8;
            }
            sub_140B8AD40((__int64)v52, (__int64)&v81);
          }
        }
        v55 = sub_146D74000(v51);
        sub_146D746E0(v55, 271);
        v57 = sub_146D74000(v56);
        sub_146D76180(v57, (unsigned __int16)v35);
        v59 = sub_146D74000(v58);
        sub_146D75CE0(v59, v39);
        v61 = sub_146D74000(v60);
        sub_146D76180(v61, v38);
        v63 = sub_146D74000(v62);
        sub_146D75CE0(v63, v43);
        v65 = sub_146D74000(v64);
        sub_146D75CC0(v65, v47);
        v67 = sub_146D74000(v66);
        sub_146D76180(v67, 0xFFFF);
        v69 = sub_146D74000(v68);
        sub_146D75CE0(v69, 0xFFFF);
        LODWORD(v76) = 0;
        v71 = sub_146D74000(v70);
        sub_146D75B10(v71, &v76, 4);
        return sub_146D75AF0(v72);
      }
    }
  }
  return result;
}
